package server

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #192 (v13.0.0, LOT-2.1) — NON-FUITE de PICTO_WORDS au niveau du ROUTAGE par type de client
// (contracts/picto.md §10) — CRITICAL. Modèle : roue_leak_routing_145_test.go. TV et animateur partagent le
// sérialiseur de base : le retrait vise la TV et les tablettes SANS priver l'animateur ni l'admin (§10 : « Les mots
// restent présents pour /ws/admin et /ws/anim »).

const routingPictoWord192 = "Éléphant"

func pictoUpdateMsg192(t *testing.T, phase string) *protocol.Message {
	t.Helper()
	q := game.Question{
		ID: "p1", Question: "Dessinez", Type: game.QuestionTypePicto,
		TypedContent: game.TypedContent{PictoWords: []string{routingPictoWord192, "Pont-levis"}},
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"GAME":    map[string]interface{}{"PHASE": phase, "QUESTION": q},
		"bumpers": map[string]interface{}{},
		"teams":   map[string]interface{}{},
	})
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	msg.Msg = raw
	return msg
}

func TestPictoLeakRouting192_WordsByClientType_AllPhases(t *testing.T) {
	cases := []struct {
		name      string
		ct        ClientType
		wantWords bool
	}{
		{"admin", ClientTypeAdmin, true},
		{"animateur", ClientTypeAnim, true},
		{"tv", ClientTypeTV, false},
		{"tablette", ClientTypeVPlayer, false},
	}
	for _, phase := range []string{"PREPARE", "READY", "COUNTDOWN", "STARTED", "PAUSED", "STOPPED", "REVEALED"} {
		for _, c := range cases {
			t.Run(phase+"/"+c.name, func(t *testing.T) {
				data, err := serializeForClientType(pictoUpdateMsg192(t, phase), c.ct)
				if err != nil {
					t.Fatalf("serializeForClientType: %v", err)
				}
				has := strings.Contains(string(data), routingPictoWord192)
				if has != c.wantWords {
					t.Errorf("client %s/%s : mots présents = %v, attendu %v — %s", c.name, phase, has, c.wantWords, data)
				}
			})
		}
	}
}

func TestPictoLeakRouting192_BuzzerNeverGetsWords(t *testing.T) {
	data, err := serializeForClientType(pictoUpdateMsg192(t, "STARTED"), ClientTypeBuzzer)
	if err != nil {
		t.Fatalf("serializeForClientType: %v", err)
	}
	if strings.Contains(string(data), routingPictoWord192) || strings.Contains(string(data), "PICTO_WORDS") {
		t.Errorf("les mots ne doivent jamais atteindre un buzzer : %s", data)
	}
}
