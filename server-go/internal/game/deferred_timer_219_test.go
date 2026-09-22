// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9 "Chronomètre différé —
// internal/game", contract sound.md §10.7) : le chronomètre de réponse
// différé — maybeStartDeferredTimerUnsafe/ReleaseDeferredTimer/
// resetDeferredTimerUnsafe (engine.go).
//
// ⚠️ Ces scénarios exigent un VRAI actualStart() (countdown réel, ~3s pour
// SPEEDY/QCM/ARDOISE) — StartImmediate() (le raccourci "for tests" de ce
// paquet) N'APPELLE PAS maybeStartDeferredTimerUnsafe : il appelle
// e.startTimer() inconditionnellement (engine.go, doc de StartImmediate :
// "mirrors actualStart(); StartImmediate bypasses it" — le chronomètre
// différé n'a PAS été ajouté à ce miroir, contrairement à RAFALE/ENTRACTE
// qui le sont explicitement). C'est un vrai trou de testabilité, signalé au
// CDP dans le rapport de ce lot — jamais contourné ici en devinant l'état
// interne : chaque scénario passe par le VRAI Start()+countdown pour
// exercer la ligne modifiée d'actualStart() elle-même (R12, le risque le
// plus élevé du plan). Les scénarios indépendants entre eux sont exécutés
// en parallèle (t.Parallel()) pour amortir ce coût réel.
//
// Convention de collision : préfixe tw219e pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package game

import (
	"sync"
	"testing"
	"time"
)

// tw219eCountdownTimeout is generous relative to Start()'s real 3s
// countdown for SPEEDY/QCM/ARDOISE (engine.go: "countdownDuration := 3 //
// default for normal/QCM questions") — bounded, never an indefinite wait.
const tw219eCountdownTimeout = 8 * time.Second

// tw219eStartDeferred takes a fresh Engine through Ready → TransitionToReady
// → Start(delay) for a SPEEDY question carrying a sound in DEFERRED mode,
// and blocks (via OnStateChange, never a blind sleep) until the real
// countdown has actually completed and the engine reports PhaseStarted.
// Fails the test if PhaseStarted isn't reached within tw219eCountdownTimeout.
func tw219eStartDeferred(t *testing.T, delay int) *Engine {
	t.Helper()
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x",
		Sound: "/question/q1/sound_1234.wav", SoundTimerDelayed: true}
	e.Ready(q.ID, q)
	e.TransitionToReady()

	var once sync.Once
	started := make(chan struct{})
	e.OnStateChange = func(phase GamePhase) {
		if phase == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(delay)

	select {
	case <-started:
	case <-time.After(tw219eCountdownTimeout):
		t.Fatalf("setup invalide : PhaseStarted jamais atteint après %s (countdown réel bloqué ?)", tw219eCountdownTimeout)
	}
	return e
}

// tw219eStartSimultaneous mirrors tw219eStartDeferred but for a question
// carrying NO sound at all — the non-regression baseline (CA3: "comportement
// strictement identique à aujourd'hui").
func tw219eStartSimultaneous(t *testing.T, delay int) *Engine {
	t.Helper()
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x"}
	e.Ready(q.ID, q)
	e.TransitionToReady()

	var once sync.Once
	started := make(chan struct{})
	e.OnStateChange = func(phase GamePhase) {
		if phase == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(delay)

	select {
	case <-started:
	case <-time.After(tw219eCountdownTimeout):
		t.Fatalf("setup invalide : PhaseStarted jamais atteint après %s", tw219eCountdownTimeout)
	}
	return e
}

// ---------------------------------------------------------------------------
// CA3 — mode simultané, comportement strictement identique à aujourd'hui.
// ---------------------------------------------------------------------------

func TestDeferredTimer_Simultaneous_StartsTickerImmediately(t *testing.T) {
	t.Parallel()
	e := tw219eStartSimultaneous(t, 20)
	defer e.Stop()

	state := e.GetState()
	if state.AnswerTimerWaiting {
		t.Error("une question SANS son (ou sans différé) ne doit jamais lever ANSWER_TIMER_WAITING")
	}
	if state.CurrentTime != 20 {
		t.Fatalf("setup invalide : CurrentTime = %d juste après STARTED, attendu 20", state.CurrentTime)
	}

	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 19 {
		t.Errorf("mode simultané : le chronomètre doit décompter normalement — CurrentTime après ~1.2s = %d, attendu 19", got)
	}
}

// ---------------------------------------------------------------------------
// CA4/§10.7 — mode différé : figé au temps plein tant que le son "joue"
// (ici : tant que personne n'a appelé ReleaseDeferredTimer), puis démarre à
// la libération.
// ---------------------------------------------------------------------------

func TestDeferredTimer_Deferred_FreezesUntilReleased_ThenStarts(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)
	defer e.Stop()

	state := e.GetState()
	if !state.AnswerTimerWaiting {
		t.Fatal("mode différé : ANSWER_TIMER_WAITING doit être true dès → STARTED (contract §10.7)")
	}
	if state.CurrentTime != 20 {
		t.Fatalf("mode différé : CurrentTime doit être au temps plein (20) dès → STARTED, got %d", state.CurrentTime)
	}

	// Aucun ticker créé : le temps ne doit PAS bouger après plus d'une
	// seconde d'attente réelle.
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 20 {
		t.Errorf("mode différé : aucun ticker ne doit être créé avant la libération — CurrentTime a bougé (got %d, attendu 20 inchangé)", got)
	}
	if !e.GetState().AnswerTimerWaiting {
		t.Error("ANSWER_TIMER_WAITING doit rester true tant que le son n'a pas été libéré")
	}

	// Fin naturelle simulée : le chien de garde/l'adaptateur (cmd/server)
	// appelleraient ceci — au niveau moteur seul, on vérifie l'EFFET de
	// l'appel exporté.
	e.ReleaseDeferredTimer()
	if e.GetState().AnswerTimerWaiting {
		t.Error("ANSWER_TIMER_WAITING doit repasser à false dès la libération")
	}
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 19 {
		t.Errorf("après ReleaseDeferredTimer(), le ticker doit démarrer et décrémenter — CurrentTime après ~1.2s = %d, attendu 19", got)
	}
}

// TestDeferredTimer_ReleaseDeferredTimer_Idempotent verrouille "la
// libération est idempotente — les deux [fin naturelle ET arrêt manuel]
// peuvent survenir" (contract §10.7) — accès direct aux champs internes
// timerDeferred/timerReleased (package `game`, même paquet que engine.go) :
// c'est le seul moyen non-fragile de distinguer "second appel authentiquement
// no-op" de "second appel qui recommence par coïncidence sans effet
// observable" (startTimer() ne remet jamais CurrentTime à zéro, donc un
// second démarrage accidentel du ticker ne serait pas visible autrement).
func TestDeferredTimer_ReleaseDeferredTimer_Idempotent(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)
	defer e.Stop()

	e.ReleaseDeferredTimer() // simule la fin naturelle du son
	e.mu.RLock()
	deferredAfter1, released1 := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferredAfter1 {
		t.Fatalf("après une première libération non pausée, timerDeferred doit être totalement résolu (false), got true")
	}

	e.ReleaseDeferredTimer() // simule un arrêt manuel qui suit la fin naturelle
	e.mu.RLock()
	deferredAfter2, released2 := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferredAfter2 != deferredAfter1 || released2 != released1 {
		t.Errorf("un second appel à ReleaseDeferredTimer() doit être un pur no-op — état avant=(%v,%v) après=(%v,%v)",
			deferredAfter1, released1, deferredAfter2, released2)
	}
}

// ---------------------------------------------------------------------------
// CA14 — libération pendant une pause : démarre à Continue(), jamais
// pendant la pause.
// ---------------------------------------------------------------------------

func TestDeferredTimer_ReleasedWhilePaused_StartsOnlyAtContinue(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)
	defer e.Stop()

	e.Pause()
	if e.GetPhase() != PhasePaused {
		t.Fatalf("setup invalide : Pause() n'a pas atteint PhasePaused, got %s", e.GetPhase())
	}

	e.ReleaseDeferredTimer() // le son se termine PENDANT la pause du jeu
	if e.GetState().AnswerTimerWaiting {
		t.Error("ANSWER_TIMER_WAITING doit repasser à false dès que le son se termine, même en pause")
	}
	// Le chronomètre ne doit PAS avoir démarré pendant la pause : quelle que
	// soit l'attente réelle, CurrentTime reste au temps plein.
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 20 {
		t.Errorf("CA14 : le chronomètre ne doit JAMAIS démarrer pendant une pause, même après la libération — CurrentTime = %d, attendu 20 (figé)", got)
	}

	e.Continue()
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("setup invalide : Continue() n'a pas atteint PhaseStarted, got %s", e.GetPhase())
	}
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 19 {
		t.Errorf("CA14 : le chronomètre doit démarrer À LA REPRISE — CurrentTime après ~1.2s de jeu repris = %d, attendu 19", got)
	}
}

// ---------------------------------------------------------------------------
// resetDeferredTimerUnsafe — Stop()/Ready()/Reveal() avant la fin du son :
// le chronomètre ne démarre jamais, les deux booléens sont remis à zéro.
// ---------------------------------------------------------------------------

func TestDeferredTimer_Stop_BeforeSoundEnds_NeverStartsAndResets(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)

	e.Stop()
	if e.GetPhase() != PhaseStopped {
		t.Fatalf("setup invalide : Stop() n'a pas atteint PhaseStopped, got %s", e.GetPhase())
	}
	if e.GetState().AnswerTimerWaiting {
		t.Error("Stop() avant la fin du son doit remettre ANSWER_TIMER_WAITING à false")
	}
	e.mu.RLock()
	deferred, released := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferred || released {
		t.Errorf("Stop() doit remettre timerDeferred/timerReleased à zéro, got (%v,%v)", deferred, released)
	}

	// Une libération tardive, arrivant APRÈS le Stop (ex: fin naturelle
	// retardée du son), ne doit rien réveiller pour cette question abandonnée.
	e.ReleaseDeferredTimer()
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 0 {
		t.Errorf("une libération tardive après Stop() ne doit jamais faire redémarrer un chronomètre pour une question abandonnée, CurrentTime = %d, attendu 0", got)
	}
}

func TestDeferredTimer_Reveal_BeforeSoundEnds_NeverStartsAndResets(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)
	defer e.Stop()

	e.Pause() // Reveal() n'est autorisé que depuis STOPPED ou PAUSED
	if e.GetPhase() != PhasePaused {
		t.Fatalf("setup invalide : Pause() n'a pas atteint PhasePaused, got %s", e.GetPhase())
	}

	e.Reveal()
	if e.GetPhase() != PhaseRevealed {
		t.Fatalf("setup invalide : Reveal() n'a pas atteint PhaseRevealed, got %s", e.GetPhase())
	}
	if e.GetState().AnswerTimerWaiting {
		t.Error("Reveal() avant la fin du son doit remettre ANSWER_TIMER_WAITING à false")
	}
	e.mu.RLock()
	deferred, released := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferred || released {
		t.Errorf("Reveal() doit remettre timerDeferred/timerReleased à zéro, got (%v,%v)", deferred, released)
	}
}

func TestDeferredTimer_Ready_NewQuestion_ResetsPendingDeferral(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)

	// Ready() n'est autorisé que depuis STOPPED/REVEALED/PREPARE/READY/
	// NEW_GAME (guard d'Engine.Ready) — un Stop() met donc fin à la
	// question précédente en premier, exactement comme le ferait la
	// production (jamais un Ready() direct depuis STARTED). Ce Stop() a
	// déjà sa propre garantie de reset (voir
	// TestDeferredTimer_Stop_BeforeSoundEnds_NeverStartsAndResets) ; ce
	// test-ci vérifie que la chaîne complète Stop()→Ready() laisse la
	// NOUVELLE question dans un état propre, sans qu'une libération tardive
	// de l'ANCIENNE question (le son qui finit après coup) ne puisse
	// réveiller son chronomètre.
	e.Stop()
	e.Ready("q2", &Question{ID: "q2", Type: QuestionTypeSpeedy, Answer: "y"})
	if e.GetState().AnswerTimerWaiting {
		t.Error("Ready() doit remettre ANSWER_TIMER_WAITING à false pour la nouvelle question")
	}
	e.mu.RLock()
	deferred, released := e.timerDeferred, e.timerReleased
	e.mu.RUnlock()
	if deferred || released {
		t.Errorf("Ready() doit remettre timerDeferred/timerReleased à zéro, got (%v,%v)", deferred, released)
	}
}

// ---------------------------------------------------------------------------
// CA16 — non-régression : MEMOTION et ENTRACTE ne démarrent jamais le
// chronomètre global, quel que soit l'état du différé (§0.3 : le champ est
// commun à tous les types, mais la garde de type existante d'actualStart()
// est préservée à l'identique — aucune garde serveur nouvelle n'est ajoutée
// ET l'ancienne n'est pas affaiblie).
// ---------------------------------------------------------------------------

func TestDeferredTimer_CA16_Memotion_NeverStartsGlobalTimerEvenWithDeferredSound(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	// Un son + différé sur une carte MEMOTION est un état QUE LE SERVEUR
	// N'INTERDIT PAS (§0.3) — mais actualStart() doit rester indifférent au
	// champ pour ce type, exactement comme avant #219.
	q := &Question{ID: "q1", Type: QuestionTypeMemotion, Answer: "x",
		Sound: "/question/q1/sound_1234.wav", SoundTimerDelayed: true}
	e.Ready(q.ID, q)
	e.TransitionToReady()

	var once sync.Once
	started := make(chan struct{})
	e.OnStateChange = func(phase GamePhase) {
		if phase == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(20)
	select {
	case <-started:
	case <-time.After(tw219eCountdownTimeout):
		t.Fatal("setup invalide : PhaseStarted jamais atteint pour une question MEMOTION (countdown MEMOTION = 0, ne devrait même pas attendre)")
	}
	defer e.Stop()

	if e.GetState().AnswerTimerWaiting {
		t.Error("CA16 : une question MEMOTION ne doit JAMAIS lever ANSWER_TIMER_WAITING, même avec un son différé attaché — le chronomètre par carte reste le seul mécanisme")
	}
}

func TestDeferredTimer_CA16_Entracte_NeverStartsGlobalTimerEvenWithDeferredSound(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeEntracte, Answer: "",
		Sound: "/question/q1/sound_1234.wav", SoundTimerDelayed: true,
		TypedContent: TypedContent{EntracteConfig: &EntracteConfig{}}}
	e.Ready(q.ID, q)
	e.TransitionToReady()

	var once sync.Once
	started := make(chan struct{})
	e.OnStateChange = func(phase GamePhase) {
		if phase == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(20)
	select {
	case <-started:
	case <-time.After(tw219eCountdownTimeout):
		t.Fatal("setup invalide : PhaseStarted jamais atteint pour une question ENTRACTE")
	}
	defer e.Stop()

	if e.GetState().AnswerTimerWaiting {
		t.Error("CA16 : une question ENTRACTE ne doit JAMAIS lever ANSWER_TIMER_WAITING — ENTRACTE n'a aucun chronomètre (contract game-state.md)")
	}
}

// TestDeferredTimer_CA13_ReplayNeverResetsAlreadyRunningTimer proves, at the
// engine level, the primitive CA13 relies on: once ReleaseDeferredTimer()
// has fully resolved the deferral (timerDeferred back to false), calling it
// AGAIN changes nothing — the same idempotence already verified above,
// restated here against the actual observable CurrentTime to show a
// "Rejouer" that (at the cmd/server layer) would call this again truly has
// no way to roll CurrentTime back.
func TestDeferredTimer_CA13_ReplayNeverResetsAlreadyRunningTimer(t *testing.T) {
	t.Parallel()
	e := tw219eStartDeferred(t, 20)
	defer e.Stop()

	e.ReleaseDeferredTimer()
	time.Sleep(1200 * time.Millisecond)
	before := e.GetState().CurrentTime
	if before != 19 {
		t.Fatalf("setup invalide : CurrentTime = %d après ~1.2s de course, attendu 19", before)
	}

	// "Rejouer" au niveau adaptateur n'appelle jamais ReleaseDeferredTimer
	// (CA13, cmd/server/question_sound.go) — mais MÊME si un appel
	// erroné avait lieu ici, l'idempotence de la méthode elle-même garantit
	// qu'il ne regèlerait rien.
	e.ReleaseDeferredTimer()
	after := e.GetState().CurrentTime
	if after > before {
		t.Errorf("CA13 : un second ReleaseDeferredTimer() ne doit jamais faire revenir CurrentTime en arrière — before=%d after=%d", before, after)
	}
}
