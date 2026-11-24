package server

// PICTO (v13.0.0, #191) — contracts/picto.md §3.1 (règles d'un mot), §4, §11
// (POST /questions, champ `picto_words`, refus PICTO_WORDS_INVALID /
// PICTO_TIME_INVALID). Chemin HTTP réel (handleUploadQuestion) : le piège de
// reconstruction (§11 : le handler recrée la question de zéro) est le test
// principal — sans relecture explicite de `picto_words`, les mots disparaissent.

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
)

func postPictoQuestion(t *testing.T, server *HTTPServer, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/questions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

func pictoFields(words string) map[string]string {
	return map[string]string{
		"question":    "Dessinez le mot",
		"type":        "PICTO",
		"category":    "HISTORY",
		"points":      "10",
		"time":        "90",
		"picto_words": words,
	}
}

// readOnlyQuestionJSON lit l'unique question.json créée sous dataDir.
func readOnlyQuestionJSON(t *testing.T, dataDir string) map[string]interface{} {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one question dir in %s, got %d (err=%v)", dir, len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name(), "question.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func newPictoServer(t *testing.T) (*HTTPServer, string) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0755)
	return server, dataDir
}

func TestPictoUpload_PersistsTypeWordsTimePoints(t *testing.T) {
	server, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, server, pictoFields("Éléphant\nJean-Pierre\nL'arbre\nchat 2"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if m["TYPE"] != "PICTO" {
		t.Errorf("TYPE = %v, want PICTO", m["TYPE"])
	}
	want := []interface{}{"Éléphant", "Jean-Pierre", "L'arbre", "chat 2"}
	if !reflect.DeepEqual(m["PICTO_WORDS"], want) {
		t.Errorf("PICTO_WORDS = %v, want %v (ordre saisi conservé)", m["PICTO_WORDS"], want)
	}
	if m["CATEGORY"] != "HISTORY" {
		t.Errorf("CATEGORY = %v", m["CATEGORY"])
	}
	if got := toStr(m["TIME"]); got != "90" {
		t.Errorf("TIME = %v, want 90", m["TIME"])
	}
	if got := toStr(m["POINTS"]); got != "10" {
		t.Errorf("POINTS = %v, want 10", m["POINTS"])
	}
}

func toStr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(jsonNum(x), ".0"), ".")
	}
	return ""
}

func jsonNum(f float64) string { b, _ := json.Marshal(f); return string(b) }

// §3.1 : espaces de début/fin retirés, espaces internes compactés ; lignes
// séparées par \n ou \r\n (textarea navigateur).
func TestPictoUpload_NormalisesWhitespaceAndCRLF(t *testing.T) {
	server, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, server, pictoFields("  grand   chat \r\nchien"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	want := []interface{}{"grand chat", "chien"}
	if got := readOnlyQuestionJSON(t, dataDir)["PICTO_WORDS"]; !reflect.DeepEqual(got, want) {
		t.Errorf("PICTO_WORDS = %v, want %v", got, want)
	}
}

// §3.1 : bornes de longueur d'un mot (1 à 40 caractères) et de la liste (1 à 20).
func TestPictoUpload_WordAndListBounds(t *testing.T) {
	word40 := strings.Repeat("a", 40)
	word41 := strings.Repeat("a", 41)
	var twenty, twentyOne []string
	for i := 0; i < 21; i++ {
		w := "mot" + string(rune('a'+i))
		if i < 20 {
			twenty = append(twenty, w)
		}
		twentyOne = append(twentyOne, w)
	}
	cases := []struct {
		name  string
		words string
		ok    bool
	}{
		{"1 mot", "chat", true},
		{"mot de 40 caractères", word40, true},
		{"mot de 40 caractères accentués (runes, pas octets)", strings.Repeat("é", 40), true},
		{"mot de 41 caractères", word41, false},
		{"20 mots", strings.Join(twenty, "\n"), true},
		{"21 mots", strings.Join(twentyOne, "\n"), false},
		{"liste vide", "", false},
		{"liste d'espaces et de lignes vides", "  \n \r\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, _ := newPictoServer(t)
			w := postPictoQuestion(t, server, pictoFields(c.words))
			if c.ok {
				if w.Code != http.StatusOK {
					t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
				}
				return
			}
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "PICTO_WORDS_INVALID") {
				t.Fatalf("want 400 PICTO_WORDS_INVALID, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// §3.1 : caractères admis = lettres (accents compris), chiffres, espace,
// apostrophe, tiret. Tout autre caractère est refusé.
func TestPictoUpload_WordCharset(t *testing.T) {
	valid := []string{"Éléphant", "œuf", "Jean-Pierre", "L'arbre", "chat 2", "2024", "çà et là"}
	for _, w := range valid {
		t.Run("ok/"+w, func(t *testing.T) {
			server, _ := newPictoServer(t)
			if r := postPictoQuestion(t, server, pictoFields(w)); r.Code != http.StatusOK {
				t.Fatalf("%q: want 200, got %d: %s", w, r.Code, r.Body.String())
			}
		})
	}
	invalid := []string{"a/b", "mot!", "a_b", "x@y", "a.b", "(chat)", "chat?", "a;b", "chat🐱", "<b>", "a\tb"}
	for _, w := range invalid {
		t.Run("refus/"+w, func(t *testing.T) {
			server, _ := newPictoServer(t)
			r := postPictoQuestion(t, server, pictoFields(w))
			if r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), "PICTO_WORDS_INVALID") {
				t.Fatalf("%q: want 400 PICTO_WORDS_INVALID, got %d: %s", w, r.Code, r.Body.String())
			}
		})
	}
}

// §3.1 : doublon de clé repliée (majuscules et accents repliés) refusé dans
// une même question ; « Éléphant » et « elephant » ont la même clé.
func TestPictoUpload_DuplicateFoldedKeyRefused(t *testing.T) {
	dups := [][2]string{
		{"Éléphant", "elephant"},
		{"chat", "CHAT"},
		{"Ça", "ca"},
		{"chat", "  chat  "},
		{"grand chat", "GRAND   CHAT"},
	}
	for _, d := range dups {
		t.Run(d[0]+"|"+d[1], func(t *testing.T) {
			server, _ := newPictoServer(t)
			r := postPictoQuestion(t, server, pictoFields("avion\n"+d[0]+"\nbateau\n"+d[1]))
			if r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), "PICTO_WORDS_INVALID") {
				t.Fatalf("want 400 PICTO_WORDS_INVALID, got %d: %s", r.Code, r.Body.String())
			}
		})
	}
}

// §4 : TIME obligatoire, de 30 à 300 ; sinon 400 PICTO_TIME_INVALID.
func TestPictoUpload_TimeBounds(t *testing.T) {
	cases := []struct {
		time string
		ok   bool
	}{
		{"30", true}, {"90", true}, {"300", true},
		{"29", false}, {"301", false}, {"0", false}, {"-5", false}, {"abc", false}, {"", false},
	}
	for _, c := range cases {
		t.Run("time="+c.time, func(t *testing.T) {
			server, _ := newPictoServer(t)
			f := pictoFields("chat")
			f["time"] = c.time
			r := postPictoQuestion(t, server, f)
			if c.ok {
				if r.Code != http.StatusOK {
					t.Fatalf("want 200, got %d: %s", r.Code, r.Body.String())
				}
				return
			}
			if r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), "PICTO_TIME_INVALID") {
				t.Fatalf("want 400 PICTO_TIME_INVALID, got %d: %s", r.Code, r.Body.String())
			}
		})
	}
}

// Piège de reconstruction (§11) : une seconde sauvegarde (même numéro) relit
// `picto_words` ; la liste est remplacée, jamais détruite ni cumulée.
func TestPictoUpload_ResaveReplacesWords(t *testing.T) {
	server, dataDir := newPictoServer(t)
	f := pictoFields("chat\nchien")
	f["number"] = "42"
	if r := postPictoQuestion(t, server, f); r.Code != http.StatusOK {
		t.Fatalf("first save: %d %s", r.Code, r.Body.String())
	}
	f = pictoFields("souris")
	f["number"] = "42"
	if r := postPictoQuestion(t, server, f); r.Code != http.StatusOK {
		t.Fatalf("second save: %d %s", r.Code, r.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if !reflect.DeepEqual(m["PICTO_WORDS"], []interface{}{"souris"}) {
		t.Errorf("PICTO_WORDS after re-save = %v, want [souris]", m["PICTO_WORDS"])
	}
}

// Une sauvegarde refusée ne détruit pas les mots déjà enregistrés.
func TestPictoUpload_RefusedResaveKeepsPreviousWords(t *testing.T) {
	server, dataDir := newPictoServer(t)
	f := pictoFields("chat\nchien")
	f["number"] = "7"
	if r := postPictoQuestion(t, server, f); r.Code != http.StatusOK {
		t.Fatalf("first save: %d %s", r.Code, r.Body.String())
	}
	f = pictoFields("chat\nCHAT")
	f["number"] = "7"
	if r := postPictoQuestion(t, server, f); r.Code != http.StatusBadRequest {
		t.Fatalf("duplicate re-save: want 400, got %d", r.Code)
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if !reflect.DeepEqual(m["PICTO_WORDS"], []interface{}{"chat", "chien"}) {
		t.Errorf("PICTO_WORDS after refused re-save = %v, want [chat chien]", m["PICTO_WORDS"])
	}
}

// §4 : le type PICTO n'est pas ajouté à generableQuestionTypes.
func TestPicto_NotInGenerableQuestionTypes(t *testing.T) {
	for _, ty := range generableQuestionTypes {
		if string(ty) == "PICTO" {
			t.Fatal("generableQuestionTypes must not contain PICTO (génération IA reportée en v13.1.0)")
		}
	}
}

// Revue M1 : la durée enregistrée est la valeur validée normalisée, pas la saisie brute.
func TestPictoUpload_TimeStoredNormalised(t *testing.T) {
	server, dataDir := newPictoServer(t)
	f := pictoFields("chat")
	f["time"] = " 090 "
	if r := postPictoQuestion(t, server, f); r.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", r.Code, r.Body.String())
	}
	if got := toStr(readOnlyQuestionJSON(t, dataDir)["TIME"]); got != "90" {
		t.Errorf("TIME = %q, want \"90\"", got)
	}
}
