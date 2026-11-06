// #282 (v11.4.0, LOT-2) — option « Alphabétique » côté HTTP : upload multipart des deux champs
// (contrat rafale.md §15.2, ⚠️ perte silencieuse #107 si non relus) et GET /api/rafale/pool → LETTERS (§15.6).
// Frontière HTTP uniquement : aucun symbole non livré.
package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// tw282PostRafale POSTe un multipart /questions de type RAFALE avec les champs fournis en plus du socle.
func tw282PostRafale(t *testing.T, s *HTTPServer, extra map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	base := map[string]string{
		"question": "Manche alpha", "answer": "-", "type": "RAFALE", "points": "10", "time": "120",
		"RAFALE_DIFFICULTY": "1", "RAFALE_MODE": "SOLO", "RAFALE_QUESTION_TIME": "3", "RAFALE_MAX_QUESTIONS": "20",
	}
	for k, v := range extra {
		base[k] = v
	}
	for k, v := range base {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var saved map[string]interface{}
	if w.Code == http.StatusOK {
		qdir := config.Get().Storage.QuestionsDir
		entries, _ := os.ReadDir(qdir)
		for _, e := range entries {
			if raw, err := os.ReadFile(filepath.Join(qdir, e.Name(), "question.json")); err == nil {
				_ = json.Unmarshal(raw, &saved)
			}
		}
	}
	return w, saved
}

// §15.2 — round-trip : les deux champs sont relus par handleUploadQuestion et persistés dans question.json.
func TestRafaleAlpha282_Upload_RoundTripsBothFields(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := tw282PostRafale(t, s, map[string]string{"rafale_alphabetical": "true", "rafale_wheel_layout": "arc"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["RAFALE_ALPHABETICAL"] != true {
		t.Errorf("RAFALE_ALPHABETICAL doit être persisté à true (sinon perte silencieuse, #107), got %v", saved["RAFALE_ALPHABETICAL"])
	}
	if saved["RAFALE_WHEEL_LAYOUT"] != "arc" {
		t.Errorf("RAFALE_WHEEL_LAYOUT doit être persisté à \"arc\", got %v", saved["RAFALE_WHEEL_LAYOUT"])
	}
}

func TestRafaleAlpha282_Upload_StripLayoutAccepted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := tw282PostRafale(t, s, map[string]string{"rafale_alphabetical": "true", "rafale_wheel_layout": "strip"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["RAFALE_WHEEL_LAYOUT"] != "strip" {
		t.Errorf("layout strip doit être persisté, got %v", saved["RAFALE_WHEEL_LAYOUT"])
	}
}

// §15.2 — champs absents : omitempty, aucune fuite, comportement inchangé (non-régression).
func TestRafaleAlpha282_Upload_FieldsAbsent_OmittedFromQuestionJSON(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := tw282PostRafale(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	for _, k := range []string{"RAFALE_ALPHABETICAL", "RAFALE_WHEEL_LAYOUT"} {
		if v, present := saved[k]; present {
			t.Errorf("%s doit être omis quand non envoyé (omitempty), got %v", k, v)
		}
	}
}

// §15.2 — rafale_alphabetical ≠ "true" ⇒ faux (omis), précédent qcm_hints_enabled.
func TestRafaleAlpha282_Upload_AlphabeticalFalse_Omitted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := tw282PostRafale(t, s, map[string]string{"rafale_alphabetical": "false"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if v, present := saved["RAFALE_ALPHABETICAL"]; present && v != false {
		t.Errorf("rafale_alphabetical=false ne doit pas persister true, got %v", v)
	}
}

// §15.2 — layout invalide ⇒ 400, rien n'est écrit.
func TestRafaleAlpha282_Upload_InvalidLayout_400(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := tw282PostRafale(t, s, map[string]string{"rafale_alphabetical": "true", "rafale_wheel_layout": "circle"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("layout invalide : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("aucune question ne doit être écrite sur un 400, got %v", saved)
	}
}

// §15.2 — « aucun champ RAFALE ne fuit sur un autre type » : une question QCM ignore ces champs.
func TestRafaleAlpha282_Upload_NonRafaleType_NeverCarriesFields(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	_ = os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0o755)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range map[string]string{
		"question": "Capitale ?", "answer": "Paris", "type": "QCM", "points": "10", "time": "30",
		"qcm_answers": `{"RED":"Paris","GREEN":"Lyon","YELLOW":"Nice","BLUE":"Lille"}`, "qcm_correct": "RED",
		"rafale_alphabetical": "true", "rafale_wheel_layout": "arc",
	} {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	entries, _ := os.ReadDir(config.Get().Storage.QuestionsDir)
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(config.Get().Storage.QuestionsDir, e.Name(), "question.json"))
		if err != nil {
			continue
		}
		var q map[string]interface{}
		_ = json.Unmarshal(raw, &q)
		for _, k := range []string{"RAFALE_ALPHABETICAL", "RAFALE_WHEEL_LAYOUT"} {
			if _, present := q[k]; present {
				t.Errorf("une question QCM ne doit jamais porter %s", k)
			}
		}
	}
}

// tw282Pool lit GET /api/rafale/pool pour le couple HISTORY/1.
func tw282Pool(t *testing.T, s *HTTPServer) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/rafale/pool?categories=HISTORY&difficulties=1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET pool : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return m
}

// §15.6 — LETTERS : lettres A–Z triées, sans doublon, `#` exclu, même normalisation que le tirage (« Élan » ⇒ E).
func TestRafaleAlpha282_Pool_LettersUsesSameNormalization(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for i, a := range []string{"Élan", "Eau", "La Seine", "L'Égypte", "Lapin", "Zèbre", "1984", "Abricot"} {
		if _, err := s.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "p" + string(rune('a'+i)), Question: "Q", Answer: a, Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("UpsertRafaleQuestion : %v", err)
		}
	}
	got := tw282Pool(t, s)
	raw, present := got["LETTERS"]
	if !present {
		t.Fatalf("GET /api/rafale/pool doit contenir LETTERS (additif §15.6), got %v", got)
	}
	var letters []string
	for _, x := range raw.([]interface{}) {
		letters = append(letters, x.(string))
	}
	want := []string{"A", "E", "L", "S", "Z"} // Abricot ; Élan/Eau/L'Égypte ; Lapin ; La Seine ; Zèbre ; « 1984 » (#) exclu
	if !reflect.DeepEqual(letters, want) {
		t.Errorf("LETTERS = %v, attendu %v (triées, sans doublon, # exclu)", letters, want)
	}
	if got["AVAILABLE"] == nil || got["USED"] == nil || got["TOTAL"] == nil {
		t.Errorf("les champs historiques AVAILABLE/USED/TOTAL doivent rester présents, got %v", got)
	}
}

// §15.6 — « Toujours présent (tableau vide si aucune) ».
func TestRafaleAlpha282_Pool_LettersAlwaysPresent_EmptyArrayWhenNone(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	got := tw282Pool(t, s)
	arr, ok := got["LETTERS"].([]interface{})
	if !ok {
		t.Fatalf("LETTERS doit être un tableau JSON même vide (jamais absent/null), got %#v", got["LETTERS"])
	}
	if len(arr) != 0 {
		t.Errorf("réservoir vide ⇒ LETTERS vide, got %v", arr)
	}
}

// §15.6 — LETTERS ne compte que les réponses DISPONIBLES (non utilisées).
func TestRafaleAlpha282_Pool_LettersIgnoreUsedQuestions(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for _, q := range []game.RafaleQuestion{
		{ID: "u1", Question: "Q", Answer: "Ail", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "u2", Question: "Q", Answer: "Banane", Category: game.CategoryHistory, Difficulty: 1},
	} {
		if _, err := s.engine.UpsertRafaleQuestion(q); err != nil {
			t.Fatalf("UpsertRafaleQuestion : %v", err)
		}
	}
	// Réinitialise puis épuise une question via l'API de pioche exportée du moteur.
	if _, err := s.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("DrawRafaleQuestion : %v", err)
	}
	got := tw282Pool(t, s)
	arr, _ := got["LETTERS"].([]interface{})
	if len(arr) != 1 {
		t.Errorf("une question déjà utilisée ne doit plus compter : LETTERS = %v (1 lettre attendue)", arr)
	}
}
