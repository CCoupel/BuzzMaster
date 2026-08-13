package protocol

// Regression tests for bugfix #80 — TCPServer legacy removal
// These tests ensure that dead TCP fields do not re-appear in the
// ClientsPayload wire format and that the serialization contract
// remains stable after the removal of the TCP server.

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClientsPayload_NoTCPField verifies that ClientsPayload JSON output
// does NOT contain BUZZER_TCP_COUNT (removed in bugfix #80).
// Regression guard: re-adding BuzzerTCP to the struct would break this test.
func TestClientsPayload_NoTCPField(t *testing.T) {
	payload := ClientsPayload{
		AdminCount:   1,
		TVCount:      2,
		VPlayerCount: 3,
		BuzzerWS:     4,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(ClientsPayload) failed: %v", err)
	}

	jsonStr := string(data)

	if strings.Contains(jsonStr, "BUZZER_TCP_COUNT") {
		t.Errorf("ClientsPayload JSON must NOT contain BUZZER_TCP_COUNT after bugfix #80, got: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "tcp") || strings.Contains(jsonStr, "TCP") {
		t.Errorf("ClientsPayload JSON must NOT contain any TCP field after bugfix #80, got: %s", jsonStr)
	}
}

// TestClientsPayload_ExpectedFields verifies that ClientsPayload JSON output
// contains exactly the expected fields (no more, no less).
//
// Field count bumped 4 -> 5 for v6.2.0 (#155, tâche B2): ANIM_COUNT is a new,
// deliberate additive field (interface animateur client count,
// contracts/websocket-endpoints.md) — not a regression of this guard's
// original bugfix #80 intent (no TCP fields reappearing), which
// TestClientsPayload_NoTCPField above still covers independently.
func TestClientsPayload_ExpectedFields(t *testing.T) {
	payload := ClientsPayload{
		AdminCount:   1,
		TVCount:      1,
		VPlayerCount: 1,
		AnimCount:    1,
		BuzzerWS:     1,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(ClientsPayload) failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedFields := []string{"ADMIN_COUNT", "TV_COUNT", "VPLAYER_COUNT", "ANIM_COUNT", "BUZZER_WS_COUNT"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("ClientsPayload JSON must contain field %q, got: %s", field, string(data))
		}
	}

	// Strict field count: exactly 5 fields expected (v6.2.0: +ANIM_COUNT)
	if len(decoded) != 5 {
		t.Errorf("ClientsPayload JSON must have exactly 5 fields, got %d: %s", len(decoded), string(data))
	}
}

// TestClientsPayload_ZeroValues verifies serialization with zero values
// (the default — as received by a client when no buzzers are connected).
func TestClientsPayload_ZeroValues(t *testing.T) {
	payload := ClientsPayload{}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(ClientsPayload{}) failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	for key, val := range decoded {
		if v, ok := val.(float64); !ok || v != 0 {
			t.Errorf("Field %q should be 0 in zero-value payload, got %v", key, val)
		}
	}
}
