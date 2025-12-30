package protocol

import (
	"encoding/json"
	"testing"
)

func TestNewMessage_WithPayload(t *testing.T) {
	payload := map[string]string{"key": "value"}
	msg, err := NewMessage(ActionHello, payload)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", msg.Action)
	}

	if msg.TimeEvent == 0 {
		t.Error("TimeEvent should be set")
	}

	// Verify MSG contains the payload
	var result map[string]string
	if err := json.Unmarshal(msg.Msg, &result); err != nil {
		t.Fatalf("Failed to unmarshal MSG: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected MSG to contain key=value")
	}
}

func TestNewMessage_NilPayload(t *testing.T) {
	msg, err := NewMessage(ActionPing, nil)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Action != ActionPing {
		t.Errorf("Expected action PING, got %s", msg.Action)
	}

	// MSG should be empty object
	if string(msg.Msg) != "{}" {
		t.Errorf("Expected MSG to be {}, got %s", string(msg.Msg))
	}
}

func TestMessage_Serialize(t *testing.T) {
	msg := &Message{
		Action: ActionHello,
		ID:     "buzzer1",
	}

	data, err := msg.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Should end with newline and null terminator
	if len(data) < 2 {
		t.Fatal("Serialized data too short")
	}

	if data[len(data)-2] != '\n' {
		t.Error("Expected newline before null terminator")
	}

	if data[len(data)-1] != 0 {
		t.Error("Expected null terminator at end")
	}

	// Verify JSON content
	jsonPart := data[:len(data)-2]
	var parsed Message
	if err := json.Unmarshal(jsonPart, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized JSON: %v", err)
	}

	if parsed.Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", parsed.Action)
	}
}

func TestMessage_SerializeForWebSocket(t *testing.T) {
	msg := &Message{
		Action: ActionUpdate,
		ID:     "web-client",
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("SerializeForWebSocket failed: %v", err)
	}

	// Should NOT end with null terminator
	if len(data) > 0 && data[len(data)-1] == 0 {
		t.Error("WebSocket message should not have null terminator")
	}

	// Verify valid JSON
	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse WebSocket JSON: %v", err)
	}
}

func TestMessage_ParseButtonPayload(t *testing.T) {
	msg := &Message{
		Action: ActionButton,
		Msg:    json.RawMessage(`{"button":"A"}`),
	}

	payload, err := msg.ParseButtonPayload()
	if err != nil {
		t.Fatalf("ParseButtonPayload failed: %v", err)
	}

	if payload.Button != "A" {
		t.Errorf("Expected button A, got %s", payload.Button)
	}
}

func TestMessage_ParseButtonPayload_Invalid(t *testing.T) {
	msg := &Message{
		Action: ActionButton,
		Msg:    json.RawMessage(`invalid`),
	}

	_, err := msg.ParseButtonPayload()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestMessage_ParseHelloPayload(t *testing.T) {
	msg := &Message{
		Action: ActionHello,
		Msg:    json.RawMessage(`{"IP":"192.168.4.2","VERSION":"1.0","NAME":"Buzzer1","TEAM":"red"}`),
	}

	payload, err := msg.ParseHelloPayload()
	if err != nil {
		t.Fatalf("ParseHelloPayload failed: %v", err)
	}

	if payload.IP != "192.168.4.2" {
		t.Errorf("Expected IP 192.168.4.2, got %s", payload.IP)
	}

	if payload.Version != "1.0" {
		t.Errorf("Expected Version 1.0, got %s", payload.Version)
	}

	if payload.Name != "Buzzer1" {
		t.Errorf("Expected Name Buzzer1, got %s", payload.Name)
	}

	if payload.Team != "red" {
		t.Errorf("Expected Team red, got %s", payload.Team)
	}
}

func TestRoundTrip_TCP(t *testing.T) {
	// Create message
	original := &Message{
		Action: ActionButton,
		ID:     "buzzer-123",
		Msg:    json.RawMessage(`{"button":"B"}`),
	}

	// Serialize for TCP
	data, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Parse with Parser (TCP style)
	p := NewParser()
	p.Append(data)
	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	parsed := messages[0]
	if parsed.Action != original.Action {
		t.Errorf("Action mismatch: %s vs %s", parsed.Action, original.Action)
	}

	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: %s vs %s", parsed.ID, original.ID)
	}
}

func TestRoundTrip_WebSocket(t *testing.T) {
	// Create message
	original := &Message{
		Action: ActionUpdate,
		ID:     "web-client",
	}

	// Serialize for WebSocket
	data, err := original.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("SerializeForWebSocket failed: %v", err)
	}

	// Parse with ParseSingle (WebSocket style)
	parsed, err := ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}

	if parsed.Action != original.Action {
		t.Errorf("Action mismatch: %s vs %s", parsed.Action, original.Action)
	}
}

func TestAllActions_Defined(t *testing.T) {
	expectedActions := []string{
		ActionHello, ActionButton, ActionPong, ActionPing,
		ActionStart, ActionStop, ActionPause, ActionContinue,
		ActionUpdate, ActionUpdateTimer, ActionReset, ActionReady,
		ActionReveal, ActionQuestions, ActionPoints, ActionRemote,
		ActionFull, ActionRAZ, ActionReboot, ActionFSInfo, ActionDelete,
	}

	for _, action := range expectedActions {
		if action == "" {
			t.Error("Found empty action constant")
		}
	}

	// Verify count
	if len(expectedActions) != 21 {
		t.Errorf("Expected 21 actions, found %d", len(expectedActions))
	}
}
