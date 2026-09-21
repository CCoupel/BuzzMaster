package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/ebitengine/oto/v3"
)

// openReadyContext opens the one process-wide oto Context and blocks until
// the backend is ready (or errors). Returns the elapsed "cold open" cost —
// task 0.5 explicitly asks to include this, since it can dominate the
// very first sound of a game if the A2DP link is not pre-armed.
func openReadyContext() (*oto.Context, time.Duration, error) {
	t0 := time.Now()
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("oto.NewContext: %w", err)
	}
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		return nil, 0, fmt.Errorf("readyChan timed out after 5s")
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("context degraded at open: %w", err)
	}
	return ctx, time.Since(t0), nil
}

func cmdPlay(cue string) error {
	pcm, ok := cues[cue]
	if !ok {
		return fmt.Errorf("unknown cue %q (known: temps-ecoule, reveal, gagne, entracte-fin)", cue)
	}
	ctx, openCost, err := openReadyContext()
	if err != nil {
		return err
	}
	fmt.Printf("context open cost: %s\n", openCost)

	player := ctx.NewPlayer(bytes.NewReader(pcm))
	defer player.Close()

	tPlay := time.Now()
	player.Play() // documented as async: returns without waiting (README "Advanced usage")
	callCost := time.Since(tPlay)
	fmt.Printf("Play() call returned in %s (this must stay ~0 — it is called from the game\n", callCost)
	fmt.Println("dispatch goroutine in the real architecture, see D.3 in the cadrage plan)")

	// Bounded wait: IsPlaying() proved unreliable as a completion signal in
	// this sandbox's virtual audio sink (see README.md "Known sandbox
	// limitation") — BufferedSize()==0 is the fallback signal, and a hard
	// deadline guarantees this demo itself never hangs, matching the
	// "never block" principle it is trying to demonstrate.
	deadline := time.Now().Add(3 * time.Second)
	for player.IsPlaying() && player.BufferedSize() > 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		fmt.Println("NOTE: hit the 3s safety deadline before IsPlaying()/BufferedSize() cleared —")
		fmt.Println("see README.md known sandbox limitation. Not a hang: the deadline is what")
		fmt.Println("stopped this loop, on purpose.")
	}
	fmt.Printf("cue %q: buffered=%d bytes, elapsed=%s\n", cue, player.BufferedSize(), time.Since(tPlay))
	fmt.Println("REMINDER: this is the SOFTWARE floor only. It does NOT include the acoustic")
	fmt.Println("latency of a real A2DP link — that number can only come from the manual")
	fmt.Println("stopwatch/recording procedure in README.md on real hardware.")
	return nil
}
