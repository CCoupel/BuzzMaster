// #145 (v12.0.0, LOT-1.1) — type ROUE côté HTTP : POST /questions (champs ROUE_*, refus 400 préfixés par leur code)
// et POST /api/roue/preview. Source : contracts/roue.md §3, §11.1, §11.2. Frontière HTTP uniquement.
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
	"strings"
	"testing"

	"buzzcontrol/internal/config"
)

const roueSamplePhrase = "Combien pour ce chien dans la vitrine"

// postRoue145 POSTe un multipart /questions de type ROUE ; extra complète/remplace le socle. Une valeur
// "\x00" retire le champ du socle. Renvoie la réponse et la question relue sur disque (nil si rien d'écrit).
func postRoue145(t *testing.T, s *HTTPServer, extra map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	base := map[string]string{
		"question": "Manche roue", "type": "ROUE", "category": "ANIMALS", "roue_phrase": roueSamplePhrase,
	}
	for k, v := range extra {
		if v == "\x00" {
			delete(base, k)
		} else {
			base[k] = v
		}
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range base {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w, readOnlyQuestion145(t)
}

// readOnlyQuestion145 relit l'unique question.json du dossier de questions (nil s'il n'y en a pas).
func readOnlyQuestion145(t *testing.T) map[string]interface{} {
	t.Helper()
	qdir := config.Get().Storage.QuestionsDir
	entries, _ := os.ReadDir(qdir)
	var saved map[string]interface{}
	for _, e := range entries {
		if raw, err := os.ReadFile(filepath.Join(qdir, e.Name(), "question.json")); err == nil {
			saved = nil
			_ = json.Unmarshal(raw, &saved)
		}
	}
	return saved
}

func questionDirCount145(t *testing.T) int {
	t.Helper()
	entries, _ := os.ReadDir(config.Get().Storage.QuestionsDir)
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}

// --- Création : smoke ----------------------------------------------------------------------------

// smoke — création d'une question ROUE : 200, phrase et réglages par défaut persistés, ANSWER vide.
func TestRoueUpload145_Smoke_CreatesQuestionWithDefaults(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved == nil {
		t.Fatal("question.json non écrit")
	}
	if saved["TYPE"] != "ROUE" {
		t.Errorf("TYPE = %v, attendu ROUE", saved["TYPE"])
	}
	if saved["ROUE_PHRASE"] != roueSamplePhrase {
		t.Errorf("ROUE_PHRASE = %v", saved["ROUE_PHRASE"])
	}
	// défauts du contrat §3
	if saved["ROUE_MODE"] != "TANT_QUE_JE_GAGNE" {
		t.Errorf("ROUE_MODE par défaut = %v, attendu TANT_QUE_JE_GAGNE", saved["ROUE_MODE"])
	}
	if saved["ROUE_GAINS_RULE"] != "WINNER_ONLY" {
		t.Errorf("ROUE_GAINS_RULE par défaut = %v, attendu WINNER_ONLY", saved["ROUE_GAINS_RULE"])
	}
	if saved["ROUE_WHEEL_LAYOUT"] != "strip" {
		t.Errorf("ROUE_WHEEL_LAYOUT par défaut = %v, attendu strip", saved["ROUE_WHEEL_LAYOUT"])
	}
	// la phrase n'est JAMAIS copiée dans ANSWER (§3)
	if a, _ := saved["ANSWER"].(string); a != "" {
		t.Errorf("ANSWER doit rester vide, got %q", a)
	}
	// absent = true : le champ n'est écrit que pour false
	if v, present := saved["ROUE_SHOW_LETTERS"]; present {
		t.Errorf("ROUE_SHOW_LETTERS ne doit pas être écrit quand il vaut true, got %v", v)
	}
}

// Round-trip complet des six champs (leçon #107 : un champ non relu serait perdu).
func TestRoueUpload145_RoundTripsAllSixFields(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_presentation": "Chanson de Line Renaud", "ROUE_MODE": "CHACUN_SON_TOUR", "roue_gains_rule": "ALL_TEAMS",
		"roue_wheel_layout": "arc", "roue_show_letters": "false",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	want := map[string]interface{}{
		"ROUE_PHRASE": roueSamplePhrase, "ROUE_PRESENTATION": "Chanson de Line Renaud", "ROUE_MODE": "CHACUN_SON_TOUR",
		"ROUE_GAINS_RULE": "ALL_TEAMS", "ROUE_WHEEL_LAYOUT": "arc", "ROUE_SHOW_LETTERS": false,
	}
	for k, v := range want {
		if !reflect.DeepEqual(saved[k], v) {
			t.Errorf("%s = %v (%T), attendu %v", k, saved[k], saved[k], v)
		}
	}
}

func TestRoueUpload145_ModesAcceptedAndStored(t *testing.T) {
	for _, mode := range []string{"SOLO", "CHACUN_SON_TOUR", "TANT_QUE_JE_GAGNE"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{"ROUE_MODE": mode})
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			if saved["ROUE_MODE"] != mode {
				t.Errorf("ROUE_MODE = %v, attendu %s", saved["ROUE_MODE"], mode)
			}
		})
	}
}

// La phrase est enregistrée « telle que saisie, espaces compactés » (§3) — accents et casse conservés.
func TestRoueUpload145_PhraseStoredAsTypedWithSpacesCompacted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_phrase": "  Été   à  Noël  "})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_PHRASE"] != "Été à Noël" {
		t.Errorf("ROUE_PHRASE = %q, attendu %q", saved["ROUE_PHRASE"], "Été à Noël")
	}
}

// Une phrase de 14 caractères pile et un 4×14 pile sont acceptés (limites incluses).
func TestRoueUpload145_BoundaryPhrasesAccepted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, _ := postRoue145(t, s, map[string]string{"roue_phrase": strings.Join([]string{
		"AAAAAAAAAAAAAA", "BBBBBBBBBBBBBB", "CCCCCCCCCCCCCC", "DDDDDDDDDDDDDD"}, " ")})
	if w.Code != http.StatusOK {
		t.Fatalf("4 lignes de 14 : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
}

func TestRoueUpload145_PresentationMax80Accepted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_presentation": strings.Repeat("é", 80)})
	if w.Code != http.StatusOK {
		t.Fatalf("80 caractères : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_PRESENTATION"] != strings.Repeat("é", 80) {
		t.Errorf("présentation de 80 caractères non conservée")
	}
}

// --- Refus 400 : code en préfixe, rien d'écrit, dossier d'une création refusée supprimé (#278) ---

func TestRoueUpload145_Refusals400WithCodeAndNothingWritten(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		code  string
	}{
		{"phrase vide", map[string]string{"roue_phrase": ""}, "ROUE_PHRASE_EMPTY"},
		{"phrase absente", map[string]string{"roue_phrase": "\x00"}, "ROUE_PHRASE_EMPTY"},
		{"phrase d'espaces", map[string]string{"roue_phrase": "     "}, "ROUE_PHRASE_EMPTY"},
		{"caractère interdit _", map[string]string{"roue_phrase": "mon_chien"}, "ROUE_PHRASE_INVALID_CHAR"},
		{"caractère interdit *", map[string]string{"roue_phrase": "mon*chien"}, "ROUE_PHRASE_INVALID_CHAR"},
		{"mot de 15", map[string]string{"roue_phrase": "ABCDEFGHIJKLMNO"}, "ROUE_PHRASE_WORD_TOO_LONG"},
		{"5 lignes", map[string]string{"roue_phrase": "AAAAAAAAAAAAAA BBBBBBBBBBBBBB CCCCCCCCCCCCCC DDDDDDDDDDDDDD EEEEEEEEEEEEEE"}, "ROUE_PHRASE_TOO_LONG"},
		{"présentation de 81", map[string]string{"roue_presentation": strings.Repeat("a", 81)}, "ROUE_PRESENTATION_TOO_LONG"},
		{"mode inconnu", map[string]string{"ROUE_MODE": "ALEATOIRE"}, "ROUE_MODE_INVALID"},
		{"MAILLON_FAIBLE refusé (propre à RAFALE)", map[string]string{"ROUE_MODE": "MAILLON_FAIBLE"}, "ROUE_MODE_INVALID"},
		{"règle de gains inconnue", map[string]string{"roue_gains_rule": "TOUT"}, "ROUE_GAINS_RULE_INVALID"},
		{"forme de roue inconnue", map[string]string{"roue_wheel_layout": "circle"}, "ROUE_WHEEL_LAYOUT_INVALID"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, c.extra)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			if !strings.HasPrefix(strings.TrimSpace(w.Body.String()), c.code) {
				t.Errorf("le corps doit commencer par %s, got %q", c.code, w.Body.String())
			}
			if saved != nil {
				t.Errorf("rien ne doit être écrit sur un 400, got %v", saved)
			}
			if n := questionDirCount145(t); n != 0 {
				t.Errorf("le dossier d'une création refusée doit être supprimé (#278), %d dossier(s) restant(s)", n)
			}
		})
	}
}

// Modification refusée : la question existante n'est pas altérée.
func TestRoueUpload145_RefusedEditLeavesExistingQuestionUntouched(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w, _ := postRoue145(t, s, nil); w.Code != http.StatusOK {
		t.Fatalf("création : %d %s", w.Code, w.Body.String())
	}
	before := readOnlyQuestion145(t)
	id, _ := before["ID"].(string)
	if id == "" {
		t.Skip("ID de question absent : l'édition par ID n'est pas pilotable ici")
	}
	w, _ := postRoue145(t, s, map[string]string{"number": id, "roue_phrase": "mauvais_caractere"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("attendu 400, got %d", w.Code)
	}
	after := readOnlyQuestion145(t)
	if after["ROUE_PHRASE"] != roueSamplePhrase {
		t.Errorf("la phrase existante a été altérée : %v", after["ROUE_PHRASE"])
	}
}

// --- Non-régression : aucun champ ROUE_* ne fuit sur un autre type ---------------------------------

func TestRoueUpload145_NonRoueTypeNeverCarriesRoueFields(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range map[string]string{
		"question": "Capitale ?", "answer": "Paris", "type": "SPEEDY", "points": "10", "time": "30",
		"roue_phrase": "Une phrase", "roue_presentation": "x", "ROUE_MODE": "SOLO", "roue_gains_rule": "ALL_TEAMS",
		"roue_wheel_layout": "arc", "roue_show_letters": "false",
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
	saved := readOnlyQuestion145(t)
	for k := range saved {
		if strings.HasPrefix(k, "ROUE_") {
			t.Errorf("champ %s inattendu sur une question SPEEDY", k)
		}
	}
}

// --- POST /api/roue/preview ------------------------------------------------------------------------

func postPreview145(t *testing.T, s *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/roue/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func TestRouePreview145_ReferenceExample(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postPreview145(t, s, `{"PHRASE":"Combien pour ce chien dans la vitrine"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var got struct {
		Cols       int      `json:"COLS"`
		MaxRows    int      `json:"MAX_ROWS"`
		Rows       []string `json:"ROWS"`
		Consonants int      `json:"CONSONANTS"`
		Letters    int      `json:"LETTERS"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	if got.Cols != 14 || got.MaxRows != 4 || got.Consonants != 12 || got.Letters != 17 {
		t.Errorf("COLS/MAX_ROWS/CONSONANTS/LETTERS = %d/%d/%d/%d, attendu 14/4/12/17", got.Cols, got.MaxRows, got.Consonants, got.Letters)
	}
	if !reflect.DeepEqual(got.Rows, []string{"COMBIEN POUR", "CE CHIEN DANS", "LA VITRINE"}) {
		t.Errorf("ROWS = %q", got.Rows)
	}
}

func TestRouePreview145_Errors400WithCodeAndMessage(t *testing.T) {
	cases := map[string]string{
		`{"PHRASE":""}`:                "ROUE_PHRASE_EMPTY",
		`{"PHRASE":"a_b"}`:             "ROUE_PHRASE_INVALID_CHAR",
		`{"PHRASE":"ABCDEFGHIJKLMNO"}`: "ROUE_PHRASE_WORD_TOO_LONG",
		`{"PHRASE":"AAAAAAAAAAAAAA BBBBBBBBBBBBBB CCCCCCCCCCCCCC DDDDDDDDDDDDDD EEEEEEEEEEEEEE"}`: "ROUE_PHRASE_TOO_LONG",
	}
	for body, code := range cases {
		t.Run(code, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postPreview145(t, s, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			var got map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("le refus doit être un JSON {CODE, MESSAGE} : %v (%s)", err, w.Body.String())
			}
			if got["CODE"] != code || got["MESSAGE"] == "" {
				t.Errorf("CODE/MESSAGE = %q/%q, attendu %s + message non vide", got["CODE"], got["MESSAGE"], code)
			}
		})
	}
}

// L'aperçu n'écrit rien (§11.2).
func TestRouePreview145_WritesNothing(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	_ = postPreview145(t, s, `{"PHRASE":"Bonjour le monde"}`)
	if n := questionDirCount145(t); n != 0 {
		t.Errorf("l'aperçu ne doit rien écrire, %d dossier(s) de question créé(s)", n)
	}
}

// La règle de mise en lignes est unique (serveur) : l'aperçu et l'enregistrement s'accordent sur l'acceptation.
func TestRouePreview145_AgreesWithUploadOnAcceptance(t *testing.T) {
	phrases := []string{roueSamplePhrase, "ABCDEFGHIJKLMNO", "a_b", "AAAAAAA BBBBBB", "Il ne faut pas vendre la peau de l'ours avant de l'avoir tué"}
	for _, p := range phrases {
		t.Run(p, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			b, _ := json.Marshal(map[string]string{"PHRASE": p})
			prev := postPreview145(t, s, string(b))
			up, _ := postRoue145(t, s, map[string]string{"roue_phrase": p})
			if (prev.Code == http.StatusOK) != (up.Code == http.StatusOK) {
				t.Errorf("aperçu=%d enregistrement=%d : la règle doit être unique", prev.Code, up.Code)
			}
		})
	}
}

func TestRouePreview145_MethodNotAllowedAndBadJSON(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	req := httptest.NewRequest("GET", "/api/roue/preview", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET : attendu 405, got %d", w.Code)
	}
	if w := postPreview145(t, s, `{pas du json`); w.Code != http.StatusBadRequest {
		t.Errorf("JSON invalide : attendu 400, got %d", w.Code)
	}
}
