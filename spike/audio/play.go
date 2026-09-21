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

	// BUGFIX (reported: "buffered=0 bytes, elapsed=0s", no sound at all on
	// Windows, PC output as well as Bluetooth): the previous version of this
	// loop required BOTH IsPlaying() AND BufferedSize()>0 to keep waiting.
	// Per oto's own source (internal/mux/mux.go, playImpl: "starts playing
	// without reading the source. The buffer is filled by the mux loop."),
	// Play() sets IsPlaying()=true SYNCHRONOUSLY but does NOT synchronously
	// fill the buffer — that happens later, on the mux's own read cycle.
	// Right after Play(), BufferedSize() is legitimately still 0, so the old
	// condition was false on the very first check and the loop below never
	// ran even once: cmdPlay returned immediately, player.Close() (deferred)
	// tore the player down, and the process exited before the mux ever had
	// a chance to read a single byte from our io.Reader — hence zero sound,
	// identically on the default output and on Bluetooth (this was never a
	// device/transport problem, it was this function returning too early).
	// The correct signal is IsPlaying() alone: per mux.go's
	// finishSourceRead, the player only transitions back out of
	// playerPlay once EOF has been reached AND the buffer has actually
	// drained to empty — i.e. IsPlaying() reliably means "still has work
	// to do," including the "not filled yet" instant right after Play().
	deadline := time.Now().Add(3 * time.Second)
	for player.IsPlaying() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		fmt.Println("NOTE: hit the 3s safety deadline before IsPlaying() cleared — a 200-300ms")
		fmt.Println("tone should never take this long; treat this as a finding, not a false alarm.")
	}
	// Grace period: IsPlaying()==false means our source buffer is drained,
	// but the OS/driver's own output buffer (e.g. PulseAudio's ~100ms
	// PlaybackLatency, WASAPI's own tail) may still hold the last chunk.
	// Closing the player/context immediately here would risk cutting that
	// tail — audible as a clipped ending, easy to misread as "no sound" on
	// a short cue. 300ms comfortably covers every buffer size in play.
	time.Sleep(300 * time.Millisecond)
	fmt.Printf("cue %q: playback signalled complete, elapsed=%s (+300ms device-drain grace before closing)\n", cue, time.Since(tPlay))
	fmt.Println("REMINDER: this program can only confirm the SOFTWARE path ran end to end.")
	fmt.Println("Whether it was actually AUDIBLE is for you to judge — that is exactly what")
	fmt.Println("this manual test is for.")
	return nil
}
