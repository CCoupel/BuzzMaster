package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

func tw286ReviewSetup(t *testing.T, enabled bool) (*App, *tw286Bridge) {
	t.Helper()
	bridge := &tw286Bridge{}
	srv := httptest.NewServer(bridge.handler())
	t.Cleanup(srv.Close)
	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: enabled, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "L1", Role: "general"}}}
	config.SetInstance(&cfg)
	return app, bridge
}

// Revue LOT-1, MAJEUR : à l'arrêt du serveur, lighting.enabled=false ⇒ ampoules intactes.
func TestLightingExtinction286_Shutdown_EnabledFalse_NoBridgeWrite(t *testing.T) {
	app, bridge := tw286ReviewSetup(t, false)
	app.shutdownExtinguishHueLighting()
	time.Sleep(200 * time.Millisecond)
	if n := bridge.writeCount(); n != 0 {
		t.Fatalf("shutdown avec enabled=false : aucune écriture attendue, got %d", n)
	}
}

// Contrôle positif : enabled=true ⇒ l'extinction d'arrêt (§10.4) écrit bien au pont.
func TestLightingExtinction286_Shutdown_EnabledTrue_Extinguishes(t *testing.T) {
	app, bridge := tw286ReviewSetup(t, true)
	app.shutdownExtinguishHueLighting()
	if n := bridge.writeCount(); n == 0 {
		t.Fatal("shutdown avec enabled=true : l'extinction doit écrire au pont")
	}
}

// Revue LOT-1, MINEUR : off→on rapide — l'extinction asynchrone ne doit pas éteindre derrière le nouveau pilote.
func TestLightingExtinction286_FastOffOn_SkipsExtinction(t *testing.T) {
	app, bridge := tw286ReviewSetup(t, true)
	app.extinguishOnLightingDisabled() // appelé alors que la config est déjà redevenue enabled=true
	time.Sleep(300 * time.Millisecond)
	if n := bridge.writeCount(); n != 0 {
		t.Fatalf("réactivation avant l'extinction : aucune écriture attendue, got %d", n)
	}
}

// Contrôle positif : enabled=false ⇒ l'extinction asynchrone écrit au pont.
func TestLightingExtinction286_Disabled_ExtinguishesAsync(t *testing.T) {
	app, bridge := tw286ReviewSetup(t, false)
	app.extinguishOnLightingDisabled()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && bridge.writeCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if bridge.writeCount() == 0 {
		t.Fatal("enabled=false : l'extinction doit écrire au pont")
	}
}
