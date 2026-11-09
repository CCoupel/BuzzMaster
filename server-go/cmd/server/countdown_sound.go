// Countdown-sound adapter (v11.4.0, #287/#288, contracts/sound.md §12). The
// THIRD audio path next to the cues engine (sound.go, notifySound) and the
// question media (question_sound.go, questionSound()): a long sound played
// while a response timer runs, on a SECOND audio.MediaPlayer sharing the
// process-wide oto context (§10.3).
//
// Frontier (§12.1, enforced by sound_sites_test.go, not by discipline): this
// file never calls notifySound/PlayCue, and neither it nor question_sound.go
// reference the other's accessor. The only place the three paths meet is
// main.go.
//
// ⚠️ Deadlock rule (§12.5): OnAnswerTimerStarted may fire WITH the engine's
// mutex held (startTimer()). Nothing in this file calls back into
// app.engine — no GetState, no broadcast — only config reads and the
// MediaPlayer (which has its own lock and never blocks).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

type countdownSoundAdapter struct {
	app    *App
	player audio.MediaPlayer

	// soundEnabled reports the master "sound on" switch. nil = production
	// default (config.Get().Sound.Enabled); tests inject their own.
	soundEnabled func() bool
	// soundsDirFn resolves data/files/sounds. nil = derived from the app config.
	soundsDirFn func() string
	// warned dedups the "round file missing" WARN (r3).
	warnMu sync.Mutex
	warned map[string]bool
}

// setupCountdownSound builds the adapter — mirrors setupQuestionSound. The
// MediaPlayer degrades silently (§5.5); it has no natural-end callback (the
// countdown has no deferred timer to release, §12.4).
func (a *App) setupCountdownSound() {
	cs := &countdownSoundAdapter{app: a}
	sc := config.Get().Sound
	cs.player = audio.NewMediaPlayer(audio.OutputConfig{Device: sc.Device}, nil)
	a.csound = cs
}

// countdownSound reads the live adapter — nil before setup() (several tests
// build an *App{} directly). Every method below is safe on a nil receiver, so
// call sites need no guard.
func (a *App) countdownSound() *countdownSoundAdapter {
	return a.csound
}

func (cs *countdownSoundAdapter) enabled() bool {
	if cs.soundEnabled != nil {
		return cs.soundEnabled()
	}
	return config.Get().Sound.Enabled
}

// catalogDir is the sounds directory (data/files/sounds).
func (cs *countdownSoundAdapter) catalogDir() string {
	switch {
	case cs.soundsDirFn != nil:
		return cs.soundsDirFn()
	case cs.app != nil:
		return soundsDir(cs.app.config)
	}
	return ""
}

// filePath is the countdown file RESOLVED for the effective round (r3, #290 §9.4):
// SoundFiles["decompte"] relative to the sounds directory, with a fallback on the
// catalogue file when the round's file is missing (WARN once per path) — the countdown
// never becomes mute because of an orphan reference.
func (cs *countdownSoundAdapter) filePath() string {
	rel := ""
	if cs.app != nil {
		rel = cs.app.ambianceSettings().SoundFiles[ambiancecfg.DecompteKey]
	}
	return cs.filePathFor(rel)
}

// filePathFor resolves rel (relative to the sounds directory; "" or "decompte.wav" = catalogue).
func (cs *countdownSoundAdapter) filePathFor(rel string) string {
	dir := cs.catalogDir()
	catalog := filepath.Join(dir, synth.CountdownFilename())
	if rel == "" || rel == synth.CountdownFilename() {
		return catalog
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return catalog
	}
	p := filepath.Join(dir, clean)
	if _, err := os.Stat(p); err != nil {
		cs.warnMissing(rel)
		return catalog
	}
	return p
}

// warnMissing journalises a missing round file once per path.
func (cs *countdownSoundAdapter) warnMissing(rel string) {
	cs.warnMu.Lock()
	first := !cs.warned[rel]
	if cs.warned == nil {
		cs.warned = map[string]bool{}
	}
	cs.warned[rel] = true
	cs.warnMu.Unlock()
	if first {
		server.LogWarn(game.LogComponentApp, "Countdown sound: round file %q missing — falling back to the catalogue file (ambiance-settings.md §9.4)", rel)
	}
}

// mayPlay is §12.4, in the contract's order. Pure config + start data.
func (cs *countdownSoundAdapter) mayPlay(start game.AnswerTimerStart) bool {
	amb := cs.app.ambianceSettings() // #290: resolved values (global ← round ← direct)
	if amb.SoundMuted {
		return false // §11 — son coupé
	}
	if !cs.enabled() {
		return false // « Bruitages activés » éteint
	}
	if !amb.CountdownEnabled {
		return false // §12.3
	}
	if start.QuestionSound != "" {
		return false // §12.6 — jamais si la question a un média audio
	}
	return true
}

// OnAnswerTimerStarted is wired to Engine.OnAnswerTimerStarted. Runs under
// the engine lock for the GLOBAL kind: never calls back into the engine.
func (cs *countdownSoundAdapter) OnAnswerTimerStarted(start game.AnswerTimerStart) {
	if cs == nil || cs.player == nil || !cs.mayPlay(start) {
		return
	}
	path := cs.filePath()
	var err error
	if !cs.app.ambianceSettings().CountdownLoop { // #290: resolved "une fois" ⇔ !loop
		err = cs.player.Play(path)
	} else {
		err = cs.player.PlayLoop(path)
	}
	if err != nil {
		if catalog := cs.filePathFor(""); path != catalog { // r3: a non-conforming round file ⇒ catalogue file
			if !cs.app.ambianceSettings().CountdownLoop {
				err = cs.player.Play(catalog)
			} else {
				err = cs.player.PlayLoop(catalog)
			}
		}
	}
	if err != nil {
		// Silent degradation (§12.4): no sound, no game effect, no timer effect.
		server.LogWarn(game.LogComponentApp, "Countdown sound: play failed — degrading to no-op (contract §12.4/§5.5): %v", err)
	}
}

// Stop, Pause and Resume are idempotent and never touch the game or the
// question-media player (§12.8).
func (cs *countdownSoundAdapter) Stop() {
	if cs == nil || cs.player == nil {
		return
	}
	cs.player.Stop()
}

func (cs *countdownSoundAdapter) Pause() {
	if cs == nil || cs.player == nil {
		return
	}
	cs.player.Pause()
}

func (cs *countdownSoundAdapter) Resume() {
	if cs == nil || cs.player == nil {
		return
	}
	cs.player.Resume()
}

// Test plays the countdown file ONCE, replacing a running countdown (single
// voice), ignoring muted / countdown_disabled (explicit gesture, §12.4).
// Never touches the game.
func (cs *countdownSoundAdapter) Test() (accepted bool) {
	if cs == nil || cs.player == nil {
		return false
	}
	return cs.player.Play(cs.filePath()) == nil
}

// TestAt plays the file rel (relative to the sounds directory, with the catalogue fallback)
// ONCE — the round dialog's "Tester" (r3, C8). Same explicit-gesture rules as Test.
func (cs *countdownSoundAdapter) TestAt(rel string) (accepted bool) {
	if cs == nil || cs.player == nil {
		return false
	}
	return cs.player.Play(cs.filePathFor(rel)) == nil
}

// OutputAvailable reports whether a REAL (non-neutral) MediaPlayer backs it.
func (cs *countdownSoundAdapter) OutputAvailable() bool {
	return cs != nil && !audio.IsNeutralMedia(cs.player)
}

// server.CountdownProvider (internal/server/http_sound.go).

// CountdownOutputAvailable implements server.CountdownProvider.
func (a *App) CountdownOutputAvailable() bool { return a.countdownSound().OutputAvailable() }

// TestCountdown implements server.CountdownProvider.
func (a *App) TestCountdown() (accepted bool) { return a.countdownSound().Test() }

// StopCountdown stops the countdown voice before decompte.wav is replaced or
// restored (Windows cannot rename over a file still held open) — used by
// the HTTP layer through an optional interface (http_sound.go).
func (a *App) StopCountdown() { a.countdownSound().Stop() }
