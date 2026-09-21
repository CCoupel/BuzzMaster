package main

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// job is one queued write — the throwaway stand-in for "one audio cue" in
// this isolated proof. It carries only what a real dispatch call site
// would carry: a name and the bytes to write.
type job struct {
	name string
	data []byte
}

// asyncEngine is a minimal, dependency-free reproduction of the
// "single-goroutine reader, bounded non-blocking queue" pattern the plan's
// D.3 requires for the real internal/audio engine (mirroring how
// internal/lighting.Writer is already built, and AckManager before it).
// It exists here ONLY to prove the pattern structurally — with a
// deliberately wedged io.Writer standing in for a hung Bluetooth stack —
// independently of any real audio hardware, which this program does not
// have access to.
type asyncEngine struct {
	queue    chan job
	dropped  atomic.Int64
	accepted atomic.Int64
	worker   func(job)
}

func newAsyncEngine(bufSize int, worker func(job)) *asyncEngine {
	e := &asyncEngine{queue: make(chan job, bufSize), worker: worker}
	go func() {
		for j := range e.queue {
			e.worker(j)
		}
	}()
	return e
}

// Play NEVER blocks: a full queue drops the job (counted) rather than
// waiting. This is the exact contract D.3 asks for on the real engine's
// entry point, called from the game's dispatch goroutine.
func (e *asyncEngine) Play(j job) (accepted bool) {
	select {
	case e.queue <- j:
		e.accepted.Add(1)
		return true
	default:
		e.dropped.Add(1)
		return false
	}
}

// wedgedWriter simulates a Bluetooth output that has stopped responding
// (speaker powered off / out of range mid-game, task 0.8) — Write() never
// returns. A real io.Writer talking to a dead A2DP socket behaves exactly
// like this: the call blocks, it does not error out instantly.
type wedgedWriter struct{ unblock chan struct{} }

func (w *wedgedWriter) Write(p []byte) (int, error) {
	<-w.unblock // blocks forever unless the test unblocks it
	return len(p), nil
}

func cmdRobustness() error {
	fmt.Println("=== Scenario A: naive synchronous write — the ANTI-PATTERN ===")
	wedged := &wedgedWriter{unblock: make(chan struct{})}
	t0 := time.Now()
	done := make(chan struct{})
	go func() {
		_, _ = io.WriteString(wedged, "boom") // this is what a direct call site would do
		close(done)
	}()
	select {
	case <-done:
		fmt.Println("UNEXPECTED: synchronous write returned without the device ever unblocking")
	case <-time.After(300 * time.Millisecond):
		fmt.Printf("CONFIRMED: a direct/synchronous call is still blocked after %s.\n", time.Since(t0))
		fmt.Println("If this call site were the game's dispatch goroutine (as all ~12 Engine")
		fmt.Println("callback sites are, engine.go:220-241), the ENTIRE GAME would freeze the")
		fmt.Println("moment the speaker died. This is exactly risk E.11 in the cadrage plan.")
	}
	close(wedged.unblock) // release the leaked goroutine before moving on

	fmt.Println("\n=== Scenario B: asyncEngine (bounded queue, single worker) — THE REQUIRED PATTERN ===")
	wedged2 := &wedgedWriter{unblock: make(chan struct{})}
	defer close(wedged2.unblock)
	engine := newAsyncEngine(4, func(j job) {
		_, _ = wedged2.Write(j.data) // the ONLY goroutine allowed to block
	})

	// Fill the queue past capacity while the single worker is stuck on the
	// first job forever (until we close(unblock) in the deferred call
	// above, which happens after this function returns).
	var maxCallLatency time.Duration
	for i := 0; i < 20; i++ {
		t := time.Now()
		accepted := engine.Play(job{name: fmt.Sprintf("cue-%d", i), data: []byte("x")})
		lat := time.Since(t)
		if lat > maxCallLatency {
			maxCallLatency = lat
		}
		_ = accepted
	}
	fmt.Printf("20 Play() calls issued against a permanently wedged output.\n")
	fmt.Printf("accepted=%d dropped=%d (bounded queue capacity=4 + 1 in flight)\n", engine.accepted.Load(), engine.dropped.Load())
	fmt.Printf("worst-case Play() call latency observed: %s\n", maxCallLatency)
	if maxCallLatency < 5*time.Millisecond {
		fmt.Println("VERDICT 0.8 (structural half): the entry point used by the game dispatch")
		fmt.Println("goroutine never blocks, however long the output stays wedged — bounded queue")
		fmt.Println("absorbs the burst, excess is dropped and counted, never awaited.")
	} else {
		fmt.Println("UNEXPECTED: Play() itself was slow — investigate before trusting this pattern.")
	}
	fmt.Println("\nNOTE: this proves the DISPATCH SIDE never blocks. It does NOT prove what the")
	fmt.Println("real oto/pulse stack does when a real BT speaker disconnects mid-stream — that")
	fmt.Println("can only be observed on real hardware (see README.md, scenario 'speaker cut').")
	fmt.Println("What CAN be said from the source read for this spike: oto's pulseContext.read()")
	fmt.Println("is only ever called FROM the audio backend's own goroutine (mux pull model), not")
	fmt.Println("from the caller's Play(), so even oto's own blocking-on-device behaviour, if any,")
	fmt.Println("would already be isolated from the caller — consistent with what this scenario")
	fmt.Println("reproduces at the architecture's outer boundary.")
	return nil
}
