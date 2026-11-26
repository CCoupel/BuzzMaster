package game

import (
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

// PICTO (v13.0.0, #192, contracts/picto.md): one team's drawer draws a secret word on a tablet, the team
// guesses it orally. This file holds the engine side of the preparation (team, drawer chosen or drawn at
// random, READY / START guards), the secret word of the round (drawn freely at START, §3.2) and the
// per-round state reset. The drawing relay lives outside the engine (internal/server/picto_canvas.go).

// PICTO_ERROR codes (contracts/picto.md §9.2).
const (
	PictoErrInvalidTeam       = "INVALID_TEAM"
	PictoErrNotACandidate     = "NOT_A_CANDIDATE"
	PictoErrDrawerOffline     = "DRAWER_OFFLINE"
	PictoErrNoConnectedPlayer = "NO_CONNECTED_PLAYER"
	PictoErrNoDrawer          = "NO_DRAWER"
	PictoErrWrongPhase        = "WRONG_PHASE"
	PictoErrNotDrawer         = "NOT_DRAWER"
	PictoErrInvalidStrokes    = "INVALID_STROKES"
	PictoErrCanvasFull        = "CANVAS_FULL"
	PictoErrRateLimited       = "RATE_LIMITED"
	PictoErrPoolEmpty         = "POOL_EMPTY" // v13.1.0 (#324): RESERVOIR source with no available word
	// v13.1.0 (#318, contracts/picto.md §17.10).
	PictoErrPassLimitReached = "PASS_LIMIT_REACHED"
	PictoErrNotRafale        = "NOT_RAFALE"
	PictoErrStaleWord        = "STALE_WORD"
)

// PictoError is a refused PICTO action: Code is one of the PictoErr* codes. State unchanged.
type PictoError struct {
	Code    string
	Message string
}

func (e *PictoError) Error() string { return e.Code + ": " + e.Message }

func pictoErr(code, msg string) *PictoError { return &PictoError{Code: code, Message: msg} }

// PictoCandidate is a VPlayer of PICTO_TEAM that may be chosen as drawer (contracts/picto.md §5).
type PictoCandidate struct {
	ID        string `json:"ID"`
	Name      string `json:"NAME"`
	Connected bool   `json:"CONNECTED"`
}

// PictoWordInfo is the secret word of a round, handed to OnPictoWord only (never to GameState).
type PictoWordInfo struct {
	Word     string
	Category string
	Epoch    int
	DrawerID string
	FromPool bool // the word was drawn (and marked used) in the Pool Picto: its used flags must be saved
}

// PICTO_OUTCOME values (contracts/picto.md §6.4).
const (
	PictoOutcomeFound   = "FOUND"
	PictoOutcomeTimeout = "TIMEOUT"
	PictoOutcomeStopped = "STOPPED"
	PictoOutcomeNoWord  = "NO_WORD"
	PictoOutcomePoolEnd = "POOL_END" // v13.1.0 (#318): no fresh word left and nothing to loop on
)

// isPictoQuestionUnsafe reports whether the current question is of the Picto family (PICTO or RAFALE_PICTO):
// the drawn round is shared (contracts/picto.md §17.1), every site goes through IsPictoFamily.
func (e *Engine) isPictoQuestionUnsafe() bool {
	return e.state.Question != nil && IsPictoFamily(e.state.Question.Type)
}

// pictoParticipantsConform is the PREPARE→READY gate of a PICTO question (§6.1): a team chosen, a duration
// in 30–300 s and at least one word. The drawer is NOT required for READY (designated "at READY"); START
// requires it (PictoStartCheck).
func pictoParticipantsConform(q *Question, st *GameState) bool {
	if st.PictoTeam == "" {
		return false
	}
	cfg := q.PictoConfig()
	if cfg.Source == PictoSourceReservoir {
		// v13.1.0 (#324): a valid filter and at least one available word (POOL_EMPTY otherwise). The count is
		// the engine's fresh one (Engine.participantsConformUnsafe), carried by st.PictoPoolAvailable.
		if !pictoFilterValid(cfg) || st.PictoPoolAvailable < 1 {
			return false
		}
	} else if len(cfg.Words) == 0 {
		return false
	}
	t, err := strconv.Atoi(strings.TrimSpace(q.Time))
	return err == nil && ValidPictoTime(t)
}

// pictoFilterValid reports whether the RESERVOIR filter is usable: at least one category and one difficulty,
// every difficulty in 1–3 (the categories are checked against the known set by the HTTP layer at save time).
func pictoFilterValid(cfg PictoConfig) bool {
	if len(cfg.Categories) == 0 || len(cfg.Difficulties) == 0 {
		return false
	}
	for _, d := range cfg.Difficulties {
		if d < 1 || d > 3 {
			return false
		}
	}
	return true
}

// pictoPoolAvailableUnsafe counts the words still drawable for the current PICTO question: the filtered, unused
// Pool Picto words (RESERVOIR) or the question's own words (WORDS). 0 for any other type. Caller holds e.mu.
func (e *Engine) pictoPoolAvailableUnsafe(q *Question) int {
	if q == nil || !IsPictoFamily(q.Type) {
		return 0
	}
	cfg := q.PictoConfig()
	if e.pictoRound != nil {
		return e.pictoRemainingUnsafe(cfg) // round in progress: never shown, plus the passed words waiting if LOOP (§17.4)
	}
	if cfg.Source == PictoSourceReservoir {
		return len(e.pictoPool.availableUnsafe(cfg.Categories, cfg.Difficulties))
	}
	return len(cfg.Words)
}

// participantsConformUnsafe is participantsConform with the fresh Pool Picto availability: the gate must not
// depend on the last refresh (words may have been added to the pool since). Works on a copy of the state, so it
// is safe under a read lock. Caller holds e.mu.
func (e *Engine) participantsConformUnsafe() bool {
	q := e.state.Question
	if q != nil && IsPictoFamily(q.Type) {
		st := e.state
		st.PictoPoolAvailable = e.pictoPoolAvailableUnsafe(q)
		return participantsConform(q, &st)
	}
	return participantsConform(q, &e.state)
}

// pictoRefreshUnsafe recomputes the derived preparation fields from the live roster: candidates (VPlayers of
// the team, by name then ID), drawer name / connection, team colour, words count. A drawer whose bumper was
// deleted or moved to another team is purged (§6.7). Cheap: called after every roster / connection change.
func (e *Engine) pictoRefreshUnsafe() {
	st := &e.state
	st.PictoCandidates = []PictoCandidate{}
	if st.PictoTeam != "" {
		for id, b := range e.data.Bumpers {
			if b != nil && b.IsVPlayer && b.Team == st.PictoTeam {
				st.PictoCandidates = append(st.PictoCandidates, PictoCandidate{ID: id, Name: b.Name, Connected: b.Connected})
			}
		}
		sort.Slice(st.PictoCandidates, func(i, j int) bool {
			a, b := st.PictoCandidates[i], st.PictoCandidates[j]
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.ID < b.ID
		})
	}
	st.PictoDrawerName, st.PictoDrawerConnected = "", false
	if st.PictoDrawer != "" {
		if b := e.data.Bumpers[st.PictoDrawer]; b == nil || b.Team != st.PictoTeam || st.PictoTeam == "" {
			st.PictoDrawer = ""
		} else {
			st.PictoDrawerName, st.PictoDrawerConnected = b.Name, b.Connected
		}
	}
	if t, ok := e.data.Teams[st.PictoTeam]; ok && st.PictoTeam != "" {
		st.PictoTeamColor = t.Color
	} else {
		st.PictoTeamColor = []int{}
	}
	if st.PictoTeamColor == nil {
		st.PictoTeamColor = []int{}
	}
	st.PictoWordsCount = 0
	st.PictoPoolAvailable = 0
	if e.isPictoQuestionUnsafe() {
		cfg := st.Question.PictoConfig()
		st.PictoWordsCount = len(cfg.Words)
		if cfg.Source == PictoSourceReservoir {
			st.PictoWordsCount = 0 // no question words: the words come from the pool
		}
		st.PictoPoolAvailable = e.pictoPoolAvailableUnsafe(st.Question)
		if e.pictoRound == nil { // no round: preparation values, the hint is the question's category (§17.4)
			st.PictoWordCategory = string(st.Question.Category)
			st.PictoPassesLeft = pictoInitialPassesLeft(cfg)
		}
	} else if e.pictoRound == nil {
		st.PictoWordCategory = ""
		st.PictoPassesLeft = -1
	}
}

// resetPictoPrepUnsafe clears the preparation (question change, InitGameWith, NEW_GAME).
func (e *Engine) resetPictoPrepUnsafe() {
	st := &e.state
	st.PictoTeam, st.PictoDrawer = "", ""
	e.pictoRefreshUnsafe()
}

// resetPictoRoundUnsafe clears what a round leaves behind (Ready on any question, InitGameWith): outcome,
// revealed word, secret word. The epoch is NEVER reset during the life of the server (§5).
func (e *Engine) resetPictoRoundUnsafe() {
	e.state.PictoOutcome = ""
	e.state.PictoRevealedWord = ""
	e.state.PictoRevealedCategory = ""
	e.state.PictoWordCategory = ""
	e.state.PictoFoundCount, e.state.PictoFoundWords = 0, []string{}
	e.state.PictoPassCount, e.state.PictoPassesLeft, e.state.PictoRoundPoints = 0, -1, 0
	e.pictoRound = nil
	e.pictoWord = ""
}

func (e *Engine) firePictoChange() {
	if e.OnPictoChange != nil {
		e.OnPictoChange()
	}
}

func (e *Engine) firePictoCanvasReset(epoch int) {
	if e.OnPictoCanvasReset != nil {
		e.OnPictoCanvasReset(epoch)
	}
}

// pictoPrepAction runs a preparation action under the lock: PICTO question, PREPARE / READY only, then
// re-evaluates PREPARE↔READY and fires the callbacks outside the lock.
func (e *Engine) pictoPrepAction(fn func() error) error {
	e.mu.Lock()
	if !e.isPictoQuestionUnsafe() || (e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady) {
		e.mu.Unlock()
		return pictoErr(PictoErrWrongPhase, "la préparation se fait avant le départ, sur une question Picto")
	}
	if err := fn(); err != nil {
		e.mu.Unlock()
		return err
	}
	e.pictoRefreshUnsafe()
	newPhase := e.reevaluatePrepareReadyUnsafe()
	stateCallback := e.OnStateChange
	e.mu.Unlock()
	if stateCallback != nil && newPhase != "" {
		stateCallback(newPhase)
	}
	e.firePictoChange()
	return nil
}

// PictoSetTeam — PICTO_SET_TEAM: chooses the drawing team ("" clears it). The drawer is cleared if not of
// that team (done by the refresh).
func (e *Engine) PictoSetTeam(team string) error {
	return e.pictoPrepAction(func() error {
		if team != "" {
			if _, ok := e.data.Teams[team]; !ok {
				return pictoErr(PictoErrInvalidTeam, "équipe inconnue")
			}
		}
		e.state.PictoTeam = team
		e.pictoRefreshUnsafe() // so the drawer / candidates follow the team before the re-evaluation
		return nil
	})
}

// PictoSetDrawer — PICTO_SET_DRAWER: chooses a connected candidate ("" clears the drawer).
func (e *Engine) PictoSetDrawer(player string) error {
	return e.pictoPrepAction(func() error {
		if player == "" {
			e.state.PictoDrawer = ""
			return nil
		}
		e.pictoRefreshUnsafe()
		for _, c := range e.state.PictoCandidates {
			if c.ID == player {
				if !c.Connected {
					return pictoErr(PictoErrDrawerOffline, "ce joueur n'est pas connecté")
				}
				e.state.PictoDrawer = player
				return nil
			}
		}
		return pictoErr(PictoErrNotACandidate, "ce joueur n'est pas un candidat de l'équipe")
	})
}

// PictoDrawDrawer — PICTO_DRAW_DRAWER: draws the drawer uniformly among the CONNECTED candidates
// (rand.Intn, no memory: may fall again on the current one). Replayable until the round starts.
func (e *Engine) PictoDrawDrawer() error {
	return e.pictoPrepAction(func() error {
		if e.state.PictoTeam == "" {
			return pictoErr(PictoErrInvalidTeam, "aucune équipe choisie")
		}
		e.pictoRefreshUnsafe()
		var connected []string
		for _, c := range e.state.PictoCandidates {
			if c.Connected {
				connected = append(connected, c.ID)
			}
		}
		if len(connected) == 0 {
			return pictoErr(PictoErrNoConnectedPlayer, "aucun joueur connecté dans l'équipe")
		}
		e.state.PictoDrawer = connected[rand.Intn(len(connected))]
		return nil
	})
}

// pictoStartCheckUnsafe is the START guard (§6.1): a drawer designated AND connected.
func (e *Engine) pictoStartCheckUnsafe() *PictoError {
	if !e.isPictoQuestionUnsafe() {
		return nil
	}
	e.pictoRefreshUnsafe()
	if e.state.PictoDrawer == "" || !e.state.PictoDrawerConnected {
		return pictoErr(PictoErrNoDrawer, "désignez un dessinateur connecté avant de lancer")
	}
	// v13.1.0 (#324, contracts/picto-pool.md §3): the RESERVOIR source needs at least one available word.
	if e.state.Question.PictoConfig().Source == PictoSourceReservoir && e.state.PictoPoolAvailable < 1 {
		return pictoErr(PictoErrPoolEmpty, "aucun mot disponible dans le Pool Picto pour ce filtre")
	}
	return nil
}

// PictoStartCheck reports whether START would be refused for lack of a connected drawer (nil = fine, or not
// a PICTO question). Called by the application layer before Start to answer PICTO_ERROR to the emitter.
func (e *Engine) PictoStartCheck() *PictoError {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pictoStartCheckUnsafe()
}

// startPictoRoundUnsafe runs at the START of a Picto-family round, under the lock: new sheet epoch, round
// state reset, draw of the first word (§3.2 / §17.6: the PICTO question has one draw, RAFALE_PICTO goes on with
// PICTO_FOUND / PICTO_PASS). Returns nil for any other type, or the word to hand over (outside the lock,
// finishPictoRoundStart). No word (question edited between READY and START): the secret word stays empty and
// nothing is sent — the NO_WORD safety net applies.
func (e *Engine) startPictoRoundUnsafe() *PictoWordInfo {
	if !e.isPictoQuestionUnsafe() {
		return nil
	}
	q := e.state.Question
	cfg := q.PictoConfig()
	e.state.PictoCanvasEpoch++
	e.state.PictoOutcome, e.state.PictoRevealedWord, e.state.PictoRevealedCategory = "", "", ""
	e.state.PictoFoundCount, e.state.PictoFoundWords, e.state.PictoPassCount, e.state.PictoRoundPoints = 0, []string{}, 0, 0
	e.state.PictoPassesLeft = pictoInitialPassesLeft(cfg)
	e.pictoWord = ""
	e.pictoRound = &pictoRoundState{shown: map[string]bool{}}
	if cfg.Source == PictoSourceReservoir {
		// Stratified draw (category x difficulty) in the Pool Picto, "used" marking at the draw
		// (contracts/picto-pool.md §3). One draw state for the whole round.
		e.pictoRound.draw = newPoolDrawState(e.pictoPool, cfg.Categories, cfg.Difficulties)
	}
	info := &PictoWordInfo{Category: string(q.Category), Epoch: e.state.PictoCanvasEpoch, DrawerID: e.state.PictoDrawer}
	if word, cat, fromPool, ok := e.pictoDrawFreshUnsafe(cfg, q); ok {
		e.pictoWord = word
		info.Word, info.Category, info.FromPool = word, cat, fromPool
	}
	e.state.PictoPoolAvailable = e.pictoRemainingUnsafe(cfg)
	e.state.PictoWordCategory = info.Category
	return info
}

// finishPictoRoundStart fires, OUTSIDE the lock, the sheet reset for the new epoch then the secret word.
func (e *Engine) finishPictoRoundStart(info *PictoWordInfo) {
	if info == nil {
		return
	}
	e.firePictoCanvasReset(info.Epoch)
	if info.FromPool {
		safeGo("SavePictoUsed", e.SavePictoUsed) // the draw marked the word used (persisted AFTER the lock)
	}
	if info.Word != "" && e.OnPictoWord != nil {
		e.OnPictoWord(*info)
	}
}

// PictoWord returns the secret word of the round in progress (word, category, epoch, drawer) for the
// reconnection of the drawer or of an admin / anim (§6.7). ok=false when there is none.
func (e *Engine) PictoWord() (PictoWordInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.isPictoQuestionUnsafe() || e.pictoWord == "" {
		return PictoWordInfo{}, false
	}
	cat := string(e.state.Question.Category)
	if e.state.Question.PictoConfig().Source == PictoSourceReservoir && e.state.PictoWordCategory != "" {
		cat = e.state.PictoWordCategory // the word's own category (pool), same as at the draw
	}
	return PictoWordInfo{Word: e.pictoWord, Category: cat, Epoch: e.state.PictoCanvasEpoch, DrawerID: e.state.PictoDrawer}, true
}

// PictoStrokeContext returns the drawer ID, the phase, the sheet epoch and whether the current question is a
// PICTO one — one consistent read for the strokes guard (§8.6).
func (e *Engine) PictoStrokeContext() (drawer string, phase GamePhase, epoch int, isPicto bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.PictoDrawer, e.state.Phase, e.state.PictoCanvasEpoch, e.isPictoQuestionUnsafe()
}

// pictoRenameTeamUnsafe / pictoPurgeTeamUnsafe follow a team rename or deletion (§6.7).
func (e *Engine) pictoRenameTeamUnsafe(oldName, newName string) bool {
	if e.state.PictoTeam == oldName {
		e.state.PictoTeam = newName
		e.pictoRefreshUnsafe()
		return true
	}
	return false
}

func (e *Engine) pictoPurgeTeamUnsafe(name string) {
	if e.state.PictoTeam == name {
		e.state.PictoTeam, e.state.PictoDrawer = "", ""
		e.pictoRefreshUnsafe()
	}
}

// enterRevealedUnsafe is the common entry into REVEALED (REVEAL, and PICTO_FOUND which stops then reveals in one
// action): ends a pending deferred timer, sets the phase and, for a PICTO question, applies §6.5 — the word of
// the round enters GameState HERE and nowhere else (PICTO_REVEALED_WORD), with the category of the credit. An
// outcome still empty at this point means the round was abandoned (REVEAL from PAUSED, or a STOP in the
// countdown): STOPPED. The other words of the question are never revealed.
func (e *Engine) enterRevealedUnsafe() {
	e.resetDeferredTimerUnsafe()
	e.state.Phase = PhaseRevealed
	e.state.TeamsLocked = false // #249
	if e.isPictoQuestionUnsafe() {
		if e.state.PictoOutcome == "" {
			e.state.PictoOutcome = PictoOutcomeStopped
		}
		e.state.PictoRevealedWord = e.pictoWord // "" for NO_WORD
		e.state.PictoRevealedCategory = string(e.state.Question.Category)
	}
}

// PictoStepResult tells the application layer what a PICTO_FOUND / PICTO_PASS did: the phase the engine ended in
// (REVEALED after a "Trouvé" on a PICTO question or on the gong, STOPPED after a POOL_END, STARTED / PAUSED when
// the round goes on with the next word).
type PictoStepResult struct {
	Phase GamePhase
}

// PictoFound — PICTO_FOUND (admin / anim) without epoch: the PICTO (one word) form. See PictoFoundAt.
func (e *Engine) PictoFound() error {
	_, err := e.PictoFoundAt(nil)
	return err
}

// PictoFoundAt — PICTO_FOUND (admin / anim): "Trouvé !". PICTO (one word): accepted in STARTED and PAUSED, and in
// STOPPED only after TIMEOUT (the answer came on the gong, I5); refused otherwise (WRONG_PHASE). Stops the round
// then reveals it in the SAME action, under one lock: two simultaneous calls create a single outcome, the second
// finds REVEALED. RAFALE_PICTO: the word is counted and the round goes on (picto_rafale.go); EPOCH is mandatory.
// The credit line is the existing mechanism (TEAM_POINTS), pre-filled by the clients.
func (e *Engine) PictoFoundAt(epoch *int) (PictoStepResult, error) {
	e.mu.Lock()
	if !e.isPictoQuestionUnsafe() {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrWrongPhase, "la question courante n'est pas une question Picto")
	}
	if e.state.Question.Type == QuestionTypeRafalePicto {
		return e.rafalePictoFoundLocked(epoch) // unlocks
	}
	ph := e.state.Phase
	afterTimeout := ph == PhaseStopped && e.state.PictoOutcome == PictoOutcomeTimeout
	if ph != PhaseStarted && ph != PhasePaused && !afterTimeout {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrWrongPhase, "« Trouvé » n'est possible que manche en cours, ou après le temps écoulé")
	}
	e.state.PictoOutcome = PictoOutcomeFound // the only replacement allowed: TIMEOUT → FOUND (§6.4)
	released := false
	if ph != PhaseStopped {
		released = e.stopUnsafe() // timers cancelled; the outcome is already posed, stopUnsafe keeps it
	}
	e.enterRevealedUnsafe()
	e.setQuestionStatus(StatusRevealed)
	callback := e.OnStateChange
	e.mu.Unlock()

	if released {
		safeGo("SaveRafaleUsed", e.SaveRafaleUsed)
	}
	if callback != nil {
		callback(PhaseRevealed)
	}
	return PictoStepResult{Phase: PhaseRevealed}, nil
}

// pictoNoWordStopUnsafe is the safety net of §6.2: a question edited by hand between READY and START may have
// no word left. The round gets the outcome NO_WORD and stops at once (REVEAL then leads to REVEALED with an empty
// revealed word); nothing is sent to the drawer. Returns the phase to announce: STOPPED then, STARTED otherwise.
func (e *Engine) pictoNoWordStopUnsafe(info *PictoWordInfo) GamePhase {
	if info == nil || info.Word != "" {
		return PhaseStarted
	}
	e.state.PictoOutcome = PictoOutcomeNoWord
	e.stopUnsafe() // cancels the timer just started; the posed outcome is kept
	log.Printf("[Engine] PICTO: no word at START, round stopped (NO_WORD)")
	return PhaseStopped
}
