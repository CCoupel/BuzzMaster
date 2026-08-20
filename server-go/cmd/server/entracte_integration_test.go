package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// T7 (#119) — cycle complet du mode ENTRACTE, à travers le dispatch réel
// (handleWebMessage), pas seulement les méthodes moteur unitaires (déjà
// couvertes côté dev-backend par B8) :
//
//	activation (admin) -> refus d'un START pendant l'entracte -> LEDs
//	éteintes -> désactivation (admin) -> LEDs restaurées.
//
// Plan : _work/reports/plan-entracte-119-20260820-140825.md, tâche T7.
// Harnais : mêmes conventions que inbound_allowlist_test.go (sendAction) et
// led_test.go (assertions via app.bumperLEDState — pas de buzzer WS réel
// nécessaire, sendLEDSet écrit directement dans cette map en l'absence de
// connexion physique). handleWebMessage lit incoming.ClientType directement
// (pas de lookup hub), donc un IncomingMessage construit à la main suffit —
// pas besoin d'une vraie connexion WS pour ce fichier.
// ---------------------------------------------------------------------------

// dispatchAs builds an IncomingMessage for action/payload as clientType and
// runs it through the real handleWebMessage dispatch (client-type allow-list
// + #119 entracte gate + the action's handler, exactly as production code
// path).
func dispatchAs(t *testing.T, app *App, clientType server.ClientType, action string, payload interface{}) {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("failed to build %s message: %v", action, err)
	}
	app.handleWebMessage(&protocol.IncomingMessage{
		Source:     "WebSocket",
		Data:       msg,
		ClientID:   "test-" + string(clientType),
		ClientType: string(clientType),
	})
}

func offLEDPayload() protocol.LEDSetPayload {
	return protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"}
}

// setupEntracteIntegrationTestApp wires an app with two physical (non-
// VPlayer) bumpers on two teams, phase READY (an entry-allowed phase, D4).
func setupEntracteIntegrationTestApp(t *testing.T) *App {
	t.Helper()
	// newTestAppWithHub (player_connect_connstate_test.go), not the bare
	// newTestApp: handleWebMessage unconditionally calls
	// a.wsHub.GetClientPlayerID(...) before reaching the dispatch switch — a
	// nil a.wsHub would nil-panic there, silently swallowed by
	// handleWebMessage's own recover(), and every assertion below would
	// fail on unchanged state with no visible cause. A real (client-less)
	// hub makes that call a safe no-op instead.
	app := newTestAppWithHub(t)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Team: "TeamA", Connected: true},
		"AA:BB:CC:DD:EE:02": {Name: "Buzzer2", IsVirtual: false, Team: "TeamB", Connected: true},
	})
	app.engine.SetPhase(game.PhaseReady)
	return app
}

// TestEntracteIntegration_FullCycle is the T7 scenario in one linear
// sequence, matching the plan's own wording.
func TestEntracteIntegration_FullCycle(t *testing.T) {
	app := setupEntracteIntegrationTestApp(t)

	// --- 1. Activation (admin) ---------------------------------------
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})

	if !app.engine.IsEntracte() {
		t.Fatal("Expected ENTRACTE to be active after ENTRACTE_SET{Active:true} from admin")
	}
	if app.engine.GetPhase() != game.PhaseReady {
		t.Errorf("Activating ENTRACTE must not touch the phase, got %s", app.engine.GetPhase())
	}

	// --- 2. LEDs éteintes (B5) -----------------------------------------
	off := offLEDPayload()
	for _, mac := range []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"} {
		s, ok := app.bumperLEDState[mac]
		if !ok {
			t.Fatalf("Expected an LED state for %s after entering ENTRACTE", mac)
		}
		if s != off {
			t.Errorf("Bumper %s: expected OFF payload %+v after entering ENTRACTE, got %+v", mac, off, s)
		}
	}

	// --- 3. Refus d'un START pendant l'entracte (D6) --------------------
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 0})

	if app.engine.GetPhase() != game.PhaseReady {
		t.Errorf("START must be REFUSED during ENTRACTE — phase changed to %s", app.engine.GetPhase())
	}
	if !app.engine.IsEntracte() {
		t.Error("ENTRACTE must still be active after a refused START")
	}
	// LEDs must remain OFF — a refused action must have zero side effect.
	for _, mac := range []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"} {
		if s := app.bumperLEDState[mac]; s != off {
			t.Errorf("Bumper %s: LED state changed by a refused START, got %+v", mac, s)
		}
	}

	// --- 4. Désactivation (admin) ---------------------------------------
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: false})

	if app.engine.IsEntracte() {
		t.Fatal("Expected ENTRACTE to be inactive after ENTRACTE_SET{Active:false}")
	}

	// --- 5. LEDs restaurées (plus OFF, recalculées depuis la phase) -----
	for _, mac := range []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"} {
		s, ok := app.bumperLEDState[mac]
		if !ok {
			t.Fatalf("Expected an LED state for %s after exiting ENTRACTE", mac)
		}
		if s == off {
			t.Errorf("Bumper %s: LED state still OFF after exiting ENTRACTE — expected it restored from the current phase", mac)
		}
	}

	// --- 6. La sélection de question (état hors-ENTRACTE) est intacte ---
	if app.engine.GetPhase() != game.PhaseReady {
		t.Errorf("Phase must be unchanged by the whole ENTRACTE cycle, got %s", app.engine.GetPhase())
	}
}

// TestEntracteIntegration_SetFromNonAdmin_Refused covers the acceptance
// criterion "ENTRACTE_SET envoyé par un client tv, player ou anim est
// refusé (admin uniquement)" — contracts/websocket-actions.md §ENTRACTE_SET.
func TestEntracteIntegration_SetFromNonAdmin_Refused(t *testing.T) {
	for _, ct := range []server.ClientType{server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim} {
		t.Run(string(ct), func(t *testing.T) {
			app := setupEntracteIntegrationTestApp(t)

			dispatchAs(t, app, ct, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})

			if app.engine.IsEntracte() {
				t.Errorf("ENTRACTE_SET from client type %q must be refused (admin-only)", ct)
			}
		})
	}
}

// TestEntracteIntegration_AllowedActionsStillWork verifies the closed
// allow-list (D6) lets ENTRACTE_SET itself through while entracte is
// active — otherwise the mode would have no exit (risk table "Entracte sans
// issue").
func TestEntracteIntegration_AllowedActionsStillWork(t *testing.T) {
	app := setupEntracteIntegrationTestApp(t)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})
	if !app.engine.IsEntracte() {
		t.Fatal("setup: expected ENTRACTE to be active")
	}

	// The exit action itself must always reach its handler while ENTRACTE
	// is active — this is the one action D6 cannot block without making
	// the mode a dead end.
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: false})

	if app.engine.IsEntracte() {
		t.Error("ENTRACTE_SET{Active:false} must always be allowed through the D6 gate — the mode must never be a dead end")
	}
}

// TestEntracteIntegration_ExitAllowedFromAnyPhase covers "La sortie
// d'entracte fonctionne depuis n'importe quelle phase" (acceptance
// criteria, game-state.md §Phases autorisées) — deactivation must succeed
// even from a phase where ENTRY would have been refused, since entracte and
// phase are two independent pieces of state once entered.
func TestEntracteIntegration_ExitAllowedFromAnyPhase(t *testing.T) {
	app := setupEntracteIntegrationTestApp(t)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})
	if !app.engine.IsEntracte() {
		t.Fatal("setup: expected ENTRACTE to be active")
	}

	// Force a phase that would refuse ENTRY (defensive: the server guard
	// prevents reaching STARTED while entracte is active in practice, but
	// SetEntracte's own deactivation branch must not re-check the phase).
	app.engine.SetPhase(game.PhaseStarted)

	if !app.engine.SetEntracte(false) {
		t.Error("Deactivation must succeed from ANY phase, including one where entry would be refused")
	}
	if app.engine.IsEntracte() {
		t.Error("Expected ENTRACTE inactive after a successful deactivation")
	}
}
