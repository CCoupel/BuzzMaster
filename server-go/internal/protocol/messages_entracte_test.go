package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// ENTRACTE / ENTRACTE_CONFIG serialization by client type (v6.5.2, #119)
//
// Contract: contracts/ws-payload-serialization.md §"ENTRACTE / ENTRACTE_CONFIG"
// — the OPPOSITE shape from QUIZ_OBJECTIVES (messages_quiz_objectives_test.go):
// both fields must reach admin, tv (SerializeForWebClient) and player
// (SerializeForVPlayer falls back to SerializeForWebClient for non-PREPARE/
// READY phases; the GAME node is never reduced there either way), and must
// NOT reach buzzer (SerializeForBuzzer is an allow-list of PHASE/TIME/
// CURRENT_TIME only).
// ---------------------------------------------------------------------------

// buildEntracteUpdateMsg builds a realistic UPDATE message whose GAME node
// carries ENTRACTE/ENTRACTE_CONFIG alongside PHASE/TIME/CURRENT_TIME, so a
// test can tell "ENTRACTE specifically present/absent" apart from "the whole
// GAME node vanished or survived by accident".
func buildEntracteUpdateMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":        phase,
			"TIME":         int64(1234567890),
			"CURRENT_TIME": 12,
			"ENTRACTE":     true,
			"ENTRACTE_CONFIG": map[string]interface{}{
				"TITLE":           "ENTRACTE",
				"SUBTITLE":        "Retour dans 20mn",
				"IMAGE_IS_CUSTOM": false,
				"PANEL_SIZE":      65,
				"ANIM_PERIOD":     10,
				"ANIM_INTENSITY":  20,
			},
		},
		"bumpers": map[string]interface{}{
			"buzzer-1": map[string]interface{}{
				"NAME": "Buzzer1", "TEAM": "TeamA", "CONNECTED": true,
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildEntracteUpdateMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("buildEntracteUpdateMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

func TestSerializeForAdmin_EntractePresent(t *testing.T) {
	msg := buildEntracteUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if game["ENTRACTE"] != true {
		t.Errorf("SerializeForAdmin: expected ENTRACTE=true, got %v", game["ENTRACTE"])
	}
	if _, present := game["ENTRACTE_CONFIG"]; !present {
		t.Error("SerializeForAdmin: ENTRACTE_CONFIG should be present but is missing")
	}
}

func TestSerializeForWebClient_EntractePresent(t *testing.T) {
	msg := buildEntracteUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if game["ENTRACTE"] != true {
		t.Errorf("SerializeForWebClient (TV): expected ENTRACTE=true to reach TV, got %v", game["ENTRACTE"])
	}
	cfg, ok := game["ENTRACTE_CONFIG"].(map[string]interface{})
	if !ok {
		t.Fatalf("SerializeForWebClient (TV): expected ENTRACTE_CONFIG object, got %v", game["ENTRACTE_CONFIG"])
	}
	if cfg["TITLE"] != "ENTRACTE" || cfg["PANEL_SIZE"] != float64(65) {
		t.Errorf("SerializeForWebClient (TV): ENTRACTE_CONFIG fields not preserved, got %v", cfg)
	}
}

// TestSerializeForVPlayer_EntractePresent_ReducedPath covers PREPARE/READY
// specifically — SerializeForVPlayer's own reduced-GAME branch (contract
// vplayer-payload-filter.md §2), which parses GAME independently of
// SerializeForWebClient and could regress on its own without the
// non-reduced test above noticing.
func TestSerializeForVPlayer_EntractePresent_ReducedPath(t *testing.T) {
	for _, phase := range []string{"PREPARE", "READY"} {
		t.Run(phase, func(t *testing.T) {
			msg := buildEntracteUpdateMsg(t, phase)
			data, err := msg.SerializeForVPlayer("buzzer-1")
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}
			game := gameNodeOf(t, data)
			if game["ENTRACTE"] != true {
				t.Errorf("SerializeForVPlayer (%s): expected ENTRACTE=true to reach VPlayer, got %v", phase, game["ENTRACTE"])
			}
			if _, present := game["ENTRACTE_CONFIG"]; !present {
				t.Errorf("SerializeForVPlayer (%s): ENTRACTE_CONFIG should be present but is missing", phase)
			}
		})
	}
}

func TestSerializeForBuzzer_EntracteAbsent(t *testing.T) {
	msg := buildEntracteUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	minimal := parseMsgMap(t, data)
	if _, present := minimal["ENTRACTE"]; present {
		t.Errorf("SerializeForBuzzer: ENTRACTE must not reach buzzers (LEDs are server-driven), got present: %v", minimal)
	}
	if _, present := minimal["ENTRACTE_CONFIG"]; present {
		t.Errorf("SerializeForBuzzer: ENTRACTE_CONFIG must not reach buzzers, got present: %v", minimal)
	}
	// Sanity: the allow-listed fields DO survive — proves this isn't a
	// coincidental empty-output pass.
	if minimal["PHASE"] != "STOPPED" {
		t.Errorf("SerializeForBuzzer: expected PHASE to survive (allow-listed), got %v", minimal["PHASE"])
	}
}
