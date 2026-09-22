// Instrumentation ciblée pour le retour QUALIF v11.1.0.4 (#219) : la
// question mono silencieuse avait SOUND_TIMER_DELAYED=true. Le fichier lui-
// même a été vérifié structurellement parfait (indépendamment, par le CDP)
// et le chemin de stockage/lecture a été vérifié octet pour octet
// (internal/audio/mono_playback_219_test.go). Reste à prouver, avec du code
// RÉELLEMENT EXÉCUTÉ (pas seulement une relecture statique), que
// questionSoundAdapter.Start() appelle bien MediaPlayer.Play() au même
// endroit, avec les mêmes arguments, QUELLE QUE SOIT la valeur de
// SoundTimerDelayed — le couplage accidentel que le CDP soupçonne entre le
// gating du chronomètre différé et le déclenchement réel du son.
//
// audio.NewMediaPlayer ne peut pas être doublé (aucun backend audio réel
// dans ce sandbox — output_228_test.go/media_219_test.go l'ont déjà
// confirmé), donc ce fichier construit un questionSoundAdapter DIRECTEMENT
// (champ non exporté, même paquet) avec un MediaPlayer factice qui
// enregistre ses appels — la seule façon d'exécuter réellement play() sans
// matériel.
//
// Convention de collision : préfixe tw219d pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"fmt"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// tw219dFakeMediaPlayer implémente audio.MediaPlayer et enregistre chaque
// appel à Play() — jamais un double d'internal/audio (test-writer/dev-
// backend l'ont déjà établi comme non-doublable, output_228_test.go), mais
// un faux **injecté directement dans le champ non exporté**
// questionSoundAdapter.player, possible ici parce que ce fichier est dans
// le même paquet (main).
type tw219dFakeMediaPlayer struct {
	playPaths []string
	state     audio.MediaState
}

func (f *tw219dFakeMediaPlayer) Play(path string) error {
	f.playPaths = append(f.playPaths, path)
	f.state = audio.MediaPlaying
	return nil
}
func (f *tw219dFakeMediaPlayer) Pause()  { f.state = audio.MediaPaused }
func (f *tw219dFakeMediaPlayer) Resume() { f.state = audio.MediaPlaying }
func (f *tw219dFakeMediaPlayer) Stop()   { f.state = audio.MediaIdle }
func (f *tw219dFakeMediaPlayer) State() audio.MediaState {
	return f.state
}
func (f *tw219dFakeMediaPlayer) Close() error { return nil }

// TestQuestionSoundStart_CallsPlay_RegardlessOfSoundTimerDelayed est la
// réponse directe, exécutée, à la question du CDP : Play() est-il appelé
// indépendamment de SOUND_TIMER_DELAYED ? Reproduit exactement la question
// #1 du QUALIF (SPEEDY, SOUND="/question/1/sound_9897.wav",
// SOUND_TIMER_DELAYED=true, TIME="30") ET son pendant en mode simultané,
// dans la même table.
func TestQuestionSoundStart_CallsPlay_RegardlessOfSoundTimerDelayed(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(fmt.Sprintf("SoundTimerDelayed=%v", delayed), func(t *testing.T) {
			app := newTestApp(t)
			app.config.Sound.Enabled = true
			t.Cleanup(func() { app.config.Sound.Enabled = false })

			fake := &tw219dFakeMediaPlayer{}
			qs := &questionSoundAdapter{app: app, player: fake}
			app.qsound = qs

			q := &game.Question{
				ID:                "1",
				Sound:             "/question/1/sound_9897.wav",
				SoundTimerDelayed: delayed,
				Time:              "30",
				Type:              game.QuestionTypeSpeedy,
			}

			qs.Start(q)

			if len(fake.playPaths) != 1 {
				t.Fatalf("Play() appelé %d fois (SoundTimerDelayed=%v), attendu EXACTEMENT 1 — le déclenchement du son ne doit JAMAIS dépendre du chronomètre différé (contract §10.7)",
					len(fake.playPaths), delayed)
			}
			wantPath := app.questionSoundPath(q.Sound)
			if fake.playPaths[0] != wantPath {
				t.Fatalf("Play() appelé avec le chemin %q, attendu %q", fake.playPaths[0], wantPath)
			}
			if fake.State() != audio.MediaPlaying {
				t.Fatalf("MediaPlayer.State() = %q après Start(), attendu MediaPlaying", fake.State())
			}
		})
	}
}

// TestOnPhaseStarted_CallsQuestionSoundStart exerce le VRAI point d'entrée
// (onPhaseStarted, cmd/server/main.go) plutôt que d'appeler
// questionSoundAdapter.Start directement — preuve que le câblage réel
// (actualStart -> OnStateChange(PhaseStarted) -> onPhaseStarted) atteint
// bien Start(), y compris pour une question en mode différé.
func TestOnPhaseStarted_CallsQuestionSoundStart(t *testing.T) {
	app := newTestAppWithHub(t)
	app.config.Sound.Enabled = true
	t.Cleanup(func() { app.config.Sound.Enabled = false })
	app.logger = server.NewBroadcastLogger(100)

	fake := &tw219dFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake}

	// onPhaseStarted is reached via OnStateChange(PhaseStarted) — the
	// "isRealStart" front-detection (soundTrackPhase) records the PREVIOUS
	// phase on EVERY OnStateChange call, so this wiring must be in place
	// BEFORE Ready() fires its own OnStateChange(PhasePrepare) below —
	// exactly like production's setupCallbacks(), wired once at startup,
	// before the first Ready() the server ever processes. Wiring it AFTER
	// Ready() (a first version of this test did) leaves a.soundPhase.last at
	// its zero value ("") when StartImmediate fires PhaseStarted, so
	// isRealStart incorrectly evaluates false — a test-setup bug, not a
	// production one, since production wires this once at process startup.
	app.engine.OnStateChange = func(phase game.GamePhase) {
		previousPhase := app.soundTrackPhase(phase)
		if phase == game.PhaseStarted {
			app.onPhaseStarted(previousPhase)
		}
	}

	q := &game.Question{
		ID:                "1",
		Sound:             "/question/1/sound_9897.wav",
		SoundTimerDelayed: true,
		Time:              "30",
		Type:              game.QuestionTypeSpeedy,
	}
	app.engine.Ready("1", q)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	app.engine.StartImmediate(30)

	if len(fake.playPaths) != 1 {
		t.Fatalf("onPhaseStarted n'a pas déclenché Play() exactement une fois (SOUND_TIMER_DELAYED=true) : got %d appel(s)", len(fake.playPaths))
	}
}
