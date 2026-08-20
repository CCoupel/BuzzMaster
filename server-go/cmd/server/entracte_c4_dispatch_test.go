package main

import (
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// C4 — UPDATE_ENTRACTE_CONFIG à travers le dispatch réel (delta #119, plan
// _work/reports/plan-entracte-119-fixes-20260820-155123.md, tâches C1-B4/
// C4-B4). Complète entracte_c4_freeze_test.go (internal/game, mécanique
// pure) par la couverture bout-en-bout demandée par le handoff :
//   - UPDATE_ENTRACTE_CONFIG est admin uniquement (comme ENTRACTE_SET).
//   - UPDATE_ENTRACTE_CONFIG reste ACCEPTÉE pendant un entracte actif — elle
//     doit figurer dans entracteAllowedActions (D6), contrairement à la
//     quasi-totalité des autres actions.
//
// Réutilise dispatchAs/setupEntracteIntegrationTestApp
// (entracte_integration_test.go, même package).
// ---------------------------------------------------------------------------

func samplePayload(title string) protocol.EntracteConfigPayload {
	return protocol.EntracteConfigPayload{
		Title: title, Subtitle: "Retour bientôt",
		PanelSize: 70, AnimPeriod: 8,
	}
}

// TestEntracteC4Dispatch_UpdateConfig_AdminOnly mirrors
// TestEntracteIntegration_SetFromNonAdmin_Refused's pattern for the new
// action: tv/vplayer/anim must never be able to alter the saved config.
func TestEntracteC4Dispatch_UpdateConfig_AdminOnly(t *testing.T) {
	for _, ct := range []server.ClientType{server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim} {
		t.Run(string(ct), func(t *testing.T) {
			app := setupEntracteIntegrationTestApp(t)
			before := app.engine.GetState().EntracteConfigSaved

			dispatchAs(t, app, ct, protocol.ActionUpdateEntracteConfig, samplePayload("Tentative non-admin"))

			after := app.engine.GetState().EntracteConfigSaved
			if after != before {
				t.Errorf("UPDATE_ENTRACTE_CONFIG from client type %q must be refused (admin-only), but EntracteConfigSaved changed: got %+v, want unchanged %+v", ct, after, before)
			}
		})
	}
}

// TestEntracteC4Dispatch_UpdateConfig_AllowedFromAdmin is the positive case:
// an admin can always save, regardless of entracte state.
func TestEntracteC4Dispatch_UpdateConfig_AllowedFromAdmin(t *testing.T) {
	app := setupEntracteIntegrationTestApp(t)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionUpdateEntracteConfig, samplePayload("Sauvegardé par un admin"))

	got := app.engine.GetState().EntracteConfigSaved
	if got.Title != "Sauvegardé par un admin" {
		t.Errorf("expected EntracteConfigSaved.Title updated by an admin save, got %q", got.Title)
	}
}

// TestEntracteC4Dispatch_UpdateConfig_AllowedDuringActiveEntracte is the D6
// exception this delta introduces: unlike virtually every other action,
// UPDATE_ENTRACTE_CONFIG must NOT be blocked by the "entracte active" gate
// (cmd/server/main.go, server.IsActionAllowedDuringEntracte) — registering
// settings stays possible during a live pause (C4), only the DIFFUSED panel
// freezes, not the ability to save.
func TestEntracteC4Dispatch_UpdateConfig_AllowedDuringActiveEntracte(t *testing.T) {
	app := setupEntracteIntegrationTestApp(t)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})
	if !app.engine.IsEntracte() {
		t.Fatal("setup: expected entracte to be active")
	}
	before := app.engine.GetState().EntracteConfig // the diffused (frozen) config

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionUpdateEntracteConfig, samplePayload("Édité pendant la pause"))

	state := app.engine.GetState()
	if state.EntracteConfigSaved.Title != "Édité pendant la pause" {
		t.Errorf("UPDATE_ENTRACTE_CONFIG must be accepted while entracte is active (persisted to Saved), got title %q", state.EntracteConfigSaved.Title)
	}
	if state.EntracteConfig != before {
		t.Errorf("the DIFFUSED config must stay frozen during the pause even though the save was accepted: got %+v, want unchanged %+v", state.EntracteConfig, before)
	}
	if !app.engine.IsEntracte() {
		t.Error("entracte must still be active — saving config must not itself end the pause")
	}
}

// TestEntracteC4Dispatch_UpdateConfig_IsActionAllowedDuringEntracte is a
// narrower unit check on the D6 allow-list itself (server package), the
// exact wording of the handoff: "mise à jour de entracteAllowedActions —
// ne doit PAS être refusée par la garde D6".
func TestEntracteC4Dispatch_UpdateConfig_IsActionAllowedDuringEntracte(t *testing.T) {
	if !server.IsActionAllowedDuringEntracte(protocol.ActionUpdateEntracteConfig) {
		t.Error("expected UPDATE_ENTRACTE_CONFIG to be in the closed allow-list of actions permitted during an active entracte (D6)")
	}
}
