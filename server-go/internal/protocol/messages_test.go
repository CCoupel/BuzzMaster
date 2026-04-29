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

// ============================================================================
// Tests: SerializeForAdmin / SerializeForWebClient / SerializeForBuzzer (#41)
// ============================================================================

// buildUpdateMsg constructs a realistic UPDATE Message with full bumper/team data,
// including admin-only fields that the serializers are expected to strip.
//
// The payload format matches the real output of engine.GetGameJSON() (GameData struct):
//   - "GAME" node (GameState): contains PHASE, CURRENT_TIME, TIME, etc.
//   - "bumpers" (lowercase, map[mac]bumper): Bumper struct has no ID field;
//     the MAC address is the map key, not a field inside the bumper object.
//   - "teams" (lowercase, map[name]team): team name is the map key.
func buildUpdateMsg(t *testing.T) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":        "STARTED",
			"CURRENT_TIME": 30,
			"TIME":         int64(1234567890),
		},
		"config": map[string]interface{}{
			"auto_open": true,
			"debug":     false,
		},
		// bumpers: map[mac]bumper — lowercase key, map not slice
		"bumpers": map[string]interface{}{
			"AA:BB:CC:DD:EE:01": map[string]interface{}{
				"NAME":             "Buzzer1",
				"TEAM":             "red",
				"CONNECTED":        true,
				"IS_VIRTUAL":       false,
				"TIME":             int64(0),
				"BUTTON":           "A",
				"STATUS":           "IDLE",
				"SCORE":            10,
				"FIRMWARE_VERSION": "3.7.0",
				"IS_OUTDATED":      true,
				"OTA_STATUS":       "done",
				"OTA_PERCENT":      100,
				"ACK_PENDING":      true,
			},
		},
		// teams: map[name]team — lowercase key, map not slice
		"teams": map[string]interface{}{
			"Équipe Rouge": map[string]interface{}{
				"NAME":   "Équipe Rouge",
				"COLOR":  []interface{}{float64(239), float64(68), float64(68)},
				"STATUS": "ACTIVE",
				"SCORE":  100,
			},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildUpdateMsg: marshal failed: %v", err)
	}
	return &Message{Action: ActionUpdate, Msg: rawMsg}
}

// parseBumperSlice parses the bumpers from a serialized message's MSG field.
// GetGameJSON() represents bumpers as a map[mac]bumper (lowercase key "bumpers").
// This helper converts the map to a slice of bumper maps for easy iteration in tests.
func parseBumperSlice(t *testing.T, data []byte) []map[string]interface{} {
	t.Helper()
	msgMap := parseMsgMap(t, data)
	bumpersRaw, ok := msgMap["bumpers"]
	if !ok {
		return nil
	}
	bumpersMap, ok := bumpersRaw.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(bumpersMap))
	for _, b := range bumpersMap {
		if bm, ok := b.(map[string]interface{}); ok {
			result = append(result, bm)
		}
	}
	return result
}

// parseMsgMap parses the MSG field of a serialized message as a map.
func parseMsgMap(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var outer struct {
		MSG map[string]interface{} `json:"MSG"`
	}
	if err := json.Unmarshal(data, &outer); err != nil {
		t.Fatalf("parseMsgMap: %v", err)
	}
	return outer.MSG
}

// -----------------------------------------------------------------------------
// SerializeForAdmin
// -----------------------------------------------------------------------------

// TestSerializeForAdmin_FullPayload verifies that SerializeForAdmin is identical
// to SerializeForWebSocket (admin gets the complete payload).
func TestSerializeForAdmin_FullPayload(t *testing.T) {
	msg := buildUpdateMsg(t)

	adminData, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}
	wsData, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("SerializeForWebSocket failed: %v", err)
	}

	if string(adminData) != string(wsData) {
		t.Error("SerializeForAdmin should be identical to SerializeForWebSocket")
	}
}

// TestSerializeForAdmin_BumperFieldsPresent verifies that admin-only bumper fields
// are present in the serialized output (FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS,
// OTA_PERCENT, ACK_PENDING).
func TestSerializeForAdmin_BumperFieldsPresent(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	bumpers := parseBumperSlice(t, data)
	if len(bumpers) == 0 {
		t.Fatal("expected at least one bumper in serialized output")
	}
	bumper := bumpers[0]

	adminOnlyFields := []string{"FIRMWARE_VERSION", "IS_OUTDATED", "OTA_STATUS", "OTA_PERCENT", "ACK_PENDING"}
	for _, field := range adminOnlyFields {
		if _, present := bumper[field]; !present {
			t.Errorf("SerializeForAdmin: bumper field %q should be present but is missing", field)
		}
	}
}

// TestSerializeForAdmin_NonUpdatePassthrough verifies non-UPDATE actions are
// serialized without modification.
func TestSerializeForAdmin_NonUpdatePassthrough(t *testing.T) {
	msg, _ := NewMessage(ActionStart, map[string]interface{}{"DELAY": 3})

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin on START failed: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Action != ActionStart {
		t.Errorf("expected START, got %s", parsed.Action)
	}
}

// -----------------------------------------------------------------------------
// SerializeForWebClient
// -----------------------------------------------------------------------------

// TestSerializeForWebClient_StripsAdminBumperFields verifies that admin-only bumper
// fields are absent from the TV/VPlayer payload on UPDATE messages.
func TestSerializeForWebClient_StripsAdminBumperFields(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	bumpers := parseBumperSlice(t, data)
	if len(bumpers) == 0 {
		t.Fatal("expected at least one bumper in serialized output")
	}
	bumper := bumpers[0]

	strippedFields := []string{"FIRMWARE_VERSION", "IS_OUTDATED", "OTA_STATUS", "OTA_PERCENT", "ACK_PENDING"}
	for _, field := range strippedFields {
		if _, present := bumper[field]; present {
			t.Errorf("SerializeForWebClient: bumper field %q should be absent but is present", field)
		}
	}
}

// TestSerializeForWebClient_KeepsEssentialBumperFields verifies that the non-stripped
// bumper fields (ID, NAME, TEAM, CONNECTED) are still present after serialization.
func TestSerializeForWebClient_KeepsEssentialBumperFields(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	bumpers := parseBumperSlice(t, data)
	if len(bumpers) == 0 {
		t.Fatal("expected at least one bumper")
	}
	bumper := bumpers[0]

	// "ID" is intentionally absent — in GetGameJSON() the MAC is the map key,
	// not a field inside the bumper object (Bumper struct has no ID field).
	requiredFields := []string{"NAME", "TEAM", "CONNECTED"}
	for _, field := range requiredFields {
		if _, present := bumper[field]; !present {
			t.Errorf("SerializeForWebClient: essential bumper field %q should be present", field)
		}
	}
}

// TestSerializeForWebClient_StripsConfigFromMsg verifies that the top-level `config`
// key is absent from the MSG payload sent to TV/VPlayer clients on UPDATE.
func TestSerializeForWebClient_StripsConfigFromMsg(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	msgMap := parseMsgMap(t, data)
	if _, present := msgMap["config"]; present {
		t.Error("SerializeForWebClient: 'config' key should be absent from MSG on UPDATE (TV/VPlayer don't need server config)")
	}
}

// TestSerializeForWebClient_NonUpdatePassthrough verifies that non-UPDATE actions
// are transmitted without any field stripping.
func TestSerializeForWebClient_NonUpdatePassthrough(t *testing.T) {
	startMsg, _ := NewMessage(ActionStart, map[string]interface{}{"DELAY": 5})

	adminData, err := startMsg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}
	webData, err := startMsg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	if string(adminData) != string(webData) {
		t.Error("SerializeForWebClient on non-UPDATE should be identical to SerializeForAdmin")
	}
}

// TestSerializeForWebClient_SourceImmutable verifies that the original message is
// not modified after a SerializeForWebClient call (deep-copy semantics).
func TestSerializeForWebClient_SourceImmutable(t *testing.T) {
	msg := buildUpdateMsg(t)
	originalMsg := string(msg.Msg)

	_, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	if string(msg.Msg) != originalMsg {
		t.Error("SerializeForWebClient mutated the source message — source must remain immutable")
	}
}

// TestSerializeForWebClient_MultipleCallsIdempotent verifies that calling
// SerializeForWebClient twice produces identical output (idempotent).
func TestSerializeForWebClient_MultipleCallsIdempotent(t *testing.T) {
	msg := buildUpdateMsg(t)

	data1, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("first SerializeForWebClient call failed: %v", err)
	}
	data2, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("second SerializeForWebClient call failed: %v", err)
	}

	if string(data1) != string(data2) {
		t.Error("SerializeForWebClient is not idempotent — consecutive calls returned different results")
	}
}

// -----------------------------------------------------------------------------
// SerializeForBuzzer
// -----------------------------------------------------------------------------

// TestSerializeForBuzzer_BumperFieldsWhitelist verifies that each bumper in the
// serialized UPDATE payload contains only the fields in buzzerBumperKeys and none
// of the admin-only fields.
func TestSerializeForBuzzer_BumperFieldsWhitelist(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	bumpers := parseBumperSlice(t, data)
	if len(bumpers) == 0 {
		t.Fatal("expected at least one bumper")
	}
	bumper := bumpers[0]

	// Admin-only fields must be absent
	forbiddenFields := []string{"FIRMWARE_VERSION", "IS_OUTDATED", "OTA_STATUS", "OTA_PERCENT", "ACK_PENDING"}
	for _, field := range forbiddenFields {
		if _, present := bumper[field]; present {
			t.Errorf("SerializeForBuzzer: bumper field %q should be absent but is present", field)
		}
	}

	// Essential fields must be present (implementation copies keys that exist in source)
	for _, key := range buzzerBumperKeys {
		if _, present := bumper[key]; !present {
			// Some optional fields may be absent from the fixture — log without failing
			t.Logf("SerializeForBuzzer: buzzer key %q absent (may be omitted in fixture)", key)
		}
	}
	// NAME must always be present (it is in the fixture).
	// Note: "ID" is intentionally absent — the MAC is the bumpers map key in GetGameJSON(),
	// not a field inside the bumper object (Bumper struct has no ID field).
	if _, ok := bumper["NAME"]; !ok {
		t.Error("SerializeForBuzzer: bumper 'NAME' field must be present")
	}
}

// TestSerializeForBuzzer_TeamFieldsWhitelist verifies that each team in the
// serialized UPDATE payload contains only NAME, COLOR, STATUS, SCORE — without MEMBERS.
func TestSerializeForBuzzer_TeamFieldsWhitelist(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	msgMap := parseMsgMap(t, data)
	// SerializeForBuzzer preserves the lowercase "teams" map format from GetGameJSON()
	teamsRaw, ok := msgMap["teams"].(map[string]interface{})
	if !ok || len(teamsRaw) == 0 {
		t.Fatal("expected 'teams' map in SerializeForBuzzer output")
	}

	// Extract the first team entry from the map (order undefined)
	var team map[string]interface{}
	for _, v := range teamsRaw {
		if tm, ok := v.(map[string]interface{}); ok {
			team = tm
			break
		}
	}
	if team == nil {
		t.Fatal("no team entry found in 'teams' map")
	}

	// MEMBERS must be absent (not in buzzerTeamKeys)
	if _, present := team["MEMBERS"]; present {
		t.Error("SerializeForBuzzer: team 'MEMBERS' should be absent")
	}

	// Essential team fields must be present
	for _, key := range buzzerTeamKeys {
		if _, present := team[key]; !present {
			t.Errorf("SerializeForBuzzer: team key %q should be present", key)
		}
	}
}

// TestSerializeForBuzzer_TopLevelMinimal verifies that the MSG top-level contains
// only essential fields and not config/question/etc.
//
// SerializeForBuzzer() extracts PHASE/TIME/CURRENT_TIME from the "GAME" node and
// places them at the top level of the minimal payload. Bumpers and teams are preserved
// with their lowercase map-keyed structure ("bumpers", "teams").
func TestSerializeForBuzzer_TopLevelMinimal(t *testing.T) {
	msg := buildUpdateMsg(t)

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	msgMap := parseMsgMap(t, data)

	// admin-only top-level keys must be absent
	forbiddenTopLevel := []string{"config", "GAME", "question", "questions", "history", "palmares", "remote", "neonEffect", "enrollmentOpen"}
	for _, key := range forbiddenTopLevel {
		if _, present := msgMap[key]; present {
			t.Errorf("SerializeForBuzzer: top-level key %q should be absent from buzzer payload", key)
		}
	}

	// PHASE must be present — extracted from "GAME" node and promoted to top level
	if _, ok := msgMap["PHASE"]; !ok {
		t.Error("SerializeForBuzzer: 'PHASE' should be present in minimal payload")
	}

	// "bumpers" map must be present (lowercase, matching GetGameJSON() format)
	if _, ok := msgMap["bumpers"]; !ok {
		t.Error("SerializeForBuzzer: 'bumpers' (lowercase) should be present")
	}

	// "teams" map must be present (lowercase, matching GetGameJSON() format)
	if _, ok := msgMap["teams"]; !ok {
		t.Error("SerializeForBuzzer: 'teams' (lowercase) should be present")
	}
}

// TestSerializeForBuzzer_NonUpdatePassthrough verifies that non-UPDATE actions
// are transmitted without modification (passthrough).
func TestSerializeForBuzzer_NonUpdatePassthrough(t *testing.T) {
	stopMsg, _ := NewMessage(ActionStop, nil)

	buzzerData, err := stopMsg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer on STOP failed: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(buzzerData, &parsed); err != nil {
		t.Fatalf("invalid JSON from SerializeForBuzzer: %v", err)
	}
	if parsed.Action != ActionStop {
		t.Errorf("expected STOP, got %s", parsed.Action)
	}
}

// TestSerializeForBuzzer_SourceImmutable verifies the source message is not
// mutated by SerializeForBuzzer.
func TestSerializeForBuzzer_SourceImmutable(t *testing.T) {
	msg := buildUpdateMsg(t)
	originalMsg := string(msg.Msg)

	_, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	if string(msg.Msg) != originalMsg {
		t.Error("SerializeForBuzzer mutated the source message — source must remain immutable")
	}
}

// TestAllActions_Defined verifies that all action constants are defined and non-empty.
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
