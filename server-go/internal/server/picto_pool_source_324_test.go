package server

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : source « Pool » du type PICTO, chemin HTTP réel (POST /questions,
// contracts/picto.md §17.2, §11). Le handler recrée la question de zéro : chaque champ lu DOIT être réécrit
// (round-trip multipart). Une question v13.0.0 (source absente) reste en source Mots, sans champ parasite.

func poolFields(extra map[string]string) map[string]string {
	f := map[string]string{
		"question": "Dessinez le mot", "type": "PICTO", "category": "HISTORY", "points": "10", "time": "90",
		"picto_source": "RESERVOIR", "picto_categories": `["HISTORY","SCIENCE"]`, "picto_difficulties": `[1,2]`,
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

func TestPictoSource_Reservoir_RoundTrip_FilterPersisted_NoWordsWritten(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, poolFields(nil))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if m["PICTO_SOURCE"] != "RESERVOIR" {
		t.Errorf("PICTO_SOURCE = %v", m["PICTO_SOURCE"])
	}
	if !reflect.DeepEqual(m["PICTO_CATEGORIES"], []interface{}{"HISTORY", "SCIENCE"}) {
		t.Errorf("PICTO_CATEGORIES = %v", m["PICTO_CATEGORIES"])
	}
	if !reflect.DeepEqual(m["PICTO_DIFFICULTIES"], []interface{}{float64(1), float64(2)}) {
		t.Errorf("PICTO_DIFFICULTIES = %v", m["PICTO_DIFFICULTIES"])
	}
	if _, has := m["PICTO_WORDS"]; has {
		t.Errorf("aucun mot ne doit être écrit en source RESERVOIR : %v", m["PICTO_WORDS"])
	}
	if got := toStr(m["TIME"]); got != "90" {
		t.Errorf("TIME = %v", m["TIME"])
	}
}

func TestPictoSource_Reservoir_IgnoresWordsSent_DuplicateCategoriesDeduplicated(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, poolFields(map[string]string{"picto_words": "chat\nchien", "picto_categories": `["HISTORY","HISTORY"]`}))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if _, has := m["PICTO_WORDS"]; has {
		t.Errorf("mots envoyés avec la source Pool : ils ne doivent pas être écrits (%v)", m["PICTO_WORDS"])
	}
	if !reflect.DeepEqual(m["PICTO_CATEGORIES"], []interface{}{"HISTORY"}) {
		t.Errorf("catégories non dédoublonnées : %v", m["PICTO_CATEGORIES"])
	}
}

func TestPictoSource_Reservoir_Refusals_400(t *testing.T) {
	cases := []struct {
		name   string
		extra  map[string]string
		wantIn string
	}{
		{"aucune catégorie", map[string]string{"picto_categories": `[]`}, "PICTO_FILTER_INVALID"},
		{"catégorie inconnue", map[string]string{"picto_categories": `["NOPE"]`}, "PICTO_FILTER_INVALID"},
		{"JSON catégories invalide", map[string]string{"picto_categories": `HISTORY`}, "PICTO_FILTER_INVALID"},
		{"aucune difficulté", map[string]string{"picto_difficulties": `[]`}, "PICTO_FILTER_INVALID"},
		{"difficulté 4", map[string]string{"picto_difficulties": `[4]`}, "PICTO_FILTER_INVALID"},
		{"source inconnue", map[string]string{"picto_source": "AUTRE"}, "PICTO_SOURCE_INVALID"},
	}
	for _, c := range cases {
		s, _ := newPictoServer(t)
		w := postPictoQuestion(t, s, poolFields(c.extra))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.wantIn) {
			t.Errorf("%s : %d %q, attendu 400 %s", c.name, w.Code, strings.TrimSpace(w.Body.String()), c.wantIn)
		}
	}
}

func TestPictoSource_Absent_V1300Question_StaysWords_NoParasiteField(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, pictoFields("chat\nchien"))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	for _, k := range []string{"PICTO_SOURCE", "PICTO_CATEGORIES", "PICTO_DIFFICULTIES"} {
		if _, has := m[k]; has {
			t.Errorf("champ parasite %s sur une question à mots saisis sans source : %v", k, m[k])
		}
	}
	if !reflect.DeepEqual(m["PICTO_WORDS"], []interface{}{"chat", "chien"}) {
		t.Errorf("PICTO_WORDS = %v", m["PICTO_WORDS"])
	}
}

func TestPictoSource_ExplicitWords_NoFilterWritten_WordsRequired(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, poolFields(map[string]string{"picto_source": "WORDS", "picto_words": "chat"}))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if _, has := m["PICTO_CATEGORIES"]; has {
		t.Errorf("source WORDS : aucun filtre écrit (%v)", m["PICTO_CATEGORIES"])
	}
	s2, _ := newPictoServer(t)
	w = postPictoQuestion(t, s2, poolFields(map[string]string{"picto_source": "WORDS", "picto_words": ""}))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "PICTO_WORDS_INVALID") {
		t.Errorf("source WORDS sans mot : %d %q", w.Code, w.Body.String())
	}
}
