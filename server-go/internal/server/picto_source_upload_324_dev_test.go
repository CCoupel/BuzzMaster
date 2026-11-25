package server

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// #324 (v13.1.0, LOT-2 T8) — POST /questions, champs picto_source / picto_categories / picto_difficulties
// (contracts/picto.md §17.2) : round-trip multipart (handleUploadQuestion reconstruit la question de zéro) et refus.

func reservoirFields(cats, diffs string) map[string]string {
	f := pictoFields("")
	delete(f, "picto_words")
	f["picto_source"] = "RESERVOIR"
	f["picto_categories"] = cats
	f["picto_difficulties"] = diffs
	return f
}

func TestPictoSourceUpload324_Reservoir_RoundTrip_NoWordsWritten(t *testing.T) {
	server, dataDir := newPictoServer(t)
	f := reservoirFields(`["HISTORY","SCIENCE","HISTORY"]`, `[1,2,2,3]`)
	f["picto_words"] = "ignoré" // « Non écrit si la source est RESERVOIR »
	if w := postPictoQuestion(t, server, f); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readOnlyQuestionJSON(t, dataDir)
	if q["PICTO_SOURCE"] != "RESERVOIR" {
		t.Errorf("PICTO_SOURCE = %v", q["PICTO_SOURCE"])
	}
	if !reflect.DeepEqual(q["PICTO_CATEGORIES"], []interface{}{"HISTORY", "SCIENCE"}) {
		t.Errorf("PICTO_CATEGORIES = %v (doublons retirés, ordre conservé)", q["PICTO_CATEGORIES"])
	}
	if !reflect.DeepEqual(q["PICTO_DIFFICULTIES"], []interface{}{float64(1), float64(2), float64(3)}) {
		t.Errorf("PICTO_DIFFICULTIES = %v", q["PICTO_DIFFICULTIES"])
	}
	if _, has := q["PICTO_WORDS"]; has {
		t.Errorf("PICTO_WORDS ne doit pas être écrit avec la source RESERVOIR : %v", q["PICTO_WORDS"])
	}
}

func TestPictoSourceUpload324_LegacyWithoutSource_StaysWords_NoNewKey(t *testing.T) {
	server, dataDir := newPictoServer(t)
	if w := postPictoQuestion(t, server, pictoFields("chat\nchien")); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readOnlyQuestionJSON(t, dataDir)
	for _, k := range []string{"PICTO_SOURCE", "PICTO_CATEGORIES", "PICTO_DIFFICULTIES"} {
		if _, has := q[k]; has {
			t.Errorf("question v13.0.0 : %s ne doit pas apparaître", k)
		}
	}
	if !reflect.DeepEqual(q["PICTO_WORDS"], []interface{}{"chat", "chien"}) {
		t.Errorf("PICTO_WORDS = %v", q["PICTO_WORDS"])
	}
}

func TestPictoSourceUpload324_Words_ExplicitSource_NoFilterWritten(t *testing.T) {
	server, dataDir := newPictoServer(t)
	f := pictoFields("chat")
	f["picto_source"] = "WORDS"
	f["picto_categories"] = `["HISTORY"]`
	if w := postPictoQuestion(t, server, f); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readOnlyQuestionJSON(t, dataDir)
	if q["PICTO_SOURCE"] != "WORDS" {
		t.Errorf("PICTO_SOURCE = %v", q["PICTO_SOURCE"])
	}
	if _, has := q["PICTO_CATEGORIES"]; has {
		t.Error("le filtre n'est pas écrit avec la source WORDS")
	}
}

func TestPictoSourceUpload324_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
		code   string
	}{
		{"source inconnue", func() map[string]string { f := pictoFields("chat"); f["picto_source"] = "POOL"; return f }(), "PICTO_SOURCE_INVALID"},
		{"filtre : catégories absentes", reservoirFields(``, `[1]`), "PICTO_FILTER_INVALID"},
		{"filtre : catégories vides", reservoirFields(`[]`, `[1]`), "PICTO_FILTER_INVALID"},
		{"filtre : catégorie inconnue", reservoirFields(`["NOPE"]`, `[1]`), "PICTO_FILTER_INVALID"},
		{"filtre : difficultés absentes", reservoirFields(`["HISTORY"]`, ``), "PICTO_FILTER_INVALID"},
		{"filtre : difficulté 0", reservoirFields(`["HISTORY"]`, `[0]`), "PICTO_FILTER_INVALID"},
		{"filtre : difficulté 4", reservoirFields(`["HISTORY"]`, `[1,4]`), "PICTO_FILTER_INVALID"},
		{"filtre : JSON invalide", reservoirFields(`HISTORY`, `[1]`), "PICTO_FILTER_INVALID"},
		{"mots : liste vide en source WORDS", pictoFields(""), "PICTO_WORDS_INVALID"},
	}
	for _, c := range cases {
		server, _ := newPictoServer(t)
		w := postPictoQuestion(t, server, c.fields)
		if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), c.code) {
			t.Errorf("%s : %d %q, attendu 400 préfixé %s", c.name, w.Code, w.Body.String(), c.code)
		}
	}
}
