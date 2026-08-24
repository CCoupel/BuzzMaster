// Test for #184 B-B6 — CardScope, contract §9.1: MOTION_CARD_ID is
// optional and its absence must not appear on the wire (omitempty).
// Run: go test ./internal/protocol/... -run TestCardScope -v

package protocol

import (
	"encoding/json"
	"testing"
)

func TestCardScope_EmptyOmittedFromJSON(t *testing.T) {
	data, err := json.Marshal(CardScope{})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(data) != `{}` {
		t.Errorf("CardScope{} marshaled to %s, want {} (MOTION_CARD_ID must be omitempty)", data)
	}
}

func TestCardScope_PopulatedRoundTrips(t *testing.T) {
	data, err := json.Marshal(CardScope{MotionCardID: "mc-1"})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(data) != `{"MOTION_CARD_ID":"mc-1"}` {
		t.Errorf("CardScope{MotionCardID: \"mc-1\"} marshaled to %s", data)
	}

	var decoded CardScope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.MotionCardID != "mc-1" {
		t.Errorf("decoded.MotionCardID = %q, want mc-1", decoded.MotionCardID)
	}
}
