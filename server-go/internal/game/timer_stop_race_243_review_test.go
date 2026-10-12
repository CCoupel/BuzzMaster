package game

import (
	"testing"
	"time"
)

// #243 review: a NEW_GAME landing between OnTimeUp and the timeout Stop must not
// put the phase back to STOPPED.
func TestTimeoutStopIsIgnoredWhenNewGameLandsFirst(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q", Type: QuestionTypeQCM})
	e.SetPhase(PhaseStarted)
	e.mu.Lock()
	e.state.CurrentTime = 1
	e.mu.Unlock()
	done := make(chan struct{})
	timeoutStopTestHook = func() {
		e.InitGameWith(DefaultNewGameOptions()) // NEW_GAME right in the window
		close(done)
	}
	defer func() { timeoutStopTestHook = nil }()
	e.startTimer()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("le chrono n'a pas expiré")
	}
	time.Sleep(200 * time.Millisecond)
	if p := e.GetPhase(); p != PhaseNewGame {
		t.Fatalf("la phase doit rester NEW_GAME, obtenu %s", p)
	}
}

func TestTimeoutStopStillStopsWhenNothingElseHappened(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q", Type: QuestionTypeQCM})
	e.SetPhase(PhaseStarted)
	e.mu.Lock()
	e.state.CurrentTime = 1
	e.mu.Unlock()
	e.startTimer()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e.GetPhase() == PhaseStopped {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("l'expiration du chrono doit toujours arrêter la question (phase %s)", e.GetPhase())
}
