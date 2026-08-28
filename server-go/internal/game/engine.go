package game

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// safeGo runs fn in a new goroutine, recovering any panic so a single
// failed background save can never take down the whole process (bugfix
// #131, plan task 10 — applied uniformly to the ~30 `go e.SaveXxx()`
// call sites in this file, all of which are fire-and-forget persistence
// after a state mutation; none of their callers were checking the returned
// error even before this change — see e.g. setQuestionStatus above the
// first call site). A panic here is logged with a stack trace and the
// goroutine simply exits; the in-memory state (already mutated by the
// synchronous caller before the `go` statement) is unaffected — only that
// one disk write is lost, exactly like any other failed save.
// fn's error return, if any, is also logged, which most existing call
// sites did not do before (errors were silently discarded by `go
// e.SaveXxx()` — a bare `go` statement drops return values).
func safeGo(name string, fn func() error) {
	go func() {
		defer recoverBackgroundPanic(name)
		if err := fn(); err != nil {
			log.Printf("[Engine] background %s failed: %v", name, err)
		}
	}()
}

// recoverBackgroundPanic recovers a panic from a background goroutine (or a
// single iteration of one) and logs it with the same grep-able format used
// throughout this file ("[Engine] recovered panic in background <name>"),
// so safeGo's fire-and-forget saves and the long-lived ticker loops below
// (#151) share one implementation instead of a third ad hoc variant.
//
// Must be invoked as `defer recoverBackgroundPanic(name)` directly — per
// Go's recover() semantics, recover() only stops a panic when called
// directly by the function that is itself deferred; this works here because
// recoverBackgroundPanic IS that deferred function (contrast with
// server.LogRecoveredPanic, which deliberately takes an already-recovered
// value instead of calling recover() itself, because it is invoked one
// frame removed from the deferred func at its call sites).
//
// Callers that wrap a single ticker iteration (not the whole goroutine) rely
// on this being scoped to just that iteration's closure so the surrounding
// `for { select { ... } }` loop survives and keeps consuming the next tick —
// see startTimer/startCountdown/StartMotionCardTimer/StartMotionMemorizeTimer.
func recoverBackgroundPanic(name string) {
	if r := recover(); r != nil {
		log.Printf("[Engine] recovered panic in background %s: %v\n%s", name, r, debug.Stack())
	}
}

// testInjectPanicFn, when non-nil, is invoked at the very start of each
// process*Tick locked body (processCountdownTick, processTimerTick,
// processMotionCardTick, processMotionMemorizeTick) with a site label —
// lets the #151 regression tests (package game) inject a real panic WHILE
// e.mu is held, proving the mutex is still released and the goroutine
// survives, without needing an otherwise-unreachable invalid engine state
// to trigger a panic naturally. Nil in production; set only by _test.go
// files in this package via setTestInjectPanic, and must be reset (e.g. via
// t.Cleanup(clearTestInjectPanic)) so one test's injection doesn't leak
// into another.
//
// Guarded by testInjectPanicMu rather than a bare package var: the ticker
// goroutines read it from inside process*Tick while e.mu is held, but a
// test assigning it directly (`testInjectPanicFn = fn`) from the test's own
// goroutine would NOT be synchronized with those reads just because the
// reader happens to also hold e.mu — e.mu only orders accesses that both
// sides actually acquire, and the test's assignment doesn't. `go test
// -race` catches exactly this if the accessors below are bypassed.
var (
	testInjectPanicMu sync.Mutex
	testInjectPanicFn func(site string)
)

// setTestInjectPanic installs the #151 panic-injection hook race-safely.
// _test.go files in this package must use this (and clearTestInjectPanic
// for cleanup) instead of assigning testInjectPanicFn directly.
func setTestInjectPanic(fn func(site string)) {
	testInjectPanicMu.Lock()
	defer testInjectPanicMu.Unlock()
	testInjectPanicFn = fn
}

// clearTestInjectPanic removes the hook installed by setTestInjectPanic.
func clearTestInjectPanic() {
	setTestInjectPanic(nil)
}

// callTestInjectPanic invokes the currently installed hook, if any. Called
// from the 4 process*Tick methods below.
func callTestInjectPanic(site string) {
	testInjectPanicMu.Lock()
	fn := testInjectPanicFn
	testInjectPanicMu.Unlock()
	if fn != nil {
		fn(site)
	}
}

// Engine manages the game state and logic
type Engine struct {
	state            GameState
	data             *TeamsAndBumpers
	questionStatuses map[string]QuestionStatus // Track question statuses across selections
	history          []GameEvent
	historyPath      string
	teamsPath        string
	bumpersPath      string
	statusesPath     string // Path to question_statuses.json
	statePath        string // Path to game_state.json (#141 — quiz metadata persistence)

	// RAFALE reservoir (#197, contracts/rafale.md §2.4/§3). Kept as two
	// independent stores, deliberately not merged: editing the reservoir
	// must never rewrite game-play state, and playing must never rewrite
	// the reservoir (contract §3.2). Both are guarded by e.mu, same as
	// questionStatuses above.
	rafaleQuestions map[string]*RafaleQuestion // reservoir, keyed by ID
	rafalePath      string                     // path to data/files/rafale/reservoir.json
	rafaleUsed      map[string]bool            // "already asked this game" flag, keyed by question ID
	rafaleUsedPath  string                     // path to data/config/rafale_used.json

	// rafaleQuestionTimer/rafaleQuestionStopCh (v8.0.0, #107, contract
	// rafale.md §2.2) — the per-question ~3s countdown, DELIBERATELY its own
	// pair of fields, never e.timer/e.stopCh. The global round timer
	// (started via the existing startTimer(), reused unchanged for RAFALE's
	// manche timer) and this question ticker run CONCURRENTLY for the
	// whole round — the first time two tickers are live at once in this
	// codebase (MEMOTION's per-card timer reuses e.timer/e.stopCh instead,
	// because it and the global timer are mutually exclusive — never both
	// running). Mixing this ticker into e.timer/e.stopCh would make
	// StartRafaleQuestionTimer's "stop any existing timer first" step also
	// kill the round timer, and vice versa.
	rafaleQuestionTimer  *time.Ticker
	rafaleQuestionStopCh chan struct{}
	mu                   sync.RWMutex
	timer                *time.Ticker
	stopCh               chan struct{}
	countdownTimer       *time.Ticker
	countdownStopCh      chan struct{}
	pendingDelay         int // Store delay for after countdown

	// motionCardRoundClosed (#187 cycle 4, B3) — true once the ACTIVE
	// MEMOTION card's own timer round has ended (natural expiry via
	// processMotionCardTick, or an explicit StopMotionCardTimer —
	// MEMOTION_STOP_TIMER, MEMOTION_REVEAL, MEMOTION_DONE, and the
	// card-scoped grid-complete path all route through it). Gates
	// FlipMotionMemoryCard independently of MotionSubPhase, which stays
	// QUESTION on natural expiry (deliberate asymmetry with the
	// grid-complete exit — plan-memotion-v710-memory-reveal-v2 §1).
	//
	// Deliberately NOT derived from CURRENT_TIME==0: StartMotionCardTimer
	// is a no-op when Question.TIME<=0 (guard, delay<=0 returns before
	// touching anything) — a MEMOTION question with no configured timer
	// never starts one, so CURRENT_TIME stays 0 forever without the round
	// ever being "closed". Gating on CURRENT_TIME==0 alone would make a
	// timerless MEMORY card permanently unplayable, silently, with no
	// failing test to catch it (the exact trap named in the plan).
	//
	// Reset to false at SelectMotionCard and FlipMotionCard — a fresh
	// card's round is always open. Not reset by DoneMotionCard: MotionSelected
	// is cleared there too, and FlipMotionMemoryCard already refuses any
	// card whose ID isn't the current MotionSelected, so a stale `true`
	// left over from the previous card has no way to reach a live guard.
	motionCardRoundClosed bool

	// Callbacks
	//
	// OnStateChange — concurrency contract (#121): invoked from every one of
	// the 12 call sites marked "Release lock BEFORE calling callback to
	// avoid deadlock" throughout this file (e.g. :636, :724, :931 — the
	// countdown goroutine started by actualStart() — and :1200, the
	// synchronous Pause() path). The engine deliberately does NOT serialize
	// these invocations: a mutex held across the callback would sequence
	// every state broadcast behind it and reintroduce exactly the lock-
	// ordering risk this release-before-call pattern exists to avoid. Two
	// calls (e.g. the countdown's PhaseStarted and a near-simultaneous
	// Pause()'s PhasePaused) can therefore run on different goroutines with
	// no ordering guarantee between them. The consumer MUST be thread-safe
	// on its own — this was the actual root cause of #121's flaky
	// TestE2E_GameStateMachine (internal/server/e2e_test.go), which
	// appended to an unsynchronized slice from this callback. The one
	// production consumer (cmd/server/main.go, wired in setupCallbacks) is
	// safe today: broadcastGameState ignores its phase argument entirely
	// (always re-reads current state), ardoiseCoalescer.Flush() has its own
	// internal mutex, and broadcastQuestions() re-reads live state too — but
	// that safety is a property of what each consumer happens to do, not a
	// guarantee this field provides.
	OnStateChange   func(phase GamePhase)
	OnTimerTick     func(currentTime int)
	OnCountdownTick func(countdownTime int)
	OnBuzzerPress   func(bumperID, teamID string, pressTime int64, button string)
	OnQCMHint       func(invalidatedColor string, remainingAnswers int) // QCM hint callback
	// RAFALE callbacks (v8.0.0, #107, contract §5.2/§2.2).
	//
	// OnRafaleAnswer fires whenever a new RAFALE question is drawn — round
	// start (startRafaleRoundUnsafe) and every subsequent advance
	// (advanceRafaleUnsafe, whether from RAFALE_VALIDATE/INVALIDATE or the
	// question timer's own expiry). The consumer (cmd/server/main.go) must
	// broadcast it via RAFALE_ANSWER to admin+anim ONLY
	// (BroadcastToTypes) — contract §2.3: the answer never rides GameState.
	OnRafaleAnswer func(id, answer string)
	// OnRafaleQuestionTick fires every second the RAFALE question ticker is
	// live and NOT expiring this tick — the consumer broadcasts RAFALE_TICK
	// (lightweight, all clients, no full GameState re-emission — contract
	// §5.2). On expiry (questionTime reaches 0), no tick fires at all —
	// the ticker instead routes straight into the SAME advance path as
	// RAFALE_INVALIDATE (contract §6.1's "identique à réponse invalide"),
	// which fires OnStateChange (full UPDATE, carrying the new
	// RAFALE_CURRENT_QUESTION/RAFALE_SUBPHASE) and OnRafaleAnswer instead.
	OnRafaleQuestionTick func(questionTime int)
	// OnRafaleTeamsChanged (v8.0.0, #199, contract §8.3) fires whenever
	// RAFALE's active/next/participating team set MAY need a buzzer LED
	// refresh: round start, RAFALE_SET_TEAMS, and every advance (VALIDATE/
	// INVALIDATE/question-timer expiry) — including an advance where
	// RafaleCurrentTeam doesn't actually change (TANT_QUE_JE_GAGNE/SOLO
	// keeping the hand), since §8.3 requires a refresh "à chaque...
	// changement de question" too, not only an actual rotation. No
	// parameters — the consumer (cmd/server/main.go, sendLEDSetRafaleTeams)
	// re-reads live GetState()/GetTeamsAndBumpers() itself, same pattern as
	// OnStateChange.
	OnRafaleTeamsChanged func()
	// #187 cycle 3 briefly added OnMotionCardAutoRevealed here (a MEMORY
	// card auto-revealing at timer expiry) — REVERTED in cycle 4: the
	// user-validated behavior is that expiry on an incomplete grid leaves
	// the card in QUESTION, closed to further flips
	// (motionCardRoundClosed) but requiring an explicit animateur
	// MEMOTION_REVEAL, asymmetric with the completed-grid exit (see
	// processMotionCardTick's doc comment). Do not reintroduce this
	// callback without re-reading plan-memotion-v710-memory-reveal-v2's
	// §1 rationale.
}

// NewEngine creates a new game engine
func NewEngine() *Engine {
	return &Engine{
		state: GameState{
			Phase:              PhaseStopped,
			Delay:              30,
			CurrentTime:        0,
			Page:               "GAME",
			VirtualPlayerLimit: 20, // Default limit
			// MEMOTION: initialize with empty (not nil) so JSON serializes [] and {} (not null)
			MotionCardStates:         make(map[string]MotionCardState),
			MotionCardTeams:          make(map[string]string),
			MotionParticipatingTeams: []string{},
			MotionCurrentTeamColor:   []int{},
			MotionActive:             MotionActive{State: map[string]interface{}{}},
			// ARDOISE: initialize empty map so JSON serializes {} (not null)
			ArdoiseAnswers: make(map[string]ArdoiseAnswer),
			// RAFALE (v8.0.0, #107): initialize empty (not nil) so JSON
			// serializes []/{} (not null) — same "no omitempty" discipline
			// as MEMOTION/ARDOISE above (contracts/rafale.md §4).
			RafaleCurrentQuestion:    RafaleCurrent{},
			RafaleTeamCounters:       map[string]int{},
			RafaleTeamBest:           map[string]int{},
			RafaleParticipatingTeams: []string{},
			RafaleCurrentTeamColor:   []int{},
			// Quiz metadata multi-values (v6.1.0, #137 Batch 2b): initialize
			// empty (not nil) so JSON serializes [] (not null) before the
			// first UPDATE_QUIZ_META (contract game-state.md §"Aucun omitempty").
			QuizPopulations:  []string{},
			QuizDifficulties: []string{},
			// TV display preference (v6.1.0, #137 Batch 2b T1.8): empty = all
			// four eligible fields shown, the desired default (contract
			// game-state.md rule H1).
			QuizHiddenFields: []string{},
			// ENTRACTE config (v6.5.2, #119, C1): compile-time defaults for
			// BOTH the diffused and saved variants — identical, since a
			// fresh engine is definitionally outside any entracte. LoadState
			// overwrites both together if game_state.json carries a
			// configured value (state_persistence.go); otherwise these are
			// what a freshly connecting client, or the Quiz page's edit
			// form, sees before any UPDATE_ENTRACTE_CONFIG has ever run.
			// IMAGE_IS_CUSTOM starts false — cmd/server/main.go recomputes
			// it from disk right after LoadState, at every startup.
			EntracteConfig: EntracteConfig{
				Title: "ENTRACTE", Subtitle: "Retour dans 20mn",
				PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000,
			},
			EntracteConfigSaved: EntracteConfig{
				Title: "ENTRACTE", Subtitle: "Retour dans 20mn",
				PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000,
			},
		},
		data:             NewTeamsAndBumpers(),
		questionStatuses: make(map[string]QuestionStatus),
		rafaleQuestions:  make(map[string]*RafaleQuestion),
		rafaleUsed:       make(map[string]bool),
		stopCh:           make(chan struct{}),
	}
}

// ConnEvent drives the Bumper.ConnState transition table (see TransitionConn).
// See contracts/websocket-actions.md and the connection-badge plan (§2) for the
// full table. Phase 1 (#109) wired Disconnect/Reconnect at the 6 connection
// call sites. Phase 2 (#109) wires MessageLost (buzzer LED_SET/OTA/WIFI send,
// VJoueur broadcast) and DeliveryConfirmed (buzzer ACK, VJoueur bidirectional
// confirm) at their real sources, plus the minimum 2s "green" display window
// (D2/D3) — see ConfirmDelivery, which is the gated entry point production
// code should call for DeliveryConfirmed instead of TransitionConn directly.
type ConnEvent string

const (
	ConnEventDisconnect        ConnEvent = "DISCONNECT"
	ConnEventReconnect         ConnEvent = "RECONNECT"
	ConnEventMessageLost       ConnEvent = "MESSAGE_LOST"
	ConnEventDeliveryConfirmed ConnEvent = "DELIVERY_CONFIRMED"
)

// transitionConnUnsafe applies the CONN_STATE transition table to a single
// bumper, without the participants-only filter (see applyConnEventUnsafe for
// the filtered entry point used everywhere else). Caller must hold e.mu.
//
//	Current  | Disconnect | Reconnect | MessageLost | DeliveryConfirmed
//	HIDDEN   | -> orange  | (n/a)     | -> HIDDEN   | -> HIDDEN
//	orange   | orange     | -> green  | -> red      | (n/a)
//	red      | red        | -> green  | red         | (n/a)
//	green    | -> orange  | green     | (n/a)       | -> HIDDEN
func transitionConnUnsafe(b *Bumper, event ConnEvent) {
	switch event {
	case ConnEventDisconnect:
		if b.ConnState == ConnStateHidden || b.ConnState == ConnStateGreen {
			b.ConnState = ConnStateOrange
			// #129: skipNextMessageLost is NOT set here anymore. It used to
			// give a one-time pass to "the broadcast that announces this
			// disconnect" — under #127, that broadcast (onPlayerDisconnected's
			// a.broadcastUpdate()) always reached VJoueurs and so was always
			// the very next call to ApplyVPlayerBroadcastConnEvents(),
			// consuming the pass harmlessly right where it was meant to.
			// #129 T1.3 retargets that call to Admin/TV/Buzzer only — it
			// never reaches VPlayer and so never calls
			// ApplyVPlayerBroadcastConnEvents() at all anymore. Setting the
			// flag here left it dangling: the NEXT unrelated VPlayer-targeting
			// broadcast would consume it instead, silently absorbing a
			// genuinely missed message for one cycle (orange incorrectly
			// stayed orange instead of turning red) — caught by
			// TestConnStateProtocol_MissedBroadcastWhileDisconnected_StillTurnsRed.
			// The self-referential case this flag protected against
			// structurally no longer exists post-#129, so the flag is no
			// longer set. skipNextMessageLost the field, and its consumption
			// in ApplyVPlayerBroadcastConnEvents, are left in place — dead
			// weight for this path only, not removed, to avoid widening this
			// fix beyond the one line that caused the regression.
		}
	case ConnEventReconnect:
		if b.ConnState == ConnStateOrange || b.ConnState == ConnStateRed {
			b.ConnState = ConnStateGreen
			// Start the D2/D3 minimum display window: a DeliveryConfirmed arriving
			// before greenMinDuration has elapsed must not hide the badge immediately
			// (see ConfirmDelivery). Only meaningful for real, timestamped Reconnects —
			// harmless for bare TransitionConn() calls in tests, which never call
			// ConfirmDelivery and so never consult these fields.
			b.greenSince = time.Now()
			b.confirmPending = false
		}
	case ConnEventMessageLost:
		if b.ConnState == ConnStateOrange {
			b.ConnState = ConnStateRed
		}
	case ConnEventDeliveryConfirmed:
		if b.ConnState == ConnStateGreen {
			b.ConnState = ConnStateHidden
		}
	}
}

// applyConnEventUnsafe is the participants-only entry point for the CONN_STATE
// machine: bumpers not assigned to a team (Team == "") never carry a visible
// badge and are always forced back to hidden. Caller must hold e.mu.
func applyConnEventUnsafe(b *Bumper, event ConnEvent) {
	if b.Team == "" {
		b.ConnState = ConnStateHidden
		return
	}
	transitionConnUnsafe(b, event)
}

// syncConnStateForTeamChangeUnsafe reacts to a bumper's TEAM field changing,
// keeping CONN_STATE consistent with the participants-only scope: becoming a
// participant while disconnected evaluates as an implicit Disconnect (→ orange)
// instead of waiting for a future disconnect event; leaving the participant
// pool forces the badge back to hidden. Caller must hold e.mu.
func (e *Engine) syncConnStateForTeamChangeUnsafe(bumper *Bumper, oldTeam string) {
	switch {
	case oldTeam == "" && bumper.Team != "":
		if !bumper.Connected {
			applyConnEventUnsafe(bumper, ConnEventDisconnect)
		}
	case oldTeam != "" && bumper.Team == "":
		bumper.ConnState = ConnStateHidden
	}
}

// TransitionConn applies a connection-state event to a bumper's CONN_STATE
// badge (see transitionConnUnsafe for the table). Thread-safe. Unknown bumper
// IDs are silently ignored (mirrors UpdateBumper's tolerant style).
func (e *Engine) TransitionConn(bumperID string, event ConnEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, exists := e.data.Bumpers[bumperID]
	if !exists {
		return
	}
	applyConnEventUnsafe(bumper, event)
}

// connGreenMinDuration is the minimum time (D2/D3, plan §2) a bumper's badge
// stays "green" after reconnecting before a DeliveryConfirmed signal is
// allowed to hide it. A package variable (not a const) so tests can shrink it
// instead of sleeping for real time.
var connGreenMinDuration = 2 * time.Second

// reclaimAuthorizationTTL bounds how long an animateur-granted reclaim
// authorization (#122 B3, ReleaseBumperName) stays valid: "a few minutes" per
// the plan — long enough for the flagged player to notice and retry, short
// enough that it can never survive into a later game/session. A package
// variable (not a const) so tests can shrink it instead of sleeping for real
// time, same convention as connGreenMinDuration above.
var reclaimAuthorizationTTL = 5 * time.Minute

// ConfirmDelivery is the production entry point for the DELIVERY_CONFIRMED
// event (#109 Phase 2, D2/D3): unlike a raw TransitionConn(id,
// ConnEventDeliveryConfirmed) call, it honors the minimum "green" display
// window. If the bumper reconnected less than connGreenMinDuration ago, the
// HIDDEN transition is deferred to fire automatically once the window
// elapses, instead of applying immediately. Bumpers not currently "green"
// (nothing to gate) fall through to the plain, immediate transition.
//
// TransitionConn itself is intentionally left untouched by this window logic
// (used as-is by tests exercising the pure table, e.g. TestTransitionConn_Table)
// — only real call sites (buzzer ACK, VJoueur broadcast/message) should call
// ConfirmDelivery.
func (e *Engine) ConfirmDelivery(bumperID string) {
	e.mu.Lock()

	bumper, exists := e.data.Bumpers[bumperID]
	if !exists {
		e.mu.Unlock()
		return
	}
	if bumper.ConnState != ConnStateGreen {
		applyConnEventUnsafe(bumper, ConnEventDeliveryConfirmed)
		e.mu.Unlock()
		return
	}

	elapsed := time.Since(bumper.greenSince)
	if elapsed >= connGreenMinDuration {
		applyConnEventUnsafe(bumper, ConnEventDeliveryConfirmed)
		e.mu.Unlock()
		return
	}

	if bumper.confirmPending {
		// Already scheduled for this green period — nothing more to do.
		e.mu.Unlock()
		return
	}
	bumper.confirmPending = true
	scheduledFor := bumper.greenSince
	remaining := connGreenMinDuration - elapsed
	e.mu.Unlock()

	time.AfterFunc(remaining, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		b, ok := e.data.Bumpers[bumperID]
		if !ok {
			return
		}
		// Re-validate under lock: only act if still the same green period we
		// scheduled for (b.greenSince.Equal(scheduledFor)) — a re-disconnect +
		// fresh reconnect in the meantime starts a NEW period with its own
		// confirmPending lifecycle; this now-stale timer must not touch it (fix
		// R1 code-review 5.1: clearing confirmPending unconditionally here could
		// wipe the flag for that new period instead of its own).
		if b.greenSince.Equal(scheduledFor) {
			b.confirmPending = false
			if b.ConnState == ConnStateGreen {
				applyConnEventUnsafe(b, ConnEventDeliveryConfirmed)
			}
		}
	})
}

// ApplyVPlayerBroadcastConnEvents evaluates the connection-badge machine for
// every participant VJoueur against a GameState broadcast that is about to go
// out (#109 Phase 2, D4): per the user-validated "no restricted list" design,
// a disconnected participant VJoueur counts this broadcast as a lost message
// (MessageLost); a connected one counts it as a successful delivery (D3),
// eligible to close the green window via ConfirmDelivery. Intended to be
// called once per GameState broadcast (see main.go broadcastUpdate()).
func (e *Engine) ApplyVPlayerBroadcastConnEvents() {
	e.mu.Lock()
	var toConfirm []string
	for id, b := range e.data.Bumpers {
		if !b.IsVPlayer || b.Team == "" {
			continue
		}
		if !b.Connected {
			if b.skipNextMessageLost {
				// This is the broadcast announcing the disconnect itself (see
				// transitionConnUnsafe's Disconnect case) — consume the pass
				// without firing MessageLost, so ORANGE is actually visible
				// before any real "message they missed" can turn it RED.
				b.skipNextMessageLost = false
				continue
			}
			applyConnEventUnsafe(b, ConnEventMessageLost)
			continue
		}
		// Only bumpers currently "green" can be affected by DeliveryConfirmed
		// (fix R1 code-review 5.2) — skip the rest to avoid re-acquiring the
		// lock via ConfirmDelivery for a guaranteed no-op on every connected,
		// already-hidden VJoueur on every single broadcast.
		if b.ConnState == ConnStateGreen {
			toConfirm = append(toConfirm, id)
		}
	}
	e.mu.Unlock()

	// ConfirmDelivery manages its own locking (and may schedule a timer via
	// time.AfterFunc), so it must run after releasing the lock above to avoid a
	// self-deadlock / re-entrant lock on e.mu.
	for _, id := range toConfirm {
		e.ConfirmDelivery(id)
	}
}

// GetState returns current game state
func (e *Engine) GetState() GameState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// GetPhase returns current phase
func (e *Engine) GetPhase() GamePhase {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase
}

// HostContext is the normalized triplet a type implementation reads
// instead of GamePhase/MEMOTION_SUBPHASE directly — contracts/
// question-types.md §4. Two hosts can produce it: the question host (the
// classic GamePhase cycle) for any non-MEMOTION question, and the
// MEMOTION-card host (MEMOTION_SUBPHASE) while a MEMOTION question is in
// play — never both, since MEMOTION disables buzzing on the top-level
// question (ProcessButtonPress) and drives everything through its own
// sub-phase while PHASE stays STARTED throughout.
//
// Never serialized: recomputed from PHASE/MEMOTION_SUBPHASE, both already
// present in GameState. Deliberately NOT sent over the wire — see the
// contract's cost/benefit note. Mirrored, field-for-field and case-for-case,
// by utils/hostContext.js on the frontend; the derivation table (§4) is the
// single specification for both, and each side's test cases share the same
// names (getHostContextTest.go's t.Run names ↔ hostContext.test.js's
// describe/it names) so a mismatch between the two implementations is
// visible from either test file alone.
type HostContext struct {
	Playable     bool   // inputs are accepted, content is in play
	Revealed     bool   // the answer is shown
	TimerRunning bool   // a countdown is running for this round
	CardID       string // "" for the question host; the active card's ID for the card host
}

// GetHostContext derives the current HostContext — contract §4's
// derivation table, the one and only place this engine computes it.
// Self-locking (RLock) like GetState/GetPhase: safe to call from outside
// the engine without pre-existing lock discipline.
func (e *Engine) GetHostContext() HostContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.hostContextUnsafe()
}

// hostContextUnsafe is GetHostContext's body, split out so call sites that
// already hold e.mu (none yet in v7.0.0 — #185/#186 are expected to need
// this) can derive the context without double-locking. Must be called with
// e.mu held (read or write).
//
// CardID — contract §4 ("règle unique, sans cas particulier", B-B9 fix for
// a Go/JS divergence test-writer caught in SELECTED, planner ruling: Go was
// wrong): CardID always equals MotionSelected, no branching, because
// MotionSelected already IS the identity of the card in play at every
// instant — "" outside a MEMOTION round (reset in Ready()/InitGame()) and
// while GRID/MEMORIZE (reset by initMotionStateUnsafe, the memorize-timer's
// auto-expiry, and every return to GRID), and the card's ID from
// SelectMotionCard onward through SELECTED/QUESTION/REVEAL. Setting it once
// here, unconditionally, before the switch below (which only ever touches
// Playable/Revealed/TimerRunning) is what makes that "no special case for
// SELECTED" property hold by construction rather than by remembering to
// repeat it in three switch arms. Must equal MotionActive.CardID (§5.2) and
// the MotionSelected ValidateCardScope (§9.2, B-B6) already compares
// against — same expression, three call sites, per the contract's internal
// consistency requirement.
func (e *Engine) hostContextUnsafe() HostContext {
	ctx := HostContext{CardID: e.state.MotionSelected}

	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		switch e.state.MotionSubPhase {
		case MotionSubPhaseQuestion:
			ctx.Playable = true
			// TimerRunning keys on CurrentTime, not e.timer (correction,
			// planner ruling, contract §4): e.timer is a private
			// *time.Ticker, never serialized — structurally impossible to
			// replicate on the JS side, which only ever sees
			// gameState.timer (CurrentTime) over the wire. CurrentTime is
			// the only basis both implementations can actually share.
			ctx.TimerRunning = e.state.CurrentTime > 0
		case MotionSubPhaseReveal:
			ctx.Revealed = true
		}
		// GRID, MEMORIZE, SELECTED: Playable/Revealed/TimerRunning stay
		// false (contract §4's "Aucun" row) — CardID was already set above
		// and needs no further action here, SELECTED included.
		return ctx
	}

	// Question host — classic GamePhase cycle. ctx.CardID is already ""
	// here: MotionSelected is always empty outside a MEMOTION round.
	ctx.Playable = e.state.Phase == PhaseStarted
	ctx.Revealed = e.state.Phase == PhaseRevealed
	// TimerRunning also keys on CurrentTime (see the MotionSubPhaseQuestion
	// case above for why) — PHASE==STARTED alone doesn't distinguish a
	// running countdown from one that already reached 0.
	ctx.TimerRunning = e.state.Phase == PhaseStarted && e.state.CurrentTime > 0
	return ctx
}

// SetPhase sets the game phase
func (e *Engine) SetPhase(phase GamePhase) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Phase = phase
	log.Printf("[Engine] Phase set to: %s", phase)
}

// GetTeamsAndBumpers returns teams and bumpers data
func (e *Engine) GetTeamsAndBumpers() *TeamsAndBumpers {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data
}

// GetTeam returns a specific team
func (e *Engine) GetTeam(id string) *Team {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data.Teams[id]
}

// GetBumper returns a specific bumper
// GetBumper returns a snapshot copy of the bumper identified by id (nil if
// unknown). Returns a COPY, not the live map entry (#109 Phase 2): since
// ConfirmDelivery may schedule a background timer (time.AfterFunc) that
// mutates a bumper's ConnState/greenSince/confirmPending fields asynchronously
// under e.mu, any caller reading fields off a *live* pointer after this
// function had already released the lock would race against that timer. All
// production callers only read fields from the result (never mutate through
// it — mutations always go through UpdateBumper/AssignVirtualPlayer/etc.), so
// this is a safe, behavior-preserving change.
func (e *Engine) GetBumper(id string) *Bumper {
	e.mu.RLock()
	defer e.mu.RUnlock()
	b, ok := e.data.Bumpers[id]
	if !ok || b == nil {
		return nil
	}
	snapshot := *b
	return &snapshot
}

// GetQuestionStatus returns the status of a question by ID
func (e *Engine) GetQuestionStatus(id string) QuestionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if status, ok := e.questionStatuses[id]; ok {
		return status
	}
	return StatusAvailable
}

// setQuestionStatus updates both the current question and the status map (must hold lock)
func (e *Engine) setQuestionStatus(status QuestionStatus) {
	if e.state.Question != nil {
		e.state.Question.Status = status
		e.questionStatuses[e.state.Question.ID] = status
		safeGo("SaveStatuses", e.SaveStatuses) // Persist to disk
	}
}

// UpdateBumper updates or creates a bumper
func (e *Engine) UpdateBumper(id string, data map[string]interface{}) {
	e.mu.Lock()

	bumper, exists := e.data.Bumpers[id]
	if !exists {
		bumper = &Bumper{}
		e.data.Bumpers[id] = bumper
	}

	// Update fields from data
	if name, ok := data["NAME"].(string); ok {
		bumper.Name = name
	}
	// Connection status (v3.6.5) — also drives the CONN_STATE badge machine (#109
	// Phase 1): every call site that flips CONNECTED implicitly fires the matching
	// Disconnect/Reconnect event, so no explicit wiring is needed at those 6 sites.
	// IMPORTANT: applied BEFORE TEAM below so that a single call setting both
	// TEAM and CONNECTED:true (e.g. a bumper created already-connected) reflects
	// bumper.Connected's final value in the TEAM-change sync, instead of firing a
	// spurious implicit-disconnect-then-reconnect ("green") for a bumper that was
	// never actually disconnected.
	if connected, ok := data["CONNECTED"].(bool); ok {
		bumper.Connected = connected
		if connected {
			applyConnEventUnsafe(bumper, ConnEventReconnect)
		} else {
			applyConnEventUnsafe(bumper, ConnEventDisconnect)
		}
	}
	if team, ok := data["TEAM"].(string); ok {
		oldTeam := bumper.Team
		bumper.Team = team
		if oldTeam != team {
			e.syncConnStateForTeamChangeUnsafe(bumper, oldTeam)
		}
	}
	if version, ok := data["VERSION"].(string); ok {
		bumper.Version = version
	}
	if ip, ok := data["IP"].(string); ok {
		bumper.IP = ip
	}
	if proto, ok := data["PROTOCOL"].(string); ok {
		bumper.Protocol = proto
	}
	// OTA firmware fields (v3.1.0+)
	if fwVersion, ok := data["FIRMWARE_VERSION"].(string); ok {
		bumper.FirmwareVersion = fwVersion
	}
	if isOutdated, ok := data["IS_OUTDATED"].(bool); ok {
		bumper.IsOutdated = isOutdated
	}
	if otaStatus, ok := data["OTA_STATUS"].(string); ok {
		bumper.OTAStatus = otaStatus
	}
	if otaPercent, ok := data["OTA_PERCENT"].(int); ok {
		bumper.OTAPercent = otaPercent
	}
	// ACK pending flag (v3.8.0)
	if ackPending, ok := data["ACK_PENDING"].(bool); ok {
		bumper.AckPending = ackPending
	}

	log.Printf("[Engine] Updated bumper %s: team=%s, name=%s, protocol=%s", id, bumper.Team, bumper.Name, bumper.Protocol)
	e.mu.Unlock()

	// Auto-save bumpers to disk
	safeGo("SaveBumpers", e.SaveBumpers)
}

// UpdateTeam updates or creates a team
func (e *Engine) UpdateTeam(id string, team *Team) {
	e.mu.Lock()
	e.data.Teams[id] = team
	e.mu.Unlock()

	// Auto-save teams to disk
	safeGo("SaveTeams", e.SaveTeams)
}

// SetTeams sets all teams
func (e *Engine) SetTeams(teams map[string]*Team) {
	e.mu.Lock()
	// Ensure team NAME field is populated from the map key
	for teamID, team := range teams {
		if team.Name == "" {
			team.Name = teamID
		}
	}
	e.data.Teams = teams
	e.mu.Unlock()

	// Auto-save teams to disk. Synchronous (unlike the per-bumper hot paths, which
	// stay async): SetTeams is a low-frequency bulk action (team roster edits), and
	// firing this in a background goroutine let it race with a caller's own
	// SaveTeams()/LoadTeams() sequence (#113 B4) — two unsynchronized writers to the
	// same file, or a goroutine still in flight after the caller moved on. Returning
	// only once the write has actually landed also closes a small durability gap:
	// a crash right after the old async SetTeams() returned could lose the update.
	if err := e.SaveTeams(); err != nil {
		log.Printf("[Engine] SetTeams: auto-save failed: %v", err)
	}
}

// SetBumpers sets all bumpers and synchronizes VirtualPlayerCount
func (e *Engine) SetBumpers(bumpers map[string]*Bumper) {
	e.mu.Lock()
	// Reconcile CONN_STATE against the previous map before swapping it out (#109
	// Phase 1): this bulk path (admin FULL/UPDATE, e.g. team assignment via
	// TeamsPage) is a real TEAM-change site, but the incoming payload's CONN_STATE
	// is whatever the client last saw — never authoritative. Preserve the
	// server-side value and only re-evaluate it when TEAM actually changed.
	oldBumpers := e.data.Bumpers
	for id, newBumper := range bumpers {
		if newBumper == nil {
			continue
		}
		oldTeam := ""
		if old, ok := oldBumpers[id]; ok && old != nil {
			oldTeam = old.Team
			newBumper.ConnState = old.ConnState
			// Also carry over the green-window bookkeeping (#109 Phase 2) — losing
			// greenSince here would zero it out, making the next ConfirmDelivery
			// think the window elapsed ages ago and hide the badge prematurely.
			newBumper.greenSince = old.greenSince
			newBumper.confirmPending = old.confirmPending
			// And the disconnect-announcement grace pass (conn-state fix,
			// code-review 20260726) — losing it mid-window (a bulk SetBumpers
			// landing between the Disconnect->orange transition and the next
			// ApplyVPlayerBroadcastConnEvents) would re-expose the skipped-orange
			// bug for that one bumper on this one occasion.
			newBumper.skipNextMessageLost = old.skipNextMessageLost
		}
		if oldTeam != newBumper.Team {
			e.syncConnStateForTeamChangeUnsafe(newBumper, oldTeam)
		}
	}
	e.data.Bumpers = bumpers
	// Synchronize VirtualPlayerCount with actual bumper count
	e.state.VirtualPlayerCount = e.countVirtualPlayersUnsafe()
	e.mu.Unlock()

	// Auto-save bumpers to disk
	safeGo("SaveBumpers", e.SaveBumpers)
}

// Ready prepares a new question round
func (e *Engine) Ready(questionID string, question *Question) {
	e.mu.Lock()

	// Allow from: STOPPED, REVEALED, PREPARE, READY, NEW_GAME
	allowedPhases := e.state.Phase == PhaseStopped || e.state.Phase == PhaseRevealed ||
		e.state.Phase == PhasePrepare || e.state.Phase == PhaseReady ||
		e.state.Phase == PhaseNewGame
	if !allowedPhases {
		log.Printf("[Engine] Cannot ready game from phase %s", e.state.Phase)
		e.mu.Unlock()
		return
	}

	// Check if this is a NEW question (different from current)
	isNewQuestion := e.state.Question == nil || e.state.Question.ID != questionID

	e.state.Phase = PhasePrepare
	e.state.Question = question
	e.setQuestionStatus(StatusPrepare)

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.Ready = false
		bumper.HintsAtBuzz = 0
	}

	// Reset team times
	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
		team.Ready = false
	}

	// Always reset QCM hints state at PREPARE (new question or same question re-PREPARED)
	e.state.QcmInvalidated = []string{}

	// Reset Memory game state ONLY for NEW question
	// This allows team selection to persist during PREPARE→READY transition
	if isNewQuestion {
		// Use empty slices/maps instead of nil so they are serialized in JSON (not omitted)
		e.state.MemoryFlippedCards = []string{}
		e.state.MemoryMatchedPairs = []int{}
		e.state.MemoryErrors = 0
		e.state.MemoryCurrentTeam = ""
		e.state.MemoryCurrentTeamColor = []int{}
		e.state.MemoryTeamPairs = map[string]int{}
		e.state.MemoryTeamErrors = map[string]int{}
		e.state.MemoryParticipatingTeams = []string{}
		e.state.MemoryPairOwners = map[int]string{}

		// Reset MEMOTION state for new question (same pattern as Memory)
		e.state.MotionSubPhase = ""
		e.state.MotionSelected = ""
		e.state.MotionCardStates = make(map[string]MotionCardState)
		e.state.MotionCardTeams = make(map[string]string)
		e.state.MotionCurrentTeam = ""
		e.state.MotionCurrentTeamColor = []int{}
		e.state.MotionActive = MotionActive{State: map[string]interface{}{}}
		e.state.MotionParticipatingTeams = []string{}

		// For MEMOTION questions, also pre-populate card states so the frontend
		// can display UNPLAYED cards during the PREPARE phase (before START).
		if question != nil && question.Type == QuestionTypeMemotion {
			e.initMotionStateUnsafe()
		}

		// Reset ARDOISE answers for new question (v5.6.0)
		e.state.ArdoiseAnswers = make(map[string]ArdoiseAnswer)

		// Reset RAFALE state for new question (v8.0.0, #107 — same pattern
		// as MEMOTION/Memory above: unconditional, regardless of the new
		// question's own type, so leftover values from a previous RAFALE
		// round never leak into the next question).
		e.state.RafaleSubPhase = RafaleSubPhaseNone
		e.state.RafaleCurrentQuestion = RafaleCurrent{}
		e.state.RafaleQuestionTime = 0
		e.state.RafaleTeamCounters = map[string]int{}
		e.state.RafaleTeamBest = map[string]int{}
		e.state.RafaleCurrentTeam = ""
		e.state.RafaleParticipatingTeams = []string{}
		e.state.RafaleCurrentTeamColor = []int{}
		e.state.RafaleAskedCount = 0
		e.state.RafalePoolRemaining = 0
		e.state.RafaleExhausted = false
	}

	log.Printf("[Engine] Game ready with question: %s", questionID)

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhasePrepare)
	}
}

// SetBumperReady marks a bumper as ready (responded to PING)
func (e *Engine) SetBumperReady(bumperID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if bumper, ok := e.data.Bumpers[bumperID]; ok {
		bumper.Ready = true

		// Update team ready status
		e.updateTeamsReady()
	}
}

// getActiveTeams returns teams that have at least one buzzer assigned.
// Must be called with e.mu held (read or write).
func (e *Engine) getActiveTeams() map[string]*Team {
	hasBumper := make(map[string]bool)
	for _, bumper := range e.data.Bumpers {
		if bumper.Team != "" {
			hasBumper[bumper.Team] = true
		}
	}
	active := make(map[string]*Team, len(hasBumper))
	for id, team := range e.data.Teams {
		if hasBumper[id] {
			active[id] = team
		}
	}
	return active
}

func (e *Engine) updateTeamsReady() {
	teamReadyCount := make(map[string]int)
	teamBumperCount := make(map[string]int)
	for _, bumper := range e.data.Bumpers {
		if bumper.Team != "" {
			teamBumperCount[bumper.Team]++
			if bumper.Ready {
				teamReadyCount[bumper.Team]++
			}
		}
	}
	for teamID, team := range e.data.Teams {
		total := teamBumperCount[teamID]
		team.Ready = total > 0 && total == teamReadyCount[teamID]
	}
}

// AreAllTeamsReady returns true when every active team (≥1 buzzer assigned) is ready.
// Empty teams are ignored, matching the frontend display filter.
func (e *Engine) AreAllTeamsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.areAllTeamsReadyUnsafe()
}

// areAllTeamsReadyUnsafe is AreAllTeamsReady without locking. Callers must already
// hold e.mu (read or write) — used by reevaluatePrepareReadyUnsafe, which runs from
// inside functions that already hold the write lock (sync.RWMutex is not reentrant).
func (e *Engine) areAllTeamsReadyUnsafe() bool {
	active := e.getActiveTeams()
	if len(active) == 0 {
		return false
	}
	for _, team := range active {
		if !team.Ready {
			return false
		}
	}
	return true
}

// participantsConform (#172 B1) reports whether the currently selected participants
// satisfy the minimum requirement for question's type. Pure function, no locking —
// safe to call from any context, including while e.mu is already held.
//
// Table of rules (plan §6/§7 B1 — no branch by type outside this function):
//   - SPEEDY, QCM, ARDOISE: no requirement of its own — "at least one active team"
//     is already enforced by AreAllTeamsReady, so this simply returns true.
//   - MEMORY SOLO: exactly one team selected.
//   - MEMORY multi (CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE): at least two teams selected.
//   - MEMOTION: at least one team selected.
//   - Unknown/future question type, or nil question: permissive by default (true).
func participantsConform(question *Question, state *GameState) bool {
	if question == nil {
		return true
	}
	switch question.Type {
	case QuestionTypeMemory:
		memoryMode := question.MemoryMode
		if memoryMode == "" {
			memoryMode = string(MemoryModeSolo)
		}
		if memoryMode == string(MemoryModeSolo) {
			return len(state.MemoryParticipatingTeams) == 1
		}
		// CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE
		return len(state.MemoryParticipatingTeams) >= 2
	case QuestionTypeMemotion:
		return len(state.MotionParticipatingTeams) >= 1
	default:
		return true
	}
}

// ParticipantsConform (#172 B1) is the locked, exported wrapper around
// participantsConform for the currently active question — used alongside
// AreAllTeamsReady at PREPARE→READY transition points (main.go). AreAllTeamsReady is
// deliberately left unmodified: the two criteria remain distinct, separately
// readable functions, combined only at the call site (plan §7 B2).
func (e *Engine) ParticipantsConform() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return participantsConform(e.state.Question, &e.state)
}

// reevaluatePrepareReadyUnsafe (#172 B3) re-checks the PREPARE↔READY transition
// after a participant-selection change (SetMemoryParticipatingTeams /
// SetMotionParticipatingTeams). Must be called with e.mu already held (write) and
// returns the new phase if a transition happened, or "" otherwise — the caller is
// responsible for invoking the OnStateChange callback AFTER releasing the lock
// (release-before-call pattern used throughout this file), exactly like Ready(),
// TransitionToReady() and ForceReady() already do.
//
// Only two directions exist, and both are bounded to PREPARE/READY on purpose:
//   - READY, conformity now false (e.g. a team was just removed): back to PREPARE.
//   - PREPARE, buzzers ready AND conformity now true (e.g. a team was just added
//     back): forward to READY, "sans geste supplémentaire" — bumper/team Ready
//     flags are untouched by this function, so no PONG wait is repeated.
//
// Never called from STARTED or any later phase: SetMemoryParticipatingTeams and
// SetMotionParticipatingTeams already refuse any phase other than PREPARE/READY
// before this is reached, so a running round can never be regressed (plan R2).
func (e *Engine) reevaluatePrepareReadyUnsafe() GamePhase {
	switch e.state.Phase {
	case PhaseReady:
		if !participantsConform(e.state.Question, &e.state) {
			e.state.Phase = PhasePrepare
			e.setQuestionStatus(StatusPrepare)
			log.Printf("[Engine] Participants no longer conform, reverting READY -> PREPARE")
			return PhasePrepare
		}
	case PhasePrepare:
		if e.areAllTeamsReadyUnsafe() && participantsConform(e.state.Question, &e.state) {
			e.state.Phase = PhaseReady
			e.setQuestionStatus(StatusReady)
			log.Printf("[Engine] Participants conform again, transitioning PREPARE -> READY")
			return PhaseReady
		}
	}
	return ""
}

// TransitionToReady moves to READY phase when all buzzers responded
func (e *Engine) TransitionToReady() {
	e.mu.Lock()

	if e.state.Phase != PhasePrepare {
		e.mu.Unlock()
		return
	}

	e.state.Phase = PhaseReady
	e.setQuestionStatus(StatusReady)
	log.Printf("[Engine] All teams ready, transitioning to READY")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseReady)
	}
}

// Start begins the game round with a countdown
// For Memory questions, uses MEMORIZE_TIME from config (default 5s) plus cascade animation times
// For other questions, uses 3-second countdown
func (e *Engine) Start(delay int) {
	e.mu.Lock()

	// #172 B4: Start refuses any phase other than READY. Without this guard the
	// PREPARE↔READY conformity mechanism (participantsConform, B1-B3) is only an
	// interface convention — START emitted while still in PREPARE (or any other
	// phase) would bypass it entirely (plan §3). With it, "a started round is
	// conform" becomes a real engine invariant: participant selection can only be
	// changed in PREPARE/READY (SetMemoryParticipatingTeams,
	// SetMotionParticipatingTeams), and READY is only reached when conform.
	if e.state.Phase != PhaseReady {
		log.Printf("[Engine] Cannot start game from phase %s (must be READY)", e.state.Phase)
		e.mu.Unlock()
		return
	}

	// Store delay for after countdown
	e.pendingDelay = delay

	// Determine countdown duration
	countdownDuration := 3 // default for normal/QCM questions

	// MEMOTION: no countdown — grid is displayed immediately after START
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		countdownDuration = 0
		log.Printf("[Engine] MEMOTION: no countdown, starting immediately")
	} else if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemory {
		memorizeTime := 5 // default
		if e.state.Question.MemoryConfig != nil && e.state.Question.MemoryConfig.MemorizeTime > 0 {
			memorizeTime = e.state.Question.MemoryConfig.MemorizeTime
		}

		// Calculate cascade animation duration for Memory questions
		// Frontend uses: STAGGER_DELAY = 200ms per card, FLIP_ANIMATION = 600ms
		cardCount := 0
		if e.state.Question.MemoryPairs != nil {
			cardCount = len(e.state.Question.MemoryPairs) * 2
		}
		// cascadeDuration = (cardCount * 200ms + 600ms) / 1000, rounded up
		cascadeDurationMs := cardCount*200 + 600
		cascadeDurationSecs := (cascadeDurationMs + 999) / 1000 // round up

		// Total = cascade_reveal + memorize_time + cascade_hide
		countdownDuration = cascadeDurationSecs + memorizeTime + cascadeDurationSecs
		log.Printf("[Engine] Memory countdown: cards=%d, cascade=%ds, memorize=%ds, total=%ds",
			cardCount, cascadeDurationSecs, memorizeTime, countdownDuration)
	}

	// Enter COUNTDOWN phase
	e.state.Phase = PhaseCountdown
	e.state.CountdownTime = countdownDuration
	e.state.Delay = delay
	e.state.CurrentTime = delay

	log.Printf("[Engine] Starting %d-second countdown before game (delay=%d)", countdownDuration, delay)

	// Start countdown timer
	e.startCountdown()

	// Capture callbacks before releasing lock
	stateCallback := e.OnStateChange
	countdownCallback := e.OnCountdownTick
	e.mu.Unlock()

	// Notify state change
	if stateCallback != nil {
		stateCallback(PhaseCountdown)
	}

	// Broadcast initial countdown value immediately
	// The ticker will then broadcast remaining values at 1-second intervals
	if countdownCallback != nil {
		countdownCallback(countdownDuration)
	}
}

// startCountdown starts the 3-2-1 countdown timer
func (e *Engine) startCountdown() {
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
	}

	// Create new stop channel for this countdown instance
	e.countdownStopCh = make(chan struct{})
	e.countdownTimer = time.NewTicker(1 * time.Second)

	// Capture references locally
	ticker := e.countdownTimer
	stopCh := e.countdownStopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				// #151: the locked section is extracted into
				// processCountdownTick so the mutex is released via `defer`
				// on every exit path (including a panic) BEFORE
				// recoverBackgroundPanic runs — a recover() posed around
				// this whole case instead would still catch the panic, but
				// with e.mu left locked forever, freezing the engine on the
				// next Lock() (worse than the crash it replaces).
				func() {
					defer recoverBackgroundPanic("countdown timer")
					active, countdownTime := e.processCountdownTick()
					if !active {
						return
					}

					if e.OnCountdownTick != nil {
						e.OnCountdownTick(countdownTime)
					}

					if countdownTime <= 0 {
						// Countdown finished, start the actual game
						e.actualStart()
					}
				}()
			case <-stopCh:
				return
			}
		}
	}()
}

// processCountdownTick applies one countdown tick under lock and returns
// whether the countdown was actually active (phase == PhaseCountdown) along
// with the new CountdownTime. Extracted from startCountdown's goroutine
// (#151) so `defer e.mu.Unlock()` guarantees the mutex is released even if
// this panics — see recoverBackgroundPanic's doc comment.
func (e *Engine) processCountdownTick() (active bool, countdownTime int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	callTestInjectPanic("countdown")

	if e.state.Phase != PhaseCountdown {
		return false, 0
	}

	e.state.CountdownTime--
	return true, e.state.CountdownTime
}

// actualStart is called after countdown finishes to start the actual game
func (e *Engine) actualStart() {
	e.mu.Lock()

	// Stop countdown timer
	if e.countdownStopCh != nil {
		select {
		case <-e.countdownStopCh:
			// Already closed
		default:
			close(e.countdownStopCh)
		}
		e.countdownStopCh = nil
	}
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
		e.countdownTimer = nil
	}

	e.state.Phase = PhaseStarted
	e.state.CountdownTime = 0
	e.state.GameTime = time.Now().UnixMicro()

	e.setQuestionStatus(StatusStarted)

	// Clear Memory game state for fresh start
	e.state.MemoryFlippedCards = nil
	e.state.MemoryMatchedPairs = nil
	e.state.MemoryErrors = 0

	// #172 A1: the automatic Memory-participant catch-up that used to live here
	// (picking teams via Go map iteration — non-deterministic order) has been
	// removed. It is now structurally impossible to reach this point without a
	// conform selection already in place: entering STARTED requires READY
	// (Engine.Start's phase guard, #172 B4), and READY requires
	// participantsConform to hold (#172 B2) — which for MEMORY SOLO means
	// exactly one team already selected via SetMemoryParticipatingTeams, and
	// for MEMORY multi-team modes means at least two. See participantsConform.

	// Initialize MEMOTION card states for fresh start
	motionMemorizeDuration := 0
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		e.initMotionStateUnsafe()
		log.Printf("[Engine] MEMOTION: grid initialized with %d cards, subphase=%s",
			len(e.state.MotionCardStates), e.state.MotionSubPhase)
		motionMemorizeDuration = e.state.Question.MotionMemorizeDuration
	}

	// Reset QCM hints state for fresh start
	e.state.QcmInvalidated = nil

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.HintsAtBuzz = 0
	}

	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
	}

	log.Printf("[Engine] Countdown finished, game started with delay %d", e.pendingDelay)

	// RAFALE (v8.0.0, #107): init counters + draw the first question. A
	// no-op (isRafale=false) for every other question type. Must run
	// BEFORE startTimer() below sets CurrentTime — RAFALE reuses the exact
	// same global timer for its round countdown (contract §2.2), it does
	// not skip it the way MEMOTION does.
	rafaleStart := e.startRafaleRoundUnsafe()

	// Start main timer — MEMOTION uses per-card timers (StartMotionCardTimer), not a global one
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemotion {
		e.startTimer()
	}

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
	}

	// Start MEMORIZE timer after broadcast — must be called after unlock to avoid deadlock
	if motionMemorizeDuration > 0 {
		e.StartMotionMemorizeTimer(motionMemorizeDuration)
	}

	// RAFALE: broadcast the first answer (admin+anim) and start the
	// per-question ticker — after unlock, same reasoning as MEMORIZE above.
	e.finishRafaleRoundStart(rafaleStart)
}

// StartImmediate starts the game immediately without countdown (for tests)
func (e *Engine) StartImmediate(delay int) {
	e.mu.Lock()

	e.pendingDelay = delay
	e.state.Phase = PhaseStarted
	e.state.CountdownTime = 0
	e.state.GameTime = time.Now().UnixMicro()
	e.state.Delay = delay
	e.state.CurrentTime = delay

	e.setQuestionStatus(StatusStarted)

	// Reset Memory and QCM state
	e.state.MemoryFlippedCards = nil
	e.state.MemoryMatchedPairs = nil
	e.state.MemoryErrors = 0
	e.state.QcmInvalidated = nil

	// Initialize MEMOTION card states (mirrors actualStart — StartImmediate bypasses it)
	motionMemorizeDuration := 0
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		e.initMotionStateUnsafe()
		motionMemorizeDuration = e.state.Question.MotionMemorizeDuration
	}

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.HintsAtBuzz = 0
	}

	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
	}

	log.Printf("[Engine] Game started immediately (no countdown) with delay %d", delay)

	// RAFALE (v8.0.0, #107) — mirrors actualStart(); StartImmediate bypasses
	// actualStart entirely (used by tests), so it needs the same init call.
	rafaleStart := e.startRafaleRoundUnsafe()

	// Start main timer
	e.startTimer()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
	}

	e.finishRafaleRoundStart(rafaleStart)

	// Start MEMORIZE timer after broadcast — must be called after unlock to avoid deadlock
	if motionMemorizeDuration > 0 {
		e.StartMotionMemorizeTimer(motionMemorizeDuration)
	}
}

func (e *Engine) startTimer() {
	if e.timer != nil {
		e.timer.Stop()
	}

	// Create new stop channel for this timer instance
	e.stopCh = make(chan struct{})
	e.timer = time.NewTicker(1 * time.Second)

	// Capture references locally to avoid nil pointer issues
	ticker := e.timer
	stopCh := e.stopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				// #151: this is the most exposed of the 5 sites — the locked
				// section calls invalidateRandomWrongAnswer(), which mutates
				// e.state.QcmInvalidated and calls rand.Intn(). It is
				// extracted into processTimerTick (Lock + `defer` Unlock) so
				// a panic there still releases e.mu before
				// recoverBackgroundPanic runs. A recover() wrapped around
				// this whole case with the original manual Lock/Unlock would
				// instead leave e.mu locked forever — the engine freezes
				// silently on the next Lock(), worse than the crash it
				// replaces.
				func() {
					defer recoverBackgroundPanic("game timer tick")
					result := e.processTimerTick()
					if !result.active {
						return
					}

					if e.OnTimerTick != nil {
						e.OnTimerTick(result.currentTime)
					}

					// Call QCM hint callback outside of lock
					if result.qcmHintCallback != nil && result.invalidatedColor != "" {
						result.qcmHintCallback(result.invalidatedColor, result.remainingAnswers)
					}

					if result.currentTime <= 0 {
						e.Stop()
					}
				}()
			case <-stopCh:
				return
			}
		}
	}()
}

// timerTickResult carries the outcome of one locked timer-tick iteration
// (processTimerTick) out to the unlocked caller, which invokes callbacks
// and may call e.Stop() — mirrors the original inline logic in startTimer's
// goroutine before #151 extracted it for panic safety.
type timerTickResult struct {
	active           bool // phase was PhaseStarted; currentTime/QCM fields are meaningful
	currentTime      int
	qcmHintCallback  func(string, int)
	invalidatedColor string
	remainingAnswers int
}

// processTimerTick applies one game-timer tick under lock: decrements
// CurrentTime and, if a QCM hint threshold is crossed, invalidates a wrong
// answer. Extracted from startTimer's goroutine (#151) so `defer
// e.mu.Unlock()` guarantees the mutex is released even if this panics — see
// recoverBackgroundPanic's doc comment.
func (e *Engine) processTimerTick() timerTickResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	callTestInjectPanic("timer")

	if e.state.Phase != PhaseStarted {
		return timerTickResult{}
	}

	e.state.CurrentTime--
	currentTime := e.state.CurrentTime
	totalTime := e.state.Delay

	result := timerTickResult{active: true, currentTime: currentTime}

	if e.shouldTriggerQCMHint(currentTime, totalTime) {
		invalidatedColor, remainingAnswers := e.invalidateRandomWrongAnswer()
		if invalidatedColor != "" {
			result.qcmHintCallback = e.OnQCMHint
			result.invalidatedColor = invalidatedColor
			result.remainingAnswers = remainingAnswers
		}
	}

	return result
}

// qcmHintShouldTrigger is the QCM hint threshold decision, extracted pure
// (#185 C-B1) from the original question-only shouldTriggerQCMHint so both
// the question host (processTimerTick) and the MEMOTION card host
// (processMotionCardTick, a QCM-typed card) can share one implementation
// instead of two copies — contracts/question-types.md §10's agnosticity
// test anticipated exactly this move; see processMotionCardTick's doc
// comment for why this is the one authorized host change in this batch.
// Reads nothing but its parameters: hintsEnabled/t1Percent/t2Percent come
// from whichever host's own config (Question or MotionCard — both carry
// identically-shaped QCMHintsEnabled/QCMHintThreshold1/2 via TypedContent,
// #184 B-B1), currentTime/totalTime/invalidatedCount from that host's own
// timer and invalidated-answers count.
func qcmHintShouldTrigger(hintsEnabled bool, t1Percent, t2Percent float64, currentTime, totalTime, invalidatedCount int) bool {
	if !hintsEnabled {
		return false
	}

	// Threshold 1: % of total time remaining for first hint (default 25%)
	// Threshold 2: % of total time remaining for second hint (default 12.5%)
	if t1Percent <= 0 {
		t1Percent = 0.25 // Default 25%
	}
	if t2Percent <= 0 {
		t2Percent = 0.125 // Default 12.5%
	}
	threshold1 := int(float64(totalTime) * t1Percent)
	threshold2 := int(float64(totalTime) * t2Percent)

	// Safety constraints:
	// - Min 1s between hints: threshold1 - threshold2 >= 1
	// - Last hint >= 1s before end: threshold2 >= 1
	if threshold1 <= 0 || threshold2 < 1 || (threshold1-threshold2) < 1 {
		// Constraints not met, disable hints for this question
		return false
	}

	// Check if we hit threshold 1 (first hint)
	if currentTime == threshold1 && invalidatedCount == 0 {
		log.Printf("[Engine] QCM hint threshold 1 reached: time=%d, total=%d, threshold=%d", currentTime, totalTime, threshold1)
		return true
	}

	// Check if we hit threshold 2 (second hint)
	if currentTime == threshold2 && invalidatedCount == 1 {
		log.Printf("[Engine] QCM hint threshold 2 reached: time=%d, total=%d, threshold=%d", currentTime, totalTime, threshold2)
		return true
	}

	return false
}

// shouldTriggerQCMHint checks if a QCM hint should be triggered at the
// current time for the top-level question. Thin wrapper around
// qcmHintShouldTrigger (#185 C-B1) — behavior byte-for-byte identical to
// before the extraction. Must be called with lock held.
func (e *Engine) shouldTriggerQCMHint(currentTime, totalTime int) bool {
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeQCM {
		return false
	}
	return qcmHintShouldTrigger(
		e.state.Question.QCMHintsEnabled,
		e.state.Question.QCMHintThreshold1,
		e.state.Question.QCMHintThreshold2,
		currentTime, totalTime, len(e.state.QcmInvalidated),
	)
}

// qcmInvalidateRandomWrongAnswer picks a random not-yet-invalidated wrong
// QCM answer, extracted pure (#185 C-B1) from the original question-only
// invalidateRandomWrongAnswer — see qcmHintShouldTrigger's doc comment for
// why. Unlike the method it replaces, this does NOT mutate any state: it
// returns the color to invalidate ("" if every wrong answer is already
// invalidated) and the resulting remaining-valid-answers count; the caller
// is responsible for appending color to its own invalidated-answers slice
// (QcmInvalidated for the question host, MEMOTION_ACTIVE.STATE.QCM_INVALIDATED
// for the card host — contract §5.3, two different slices, same shape).
func qcmInvalidateRandomWrongAnswer(answers *QCMAnswers, correct string, invalidated []string) (color string, remaining int) {
	if answers == nil {
		return "", 0
	}

	allColors := []string{"RED", "GREEN", "YELLOW", "BLUE"}

	// Find wrong answers that haven't been invalidated yet
	var availableWrongAnswers []string
	for _, c := range allColors {
		if c == correct {
			continue // Skip correct answer
		}
		isInvalidated := false
		for _, inv := range invalidated {
			if inv == c {
				isInvalidated = true
				break
			}
		}
		if !isInvalidated {
			availableWrongAnswers = append(availableWrongAnswers, c)
		}
	}

	if len(availableWrongAnswers) == 0 {
		return "", 4 - len(invalidated)
	}

	// Pick a random wrong answer to invalidate
	randomIndex := rand.Intn(len(availableWrongAnswers))
	color = availableWrongAnswers[randomIndex]

	// Calculate remaining valid answers (4 total - invalidated count, +1 for
	// the one about to be appended by the caller)
	remaining = 4 - (len(invalidated) + 1)

	log.Printf("[Engine] QCM hint: invalidated %s, remaining answers: %d", color, remaining)
	return color, remaining
}

// invalidateRandomWrongAnswer invalidates a random wrong QCM answer for the
// top-level question, mutating e.state.QcmInvalidated. Thin wrapper around
// qcmInvalidateRandomWrongAnswer (#185 C-B1) — behavior byte-for-byte
// identical to before the extraction. Must be called with lock held.
// Returns the invalidated color and the number of remaining valid answers.
func (e *Engine) invalidateRandomWrongAnswer() (string, int) {
	if e.state.Question == nil {
		return "", 0
	}
	color, remaining := qcmInvalidateRandomWrongAnswer(e.state.Question.QCMAnswers, e.state.Question.QCMCorrect, e.state.QcmInvalidated)
	if color != "" {
		e.state.QcmInvalidated = append(e.state.QcmInvalidated, color)
	}
	return color, remaining
}

// Stop ends the game round
func (e *Engine) Stop() {
	e.mu.Lock()
	e.stopUnsafe()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStopped)
	}
}

// stopUnsafe is Stop()'s locked core — caller must hold e.mu (write lock).
// Extracted (v8.0.0, #107) so RAFALE's pool-exhausted/RAFALE_MAX_QUESTIONS
// round-end paths (advanceRafaleUnsafe/startRafaleRoundUnsafe, themselves
// invoked from an already-locked section) can reuse the exact same
// termination semantics as a manual STOP or the global timer's own expiry,
// without releasing and re-acquiring e.mu mid-transition — which would open
// a window for a second concurrent action (another VALIDATE, a manual STOP)
// to interleave and observe a half-transitioned state.
func (e *Engine) stopUnsafe() {
	// Signal countdown timer goroutine to stop (if running)
	if e.countdownStopCh != nil {
		select {
		case <-e.countdownStopCh:
			// Already closed
		default:
			close(e.countdownStopCh)
		}
		e.countdownStopCh = nil
	}
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
		e.countdownTimer = nil
	}

	// Signal main timer goroutine to stop
	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}

	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	e.state.Phase = PhaseStopped
	e.state.CurrentTime = 0
	e.state.CountdownTime = 0

	e.setQuestionStatus(StatusStopped)

	// RAFALE (#107, contract §7.1/§24): a round in progress ends here too,
	// whichever of the 4 conditions triggered this Stop (manual STOP, the
	// global round timer reaching 0 — both reach this function unchanged —
	// or the pool-exhausted/cap-reached paths, which call stopUnsafe
	// directly from advanceRafaleUnsafe/startRafaleRoundUnsafe). The
	// question ticker is stopped and the sub-phase moves to ROUND_END so
	// the animateur can attribute points (§6.2). Guarded on RafaleSubPhase
	// == QUESTION so a round that already reached ROUND_END by another path
	// (e.g. startRafaleRoundUnsafe's own pool-empty-at-start branch, which
	// sets ROUND_END itself before calling Stop()) isn't touched twice.
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeRafale && e.state.RafaleSubPhase == RafaleSubPhaseQuestion {
		e.stopRafaleQuestionTimerUnsafe()
		e.state.RafaleSubPhase = RafaleSubPhaseRoundEnd
	}

	log.Printf("[Engine] Game stopped")
}

// ---------------------------------------------------------------------------
// RAFALE — moteur solo (v8.0.0, #107, contracts/rafale.md §2/§6/§7/§8.1).
//
// Scope of this block: the round MACHINERY (draw → judge → advance → end),
// shared unchanged by every RAFALE_MODE. Team rotation and the 4 modes'
// distinct counter/rotation policies (CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE/
// MAILLON_FAIBLE, contract §3.4/§6.1) are Phase 3 (#199) — until
// RAFALE_SET_TEAMS (also Phase 3) sets GameState.RafaleCurrentTeam, it stays
// "" and the counter-increment below is a documented, forward-compatible
// no-op: this batch's SOLO round runs end-to-end (draw/timer/judge/advance/
// end) without a participating team ever being required to do so.
// ---------------------------------------------------------------------------

// ErrRafaleNotInQuestion is returned by RafaleValidate/RafaleInvalidate when
// the engine isn't currently in a live RAFALE question (wrong phase, wrong
// question type, or RafaleSubPhase != QUESTION — e.g. already ROUND_END, or
// a race against the question timer's own expiry).
var ErrRafaleNotInQuestion = errors.New("rafale_not_in_question")

// StartRafaleQuestionTimer starts the per-question countdown ticker for a
// RAFALE round (contract §2.2's "seul mécanisme réellement nouveau") — runs
// CONCURRENTLY with the round's own global timer, see rafaleQuestionTimer's
// field doc comment for why this needs its own fields. Mirrors
// StartMotionCardTimer's shape/discipline (defer e.mu.Unlock() in the tick,
// recoverBackgroundPanic, callbacks fired outside the lock).
func (e *Engine) StartRafaleQuestionTimer(seconds int) {
	e.mu.Lock()

	if seconds <= 0 {
		log.Printf("[Engine] RAFALE StartRafaleQuestionTimer: seconds<=0, no timer started")
		e.mu.Unlock()
		return
	}

	e.stopRafaleQuestionTimerUnsafe() // defensive — a fresh Start must never leak a prior goroutine

	e.state.RafaleQuestionTime = seconds

	e.rafaleQuestionStopCh = make(chan struct{})
	e.rafaleQuestionTimer = time.NewTicker(1 * time.Second)

	ticker := e.rafaleQuestionTimer
	stopCh := e.rafaleQuestionStopCh

	log.Printf("[Engine] RAFALE StartRafaleQuestionTimer: seconds=%d", seconds)
	e.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				func() {
					defer recoverBackgroundPanic("RAFALE question timer")
					result := e.processRafaleQuestionTick()

					if result.guardFailed {
						ticker.Stop()
						return
					}
					if result.paused {
						// Mirror processTimerTick's own pause handling —
						// skip this tick, keep the ticker running so it
						// resumes transparently on Continue().
						return
					}
					if !result.expired {
						if e.OnRafaleQuestionTick != nil {
							e.OnRafaleQuestionTick(result.questionTime)
						}
						return
					}

					// Expired — contract §6.1 "identique à réponse
					// invalide": already judged (as incorrect) and
					// advanced/ended under lock by processRafaleQuestionTick
					// itself. This ticker's job is done either way.
					ticker.Stop()
					e.fireRafaleAdvanceCallbacks(result.advance)
				}()
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// StopRafaleQuestionTimer stops the RAFALE per-question ticker, if running.
// Public counterpart to StopMotionCardTimer.
func (e *Engine) StopRafaleQuestionTimer() {
	e.mu.Lock()
	e.stopRafaleQuestionTimerUnsafe()
	e.mu.Unlock()
}

// stopRafaleQuestionTimerUnsafe is StopRafaleQuestionTimer's locked core —
// caller must hold e.mu (write lock). Idempotent — safe to call when no
// ticker is running.
func (e *Engine) stopRafaleQuestionTimerUnsafe() {
	if e.rafaleQuestionStopCh != nil {
		select {
		case <-e.rafaleQuestionStopCh:
			// Already closed
		default:
			close(e.rafaleQuestionStopCh)
		}
		e.rafaleQuestionStopCh = nil
	}
	if e.rafaleQuestionTimer != nil {
		e.rafaleQuestionTimer.Stop()
		e.rafaleQuestionTimer = nil
	}
}

// rafaleQuestionTickResult carries the outcome of one locked RAFALE
// question-timer tick (processRafaleQuestionTick) out to the unlocked
// caller — mirrors motionCardTickResult/timerTickResult's "compute under
// lock, act after unlock" discipline (#151).
type rafaleQuestionTickResult struct {
	guardFailed  bool // no longer a live RAFALE question at all — caller stops the ticker for good
	paused       bool // Phase != STARTED — caller skips this tick but keeps the ticker running
	questionTime int  // meaningful when !guardFailed && !paused && !expired
	expired      bool // hit 0 this tick; advance already computed under this same lock
	advance      rafaleAdvanceResult
}

// processRafaleQuestionTick applies one RAFALE question-timer tick under
// lock. Extracted (like every other tick handler in this file, #151) so
// `defer e.mu.Unlock()` guarantees the mutex is released even on a panic.
func (e *Engine) processRafaleQuestionTick() rafaleQuestionTickResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Question == nil || e.state.Question.Type != QuestionTypeRafale || e.state.RafaleSubPhase != RafaleSubPhaseQuestion {
		return rafaleQuestionTickResult{guardFailed: true}
	}
	if e.state.Phase != PhaseStarted {
		return rafaleQuestionTickResult{paused: true}
	}

	e.state.RafaleQuestionTime--
	qt := e.state.RafaleQuestionTime
	if qt > 0 {
		return rafaleQuestionTickResult{questionTime: qt}
	}

	// Expired — judge as incorrect (contract §6.1) and advance/end the
	// round in the SAME locked section (advanceRafaleUnsafe assumes e.mu
	// is already held — no re-entrant Lock here).
	advance := e.advanceRafaleUnsafe(false)
	return rafaleQuestionTickResult{expired: true, advance: advance}
}

// rafaleAdvanceResult carries the outcome of one RAFALE question-advance
// decision (RAFALE_VALIDATE, RAFALE_INVALIDATE, or a question-timer expiry —
// contract §6.1, all three funnel through advanceRafaleUnsafe) out to the
// unlocked caller (fireRafaleAdvanceCallbacks).
type rafaleAdvanceResult struct {
	guardFailed bool // not in an active RAFALE question — caller does nothing (ErrRafaleNotInQuestion)
	roundEnded  bool // pool exhausted or RAFALE_MAX_QUESTIONS reached — RafaleSubPhase is now ROUND_END, Phase is now STOPPED (stopUnsafe already ran)
	// nextQuestionID/nextAnswer/nextQuestionTime are set when !roundEnded —
	// the caller must broadcast RAFALE_ANSWER and (re)start the question
	// ticker.
	nextQuestionID   string
	nextAnswer       string
	nextQuestionTime int
}

// advanceRafaleUnsafe judges the current RAFALE question (correct==true for
// RAFALE_VALIDATE, false for RAFALE_INVALIDATE or a question-timer expiry)
// and moves the round to its next question, or ends it (ROUND_END) if the
// pool is exhausted or the question cap is reached. Caller must hold e.mu
// (write lock).
func (e *Engine) advanceRafaleUnsafe(correct bool) rafaleAdvanceResult {
	if e.state.Phase != PhaseStarted || e.state.Question == nil ||
		e.state.Question.Type != QuestionTypeRafale || e.state.RafaleSubPhase != RafaleSubPhaseQuestion {
		return rafaleAdvanceResult{guardFailed: true}
	}

	e.stopRafaleQuestionTimerUnsafe()

	// The 4 modes (v8.0.0, #199, contract §3.4/§6.1) — a question-timer
	// expiry (correct==false) is deliberately routed through the exact same
	// switch as RAFALE_INVALIDATE ("identique à réponse invalide", §6.1),
	// never a separate branch. RafaleCurrentTeam=="" (RAFALE_SET_TEAMS never
	// called — e.g. a solo round with no team concept in play at all) makes
	// every branch below a no-op on the counter/rotation, matching this
	// batch's predecessor behavior exactly.
	team := e.state.RafaleCurrentTeam
	mode := e.state.Question.RafaleMode
	if mode == "" {
		mode = string(RafaleModeSolo)
	}
	switch mode {
	case string(RafaleModeSolo):
		// "Aucune rotation" (§3.4) — counter[team]++ on correct, nothing on
		// incorrect/timeout (§6.1's "—" for every SOLO cell besides the
		// first row).
		if correct && team != "" {
			e.state.RafaleTeamCounters[team]++
		}
	case string(RafaleModeChacunSonTour):
		// Rotates after EVERY question, regardless of outcome.
		if correct && team != "" {
			e.state.RafaleTeamCounters[team]++
		}
		e.rotateRafaleTeam()
	case string(RafaleModeTantQueJeGagne):
		// Correct answer keeps the hand (no rotation); incorrect/timeout
		// rotates.
		if correct {
			if team != "" {
				e.state.RafaleTeamCounters[team]++
			}
		} else {
			e.rotateRafaleTeam()
		}
	case string(RafaleModeMaillonFaible):
		// Like CHACUN_SON_TOUR (rotates every question either way), but an
		// incorrect answer/timeout also resets THIS team's running counter
		// to 0 — the best value it reached is kept separately in
		// RafaleTeamBest, read by the animateur's point-attribution UI
		// (§6.2), never overwritten by the reset.
		if correct {
			if team != "" {
				e.state.RafaleTeamCounters[team]++
				if e.state.RafaleTeamCounters[team] > e.state.RafaleTeamBest[team] {
					e.state.RafaleTeamBest[team] = e.state.RafaleTeamCounters[team]
				}
			}
		} else if team != "" {
			e.state.RafaleTeamCounters[team] = 0
		}
		e.rotateRafaleTeam()
	default:
		// Unknown mode string — treat as SOLO (same "absent ⇒ default"
		// discipline as mode=="" above), never panics or silently drops the
		// question.
		if correct && team != "" {
			e.state.RafaleTeamCounters[team]++
		}
	}

	// Hard cap (contract §7.2), default/max 100.
	maxQuestions := 100
	if e.state.Question.RafaleMaxQuestions > 0 && e.state.Question.RafaleMaxQuestions < 100 {
		maxQuestions = e.state.Question.RafaleMaxQuestions
	}
	if e.state.RafaleAskedCount >= maxQuestions {
		log.Printf("[Engine] RAFALE: RAFALE_MAX_QUESTIONS (%d) reached, ending round", maxQuestions)
		e.stopUnsafe()
		return rafaleAdvanceResult{roundEnded: true}
	}

	drawn, err := e.drawRafaleQuestionUnsafe(e.state.Question.RafaleCategories, e.state.Question.RafaleDifficulty)
	if err != nil {
		// Pool exhausted mid-round (contract §7.1) — never reproposes a
		// question already seen; ends the round instead.
		log.Printf("[Engine] RAFALE: pool exhausted mid-round, ending round")
		e.state.RafaleExhausted = true
		e.stopUnsafe()
		return rafaleAdvanceResult{roundEnded: true}
	}

	e.state.RafaleCurrentQuestion = RafaleCurrent{
		ID: drawn.ID, Question: drawn.Question,
		Category: string(drawn.Category), Difficulty: drawn.Difficulty,
	}
	e.state.RafaleAskedCount++
	e.state.RafalePoolRemaining = len(e.rafalePoolUnsafe(e.state.Question.RafaleCategories, e.state.Question.RafaleDifficulty))

	questionTime := e.state.Question.RafaleQuestionTime
	if questionTime <= 0 {
		questionTime = 3
	}

	return rafaleAdvanceResult{
		nextQuestionID: drawn.ID, nextAnswer: drawn.Answer, nextQuestionTime: questionTime,
	}
}

// rotateRafaleTeam advances RafaleCurrentTeam to the next team in
// RafaleParticipatingTeams (circular) via the shared rotateTeam helper
// (team_rotation.go, #199 task 28) — RAFALE's counterpart to
// rotateMotionTeam. A no-op when no teams are participating (RAFALE_SET_TEAMS
// never called, or called with an empty list) — matches rotateMotionTeam's
// own early-return. Caller must hold e.mu (write lock).
func (e *Engine) rotateRafaleTeam() {
	if len(e.state.RafaleParticipatingTeams) == 0 {
		return
	}

	prev := e.state.RafaleCurrentTeam
	next, color := rotateTeam(e.state.RafaleParticipatingTeams, e.state.RafaleCurrentTeam, e.data.Teams)
	e.state.RafaleCurrentTeam = next
	e.state.RafaleCurrentTeamColor = color

	log.Printf("[Engine] RAFALE team rotated: %s → %s", prev, next)
}

// fireRafaleAdvanceCallbacks performs the UNLOCKED side effects of one
// RAFALE advance — shared by RafaleValidate/RafaleInvalidate and
// StartRafaleQuestionTimer's own expiry handling so both paths broadcast
// identically (contract §6.1's "identique à réponse invalide" must actually
// look identical on the wire too). Must be called with e.mu NOT held.
func (e *Engine) fireRafaleAdvanceCallbacks(result rafaleAdvanceResult) {
	if result.roundEnded {
		if e.OnStateChange != nil {
			e.OnStateChange(PhaseStopped)
		}
		return
	}
	if e.OnRafaleAnswer != nil {
		e.OnRafaleAnswer(result.nextQuestionID, result.nextAnswer)
	}
	e.StartRafaleQuestionTimer(result.nextQuestionTime)
	// #199 task 36: LEDs are refreshed on EVERY advance, not just an actual
	// rotation — contract §8.3 says "à chaque... changement de question",
	// and TANT_QUE_JE_GAGNE/SOLO can advance without RafaleCurrentTeam
	// changing at all. The consumer (cmd/server/main.go) re-reads live
	// state, same as OnStateChange below — no data passed here.
	if e.OnRafaleTeamsChanged != nil {
		e.OnRafaleTeamsChanged()
	}
	if e.OnStateChange != nil {
		// Not a real phase transition (Phase stays STARTED) — OnStateChange
		// is reused here purely as "please rebroadcast full state"
		// (broadcastGameState re-reads live state and ignores this
		// argument, per its own doc comment). This is what delivers the
		// new RAFALE_CURRENT_QUESTION/RAFALE_ASKED_COUNT/
		// RAFALE_POOL_REMAINING to clients for the question-timer-expiry
		// path, which — unlike RAFALE_VALIDATE/INVALIDATE — has no WS
		// handler of its own to call broadcastUpdate() afterward.
		e.OnStateChange(PhaseStarted)
	}
}

// RafaleValidate processes RAFALE_VALIDATE (contract §5.1): the current
// question's answer was judged correct. Advances to the next question or
// ends the round — see advanceRafaleUnsafe.
func (e *Engine) RafaleValidate() error {
	e.mu.Lock()
	result := e.advanceRafaleUnsafe(true)
	e.mu.Unlock()

	if result.guardFailed {
		return ErrRafaleNotInQuestion
	}
	e.fireRafaleAdvanceCallbacks(result)
	return nil
}

// RafaleInvalidate processes RAFALE_INVALIDATE (contract §5.1): the current
// question's answer was judged incorrect. Same advance/end logic as
// RafaleValidate, just without the counter increment (correct=false).
func (e *Engine) RafaleInvalidate() error {
	e.mu.Lock()
	result := e.advanceRafaleUnsafe(false)
	e.mu.Unlock()

	if result.guardFailed {
		return ErrRafaleNotInQuestion
	}
	e.fireRafaleAdvanceCallbacks(result)
	return nil
}

// rafaleRoundStartResult carries the outcome of initializing a fresh RAFALE
// round (startRafaleRoundUnsafe) out to the unlocked caller (actualStart/
// StartImmediate) — same "compute under lock, act after unlock" discipline
// as actualStart's own motionMemorizeDuration.
type rafaleRoundStartResult struct {
	isRafale     bool // false ⇒ not a RAFALE question, caller does nothing further
	roundEnded   bool // pool was already empty at round start (contract §7.1) — caller must call Stop()
	questionID   string
	answer       string
	questionTime int
}

// startRafaleRoundUnsafe initializes a fresh RAFALE round: resets the
// per-round counters and draws the first question. Caller must hold e.mu
// (write lock) — called from actualStart()/StartImmediate() while already
// locked, mirroring the MEMOTION grid-init call in the same functions.
func (e *Engine) startRafaleRoundUnsafe() rafaleRoundStartResult {
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeRafale {
		return rafaleRoundStartResult{}
	}

	e.state.RafaleAskedCount = 0
	e.state.RafaleExhausted = false
	e.state.RafaleTeamCounters = map[string]int{}
	e.state.RafaleTeamBest = map[string]int{}

	drawn, err := e.drawRafaleQuestionUnsafe(e.state.Question.RafaleCategories, e.state.Question.RafaleDifficulty)
	if err != nil {
		// contract §7.1: never reproposes anything, ends immediately. Should
		// not normally happen — §7.2's pre-round alert blocks START when
		// available==0 — but a race (reservoir edited concurrently) is
		// always possible; this is the safety net, not the primary defense.
		log.Printf("[Engine] RAFALE: pool already empty at round start, ending immediately")
		e.state.RafaleExhausted = true
		e.state.RafaleSubPhase = RafaleSubPhaseRoundEnd
		e.state.RafalePoolRemaining = 0
		return rafaleRoundStartResult{isRafale: true, roundEnded: true}
	}

	e.state.RafaleSubPhase = RafaleSubPhaseQuestion
	e.state.RafaleCurrentQuestion = RafaleCurrent{
		ID: drawn.ID, Question: drawn.Question,
		Category: string(drawn.Category), Difficulty: drawn.Difficulty,
	}
	e.state.RafaleAskedCount = 1
	e.state.RafalePoolRemaining = len(e.rafalePoolUnsafe(e.state.Question.RafaleCategories, e.state.Question.RafaleDifficulty))

	questionTime := e.state.Question.RafaleQuestionTime
	if questionTime <= 0 {
		questionTime = 3
	}

	return rafaleRoundStartResult{
		isRafale: true, questionID: drawn.ID, answer: drawn.Answer, questionTime: questionTime,
	}
}

// finishRafaleRoundStart performs the UNLOCKED side effects of
// startRafaleRoundUnsafe — called from actualStart()/StartImmediate() after
// e.mu.Unlock() and after callback(PhaseStarted) has already fired (so the
// round-ended branch's own e.Stop() call, if taken, is a real, visible
// second transition — see its own comment for why that's an acceptable
// trade-off over a more tangled in-lock special case).
func (e *Engine) finishRafaleRoundStart(result rafaleRoundStartResult) {
	if !result.isRafale {
		return
	}
	if result.roundEnded {
		// Pool was already empty — the round is over before it visibly
		// started. Started's own startTimer() call already ran (harmless:
		// this stops it again a moment later) — reusing the public Stop()
		// here, rather than a special in-lock case in actualStart, keeps
		// that rare edge case out of the hot path.
		e.Stop()
		return
	}
	if e.OnRafaleAnswer != nil {
		e.OnRafaleAnswer(result.questionID, result.answer)
	}
	e.StartRafaleQuestionTimer(result.questionTime)
	// #199 task 36 — initial LED grid for the round (active team, if any
	// RAFALE_SET_TEAMS was called before START; all-off in SOLO or if no
	// teams were set).
	if e.OnRafaleTeamsChanged != nil {
		e.OnRafaleTeamsChanged()
	}
}

// Pause pauses the game (single buzzer)
func (e *Engine) Pause() {
	e.mu.Lock()

	e.state.Phase = PhasePaused

	e.setQuestionStatus(StatusPaused)

	log.Printf("[Engine] Game paused")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhasePaused)
	}
}

// PauseAll pauses for all buzzers
func (e *Engine) PauseAll() {
	e.Pause()
}

// Continue resumes the game
func (e *Engine) Continue() {
	e.mu.Lock()

	e.state.Phase = PhaseStarted

	e.setQuestionStatus(StatusStarted)

	log.Printf("[Engine] Game continued")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
	}
}

// Reveal shows the answer
func (e *Engine) Reveal() string {
	e.mu.Lock()

	// Allow reveal from STOPPED or PAUSED
	if e.state.Phase != PhaseStopped && e.state.Phase != PhasePaused {
		log.Printf("[Engine] Cannot reveal from phase %s (must be STOPPED or PAUSED)", e.state.Phase)
		e.mu.Unlock()
		return ""
	}

	// Stop timer if revealing from PAUSED
	if e.state.Phase == PhasePaused {
		// Signal countdown timer goroutine to stop (if running)
		if e.countdownStopCh != nil {
			select {
			case <-e.countdownStopCh:
				// Already closed
			default:
				close(e.countdownStopCh)
			}
			e.countdownStopCh = nil
		}
		if e.countdownTimer != nil {
			e.countdownTimer.Stop()
			e.countdownTimer = nil
		}

		// Signal main timer goroutine to stop
		if e.stopCh != nil {
			select {
			case <-e.stopCh:
				// Already closed
			default:
				close(e.stopCh)
			}
			e.stopCh = nil
		}

		if e.timer != nil {
			e.timer.Stop()
			e.timer = nil
		}
	}

	e.state.Phase = PhaseRevealed

	var answer string
	if e.state.Question != nil {
		e.setQuestionStatus(StatusRevealed)
		answer = e.state.Question.Answer
		log.Printf("[Engine] Answer revealed")
	}

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseRevealed)
	}

	return answer
}

// ProcessButtonPress handles a button press from a buzzer
func (e *Engine) ProcessButtonPress(bumperID string, pressTime int64, button string) {
	e.mu.Lock()

	if e.state.Phase != PhaseStarted {
		log.Printf("[Engine] Ignoring button press, game not started")
		e.mu.Unlock()
		return
	}

	// Ignore buzz for MEMORY questions - admin controls the game
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemory {
		log.Printf("[Engine] Ignoring buzz for MEMORY question from %s", bumperID)
		e.mu.Unlock()
		return
	}

	// Ignore buzz for MEMOTION questions - admin controls the card selection
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		log.Printf("[Engine] Ignoring buzz for MEMOTION question from %s", bumperID)
		e.mu.Unlock()
		return
	}

	// Ignore buzz for RAFALE questions (v8.0.0, #107, contract §8.1) — no
	// player interaction during RAFALE, buzzer presses are always ignored.
	// The answer is judged by admin/anim via RAFALE_VALIDATE/INVALIDATE.
	// This guard is deliberately about INPUT only — it does not touch LED
	// output, which is piloted separately (contract §8.3, Phase 3 tasks
	// 35-36): a RAFALE buzzer's LED can be lit to show its team is active
	// even though pressing it here still has zero effect on game state.
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeRafale {
		log.Printf("[Engine] Ignoring buzz for RAFALE question from %s", bumperID)
		e.mu.Unlock()
		return
	}

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		log.Printf("[Engine] Unknown bumper: %s", bumperID)
		e.mu.Unlock()
		return
	}

	// Check if bumper already pressed
	if bumper.Time > 0 {
		log.Printf("[Engine] Bumper %s already pressed", bumperID)
		e.mu.Unlock()
		return
	}

	teamID := bumper.Team
	if teamID == "" {
		// Bumper has no team - allow individual press
		bumper.Time = pressTime
		bumper.Button = button
		bumper.Status = "PAUSE"
		e.mu.Unlock()
		return
	}

	team, ok := e.data.Teams[teamID]
	if !ok {
		e.mu.Unlock()
		return
	}

	// Phase 3: QCM VPlayer invalidation logic
	// For QCM questions only, if the team has a VPlayer and this is a physical buzzer,
	// ignore the buzz (VPlayer replaces physical buzzers for QCM)
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeQCM {
		if !bumper.IsVPlayer && e.teamHasVPlayerUnsafe(teamID) {
			log.Printf("[Engine] Buzz ignored: team %s has VPlayer, physical bumper %s cannot buzz on QCM", teamID, bumperID)
			e.mu.Unlock()
			return
		}
	}

	// Check if team already has a press - only ONE player per team can buzz
	if team.Time > 0 {
		log.Printf("[Engine] Team %s already buzzed, ignoring bumper %s", teamID, bumperID)
		e.mu.Unlock()
		return
	}

	// Record the press for both bumper and team
	bumper.Time = pressTime
	bumper.Button = button
	bumper.Status = "PAUSE"

	// For QCM questions, map button to AnswerColor (A=RED, B=GREEN, C=YELLOW, D=BLUE)
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeQCM {
		buttonToColor := map[string]AnswerColor{
			"A": AnswerColorRed,
			"B": AnswerColorGreen,
			"C": AnswerColorYellow,
			"D": AnswerColorBlue,
		}
		if color, ok := buttonToColor[button]; ok {
			bumper.AnswerColor = color
		}
	}

	// Store the number of QCM hints at buzz time for per-player penalty calculation
	bumper.HintsAtBuzz = len(e.state.QcmInvalidated)

	team.Time = pressTime
	team.Bumper = bumperID
	team.Status = "PAUSE"

	log.Printf("[Engine] Button press: bumper=%s, team=%s, button=%s, time=%d",
		bumperID, teamID, button, pressTime)

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnBuzzerPress
	e.mu.Unlock()

	if callback != nil {
		callback(bumperID, teamID, pressTime, button)
	}
}

// teamHasVPlayerUnsafe checks if a team has at least one VPlayer (virtual player)
// Caller must hold lock
func (e *Engine) teamHasVPlayerUnsafe(teamID string) bool {
	for _, bumper := range e.data.Bumpers {
		if bumper.Team == teamID && bumper.IsVPlayer {
			return true
		}
	}
	return false
}

// UpdateScore updates bumper and team scores
func (e *Engine) UpdateScore(bumperID string, points int) (bumperScore, teamScore int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		return 0, 0
	}

	bumper.Score += points
	bumperScore = bumper.Score

	if bumper.Team != "" {
		if team, ok := e.data.Teams[bumper.Team]; ok {
			team.Score += points
			teamScore = team.Score
		}
	}

	log.Printf("[Engine] Score update: bumper=%s, points=%d, bumperScore=%d, teamScore=%d",
		bumperID, points, bumperScore, teamScore)

	return bumperScore, teamScore
}

// UpdateBumperScore updates the bumper score and recalculates team score
func (e *Engine) UpdateBumperScore(bumperID string, points int) int {
	e.mu.Lock()

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		log.Printf("[Engine] UpdateBumperScore: bumper not found: %s", bumperID)
		e.mu.Unlock()
		return 0
	}

	bumper.Score += points
	log.Printf("[Engine] Bumper score update: bumper=%s, points=%+d, newScore=%d",
		bumperID, points, bumper.Score)

	// Recalculate team score as sum of all bumpers in team
	if bumper.Team != "" {
		e.recalculateTeamScoreUnsafe(bumper.Team)
	}

	score := bumper.Score
	e.mu.Unlock()

	// Auto-save (scores are part of bumper/team data)
	safeGo("SaveBumpers", e.SaveBumpers)
	safeGo("SaveTeams", e.SaveTeams)

	return score
}

// UpdateTeamScore adds points directly to team's own TeamPoints (not bumpers)
func (e *Engine) UpdateTeamScore(teamName string, points int) int {
	e.mu.Lock()

	team, ok := e.data.Teams[teamName]
	if !ok {
		log.Printf("[Engine] UpdateTeamScore: team not found: %s", teamName)
		e.mu.Unlock()
		return 0
	}

	// Add points directly to TeamPoints (independent from bumper scores)
	team.TeamPoints += points
	log.Printf("[Engine] Team points update: team=%s, points=%+d, newTeamPoints=%d",
		teamName, points, team.TeamPoints)

	// Recalculate total team score (TeamPoints + bumper scores)
	e.recalculateTeamScoreUnsafe(teamName)

	score := team.Score
	e.mu.Unlock()

	// Auto-save teams to disk
	safeGo("SaveTeams", e.SaveTeams)

	return score
}

// recalculateTeamScoreUnsafe sets team score to TeamPoints + sum of bumper scores (caller must hold lock)
func (e *Engine) recalculateTeamScoreUnsafe(teamName string) {
	team, ok := e.data.Teams[teamName]
	if !ok {
		return
	}

	var bumperTotal int
	for _, bumper := range e.data.Bumpers {
		if bumper.Team == teamName {
			bumperTotal += bumper.Score
		}
	}

	team.Score = team.TeamPoints + bumperTotal
	log.Printf("[Engine] Team score recalculated: team=%s, teamPoints=%d, bumperTotal=%d, totalScore=%d",
		teamName, team.TeamPoints, bumperTotal, team.Score)
}

// RecalculateAllTeamScores recalculates scores for all teams based on bumper scores
func (e *Engine) RecalculateAllTeamScores() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for teamName := range e.data.Teams {
		e.recalculateTeamScoreUnsafe(teamName)
	}
	log.Printf("[Engine] All team scores recalculated")
}

// InitGame resets the game completely: scores, history, question statuses, and sets phase to NEW_GAME.
// This is the implementation of issue #66 — Bouton Start GAME.
// InitGame resets scores/history/statuses for a new game (NEW_GAME action) and
// purges the entire VJoueur roster. It returns the bumper IDs of the VJoueurs
// purged so the caller can notify each of them (PLAYER_EVICTED{GAME_RESET},
// #120) — the engine has no knowledge of WebSocket clients, so that
// notification is the caller's responsibility (main.go), keeping the existing
// engine/transport separation intact.
func (e *Engine) InitGame() []string {
	e.mu.Lock()

	// Reset all bumper scores/times; purge VJoueurs entirely (fix R1 follow-up
	// — product invariant: a new game always starts with a clean VJoueur
	// roster, there is no such thing as a "legacy" VJoueur from a prior
	// session). Physical buzzers are persistent hardware and are kept, just
	// with their score/time reset like before.
	var purgedVPlayerIDs []string
	for id, bumper := range e.data.Bumpers {
		if bumper.IsVirtual {
			delete(e.data.Bumpers, id)
			purgedVPlayerIDs = append(purgedVPlayerIDs, id)
			continue
		}
		bumper.Score = 0
		bumper.Time = 0
	}
	if len(purgedVPlayerIDs) > 0 {
		e.state.VirtualPlayerCount = e.countVirtualPlayersUnsafe()
		log.Printf("[Engine] InitGame: purged %d virtual player(s) — fresh VJoueur roster for the new game", len(purgedVPlayerIDs))
	}

	// Reset all team scores
	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
		team.Time = 0
	}

	// Clear history
	e.history = nil

	// Reset all question statuses
	e.questionStatuses = make(map[string]QuestionStatus)

	// Clear current question so admin UI doesn't show a stale selection
	e.state.Question = nil

	// Set phase to NEW_GAME
	e.state.Phase = PhaseNewGame

	// Reset MEMOTION state completely
	e.state.MotionSubPhase = ""
	e.state.MotionSelected = ""
	e.state.MotionCardStates = make(map[string]MotionCardState)
	e.state.MotionCardTeams = make(map[string]string)
	e.state.MotionCurrentTeam = ""
	e.state.MotionCurrentTeamColor = []int{}
	e.state.MotionActive = MotionActive{State: map[string]interface{}{}}
	e.state.MotionParticipatingTeams = []string{}

	// Reset ARDOISE answers (v5.6.0)
	e.state.ArdoiseAnswers = make(map[string]ArdoiseAnswer)

	// Reset RAFALE "already used" flags (#197, contract §3.2) — a fresh
	// game must be able to draw the whole reservoir again. The reservoir
	// itself (rafaleQuestions) is untouched: NEW_GAME resets game-play
	// state, never the editor's question bank.
	e.rafaleUsed = make(map[string]bool)

	// Reset RAFALE ephemeral GameState fields (#107, contract §4) — same
	// defense-in-depth as the MEMOTION/ARDOISE resets just above.
	e.state.RafaleSubPhase = RafaleSubPhaseNone
	e.state.RafaleCurrentQuestion = RafaleCurrent{}
	e.state.RafaleQuestionTime = 0
	e.state.RafaleTeamCounters = map[string]int{}
	e.state.RafaleTeamBest = map[string]int{}
	e.state.RafaleCurrentTeam = ""
	e.state.RafaleParticipatingTeams = []string{}
	e.state.RafaleCurrentTeamColor = []int{}
	e.state.RafaleAskedCount = 0
	e.state.RafalePoolRemaining = 0
	e.state.RafaleExhausted = false

	log.Printf("[Engine] Game initialized: scores, history, question statuses reset")
	e.mu.Unlock()

	// Persist async
	safeGo("SaveHistory", e.SaveHistory)
	safeGo("SaveTeams", e.SaveTeams)
	safeGo("SaveBumpers", e.SaveBumpers)
	safeGo("SaveStatuses", e.SaveStatuses)
	safeGo("SaveRafaleUsed", e.SaveRafaleUsed)

	return purgedVPlayerIDs
}

// SetQuizMeta sets the quiz metadata (name, theme, notes, and — since v6.0.0,
// #8 — populations/difficulties/language, plus objectives since v6.1.0,
// #137 Batch 2b) on the game state.
// This is the implementation of issue #67 — Change QUESTIONS en QUIZ.
//
// populations/difficulties are normalized to a non-nil, possibly-empty slice
// (never left nil) so GameState always serializes them as [] rather than
// null (contract game-state.md §"Aucun omitempty" — a null would break any
// client iterating without a guard).
func (e *Engine) SetQuizMeta(name, theme, notes string, populations, difficulties []string, language, objectives string) {
	e.mu.Lock()
	if populations == nil {
		populations = []string{}
	}
	if difficulties == nil {
		difficulties = []string{}
	}
	e.state.QuizName = name
	e.state.QuizTheme = theme
	e.state.QuizNotes = notes
	e.state.QuizPopulations = populations
	e.state.QuizDifficulties = difficulties
	e.state.QuizLanguage = language
	e.state.QuizObjectives = objectives
	e.mu.Unlock()

	log.Printf("[Engine] Quiz meta set: name=%q, theme=%q, populations=%v, difficulties=%v, language=%q, objectives_len=%d", name, theme, populations, difficulties, language, len(objectives))

	// #141 — persist synchronously, same rationale as SetTeams: this is a
	// low-frequency admin action (not a hot path), and firing it in a
	// background goroutine would let it race with a caller's own
	// SaveState()/LoadState() sequence.
	if err := e.SaveState(); err != nil {
		log.Printf("[Engine] Failed to persist game state after quiz meta change: %v", err)
	}
}

// quizHiddenFieldAllowedValues are the only values SetQuizDisplay accepts
// (contract game-state.md rule H2). OBJECTIVES is deliberately not a member
// (rule H3): the objective is never broadcast at all, so it can't be a
// "shown/hidden on TV" choice — accepting it here would suggest otherwise.
// NAME/NOTES are not members either (rule H4): not pilotable in this version.
var quizHiddenFieldAllowedValues = map[string]bool{
	"THEME": true, "POPULATIONS": true, "DIFFICULTIES": true, "LANGUAGE": true,
}

// SetQuizDisplay sets which QUIZ_* fields the admin chose to hide from the TV
// NEW_GAME screen (v6.1.0, #137 Batch 2b T1.8, contract game-state.md
// §"QUIZ_HIDDEN_FIELDS"). A dedicated setter, deliberately NOT an 8th
// SetQuizMeta parameter — that signature was just extended to 7 and the
// contract explicitly calls for a separate setter here.
//
// hidden is normalized to a non-nil, possibly-empty slice (rule H1 — same
// "never null" requirement as the other QUIZ_* arrays) and filtered against
// quizHiddenFieldAllowedValues: an unknown value is dropped and logged, never
// treated as an error (rule H2) — a newer client sending a label this build
// doesn't know yet must not fail the whole save.
func (e *Engine) SetQuizDisplay(hidden []string) {
	e.mu.Lock()
	filtered := make([]string, 0, len(hidden))
	for _, field := range hidden {
		if quizHiddenFieldAllowedValues[field] {
			filtered = append(filtered, field)
		} else {
			log.Printf("[Engine] SetQuizDisplay: ignoring unknown field %q", field)
		}
	}
	e.state.QuizHiddenFields = filtered
	e.mu.Unlock()

	log.Printf("[Engine] Quiz display set: hidden=%v", filtered)

	// #141 — persist synchronously, same rationale as SetQuizMeta above.
	if err := e.SaveState(); err != nil {
		log.Printf("[Engine] Failed to persist game state after quiz display change: %v", err)
	}
}

// RAZScores resets all scores to zero
func (e *Engine) RAZScores() {
	e.mu.Lock()

	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
		bumper.Time = 0
	}

	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
		team.Time = 0
	}

	// Clear history as well
	e.history = nil

	log.Printf("[Engine] All scores and history reset")
	e.mu.Unlock()

	// Save all data to disk
	safeGo("SaveHistory", e.SaveHistory)
	safeGo("SaveTeams", e.SaveTeams)
	safeGo("SaveBumpers", e.SaveBumpers)
}

// ClearBumpers removes all bumpers (keeps teams intact)
func (e *Engine) ClearBumpers() {
	e.mu.Lock()
	e.data.Bumpers = make(map[string]*Bumper)

	// Reset team references to bumpers
	for _, team := range e.data.Teams {
		team.Bumper = ""
		team.Time = 0
		team.Status = ""
		team.Ready = false
	}

	log.Printf("[Engine] All bumpers cleared and dissociated from teams")
	e.mu.Unlock()

	// Auto-save empty bumpers and updated teams
	safeGo("SaveBumpers", e.SaveBumpers)
	safeGo("SaveTeams", e.SaveTeams)
}

// ClearAll removes all teams and bumpers
func (e *Engine) ClearAll() {
	e.mu.Lock()
	e.data.Bumpers = make(map[string]*Bumper)
	e.data.Teams = make(map[string]*Team)
	log.Printf("[Engine] All teams and bumpers cleared")
	e.mu.Unlock()

	// Auto-save empty data
	safeGo("SaveTeams", e.SaveTeams)
	safeGo("SaveBumpers", e.SaveBumpers)
}

// SetPage sets the remote page
func (e *Engine) SetPage(page string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if page == "" || page == "null" {
		page = "GAME"
	}
	e.state.Page = page
}

// SetBackgrounds sets all backgrounds
func (e *Engine) SetBackgrounds(backgrounds []Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = backgrounds
}

// GetBackgrounds returns all backgrounds
func (e *Engine) GetBackgrounds() []Background {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Backgrounds
}

// SetNewGameBackgrounds sets all NEW_GAME screen backgrounds (v4.0.4)
func (e *Engine) SetNewGameBackgrounds(backgrounds []Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.NewGameBackgrounds = backgrounds
}

// GetNewGameBackgrounds returns all NEW_GAME screen backgrounds (v4.0.4)
func (e *Engine) GetNewGameBackgrounds() []Background {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.NewGameBackgrounds
}

// SetNetworkOnlyLocalhost sets whether the server has no non-loopback network interface active (v5.6.2)
func (e *Engine) SetNetworkOnlyLocalhost(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.NetworkOnlyLocalhost = v
}

// GetNetworkOnlyLocalhost returns whether the server has no non-loopback network interface active (v5.6.2)
func (e *Engine) GetNetworkOnlyLocalhost() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.NetworkOnlyLocalhost
}

// AddBackground adds a background
func (e *Engine) AddBackground(bg Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = append(e.state.Backgrounds, bg)
}

// RemoveBackground removes a background by path
func (e *Engine) RemoveBackground(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	newBgs := make([]Background, 0, len(e.state.Backgrounds))
	for _, bg := range e.state.Backgrounds {
		if bg.Path != path {
			newBgs = append(newBgs, bg)
		}
	}
	e.state.Backgrounds = newBgs
}

// ClearBackgrounds removes all backgrounds
func (e *Engine) ClearBackgrounds() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = nil
	e.state.CurrentBackgroundIndex = 0
}

// GetCurrentBackgroundIndex returns the current background index
func (e *Engine) GetCurrentBackgroundIndex() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.CurrentBackgroundIndex
}

// SetCurrentBackgroundIndex sets the current background index
func (e *Engine) SetCurrentBackgroundIndex(index int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Backgrounds) > 0 {
		e.state.CurrentBackgroundIndex = index % len(e.state.Backgrounds)
	} else {
		e.state.CurrentBackgroundIndex = 0
	}
}

// NextBackground advances to the next background and returns the new index
func (e *Engine) NextBackground() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Backgrounds) > 0 {
		e.state.CurrentBackgroundIndex = (e.state.CurrentBackgroundIndex + 1) % len(e.state.Backgrounds)
	}
	return e.state.CurrentBackgroundIndex
}

// GetCurrentBackgroundDuration returns the duration of the current background in seconds
func (e *Engine) GetCurrentBackgroundDuration() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.state.Backgrounds) == 0 {
		return 0
	}
	bg := e.state.Backgrounds[e.state.CurrentBackgroundIndex]
	if bg.Duration <= 0 {
		return 10 // Default 10 seconds
	}
	return bg.Duration
}

// GetGameJSON returns game state as JSON
func (e *Engine) GetGameJSON() json.RawMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	bumpers := e.data.Bumpers
	if bumpers == nil {
		bumpers = make(map[string]*Bumper)
	}
	teams := e.data.Teams
	if teams == nil {
		teams = make(map[string]*Team)
	}

	data := &GameData{
		Game:    &e.state,
		Teams:   teams,
		Bumpers: bumpers,
	}

	result, _ := json.Marshal(data)
	return result
}

// GetTeamsAndBumpersJSON returns teams and bumpers as JSON
func (e *Engine) GetTeamsAndBumpersJSON() json.RawMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, _ := json.Marshal(e.data)
	return result
}

// ForceReady forces transition to READY phase (debug function, skips PONG wait)
func (e *Engine) ForceReady() {
	e.mu.Lock()

	if e.state.Phase != PhasePrepare {
		log.Printf("[Engine] ForceReady: not in PREPARE phase, current: %s", e.state.Phase)
		e.mu.Unlock()
		return
	}

	// Mark all bumpers as ready
	for _, bumper := range e.data.Bumpers {
		bumper.Ready = true
	}

	// Mark all teams as ready
	for _, team := range e.data.Teams {
		team.Ready = true
	}

	// #172 B5 (arbitrage G): ForceReady only skips the PONG wait — it must not
	// also skip participant-selection conformity, or the original bug (a
	// non-conform MEMORY/MEMOTION round reaching STARTED) would return through
	// this debug/admin door. Bumpers/teams stay marked ready above so that, once
	// the admin selects a conform participant set, the PREPARE→READY reevaluation
	// (reevaluatePrepareReadyUnsafe, triggered by Set*ParticipatingTeams) fires
	// immediately without waiting on PONG again.
	if !participantsConform(e.state.Question, &e.state) {
		log.Printf("[Engine] FORCE_READY: buzzers marked ready, but participants do not conform — staying in PREPARE")
		e.mu.Unlock()
		return
	}

	e.state.Phase = PhaseReady
	e.setQuestionStatus(StatusReady)
	log.Printf("[Engine] FORCE_READY: transitioning to READY")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseReady)
	}
}

// IsGameStopped returns true if game is stopped
func (e *Engine) IsGameStopped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseStopped
}

// IsGamePrepare returns true if game is in prepare phase
func (e *Engine) IsGamePrepare() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhasePrepare
}

// IsGameReady returns true if game is ready
func (e *Engine) IsGameReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseReady
}

// IsGameStarted returns true if game is started
func (e *Engine) IsGameStarted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseStarted
}

// AddGameEvent adds an event to the history and saves to disk
func (e *Engine) AddGameEvent(event GameEvent) {
	e.mu.Lock()
	e.history = append(e.history, event)
	log.Printf("[Engine] Game event added: type=%s, winner=%s, points=%d",
		event.EventType, event.WinnerName, event.Points)
	e.mu.Unlock()

	// Save history to disk (non-blocking, async)
	safeGo("SaveHistory", e.SaveHistory)
}

// GetHistory returns the game event history
func (e *Engine) GetHistory() []GameEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make([]GameEvent, len(e.history))
	copy(result, e.history)
	return result
}

// ClearHistory clears the game event history
func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = nil
	log.Printf("[Engine] History cleared")
}

// SetHistoryPath sets the path for history persistence
func (e *Engine) SetHistoryPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.historyPath = path
	log.Printf("[Engine] History path set to: %s", path)
}

// SaveHistory persists history to disk
func (e *Engine) SaveHistory() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.historyPath == "" {
		return nil // No path configured, skip
	}

	// Ensure directory exists
	dir := filepath.Dir(e.historyPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create history directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.history, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal history: %v", err)
		return err
	}

	if err := os.WriteFile(e.historyPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save history: %v", err)
		return err
	}

	log.Printf("[Engine] History saved: %d events to %s", len(e.history), e.historyPath)
	return nil
}

// LoadHistory loads history from disk
func (e *Engine) LoadHistory() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.historyPath == "" {
		return nil // No path configured, skip
	}

	data, err := os.ReadFile(e.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No history file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read history: %v", err)
		return err
	}

	var history []GameEvent
	if err := json.Unmarshal(data, &history); err != nil {
		log.Printf("[Engine] Failed to parse history: %v", err)
		return err
	}

	e.history = history
	log.Printf("[Engine] History loaded: %d events from %s", len(history), e.historyPath)
	return nil
}

// RecalculateScoresFromHistory recalculates all scores from history events
func (e *Engine) RecalculateScoresFromHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Reset all scores first
	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
	}
	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
	}

	// Replay all events
	playerPoints := 0
	teamPoints := 0

	for _, event := range e.history {
		switch event.WinnerType {
		case "PLAYER":
			// Add points to bumper
			if bumper, ok := e.data.Bumpers[event.WinnerID]; ok {
				bumper.Score += event.Points
				playerPoints += event.Points
			}
		case "TEAM":
			// Add points to team's TeamPoints
			if team, ok := e.data.Teams[event.TeamName]; ok {
				team.TeamPoints += event.Points
				teamPoints += event.Points
			}
		}
	}

	// Recalculate all team total scores (TeamPoints + bumper scores)
	for teamName := range e.data.Teams {
		e.recalculateTeamScoreUnsafe(teamName)
	}

	log.Printf("[Engine] Scores recalculated from history: %d events, playerPoints=%d, teamPoints=%d",
		len(e.history), playerPoints, teamPoints)
}

// SetTeamsPath sets the path for teams persistence
func (e *Engine) SetTeamsPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teamsPath = path
	log.Printf("[Engine] Teams path set to: %s", path)
}

// SetBumpersPath sets the path for bumpers persistence
func (e *Engine) SetBumpersPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bumpersPath = path
	log.Printf("[Engine] Bumpers path set to: %s", path)
}

// SaveTeams persists teams to disk
func (e *Engine) SaveTeams() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.teamsPath == "" {
		return nil
	}

	dir := filepath.Dir(e.teamsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create teams directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.data.Teams, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal teams: %v", err)
		return err
	}

	// Write to a uniquely-named temp file and atomically rename into place (#113 B4
	// hardening). SaveTeams is fired from a background goroutine by both SetTeams and
	// UpdateTeam, so two saves can legitimately overlap; os.WriteFile truncates the
	// destination in place, so a concurrent LoadTeams (or the other save) could
	// observe an empty/partial file mid-write. os.CreateTemp gives each call its own
	// file (no two saves can collide on the same temp path), and os.Rename is atomic
	// on the same filesystem, so readers only ever see a fully-formed old or new file.
	tmpFile, err := os.CreateTemp(dir, filepath.Base(e.teamsPath)+".tmp-*")
	if err != nil {
		log.Printf("[Engine] Failed to create temp file for teams save: %v", err)
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to write temp teams file: %v", err)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to close temp teams file: %v", err)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to chmod temp teams file: %v", err)
		return err
	}
	if err := os.Rename(tmpPath, e.teamsPath); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to save teams: %v", err)
		return err
	}

	log.Printf("[Engine] Teams saved: %d teams to %s", len(e.data.Teams), e.teamsPath)
	return nil
}

// LoadTeams loads teams from disk
func (e *Engine) LoadTeams() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.teamsPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.teamsPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No teams file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read teams: %v", err)
		return err
	}

	var teams map[string]*Team
	if err := json.Unmarshal(data, &teams); err != nil {
		log.Printf("[Engine] Failed to parse teams: %v", err)
		return err
	}

	e.data.Teams = teams
	log.Printf("[Engine] Teams loaded: %d teams from %s", len(teams), e.teamsPath)
	return nil
}

// SaveBumpers persists bumpers to disk
func (e *Engine) SaveBumpers() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.bumpersPath == "" {
		return nil
	}

	dir := filepath.Dir(e.bumpersPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create bumpers directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.data.Bumpers, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal bumpers: %v", err)
		return err
	}

	// Write to a uniquely-named temp file and atomically rename into place (#120 B2
	// hardening, same pattern already applied to SaveTeams in #113 B4). SaveBumpers
	// is fired from a background goroutine on every enrollment and reconnection, so
	// saves can legitimately overlap; os.WriteFile truncates the destination in
	// place, so a concurrent LoadBumpers (or the other save) could observe an
	// empty/partial file mid-write. os.CreateTemp gives each call its own file (no
	// two saves can collide on the same temp path), and os.Rename is atomic on the
	// same filesystem, so readers only ever see a fully-formed old or new file.
	tmpFile, err := os.CreateTemp(dir, filepath.Base(e.bumpersPath)+".tmp-*")
	if err != nil {
		log.Printf("[Engine] Failed to create temp file for bumpers save: %v", err)
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to write temp bumpers file: %v", err)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to close temp bumpers file: %v", err)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to chmod temp bumpers file: %v", err)
		return err
	}
	if err := os.Rename(tmpPath, e.bumpersPath); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to save bumpers: %v", err)
		return err
	}

	log.Printf("[Engine] Bumpers saved: %d bumpers to %s", len(e.data.Bumpers), e.bumpersPath)
	return nil
}

// LoadBumpers loads bumpers from disk
func (e *Engine) LoadBumpers() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.bumpersPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.bumpersPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No bumpers file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read bumpers: %v", err)
		return err
	}

	var bumpers map[string]*Bumper
	if err := json.Unmarshal(data, &bumpers); err != nil {
		log.Printf("[Engine] Failed to parse bumpers: %v", err)
		return err
	}

	e.data.Bumpers = bumpers
	log.Printf("[Engine] Bumpers loaded: %d bumpers from %s", len(bumpers), e.bumpersPath)
	return nil
}

// SetStatusesPath sets the path for question statuses persistence
func (e *Engine) SetStatusesPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusesPath = path
	log.Printf("[Engine] Statuses path set to: %s", path)
}

// SaveStatuses persists question statuses to disk
func (e *Engine) SaveStatuses() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.statusesPath == "" {
		return nil
	}

	dir := filepath.Dir(e.statusesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create statuses directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.questionStatuses, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal statuses: %v", err)
		return err
	}

	if err := os.WriteFile(e.statusesPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save statuses: %v", err)
		return err
	}

	log.Printf("[Engine] Statuses saved: %d statuses to %s", len(e.questionStatuses), e.statusesPath)
	return nil
}

// LoadStatuses loads question statuses from disk
func (e *Engine) LoadStatuses() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.statusesPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.statusesPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No statuses file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read statuses: %v", err)
		return err
	}

	var statuses map[string]QuestionStatus
	if err := json.Unmarshal(data, &statuses); err != nil {
		log.Printf("[Engine] Failed to parse statuses: %v", err)
		return err
	}

	e.questionStatuses = statuses
	log.Printf("[Engine] Statuses loaded: %d statuses from %s", len(statuses), e.statusesPath)
	return nil
}

// ClearStatuses resets all question statuses
func (e *Engine) ClearStatuses() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.questionStatuses = make(map[string]QuestionStatus)
	log.Printf("[Engine] Question statuses cleared")
}

// SaveAll saves teams, bumpers, history and statuses to disk
func (e *Engine) SaveAll() {
	safeGo("SaveTeams", e.SaveTeams)
	safeGo("SaveBumpers", e.SaveBumpers)
	safeGo("SaveHistory", e.SaveHistory)
	safeGo("SaveStatuses", e.SaveStatuses)
}

// ErrRafalePoolEmpty is returned by DrawRafaleQuestion when no reservoir
// question matches the requested categories/difficulty filter and is not
// already used — contracts/rafale.md §7.1. The caller must react by ending
// the round (RAFALE_EXHAUSTED), never by silently repeating an already-seen
// question.
var ErrRafalePoolEmpty = errors.New("rafale_pool_empty")

// rafalePoolUnsafe returns the reservoir questions matching categories
// (multi, OR) ∩ difficulty (exact) ∩ not-yet-used — contracts/rafale.md §7.
// Caller must hold e.mu (read or write lock).
func (e *Engine) rafalePoolUnsafe(categories []string, difficulty int) []*RafaleQuestion {
	catSet := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		catSet[c] = struct{}{}
	}
	var pool []*RafaleQuestion
	for _, q := range e.rafaleQuestions {
		if _, ok := catSet[string(q.Category)]; !ok {
			continue
		}
		if q.Difficulty != difficulty {
			continue
		}
		if e.rafaleUsed[q.ID] {
			continue
		}
		pool = append(pool, q)
	}
	return pool
}

// CountRafalePool returns (available, used, total) reservoir questions for
// a categories/difficulty filter — contracts/rafale.md §7.2, the pre-round
// alert (GET /api/rafale/pool): blocking when available==0, a warning when
// available < the estimated need, informational otherwise.
func (e *Engine) CountRafalePool(categories []string, difficulty int) (available, used, total int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	catSet := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		catSet[c] = struct{}{}
	}
	for _, q := range e.rafaleQuestions {
		if _, ok := catSet[string(q.Category)]; !ok {
			continue
		}
		if q.Difficulty != difficulty {
			continue
		}
		total++
		if e.rafaleUsed[q.ID] {
			used++
		} else {
			available++
		}
	}
	return available, used, total
}

// DrawRafaleQuestion draws one question uniformly at random from the pool
// (categories ∩ difficulty ∩ not-used — contracts/rafale.md §7), marks it
// used immediately and persists the flag asynchronously so a restart mid-
// round never re-proposes it. Returns ErrRafalePoolEmpty when the pool is
// empty; the caller (Phase 2, #107) reacts per contract §7.1.
//
// Public entry point for callers NOT already holding e.mu (e.g. a future
// HTTP/test caller outside the engine). The RAFALE round engine (#107,
// advanceRafaleUnsafe/startRafaleRoundUnsafe) is already inside a locked
// section when it needs to draw — it calls drawRafaleQuestionUnsafe
// directly instead, to stay within one atomic transition rather than
// releasing and re-acquiring e.mu mid-round-advance.
func (e *Engine) DrawRafaleQuestion(categories []string, difficulty int) (*RafaleQuestion, error) {
	e.mu.Lock()
	drawn, err := e.drawRafaleQuestionUnsafe(categories, difficulty)
	e.mu.Unlock()

	if err != nil {
		return nil, err
	}

	safeGo("SaveRafaleUsed", e.SaveRafaleUsed)

	return drawn, nil
}

// drawRafaleQuestionUnsafe is DrawRafaleQuestion's locked core. Caller must
// hold e.mu (write lock) and is responsible for persisting rafaleUsed
// (safeGo("SaveRafaleUsed", e.SaveRafaleUsed)) once its own locked section
// ends — this function only mutates in-memory state.
func (e *Engine) drawRafaleQuestionUnsafe(categories []string, difficulty int) (*RafaleQuestion, error) {
	pool := e.rafalePoolUnsafe(categories, difficulty)
	if len(pool) == 0 {
		return nil, ErrRafalePoolEmpty
	}

	picked := pool[rand.Intn(len(pool))]
	drawn := *picked // copy: the caller must never hold a pointer into e.rafaleQuestions
	e.rafaleUsed[drawn.ID] = true

	return &drawn, nil
}

// FlipMemoryCard handles flipping a Memory card with game logic
// Returns: (isMatch bool, shouldFlipBack bool, flipDelay int, isComplete bool)
// - isMatch: true if the 2nd card matches the 1st (same pair)
// - shouldFlipBack: true if we have 2 non-matching cards and need to flip back
// - flipDelay: milliseconds to wait before flipping back (from question config)
// - isComplete: true if all pairs have been matched (game complete)
func (e *Engine) FlipMemoryCard(cardID string) (isMatch bool, shouldFlipBack bool, flipDelay int, isComplete bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Only allow flipping during STARTED phase
	if e.state.Phase != PhaseStarted {
		log.Printf("[Engine] Memory flip ignored: game not in STARTED phase (current: %s)", e.state.Phase)
		return false, false, 0, false
	}

	// Extract pair ID from card ID (format: "pairID-cardNum", e.g., "1-1", "2-2")
	pairID := e.extractPairID(cardID)
	if pairID == 0 {
		log.Printf("[Engine] Memory flip ignored: invalid card ID format: %s", cardID)
		return false, false, 0, false
	}

	// Check if this pair is already matched
	for _, matchedPairID := range e.state.MemoryMatchedPairs {
		if matchedPairID == pairID {
			log.Printf("[Engine] Memory flip ignored: pair %d already matched", pairID)
			return false, false, 0, false
		}
	}

	// Check if card is already flipped
	for _, id := range e.state.MemoryFlippedCards {
		if id == cardID {
			log.Printf("[Engine] Memory flip ignored: card %s already flipped", cardID)
			return false, false, 0, false
		}
	}

	// Don't allow more than 2 cards to be flipped at once
	if len(e.state.MemoryFlippedCards) >= 2 {
		log.Printf("[Engine] Memory flip ignored: already 2 cards flipped")
		return false, false, 0, false
	}

	// Add card to flipped cards
	e.state.MemoryFlippedCards = append(e.state.MemoryFlippedCards, cardID)
	log.Printf("[Engine] Memory card %s flipped (revealed)", cardID)

	// If only 1 card flipped, just wait for second card
	if len(e.state.MemoryFlippedCards) == 1 {
		return false, false, 0, false
	}

	// Two cards are now flipped - check if they match
	firstCardID := e.state.MemoryFlippedCards[0]
	secondCardID := e.state.MemoryFlippedCards[1]
	firstPairID := e.extractPairID(firstCardID)
	secondPairID := e.extractPairID(secondCardID)

	// Get flip delay from question config (config is in seconds, convert to ms)
	flipDelay = 3000 // Default 3 seconds
	if e.state.Question != nil && e.state.Question.MemoryConfig != nil && e.state.Question.MemoryConfig.FlipDelay > 0 {
		flipDelay = int(e.state.Question.MemoryConfig.FlipDelay * 1000)
	}

	if firstPairID == secondPairID {
		// MATCH! Add to matched pairs and clear flipped cards
		e.state.MemoryMatchedPairs = append(e.state.MemoryMatchedPairs, firstPairID)
		e.state.MemoryFlippedCards = nil

		// Award pair to current team in multi-team mode
		if e.state.MemoryCurrentTeam != "" && e.state.MemoryTeamPairs != nil {
			e.state.MemoryTeamPairs[e.state.MemoryCurrentTeam]++
			log.Printf("[Engine] Pair awarded to team %s (total: %d pairs)", e.state.MemoryCurrentTeam, e.state.MemoryTeamPairs[e.state.MemoryCurrentTeam])
		}

		// Track ownership of this pair
		if e.state.MemoryCurrentTeam != "" {
			if e.state.MemoryPairOwners == nil {
				e.state.MemoryPairOwners = make(map[int]string)
			}
			e.state.MemoryPairOwners[firstPairID] = e.state.MemoryCurrentTeam
			log.Printf("[Engine] Pair %d ownership: team %s", firstPairID, e.state.MemoryCurrentTeam)
		}

		// Check if all pairs are matched (game complete)
		totalPairs := 0
		if e.state.Question != nil && e.state.Question.MemoryPairs != nil {
			totalPairs = len(e.state.Question.MemoryPairs)
		}
		isComplete = len(e.state.MemoryMatchedPairs) >= totalPairs && totalPairs > 0

		// Handle team rotation based on memory mode
		memoryMode := ""
		if e.state.Question != nil {
			memoryMode = e.state.Question.MemoryMode
			if memoryMode == "" {
				memoryMode = string(MemoryModeSolo)
			}
		}

		// Rotate team after match (mode: CHACUN_SON_TOUR)
		if memoryMode == string(MemoryModeChacunSonTour) {
			e.rotateToNextTeam()
		}
		// Mode TANT_QUE_JE_GAGNE: team keeps playing, no rotation

		log.Printf("[Engine] Memory MATCH! Pair %d found. Total matched: %d/%d. Complete: %v", firstPairID, len(e.state.MemoryMatchedPairs), totalPairs, isComplete)
		return true, false, 0, isComplete
	}

	// No match - increment error counter, caller should schedule flip-back after delay
	e.state.MemoryErrors++

	// Track error for current team in multi-team mode
	if e.state.MemoryCurrentTeam != "" && e.state.MemoryTeamErrors != nil {
		e.state.MemoryTeamErrors[e.state.MemoryCurrentTeam]++
	}

	// NOTE: Team rotation will happen in ClearMemoryFlippedCards() AFTER cards are hidden
	// This ensures players see which team made the error before the turn changes

	log.Printf("[Engine] Memory NO MATCH (error #%d). Cards %s and %s will flip back after %dms", e.state.MemoryErrors, firstCardID, secondCardID, flipDelay)
	return false, true, flipDelay, false
}

// extractPairID extracts the pair ID from a card ID (format: "pairID-cardNum")
func (e *Engine) extractPairID(cardID string) int {
	var pairID, cardNum int
	_, err := parseCardID(cardID, &pairID, &cardNum)
	if err != nil {
		return 0
	}
	return pairID
}

// parseCardID parses "pairID-cardNum" format
func parseCardID(cardID string, pairID, cardNum *int) (bool, error) {
	n, _ := parseCardIDParts(cardID)
	if n >= 1 {
		parts := splitCardID(cardID)
		if len(parts) == 2 {
			*pairID = parseInt(parts[0])
			*cardNum = parseInt(parts[1])
			return true, nil
		}
	}
	return false, nil
}

func splitCardID(cardID string) []string {
	var parts []string
	current := ""
	for _, c := range cardID {
		if c == '-' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func parseCardIDParts(cardID string) (int, error) {
	count := 0
	for _, c := range cardID {
		if c == '-' {
			count++
		}
	}
	return count + 1, nil
}

func parseInt(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// ClearMemoryFlippedCards resets all flipped cards and rotates team if needed
// This is called after the flip-back delay when cards are hidden
func (e *Engine) ClearMemoryFlippedCards() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear the flipped cards
	wasFlipped := len(e.state.MemoryFlippedCards) > 0
	e.state.MemoryFlippedCards = nil

	// Rotate team AFTER cards are hidden (only if we had 2 cards that didn't match)
	// This ensures the team rotation happens when cards are face-down, not while visible
	if wasFlipped && len(e.state.MemoryParticipatingTeams) > 0 {
		memoryMode := ""
		if e.state.Question != nil {
			memoryMode = e.state.Question.MemoryMode
			if memoryMode == "" {
				memoryMode = string(MemoryModeSolo)
			}
		}

		// Rotate team after error (both CHACUN_SON_TOUR and TANT_QUE_JE_GAGNE)
		// Only rotate in multi-team modes
		if memoryMode == string(MemoryModeChacunSonTour) || memoryMode == string(MemoryModeTantQueJeGagne) {
			e.rotateToNextTeam()
		}
	}

	log.Printf("[Engine] Memory flipped cards cleared")
}

// GetMemoryFlippedCards returns the list of flipped card IDs
func (e *Engine) GetMemoryFlippedCards() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.MemoryFlippedCards
}

// SetEnrollmentActive enables or disables virtual player enrollment (QR code display)
func (e *Engine) SetEnrollmentActive(active bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.EnrollmentActive = active
	log.Printf("[Engine] Enrollment active: %v", active)
}

// GetEnrollmentActive returns whether enrollment is currently active
func (e *Engine) GetEnrollmentActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.EnrollmentActive
}

// SetShowQRCode sets whether the QR code should be displayed on TV
func (e *Engine) SetShowQRCode(show bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.ShowQRCode = show

	// When showing QR code, switch to ENROLL phase
	// When hiding QR code, return to STOPPED phase
	if show {
		e.state.Phase = PhaseEnroll
		log.Printf("[Engine] Show QR code: enabled, switching to ENROLL phase")
	} else {
		e.state.Phase = PhaseStopped
		log.Printf("[Engine] Show QR code: disabled, returning to STOPPED phase")
	}
}

// GetShowQRCode returns whether the QR code should be displayed
func (e *Engine) GetShowQRCode() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.ShowQRCode
}

// SetEntracte activates or deactivates ENTRACTE mode (v6.5.2, #119, D3: an
// explicit idempotent command carrying the desired state, not a toggle —
// two rapid clicks or a network resend can never leave the state
// inverted). Returns whether the change was applied.
//
// Activation is gated by phase (contract game-state.md §"Phases
// autorisées", D4): only allowed from STOPPED/PREPARE/READY/NEW_GAME/
// REVEALED — never while a round is live (COUNTDOWN/STARTED/PAUSED), never
// from ENROLL (the QR code screen players need). A refused activation is
// logged and returns false, changing nothing else in state.
//
// Deactivation always succeeds, from ANY phase — ENTRACTE must never be a
// dead end (contract, risk table "Entracte sans issue").
//
// Deliberately touches ONLY e.state.Entracte and (on activation)
// e.state.EntracteConfig: Phase, Question and everything else are read-only
// here, so a question selected in PREPARE is found intact on exit.
//
// C4 (2026-08-20, arbitrage): on activation, recopies EntracteConfigSaved
// into EntracteConfig — under this SAME lock, BEFORE raising the flag — so
// the panel that's about to be shown always reflects the latest saved
// settings, and no client can ever observe ENTRACTE=true paired with a
// stale diffused config left over from a previous pause (contract
// game-state.md §"Configuration gelée à l'activation"). This is also what
// makes EntracteConfig/EntracteConfigSaved provably identical immediately
// after every activation, and after every LoadState() at startup — "equal
// outside a pause" is not just true by convention, it's enforced at the
// one place the pause begins.
func (e *Engine) SetEntracte(active bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !active {
		e.state.Entracte = false
		log.Printf("[Engine] Entracte: deactivated (phase=%s)", e.state.Phase)
		return true
	}

	allowedPhases := e.state.Phase == PhaseStopped || e.state.Phase == PhasePrepare ||
		e.state.Phase == PhaseReady || e.state.Phase == PhaseNewGame ||
		e.state.Phase == PhaseRevealed
	if !allowedPhases {
		log.Printf("[Engine] Entracte: activation refused, phase=%s not eligible", e.state.Phase)
		return false
	}

	e.state.EntracteConfig = e.state.EntracteConfigSaved
	e.state.Entracte = true
	log.Printf("[Engine] Entracte: activated (phase=%s)", e.state.Phase)
	return true
}

// IsEntracte returns whether ENTRACTE mode is currently active.
func (e *Engine) IsEntracte() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Entracte
}

// SetEntracteConfig saves a new entracte panel configuration (v6.5.2,
// #119, C1/C4) — called by cmd/server/main.go's handleUpdateEntracteConfig
// after validating/clamping the incoming payload. cfg is expected fully
// resolved (no zero-value ambiguity left to resolve here — the caller
// already merged any "field absent" pointer against the current saved
// config and clamped every bound).
//
// ALWAYS updates EntracteConfigSaved and persists synchronously
// (SaveState, same pattern as SetQuizMeta: unlock first, then save, so a
// slow disk write never holds e.mu). Only ALSO refreshes the diffused
// EntracteConfig — what the panel currently on screen actually shows —
// when no entracte is active (C4's freeze rule): editing settings mid-pause
// must reach the Quiz page's own form (which always reads
// EntracteConfigSaved) without touching what's already displayed. The new
// values take effect at the next SetEntracte(true), which recopies Saved
// into the diffused field under its own lock.
func (e *Engine) SetEntracteConfig(cfg EntracteConfig) {
	e.mu.Lock()
	e.state.EntracteConfigSaved = cfg
	frozen := e.state.Entracte
	if !frozen {
		e.state.EntracteConfig = cfg
	}
	e.mu.Unlock()

	log.Printf("[Engine] Entracte config saved: title=%q panel_size=%d anim_period=%d anim_intensity=%d transition_ms=%d (diffused %s)",
		cfg.Title, cfg.PanelSize, cfg.AnimPeriod, cfg.AnimIntensity, cfg.TransitionMs,
		map[bool]string{true: "frozen (entracte active)", false: "refreshed"}[frozen])

	if err := e.SaveState(); err != nil {
		log.Printf("[Engine] Failed to persist game state after entracte config change: %v", err)
	}
}

// RefreshEntracteImageIsCustom updates the disk-derived IMAGE_IS_CUSTOM
// flag (v6.5.2, #119, C1/C4) — called by cmd/server/main.go once at
// startup (right after LoadState) and after every panel-image upload/
// delete (/api/game/entracte-image). NEVER persisted — see EntracteConfig's
// own doc comment and PersistedGameState's — so there is no SaveState()
// call here, unlike SetEntracteConfig.
//
// Follows the exact same C4 freeze rule as SetEntracteConfig, extended to
// this field for consistency: the Quiz page's edit form (reading
// EntracteConfigSaved) must see an uploaded image immediately, but a panel
// already on screen during an active pause must not change out from under
// the audience just because someone swapped the image file mid-pause.
func (e *Engine) RefreshEntracteImageIsCustom(isCustom bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.EntracteConfigSaved.ImageIsCustom = isCustom
	if !e.state.Entracte {
		e.state.EntracteConfig.ImageIsCustom = isCustom
	}
}

// GetVirtualPlayerCount returns the current count of enrolled virtual players
func (e *Engine) GetVirtualPlayerCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.VirtualPlayerCount
}

// IncrementVirtualPlayerCount increments the virtual player counter
func (e *Engine) IncrementVirtualPlayerCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount++
	count := e.state.VirtualPlayerCount
	log.Printf("[Engine] Virtual player count: %d", count)
	return count
}

// ResetVirtualPlayerCount resets the virtual player counter to zero
func (e *Engine) ResetVirtualPlayerCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount = 0
	log.Printf("[Engine] Virtual player count reset")
}

// GetVirtualPlayerLimit returns the maximum number of virtual players allowed
func (e *Engine) GetVirtualPlayerLimit() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.state.VirtualPlayerLimit == 0 {
		return 20 // Default limit
	}
	return e.state.VirtualPlayerLimit
}

// SetVirtualPlayerLimit sets the maximum number of virtual players allowed
func (e *Engine) SetVirtualPlayerLimit(limit int) {
	e.mu.Lock()
	if limit < 1 {
		limit = 20 // Minimum 1, default to 20 if invalid
	}
	e.state.VirtualPlayerLimit = limit
	e.mu.Unlock()

	log.Printf("[Engine] Virtual player limit set to: %d", limit)

	// #141 — persist synchronously, same rationale as SetQuizMeta above.
	if err := e.SaveState(); err != nil {
		log.Printf("[Engine] Failed to persist game state after virtual player limit change: %v", err)
	}
}

// SetArdoiseAnswer updates the free-text answer for a team during an ARDOISE question.
// It validates that the game is in STARTED phase and the current question is ARDOISE.
// Returns false (silently) if the guard conditions are not met, following the plan's
// "ignore silently" specification.
func (e *Engine) SetArdoiseAnswer(teamName string, text string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Phase guard: only accept answers during STARTED phase
	if e.state.Phase != PhaseStarted {
		return false
	}

	// Type guard: only accept answers for ARDOISE questions
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeArdoise {
		return false
	}

	// StartedAt is frozen at the first non-empty text received for this team/question
	// (#117): it must survive subsequent keystrokes, including a full clear-and-retype.
	// SubmittedAt, in contrast, is overwritten on every call.
	now := time.Now().UnixMicro()
	startedAt := e.state.ArdoiseAnswers[teamName].StartedAt
	if startedAt == 0 && text != "" {
		startedAt = now
	}

	e.state.ArdoiseAnswers[teamName] = ArdoiseAnswer{
		Text:        text,
		StartedAt:   startedAt,
		SubmittedAt: now,
	}
	return true
}

// CreateVirtualPlayer creates a new virtual player (bumper) during enrollment
func (e *Engine) CreateVirtualPlayer(name string) (string, *Bumper, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.createVirtualPlayerUnsafe(name)
}

// createVirtualPlayerUnsafe creates a new virtual player bumper. Caller must
// hold e.mu (used directly by ReconnectOrCreateVirtualPlayer to keep the
// find-or-create sequence atomic — see #109 R1 below).
func (e *Engine) createVirtualPlayerUnsafe(name string) (string, *Bumper, error) {
	// Check phase ENROLL
	if e.state.Phase != PhaseEnroll {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_CLOSED"}
	}

	// Check limit
	currentCount := e.countVirtualPlayersUnsafe()
	limit := e.state.VirtualPlayerLimit
	if limit == 0 {
		limit = 20 // Default
	}
	if currentCount >= limit {
		return "", nil, &EnrollmentError{Reason: "LIMIT_REACHED"}
	}

	// Generate unique ID
	id := "vjoueur_" + name + "_" + time.Now().Format("20060102_150405")

	// Create virtual bumper (VPlayer = true for virtual players, allows all QCM colors)
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		IsVPlayer: true, // VPlayer can answer all QCM colors
		Status:    "READY",
		Connected: true, // VJoueur just opened its WS session at enrollment time, so it's connected by definition
	}
	// CONN_STATE: no-op today (Team == "" until assigned to a team), kept for
	// consistency with the other 5 connection call sites (#109 Phase 1).
	applyConnEventUnsafe(bumper, ConnEventReconnect)

	e.data.Bumpers[id] = bumper

	// Increment virtual player count in GameState
	e.state.VirtualPlayerCount++
	log.Printf("[Engine] Virtual player created: id=%s, name=%s, count=%d", id, name, e.state.VirtualPlayerCount)

	// Save bumpers to disk (in goroutine to avoid blocking)
	safeGo("SaveBumpers", e.SaveBumpers)

	return id, bumper, nil
}

// ReconnectOrCreateVirtualPlayer resolves a PLAYER_CONNECT request per the
// backend-ID decision matrix (fix R1, code-review CRITICAL —
// _work/reports/planner-20260725-143029-r1-fix.md §3.3). The bumper ID
// (already generated and returned to the client at enrollment, in
// PLAYER_CONNECTED.ID — the map key itself) is the sole source of identity
// for reconnection; the name is used only for the free/taken check on a new
// enrollment. This eliminates the ambiguity that made the previous
// name-based version merge/delete distinct players who happened to share a
// name (code-review CRITICAL finding — silent, irreversible data loss).
//
//  1. id given, Bumpers[id] exists and IsVirtual -> legitimate reconnection,
//     unambiguous (the ID is unique and stable). Reused as-is (-> green);
//     Name is refreshed if the client's local copy had changed. Allowed in
//     any phase, not just ENROLL.
//  2. id given but not found (stale localStorage, bumper deleted by admin,
//     server restart before persistence, ...) -> treated as no id, falls to 3/4.
//  3. no id, name already taken by another VJoueur (connected OR
//     disconnected) -> REJECTED (NAME_TAKEN). Never merged, never replaced —
//     this is the actual fix: no more silent consolidation of homonyms.
//  4. no id, name free -> new enrollment (subject to ENROLL phase / limit,
//     enforced by createVirtualPlayerUnsafe). Its ID is returned to the
//     client, which must store it for future reconnections.
//  5. id given, not found, name free -> same as 4.
//
// Atomic under a single engine lock — this IS the real #109 fix (the
// find-or-create sequence used to be two separate locked steps in main.go,
// racy for two near-simultaneous PLAYER_CONNECT calls for the same identity).
// Unlike the previous version of this method, nothing is ever deleted here.
//
// Returns (id, bumper, reconnected, err). reconnected is false only when a
// brand-new bumper was created.
func (e *Engine) ReconnectOrCreateVirtualPlayer(id, name string) (string, *Bumper, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	normalized := strings.TrimSpace(name)

	// Case 1: a resolvable ID is unambiguous — always a genuine reconnection.
	if id != "" {
		if bumper, ok := e.data.Bumpers[id]; ok && bumper != nil && bumper.IsVirtual {
			if bumper.Name != normalized {
				bumper.Name = normalized
			}
			bumper.Connected = true
			applyConnEventUnsafe(bumper, ConnEventReconnect)
			// #122 B2/B3 — the player found their way back on their own: any
			// pending "place demandée" signal or reclaim authorization for this
			// bumper is now moot.
			bumper.ReclaimRequested = false
			bumper.reclaimAuthorizedUntil = time.Time{}
			log.Printf("[Engine] Virtual player reconnected by ID: id=%s, name=%s", id, bumper.Name)
			safeGo("SaveBumpers", e.SaveBumpers)
			return id, bumper, true, nil
		}
		// Case 2: stale/unknown ID — fall through as if none was provided.
	}

	// #122 B3 — reclaim authorization: an animateur-granted, single-use,
	// time-bounded exception to the ID-only identity rule (#109 R1),
	// deliberately narrow — checked BEFORE the case-3 rejection, and only for
	// a request with NO id at all (never for a stale/unresolvable one). The
	// authorization is read and consumed in this same locked section, so two
	// concurrent PLAYER_CONNECT calls racing the same authorization can never
	// both succeed.
	if id == "" {
		for oldID, cand := range e.data.Bumpers {
			if cand == nil || !cand.IsVirtual {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(cand.Name), normalized) {
				continue
			}
			if cand.reclaimAuthorizedUntil.IsZero() || time.Now().After(cand.reclaimAuthorizedUntil) {
				continue // no authorization for this name, or it has expired
			}

			newID := e.reattachVirtualPlayerUnsafe(oldID, cand, normalized)
			log.Printf("[Engine] Virtual player reclaimed: old_id=%s, new_id=%s, name=%s", oldID, newID, normalized)
			return newID, cand, true, nil
		}
	}

	// Case 3: reject a name collision outright — no id means we cannot tell a
	// genuine reconnection (client lost its stored ID) from a new person who
	// happens to share a name with an existing (possibly long-gone) VJoueur.
	// The legitimate owner always has their ID and takes the case-1 path.
	for _, cand := range e.data.Bumpers {
		if cand == nil || !cand.IsVirtual {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cand.Name), normalized) {
			// #122 B1 — distinguish the reason by the holder's connection
			// state: a CONNECTED holder is a genuine homonym collision
			// (message unchanged, #109 non-regression); a DISCONNECTED holder
			// is most likely this very player having lost their ID — invite
			// them to request the seat instead of picking another name.
			if !cand.Connected {
				// #122 B2 — this rejection IS the signal: flag the holder's own
				// card for the animateur right now, not after some delay.
				cand.ReclaimRequested = true
				safeGo("SaveBumpers", e.SaveBumpers)
				return "", nil, false, &EnrollmentError{Reason: "NAME_TAKEN_OFFLINE"}
			}
			return "", nil, false, &EnrollmentError{Reason: "NAME_TAKEN"}
		}
	}

	// Case 4/5: no usable id, name free -> new enrollment.
	newID, newBumper, err := e.createVirtualPlayerUnsafe(normalized)
	return newID, newBumper, false, err
}

// rekeyBumperUnsafe moves bumper from oldID to a freshly generated ID —
// the re-keying core shared by reattachVirtualPlayerUnsafe (#122, seat
// reclaimed by a nameless PLAYER_CONNECT) and ReleaseSeat (#134, seat
// released by the animateur while still occupied). Only the map key and
// Name change here: Connected, ConnState, ReclaimRequested and
// reclaimAuthorizedUntil are left to the caller, deliberately — a reclaim
// and a release leave the bumper in opposite connection states, so there is
// no single "after re-key" value correct for both. Caller must hold e.mu.
func (e *Engine) rekeyBumperUnsafe(oldID string, bumper *Bumper, name string) string {
	newID := "vjoueur_" + name + "_" + time.Now().Format("20060102_150405")

	delete(e.data.Bumpers, oldID)
	e.data.Bumpers[newID] = bumper
	bumper.Name = name

	return newID
}

// reattachVirtualPlayerUnsafe reattaches an existing bumper (whose owner lost
// their ID) under a freshly generated ID, consuming its reclaim authorization
// (#122 B3). Caller must hold e.mu and have already verified the
// authorization is valid for this bumper. Score, Team, and every other field
// are preserved as-is — only the map key (ID) changes; the seat is rendered,
// not recreated, which is precisely what preserves the score/team/history.
func (e *Engine) reattachVirtualPlayerUnsafe(oldID string, bumper *Bumper, name string) string {
	newID := e.rekeyBumperUnsafe(oldID, bumper, name)

	bumper.Connected = true
	bumper.ReclaimRequested = false
	bumper.reclaimAuthorizedUntil = time.Time{} // consumed — single use
	applyConnEventUnsafe(bumper, ConnEventReconnect)

	safeGo("SaveBumpers", e.SaveBumpers)
	return newID
}

// ReleaseBumperName grants a one-time, time-bounded authorization (#122 B3)
// for the NEXT nameless PLAYER_CONNECT under this bumper's name to reattach
// to it instead of being rejected — the only exception to the #109 R1
// ID-only identity rule, and an explicit, human (animateur) decision rather
// than an automatic one. Returns false if id doesn't resolve to a virtual
// bumper (nothing to release).
//
// Unchanged by #134: this is exactly the #122 behavior, locked down by the
// non-regression tests in name_recovery_test.go (both packages) — it is
// called as-is by ReleaseSeat below for a bumper that is NOT connected
// (contracts/seat-release.md §3). ReleaseSeat cannot call this method
// directly (it already holds e.mu; sync.Mutex is not reentrant) — see the
// duplicated three lines there, deliberately kept in lockstep with this
// body rather than factored, to avoid touching a function multiple existing
// tests assert on byte-for-byte (log line included).
func (e *Engine) ReleaseBumperName(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, ok := e.data.Bumpers[id]
	if !ok || bumper == nil || !bumper.IsVirtual {
		return false
	}

	bumper.reclaimAuthorizedUntil = time.Now().Add(reclaimAuthorizationTTL)
	bumper.ReclaimRequested = false
	log.Printf("[Engine] Bumper name released for reclaim: id=%s, name=%s, expires=%s",
		id, bumper.Name, bumper.reclaimAuthorizedUntil.Format(time.RFC3339))

	safeGo("SaveBumpers", e.SaveBumpers)
	return true
}

// ReleaseSeat is the #134 entry point for RELEASE_BUMPER_NAME, handling both
// the pre-existing #122 case (bumper disconnected — delegates verbatim,
// unchanged) and the new #134 case (bumper connected — evict + re-key,
// preserving the struct so score/team/history survive under a fresh ID).
// See contracts/seat-release.md §2-3 for the full behavioral contract.
//
// Returns:
//   - newID: the bumper's new ID after a connected release; "" when the
//     bumper was disconnected (its ID never changes in that case, #122).
//   - wasConnected: true only when this call actually evicted+re-keyed a
//     connected bumper. The caller (handleReleaseBumperName) uses this to
//     decide whether a PLAYER_EVICTED notification is due at all.
//   - ok: false if id doesn't resolve to a virtual bumper (nothing to
//     release) — mirrors ReleaseBumperName's own not-found contract.
//
// Atomic under a single lock: the connected/disconnected check, the re-key,
// and the fresh reclaimAuthorizedUntil are all set together, so a concurrent
// PLAYER_CONNECT can never observe a half-released seat (same discipline
// that motivated ReconnectOrCreateVirtualPlayer's single lock, #109 R1).
func (e *Engine) ReleaseSeat(id string) (newID string, wasConnected bool, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, exists := e.data.Bumpers[id]
	if !exists || bumper == nil || !bumper.IsVirtual {
		return "", false, false
	}

	if !bumper.Connected {
		// #122 unchanged, verbatim (see ReleaseBumperName's doc comment for
		// why this is duplicated rather than called): no eviction, no
		// re-key — just the deferred reclaim authorization.
		bumper.reclaimAuthorizedUntil = time.Now().Add(reclaimAuthorizationTTL)
		bumper.ReclaimRequested = false
		log.Printf("[Engine] Bumper name released for reclaim: id=%s, name=%s, expires=%s",
			id, bumper.Name, bumper.reclaimAuthorizedUntil.Format(time.RFC3339))
		safeGo("SaveBumpers", e.SaveBumpers)
		return "", false, true
	}

	// #134 — bumper is connected: evict and re-key, preserving the struct
	// (score/team/history) under a fresh ID the stale one can never resolve
	// to again (contracts/seat-release.md §2, "pourquoi le re-clé est
	// obligatoire"). Deliberately Connected=false and NO ConnEventReconnect
	// (unlike reattachVirtualPlayerUnsafe above): this bumper was NOT just
	// reconnected, it was just evicted — ConnEventDisconnect is the correct
	// transition, matching the codebase's own general rule that every call
	// site flipping CONNECTED fires the matching event (see UpdateBumper's
	// CONNECTED handling a few hundred lines up).
	newID = e.rekeyBumperUnsafe(id, bumper, bumper.Name)
	bumper.Connected = false
	applyConnEventUnsafe(bumper, ConnEventDisconnect)
	bumper.ReclaimRequested = false
	bumper.reclaimAuthorizedUntil = time.Now().Add(reclaimAuthorizationTTL)

	// #134 T2.4 — narrow PREPARE mitigation (planner-20260804-115318.md,
	// "Le point non identifié dans le handoff"): this bumper can never PONG
	// again under its old identity, and updateTeamsReady() would otherwise
	// keep its team permanently un-Ready, blocking PREPARE->READY on a
	// single admin click. Scope strictly limited to THIS bumper — the
	// general "a disconnected participant blocks READY" rule is left
	// untouched (a signal the animateur relies on elsewhere, physical
	// buzzers included). Ready is unconditionally reset to false for every
	// bumper at the start of the NEXT Ready() call, so this can never leak
	// past the current PREPARE window.
	if e.state.Phase == PhasePrepare {
		bumper.Ready = true
	}
	e.updateTeamsReady()

	log.Printf("[Engine] Seat released while connected: old_id=%s, new_id=%s, name=%s", id, newID, bumper.Name)
	safeGo("SaveBumpers", e.SaveBumpers)
	return newID, true, true
}

// countVirtualPlayersUnsafe counts virtual players (caller must hold lock)
func (e *Engine) countVirtualPlayersUnsafe() int {
	count := 0
	for _, b := range e.data.Bumpers {
		if b.IsVirtual {
			count++
		}
	}
	return count
}

// CountVirtualPlayers counts virtual players (thread-safe)
func (e *Engine) CountVirtualPlayers() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.countVirtualPlayersUnsafe()
}

// SyncVirtualPlayerCount synchronizes VirtualPlayerCount with actual bumper count
// This should be called after loading bumpers from disk
func (e *Engine) SyncVirtualPlayerCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount = e.countVirtualPlayersUnsafe()
	log.Printf("[Engine] Virtual player count synchronized: %d", e.state.VirtualPlayerCount)
}

// EnrollmentError represents an enrollment rejection error
type EnrollmentError struct {
	Reason string
}

func (e *EnrollmentError) Error() string {
	return e.Reason
}

// StartEnrollment starts the virtual player enrollment process
func (e *Engine) StartEnrollment(maxPlayers int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// No VJoueur purge here (fix R1 follow-up): a product invariant now holds
	// that a game session is always started clean (see InitGame, which purges
	// the entire VJoueur roster on NEW_GAME) — there is no such thing as a
	// "legacy" VJoueur to clean up here. StartEnrollment can legitimately be
	// re-opened mid-session (e.g. to invite more people), and purging
	// disconnected VJoueurs at that point would evict a temporarily-dropped
	// active player instead of a stale one — NAME_TAKEN rejection alone
	// already fully prevents any collision-related data loss regardless.

	e.state.EnrollmentActive = true
	e.state.ShowQRCode = true
	e.state.VirtualPlayerLimit = maxPlayers
	log.Printf("[Engine] Enrollment started with limit: %d", maxPlayers)
}

// StopEnrollment stops the virtual player enrollment process
func (e *Engine) StopEnrollment() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.EnrollmentActive = false
	e.state.ShowQRCode = false
	log.Printf("[Engine] Enrollment stopped")
}

// HandleVirtualPlayerConnect handles a virtual player connection request
// Returns (bumperID, bumper, error)
func (e *Engine) HandleVirtualPlayerConnect(name, sessionID string) (string, *Bumper, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if enrollment is active
	if !e.state.EnrollmentActive {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_CLOSED"}
	}

	// Check if limit is reached
	virtualCount := e.countVirtualPlayersUnsafe()
	if virtualCount >= e.state.VirtualPlayerLimit {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_FULL"}
	}

	// Validate name (2-20 characters, alphanumeric + spaces)
	if len(name) < 2 || len(name) > 20 {
		return "", nil, &EnrollmentError{Reason: "INVALID_NAME"}
	}

	// Check if name is already taken
	for _, bumper := range e.data.Bumpers {
		if bumper.Name == name && bumper.IsVirtual {
			return "", nil, &EnrollmentError{Reason: "PSEUDO_TAKEN"}
		}
	}

	// Generate unique ID using sessionID
	id := sessionID
	if id == "" {
		id = "vplayer_" + name + "_" + time.Now().Format("20060102_150405")
	}

	// Create virtual bumper (VPlayer = true for virtual players, allows all QCM colors)
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		IsVPlayer: true, // VPlayer can answer all QCM colors
		Status:    "READY",
	}

	e.data.Bumpers[id] = bumper
	e.state.VirtualPlayerCount++

	log.Printf("[Engine] Virtual player connected: id=%s, name=%s, sessionID=%s", id, name, sessionID)
	return id, bumper, nil
}

// reconnectVPlayer reconnects an existing virtual player
func (e *Engine) reconnectVPlayer(sessionID string) (string, *Bumper, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find existing bumper with this sessionID
	for id, bumper := range e.data.Bumpers {
		if bumper.IsVirtual && id == sessionID {
			log.Printf("[Engine] Virtual player reconnected: id=%s, name=%s", id, bumper.Name)
			return id, bumper, nil
		}
	}

	return "", nil, &EnrollmentError{Reason: "SESSION_NOT_FOUND"}
}

// AssignVirtualPlayer assigns a virtual player to a team and answer color
func (e *Engine) AssignVirtualPlayer(bumperID, team string, answerColor AnswerColor) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, exists := e.data.Bumpers[bumperID]
	if !exists {
		return &EnrollmentError{Reason: "BUMPER_NOT_FOUND"}
	}

	if !bumper.IsVirtual {
		return &EnrollmentError{Reason: "NOT_VIRTUAL_PLAYER"}
	}

	// Check if team exists
	if _, exists := e.data.Teams[team]; !exists {
		return &EnrollmentError{Reason: "TEAM_NOT_FOUND"}
	}

	// Assign team and answer color (only physical buzzers get color, VPlayers respond to all)
	oldTeam := bumper.Team
	bumper.Team = team
	if oldTeam != team {
		e.syncConnStateForTeamChangeUnsafe(bumper, oldTeam)
	}
	if !bumper.IsVPlayer {
		bumper.AnswerColor = answerColor
	}
	// VPlayers keep AnswerColor empty (they respond to all colors)

	log.Printf("[Engine] Virtual player assigned: id=%s, team=%s, color=%s, isVPlayer=%v", bumperID, team, answerColor, bumper.IsVPlayer)

	// Save bumpers to disk
	safeGo("SaveBumpers", e.SaveBumpers)

	return nil
}

// GetEnrollmentStatus returns enrollment active status
func (e *Engine) GetEnrollmentStatus() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.EnrollmentActive
}

// SetMemoryParticipatingTeams sets the teams participating in a Memory game
func (e *Engine) SetMemoryParticipatingTeams(teams []string) error {
	e.mu.Lock()

	// Allow setting teams during PREPARE or READY phases (before game starts)
	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		e.mu.Unlock()
		return &MemoryError{Reason: "NOT_IN_PREPARE_OR_READY_PHASE"}
	}

	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemory {
		e.mu.Unlock()
		return &MemoryError{Reason: "NOT_MEMORY_QUESTION"}
	}

	// Get memory mode (default to SOLO if empty)
	memoryMode := e.state.Question.MemoryMode
	if memoryMode == "" {
		memoryMode = string(MemoryModeSolo)
	}

	// Note: We don't validate team count here - allow incremental selection
	// The validation for minimum 2 teams happens at game START, not during selection

	// Validate all teams exist
	for _, teamName := range teams {
		if _, exists := e.data.Teams[teamName]; !exists {
			e.mu.Unlock()
			return &MemoryError{Reason: "TEAM_NOT_FOUND"}
		}
	}

	// Initialize Memory multi-team state
	e.state.MemoryParticipatingTeams = teams
	if len(teams) > 0 {
		e.state.MemoryCurrentTeam = teams[0]
		// Set current team color
		if team, exists := e.data.Teams[teams[0]]; exists {
			e.state.MemoryCurrentTeamColor = team.Color
		}
	}
	e.state.MemoryTeamPairs = make(map[string]int)
	e.state.MemoryTeamErrors = make(map[string]int)
	e.state.MemoryPairOwners = make(map[int]string)
	for _, teamName := range teams {
		e.state.MemoryTeamPairs[teamName] = 0
		e.state.MemoryTeamErrors[teamName] = 0
	}

	log.Printf("[Engine] Memory participating teams set: %v, current team: %s, mode: %s", teams, e.state.MemoryCurrentTeam, memoryMode)

	// #172 B3: the selection just changed — re-check whether PREPARE↔READY
	// should flip (e.g. removing a team from a conform READY selection sends
	// the round back to PREPARE; restoring it returns to READY without
	// repeating the PONG wait). See reevaluatePrepareReadyUnsafe.
	newPhase := e.reevaluatePrepareReadyUnsafe()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil && newPhase != "" {
		callback(newPhase)
	}

	return nil
}

// rotateToNextTeam rotates to the next team in multi-team Memory mode
// Must be called with lock held
func (e *Engine) rotateToNextTeam() {
	if len(e.state.MemoryParticipatingTeams) == 0 {
		return
	}

	// Find current team index
	currentIndex := -1
	for i, team := range e.state.MemoryParticipatingTeams {
		if team == e.state.MemoryCurrentTeam {
			currentIndex = i
			break
		}
	}

	// Calculate next index (circular rotation)
	nextIndex := (currentIndex + 1) % len(e.state.MemoryParticipatingTeams)
	e.state.MemoryCurrentTeam = e.state.MemoryParticipatingTeams[nextIndex]

	// Update current team color
	if team, exists := e.data.Teams[e.state.MemoryCurrentTeam]; exists {
		e.state.MemoryCurrentTeamColor = team.Color
	}

	log.Printf("[Engine] Memory team rotated: %s → %s (index %d)",
		e.state.MemoryParticipatingTeams[currentIndex],
		e.state.MemoryCurrentTeam,
		nextIndex)
}

// MemoryError represents a Memory-specific error
type MemoryError struct {
	Reason string
}

func (e *MemoryError) Error() string {
	return e.Reason
}

// ============================================================
// MEMOTION — v5.0.0
// ============================================================

// motionDifficultyPoints maps card difficulty (1|2|3) to points awarded.
var motionDifficultyPoints = map[int]int{
	1: 1,
	2: 3,
	3: 5,
}

// motionCardPoints returns the points for a card difficulty.
// It uses MotionConfig from the current question if configured, otherwise falls back to motionDifficultyPoints.
func (e *Engine) motionCardPoints(difficulty int) int {
	if e.state.Question != nil && e.state.Question.MotionConfig != nil {
		cfg := e.state.Question.MotionConfig
		switch difficulty {
		case 1:
			if cfg.Points1Star > 0 {
				return cfg.Points1Star
			}
		case 2:
			if cfg.Points2Star > 0 {
				return cfg.Points2Star
			}
		case 3:
			if cfg.Points3Star > 0 {
				return cfg.Points3Star
			}
		}
	}
	return motionDifficultyPoints[difficulty]
}

// motionCardPointsForOutcome computes the points to award for card given a
// type's outcome — units realised and unitsTotal realisable (MEMOTION_DONE.
// UNITS for units under FIXED/PER_UNIT — #184 B-B5, contract §6.2; both
// server-derived for a MEMORY card under STARS_PRORATA — #187, contract
// §9.3). The SOLE reader of MotionCard.PointsRule in this codebase.
//
// Mode resolution, in priority order (#187 cycle 5):
//  1. card.PointsRule.Mode, if explicitly set — absolute priority, contract
//     §6.3. A MEMORY card can still be scored tout-ou-rien by setting STARS
//     or FIXED explicitly.
//  2. Otherwise, the active card's TypeDescriptor.DefaultPointsRule
//     (question_types.go registry), resolved fresh from
//     card.EffectiveType() on every call — never a value written onto the
//     card itself. See that field's doc comment for why: POINTS_RULE is a
//     CARD field that survives a TYPE change verbatim, so writing a
//     type-specific default onto the card (client-side at creation, or
//     server-side at save) would leave a stale, silently-wrong rule behind
//     after the card's TYPE is later changed. Resolving from the registry
//     at read time has no such staleness — a card is always scored under
//     ITS CURRENT type's default.
//  3. Otherwise (no registry entry, or its default is ""), PointsRuleModeStars
//     — the pre-#184 star-based scale (motionCardPoints), unchanged.
//
// ⚠️ Signature changed by #187 (card, units) → (card, units, unitsTotal) —
// declared host modification, contract §10.2: #184 had anticipated PER_UNIT
// as MEMORY's landing mode, not a prorata of the card's own total. The
// change stays within the host's own points vocabulary (§6.1) — no MEMORY
// knowledge (e.g. "a pair") enters this function or this file: it queries
// the registry generically, exactly like MediaSlots/OwnedFields elsewhere.
func (e *Engine) motionCardPointsForOutcome(card *MotionCard, units, unitsTotal int) int {
	mode := PointsRuleModeStars
	value := 0
	if card.PointsRule != nil && card.PointsRule.Mode != "" {
		// Explicit override on the card — absolute priority (contract
		// §6.3), regardless of what the type's own default would be.
		mode = card.PointsRule.Mode
		value = card.PointsRule.Value
	} else if d, ok := TypeDescriptorFor(card.EffectiveType()); ok && d.DefaultPointsRule != "" {
		// #187 cycle 5 — the type's own default (registry, contract §7),
		// resolved fresh from card.EffectiveType() on every call: a card
		// that changed TYPE is re-resolved from its CURRENT type, never
		// from a stale value written earlier onto the card (see
		// TypeDescriptor.DefaultPointsRule's doc comment for the trap this
		// avoids). No `if card.EffectiveType() == QuestionTypeMemory`
		// anywhere — this stays agnostic of which type declared a default.
		mode = d.DefaultPointsRule
	}
	switch mode {
	case PointsRuleModeFixed:
		if units > 0 {
			return value
		}
		return 0
	case PointsRuleModePerUnit:
		return value * units
	case PointsRuleModeStarsProrata:
		// #187 contract §6.2 — normative order of operations: multiply
		// BEFORE dividing. Precomputing a "value per unit"
		// (motionCardPoints(...)/unitsTotal first) truncates to 0 in
		// integer arithmetic whenever unitsTotal exceeds the card's own
		// point value (e.g. 5 points / 8 pairs), making the whole card
		// worth 0 no matter how many units were realised. Multiplying
		// first guarantees a complete grid (units==unitsTotal) always
		// rewards the card's exact nominal value, with no cumulative
		// rounding loss.
		if unitsTotal <= 0 {
			return 0
		}
		return e.motionCardPoints(card.Difficulty) * units / unitsTotal
	default: // STARS
		return e.motionCardPoints(card.Difficulty)
	}
}

// initMotionStateUnsafe initialises MEMOTION card states for the current question.
// All cards are set to "UNPLAYED". MotionSubPhase is set to "MEMORIZE" when
// MotionMemorizeDuration > 0 (Secret Mode), otherwise "GRID" (standard mode).
// In Secret Mode, CurrentTime and Delay are also initialised to MotionMemorizeDuration
// so that the first broadcastUpdate (fired right after this call) already carries the
// correct countdown value — preventing any residual flash from a previous question.
// Must be called with e.mu held (write).
func (e *Engine) initMotionStateUnsafe() {
	// Secret mode: start with MEMORIZE subphase if duration is configured
	if e.state.Question != nil && e.state.Question.MotionMemorizeDuration > 0 {
		e.state.MotionSubPhase = MotionSubPhaseMemorize
		// Initialise CurrentTime synchronously so the first broadcast reflects the
		// correct MEMORIZE countdown — StartMotionMemorizeTimer is called after the
		// broadcast and would be too late to set this for the first frame.
		e.state.CurrentTime = e.state.Question.MotionMemorizeDuration
		e.state.Delay = e.state.Question.MotionMemorizeDuration
	} else {
		e.state.MotionSubPhase = MotionSubPhaseGrid
		e.state.CurrentTime = 0 // clear any residual from a previous question
	}
	e.state.MotionSelected = ""
	e.state.MotionCardStates = make(map[string]MotionCardState)
	e.state.MotionActive = MotionActive{State: map[string]interface{}{}}
	if e.state.Question != nil {
		for _, card := range e.state.Question.MotionCards {
			e.state.MotionCardStates[card.ID] = MotionCardStateUnplayed
		}
	}
}

// InitMotionState initialises MEMOTION card states (exported for direct use in tests).
func (e *Engine) InitMotionState() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.initMotionStateUnsafe()
}

// SelectMotionCard transitions a card from UNPLAYED to SELECTED and sets the sub-phase to SELECTED.
// The card is displayed fullscreen (RECTO face) on the TV, no timer starts here.
// Preconditions: Phase==STARTED, MotionSubPhase=="GRID", card must be UNPLAYED.
func (e *Engine) SelectMotionCard(cardID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != MotionSubPhaseGrid {
		return &MotionError{Reason: "NOT_IN_GRID_SUBPHASE"}
	}
	st, exists := e.state.MotionCardStates[cardID]
	if !exists {
		return &MotionError{Reason: "CARD_NOT_FOUND"}
	}
	if st != MotionCardStateUnplayed {
		return &MotionError{Reason: "CARD_NOT_UNPLAYED"}
	}

	// #184 B-B2: refuse selecting a card whose declared TYPE isn't a known,
	// nestable type (registry, question_types.go) — belt-and-suspenders
	// alongside the upload-time check in http.go (handleUploadQuestion),
	// since a card's TYPE could in principle become non-nestable after it
	// was saved (registry change) or reach here via a non-HTTP path.
	effectiveType := QuestionTypeSpeedy
	if e.state.Question != nil {
		for i := range e.state.Question.MotionCards {
			if e.state.Question.MotionCards[i].ID == cardID {
				effectiveType = e.state.Question.MotionCards[i].EffectiveType()
				if !IsNestableInMotionCard(effectiveType) {
					return &MotionError{Reason: "CARD_TYPE_NOT_NESTABLE"}
				}
				break
			}
		}
	}

	e.state.MotionCardStates[cardID] = MotionCardStateSelected
	e.state.MotionSelected = cardID
	e.state.MotionSubPhase = MotionSubPhaseSelected
	// #184 B-B4: (re)initialise the active-card slot — contract §5.1, reset
	// at every MEMOTION_SELECT, State starts empty (the type's own
	// handlers populate it, none do yet in v7.0.0).
	e.state.MotionActive = MotionActive{CardID: cardID, Type: effectiveType, State: map[string]interface{}{}}
	e.motionCardRoundClosed = false // #187 cycle 4, B3 — a freshly selected card's round is always open

	log.Printf("[Engine] MEMOTION SelectMotionCard: cardID=%s → SELECTED", cardID)
	return nil
}

// FlipMotionCard transitions the selected card from SELECTED to QUESTION.
// Preconditions: Phase==STARTED, MotionSubPhase=="SELECTED".
// The caller (main.go) is responsible for starting the per-card timer after this call.
func (e *Engine) FlipMotionCard() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != MotionSubPhaseSelected {
		return &MotionError{Reason: "NOT_IN_SELECTED_SUBPHASE"}
	}

	cardID := e.state.MotionSelected
	if cardID == "" {
		return &MotionError{Reason: "NO_CARD_SELECTED"}
	}
	if e.state.MotionCardStates[cardID] != MotionCardStateSelected {
		return &MotionError{Reason: "CARD_NOT_IN_SELECTED_STATE"}
	}

	e.state.MotionCardStates[cardID] = MotionCardStateQuestion
	e.state.MotionSubPhase = MotionSubPhaseQuestion
	e.motionCardRoundClosed = false // #187 cycle 4, B3 — belt-and-suspenders with SelectMotionCard's own reset

	log.Printf("[Engine] MEMOTION FlipMotionCard: cardID=%s → QUESTION", cardID)
	return nil
}

// RevealMotionCard transitions the current card from QUESTION to REVEAL.
// Preconditions: Phase==STARTED, MotionSubPhase=="QUESTION".
func (e *Engine) RevealMotionCard() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != MotionSubPhaseQuestion {
		return &MotionError{Reason: "NOT_IN_QUESTION_SUBPHASE"}
	}

	cardID := e.state.MotionSelected
	e.revealMotionCardUnsafe()

	log.Printf("[Engine] MEMOTION RevealMotionCard: cardID=%s → REVEAL", cardID)
	return nil
}

// revealMotionCardUnsafe transitions the active card from QUESTION to
// REVEAL — the state mutation shared by RevealMotionCard (explicit,
// animateur-triggered via MEMOTION_REVEAL) and processMotionCardTick's own
// auto-reveal of an expired MEMORY card (#187 QUALIF bugfix, below). Must
// be called with e.mu held; caller is responsible for the precondition
// checks (Phase==STARTED, MotionSubPhase==QUESTION) where they apply.
func (e *Engine) revealMotionCardUnsafe() {
	cardID := e.state.MotionSelected
	if cardID != "" {
		e.state.MotionCardStates[cardID] = MotionCardStateRevealed
	}
	e.state.MotionSubPhase = MotionSubPhaseReveal
}

// DoneMotionCard marks the active card as DONE, optionally awards points to winnerTeam,
// rotates the team if needed, and returns to the GRID sub-phase.
// units is the type's outcome (MEMOTION_DONE.UNITS, #184 B-B5, contract
// §6/§9.3) — the caller must resolve an absent UNITS to 1 before calling
// (see main.go's handleMotionDone), so this function's own default (a
// units==0 zero value) is never accidentally the "not specified" case.
// Consumed only by card.PointsRule.MODE == FIXED/PER_UNIT
// (motionCardPointsForOutcome); ignored under the default STARS scale,
// which is why 1 as a "no-op" default is correct here too — units is
// simply unused in that branch.
// Returns (pointsAwarded, isComplete, error).
func (e *Engine) DoneMotionCard(cardID string, winnerTeam string, units int) (int, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return 0, false, &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != MotionSubPhaseQuestion && e.state.MotionSubPhase != MotionSubPhaseReveal && e.state.MotionSubPhase != MotionSubPhaseSelected {
		return 0, false, &MotionError{Reason: "INVALID_SUBPHASE"}
	}

	// Cancellation from SELECTED subphase: reset card to UNPLAYED, return to GRID.
	// Use e.state.MotionSelected (server-authoritative) rather than client-supplied cardID.
	if e.state.MotionSubPhase == MotionSubPhaseSelected {
		e.state.MotionCardStates[e.state.MotionSelected] = MotionCardStateUnplayed
		e.state.MotionSelected = ""
		e.state.MotionSubPhase = MotionSubPhaseGrid
		e.state.MotionActive = MotionActive{State: map[string]interface{}{}} // #184 B-B4: emptied on return to GRID
		log.Printf("[Engine] MEMOTION DoneMotionCard: SELECTED → cancelled, cardID=%s back to UNPLAYED", cardID)
		return 0, false, nil
	}

	// Validate card exists
	if _, exists := e.state.MotionCardStates[cardID]; !exists {
		return 0, false, &MotionError{Reason: "CARD_NOT_FOUND"}
	}

	// Mark card as DONE
	e.state.MotionCardStates[cardID] = MotionCardStateDone

	// Award points and record winner if winnerTeam is provided
	points := 0
	if winnerTeam != "" {
		// Find the card to compute points via its own POINTS_RULE (#184
		// B-B5) — STARS (absent/default) still goes through
		// motionCardPoints/difficulty exactly as before.
		if e.state.Question != nil {
			for i := range e.state.Question.MotionCards {
				card := &e.state.Question.MotionCards[i]
				if card.ID == cardID {
					effUnits, unitsTotal := units, 0
					if card.EffectiveType() == QuestionTypeMemory {
						// #187 contract §9.3 — the server is sole authority
						// on a MEMORY card's outcome: derive Units AND
						// UnitsTotal from the card's own active grid state,
						// ignoring whatever UNITS the client sent. With
						// STARS_PRORATA as the default MODE, Units *is* the
						// score — leaving it to the caller would recreate,
						// in this new mechanism, exactly the dette #12 owes
						// to close.
						effUnits, unitsTotal = e.memoryCardOutcomeUnsafe(card)
					}
					pts := e.motionCardPointsForOutcome(card, effUnits, unitsTotal)
					ok := pts > 0
					if ok {
						points = pts
					}
					break
				}
			}
		}
		if points > 0 {
			// UpdateTeamScore requires mu.Lock — we hold it here, so use unsafe version
			team, ok := e.data.Teams[winnerTeam]
			if ok {
				team.TeamPoints += points
				e.recalculateTeamScoreUnsafe(winnerTeam)
				log.Printf("[Engine] MEMOTION DoneMotionCard: team=%s +%dpts", winnerTeam, points)
			}
		}
		e.state.MotionCardTeams[cardID] = winnerTeam
	}

	// Team rotation based on question mode
	motionMode := ""
	if e.state.Question != nil {
		motionMode = e.state.Question.MotionMode
		if motionMode == "" {
			motionMode = string(MemoryModeSolo)
		}
	}

	switch motionMode {
	case string(MemoryModeChacunSonTour):
		// Rotate after every card regardless of outcome
		e.rotateMotionTeam()
	case string(MemoryModeTantQueJeGagne):
		// Rotate if no winner; keep if winner
		if winnerTeam == "" {
			e.rotateMotionTeam()
		}
		// MemoryModeSolo: no rotation
	}

	// Return to grid
	e.state.MotionSubPhase = MotionSubPhaseGrid
	e.state.MotionSelected = ""
	e.state.MotionActive = MotionActive{State: map[string]interface{}{}} // #184 B-B4: emptied on return to GRID

	// Check if all cards are DONE
	isComplete := true
	for _, st := range e.state.MotionCardStates {
		if st != MotionCardStateDone {
			isComplete = false
			break
		}
	}
	if len(e.state.MotionCardStates) == 0 {
		isComplete = false
	}

	log.Printf("[Engine] MEMOTION DoneMotionCard: cardID=%s winner=%s pts=%d complete=%v",
		cardID, winnerTeam, points, isComplete)
	return points, isComplete, nil
}

// SetMotionParticipatingTeams sets the teams participating in a MEMOTION game.
// Mirrors SetMemoryParticipatingTeams — allowed during PREPARE or READY phases.
func (e *Engine) SetMotionParticipatingTeams(teams []string) error {
	e.mu.Lock()

	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		e.mu.Unlock()
		return &MotionError{Reason: "NOT_IN_PREPARE_OR_READY_PHASE"}
	}
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemotion {
		e.mu.Unlock()
		return &MotionError{Reason: "NOT_MEMOTION_QUESTION"}
	}

	// Validate all teams exist
	for _, teamName := range teams {
		if _, exists := e.data.Teams[teamName]; !exists {
			e.mu.Unlock()
			return &MotionError{Reason: "TEAM_NOT_FOUND"}
		}
	}

	e.state.MotionParticipatingTeams = teams
	if len(teams) > 0 {
		e.state.MotionCurrentTeam = teams[0]
		if team, exists := e.data.Teams[teams[0]]; exists {
			e.state.MotionCurrentTeamColor = team.Color
		}
	} else {
		e.state.MotionCurrentTeam = ""
		e.state.MotionCurrentTeamColor = []int{}
	}

	log.Printf("[Engine] MEMOTION SetMotionParticipatingTeams: teams=%v currentTeam=%s",
		teams, e.state.MotionCurrentTeam)

	// #172 B3: re-check PREPARE↔READY now that the selection changed — see
	// reevaluatePrepareReadyUnsafe (same mechanism as SetMemoryParticipatingTeams).
	newPhase := e.reevaluatePrepareReadyUnsafe()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil && newPhase != "" {
		callback(newPhase)
	}

	return nil
}

// RafaleError represents a RAFALE-specific error (v8.0.0, #199) — same
// {Reason string} shape as MotionError/MemoryError, for consistency with
// the two sibling "set participating teams" families this mirrors.
type RafaleError struct {
	Reason string
}

func (e *RafaleError) Error() string {
	return e.Reason
}

// SetRafaleParticipatingTeams sets the RAFALE round's participating teams
// and play order (contract §5.1: "équipes participantes + ordre de
// passage", RAFALE_SET_TEAMS action) — mirrors SetMemoryParticipatingTeams/
// SetMotionParticipatingTeams above. Allowed during PREPARE or READY (before
// START), same as those two. The first team in the list becomes
// RafaleCurrentTeam immediately (so a round that never rotates — SOLO, or
// TANT_QUE_JE_GAGNE staying on a winning streak — still has an active team
// from the very first question); an empty list clears it back to "".
//
// No minimum-team-count gate is imposed here (unlike MEMORY SOLO/multi —
// participantsConform's default case already returns true for RAFALE,
// engine.go:1038): a RAFALE round has never required a participating team
// at all (contract §3.4 SOLO, Batch 2/#107) — this action is additive, not
// a new precondition on reaching READY/START.
func (e *Engine) SetRafaleParticipatingTeams(teams []string) error {
	e.mu.Lock()

	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		e.mu.Unlock()
		return &RafaleError{Reason: "NOT_IN_PREPARE_OR_READY_PHASE"}
	}
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeRafale {
		e.mu.Unlock()
		return &RafaleError{Reason: "NOT_RAFALE_QUESTION"}
	}

	// Validate all teams exist — same discipline as the two sibling
	// functions above.
	for _, teamName := range teams {
		if _, exists := e.data.Teams[teamName]; !exists {
			e.mu.Unlock()
			return &RafaleError{Reason: "TEAM_NOT_FOUND"}
		}
	}

	e.state.RafaleParticipatingTeams = teams
	if len(teams) > 0 {
		e.state.RafaleCurrentTeam = teams[0]
		if team, exists := e.data.Teams[teams[0]]; exists {
			e.state.RafaleCurrentTeamColor = team.Color
		}
	} else {
		e.state.RafaleCurrentTeam = ""
		e.state.RafaleCurrentTeamColor = []int{}
	}

	log.Printf("[Engine] RAFALE SetRafaleParticipatingTeams: teams=%v currentTeam=%s",
		teams, e.state.RafaleCurrentTeam)

	// Same PREPARE↔READY re-check as SetMemoryParticipatingTeams/
	// SetMotionParticipatingTeams — a no-op for RAFALE today
	// (participantsConform's default case, engine.go:1038), kept for
	// consistency with the two sibling functions rather than a
	// special-cased simpler version that would silently diverge if a
	// future contract change ever added a RAFALE participant-count rule.
	newPhase := e.reevaluatePrepareReadyUnsafe()

	// Release lock BEFORE calling callback to avoid deadlock
	stateCallback := e.OnStateChange
	teamsCallback := e.OnRafaleTeamsChanged
	e.mu.Unlock()

	// #199 task 36 — fired unconditionally; the consumer
	// (sendLEDSetRafaleTeams, cmd/server/main.go) gates on Phase==STARTED
	// itself (mirrors sendLEDSetForBuzzerMemory's own phase dispatch — the
	// multi-team grid is a STARTED-only concept, same as MEMORY's), so
	// calling this during PREPARE/READY (this function's only phases) is a
	// harmless no-op today, not a premature LED change.
	if teamsCallback != nil {
		teamsCallback()
	}

	if stateCallback != nil && newPhase != "" {
		stateCallback(newPhase)
	}

	return nil
}

// rotateMotionTeam advances MotionCurrentTeam to the next team in MotionParticipatingTeams.
// Must be called with e.mu held (write).
func (e *Engine) rotateMotionTeam() {
	if len(e.state.MotionParticipatingTeams) == 0 {
		return
	}

	prev := e.state.MotionCurrentTeam
	next, color := rotateTeam(e.state.MotionParticipatingTeams, e.state.MotionCurrentTeam, e.data.Teams)
	e.state.MotionCurrentTeam = next
	e.state.MotionCurrentTeamColor = color

	log.Printf("[Engine] MEMOTION team rotated: %s → %s", prev, next)
}

// StartMotionCardTimer starts a per-card countdown for MEMOTION.
// Unlike startTimer(), expiry does NOT stop the game — the admin can still act.
// If delay <= 0, no timer is started.
func (e *Engine) StartMotionCardTimer(delay int) {
	e.mu.Lock()

	if delay <= 0 {
		log.Printf("[Engine] MEMOTION StartMotionCardTimer: delay<=0, no timer started")
		e.mu.Unlock()
		return
	}

	// Stop any existing timer goroutine first (reuse same infrastructure as startTimer)
	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	// Set timer values
	e.state.CurrentTime = delay
	e.state.Delay = delay

	// Create new stop channel and ticker
	e.stopCh = make(chan struct{})
	e.timer = time.NewTicker(1 * time.Second)

	ticker := e.timer
	stopCh := e.stopCh

	log.Printf("[Engine] MEMOTION StartMotionCardTimer: delay=%ds", delay)
	e.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				// #151: locked section extracted into processMotionCardTick
				// (Lock + `defer` Unlock) so a panic there still releases
				// e.mu before recoverBackgroundPanic runs — see that
				// function's doc comment.
				stop := func() bool {
					defer recoverBackgroundPanic("MEMOTION card timer")
					result := e.processMotionCardTick()
					if result.guardFailed {
						ticker.Stop()
						return true
					}
					// #187 cycle 6 bugfix — PAUSE: leave the ticker running
					// (do NOT ticker.Stop()/return), just skip this tick's
					// broadcast/decrement entirely. See
					// processMotionCardTick's doc comment.
					if result.paused {
						return false
					}

					if e.OnTimerTick != nil {
						e.OnTimerTick(result.currentTime)
					}

					// Call QCM hint callback outside of lock (#185 C-B1) —
					// same pattern as startTimer's goroutine for the
					// question host.
					if result.qcmHintCallback != nil && result.invalidatedColor != "" {
						result.qcmHintCallback(result.invalidatedColor, result.remainingAnswers)
					}

					if result.currentTime <= 0 {
						// Timer expired — do NOT call e.Stop(). Phase stays
						// STARTED, MotionSubPhase stays QUESTION for every
						// card type (#187 cycle 4, B1 — reverted cycle 3's
						// MEMORY-specific auto-reveal here; see
						// processMotionCardTick's doc comment for why).
						// motionCardRoundClosed is set inside
						// processMotionCardTick itself (under lock), so a
						// MEMORY card already stops accepting flips at this
						// point even though its sub-phase hasn't moved —
						// admin must act via MEMOTION_REVEAL.
						ticker.Stop()
						log.Printf("[Engine] MEMOTION card timer expired — phase stays STARTED, admin must act")
						return true
					}
					return false
				}()
				if stop {
					return
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// motionCardTickResult carries the outcome of one locked MEMOTION
// card-timer iteration (processMotionCardTick) out to the unlocked caller.
type motionCardTickResult struct {
	guardFailed bool // phase/subphase changed unexpectedly; caller must stop the ticker and return
	// paused (#187 cycle 6 bugfix, QUALIF v7.1.0.3) — Phase==PAUSED: this
	// tick is a pure no-op, and the caller must NOT stop the ticker for
	// it — see processMotionCardTick's doc comment for why this has to be
	// a distinct outcome from guardFailed.
	paused      bool
	currentTime int
	// qcmHintCallback/invalidatedColor/remainingAnswers (#185 C-B1) — set
	// only when the active card is QCM-typed and a hint threshold was
	// crossed this tick, mirroring timerTickResult's fields for the
	// question host.
	qcmHintCallback  func(string, int)
	invalidatedColor string
	remainingAnswers int
}

// processMotionCardTick applies one MEMOTION card-timer tick under lock.
// Extracted from StartMotionCardTimer's goroutine (#151) so `defer
// e.mu.Unlock()` guarantees the mutex is released even if this panics — see
// recoverBackgroundPanic's doc comment.
//
// #185 C-B1 — the QCM hint branch below is the ONE authorized host
// modification of this batch: contracts/question-types.md §10's
// agnosticity test explicitly anticipated a nested type needing its own
// per-tick logic branched onto the MEMOTION card host, and QCM-in-card
// (the GATE decision behind #185) is exactly that case. No new ticker is
// created — this reuses the per-card timer StartMotionCardTimer already
// starts (e.timer/e.stopCh, Risque R2 of the plan, untouched). The
// decision/invalidation logic itself (qcmHintShouldTrigger,
// qcmInvalidateRandomWrongAnswer, above) is fully host-agnostic — it was
// extracted for exactly this reuse, not written twice.
//
// #187 cycle 6 bugfix (QUALIF v7.1.0.3) — PAUSE during a MEMOTION card's
// own timer used to permanently kill the ticker goroutine: the combined
// guard below (`Phase != STARTED || MotionSubPhase != QUESTION`) treated
// PhasePaused exactly like any other phase change, returning
// guardFailed=true, which makes StartMotionCardTimer's goroutine
// `ticker.Stop()` and RETURN FROM THE GOROUTINE ENTIRELY. Continue()
// (engine.go) only flips Phase back to STARTED — it has no idea a
// MEMOTION card timer even exists, so nothing ever restarts it: the
// countdown stayed frozen forever, exactly the QUALIF report ("le
// décompte ne reprend pas"). Pre-existing since #151 (this ticker
// predates #183-#187 entirely) — not a regression introduced by any
// #187 cycle, simply never exercised by manual QUALIF testing before
// MEMORY-in-card gave pause/resume during a card timer real attention.
//
// Fix: PAUSE gets its OWN, distinct outcome (paused=true) — mirrors
// processTimerTick's question-host handling (`if Phase != STARTED {
// return timerTickResult{} }`, whose caller's `if !result.active {
// return }` leaves the ticker running and simply skips this tick). The
// ticker keeps firing every second while paused, each tick a pure
// no-op; the very next tick after Continue() resumes decrementing
// normally, no separate "resume the card timer" call needed anywhere —
// same mechanism that already made pause/resume work correctly for the
// question-host timer all along.
func (e *Engine) processMotionCardTick() motionCardTickResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	callTestInjectPanic("motion-card")

	if e.state.Phase == PhasePaused {
		return motionCardTickResult{paused: true}
	}

	// Guard: exit if game state changed unexpectedly (STOP, card
	// revealed/done/completed while a stray tick was in flight, ...) —
	// PhasePaused is handled above and never reaches here.
	if e.state.Phase != PhaseStarted || e.state.MotionSubPhase != MotionSubPhaseQuestion {
		return motionCardTickResult{guardFailed: true}
	}

	e.state.CurrentTime--
	currentTime := e.state.CurrentTime
	result := motionCardTickResult{currentTime: currentTime}

	// MotionActive.Type is checked first (cheap, no card lookup) since the
	// overwhelmingly common case is a SPEEDY card, which never needs this
	// branch at all.
	if e.state.MotionActive.Type == QuestionTypeQCM {
		if card := e.activeMotionCardUnsafe(); card != nil {
			invalidated := motionActiveQCMInvalidated(e.state.MotionActive.State)
			if qcmHintShouldTrigger(card.QCMHintsEnabled, card.QCMHintThreshold1, card.QCMHintThreshold2, currentTime, e.state.Delay, len(invalidated)) {
				color, remaining := qcmInvalidateRandomWrongAnswer(card.QCMAnswers, card.QCMCorrect, invalidated)
				if color != "" {
					e.state.MotionActive.State["QCM_INVALIDATED"] = append(invalidated, color)
					result.qcmHintCallback = e.OnQCMHint
					result.invalidatedColor = color
					result.remainingAnswers = remaining
				}
			}
		}
	}

	// #187 cycle 4, B3 — a card's round closes on natural timer expiry,
	// same as an explicit StopMotionCardTimer (MEMOTION_STOP_TIMER). This
	// is deliberately generic (any card type, not just MEMORY): it's a
	// host-level "is this card's timer round still open" fact, gating
	// FlipMotionMemoryCard independently of MotionSubPhase — which stays
	// QUESTION here, unchanged for every type (see below).
	if currentTime <= 0 {
		e.motionCardRoundClosed = true
	}

	// ⚠️ Deliberately NOT auto-revealing here (reverted from an earlier
	// cycle — plan-memotion-v710-memory-reveal-v2-20260824... — do not
	// reintroduce). A MEMORY card's timer expiring on an INCOMPLETE grid
	// requires an explicit animateur MEMOTION_REVEAL: the reveal gesture
	// has real content there (it uncovers pairs nobody found, and is the
	// moment the animateur reads the score before crediting it). This is
	// deliberately ASYMMETRIC with the completed-grid exit
	// (main.go's handleFlipMemoryCard, cardScoped isComplete branch, cycle
	// 2 — unchanged, still auto-reveals): once every pair is found there is
	// nothing left to discover, so requiring a gesture there would be
	// purely ceremonial. A reviewer "harmonizing" the two exits would
	// break behavior the user explicitly validated — same caution as the
	// FLIP_MEMORY_CARD off-turn ignore dérogation (contract §9.2).
	//
	// A previous cycle DID auto-reveal here for MEMORY, discovering along
	// the way that its own notification (OnTimerTick → ActionUpdateTimer)
	// doesn't reach the frontend's live MEMOTION rendering — the reduced
	// UPDATE_TIMER handler only copies phase/timer/countdownTime/gameTime,
	// never MEMOTION_SUBPHASE/MEMOTION_CARD_STATES/MEMOTION_ACTIVE. That
	// discovery stays true and is still a trap for a FUTURE broadcast on
	// this same tick path — just no longer this function's problem, since
	// nothing here mutates MEMOTION state at expiry anymore.

	return result
}

// activeMotionCardUnsafe returns a pointer to the currently active MEMOTION
// card — e.state.Question.MotionCards[i] where ID == e.state.MotionSelected
// (== MotionActive.CardID, contract §4's internal-consistency requirement)
// — or nil if there is none (no MEMOTION round, or no card selected yet).
// Must be called with e.mu held.
func (e *Engine) activeMotionCardUnsafe() *MotionCard {
	if e.state.Question == nil || e.state.MotionSelected == "" {
		return nil
	}
	for i := range e.state.Question.MotionCards {
		if e.state.Question.MotionCards[i].ID == e.state.MotionSelected {
			return &e.state.Question.MotionCards[i]
		}
	}
	return nil
}

// motionActiveQCMInvalidated reads the QCM_INVALIDATED slice from
// MEMOTION_ACTIVE.STATE — the card-scoped equivalent of the top-level
// QcmInvalidated field (contract §5.3: a nested type's live state lives in
// MEMOTION_ACTIVE.STATE, never the flat question-scoped fields). Absent or
// unexpectedly-shaped ⇒ nil (no exclusions yet); never panics.
func motionActiveQCMInvalidated(state map[string]interface{}) []string {
	v, ok := state["QCM_INVALIDATED"]
	if !ok {
		return nil
	}
	s, ok := v.([]string)
	if !ok {
		return nil
	}
	return s
}

// ============================================================
// MEMOTION — MEMORY-typed card (#187, v7.1.0, contract §5.4/§6.3/§7.3)
// ============================================================

// motionActiveMemoryFlippedCards reads MEMORY_FLIPPED_CARDS from
// MEMOTION_ACTIVE.STATE — the card-scoped counterpart of the question-host
// MemoryFlippedCards field (contract §5.4). Absent or unexpectedly-shaped ⇒
// nil; never panics.
func motionActiveMemoryFlippedCards(state map[string]interface{}) []string {
	v, ok := state["MEMORY_FLIPPED_CARDS"]
	if !ok {
		return nil
	}
	s, ok := v.([]string)
	if !ok {
		return nil
	}
	return s
}

// motionActiveMemoryMatchedPairs reads MEMORY_MATCHED_PAIRS from
// MEMOTION_ACTIVE.STATE — the card-scoped counterpart of the question-host
// MemoryMatchedPairs field (contract §5.4). Absent or unexpectedly-shaped ⇒
// nil; never panics.
func motionActiveMemoryMatchedPairs(state map[string]interface{}) []int {
	v, ok := state["MEMORY_MATCHED_PAIRS"]
	if !ok {
		return nil
	}
	s, ok := v.([]int)
	if !ok {
		return nil
	}
	return s
}

// motionActiveMemoryErrors reads MEMORY_ERRORS from MEMOTION_ACTIVE.STATE —
// the card-scoped counterpart of the question-host MemoryErrors field
// (contract §5.4). Absent or unexpectedly-shaped ⇒ 0; never panics.
func motionActiveMemoryErrors(state map[string]interface{}) int {
	v, ok := state["MEMORY_ERRORS"]
	if !ok {
		return 0
	}
	n, ok := v.(int)
	if !ok {
		return 0
	}
	return n
}

// memoryCardOutcomeUnsafe derives (units, unitsTotal) for the active card's
// own MEMORY grid — contract §6.3/§9.3: "le serveur dérive Units ET
// UnitsTotal de son propre état [...] et ignore tout UNITS reçu [du
// client]". Must be called with e.mu held; card must be the card DoneMotionCard
// is closing, already known to be MEMORY-typed (card.EffectiveType() ==
// QuestionTypeMemory) by the caller.
func (e *Engine) memoryCardOutcomeUnsafe(card *MotionCard) (units, unitsTotal int) {
	matched := motionActiveMemoryMatchedPairs(e.state.MotionActive.State)
	return len(matched), len(card.MemoryPairs)
}

// StartMotionMemorizeTimer starts a countdown for the MEMORIZE phase in Secret Mode (v5.5.0).
// At expiry, automatically transitions MotionSubPhase from "MEMORIZE" to "GRID".
// If duration <= 0, the call is a no-op (standard mode compatibility).
func (e *Engine) StartMotionMemorizeTimer(duration int) {
	e.mu.Lock()

	if duration <= 0 {
		log.Printf("[Engine] MEMOTION StartMotionMemorizeTimer: duration<=0, no timer started")
		e.mu.Unlock()
		return
	}

	// Stop any existing timer goroutine first (reuse same infrastructure as StartMotionCardTimer)
	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	// Set timer values
	e.state.CurrentTime = duration
	e.state.Delay = duration

	// Create new stop channel and ticker
	e.stopCh = make(chan struct{})
	e.timer = time.NewTicker(1 * time.Second)

	ticker := e.timer
	stopCh := e.stopCh

	log.Printf("[Engine] MEMOTION StartMotionMemorizeTimer: duration=%ds", duration)
	e.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				// #151: locked section extracted into
				// processMotionMemorizeTick (Lock + `defer` Unlock) so a
				// panic there still releases e.mu before
				// recoverBackgroundPanic runs — see that function's doc
				// comment.
				stop := func() bool {
					defer recoverBackgroundPanic("MEMOTION memorize timer")
					result := e.processMotionMemorizeTick()
					switch {
					case result.guardFailed:
						ticker.Stop()
						return true
					case result.paused:
						// #187 cycle 6 bugfix — leave the ticker running,
						// skip this tick's broadcast/decrement entirely.
						// See processMotionMemorizeTick's doc comment.
						return false
					case result.expired:
						ticker.Stop()
						log.Printf("[Engine] MEMOTION MEMORIZE timer expired → transitioned to GRID")
						if result.callback != nil {
							result.callback(PhaseStarted)
						}
						return true
					default:
						// Full state broadcast on each tick so the TV display receives the
						// updated CurrentTime immediately (matches StartMotionCardTimer pattern).
						if e.OnStateChange != nil {
							e.OnStateChange(PhaseStarted)
						}
						return false
					}
				}()
				if stop {
					return
				}

			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// motionMemorizeTickResult carries the outcome of one locked MEMOTION
// memorize-timer iteration (processMotionMemorizeTick) out to the unlocked
// caller, which stops the ticker / invokes OnStateChange as needed.
type motionMemorizeTickResult struct {
	guardFailed bool // phase/subphase changed unexpectedly; caller must stop the ticker and return
	// paused (#187 cycle 6 bugfix — same root cause and fix as
	// motionCardTickResult.paused, applied here proactively: the MEMORIZE
	// countdown shares the identical vulnerable guard pattern, and PAUSE
	// is reachable during it the same way it is during QUESTION).
	paused   bool
	expired  bool            // CurrentTime reached 0; MotionSubPhase was transitioned to GRID under lock
	callback func(GamePhase) // OnStateChange captured under lock, to invoke after unlock when expired
}

// processMotionMemorizeTick applies one MEMOTION memorize-timer tick under
// lock: decrements CurrentTime and, on expiry, transitions MotionSubPhase
// from MEMORIZE to GRID. Extracted from StartMotionMemorizeTimer's goroutine
// (#151) so `defer e.mu.Unlock()` guarantees the mutex is released even if
// this panics — see recoverBackgroundPanic's doc comment.
//
// #187 cycle 6 bugfix (QUALIF v7.1.0.3) — see processMotionCardTick's doc
// comment for the full root-cause writeup (same bug, same fix, found by
// auditing every ticker sharing this guard pattern while fixing the
// reported one): PhasePaused used to fall into the combined guard below
// exactly like a genuine state change, permanently killing this ticker —
// Continue() has no idea a MEMORIZE countdown even exists, so nothing
// would ever have resumed it.
func (e *Engine) processMotionMemorizeTick() motionMemorizeTickResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	callTestInjectPanic("motion-memorize")

	if e.state.Phase == PhasePaused {
		return motionMemorizeTickResult{paused: true}
	}

	// Guard: exit if game state changed unexpectedly — PhasePaused is
	// handled above and never reaches here.
	if e.state.Phase != PhaseStarted || e.state.MotionSubPhase != MotionSubPhaseMemorize {
		return motionMemorizeTickResult{guardFailed: true}
	}

	e.state.CurrentTime--
	currentTime := e.state.CurrentTime

	if currentTime <= 0 {
		// Timer expired: transition MEMORIZE → GRID automatically
		e.state.MotionSubPhase = MotionSubPhaseGrid
		e.state.MotionSelected = ""
		e.state.MotionActive = MotionActive{State: map[string]interface{}{}} // #184 B-B4: emptied on return to GRID (no-op here — never set during MEMORIZE — kept for consistency with every other GRID transition)
		e.state.CurrentTime = 0
		return motionMemorizeTickResult{expired: true, callback: e.OnStateChange}
	}

	return motionMemorizeTickResult{}
}

// StopMotionMemorizeTimer stops the MEMORIZE phase timer without changing the game phase.
// Resets CurrentTime to 0. Safe to call even if no timer is running.
func (e *Engine) StopMotionMemorizeTimer() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	e.state.CurrentTime = 0
	log.Printf("[Engine] MEMOTION StopMotionMemorizeTimer: timer stopped, CURRENT_TIME=0")
}

// StopMotionCardTimer stops the per-card timer without changing the game phase.
// Resets CurrentTime to 0. Safe to call even if no timer is running.
func (e *Engine) StopMotionCardTimer() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	// Reset timer display without changing game phase
	e.state.CurrentTime = 0
	// #187 cycle 4, B3 — an explicitly stopped timer closes the active
	// card's round (MEMOTION_STOP_TIMER, MEMOTION_REVEAL, MEMOTION_DONE,
	// and the card-scoped grid-complete path all route through here) —
	// same effect as natural expiry (processMotionCardTick), gating
	// FlipMotionMemoryCard.
	e.motionCardRoundClosed = true

	log.Printf("[Engine] MEMOTION StopMotionCardTimer: timer stopped, CURRENT_TIME=0")
}

// MotionError represents a MEMOTION-specific error
type MotionError struct {
	Reason string
}

func (e *MotionError) Error() string {
	return e.Reason
}

// ValidateCardScope enforces contracts/question-types.md §9.2's invariant:
// an action scoped to a MEMOTION card (protocol.CardScope.MotionCardID,
// passed here as motionCardID — game can't import protocol, which already
// imports game) may only apply to the card currently active in a MEMOTION
// round. motionCardID == "" means the caller's payload carried no
// MOTION_CARD_ID at all (contract §9.1: the field is optional and
// omitempty on the wire).
//
// Generalizes the contract table's two named rows ("aucune manche MEMOTION
// en cours" / "manche MEMOTION, emplacement actif") to every subphase by
// keying on whether a card is actually selected (MotionSelected != ""),
// which subsumes both: SUBPHASE=="" and SUBPHASE running GRID/MEMORIZE
// (round in progress, nothing selected yet) both have MotionSelected=="",
// and are treated identically — no active card, so no MOTION_CARD_ID is
// expected. The table itself only names the two extremes explicitly; this
// is the natural reading of its own stated purpose ("empêche une action
// typée de s'appliquer à une carte qui n'est pas celle en jeu").
//
// Posed and tested ahead of any real consumer — no v7.0.0 action actually
// carries CardScope (see CardScope's doc comment); #186 is the first.
func (e *Engine) ValidateCardScope(motionCardID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state.MotionSelected == "" {
		if motionCardID != "" {
			return &MotionError{Reason: "CARD_SCOPE_UNEXPECTED"}
		}
		return nil
	}

	if motionCardID != e.state.MotionSelected {
		return &MotionError{Reason: "CARD_SCOPE_MISMATCH"}
	}
	return nil
}

// FlipMotionMemoryCard flips one face-down card in a MEMOTION card's own
// MEMORY grid — the card-scoped counterpart of FlipMemoryCard (contract
// §5.4/§6.3/§7.3). motionCardID must already have been accepted by
// ValidateCardScope by the caller (main.go); this method re-derives the
// same "is this really the active card" fact itself rather than trusting
// the caller's earlier check, since nothing prevents the active card from
// changing between that check and this call in a concurrent server.
//
// Unlike the question host (FlipMemoryCard), state lives in
// MotionActive.State — never the question-scoped Memory* fields (contract
// §5.4) — and there is NEVER a team rotation: a MEMORY card is played by
// exactly one team, MotionCurrentTeam, and MEMORY_MODE/rotateToNextTeam
// have no meaning in this context (contract §6.3). Returns the same 4-tuple
// shape as FlipMemoryCard for the caller's convenience (main.go shares the
// post-flip broadcast/LED/scheduling logic across both hosts); isComplete
// signals "all pairs found", NOT "the round is over" — the caller must hand
// control back to the MEMOTION card's REVEAL sub-phase, never call Stop()
// (contract note, plan-memotion-v710-memory-20260824-154844.md §5 tâche 4).
func (e *Engine) FlipMotionMemoryCard(motionCardID, cardID string) (isMatch, shouldFlipBack bool, flipDelay int, isComplete bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return false, false, 0, false
	}
	// Playable per contract §4's HostContext table: MOTION_SUBPHASE==QUESTION.
	if e.state.MotionSubPhase != MotionSubPhaseQuestion {
		return false, false, 0, false
	}
	if motionCardID == "" || e.state.MotionSelected != motionCardID {
		return false, false, 0, false
	}
	// #187 cycle 4, B3 — MotionSubPhase alone isn't enough to gate a flip:
	// it stays QUESTION even after this card's own timer has expired (the
	// asymmetric-exit design, processMotionCardTick's doc comment). A
	// closed round refuses further flips regardless of sub-phase, until an
	// explicit MEMOTION_REVEAL/next SelectMotionCard reopens one.
	if e.motionCardRoundClosed {
		return false, false, 0, false
	}
	card := e.activeMotionCardUnsafe()
	if card == nil || card.EffectiveType() != QuestionTypeMemory {
		return false, false, 0, false
	}

	pairID := e.extractPairID(cardID)
	if pairID == 0 {
		return false, false, 0, false
	}

	matched := motionActiveMemoryMatchedPairs(e.state.MotionActive.State)
	for _, m := range matched {
		if m == pairID {
			return false, false, 0, false
		}
	}

	flipped := motionActiveMemoryFlippedCards(e.state.MotionActive.State)
	for _, id := range flipped {
		if id == cardID {
			return false, false, 0, false
		}
	}
	if len(flipped) >= 2 {
		return false, false, 0, false
	}

	flipped = append(flipped, cardID)
	e.state.MotionActive.State["MEMORY_FLIPPED_CARDS"] = flipped

	if len(flipped) == 1 {
		return false, false, 0, false
	}

	firstCardID := flipped[0]
	secondCardID := flipped[1]
	firstPairID := e.extractPairID(firstCardID)
	secondPairID := e.extractPairID(secondCardID)

	// Own FLIP_DELAY (contract §6.3: points-related MEMORY_CONFIG settings
	// are neutralised in card context, but FLIP_DELAY is not one of them —
	// it stays active).
	flipDelay = 3000
	if card.MemoryConfig != nil && card.MemoryConfig.FlipDelay > 0 {
		flipDelay = int(card.MemoryConfig.FlipDelay * 1000)
	}

	if firstPairID == secondPairID {
		matched = append(matched, firstPairID)
		e.state.MotionActive.State["MEMORY_MATCHED_PAIRS"] = matched
		e.state.MotionActive.State["MEMORY_FLIPPED_CARDS"] = []string{}

		totalPairs := len(card.MemoryPairs)
		isComplete = len(matched) >= totalPairs && totalPairs > 0

		log.Printf("[Engine] MEMOTION memory card MATCH! cardId=%s pair %d found. Total matched: %d/%d. Complete: %v",
			motionCardID, firstPairID, len(matched), totalPairs, isComplete)
		return true, false, 0, isComplete
	}

	errs := motionActiveMemoryErrors(e.state.MotionActive.State)
	e.state.MotionActive.State["MEMORY_ERRORS"] = errs + 1

	log.Printf("[Engine] MEMOTION memory card NO MATCH (error #%d, cardId=%s). Cards %s and %s will flip back after %dms",
		errs+1, motionCardID, firstCardID, secondCardID, flipDelay)
	return false, true, flipDelay, false
}

// ClearMotionMemoryFlippedCards resets the flipped-cards slot of a MEMOTION
// card's own MEMORY grid after the flip-back delay — the card-scoped
// counterpart of ClearMemoryFlippedCards. motionCardID is the card identity
// the caller captured at scheduling time (plan Risque R2): if the active
// card has since changed — a different card selected, or the round moved
// on — this is a no-op. It must NEVER mutate a card that isn't still the
// one it was scheduled for, or it would blank out the NEXT card's flipped
// cards instead of the one whose delay actually elapsed.
//
// Never rotates a team, unlike the question host's ClearMemoryFlippedCards
// — contract §6.3, MEMORY rotation has no meaning inside a MEMOTION card.
func (e *Engine) ClearMotionMemoryFlippedCards(motionCardID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if motionCardID == "" || e.state.MotionSelected != motionCardID {
		log.Printf("[Engine] MEMOTION memory auto-flip-back skipped: active card changed (scheduled for %s)", motionCardID)
		return
	}
	e.state.MotionActive.State["MEMORY_FLIPPED_CARDS"] = []string{}
	log.Printf("[Engine] MEMOTION memory card flipped cards cleared (cardId=%s)", motionCardID)
}
