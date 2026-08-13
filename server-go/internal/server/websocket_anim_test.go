package server

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #155 (v6.2.0) — Interface Animateur, tâche B1/B2.
//
// B1's critical risk (plan _work/reports/plan-20260813-092950.md §0.2,
// contracts/ws-payload-serialization.md §"Animateur" ⚠️ Piège) is that
// serializeForClientType's `default:` branch returns SerializeForAdmin() —
// a ClientTypeAnim not explicitly routed in the switch would silently leak
// the FULL admin payload (firmware/OTA/ACK bumper fields, QUIZ_OBJECTIVES,
// config) to the animateur tablet. TestSerializeForClientType_Anim_* below
// is the dedicated test the plan calls for — it must fail loudly if that
// case is ever removed or the switch falls through to default again.
// ---------------------------------------------------------------------------

// buildUpdateMessageWithAdminOnlyFields builds an ActionUpdate message whose
// MSG carries one bumper with every AdminOnlyBumperFields entry set, a GAME
// node with QUIZ_OBJECTIVES set, and a top-level "config" key — the exact
// shape SerializeForWebClient is documented to strip (messages.go).
func buildUpdateMessageWithAdminOnlyFields(t *testing.T) *protocol.Message {
	t.Helper()
	raw := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":           "STARTED",
			"QUIZ_OBJECTIVES": "secret animation cue",
			"QUIZ_NAME":       "Quiz du jeudi",
		},
		"teams": map[string]interface{}{},
		"bumpers": map[string]interface{}{
			"mac-1": map[string]interface{}{
				"NAME":             "Bob",
				"TEAM":             "TeamA",
				"FIRMWARE_VERSION": "1.2.3",
				"IS_OUTDATED":      true,
				"OTA_STATUS":       "idle",
				"OTA_PERCENT":      0,
				"ACK_PENDING":      false,
			},
		},
		"config": map[string]interface{}{"secret": "server-only"},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal fixture: %v", err)
	}
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	msg.Msg = data
	return msg
}

// TestSerializeForClientType_Anim_MatchesSerializeForWebClient is the piège-
// closing test: ClientTypeAnim must be explicitly routed to
// SerializeForWebClient (byte-identical to calling it directly), never fall
// through to the default branch's SerializeForAdmin().
func TestSerializeForClientType_Anim_MatchesSerializeForWebClient(t *testing.T) {
	msg := buildUpdateMessageWithAdminOnlyFields(t)

	got, err := serializeForClientType(msg, ClientTypeAnim)
	if err != nil {
		t.Fatalf("serializeForClientType(ClientTypeAnim) failed: %v", err)
	}

	want, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("serializeForClientType(ClientTypeAnim) diverges from SerializeForWebClient()\n got:  %s\nwant: %s", got, want)
	}
}

// TestSerializeForClientType_Anim_NeverEqualsSerializeForAdmin is the same
// risk from the other direction: even if a future refactor accidentally
// re-widens the switch (e.g. merges ClientTypeAnim into a case that resolves
// to Admin), this must catch it independently of the exact-match test above.
func TestSerializeForClientType_Anim_NeverEqualsSerializeForAdmin(t *testing.T) {
	msg := buildUpdateMessageWithAdminOnlyFields(t)

	got, err := serializeForClientType(msg, ClientTypeAnim)
	if err != nil {
		t.Fatalf("serializeForClientType(ClientTypeAnim) failed: %v", err)
	}

	adminData, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	if string(got) == string(adminData) {
		t.Fatal("serializeForClientType(ClientTypeAnim) is byte-identical to SerializeForAdmin() — the animateur is receiving the FULL admin payload (§0.2 piège: default branch not overridden for ClientTypeAnim)")
	}

	var body struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
			Game    map[string]interface{}             `json:"GAME"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatalf("failed to unmarshal animateur payload: %v", err)
	}

	for _, field := range protocol.AdminOnlyBumperFields {
		if _, ok := body.Msg.Bumpers["mac-1"][field]; ok {
			t.Errorf("animateur payload must not contain admin-only bumper field %q", field)
		}
	}
	for _, field := range protocol.AdminOnlyGameFields {
		if _, ok := body.Msg.Game[field]; ok {
			t.Errorf("animateur payload must not contain admin-only GAME field %q (e.g. QUIZ_OBJECTIVES is a confidentiality rule, not just a size optimization)", field)
		}
	}
	if _, ok := body.Msg.Bumpers["mac-1"]["NAME"]; !ok {
		t.Error("animateur payload lost an essential (non admin-only) bumper field — over-stripping, not just under-stripping, would also be wrong")
	}
}

// TestGetClientCounts_Anim_CountedSeparatelyFromAdmin is B2's acceptance
// criterion: a connected animateur must increment ANIM_COUNT and neither
// ADMIN_COUNT nor TV_COUNT (contracts/websocket-endpoints.md, plan B2).
func TestGetClientCounts_Anim_CountedSeparatelyFromAdmin(t *testing.T) {
	hub := NewWebSocketHub()
	hub.clients[&WebSocketClient{ID: "admin-1", Type: ClientTypeAdmin}] = true
	hub.clients[&WebSocketClient{ID: "tv-1", Type: ClientTypeTV}] = true
	hub.clients[&WebSocketClient{ID: "anim-1", Type: ClientTypeAnim}] = true
	hub.clients[&WebSocketClient{ID: "anim-2", Type: ClientTypeAnim}] = true

	adminCount, tvCount, vplayerCount, animCount := hub.GetClientCounts()
	if animCount != 2 {
		t.Errorf("animCount = %d, want 2", animCount)
	}
	if adminCount != 1 {
		t.Errorf("adminCount = %d, want 1 (animateur clients must not inflate it)", adminCount)
	}
	if tvCount != 1 {
		t.Errorf("tvCount = %d, want 1", tvCount)
	}
	if vplayerCount != 0 {
		t.Errorf("vplayerCount = %d, want 0", vplayerCount)
	}
}

// TestHandleConnectionWithType_AnimType is the connection-level counterpart
// of TestHandleConnectionWithType_TVType/VPlayerType: a client dialing
// /ws/anim must be registered with Type == ClientTypeAnim and counted only
// as such.
func TestHandleConnectionWithType_AnimType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/anim")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, vplayerCount, animCount := hub.GetClientCounts()
	if animCount != 1 {
		t.Errorf("Expected 1 anim client, got %d", animCount)
	}
	if adminCount != 0 || tvCount != 0 || vplayerCount != 0 {
		t.Errorf("Expected 0 admin/tv/vplayer, got admin=%d tv=%d vplayer=%d", adminCount, tvCount, vplayerCount)
	}
}

// TestBroadcastToTypes_AnimOnly mirrors TestBroadcastToTypes_TVOnly: a
// broadcast targeted at ClientTypeAnim must reach only the animateur
// connection, and its payload must already be admin-fields-stripped
// (SerializeForWebClient, not the raw/full serialization).
func TestBroadcastToTypes_AnimOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	anim := dialWSPath(t, srv, "/ws/anim")
	defer anim.Close()

	time.Sleep(50 * time.Millisecond)

	msg := buildUpdateMessageWithAdminOnlyFields(t)
	hub.BroadcastToTypes(msg, ClientTypeAnim)

	received := readWSMsg(t, anim, 500*time.Millisecond)
	if received.Action != protocol.ActionUpdate {
		t.Errorf("anim: expected UPDATE, got %s", received.Action)
	}
	var body struct {
		Bumpers map[string]map[string]interface{} `json:"bumpers"`
	}
	if err := json.Unmarshal(received.Msg, &body); err != nil {
		t.Fatalf("failed to unmarshal anim UPDATE: %v", err)
	}
	if _, ok := body.Bumpers["mac-1"]["FIRMWARE_VERSION"]; ok {
		t.Error("anim must not receive FIRMWARE_VERSION via BroadcastToTypes")
	}

	// Admin and TV should NOT receive (not in the target type list).
	expectNoMessage(t, admin, 150*time.Millisecond)
	expectNoMessage(t, tv, 150*time.Millisecond)
}
