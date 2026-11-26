package server

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// #318 (v13.1.0, LOT-3, T30) — upload multipart du type RAFALE_PICTO (picto.md §17.2) : le handler recrée la question
// de zéro, donc chaque champ `rafale_picto_*` lu DOIT être réécrit (round-trip). Refus : PICTO_SOURCE_INVALID,
// PICTO_FILTER_INVALID, PICTO_WORDS_INVALID, PICTO_PASS_INVALID, PICTO_POOL_END_INVALID (mêmes codes que Picto).

func rpFields(extra map[string]string) map[string]string {
	f := map[string]string{
		"question": "Faites deviner", "type": "RAFALE_PICTO", "category": "HISTORY", "points": "10", "time": "120",
		"rafale_picto_source": "WORDS", "rafale_picto_words": "chat\nchien\nlion",
		"rafale_picto_pass_cost": "5", "rafale_picto_pass_limit": "1", "rafale_picto_pool_end": "LOOP",
	}
	for k, v := range extra {
		if v == "-" {
			delete(f, k)
			continue
		}
		f[k] = v
	}
	return f
}

func TestRafalePictoUpload_Words_RoundTrip_AllFieldsPersisted(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, rpFields(nil))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if m["TYPE"] != "RAFALE_PICTO" {
		t.Errorf("TYPE = %v", m["TYPE"])
	}
	if !reflect.DeepEqual(m["RAFALE_PICTO_WORDS"], []interface{}{"chat", "chien", "lion"}) {
		t.Errorf("RAFALE_PICTO_WORDS = %v", m["RAFALE_PICTO_WORDS"])
	}
	if m["RAFALE_PICTO_SOURCE"] != "WORDS" || m["RAFALE_PICTO_POOL_END"] != "LOOP" {
		t.Errorf("source / fin de pool : %v %v", m["RAFALE_PICTO_SOURCE"], m["RAFALE_PICTO_POOL_END"])
	}
	if m["RAFALE_PICTO_PASS_COST"] != float64(5) || m["RAFALE_PICTO_PASS_LIMIT"] != float64(1) {
		t.Errorf("coût / limite : %v %v", m["RAFALE_PICTO_PASS_COST"], m["RAFALE_PICTO_PASS_LIMIT"])
	}
	if got := toStr(m["TIME"]); got != "120" {
		t.Errorf("TIME = %v", m["TIME"])
	}
	if got := toStr(m["POINTS"]); got != "10" {
		t.Errorf("POINTS (par mot trouvé) = %v", m["POINTS"])
	}
	for _, k := range []string{"PICTO_WORDS", "PICTO_SOURCE", "PICTO_CATEGORIES", "PICTO_DIFFICULTIES"} {
		if _, has := m[k]; has {
			t.Errorf("champ du type PICTO %s écrit sur une question RAFALE_PICTO", k)
		}
	}
}

func TestRafalePictoUpload_Reservoir_FilterPersisted_NoWordsWritten(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, rpFields(map[string]string{
		"rafale_picto_source": "RESERVOIR", "rafale_picto_categories": `["HISTORY","SCIENCE"]`, "rafale_picto_difficulties": `[1,2]`,
		"rafale_picto_words": "-",
	}))
	if w.Code != http.StatusOK {
		t.Fatalf("POST : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	if m["RAFALE_PICTO_SOURCE"] != "RESERVOIR" {
		t.Errorf("source = %v", m["RAFALE_PICTO_SOURCE"])
	}
	if !reflect.DeepEqual(m["RAFALE_PICTO_CATEGORIES"], []interface{}{"HISTORY", "SCIENCE"}) ||
		!reflect.DeepEqual(m["RAFALE_PICTO_DIFFICULTIES"], []interface{}{float64(1), float64(2)}) {
		t.Errorf("filtre : %v %v", m["RAFALE_PICTO_CATEGORIES"], m["RAFALE_PICTO_DIFFICULTIES"])
	}
	if _, has := m["RAFALE_PICTO_WORDS"]; has {
		t.Errorf("aucun mot écrit avec la source RESERVOIR")
	}
}

func TestRafalePictoUpload_Refusals_400_WithContractCodes(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		code  string
	}{
		{"coût négatif", map[string]string{"rafale_picto_pass_cost": "-1"}, "PICTO_PASS_INVALID"},
		{"coût > 100", map[string]string{"rafale_picto_pass_cost": "101"}, "PICTO_PASS_INVALID"},
		{"coût non numérique", map[string]string{"rafale_picto_pass_cost": "x"}, "PICTO_PASS_INVALID"},
		{"limite 2 (seulement 0 ou 1)", map[string]string{"rafale_picto_pass_limit": "2"}, "PICTO_PASS_INVALID"},
		{"limite négative", map[string]string{"rafale_picto_pass_limit": "-1"}, "PICTO_PASS_INVALID"},
		{"fin de pool inconnue", map[string]string{"rafale_picto_pool_end": "FOREVER"}, "PICTO_POOL_END_INVALID"},
		{"source inconnue", map[string]string{"rafale_picto_source": "AUTRE"}, "PICTO_SOURCE_INVALID"},
		{"mots vides en source WORDS", map[string]string{"rafale_picto_words": ""}, "PICTO_WORDS_INVALID"},
		{"mot invalide", map[string]string{"rafale_picto_words": "chat<3"}, "PICTO_WORDS_INVALID"},
		{"RESERVOIR sans catégorie", map[string]string{"rafale_picto_source": "RESERVOIR", "rafale_picto_categories": "[]", "rafale_picto_difficulties": "[1]", "rafale_picto_words": "-"}, "PICTO_FILTER_INVALID"},
		{"RESERVOIR difficulté 4", map[string]string{"rafale_picto_source": "RESERVOIR", "rafale_picto_categories": `["HISTORY"]`, "rafale_picto_difficulties": "[4]", "rafale_picto_words": "-"}, "PICTO_FILTER_INVALID"},
		{"durée hors 30–300", map[string]string{"time": "10"}, "PICTO_TIME_INVALID"},
	}
	for _, c := range cases {
		s, _ := newPictoServer(t)
		w := postPictoQuestion(t, s, rpFields(c.extra))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.code) {
			t.Errorf("%s : %d %q, attendu 400 %s", c.name, w.Code, strings.TrimSpace(w.Body.String()), c.code)
		}
	}
}

func TestRafalePictoUpload_Defaults_PassLimitZeroIsUnlimited_PoolEndAbsentIsEND(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, rpFields(map[string]string{"rafale_picto_pass_cost": "-", "rafale_picto_pass_limit": "-", "rafale_picto_pool_end": "-"}))
	if w.Code != http.StatusOK {
		t.Fatalf("POST sans coût / limite / fin de pool : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	for _, k := range []string{"RAFALE_PICTO_PASS_COST", "RAFALE_PICTO_PASS_LIMIT", "RAFALE_PICTO_POOL_END"} {
		if v, has := m[k]; has && v != float64(0) && v != "END" && v != "" {
			t.Errorf("%s = %v : valeur par défaut attendue (0 / END) ou champ omis", k, v)
		}
	}
}

// non-régression : le type PICTO v13.0.0 n'est pas affecté
func TestRafalePictoUpload_NonRegression_PictoOneWordUnchanged(t *testing.T) {
	s, dataDir := newPictoServer(t)
	w := postPictoQuestion(t, s, pictoFields("chat\nchien"))
	if w.Code != http.StatusOK {
		t.Fatalf("POST Picto : %d %s", w.Code, w.Body.String())
	}
	m := readOnlyQuestionJSON(t, dataDir)
	for k := range m {
		if strings.HasPrefix(k, "RAFALE_PICTO_") {
			t.Errorf("champ %s parasite sur une question PICTO", k)
		}
	}
}
