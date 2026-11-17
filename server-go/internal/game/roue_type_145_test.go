// #145 (v12.0.0, LOT-1.1) — type de question ROUE : registre et aller-retour JSON des champs ROUE_*.
// Source : contracts/roue.md §2 (décision 1), §3 ; contracts/question-types.md §7.
package game

import (
	"encoding/json"
	"testing"
)

func TestRoueType145_IsInAllQuestionTypes(t *testing.T) {
	found := false
	for _, qt := range AllQuestionTypes() {
		if string(qt) == "ROUE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ROUE doit figurer dans AllQuestionTypes() : %v", AllQuestionTypes())
	}
}

// Aller-retour question.json : aucun champ ROUE_* ne se perd (leçon #107, champ non relu = perte silencieuse).
func TestRoueType145_QuestionJSONRoundTrip(t *testing.T) {
	in := `{"ID":"r1","QUESTION":"x","ANSWER":"","TYPE":"ROUE","CATEGORY":"ANIMALS","POINTS":"","TIME":"",` +
		`"ROUE_PHRASE":"Combien pour ce chien dans la vitrine","ROUE_PRESENTATION":"Chanson de Line Renaud",` +
		`"ROUE_MODE":"CHACUN_SON_TOUR","ROUE_GAINS_RULE":"ALL_TEAMS","ROUE_WHEEL_LAYOUT":"arc","ROUE_SHOW_LETTERS":false}`
	var q Question
	if err := json.Unmarshal([]byte(in), &q); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	want := map[string]interface{}{
		"TYPE": "ROUE", "ROUE_PHRASE": "Combien pour ce chien dans la vitrine", "ROUE_PRESENTATION": "Chanson de Line Renaud",
		"ROUE_MODE": "CHACUN_SON_TOUR", "ROUE_GAINS_RULE": "ALL_TEAMS", "ROUE_WHEEL_LAYOUT": "arc", "ROUE_SHOW_LETTERS": false,
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v après aller-retour, attendu %v", k, m[k], v)
		}
	}
}

// Contrat §3 : les champs vides sont omis ; la phrase n'est jamais copiée dans ANSWER.
func TestRoueType145_OmitsEmptyFieldsAndNeverCopiesPhraseIntoAnswer(t *testing.T) {
	in := `{"ID":"r2","QUESTION":"x","ANSWER":"","TYPE":"ROUE","ROUE_PHRASE":"Bonjour"}`
	var q Question
	if err := json.Unmarshal([]byte(in), &q); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, _ := json.Marshal(q)
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	for _, k := range []string{"ROUE_PRESENTATION", "ROUE_MODE", "ROUE_GAINS_RULE", "ROUE_WHEEL_LAYOUT", "ROUE_SHOW_LETTERS"} {
		if _, present := m[k]; present {
			t.Errorf("%s absent en entrée doit rester omis (omitempty), got %v", k, m[k])
		}
	}
	if m["ANSWER"] != "" {
		t.Errorf("la phrase ne doit jamais être copiée dans ANSWER, got %v", m["ANSWER"])
	}
}

// Les champs ROUE_* d'une question d'un autre type ne contaminent pas son JSON.
func TestRoueType145_OtherTypesCarryNoRoueField(t *testing.T) {
	in := `{"ID":"s1","QUESTION":"Q ?","ANSWER":"R","TYPE":"SPEEDY","POINTS":"10","TIME":"30"}`
	var q Question
	_ = json.Unmarshal([]byte(in), &q)
	out, _ := json.Marshal(q)
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	for k := range m {
		if len(k) >= 5 && k[:5] == "ROUE_" {
			t.Errorf("champ %s inattendu sur une question SPEEDY", k)
		}
	}
}
