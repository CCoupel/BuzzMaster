package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// ENTRACTE_CONFIG_SAVED serialization (delta C4, #119, plan
// _work/reports/plan-entracte-119-fixes-20260820-155123.md, C4-B2) — the
// OPPOSITE shape from ENTRACTE_CONFIG (messages_entracte_test.go, which
// reaches admin/tv/player): ENTRACTE_CONFIG_SAVED is admin-only, same
// restriction as QUIZ_OBJECTIVES (AdminOnlyGameFields), because only the
// Quiz page's edit form needs the ALWAYS-current saved value — TV/VJoueur/
// anim only ever need the (possibly frozen) diffused ENTRACTE_CONFIG.
//
// New file rather than extending messages_entracte_test.go directly (owned
// by dev-backend, actively edited in parallel for this same delta) — same
// buildEntracteUpdateMsg-style helper, scoped to this one field, avoiding a
// concurrent-edit collision on a shared file.
// ---------------------------------------------------------------------------

func buildEntracteConfigSavedUpdateMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":        phase,
			"TIME":         int64(1234567890),
			"CURRENT_TIME": 12,
			"ENTRACTE":     false,
			"ENTRACTE_CONFIG": map[string]interface{}{
				"TITLE": "ENTRACTE", "PANEL_SIZE": 65,
			},
			"ENTRACTE_CONFIG_SAVED": map[string]interface{}{
				"TITLE": "Config enregistrée — jamais diffusée telle quelle", "PANEL_SIZE": 80,
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
		t.Fatalf("buildEntracteConfigSavedUpdateMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("buildEntracteConfigSavedUpdateMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

func TestSerializeForAdmin_EntracteConfigSavedPresent(t *testing.T) {
	msg := buildEntracteConfigSavedUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	game := gameNodeOf(t, data)
	cfg, ok := game["ENTRACTE_CONFIG_SAVED"].(map[string]interface{})
	if !ok {
		t.Fatalf("SerializeForAdmin: expected ENTRACTE_CONFIG_SAVED object, got %v", game["ENTRACTE_CONFIG_SAVED"])
	}
	if cfg["PANEL_SIZE"] != float64(80) {
		t.Errorf("SerializeForAdmin: ENTRACTE_CONFIG_SAVED fields not preserved, got %v", cfg)
	}
}

func TestSerializeForWebClient_EntracteConfigSavedAbsent(t *testing.T) {
	msg := buildEntracteConfigSavedUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if _, present := game["ENTRACTE_CONFIG_SAVED"]; present {
		t.Errorf("SerializeForWebClient (TV/anim): ENTRACTE_CONFIG_SAVED must NOT reach tv/anim (admin-only, C4), got present: %v", game)
	}
	// Sanity: ENTRACTE_CONFIG (the diffused, non-restricted field) DOES
	// survive — proves this isn't a coincidental empty-GAME-node pass.
	if _, present := game["ENTRACTE_CONFIG"]; !present {
		t.Error("SerializeForWebClient: sanity check failed — ENTRACTE_CONFIG (unrestricted) unexpectedly absent too")
	}
}

func TestSerializeForVPlayer_EntracteConfigSavedAbsent_ReducedPath(t *testing.T) {
	for _, phase := range []string{"PREPARE", "READY"} {
		t.Run(phase, func(t *testing.T) {
			msg := buildEntracteConfigSavedUpdateMsg(t, phase)
			data, err := msg.SerializeForVPlayer("buzzer-1")
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}
			game := gameNodeOf(t, data)
			if _, present := game["ENTRACTE_CONFIG_SAVED"]; present {
				t.Errorf("SerializeForVPlayer (%s): ENTRACTE_CONFIG_SAVED must NOT reach VJoueur, got present: %v", phase, game)
			}
			if _, present := game["ENTRACTE_CONFIG"]; !present {
				t.Errorf("SerializeForVPlayer (%s): sanity check failed — ENTRACTE_CONFIG unexpectedly absent too", phase)
			}
		})
	}
}

func TestSerializeForBuzzer_EntracteConfigSavedAbsent(t *testing.T) {
	msg := buildEntracteConfigSavedUpdateMsg(t, "STOPPED")

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	minimal := parseMsgMap(t, data)
	if _, present := minimal["ENTRACTE_CONFIG_SAVED"]; present {
		t.Errorf("SerializeForBuzzer: ENTRACTE_CONFIG_SAVED must not reach buzzers, got present: %v", minimal)
	}
}
