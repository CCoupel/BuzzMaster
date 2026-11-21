package game

import (
	"log"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

// ROUE « Roue de la Fortune » (v12.0.0, #144, contracts/roue.md §4–§7): the
// round engine. A ROUE question is a normal question played from START to the
// resolution of its phrase; the inside of the round is carried by
// GameState.RoueSubPhase while the phase stays STARTED. Everything below runs
// under e.mu (write); the timers (maximum spin, braking, flip) are cancellable
// by generation; every public action releases the lock BEFORE calling the
// callbacks (OnRoueChange / OnRoueNotice).

// ROUE sub-phases (GameState.RoueSubPhase, contract §4/§5).
const (
	RoueSubTurn      = "TURN"
	RoueSubSpinning  = "SPINNING"
	RoueSubStopping  = "STOPPING"
	RoueSubLetter    = "LETTER"
	RoueSubLit       = "LIT"
	RoueSubRevealing = "REVEALING"
	RoueSubAnswer    = "ANSWER"
	RoueSubTurnEnd   = "TURN_END"
	RoueSubSolved    = "SOLVED"
)

// ROUE_NEXT hands and the advised button (contract §5.2).
const (
	RoueHandSame = "SAME"
	RoueHandNext = "NEXT"
)

// ROUE_ERROR codes (contract §8.3) — RoueError.Code.
const (
	RoueErrWrongSubPhase     = "WRONG_SUBPHASE"
	RoueErrNotAllowed        = "NOT_ALLOWED"
	RoueErrVowelNotAvailable = "VOWEL_NOT_AVAILABLE"
	RoueErrNotALetter        = "NOT_A_LETTER"
	RoueErrInvalidTeams      = "INVALID_TEAMS"
	RoueErrNotFullSpeed      = "NOT_FULL_SPEED" // ROUE_STOP (admin or animateur) during the climb (contract §5.1)
)

// ROUE_LAST_EVENT kinds (contract §4).
const (
	RoueEvSpin         = "SPIN"
	RoueEvLandValue    = "LAND_VALUE"
	RoueEvBankrupt     = "BANKRUPT"
	RoueEvLoseTurn     = "LOSE_TURN"
	RoueEvLetterHit    = "LETTER_HIT"
	RoueEvLetterMiss   = "LETTER_MISS"
	RoueEvLetterRepeat = "LETTER_REPEAT"
	RoueEvFlip         = "FLIP"
	RoueEvAnswerOK     = "ANSWER_OK"
	RoueEvTurnEnd      = "TURN_END"
	RoueEvPass         = "PASS"
	RoueEvReveal       = "REVEAL"
	RoueEvBonus        = "BONUS" // v12.1.0 #308
)

// ROUE notices (OnRoueNotice) — what the application layer turns into sounds,
// comet LEDs and score flashes. Kind is one of the RoueNotice* values.
const (
	RoueNoticeBad    = "BAD"    // letter absent / repeated, BANQUEROUTE: the existing "mauvaise réponse" sound
	RoueNoticeCredit = "CREDIT" // a team was credited (Team, Points): the existing gain sound + comet + flash
	RoueNoticeBonus  = "BONUS"  // v12.1.0 #308: stop on a BONUS case — the existing "bonne réponse" sound + comet, no flash
)

// RoueNotice is one side effect of a transition for the application layer.
type RoueNotice struct {
	Kind   string
	Team   string
	Points int
}

type roueFx struct {
	notices []RoueNotice
	saved   bool // teams / history changed: persist after unlock
}

// roueRuntime is the ephemeral, never serialised state of the running round.
type roueRuntime struct {
	rows      []string // full board, folded, one string per line
	revealed  map[rune]bool
	lit       rune // letter whose cases are lit (step 4), 0 if none
	solved    bool
	gainsRule string
	mode      string

	// Timers: kind of the ARMED timer ("" = none); gen invalidates any callback
	// already in flight; remaining holds the time left while the game is paused.
	timer     *time.Timer
	timerKind string
	gen       int
	due       time.Time
	running   bool
	remaining time.Duration

	stopDuration time.Duration
}

const (
	roueTimerUp   = "up" // climb to full speed (SPIN_UP_MS), then the maximum duration is armed
	roueTimerMax  = "max"
	roueTimerStop = "stop"
	roueTimerFlip = "flip"
)

// roueGenSeq is the engine-wide generation counter of the ROUE timers: a
// generation is never reused, even by the runtime of a later round, so a
// callback still in flight from a previous round can never match a new one.
var roueGenSeq int64

func nextRoueGen() int { return int(atomic.AddInt64(&roueGenSeq, 1)) }

func (rt *roueRuntime) cancelTimer() {
	if rt.timer != nil {
		rt.timer.Stop()
		rt.timer = nil
	}
	rt.gen = nextRoueGen()
	rt.timerKind = ""
	rt.running = false
	rt.remaining = 0
}

// SetRoueWheelParams sets the wheel configuration used at the next START
// (called by the application layer from game-config.json). Invalid
// parameters fall back to the defaults.
func (e *Engine) SetRoueWheelParams(p RoueWheelParams) {
	np, _, err := NormalizeRoueWheelParams(p)
	if err != nil {
		np, _, _ = NormalizeRoueWheelParams(DefaultRoueWheelParams())
	}
	e.mu.Lock()
	e.roueParams = np
	e.mu.Unlock()
}

// SetRoueSpinUpMs shortens the climb of the CURRENT round (tests only: the real value is 2 × SPIN_PERIOD_MS, computed
// at START, and the period is clamped to 1 000 ms at least). It takes effect at the next ROUE_SPIN.
func (e *Engine) SetRoueSpinUpMs(ms int) {
	e.mu.Lock()
	e.state.RoueWheel.SpinUpMs = ms
	e.mu.Unlock()
}

// SetRoueRand injects the random source of the wheel draw (tests): f(n)
// returns an int in [0, n).
func (e *Engine) SetRoueRand(f func(n int) int) {
	e.mu.Lock()
	e.roueRand = f
	e.mu.Unlock()
}

func (e *Engine) roueRandN(n int) int {
	if n <= 0 {
		return 0
	}
	if e.roueRand != nil {
		return e.roueRand(n) % n
	}
	return rand.Intn(n)
}

func (e *Engine) isRoueQuestionUnsafe() bool {
	return e.state.Question != nil && e.state.Question.Type == QuestionTypeRoue
}

// resetRoueStateUnsafe clears the whole ROUE state (Ready, stopUnsafe,
// InitGameWith): timers cancelled, every field back to its empty value.
func (e *Engine) resetRoueStateUnsafe() {
	if e.roue != nil {
		e.roue.cancelTimer()
		e.roue = nil
	}
	st := &e.state
	st.RoueSubPhase = ""
	st.RoueStep = 0
	st.RoueTeams = []string{}
	st.RoueCurrentTeam = ""
	st.RoueCurrentTeamColor = []int{}
	st.RoueNextTeam = ""
	st.RoueNextDefault = ""
	st.RoueTeamBank = map[string]int{}
	st.RoueTurnGain = 0
	st.RoueBoard = RoueBoard{Cols: RoueBoardCols, Rows: []string{}}
	st.RoueLetters = []RoueLetter{}
	st.RoueShowLetters = false
	st.RoueConsonantsLeft = false
	st.RoueWheel = RoueWheel{Segments: []RoueSegment{}, Current: -1}
	st.RoueLastEvent = RoueEvent{}
	st.RoueWinner = ""
	st.RoueAwards = map[string]int{}
	e.roueSeq = 0
}

func roueStepOf(sub string) int {
	switch sub {
	case RoueSubTurn:
		return 1
	case RoueSubSpinning:
		return 2
	case RoueSubStopping:
		return 3
	case RoueSubLetter:
		return 4
	case RoueSubLit, RoueSubRevealing:
		return 5
	case RoueSubAnswer:
		return 6
	case RoueSubTurnEnd:
		return 7
	}
	return 0
}

func (e *Engine) roueSetSubUnsafe(sub string) {
	if sub != RoueSubSpinning {
		e.state.RoueWheel.FullSpeed = false // true only in SPINNING, once the climb is over (contract §4)
	}
	e.state.RoueSubPhase = sub
	e.state.RoueStep = roueStepOf(sub)
}

func (e *Engine) roueEventUnsafe(kind, team, letter string, count, value, gain int) {
	e.roueSeq++
	e.state.RoueLastEvent = RoueEvent{Seq: e.roueSeq, Kind: kind, Team: team, Letter: letter, Count: count, Value: value, Gain: gain}
}

// roueRefreshBoardUnsafe rebuilds ROUE_BOARD: a letter not yet revealed is
// ONLY ever "_" (or "*" while lit) — never the letter itself (contract §9).
func (e *Engine) roueRefreshBoardUnsafe() {
	rt := e.roue
	rows := make([]string, len(rt.rows))
	for i, row := range rt.rows {
		var b strings.Builder
		for _, r := range row {
			switch {
			case r >= 'A' && r <= 'Z':
				switch {
				case rt.solved || rt.revealed[r]:
					b.WriteRune(r)
				case rt.lit == r:
					b.WriteByte('*')
				default:
					b.WriteByte('_')
				}
			default:
				b.WriteRune(r)
			}
		}
		rows[i] = b.String()
	}
	e.state.RoueBoard = RoueBoard{Cols: RoueBoardCols, Rows: rows}
}

func (e *Engine) roueRefreshConsonantsLeftUnsafe() {
	proposed := map[rune]bool{}
	for _, l := range e.state.RoueLetters {
		for _, r := range l.Letter {
			proposed[r] = true
		}
	}
	left := false
	for _, row := range e.roue.rows {
		for _, r := range row {
			if r >= 'A' && r <= 'Z' && !IsRoueVowel(r) && !proposed[r] {
				left = true
			}
		}
	}
	e.state.RoueConsonantsLeft = left
}

func (e *Engine) roueSetCurrentTeamUnsafe(team string) {
	e.state.RoueCurrentTeam = team
	if t, ok := e.data.Teams[team]; ok {
		e.state.RoueCurrentTeamColor = t.Color
	} else {
		e.state.RoueCurrentTeamColor = []int{}
	}
}

func (e *Engine) roueModeUnsafe() string {
	if e.roue != nil && e.roue.mode != "" {
		return e.roue.mode
	}
	return RoueModeTantQueJeGagne
}

func (e *Engine) roueSoloUnsafe() bool { return len(e.state.RoueTeams) <= 1 }

func (e *Engine) roueNextTeamUnsafe() string {
	if e.roueSoloUnsafe() {
		return ""
	}
	next, _ := rotateTeam(e.state.RoueTeams, e.state.RoueCurrentTeam, e.data.Teams)
	return next
}

// roueAdviceUnsafe computes ROUE_NEXT_TEAM / ROUE_NEXT_DEFAULT (contract §5.2,
// table): the server only ADVISES a button, the admin or the animateur decides.
func (e *Engine) roueAdviceUnsafe(found bool) {
	e.state.RoueNextTeam = e.roueNextTeamUnsafe()
	switch {
	case e.roueSoloUnsafe(), e.roueModeUnsafe() == RoueModeSolo:
		e.state.RoueNextDefault = RoueHandSame
	case e.roueModeUnsafe() == RoueModeChacunSonTour:
		e.state.RoueNextDefault = RoueHandNext
	case found:
		e.state.RoueNextDefault = RoueHandSame
	default:
		e.state.RoueNextDefault = RoueHandNext
	}
}

func (e *Engine) roueBankUnsafe(team string, add int) {
	bank := make(map[string]int, len(e.state.RoueTeamBank)+1)
	for k, v := range e.state.RoueTeamBank {
		bank[k] = v
	}
	bank[team] += add
	e.state.RoueTeamBank = bank
}

// roueEnterTurnUnsafe starts a turn for the team that has the hand.
func (e *Engine) roueEnterTurnUnsafe() {
	e.roueSetSubUnsafe(RoueSubTurn)
	e.state.RoueTurnGain = 0
	e.state.RoueNextTeam = ""
	e.state.RoueNextDefault = ""
	e.state.RoueWheel.Stop = nil
	e.roue.lit = 0
	e.roueRefreshBoardUnsafe()
}

// roueEnterTurnEndUnsafe ends the turn (step 7): the turn gain joins the
// team's pot, the advised button is computed.
//
// emit: the TURN_END event is only the "last event" when the turn ends by the
// gesture itself (ANSWER → ROUE_NEXT); when the turn ends on a BANQUEROUTE, a
// TOUR PERDU or a missed letter, that more informative event stays the last one
// (the sub-phase TURN_END carries the step).
func (e *Engine) roueEnterTurnEndUnsafe(found, emit bool) {
	team := e.state.RoueCurrentTeam
	gain := e.state.RoueTurnGain
	e.roueBankUnsafe(team, gain)
	e.state.RoueTurnGain = 0
	e.roueSetSubUnsafe(RoueSubTurnEnd)
	e.roueAdviceUnsafe(found)
	if emit {
		e.roueEventUnsafe(RoueEvTurnEnd, team, "", 0, 0, gain)
	}
}

// roueApplyHandUnsafe gives the hand per the admin's choice (SAME keeps it,
// NEXT rotates), then enters the next turn.
func (e *Engine) roueApplyHandUnsafe(hand string) {
	if hand == RoueHandNext {
		e.roueSetCurrentTeamUnsafe(e.roueNextTeamUnsafe())
	}
	e.roueEnterTurnUnsafe()
}

// startRoueRoundUnsafe prepares the round at START (actualStart /
// StartImmediate). A no-op for any other question type.
func (e *Engine) startRoueRoundUnsafe() {
	e.resetRoueTransientUnsafe()
	if !e.isRoueQuestionUnsafe() {
		return
	}
	q := e.state.Question
	layout, err := RoueBuildBoard(q.RouePhrase)
	if err != nil {
		log.Printf("[Engine] ROUE: phrase of question %s refused at start: %v", q.ID, err)
		return
	}
	params := e.roueParams
	if e.RoueWheelParamsFn != nil {
		params = e.RoueWheelParamsFn()
	}
	if params.MaxPoints == 0 {
		params = DefaultRoueWheelParams()
	}
	params, _, perr := NormalizeRoueWheelParams(params)
	if perr != nil {
		params, _, _ = NormalizeRoueWheelParams(DefaultRoueWheelParams())
	}
	params.BonusCount = q.RoueBonusCount
	if params.BonusCount < 0 {
		params.BonusCount = 0
	}
	if max := RoueMaxBonusFor(params); params.BonusCount > max { // configuration changed since the question was saved: never refuse the start
		log.Printf("[Engine] ROUE: question %s asks %d BONUS cases, reduced to %d to stay within %d cases", q.ID, params.BonusCount, max, RoueMaxSegments)
		params.BonusCount = max
	}
	segs := BuildRoueWheel(params)
	rt := &roueRuntime{
		rows: layout.Rows, revealed: map[rune]bool{},
		gainsRule: q.RoueGainsRule, mode: q.RoueMode,
		stopDuration: time.Duration(params.StopDurationMs) * time.Millisecond,
	}
	if rt.gainsRule == "" {
		rt.gainsRule = RoueGainsWinnerOnly
	}
	if rt.mode == "" {
		rt.mode = RoueModeTantQueJeGagne
	}
	e.roue = rt
	wl := q.RoueWheelLayout
	if wl == "" {
		wl = RafaleWheelLayoutStrip
	}

	st := &e.state
	st.RoueTeams = append([]string{}, st.RoueTeams...)
	st.RoueTeamBank = map[string]int{}
	for _, t := range st.RoueTeams {
		st.RoueTeamBank[t] = 0
	}
	st.RoueLetters = []RoueLetter{}
	st.RoueShowLetters = q.EffectiveRoueShowLetters()
	st.RoueWinner = ""
	st.RoueAwards = map[string]int{}
	st.RoueWheel = RoueWheel{
		Layout: wl, SpinPeriodMs: params.SpinPeriodMs, SpinMaxMs: params.SpinMaxMs, SpinUpMs: RoueSpinUpMs(params.SpinPeriodMs),
		FlipIntervalMs: params.FlipIntervalMs, Segments: segs, Current: -1,
	}
	first := ""
	if len(st.RoueTeams) > 0 {
		first = st.RoueTeams[0]
	}
	e.roueSetCurrentTeamUnsafe(first)
	e.roueRefreshConsonantsLeftUnsafe()
	e.roueEnterTurnUnsafe()
	e.roueSeq = 0
	e.state.RoueLastEvent = RoueEvent{}
}

// resetRoueTransientUnsafe drops a previous round's runtime (timers) without
// touching the selected teams (chosen in PREPARE / READY).
func (e *Engine) resetRoueTransientUnsafe() {
	if e.roue != nil {
		e.roue.cancelTimer()
		e.roue = nil
	}
}

// ---- timers ----------------------------------------------------------------

func (e *Engine) roueArmUnsafe(kind string, d time.Duration) {
	rt := e.roue
	rt.cancelTimer()
	gen := rt.gen
	rt.timerKind = kind
	rt.due = time.Now().Add(d)
	rt.running = true
	rt.timer = time.AfterFunc(d, func() { e.roueTimerFired(gen, kind) })
}

// rouePauseTimersUnsafe suspends the mode's timers (PAUSE, contract §5.1).
func (e *Engine) rouePauseTimersUnsafe() {
	rt := e.roue
	if rt == nil || !rt.running || rt.timerKind == "" {
		return
	}
	rem := time.Until(rt.due)
	if rem < 0 {
		rem = 0
	}
	if rt.timer != nil {
		rt.timer.Stop()
		rt.timer = nil
	}
	rt.gen = nextRoueGen() // a callback already in flight is invalidated
	rt.running = false
	rt.remaining = rem
}

// roueResumeTimersUnsafe resumes the suspended timer (CONTINUE).
func (e *Engine) roueResumeTimersUnsafe() {
	rt := e.roue
	if rt == nil || rt.running || rt.timerKind == "" {
		return
	}
	kind, rem := rt.timerKind, rt.remaining
	e.roueArmUnsafe(kind, rem)
}

func (e *Engine) roueTimerFired(gen int, kind string) {
	defer recoverBackgroundPanic("roueTimerFired")
	e.mu.Lock()
	rt := e.roue
	if rt == nil || gen != rt.gen || e.state.Phase != PhaseStarted || !e.isRoueQuestionUnsafe() {
		e.mu.Unlock()
		return
	}
	rt.timer = nil
	rt.running = false
	rt.timerKind = ""
	var fx roueFx
	switch kind {
	case roueTimerUp:
		// End of the climb: FULL_SPEED and the maximum duration armed in the SAME locked section, so there is no
		// window where a stop is lost (contract §5.1). The UPDATE comes from the fireRoueFx below.
		if e.state.RoueSubPhase == RoueSubSpinning {
			e.state.RoueWheel.FullSpeed = true
			e.roueArmUnsafe(roueTimerMax, time.Duration(e.state.RoueWheel.SpinMaxMs)*time.Millisecond)
		}
	case roueTimerMax:
		if e.state.RoueSubPhase == RoueSubSpinning {
			e.roueTriggerStopUnsafe()
		}
	case roueTimerStop:
		if e.state.RoueSubPhase == RoueSubStopping {
			e.roueFinishStoppingUnsafe(&fx)
		}
	case roueTimerFlip:
		if e.state.RoueSubPhase == RoueSubRevealing {
			e.roueSetSubUnsafe(RoueSubAnswer)
			e.roueAdviceUnsafe(true)
		}
	}
	e.mu.Unlock()
	e.fireRoueFx(fx)
}

// ---- transitions -----------------------------------------------------------

// roueTriggerStopUnsafe: the stop was requested (buzz of the team that has the
// hand, ROUE_STOP, or the maximum spin duration). The SERVER draws the case,
// uniformly (contract §6), at THIS moment, whichever the trigger.
func (e *Engine) roueTriggerStopUnsafe() {
	rt := e.roue
	rt.cancelTimer()
	segs := e.state.RoueWheel.Segments
	idx := e.roueRandN(len(segs))
	e.state.RoueWheel.Stop = &RoueStop{To: idx, DurationMs: int(rt.stopDuration / time.Millisecond)}
	e.roueSetSubUnsafe(RoueSubStopping)
	e.roueArmUnsafe(roueTimerStop, rt.stopDuration)
}

// roueFinishStoppingUnsafe applies the effect of the case, at the END of the
// braking (never at the requested stop).
func (e *Engine) roueFinishStoppingUnsafe(fx *roueFx) {
	w := &e.state.RoueWheel
	stop := w.Stop
	w.Stop = nil
	if stop == nil || stop.To < 0 || stop.To >= len(w.Segments) {
		e.roueEnterTurnUnsafe()
		return
	}
	w.Current = stop.To
	seg := w.Segments[stop.To]
	team := e.state.RoueCurrentTeam
	switch seg.Kind {
	case RoueKindBankrupt:
		lost := e.state.RoueTeamBank[team]
		e.roueBankUnsafe(team, -lost)
		e.state.RoueTurnGain = 0
		e.roueEventUnsafe(RoueEvBankrupt, team, "", 0, lost, 0) // VALUE = the pot lost (contract 5a4f70fe)
		fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBad, Team: team})
		e.roueEnterTurnEndUnsafe(false, false)
	case RoueKindLoseTurn:
		e.roueEventUnsafe(RoueEvLoseTurn, team, "", 0, 0, 0)
		e.roueEnterTurnEndUnsafe(false, false)
	case RoueKindBonus:
		e.roueBonusStopUnsafe(team, fx)
	default:
		e.roueEventUnsafe(RoueEvLandValue, team, "", 0, seg.Value, 0)
		e.roueSetSubUnsafe(RoueSubLetter)
	}
}

// roueButtonUnsafe: a BUTTON press during a ROUE question. Only the team that
// has the hand, during SPINNING, stops the wheel; every other press is ignored
// with no effect (contract §5.1).
func (e *Engine) roueButtonUnsafe(bumperID string) {
	if e.roue == nil || e.state.RoueSubPhase != RoueSubSpinning {
		return
	}
	if !e.state.RoueWheel.FullSpeed {
		return // during the climb a press is ignored in silence and not remembered (contract §5.1)
	}
	b, ok := e.data.Bumpers[bumperID]
	if !ok || b == nil || b.Team == "" || b.Team != e.state.RoueCurrentTeam {
		return
	}
	e.roueTriggerStopUnsafe()
}

// ---- public actions --------------------------------------------------------

func roueErr(code, msg string) error { return &RoueError{Code: code, Message: msg} }

// roueAction runs fn under e.mu with the common guards (STARTED, ROUE
// question) and fires the callbacks AFTER the unlock.
func (e *Engine) roueAction(fn func(fx *roueFx) error) error {
	e.mu.Lock()
	if e.state.Phase != PhaseStarted || !e.isRoueQuestionUnsafe() || e.roue == nil {
		e.mu.Unlock()
		return roueErr(RoueErrNotAllowed, "aucune manche ROUE en cours")
	}
	var fx roueFx
	err := fn(&fx)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	e.fireRoueFx(fx)
	return nil
}

func (e *Engine) fireRoueFx(fx roueFx) {
	if fx.saved {
		safeGo("SaveTeams", e.SaveTeams)
		safeGo("SaveHistory", e.SaveHistory)
	}
	if e.OnRoueNotice != nil {
		for _, n := range fx.notices {
			e.OnRoueNotice(n)
		}
	}
	if e.OnRoueChange != nil {
		e.OnRoueChange()
	}
}

// fireRoueChange is the post-unlock callback for paths that have no fx
// (button press, timers firing outside an action).
func (e *Engine) fireRoueChange() { e.fireRoueFx(roueFx{}) }

// RoueSpin — ROUE_SPIN: step 1 → 2, the wheel spins.
func (e *Engine) RoueSpin() error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase != RoueSubTurn || !e.state.RoueConsonantsLeft {
			return roueErr(RoueErrWrongSubPhase, "la roue ne peut pas être lancée maintenant")
		}
		e.state.RoueWheel.Stop = nil
		e.roueSetSubUnsafe(RoueSubSpinning)
		e.roueEventUnsafe(RoueEvSpin, e.state.RoueCurrentTeam, "", 0, 0, 0)
		e.state.RoueWheel.FullSpeed = false
		if up := e.state.RoueWheel.SpinUpMs; up > 0 {
			e.roueArmUnsafe(roueTimerUp, time.Duration(up)*time.Millisecond) // the maximum duration starts at full speed
		} else { // no climb configured (direct engine use): full speed at once
			e.state.RoueWheel.FullSpeed = true
			e.roueArmUnsafe(roueTimerMax, time.Duration(e.state.RoueWheel.SpinMaxMs)*time.Millisecond)
		}
		return nil
	})
}

// RoueStop — ROUE_STOP (« Forcer l'arrêt »), admin or animateur: refused with NOT_FULL_SPEED during the climb, state
// unchanged (contract roue.md §5.1, decision 2026-11-17 — no exception, the admin included).
func (e *Engine) RoueStop() error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase != RoueSubSpinning {
			return roueErr(RoueErrWrongSubPhase, "la roue ne tourne pas")
		}
		if !e.state.RoueWheel.FullSpeed {
			return roueErr(RoueErrNotFullSpeed, "la roue prend de la vitesse")
		}
		e.roueTriggerStopUnsafe() // cancels the maximum timer
		return nil
	})
}

// RoueLetter — ROUE_LETTER: the team's consonant (step 4).
func (e *Engine) RoueLetter(letter string) error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase != RoueSubLetter {
			return roueErr(RoueErrWrongSubPhase, "aucune consonne n'est attendue")
		}
		folded := rafaleFold(strings.TrimSpace(letter))
		runes := []rune(folded)
		if len(runes) != 1 || runes[0] < 'A' || runes[0] > 'Z' {
			return roueErr(RoueErrNotALetter, "ce n'est pas une lettre")
		}
		l := runes[0]
		if IsRoueVowel(l) {
			return roueErr(RoueErrVowelNotAvailable, "voyelle non disponible")
		}
		team := e.state.RoueCurrentTeam
		ls := string(l)
		for _, p := range e.state.RoueLetters {
			if p.Letter == ls {
				e.roueEventUnsafe(RoueEvLetterRepeat, team, ls, 0, 0, 0)
				fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBad, Team: team})
				e.roueEnterTurnEndUnsafe(false, false)
				return nil
			}
		}
		count := 0
		for _, row := range e.roue.rows {
			for _, r := range row {
				if r == l {
					count++
				}
			}
		}
		hit := count > 0
		e.state.RoueLetters = append(append([]RoueLetter{}, e.state.RoueLetters...), RoueLetter{Letter: ls, Team: team, Hit: hit})
		e.roueRefreshConsonantsLeftUnsafe()
		if !hit {
			e.roueEventUnsafe(RoueEvLetterMiss, team, ls, 0, 0, 0)
			fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBad, Team: team})
			e.roueEnterTurnEndUnsafe(false, false)
			return nil
		}
		value := 0
		if cur := e.state.RoueWheel.Current; cur >= 0 && cur < len(e.state.RoueWheel.Segments) {
			value = e.state.RoueWheel.Segments[cur].Value
		}
		e.roue.lit = l
		e.state.RoueTurnGain = count * value
		e.roueRefreshBoardUnsafe()
		e.roueSetSubUnsafe(RoueSubLit)
		e.roueEventUnsafe(RoueEvLetterHit, team, ls, count, value, count*value)
		return nil
	})
}

// RoueFlip — ROUE_FLIP (« Retourner les cases »): the lit cases turn over one
// by one, then the step ends by itself (timer).
func (e *Engine) RoueFlip() error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase != RoueSubLit {
			return roueErr(RoueErrWrongSubPhase, "aucune case éclairée")
		}
		rt := e.roue
		letter := rt.lit
		count := 0
		for _, row := range rt.rows {
			for _, r := range row {
				if r == letter {
					count++
				}
			}
		}
		rt.revealed[letter] = true
		rt.lit = 0
		e.roueRefreshBoardUnsafe()
		e.roueSetSubUnsafe(RoueSubRevealing)
		e.roueEventUnsafe(RoueEvFlip, e.state.RoueCurrentTeam, string(letter), count, 0, 0)
		e.roueArmUnsafe(roueTimerFlip, time.Duration(count*e.state.RoueWheel.FlipIntervalMs)*time.Millisecond)
		return nil
	})
}

// RoueAnswer — ROUE_ANSWER (« Réponse validée »): the phrase is solved.
func (e *Engine) RoueAnswer() error {
	return e.roueAction(func(fx *roueFx) error {
		sub := e.state.RoueSubPhase
		if sub != RoueSubAnswer && !(sub == RoueSubTurn && !e.state.RoueConsonantsLeft) {
			return roueErr(RoueErrWrongSubPhase, "aucune réponse n'est attendue")
		}
		team := e.state.RoueCurrentTeam
		e.roueBankUnsafe(team, e.state.RoueTurnGain)
		e.state.RoueTurnGain = 0
		e.roueSolveUnsafe(team, fx)
		return nil
	})
}

// roueSolveUnsafe ends the round on a validated answer: the winner is the team
// that has the hand, the pots are credited per the gains rule (contract §7).
func (e *Engine) roueSolveUnsafe(winner string, fx *roueFx) {
	rt := e.roue
	rt.cancelTimer()
	rt.solved = true
	rt.lit = 0
	e.roueRefreshBoardUnsafe()
	e.roueSetSubUnsafe(RoueSubSolved)
	e.state.RoueNextTeam, e.state.RoueNextDefault = "", ""
	e.state.RoueStep = 0
	e.state.RoueWinner = winner
	e.roueEventUnsafe(RoueEvAnswerOK, winner, "", 0, 0, e.state.RoueTeamBank[winner])

	awards := map[string]int{}
	for _, t := range e.state.RoueTeams {
		pot := e.state.RoueTeamBank[t]
		if pot <= 0 {
			continue
		}
		if rt.gainsRule == RoueGainsAllTeams || t == winner {
			awards[t] = pot
		}
	}
	e.state.RoueAwards = awards
	var gains gainsAcc
	var credited []string
	for _, t := range e.state.RoueTeams { // rotation order, deterministic
		pts, ok := awards[t]
		if !ok || !e.roueCreditTeamUnsafe(t, pts) {
			continue
		}
		credited = append(credited, t)
		gains.add(t, pts)
		fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeCredit, Team: t, Points: pts})
		fx.saved = true
	}
	if len(credited) > 0 {
		e.recordScoreChangeUnsafe(true, credited, nil, gains.list)
	}
}

// roueCreditTeamUnsafe credits a team at team level, same path as TEAM_POINTS
// (team score, POINTS_AWARDED history event with the question's category).
func (e *Engine) roueCreditTeamUnsafe(name string, points int) bool {
	team, ok := e.data.Teams[name]
	if !ok || team == nil {
		return false
	}
	q := e.state.Question
	team.TeamPoints += points
	e.recalculateTeamScoreUnsafe(name)
	var meta CategoryMeta
	if e.RoueCategoryMeta != nil && q != nil {
		meta = e.RoueCategoryMeta(string(q.Category))
	}
	ev := GameEvent{
		Timestamp: time.Now().UnixMicro(), EventType: "POINTS_AWARDED",
		CategoryDisplayName: meta.Name, CategoryImageURL: meta.ImageURL, CategoryColor: meta.Color,
		WinnerID: name, WinnerName: name, WinnerType: "TEAM", TeamName: name, TeamColor: team.Color, Points: points,
	}
	if q != nil {
		ev.QuestionID, ev.QuestionText, ev.QuestionCategory = q.ID, q.Question, string(q.Category)
	}
	e.history = append(e.history, ev)
	return true
}

// RoueNext — ROUE_NEXT {HAND}: end of the turn (step 7). SAME = « Tour
// suivant » (the same team plays again), NEXT = « Équipe suivante ».
func (e *Engine) RoueNext(hand string) error {
	return e.roueAction(func(fx *roueFx) error {
		if hand != RoueHandSame && hand != RoueHandNext {
			return roueErr(RoueErrNotAllowed, "HAND doit valoir SAME ou NEXT")
		}
		sub := e.state.RoueSubPhase
		noConsonant := sub == RoueSubTurn && !e.state.RoueConsonantsLeft
		if sub != RoueSubTurnEnd && sub != RoueSubAnswer && !noConsonant {
			return roueErr(RoueErrWrongSubPhase, "la fin de tour n'est pas atteinte")
		}
		if hand == RoueHandNext && e.roueSoloUnsafe() {
			return roueErr(RoueErrNotAllowed, "« Équipe suivante » est refusé en SOLO")
		}
		if sub == RoueSubAnswer { // no validated answer: the turn ends with this very gesture
			e.roueEnterTurnEndUnsafe(true, true)
		} else if noConsonant { // nothing left to find: the hand moves, the turn ends with this gesture
			e.roueEventUnsafe(RoueEvTurnEnd, e.state.RoueCurrentTeam, "", 0, 0, 0)
		}
		e.roueApplyHandUnsafe(hand)
		return nil
	})
}

// RoucePass — ROUE_PASS: the hand passes at once, the pot is kept (admin only).
func (e *Engine) RoucePass() error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase != RoueSubTurn {
			return roueErr(RoueErrWrongSubPhase, "on ne peut passer la main qu'au début du tour")
		}
		if e.roueSoloUnsafe() {
			return roueErr(RoueErrNotAllowed, "passer la main est refusé en SOLO")
		}
		team := e.state.RoueCurrentTeam
		e.roueSetCurrentTeamUnsafe(e.roueNextTeamUnsafe())
		e.roueEnterTurnUnsafe()
		e.roueEventUnsafe(RoueEvPass, team, "", 0, 0, 0)
		return nil
	})
}

// RoueReveal — ROUE_REVEAL (« Dévoiler »): SOLVED with no winner, no credit,
// the whole board visible.
func (e *Engine) RoueReveal() error {
	return e.roueAction(func(fx *roueFx) error {
		if e.state.RoueSubPhase == RoueSubSolved || e.state.RoueSubPhase == "" {
			return roueErr(RoueErrWrongSubPhase, "la phrase est déjà résolue")
		}
		rt := e.roue
		rt.cancelTimer()
		rt.solved = true
		rt.lit = 0
		e.state.RoueWheel.Stop = nil
		e.state.RoueTurnGain = 0
		e.roueRefreshBoardUnsafe()
		e.roueSetSubUnsafe(RoueSubSolved)
		e.state.RoueStep = 0
		e.state.RoueNextTeam, e.state.RoueNextDefault = "", ""
		e.state.RoueWinner = ""
		e.roueEventUnsafe(RoueEvReveal, "", "", 0, 0, 0)
		return nil
	})
}

// RoueSetTeams — ROUE_SET_TEAMS (PREPARE / READY): participating teams in
// rotation order; an empty list clears the selection. Only existence and
// duplicates are checked here: the picker sends the list at every click, so a
// count rule (SOLO = 1, others >= 2) would make two teams unreachable. The count
// is carried by participantsConform / roueTeamsConform alone (PREPARE→READY and
// FORCE_READY), like RAFALE_SET_TEAMS.
func (e *Engine) RoueSetTeams(teams []string) error {
	e.mu.Lock()
	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		e.mu.Unlock()
		return roueErr(RoueErrNotAllowed, "la sélection des équipes se fait avant le départ")
	}
	if !e.isRoueQuestionUnsafe() {
		e.mu.Unlock()
		return roueErr(RoueErrNotAllowed, "la question courante n'est pas une question ROUE")
	}
	seen := map[string]bool{}
	for _, t := range teams {
		if _, ok := e.data.Teams[t]; !ok || seen[t] {
			e.mu.Unlock()
			return roueErr(RoueErrInvalidTeams, "équipe inconnue ou en double")
		}
		seen[t] = true
	}
	e.state.RoueTeams = append([]string{}, teams...)
	if len(teams) > 0 {
		e.roueSetCurrentTeamUnsafe(teams[0])
	} else {
		e.state.RoueCurrentTeam = ""
		e.state.RoueCurrentTeamColor = []int{}
	}
	newPhase := e.reevaluatePrepareReadyUnsafe()
	stateCallback := e.OnStateChange
	e.mu.Unlock()
	if stateCallback != nil && newPhase != "" {
		stateCallback(newPhase)
	}
	e.fireRoueChange()
	return nil
}

// RoueTeamsConform reports whether the selected teams satisfy the mode.
func roueTeamsConform(q *Question, count int) bool {
	mode := q.RoueMode
	if mode == "" {
		mode = RoueModeTantQueJeGagne
	}
	return participantsCountConform(mode, count)
}
