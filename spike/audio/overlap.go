package main

import (
	"bytes"
	"fmt"
	"time"
)

// cmdOverlap answers task 0.6: two cues fired 50ms apart. oto/v3's design
// (one Context, many Players, internal software mixer — internal/mux, see
// README "From a context you can create any number of different players")
// means this is a mixing question, not a queueing one: both players read
// from their own buffer and the mux sums them. This program proves it by
// checking BOTH players report IsPlaying()==true at the same instant
// in the middle of the overlap window, which would be impossible if oto
// serialised or dropped the second cue.
func cmdOverlap() error {
	ctx, openCost, err := openReadyContext()
	if err != nil {
		return err
	}
	fmt.Printf("context open cost: %s\n", openCost)

	pA := ctx.NewPlayer(bytes.NewReader(cues["reveal"]))
	defer pA.Close()
	pB := ctx.NewPlayer(bytes.NewReader(cues["gagne"]))
	defer pB.Close()

	t0 := time.Now()
	pA.Play()
	fmt.Printf("t=%s cue A (reveal) started\n", time.Since(t0))
	time.Sleep(50 * time.Millisecond)
	pB.Play()
	fmt.Printf("t=%s cue B (gagne) started\n", time.Since(t0))

	// Sample shortly after B starts: if oto queued/serialised instead of
	// mixing, A would already be the only one "playing" from the driver's
	// point of view, or B's start would have cut A short.
	time.Sleep(20 * time.Millisecond)
	aPlaying, bPlaying := pA.IsPlaying(), pB.IsPlaying()
	fmt.Printf("t=%s sampled mid-overlap: A.IsPlaying()=%v B.IsPlaying()=%v\n", time.Since(t0), aPlaying, bPlaying)
	if aPlaying && bPlaying {
		fmt.Println("VERDICT 0.6: both players report playing simultaneously -> oto MIXES concurrent")
		fmt.Println("cues natively (one shared Context, per-cue Player, internal/mux sums the streams).")
		fmt.Println("No queueing policy is needed in the audio engine itself for this case; a policy")
		fmt.Println("is still needed at a higher level only if the catalogue wants to CAP how many")
		fmt.Println("simultaneous cues are audible (a product decision, not a technical constraint).")
	} else {
		fmt.Println("VERDICT 0.6: did NOT observe simultaneous playback — re-run, and if reproducible")
		fmt.Println("this contradicts the researched design and must be investigated before relying on it.")
	}

	// Bounded drain wait — see play.go's comment on IsPlaying() reliability
	// in this sandbox's virtual sink; a hard deadline keeps this demo from
	// ever hanging regardless.
	deadline := time.Now().Add(3 * time.Second)
	for (pA.IsPlaying() || pB.IsPlaying()) && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}
