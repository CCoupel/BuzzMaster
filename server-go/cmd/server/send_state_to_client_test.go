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

	// QUESTIONS is admin-only since #155/#156 B4 (contracts/CHANGELOG.md
	// [20260813]) — VPlayer never receives it at HELLO anymore. Drain CLIENTS
	// so the CONFIG_UPDATE assertion below isn't fooled by ordering.
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

// TestSendStateToClient_TV_IncludesConfigUpdate is the code-review minor-fix
// follow-up: main.go:4182's gate is `clientType == ClientTypeAdmin ||
// clientType == ClientTypeTV` — only the Admin and VPlayer branches were
// covered above, leaving the TV half of that condition unverified against a
// future accidental simplification (e.g. down to `== ClientTypeAdmin` alone).
func TestSendStateToClient_TV_IncludesConfigUpdate(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3",
	})
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/tv")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeTV)

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
		t.Error("#154 E1: TV must NOT receive FIRMWARE_VERSION on its HELLO UPDATE (SerializeForWebClient)")
	}

	// QUESTIONS is admin-only since #155/#156 B4 — TV never receives it at
	// HELLO anymore (contracts/CHANGELOG.md [20260813]).
	readActionMatching(t, conn, protocol.ActionClients)

	// TV must still receive CONFIG_UPDATE — same established policy as
	// admin (broadcastConfigUpdate targets Admin+TV, never VPlayer).
	readActionMatching(t, conn, protocol.ActionConfigUpdate)
}
