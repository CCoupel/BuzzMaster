package server

import (
	"fmt"
	"strings"

	"buzzcontrol/internal/game"
)

// Pool generation descriptors (v13.1.0, issue fille #324, contracts/
// picto-pool.md §5). The RAFALE generation path (ai_generator_rafale.go:
// request validation, couple repartition, batches, retries, progress,
// cancellation, single job) is shared; a descriptor supplies what differs per
// pool: volume presets, JSON schema, prompt, validator, anti-duplicate
// context, persistence, round guard and the job TARGET. The `rafale`
// descriptor leaves prompt, schema, answers and presets byte-identical to the
// pre-v13.1.0 path.
type poolGenDescriptor struct {
	key       string // "rafale" | "picto"
	target    string // AI_GENERATION_PROGRESS TARGET: "RAFALE" | "PICTO"
	presets   []int  // closed volume enumeration, mirrored Go ↔ JS
	guard     func(st game.GameState) (code, message string, blocked bool)
	schema    func(categories []string, difficulties []int) map[string]any
	prompt    func(h *HTTPServer, req generateRafaleRequest, batchCount int, cat string, diff int, extraContext []string) string
	validate  func(raw []llmRawRafaleQuestion, cats []string, diffs []int, max int, existing map[string]bool) (valid []llmRawRafaleQuestion, reasons []string)
	context   func(h *HTTPServer, categories []string, difficulties []int) []string
	normalize func(s string) string
	// persist writes the valid entries (one save), returns the stored texts
	// (anti-duplicate context for the next batches) and how many entries the
	// store itself still refused (always 0 for RAFALE).
	persist func(h *HTTPServer, valid []llmRawRafaleQuestion) (stored []string, refused int, err error)
}

// pictoGenerationPresets are the Pool Picto volume presets (contracts/
// picto-pool.md §5) — filtered by ai.max_questions like any other count.
var pictoGenerationPresets = []int{10, 20, 50}

func rafaleGenDescriptor() *poolGenDescriptor {
	return &poolGenDescriptor{
		key:     "rafale",
		target:  "RAFALE",
		presets: rafaleGenerationPresets,
		guard: func(st game.GameState) (string, string, bool) {
			// Contract §7: refuse while a RAFALE round is currently playing —
			// the current question is of type RAFALE AND the phase is STARTED
			// or PAUSED.
			if st.Question != nil && st.Question.Type == game.QuestionTypeRafale &&
				(st.Phase == game.PhaseStarted || st.Phase == game.PhasePaused) {
				return "rafale_round_in_progress", "Une manche RAFALE est en cours — la génération reprendra une fois la manche terminée.", true
			}
			return "", "", false
		},
		schema: buildRafaleQuestionSchema,
		prompt: func(h *HTTPServer, req generateRafaleRequest, n int, cat string, diff int, ctx []string) string {
			return h.buildRafaleGenerationPrompt(req, n, cat, diff, ctx)
		},
		validate: validateGeneratedRafaleQuestions,
		context: func(h *HTTPServer, categories []string, difficulties []int) []string {
			return h.collectExistingRafaleContext(categories, difficulties)
		},
		normalize: normalizeRafaleQuestionText,
		persist: func(h *HTTPServer, valid []llmRawRafaleQuestion) ([]string, int, error) {
			toWrite := make([]game.RafaleQuestion, 0, len(valid))
			for _, vq := range valid {
				toWrite = append(toWrite, game.RafaleQuestion{
					Question:   vq.Question,
					Answer:     vq.Answer,
					Category:   game.QuestionCategory(vq.Category),
					Difficulty: vq.Difficulty,
				})
			}
			if len(toWrite) == 0 {
				return nil, 0, nil
			}
			// Single locked batch write, single SaveRafale() — contract §8.
			stored, err := h.engine.AppendRafaleQuestions(toWrite)
			if err != nil {
				return nil, 0, err
			}
			texts := make([]string, 0, len(stored))
			for _, s := range stored {
				texts = append(texts, s.Question)
			}
			return texts, 0, nil
		},
	}
}

func pictoGenDescriptor() *poolGenDescriptor {
	return &poolGenDescriptor{
		key:       "picto",
		target:    "PICTO",
		presets:   pictoGenerationPresets,
		guard:     poolGuardPicto,
		schema:    buildPictoWordSchema,
		prompt:    (*HTTPServer).buildPictoGenerationPrompt,
		validate:  validateGeneratedPictoWords,
		context:   (*HTTPServer).collectExistingPictoContext,
		normalize: game.PictoWordKey,
		persist: func(h *HTTPServer, valid []llmRawRafaleQuestion) ([]string, int, error) {
			// One AddWords call per (category, difficulty) group: the store
			// applies the word rules and the per-category duplicate rule
			// (authoritative) and saves once per group.
			type couple struct {
				cat  string
				diff int
			}
			groups := map[couple][]string{}
			var order []couple
			for _, vq := range valid {
				c := couple{vq.Category, vq.Difficulty}
				if _, ok := groups[c]; !ok {
					order = append(order, c)
				}
				groups[c] = append(groups[c], vq.Question)
			}
			var texts []string
			refused := 0
			for _, c := range order {
				created, ref, err := h.engine.PictoPool().AddWords(game.PictoWordFieldSet, groups[c], game.QuestionCategory(c.cat), c.diff)
				if err != nil {
					return texts, refused, err
				}
				refused += len(ref)
				for _, e := range created {
					texts = append(texts, e.Question)
				}
			}
			return texts, refused, nil
		},
	}
}

// buildPictoWordSchema is the flat Pool Picto schema: no ANSWER, no ID, no
// anyOf (same reasons as buildRafaleQuestionSchema, contract rafale-ai
// generation §4).
func buildPictoWordSchema(categories []string, difficulties []int) map[string]any {
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
					"required":             []string{"QUESTION", "CATEGORY", "DIFFICULTY"},
					"properties": map[string]any{
						"QUESTION":   map[string]any{"type": "string"},
						"CATEGORY":   map[string]any{"type": "string", "enum": catEnum},
						"DIFFICULTY": map[string]any{"type": "integer", "enum": diffEnum},
					},
				},
			},
		},
	}
}

// collectExistingPictoContext returns the words of the Pool Picto already
// present for the requested couples, capped at 150 entries / 200 characters
// (same caps as the RAFALE context).
func (h *HTTPServer) collectExistingPictoContext(categories []string, difficulties []int) []string {
	allowedCat := make(map[string]bool, len(categories))
	for _, c := range categories {
		allowedCat[c] = true
	}
	allowedDiff := make(map[int]bool, len(difficulties))
	for _, d := range difficulties {
		allowedDiff[d] = true
	}
	words, _ := h.engine.PictoPool().Snapshot()
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !allowedCat[string(w.Category)] || !allowedDiff[w.Difficulty] {
			continue
		}
		text := w.Question
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

// buildPictoGenerationPrompt assembles the instruction for ONE batch (one
// category × difficulty couple): words or short expressions that can be DRAWN.
func (h *HTTPServer) buildPictoGenerationPrompt(req generateRafaleRequest, batchCount int, targetCategory string, targetDifficulty int, extraContext []string) string {
	var b strings.Builder

	b.WriteString("Tu es un générateur de mots pour un jeu de dessin : un joueur dessine un mot et ses coéquipiers doivent le deviner. ")
	b.WriteString("Génère des mots strictement conformes au schéma JSON fourni — n'ajoute aucun champ hors schéma.\n\n")

	fmt.Fprintf(&b, "Thème : %s\n", req.Theme)
	fmt.Fprintf(&b, "Publics cibles : %s\n", strings.Join(req.Populations, ", "))
	fmt.Fprintf(&b, "Langue de rédaction : %s\n", req.Language)
	fmt.Fprintf(&b, "Couple visé pour cet appel : catégorie %s, difficulté %s\n", targetCategory, rafaleStarsLabel(targetDifficulty))
	if strings.TrimSpace(req.Instructions) != "" {
		fmt.Fprintf(&b, "Précisions pour cette génération : %s\n", req.Instructions)
	}
	fmt.Fprintf(&b, "Nombre de mots à générer dans cet appel : %d\n", batchCount)

	b.WriteString("\n🔴 Contraintes impératives :\n")
	b.WriteString("- QUESTION : un mot ou une expression courte (au plus 3 mots, 40 caractères maximum) qui se DESSINE : un objet, un animal, un lieu, une action ou une scène reconnaissable.\n")
	b.WriteString("- Uniquement des lettres (accents admis), chiffres, espaces, apostrophes et tirets : aucune ponctuation ni autre symbole.\n")
	b.WriteString("- Ni nom propre, ni marque, ni mot abstrait impossible à représenter par un dessin.\n")
	b.WriteString("- Aucun doublon, aucune variante d'un même mot.\n")

	if len(extraContext) > 0 {
		b.WriteString("\nMots déjà présents dans le pool pour ces catégories/difficultés, y compris ceux déjà produits plus tôt dans cette génération — NE LES DUPLIQUE PAS :\n")
		b.WriteString(strings.Join(extraContext, ", "))
		b.WriteString("\n")
	}

	return b.String()
}

// validateGeneratedPictoWords keeps only the words passing the Pool Picto
// rules (the editor's own: game.NormalizePictoWord) inside the requested
// category/difficulty sets, without duplicate (folded key, per category) within the
// batch; duplicates against the pool are refused by the store. Invalid or duplicate words
// are dropped, NEVER truncated. maxQuestions caps the batch.
func validateGeneratedPictoWords(raw []llmRawRafaleQuestion, allowedCategories []string, allowedDifficulties []int, maxQuestions int, existingNormalized map[string]bool) (valid []llmRawRafaleQuestion, reasons []string) {
	allowedCat := make(map[string]bool, len(allowedCategories))
	for _, c := range allowedCategories {
		allowedCat[c] = true
	}
	allowedDiff := make(map[int]bool, len(allowedDifficulties))
	for _, d := range allowedDifficulties {
		allowedDiff[d] = true
	}
	seen := make(map[string]bool, len(raw))

	for _, q := range raw {
		word, err := game.NormalizePictoWord(q.Question)
		if err != nil {
			reasons = append(reasons, "mot invalide")
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
		key := game.PictoWordKey(word)
		catKey := q.Category + "|" + key
		// Duplicates are per CATEGORY: the store (AddWords) is authoritative
		// against existing words; existingNormalized (flat context) is
		// deliberately not used here.
		if seen[catKey] {
			reasons = append(reasons, "doublon")
			continue
		}
		if maxQuestions > 0 && len(valid) >= maxQuestions {
			reasons = append(reasons, "plafond de volume atteint")
			continue
		}
		seen[catKey] = true
		q.Question = word
		q.Answer = ""
		valid = append(valid, q)
	}
	return valid, reasons
}
