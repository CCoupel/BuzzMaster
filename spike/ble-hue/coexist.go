package main

// coexist — 2.4 GHz before/after protocol (issue #204 point 6).
//
// The program itself produces the numbers: it pings every buzzer at a fixed
// cadence through the whole run and buckets RTT/loss by phase, while an
// optional /ws/logs listener counts LED_SET ACK retries/expirations and buzzer
// WebSocket drops reported by the server. Phases (default):
//
//	baseline     no BLE activity at all (reference)
//	ble-idle     N bulbs connected, no traffic
//	ble-traffic  N bulbs connected, group colour change every -interval
//	ble-off      bulbs disconnected again (control: back to baseline?)
//
// Run it ON THE SERVER MACHINE (the Raspberry Pi or the Windows box) so that
// the ping path is the same radio path the server uses for the buzzers.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

type phaseProbe struct {
	Sent int      `json:"sent"`
	Lost int      `json:"lost"`
	RTT  *Samples `json:"-"`
}

type phaseProbeJSON struct {
	Sent    int     `json:"sent"`
	Lost    int     `json:"lost"`
	LossPct float64 `json:"loss_pct"`
	RTT     Stats   `json:"rtt_ms"`
}

type prober struct {
	mu    sync.Mutex
	phase string
	data  map[string]map[string]*phaseProbe // phase -> buzzer IP -> probe
	hosts []string
}

func newProber(hosts []string) *prober {
	return &prober{phase: "init", data: map[string]map[string]*phaseProbe{}, hosts: hosts}
}

func (p *prober) setPhase(name string) {
	p.mu.Lock()
	p.phase = name
	p.mu.Unlock()
}

func (p *prober) record(host string, rtt float64, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	byHost := p.data[p.phase]
	if byHost == nil {
		byHost = map[string]*phaseProbe{}
		p.data[p.phase] = byHost
	}
	pr := byHost[host]
	if pr == nil {
		pr = &phaseProbe{RTT: &Samples{}}
		byHost[host] = pr
	}
	pr.Sent++
	if ok {
		pr.RTT.AddMs(rtt)
	} else {
		pr.Lost++
	}
}

func (p *prober) run(ctx context.Context, interval, timeout time.Duration) {
	var wg sync.WaitGroup
	for _, h := range p.hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
				rtt, ok, _ := pingOnce(ctx, host, timeout)
				if ctx.Err() != nil {
					return
				}
				p.record(host, rtt, ok)
			}
		}(h)
	}
	wg.Wait()
}

func (p *prober) snapshot() map[string]map[string]phaseProbeJSON {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]map[string]phaseProbeJSON{}
	for phase, byHost := range p.data {
		out[phase] = map[string]phaseProbeJSON{}
		for host, pr := range byHost {
			loss := 0.0
			if pr.Sent > 0 {
				loss = round1(100 * float64(pr.Lost) / float64(pr.Sent))
			}
			out[phase][host] = phaseProbeJSON{Sent: pr.Sent, Lost: pr.Lost, LossPct: loss, RTT: pr.RTT.Stats()}
		}
	}
	return out
}

func cmdCoexist(adapter *bluetooth.Adapter, macs []string, mode WriteMode, report *Report) error {
	phases := splitList(*flagPhases)
	if len(phases) == 0 {
		return errors.New("-phases is empty")
	}

	// 1. Buzzers to probe.
	var hosts []string
	var buzzers []Buzzer
	if strings.EqualFold(*flagBuzzers, "auto") {
		var err error
		buzzers, err = fetchBuzzers(*flagServer)
		if err != nil {
			return fmt.Errorf("buzzer discovery: %w (pass -buzzers ip1,ip2 to bypass)", err)
		}
		for _, b := range buzzers {
			hosts = append(hosts, b.IP)
			logf("buzzer %-18s ip=%-15s status=%-12s name=%q team=%q", b.MAC, b.IP, b.Status, b.Name, b.Team)
		}
	} else {
		hosts = splitList(*flagBuzzers)
	}
	if len(hosts) == 0 {
		return errors.New("no buzzer IP to probe (server knows none, or -buzzers empty)")
	}
	sort.Strings(hosts)
	report.Sections["buzzers"] = buzzers

	// 2. Sanity ping: an unreachable host would only add noise.
	for _, h := range hosts {
		rtt, ok, raw := pingOnce(context.Background(), h, 1500*time.Millisecond)
		if !ok {
			logf("WARNING: %s does not answer ping (%s) — it will show 100%% loss", h, firstLine(raw))
		} else {
			logf("ping %-15s ok %.1fms", h, rtt)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Server logs (optional).
	var lw *LogWatcher
	if !*flagNoLogs {
		var err error
		lw, err = startLogWatcher(ctx, *flagServer)
		if err != nil {
			logf("WARNING: /ws/logs unavailable (%v) — continuing with ping only", err)
			lw = nil
		} else {
			logf("attached to %s/ws/logs — ACK retries / expirations / buzzer drops will be counted per phase", *flagServer)
		}
	}

	// 4. Probe loop for the whole run.
	pr := newProber(hosts)
	probeDone := make(chan struct{})
	go func() { pr.run(ctx, *flagPingInt, 1000*time.Millisecond); close(probeDone) }()

	// 5. Phases.
	var bulbs []*Bulb
	var connectResults []ConnectResult
	bleWrites := &Samples{}
	bleWriteFailures := 0
	var trafficStrategy *groupStrategy
	phaseTimes := map[string]map[string]time.Time{}

	setPhase := func(name string) {
		pr.setPhase(name)
		if lw != nil {
			lw.SetPhase(name)
		}
	}

	for _, ph := range phases {
		logf("=============== phase %-12s (%s) ===============", ph, *flagPhaseDur)
		phaseTimes[ph] = map[string]time.Time{"start": time.Now()}
		switch ph {
		case "baseline":
			setPhase(ph)
			time.Sleep(*flagPhaseDur)

		case "ble-idle":
			setPhase("ble-connecting")
			bulbs, connectResults = connectAll(adapter, macs, *flagTimeout, logf)
			report.Bulbs = connectResults
			if len(bulbs) == 0 {
				logf("WARNING: no bulb connected — BLE phases are meaningless")
			}
			setPhase(ph)
			time.Sleep(*flagPhaseDur)

		case "ble-traffic":
			if len(bulbs) == 0 {
				setPhase("ble-connecting")
				bulbs, connectResults = connectAll(adapter, macs, *flagTimeout, logf)
				report.Bulbs = connectResults
			}
			setPhase(ph)
			if len(bulbs) > 0 {
				for _, b := range bulbs {
					_, _ = b.SetPower(true, mode)
				}
				iterations := int(*flagPhaseDur / (*flagInterval + 50*time.Millisecond))
				if iterations < 1 {
					iterations = 1
				}
				gs := runGroupWrites(bulbs, iterations, false, mode, *flagInterval, func(d time.Duration) { bleWrites.Add(d) })
				bleWriteFailures += gs.Failures
				trafficStrategy = &gs
			} else {
				time.Sleep(*flagPhaseDur)
			}

		case "ble-off":
			disconnectAll(bulbs, logf)
			bulbs = nil
			time.Sleep(2 * time.Second) // let the radio settle
			setPhase(ph)
			time.Sleep(*flagPhaseDur)

		default:
			return fmt.Errorf("unknown phase %q (baseline|ble-idle|ble-traffic|ble-off)", ph)
		}
		phaseTimes[ph]["end"] = time.Now()
	}
	setPhase("done")
	disconnectAll(bulbs, logf)
	cancel()
	<-probeDone

	// 6. Results.
	probes := pr.snapshot()
	var logs map[string]LogCounters
	if lw != nil {
		logs = lw.Snapshot()
	}
	printCoexistTable(phases, hosts, probes, logs)

	if bleWrites.Len() > 0 {
		logf("BLE write latency during ble-traffic: %s (failures %d)", bleWrites.Stats(), bleWriteFailures)
	}
	report.Sections["coexist"] = map[string]any{
		"phases":              phases,
		"phase_duration_s":    flagPhaseDur.Seconds(),
		"ping_interval_ms":    ms(*flagPingInt),
		"hosts":               hosts,
		"probes":              probes,
		"server_logs":         logs,
		"ble_write_latency":   bleWrites.Stats(),
		"ble_write_failures":  bleWriteFailures,
		"ble_traffic":         trafficStrategy,
		"bulbs_connected":     len(connectResults),
		"phase_times":         phaseTimes,
		"log_watcher_errors":  errsOrNil(lw),
		"note_ack_received":   "ACK received lines are DEBUG level on the server: 0 does not mean no ACK, unless the server runs in DEBUG",
		"note_ping_semantics": "RTT measured from this machine to each buzzer with the OS ping; loss = no echo within 1 s",
	}
	return nil
}

func errsOrNil(lw *LogWatcher) []string {
	if lw == nil {
		return nil
	}
	return lw.Errors()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func printCoexistTable(phases, hosts []string, probes map[string]map[string]phaseProbeJSON, logs map[string]LogCounters) {
	fmt.Println()
	fmt.Println("PING RTT per phase (ms) — delta columns compare p95 with the baseline phase")
	fmt.Printf("%-12s %-15s %6s %6s %7s %7s %7s %7s %9s\n", "phase", "buzzer", "sent", "lost", "loss%", "p50", "p95", "max", "Δp95")
	for _, ph := range phases {
		for _, h := range hosts {
			p, ok := probes[ph][h]
			if !ok {
				fmt.Printf("%-12s %-15s %6s\n", ph, h, "-")
				continue
			}
			delta := "-"
			if base, ok := probes["baseline"][h]; ok && ph != "baseline" && base.RTT.Count > 0 && p.RTT.Count > 0 {
				delta = fmt.Sprintf("%+.1f", p.RTT.P95Ms-base.RTT.P95Ms)
			}
			fmt.Printf("%-12s %-15s %6d %6d %6.1f%% %7.1f %7.1f %7.1f %9s\n", ph, h, p.Sent, p.Lost, p.LossPct, p.RTT.P50Ms, p.RTT.P95Ms, p.RTT.MaxMs, delta)
		}
	}
	if logs != nil {
		fmt.Println()
		fmt.Println("SERVER LOG COUNTERS per phase (from /ws/logs)")
		fmt.Printf("%-12s %7s %8s %8s %8s %8s %8s %6s %6s\n", "phase", "entries", "ack_ok*", "ack_rtry", "ack_exp", "bz_conn", "bz_disc", "button", "warn")
		for _, ph := range phases {
			c := logs[ph]
			fmt.Printf("%-12s %7d %8d %8d %8d %8d %8d %6d %6d\n", ph, c.Entries, c.AckReceived, c.AckRetry, c.AckExpired, c.BuzzerConnected, c.BuzzerDisconnected, c.ButtonPress, c.Warn)
		}
		fmt.Println("* ack_ok is logged at DEBUG level by the server — stays 0 unless the server log level is DEBUG")
	}
	fmt.Println()
}
