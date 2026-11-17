package server

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #145 (v12.0.0, LOT-1.1) — NON-FUITE de la phrase ROUE au niveau du ROUTAGE par type de client
// (contracts/roue.md §9) — CRITICAL. TV et animateur partageaient le même sérialiseur
// (serializeForClientType) : le retrait de ROUE_PHRASE doit viser la TV et les tablettes SANS priver
// l'animateur, qui conduit la manche. Un type de client oublié = fuite ou animateur aveugle.

const routingPhrase145 = "Combien pour ce chien dans la vitrine"

func roueUpdateMsg145(t *testing.T) *protocol.Message {
	t.Helper()
	q := game.Question{
		ID: "r1", Question: "Manche roue", Type: game.QuestionTypeRoue,
		TypedContent: game.TypedContent{RouePhrase: routingPhrase145},
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"GAME":    map[string]interface{}{"PHASE": "STARTED", "QUESTION": q},
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

func TestRoueLeakRouting145_PhraseByClientType(t *testing.T) {
	cases := []struct {
		name       string
		ct         ClientType
		wantPhrase bool
	}{
		{"admin", ClientTypeAdmin, true},
		{"animateur", ClientTypeAnim, true},
		{"tv", ClientTypeTV, false},
		{"tablette", ClientTypeVPlayer, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := serializeForClientType(roueUpdateMsg145(t), c.ct)
			if err != nil {
				t.Fatalf("serializeForClientType: %v", err)
			}
			has := strings.Contains(string(data), routingPhrase145)
			if has != c.wantPhrase {
				t.Errorf("client %s : phrase présente = %v, attendu %v — %s", c.name, has, c.wantPhrase, data)
			}
		})
	}
}

// Les buzzers ne reçoivent jamais la phrase (§9 : « aucun message aux buzzers »).
func TestRoueLeakRouting145_BuzzerNeverGetsThePhrase(t *testing.T) {
	data, err := serializeForClientType(roueUpdateMsg145(t), ClientTypeBuzzer)
	if err != nil {
		t.Fatalf("serializeForClientType: %v", err)
	}
	if strings.Contains(string(data), routingPhrase145) || strings.Contains(string(data), "ROUE_PHRASE") {
		t.Errorf("la phrase ne doit jamais atteindre un buzzer : %s", data)
	}
}
