package main

// Bugfix — QUALIF v10.0.0.8, bug 2 (retour utilisateur, réel pont Hue) :
// _work/handoff/task-dev-backend-bugfix-qualif-v10-20260907.md.
//
// Reproduction utilisateur : pont associé (ampoules détectées, fonctionnel),
// puis arrêt/relance du serveur → "pont injoignable", obligeant à dissocier
// et réassocier depuis zéro. Exigence explicite : "L'association doit
// survivre à un redémarrage !" — comme les buzzers.
//
// docs/SERVER_PARAMETERS.md (doc-updater, Batch 4) affirme : "La clé API du
// pont est un secret et n'est jamais sauvegardée en clair sur disque" /
// "La clé API n'est jamais écrite dans config.json — elle est gérée
// uniquement via mémoire ou environnement". Ce test vérifie empiriquement
// le round-trip réel (Save → redémarrage simulé → Load) plutôt que de se
// fier à la doc ou à une lecture de code seule.
//
// Ce test PILOTE UNIQUEMENT L'API PUBLIQUE existante (config.Save/Load/
// SetInstance, App.setupAmbiance) — pas de reach dans les champs privés.

import (
	"testing"

	"buzzcontrol/internal/config"
)

// TestDevQualifBug2_HueAssociationSurvivesRestart simulates a full
// "associate, then restart the process" cycle: what handleLightingRegister
// (register) and the generic POST /config.json (enable) already write to
// disk, reloaded exactly as a fresh process would at startup — App.init()
// calling config.Get() (which resolves via config.Load when no in-memory
// instance exists yet) and then App.setupAmbiance().
func TestDevQualifBug2_HueAssociationSurvivesRestart(t *testing.T) {
	saved := *config.Get() // snapshot the real singleton — restored below, never left nil for later tests
	t.Cleanup(func() { config.SetInstance(&saved) })

	tmpPath := t.TempDir() + "/config.json"
	config.SetConfigPath(tmpPath)
	t.Cleanup(func() { config.SetConfigPath("config.json") })

	// --- Session 1 : association (register) + activation (enable) ---
	cfg := config.Config{}
	config.ApplyDefaults(&cfg)
	// Ce que handleLightingRegister (http_lighting.go) écrit après un
	// appairage réussi.
	cfg.Lighting.BridgeIP = "192.168.1.101"
	cfg.Lighting.BridgeID = "001788fffea0591e"
	cfg.Lighting.APIKey = "deadbeef1234567890deadbeef123456"
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "Salon", Role: "general"}}
	// Ce que le POST /config.json générique écrit quand l'utilisateur active
	// l'éclairage depuis l'écran Ambiance (handleConfig, section "lighting",
	// clé "enabled" présente dans le corps).
	cfg.Lighting.Enabled = true
	if err := config.Save(&cfg); err != nil {
		t.Fatalf("Save (session 1): %v", err)
	}

	// --- « Redémarrage » : nouveau process, aucun état en mémoire ---
	// config.Load lit le fichier écrit ci-dessus — c'est EXACTEMENT ce que
	// config.Get() ferait au tout premier appel d'un process qui démarre
	// (chemin `once.Do` de Get(), internal/config/config.go).
	loaded, err := config.Load(tmpPath)
	if err != nil {
		t.Fatalf("Load after restart: %v", err)
	}
	config.SetInstance(loaded)

	if loaded.Lighting.APIKey == "" {
		t.Fatal("QUALIF bug 2 : la clé API Hue n'a pas survécu au redémarrage (Load renvoie APIKey vide) — l'association devra être refaite entièrement")
	}
	if loaded.Lighting.APIKey != cfg.Lighting.APIKey {
		t.Fatalf("clé API altérée par le cycle Save/Load : got %q, want %q", loaded.Lighting.APIKey, cfg.Lighting.APIKey)
	}
	if !loaded.Lighting.Enabled {
		t.Fatal("QUALIF bug 2 : lighting.enabled n'a pas survécu au redémarrage")
	}
	if loaded.Lighting.BridgeIP != cfg.Lighting.BridgeIP || loaded.Lighting.BridgeID != cfg.Lighting.BridgeID {
		t.Fatalf("bridge_ip/bridge_id altérés : got ip=%q id=%q, want ip=%q id=%q",
			loaded.Lighting.BridgeIP, loaded.Lighting.BridgeID, cfg.Lighting.BridgeIP, cfg.Lighting.BridgeID)
	}

	// --- La conséquence qui compte réellement pour l'utilisateur : le
	// serveur doit reconstruire un driver Hue utilisable sans aucun geste
	// manuel, exactement comme au moment de l'association initiale. ---
	app := newTestApp(t)
	app.config = loaded
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("QUALIF bug 2 : setupAmbiance() ne reconstruit PAS le driver Hue depuis la config rechargée — l'association apparaît perdue au redémarrage alors que la config elle-même a bien survécu")
	}
	if !app.ambianceIsConfigured() {
		t.Fatal("ambianceIsConfigured() doit être vrai après un redémarrage avec une association déjà sauvegardée")
	}
}
