package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #154 E1 (sec) — sendStateToClient (HELLO) must serialize per the
// connecting client's own type, and must gate CONFIG_UPDATE to Admin+TV —
// same policy broadcastConfigUpdate already applies to ongoing broadcasts.
//
// Before this fix, every message sendStateToClient sent went through
// WebSocketHub.SendToClient, which always calls the unfiltered
// SerializeForWebSocket regardless of client type — a VPlayer connecting
// received the full admin-only bumper fields (FIRMWARE_VERSION/OTA/ACK) on
// its very first UPDATE, and CONFIG_UPDATE despite never receiving it again
// for the rest of its connection.
// ---------------------------------------------------------------------------

func TestSendStateToClient_VPlayer_StripsAdminFieldsAndConfigUpdate(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3",
	})

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	// #154 E1 fix under test: no a.httpServer set up here at all — the
	// VPlayer path must never dereference it (CONFIG_UPDATE construction is
	// now fully inside the Admin/TV gate).
	app.sendStateToClient(clientID, server.ClientTypeVPlayer)

	_, updateRaw := readActionMatching(t, conn, protocol.ActionUpdate)
	var updateEnvelope struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(updateRaw), &updateEnvelope); err != nil {
		t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, updateRaw)
	}
	if _, ok := updateEnvelope.Msg.Bumpers["bumper-1"]["FIRMWARE_VERSION"]; ok {
		t.Error("#154 E1: VPlayer must not receive FIRMWARE_VERSION on its HELLO UPDATE")
	}

	// QUESTIONS and CLIENTS are unaffected by #154 E1 (out of scope — see
	// E5 in _work/reports/plan-20260812-141735.md §4.3, deferred to #155) —
	// drain them so the CONFIG_UPDATE assertion below isn't fooled by
	// ordering.
	readActionMatching(t, conn, protocol.ActionQuestions)
	readActionMatching(t, conn, protocol.ActionClients)

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("#154 E1: VPlayer must NOT receive CONFIG_UPDATE at HELLO (broadcastConfigUpdate already excludes VPlayer from ongoing broadcasts)")
	}
}

func TestSendStateToClient_Admin_IncludesAdminFieldsAndConfigUpdate(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3",
	})
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/admin")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAdmin)

	_, updateRaw := readActionMatching(t, conn, protocol.ActionUpdate)
	var updateEnvelope struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(updateRaw), &updateEnvelope); err != nil {
		t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, updateRaw)
	}
	if _, ok := updateEnvelope.Msg.Bumpers["bumper-1"]["FIRMWARE_VERSION"]; !ok {
		t.Error("admin must still receive FIRMWARE_VERSION on its HELLO UPDATE")
	}

	readActionMatching(t, conn, protocol.ActionQuestions)
	readActionMatching(t, conn, protocol.ActionClients)

	// Admin must still receive CONFIG_UPDATE — broadcastConfigUpdate's
	// established policy, unchanged by #154.
	readActionMatching(t, conn, protocol.ActionConfigUpdate)
}
