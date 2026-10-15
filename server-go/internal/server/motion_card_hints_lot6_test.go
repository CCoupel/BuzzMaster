package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// #279 (LOT-6, v11.2.2) — « Pas d'indices sur les cartes QCM d'une question
// MEMOTION ». Décision utilisateur 2026-10-15, option B « ignorer
// silencieusement » (contrat question-types.md §7.1, §3.1/§3.2, CHANGELOG
// [20261015]) : à l'enregistrement (POST /questions), les cinq clés d'indices
//
//	QCM_HINTS_ENABLED, QCM_HINT_THRESHOLD_1, QCM_HINT_THRESHOLD_2,
//	QCM_PENALTY_1, QCM_PENALTY_2
//
// sont IGNORÉES et RETIRÉES d'une carte QCM — HTTP 200, aucune erreur, aucun
// code nouveau — et ne figurent donc jamais dans question.json. L'hôte
// QUESTION QCM est strictement inchangé (indices persistés, #254).
//
// Règle TEST_PROCEDURE.md §2.3 (clone partagé) : aucun symbole nouveau du
// paquet game (StripCardHints…) — frontière HTTP uniquement (httptest +
// multipart `motion_cards`, motif de motion_card_type_lock_184_test.go). Ces
// tests compilent dès maintenant et sont RED fonctionnellement (les clés sont
// encore présentes dans question.json) jusqu'à la neutralisation côté serveur.
//
// question.json est relu en JSON BRUT (map), jamais via game.Question : les
// clés sont `omitempty` côté modèle, un décodage typé masquerait une clé
// persistée à `false`/`0`.

var lot6HintKeys = []string{
	"QCM_HINTS_ENABLED",
	"QCM_HINT_THRESHOLD_1",
	"QCM_HINT_THRESHOLD_2",
	"QCM_PENALTY_1",
	"QCM_PENALTY_2",
}

// lot6QCMCard : carte QCM valide (réponses + désignation) ; extra s'ajoute.
func lot6QCMCard(id string, extra map[string]interface{}) map[string]interface{} {
	card := map[string]interface{}{
		"ID": id, "RECTO_THEME": "Capitales", "DIFFICULTY": 2, "TYPE": "QCM",
		"QCM_ANSWERS":   map[string]string{"RED": "Paris", "GREEN": "Lyon", "YELLOW": "Nice", "BLUE": "Metz"},
		"QCM_CORRECT":   "RED",
		"QUESTION_TEXT": "Capitale de la France ?",
	}
	for k, v := range extra {
		card[k] = v
	}
	return card
}

// lot6HintsFull : indices activés + les quatre réglages (valeurs de l'éditeur).
func lot6HintsFull() map[string]interface{} {
	return map[string]interface{}{
		"QCM_HINTS_ENABLED":    true,
		"QCM_HINT_THRESHOLD_1": 0.25,
		"QCM_HINT_THRESHOLD_2": 0.125,
		"QCM_PENALTY_1":        0.67,
		"QCM_PENALTY_2":        0.33,
	}
}

func lot6Post(t *testing.T, cards []map[string]interface{}) (*httptest.ResponseRecorder, string) {
	t.Helper()
	server, dataDir := setupTestHTTPServer(t)
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, newMEMOTIONUploadRequest(t, string(cardsJSON)))
	return w, dataDir
}

// lot6ReadRaw relit l'unique question.json écrit, en JSON brut.
func lot6ReadRaw(t *testing.T, dataDir string) map[string]interface{} {
	t.Helper()
	questionsDir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(questionsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one question directory, got %v (err=%v)", entries, err)
	}
	raw, err := os.ReadFile(filepath.Join(questionsDir, entries[0].Name(), "question.json"))
	if err != nil {
		t.Fatalf("read question.json: %v", err)
	}
	var q map[string]interface{}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("unmarshal question.json: %v", err)
	}
	return q
}

// lot6Cards renvoie MOTION_CARDS persistées, en maps brutes.
func lot6Cards(t *testing.T, q map[string]interface{}) []map[string]interface{} {
	t.Helper()
	list, ok := q["MOTION_CARDS"].([]interface{})
	if !ok {
		t.Fatalf("MOTION_CARDS missing or not an array: %v", q["MOTION_CARDS"])
	}
	cards := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("a motion card is not an object: %v", item)
		}
		cards = append(cards, m)
	}
	return cards
}

func lot6FindCard(t *testing.T, cards []map[string]interface{}, id string) map[string]interface{} {
	t.Helper()
	for _, c := range cards {
		if c["ID"] == id {
			return c
		}
	}
	t.Fatalf("card %s not found in %v", id, cards)
	return nil
}

func lot6AssertNoHintKeys(t *testing.T, card map[string]interface{}) {
	t.Helper()
	for _, key := range lot6HintKeys {
		if v, present := card[key]; present {
			t.Errorf("card %v must not persist %s (LOT-6), got %v", card["ID"], key, v)
		}
	}
}

// lot6AssertQCMContentIntact : tout ce qui n'est PAS un indice reste intact.
func lot6AssertQCMContentIntact(t *testing.T, card map[string]interface{}) {
	t.Helper()
	if card["TYPE"] != "QCM" {
		t.Errorf("TYPE = %v, want QCM", card["TYPE"])
	}
	if card["RECTO_THEME"] != "Capitales" {
		t.Errorf("RECTO_THEME = %v, want Capitales", card["RECTO_THEME"])
	}
	if card["QUESTION_TEXT"] != "Capitale de la France ?" {
		t.Errorf("QUESTION_TEXT = %v, want preserved", card["QUESTION_TEXT"])
	}
	if card["QCM_CORRECT"] != "RED" {
		t.Errorf("QCM_CORRECT = %v, want RED", card["QCM_CORRECT"])
	}
	answers, ok := card["QCM_ANSWERS"].(map[string]interface{})
	if !ok {
		t.Fatalf("QCM_ANSWERS missing or not an object: %v", card["QCM_ANSWERS"])
	}
	for color, want := range map[string]string{"RED": "Paris", "GREEN": "Lyon", "YELLOW": "Nice", "BLUE": "Metz"} {
		if answers[color] != want {
			t.Errorf("QCM_ANSWERS.%s = %v, want %s", color, answers[color], want)
		}
	}
	if d, _ := card["DIFFICULTY"].(float64); d != 2 {
		t.Errorf("DIFFICULTY = %v, want 2", card["DIFFICULTY"])
	}
}

// ---------------------------------------------------------------------------
// Carte QCM avec indices : 200, les 5 clés retirées, le reste intact
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_QCMCard_HintsIgnoredAndStripped(t *testing.T) {
	tests := []struct {
		name  string
		hints map[string]interface{}
	}{
		{"indices activés + seuils + pénalités (cas de l'éditeur avant LOT-6)", lot6HintsFull()},
		{"QCM_HINTS_ENABLED:true seul, sans réglages", map[string]interface{}{"QCM_HINTS_ENABLED": true}},
		{"QCM_HINTS_ENABLED:false explicite + réglages non défaut", map[string]interface{}{
			"QCM_HINTS_ENABLED": false, "QCM_HINT_THRESHOLD_1": 0.5, "QCM_HINT_THRESHOLD_2": 0.2,
			"QCM_PENALTY_1": 0.9, "QCM_PENALTY_2": 0.1,
		}},
		{"QCM_HINTS_ENABLED:false explicite seul (clé morte à ne pas persister)", map[string]interface{}{"QCM_HINTS_ENABLED": false}},
		{"réglages sans QCM_HINTS_ENABLED", map[string]interface{}{
			"QCM_HINT_THRESHOLD_1": 0.3, "QCM_HINT_THRESHOLD_2": 0.1, "QCM_PENALTY_1": 0.5, "QCM_PENALTY_2": 0.2,
		}},
		{"seuil 1 seul", map[string]interface{}{"QCM_HINT_THRESHOLD_1": 0.25}},
		{"pénalité 2 seule", map[string]interface{}{"QCM_PENALTY_2": 0.33}},
		{"valeurs hors plage (jamais refusées, simplement ignorées)", map[string]interface{}{
			"QCM_HINTS_ENABLED": true, "QCM_HINT_THRESHOLD_1": 7, "QCM_HINT_THRESHOLD_2": -1,
			"QCM_PENALTY_1": 5, "QCM_PENALTY_2": 0,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards := []map[string]interface{}{lot6QCMCard("mc-1", tt.hints)}
			w, dataDir := lot6Post(t, cards)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 (hints silently ignored), got %d: %s", w.Code, w.Body.String())
			}
			card := lot6FindCard(t, lot6Cards(t, lot6ReadRaw(t, dataDir)), "mc-1")
			lot6AssertNoHintKeys(t, card)
			lot6AssertQCMContentIntact(t, card)
		})
	}
}

func TestHTTPServer_MEMOTIONUpload_QCMCard_NoNewErrorCode(t *testing.T) {
	// Option B : aucun code d'erreur nouveau (l'option A, refus
	// CARD_HINTS_NOT_ALLOWED, a été écartée). Même avec des indices, jamais de 4xx.
	w, _ := lot6Post(t, []map[string]interface{}{lot6QCMCard("mc-1", lot6HintsFull())})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("CARD_HINTS_NOT_ALLOWED")) {
		t.Errorf("no CARD_HINTS_NOT_ALLOWED code must exist (option B), got: %s", w.Body.String())
	}
}

func TestHTTPServer_MEMOTIONUpload_QCMCards_AllStrippedIndependently(t *testing.T) {
	cards := []map[string]interface{}{
		lot6QCMCard("mc-1", lot6HintsFull()),
		lot6QCMCard("mc-2", map[string]interface{}{"QCM_HINTS_ENABLED": true}),
		lot6QCMCard("mc-3", nil),
	}
	w, dataDir := lot6Post(t, cards)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	persisted := lot6Cards(t, lot6ReadRaw(t, dataDir))
	if len(persisted) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(persisted))
	}
	for _, id := range []string{"mc-1", "mc-2", "mc-3"} {
		card := lot6FindCard(t, persisted, id)
		lot6AssertNoHintKeys(t, card)
		lot6AssertQCMContentIntact(t, card)
	}
}

func TestHTTPServer_MEMOTIONUpload_QCMCard_HintsStrippedButPointsRuleKept(t *testing.T) {
	// Interaction avec #190 (LOT-5) : la neutralisation ne touche que les 5 clés
	// d'indices — la règle de points de la carte est conservée telle quelle.
	extra := lot6HintsFull()
	extra["POINTS_RULE"] = map[string]interface{}{"MODE": "FIXED", "VALUE": 10}
	w, dataDir := lot6Post(t, []map[string]interface{}{lot6QCMCard("mc-1", extra)})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	card := lot6FindCard(t, lot6Cards(t, lot6ReadRaw(t, dataDir)), "mc-1")
	lot6AssertNoHintKeys(t, card)
	rule, ok := card["POINTS_RULE"].(map[string]interface{})
	if !ok {
		t.Fatalf("POINTS_RULE must be kept on the card, got %v", card["POINTS_RULE"])
	}
	if rule["MODE"] != "FIXED" {
		t.Errorf("POINTS_RULE.MODE = %v, want FIXED", rule["MODE"])
	}
	if v, _ := rule["VALUE"].(float64); v != 10 {
		t.Errorf("POINTS_RULE.VALUE = %v, want 10", rule["VALUE"])
	}
}

func TestHTTPServer_MEMOTIONUpload_QCMCard_HintsStrippedWithImages(t *testing.T) {
	// Le traitement des images de la carte n'est pas perturbé par la neutralisation.
	cardsJSON, err := json.Marshal([]map[string]interface{}{lot6QCMCard("mc-1", lot6HintsFull())})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	server, dataDir := setupTestHTTPServer(t)
	req := newMEMOTIONUploadRequestWithImages(t, string(cardsJSON), map[string][]byte{"motion_card_mc-1_recto": minimalPNG})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	card := lot6FindCard(t, lot6Cards(t, lot6ReadRaw(t, dataDir)), "mc-1")
	lot6AssertNoHintKeys(t, card)
	if img, _ := card["RECTO_IMAGE"].(string); img == "" {
		t.Error("RECTO_IMAGE must still be set on the QCM card")
	}
}

// ---------------------------------------------------------------------------
// Aucune régression : cartes sans indices, autres types, refus existants
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_QCMCard_WithoutHintsUnchanged(t *testing.T) {
	w, dataDir := lot6Post(t, []map[string]interface{}{lot6QCMCard("mc-1", nil)})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	card := lot6FindCard(t, lot6Cards(t, lot6ReadRaw(t, dataDir)), "mc-1")
	lot6AssertNoHintKeys(t, card) // aucune clé d'indice ajoutée par le serveur
	lot6AssertQCMContentIntact(t, card)
}

func TestHTTPServer_MEMOTIONUpload_OtherCardTypes_Unchanged(t *testing.T) {
	cards := []map[string]interface{}{
		{"ID": "sp", "RECTO_THEME": "Histoire", "DIFFICULTY": 1, "TYPE": "SPEEDY", "ANSWER_TEXT": "1789"},
		{"ID": "legacy", "RECTO_THEME": "Sport", "DIFFICULTY": 3, "ANSWER_TEXT": "Zidane"},
		{
			"ID": "mem", "RECTO_THEME": "Paires", "DIFFICULTY": 2, "TYPE": "MEMORY",
			"MEMORY_PAIRS": []map[string]interface{}{
				{"ID": 1, "CARD1": map[string]interface{}{"TEXT": "A"}, "CARD2": map[string]interface{}{"TEXT": "A"}},
				{"ID": 2, "CARD1": map[string]interface{}{"TEXT": "B"}, "CARD2": map[string]interface{}{"TEXT": "B"}},
			},
		},
		{
			"ID": "raf", "RECTO_THEME": "Rafale", "DIFFICULTY": 2, "TYPE": "RAFALE",
			"RAFALE_CATEGORIES": []string{"HISTORY"}, "RAFALE_DIFFICULTIES": []int{1, 2},
		},
	}
	w, dataDir := lot6Post(t, cards)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	persisted := lot6Cards(t, lot6ReadRaw(t, dataDir))
	if len(persisted) != 4 {
		t.Fatalf("expected 4 cards, got %d", len(persisted))
	}
	if lot6FindCard(t, persisted, "sp")["ANSWER_TEXT"] != "1789" {
		t.Error("SPEEDY card ANSWER_TEXT must be preserved")
	}
	if lot6FindCard(t, persisted, "legacy")["ANSWER_TEXT"] != "Zidane" {
		t.Error("TYPE-less card ANSWER_TEXT must be preserved")
	}
	if pairs, _ := lot6FindCard(t, persisted, "mem")["MEMORY_PAIRS"].([]interface{}); len(pairs) != 2 {
		t.Errorf("MEMORY card must keep its 2 pairs, got %d", len(pairs))
	}
	if cats, _ := lot6FindCard(t, persisted, "raf")["RAFALE_CATEGORIES"].([]interface{}); len(cats) != 1 {
		t.Errorf("RAFALE card must keep its categories, got %v", cats)
	}
	for _, c := range persisted {
		lot6AssertNoHintKeys(t, c)
	}
}

func TestHTTPServer_MEMOTIONUpload_SpeedyCardCarryingHints_StillContentMismatch(t *testing.T) {
	// Garde existante (#184, contrat §3.2) : un champ d'indices sur une carte
	// qui n'est pas QCM reste un CARD_TYPE_CONTENT_MISMATCH — la neutralisation
	// est propre aux cartes QCM, elle ne masque pas ce refus.
	card := map[string]interface{}{
		"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "SPEEDY",
		"QCM_PENALTY_1": 0.5,
	}
	w, dataDir := lot6Post(t, []map[string]interface{}{card})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 CARD_TYPE_CONTENT_MISMATCH, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("CARD_TYPE_CONTENT_MISMATCH")) {
		t.Errorf("expected CARD_TYPE_CONTENT_MISMATCH, got: %s", w.Body.String())
	}
	assertNothingWritten(t, dataDir)
}

func TestHTTPServer_MEMOTIONUpload_InvalidSecondCard_WholeRequestRejected_NothingWritten(t *testing.T) {
	// La neutralisation n'a lieu qu'à l'acceptation : une requête refusée
	// n'écrit rien, même si sa première carte (QCM à indices) était valide.
	cards := []map[string]interface{}{
		lot6QCMCard("mc-1", lot6HintsFull()),
		{"ID": "mc-2", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "BOGUS_TYPE"},
	}
	w, dataDir := lot6Post(t, cards)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertNothingWritten(t, dataDir)
}

// ---------------------------------------------------------------------------
// NR hôte : une question QCM CLASSIQUE garde ses indices (#254, LOT-6 ne la touche pas)
// ---------------------------------------------------------------------------

func lot6PostHostQCM(t *testing.T, fields map[string]string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	server, dataDir := setupTestHTTPServer(t)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	base := map[string]string{
		"question":    "Capitale de la France ?",
		"answer":      "Paris",
		"type":        "QCM",
		"points":      "10",
		"time":        "30",
		"qcm_answers": `{"RED":"Paris","GREEN":"Lyon","YELLOW":"Marseille","BLUE":"Nice"}`,
		"qcm_correct": "RED",
	}
	for k, v := range fields {
		base[k] = v
	}
	for k, v := range base {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w, dataDir
}

func TestHTTPServer_HostQCMQuestion_HintsStillAcceptedAndPersisted(t *testing.T) {
	w, dataDir := lot6PostHostQCM(t, map[string]string{
		"qcm_hints_enabled":    "true",
		"qcm_hint_threshold_1": "0.25",
		"qcm_hint_threshold_2": "0.125",
		"qcm_penalty_1":        "0.67",
		"qcm_penalty_2":        "0.33",
		"qcm_wrong_penalty":    "-2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := lot6ReadRaw(t, dataDir)
	if q["QCM_HINTS_ENABLED"] != true {
		t.Errorf("host QCM_HINTS_ENABLED = %v, want true (unchanged by LOT-6)", q["QCM_HINTS_ENABLED"])
	}
	for key, want := range map[string]float64{
		"QCM_HINT_THRESHOLD_1": 0.25, "QCM_HINT_THRESHOLD_2": 0.125, "QCM_PENALTY_1": 0.67, "QCM_PENALTY_2": 0.33,
	} {
		if got, _ := q[key].(float64); got != want {
			t.Errorf("host %s = %v, want %v", key, q[key], want)
		}
	}
	if got, _ := q["QCM_WRONG_PENALTY"].(float64); got != -2 {
		t.Errorf("host QCM_WRONG_PENALTY = %v, want -2 (#254 unchanged)", q["QCM_WRONG_PENALTY"])
	}
	if q["QCM_CORRECT"] != "RED" {
		t.Errorf("host QCM_CORRECT = %v, want RED", q["QCM_CORRECT"])
	}
}

func TestHTTPServer_HostQCMQuestion_HintsDisabledStillWrittenFalse(t *testing.T) {
	w, dataDir := lot6PostHostQCM(t, map[string]string{"qcm_hints_enabled": "false"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := lot6ReadRaw(t, dataDir)
	if v, present := q["QCM_HINTS_ENABLED"]; !present || v != false {
		t.Errorf("host QCM_HINTS_ENABLED must stay written as false (unchanged), got present=%v value=%v", present, v)
	}
}
