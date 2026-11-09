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
	"os"
	"path/filepath"
	"strings"
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

// soundSource décrit le média à jouer (v11.6.0, #235) : la question (hôte,
// tous types), une carte MEMOTION, ou le son d'un ENTRACTE. owner suit le
// vocabulaire de GameState.QuestionSoundOwner : "HOST" | "CARD:<id>" | "ENTRACTE".
type soundSource struct {
	url        string
	delayed    bool
	loop       bool
	owner      string
	questionID string
}

// hostSource : source de la question elle-même. Le son d'une question de type
// ENTRACTE a pour propriétaire "ENTRACTE". Le différé n'a pas de sens pour
// MEMOTION (hôte) ni ENTRACTE (pas de chrono global) : ignoré.
func hostSource(q *game.Question) soundSource {
	src := soundSource{url: q.Sound, delayed: q.SoundTimerDelayed, loop: q.SoundLoop, owner: "HOST", questionID: q.ID}
	switch q.Type {
	case game.QuestionTypeEntracte:
		src.owner = "ENTRACTE"
		src.delayed = false
	case game.QuestionTypeMemotion:
		src.delayed = false
	}
	return src
}

// cardSource : source d'une carte MEMOTION.
func cardSource(q *game.Question, c *game.MotionCard) soundSource {
	return soundSource{url: c.Sound, delayed: c.SoundTimerDelayed, loop: c.SoundLoop, owner: "CARD:" + c.ID, questionID: q.ID}
}

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

	// cur : source du dernier média lancé (v11.6.0, #235) — sert à la boucle
	// (onNaturalEnd) et au geste « Rejouer ». pendingCardTimer : démarrage
	// différé du chrono de la carte (SOUND_TIMER_DELAYED d'une carte), libéré
	// avec le chrono global par release().
	cur              *soundSource
	pendingCardTimer func() (consumed bool)
	heldCardTimer    func() (consumed bool) // libéré pendant une PAUSE du jeu : démarre à Resume

	// soundEnabled reports the master "sound on" switch. nil = production default
	// (config.Get().Sound.Enabled); tests inject their own so they never touch the
	// process-wide config singleton (#257/#233).
	soundEnabled func() bool

	// watchdogDelay overrides the watchdog period (0 = production default,
	// audio.MaxQuestionSoundDuration + questionSoundWatchdogGrace). Per-adapter,
	// not package-level: tests run in parallel and a shortened global would
	// fire the watchdog of unrelated tests (#291, Phase A).
	watchdogDelay time.Duration

	// validationMu/validationCache back questionSoundAvailability's file
	// cache (contract sound.md §10.8.3) — a SEPARATE mutex from mu above:
	// this cache is read/written far more often (every PONG, §10.8.7) and
	// has nothing to do with the watchdog/playback state mu guards, so
	// sharing one lock would only add unrelated contention.
	validationMu    sync.Mutex
	validationCache map[string]soundValidationEntry
}

// soundValidationEntry is one entry of the media-file validation cache —
// shape imposed by contract sound.md §10.8.3, normative: keyed by the
// sound file's ABSOLUTE path, fingerprinted by mtime+size (a cheap
// os.Stat), NEGATIVE verdicts cached exactly like positive ones (a
// corrupt file left uncached would be re-parsed on every single PONG —
// precisely the case where the cost is highest). reason is "" iff ok.
type soundValidationEntry struct {
	modTime time.Time
	size    int64
	ok      bool
	reason  string
}

// enabled reports the master sound switch (injected in tests, config otherwise).
func (qs *questionSoundAdapter) enabled() bool {
	if qs.soundEnabled != nil {
		return qs.soundEnabled()
	}
	return config.Get().Sound.Enabled
}

// setupQuestionSound builds the question-sound adapter — mirrors
// setupSound()'s exact role for the cues engine. The MediaPlayer degrades
// silently (contract §5.5/§10.2) on any platform/hardware failure; that
// degradation is read back later via audio.IsNeutralMedia, never here.
func (a *App) setupQuestionSound() {
	qs := &questionSoundAdapter{app: a, validationCache: make(map[string]soundValidationEntry)}
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
	qs.playSource(hostSource(q), true)
}

// StartCard joue le son d'une carte MEMOTION au retournement (v11.6.0, #235,
// sound.md §14.4) et démarre le chrono de la carte via startTimer : tout de
// suite, ou — si la carte a SOUND_TIMER_DELAYED — à la fin du premier passage
// (ou au Stop animateur). Une carte sans son laisse l'hôte continuer (D9) et
// démarre simplement son chrono. Un média indisponible n'empêche jamais le
// retournement (§14.4 gate T0) : pas de son, chrono libéré.
func (qs *questionSoundAdapter) StartCard(q *game.Question, card *game.MotionCard, startTimer func()) {
	if startTimer == nil {
		startTimer = func() {}
	}
	if qs == nil || q == nil || card == nil || card.Sound == "" {
		startTimer()
		return
	}
	// Gate T0 au retournement (§14.4) : média indisponible ⇒ la carte s'ouvre quand
	// même, sans son, chrono immédiat ; le motif remonte comme un état (§10.9).
	if ok, _ := qs.sourceAvailability(card.Sound); !ok {
		startTimer()
		qs.app.refreshQuestionSoundAvailability()
		qs.app.broadcastUpdate()
		return
	}
	qs.app.refreshQuestionSoundAvailability() // efface un motif de carte précédente
	if card.SoundTimerDelayed {
		cardID := card.ID
		qs.mu.Lock()
		qs.heldCardTimer = nil
		qs.pendingCardTimer = func() bool {
			// Ne démarre que si la carte est toujours ouverte (face question).
			st := qs.app.engine.GetState()
			if st.Phase == game.PhasePaused {
				return false // libéré pendant la pause : démarre à la reprise (Resume)
			}
			if st.Phase == game.PhaseStarted && st.MotionSelected == cardID && st.MotionCardStates[cardID] == game.MotionCardStateQuestion {
				startTimer()
			}
			return true
		}
		qs.mu.Unlock()
	} else {
		startTimer()
	}
	qs.playSource(cardSource(q, card), true)
}

// StopCard arrête le son d'une carte (fermeture MEMOTION_DONE, révélation) :
// l'hôte ne reprend PAS (D9) et un chrono de carte encore différé est abandonné.
// Sans effet si le média courant n'appartient pas à une carte.
func (qs *questionSoundAdapter) StopCard() {
	if qs == nil {
		return
	}
	qs.mu.Lock()
	qs.pendingCardTimer = nil
	qs.heldCardTimer = nil
	isCard := qs.cur != nil && strings.HasPrefix(qs.cur.owner, "CARD:")
	qs.mu.Unlock()
	if isCard {
		qs.Stop()
	}
	qs.app.refreshQuestionSoundAvailability() // le motif d'une carte fermée disparaît
}

// StartEntracte joue le son d'un entracte activé manuellement (ENTRACTE_SET
// ACTIVE:true) : seulement si la question chargée est de type ENTRACTE et porte
// un son (v11.6.0, #235, §14.3). L'entracte programmé passe par Start().
func (qs *questionSoundAdapter) StartEntracte(q *game.Question) {
	if qs == nil || q == nil || q.Type != game.QuestionTypeEntracte || q.Sound == "" {
		return
	}
	qs.playSource(hostSource(q), false)
}

// StopEntracte arrête le son d'entracte à la sortie de l'entracte.
func (qs *questionSoundAdapter) StopEntracte() {
	if qs == nil {
		return
	}
	qs.mu.Lock()
	isEntracte := qs.cur != nil && qs.cur.owner == "ENTRACTE"
	qs.mu.Unlock()
	if isEntracte {
		qs.Stop()
	}
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
		// « Rejouer » : le média courant s'il est encore valide (carte ouverte,
		// entracte), sinon le son de la question.
		if src := qs.replaySource(state); src != nil {
			qs.playSource(*src, false)
			return
		}
		if q == nil || q.Sound == "" {
			return
		}
		qs.playSource(hostSource(q), false)
	case protocol.QuestionSoundCommandPause:
		qs.Pause()
	case protocol.QuestionSoundCommandResume:
		qs.Resume()
	case protocol.QuestionSoundCommandStop:
		qs.Stop()
	}
}

// questionSoundAvailability is the SINGLE definition of "média disponible"
// (contract sound.md §10.8.2), reused by BOTH the T0 gate — pushed into
// GameState via Engine.SetQuestionSoundUnavailable, refreshed by the
// application layer at the four moments of §10.8.7 — and the T1 non-
// blocking degradation (play(), below, which used to inline only the
// first two conditions). Two parallel definitions would drift and produce
// the worst possible symptom: a question blocked when the sound would
// have worked, or the reverse (CA21, R19).
//
// Returns (true, "") when available, or a question that carries no sound
// at all (nothing to check — same "immediate exit" as
// participantsConform's own branch, contract §10.8.4). Otherwise
// (false, reason), reason being one of DISABLED/OUTPUT/FILE.
//
// Condition 3 (the file) is a FULL revalidation — never a bare os.Stat —
// reusing audio.ValidateQuestionSound (itself built on
// extractCanonicalPCM), never a second parser (§10.8.2's arbitrage). Its
// cost is made sustainable by validateSoundFile's cache (§10.8.3): calling
// this with an unchanged file is a cheap os.Stat, not a re-parse.
func (qs *questionSoundAdapter) questionSoundAvailability(q *game.Question) (ok bool, reason string) {
	if qs == nil || q == nil {
		return true, ""
	}
	return qs.sourceAvailability(q.Sound)
}

// sourceAvailability : la définition unique de « média disponible » (§10.8.2)
// appliquée à une URL de son — question, carte ou entracte (#235).
func (qs *questionSoundAdapter) sourceAvailability(soundURL string) (ok bool, reason string) {
	if qs == nil || soundURL == "" {
		return true, ""
	}
	if !qs.enabled() {
		return false, "DISABLED"
	}
	if audio.IsNeutralMedia(qs.player) {
		return false, "OUTPUT"
	}
	if reason := qs.validateSoundFile(qs.app.questionSoundPath(soundURL)); reason != "" {
		return false, reason
	}
	return true, ""
}

// validateSoundFile implements the mandatory cache of contract §10.8.3 —
// questionSoundAvailability's third condition. Keyed by absolute path,
// fingerprinted by mtime+size via a cheap os.Stat:
//   - a HIT (fingerprint unchanged) reuses the stored verdict — POSITIVE
//     or NEGATIVE — without ever reading the file's content again;
//   - a MISS (no entry, or a different fingerprint) revalidates fully via
//     audio.ValidateQuestionSound, then memorizes the verdict;
//   - os.Stat failing (file gone) reports "FILE" AND deletes the entry,
//     so a file later restored is revalidated rather than trusting a
//     stale verdict (§10.8.3's own table).
//
// No eviction: the cache is bounded by the number of questions that carry
// a sound (§10.8.3's own "aucune éviction n'est nécessaire").
func (qs *questionSoundAdapter) validateSoundFile(path string) (reason string) {
	info, err := os.Stat(path)
	if err != nil {
		qs.validationMu.Lock()
		delete(qs.validationCache, path)
		qs.validationMu.Unlock()
		return "FILE"
	}

	qs.validationMu.Lock()
	if entry, hit := qs.validationCache[path]; hit && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		qs.validationMu.Unlock()
		return entry.reason
	}
	qs.validationMu.Unlock()

	entry := soundValidationEntry{modTime: info.ModTime(), size: info.Size()}
	if raw, rerr := os.ReadFile(path); rerr != nil {
		entry.reason = "FILE"
	} else if _, verr := audio.ValidateQuestionSound(raw); verr != nil {
		entry.reason = "FILE"
	} else {
		entry.ok = true
	}

	qs.validationMu.Lock()
	if qs.validationCache == nil {
		// Defensive: several tests construct questionSoundAdapter{} directly
		// (bypassing setupQuestionSound's initialization) — never nil in
		// production, but a nil map must never panic on write here.
		qs.validationCache = make(map[string]soundValidationEntry)
	}
	qs.validationCache[path] = entry
	qs.validationMu.Unlock()

	return entry.reason
}

// play is the shared core of Start (automatic, on phase start) and the
// manual Rejouer gesture (HandleCommand's PLAY) — releaseOnFailure
// controls §10.7's non-blocking rule: true for the automatic path (a fresh
// question must never freeze), false for Rejouer (CA13: never touches the
// timer, success or failure).
func (qs *questionSoundAdapter) play(q *game.Question, releaseOnFailure bool) {
	qs.playSource(hostSource(q), releaseOnFailure)
}

// replaySource renvoie la source à rejouer pour « Rejouer », ou nil si le dernier
// média n'est plus pertinent (carte fermée, autre question, entracte terminé).
func (qs *questionSoundAdapter) replaySource(state game.GameState) *soundSource {
	qs.mu.Lock()
	cur := qs.cur
	qs.mu.Unlock()
	if cur == nil || state.Question == nil || state.Question.ID != cur.questionID {
		return nil
	}
	switch {
	case strings.HasPrefix(cur.owner, "CARD:"):
		id := strings.TrimPrefix(cur.owner, "CARD:")
		if state.MotionSelected == id && state.MotionCardStates[id] == game.MotionCardStateQuestion {
			for i := range state.Question.MotionCards {
				if c := &state.Question.MotionCards[i]; c.ID == id && c.Sound != "" {
					src := cardSource(state.Question, c)
					return &src
				}
			}
		}
		return nil
	case cur.owner == "ENTRACTE":
		if state.Entracte {
			src := *cur
			return &src
		}
		return nil
	}
	return nil // HOST : le chemin habituel (hostSource) s'en charge
}

// release libère les chronomètres différés : le chrono global du moteur
// (idempotent) et, le cas échéant, le chrono de la carte en attente (#235).
func (qs *questionSoundAdapter) release() {
	qs.mu.Lock()
	pending := qs.pendingCardTimer
	qs.pendingCardTimer = nil
	qs.mu.Unlock()
	qs.app.engine.ReleaseDeferredTimer()
	qs.runPending(pending)
}

// runPending exécute le démarrage différé d'un chrono de carte ; s'il n'est pas
// encore possible (jeu en pause) il est remis en attente jusqu'à Resume.
func (qs *questionSoundAdapter) runPending(pending func() bool) {
	if pending == nil {
		return
	}
	if !pending() {
		qs.mu.Lock()
		qs.heldCardTimer = pending
		qs.mu.Unlock()
	}
}

// setIdle : le média courant est terminé ou arrêté — état IDLE, plus de propriétaire.
func (qs *questionSoundAdapter) setIdle() {
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundIdle)
	qs.app.engine.SetQuestionSoundOwner("")
}

// playSource est le cœur commun de Start, du geste « Rejouer », du son de carte
// et du son d'entracte (#235) — releaseOnFailure comme play ci-dessus.
func (qs *questionSoundAdapter) playSource(src soundSource, releaseOnFailure bool) {
	qs.mu.Lock()
	qs.cancelWatchdogLocked()
	qs.mu.Unlock()

	// CA12/CA21, non-blocking rule — see the pre-#235 comment history: DISABLED/OUTPUT
	// pre-empt; FILE deliberately does not (the Play() attempt surfaces it).
	if ok, reason := qs.sourceAvailability(src.url); !ok && reason != "FILE" {
		if releaseOnFailure {
			qs.release()
		}
		return
	}

	// #286 (contract sound.md §11.2): muted is treated as DISABLED at T1 only.
	if config.Get().Sound.Muted {
		if releaseOnFailure {
			qs.release()
		}
		return
	}

	path := qs.app.questionSoundPath(src.url)

	qs.mu.Lock()
	cur := src
	qs.cur = &cur
	qs.mu.Unlock()

	qs.app.engine.SetQuestionSoundOwner(src.owner)
	qs.app.engine.SetQuestionSoundState(game.QuestionSoundPlaying)
	qs.app.broadcastUpdate()

	// #288 (contract sound.md §13.3): SOUND_LOOP ⇒ PlayLoop.
	var err error
	if src.loop {
		err = qs.player.PlayLoop(path)
	} else {
		err = qs.player.Play(path)
	}
	if err != nil {
		server.LogWarn(game.LogComponentApp, "Question sound: Play(%s) failed — degrading to no-op (contract §5.5/§10.2): %v", path, err)
		qs.setIdle()
		qs.app.broadcastUpdate()
		if releaseOnFailure {
			qs.release()
		}
		return
	}

	qs.armWatchdog(src.questionID)
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
	// #235 : chrono de carte libéré pendant la pause — démarre à la reprise.
	qs.mu.Lock()
	held := qs.heldCardTimer
	qs.heldCardTimer = nil
	qs.mu.Unlock()
	qs.runPending(held)
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
	qs.setIdle()
	qs.app.broadcastUpdate()
	qs.release()
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
	// #288 (§13.3): with a loop, onNaturalEnd is the end of the FIRST pass —
	// the media keeps playing, so QUESTION_SOUND_STATE must not drop to IDLE.
	// Only for a looped question (SOUND_LOOP): a one-shot's natural end is
	// always the end of playback (v11.1 behaviour, byte for byte).
	looping := false
	qs.mu.Lock()
	curLoop := qs.cur != nil && qs.cur.loop
	qs.mu.Unlock()
	if curLoop {
		looping = qs.player.State() == audio.MediaPlaying
	}
	if !looping {
		qs.setIdle()
		qs.app.broadcastUpdate()
	}
	qs.release()
}

// watchdogPeriod is the watchdog delay: the injected watchdogDelay, else the production
// default audio.MaxQuestionSoundDuration + questionSoundWatchdogGrace (32 s).
func (qs *questionSoundAdapter) watchdogPeriod() time.Duration {
	if qs.watchdogDelay > 0 {
		return qs.watchdogDelay
	}
	return audio.MaxQuestionSoundDuration + questionSoundWatchdogGrace
}

// armWatchdog starts a goroutine that releases the deferred timer at the
// latest audio.MaxQuestionSoundDuration+questionSoundWatchdogGrace after
// this Play() call, in case no end-of-playback signal ever arrives (a dead
// monitoring goroutine, a wedged driver — contract §10.7's "chien de
// garde"). Cancelled (never fires) by any subsequent play/Stop/
// onNaturalEnd. Caller must NOT hold qs.mu.
func (qs *questionSoundAdapter) armWatchdog(questionID string) {
	ctx, cancel := context.WithCancel(context.Background())
	delay := qs.watchdogPeriod()
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
		case <-time.After(delay):
			server.LogWarn(game.LogComponentApp, "Question sound: watchdog fired for question %s — releasing the deferred timer (contract §10.7)", questionID)
			qs.release()
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

// refreshQuestionSoundAvailability recomputes the media-availability
// verdict for the CURRENT question and pushes it into GameState (contract
// sound.md §10.8.7's "quatre moments" — before every PONG-driven
// PREPARE↔READY evaluation, at question selection, on OnConfigUpdate, and
// just before Engine.Start()). Computed entirely in THIS application
// layer, never under the engine's own lock (§10.8.3: internal/game imports
// neither internal/audio nor internal/config — participantsConform must
// stay pure). Cheap and safe to call unconditionally at every one of those
// moments: Engine.SetQuestionSoundUnavailable is itself a no-op when the
// verdict hasn't changed, and validateSoundFile's cache (§10.8.3) makes an
// unchanged file a bare os.Stat, never a re-parse.
func (a *App) refreshQuestionSoundAvailability() {
	state := a.engine.GetState()
	_, reason := a.questionSound().questionSoundAvailability(state.Question)
	// #235 : une carte MEMOTION ouverte dont le média est indisponible remonte son
	// motif comme un état (le retournement n'est jamais refusé, §14.4).
	if reason == "" && state.Question != nil && state.MotionSelected != "" &&
		state.MotionCardStates[state.MotionSelected] == game.MotionCardStateQuestion {
		for i := range state.Question.MotionCards {
			if c := &state.Question.MotionCards[i]; c.ID == state.MotionSelected && c.Sound != "" {
				_, reason = a.questionSound().sourceAvailability(c.Sound)
			}
		}
	}
	a.engine.SetQuestionSoundUnavailable(reason)
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
