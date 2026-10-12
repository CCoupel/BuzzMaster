package game

// #243 (v11.2) — NEW_GAME : options de purge au niveau du moteur (InitGameWith), réservoir RAFALE, minuteurs.
// Contrats : contracts/websocket-actions.md §NEW_GAME. Scénarios 1, 6 bis, 7 du plan (_work/reports/planner-20261011-121500.md).

import (
	"testing"
	"time"
)

func allOptions243(scores, players, teams, questions bool) NewGameOptions {
	return NewGameOptions{PurgeScores: scores, PurgePlayers: players, PurgeTeams: teams, ResetQuestions: questions}
}

func TestNewGame243_DefaultOptionsAreTheV111Behaviour(t *testing.T) {
	d := DefaultNewGameOptions()
	if !d.PurgeScores || !d.PurgePlayers || d.PurgeTeams || !d.ResetQuestions {
		t.Errorf("sans option : PURGE_SCORES, PURGE_PLAYERS, RESET_QUESTIONS vrais, PURGE_TEAMS faux — %+v", d)
	}
}

func TestNewGame243_RafaleReservoirFollowsResetQuestions(t *testing.T) {
	for _, reset := range []bool{true, false} {
		e := NewEngine()
		e.mu.Lock()
		e.rafaleUsed["r1"] = true
		e.rafaleUsed["r2"] = true
		e.mu.Unlock()

		e.InitGameWith(allOptions243(false, false, false, reset))

		e.mu.RLock()
		n := len(e.rafaleUsed)
		e.mu.RUnlock()
		if reset && n != 0 {
			t.Errorf("RESET_QUESTIONS=true : réservoir RAFALE remis à zéro (%d entrées)", n)
		}
		if !reset && n != 2 {
			t.Errorf("RESET_QUESTIONS=false : réservoir RAFALE conservé (%d entrées)", n)
		}
	}
}

func TestNewGame243_QuestionStatusesFollowResetQuestions(t *testing.T) {
	for _, reset := range []bool{true, false} {
		e := NewEngine()
		e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
		e.SetPhase(PhaseStarted)
		e.Stop()
		if e.GetQuestionStatus("q1") == "AVAILABLE" {
			t.Fatal("précondition : q1 jouée")
		}

		e.InitGameWith(allOptions243(false, false, false, reset))

		got := e.GetQuestionStatus("q1")
		if reset && got != "AVAILABLE" && got != "" {
			t.Errorf("remise à jouer : %q", got)
		}
		if !reset && (got == "AVAILABLE" || got == "") {
			t.Errorf("statut conservé, obtenu %q", got)
		}
	}
}

func TestNewGame243_ResultReportsWhatWasPurged(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.SetTeams(map[string]*Team{"A": {Name: "A"}, "B": {Name: "B"}})
	for _, n := range []string{"Paul", "Mamie"} {
		if _, _, _, err := e.ReconnectOrCreateVirtualPlayer("", n); err != nil {
			t.Fatal(err)
		}
	}

	res := e.InitGameWith(allOptions243(true, true, false, true))

	if len(res.PurgedPlayerIDs) != 2 {
		t.Errorf("2 joueurs purgés rapportés : %+v", res)
	}
	if res.PhaseBefore != PhaseEnroll {
		t.Errorf("PhaseBefore = %s", res.PhaseBefore)
	}
}

// --- 7. Aucun minuteur ne survit : la phase ne revient jamais à STOPPED ----------------------

func TestNewGame243_Critical_CountdownAndTimerStopWithoutFallingBackToStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 6 s), exclu de la boucle rapide (-short)")
	}
	for _, stage := range []string{"décompte", "chrono"} {
		t.Run(stage, func(t *testing.T) {
			e := NewEngine()
			e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
			e.SetPhase(PhaseReady)
			e.Start(2)
			if stage == "chrono" {
				deadline := time.Now().Add(5 * time.Second)
				for e.GetPhase() != PhaseStarted && time.Now().Before(deadline) {
					time.Sleep(50 * time.Millisecond)
				}
			} else {
				time.Sleep(300 * time.Millisecond)
			}

			e.InitGameWith(allOptions243(true, true, false, true))

			deadline := time.Now().Add(6 * time.Second)
			for time.Now().Before(deadline) {
				if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
					t.Fatalf("la phase doit rester NEW_GAME, obtenu %s (minuteur survivant)", ph)
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}

func TestNewGame243_Critical_DoubleCallsAreIdempotent(t *testing.T) {
	e := NewEngine()
	e.InitGameWith(allOptions243(true, true, true, true))
	e.InitGameWith(allOptions243(true, true, true, true))
	if e.GetPhase() != GamePhase("NEW_GAME") {
		t.Errorf("phase = %s", e.GetPhase())
	}
}

// Un minuteur survivant n'a pas d'effet tant que la phase est NEW_GAME (le tick est ignoré hors STARTED) :
// il se réveille dès que la question suivante repasse en STARTED. Ces deux tests lancent donc la question
// SUIVANTE juste après le NEW_GAME — c'est là que l'ancien comportement se voyait (chrono qui va deux fois trop vite,
// décompte de l'ancienne question qui fait passer la nouvelle en STARTED trop tôt).

func TestNewGame243_Critical_NoSurvivingTickerDoublesTheNextQuestionsClock(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 10 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	ticks := make(chan int, 100)
	e.OnTimerTick = func(ct int) { ticks <- ct }
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.Start(30)
	waitPhase243(t, e, PhaseStarted, 6*time.Second)

	e.InitGameWith(allOptions243(true, false, false, true)) // NEW_GAME pendant le chrono

	e.Ready("", &Question{ID: "q2", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.Start(30)
	waitPhase243(t, e, PhaseStarted, 6*time.Second)
	for len(ticks) > 0 {
		<-ticks
	}
	time.Sleep(3300 * time.Millisecond)

	// 1 tick par seconde : 3 (ou 4 avec la gigue du premier tick). Un ancien ticker survivant en ajouterait 3 de plus.
	if n := len(ticks); n < 2 || n > 4 {
		t.Errorf("≈ 3 ticks en 3,3 s attendus, obtenu %d (un minuteur de l'ancienne question a survécu au NEW_GAME)", n)
	}
}

func TestNewGame243_Critical_OldCountdownCannotStartTheNextQuestionEarly(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 5 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.Start(30) // décompte de 3 s : fin à t ≈ 3,0 s
	time.Sleep(500 * time.Millisecond)

	e.InitGameWith(allOptions243(true, false, false, true))
	e.Ready("", &Question{ID: "q2", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.Start(30) // nouveau décompte : fin à t ≈ 3,6 s

	time.Sleep(2800 * time.Millisecond) // t ≈ 3,3 s : l'ancien décompte aurait déjà abouti
	if ph := e.GetPhase(); ph != PhaseCountdown {
		t.Errorf("à t ≈ 3,3 s la nouvelle question doit encore être en COUNTDOWN (fin prévue à ≈ 3,6 s), obtenu %s : le décompte de l'ancienne question a survécu", ph)
	}
}

func waitPhase243(t *testing.T, e *Engine, want GamePhase, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for e.GetPhase() != want && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if e.GetPhase() != want {
		t.Fatalf("phase %s attendue, obtenu %s", want, e.GetPhase())
	}
}
