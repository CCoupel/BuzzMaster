package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// This file implements the AI generation path dedicated to the RAFALE
// reservoir — contracts/rafale-ai-generation.md, issue #203, v8.1.0. It is a
// SECOND, independent generation path (own endpoint, own flat schema, own
// persistence into data/files/rafale/reservoir.json) that reuses the
// existing job infrastructure wholesale (registry, batching model,
// cancellation, retries, progression) — see ai_job.go's runAIRafaleJob,
// which mirrors runAIJob's control flow without touching it (contract §1,
// plan risk R1: the Quiz path's battle-tested loop is never modified).
//
// RAFALE is deliberately NOT part of generableQuestionTypes/buildQuestionSchema
// (ai_generator.go) — contract §1.1 lists four structural incompatibilities
// (RAFALE is already a real QuestionType/manche-configuration, two different
// storage destinations, diverging form axes, Groq token budget) that make a
// shared schema/distribution wrong, not merely inconvenient.

// ============================================================================
// Length caps (contract §5.1) — a single source of truth consumed by BOTH
// the generation path (validateGeneratedRafaleQuestions below) and the
// manual editor (http.go's handlePostRafaleQuestion) — contract §5.3.
// ============================================================================

const (
	// rafaleMaxQuestionRunes caps QUESTION, calibrated on the most
	// constraining display surface (/anim's 3-line clamp) — contract §5.1.
	rafaleMaxQuestionRunes = 100
	// rafaleMaxAnswerRunes caps ANSWER, calibrated on /anim's single-line,
	// unclamped answer strip — contract §5.1.
	rafaleMaxAnswerRunes = 40
)

// rafaleRuneLen measures s in runes (utf8.RuneCountInString), after
// strings.TrimSpace — never in bytes, so an accented or multi-byte
// character never costs double (contract §5.1: "« é » ne doit pas coûter
// double"). Shared by the generation validator and the manual editor.
func rafaleRuneLen(s string) int {
	return utf8.RuneCountInString(strings.TrimSpace(s))
}

// ============================================================================
// Volume presets (contract §2bis) — count is a closed enumeration, not a
// free integer. This exact list is mirrored on the frontend (aiJobHelpers.js
// per the plan) as a Go↔JS shared constant; a preset accepted by the UI and
// rejected by the server would be an incomprehensible 400.
// ============================================================================

var rafaleGenerationPresets = []int{10, 20, 50, 100, 200}

func isRafaleGenerationPreset(n int) bool {
	for _, p := range rafaleGenerationPresets {
		if p == n {
			return true
		}
	}
	return false
}

// ============================================================================
// Request & validation (contract §2)
// ============================================================================

// generateRafaleRequest is the RAFALE counterpart of generateQuestionsRequest
// (ai_generator.go) — deliberately NOT reusing that struct: Difficulties is
// an int scale (1..3, RafaleQuestion's own scale) rather than the Quiz
// path's 4 labels, Count replaces Volume (a reservoir has no duration), and
// there is no Objectives/Distribution (contract §2 "Différences de forme
// assumées").
type generateRafaleRequest struct {
	Theme        string   `json:"theme"`
	Populations  []string `json:"populations"`
	Language     string   `json:"language"`
	Instructions string   `json:"instructions"`
	Categories   []string `json:"categories"`
	Difficulties []int    `json:"difficulties"`
	Count        int      `json:"count"`
}

// validateGenerateRafaleRequest returns "" when req is valid, or a
// human-readable message (never containing the API key) describing the
// first violation found — same idiom as validateGenerateRequest. Categories
// are checked via isKnownRafaleCategory (http.go), NOT ResolveCategoryMeta:
// contract §3, a question generated here must be validated by the exact
// same rule as the manual editor of its destination store.
func (h *HTTPServer) validateGenerateRafaleRequest(req *generateRafaleRequest, aiCfg config.AIConfig) string {
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
	if len(req.Instructions) > 2000 {
		return "instructions must be at most 2000 characters"
	}
	if len(req.Categories) == 0 {
		return "categories must contain at least one value"
	}
	seenCat := make(map[string]bool, len(req.Categories))
	for _, c := range req.Categories {
		if !h.isKnownRafaleCategory(c) {
			return fmt.Sprintf("unknown category: %q", c)
		}
		if seenCat[c] {
			return fmt.Sprintf("categories contains a duplicate value: %q", c)
		}
		seenCat[c] = true
	}
	if len(req.Difficulties) == 0 {
		return "difficulties must contain at least one value"
	}
	seenDiff := make(map[int]bool, len(req.Difficulties))
	for _, d := range req.Difficulties {
		if d < 1 || d > 3 {
			return fmt.Sprintf("difficulties contains an out-of-range value: %d", d)
		}
		if seenDiff[d] {
			return fmt.Sprintf("difficulties contains a duplicate value: %d", d)
		}
		seenDiff[d] = true
	}

	maxQuestions := aiCfg.MaxQuestions
	if maxQuestions <= 0 {
		maxQuestions = 200
	}
	if !isRafaleGenerationPreset(req.Count) {
		return fmt.Sprintf("count must be one of %v", rafaleGenerationPresets)
	}
	if req.Count > maxQuestions {
		return fmt.Sprintf("count must be at most %d", maxQuestions)
	}

	return ""
}

// ============================================================================
// Volume repartition over category × difficulty couples (contract §2 —
// "Répartition du volume")
// ============================================================================

// rafaleCouple is one category × difficulty cell of the repartition, with
// its allocated share of req.Count.
type rafaleCouple struct {
	Category   string
	Difficulty int
	Count      int
}

// allocateRafaleCounts splits count uniformly over every
// len(categories)*len(difficulties) couple, attributing the integer
// division's remainder to the first couples in request order (contract §2:
// "le reste de la division entière est attribué aux premiers couples dans
// l'ordre de la requête"). Couples that end up with a zero share (count
// smaller than the number of couples) are omitted from the result — there
// is nothing to generate for them.
func allocateRafaleCounts(categories []string, difficulties []int, count int) []rafaleCouple {
	n := len(categories) * len(difficulties)
	if n == 0 {
		return nil
	}
	per := count / n
	rest := count % n

	out := make([]rafaleCouple, 0, n)
	i := 0
	for _, cat := range categories {
		for _, diff := range difficulties {
			c := per
			if i < rest {
				c++
			}
			if c > 0 {
				out = append(out, rafaleCouple{Category: cat, Difficulty: diff, Count: c})
			}
			i++
		}
	}
	return out
}

// rafaleBatchItem is one provider call's worth of work: a single couple
// (never mixed, contract §5 "couple visé pour ce lot"), capped at the
// configured batch size — a couple needing more than one batch's worth is
// split across ceil(couple.Count / batchSize) items.
type rafaleBatchItem struct {
	Category   string
	Difficulty int
	Count      int
}

// planRafaleBatches turns the couple allocation into the flat, ordered list
// of provider calls the job loop (runAIRafaleJob, ai_job.go) executes
// sequentially.
func planRafaleBatches(couples []rafaleCouple, batchSize int) []rafaleBatchItem {
	var plan []rafaleBatchItem
	for _, couple := range couples {
		remaining := couple.Count
		for remaining > 0 {
			thisBatch := batchSize
			if thisBatch > remaining {
				thisBatch = remaining
			}
			plan = append(plan, rafaleBatchItem{Category: couple.Category, Difficulty: couple.Difficulty, Count: thisBatch})
			remaining -= thisBatch
		}
	}
	return plan
}

// ============================================================================
// LLM output schema (contract §4) — flat, no anyOf, no ID: the model is
// structurally incapable of designating (and so overwriting) an existing
// reservoir entry. minLength/maxLength are NOT used (structured outputs
// don't support them, same limitation as the Quiz schema) — the length
// contract is enforced by the prompt (buildRafaleGenerationPrompt) and the
// server-side validator (validateGeneratedRafaleQuestions) alone.
// ============================================================================

func buildRafaleQuestionSchema(categories []string, difficulties []int) map[string]any {
	catEnum := make([]any, len(categories))
	for i, c := range categories {
		catEnum[i] = c
	}
	diffEnum := make([]any, len(difficulties))
	for i, d := range difficulties {
		diffEnum[i] = d
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"questions"},
		"properties": map[string]any{
			"questions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"QUESTION", "ANSWER", "CATEGORY", "DIFFICULTY"},
					"properties": map[string]any{
						"QUESTION":   map[string]any{"type": "string"},
						"ANSWER":     map[string]any{"type": "string"},
						"CATEGORY":   map[string]any{"type": "string", "enum": catEnum},
						"DIFFICULTY": map[string]any{"type": "integer", "enum": diffEnum},
					},
				},
			},
		},
	}
}

// llmRawRafaleQuestion is the RAFALE counterpart of llmRawQuestion — flat,
// no TYPE (a single implicit "type" for this whole path), no ID.
type llmRawRafaleQuestion struct {
	Question   string `json:"QUESTION"`
	Answer     string `json:"ANSWER"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
}

type llmRafaleResponseRoot struct {
	Questions []llmRawRafaleQuestion `json:"questions"`
}

// parseLLMRafaleResponse mirrors parseLLMResponse for the flat RAFALE shape.
func parseLLMRafaleResponse(rawJSON string) ([]llmRawRafaleQuestion, error) {
	var root llmRafaleResponseRoot
	if err := json.Unmarshal([]byte(rawJSON), &root); err != nil {
		return nil, fmt.Errorf("AI response is not valid JSON matching the expected schema: %w", err)
	}
	return root.Questions, nil
}

// ============================================================================
// Anti-duplicate context (contract §5, prompt §5) — sourced from the
// RESERVOIR (not files/questions/ like the Quiz path), for the couples
// targeted by the whole request, capped at 150 entries / 200 characters —
// same cap as the Quiz path's collectExistingQuestionsContext.
// ============================================================================

// collectExistingRafaleContext returns the statement text of every reservoir
// question already matching one of the requested category × difficulty
// couples, capped at 150 entries. Text-only (no CATEGORY/TYPE needed here,
// unlike the Quiz path's context, since this whole generation targets one
// implicit "type").
func (h *HTTPServer) collectExistingRafaleContext(categories []string, difficulties []int) []string {
	allowedCat := make(map[string]bool, len(categories))
	for _, c := range categories {
		allowedCat[c] = true
	}
	allowedDiff := make(map[int]bool, len(difficulties))
	for _, d := range difficulties {
		allowedDiff[d] = true
	}

	questions, _ := h.engine.SnapshotRafaleReservoir()
	out := make([]string, 0, len(questions))
	for _, q := range questions {
		if !allowedCat[string(q.Category)] || !allowedDiff[q.Difficulty] {
			continue
		}
		text := q.Question
		if len(text) > 200 {
			text = text[:200]
		}
		out = append(out, text)
	}
	if len(out) > 150 {
		out = out[:150]
	}
	return out
}

// normalizeRafaleQuestionText is the doublon comparison key (contract §5.4:
// "insensible à la casse et aux espaces") — lowercased, internal whitespace
// runs collapsed to one space, leading/trailing trimmed.
func normalizeRafaleQuestionText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// ============================================================================
// Prompt (contract §5) — normative order: Thème → Publics → Langue → couple
// visé → Précisions → consigne de brièveté → contexte anti-doublon.
// ============================================================================

// rafaleStarsLabel renders a 1..3 difficulty as the star notation the admin
// sees in the UI, so the prompt speaks the same vocabulary as the modale.
func rafaleStarsLabel(n int) string {
	switch n {
	case 1:
		return "★☆☆ (facile)"
	case 2:
		return "★★☆ (moyen)"
	case 3:
		return "★★★ (difficile)"
	default:
		return fmt.Sprintf("%d", n)
	}
}

// buildRafaleGenerationPrompt assembles the instruction for ONE batch —
// always targeting exactly one category × difficulty couple (contract §5
// "couple visé pour ce lot"). extraContext is the anti-duplicate list,
// already capped by the caller (job-accumulated questions first, then
// on-disk reservoir, contract §5 "plafond 150 entrées / 200 caractères").
func (h *HTTPServer) buildRafaleGenerationPrompt(req generateRafaleRequest, batchCount int, targetCategory string, targetDifficulty int, extraContext []string) string {
	var b strings.Builder

	b.WriteString("Tu es un générateur de questions pour une manche RAFALE — un mode de jeu buzzer très rapide où chaque question dispose d'environ 3 secondes de temps de réponse. ")
	b.WriteString("Génère des questions strictement conformes au schéma JSON fourni — n'ajoute aucun champ hors schéma.\n\n")

	fmt.Fprintf(&b, "Thème : %s\n", req.Theme)
	fmt.Fprintf(&b, "Publics cibles : %s\n", strings.Join(req.Populations, ", "))
	fmt.Fprintf(&b, "Langue de rédaction : %s\n", req.Language)
	fmt.Fprintf(&b, "Couple visé pour cet appel : catégorie %s, difficulté %s\n", targetCategory, rafaleStarsLabel(targetDifficulty))
	if strings.TrimSpace(req.Instructions) != "" {
		fmt.Fprintf(&b, "Précisions pour cette génération : %s\n", req.Instructions)
	}
	fmt.Fprintf(&b, "Nombre de questions à générer dans cet appel : %d\n", batchCount)

	fmt.Fprintf(&b, "\n🔴 Contrainte de brièveté impérative — une manche RAFALE ne laisse qu'environ 3 secondes par question :\n")
	fmt.Fprintf(&b, "- QUESTION : au plus 80 caractères, une seule phrase interrogative courte.\n")
	fmt.Fprintf(&b, "- ANSWER : au plus 25 caractères, idéalement 1 à 3 mots.\n")
	b.WriteString("- Exemple correct : {\"QUESTION\": \"Quelle planète est la plus proche du Soleil ?\", \"ANSWER\": \"Mercure\"}\n")
	b.WriteString("- Contre-exemple à éviter (bien trop long pour 3 secondes de réponse) : {\"QUESTION\": \"Dans le système solaire, quelle est la planète la plus proche de notre étoile en termes de distance orbitale moyenne ?\", \"ANSWER\": \"Il s'agit de la planète Mercure\"}\n")

	if len(extraContext) > 0 {
		b.WriteString("\nQuestions déjà présentes dans le réservoir pour ces catégories/difficultés, y compris celles déjà produites plus tôt dans cette génération — NE LES DUPLIQUE PAS :\n")
		if data, err := json.Marshal(extraContext); err == nil {
			b.Write(data)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ============================================================================
// Server-side validation (contract §5.2/§5.4) — rejects, NEVER truncates.
// ============================================================================

// validateGeneratedRafaleQuestions filters raw to only the questions passing
// every rule of contract §5.4, and caps the result at maxQuestions (excess
// dropped — "au-delà de count, l'excédent est écarté"; the caller passes the
// CURRENT BATCH's own target count, since each batch is already sized so
// the couples' sums equal the job's total requested count). existingNormalized
// is the doublon lookup set (contract §5.4), pre-normalized by the caller via
// normalizeRafaleQuestionText — it is NOT mutated here; duplicates WITHIN
// raw itself are caught by a separate, batch-local set.
func validateGeneratedRafaleQuestions(raw []llmRawRafaleQuestion, allowedCategories []string, allowedDifficulties []int, maxQuestions int, existingNormalized map[string]bool) (valid []llmRawRafaleQuestion, reasons []string) {
	allowedCat := make(map[string]bool, len(allowedCategories))
	for _, c := range allowedCategories {
		allowedCat[c] = true
	}
	allowedDiff := make(map[int]bool, len(allowedDifficulties))
	for _, d := range allowedDifficulties {
		allowedDiff[d] = true
	}

	seenThisBatch := make(map[string]bool, len(raw))

	for _, q := range raw {
		question := strings.TrimSpace(q.Question)
		answer := strings.TrimSpace(q.Answer)

		if question == "" {
			reasons = append(reasons, "champ vide (énoncé)")
			continue
		}
		if answer == "" {
			reasons = append(reasons, "champ vide (réponse)")
			continue
		}
		if rafaleRuneLen(question) > rafaleMaxQuestionRunes {
			reasons = append(reasons, "énoncé trop long")
			continue
		}
		if rafaleRuneLen(answer) > rafaleMaxAnswerRunes {
			reasons = append(reasons, "réponse trop longue")
			continue
		}
		if !allowedCat[q.Category] {
			reasons = append(reasons, fmt.Sprintf("catégorie hors filtre (%q)", q.Category))
			continue
		}
		if !allowedDiff[q.Difficulty] {
			reasons = append(reasons, fmt.Sprintf("difficulté hors filtre (%d)", q.Difficulty))
			continue
		}

		norm := normalizeRafaleQuestionText(question)
		if existingNormalized[norm] || seenThisBatch[norm] {
			reasons = append(reasons, "doublon")
			continue
		}

		if maxQuestions > 0 && len(valid) >= maxQuestions {
			reasons = append(reasons, "plafond de volume atteint")
			continue
		}

		seenThisBatch[norm] = true
		q.Question = question
		q.Answer = answer
		valid = append(valid, q)
	}

	return valid, reasons
}

// ============================================================================
// HTTP handler — POST /api/rafale/generate-questions (contract §2)
// ============================================================================

// handleGenerateRafaleQuestions starts a RAFALE reservoir generation job.
// Mirrors handleGenerateQuestions (ai_generator.go) almost exactly, plus one
// extra guard: refusing to start while a RAFALE round is currently being
// played (contract §7, `rafale_round_in_progress`) — a check with no Quiz
// equivalent, since the Quiz generation path has no notion of "a round in
// progress" to protect against.
func (h *HTTPServer) handleGenerateRafaleQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Hard size limit before decoding — same idiom/bound as
	// handleGenerateQuestions (contract ai-generation.md §8 S2 / this
	// contract §10).
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req generateRafaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGenerateError(w, http.StatusBadRequest, "invalid_request", "Corps de requête JSON invalide.", 0)
		return
	}

	aiCfg := config.Get().AI

	if msg := h.validateGenerateRafaleRequest(&req, aiCfg); msg != "" {
		writeGenerateError(w, http.StatusBadRequest, "invalid_request", msg, 0)
		return
	}

	if !providerAPIKeyConfigured(aiCfg) {
		message := "Aucune clé API Claude configurée."
		if aiCfg.Provider == "groq" {
			message = "Aucune clé API Groq configurée."
		}
		writeGenerateError(w, http.StatusConflict, "no_api_key", message, 0)
		return
	}

	// Contract §7: refuse while a RAFALE round is currently playing — the
	// current question is of type RAFALE AND the phase is STARTED or
	// PAUSED. Checked AFTER validation/key so a malformed request or a
	// missing key still gets its own, more specific error first.
	state := h.engine.GetState()
	if state.Question != nil && state.Question.Type == game.QuestionTypeRafale &&
		(state.Phase == game.PhaseStarted || state.Phase == game.PhasePaused) {
		writeGenerateError(w, http.StatusConflict, "rafale_round_in_progress", "Une manche RAFALE est en cours — la génération reprendra une fois la manche terminée.", 0)
		return
	}

	jobID, batchesTotal, started := h.startAIRafaleGenerationJob(req, aiCfg)
	if !started {
		// contract §1.2: a single job at a time, Quiz or RAFALE combined —
		// same atomic check-and-start as the Quiz path (aiJobRegistry.tryStart).
		writeGenerateError(w, http.StatusConflict, "generation_in_progress", "Une génération est déjà en cours.", 0)
		return
	}

	LogInfo(game.LogComponentHTTP, "AI RAFALE generation job %s started: provider=%s batches_total=%d",
		jobID, aiCfg.Provider, batchesTotal)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(generateAcceptedBody{
		Status:       "accepted",
		JobID:        jobID,
		BatchesTotal: batchesTotal,
	})
}

// ============================================================================
// Job execution (contract §1.2, §8) — the RAFALE counterpart of ai_job.go's
// startAIGenerationJob/runAIJob. Deliberately a SEPARATE loop rather than a
// generalization of runAIJob: RAFALE batches are couple-targeted (one
// category × difficulty per call) while Quiz batches are type-mixed
// (distribution-driven) — genuinely different units of work, not the same
// loop with a swapped-out persistence step. Restructuring runAIJob to
// accommodate both would have meant touching its battle-tested control flow
// (contract's own risk R1: "ne pas la réécrire ; n'extraire que le bloc de
// persistance"). What IS shared, verbatim, unmodified: aiJobRegistry
// (tryStart/current, "un seul job à la fois tous chemins confondus"), the
// aiJob struct and its cancellation/snapshot machinery, broadcastAIJobProgress,
// classifyJobErrorCode, backoffDelay, interruptibleSleep, clampBatchSize,
// timeoutOrDefault, selectProvider, and every provider/error type. Only the
// per-batch "build prompt/schema → call provider → parse → validate →
// persist" sequence differs, because the shape of a batch differs.
// ============================================================================

// startAIRafaleGenerationJob is startAIGenerationJob's RAFALE counterpart.
// batchesTotal is derived from the couple allocation (§2 "Répartition du
// volume"), not a flat total/batchSize division: each RAFALE batch targets
// exactly ONE couple (§5 "couple visé pour ce lot"), so a couple needing
// more than one batch's worth of questions is itself split across
// ceil(couple.Count / batchSize) calls (planRafaleBatches).
func (h *HTTPServer) startAIRafaleGenerationJob(req generateRafaleRequest, aiCfg config.AIConfig) (jobID string, batchesTotal int, ok bool) {
	couples := allocateRafaleCounts(req.Categories, req.Difficulties, req.Count)
	batchSize := clampBatchSize(aiCfg.BatchSize)
	plan := planRafaleBatches(couples, batchSize)
	batchesTotal = len(plan)
	if batchesTotal < 1 {
		batchesTotal = 1
	}

	job, started := globalAIJobRegistry.tryStart(aiCfg.Provider, batchesTotal, "RAFALE")
	if !started {
		return "", 0, false
	}

	go h.runAIRafaleJob(job, req, aiCfg, plan)

	return job.ID, batchesTotal, true
}

// runAIRafaleJob is runAIJob's RAFALE counterpart — same cancellation
// (checked only BETWEEN batches), same retry-same-batch-on-rate-limit
// behavior, same max_consecutive_failures termination rule, same
// inter-batch delay, applied to the fixed couple-targeted plan instead of a
// flat batch count. Two things the RAFALE path never does, unlike Quiz:
// emit "id_exhausted" (r-NNN identifiers aren't bounded to 999 by directory
// scanning — AppendRafaleQuestions/nextRafaleIDUnsafe simply keep counting)
// and call h.OnQuestionUpload() (that hook is specific to files/questions/;
// the reservoir has no equivalent broadcast, contract §6 "Rafraîchissement
// de la liste du réservoir" — the frontend refetches on its own instead).
func (h *HTTPServer) runAIRafaleJob(job *aiJob, req generateRafaleRequest, aiCfg config.AIConfig, plan []rafaleBatchItem) {
	provider := h.selectProvider(aiCfg)
	maxFailures := aiCfg.MaxConsecutiveFailures
	if maxFailures <= 0 {
		maxFailures = 2
	}
	interBatchDelay := time.Duration(aiCfg.InterBatchDelayMs) * time.Millisecond
	if interBatchDelay <= 0 {
		interBatchDelay = 60 * time.Second
	}

	// The schema's CATEGORY/DIFFICULTY enums cover the WHOLE request (contract
	// §4), not just the couple targeted by the current batch — the prompt
	// alone steers the model toward the couple; the schema and the validator
	// both accept anything within the request's full allowed sets. It is
	// therefore built once, outside the loop, unlike the Quiz path's schema
	// (which varies batch to batch only in which TYPES are active — not
	// applicable here, RAFALE has no per-batch type mixing).
	schema := provider.AdaptSchema(buildRafaleQuestionSchema(req.Categories, req.Difficulties))

	var jobExtraContext []string // job-accumulated statements, freshest last — anti-duplicate context (contract §5)
	consecutiveFailures := 0
	var lastErr error

	finalState := "DONE"
	finalErrorCode := ""

batchLoop:
	for batchIndex := 0; batchIndex < len(plan); {
		if job.isCancelRequested() {
			finalState = "CANCELLED"
			finalErrorCode = ""
			break batchLoop
		}

		item := plan[batchIndex]

		existingContext := h.collectExistingRafaleContext(req.Categories, req.Difficulties)
		combined := make([]string, 0, len(jobExtraContext)+len(existingContext))
		combined = append(combined, jobExtraContext...)
		combined = append(combined, existingContext...)
		if len(combined) > 150 {
			combined = combined[:150]
		}

		prompt := h.buildRafaleGenerationPrompt(req, item.Count, item.Category, item.Difficulty, combined)

		callCtx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(aiCfg.TimeoutSeconds))
		rawJSON, err := provider.Generate(callCtx, aiCfg, prompt, schema)
		cancel()

		if err != nil {
			lastErr = err
			consecutiveFailures++
			LogWarn(game.LogComponentHTTP, "AI RAFALE generation job %s: batch %d/%d failed (%d/%d consecutive failures): %v",
				job.ID, batchIndex+1, len(plan), consecutiveFailures, maxFailures, err)

			if consecutiveFailures >= maxFailures {
				finalState = "FAILED"
				finalErrorCode = classifyJobErrorCode(lastErr)
				break batchLoop
			}

			var rateLimitErr *aiRateLimitError
			if errors.As(err, &rateLimitErr) {
				wait := rateLimitErr.retryAfter
				if wait <= 0 {
					wait = backoffDelay(consecutiveFailures)
				}
				if cancelled := interruptibleSleep(job.cancelRequested, wait); cancelled {
					finalState = "CANCELLED"
					finalErrorCode = ""
					break batchLoop
				}
				continue // retry the SAME batchIndex (contract ai-multi-provider.md §3)
			}

			// Any other failure: this batch contributes 0 questions, move on
			// to the next one — a single bad batch must not stall the whole
			// run, same rule as the Quiz path.
			batchIndex++
			continue
		}

		llmQuestions, parseErr := parseLLMRafaleResponse(rawJSON)
		if parseErr != nil {
			lastErr = parseErr
			consecutiveFailures++
			LogWarn(game.LogComponentHTTP, "AI RAFALE generation job %s: batch %d/%d response schema mismatch (%d/%d consecutive failures): %v",
				job.ID, batchIndex+1, len(plan), consecutiveFailures, maxFailures, parseErr)
			if consecutiveFailures >= maxFailures {
				finalState = "FAILED"
				finalErrorCode = classifyJobErrorCode(lastErr)
				break batchLoop
			}
			batchIndex++
			continue
		}
		consecutiveFailures = 0

		existingNormalized := make(map[string]bool, len(combined))
		for _, txt := range combined {
			existingNormalized[normalizeRafaleQuestionText(txt)] = true
		}

		validQuestions, skipped := validateGeneratedRafaleQuestions(llmQuestions, req.Categories, req.Difficulties, item.Count, existingNormalized)

		toWrite := make([]game.RafaleQuestion, 0, len(validQuestions))
		for _, vq := range validQuestions {
			toWrite = append(toWrite, game.RafaleQuestion{
				Question:   vq.Question,
				Answer:     vq.Answer,
				Category:   game.QuestionCategory(vq.Category),
				Difficulty: vq.Difficulty,
			})
		}

		var stored []game.RafaleQuestion
		var writeErr error
		if len(toWrite) > 0 {
			// Single locked batch write, single SaveRafale() — contract §8.
			stored, writeErr = h.engine.AppendRafaleQuestions(toWrite)
		}

		job.mu.Lock()
		job.CreatedCount += len(stored)
		job.SkippedCount += len(skipped)
		job.mu.Unlock()

		if writeErr != nil {
			finalState = "FAILED"
			finalErrorCode = "internal_error"
			LogError(game.LogComponentHTTP, "AI RAFALE generation job %s: failed to persist batch: %v", job.ID, writeErr)
			h.broadcastAIJobProgress(job, "FAILED", finalErrorCode, sanitizeUpstreamMessage(writeErr.Error()))
			return
		}

		for _, s := range stored {
			jobExtraContext = append(jobExtraContext, s.Question)
		}
		if len(jobExtraContext) > 150 {
			jobExtraContext = jobExtraContext[len(jobExtraContext)-150:]
		}

		job.mu.Lock()
		job.BatchesDone = batchIndex + 1
		job.mu.Unlock()

		// Broadcast AFTER persistence, same rule as the Quiz path (contract
		// §6: "Émis après la persistance du lot, pour que le réservoir soit
		// déjà à jour côté client") — RafalePage's own refetch (triggered by
		// this same progress message client-side) picks up the new rows.
		h.broadcastAIJobProgress(job, "RUNNING", "", "")

		batchIndex++
		if batchIndex < len(plan) {
			if cancelled := interruptibleSleep(job.cancelRequested, interBatchDelay); cancelled {
				finalState = "CANCELLED"
				finalErrorCode = ""
				break batchLoop
			}
		}
	}

	// Same "never a false DONE" guard as runAIJob: zero questions created AND
	// a recorded generation-level error means FAILED, not an empty success.
	if finalState == "DONE" {
		job.mu.Lock()
		created := job.CreatedCount
		job.mu.Unlock()
		if created == 0 && lastErr != nil {
			finalState = "FAILED"
			finalErrorCode = classifyJobErrorCode(lastErr)
		}
	}

	finalErrorMessage := ""
	if finalState == "FAILED" && lastErr != nil {
		finalErrorMessage = sanitizeUpstreamMessage(lastErr.Error())
	}
	h.broadcastAIJobProgress(job, finalState, finalErrorCode, finalErrorMessage)
}
