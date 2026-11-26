package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : descripteur de génération IA `picto` (contracts/picto-pool.md §5).
// Prompt « mot dessinable », schéma plat sans ANSWER, validateur = règles de l'éditeur (écarté, jamais tronqué),
// TARGET = PICTO, paliers 10/20/50, job unique (déjà couvert croisé RAFALE ↔ Picto dans pool_picto_324_test.go).

func TestPictoAI_Descriptor_KeyTargetPresets(t *testing.T) {
	d := pictoGenDescriptor()
	if d.key != "picto" || d.target != "PICTO" {
		t.Errorf("descripteur picto : key=%q target=%q", d.key, d.target)
	}
	if len(d.presets) != 3 || d.presets[0] != 10 || d.presets[1] != 20 || d.presets[2] != 50 {
		t.Errorf("paliers = %v, attendu [10 20 50]", d.presets)
	}
	r := rafaleGenDescriptor()
	if r.key != "rafale" || r.target != "RAFALE" {
		t.Errorf("descripteur rafale : key=%q target=%q", r.key, r.target)
	}
	if len(r.presets) != 5 {
		t.Errorf("paliers rafale = %v, attendu [10 20 50 100 200] inchangés", r.presets)
	}
}

func TestPictoAI_Prompt_AsksForDrawableWords_NoAnswerField_ContextForwarded(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	req := generateRafaleRequest{Theme: "Animaux", Populations: []string{"enfants"}, Language: "français"}
	p := s.buildPictoGenerationPrompt(req, 10, "SCIENCE", 2, []string{"chat", "chien"})
	for _, must := range []string{"DESSINE", "au plus 3 mots", "40 caractères", "Ni nom propre, ni marque", "Thème : Animaux", "catégorie SCIENCE", "Nombre de mots à générer dans cet appel : 10", "chat, chien", "NE LES DUPLIQUE PAS"} {
		if !strings.Contains(p, must) {
			t.Errorf("prompt Picto sans %q :\n%s", must, p)
		}
	}
	for _, mustNot := range []string{"ANSWER", "RAFALE", "3 secondes"} {
		if strings.Contains(p, mustNot) {
			t.Errorf("le prompt Picto ne doit pas contenir %q (prompt RAFALE) :\n%s", mustNot, p)
		}
	}
}

func TestPictoAI_Schema_Flat_NoAnswer_EnumsFromRequest(t *testing.T) {
	b, _ := json.Marshal(buildPictoWordSchema([]string{"HISTORY", "SCIENCE"}, []int{1, 3}))
	s := string(b)
	if strings.Contains(s, "ANSWER") || strings.Contains(s, "anyOf") || strings.Contains(s, "oneOf") {
		t.Errorf("schéma plat sans ANSWER ni discriminant : %s", s)
	}
	for _, must := range []string{`"QUESTION"`, `"CATEGORY"`, `"DIFFICULTY"`, `"HISTORY"`, `"SCIENCE"`, `"additionalProperties":false`} {
		if !strings.Contains(s, must) {
			t.Errorf("schéma sans %s : %s", must, s)
		}
	}
}

func TestPictoAI_Validator_DropsInvalidAndDuplicates_NeverTruncates_Caps(t *testing.T) {
	raw := []llmRawRafaleQuestion{
		{Question: "chat", Category: "HISTORY", Difficulty: 1},
		{Question: "  CHAT ", Category: "HISTORY", Difficulty: 1},               // doublon replié, même catégorie
		{Question: "chat", Category: "SCIENCE", Difficulty: 1},                  // autre catégorie : accepté
		{Question: "chat<3", Category: "HISTORY", Difficulty: 1},                // caractère interdit
		{Question: strings.Repeat("a", 41), Category: "HISTORY", Difficulty: 1}, // trop long : écarté, pas tronqué
		{Question: "lion", Category: "SPORTS", Difficulty: 1},                   // catégorie hors filtre
		{Question: "tigre", Category: "HISTORY", Difficulty: 3},                 // difficulté hors filtre
		{Question: "ours", Category: "HISTORY", Difficulty: 2, Answer: "x"},     // ANSWER effacé
		{Question: "zèbre", Category: "HISTORY", Difficulty: 1},                 // plafond atteint
	}
	valid, reasons := validateGeneratedPictoWords(raw, []string{"HISTORY", "SCIENCE"}, []int{1, 2}, 3, nil)
	if len(valid) != 3 {
		t.Fatalf("valides = %d (%v), attendu 3 (plafond du lot)", len(valid), valid)
	}
	if valid[0].Question != "chat" || valid[1].Category != "SCIENCE" || valid[2].Question != "ours" || valid[2].Answer != "" {
		t.Errorf("valides = %+v", valid)
	}
	if len(reasons) != len(raw)-3 {
		t.Errorf("raisons = %d, attendu %d (une par mot écarté)", len(reasons), len(raw)-3)
	}
	for _, v := range valid {
		if len([]rune(v.Question)) > 40 {
			t.Errorf("mot tronqué accepté : %q", v.Question)
		}
	}
}

func TestPictoAI_FullCycle_PoolGrows_OnlyValidWords_ProgressTargetPicto(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		{"QUESTION": "éléphant", "CATEGORY": "HISTORY", "DIFFICULTY": 1},
		{"QUESTION": "pont-levis", "CATEGORY": "HISTORY", "DIFFICULTY": 1},
		{"QUESTION": "mot<invalide", "CATEGORY": "HISTORY", "DIFFICULTY": 1},
		{"QUESTION": "ÉLÉPHANT", "CATEGORY": "HISTORY", "DIFFICULTY": 1},
	})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, s)

	b, _ := json.Marshal(map[string]any{"theme": "Animaux", "populations": []string{"Adulte (18-64 ans)"}, "language": "Français",
		"categories": []string{"HISTORY"}, "difficulties": []int{1}, "count": 10})
	expectAccepted(t, nrDo(t, s, "POST", "/api/picto/generate-words", string(b)))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("STATE = %v (%v)", final["STATE"], final)
	}
	if final["TARGET"] != "PICTO" {
		t.Errorf("TARGET = %v, attendu PICTO", final["TARGET"])
	}
	qs, used := s.engine.PictoPool().Snapshot()
	if len(qs) < 2 {
		t.Fatalf("mots ajoutés = %d", len(qs))
	}
	seen := map[string]bool{}
	for _, q := range qs {
		if strings.Contains(q.Question, "<") || q.Answer != "" || !strings.HasPrefix(q.ID, "p-") || used[q.ID] {
			t.Errorf("entrée invalide %+v (USED=%v)", q, used[q.ID])
		}
		k := strings.ToLower(q.Question)
		if (k == "éléphant") && seen["e"] {
			t.Errorf("doublon replié accepté : %q", q.Question)
		}
		if k == "éléphant" {
			seen["e"] = true
		}
	}
	// le Pool Speedy n'est pas touché par la génération Picto
	if rq, _ := s.engine.RafalePool().Snapshot(); len(rq) != 0 {
		t.Errorf("Pool Speedy modifié par une génération Picto : %d entrées", len(rq))
	}
}
