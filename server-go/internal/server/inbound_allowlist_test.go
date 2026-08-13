package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

	adminCount, tvCount, vplayerCount, animCount := hub.GetClientCounts()
	if adminCount != 1 {
		t.Errorf("#154 E2: adminCount = %d, want 1 (buzzer/unknown types must not inflate it)", adminCount)
	}
	if tvCount != 0 || vplayerCount != 0 || animCount != 0 {
		t.Errorf("#154 E2: tvCount=%d vplayerCount=%d animCount=%d, want 0/0/0", tvCount, vplayerCount, animCount)
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

// TestReadPump_TypeSnapshot_NoRaceOnConsecutiveMessages is the code-review
// CRITIQUE 2 regression test: a client sending two messages back-to-back on
// the same /ws connection, without waiting for the first to be dispatched,
// must not race readPump's read of c.Type against SetClientType's locked
// write.
//
// Deliberately does NOT use the sendAction-style "write, then synchronously
// wait+dispatch" pattern other #154 tests use — that pattern can structurally
// never reproduce this race (readPump never blocks between two
// ReadMessage() calls waiting for the previous message's dispatch to
// finish, but a strictly-sequential test harness does exactly that
// waiting). Instead this spawns a REAL dispatch goroutine — the same shape
// as cmd/server's setupCallbacks (`for msg := range hub.Incoming { ... }`)
// — running concurrently with the client writing both frames without any
// wait in between, so readPump's second read genuinely overlaps the first
// message's SetClientType call. Run under `go test -race`: before the
// TypeSnapshot fix this reliably reported a DATA RACE on WebSocketClient.Type
// (reproduced by code-reviewer); after the fix it passes clean.
func TestReadPump_TypeSnapshot_NoRaceOnConsecutiveMessages(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	// Legacy endpoint — defaults to Admin, the one client type SET_CLIENT_TYPE
	// can legitimately be sent from (IsSetClientTypeAllowed).
	conn := dialWSPath(t, srv, "/")
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			select {
			case incoming := <-hub.Incoming:
				if incoming.Data.Action == protocol.ActionSetClientType {
					// Mirrors cmd/server's handleSetClientType (IsSetClientTypeAllowed
					// already covered by its own test) — the write this test's
					// second message must not race.
					hub.SetClientType(incoming.ClientID, ClientTypeTV)
				}
			case <-time.After(2 * time.Second):
				t.Errorf("timed out waiting for message %d/2 on hub.Incoming", i+1)
				return
			}
		}
	}()

	setTypeMsg, err := protocol.NewMessage(protocol.ActionSetClientType, protocol.SetClientTypePayload{Type: "tv"})
	if err != nil {
		t.Fatalf("failed to build SET_CLIENT_TYPE: %v", err)
	}
	setTypeData, err := setTypeMsg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize SET_CLIENT_TYPE: %v", err)
	}

	followUpMsg, err := protocol.NewMessage(protocol.ActionHello, nil)
	if err != nil {
		t.Fatalf("failed to build HELLO: %v", err)
	}
	followUpData, err := followUpMsg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize HELLO: %v", err)
	}

	// No wait between the two writes — the exact interleaving CRITIQUE 2
	// flagged: readPump loops straight to its next ReadMessage()/TypeSnapshot
	// call while the dispatch goroutine above may still be inside
	// SetClientType's locked section for the first message.
	if err := conn.WriteMessage(websocket.TextMessage, setTypeData); err != nil {
		t.Fatalf("failed to send SET_CLIENT_TYPE: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, followUpData); err != nil {
		t.Fatalf("failed to send HELLO: %v", err)
	}

	<-done
}

// TestReadPump_CancelAIGeneration_NonAdminRejectionIsLogged is the
// code-review minor-fix follow-up: CANCEL_AI_GENERATION is the one action
// that bypasses handleWebMessage's switch entirely (handled directly in
// readPump, contract ai-multi-provider.md §11), so before this fix its
// rejection for a non-admin client was completely silent — unlike every
// other allow-list rejection, which logs a WARN via handleWebMessage.
//
// ⚠️ Mutates the package-level global logger (SetGlobalLogger) — restored via
// t.Cleanup. Per this package's convention (see ai_job_test.go et al.), no
// t.Parallel() here.
func TestReadPump_CancelAIGeneration_NonAdminRejectionIsLogged(t *testing.T) {
	previous := GetGlobalLogger()
	bl := NewBroadcastLogger(50)
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(previous) })

	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	// TV — a dedicated, fixed-type endpoint, definitely not admin.
	conn := dialWSPath(t, srv, "/ws/tv")
	defer conn.Close()

	msg, err := protocol.NewMessage(protocol.ActionCancelAIGeneration, protocol.CancelAIGenerationPayload{JobID: "job-1"})
	if err != nil {
		t.Fatalf("failed to build CANCEL_AI_GENERATION: %v", err)
	}
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize CANCEL_AI_GENERATION: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send CANCEL_AI_GENERATION: %v", err)
	}

	// Drain the resulting IncomingMessage so the test doesn't depend on
	// readPump's internal timing beyond "the WARN was logged before the
	// message reached the Incoming channel" (it's logged first in readPump).
	select {
	case <-hub.Incoming:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the message to reach hub.Incoming")
	}

	found := false
	for _, entry := range bl.GetRecent(50) {
		if entry.Level == game.LogLevelWarn && strings.Contains(entry.Message, "CANCEL_AI_GENERATION") {
			found = true
			break
		}
	}
	if !found {
		t.Error("#154 code review (mineur): CANCEL_AI_GENERATION rejected for a non-admin client must log a WARN, same as every other allow-list rejection")
	}
}
