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

// ---------------------------------------------------------------------------
// Tests: RegieMessagePayload (v6.4.x, #167 — plan tâche B1/T7,
// contracts/websocket-actions.md §"Messagerie régie" → REGIE_MESSAGE).
//
// The four fields (ACTIVE/TEXT/SENT_AT/CLEARED_BY) deliberately carry no
// `omitempty` — same discipline as GameState (CLAUDE.md) and ClientsPayload
// above: an `ACTIVE: false` silently omitted from the wire would leave the
// frontend displaying an already-cleared message forever, because it would
// never receive the explicit "it's gone" signal.
// ---------------------------------------------------------------------------

func TestRegieMessagePayload_JSONFieldNames(t *testing.T) {
	payload := RegieMessagePayload{
		Active:    true,
		Text:      "Question 12 annulée",
		SentAt:    1755511234567,
		ClearedBy: "",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal RegieMessagePayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal into raw map: %v", err)
	}

	for key, want := range map[string]interface{}{
		"ACTIVE":     true,
		"TEXT":       "Question 12 annulée",
		"SENT_AT":    float64(1755511234567),
		"CLEARED_BY": "",
	} {
		got, ok := raw[key]
		if !ok {
			t.Errorf("RegieMessagePayload JSON is missing %q entirely: %s", key, data)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func TestRegieMessagePayload_RoundTrip(t *testing.T) {
	src := `{"ACTIVE":true,"TEXT":"Vu par l'animateur ?","SENT_AT":42,"CLEARED_BY":"ANIM"}`
	var payload RegieMessagePayload
	if err := json.Unmarshal([]byte(src), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !payload.Active || payload.Text != "Vu par l'animateur ?" || payload.SentAt != 42 || payload.ClearedBy != "ANIM" {
		t.Errorf("round-trip mismatch: got %+v", payload)
	}
}

// TestRegieMessagePayload_InactiveZeroValue_NoOmitempty is the contract's own
// example of the "après acquittement" wire frame: every field must still be
// PRESENT (not omitted) even at its zero value, because the absence of
// ACTIVE/TEXT/SENT_AT/CLEARED_BY is indistinguishable from "field not
// updated" on the frontend — the explicit `false`/`""`/`0` IS the
// information "this message was just cleared".
func TestRegieMessagePayload_InactiveZeroValue_NoOmitempty(t *testing.T) {
	payload := RegieMessagePayload{} // zero value: Active=false, Text="", SentAt=0, ClearedBy=""

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	for _, key := range []string{"ACTIVE", "TEXT", "SENT_AT", "CLEARED_BY"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("%s must be present in the JSON even at its zero value (no omitempty) — got: %s", key, data)
		}
	}
}
