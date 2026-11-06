// #287/#288 (v11.4.0, LOT-3) — tests unitaires dev-backend de l'adaptateur de décompte
// (contracts/sound.md §12) : gardes, Play/PlayLoop, idempotence, frontière AST des trois chemins audio
// et non-deadlock du rappel moteur (émis sous e.mu). Lecteur n°2 remplacé par un faux journalisé.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

type dev287Fake struct {
	mu    sync.Mutex
	calls []string
}

func (f *dev287Fake) rec(c string) { f.mu.Lock(); f.calls = append(f.calls, c); f.mu.Unlock() }
func (f *dev287Fake) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
func (f *dev287Fake) Play(p string) error     { f.rec("Play"); return nil }
func (f *dev287Fake) PlayLoop(p string) error { f.rec("PlayLoop"); return nil }
func (f *dev287Fake) Pause()                  { f.rec("Pause") }
func (f *dev287Fake) Resume()                 { f.rec("Resume") }
func (f *dev287Fake) Stop()                   { f.rec("Stop") }
func (f *dev287Fake) State() audio.MediaState { return audio.MediaIdle }
func (f *dev287Fake) Close() error            { return nil }

func dev287Adapter(t *testing.T, sc config.SoundConfig) (*countdownSoundAdapter, *dev287Fake) {
	t.Helper()
	prev := config.Get()
	config.SetInstance(&config.Config{Sound: sc})
	t.Cleanup(func() { config.SetInstance(prev) })
	f := &dev287Fake{}
	cs := &countdownSoundAdapter{app: newTestApp(t), player: f, soundEnabled: func() bool { return sc.Enabled }}
	return cs, f
}

func TestCountdownAdapter_Guards(t *testing.T) {
	global := game.AnswerTimerStart{Kind: game.AnswerTimerGlobal, Seconds: 30}
	for _, tc := range []struct {
		name  string
		sc    config.SoundConfig
		start game.AnswerTimerStart
		want  []string
	}{
		{"default loops", config.SoundConfig{Enabled: true}, global, []string{"PlayLoop"}},
		{"once plays once", config.SoundConfig{Enabled: true, CountdownOnce: true}, global, []string{"Play"}},
		{"muted", config.SoundConfig{Enabled: true, Muted: true}, global, nil},
		{"sound disabled", config.SoundConfig{Enabled: false}, global, nil},
		{"countdown disabled", config.SoundConfig{Enabled: true, CountdownDisabled: true}, global, nil},
		{"question with media", config.SoundConfig{Enabled: true}, game.AnswerTimerStart{Kind: game.AnswerTimerGlobal, Seconds: 30, QuestionSound: "/question/1/sound_1.wav"}, nil},
		{"motion card", config.SoundConfig{Enabled: true}, game.AnswerTimerStart{Kind: game.AnswerTimerMotionCard, Seconds: 10}, []string{"PlayLoop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs, f := dev287Adapter(t, tc.sc)
			cs.OnAnswerTimerStarted(tc.start)
			got := f.log()
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("appels = %v, attendu %v", got, tc.want)
			}
		})
	}
}

func TestCountdownAdapter_StopPauseResumeAndNilSafe(t *testing.T) {
	cs, f := dev287Adapter(t, config.SoundConfig{Enabled: true})
	cs.Pause()
	cs.Resume()
	cs.Stop()
	cs.Stop() // idempotent
	if got := f.log(); len(got) != 4 || got[0] != "Pause" || got[1] != "Resume" || got[2] != "Stop" {
		t.Fatalf("journal = %v", got)
	}
	var nilCS *countdownSoundAdapter // nil-safe: a.countdownSound() is nil before setup()
	nilCS.Stop()
	nilCS.Pause()
	nilCS.Resume()
	nilCS.OnAnswerTimerStarted(game.AnswerTimerStart{Seconds: 5})
	if nilCS.Test() || nilCS.OutputAvailable() {
		t.Fatal("un adaptateur nil ne doit ni jouer ni se dire disponible")
	}
}

func TestCountdownAdapter_Test_IgnoresMutedAndDisabled(t *testing.T) {
	cs, f := dev287Adapter(t, config.SoundConfig{Enabled: true, Muted: true, CountdownDisabled: true, CountdownOnce: false})
	if !cs.Test() {
		t.Fatal("Test() doit jouer même coupé / désactivé (geste explicite, §12.4)")
	}
	if got := f.log(); len(got) != 1 || got[0] != "Play" {
		t.Fatalf("Test() doit jouer UNE fois (Play, jamais PlayLoop), journal = %v", got)
	}
}

// §12.5 : OnAnswerTimerStarted est émis SOUS e.mu — un rappel qui rappellerait le moteur se bloquerait.
// Le vrai câblage (setupCallbacks) + vrai moteur : démarrer une question ne doit jamais bloquer.
func TestCountdown_EngineCallbackUnderLock_NoDeadlock(t *testing.T) {
	app := newTestApp(t)
	f := &dev287Fake{}
	app.csound = &countdownSoundAdapter{app: app, player: f, soundEnabled: func() bool { return true }}
	app.engine.OnAnswerTimerStarted = func(s game.AnswerTimerStart) { app.countdownSound().OnAnswerTimerStarted(s) }
	app.engine.Ready("q", &game.Question{ID: "q", Question: "Q", Type: game.QuestionTypeSpeedy, Time: "30", Points: "1"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.engine.StartImmediate(30)
		_ = app.engine.GetState() // prend e.mu : bloquerait si le rappel l'avait gardé
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock : StartImmediate/GetState bloqués — le rappel sous verrou a rappelé le moteur")
	}
	app.engine.Stop()
	if got := f.log(); len(got) == 0 || got[0] != "PlayLoop" {
		t.Fatalf("le démarrage du chrono global doit lancer le décompte en boucle, journal = %v", got)
	}
}

// §12.1 : frontière des trois chemins audio, vérifiée sur l'AST (sélecteurs), pas par discipline.
func TestCountdownSound_AudioPathFrontier(t *testing.T) {
	selectors := func(file string) map[string]bool {
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, filepath.Join(cmdServerDir227(t), file), nil, 0)
		if err != nil {
			t.Fatalf("parse %s : %v", file, err)
		}
		out := map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if se, ok := n.(*ast.SelectorExpr); ok {
				out[se.Sel.Name] = true
			}
			return true
		})
		return out
	}
	cd := selectors("countdown_sound.go")
	for _, forbidden := range []string{"notifySound", "PlayCue", "questionSound"} {
		if cd[forbidden] {
			t.Errorf("countdown_sound.go ne doit jamais référencer %q (§12.1)", forbidden)
		}
	}
	// Rappel sous verrou : aucun appel moteur depuis l'adaptateur (§12.5).
	for _, forbidden := range []string{"GetState", "ReleaseDeferredTimer", "SetQuestionSoundState", "broadcastUpdate"} {
		if cd[forbidden] {
			t.Errorf("countdown_sound.go ne doit pas appeler %q (rappel émis sous e.mu, §12.5)", forbidden)
		}
	}
	if selectors("question_sound.go")["countdownSound"] {
		t.Error("question_sound.go ne doit pas référencer countdownSound (§12.1)")
	}
}

// #288 (§13.3) : boucle + différé — la fin du 1er passage libère le chrono SANS passer
// QUESTION_SOUND_STATE à IDLE (le média continue) ; PlayLoop est bien utilisé.
func TestQuestionSoundLoop_FirstPassEnd_ReleasesTimerKeepsPlaying(t *testing.T) {
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()
	q.SoundLoop = true
	app.engine.Ready(q.ID, q)

	tw219qSetSoundEnabledFor(app, true)
	app.qsound.Start(q)
	if fake.playCount() != 1 {
		t.Fatalf("lecture attendue 1 fois, got %d", fake.playCount())
	}
	app.qsound.onNaturalEnd() // fin du 1er passage ; le faux lecteur reste PLAYING (boucle)
	st := app.engine.GetState()
	if st.AnswerTimerWaiting {
		t.Error("la fin du 1er passage doit libérer le chrono différé")
	}
	if st.QuestionSoundState != game.QuestionSoundPlaying {
		t.Errorf("QUESTION_SOUND_STATE doit rester PLAYING pendant la boucle, got %q", st.QuestionSoundState)
	}
}
