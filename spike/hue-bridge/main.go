// spike-hue-bridge — throwaway confirmation spike for the Hue Bridge path
// (milestone v10.0.0, after the BLE path was ruled out on Windows in #204).
//
// It runs against the user's HOME bridge: see guard.go for the hard limits
// (three allowed operations, one target light found by name, nothing else).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	flagBridgeIP   = flag.String("bridge-ip", "", "bridge IP or scheme://host; empty = discover (mDNS then SSDP)")
	flagHTTPS      = flag.Bool("https", false, "use https:// to the bridge (self-signed cert accepted) instead of http://")
	flagTarget     = flag.String("target", "BuzzHue1", "exact name of the ONLY light this program may write to")
	flagOut        = flag.String("out", "", "write a JSON report to this file")
	flagTimeout    = flag.Duration("timeout", 3*time.Second, "HTTP timeout per request")
	flagWait       = flag.Duration("link-wait", 45*time.Second, "register: how long to wait for the link button")
	flagDevType    = flag.String("devicetype", "buzzmaster-spike#test", "register: Hue devicetype")
	flagIterations = flag.Int("iterations", 12, "bench: number of state changes")
	flagInterval   = flag.Duration("interval", 300*time.Millisecond, "bench/demo: pause between commands")
	flagTransition = flag.Int("transition", 0, "transitiontime sent with every write, in 100 ms units (0 = instant)")
	flagDiscoverT  = flag.Duration("discover-timeout", 4*time.Second, "discover: mDNS/SSDP listen time each")
)

func usage() {
	fmt.Fprintf(os.Stderr, `spike-hue-bridge — Hue Bridge confirmation spike (home bridge: see guard.go)

Usage: spike-hue-bridge [flags] <command>

Commands:
  discover   find bridges on the LAN (mDNS _hue._tcp, then SSDP) — read-only, no bridge call
  register   obtain an API key (press the bridge link button) and store it in .hue-username
  lights     list light ids/names (GET only) and show which one is the target
  state      read the target light's state (GET only)
  demo       target light only: on → bri 254 → red → green → blue → white → bri 60 → off, latency per write
  bench      target light only: -iterations rapid state changes, success rate and p50/p95 latency
  on | off | bri <1-254> | colour <red|green|blue|white|...>   single command on the target light

Flags:
`)
	flag.PrintDefaults()
}

func logf(format string, a ...any) {
	fmt.Printf(time.Now().Format("15:04:05.000")+" "+format+"\n", a...)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}
	cmd := strings.ToLower(flag.Arg(0))
	report := newReport(cmd, os.Args[1:])
	ctx := context.Background()

	if cmd == "discover" {
		bridges := runDiscover(ctx, report)
		if err := report.Save(*flagOut); err != nil {
			logf("WARNING: %v", err)
		}
		if len(bridges) == 0 {
			fatal(fmt.Errorf("no bridge found — pass -bridge-ip (visible in the Hue app: Settings → My Hue system → bridge → i)"))
		}
		return
	}

	base, err := resolveBridge(ctx, report)
	if err != nil {
		fatal(err)
	}
	report.Bridge = base
	logf("bridge: %s", base)

	if cmd == "register" {
		user, err := Register(ctx, base, *flagDevType, *flagTimeout, *flagWait, logf)
		if err != nil {
			fatal(fmt.Errorf("registration: %w", err))
		}
		if err := saveUser(user); err != nil {
			fatal(err)
		}
		logf("registered — API key stored in %s (%d chars, not printed)", userFile, len(user))
		_ = report.Save(*flagOut)
		return
	}

	user := loadUser()
	if user == "" {
		fatal(fmt.Errorf("no API key: run `spike-hue-bridge register` first (press the bridge button)"))
	}
	client, err := NewClient(base, user, *flagTimeout)
	if err != nil {
		fatal(err)
	}

	err = runCommand(ctx, client, cmd, flag.Args()[1:], report)
	if serr := report.Save(*flagOut); serr != nil {
		logf("WARNING: could not save report: %v", serr)
	} else if *flagOut != "" {
		logf("report written to %s", *flagOut)
	}
	if err != nil {
		fatal(err)
	}
}

func runDiscover(ctx context.Context, report *Report) []Bridge {
	logf("mDNS _hue._tcp (%s) ...", *flagDiscoverT)
	m, err := discoverMDNS(ctx, *flagDiscoverT, logf)
	if err != nil {
		logf("  mDNS error: %v", err)
	}
	logf("SSDP M-SEARCH (%s) ...", *flagDiscoverT)
	s, err := discoverSSDP(*flagDiscoverT, logf)
	if err != nil {
		logf("  SSDP error: %v", err)
	}
	all := append(m, s...)
	report.Sections["discover"] = map[string]any{"mdns": m, "ssdp": s}
	logf("discovery: %d via mDNS, %d via SSDP", len(m), len(s))
	return all
}

// resolveBridge returns scheme://host from -bridge-ip or discovery.
func resolveBridge(ctx context.Context, report *Report) (string, error) {
	scheme := "http"
	if *flagHTTPS {
		scheme = "https"
	}
	if ip := strings.TrimSpace(*flagBridgeIP); ip != "" {
		if strings.Contains(ip, "://") {
			return strings.TrimRight(ip, "/"), nil
		}
		return scheme + "://" + ip, nil
	}
	bridges := runDiscover(ctx, report)
	if len(bridges) == 0 {
		return "", fmt.Errorf("no bridge discovered — pass -bridge-ip")
	}
	if len(bridges) > 1 {
		ips := map[string]bool{}
		for _, b := range bridges {
			ips[b.IP] = true
		}
		if len(ips) > 1 {
			return "", fmt.Errorf("%d bridges discovered — pass -bridge-ip to choose", len(ips))
		}
	}
	return scheme + "://" + bridges[0].IP, nil
}

func runCommand(ctx context.Context, c *Client, cmd string, args []string, report *Report) error {
	// Every command below first resolves the target by exact name.
	target, lights, err := c.FindTarget(ctx, *flagTarget)
	if cmd == "lights" {
		ids := make([]string, 0, len(lights))
		for id := range lights {
			ids = append(ids, id)
		}
		sortIDs(ids)
		for _, id := range ids {
			mark := "   "
			if err == nil && id == target.ID {
				mark = "-> "
			}
			logf("%s light %-3s %q", mark, id, lights[id].Name)
		}
		if err != nil {
			return err
		}
		logf("target: id=%s name=%q", target.ID, target.Name)
		report.Target = &target
		return nil
	}
	if err != nil {
		return err
	}
	report.Target = &target
	logf("target light: id=%s name=%q (the only light this program will write to)", target.ID, target.Name)

	tt := intp(*flagTransition)
	set := func(label string, st State) (time.Duration, error) {
		st.TransitionTime = tt
		d, err := c.SetState(ctx, target, st)
		if err != nil {
			logf("%-14s FAILED after %.0fms: %v", label, ms(d), err)
		} else {
			logf("%-14s ok %.0fms", label, ms(d))
		}
		return d, err
	}

	switch cmd {
	case "state":
		l, d, err := c.Light(ctx, target.ID)
		if err != nil {
			return err
		}
		logf("state: on=%v bri=%d xy=%v reachable=%v type=%q model=%q (%.0fms)", l.State.On, l.State.Bri, l.State.XY, l.State.Reachable, l.Type, l.Model, ms(d))
		report.Sections["state"] = l
		return nil

	case "on":
		_, err := set("on", State{On: boolp(true)})
		return err
	case "off":
		_, err := set("off", State{On: boolp(false)})
		return err
	case "bri":
		if len(args) != 1 {
			return fmt.Errorf("bri needs a value 1-254")
		}
		var v int
		if _, err := fmt.Sscanf(args[0], "%d", &v); err != nil || v < 1 || v > 254 {
			return fmt.Errorf("bri needs a value 1-254")
		}
		_, err := set("bri", State{Bri: intp(v)})
		return err
	case "colour", "color":
		if len(args) != 1 {
			return fmt.Errorf("colour needs a name")
		}
		xy, err := colorXY(args[0])
		if err != nil {
			return err
		}
		_, err = set("colour "+args[0], State{XY: xy})
		return err

	case "demo":
		return runDemo(set, report)
	case "bench":
		return runBench(set, report)
	}
	usage()
	return fmt.Errorf("unknown command %q", cmd)
}

type stepResult struct {
	Step      string  `json:"step"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

func runDemo(set func(string, State) (time.Duration, error), report *Report) error {
	xy := func(n string) []float64 { v, _ := colorXY(n); return v }
	steps := []struct {
		name string
		st   State
	}{
		{"power on", State{On: boolp(true)}},
		{"brightness 254", State{Bri: intp(254)}},
		{"colour red", State{XY: xy("red")}},
		{"colour green", State{XY: xy("green")}},
		{"colour blue", State{XY: xy("blue")}},
		{"colour white", State{XY: xy("white")}},
		{"brightness 60", State{Bri: intp(60)}},
		{"power off", State{On: boolp(false)}},
	}
	var lat []time.Duration
	var results []stepResult
	failures := 0
	for _, s := range steps {
		d, err := set(s.name, s.st)
		r := stepResult{Step: s.name, LatencyMs: ms(d)}
		if err != nil {
			r.Error = err.Error()
			failures++
		} else {
			lat = append(lat, d)
		}
		results = append(results, r)
		time.Sleep(*flagInterval + 400*time.Millisecond)
	}
	st := statsOf(lat)
	report.Sections["demo"] = map[string]any{"steps": results, "latency": st, "failures": failures}
	logf("demo summary: %d/%d writes ok, latency %s", len(steps)-failures, len(steps), st)
	if failures > 0 {
		return fmt.Errorf("%d write(s) failed", failures)
	}
	return nil
}

func runBench(set func(string, State) (time.Duration, error), report *Report) error {
	n := *flagIterations
	if n < 1 {
		n = 1
	}
	colours := []string{"red", "green", "blue", "white"}
	var lat []time.Duration
	failures := 0
	_, _ = set("power on", State{On: boolp(true), Bri: intp(254)})
	time.Sleep(*flagInterval)
	for i := 0; i < n; i++ {
		var st State
		label := ""
		switch i % 3 {
		case 0:
			c := colours[(i/3)%len(colours)]
			xy, _ := colorXY(c)
			st, label = State{XY: xy}, "colour "+c
		case 1:
			st, label = State{Bri: intp(40 + (i*37)%200)}, "brightness"
		default:
			st, label = State{On: boolp(i%2 == 0)}, "power toggle"
		}
		d, err := set(fmt.Sprintf("%2d/%d %s", i+1, n, label), st)
		if err != nil {
			failures++
		} else {
			lat = append(lat, d)
		}
		time.Sleep(*flagInterval)
	}
	_, _ = set("restore off", State{On: boolp(false)})
	st := statsOf(lat)
	rate := 100.0 * float64(n-failures) / float64(n)
	report.Sections["bench"] = map[string]any{"iterations": n, "failures": failures, "success_pct": rate, "latency": st, "interval_ms": ms(*flagInterval), "transition_100ms": *flagTransition}
	logf("bench summary: %d/%d ok (%.1f%%), latency %s", n-failures, n, rate, st)
	if failures > 0 {
		return fmt.Errorf("%d of %d writes failed", failures, n)
	}
	return nil
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func sortIDs(ids []string) {
	// numeric-aware sort for light ids ("1","2",...,"10")
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && lessID(ids[j], ids[j-1]); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

func lessID(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}
