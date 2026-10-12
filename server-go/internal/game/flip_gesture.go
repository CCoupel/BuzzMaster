package game

// #248 (lot 5b) — idempotent card flips: FLIP_MEMORY_CARD.FLIP_ID.
// Contract: contracts/websocket-actions.md §"Idempotence — FLIP_ID".
//
// The per-bumper memory of recent FLIP_IDs lives in the Engine and is read,
// evaluated and written under e.mu — the SAME lock as the checks and the flip
// itself — so two concurrent copies of one gesture (old and new connection of
// the same bumper) can never both flip the card, and a re-send after a missed
// pair or a change of hand is answered from memory instead of being evaluated
// again. The auto flip-back waiting goroutine is NOT started here (the caller
// does it after this returns), so the lock never covers the wait.

// Refusal reasons of a FLIP_MEMORY_CARD_ACK (contract §FLIP_MEMORY_CARD_ACK).
const (
	FlipReasonNotTurn      = "NOT_TURN"
	FlipReasonNotFlippable = "NOT_FLIPPABLE"
	FlipReasonNotPlayable  = "NOT_PLAYABLE"
	FlipReasonCardScope    = "CARD_SCOPE"
)

// FlipMemoryPerBumper is the number of FLIP_IDs remembered per bumper.
const FlipMemoryPerBumper = 16

// FlipGesture is one FLIP_MEMORY_CARD carrying a valid FLIP_ID from a resolved vplayer.
type FlipGesture struct {
	BumperID     string
	Team         string // emitting team (turn check)
	FlipID       string
	CardID       string
	MotionCardID string // "" = question-scoped MEMORY grid
}

// FlipGestureResult is the remembered answer plus what the caller needs to do
// after an APPLIED first evaluation (match / flip-back / completion).
type FlipGestureResult struct {
	FlipID, CardID, MotionCardID string
	Applied                      bool
	Reason                       string // "" when Applied
	Duplicate                    bool   // answered from memory
	ScopeError                   string // MotionError reason when Reason==CARD_SCOPE (first evaluation only)

	// Post-flip data, meaningful only for a first (non-duplicate) APPLIED result.
	IsMatch, ShouldFlipBack, IsComplete bool
	FlipDelay                           int
}

type flipMemoEntry struct {
	flipID string
	res    FlipGestureResult
}

// FlipCardGesture evaluates a FLIP_ID gesture atomically: lookup in memory
// (before any other check) → scope → playable → turn → flippable → flip →
// memorize. See the file comment.
func (e *Engine) FlipCardGesture(g FlipGesture) FlipGestureResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, en := range e.flipMemo[g.BumperID] {
		if en.flipID == g.FlipID {
			res := en.res
			res.Duplicate = true
			res.IsMatch, res.ShouldFlipBack, res.IsComplete, res.FlipDelay = false, false, false, 0
			res.ScopeError = ""
			return res
		}
	}

	res := FlipGestureResult{FlipID: g.FlipID, CardID: g.CardID, MotionCardID: g.MotionCardID}
	cardScoped := g.MotionCardID != ""

	var refusal string
	switch {
	case cardScoped && e.scopeErrorUnsafe(g.MotionCardID) != "":
		res.ScopeError = e.scopeErrorUnsafe(g.MotionCardID)
		refusal = FlipReasonCardScope
	case cardScoped:
		res.IsMatch, res.ShouldFlipBack, res.FlipDelay, res.IsComplete, refusal =
			e.flipMotionMemoryCardUnsafe(g.MotionCardID, g.CardID, true, g.Team)
	case e.state.Question == nil || e.state.Question.Type != QuestionTypeMemory:
		refusal = FlipReasonNotPlayable
	default:
		res.IsMatch, res.ShouldFlipBack, res.FlipDelay, res.IsComplete, refusal =
			e.flipMemoryCardUnsafe(g.CardID, true, g.Team)
	}

	res.Reason = refusal
	res.Applied = refusal == ""
	if !res.Applied {
		res.IsMatch, res.ShouldFlipBack, res.IsComplete, res.FlipDelay = false, false, false, 0
	}

	if e.flipMemo == nil {
		e.flipMemo = map[string][]flipMemoEntry{}
	}
	mem := append(e.flipMemo[g.BumperID], flipMemoEntry{flipID: g.FlipID, res: res})
	if len(mem) > FlipMemoryPerBumper {
		mem = mem[len(mem)-FlipMemoryPerBumper:]
	}
	e.flipMemo[g.BumperID] = mem
	return res
}

// scopeErrorUnsafe is ValidateCardScope's check without taking the lock.
// Caller holds e.mu. Returns the MotionError reason, "" when valid.
func (e *Engine) scopeErrorUnsafe(motionCardID string) string {
	if e.state.MotionSelected == "" {
		if motionCardID != "" {
			return "CARD_SCOPE_UNEXPECTED"
		}
		return ""
	}
	if motionCardID != e.state.MotionSelected {
		return "CARD_SCOPE_MISMATCH"
	}
	return ""
}

// purgeFlipMemoryUnsafe forgets every remembered FLIP_ID (question change,
// PREPARE, NEW_GAME). Caller holds e.mu.
func (e *Engine) purgeFlipMemoryUnsafe() {
	e.flipMemo = nil
}
