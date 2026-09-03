package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

// ---------------------------------------------------------------------------
// demo — functional proof: on / colours / dim / off on every bulb
// ---------------------------------------------------------------------------

type stepResult struct {
	Step      string  `json:"step"`
	Bulb      string  `json:"bulb"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

func cmdDemo(adapter *bluetooth.Adapter, macs []string, mode WriteMode, report *Report) error {
	bulbs, results := connectAll(adapter, macs, *flagTimeout, logf)
	report.Bulbs = results
	defer disconnectAll(bulbs, logf)
	if len(bulbs) == 0 {
		return errors.New("no bulb connected")
	}

	// Security probe — how the OS stack sees each Hue characteristic
	// (properties, protection level). Read this first when writes fail with
	// ATT 0x05/0x0F: it tells whether the link security was ever raised.
	for _, b := range bulbs {
		logf("%-20s security probe (-protection %s):", b.Label, protectionMode)
		for _, line := range b.SecurityProbe() {
			logf("    %s", line)
		}
	}

	// Initial state read — proves GATT reads work without any pairing prompt.
	for _, b := range bulbs {
		on, dOn, err1 := b.ReadPower()
		bri, dBri, err2 := b.ReadBrightness()
		logf("%-20s power=%v (%.0fms) brightness=%d (%.0fms) err=%v/%v", b.Label, on, ms(dOn), bri, ms(dBri), err1, err2)
	}

	type step struct {
		name string
		fn   func(*Bulb) (time.Duration, error)
	}
	col := func(name string) XY { c, _ := parseColor(name); return c }
	steps := []step{
		{"power on", func(b *Bulb) (time.Duration, error) { return b.SetPower(true, mode) }},
		{"brightness 254", func(b *Bulb) (time.Duration, error) { return b.SetBrightness(254, mode) }},
		{"colour red", func(b *Bulb) (time.Duration, error) { return b.SetColor(col("red"), mode) }},
		{"colour green", func(b *Bulb) (time.Duration, error) { return b.SetColor(col("green"), mode) }},
		{"colour blue", func(b *Bulb) (time.Duration, error) { return b.SetColor(col("blue"), mode) }},
		{"colour white", func(b *Bulb) (time.Duration, error) { return b.SetColor(col("white"), mode) }},
		{"brightness 60", func(b *Bulb) (time.Duration, error) { return b.SetBrightness(60, mode) }},
		{"power off", func(b *Bulb) (time.Duration, error) { return b.SetPower(false, mode) }},
	}

	var all Samples
	var steps2 []stepResult
	failures := 0
	for _, st := range steps {
		for _, b := range bulbs {
			if st.name[:6] == "colour" && !b.Has(uuidHueColorXY) {
				logf("%-14s %-20s skipped (no colour characteristic)", st.name, b.Label)
				continue
			}
			d, err := st.fn(b)
			r := stepResult{Step: st.name, Bulb: b.Label, LatencyMs: ms(d)}
			if err != nil {
				r.Error = err.Error()
				failures++
				logf("%-14s %-20s FAILED after %.0fms: %v", st.name, b.Label, ms(d), err)
			} else {
				all.Add(d)
				logf("%-14s %-20s ok %.0fms", st.name, b.Label, ms(d))
			}
			steps2 = append(steps2, r)
		}
		time.Sleep(700 * time.Millisecond) // let the eye see each state
	}

	st := all.Stats()
	report.Sections["demo"] = map[string]any{"steps": steps2, "write_latency": st, "failures": failures, "write_mode": mode.String()}
	logf("demo summary: %d bulb(s) connected / %d requested, %d write failure(s), latency %s", len(bulbs), len(macs), failures, st)
	if st.MaxMs > 5000 {
		report.Note("a write took > 5 s: on Windows this usually means the OS raised a pairing/consent prompt in the background — check the notification area")
		logf("NOTE: %s", report.Notes[len(report.Notes)-1])
	}
	if failures > 0 {
		return fmt.Errorf("%d write(s) failed", failures)
	}
	return nil
}

// ---------------------------------------------------------------------------
// bench — latency + group desync, sequential vs parallel
// ---------------------------------------------------------------------------

type groupStrategy struct {
	Name        string           `json:"name"`
	PerBulb     map[string]Stats `json:"per_bulb_write_ms"`
	Desync      Stats            `json:"desync_first_last_ms"`
	GroupTotal  Stats            `json:"group_total_ms"`
	Failures    int              `json:"failures"`
	Iterations  int              `json:"iterations"`
	Description string           `json:"description"`
}

func runGroupWrites(bulbs []*Bulb, iterations int, parallel bool, mode WriteMode, interval time.Duration, onWrite func(time.Duration)) groupStrategy {
	gs := groupStrategy{Name: "sequential", PerBulb: map[string]Stats{}, Iterations: iterations,
		Description: "one goroutine writes bulb 1, then bulb 2, ... (server-like simple loop)"}
	if parallel {
		gs.Name = "parallel"
		gs.Description = "one goroutine per bulb, all writes issued at the same instant"
	}
	per := map[string]*Samples{}
	for _, b := range bulbs {
		per[b.Label] = &Samples{}
	}
	var desync, total Samples
	colours := []XY{mustColor("red"), mustColor("blue")}

	for i := 0; i < iterations; i++ {
		c := colours[i%len(colours)]
		done := make([]time.Time, len(bulbs))
		errs := make([]error, len(bulbs))
		start := time.Now()
		if parallel {
			var wg sync.WaitGroup
			for idx, b := range bulbs {
				wg.Add(1)
				go func(idx int, b *Bulb) {
					defer wg.Done()
					d, err := b.SetColor(c, mode)
					done[idx] = time.Now()
					errs[idx] = err
					if err == nil {
						per[b.Label].Add(d)
						if onWrite != nil {
							onWrite(d)
						}
					}
				}(idx, b)
			}
			wg.Wait()
		} else {
			for idx, b := range bulbs {
				d, err := b.SetColor(c, mode)
				done[idx] = time.Now()
				errs[idx] = err
				if err == nil {
					per[b.Label].Add(d)
					if onWrite != nil {
						onWrite(d)
					}
				}
			}
		}
		first, last := done[0], done[0]
		okCount := 0
		for idx := range bulbs {
			if errs[idx] != nil {
				gs.Failures++
				continue
			}
			okCount++
			if done[idx].Before(first) {
				first = done[idx]
			}
			if done[idx].After(last) {
				last = done[idx]
			}
		}
		if okCount >= 2 {
			desync.Add(last.Sub(first))
		}
		if okCount > 0 {
			total.Add(last.Sub(start))
		}
		vlogf("%s iter %d/%d: total=%.0fms desync=%.0fms failures=%d", gs.Name, i+1, iterations, ms(last.Sub(start)), ms(last.Sub(first)), gs.Failures)
		time.Sleep(interval)
	}
	for k, s := range per {
		gs.PerBulb[k] = s.Stats()
	}
	gs.Desync = desync.Stats()
	gs.GroupTotal = total.Stats()
	return gs
}

func mustColor(name string) XY {
	c, err := parseColor(name)
	if err != nil {
		panic(err)
	}
	return c
}

func cmdBench(adapter *bluetooth.Adapter, macs []string, mode WriteMode, report *Report) error {
	bulbs, results := connectAll(adapter, macs, *flagTimeout, logf)
	report.Bulbs = results
	defer disconnectAll(bulbs, logf)
	if len(bulbs) == 0 {
		return errors.New("no bulb connected")
	}
	for _, b := range bulbs {
		if !b.Has(uuidHueColorXY) {
			return fmt.Errorf("%s has no colour characteristic — bench needs colour bulbs", b.Label)
		}
		if _, err := b.SetPower(true, mode); err != nil {
			return err
		}
		if _, err := b.SetBrightness(200, mode); err != nil {
			return err
		}
	}

	var strategies []groupStrategy
	for _, parallel := range []bool{false, true} {
		logf("--- strategy %s: %d iterations, %s between groups ---", map[bool]string{false: "sequential", true: "parallel"}[parallel], *flagIterations, *flagInterval)
		gs := runGroupWrites(bulbs, *flagIterations, parallel, mode, *flagInterval, nil)
		for name, st := range gs.PerBulb {
			logf("  %-20s write latency %s", name, st)
		}
		logf("  desync first→last bulb : %s", gs.Desync)
		logf("  group total (start→last): %s", gs.GroupTotal)
		logf("  failures: %d", gs.Failures)
		strategies = append(strategies, gs)
	}

	// Single-bulb write procedures compared (request vs command) on bulb 0.
	procs := map[string]Stats{}
	for _, m := range []WriteMode{WriteRequest, WriteCommand} {
		var s Samples
		fails := 0
		for i := 0; i < *flagIterations; i++ {
			d, err := bulbs[0].SetBrightness(100+(i%2)*100, m)
			if err != nil {
				fails++
				continue
			}
			s.Add(d)
			time.Sleep(100 * time.Millisecond)
		}
		procs[m.String()] = s.Stats()
		logf("  single bulb %-20s write %-7s : %s (failures %d)", bulbs[0].Label, m.String(), s.Stats(), fails)
	}

	for _, b := range bulbs {
		_, _ = b.SetColor(mustColor("white"), mode)
	}
	report.Sections["bench"] = map[string]any{"strategies": strategies, "write_procedures_bulb0": procs, "write_mode": mode.String(), "bulbs_connected": len(bulbs)}
	return nil
}

// ---------------------------------------------------------------------------
// hold — persistent connections over time
// ---------------------------------------------------------------------------

type holdBulb struct {
	Label        string  `json:"label"`
	ReadsOK      int     `json:"reads_ok"`
	ReadsFailed  int     `json:"reads_failed"`
	Disconnects  int     `json:"disconnects"`
	Reconnects   int     `json:"reconnects"`
	ReconnectsKO int     `json:"reconnects_failed"`
	UptimePct    float64 `json:"uptime_pct"`
	ReadLatency  Stats   `json:"read_latency_ms"`
}

func cmdHold(adapter *bluetooth.Adapter, macs []string, report *Report) error {
	bulbs, results := connectAll(adapter, macs, *flagTimeout, logf)
	report.Bulbs = results
	defer disconnectAll(bulbs, logf)
	logf("hold: %d/%d bulb(s) connected simultaneously — holding for %s, liveness read every %s", len(bulbs), len(macs), *flagHoldDur, *flagKeepalive)
	if len(bulbs) == 0 {
		return errors.New("no bulb connected")
	}

	state := make([]holdBulb, len(bulbs))
	lat := make([]*Samples, len(bulbs))
	down := make([]time.Duration, len(bulbs))
	downSince := make([]time.Time, len(bulbs))
	for i, b := range bulbs {
		state[i].Label = b.Label
		lat[i] = &Samples{}
	}
	peak := len(bulbs)
	start := time.Now()
	ticker := time.NewTicker(*flagKeepalive)
	defer ticker.Stop()
	deadline := time.After(*flagHoldDur)

loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-ticker.C:
		}
		alive := 0
		for i, b := range bulbs {
			connected, cerr := b.Connected()
			_, d, rerr := b.ReadPower()
			if cerr == nil && connected && rerr == nil {
				state[i].ReadsOK++
				lat[i].Add(d)
				alive++
				if !downSince[i].IsZero() {
					down[i] += time.Since(downSince[i])
					downSince[i] = time.Time{}
				}
				continue
			}
			state[i].ReadsFailed++
			if downSince[i].IsZero() {
				state[i].Disconnects++
				downSince[i] = time.Now()
				logf("%-20s DOWN (connected=%v err=%v / %v)", b.Label, connected, cerr, rerr)
			}
			// Try to come back.
			nb, err := connectBulb(adapter, b.MAC, *flagTimeout)
			if err != nil {
				state[i].ReconnectsKO++
				logf("%-20s reconnect failed: %v", b.Label, err)
				continue
			}
			_ = b.Disconnect()
			bulbs[i] = nb
			state[i].Reconnects++
			logf("%-20s reconnected in %.0fms", nb.Label, ms(nb.ConnectDuration+nb.DiscoverDuration))
		}
		if alive > peak {
			peak = alive
		}
		logf("tick +%s: %d/%d alive", time.Since(start).Truncate(time.Second), alive, len(bulbs))
	}

	elapsed := time.Since(start)
	for i := range bulbs {
		if !downSince[i].IsZero() {
			down[i] += time.Since(downSince[i])
		}
		state[i].UptimePct = round1(100 * (1 - float64(down[i])/float64(elapsed)))
		state[i].ReadLatency = lat[i].Stats()
		logf("%-20s uptime=%.1f%% reads ok/ko=%d/%d disconnects=%d reconnects ok/ko=%d/%d read %s",
			state[i].Label, state[i].UptimePct, state[i].ReadsOK, state[i].ReadsFailed, state[i].Disconnects, state[i].Reconnects, state[i].ReconnectsKO, state[i].ReadLatency)
	}
	report.Sections["hold"] = map[string]any{
		"requested": len(macs), "connected_at_start": len(bulbs), "peak_simultaneous": peak,
		"duration_s": elapsed.Seconds(), "bulbs": state,
	}
	logf("hold summary: requested=%d connected=%d peak=%d over %s", len(macs), len(bulbs), peak, elapsed.Truncate(time.Second))
	return nil
}
