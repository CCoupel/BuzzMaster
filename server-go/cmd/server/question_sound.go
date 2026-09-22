// Question-sound adapter (v11.1, #219, contracts/sound.md §10). This file
// is the game ↔ internal/audio.MediaPlayer bridge — symmetric to sound.go's
// own role for the cues bruitage engine (§10.5: the two audio paths never
// call into each other):
//
//   - it owns the single, process-lifetime audio.MediaPlayer instance;
//   - it enforces the non-blocking rule (§10.7, CA12): a sound that never
//     actually starts playing (neutral player, disabled, unreadable file)
//     releases the engine's deferred answer timer IMMEDIATELY, never
//     leaving a question frozen;
//   - it enforces the watchdog (§10.7's "chien de garde"): the deferred
//     timer is released at the latest MaxQuestionSoundDuration+2s after a
//     Play() call, even if no end-of-playback signal — natural or manual —
//     ever arrives;
//   - it translates the four conduite gestures (QUESTION_SOUND's PLAY/
//     PAUSE/RESUME/STOP) into MediaPlayer calls and keeps
//     GameState.QuestionSoundState in sync, broadcasting on every change.
//
// Never calls notifySound or PlayCue (cmd/server/sound.go) — a media is not
// a cue (§10.5) — covered by cmd/server/question_sound_sites_test.go
// (test-writer).
package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// questionSoundWatchdogGrace is added to audio.MaxQuestionSoundDuration for
// the watchdog timer (contract §10.7's "chien de garde", plan §7 tâche 10)
// — belt-and-braces against a monitoring goroutine that fails to signal.
const questionSoundWatchdogGrace = 2 * time.Second

// questionSoundAdapter owns the MediaPlayer and the bookkeeping needed to
// enforce §10.7's non-blocking rule and idempotent release. Constructed
// once in setup() (setupQuestionSound), never rebuilt — same "built once,
// lives for the process" discipline as a.sound()'s own audio.Engine,
// though a plain field suffices here (never swapped, so no atomic.Pointer
// is needed — see App.qsound's own doc comment).
type questionSoundAdapter struct {
	app    *App
	player audio.MediaPlayer

	mu             sync.Mutex
	watchdogCancel context.CancelFunc // cancels the CURRENT watchdog goroutine, if any
}

// setupQuestionSound builds the question-sound adapter — mirrors
// setupSound()'s exact role for the cues engine. The MediaPlayer degrades
// silently (contract §5.5/§10.2) on any platform/hardware failure; that
// degradation is read back later via audio.IsNeutralMedia, never here.
func (a *App) setupQuestionSound() {
	qs := &questionSoundAdapter{app: a}
	sc := config.Get().Sound
	qs.player = audio.NewMediaPlayer(audio.OutputConfig{Device: sc.Device}, qs.onNaturalEnd)
	a.qsound = qs
}

// questionSound reads the live question-sound adapter — nil before
// setup() runs (several cmd/server tests construct an *App{} directly,
// without calling setup()). Every questionSoundAdapter method above is
// documented safe on a nil receiver — same discipline as a.sound()'s
// audio.Engine and a.ambiance()'s lighting.Writer — so every call site in
// this package calls it unconditionally, with no nil guard of its own.
func (a *App) questionSound() *questionSoundAdapter {
	return a.qsound
}

// questionSoundPath resolves a Question.Sound URL
// ("/question/<id>/sound_<rand4>.wav", contracts/models.md) to its on-disk
// path — same config field and default fallback as loadQuestion (main.go),
// the other cmd/server reader of this same directory.
func (a *App) questionSoundPath(soundURL string) string {
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}
	rel := filepath.FromSlash(soundURL) // "/question/<id>/<file>"
	return filepath.Join(questionsDir, filepath.Base(filepath.Dir(rel)), filepath.Base(rel))
}

// Start begins playback of q's attached sound, if any (contract §10.2) —
// called from onPhaseStarted on every REAL start (isRealStart), for ANY
// question type: the field is structurally common to every type (§0.3/
// §10.4bis), the server never gates by type. No-op — no goroutine, no log
// — when q is nil or carries no sound (CA9).
//
// The deferred-timer WIRING (§10.7) already happened inside the engine
// (maybeStartDeferredTimerUnsafe, called from actualStart() under its own
// lock, BEFORE this method ever runs — this runs from the OnStateChange
// callback, fired AFTER that lock is released). This method's own job is
// only the non-blocking rule (CA12): release the timer immediately if the
// sound doesn't actually end up playing.
func (qs *questionSoundAdapter) Start(q *game.Question) {
	if qs == nil || q == nil || q.Sound == "" {
		return
	}
	qs.play(q, true)
}

// HandleCommand processes an admin/anim QUESTION_SOUND command (contract
// websocket-actions.md §"QUESTION_SOUND"). No-op if the current question
// carries no sound. PLAY here is the MANUAL "Rejouer" gesture: restarts
// the current question's sound from the beginning but — unlike Start
// above — NEVER releases the deferred timer, on success or failure alike
// (CA13: a Rejouer must never re-arm or otherwise touch a timer that has
// already started, or one that's still legitimately waiting).
func (qs *questionSoundAdapter) HandleCommand(command protocol.QuestionSoundCommand) {
	if qs == nil {
		return
	}
	state := qs.app.engine.GetState()
	q := state.Question
	switch command {
	case protocol.QuestionSoundCommandPlay:
		if q == nil || q.Sound == "" {
			return
		}
		qs.play(q, false)
	case protocol.QuestionSoundCommandPause:
		qs.Pause()
	case protocol.QuestionSoundCommandResume:
		qs.Resume()
	case protocol.QuestionSoundCommandStop:
		qs.Stop()
	}
}

// play is the shared core of Start (automatic, on phase start) and the
// manual Rejouer gesture (HandleCommand's PLAY) — releaseOnFailure
// controls §10.7's non-blocking rule: true for the automatic path (a fresh
// question must never freeze), false for Rejouer (CA13: never touches the
// timer, success or failure).
func (qs *questionSoundAdapter) play(q *game.Question, releaseOnFailure bool) {
	qs.mu.Lock()
	qs.cancelWatchdogLocked()
	qs.mu.Unlock()

	// CA12, non-blocking rule — checked BEFORE ever touching
	// QuestionSoundState or the player, so a degraded case never flickers
	// PLAYING before immediately reverting to IDLE:
	//   - son désactivé (config)
	//   - enceinte indisponible (neutral MediaPlayer)
	if !config.Get().Sound.Enabled || audio.IsNeutralMedia(qs.player) {
		if releaseOnFailure {
			qs.app.engine.ReleaseDeferredTimer()
		}
		return
	}

	path := qs.app.questionSoundPath(q.Sound)

	qs.app.engine.SetQuestionSoundState(game.QuestionSoundPlaying)
	qs.app.broadcastUpdate()

	if err := qs.player.Play(path); err != nil {
		// Third non-blocking case: fichier illisible/non conforme.
		server.LogWarn(game.LogComponentApp, "Question sound: Play(%s) failed — degrading to no-op (contract §5.5/§10.2): %v", path, err)
		qs.app.engine.SetQuestionSoundState(game.QuestionSoundIdle)
		qs.app.broadcastUpdate()
		if releaseOnFailure {
			qs.app.engine.ReleaseDeferredTimer()
		}
		return
	}

	qs.armWatchdog(q.ID)
}

// Pause implements QUESTION_SOUND's PAUSE gesture, and is also called from
// ActionPause's handler (main.go, contract §10.7: "PAUSE du jeu met le son
// en pause") — safe to call unconditionally, same discipline as
// notifySound: a no-op on a player that isn't currently playing (neutral,
// idle, or already paused).
func (qs *questionSoundAdapter) Pause() {
	if qs == nil {
		return
	}
	qs.player.Pause()
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundPaused)
	qs.app.broadcastUpdate()
}

// Resume implements QUESTION_SOUND's RESUME gesture, and is also called
// from ActionContinue's handler (contract §10.7: "CONTINUER le reprend
// sans coupure") — same unconditional-safety discipline as Pause above.
func (qs *questionSoundAdapter) Resume() {
	if qs == nil {
		return
	}
	qs.player.Resume()
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundPlaying)
	qs.app.broadcastUpdate()
}

// Stop implements QUESTION_SOUND's STOP gesture, and is also called
// wherever a question's active life ends (ActionStop, OnTimeUp, Ready,
// Reveal — plan §7 tâche 11): releases the deferred timer explicitly
// (§10.7's "arrêt manuel" branch — idempotent, harmless if the engine
// already reset it itself, e.g. via stopUnsafe's own
// resetDeferredTimerUnsafe). Safe to call unconditionally, even with no
// sound attached or none currently playing.
func (qs *questionSoundAdapter) Stop() {
	if qs == nil {
		return
	}
	qs.mu.Lock()
	qs.cancelWatchdogLocked()
	qs.mu.Unlock()
	qs.player.Stop()
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundIdle)
	qs.app.broadcastUpdate()
	qs.app.engine.ReleaseDeferredTimer()
}

// onNaturalEnd is the MediaPlayer's natural-end callback (registered ONCE
// at construction — audio.NewMediaPlayer's own doc comment). Fires ONLY
// when a Play() call's audio reaches its end ON ITS OWN, never on an
// explicit Stop/Pause/replace — already guaranteed by internal/audio's own
// generation-numbered monitoring goroutine (media_oto.go). Cancels the
// watchdog (no longer needed — a legitimate end already happened) and
// releases the deferred timer (§10.7's "fin naturelle" branch).
func (qs *questionSoundAdapter) onNaturalEnd() {
	qs.mu.Lock()
	qs.cancelWatchdogLocked()
	qs.mu.Unlock()
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundIdle)
	qs.app.broadcastUpdate()
	qs.app.engine.ReleaseDeferredTimer()
}

// armWatchdog starts a goroutine that releases the deferred timer at the
// latest audio.MaxQuestionSoundDuration+questionSoundWatchdogGrace after
// this Play() call, in case no end-of-playback signal ever arrives (a dead
// monitoring goroutine, a wedged driver — contract §10.7's "chien de
// garde"). Cancelled (never fires) by any subsequent play/Stop/
// onNaturalEnd. Caller must NOT hold qs.mu.
func (qs *questionSoundAdapter) armWatchdog(questionID string) {
	ctx, cancel := context.WithCancel(context.Background())
	qs.mu.Lock()
	qs.watchdogCancel = cancel
	qs.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				server.LogRecoveredPanic(game.LogComponentApp, "question sound watchdog", r)
			}
		}()
		select {
		case <-time.After(audio.MaxQuestionSoundDuration + questionSoundWatchdogGrace):
			server.LogWarn(game.LogComponentApp, "Question sound: watchdog fired for question %s — releasing the deferred timer (contract §10.7)", questionID)
			qs.app.engine.ReleaseDeferredTimer()
		case <-ctx.Done():
		}
	}()
}

// cancelWatchdogLocked cancels the current watchdog goroutine, if any —
// caller must hold qs.mu.
func (qs *questionSoundAdapter) cancelWatchdogLocked() {
	if qs.watchdogCancel != nil {
		qs.watchdogCancel()
		qs.watchdogCancel = nil
	}
}

// handleQuestionSound dispatches an inbound QUESTION_SOUND message (admin/
// anim only — internal/server/inbound_allowlist.go) to the adapter —
// contracts/websocket-actions.md §"QUESTION_SOUND". Same style as the
// sibling handleRafaleSetTeams/handleMotionReveal handlers in main.go.
func (a *App) handleQuestionSound(msg *protocol.Message) {
	var payload protocol.QuestionSoundPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse QUESTION_SOUND: %v", err)
		return
	}
	a.questionSound().HandleCommand(payload.Command)
}
