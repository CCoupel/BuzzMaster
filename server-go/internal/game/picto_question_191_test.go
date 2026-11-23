package game

// PICTO (v13.0.0, #191) — contrat contracts/picto.md §3.1, §4, §16.
// Tests écrits depuis le contrat seul : ils ne dépendent d'aucun nom interne
// du moteur (seuls QuestionTypePicto, le registre et le JSON de Question).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPicto_TypeConstantAndAllQuestionTypes(t *testing.T) {
	if string(QuestionTypePicto) != "PICTO" {
		t.Fatalf("QuestionTypePicto = %q, want PICTO", QuestionTypePicto)
	}
	found := false
	for _, qt := range AllQuestionTypes() {
		if qt == QuestionTypePicto {
			found = true
		}
	}
	if !found {
		t.Error("AllQuestionTypes() must list PICTO (picto.md §4)")
	}
}

// Valeurs du registre — picto.md §4, tableau « Registre ».
func TestPicto_RegistryDescriptor(t *testing.T) {
	d, ok := TypeDescriptorFor(QuestionTypePicto)
	if !ok {
		t.Fatal("PICTO has no registry entry")
	}
	if d.Type != QuestionTypePicto {
		t.Errorf("descriptor.Type = %q, want PICTO", d.Type)
	}
	if !reflect.DeepEqual(d.OwnedFields, []string{"PICTO_WORDS"}) {
		t.Errorf("OwnedFields = %v, want [PICTO_WORDS]", d.OwnedFields)
	}
	if len(d.MediaSlots) != 0 {
		t.Errorf("MediaSlots = %v, want none (le mot est du texte)", d.MediaSlots)
	}
	if d.NestableInMotionCard {
		t.Error("PICTO must not be nestable in a MEMOTION card")
	}
	if !d.HasPlayerInput {
		t.Error("PICTO.HasPlayerInput must be true (PICTO_STROKES)")
	}
	if d.DefaultPointsRule != "" {
		t.Errorf("DefaultPointsRule = %q, want empty", d.DefaultPointsRule)
	}
	if IsNestableInMotionCard(QuestionTypePicto) {
		t.Error("IsNestableInMotionCard(PICTO) must be false")
	}
}

// #183 / picto.md §16 : un type inconnu ne retombe pas sur SPEEDY.
func TestPicto_UnknownTypeDoesNotFallBackToSpeedy(t *testing.T) {
	for _, bad := range []QuestionType{"PICTOS", "PICSONARY", "PICTIONARY", "picto"} {
		if _, ok := TypeDescriptorFor(bad); ok {
			t.Errorf("TypeDescriptorFor(%q) must be unknown", bad)
		}
		if IsNestableInMotionCard(bad) {
			t.Errorf("IsNestableInMotionCard(%q) must be false", bad)
		}
	}
}

// PICTO_WORDS est `omitempty` (picto.md §4) : aucun autre type ne le sérialise,
// et une liste de mots survit à l'aller-retour dans l'ordre saisi.
func TestPicto_WordsOmitEmptyAndRoundTrip(t *testing.T) {
	var speedy Question
	if err := json.Unmarshal([]byte(`{"ID":"s","TYPE":"SPEEDY","QUESTION":"q","ANSWER":"a"}`), &speedy); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(speedy)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	if _, present := m["PICTO_WORDS"]; present {
		t.Error("PICTO_WORDS must be omitted for a question without words (omitempty)")
	}

	var q Question
	in := `{"ID":"p","TYPE":"PICTO","QUESTION":"q","PICTO_WORDS":["Éléphant","chat 2","L'arbre"],"TIME":"90"}`
	if err := json.Unmarshal([]byte(in), &q); err != nil {
		t.Fatal(err)
	}
	out, err = json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	m = nil
	_ = json.Unmarshal(out, &m)
	got, _ := m["PICTO_WORDS"].([]interface{})
	want := []interface{}{"Éléphant", "chat 2", "L'arbre"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PICTO_WORDS round-trip = %v, want %v", got, want)
	}
	if m["TYPE"] != "PICTO" {
		t.Errorf("TYPE round-trip = %v, want PICTO", m["TYPE"])
	}
}

// La fixture PICTO est lue par le round-trip générique (picto.md §16) ; on
// vérifie ici qu'elle est conforme au §3.1 (1 à 20 mots, 1 à 40 caractères).
func TestPicto_FixtureWordsFollowContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "questions", "picto.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Type  string   `json:"TYPE"`
		Words []string `json:"PICTO_WORDS"`
		Time  string   `json:"TIME"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Type != "PICTO" || len(m.Words) < 1 || len(m.Words) > 20 {
		t.Fatalf("fixture picto.json invalid: type=%q words=%d", m.Type, len(m.Words))
	}
	for _, w := range m.Words {
		if n := len([]rune(w)); n < 1 || n > 40 {
			t.Errorf("word %q has %d chars, want 1..40", w, n)
		}
	}
}
