// spike-ble-hue — throwaway feasibility demo for issue #204 (milestone v10.0.0).
//
// Drives Philips Hue Bluetooth bulbs that were paired OUT OF BAND (bluetoothctl on
// Linux, Bluetooth settings on Windows) from a plain Go program, and measures:
// connection count, GATT write latency, group desynchronisation and 2.4 GHz
// coexistence with the BuzzControl buzzers. It is NOT part of the server build
// (separate go.mod). See README.md next to this file.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	flagBulbs     = flag.String("bulbs", "", "comma-separated bulb MAC addresses (AA:BB:CC:DD:EE:FF,...), already paired with the OS")
	flagOut       = flag.String("out", "", "write a JSON report to this file")
	flagTimeout   = flag.Duration("timeout", 20*time.Second, "per-bulb connect / discover timeout")
	flagWriteMode = flag.String("write-mode", "auto", "GATT write procedure: auto|request|command")
	flagAdapter   = flag.String("adapter", "", "Linux only: BlueZ adapter id (default hci0); ignored on Windows")
	flagVerbose   = flag.Bool("v", false, "verbose logging")

	// scan
	flagScanDur = flag.Duration("scan-duration", 10*time.Second, "scan: how long to listen")
	flagScanAll = flag.Bool("scan-all", false, "scan: list every BLE device, not only Hue bulbs")

	// bench
	flagIterations = flag.Int("iterations", 30, "bench: group colour changes per strategy")
	flagInterval   = flag.Duration("interval", 400*time.Millisecond, "bench/coexist: pause between group writes")

	// hold
	flagHoldDur   = flag.Duration("duration", 5*time.Minute, "hold: how long to keep connections up")
	flagKeepalive = flag.Duration("keepalive", 15*time.Second, "hold: period of the liveness read on each bulb")

	// coexist
	flagServer   = flag.String("server", "http://127.0.0.1", "coexist: BuzzControl server URL (for /api/buzzers and /ws/logs)")
	flagBuzzers  = flag.String("buzzers", "auto", "coexist: buzzer IPs to ping, or 'auto' to read them from the server")
	flagPhaseDur = flag.Duration("phase", 60*time.Second, "coexist: duration of each phase")
	flagPhases   = flag.String("phases", "baseline,ble-idle,ble-traffic,ble-off", "coexist: ordered phase list")
	flagPingInt  = flag.Duration("ping-interval", 250*time.Millisecond, "coexist: echo request period per buzzer")
	flagNoLogs   = flag.Bool("no-logs", false, "coexist: do not attach to /ws/logs")
)

func usage() {
	fmt.Fprintf(os.Stderr, `spike-ble-hue — Hue BLE feasibility demo (#204)

Usage: spike-ble-hue [flags] <command>

Commands:
  scan      list Hue bulbs advertising nearby (to find MAC addresses before pairing)
  demo      connect to -bulbs, read state, then on / red / green / blue / white / dim / off
  bench     measure GATT write latency and group desync (sequential vs parallel writes)
  hold      keep persistent connections to -bulbs for -duration, log drops/reconnects
  coexist   2.4 GHz protocol: ping the buzzers (+ server logs) across BLE on/off phases
  info      print OS / library versions and exit

Flags:
`)
	flag.PrintDefaults()
}

func logf(format string, a ...any) {
	fmt.Printf(time.Now().Format("15:04:05.000")+" "+format+"\n", a...)
}

func vlogf(format string, a ...any) {
	if *flagVerbose {
		logf(format, a...)
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	cmd := strings.ToLower(flag.Arg(0))

	writeMode, err := parseWriteMode(*flagWriteMode)
	if err != nil {
		fatal(err)
	}

	report := newReport(cmd, os.Args[1:])
	logf("spike-ble-hue  os=%s/%s  go=%s  lib=%s", runtime.GOOS, runtime.GOARCH, runtime.Version(), report.Library)

	if cmd == "info" {
		return
	}

	adapter := bluetooth.DefaultAdapter
	if *flagAdapter != "" && runtime.GOOS == "linux" {
		adapter = newLinuxAdapter(*flagAdapter)
	}
	if err := adapter.Enable(); err != nil {
		fatal(fmt.Errorf("enable Bluetooth adapter: %w (Linux: is bluetoothd running and the adapter powered? Windows: is Bluetooth on?)", err))
	}

	macs := splitList(*flagBulbs)
	needBulbs := cmd != "scan"
	if needBulbs && len(macs) == 0 {
		fatal(fmt.Errorf("command %q needs -bulbs MAC[,MAC...]", cmd))
	}

	switch cmd {
	case "scan":
		err = cmdScan(adapter, report)
	case "demo":
		err = cmdDemo(adapter, macs, writeMode, report)
	case "bench":
		err = cmdBench(adapter, macs, writeMode, report)
	case "hold":
		err = cmdHold(adapter, macs, report)
	case "coexist":
		err = cmdCoexist(adapter, macs, writeMode, report)
	default:
		usage()
		os.Exit(2)
	}

	if serr := report.Save(*flagOut); serr != nil {
		logf("WARNING: could not save report: %v", serr)
	} else if *flagOut != "" {
		logf("report written to %s", *flagOut)
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}

// cmdScan lists advertising bulbs. Hue bulbs are recognised by the Signify
// service UUID 0xFE0F in their advertisement or a local name containing "hue".
func cmdScan(adapter *bluetooth.Adapter, report *Report) error {
	type seen struct {
		MAC  string `json:"mac"`
		Name string `json:"name"`
		RSSI int16  `json:"rssi"`
		Hue  bool   `json:"hue"`
	}
	found := map[string]*seen{}
	hueUUID, _ := bluetooth.ParseUUID(uuidHueAdvService)

	logf("scanning for %s (Ctrl+C to stop early) ...", *flagScanDur)
	stop := time.AfterFunc(*flagScanDur, func() { _ = adapter.StopScan() })
	defer stop.Stop()

	err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
		mac := strings.ToUpper(r.Address.String())
		name := r.LocalName()
		isHue := r.HasServiceUUID(hueUUID) || strings.Contains(strings.ToLower(name), "hue")
		if !isHue && !*flagScanAll {
			return
		}
		if s, ok := found[mac]; ok {
			s.RSSI = r.RSSI
			if s.Name == "" {
				s.Name = name
			}
			return
		}
		found[mac] = &seen{MAC: mac, Name: name, RSSI: r.RSSI, Hue: isHue}
		tag := "   "
		if isHue {
			tag = "HUE"
		}
		logf("  %s %s rssi=%4d name=%q", tag, mac, r.RSSI, name)
	})
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	list := make([]seen, 0, len(found))
	hue := 0
	for _, s := range found {
		list = append(list, *s)
		if s.Hue {
			hue++
		}
	}
	report.Sections["scan"] = list
	logf("scan done: %d device(s), %d Hue bulb(s)", len(list), hue)
	if hue == 0 {
		logf("no Hue bulb seen — a bulb already connected to another central (phone app) does NOT advertise; power-cycle it or close the Hue app")
	}
	return nil
}
