package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// ============================================================================
// Enumerations shared with the frontend (contract ai-generation.md §6) — this
// file is the source of truth on the backend; ConfigPage/QuestionsPage must
// mirror these lists exactly.
// ============================================================================

var quizPopulations = []string{
	"Junior (6-12 ans)", "Ado (13-17 ans)", "Adulte (18-64 ans)", "Senior (65+ ans)", "Famille",
}
var quizDifficulties = []string{"Facile", "Moyen", "Difficile", "Expert"}
var quizLanguages = []string{"Français", "Anglais", "Espagnol"}

// ARDOISE joined this list in v6.1.x (#137 Batch 2a, QUALIF feedback point 2,
// _work/reports/planner-20260806-121743-qualif-137.md §2): the original #8
// spec (backlog/TODO/generateur-ia.md) mischaracterized it as a display mode
// reusing other questions' content, but the model says otherwise
// (game/models.go:161 — a QuestionType at the same rank as SPEEDY/QCM/
// MEMORY/MEMOTION, with its own persisted content). Structurally
// ARDOISE = SPEEDY + ARDOISE_KEYBOARD_TYPE; contracts/ai-generation.md §5-6
// amended accordingly (dev-backend, same commit as this change per the
// team-lead's explicit instruction to land the contract fix with the code,
// not after — cf. the §0bis incident on a prior fix in this same file).
// MEMOTION_PLUS joined this list in v7.1.0 (#196, contract ai-generation.md
// §3ter). It is a GENERATION-ONLY pseudo-type — never a game.QuestionType,
// never allowed to reach questionTypeRegistry or AllQuestionTypes() — that
// only exists to select an alternate schema variant (MOTION_CARDS items
// carrying their own per-card TYPE, SPEEDY or QCM). A question generated
// under it is ALWAYS persisted with TYPE="MEMOTION" (mapGeneratedQuestion
// normalizes this — the contract's "invariant central": the string
// "MEMOTION_PLUS" must never reach a question.json). Deliberately kept
// distinct from "MEMOTION" as its own top-level schema variant (see
// buildQuestionSchema) rather than a request-level flag, so the model can
// mix classic and "+" MEMOTION cards freely across questions in the same
// batch — the same mechanism already used to mix all the other types.
var generableQuestionTypes = []string{"SPEEDY", "QCM", "MEMORY", "MEMOTION", "MEMOTION_PLUS", "ARDOISE"}

func stringInSlice(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ============================================================================
// Request (contract §3)
// ============================================================================

// Populations replaces the v6.0.0 singular Population (string) as of v6.1.0
// (#137 Batch 2b, contract ai-generation.md §3bis) — ⚠️ BREAKING: a caller
// still posting the old "population" field gets a 400 (the field simply
// isn't recognized; there's no fallback, cf. §3ter). Objectives is new:
// the game's global objective, optional, distinct from Instructions (which
// stays per-generation and is never persisted — see buildGenerationPrompt).
type generateQuestionsRequest struct {
	Theme        string         `json:"theme"`
	Populations  []string       `json:"populations"`
	Language     string         `json:"language"`
	Difficulties []string       `json:"difficulties"`
	Objectives   string         `json:"objectives"`
	Instructions string         `json:"instructions"`
	Categories   []string       `json:"categories"`
	Volume       generateVolume `json:"volume"`
	Distribution map[string]int `json:"distribution"`
}

type generateVolume struct {
	Mode  string `json:"mode"`
	Value int    `json:"value"`
}

// resolveTotalQuestionTarget converts the request's volume (count or
// duration) into a concrete total question count to reach across all
// batches (contract ai-multi-provider.md §2 — batching needs a fixed target,
// unlike #8's single call where "estimate yourself" was fine for duration
// mode). For duration mode, ~90 seconds of game time per question is a
// reasonable average across SPEEDY/QCM (quick buzzer rounds) and
// MEMORY/MEMOTION (slower, multi-step) — not specified by either contract,
// a documented engineering default for #137.
func resolveTotalQuestionTarget(vol generateVolume, maxQuestions int) int {
	var total int
	switch vol.Mode {
	case "count":
		total = vol.Value
	case "duration":
		total = (vol.Value * 60) / 90
		if total < 1 {
			total = 1
		}
	}
	if maxQuestions > 0 && total > maxQuestions {
		total = maxQuestions
	}
	return total
}

// validateGenerateRequest returns "" when req is valid, or a human-readable
// message (never containing the API key) describing the first violation
// found. Contract §3's validation table.
func (h *HTTPServer) validateGenerateRequest(req *generateQuestionsRequest, aiCfg config.AIConfig) string {
	if strings.TrimSpace(req.Theme) == "" {
		return "theme is required"
	}
	if len(req.Theme) > 200 {
		return "theme must be at most 200 characters"
	}
	if len(req.Populations) == 0 {
		return "populations must contain at least one value"
	}
	seenPop := make(map[string]bool, len(req.Populations))
	for _, p := range req.Populations {
		if !stringInSlice(quizPopulations, p) {
			return fmt.Sprintf("populations contains an unknown value: %q", p)
		}
		if seenPop[p] {
			return fmt.Sprintf("populations contains a duplicate value: %q", p)
		}
		seenPop[p] = true
	}
	if !stringInSlice(quizLanguages, req.Language) {
		return "language must be one of the known values"
	}
	if len(req.Difficulties) == 0 {
		return "difficulties must contain at least one value"
	}
	seenDiff := make(map[string]bool, len(req.Difficulties))
	for _, d := range req.Difficulties {
		if !stringInSlice(quizDifficulties, d) {
			return fmt.Sprintf("difficulties contains an unknown value: %q", d)
		}
		if seenDiff[d] {
			return fmt.Sprintf("difficulties contains a duplicate value: %q", d)
		}
		seenDiff[d] = true
	}
	if len(req.Objectives) > 2000 {
		return "objectives must be at most 2000 characters"
	}
	if len(req.Instructions) > 2000 {
		return "instructions must be at most 2000 characters"
	}
	if len(req.Categories) == 0 {
		return "categories must contain at least one value"
	}
	for _, c := range req.Categories {
		name, imageURL, color := h.ResolveCategoryMeta(c)
		if name == "" && imageURL == "" && color == "" {
			return fmt.Sprintf("unknown category: %q", c)
		}
	}

	switch req.Volume.Mode {
	case "count":
		maxQuestions := aiCfg.MaxQuestions
		if maxQuestions <= 0 {
			maxQuestions = 200
		}
		if req.Volume.Value < 1 || req.Volume.Value > maxQuestions {
			return fmt.Sprintf("volume.value must be between 1 and %d for mode=count", maxQuestions)
		}
	case "duration":
		if req.Volume.Value < 5 || req.Volume.Value > 240 {
			return "volume.value must be between 5 and 240 (minutes) for mode=duration"
		}
	default:
		return `volume.mode must be "count" or "duration"`
	}

	if len(req.Distribution) == 0 {
		return "distribution is required"
	}
	sum := 0
	anyPositive := false
	for k, v := range req.Distribution {
		if !stringInSlice(generableQuestionTypes, k) {
			return fmt.Sprintf("distribution contains an unknown type: %q", k)
		}
		if v < 0 {
			return "distribution values must be >= 0"
		}
		if v > 0 {
			anyPositive = true
		}
		sum += v
	}
	if sum != 100 {
		return fmt.Sprintf("distribution values must sum to 100, got %d", sum)
	}
	if !anyPositive {
		return "distribution must have at least one type with a value > 0"
	}

	return ""
}

// ============================================================================
// LLM output schema (contract §5) — no minItems/maxItems/minLength: structured
// outputs don't support them (contract §5, "Limites des structured outputs à
// connaître"). Cardinality is enforced server-side in validateGeneratedQuestions.
// ============================================================================

func mergeJSONSchemaProps(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// activeGenerableTypes returns the subset of generableQuestionTypes present
// in distribution with a value > 0 — the ONLY types buildQuestionSchema
// needs to include in its anyOf for a given batch (#196 QUALIF v7.1.0.7
// bugfix, see buildQuestionSchema's own doc comment). Order follows
// generableQuestionTypes, purely for deterministic output — the schema
// itself doesn't care about anyOf ordering.
func activeGenerableTypes(distribution map[string]int) []string {
	var out []string
	for _, t := range generableQuestionTypes {
		if v, ok := distribution[t]; ok && v > 0 {
			out = append(out, t)
		}
	}
	return out
}

// buildQuestionSchema builds the json_schema output_config format for the
// batch generation call, restricting CATEGORY/DIFFICULTY to exactly the
// values the admin selected in this request (contract §5), and its anyOf to
// exactly the types the caller actually requested (activeTypes —
// activeGenerableTypes(req.Distribution) in production; validateGenerateRequest
// already guarantees at least one type has a positive value, so this is
// never empty on the real path).
//
// #196 QUALIF v7.1.0.7 bugfix — activeTypes filtering added here. Before
// this fix, EVERY anyOf branch (all 6 types, unconditionally) was sent on
// EVERY batch call regardless of what the admin's distribution actually
// requested — a 100%-SPEEDY job still paid for QCM/MEMORY/MEMOTION/
// MEMOTION_PLUS/ARDOISE's full schemas. Already wasteful before #196 (a
// pre-existing inefficiency, not itself a bug — Groq's Anthropic-sized
// context budget headroom absorbed it), it became a real failure once
// MEMOTION_PLUS's own schema (its MOTION_CARDS items are a nested
// discriminated union, meaningfully larger than the other variants) pushed
// an ALREADY-tight request over Groq's 8 000 TPM ceiling
// (contracts/ai-multi-provider.md §1) — the reported symptom ("rate limit
// exceeded" on the very FIRST call, no prior usage this session) is Groq's
// 413 pre-check on request size, not an actual quota exhaustion; see
// classifyGroqError's own doc comment for the second half of this fix (the
// misleading generic error message). Filtering to only the requested types
// both fixes the immediate regression and removes the pre-existing waste —
// a MEMOTION_PLUS-only job no longer pays for 5 schemas it will never use.
func buildQuestionSchema(categories, difficulties []string, activeTypes []string) map[string]any {
	common := map[string]any{
		"TYPE":       map[string]any{"type": "string", "enum": generableQuestionTypes},
		"CATEGORY":   map[string]any{"type": "string", "enum": categories},
		"QUESTION":   map[string]any{"type": "string"},
		"TIME":       map[string]any{"type": "integer"},
		"DIFFICULTY": map[string]any{"type": "string", "enum": difficulties},
	}

	speedyProps := mergeJSONSchemaProps(common, map[string]any{
		"ANSWER": map[string]any{"type": "string"},
	})
	ardoiseProps := mergeJSONSchemaProps(common, map[string]any{
		"ANSWER":                map[string]any{"type": "string"},
		"ARDOISE_KEYBOARD_TYPE": map[string]any{"type": "string", "enum": []string{"AZERTY", "NUMPAD"}},
	})
	// qcmAnswersSchema/qcmCorrectSchema factored out (#196) — reused
	// byte-for-byte by the MEMOTION_PLUS QCM-card variant below, which
	// needs the identical shape one level deeper (inside a MOTION_CARDS
	// item instead of at question root).
	qcmAnswersSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"RED", "GREEN", "YELLOW", "BLUE"},
		"properties": map[string]any{
			"RED":    map[string]any{"type": "string"},
			"GREEN":  map[string]any{"type": "string"},
			"YELLOW": map[string]any{"type": "string"},
			"BLUE":   map[string]any{"type": "string"},
		},
	}
	qcmCorrectSchema := map[string]any{"type": "string", "enum": []string{"RED", "GREEN", "YELLOW", "BLUE"}}
	qcmProps := mergeJSONSchemaProps(common, map[string]any{
		"QCM_ANSWERS": qcmAnswersSchema,
		"QCM_CORRECT": qcmCorrectSchema,
	})
	memoryProps := mergeJSONSchemaProps(common, map[string]any{
		"MEMORY_PAIRS": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"LEFT", "RIGHT"},
				"properties": map[string]any{
					"LEFT":  map[string]any{"type": "string"},
					"RIGHT": map[string]any{"type": "string"},
				},
			},
		},
	})
	motionProps := mergeJSONSchemaProps(common, map[string]any{
		"MOTION_CARDS": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"RECTO_THEME", "QUESTION_TEXT", "ANSWER_TEXT", "DIFFICULTY"},
				"properties": map[string]any{
					"RECTO_THEME":   map[string]any{"type": "string"},
					"QUESTION_TEXT": map[string]any{"type": "string"},
					"ANSWER_TEXT":   map[string]any{"type": "string"},
					"DIFFICULTY":    map[string]any{"type": "integer", "enum": []int{1, 2, 3}},
				},
			},
		},
	})

	// motionPlusProps (#196, contract §3ter "Portée du mode MEMOTION_PLUS")
	// — the ONLY schema difference from classic MEMOTION: MOTION_CARDS
	// items are themselves a discriminated union (their OWN "TYPE", const
	// per branch, exactly the same pattern as the top-level `variant`
	// below) instead of the flat 4-property object motionProps declares.
	// motionProps itself is byte-for-byte UNCHANGED above — that is what
	// makes classic MEMOTION's non-regression a structural guarantee
	// rather than a promise: the model literally cannot attach a TYPE (or
	// QCM_* fields) to a classic MEMOTION card, additionalProperties:false
	// on that object still forbids it.
	//
	// Only SPEEDY and QCM appear here — the two, and only two, types the
	// registry marks NestableInMotionCard (contracts/question-types.md
	// §7); MEMORY/ARDOISE/MEMOTION itself are not offered as card types
	// and never will be without a registry change of their own.
	motionCardCommon := map[string]any{
		"RECTO_THEME":   map[string]any{"type": "string"},
		"QUESTION_TEXT": map[string]any{"type": "string"},
		"DIFFICULTY":    map[string]any{"type": "integer", "enum": []int{1, 2, 3}},
	}
	motionCardSpeedyProps := mergeJSONSchemaProps(motionCardCommon, map[string]any{
		"ANSWER_TEXT": map[string]any{"type": "string"},
	})
	// QCM's OwnedFields are QCM_ANSWERS/QCM_CORRECT, never ANSWER_TEXT
	// (contracts/question-types.md §3.1/§3.2 — a card must never carry
	// content belonging to a different type than its own declared TYPE,
	// enforced server-side by ValidateCardTypeContent on every card
	// regardless of origin). additionalProperties:false on the QCM-card
	// variant below makes the LLM structurally unable to attach
	// ANSWER_TEXT to a QCM card — this cannot fail
	// CARD_TYPE_CONTENT_MISMATCH by construction, the same guarantee
	// classic MEMOTION already relies on.
	motionCardQCMProps := mergeJSONSchemaProps(motionCardCommon, map[string]any{
		"QCM_ANSWERS": qcmAnswersSchema,
		"QCM_CORRECT": qcmCorrectSchema,
	})
	motionCardVariant := func(props map[string]any, typeConst string, extraRequired ...string) map[string]any {
		p := mergeJSONSchemaProps(props, map[string]any{
			"TYPE": map[string]any{"type": "string", "const": typeConst},
		})
		required := append([]string{"TYPE", "RECTO_THEME", "QUESTION_TEXT", "DIFFICULTY"}, extraRequired...)
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             required,
			"properties":           p,
		}
	}
	motionPlusProps := mergeJSONSchemaProps(common, map[string]any{
		"MOTION_CARDS": map[string]any{
			"type": "array",
			"items": map[string]any{
				"anyOf": []any{
					motionCardVariant(motionCardSpeedyProps, "SPEEDY", "ANSWER_TEXT"),
					motionCardVariant(motionCardQCMProps, "QCM", "QCM_ANSWERS", "QCM_CORRECT"),
				},
			},
		},
	})

	variant := func(props map[string]any, typeConst string, extraRequired ...string) map[string]any {
		p := mergeJSONSchemaProps(props, map[string]any{
			"TYPE": map[string]any{"type": "string", "const": typeConst},
		})
		required := append([]string{"TYPE", "CATEGORY", "QUESTION", "TIME", "DIFFICULTY"}, extraRequired...)
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             required,
			"properties":           p,
		}
	}

	// #196 QUALIF v7.1.0.7 bugfix — only build the anyOf branches for types
	// actually present in activeTypes (see this function's own doc comment
	// above for why). isActive/allVariants keep the six variant() calls
	// exactly as they were — declared unconditionally, filtered here — so a
	// future 7th type only ever needs one more map entry, not a new
	// if-cascade.
	isActive := make(map[string]bool, len(activeTypes))
	for _, t := range activeTypes {
		isActive[t] = true
	}
	allVariants := []struct {
		typeConst string
		schema    map[string]any
	}{
		{"SPEEDY", variant(speedyProps, "SPEEDY", "ANSWER")},
		{"QCM", variant(qcmProps, "QCM", "QCM_ANSWERS", "QCM_CORRECT")},
		{"MEMORY", variant(memoryProps, "MEMORY", "MEMORY_PAIRS")},
		{"MEMOTION", variant(motionProps, "MEMOTION", "MOTION_CARDS")},
		{"MEMOTION_PLUS", variant(motionPlusProps, "MEMOTION_PLUS", "MOTION_CARDS")}, // #196
		{"ARDOISE", variant(ardoiseProps, "ARDOISE", "ANSWER", "ARDOISE_KEYBOARD_TYPE")},
	}
	anyOf := make([]any, 0, len(allVariants))
	for _, v := range allVariants {
		if isActive[v.typeConst] {
			anyOf = append(anyOf, v.schema)
		}
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"questions"},
		"properties": map[string]any{
			"questions": map[string]any{
				"type":  "array",
				"items": map[string]any{"anyOf": anyOf},
			},
		},
	}
}

// ============================================================================
// Raw LLM output (contract §5) — deliberately has NO id/order/media field:
// "garantie d'additivité par construction", the schema above enforces the
// same via additionalProperties:false.
// ============================================================================

type llmQCMAnswers struct {
	Red    string `json:"RED"`
	Green  string `json:"GREEN"`
	Yellow string `json:"YELLOW"`
	Blue   string `json:"BLUE"`
}

type llmMemoryPair struct {
	Left  string `json:"LEFT"`
	Right string `json:"RIGHT"`
}

type llmMotionCard struct {
	// Type (#196) — "" for a classic MEMOTION card (the schema's motionProps
	// item has no TYPE property at all; the field simply never gets set
	// during unmarshal) or, under MEMOTION_PLUS, an explicit "SPEEDY"/"QCM"
	// from the model. Never any other value — motionPlusCardsValid rejects
	// anything else, defensively, even though the schema itself already
	// makes a different value structurally unreachable.
	Type         string         `json:"TYPE,omitempty"`
	RectoTheme   string         `json:"RECTO_THEME"`
	QuestionText string         `json:"QUESTION_TEXT"`
	AnswerText   string         `json:"ANSWER_TEXT,omitempty"`
	Difficulty   int            `json:"DIFFICULTY"`
	QCMAnswers   *llmQCMAnswers `json:"QCM_ANSWERS,omitempty"`
	QCMCorrect   string         `json:"QCM_CORRECT,omitempty"`
}

type llmRawQuestion struct {
	Type                string          `json:"TYPE"`
	Category            string          `json:"CATEGORY"`
	Question            string          `json:"QUESTION"`
	Time                int             `json:"TIME"`
	Difficulty          string          `json:"DIFFICULTY"`
	Answer              string          `json:"ANSWER,omitempty"`
	QCMAnswers          *llmQCMAnswers  `json:"QCM_ANSWERS,omitempty"`
	QCMCorrect          string          `json:"QCM_CORRECT,omitempty"`
	MemoryPairs         []llmMemoryPair `json:"MEMORY_PAIRS,omitempty"`
	MotionCards         []llmMotionCard `json:"MOTION_CARDS,omitempty"`
	ArdoiseKeyboardType string          `json:"ARDOISE_KEYBOARD_TYPE,omitempty"`
}

type llmResponseRoot struct {
	Questions []llmRawQuestion `json:"questions"`
}

// parseLLMResponse decodes the accumulated text content of the streamed
// response. A decode failure means the model's output didn't conform to the
// requested schema (contract §3 — mapped to 502 upstream_error by the caller,
// "réponse non conforme au schéma").
func parseLLMResponse(rawJSON string) ([]llmRawQuestion, error) {
	var root llmResponseRoot
	if err := json.Unmarshal([]byte(rawJSON), &root); err != nil {
		return nil, fmt.Errorf("AI response is not valid JSON matching the expected schema: %w", err)
	}
	return root.Questions, nil
}

// ============================================================================
// Server-side validation (contract §5.1) — structured outputs cannot enforce
// cardinality (minItems/minLength/min/max), so every question is checked here
// before being written. An invalid question is skipped, never a hard failure.
// ============================================================================

func qcmAnswersValid(a *llmQCMAnswers) bool {
	if a == nil {
		return false
	}
	values := []string{a.Red, a.Green, a.Yellow, a.Blue}
	seen := make(map[string]bool, 4)
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return false
		}
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func memoryPairsValid(pairs []llmMemoryPair) bool {
	for _, p := range pairs {
		if strings.TrimSpace(p.Left) == "" || strings.TrimSpace(p.Right) == "" {
			return false
		}
	}
	return true
}

func motionCardsValid(cards []llmMotionCard) bool {
	for _, c := range cards {
		if strings.TrimSpace(c.RectoTheme) == "" || strings.TrimSpace(c.AnswerText) == "" {
			return false
		}
		if c.Difficulty < 1 || c.Difficulty > 3 {
			return false
		}
	}
	return true
}

// motionPlusCardsValid is motionCardsValid's #196 counterpart: each card's
// per-card TYPE (SPEEDY or QCM — the two contracts/question-types.md §7
// marks NestableInMotionCard, contract §3ter "Portée du mode MEMOTION_PLUS")
// determines which content fields apply. Mirrors, per card, the exact same
// checks validateGeneratedQuestions already applies per QUESTION for SPEEDY
// (ANSWER non-empty) and QCM (qcmAnswersValid + QCM_CORRECT ∈ enum) —
// deliberately not a new rule, the same rule at one nesting level deeper.
func motionPlusCardsValid(cards []llmMotionCard) bool {
	for _, c := range cards {
		if strings.TrimSpace(c.RectoTheme) == "" {
			return false
		}
		if c.Difficulty < 1 || c.Difficulty > 3 {
			return false
		}
		switch c.Type {
		case "", "SPEEDY":
			if strings.TrimSpace(c.AnswerText) == "" {
				return false
			}
		case "QCM":
			if !qcmAnswersValid(c.QCMAnswers) {
				return false
			}
			switch c.QCMCorrect {
			case "RED", "GREEN", "YELLOW", "BLUE":
			default:
				return false
			}
		default:
			// Structurally unreachable (the schema's per-card TYPE is a
			// const ∈ {SPEEDY,QCM}) — defended anyway, same posture as the
			// question-level `default:` case in validateGeneratedQuestions.
			return false
		}
	}
	return true
}

// validateGeneratedQuestions filters raw to only the questions that pass
// contract §5.1's per-type rules, clamping TIME in place, and caps the result
// at maxQuestions (excess dropped, contract §5.1 "Volume"). Returns the
// accepted questions plus one human-readable reason per rejected/dropped one
// (never the raw LLM content verbatim beyond the TYPE/CATEGORY needed to
// locate it — no secret material flows through this path anyway).
// allowedDifficulties (v6.1.1, issue #142) is checked server-side now for
// both providers: Groq's schema no longer restricts DIFFICULTY via enum
// (ai-multi-provider.md §7, groqProvider.AdaptSchema) so this is Groq's ONLY
// remaining guarantee against an off-list value; Anthropic's schema still
// enforces it structurally (the LLM can't emit a DIFFICULTY outside its
// enum), so this check is a no-op in practice there — contract §5.1.
func validateGeneratedQuestions(raw []llmRawQuestion, allowedCategories, allowedDifficulties []string, maxQuestions int) (valid []llmRawQuestion, reasons []string) {
	allowed := make(map[string]bool, len(allowedCategories))
	for _, c := range allowedCategories {
		allowed[c] = true
	}
	allowedDiff := make(map[string]bool, len(allowedDifficulties))
	for _, d := range allowedDifficulties {
		allowedDiff[d] = true
	}

	for _, q := range raw {
		q.Question = strings.TrimSpace(q.Question)

		if q.Category == "" || !allowed[q.Category] {
			reasons = append(reasons, fmt.Sprintf("%s: catégorie inconnue ou non demandée (%q)", q.Type, q.Category))
			continue
		}
		if q.Difficulty == "" || !allowedDiff[q.Difficulty] {
			reasons = append(reasons, fmt.Sprintf("%s: difficulté inconnue ou non demandée (%q)", q.Type, q.Difficulty))
			continue
		}
		if q.Question == "" {
			reasons = append(reasons, fmt.Sprintf("%s: énoncé vide", q.Type))
			continue
		}
		if q.Time < 5 {
			q.Time = 5
		} else if q.Time > 300 {
			q.Time = 300
		}

		switch q.Type {
		case "SPEEDY":
			if strings.TrimSpace(q.Answer) == "" {
				reasons = append(reasons, "SPEEDY: réponse vide")
				continue
			}
		case "QCM":
			if !qcmAnswersValid(q.QCMAnswers) {
				reasons = append(reasons, "QCM: moins de 4 réponses valides et distinctes")
				continue
			}
			switch q.QCMCorrect {
			case "RED", "GREEN", "YELLOW", "BLUE":
			default:
				reasons = append(reasons, "QCM: QCM_CORRECT invalide")
				continue
			}
		case "MEMORY":
			if len(q.MemoryPairs) < 2 || len(q.MemoryPairs) > 12 || !memoryPairsValid(q.MemoryPairs) {
				reasons = append(reasons, "MEMORY: moins de 2 paires valides")
				continue
			}
		case "MEMOTION":
			if len(q.MotionCards) < 4 || len(q.MotionCards) > 12 || !motionCardsValid(q.MotionCards) {
				reasons = append(reasons, "MEMOTION: moins de 4 cartes valides")
				continue
			}
		case "MEMOTION_PLUS": // #196
			if len(q.MotionCards) < 4 || len(q.MotionCards) > 12 || !motionPlusCardsValid(q.MotionCards) {
				reasons = append(reasons, "MEMOTION_PLUS: moins de 4 cartes valides")
				continue
			}
		case "ARDOISE":
			if strings.TrimSpace(q.Answer) == "" {
				reasons = append(reasons, "ARDOISE: réponse vide")
				continue
			}
			switch q.ArdoiseKeyboardType {
			case "AZERTY", "NUMPAD":
			default:
				reasons = append(reasons, "ARDOISE: ARDOISE_KEYBOARD_TYPE invalide")
				continue
			}
		default:
			reasons = append(reasons, fmt.Sprintf("type inconnu: %q", q.Type))
			continue
		}

		valid = append(valid, q)
	}

	if maxQuestions <= 0 {
		maxQuestions = 200
	}
	if len(valid) > maxQuestions {
		excess := len(valid) - maxQuestions
		valid = valid[:maxQuestions]
		for i := 0; i < excess; i++ {
			reasons = append(reasons, "volume: plafond max_questions atteint")
		}
	}

	return valid, reasons
}

// ============================================================================
// Mapping to question.json (contract §5.2) — must be structurally identical
// to handleUploadQuestion's output, hence the same map[string]interface{}
// construction (not the game.Question struct, which handleUploadQuestion
// doesn't use either).
// ============================================================================

func speedyOrQCMPoints(difficulty string) string {
	switch difficulty {
	case "Facile":
		return "10"
	case "Moyen":
		return "20"
	case "Difficile":
		return "30"
	case "Expert":
		return "50"
	default:
		return "10"
	}
}

func mapGeneratedQuestion(q llmRawQuestion, id string, order int) map[string]interface{} {
	// #196, contract §3ter's central invariant — 🔴 normalized here, BEFORE
	// any field of `question` is written: MEMOTION_PLUS is a generation-only
	// pseudo-type and must NEVER reach question.json. persistedType (not
	// q.Type) is what actually gets written to the TYPE field below; the
	// switch further down still dispatches on q.Type (it needs to tell
	// MEMOTION_PLUS apart from MEMOTION to know whether cards may carry
	// their own TYPE), but every map key ends up identical either way once
	// normalized — see the merged "MEMOTION", "MEMOTION_PLUS" case.
	persistedType := q.Type
	if persistedType == "MEMOTION_PLUS" {
		persistedType = "MEMOTION"
	}

	pointsTarget := "TEAM"
	if persistedType == "SPEEDY" {
		pointsTarget = "PLAYER"
	}

	question := map[string]interface{}{
		"ID":                id,
		"QUESTION":          q.Question,
		"TYPE":              persistedType,
		"TIME":              strconv.Itoa(q.Time),
		"POINTS_TARGET":     pointsTarget,
		"ORDER":             order,
		"QCM_HINTS_ENABLED": false,
	}
	if q.Category != "" {
		question["CATEGORY"] = q.Category
	}

	switch q.Type {
	case "SPEEDY":
		question["ANSWER"] = q.Answer
		question["POINTS"] = speedyOrQCMPoints(q.Difficulty)

	case "ARDOISE":
		// POINTS_TARGET already defaults to "TEAM" above (only SPEEDY gets
		// "PLAYER"), matching the real sample question.json and the plan's
		// Q2.2 decision — no override needed here.
		question["ANSWER"] = q.Answer
		question["POINTS"] = speedyOrQCMPoints(q.Difficulty)
		question["ARDOISE_KEYBOARD_TYPE"] = q.ArdoiseKeyboardType

	case "QCM":
		question["ANSWER"] = ""
		question["POINTS"] = speedyOrQCMPoints(q.Difficulty)
		question["QCM_ANSWERS"] = map[string]string{
			"RED":    q.QCMAnswers.Red,
			"GREEN":  q.QCMAnswers.Green,
			"YELLOW": q.QCMAnswers.Yellow,
			"BLUE":   q.QCMAnswers.Blue,
		}
		question["QCM_CORRECT"] = q.QCMCorrect

	case "MEMORY":
		question["ANSWER"] = fmt.Sprintf("%d paires", len(q.MemoryPairs))
		question["POINTS"] = "0"
		question["MEMORY_MODE"] = "SOLO"
		pairs := make([]map[string]interface{}, len(q.MemoryPairs))
		for i, p := range q.MemoryPairs {
			pairs[i] = map[string]interface{}{
				"ID":    i + 1,
				"CARD1": map[string]interface{}{"TEXT": p.Left, "IS_IMAGE": false},
				"CARD2": map[string]interface{}{"TEXT": p.Right, "IS_IMAGE": false},
			}
		}
		question["MEMORY_PAIRS"] = pairs
		question["MEMORY_CONFIG"] = map[string]interface{}{
			"FLIP_DELAY":           3,
			"POINTS_PER_PAIR":      10,
			"ERROR_PENALTY":        0,
			"COMPLETION_BONUS":     20,
			"USE_TIMER":            true,
			"MEMORIZE_TIME":        5,
			"SHOW_DURING_MEMORIZE": true,
			"REVEAL_DELAY":         0.5,
		}

	case "MEMOTION", "MEMOTION_PLUS": // #196 — same construction, persistedType already normalized above
		question["ANSWER"] = fmt.Sprintf("%d cartes", len(q.MotionCards))
		question["POINTS"] = "1"
		question["MOTION_MODE"] = "CHACUN_SON_TOUR"
		question["MOTION_MEMORIZE_DURATION"] = 0
		cards := make([]map[string]interface{}, len(q.MotionCards))
		for i, c := range q.MotionCards {
			card := map[string]interface{}{
				"ID":            fmt.Sprintf("mc-%d", i+1),
				"RECTO_THEME":   c.RectoTheme,
				"DIFFICULTY":    c.Difficulty,
				"QUESTION_TEXT": c.QuestionText,
			}
			// #196 — a classic MEMOTION card never has c.Type set (the
			// schema's plain motionProps item has no TYPE property at all),
			// so this always takes the SPEEDY branch below: byte-for-byte
			// the pre-#196 output. A MEMOTION_PLUS card carries its own
			// TYPE — SPEEDY (same shape as a classic card) or QCM (its own
			// OwnedFields, contract §3.1; NEVER ANSWER_TEXT — a card must
			// never carry another type's content, contract §3.2, enforced
			// downstream by the same ValidateCardTypeContent an editor
			// upload goes through, contract §3ter "Aucun élargissement de
			// la validation d'imbrication n'est nécessaire").
			switch c.Type {
			case "QCM":
				card["TYPE"] = "QCM"
				card["QCM_ANSWERS"] = map[string]string{
					"RED":    c.QCMAnswers.Red,
					"GREEN":  c.QCMAnswers.Green,
					"YELLOW": c.QCMAnswers.Yellow,
					"BLUE":   c.QCMAnswers.Blue,
				}
				card["QCM_CORRECT"] = c.QCMCorrect
			default: // "" (classic MEMOTION) or explicit "SPEEDY"
				card["ANSWER_TEXT"] = c.AnswerText
			}
			cards[i] = card
		}
		question["MOTION_CARDS"] = cards
		question["MOTION_CONFIG"] = map[string]interface{}{
			"POINTS_1_STAR": 1,
			"POINTS_2_STAR": 3,
			"POINTS_3_STAR": 5,
		}
	}

	return question
}

// ============================================================================
// Context injected into the prompt (contract §4) — capped at 150 questions,
// reduced to {TYPE, CATEGORY, QUESTION}, each truncated to 200 characters.
// ============================================================================

type existingQuestionContext struct {
	Type     string `json:"TYPE"`
	Category string `json:"CATEGORY"`
	Question string `json:"QUESTION"`
}

// collectExistingQuestionsContext reads every question already stored in the
// requested categories, most recent first (by ORDER, matching how generated
// questions are placed at the end of the list), capped at 150 (contract §4 —
// without this cap, a large question bank would blow up the request's input
// cost on every single generation).
func (h *HTTPServer) collectExistingQuestionsContext(categories []string) []existingQuestionContext {
	allowed := make(map[string]bool, len(categories))
	for _, c := range categories {
		allowed[c] = true
	}

	questionsDir := filepath.Join(h.dataDir, "files", "questions")
	entries, err := os.ReadDir(questionsDir)
	if err != nil {
		return nil
	}

	type withOrder struct {
		ctx   existingQuestionContext
		order int
	}
	var collected []withOrder
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(questionsDir, entry.Name(), "question.json"))
		if err != nil {
			continue
		}
		var q struct {
			Type     string `json:"TYPE"`
			Category string `json:"CATEGORY"`
			Question string `json:"QUESTION"`
			Order    int    `json:"ORDER"`
		}
		if json.Unmarshal(data, &q) != nil || !allowed[q.Category] {
			continue
		}
		text := q.Question
		if len(text) > 200 {
			text = text[:200]
		}
		collected = append(collected, withOrder{
			ctx:   existingQuestionContext{Type: q.Type, Category: q.Category, Question: text},
			order: q.Order,
		})
	}

	sort.Slice(collected, func(i, j int) bool { return collected[i].order > collected[j].order })
	if len(collected) > 150 {
		collected = collected[:150]
	}

	out := make([]existingQuestionContext, len(collected))
	for i, c := range collected {
		out[i] = c.ctx
	}
	return out
}

// buildGenerationPrompt assembles the instruction sent to the model for ONE
// batch: the admin's parameters, an explicit count for THIS call (batching,
// contract ai-multi-provider.md §2 — never the full requested volume), and
// the anti-duplicate context (contract ai-generation.md §4), extended with
// extraContext — the questions already produced by earlier batches of the
// same run, so a multi-batch job doesn't duplicate itself (contract
// ai-multi-provider.md §2 "enrichi à chaque lot").
func (h *HTTPServer) buildGenerationPrompt(req generateQuestionsRequest, batchCount int, extraContext []existingQuestionContext) string {
	var b strings.Builder

	b.WriteString("Tu es un générateur de questions pour un jeu de quiz de type buzzer/plateau. ")
	b.WriteString("Génère des questions strictement conformes au schéma JSON fourni — n'ajoute aucun champ hors schéma.\n\n")

	// Ordre normatif contract ai-generation.md §4 (v6.1.0) — l'objectif global
	// (rang 5) précède toujours les précisions de génération (rang 6) : le
	// cadre avant l'ajustement. Un champ optionnel vide n'émet aucune ligne.
	fmt.Fprintf(&b, "Thème : %s\n", req.Theme)
	fmt.Fprintf(&b, "Publics cibles : %s\n", strings.Join(req.Populations, ", "))
	fmt.Fprintf(&b, "Langue de rédaction : %s\n", req.Language)
	fmt.Fprintf(&b, "Niveaux de difficulté autorisés : %s\n", strings.Join(req.Difficulties, ", "))
	if strings.TrimSpace(req.Objectives) != "" {
		fmt.Fprintf(&b, "Objectif de la partie : %s\n", req.Objectives)
	}
	if strings.TrimSpace(req.Instructions) != "" {
		fmt.Fprintf(&b, "Précisions pour cette génération : %s\n", req.Instructions)
	}
	fmt.Fprintf(&b, "Catégories autorisées (utilise exactement ces clés dans CATEGORY) : %s\n", strings.Join(req.Categories, ", "))

	// Batching (#137): each provider call asks for an explicit, bounded
	// count — never the admin's full requested volume in one shot.
	fmt.Fprintf(&b, "Nombre de questions à générer dans cet appel : %d\n", batchCount)

	var parts []string
	for _, t := range generableQuestionTypes {
		if v, ok := req.Distribution[t]; ok && v > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d%%", t, v))
		}
	}
	fmt.Fprintf(&b, "Répartition cible par type : %s\n", strings.Join(parts, ", "))

	b.WriteString("Pour chaque question, indique aussi TIME (durée de jeu en secondes, adaptée au type/à la difficulté/au public) ")
	b.WriteString("et DIFFICULTY (une valeur parmi celles autorisées ci-dessus).\n")

	// MEMOTION-specific guidance (#137, QA report qa-20260806-111416.md §5.4):
	// on Groq/gpt-oss-120b, MEMOTION was massively under-produced (2% actual
	// vs 15% requested) compared to the other 3 types. Two structural
	// reasons the schema alone can't convey to a smaller model: (1) the
	// json_schema output has no minItems/maxItems support (see the comment
	// above buildQuestionSchema), so nothing stops the model from emitting
	// fewer than the 4 cards validateGeneratedQuestions requires — it has to
	// be told in prose; (2) RECTO_THEME/QUESTION_TEXT/ANSWER_TEXT is a
	// three-way relationship (short grid label → riddle → precise answer,
	// docs/DATA_MODELS.md "Question (MEMOTION type)") that field names alone
	// don't make obvious — a real rejected sample from QA (RECTO_THEME:
	// "Pâte de soja fermentée" while QUESTION_TEXT asked about a different
	// food, kimchi) is included verbatim below as a concrete negative
	// example, which grounds the instruction far better than an abstract
	// rule. Kept conditional on MEMOTION actually being requested this job,
	// so batches with distribution["MEMOTION"]==0 don't pay the extra prompt
	// tokens for guidance they don't need.
	if v, ok := req.Distribution["MEMOTION"]; ok && v > 0 {
		b.WriteString("\nFormat détaillé pour TYPE=\"MEMOTION\" (jeu de cartes à 3 faces — le type le plus exigeant structurellement, à suivre à la lettre) :\n")
		b.WriteString("- MOTION_CARDS doit contenir ENTRE 4 ET 12 cartes. En dessous de 4, la question MEMOTION entière est rejetée — 4 cartes suffisent largement, n'en génère pas plus que nécessaire.\n")
		b.WriteString("- RECTO_THEME : le texte affiché sur la face visible AVANT que la carte soit retournée. C'est un thème/indice COURT et GÉNÉRIQUE, commun à toutes les cartes de cette question (ex: \"Capitale d'Europe\", \"Ingrédient fermenté\") — jamais une réponse précise, et jamais identique à ANSWER_TEXT.\n")
		b.WriteString("- QUESTION_TEXT : la question posée une fois la carte retournée. Elle doit se rattacher au même thème que RECTO_THEME et avoir pour réponse exacte ANSWER_TEXT.\n")
		b.WriteString("- ANSWER_TEXT : la réponse précise et unique à QUESTION_TEXT — différente pour chaque carte d'une même question (jamais deux cartes avec la même réponse).\n")
		b.WriteString("- Exemple correct, deux cartes d'une même question sur le thème \"Capitales d'Europe\" :\n")
		b.WriteString("  {\"RECTO_THEME\": \"Capitale d'Europe\", \"QUESTION_TEXT\": \"Quelle capitale est traversée par la Seine ?\", \"ANSWER_TEXT\": \"Paris\", \"DIFFICULTY\": 1}\n")
		b.WriteString("  {\"RECTO_THEME\": \"Capitale d'Europe\", \"QUESTION_TEXT\": \"Quelle capitale abrite le Colisée ?\", \"ANSWER_TEXT\": \"Rome\", \"DIFFICULTY\": 1}\n")
		b.WriteString("- Erreur à éviter (exemple réel rejeté en test) : RECTO_THEME=\"Pâte de soja fermentée\" alors que QUESTION_TEXT porte sur un aliment différent (le kimchi) — RECTO_THEME doit rester le thème générique partagé par toutes les cartes de la question, jamais déjà une réponse précise qui entre en concurrence avec une autre carte.\n")
	}

	// MEMOTION_PLUS-specific guidance (#196, contract §3ter) — a distinct
	// distribution key from "MEMOTION" (not a flag under it, contract's own
	// wording: "au même niveau que les cinq autres"), so this block is
	// self-contained rather than assuming the MEMOTION block above also
	// fired — a job can request MEMOTION_PLUS with MEMOTION at 0%. The
	// RECTO_THEME/QUESTION_TEXT/ANSWER_TEXT rules and pitfall example are
	// therefore restated here rather than only in the MEMOTION block, same
	// reasoning as B-B1/B5 in the plan. Kept conditional on MEMOTION_PLUS
	// actually being requested this job, same pattern as every other
	// type-specific block in this function.
	if v, ok := req.Distribution["MEMOTION_PLUS"]; ok && v > 0 {
		b.WriteString("\nFormat détaillé pour TYPE=\"MEMOTION_PLUS\" (variante de MEMOTION où CHAQUE carte choisit son propre type) :\n")
		b.WriteString("- Même structure de base que MEMOTION : MOTION_CARDS doit contenir ENTRE 4 ET 12 cartes ; RECTO_THEME est un thème/indice COURT et GÉNÉRIQUE commun à toutes les cartes de la question (ex: \"Capitale d'Europe\"), jamais une réponse précise et jamais identique à la réponse d'une carte.\n")
		b.WriteString("- Différence : chaque carte de MOTION_CARDS DOIT porter un champ TYPE valant \"SPEEDY\" ou \"QCM\" — choisis-le carte par carte selon ce qui convient le mieux à SON contenu, pas un choix unique pour toute la question.\n")
		b.WriteString("- Carte TYPE=\"SPEEDY\" : QUESTION_TEXT (la question) et ANSWER_TEXT (la réponse précise et unique) — exactement comme une carte MEMOTION classique.\n")
		b.WriteString("- Carte TYPE=\"QCM\" : QUESTION_TEXT (la question), QCM_ANSWERS (objet RED/GREEN/YELLOW/BLUE, 4 réponses plausibles et distinctes, une seule correcte) et QCM_CORRECT (la couleur de la bonne réponse) — jamais de champ ANSWER_TEXT sur une carte QCM.\n")
		b.WriteString("- Choisis QCM quand des réponses fausses plausibles renforcent la question (dates, chiffres, noms proches) ; choisis SPEEDY quand la réponse est ouverte ou qu'aucun distracteur crédible n'existe.\n")
		b.WriteString("- Exemple, deux cartes d'une même question sur le thème \"Capitales d'Europe\", l'une SPEEDY et l'autre QCM :\n")
		b.WriteString("  {\"TYPE\": \"SPEEDY\", \"RECTO_THEME\": \"Capitale d'Europe\", \"QUESTION_TEXT\": \"Quelle capitale est traversée par la Seine ?\", \"ANSWER_TEXT\": \"Paris\", \"DIFFICULTY\": 1}\n")
		b.WriteString("  {\"TYPE\": \"QCM\", \"RECTO_THEME\": \"Capitale d'Europe\", \"QUESTION_TEXT\": \"Quelle capitale abrite le Colisée ?\", \"QCM_ANSWERS\": {\"RED\": \"Rome\", \"GREEN\": \"Madrid\", \"YELLOW\": \"Lisbonne\", \"BLUE\": \"Athènes\"}, \"QCM_CORRECT\": \"RED\", \"DIFFICULTY\": 1}\n")
	}

	// ARDOISE-specific guidance (#137 Batch 2a, QUALIF feedback point 2 —
	// _work/reports/planner-20260806-121743-qualif-137.md §2, arbitrage
	// utilisateur Q2.1): the model picks ARDOISE_KEYBOARD_TYPE itself
	// instead of it being forced to a fixed value, since nothing in the
	// schema can express this content-dependent rule (an enum has no way to
	// say "value depends on ANSWER's content").
	if v, ok := req.Distribution["ARDOISE"]; ok && v > 0 {
		b.WriteString("\nPour TYPE=\"ARDOISE\" : choisis ARDOISE_KEYBOARD_TYPE selon la nature de la réponse — \"NUMPAD\" si ANSWER est purement numérique (un nombre, une date, une année), \"AZERTY\" dans tous les autres cas (texte, nom propre, mot).\n")
	}

	// Anti-duplicate context: in-job questions first (freshest, most likely
	// to be duplicated by the very next batch), then on-disk history,
	// capped at 150 total (contract ai-generation.md §4). Truncating the
	// combined list keeps extraContext intact rather than dropping it in
	// favor of older on-disk entries.
	combined := make([]existingQuestionContext, 0, len(extraContext)+150)
	combined = append(combined, extraContext...)
	combined = append(combined, h.collectExistingQuestionsContext(req.Categories)...)
	if len(combined) > 150 {
		combined = combined[:150]
	}
	if len(combined) > 0 {
		b.WriteString("\nQuestions déjà présentes dans ces catégories, y compris celles déjà produites plus tôt dans cette génération — NE LES DUPLIQUE PAS :\n")
		if data, err := json.Marshal(combined); err == nil {
			b.Write(data)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ============================================================================
// HTTP handler (contract §3)
// ============================================================================

// createdEntry describes one question actually written to disk, in both the
// success response ("created") and — since the M-2 code-review fix
// (_work/reports/code-review-20260805-163118.md) — a batch-write error
// response, so a partial batch is never silently invisible to the caller.
type createdEntry struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Category string `json:"category"`
}

// generateErrorBody is used only for the SYNCHRONOUS pre-flight errors of
// POST /api/generate-questions (400 invalid_request, 405, 409 no_api_key /
// generation_in_progress — contract ai-multi-provider.md §9). Everything
// that can happen mid-generation (upstream_error, timeout, id_exhausted,
// provider_quota, internal_error, and the M-2 "questions created before a
// failure" concern) now transits exclusively through AI_GENERATION_PROGRESS
// (contract §10) — a batch failure has nowhere to "return" to once the HTTP
// response has already been sent as 202.
type generateErrorBody struct {
	Status         string `json:"status"`
	Code           string `json:"code"`
	Message        string `json:"message"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
}

func writeGenerateError(w http.ResponseWriter, httpStatus int, code, message string, upstreamStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(generateErrorBody{
		Status:         "error",
		Code:           code,
		Message:        message,
		UpstreamStatus: upstreamStatus,
	})
}

// maxQuestionOrder scans every existing question.json for its ORDER field and
// returns the highest value found (0 if none/unset) — the base for
// ORDER = max+1+i on generated questions (contract §5.2).
func (h *HTTPServer) maxQuestionOrder() int {
	questionsDir := filepath.Join(h.dataDir, "files", "questions")
	entries, err := os.ReadDir(questionsDir)
	if err != nil {
		return 0
	}
	max := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(questionsDir, entry.Name(), "question.json"))
		if err != nil {
			continue
		}
		var q struct {
			Order int `json:"ORDER"`
		}
		if json.Unmarshal(data, &q) == nil && q.Order > max {
			max = q.Order
		}
	}
	return max
}

// generateAcceptedBody is the 202 response body (contract
// ai-multi-provider.md §9 — BREAKING vs #8: this endpoint no longer returns
// the generation result directly; it starts a background job and the result
// streams via AI_GENERATION_PROGRESS, contract §10).
type generateAcceptedBody struct {
	Status       string `json:"status"`
	JobID        string `json:"job_id"`
	BatchesTotal int    `json:"batches_total"`
}

// handleGenerateQuestions implements POST /api/generate-questions (contract
// ai-generation.md §3, superseded by ai-multi-provider.md §9 for the
// asynchronous job model): validates the request and the selected
// provider's API key, then starts a background generation job and returns
// 202 immediately. All of the actual generation (batching, validation,
// writing question.json files — creation only, no existing question ever
// touched) happens in that job (ai_job.go), reported via
// AI_GENERATION_PROGRESS over /ws/admin.
func (h *HTTPServer) handleGenerateQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Hard size limit before decoding (security audit M1/M4,
	// _work/reports/security-20260805-125747.md) — the request body is a
	// small structured payload (theme/population/categories/...), 64 KB is
	// generous headroom.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req generateQuestionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGenerateError(w, http.StatusBadRequest, "invalid_request", "Corps de requête JSON invalide.", 0)
		return
	}

	aiCfg := config.Get().AI

	if msg := h.validateGenerateRequest(&req, aiCfg); msg != "" {
		writeGenerateError(w, http.StatusBadRequest, "invalid_request", msg, 0)
		return
	}

	// Security finding M3: read strictly the SELECTED provider's key, never
	// both keys OR'd together — a Groq-selected admin with only an Anthropic
	// key configured must still see "no key configured".
	if !providerAPIKeyConfigured(aiCfg) {
		message := "Aucune clé API Claude configurée."
		if aiCfg.Provider == "groq" {
			message = "Aucune clé API Groq configurée."
		}
		writeGenerateError(w, http.StatusConflict, "no_api_key", message, 0)
		return
	}

	jobID, batchesTotal, started := h.startAIGenerationJob(req, aiCfg)
	if !started {
		// contract §9: a single job at a time, tout admin confondu (security
		// M1 — the check-and-start is atomic under aiJobRegistry.mu, no
		// TOCTOU window for two concurrent POSTs to both succeed).
		writeGenerateError(w, http.StatusConflict, "generation_in_progress", "Une génération est déjà en cours.", 0)
		return
	}

	LogInfo(game.LogComponentHTTP, "AI generation job %s started: provider=%s batches_total=%d",
		jobID, aiCfg.Provider, batchesTotal)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(generateAcceptedBody{
		Status:       "accepted",
		JobID:        jobID,
		BatchesTotal: batchesTotal,
	})
}
