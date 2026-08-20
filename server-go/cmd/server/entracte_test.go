package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// ENTRACTE (v6.5.2, #119) — end-to-end dispatch tests, exercised through
// handleWebMessage (same harness as inbound_allowlist_test.go:
// newTestAppWithHub/startEvictionTestServer/dialWS/sendAction).
//
// Contract: contracts/websocket-actions.md §"ENTRACTE_SET" and §"Actions
// refusées pendant l'entracte" (D6), contracts/game-state.md §"ENTRACTE".
// ---------------------------------------------------------------------------

// newEntracteTestApp sets up an app with two non-VPlayer bumpers (so LED
// side effects have something to observe) plus the logger/udpBcast every
// handler in handleWebMessage's switch may unconditionally touch — same
// pattern as newAnimTestApp (inbound_allowlist_anim_test.go).
func newEntracteTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()

	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"buzzer-1":  {Name: "buzzer-1", Team: "TeamA", Connected: true},
		"buzzer-2":  {Name: "buzzer-2", Team: "TeamA", Connected: true},
		"vplayer-1": {Name: "vplayer-1", Team: "TeamA", Connected: true, IsVPlayer: true, IsVirtual: true},
	})
	return app
}

func entracteSetMsg(active bool) *protocol.Message {
	msg, _ := protocol.NewMessage(protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: active})
	return msg
}

// TestHandleEntracteSet_ActivationFromEligiblePhase covers the D4 phase
// table end-to-end: activation from STOPPED succeeds, GameState.ENTRACTE
// flips, and every non-VPlayer buzzer's stored LED state goes to OFF (B5).
func TestHandleEntracteSet_ActivationFromEligiblePhase(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseStopped)

	app.handleEntracteSet(entracteSetMsg(true))

	if !app.engine.IsEntracte() {
		t.Fatal("expected IsEntracte()=true after ENTRACTE_SET{ACTIVE:true} from an eligible phase")
	}

	off := protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"}
	for _, mac := range []string{"buzzer-1", "buzzer-2"} {
		got, ok := app.bumperLEDState[mac]
		if !ok || got != off {
			t.Errorf("expected buzzer %s LED state to be OFF %+v, got %+v (present=%v)", mac, off, got, ok)
		}
	}
	if _, ok := app.bumperLEDState["vplayer-1"]; ok {
		t.Error("VPlayer bumper should never receive an LED_SET (IsVPlayer skip)")
	}
}

// TestHandleEntracteSet_ActivationRefusedFromIneligiblePhase: STARTED is not
// in the D4 allow-list — the flag must stay false and no LED side effect
// must occur.
func TestHandleEntracteSet_ActivationRefusedFromIneligiblePhase(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseStarted)
	app.bumperLEDState["buzzer-1"] = protocol.LEDSetPayload{Color: [3]int{9, 9, 9}, Intensity: 42, Effect: "SOLID"}

	app.handleEntracteSet(entracteSetMsg(true))

	if app.engine.IsEntracte() {
		t.Fatal("expected IsEntracte()=false: activation from STARTED must be refused")
	}
	if got := app.bumperLEDState["buzzer-1"]; got.Intensity != 42 {
		t.Errorf("refused activation must not touch LED state, got %+v", got)
	}
}

// TestHandleEntracteSet_DeactivationRestoresLEDs: exiting entracte restores
// per-buzzer LEDs derived from the current phase (sendLEDSetAllBuzzers), not
// left OFF.
func TestHandleEntracteSet_DeactivationRestoresLEDs(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseStopped)

	app.handleEntracteSet(entracteSetMsg(true))
	if !app.engine.IsEntracte() {
		t.Fatal("setup: activation should have succeeded")
	}

	app.handleEntracteSet(entracteSetMsg(false))
	if app.engine.IsEntracte() {
		t.Fatal("expected IsEntracte()=false after deactivation")
	}

	// PhaseStopped -> sendLEDSetForBuzzerNormal sends the team color, SOLID
	// 255 — anything but the OFF payload proves LEDs were restored.
	off := protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"}
	for _, mac := range []string{"buzzer-1", "buzzer-2"} {
		if got := app.bumperLEDState[mac]; got == off {
			t.Errorf("expected buzzer %s LED to be restored (non-OFF) after deactivation, still OFF", mac)
		}
	}
}

// TestHandleEntracteSet_DeactivationAlwaysSucceeds mirrors the engine-level
// test but through the full dispatch path, from an ineligible-for-ACTIVATION
// phase — deactivation must still work everywhere.
func TestHandleEntracteSet_DeactivationAlwaysSucceeds(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseStopped)
	app.handleEntracteSet(entracteSetMsg(true))

	app.engine.SetPhase(game.PhaseStarted) // move to an activation-ineligible phase
	app.handleEntracteSet(entracteSetMsg(false))

	if app.engine.IsEntracte() {
		t.Fatal("expected deactivation to succeed even from a phase where activation would be refused")
	}
}

// TestHandleWebMessage_EntracteBlocksStart is the core D6 regression test:
// while ENTRACTE is active, a START sent by an admin must be silently
// rejected — the phase must not change.
func TestHandleWebMessage_EntracteBlocksStart(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseReady)
	if !app.engine.SetEntracte(true) {
		t.Fatal("setup: activation from READY should succeed")
	}

	srv := startEvictionTestServer(t, app)
	conn := dialWS(t, srv, "/admin")
	sendAction(t, app, conn, protocol.ActionStart, protocol.StartPayload{Delay: 5})

	if app.engine.GetState().Phase != game.PhaseReady {
		t.Errorf("expected phase to stay READY (START blocked by entracte), got %s", app.engine.GetState().Phase)
	}
}

// TestHandleWebMessage_EntracteAllowsEntracteSetFromAdmin: the one action
// that must always get through — otherwise entracte would have no exit.
func TestHandleWebMessage_EntracteAllowsEntracteSetFromAdmin(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseReady)
	if !app.engine.SetEntracte(true) {
		t.Fatal("setup: activation from READY should succeed")
	}

	srv := startEvictionTestServer(t, app)
	conn := dialWS(t, srv, "/admin")
	sendAction(t, app, conn, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: false})

	if app.engine.IsEntracte() {
		t.Error("expected ENTRACTE_SET{ACTIVE:false} to be let through during entracte and deactivate it")
	}
}

// TestHandleWebMessage_EntracteSetRefusedFromNonAdmin covers the client-type
// gate (contract: "admin uniquement") end-to-end, for tv/vplayer/anim.
func TestHandleWebMessage_EntracteSetRefusedFromNonAdmin(t *testing.T) {
	paths := []string{"/tv", "/vplayer", "/anim"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			app := newEntracteTestApp(t)
			app.engine.SetPhase(game.PhaseStopped)

			srv := startAnimAllowlistTestServer(t, app)
			conn := dialWS(t, srv, path)
			sendAction(t, app, conn, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})

			if app.engine.IsEntracte() {
				t.Errorf("ENTRACTE_SET from %s must be refused (admin only), but entracte activated", path)
			}
		})
	}
}

// TestPhysicalButton_InertDuringEntracte is the B5/D4 non-regression test:
// entracte is only reachable outside PhaseStarted, and handleButton already
// no-ops outside PhaseStarted (main.go: "only accept BUTTON in STARTED
// phase") — this test locks that invariant down explicitly, through the
// real physical-buzzer path, rather than relying on it being true by
// accident.
func TestPhysicalButton_InertDuringEntracte(t *testing.T) {
	app := newEntracteTestApp(t)
	app.engine.SetPhase(game.PhaseStopped)
	if !app.engine.SetEntracte(true) {
		t.Fatal("setup: activation from STOPPED should succeed")
	}

	before := app.engine.GetBumper("buzzer-1").Score
	msg := &protocol.Message{Action: protocol.ActionButton, Msg: []byte(`{"button":"A"}`)}
	app.handleButton("buzzer-1", msg, 1234567890)

	after := app.engine.GetBumper("buzzer-1").Score
	if before != after {
		t.Errorf("physical buzz during entracte must not change score: before=%d after=%d", before, after)
	}
	if app.engine.GetState().Phase != game.PhaseStopped {
		t.Errorf("physical buzz during entracte must not change phase: got %s", app.engine.GetState().Phase)
	}
}
