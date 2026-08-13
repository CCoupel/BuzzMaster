package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: ClientsPayload.AnimCount (v6.2.0, #155 — Interface Animateur, tâche
// B2, contracts/websocket-endpoints.md).
//
// ANIM_COUNT is a NEW additive field — CLIENTS stays admin-only (unchanged),
// this only verifies the field itself round-trips under the exact wire name
// the contract fixes ("ANIM_COUNT"), symmetrical with ADMIN_COUNT/TV_COUNT/
// VPLAYER_COUNT/BUZZER_WS_COUNT already covered indirectly elsewhere.
// ---------------------------------------------------------------------------

func TestClientsPayload_AnimCount_JSONFieldName(t *testing.T) {
	payload := ClientsPayload{
		AdminCount:   1,
		TVCount:      2,
		VPlayerCount: 3,
		AnimCount:    4,
		BuzzerWS:     5,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal ClientsPayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal into raw map: %v", err)
	}

	got, ok := raw["ANIM_COUNT"]
	if !ok {
		t.Fatalf("ClientsPayload JSON is missing ANIM_COUNT key entirely: %s", data)
	}
	if got != float64(4) {
		t.Errorf("ANIM_COUNT = %v, want 4", got)
	}

	// The other fields must stay exactly as they were before this addition —
	// an additive field must never shift the wire name of a sibling.
	for key, want := range map[string]float64{
		"ADMIN_COUNT":     1,
		"TV_COUNT":        2,
		"VPLAYER_COUNT":   3,
		"BUZZER_WS_COUNT": 5,
	} {
		if raw[key] != want {
			t.Errorf("%s = %v, want %v (ANIM_COUNT addition must not disturb sibling fields)", key, raw[key], want)
		}
	}
}

func TestClientsPayload_AnimCount_RoundTrip(t *testing.T) {
	src := `{"ADMIN_COUNT":0,"TV_COUNT":0,"VPLAYER_COUNT":0,"ANIM_COUNT":7,"BUZZER_WS_COUNT":0}`
	var payload ClientsPayload
	if err := json.Unmarshal([]byte(src), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if payload.AnimCount != 7 {
		t.Errorf("AnimCount = %d, want 7", payload.AnimCount)
	}
}

func TestClientsPayload_AnimCount_ZeroValueOmittedIsStillPresent(t *testing.T) {
	// No omitempty on ClientsPayload fields (project convention, CLAUDE.md:
	// "No `omitempty` on GameState fields" — same discipline applies here so
	// a frontend badge reading MSG.ANIM_COUNT never sees `undefined` when the
	// count legitimately drops back to zero).
	payload := ClientsPayload{}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := raw["ANIM_COUNT"]; !ok {
		t.Error("ANIM_COUNT must be present (as 0) even when the count is zero — no omitempty")
	}
}
