package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// #190 (LOT-5, v11.2.2) — validation de POINTS_RULE à l'enregistrement d'une
// question MEMOTION, par la FRONTIÈRE HTTP uniquement (contrat
// contracts/question-types.md §6.2, « Applicabilité par type et validation à
// l'enregistrement »).
//
// Règle TEST_PROCEDURE.md §2.3 (clone partagé) : ce fichier ne référence AUCUN
// symbole nouveau du paquet game (ValidatePointsRule, AllowedPointsRuleModes,
// PointsRuleError…) — seulement httptest + multipart `motion_cards`, motif de
// motion_card_type_lock_184_test.go. Il compile donc dès maintenant et reste
// fonctionnellement ROUGE (200 au lieu de 400) tant que la validation n'existe
// pas dans POST /questions. Les unitaires de game.* sont écrits par dev-backend.
//
// Table de vérité du contrat (type effectif ; TYPE absent ⇒ SPEEDY) :
//
//	binaire     SPEEDY, QCM      : STARS, FIXED            (défaut STARS = champ absent)
//	progression MEMORY, RAFALE   : STARS_PRORATA, STARS, FIXED, PER_UNIT (défaut STARS_PRORATA)
//
// Refus (HTTP 400, requête entière rejetée, rien d'écrit) :
//
//	POINTS_RULE_INVALID_MODE        MODE hors {STARS, FIXED, PER_UNIT, STARS_PRORATA}
//	POINTS_RULE_MODE_NOT_APPLICABLE STARS_PRORATA ou PER_UNIT sur un type binaire
//	POINTS_RULE_VALUE_REQUIRED      FIXED / PER_UNIT sans VALUE entière 1–999
//
// Ordre d'évaluation (plan #190) : mode connu → mode applicable → VALUE.

const (
	codeInvalidMode   = "POINTS_RULE_INVALID_MODE"
	codeNotApplicable = "POINTS_RULE_MODE_NOT_APPLICABLE"
	codeValueRequired = "POINTS_RULE_VALUE_REQUIRED"
)

// pointsRuleCard construit une carte MEMOTION valide pour son type (seulement
// ses propres OwnedFields, pour ne pas déclencher CARD_TYPE_CONTENT_MISMATCH).
// typ == "" ⇒ aucun TYPE (carte antérieure à #184 ⇒ SPEEDY).
func pointsRuleCard(id, typ string) map[string]interface{} {
	card := map[string]interface{}{"ID": id, "RECTO_THEME": "Thème " + id, "DIFFICULTY": 2}
	switch typ {
	case "":
		card["ANSWER_TEXT"] = "Paris"
	case "SPEEDY":
		card["TYPE"] = "SPEEDY"
		card["ANSWER_TEXT"] = "Paris"
	case "QCM":
		card["TYPE"] = "QCM"
		card["QCM_ANSWERS"] = map[string]string{"RED": "Paris", "GREEN": "Lyon", "YELLOW": "Nice", "BLUE": "Metz"}
		card["QCM_CORRECT"] = "RED"
	case "MEMORY":
		card["TYPE"] = "MEMORY"
		card["MEMORY_PAIRS"] = []map[string]interface{}{
			{"ID": 1, "CARD1": map[string]interface{}{"TEXT": "A"}, "CARD2": map[string]interface{}{"TEXT": "A"}},
			{"ID": 2, "CARD1": map[string]interface{}{"TEXT": "B"}, "CARD2": map[string]interface{}{"TEXT": "B"}},
		}
	case "RAFALE":
		card["TYPE"] = "RAFALE"
		card["RAFALE_CATEGORIES"] = []string{"HISTORY"}
		card["RAFALE_DIFFICULTIES"] = []int{1, 2}
	}
	return card
}

// withRule pose POINTS_RULE sur une carte (rule peut être nil ⇒ "null" JSON).
func withRule(card map[string]interface{}, rule interface{}) map[string]interface{} {
	card["POINTS_RULE"] = rule
	return card
}

func rule(mode string) map[string]interface{} {
	return map[string]interface{}{"MODE": mode}
}

func ruleValue(mode string, value interface{}) map[string]interface{} {
	return map[string]interface{}{"MODE": mode, "VALUE": value}
}

// postPointsRuleCards envoie POST /questions (multipart, MEMOTION) avec ces
// cartes et, optionnellement, des images (champs motion_card_<id>_<slot>).
func postPointsRuleCards(t *testing.T, cards []map[string]interface{}, images map[string][]byte) (*httptest.ResponseRecorder, string) {
	t.Helper()
	server, dataDir := setupTestHTTPServer(t)
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}
	var req *http.Request
	if len(images) > 0 {
		req = newMEMOTIONUploadRequestWithImages(t, string(cardsJSON), images)
	} else {
		req = newMEMOTIONUploadRequest(t, string(cardsJSON))
	}
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w, dataDir
}

// assertNothingWritten : après un refus, ni question.json ni image orpheline
// (aucun fichier régulier sous files/questions).
func assertNothingWritten(t *testing.T, dataDir string) {
	t.Helper()
	root := filepath.Join(dataDir, "files", "questions")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Errorf("expected nothing written after a rejected upload, found file %s", path)
		}
		return nil
	})
}

func assertRefused(t *testing.T, w *httptest.ResponseRecorder, dataDir, wantCode string) {
	t.Helper()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 %s, got %d: %s", wantCode, w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(wantCode)) {
		t.Errorf("expected body to name %s, got: %s", wantCode, w.Body.String())
	}
	assertNothingWritten(t, dataDir)
}

// ---------------------------------------------------------------------------
// 400 — POINTS_RULE_INVALID_MODE
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_PointsRule_InvalidMode(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		rule interface{}
	}{
		{"MODE BONUS sur SPEEDY", "SPEEDY", rule("BONUS")},
		{"MODE BONUS sur carte sans TYPE (⇒ SPEEDY)", "", rule("BONUS")},
		{"MODE BONUS sur QCM", "QCM", rule("BONUS")},
		{"MODE BONUS sur MEMORY", "MEMORY", rule("BONUS")},
		{"MODE BONUS sur RAFALE", "RAFALE", rule("BONUS")},
		{"MODE BONUS même avec une VALUE valide : le mode est refusé avant la valeur", "QCM", ruleValue("BONUS", 10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := withRule(pointsRuleCard("mc-1", tt.typ), tt.rule)
			w, dataDir := postPointsRuleCards(t, []map[string]interface{}{card}, nil)
			assertRefused(t, w, dataDir, codeInvalidMode)
		})
	}
}

// ---------------------------------------------------------------------------
// 400 — POINTS_RULE_MODE_NOT_APPLICABLE (STARS_PRORATA / PER_UNIT sur binaire)
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_PointsRule_ModeNotApplicable(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		rule interface{}
	}{
		{"STARS_PRORATA sur SPEEDY", "SPEEDY", rule("STARS_PRORATA")},
		{"STARS_PRORATA sur carte sans TYPE (⇒ SPEEDY)", "", rule("STARS_PRORATA")},
		{"STARS_PRORATA sur QCM", "QCM", rule("STARS_PRORATA")},
		{"PER_UNIT (VALUE 2) sur QCM", "QCM", ruleValue("PER_UNIT", 2)},
		{"PER_UNIT (VALUE 2) sur SPEEDY", "SPEEDY", ruleValue("PER_UNIT", 2)},
		{"PER_UNIT (VALUE 2) sur carte sans TYPE (⇒ SPEEDY)", "", ruleValue("PER_UNIT", 2)},
		{"PER_UNIT sans VALUE sur QCM : « non applicable » prime sur « VALUE requise »", "QCM", rule("PER_UNIT")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := withRule(pointsRuleCard("mc-1", tt.typ), tt.rule)
			w, dataDir := postPointsRuleCards(t, []map[string]interface{}{card}, nil)
			assertRefused(t, w, dataDir, codeNotApplicable)
		})
	}
}

// ---------------------------------------------------------------------------
// 400 — POINTS_RULE_VALUE_REQUIRED (FIXED / PER_UNIT : VALUE entière 1–999)
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_PointsRule_ValueRequired(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		rule interface{}
	}{
		{"FIXED sans VALUE (QCM)", "QCM", rule("FIXED")},
		{"FIXED sans VALUE (SPEEDY)", "SPEEDY", rule("FIXED")},
		{"FIXED VALUE 0", "QCM", ruleValue("FIXED", 0)},
		{"FIXED VALUE négative", "QCM", ruleValue("FIXED", -5)},
		{"FIXED VALUE 1000 (au-dessus de 999)", "QCM", ruleValue("FIXED", 1000)},
		{"FIXED VALUE 2.5 (non entière)", "QCM", ruleValue("FIXED", 2.5)},
		{"FIXED VALUE chaîne « 10 »", "QCM", ruleValue("FIXED", "10")},
		{"FIXED VALUE null", "QCM", ruleValue("FIXED", nil)},
		{"FIXED sans VALUE sur MEMORY", "MEMORY", rule("FIXED")},
		{"PER_UNIT sans VALUE (RAFALE)", "RAFALE", rule("PER_UNIT")},
		{"PER_UNIT VALUE 0 (MEMORY)", "MEMORY", ruleValue("PER_UNIT", 0)},
		{"PER_UNIT VALUE 1000 (RAFALE)", "RAFALE", ruleValue("PER_UNIT", 1000)},
		{"PER_UNIT VALUE 2.5 (MEMORY)", "MEMORY", ruleValue("PER_UNIT", 2.5)},
		{"PER_UNIT VALUE chaîne (RAFALE)", "RAFALE", ruleValue("PER_UNIT", "2")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := withRule(pointsRuleCard("mc-1", tt.typ), tt.rule)
			w, dataDir := postPointsRuleCards(t, []map[string]interface{}{card}, nil)
			assertRefused(t, w, dataDir, codeValueRequired)
		})
	}
}

// ---------------------------------------------------------------------------
// 200 — règles acceptées (et persistées telles quelles)
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_PointsRule_Accepted(t *testing.T) {
	tests := []struct {
		name      string
		typ       string
		rule      interface{}
		absent    bool   // POINTS_RULE absent de la carte envoyée
		wantMode  string // "" ⇒ aucune règle persistée
		wantValue int
	}{
		{name: "POINTS_RULE absent (SPEEDY) — défaut, comportement historique", typ: "SPEEDY", absent: true},
		{name: "POINTS_RULE absent (carte sans TYPE)", typ: "", absent: true},
		{name: "POINTS_RULE absent (MEMORY)", typ: "MEMORY", absent: true},
		{name: "POINTS_RULE null (QCM) — traité comme absent", typ: "QCM", rule: nil},
		{name: "MODE vide (RAFALE) — traité comme défaut", typ: "RAFALE", rule: rule("")},
		{name: "STARS sur SPEEDY", typ: "SPEEDY", rule: rule("STARS"), wantMode: "STARS"},
		{name: "STARS explicite sur QCM — idempotence du défaut", typ: "QCM", rule: rule("STARS"), wantMode: "STARS"},
		{name: "FIXED 10 sur QCM", typ: "QCM", rule: ruleValue("FIXED", 10), wantMode: "FIXED", wantValue: 10},
		{name: "FIXED 10 sur carte sans TYPE (⇒ SPEEDY)", typ: "", rule: ruleValue("FIXED", 10), wantMode: "FIXED", wantValue: 10},
		{name: "FIXED 1 (borne basse)", typ: "QCM", rule: ruleValue("FIXED", 1), wantMode: "FIXED", wantValue: 1},
		{name: "FIXED 999 (borne haute)", typ: "SPEEDY", rule: ruleValue("FIXED", 999), wantMode: "FIXED", wantValue: 999},
		{name: "STARS_PRORATA sur MEMORY — idempotence du défaut du type", typ: "MEMORY", rule: rule("STARS_PRORATA"), wantMode: "STARS_PRORATA"},
		{name: "STARS_PRORATA sur RAFALE", typ: "RAFALE", rule: rule("STARS_PRORATA"), wantMode: "STARS_PRORATA"},
		{name: "STARS sur MEMORY (étoiles pleines, tout ou rien)", typ: "MEMORY", rule: rule("STARS"), wantMode: "STARS"},
		{name: "PER_UNIT 2 sur RAFALE", typ: "RAFALE", rule: ruleValue("PER_UNIT", 2), wantMode: "PER_UNIT", wantValue: 2},
		{name: "PER_UNIT 5 sur MEMORY", typ: "MEMORY", rule: ruleValue("PER_UNIT", 5), wantMode: "PER_UNIT", wantValue: 5},
		{name: "FIXED 7 sur MEMORY", typ: "MEMORY", rule: ruleValue("FIXED", 7), wantMode: "FIXED", wantValue: 7},
		{name: "VALUE ignorée pour STARS (même hors bornes) : acceptée", typ: "SPEEDY", rule: ruleValue("STARS", 5000), wantMode: "STARS"},
		{name: "VALUE ignorée pour STARS_PRORATA (même nulle) : acceptée", typ: "MEMORY", rule: ruleValue("STARS_PRORATA", 0), wantMode: "STARS_PRORATA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := pointsRuleCard("mc-1", tt.typ)
			if !tt.absent {
				withRule(card, tt.rule)
			}
			w, dataDir := postPointsRuleCards(t, []map[string]interface{}{card}, nil)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			q := readWrittenMEMOTIONQuestion(t, dataDir)
			if len(q.MotionCards) != 1 {
				t.Fatalf("expected 1 motion card, got %d", len(q.MotionCards))
			}
			got := q.MotionCards[0].PointsRule
			if tt.wantMode == "" {
				if got != nil && string(got.Mode) != "" {
					t.Errorf("expected no persisted POINTS_RULE, got MODE=%q VALUE=%d", got.Mode, got.Value)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected POINTS_RULE %s to be persisted verbatim, got none", tt.wantMode)
			}
			if string(got.Mode) != tt.wantMode {
				t.Errorf("persisted MODE = %q, want %q", got.Mode, tt.wantMode)
			}
			if tt.wantValue != 0 && got.Value != tt.wantValue {
				t.Errorf("persisted VALUE = %d, want %d", got.Value, tt.wantValue)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Requête entière rejetée, rien d'écrit
// ---------------------------------------------------------------------------

// Une 2e carte invalide fait échouer TOUTE la requête, même si la 1re est valide.
func TestHTTPServer_MEMOTIONUpload_PointsRule_SecondCardInvalidFailsWholeRequest(t *testing.T) {
	cards := []map[string]interface{}{
		withRule(pointsRuleCard("mc-1", "QCM"), ruleValue("FIXED", 10)),
		withRule(pointsRuleCard("mc-2", "SPEEDY"), ruleValue("PER_UNIT", 2)),
	}
	w, dataDir := postPointsRuleCards(t, cards, nil)
	assertRefused(t, w, dataDir, codeNotApplicable)
}

// Symétrique : 1re carte invalide, 2e valide — même résultat.
func TestHTTPServer_MEMOTIONUpload_PointsRule_FirstCardInvalidFailsWholeRequest(t *testing.T) {
	cards := []map[string]interface{}{
		withRule(pointsRuleCard("mc-1", "MEMORY"), rule("BONUS")),
		withRule(pointsRuleCard("mc-2", "QCM"), ruleValue("FIXED", 10)),
	}
	w, dataDir := postPointsRuleCards(t, cards, nil)
	assertRefused(t, w, dataDir, codeInvalidMode)
}

// Une carte invalide parmi des cartes sans règle : refus, y compris sa VALUE.
func TestHTTPServer_MEMOTIONUpload_PointsRule_InvalidAmongDefaultCards(t *testing.T) {
	cards := []map[string]interface{}{
		pointsRuleCard("mc-1", "SPEEDY"),
		pointsRuleCard("mc-2", "MEMORY"),
		withRule(pointsRuleCard("mc-3", "RAFALE"), ruleValue("FIXED", 1000)),
	}
	w, dataDir := postPointsRuleCards(t, cards, nil)
	assertRefused(t, w, dataDir, codeValueRequired)
}

// Aucune image orpheline : une carte refusée qui accompagnait une image
// (champ motion_card_<id>_recto) ne laisse AUCUN fichier sur disque.
func TestHTTPServer_MEMOTIONUpload_PointsRule_RefusedUploadLeavesNoOrphanImage(t *testing.T) {
	cards := []map[string]interface{}{
		withRule(pointsRuleCard("mc-1", "QCM"), ruleValue("FIXED", 0)),
	}
	images := map[string][]byte{"motion_card_mc-1_recto": minimalPNG}
	w, dataDir := postPointsRuleCards(t, cards, images)
	assertRefused(t, w, dataDir, codeValueRequired)
}

// Non-régression : une carte valide avec image est toujours acceptée quand
// elle porte une règle valide (la validation ne casse pas le traitement des images).
func TestHTTPServer_MEMOTIONUpload_PointsRule_AcceptedWithImageStillWritesIt(t *testing.T) {
	cards := []map[string]interface{}{
		withRule(pointsRuleCard("mc-1", "QCM"), ruleValue("FIXED", 10)),
	}
	images := map[string][]byte{"motion_card_mc-1_recto": minimalPNG}
	w, dataDir := postPointsRuleCards(t, cards, images)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if q.MotionCards[0].RectoImage == "" {
		t.Error("expected RECTO_IMAGE to be set for the accepted card")
	}
	if q.MotionCards[0].PointsRule == nil || string(q.MotionCards[0].PointsRule.Mode) != "FIXED" {
		t.Errorf("expected POINTS_RULE FIXED persisted, got %+v", q.MotionCards[0].PointsRule)
	}
}

// Non-régression : une question MEMOTION sans aucune POINTS_RULE (cas de toutes
// les questions antérieures à #190) reste acceptée à l'identique.
func TestHTTPServer_MEMOTIONUpload_PointsRule_LegacyQuestionWithoutRuleAccepted(t *testing.T) {
	cards := []map[string]interface{}{
		pointsRuleCard("mc-1", ""),
		pointsRuleCard("mc-2", "QCM"),
		pointsRuleCard("mc-3", "MEMORY"),
		pointsRuleCard("mc-4", "RAFALE"),
	}
	w, dataDir := postPointsRuleCards(t, cards, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if len(q.MotionCards) != 4 {
		t.Fatalf("expected 4 motion cards, got %d", len(q.MotionCards))
	}
	for i, c := range q.MotionCards {
		if c.PointsRule != nil && string(c.PointsRule.Mode) != "" {
			t.Errorf("card %d: no POINTS_RULE expected, got %+v", i+1, c.PointsRule)
		}
	}
}
