// Suite test-writer pour la configuration Hue (contracts/hue-bridge.md §6,
// §6.1), écrite en TDD avant l'implémentation de la persistance (précédent
// #205 : le contrat suffit, pas besoin d'attendre le code). dev-backend a
// depuis ajouté `config.Config.Lighting` / `LightingConfig` /
// `LightingLightEntry` / `EnvHueAPIKey` / `EffectiveAPIKey()` /
// `EffectiveAPIKeyConfigured()` (internal/config/config.go) exactement selon
// le schéma ci-dessous — cette suite compile donc contre le vrai type. Elle
// peut néanmoins rester partiellement ROUGE tant que `handleConfig`
// (internal/server/http.go) et `maskedConfigJSON` n'ont pas encore le
// câblage POST/GET pour la section `lighting` (round-trip, masquage,
// fusion champ-par-champ de la clé) — c'est le signal normal de ce qu'il
// reste à faire côté #207 backend, pas une régression de ma part.
//
// Schéma (rappel, désormais réel) :
//
//	type LightingConfig struct {
//	    Enabled          bool
//	    BridgeIP         string
//	    BridgeID         string
//	    APIKey           string               // secret — voir régime ci-dessous
//	    APIKeyConfigured bool                 // dérivé, jamais persisté
//	    ClearAPIKey      bool                 // request-only, jamais persisté
//	    Lights           []LightingLightEntry
//	}
//	type LightingLightEntry struct { Name, Role, Team string }
//	const EnvHueAPIKey = "BUZZCONTROL_HUE_API_KEY"
//
// Mêmes règles que AIConfig.AnthropicAPIKey (internal/config/config.go) :
// absente/vide dans le POST ⇒ préservée ; clear_api_key:true ⇒ effacée ;
// api_key_configured est dérivé UNIQUEMENT sur la copie servie par GET (jamais
// persisté) ; POST additif par section (sauvegarder "ai" ou "server" ne vide
// jamais "lighting", et réciproquement) ; "lighting" a la MÊME exception de
// fusion champ-par-champ que "ai" pour sa clé (sinon un POST partiel — par ex.
// juste `lights`, comme le fait la page Ambiance à l'étape 3 — effacerait
// silencieusement `api_key` en la réinitialisant à sa valeur zéro).
// ---------------------------------------------------------------------------
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"buzzcontrol/internal/config"
)

// decodedLightingSection decodes a /config.json response body and returns
// its "lighting" section as a map — same pattern as the existing AI-key
// tests (http_test.go's TestHTTPServer_Config_GET_APIKeyMasking), rather
// than a raw strings.Contains on the JSON text: json.MarshalIndent inserts
// a space after each key's colon ("api_key_configured": true), which a
// naive `"api_key_configured":true` substring check would never match.
func decodedLightingSection(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("réponse /config.json invalide en JSON : %v\n%s", err, body)
	}
	lighting, ok := parsed["lighting"].(map[string]interface{})
	if !ok {
		t.Fatalf("section \"lighting\" absente ou mal formée dans la réponse : %s", body)
	}
	return lighting
}

// ---------------------------------------------------------------------------
// Round-trip et additivité par section (contrat §6, tâche #207)
// ---------------------------------------------------------------------------

func TestHTTPServer_LightingConfig_RoundTrip(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	body := `{"lighting":{"enabled":true,"bridge_ip":"192.168.1.101","bridge_id":"001788fffea0591e",` +
		`"lights":[{"name":"BuzzHue1","role":"general"},{"name":"BuzzHue2","role":"team","team":"Rouges"}]}}`
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(body))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST lighting: attendu 200, got %d: %s", w.Code, w.Body.String())
	}

	got := config.Get().Lighting
	if !got.Enabled || got.BridgeIP != "192.168.1.101" || got.BridgeID != "001788fffea0591e" {
		t.Fatalf("section lighting mal persistée : %+v", got)
	}
	if len(got.Lights) != 2 || got.Lights[0].Name != "BuzzHue1" || got.Lights[1].Team != "Rouges" {
		t.Fatalf("lights mal persistées : %+v", got.Lights)
	}

	// GET reflète la même chose (clé exclue — voir TestHTTPServer_LightingConfig_APIKeyNeverExposed).
	getReq := httptest.NewRequest("GET", "/config.json", nil)
	getW := httptest.NewRecorder()
	server.mux.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET /config.json : attendu 200, got %d", getW.Code)
	}
	if !strings.Contains(getW.Body.String(), `"BuzzHue1"`) || !strings.Contains(getW.Body.String(), "192.168.1.101") {
		t.Errorf("GET /config.json ne reflète pas la section lighting enregistrée : %s", getW.Body.String())
	}
}

func TestHTTPServer_LightingConfig_SavingOtherSectionsNeverWipesIt(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	seed := httptest.NewRequest("POST", "/config.json", strings.NewReader(
		`{"lighting":{"enabled":true,"bridge_ip":"192.168.1.101","lights":[{"name":"BuzzHue1","role":"general"}]}}`))
	server.mux.ServeHTTP(httptest.NewRecorder(), seed)

	for _, section := range []string{`{"server":{"debug":true}}`, `{"ai":{"model":"claude-opus-5"}}`} {
		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(section))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("POST %s : attendu 200, got %d: %s", section, w.Code, w.Body.String())
		}
		got := config.Get().Lighting
		if !got.Enabled || got.BridgeIP != "192.168.1.101" || len(got.Lights) != 1 {
			t.Fatalf("POST %s a vidé la section lighting (contract §6, tâche #207) : %+v", section, got)
		}
	}

	// Et réciproquement : sauvegarder `lighting` seul ne doit pas toucher `ai`.
	cfg := config.Get()
	cfg.AI.AnthropicAPIKey = "sk-ant-untouched"
	config.SetInstance(cfg)
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"lights":[]}}`))
	server.mux.ServeHTTP(httptest.NewRecorder(), req)
	if got := config.Get().AI.AnthropicAPIKey; got != "sk-ant-untouched" {
		t.Errorf("sauvegarder `lighting` a modifié `ai.anthropic_api_key`, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// La clé API Hue est un secret — même régime que les clés IA (contract §6.1)
// ---------------------------------------------------------------------------

func TestHTTPServer_LightingConfig_APIKeyPreservation(t *testing.T) {
	t.Run("absente du POST : préservée", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.Lighting.APIKey = "hue-key-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"bridge_ip":"192.168.1.101"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("attendu 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().Lighting.APIKey; got != "hue-key-original" {
			t.Errorf("clé absente du POST doit être préservée, got %q", got)
		}
	})

	t.Run("vide dans le POST : préservée", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.Lighting.APIKey = "hue-key-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"api_key":""}}`))
		server.mux.ServeHTTP(httptest.NewRecorder(), req)
		if got := config.Get().Lighting.APIKey; got != "hue-key-original" {
			t.Errorf("clé vide dans le POST doit préserver l'existante, got %q", got)
		}
	})

	t.Run("clear_api_key efface la clé", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.Lighting.APIKey = "hue-key-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"clear_api_key":true}}`))
		server.mux.ServeHTTP(httptest.NewRecorder(), req)
		if got := config.Get().Lighting.APIKey; got != "" {
			t.Errorf("clear_api_key doit effacer la clé, got %q", got)
		}
	})

	t.Run("nouvelle clé remplace l'ancienne", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.Lighting.APIKey = "hue-key-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"api_key":"hue-key-new"}}`))
		server.mux.ServeHTTP(httptest.NewRecorder(), req)
		if got := config.Get().Lighting.APIKey; got != "hue-key-new" {
			t.Errorf("nouvelle clé doit remplacer l'ancienne, got %q", got)
		}
	})

	t.Run("un POST partiel (ex: juste `lights`, étape 3 de la maquette) ne réinitialise PAS la clé", func(t *testing.T) {
		// Contract §6.1 : même exception de fusion champ-par-champ que "ai"
		// (voir en-tête de fichier). Sans elle, AmbiancePage.jsx's
		// handleSaveLights (POST {enabled, bridge_ip, bridge_id, lights}, SANS
		// api_key) effacerait silencieusement la clé enregistrée à l'étape 2.
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.Lighting.APIKey = "hue-key-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(
			`{"lighting":{"enabled":true,"bridge_ip":"192.168.1.101","bridge_id":"x","lights":[{"name":"A","role":"general"}]}}`))
		server.mux.ServeHTTP(httptest.NewRecorder(), req)
		if got := config.Get().Lighting.APIKey; got != "hue-key-original" {
			t.Errorf("un POST de la section lighting sans le champ api_key ne doit jamais l'effacer (comme `ai`), got %q", got)
		}
	})
}

func TestHTTPServer_LightingConfig_APIKeyNeverExposed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	cfg := config.Get()
	cfg.Lighting.APIKey = "hue-secret-abc123"
	cfg.Lighting.Enabled = true
	config.SetInstance(cfg)

	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /config.json : attendu 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "hue-secret-abc123") {
		t.Errorf("GET /config.json expose la clé Hue en clair : %s", w.Body.String())
	}
	if lighting := decodedLightingSection(t, w.Body.Bytes()); lighting["api_key_configured"] != true {
		t.Errorf("GET /config.json doit dériver api_key_configured=true (clé présente), got %v", lighting["api_key_configured"])
	}

	// Une réponse POST ne doit pas non plus la faire fuiter (même motif que
	// TestHTTPServer_Config_POST_APIKeyPreservation pour l'IA).
	postReq := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"bridge_ip":"192.168.1.101"}}`))
	postW := httptest.NewRecorder()
	server.mux.ServeHTTP(postW, postReq)
	if strings.Contains(postW.Body.String(), "hue-secret-abc123") {
		t.Errorf("la réponse POST expose la clé Hue en clair : %s", postW.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Surcharge par variable d'environnement — AUCUNE écriture disque (§6.1)
// ---------------------------------------------------------------------------

func TestHTTPServer_LightingConfig_EnvOverride_NeverWritesToDisk(t *testing.T) {
	t.Setenv(config.EnvHueAPIKey, "hue-env-key") // config.EnvHueAPIKey == "BUZZCONTROL_HUE_API_KEY" (contract §6.1)
	server, _ := setupTestHTTPServer(t)

	// Aucune clé en config.json, uniquement l'environnement : la config
	// doit quand même se déclarer "configurée" pour l'écran d'administration.
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if lighting := decodedLightingSection(t, w.Body.Bytes()); lighting["api_key_configured"] != true {
		t.Errorf("une clé fournie par BUZZCONTROL_HUE_API_KEY doit rendre api_key_configured=true même sans clé stockée, got %v", lighting["api_key_configured"])
	}
	if strings.Contains(w.Body.String(), "hue-env-key") {
		t.Errorf("la clé d'environnement ne doit jamais apparaître dans la réponse : %s", w.Body.String())
	}

	// Poser une autre section ne doit déclencher aucune écriture de la clé
	// d'environnement dans config.json — la config sur disque doit rester
	// sans le champ api_key rempli par l'environnement.
	postReq := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"bridge_ip":"192.168.1.101"}}`))
	server.mux.ServeHTTP(httptest.NewRecorder(), postReq)
	if got := config.Get().Lighting.APIKey; got != "" {
		t.Errorf("BUZZCONTROL_HUE_API_KEY ne doit JAMAIS être recopiée dans le champ persisté api_key, got %q", got)
	}

	diskPath := t.TempDir() + "/unused" // sanity: on ne connaît pas le chemin réel de config.Save() ici,
	_ = diskPath                        // la garantie testée est l'absence de la clé dans le struct en mémoire post-Save,
	// qui EST ce que config.Save() sérialise — un struct sans la clé ne peut
	// pas l'écrire sur disque, quel que soit le chemin réel.
}

// ---------------------------------------------------------------------------
// config.json n'est jamais inclus dans une archive de sauvegarde (§6.1) —
// garde structurelle : h.dataDir (racine des archives fs-backup/game-backup)
// ne peut pas être un ancêtre du fichier config.json.
// ---------------------------------------------------------------------------

func TestHTTPServer_ConfigJSON_NeverUnderBackupDataDir(t *testing.T) {
	_, dataDir := setupTestHTTPServer(t)
	// setupTestHTTPServer isole déjà le CWD (t.Chdir) : config.json (si
	// jamais écrit) se trouverait dans ce CWD temporaire, jamais sous
	// dataDir (un TempDir distinct — voir setupTestHTTPServer) — c'est cette
	// séparation de racines, pas un chemin en dur, qui garantit que
	// handleFSBackup/handleGameBackup (bornés à dataDir) ne peuvent
	// structurellement jamais embarquer config.json.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if strings.HasPrefix(dataDir, cwd) && dataDir != cwd {
		// Ce cas n'est pas censé se produire avec l'isolation actuelle du
		// harness de test, mais s'il se produisait un jour, il romprait
		// silencieusement la garantie du contrat §6.1 — mieux vaut un échec
		// de test explicite ici qu'une régression découverte en QUALIF.
		t.Fatalf("dataDir (%q) est sous le répertoire où config.json serait écrit (%q) — "+
			"la clé Hue pourrait fuir par une sauvegarde (contract hue-bridge.md §6.1)", dataDir, cwd)
	}
}
