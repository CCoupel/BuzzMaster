// Test for #184 B-B5 — MotionDonePayload.UNITS, contract §9.3: optional,
// *int so "absent" and "explicit 0" decode differently.
// Run: go test ./internal/protocol/... -run TestMotionDonePayload_Units -v

package protocol

import (
	"encoding/json"
	"testing"
)

func TestMotionDonePayload_Units_AbsentIsNil(t *testing.T) {
	var p MotionDonePayload
	if err := json.Unmarshal([]byte(`{"CARD_ID":"mc-1","WINNER_TEAM":"red"}`), &p); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if p.Units != nil {
		t.Errorf("Units = %v, want nil when UNITS is absent from the payload", p.Units)
	}
}

func TestMotionDonePayload_Units_ExplicitZeroIsNotNil(t *testing.T) {
	var p MotionDonePayload
	if err := json.Unmarshal([]byte(`{"CARD_ID":"mc-1","WINNER_TEAM":"red","UNITS":0}`), &p); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if p.Units == nil {
		t.Fatal("Units = nil, want a non-nil pointer to 0 when UNITS:0 is explicit in the payload")
	}
	if *p.Units != 0 {
		t.Errorf("*Units = %d, want 0", *p.Units)
	}
}

func TestMotionDonePayload_Units_ExplicitPositive(t *testing.T) {
	var p MotionDonePayload
	if err := json.Unmarshal([]byte(`{"CARD_ID":"mc-1","WINNER_TEAM":"red","UNITS":3}`), &p); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if p.Units == nil || *p.Units != 3 {
		t.Errorf("Units = %v, want *3", p.Units)
	}
}

func TestMotionDonePayload_Units_OmitemptyOnMarshalWhenNil(t *testing.T) {
	p := MotionDonePayload{CardID: "mc-1", WinnerTeam: "red"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, exists := m["UNITS"]; exists {
		t.Errorf("UNITS should be absent from the marshaled payload when nil, got: %s", data)
	}
}
