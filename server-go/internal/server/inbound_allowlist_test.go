package server

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #154 (sec) — IsActionAllowed / IsSetClientTypeAllowed (pure policy,
// no WebSocket wiring — see cmd/server/inbound_allowlist_test.go for the
// end-to-end handleWebMessage exercise) — plus E2 (GetClientCounts) and E4
// (BroadcastToTypes) regression coverage.
// ---------------------------------------------------------------------------

func TestIsActionAllowed(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		clientType ClientType
		want       bool
	}{
		// Handshake — every web client type.
		{"HELLO from admin", protocol.ActionHello, ClientTypeAdmin, true},
		{"HELLO from tv", protocol.ActionHello, ClientTypeTV, true},
		{"HELLO from vplayer", protocol.ActionHello, ClientTypeVPlayer, true},

		// Admin-only game control — the exact examples from issue #154.
		{"START from admin", protocol.ActionStart, ClientTypeAdmin, true},
		{"START from tv", protocol.ActionStart, ClientTypeTV, false},
		{"START from vplayer", protocol.ActionStart, ClientTypeVPlayer, false},
		{"STOP from tv", protocol.ActionStop, ClientTypeTV, false},
		{"RAZ from vplayer", protocol.ActionRAZ, ClientTypeVPlayer, false},
		{"DELETE from tv", protocol.ActionDelete, ClientTypeTV, false},
		{"DELETE_BUMPER from vplayer", protocol.ActionDeleteBumper, ClientTypeVPlayer, false},
		{"NEW_GAME from tv", protocol.ActionNewGame, ClientTypeTV, false},
		{"NEW_GAME from admin", protocol.ActionNewGame, ClientTypeAdmin, true},
		{"BUMPER_POINTS from vplayer", protocol.ActionBumperPoints, ClientTypeVPlayer, false},
		{"BUMPER_POINTS from admin", protocol.ActionBumperPoints, ClientTypeAdmin, true},
		{"CANCEL_AI_GENERATION from tv", protocol.ActionCancelAIGeneration, ClientTypeTV, false},
		{"CANCEL_AI_GENERATION from admin", protocol.ActionCancelAIGeneration, ClientTypeAdmin, true},

		// MEMORY/MEMOTION — TV carries both the admin preview iframe and a
		// real spectator's own click; MEMOTION_SELECT is admin-preview-only.
		{"FLIP_MEMORY_CARD from tv", protocol.ActionFlipMemoryCard, ClientTypeTV, true},
		{"FLIP_MEMORY_CARD from vplayer", protocol.ActionFlipMemoryCard, ClientTypeVPlayer, true},
		{"FLIP_MEMORY_CARD from admin", protocol.ActionFlipMemoryCard, ClientTypeAdmin, false},
		{"MEMOTION_SELECT from tv", protocol.ActionMotionSelect, ClientTypeTV, true},
		{"MEMOTION_SELECT from vplayer", protocol.ActionMotionSelect, ClientTypeVPlayer, false},
		{"MEMOTION_FLIP from admin", protocol.ActionMotionFlip, ClientTypeAdmin, true},
		{"MEMOTION_FLIP from tv", protocol.ActionMotionFlip, ClientTypeTV, false},

		// VPlayer-only.
		{"PLAYER_CONNECT from vplayer", protocol.ActionPlayerConnect, ClientTypeVPlayer, true},
		{"PLAYER_CONNECT from tv", protocol.ActionPlayerConnect, ClientTypeTV, false},
		{"PLAYER_CONNECT from admin", protocol.ActionPlayerConnect, ClientTypeAdmin, false},
		{"VPLAYER_QCM_ANSWER from vplayer", protocol.ActionVPlayerQCMAnswer, ClientTypeVPlayer, true},
		{"VPLAYER_QCM_ANSWER from admin", protocol.ActionVPlayerQCMAnswer, ClientTypeAdmin, false},
		{"ARDOISE_INPUT from vplayer", protocol.ActionArdoiseInput, ClientTypeVPlayer, true},
		{"ARDOISE_INPUT from tv", protocol.ActionArdoiseInput, ClientTypeTV, false},

		// Default-deny: unknown action, unknown/empty client type.
		{"unknown action from admin", "NOT_A_REAL_ACTION", ClientTypeAdmin, false},
		{"known action from empty clientType", protocol.ActionStart, "", false},
		{"known action from unrecognized clientType", protocol.ActionHello, ClientTypeBuzzer, false},

		// SET_CLIENT_TYPE is NOT in the map — IsActionAllowed must
		// default-deny it too (cmd/server's handleWebMessage special-cases
		// it explicitly before ever calling IsActionAllowed).
		{"SET_CLIENT_TYPE not in the static map", protocol.ActionSetClientType, ClientTypeAdmin, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsActionAllowed(tt.action, tt.clientType); got != tt.want {
				t.Errorf("IsActionAllowed(%q, %q) = %v, want %v", tt.action, tt.clientType, got, tt.want)
			}
		})
	}
}

func TestIsSetClientTypeAllowed(t *testing.T) {
	tests := []struct {
		name        string
		currentType ClientType
		want        bool
	}{
		{"admin (legacy /ws default) may self-declare", ClientTypeAdmin, true},
		{"tv (already self-identified) may not re-declare", ClientTypeTV, false},
		{"vplayer (already self-identified) may not re-declare", ClientTypeVPlayer, false},
		{"buzzer may not", ClientTypeBuzzer, false},
		{"empty/unknown may not", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSetClientTypeAllowed(tt.currentType); got != tt.want {
				t.Errorf("IsSetClientTypeAllowed(%q) = %v, want %v", tt.currentType, got, tt.want)
			}
		})
	}
}

// TestGetClientCounts_UnrecognizedType_CountsTowardNone is #154 E2: before
// the fix, `default: adminCount++` meant a client whose Type wasn't TV or
// VPlayer silently inflated adminCount, including a type this hub doesn't
// actually expect to see (ClientTypeBuzzer never registers here in
// production — see BuzzerWebSocketHub — but nothing previously enforced
// that at the counting level).
func TestGetClientCounts_UnrecognizedType_CountsTowardNone(t *testing.T) {
	hub := NewWebSocketHub()
	hub.clients[&WebSocketClient{ID: "admin-1", Type: ClientTypeAdmin}] = true
	hub.clients[&WebSocketClient{ID: "buzzer-1", Type: ClientTypeBuzzer}] = true
	hub.clients[&WebSocketClient{ID: "mystery-1", Type: "future-role"}] = true

	adminCount, tvCount, vplayerCount := hub.GetClientCounts()
	if adminCount != 1 {
		t.Errorf("#154 E2: adminCount = %d, want 1 (buzzer/unknown types must not inflate it)", adminCount)
	}
	if tvCount != 0 || vplayerCount != 0 {
		t.Errorf("#154 E2: tvCount=%d vplayerCount=%d, want 0/0", tvCount, vplayerCount)
	}
}

// TestBroadcastToTypes_PerTypeSerialization is #154 E4: BroadcastToTypes must
// serialize per-recipient-type rather than once globally, so a future
// type-sensitive action (today: only ActionUpdate reduces content at all,
// per messages.go's SerializeForXxx family) is filtered correctly for every
// requested type instead of everyone getting the Admin/full bytes.
func TestBroadcastToTypes_PerTypeSerialization(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()

	time.Sleep(50 * time.Millisecond)

	// A GetGameJSON()-shaped UPDATE payload with an admin-only bumper field.
	raw := `{"GAME":{"PHASE":"STARTED"},"teams":{},"bumpers":{"mac-1":{"NAME":"Bob","FIRMWARE_VERSION":"1.2.3"}}}`
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	msg.Msg = json.RawMessage(raw)

	hub.BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeTV)

	adminMsg := readWSMsg(t, admin, 500*time.Millisecond)
	tvMsg := readWSMsg(t, tv, 500*time.Millisecond)

	var adminBody, tvBody struct {
		Bumpers map[string]map[string]interface{} `json:"bumpers"`
	}
	if err := json.Unmarshal(adminMsg.Msg, &adminBody); err != nil {
		t.Fatalf("failed to unmarshal admin body: %v", err)
	}
	if err := json.Unmarshal(tvMsg.Msg, &tvBody); err != nil {
		t.Fatalf("failed to unmarshal tv body: %v", err)
	}

	if _, ok := adminBody.Bumpers["mac-1"]["FIRMWARE_VERSION"]; !ok {
		t.Error("#154 E4: admin must still receive FIRMWARE_VERSION (SerializeForAdmin)")
	}
	if _, ok := tvBody.Bumpers["mac-1"]["FIRMWARE_VERSION"]; ok {
		t.Error("#154 E4: TV must NOT receive FIRMWARE_VERSION via BroadcastToTypes (SerializeForWebClient)")
	}
}
