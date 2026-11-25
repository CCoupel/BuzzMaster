package game

import (
	"math/rand"
	"strconv"
	"strings"
)

// RAFALE_PICTO engine (v13.1.0, #318, contracts/picto.md §17.3 to §17.8): the Picto round with many words. One
// team, one drawer for the whole round, the global chrono of the question (TIME). The drawer announces a word
// found or a pass aloud; the admin / anim trigger PICTO_FOUND / PICTO_PASS (never a tablet). Each action moves
// to the NEXT WORD under the engine lock, in the same action: epoch + 1 (clean sheet), draw (§17.6), marking
// "used" (pool), then PICTO_WORD to the drawer + admin + anim and one UPDATE — all fired after the lock.

// pictoRoundState is the engine-private state of the Picto-family round in progress (nil outside a round). It is
// never serialised: the word in play and the passed words never reach GameState.
type pictoRoundState struct {
	draw  poolDrawState   // RESERVOIR source: the round's stratified draw (shared by every word of the round)
	shown map[string]bool // WORDS source: words already shown this round
	queue []pictoQueued   // passed words waiting (FIFO), used only with POOL_END = LOOP
}

type pictoQueued struct {
	word, category string
}

// pictoInitialPassesLeft is the PICTO_PASSES_LEFT of a round that has not used any pass: -1 (unlimited) for a
// PICTO question and for RAFALE_PICTO_PASS_LIMIT = 0, else the limit.
func pictoInitialPassesLeft(cfg PictoConfig) int {
	if cfg.Rafale && cfg.PassLimit > 0 {
		return cfg.PassLimit
	}
	return -1
}

// pictoDrawFreshUnsafe draws a word NEVER shown in the round (§17.6 point 1): RESERVOIR = stratified draw of the
// Pool Picto among the unused entries (marked used at the draw), WORDS = uniform among the question's words not
// shown yet. ok = false when nothing fresh is left. Caller holds e.mu; e.pictoRound is set.
func (e *Engine) pictoDrawFreshUnsafe(cfg PictoConfig, q *Question) (word, category string, fromPool, ok bool) {
	if cfg.Source == PictoSourceReservoir {
		entry, err := e.pictoRound.draw.next(e.pictoPool)
		if err != nil {
			return "", "", false, false
		}
		return entry.Question, string(entry.Category), true, true
	}
	var fresh []string
	for _, w := range cfg.Words {
		if !e.pictoRound.shown[w] {
			fresh = append(fresh, w)
		}
	}
	if len(fresh) == 0 {
		return "", "", false, false
	}
	w := fresh[rand.Intn(len(fresh))]
	e.pictoRound.shown[w] = true
	return w, string(q.Category), false, true
}

// pictoRemainingUnsafe counts the words still drawable in the round in progress: never shown, plus the passed
// words waiting if LOOP (§17.4). Caller holds e.mu.
func (e *Engine) pictoRemainingUnsafe(cfg PictoConfig) int {
	if e.pictoRound == nil {
		return 0
	}
	n := 0
	if cfg.Source == PictoSourceReservoir {
		n = len(e.pictoPool.availableUnsafe(cfg.Categories, cfg.Difficulties))
	} else {
		for _, w := range cfg.Words {
			if !e.pictoRound.shown[w] {
				n++
			}
		}
	}
	if cfg.Rafale && cfg.PoolLoop {
		n += len(e.pictoRound.queue)
	}
	return n
}

// pictoRoundPointsUnsafe recomputes PICTO_ROUND_POINTS = max(0, found x POINTS - passes x cost): the suggested
// credit, never negative (the passes never debit the score, §17.7). Caller holds e.mu.
func (e *Engine) pictoRoundPointsUnsafe() {
	q := e.state.Question
	if q == nil || q.Type != QuestionTypeRafalePicto {
		e.state.PictoRoundPoints = 0
		return
	}
	pts, _ := strconv.Atoi(strings.TrimSpace(q.Points))
	v := e.state.PictoFoundCount*pts - e.state.PictoPassCount*q.PictoConfig().PassCost
	if v < 0 {
		v = 0
	}
	e.state.PictoRoundPoints = v
}

// pictoAdvanceUnsafe moves the round to the next word (§17.5 "Mot suivant"): draws a fresh word, else (LOOP) the
// oldest passed one, else nothing — the pool is over. On success: epoch + 1 and the word becomes current; the
// info carries the PICTO_WORD to send. poolEnd = true when no word could be drawn (the round must end,
// POOL_END): the word in play is then cleared and the epoch is left untouched. Caller holds e.mu.
func (e *Engine) pictoAdvanceUnsafe(cfg PictoConfig, q *Question) (info *PictoWordInfo, poolEnd bool) {
	word, cat, fromPool, ok := e.pictoDrawFreshUnsafe(cfg, q)
	if !ok && cfg.PoolLoop && len(e.pictoRound.queue) > 0 {
		first := e.pictoRound.queue[0]
		e.pictoRound.queue = e.pictoRound.queue[1:]
		word, cat, fromPool, ok = first.word, first.category, false, true
	}
	if !ok {
		e.pictoWord = ""
		e.state.PictoPoolAvailable = e.pictoRemainingUnsafe(cfg)
		return nil, true
	}
	e.state.PictoCanvasEpoch++
	e.pictoWord = word
	e.state.PictoWordCategory = cat
	e.state.PictoPoolAvailable = e.pictoRemainingUnsafe(cfg)
	return &PictoWordInfo{Word: word, Category: cat, Epoch: e.state.PictoCanvasEpoch, DrawerID: e.state.PictoDrawer, FromPool: fromPool}, false
}

// rafalePictoCheckEpochUnsafe refuses an action of another epoch (§17.5 "EPOCH obligatoire"): admin ‖ anim
// double gesture, a late click on a word already left. A missing EPOCH is refused too.
func (e *Engine) rafalePictoCheckEpochUnsafe(epoch *int) *PictoError {
	if epoch == nil || *epoch != e.state.PictoCanvasEpoch {
		return pictoErr(PictoErrStaleWord, "ce mot n'est plus le mot en cours")
	}
	return nil
}

// rafalePictoFoundLocked is PICTO_FOUND for RAFALE_PICTO. Caller holds e.mu (write) and it is RELEASED here.
//   - STARTED / PAUSED: the word in play is counted found, then the next word (or POOL_END).
//   - STOPPED after TIMEOUT, word still in play: the gong answer — counted once, then REVEALED in the same
//     action (§17.7); the outcome stays TIMEOUT.
func (e *Engine) rafalePictoFoundLocked(epoch *int) (PictoStepResult, error) {
	ph := e.state.Phase
	gong := ph == PhaseStopped && e.state.PictoOutcome == PictoOutcomeTimeout && e.pictoWord != ""
	if ph != PhaseStarted && ph != PhasePaused && !gong {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrWrongPhase, "« Trouvé » n'est possible que manche en cours, ou sur le gong")
	}
	if perr := e.rafalePictoCheckEpochUnsafe(epoch); perr != nil {
		e.mu.Unlock()
		return PictoStepResult{}, perr
	}
	q := e.state.Question
	cfg := q.PictoConfig()
	e.state.PictoFoundCount++
	e.state.PictoFoundWords = append(e.state.PictoFoundWords, e.pictoWord)
	e.pictoRoundPointsUnsafe()

	if gong {
		e.pictoWord = "" // the word in play has been found: nothing left to reveal
		e.state.PictoPoolAvailable = e.pictoRemainingUnsafe(cfg)
		e.enterRevealedUnsafe()
		e.setQuestionStatus(StatusRevealed)
		callback := e.OnStateChange
		e.mu.Unlock()
		if e.OnRafaleCorrect != nil {
			e.OnRafaleCorrect()
		}
		if callback != nil {
			callback(PhaseRevealed)
		}
		e.firePictoChange()
		return PictoStepResult{Phase: PhaseRevealed}, nil
	}
	return e.rafalePictoMoveOnLocked(cfg, q, e.OnRafaleCorrect), nil
}

// PictoPass — PICTO_PASS (admin / anim, RAFALE_PICTO only, STARTED only): the drawer announced a pass. Refused
// NOT_RAFALE on a PICTO question, PASS_LIMIT_REACHED when no pass is left, STALE_WORD on another epoch. The word
// joins the queue of the passed words (FIFO, recycled only with LOOP) — never published — then the next word.
func (e *Engine) PictoPass(epoch *int) (PictoStepResult, error) {
	e.mu.Lock()
	if !e.isPictoQuestionUnsafe() {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrWrongPhase, "la question courante n'est pas une question Picto")
	}
	q := e.state.Question
	if q.Type != QuestionTypeRafalePicto {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrNotRafale, "la passe n'existe qu'en Rafale Picto")
	}
	if e.state.Phase != PhaseStarted || e.pictoRound == nil {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrWrongPhase, "« Passer » n'est possible que manche en cours")
	}
	if perr := e.rafalePictoCheckEpochUnsafe(epoch); perr != nil {
		e.mu.Unlock()
		return PictoStepResult{}, perr
	}
	if e.state.PictoPassesLeft == 0 {
		e.mu.Unlock()
		return PictoStepResult{}, pictoErr(PictoErrPassLimitReached, "plus de passe disponible")
	}
	cfg := q.PictoConfig()
	e.state.PictoPassCount++
	if e.state.PictoPassesLeft > 0 {
		e.state.PictoPassesLeft--
	}
	e.pictoRound.queue = append(e.pictoRound.queue, pictoQueued{word: e.pictoWord, category: e.state.PictoWordCategory})
	e.pictoRoundPointsUnsafe()
	return e.rafalePictoMoveOnLocked(cfg, q, e.OnRafaleCardInvalid), nil
}

// rafalePictoMoveOnLocked draws the next word after a found / pass and fires, AFTER releasing e.mu (which it
// releases): the sheet reset, PICTO_WORD, the verdict cue, then — on POOL_END — the stop (STOPPED) and the lot-end
// cue; one UPDATE (OnPictoChange). Caller holds e.mu.
func (e *Engine) rafalePictoMoveOnLocked(cfg PictoConfig, q *Question, verdictCue func()) PictoStepResult {
	info, poolEnd := e.pictoAdvanceUnsafe(cfg, q)
	released := false
	var stateCallback func(GamePhase)
	phase := e.state.Phase
	if poolEnd {
		e.state.PictoOutcome = PictoOutcomePoolEnd
		released = e.stopUnsafe()
		stateCallback = e.OnStateChange
		phase = PhaseStopped
	}
	e.mu.Unlock()

	if info != nil {
		e.firePictoCanvasReset(info.Epoch)
		if info.FromPool {
			safeGo("SavePictoUsed", e.SavePictoUsed)
		}
		if e.OnPictoWord != nil {
			e.OnPictoWord(*info)
		}
	}
	if verdictCue != nil {
		verdictCue()
	}
	if poolEnd {
		if released {
			safeGo("SaveRafaleUsed", e.SaveRafaleUsed)
		}
		if e.OnRafaleLotEnd != nil {
			e.OnRafaleLotEnd() // rafale-fin-lot, AFTER the verdict cue (review m1)
		}
		if stateCallback != nil {
			stateCallback(PhaseStopped)
		}
	}
	e.firePictoChange()
	return PictoStepResult{Phase: phase}
}
