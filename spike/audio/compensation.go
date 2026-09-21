package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// This file answers task 0.11 in isolation from the real server: it does
// NOT touch cmd/server or internal/lighting (forbidden on this branch, see
// the plan §4). It reproduces, at the scale of a standalone program, the
// exact mechanism internal/lighting.Writer already uses — re-derive the
// LIVE state at APPLY time, never carry a captured payload (writer.go:
// "the writer NEVER buffers a state [...] re-derives from the live
// GameState when it actually runs") — plus the ONE new ingredient task
// 0.11 asks for: delaying only the light side of the fan-out, keyed on
// which cue is playing, never the sound and never the buzzer LEDs.
//
// It exists to answer, with numbers instead of assertion: does delaying
// only the light really line up sound and light, or can it silently make
// the light SKIP an intermediate state on a fast sequence? (§3.3's
// "contre-intuitive" concern.)

// compEvent is the tiny stand-in for lighting.Event in this isolated demo.
type compEvent struct {
	kind string
	t    time.Time
}

// ambianceSim stands in for the future a.notifyAmbiance() fan-out
// (cadrage plan §D.1, not yet implemented in cmd/server — this spike does
// not assume it exists, it only proves the mechanism that would sit behind
// it once #227+ builds it).
type ambianceSim struct {
	mu   sync.Mutex
	live compEvent // the single source of truth, exactly like deriveAmbianceEvent() reads GameState

	// compensateDelay is the configurable parameter task 3.4 requires —
	// deliberately NOT a constant. Its default would come from task 0.5's
	// measured latency; here it is passed in so the scenario can show both
	// a realistic value and a stress value.
	compensateDelay time.Duration

	// onlyCues is the selective scope from §3.5 — compensation applies to
	// this set only, everything else is immediate on both legs.
	onlyCues map[string]bool

	inFlight    atomic.Int32
	maxInFlight int32 // bound from §3.3's "borner le nombre de différés en vol"

	log []string
}

func newAmbianceSim(delay time.Duration, maxInFlight int32, cues ...string) *ambianceSim {
	set := map[string]bool{}
	for _, c := range cues {
		set[c] = true
	}
	return &ambianceSim{compensateDelay: delay, onlyCues: set, maxInFlight: maxInFlight}
}

func (a *ambianceSim) logf(format string, args ...interface{}) {
	a.log = append(a.log, fmt.Sprintf(format, args...))
}

// notify is the fan-out point. Sound is synchronous-immediate (no delay,
// ever — it is the leg being compensated FOR). The light leg is either
// applied immediately (event not in the compensated scope) or deferred via
// time.AfterFunc — never a blocking sleep, matching sendLEDSetComet's own
// precedent (main.go:4961) and Writer's TimerFactory contract.
func (a *ambianceSim) notify(kind string, t0 time.Time, wg *sync.WaitGroup) {
	now := time.Since(t0)
	a.mu.Lock()
	a.live = compEvent{kind: kind, t: time.Now()}
	a.mu.Unlock()

	a.logf("t=%-6s SOUND  fires immediately for %q", now.Round(time.Millisecond), kind)

	if !a.onlyCues[kind] {
		a.logf("t=%-6s LIGHT  applies immediately for %q (not in the compensated scope)", now.Round(time.Millisecond), kind)
		return
	}

	if a.inFlight.Load() >= a.maxInFlight {
		a.logf("t=%-6s LIGHT  compensation SKIPPED for %q — %d deferred already in flight (bound reached, §3.3)",
			now.Round(time.Millisecond), kind, a.maxInFlight)
		a.logf("t=%-6s LIGHT  applies immediately instead (degrade to no-compensation, never unbounded)", now.Round(time.Millisecond))
		return
	}
	a.inFlight.Add(1)
	if wg != nil {
		wg.Add(1)
	}
	scheduledFor := kind
	time.AfterFunc(a.compensateDelay, func() {
		defer a.inFlight.Add(-1)
		defer func() {
			if wg != nil {
				wg.Done()
			}
		}()
		a.mu.Lock()
		liveNow := a.live
		a.mu.Unlock()
		fireTime := time.Since(t0)
		if liveNow.kind == scheduledFor {
			a.logf("t=%-6s LIGHT  applies %q (delayed %s, live state unchanged since scheduling — sound/light now line up)",
				fireTime.Round(time.Millisecond), liveNow.kind, a.compensateDelay)
		} else {
			a.logf("t=%-6s LIGHT  applies %q -- but this timer was SCHEDULED for %q! Live state moved on;",
				fireTime.Round(time.Millisecond), liveNow.kind, scheduledFor)
			a.logf("              %-24s the light for %q is SKIPPED ENTIRELY, never rendered (§3.3 confirmed).", "", scheduledFor)
		}
	})
}

func cmdCompensation() {
	fmt.Println("=== Scenario 1: isolated temps-ecoule, nothing else happens for 300ms ===")
	sim := newAmbianceSim(200*time.Millisecond, 4, "temps-ecoule")
	var wg sync.WaitGroup
	t0 := time.Now()
	sim.notify("temps-ecoule", t0, &wg)
	wg.Wait()
	for _, l := range sim.log {
		fmt.Println(" ", l)
	}
	fmt.Println("  -> clean: sound and (delayed) light both eventually reflect temps-ecoule.")

	fmt.Println("\n=== Scenario 2: a second event lands 50ms into the 200ms compensation window ===")
	fmt.Println("(the exact case §3.3 warns is counter-intuitive: reveal happens DURING the")
	fmt.Println("temps-ecoule light's deferred window)")
	sim2 := newAmbianceSim(200*time.Millisecond, 4, "temps-ecoule")
	var wg2 sync.WaitGroup
	t0b := time.Now()
	sim2.notify("temps-ecoule", t0b, &wg2)
	time.AfterFunc(50*time.Millisecond, func() { sim2.notify("reveal", t0b, nil) })
	time.Sleep(260 * time.Millisecond)
	wg2.Wait()
	for _, l := range sim2.log {
		fmt.Println(" ", l)
	}
	fmt.Println("  -> VERDICT 0.11 (mechanism): the temps-ecoule LIGHT never renders — it is not late,")
	fmt.Println("     it is SKIPPED, because the deferred read re-derives the state that is live AT")
	fmt.Println("     FIRE TIME, and reveal has already overwritten it. This is the exact behaviour")
	fmt.Println("     documented for internal/lighting.Writer today (never a stale payload, always")
	fmt.Println("     re-derived) and it is NOT a bug to fix — it is a property to design around:")
	fmt.Println("     recommendation is to accept the skip for temps-ecoule specifically, because")
	fmt.Println("     what follows it (reveal) is itself an unambiguous, high-salience scene change —")
	fmt.Println("     losing the intermediate frame there is far cheaper than the alternative (queueing")
	fmt.Println("     light states would reintroduce backlog/coalescence problems the Writer already")
	fmt.Println("     solved for #205 — see D.1 in the cadrage plan).")

	fmt.Println("\n=== Scenario 3: bound on in-flight deferred timers ===")
	sim3 := newAmbianceSim(150*time.Millisecond, 2, "temps-ecoule")
	var wg3 sync.WaitGroup
	t0c := time.Now()
	for i := 0; i < 5; i++ {
		sim3.notify("temps-ecoule", t0c, &wg3)
		time.Sleep(5 * time.Millisecond)
	}
	wg3.Wait()
	for _, l := range sim3.log {
		fmt.Println(" ", l)
	}
	fmt.Println("  -> VERDICT 0.11 (bound): with maxInFlight=2, the 3rd+ rapid-fire temps-ecoule")
	fmt.Println("     notifications degrade to immediate light (no compensation) instead of piling")
	fmt.Println("     up timers — satisfies §3.3's \"borner le nombre de différés en vol\".")

	fmt.Println("\n=== Which visual reference does the animateur actually use? (§3.2) ===")
	fmt.Println("Not measured by this program — it is a UX/perception question, not a code one.")
	fmt.Println("Recorded as-is in the verdict report: TV/écrans update over WebSocket on the same")
	fmt.Println("goroutine ordering as the Hue notify call (no comparable transport delay documented")
	fmt.Println("for TV), so compensating ONLY the Hue ambiance light leaves the on-screen countdown")
	fmt.Println("reaching zero in sync with the (now-delayed) room light, but the sound still arrives")
	fmt.Println("before BOTH if Bluetooth latency exceeds compensateDelay. See verdict report §latency.")
}
