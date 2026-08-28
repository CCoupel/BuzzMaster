package game

// TypeDescriptor documents one QuestionType's shape and capabilities: the
// TypedContent fields it owns, its media slots when nested in a MEMOTION
// card, whether it may be nested at all, and whether it needs its own
// inbound-whitelist entry when nested. Source of truth for the type
// registry — contracts/question-types.md §7.
type TypeDescriptor struct {
	Type QuestionType
	// OwnedFields lists the TypedContent/MotionCard JSON field names (as
	// they appear on the wire, e.g. "QCM_ANSWERS") that belong exclusively
	// to this type — contract §3.1(a). Drives the CARD_TYPE_CONTENT_MISMATCH
	// check (§3.2, ValidateCardTypeContent below) and the sub-editor to
	// mount client-side. NOT the same predicate as the UI's blank-card lock,
	// which also considers card-common fields (RECTO_THEME, DIFFICULTY, …).
	OwnedFields []string
	// MediaSlots lists the upload slot names ("recto", "question",
	// "answer", ...) this type declares when hosted by a MEMOTION card —
	// contract §8. Nil for MEMOTION itself (never nestable).
	MediaSlots []string
	// NestableInMotionCard is false for any type that cannot appear inside
	// a MEMOTION card — true for SPEEDY/QCM/MEMORY (#184/#185/#187), false
	// for ARDOISE (closed "not planned" 2026-08-24, contract §7.2 — the
	// only differentiator vs SPEEDY, multi-team simultaneous input, has no
	// object once nested in a card that plays a single team at a time) and,
	// permanently, for MEMOTION itself (nesting depth capped at 1 —
	// contract §1).
	NestableInMotionCard bool
	// HasPlayerInput documents whether nesting this type opens a new
	// inbound action / whitelist entry (contract §7.1) — informational in
	// v7.0.0: no nested type actually exercises player input yet (QCM-in-
	// card is display+designation only, no buzz, no VPLAYER_QCM_ANSWER).
	HasPlayerInput bool
	// DefaultPointsRule (#187 cycle 5) — the POINTS_RULE.MODE a card of
	// this type is scored under when it carries NO explicit PointsRule.
	// Empty ⇒ PointsRuleModeStars (the pre-#184 star-based scale,
	// unchanged default for every type that doesn't override this).
	// Resolved by motionCardPointsForOutcome (engine.go) — the SOLE reader
	// of MotionCard.PointsRule — never by a `card.EffectiveType() ==
	// QuestionTypeMemory` check anywhere else: the barème queries this
	// registry generically, exactly like MediaSlots/OwnedFields, so no
	// MEMORY-specific knowledge enters the host (contract §10 agnosticity).
	//
	// Why a registry fact instead of writing POINTS_RULE onto the card
	// (at creation, client-side, or at save, server-side) — both were
	// considered and rejected (code-review 20260825-214659,
	// plan-memotion-v710-memory-pointsrule-20260825-215050 §2.1/§2.2):
	// POINTS_RULE is a CARD field (§3.1), not a TYPE field — it survives a
	// TYPE change verbatim. A card that carried MEMORY, had its pairs
	// cleared (unlocking the type per §3.2), and was switched to SPEEDY
	// would keep a written-down {MODE: STARS_PRORATA}: a binary type
	// reports UnitsTotal=0, and §6.2's own zero-division guard then makes
	// the card worth 0 points — SILENTLY. Writing the default onto the
	// card would need a matching cleanup rule on every type change, on
	// TWO independent write paths (client creation, server save), and
	// would never repair a card already persisted before this fix. A
	// registry-resolved default has none of that: nothing is ever
	// written, an existing MEMORY card gets the right default the moment
	// this code ships (no re-save needed), and a card that changes TYPE
	// is re-resolved from its CURRENT EffectiveType() on every read —
	// there is no stale value to leak.
	DefaultPointsRule PointsRuleMode
}

// questionTypeRegistry is the exhaustive, hard-coded table of type
// descriptors — contracts/question-types.md §7. Deliberately not a DSL or
// plugin mechanism (#184 GATE decision: no speculative abstraction for
// types that don't exist yet). AllQuestionTypes() (models.go, #183/A-B2)
// and this registry must together cover every QuestionType — enforced by
// the exhaustiveness test in question_types_test.go.
var questionTypeRegistry = map[QuestionType]TypeDescriptor{
	QuestionTypeSpeedy: {
		Type:                 QuestionTypeSpeedy,
		OwnedFields:          []string{"ANSWER_TEXT", "ANSWER_IMAGE"},
		MediaSlots:           []string{"recto", "question", "answer"},
		NestableInMotionCard: true,
		HasPlayerInput:       false,
	},
	QuestionTypeQCM: {
		Type: QuestionTypeQCM,
		OwnedFields: []string{
			"QCM_ANSWERS", "QCM_CORRECT", "QCM_HINTS_ENABLED",
			"QCM_HINT_THRESHOLD_1", "QCM_HINT_THRESHOLD_2",
			"QCM_PENALTY_1", "QCM_PENALTY_2",
		},
		MediaSlots:           []string{"recto", "question"},
		NestableInMotionCard: true,
		// §7.1: displayed + invalidated + designated via MEMOTION_DONE,
		// exactly like SPEEDY — no buzz, no VPLAYER_QCM_ANSWER, no
		// whitelist change for #185.
		HasPlayerInput: false,
	},
	QuestionTypeArdoise: {
		Type:                 QuestionTypeArdoise,
		OwnedFields:          []string{"ANSWER", "ARDOISE_KEYBOARD_TYPE"},
		MediaSlots:           []string{"recto", "question"},
		NestableInMotionCard: false, // #186 closed "not planned" (2026-08-24), no échéance — contract §7.2
		HasPlayerInput:       true,
	},
	QuestionTypeMemory: {
		Type:                 QuestionTypeMemory,
		OwnedFields:          []string{"MEMORY_PAIRS", "MEMORY_CONFIG", "MEMORY_MODE"},
		MediaSlots:           []string{"recto"}, // + N pair slots — not modeled as a flat list (#187)
		NestableInMotionCard: true,              // #187, v7.1.0 — contract §7.3
		// #187: a nested MEMORY card is the first nestable type to accept
		// player input (flipping a card) — no new inbound-whitelist entry
		// though, FLIP_MEMORY_CARD is already open to tv/vplayer/anim; what
		// changes is scope (MOTION_CARD_ID) and server-side turn checking,
		// not the right to emit (contract §7.3).
		HasPlayerInput: true,
		// #187 cycle 5 — the "on compte les points en fonction du nombre
		// de paires trouvées" decision (contract §6.3) is a fact about the
		// TYPE, resolved here rather than written onto any card — see this
		// field's own doc comment on TypeDescriptor for why.
		DefaultPointsRule: PointsRuleModeStarsProrata,
	},
	QuestionTypeMemotion: {
		Type:                 QuestionTypeMemotion,
		OwnedFields:          nil,
		MediaSlots:           nil,
		NestableInMotionCard: false, // never — nesting depth capped at 1, contract §1
		HasPlayerInput:       false,
	},
}

// TypeDescriptorFor returns the registry entry for t and whether t is a
// known, registered QuestionType.
func TypeDescriptorFor(t QuestionType) (TypeDescriptor, bool) {
	d, ok := questionTypeRegistry[t]
	return d, ok
}

// IsNestableInMotionCard reports whether t may be assigned as a MEMOTION
// card's TYPE — false for an unknown type as well as for a known,
// non-nestable one (both cases the caller must reject the same way: no
// distinct "unknown" bit here, callers needing to tell the two apart use
// TypeDescriptorFor directly).
func IsNestableInMotionCard(t QuestionType) bool {
	d, ok := questionTypeRegistry[t]
	return ok && d.NestableInMotionCard
}

// isJSONContentZero reports whether v — a value as decoded by
// encoding/json.Unmarshal into interface{}, or set directly in a
// map[string]interface{} built by this codebase's form-parsing code
// (strings, float64/int/bool, nil, []interface{}, map[string]interface{})
// — is "no content" for the purpose of the type-lock content check: the
// same shapes contracts/question-types.md §3.2's "valeur de création" table
// treats as absent. This is deliberately narrower than a generic JSON-zero
// helper: a *QCMAnswers-shaped map with all-empty string values still
// counts as content-bearing at the field level (ValidateCardTypeContent
// only asks "is this OwnedField present with SOME value", not "is it at
// its own sub-field creation default" — that finer-grained locking
// judgment is the UI's job per contract §3.2, not the server's).
func isJSONContentZero(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return x == ""
	case float64:
		return x == 0
	case int:
		return x == 0
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	default:
		return false
	}
}

// ValidateCardTypeContent checks one MEMOTION card payload — a
// map[string]interface{} decoded from the "motion_cards" upload form field,
// one entry per card — for orphaned typed content: contract §3.2's server
// guarantee that "une carte ne doit jamais porter de contenu appartenant à
// un autre type que son TYPE déclaré". cardType is the card's own
// (possibly empty ⇒ SPEEDY) TYPE. Returns a non-nil error naming the first
// offending field and the type it belongs to if any OwnedField of a
// DIFFERENT registered type is present with non-zero content; nil if the
// card is internally consistent.
//
// Deliberately stateless (no comparison against a previously stored
// version) — see contract §3.2's rationale: handleUploadQuestion
// reconstructs the question from scratch on every save, and a stateful
// check would block the legitimate "clear all owned fields → change TYPE →
// save" flow in one request.
func ValidateCardTypeContent(cardType QuestionType, card map[string]interface{}) error {
	effective := cardType
	if effective == "" {
		effective = QuestionTypeSpeedy
	}
	for t, desc := range questionTypeRegistry {
		if t == effective {
			continue
		}
		for _, field := range desc.OwnedFields {
			val, exists := card[field]
			if exists && !isJSONContentZero(val) {
				return &CardTypeContentMismatchError{
					CardID: stringOrEmpty(card["ID"]),
					Field:  field,
					Owner:  t,
					Type:   effective,
				}
			}
		}
	}
	return nil
}

// stringOrEmpty extracts a string from a decoded-JSON interface{} value,
// returning "" for anything else (including nil) — used only to make
// CardTypeContentMismatchError's message helpful, never for control flow.
func stringOrEmpty(v interface{}) string {
	s, _ := v.(string)
	return s
}

// CardTypeContentMismatchError is the HTTP 400 CARD_TYPE_CONTENT_MISMATCH
// error (contract §3.2), named on the same model as engine.MotionError so
// both error families read consistently across the codebase.
type CardTypeContentMismatchError struct {
	CardID string
	Field  string
	Owner  QuestionType
	Type   QuestionType
}

func (e *CardTypeContentMismatchError) Error() string {
	return "CARD_TYPE_CONTENT_MISMATCH: card " + e.CardID + " declares TYPE=" + string(e.Type) +
		" but carries " + e.Field + ", which belongs to " + string(e.Owner)
}
