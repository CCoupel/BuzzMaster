// #287 (v11.4.0, LOT-3) — contracts/sound.md §12.5 : le moteur émet OnAnswerTimerStarted quand un chronomètre de
// RÉPONSE démarre réellement (startTimer() global, y compris la MANCHE RAFALE ; StartMotionCardTimer(delay>0)), et
// JAMAIS pour le ticker par question RAFALE, la mémorisation MEMOTION ni la phase COUNTDOWN. §12.8 : OnMotionCardTimeUp
// à l'expiration d'une carte. Frontière publique uniquement (rappels + méthodes exportées) ; helpers existants
// (tw282Engine, makeMotionQuestion, memoryMotionCard, startMEMOTION). Préfixe tw287.
//
// Écrits AVANT le code (TDD) : ne compilent qu'une fois AnswerTimerStart / AnswerTimerKind / OnAnswerTimerStarted /
// OnMotionCardTimeUp livrés (B5).
package game

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

// tw287Start = copie de game.AnswerTimerStart lue par réflexion : le fichier COMPILE avant le code (TDD) et
// échoue par assertion tant que le champ/ le type n'existe pas.
type tw287Start struct {
	Kind    string
	Seconds int
	Sound   string
}

type tw287Recorder struct {
	mu     sync.Mutex
	starts []tw287Start
}

func (r *tw287Recorder) add(s tw287Start) {
	r.mu.Lock()
	r.starts = append(r.starts, s)
	r.mu.Unlock()
}

func (r *tw287Recorder) snapshot() []tw287Start {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]tw287Start(nil), r.starts...)
}

// tw287Hook branche le rappel Engine.OnAnswerTimerStarted (type func(AnswerTimerStart)) sur le recorder.
func tw287Hook(t *testing.T, e *Engine) *tw287Recorder {
	t.Helper()
	rec := &tw287Recorder{}
	f := reflect.ValueOf(e).Elem().FieldByName("OnAnswerTimerStarted")
	if !f.IsValid() {
		t.Fatal("Engine.OnAnswerTimerStarted absent (contrat sound.md §12.5 non livré)")
	}
	f.Set(reflect.MakeFunc(f.Type(), func(args []reflect.Value) []reflect.Value {
		a := args[0]
		rec.add(tw287Start{
			Kind:    a.FieldByName("Kind").String(),
			Seconds: int(a.FieldByName("Seconds").Int()),
			Sound:   a.FieldByName("QuestionSound").String(),
		})
		return nil
	}))
	return rec
}

// tw287HookMotionTimeUp branche Engine.OnMotionCardTimeUp (func()).
func tw287HookMotionTimeUp(t *testing.T, e *Engine, fn func()) {
	t.Helper()
	f := reflect.ValueOf(e).Elem().FieldByName("OnMotionCardTimeUp")
	if !f.IsValid() {
		t.Fatal("Engine.OnMotionCardTimeUp absent (contrat sound.md §12.5 non livré)")
	}
	f.Set(reflect.ValueOf(fn))
}

func (r *tw287Recorder) wantExactly(t *testing.T, want ...tw287Start) {
	t.Helper()
	got := r.snapshot()
	if len(got) != len(want) {
		t.Fatalf("OnAnswerTimerStarted émis %d fois (%+v), attendu %d (%+v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("émission %d = %+v, attendu %+v", i, got[i], want[i])
		}
	}
}

// ----- (a) chronomètre global : SPEEDY / QCM / ARDOISE -----

func TestAnswerTimerStarted287_GlobalKinds_EmitOnceWithKindSecondsAndQuestionSound(t *testing.T) {
	for _, typ := range []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeArdoise} {
		t.Run(string(typ), func(t *testing.T) {
			e := NewEngine()
			rec := tw287Hook(t, e)
			e.Ready("q1", &Question{ID: "q1", Type: typ, Answer: "x"})
			e.StartImmediate(30)
			t.Cleanup(e.Stop)
			rec.wantExactly(t, tw287Start{"GLOBAL", 30, ""})
		})
	}
}

func TestAnswerTimerStarted287_CarriesQuestionSoundURL(t *testing.T) {
	e := NewEngine()
	rec := tw287Hook(t, e)
	e.Ready("q1", &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav"})
	e.StartImmediate(20)
	t.Cleanup(e.Stop)
	rec.wantExactly(t, tw287Start{"GLOBAL", 20, "/question/q1/sound_1.wav"})
}

// §12.5 : « émis sous e.mu » — le rappel s'exécute pendant que le moteur tient son verrou ; un rappel qui rappelle
// le moteur de façon synchrone se bloquerait. Ici le rappel ne rappelle rien : on vérifie seulement que l'émission
// ne bloque ni ne panique et que le moteur reste utilisable juste après (GetState prend le même verrou).
func TestAnswerTimerStarted287_EmittedUnderLock_EngineStaysUsable(t *testing.T) {
	e := NewEngine()
	rec := tw287Hook(t, e)
	e.Ready("q1", &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x"})
	done := make(chan struct{})
	go func() { e.StartImmediate(10); _ = e.GetState(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("StartImmediate/GetState bloqués : deadlock autour de l'émission sous verrou")
	}
	t.Cleanup(e.Stop)
	if len(rec.snapshot()) != 1 {
		t.Fatal("OnAnswerTimerStarted non émis exactement une fois")
	}
}

// ----- chronomètre différé (§10.7) : émis seulement quand le chrono tourne réellement -----

func TestAnswerTimerStarted287_DeferredTimer_NotEmittedWhileWaiting_EmittedOnRelease(t *testing.T) {
	e := NewEngine()
	rec := tw287Hook(t, e)
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav", SoundTimerDelayed: true}
	e.Ready(q.ID, q)
	e.TransitionToReady()

	started := make(chan struct{})
	var once sync.Once
	e.OnStateChange = func(p GamePhase) {
		if p == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(25)
	select {
	case <-started:
	case <-time.After(8 * time.Second):
		t.Fatal("PhaseStarted jamais atteint")
	}
	t.Cleanup(e.Stop)

	rec.wantExactly(t) // STARTED mais chrono en attente : aucune émission (§12.5, « pas un changement de phase »)

	e.ReleaseDeferredTimer()
	got := rec.snapshot()
	if len(got) != 1 || got[0].Kind != "GLOBAL" || got[0].Seconds != 25 {
		t.Fatalf("après libération : %+v, attendu 1 émission GLOBAL 25 s", got)
	}
	e.ReleaseDeferredTimer() // idempotent
	if len(rec.snapshot()) != 1 {
		t.Fatalf("seconde libération : %d émissions, attendu toujours 1", len(rec.snapshot()))
	}
}

func TestAnswerTimerStarted287_DeferredReleasedWhilePaused_EmittedOnContinue(t *testing.T) {
	e := NewEngine()
	rec := tw287Hook(t, e)
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav", SoundTimerDelayed: true}
	e.Ready(q.ID, q)
	e.TransitionToReady()
	started := make(chan struct{})
	var once sync.Once
	e.OnStateChange = func(p GamePhase) {
		if p == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(25)
	select {
	case <-started:
	case <-time.After(8 * time.Second):
		t.Fatal("PhaseStarted jamais atteint")
	}
	t.Cleanup(e.Stop)

	e.PauseAll()
	e.ReleaseDeferredTimer() // le son se termine PENDANT la pause
	rec.wantExactly(t)       // chrono toujours figé : rien d'émis
	e.Continue()
	got := rec.snapshot()
	if len(got) != 1 || got[0].Kind != "GLOBAL" {
		t.Fatalf("après CONTINUE : %+v, attendu 1 émission GLOBAL (§12.5, 3ᵉ appelant de startTimer)", got)
	}
}

// ----- (a) manche RAFALE : UNE émission pour toute la manche, jamais par question -----

func TestAnswerTimerStarted287_RafaleRound_OneGlobalEmission_NotPerQuestion(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"a1", "Abricot"}, {"b1", "Banane"}, {"c1", "Cerise"}, {"d1", "Datte"}, {"e1", "Eau"}, {"f1", "Figue"}}, false, 100)
	rec := tw287Hook(t, e)
	e.StartImmediate(120)
	t.Cleanup(e.Stop)

	got := rec.snapshot()
	if len(got) != 1 || got[0].Kind != "GLOBAL" || got[0].Seconds != 120 {
		t.Fatalf("démarrage de manche : %+v, attendu 1 émission GLOBAL 120 s (chrono de manche)", got)
	}

	// 5 questions enchaînées (validation, invalidation) + ticker par question : AUCUNE émission de plus.
	e.StartRafaleQuestionTimer(3)
	for i := 0; i < 2; i++ {
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate %d : %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := e.RafaleInvalidate(); err != nil {
			t.Fatalf("RafaleInvalidate %d : %v", i, err)
		}
	}
	e.StartRafaleQuestionTimer(3)
	if n := len(rec.snapshot()); n != 1 {
		t.Fatalf("%d émissions après 5 questions, attendu 1 (une seule lecture par manche, §12.5/§12.7)", n)
	}
}

// ----- (a) carte MEMOTION : StartMotionCardTimer -----

func TestAnswerTimerStarted287_MotionCardTimer_EmitsMotionCardKind_OnlyWhenDelayPositive(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-1", 1, 2, 0)
	startMEMOTION(t, e, "mq-1", makeMotionQuestion("mq-1", []MotionCard{card}, "SOLO"))
	t.Cleanup(e.Stop)
	rec := tw287Hook(t, e)

	e.StartMotionCardTimer(0)
	rec.wantExactly(t) // delay <= 0 : aucun chrono, aucune émission

	e.StartMotionCardTimer(30)
	got := rec.snapshot()
	if len(got) != 1 || got[0].Kind != "MOTION_CARD" || got[0].Seconds != 30 {
		t.Fatalf("StartMotionCardTimer(30) : %+v, attendu 1 émission MOTION_CARD 30 s", got)
	}
}

func TestAnswerTimerStarted287_MotionMemorizeTimer_NeverEmits(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-1", 1, 2, 0)
	startMEMOTION(t, e, "mq-1", makeMotionQuestion("mq-1", []MotionCard{card}, "SOLO"))
	t.Cleanup(e.Stop)
	rec := tw287Hook(t, e)
	e.StartMotionMemorizeTimer(5)
	rec.wantExactly(t) // la mémorisation n'est pas un chronomètre de réponse (§12.5)
}

// ----- phase COUNTDOWN (3-2-1) : jamais -----

func TestAnswerTimerStarted287_PreStartCountdown_NeverEmitsBeforeTheRealStart(t *testing.T) {
	e := NewEngine()
	rec := tw287Hook(t, e)
	e.Ready("q1", &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x"})
	e.TransitionToReady()

	ticks := make(chan int, 8)
	e.OnCountdownTick = func(n int) { ticks <- n }
	started := make(chan struct{})
	var once sync.Once
	e.OnStateChange = func(p GamePhase) {
		if p == PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	e.Start(15)
	select {
	case <-ticks: // premier tick du 3-2-1
		if n := len(rec.snapshot()); n != 0 {
			t.Fatalf("%d émission(s) pendant le 3-2-1, attendu 0 (§12.5 : la phase COUNTDOWN ne joue rien)", n)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("aucun tick de décompte observé")
	}
	select {
	case <-started:
	case <-time.After(8 * time.Second):
		t.Fatal("PhaseStarted jamais atteint")
	}
	t.Cleanup(e.Stop)
	if n := len(rec.snapshot()); n != 1 {
		t.Fatalf("%d émission(s) après le vrai START, attendu 1", n)
	}
}

// ----- OnMotionCardTimeUp -----

func TestOnMotionCardTimeUp287_FiredOnceAtCardExpiry_PhaseStaysStarted(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-1", 1, 2, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card) // carte sélectionnée + retournée : le ticker de carte a un état actif
	t.Cleanup(e.Stop)

	fired := make(chan struct{}, 4)
	tw287HookMotionTimeUp(t, e, func() { fired <- struct{}{} })
	e.StartMotionCardTimer(1)

	select {
	case <-fired:
	case <-time.After(4 * time.Second):
		t.Fatal("OnMotionCardTimeUp jamais appelé à l'expiration de la carte (§12.8)")
	}
	if p := e.GetState().Phase; p != PhaseStarted {
		t.Errorf("phase = %s après expiration de carte, attendu STARTED (l'admin doit agir)", p)
	}
	select {
	case <-fired:
		t.Error("OnMotionCardTimeUp appelé deux fois pour une seule expiration")
	case <-time.After(1500 * time.Millisecond):
	}
}

func TestOnMotionCardTimeUp287_NotFiredWhenCardTimerStoppedBeforeExpiry(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-1", 1, 2, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card) // carte sélectionnée + retournée : le ticker de carte a un état actif
	t.Cleanup(e.Stop)
	fired := make(chan struct{}, 1)
	tw287HookMotionTimeUp(t, e, func() { fired <- struct{}{} })
	e.StartMotionCardTimer(2)
	e.StopMotionCardTimer()
	select {
	case <-fired:
		t.Error("OnMotionCardTimeUp appelé alors que le chrono de carte a été arrêté avant expiration")
	case <-time.After(2500 * time.Millisecond):
	}
}
