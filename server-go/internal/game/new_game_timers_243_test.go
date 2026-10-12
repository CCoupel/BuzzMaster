package game

// #243 (v11.2) — NEW_GAME : garantie défensive « plus aucun minuteur ne survit » pour TOUS les types
// (contracts/websocket-actions.md §NEW_GAME, effet 1 ; planner-20261011-121500.md « MISE À JOUR 2 »).
// Critère : après un NEW_GAME lancé en pleine action, pendant au moins 2 × la plus longue durée en cours, la phase
// reste NEW_GAME, aucun tick (chrono, RAFALE), aucun OnTimeUp, et le chrono différé de fin de son n'est jamais relancé.
// Les cas (a) RAFALE classique, (b) RAFALE de carte, (c) carte MEMOTION, (d) mémorisation MEMOTION sont des verrous
// (le code v11.1 les ignorait déjà hors STARTED) ; (e) chrono différé est le cas qui échouait avant le correctif.

import (
	"sync/atomic"
	"testing"
	"time"
)

type timerProbe243 struct {
	ticks, timeUps, rafaleTicks atomic.Int32
}

func probe243(e *Engine) *timerProbe243 {
	p := &timerProbe243{}
	e.OnTimerTick = func(int) { p.ticks.Add(1) }
	e.OnTimeUp = func() { p.timeUps.Add(1) }
	e.OnRafaleQuestionTick = func(string, int) { p.rafaleTicks.Add(1) }
	return p
}

// assertQuiet243 : pendant `window`, la phase reste NEW_GAME et aucun minuteur ne s'exprime.
func assertQuiet243(t *testing.T, e *Engine, p *timerProbe243, window time.Duration) {
	t.Helper()
	p.ticks.Store(0)
	p.timeUps.Store(0)
	p.rafaleTicks.Store(0)
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
			t.Fatalf("la phase doit rester NEW_GAME, obtenu %s", ph)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if n := p.ticks.Load(); n != 0 {
		t.Errorf("UPDATE_TIMER après NEW_GAME : %d tick(s)", n)
	}
	if n := p.rafaleTicks.Load(); n != 0 {
		t.Errorf("RAFALE_TICK après NEW_GAME : %d tick(s)", n)
	}
	if n := p.timeUps.Load(); n != 0 {
		t.Errorf("OnTimeUp après NEW_GAME : %d appel(s)", n)
	}
}

// rafaleInQuestion243 place la manche RAFALE en sous-phase QUESTION (sans tirer de question réelle) : sans elle le tick
// du chrono RAFALE est déjà inerte (garde de sous-phase) et les tests ne prouveraient rien.
func rafaleInQuestion243(e *Engine, cardScoped bool) {
	e.SetPhase(PhaseStarted)
	e.mu.Lock()
	defer e.mu.Unlock()
	if cardScoped {
		e.state.MotionActive.State["RAFALE_SUBPHASE"] = string(RafaleSubPhaseQuestion)
		return
	}
	e.state.RafaleSubPhase = RafaleSubPhaseQuestion
}

func newGameNow243(e *Engine) { e.InitGameWith(allOptions243(true, false, false, true)) }

func TestNewGameTimers243_Critical_ClassicRafaleQuestionTimer(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 5 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	p := probe243(e)
	e.Ready("r1", &Question{ID: "r1", Type: QuestionTypeRafale})
	e.StartImmediate(0)
	rafaleInQuestion243(e, false)
	e.StartRafaleQuestionTimer(4)
	time.Sleep(300 * time.Millisecond)

	newGameNow243(e)

	assertQuiet243(t, e, p, 9*time.Second) // 2 × 4 s + marge
}

func TestNewGameTimers243_Critical_MemotionCardTimer(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 5 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	p := probe243(e)
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	e.StartMotionCardTimer(2)
	time.Sleep(300 * time.Millisecond)

	newGameNow243(e)

	assertQuiet243(t, e, p, 5*time.Second)
	if sub := e.GetState().MotionSubPhase; sub != "" && sub != MotionSubPhaseGrid {
		// la sous-phase est remise à zéro ; elle ne doit surtout pas avoir avancé tout seule
		t.Logf("MotionSubPhase après NEW_GAME : %q", sub)
	}
}

func TestNewGameTimers243_Critical_MemotionMemorizeTimer(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 5 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	p := probe243(e)
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 2
	e.Ready("mq1", q)
	e.StartImmediate(0)
	e.InitMotionState() // MEMORIZE
	e.StartMotionMemorizeTimer(2)
	time.Sleep(300 * time.Millisecond)

	newGameNow243(e)

	assertQuiet243(t, e, p, 5*time.Second)
	if sub := e.GetState().MotionSubPhase; sub == MotionSubPhaseGrid {
		t.Errorf("la mémorisation expirée ne doit pas faire passer la grille à GRID après NEW_GAME")
	}
}

// (e) Chrono différé à la fin du son (#219) : NEW_GAME avant la fin du son, puis fin du son simulée.
// Avant le correctif, timerDeferred n'était pas remis à zéro : ReleaseDeferredTimer relançait un chrono vivant en NEW_GAME.
func TestNewGameTimers243_Critical_DeferredAnswerTimerIsNeverRelaunchedAfterNewGame(t *testing.T) {
	if testing.Short() {
		t.Skip("décompte réel de 3 s, exclu de la boucle rapide (-short)")
	}
	e := tw219eStartDeferred(t, 30) // SPEEDY avec son en mode différé, STARTED, chrono en attente de la fin du son
	if !e.GetState().AnswerTimerWaiting {
		t.Fatal("précondition : le chrono de réponse attend la fin du son (ANSWER_TIMER_WAITING)")
	}

	newGameNow243(e)
	e.ReleaseDeferredTimer() // fin du son (naturelle ou chien de garde)

	e.mu.RLock()
	live := e.timer != nil
	e.mu.RUnlock()
	if live {
		t.Error("aucun minuteur ne doit être (re)lancé par la fin du son après NEW_GAME")
	}
	if e.GetState().AnswerTimerWaiting {
		t.Error("ANSWER_TIMER_WAITING doit être faux après NEW_GAME")
	}
	if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
		t.Errorf("phase = %s", ph)
	}
}

func TestNewGameTimers243_DeferredTimerBookkeepingIsClearedByNewGame(t *testing.T) {
	if testing.Short() {
		t.Skip("décompte réel de 3 s, exclu de la boucle rapide (-short)")
	}
	e := tw219eStartDeferred(t, 30)
	newGameNow243(e)
	e.mu.RLock()
	deferred, released := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferred || released {
		t.Errorf("l'état du chrono différé est remis à zéro (timerDeferred=%v, timerReleased=%v)", deferred, released)
	}
}

// Témoins positifs : SANS NEW_GAME, les mêmes minuteurs s'expriment bien (sinon les tests ci-dessus ne prouveraient rien).
func TestNewGameTimers243_Control_TimersDoTickWithoutNewGame(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteurs réels (≈ 3 s), exclu de la boucle rapide (-short)")
	}
	t.Run("chrono de question (TimerTick)", func(t *testing.T) {
		e := NewEngine()
		p := probe243(e)
		e.Ready("q", &Question{ID: "q", Type: QuestionTypeArdoise})
		e.StartImmediate(30)
		time.Sleep(2500 * time.Millisecond)
		if p.ticks.Load() < 1 {
			t.Error("le chrono de question doit ticker (sinon les tests d'arrêt sont vides)")
		}
		e.Stop()
	})
	t.Run("RAFALE classique (RafaleQuestionTick)", func(t *testing.T) {
		e := NewEngine()
		p := probe243(e)
		e.Ready("r1", &Question{ID: "r1", Type: QuestionTypeRafale})
		e.StartImmediate(0)
		rafaleInQuestion243(e, false)
		e.StartRafaleQuestionTimer(5)
		time.Sleep(2500 * time.Millisecond)
		n := p.rafaleTicks.Load()
		e.Stop()
		if n < 1 {
			t.Error("le chrono de question RAFALE classique doit ticker (témoin positif)")
		}
	})
}

// Chaque tick, appelé directement (déterministe), est actif avant le NEW_GAME et devient inerte (garde de sous-phase) après.
// C'est ce qui garantit qu'un ticker encore vivant ne peut plus rien faire. (La RAFALE de carte n'est pas testable en temps
// réel avec les aides existantes : le ticker réel s'arrête sur la garde dès le premier tick.)
func TestNewGameTimers243_Critical_RafaleMotionCardTickBecomesInert(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 5, CategoryHistory, 1)
	card := rafaleMotionCard("mc-r1", []string{string(CategoryHistory)}, []int{1}, 5, 10)
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	rafaleInQuestion243(e, true)
	e.StartRafaleQuestionTimer(5)
	defer e.Stop()
	if r := e.processRafaleQuestionTick(); r.guardFailed || r.paused || r.expired || r.questionTime != 4 {
		t.Fatalf("témoin : le tick doit être actif avant le NEW_GAME (%+v)", r)
	}

	newGameNow243(e)

	if r := e.processRafaleQuestionTick(); !r.guardFailed {
		t.Errorf("après NEW_GAME le tick RAFALE de carte doit être inerte (guardFailed), obtenu %+v", r)
	}
	if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
		t.Errorf("phase = %s", ph)
	}
}

func TestNewGameTimers243_Critical_MemotionCardAndMemorizeTicksBecomeInert(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	e.mu.Lock()
	e.state.CurrentTime = 10
	e.mu.Unlock()
	if r := e.processMotionCardTick(); r.guardFailed || r.paused {
		t.Fatalf("témoin : le tick de carte doit être actif avant le NEW_GAME (%+v)", r)
	}
	newGameNow243(e)
	if r := e.processMotionCardTick(); !r.guardFailed {
		t.Errorf("après NEW_GAME le tick de carte MEMOTION doit être inerte, obtenu %+v", r)
	}

	e2 := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 30
	e2.Ready("mq1", q)
	e2.StartImmediate(0)
	e2.InitMotionState()
	e2.mu.Lock()
	e2.state.CurrentTime = 10
	e2.mu.Unlock()
	if r := e2.processMotionMemorizeTick(); r.guardFailed || r.paused {
		t.Fatalf("témoin : le tick de mémorisation doit être actif avant le NEW_GAME (%+v)", r)
	}
	newGameNow243(e2)
	if r := e2.processMotionMemorizeTick(); !r.guardFailed {
		t.Errorf("après NEW_GAME le tick de mémorisation MEMOTION doit être inerte, obtenu %+v", r)
	}
}

// --- Réserves de la revue (#243) ---------------------------------------------------------------

// (b) Un tick DÉJÀ EN VOL au moment du NEW_GAME (dernier tick calculé à temps 0, verrou relâché) exécute ensuite
// OnTimeUp puis Stop() : cela ne doit ni appeler OnTimeUp ni remettre la phase à STOPPED. La course est rendue
// déterministe en lançant le NEW_GAME DEPUIS le callback OnTimerTick, qui s'exécute précisément entre le calcul du
// tick (processTimerTick) et le traitement de l'expiration.
func TestNewGameTimers243_Critical_InFlightExpiryTickNeverFiresTimeUpNorRestoresStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteur réel (≈ 3 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	var timeUps atomic.Int32
	var injected atomic.Bool
	e.OnTimeUp = func() { timeUps.Add(1) }
	e.OnTimerTick = func(ct int) {
		if ct <= 0 && injected.CompareAndSwap(false, true) {
			e.InitGameWith(allOptions243(true, false, false, true)) // NEW_GAME pile après le calcul du tick d'expiration
		}
	}
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.StartImmediate(1) // un seul tick avant expiration

	deadline := time.Now().Add(3 * time.Second)
	for !injected.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !injected.Load() {
		t.Fatal("précondition : le tick d'expiration n'a pas eu lieu")
	}
	time.Sleep(500 * time.Millisecond) // laisse le goroutine du tick finir son traitement

	if n := timeUps.Load(); n != 0 {
		t.Errorf("OnTimeUp ne doit pas être appelé pour un tick périmé par le NEW_GAME (%d appel(s))", n)
	}
	if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
		t.Errorf("le Stop() tardif du tick ne doit pas remettre la phase à STOPPED, obtenu %s", ph)
	}
}

// (d) Sans RESET_QUESTIONS, le statut de la question INTERROMPUE dépend de l'avancement (contrat « statut de la question
// interrompue ») : jamais lancée → AVAILABLE ; lancée → STOPPED. Avec RESET_QUESTIONS, tout est AVAILABLE.
func TestNewGame243_Critical_InterruptedQuestionStatusWithoutResetQuestions(t *testing.T) {
	cases := []struct {
		phase GamePhase
		want  QuestionStatus
	}{
		{PhasePrepare, "AVAILABLE"},
		{PhaseReady, "AVAILABLE"},
		{PhaseCountdown, "AVAILABLE"},
		{PhaseStarted, "STOPPED"},
		{PhasePaused, "STOPPED"},
	}
	for _, tc := range cases {
		t.Run(string(tc.phase), func(t *testing.T) {
			e := NewEngine()
			e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
			e.SetPhase(tc.phase)
			if tc.phase == PhaseStarted || tc.phase == PhasePaused {
				e.mu.Lock()
				e.setQuestionStatus(StatusStarted) // une question lancée est marquée par le moteur
				e.mu.Unlock()
			}

			e.InitGameWith(allOptions243(false, false, false, false))

			got := e.GetQuestionStatus("q1")
			if got != tc.want && !(tc.want == "AVAILABLE" && got == "") {
				t.Errorf("question interrompue en %s sans RESET_QUESTIONS : statut %q, attendu %q", tc.phase, got, tc.want)
			}
		})
	}
}

func TestNewGame243_InterruptedQuestionIsAvailableWhenQuestionsAreReset(t *testing.T) {
	e := NewEngine()
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseStarted)
	e.InitGameWith(allOptions243(true, false, false, true))
	if got := e.GetQuestionStatus("q1"); got != "AVAILABLE" && got != "" {
		t.Errorf("RESET_QUESTIONS : toutes à jouer, obtenu %q", got)
	}
}

// Résidu de la relecture ciblée : un NEW_GAME qui survient ENTRE le contrôle « le tick est toujours le tick courant »
// et l'arrêt (Stop()) ne doit pas voir sa phase remise à STOPPED. La fenêtre est celle de OnTimeUp (appelé après le contrôle,
// avant Stop()) : on y lance le NEW_GAME, ce qui rend la course déterministe.
func TestNewGameTimers243_Critical_NewGameInsideOnTimeUpIsNotUndoneByTheFollowingStop(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteur réel (≈ 3 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	var injected atomic.Bool
	e.OnTimeUp = func() {
		if injected.CompareAndSwap(false, true) {
			e.InitGameWith(allOptions243(true, false, false, true)) // NEW_GAME pile après le contrôle, avant e.Stop()
		}
	}
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.StartImmediate(1)

	deadline := time.Now().Add(3 * time.Second)
	for !injected.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !injected.Load() {
		t.Fatal("précondition : OnTimeUp n'a pas été appelé (l'expiration normale doit l'appeler)")
	}
	time.Sleep(500 * time.Millisecond)

	if ph := e.GetPhase(); ph != GamePhase("NEW_GAME") {
		t.Errorf("le Stop() qui suit OnTimeUp ne doit pas défaire un NEW_GAME survenu entre-temps, phase = %s", ph)
	}
}

// Témoin : sans NEW_GAME, la même expiration arrête bien la question (le correctif ne désactive pas le STOP normal).
func TestNewGameTimers243_Control_NormalExpiryStillStopsTheQuestion(t *testing.T) {
	if testing.Short() {
		t.Skip("minuteur réel (≈ 3 s), exclu de la boucle rapide (-short)")
	}
	e := NewEngine()
	var timeUps atomic.Int32
	e.OnTimeUp = func() { timeUps.Add(1) }
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseReady)
	e.StartImmediate(1)

	deadline := time.Now().Add(4 * time.Second)
	for e.GetPhase() != PhaseStopped && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if e.GetPhase() != PhaseStopped || timeUps.Load() != 1 {
		t.Errorf("l'expiration normale appelle OnTimeUp une fois puis passe en STOPPED (phase %s, OnTimeUp ×%d)", e.GetPhase(), timeUps.Load())
	}
}
