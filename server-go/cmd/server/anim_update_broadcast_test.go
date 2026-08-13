package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #162 — broadcasts never reached ClientTypeAnim (scores/timer/phase
// frozen on the tablet after connecting).
//
// This file originally held dev-backend's own pre-T3 verification pass,
// written before test-writer's parallel T3 suite
// (cmd/server/broadcast_anim_test.go) landed. Once it did, every test here
// turned out to duplicate one already in that file (same behavior, some
// literally the same name — TestBroadcastQuestions_StillAdminOnly,
// TestBroadcastClientCounts_StillAdminOnly,
// TestBroadcastConfigUpdate_StillAdminAndTVOnly, TestHandlePong_StillExcludesAnim
// collided outright) except the one kept below, which broadcast_anim_test.go
// does not cover: the #127 CA7/#129 CA8 ApplyVPlayerBroadcastConnEvents
// exclusion for an Anim-only broadcastUpdateTo call — an explicit T1
// acceptance criterion (_work/reports/plan-20260813-174513.md §3 T1)
// distinct from payload-equality (which broadcast_anim_test.go's
// TestBroadcastUpdateTo_Anim_ReceivesWebClientPayload already covers well).
// ---------------------------------------------------------------------------

// TestBroadcastUpdateTo_Anim_DoesNotTriggerVPlayerConnEvents guards T1's
// explicit exclusion: ApplyVPlayerBroadcastConnEvents (#127 CA7/#129 CA8)
// must never fire from an Anim-only broadcast — it evaluates MessageLost/
// DeliveryConfirmed for the whole VJoueur roster, which has nothing to do
// with an Anim recipient.
func TestBroadcastUpdateTo_Anim_DoesNotTriggerVPlayerConnEvents(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	bobID := "vjoueur-bob"
	app.engine.UpdateBumper(bobID, map[string]interface{}{"TEAM": "TeamA", "IS_VIRTUAL": true, "CONNECTED": false})
	if got := app.engine.GetBumper(bobID).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	app.broadcastUpdateTo(server.ClientTypeAnim)
	readActionMatching(t, anim, protocol.ActionUpdate)

	if got := app.engine.GetBumper(bobID).ConnState; got != "orange" {
		t.Errorf("#162: an Anim-only broadcastUpdateTo must not evaluate VJoueur conn events — ConnState=%q, expected still 'orange' (would be 'red' if ApplyVPlayerBroadcastConnEvents ran)", got)
	}
}
