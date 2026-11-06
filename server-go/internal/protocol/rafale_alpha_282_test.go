// #282 (v11.4.0, LOT-2) — diffusion de RAFALE_CURRENT_LETTER / RAFALE_LETTERS (contrat rafale.md §15.5,
// ws-payload-serialization.md) : admin, TV, /anim et VPlayer les reçoivent, jamais les buzzers ;
// ANSWER / NEXT ne les accompagnent jamais. Frontière JSON uniquement.
package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

func tw282AlphaMsg(t *testing.T, action string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE": "STARTED", "TIME": int64(1234567890), "CURRENT_TIME": 12,
			"RAFALE_CURRENT_QUESTION": game.RafaleCurrent{ID: "r-042", Question: "Capitale de l'Australie ?", Category: "GEOGRAPHY", Difficulty: 2},
			"RAFALE_SUBPHASE":         "QUESTION",
			"RAFALE_CURRENT_LETTER":   "C",
			"RAFALE_LETTERS":          []string{"D", "M", "S"},
		},
		"bumpers": map[string]interface{}{},
		"teams":   map[string]interface{}{},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal : %v", err)
	}
	msg, err := NewMessage(action, nil)
	if err != nil {
		t.Fatalf("NewMessage : %v", err)
	}
	msg.Msg = raw
	return msg
}

func tw282AssertLetters(t *testing.T, who string, data []byte) {
	t.Helper()
	g := gameNodeOf(t, data)
	if g["RAFALE_CURRENT_LETTER"] != "C" {
		t.Errorf("%s doit recevoir RAFALE_CURRENT_LETTER=\"C\", got %v", who, g["RAFALE_CURRENT_LETTER"])
	}
	arr, ok := g["RAFALE_LETTERS"].([]interface{})
	if !ok || len(arr) != 3 {
		t.Errorf("%s doit recevoir RAFALE_LETTERS=[D M S], got %v", who, g["RAFALE_LETTERS"])
	}
	if strings.Contains(string(data), "Canberra") {
		t.Errorf("%s : la réponse ne doit jamais apparaître dans la charge utile", who)
	}
}

var tw282Actions = []string{ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer}

func TestRafaleAlpha282_Serialize_TV_Anim_ReceiveLetters(t *testing.T) {
	for _, action := range tw282Actions {
		t.Run(action, func(t *testing.T) {
			data, err := tw282AlphaMsg(t, action).SerializeForWebClient()
			if err != nil {
				t.Fatalf("SerializeForWebClient : %v", err)
			}
			tw282AssertLetters(t, "TV/anim", data)
		})
	}
}

func TestRafaleAlpha282_Serialize_VPlayer_ReceivesLetters(t *testing.T) {
	for _, action := range tw282Actions {
		t.Run(action, func(t *testing.T) {
			data, err := tw282AlphaMsg(t, action).SerializeForVPlayerCommon()
			if err != nil {
				t.Fatalf("SerializeForVPlayerCommon : %v", err)
			}
			tw282AssertLetters(t, "VPlayer (commun)", data)
		})
	}
	data, err := tw282AlphaMsg(t, ActionUpdate).SerializeForVPlayer("buzzer-1")
	if err != nil {
		t.Fatalf("SerializeForVPlayer : %v", err)
	}
	tw282AssertLetters(t, "VPlayer (identifié)", data)
}

func TestRafaleAlpha282_Serialize_Admin_ReceivesLetters(t *testing.T) {
	data, err := tw282AlphaMsg(t, ActionUpdate).SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin : %v", err)
	}
	tw282AssertLetters(t, "admin", data)
}

// Les buzzers physiques reçoivent un payload minimal (liste blanche) : jamais les lettres.
func TestRafaleAlpha282_Serialize_Buzzer_NeverReceivesLetters(t *testing.T) {
	data, err := tw282AlphaMsg(t, ActionUpdate).SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer : %v", err)
	}
	for _, k := range []string{"RAFALE_CURRENT_LETTER", "RAFALE_LETTERS"} {
		if strings.Contains(string(data), k) {
			t.Errorf("les buzzers ne doivent jamais recevoir %s : %s", k, data)
		}
	}
}

// §15.5 — la lettre est une seule lettre : jamais plus d'un caractère, jamais ANSWER/NEXT à côté.
func TestRafaleAlpha282_Serialize_LetterIsSingleCharacter_NoAnswerNode(t *testing.T) {
	data, err := tw282AlphaMsg(t, ActionUpdate).SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient : %v", err)
	}
	g := gameNodeOf(t, data)
	if l, _ := g["RAFALE_CURRENT_LETTER"].(string); len([]rune(l)) != 1 {
		t.Errorf("RAFALE_CURRENT_LETTER doit tenir en 1 caractère, got %q", l)
	}
	for _, k := range []string{"RAFALE_ANSWER", "RAFALE_NEXT", "NEXT", "ANSWER"} {
		if _, present := g[k]; present {
			t.Errorf("GAME.%s ne doit jamais atteindre TV/VPlayer", k)
		}
	}
}
