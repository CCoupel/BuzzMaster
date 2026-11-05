// #286 (v11.4.0, LOT-1) — `sound.muted` côté App : cues silencieuses, média de question libéré à T1,
// gate T0 inchangée. Contrat : contracts/sound.md §11.2.
//
// Frontière uniquement (docs/TEST_PROCEDURE.md §2.3) : la config est posée par JSON
// (json.Unmarshal dans config.Get()), jamais par le champ typé — le fichier compile avant le dev
// et échoue par assertion. Réutilise twa227WireSound / tw219qDeferredApp (même paquet).
package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// tw286SetMuted pose sound.muted dans la config process-wide par le JSON, et restaure à la fin.
func tw286SetMuted(t *testing.T, muted bool) {
	t.Helper()
	set := func(v bool) {
		raw := `{"sound":{"muted":false}}`
		if v {
			raw = `{"sound":{"muted":true}}`
		}
		if err := json.Unmarshal([]byte(raw), config.Get()); err != nil {
			t.Fatalf("setup invalide : %v", err)
		}
	}
	set(muted)
	t.Cleanup(func() { set(false) })
}

// §11.2 « Cues » : muet ⇒ aucune cue n'atteint le moteur (garde en tête de notifySound).
func TestSoundMuted286_Cues_Muted_NoCuePlayed(t *testing.T) {
	config.Get().Sound.CuesDisabled = nil
	app, fake := twa227WireSound(t)
	tw286SetMuted(t, true)

	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	time.Sleep(150 * time.Millisecond)
	if got := fake.Played(); len(got) != 0 {
		t.Fatalf("sound.muted=true : aucune cue ne doit sonner (depart attendu silencieux), got %v", got)
	}
}

// §11.2 « Passage à false ⇒ rien n'est repris » mais les cues suivantes sonnent de nouveau, sans redémarrage.
func TestSoundMuted286_Cues_Unmuted_PlaysAgainImmediately(t *testing.T) {
	config.Get().Sound.CuesDisabled = nil
	app, fake := twa227WireSound(t)
	tw286SetMuted(t, true)
	tw286SetMuted(t, false) // rétablissement : le cleanup du premier appel restaure aussi false

	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueDepart {
		t.Fatalf("muted rétabli à false : depart doit sonner, got %v", got)
	}
}

// §11.2 « Média de question (T1) » : muet traité comme DISABLED à T1 — aucun Play, chrono différé libéré.
func TestSoundMuted286_QuestionMedia_T1_Muted_NoPlay_TimerReleased(t *testing.T) {
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabledFor(app, true) // sound.enabled actif : seule la garde muted doit agir
	tw286SetMuted(t, true)
	app.qsound.Start(q)

	if fake.playCount() != 0 {
		t.Error("sound.muted=true : Play() ne doit JAMAIS être appelé pour le média de question")
	}
	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("sound.muted=true : le chronomètre différé doit être libéré immédiatement (règle de non-blocage §10.7)")
	}
	if got := app.engine.GetState().QuestionSoundState; got == game.QuestionSoundPlaying {
		t.Errorf("QUESTION_SOUND_STATE ne doit pas passer à PLAYING quand le son est coupé, got %q", got)
	}
}

// §11.2 contrôle : non coupé, le même scénario joue bel et bien (le test ci-dessus ne passe pas pour la mauvaise raison).
func TestSoundMuted286_QuestionMedia_T1_NotMuted_Plays(t *testing.T) {
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabledFor(app, true)
	tw286SetMuted(t, false)
	app.qsound.Start(q)

	if fake.playCount() != 1 {
		t.Errorf("non coupé : Play() attendu une fois, got %d", fake.playCount())
	}
}

// §11.2 « La gate T0 ne lit jamais Muted » : une question sonore démarre normalement son coupé —
// QUESTION_SOUND_UNAVAILABLE reste vide. On observe la disponibilité par l'état de jeu diffusé.
func TestSoundMuted286_GateT0_Unchanged_ByMuted(t *testing.T) {
	fake := &tw219qFakeMediaPlayer{}
	app, _ := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()
	tw219qSetSoundEnabledFor(app, true)

	before := app.engine.GetState().QuestionSoundUnavailable
	tw286SetMuted(t, true)
	app.refreshQuestionSoundAvailability()
	after := app.engine.GetState().QuestionSoundUnavailable

	if after != before {
		t.Errorf("sound.muted ne doit pas modifier QUESTION_SOUND_UNAVAILABLE (T0) : %q → %q", before, after)
	}
	if after != "" {
		t.Errorf("une question sonore valide, son coupé, doit rester démarrable (UNAVAILABLE vide), got %q", after)
	}
}

// §11.2 « Effet de bord » : setupCallbacks() câble OnSoundMutedChange(func(bool)) et l'appeler à true
// arrête le média de question de CETTE App (QUESTION_SOUND_STATE → IDLE). Champ cherché par nom
// (réflexion) : le fichier compile avant la livraison du dev.
func TestSoundMuted286_OnSoundMutedChange_Wired_StopsQuestionMedia(t *testing.T) {
	app, _ := twa227WireSound(t)
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, soundEnabled: func() bool { return true }}
	t.Cleanup(func() { tw257CancelWatchdog(app) })

	field := reflect.ValueOf(app.httpServer).Elem().FieldByName("OnSoundMutedChange")
	if !field.IsValid() {
		t.Fatal("HTTPServer.OnSoundMutedChange absent : rappel de contracts/sound.md §11.2 non livré")
	}
	if field.IsNil() {
		t.Fatal("OnSoundMutedChange non câblé par setupCallbacks()")
	}
	cb, ok := field.Interface().(func(bool))
	if !ok {
		t.Fatalf("OnSoundMutedChange doit avoir la signature func(bool), got %s", field.Type())
	}

	q := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1234.wav"}
	app.engine.Ready(q.ID, q)
	app.qsound.Start(q)
	if fake.playCount() != 1 {
		t.Skipf("setup : le média n'a pas démarré (fichier absent du harnais), Play=%d", fake.playCount())
	}

	cb(true)
	if got := app.engine.GetState().QuestionSoundState; got != game.QuestionSoundIdle {
		t.Errorf("après OnSoundMutedChange(true), QUESTION_SOUND_STATE doit être IDLE, got %q", got)
	}
}
