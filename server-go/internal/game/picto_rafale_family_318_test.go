package game

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// #318 (v13.1.0, LOT-3, T30) — registre RAFALE_PICTO, prédicat de famille IsPictoFamily (cas nommés à l'identique du
// JS : `isPictoFamily`), fixtures (round-trip, aucun champ parasite) — contracts/picto.md §17.1, §17.2, §17.12.
// Écrit depuis le contrat avec des littéraux (type `QuestionType("RAFALE_PICTO")`).

const rafalePictoType = QuestionType("RAFALE_PICTO")

var rafalePictoOwned = []string{
	"RAFALE_PICTO_SOURCE", "RAFALE_PICTO_WORDS", "RAFALE_PICTO_CATEGORIES", "RAFALE_PICTO_DIFFICULTIES",
	"RAFALE_PICTO_PASS_COST", "RAFALE_PICTO_PASS_LIMIT", "RAFALE_PICTO_POOL_END",
}

func TestRafalePicto_Registry_Descriptor(t *testing.T) {
	d, ok := TypeDescriptorFor(rafalePictoType)
	if !ok {
		t.Fatal("RAFALE_PICTO absent du registre")
	}
	got := append([]string(nil), d.OwnedFields...)
	want := append([]string(nil), rafalePictoOwned...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OwnedFields = %v, attendu les 7 champs RAFALE_PICTO_* %v", got, want)
	}
	if len(d.MediaSlots) != 0 {
		t.Errorf("aucun média : %v", d.MediaSlots)
	}
	if d.NestableInMotionCard || IsNestableInMotionCard(rafalePictoType) {
		t.Error("RAFALE_PICTO n'est pas imbriquable dans une carte MEMOTION")
	}
	if !d.HasPlayerInput {
		t.Error("HasPlayerInput = true (PICTO_STROKES vient d'une tablette)")
	}
	if d.DefaultPointsRule != "" {
		t.Errorf("DefaultPointsRule vide, got %q", d.DefaultPointsRule)
	}
}

func TestRafalePicto_Registry_InAllQuestionTypes_AndFieldsOwnedByOneTypeOnly(t *testing.T) {
	found := false
	for _, qt := range AllQuestionTypes() {
		if qt == rafalePictoType {
			found = true
		}
	}
	if !found {
		t.Error("RAFALE_PICTO absent de AllQuestionTypes()")
	}
	owner := map[string]QuestionType{}
	for _, qt := range AllQuestionTypes() {
		d, _ := TypeDescriptorFor(qt)
		for _, f := range d.OwnedFields {
			if prev, dup := owner[f]; dup && prev != qt {
				t.Errorf("le champ %s appartient à deux types : %s et %s (un champ n'appartient qu'à un type)", f, prev, qt)
			}
			owner[f] = qt
		}
	}
	for _, f := range rafalePictoOwned {
		if owner[f] != rafalePictoType {
			t.Errorf("%s appartient à %q, attendu RAFALE_PICTO", f, owner[f])
		}
	}
	for _, f := range []string{"PICTO_WORDS", "PICTO_SOURCE", "PICTO_CATEGORIES", "PICTO_DIFFICULTIES"} {
		if owner[f] != QuestionTypePicto {
			t.Errorf("%s doit rester à PICTO, got %q", f, owner[f])
		}
	}
}

func TestRafalePicto_UnknownTypeDoesNotFallBackToSpeedy(t *testing.T) {
	for _, bad := range []QuestionType{"RAFALE_PICTOS", "rafale_picto", "RAFALEPICTO", "PICTO_RAFALE"} {
		if _, ok := TypeDescriptorFor(bad); ok {
			t.Errorf("%q ne doit pas être un type connu", bad)
		}
		if IsNestableInMotionCard(bad) {
			t.Errorf("IsNestableInMotionCard(%q) doit être false", bad)
		}
		if IsPictoFamily(bad) {
			t.Errorf("IsPictoFamily(%q) doit être false", bad)
		}
	}
}

// Tableau « famille » : mêmes cas nommés côté JS (utils/pictoFamily — isPictoFamily).
func TestIsPictoFamily_Table(t *testing.T) {
	cases := []struct {
		name string
		in   QuestionType
		want bool
	}{
		{"picto", QuestionTypePicto, true},
		{"rafale_picto", rafalePictoType, true},
		{"rafale_speedy", QuestionTypeRafale, false},
		{"speedy", QuestionTypeSpeedy, false},
		{"qcm", QuestionTypeQCM, false},
		{"memory", QuestionTypeMemory, false},
		{"ardoise", QuestionTypeArdoise, false},
		{"roue", QuestionTypeRoue, false},
		{"entracte", QuestionTypeEntracte, false},
		{"empty", "", false},
		{"unknown", "PICTOS", false},
		{"lowercase", "picto", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsPictoFamily(c.in); got != c.want {
				t.Errorf("IsPictoFamily(%q) = %v, attendu %v", c.in, got, c.want)
			}
		})
	}
}

func marshalMap(t *testing.T, q Question) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}

func TestRafalePicto_Fixtures_NoParasiteField_OnOtherTypes(t *testing.T) {
	for _, in := range []string{
		`{"ID":"s","TYPE":"SPEEDY","QUESTION":"q","ANSWER":"a"}`,
		`{"ID":"p","TYPE":"PICTO","QUESTION":"q","PICTO_WORDS":["chat"],"TIME":"90"}`, // question v13.0.0
		`{"ID":"r","TYPE":"RAFALE","QUESTION":"q","ANSWER":"a"}`,
	} {
		var q Question
		if err := json.Unmarshal([]byte(in), &q); err != nil {
			t.Fatal(err)
		}
		m := marshalMap(t, q)
		for k := range m {
			if strings.HasPrefix(k, "RAFALE_PICTO_") {
				t.Errorf("champ parasite %s sur %s", k, in)
			}
		}
		if q.Type == QuestionTypePicto {
			if _, has := m["PICTO_SOURCE"]; has {
				t.Errorf("PICTO_SOURCE parasite sur une question v13.0.0 : %v", m["PICTO_SOURCE"])
			}
			if _, has := m["PICTO_CATEGORIES"]; has {
				t.Errorf("PICTO_CATEGORIES parasite")
			}
		}
	}
}

func TestRafalePicto_Fixtures_RoundTrip_KeepsAllSevenFields_WordsOrder(t *testing.T) {
	in := `{"ID":"rp","TYPE":"RAFALE_PICTO","QUESTION":"q","CATEGORY":"HISTORY","POINTS":"10","TIME":"120",` +
		`"RAFALE_PICTO_SOURCE":"WORDS","RAFALE_PICTO_WORDS":["Zèbre","chat 2","L'arbre"],` +
		`"RAFALE_PICTO_PASS_COST":15,"RAFALE_PICTO_PASS_LIMIT":1,"RAFALE_PICTO_POOL_END":"LOOP"}`
	var q Question
	if err := json.Unmarshal([]byte(in), &q); err != nil {
		t.Fatal(err)
	}
	m := marshalMap(t, q)
	if m["TYPE"] != "RAFALE_PICTO" || m["RAFALE_PICTO_SOURCE"] != "WORDS" || m["RAFALE_PICTO_POOL_END"] != "LOOP" {
		t.Errorf("round-trip : %v", m)
	}
	if !reflect.DeepEqual(m["RAFALE_PICTO_WORDS"], []interface{}{"Zèbre", "chat 2", "L'arbre"}) {
		t.Errorf("mots (ordre saisi conservé) : %v", m["RAFALE_PICTO_WORDS"])
	}
	if m["RAFALE_PICTO_PASS_COST"] != float64(15) || m["RAFALE_PICTO_PASS_LIMIT"] != float64(1) {
		t.Errorf("coût / limite : %v %v", m["RAFALE_PICTO_PASS_COST"], m["RAFALE_PICTO_PASS_LIMIT"])
	}
	// source RESERVOIR : filtre conservé, aucun mot écrit (omitempty)
	var r Question
	_ = json.Unmarshal([]byte(`{"ID":"rr","TYPE":"RAFALE_PICTO","QUESTION":"q","RAFALE_PICTO_SOURCE":"RESERVOIR","RAFALE_PICTO_CATEGORIES":["HISTORY"],"RAFALE_PICTO_DIFFICULTIES":[1,3]}`), &r)
	mr := marshalMap(t, r)
	if _, has := mr["RAFALE_PICTO_WORDS"]; has {
		t.Errorf("aucun mot en source RESERVOIR : %v", mr["RAFALE_PICTO_WORDS"])
	}
	if !reflect.DeepEqual(mr["RAFALE_PICTO_CATEGORIES"], []interface{}{"HISTORY"}) || !reflect.DeepEqual(mr["RAFALE_PICTO_DIFFICULTIES"], []interface{}{float64(1), float64(3)}) {
		t.Errorf("filtre RESERVOIR : %v", mr)
	}
}

func TestRafalePicto_Fixtures_DefaultsOmitted_PassLimitZeroMeansUnlimited(t *testing.T) {
	var q Question
	_ = json.Unmarshal([]byte(`{"ID":"rp","TYPE":"RAFALE_PICTO","QUESTION":"q","RAFALE_PICTO_SOURCE":"WORDS","RAFALE_PICTO_WORDS":["chat"]}`), &q)
	m := marshalMap(t, q)
	for _, k := range []string{"RAFALE_PICTO_PASS_COST", "RAFALE_PICTO_PASS_LIMIT", "RAFALE_PICTO_POOL_END"} {
		if _, has := m[k]; has {
			t.Errorf("%s à sa valeur par défaut doit être omis (omitempty, round-trip octet pour octet) : %v", k, m[k])
		}
	}
}
