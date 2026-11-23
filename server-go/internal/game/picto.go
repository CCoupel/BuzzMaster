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
}

// PICTO_OUTCOME values (contracts/picto.md §6.4).
const (
	PictoOutcomeFound   = "FOUND"
	PictoOutcomeTimeout = "TIMEOUT"
	PictoOutcomeStopped = "STOPPED"
	PictoOutcomeNoWord  = "NO_WORD"
)

func (e *Engine) isPictoQuestionUnsafe() bool {
	return e.state.Question != nil && e.state.Question.Type == QuestionTypePicto
}

// pictoParticipantsConform is the PREPARE→READY gate of a PICTO question (§6.1): a team chosen, a duration
// in 30–300 s and at least one word. The drawer is NOT required for READY (designated "at READY"); START
// requires it (PictoStartCheck).
func pictoParticipantsConform(q *Question, st *GameState) bool {
	if st.PictoTeam == "" || len(q.PictoWords) == 0 {
		return false
	}
	t, err := strconv.Atoi(strings.TrimSpace(q.Time))
	return err == nil && ValidPictoTime(t)
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
	if e.isPictoQuestionUnsafe() {
		st.PictoWordsCount = len(st.Question.PictoWords)
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
	return nil
}

// PictoStartCheck reports whether START would be refused for lack of a connected drawer (nil = fine, or not
// a PICTO question). Called by the application layer before Start to answer PICTO_ERROR to the emitter.
func (e *Engine) PictoStartCheck() *PictoError {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pictoStartCheckUnsafe()
}

// startPictoRoundUnsafe runs at the START of a PICTO round, under the lock: new sheet epoch, free draw of
// the word among all the question's words (§3.2, uniform, no memory). Returns nil for any other type, or
// the word to hand over (outside the lock, finishPictoRoundStart). No word (question edited between READY
// and START): the secret word stays empty and nothing is sent — the NO_WORD outcome belongs to LOT-2.2.
func (e *Engine) startPictoRoundUnsafe() *PictoWordInfo {
	if !e.isPictoQuestionUnsafe() {
		return nil
	}
	e.state.PictoCanvasEpoch++
	e.state.PictoOutcome, e.state.PictoRevealedWord, e.state.PictoRevealedCategory = "", "", ""
	e.pictoWord = ""
	words := e.state.Question.PictoWords
	info := &PictoWordInfo{Category: string(e.state.Question.Category), Epoch: e.state.PictoCanvasEpoch, DrawerID: e.state.PictoDrawer}
	if len(words) > 0 {
		e.pictoWord = words[rand.Intn(len(words))]
		info.Word = e.pictoWord
	}
	return info
}

// finishPictoRoundStart fires, OUTSIDE the lock, the sheet reset for the new epoch then the secret word.
func (e *Engine) finishPictoRoundStart(info *PictoWordInfo) {
	if info == nil {
		return
	}
	e.firePictoCanvasReset(info.Epoch)
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
	return PictoWordInfo{Word: e.pictoWord, Category: string(e.state.Question.Category), Epoch: e.state.PictoCanvasEpoch, DrawerID: e.state.PictoDrawer}, true
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

// PictoFound — PICTO_FOUND (admin / anim): "Trouvé !". Accepted in STARTED and PAUSED, and in STOPPED only after
// TIMEOUT (the answer came on the gong, I5); refused otherwise (WRONG_PHASE). Stops the round then reveals it in
// the SAME action, under one lock: two simultaneous calls create a single outcome, the second finds REVEALED.
// The credit line is the existing mechanism (TEAM_POINTS), pre-filled by the clients at the question's points.
func (e *Engine) PictoFound() error {
	e.mu.Lock()
	if !e.isPictoQuestionUnsafe() {
		e.mu.Unlock()
		return pictoErr(PictoErrWrongPhase, "la question courante n'est pas une question Picto")
	}
	ph := e.state.Phase
	afterTimeout := ph == PhaseStopped && e.state.PictoOutcome == PictoOutcomeTimeout
	if ph != PhaseStarted && ph != PhasePaused && !afterTimeout {
		e.mu.Unlock()
		return pictoErr(PictoErrWrongPhase, "« Trouvé » n'est possible que manche en cours, ou après le temps écoulé")
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
	return nil
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
