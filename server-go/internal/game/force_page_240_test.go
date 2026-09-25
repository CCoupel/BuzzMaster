package game

import "testing"

// #240 — the TV/VPlayer view (state.Page) is forced back to GAME when a round
// is launched, from any view, and is never locked afterwards.

func page240(e *Engine) string { s := e.GetState(); return s.Page }

func TestForcePage240_ReadyFromEachView(t *testing.T) {
	for _, view := range []string{"SCORE", "PLAYERS", "PALMARES"} {
		t.Run(view, func(t *testing.T) {
			e := NewEngine()
			e.SetPage(view)
			e.Ready("q1", &Question{ID: "q1", Answer: "42"})
			if got := page240(e); got != "GAME" {
				t.Fatalf("Ready from %s: Page=%q, want GAME", view, got)
			}
			// not a lock: manual switch still honoured
			e.SetPage("SCORE")
			if got := page240(e); got != "SCORE" {
				t.Fatalf("manual SetPage after force: %q", got)
			}
		})
	}
}

func TestForcePage240_StartAndImmediate(t *testing.T) {
	e := NewEngine()
	e.SetPage("PLAYERS")
	e.StartImmediate(30)
	if got := page240(e); got != "GAME" {
		t.Fatalf("StartImmediate: Page=%q", got)
	}
	e.Stop()
}

func TestForcePage240_StartCountdown(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.state.Phase = PhaseReady
	e.state.Page = "PALMARES"
	e.mu.Unlock()
	e.Start(30)
	if got := page240(e); got != "GAME" {
		t.Fatalf("Start: Page=%q", got)
	}
	e.Stop()
}

func TestForcePage240_ContinueAfterPause(t *testing.T) {
	e := NewEngine()
	e.StartImmediate(30)
	e.Pause()
	e.SetPage("SCORE")
	e.Continue()
	if got := page240(e); got != "GAME" {
		t.Fatalf("Continue: Page=%q", got)
	}
	e.Stop()
}

func TestForcePage240_NoForceOnPauseStopReveal(t *testing.T) {
	e := NewEngine()
	e.StartImmediate(30)
	e.SetPage("SCORE")
	e.Pause()
	if got := page240(e); got != "SCORE" {
		t.Fatalf("Pause forced Page: %q", got)
	}
	e.Continue()
	e.SetPage("SCORE")
	e.Stop()
	if got := page240(e); got != "SCORE" {
		t.Fatalf("Stop forced Page: %q", got)
	}
}

func TestForcePage240_ReevaluateNoForce(t *testing.T) {
	e := NewEngine()
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})
	e.SetPage("SCORE")
	e.mu.Lock()
	e.state.Phase = PhaseReady
	e.reevaluatePrepareReadyUnsafe()
	e.mu.Unlock()
	if got := page240(e); got != "SCORE" {
		t.Fatalf("READY<->PREPARE re-evaluation forced Page: %q", got)
	}
}
