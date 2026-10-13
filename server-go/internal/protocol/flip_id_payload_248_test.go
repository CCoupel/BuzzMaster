package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// #248 lot 5b — la charge entrante de FLIP_MEMORY_CARD porte FLIP_ID (contrat websocket-actions.md §FLIP_MEMORY_CARD,
// champ optionnel, `omitempty`). Lu par réflexion sur la balise JSON : le test compile avant le correctif.

func TestFlipMemoryCardPayload_HasOptionalFlipIDField_248(t *testing.T) {
	typ := reflect.TypeOf(FlipMemoryCardPayload{})
	var found *reflect.StructField
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if strings.Split(f.Tag.Get("json"), ",")[0] == "FLIP_ID" {
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatal("FlipMemoryCardPayload doit porter un champ JSON FLIP_ID (string, optionnel)")
	}
	if found.Type.Kind() != reflect.String {
		t.Errorf("FLIP_ID doit être une chaîne, a %s", found.Type)
	}
	if !strings.Contains(found.Tag.Get("json"), "omitempty") {
		t.Errorf("FLIP_ID doit être omitempty (absent = comportement actuel), balise : %q", found.Tag.Get("json"))
	}
}

func TestFlipMemoryCardPayload_UnchangedWithoutFlipID_248(t *testing.T) {
	// non-régression : une charge sans FLIP_ID se lit comme avant et ne fabrique pas de FLIP_ID à la ré-émission
	var p FlipMemoryCardPayload
	if err := json.Unmarshal([]byte(`{"CARD_ID":"1-1","ID":"p1","MOTION_CARD_ID":"card-3"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.CardID != "1-1" || p.ID != "p1" || p.MotionCardID != "card-3" {
		t.Errorf("lecture inchangée attendue, a %+v", p)
	}
	out, _ := json.Marshal(p)
	if strings.Contains(string(out), "FLIP_ID") {
		t.Errorf("sans FLIP_ID en entrée, aucun FLIP_ID ne doit apparaître à la sérialisation : %s", out)
	}
}
