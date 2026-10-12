package main

// #243 (v11.2) — NEW_GAME coupe le son de question et remet l'état de lecture au repos ; la fin du son (naturelle ou
// chien de garde) ne relance jamais de chrono. Contrat : websocket-actions.md §NEW_GAME effet 1, sound.md §10.7.
// Lecteur factice injecté (tw219dFakeMediaPlayer, question_sound_delayed_219_test.go).

import (
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"encoding/json"
	"testing"
)

func questionSoundState243(e *env246) string {
	var raw struct {
		Game struct {
			State string `json:"QUESTION_SOUND_STATE"`
			Wait  bool   `json:"ANSWER_TIMER_WAITING"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	return raw.Game.State
}

func answerTimerWaiting243(e *env246) bool {
	var raw struct {
		Game struct {
			Wait bool `json:"ANSWER_TIMER_WAITING"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	return raw.Game.Wait
}

func playingSoundEnv243(t *testing.T) (*env246, *tw219dFakeMediaPlayer, *questionSoundAdapter, string) {
	t.Helper()
	e := newEnv249(t, false)
	e.app.config.Sound.Enabled = true
	t.Cleanup(func() { e.app.config.Sound.Enabled = false })
	fake := &tw219dFakeMediaPlayer{}
	qs := &questionSoundAdapter{app: e.app, player: fake}
	e.app.qsound = qs
	_, admin := e.dial(t, "/ws/admin")

	q := &game.Question{ID: "qs1", Type: game.QuestionTypeSpeedy, Sound: "/question/qs1/sound_1.wav", SoundTimerDelayed: true, Time: "30"}
	e.app.engine.Ready("", q)
	e.app.engine.SetPhase(game.PhaseStarted)
	qs.Start(q)
	if fake.State() != audio.MediaPlaying || questionSoundState243(e) != string(game.QuestionSoundPlaying) {
		t.Fatalf("précondition : son en lecture (%v / %s)", fake.State(), questionSoundState243(e))
	}
	return e, fake, qs, admin
}

func TestNewGameSound243_Critical_NewGameStopsTheQuestionSoundAndResetsItsState(t *testing.T) {
	e, fake, _, admin := playingSoundEnv243(t)

	e.send(t, admin, "admin", "NEW_GAME", map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})

	if fake.State() != audio.MediaIdle {
		t.Errorf("le son de question doit être arrêté par NEW_GAME (état du lecteur : %v)", fake.State())
	}
	if got := questionSoundState243(e); got != string(game.QuestionSoundIdle) {
		t.Errorf("QUESTION_SOUND_STATE doit être au repos (IDLE), obtenu %q", got)
	}
}

func TestNewGameSound243_Critical_EndOfSoundAfterNewGameRelaunchesNothing(t *testing.T) {
	e, _, qs, admin := playingSoundEnv243(t)
	e.send(t, admin, "admin", "NEW_GAME", map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})

	qs.onNaturalEnd() // la fin du son arrive quand même (course, ou lecteur qui ne s'est pas encore arrêté)

	if ph := e.app.engine.GetPhase(); ph != game.GamePhase("NEW_GAME") {
		t.Errorf("la phase doit rester NEW_GAME, obtenu %s", ph)
	}
	if answerTimerWaiting243(e) {
		t.Error("ANSWER_TIMER_WAITING doit être faux")
	}
	if got := questionSoundState243(e); got != string(game.QuestionSoundIdle) {
		t.Errorf("QUESTION_SOUND_STATE = %q", got)
	}
}

func TestNewGameSound243_NewGameWithNoSoundAtAllIsHarmless(t *testing.T) {
	e := newEnv249(t, true)
	_, admin := e.dial(t, "/ws/admin")
	e.send(t, admin, "admin", "NEW_GAME", map[string]interface{}{})
	if got := questionSoundState243(e); got != string(game.QuestionSoundIdle) && got != "" {
		t.Errorf("QUESTION_SOUND_STATE au repos : %q", got)
	}
}

func TestNewGameSound243_RefusedNewGameLeavesTheSoundPlaying(t *testing.T) {
	e, fake, _, admin := playingSoundEnv243(t)

	e.send(t, admin, "admin", "NEW_GAME", map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": true})

	if fake.State() != audio.MediaPlaying {
		t.Errorf("un NEW_GAME refusé ne change rien : le son continue (%v)", fake.State())
	}
}
