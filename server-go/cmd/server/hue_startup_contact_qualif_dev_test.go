package main

// Bugfix — QUALIF v10.0.0.10, round 2, bug 2 (retour utilisateur confirmé
// avec une association FRAÎCHE refaite sur le binaire corrigé du round 1) :
// _work/handoff/task-dev-backend-bugfix-round2-20260907.md.
//
// Piste confirmée : l'architecture "sans poller" du module Ambiance
// (contract lighting.md §4) s'applique aussi au DÉMARRAGE du serveur, pas
// seulement en cours de fonctionnement — rien ne déclenchait le premier
// contact réel avec le pont tant qu'aucun événement de jeu ne survenait.
// Voir (*App).startAmbianceWriter (cmd/server/ambiance.go) pour le
// correctif ; ce fichier prouve qu'il fonctionne bout en bout contre un
// vrai *hue.Driver et un faux pont réellement joignable — sans jamais
// appeler NotifyState() depuis le test lui-même (ce qui retomberait dans
// l'angle mort qui a laissé passer round 1 : un test qui déclenche
// lui-même le contact ne teste pas si le DÉMARRAGE le fait tout seul).

import (
	"context"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting/hue"
)

// TestDevQualifRound2Bug2_StartupAloneReachesBridgeWithoutAnyGameEvent is
// the direct regression test for the missing startup kick: it calls ONLY
// (*App).setupAmbiance() (build) then (*App).startAmbianceWriter() (the
// REAL function (*App).start() now calls) — the exact same two calls
// App.init()/App.start() make in production — and asserts the driver
// reaches StateOK on its own. No NotifyState() call anywhere else in this
// test: if one were needed to make it pass, the test would not be
// exercising the fix.
func TestDevQualifRound2Bug2_StartupAloneReachesBridgeWithoutAnyGameEvent(t *testing.T) {
	srv, _ := newDevReconnectBridge(t, "Salon")

	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{
		Enabled: true, BridgeIP: srv.URL, APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "Salon", Role: "general"}},
	}
	config.SetInstance(&cfg)

	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	defer cancel()

	// Exactement ce qu'App.init() puis App.start() font en production —
	// aucun autre appel, en particulier AUCUN NotifyState() manuel ici.
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("setup invalide : le driver doit être construit depuis une config valide")
	}
	if got := app.LightingDriver().Status().State; got != hue.StateUnreachable {
		t.Fatalf("avant tout démarrage, le statut doit être la valeur zéro 'not contacted yet', got %v", got)
	}
	app.startAmbianceWriter()

	devWaitDriverState(t, app.LightingDriver(), hue.StateOK)
}
