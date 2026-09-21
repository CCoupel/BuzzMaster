// Command buzzmaster-spike-audio is the throwaway demonstration program for
// issue #226 (spike: faisabilité audio + Bluetooth). It is NOT production
// code — see _work/reports/spike-226-*.md for the verdict this program
// exists to support, and README.md in this directory for how to reproduce
// each test on real hardware.
//
// Usage:
//
//	go run . diag                 — report which audio transport is reached
//	go run . play <cue>           — play one cue, report timings
//	go run . overlap              — play two cues 50ms apart (task 0.6)
//	go run . robustness           — prove the dispatch path never blocks (task 0.8)
//	go run . compensation         — simulate the sound/light delay design (task 0.11)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: buzzmaster-spike-audio <diag|play|overlap|robustness|compensation> [args]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "diag":
		err = cmdDiag()
	case "play":
		cue := "reveal"
		if len(os.Args) > 2 {
			cue = os.Args[2]
		}
		err = cmdPlay(cue)
	case "overlap":
		err = cmdOverlap()
	case "robustness":
		err = cmdRobustness()
	case "compensation":
		cmdCompensation()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
