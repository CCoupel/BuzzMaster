// #288 (v11.4.0, LOT-3) — média de question « une fois / en boucle » côté App, contracts/sound.md §13.3 :
// `SOUND_LOOP` ⇒ PlayLoop au lieu de Play ; boucle + différé ⇒ le chrono démarre à la fin du 1ᵉʳ passage
// (onNaturalEnd) exactement une fois, QUESTION_SOUND_STATE reste PLAYING ; la boucle s'arrête sur tous les sites
// existants de questionSound().Stop(), jamais au buzz ; PAUSE/CONTINUE la suspend/reprend.
// Réutilise tw287Fake / tw287App / tw287Config (countdown_sound_287_test.go) ; décompte désactivé par config.
// Compile avant le code (champ game.Question.SoundLoop déjà présent ; PlayLoop porté par tw287Fake).
package main

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

const tw288Cfg = `{"sound":{"enabled":true,"muted":false,"countdown_disabled":true,"countdown_once":false}}`

func tw288Question(loop, delayed bool) *game.Question {
	return &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Time: "30",
		Sound: "/question/q1/sound_1.wav", SoundLoop: loop, SoundTimerDelayed: delayed}
}

func tw288QuestionSoundState(app *App) string {
	var raw struct {
		Game struct {
			State string `json:"QUESTION_SOUND_STATE"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(app.engine.GetGameJSON(), &raw)
	return raw.Game.State
}

// tw288Install branche un faux lecteur n°1 (média de question) sur l'App de tw287App.
func tw288Install(t *testing.T) (*App, *tw287Fake, *questionSoundAdapter) {
	t.Helper()
	app, _ := tw287App(t, tw288Cfg)
	fake := &tw287Fake{}
	qs := &questionSoundAdapter{app: app, player: fake}
	app.qsound = qs
	t.Cleanup(func() { tw257CancelWatchdog(app) })
	return app, fake, qs
}

// ---------- lecture ----------

func TestQuestionSoundLoop288_Start_LoopUsesPlayLoop_NotPlay(t *testing.T) {
	app, fake, qs := tw288Install(t)
	q := tw288Question(true, false)
	app.engine.Ready(q.ID, q)
	qs.Start(q)
	if fake.count("PlayLoop:sound_1.wav") != 1 || fake.count("Play:") != 0 {
		t.Fatalf("SOUND_LOOP=true : un PlayLoop et aucun Play attendu, journal %v", fake.journal())
	}
}

func TestQuestionSoundLoop288_Start_NoLoop_StillUsesPlay_NonRegression(t *testing.T) {
	app, fake, qs := tw288Install(t)
	q := tw288Question(false, false)
	app.engine.Ready(q.ID, q)
	qs.Start(q)
	if fake.count("Play:sound_1.wav") != 1 || fake.count("PlayLoop") != 0 {
		t.Fatalf("SOUND_LOOP absent : Play inchangé (§13.4), journal %v", fake.journal())
	}
}

func TestQuestionSoundLoop288_Rejouer_FollowsTheLoopSetting(t *testing.T) {
	app, fake, qs := tw288Install(t)
	q := tw288Question(true, false)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(30) // le câblage de production (OnStateChange → Start) lance déjà un premier PlayLoop
	fake.reset()
	qs.HandleCommand(protocol.QuestionSoundCommandPlay)
	if fake.count("PlayLoop:sound_1.wav") != 1 {
		t.Fatalf("« Rejouer » avec boucle ⇒ PlayLoop (§13.3), journal %v", fake.journal())
	}
}

// ---------- boucle × différé ----------

// tw288Deferred démarre réellement (Start + 3-2-1) une question différée+boucle ; le chrono attend le son.
func tw288Deferred(t *testing.T) (*App, *tw287Fake, *questionSoundAdapter, *tw287EmissionCounter) {
	t.Helper()
	app, fake, qs := tw288Install(t)
	q := tw288Question(true, true)
	app.engine.Ready(q.ID, q)
	app.engine.TransitionToReady()
	counter := tw287CountEmissions(t, app.engine)
	var once sync.Once
	started := make(chan struct{})
	app.engine.OnStateChange = func(p game.GamePhase) {
		if p == game.PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	app.engine.Start(20)
	select {
	case <-started:
	case <-time.After(8 * time.Second):
		t.Fatal("setup invalide : PhaseStarted jamais atteint")
	}
	if !app.engine.GetState().AnswerTimerWaiting {
		t.Fatal("setup invalide : ANSWER_TIMER_WAITING attendu en mode différé")
	}
	qs.Start(q)
	return app, fake, qs, counter
}

func TestQuestionSoundLoop288_LoopPlusDeferred_TimerStartsAtFirstPassEnd_ExactlyOnce(t *testing.T) {
	app, fake, qs, counter := tw288Deferred(t)
	if fake.count("PlayLoop") != 1 {
		t.Fatalf("PlayLoop attendu, journal %v", fake.journal())
	}
	if counter.n() != 0 {
		t.Fatalf("chrono démarré avant la fin du 1er passage (%d émission)", counter.n())
	}

	qs.onNaturalEnd() // fin du 1er passage (le fake simule l'évènement du lecteur)
	fake.mu.Lock()
	fake.state = audio.MediaPlaying // la boucle tourne toujours
	fake.mu.Unlock()

	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("ANSWER_TIMER_WAITING doit tomber à la fin du 1er passage (§13.3)")
	}
	if counter.n() != 1 {
		t.Errorf("le chrono doit démarrer exactement une fois à la fin du 1er passage, émissions=%d", counter.n())
	}
	if got := tw288QuestionSoundState(app); got != string(game.QuestionSoundPlaying) {
		t.Errorf("QUESTION_SOUND_STATE = %q, attendu PLAYING tant que la boucle joue (§13.3)", got)
	}
	if fake.count("Stop") != 0 {
		t.Errorf("la boucle ne doit pas être arrêtée par la fin du 1er passage, journal %v", fake.journal())
	}
}

func TestQuestionSoundLoop288_LoopPlusDeferred_LaterPassesReleaseNothing(t *testing.T) {
	app, fake, qs, counter := tw288Deferred(t)
	qs.onNaturalEnd()
	fake.mu.Lock()
	fake.state = audio.MediaPlaying
	fake.mu.Unlock()
	qs.onNaturalEnd() // un passage suivant qui signalerait à tort (hors contrat) : inoffensif
	qs.onNaturalEnd()
	if counter.n() != 1 {
		t.Errorf("émissions du chrono = %d après plusieurs signaux, attendu 1", counter.n())
	}
	if got := tw288QuestionSoundState(app); got != string(game.QuestionSoundPlaying) {
		t.Errorf("QUESTION_SOUND_STATE = %q, attendu PLAYING", got)
	}
}

// ---------- arrêt : tous les sites existants, jamais au buzz ----------

func tw288Looping(t *testing.T) (*App, *tw287Fake) {
	t.Helper()
	app, fake, _ := tw288Install(t)
	q := tw288Question(true, false)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(30) // OnStateChange(STARTED) → questionSound().Start(q) (câblage de production)
	if fake.count("PlayLoop") != 1 {
		t.Fatalf("setup invalide : boucle non démarrée, journal %v", fake.journal())
	}
	return app, fake
}

func TestQuestionSoundLoop288_StopSites_EachStopsTheLoop(t *testing.T) {
	sites := map[string]func(t *testing.T, app *App){
		"OnTimeUp": func(t *testing.T, app *App) { app.engine.OnTimeUp() },
		"STOP":     func(t *testing.T, app *App) { tw287Send(t, app, protocol.ActionStop, map[string]interface{}{}) },
		"REVEAL":   func(t *testing.T, app *App) { tw287Send(t, app, protocol.ActionReveal, map[string]interface{}{}) },
		"NEW_GAME": func(t *testing.T, app *App) {
			tw287Send(t, app, protocol.ActionNewGame, map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})
		},
		"READY": func(t *testing.T, app *App) {
			msg, _ := protocol.NewMessage(protocol.ActionReady, protocol.ReadyPayload{Question: ""})
			app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: "adm-288", ClientType: "admin"})
		},
		"muted → true": func(t *testing.T, app *App) { app.httpServer.OnSoundMutedChange(true) },
	}
	for name, do := range sites {
		t.Run(name, func(t *testing.T) {
			app, fake := tw288Looping(t)
			fake.reset()
			do(t, app)
			if fake.count("Stop") < 1 {
				t.Fatalf("site %q : la boucle doit être arrêtée (Stop), journal %v", name, fake.journal())
			}
		})
	}
}

func TestQuestionSoundLoop288_Buzz_LoopKeepsPlaying(t *testing.T) {
	app, fake := tw288Looping(t)
	fake.reset()
	app.engine.Pause() // le buzz
	time.Sleep(150 * time.Millisecond)
	if got := fake.journal(); len(got) != 0 {
		t.Fatalf("au buzz la boucle continue : aucun appel au lecteur attendu, journal %v", got)
	}
}

func TestQuestionSoundLoop288_ManualPauseContinue_SuspendsThenResumesLoop(t *testing.T) {
	app, fake := tw288Looping(t)
	fake.reset()
	tw287Send(t, app, protocol.ActionPause, map[string]interface{}{})
	tw287Waiting(t, fake, "Pause", 1)
	tw287Send(t, app, protocol.ActionContinue, map[string]interface{}{})
	tw287Waiting(t, fake, "Resume", 1)
	if fake.count("PlayLoop") != 0 || fake.count("Play:") != 0 {
		t.Fatalf("PAUSE/CONTINUE ne relance pas la boucle depuis le début, journal %v", fake.journal())
	}
}

// Le décompte n'est jamais lancé sur une question à média, boucle comprise (§12.6/§13.3) — vérifié avec le
// décompte ACTIF cette fois.
func TestQuestionSoundLoop288_WithLoopMedia_NoCountdownPlayed(t *testing.T) {
	app, cd := tw287App(t, tw287Active)
	qfake := &tw287Fake{}
	app.qsound = &questionSoundAdapter{app: app, player: qfake}
	t.Cleanup(func() { tw257CancelWatchdog(app) })
	q := tw288Question(true, false)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(30)
	time.Sleep(150 * time.Millisecond)
	if cd.count("Play") != 0 {
		t.Errorf("décompte lancé sur une question à média en boucle : %v", cd.journal())
	}
	if qfake.count("PlayLoop") != 1 {
		t.Errorf("le média doit boucler, journal %v", qfake.journal())
	}
}
