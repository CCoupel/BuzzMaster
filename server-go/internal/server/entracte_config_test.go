package server

import (
	"buzzcontrol/internal/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// T2 ter (#119) — le piège documenté au plan
// (_work/reports/plan-entracte-119-20260820-140825.md, B6) :
// ApplyGameSettingsDefaults remplit tout champ "à zéro" par son défaut. Sans
// traitement à part, ANIM_INTENSITY=0 (désactivation explicite du panneau)
// serait ré-écrasé à 20 dès le prochain POST /game-config.json — rendant
// "désactiver l'animation" impossible à enregistrer durablement.
//
// internal/config/gameconfig.go traite déjà AnimIntensity comme *int (nil =
// absent -> défaut ; non-nil pointant sur 0 = désactivation explicite,
// conservée) — ce fichier verrouille ce comportement au niveau HTTP, à
// travers un aller-retour sauvegarde -> rechargement réel (pas seulement en
// mémoire), comme demandé par le plan ("test serveur dédié").
// ---------------------------------------------------------------------------

func TestHTTPServer_GameConfig_POST_EntracteAnimIntensityZero_Survives(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{
		"entracte": {
			"title": "Pause déjeuner",
			"subtitle": "Retour à 13h30",
			"panel_size": 70,
			"anim_period": 8,
			"anim_intensity": 0
		}
	}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	entracte, ok := gs["entracte"].(map[string]interface{})
	if !ok {
		t.Fatalf("entracte section not found in response: %v", gs)
	}
	if entracte["anim_intensity"] != float64(0) {
		t.Errorf("Expected anim_intensity=0 in the immediate response (not re-defaulted to 20), got %v", entracte["anim_intensity"])
	}
	if entracte["title"] != "Pause déjeuner" {
		t.Errorf("Expected title to be saved, got %v", entracte["title"])
	}

	// Le vrai piège n'est visible qu'à un RECHARGEMENT réel depuis le disque
	// (game-config.json), pas seulement dans la réponse HTTP immédiate — donc
	// on relit indépendamment du singleton en mémoire, exactement comme un
	// redémarrage du serveur le ferait (config.LoadGameSettings, appelé par
	// GetGameSettings() au premier accès du process).
	reloaded, err := config.LoadGameSettings(config.GameConfigPath())
	if err != nil {
		t.Fatalf("LoadGameSettings failed: %v", err)
	}
	if reloaded.Entracte.AnimIntensity == nil {
		t.Fatalf("Expected AnimIntensity to be non-nil after reload, got nil (defaulted away)")
	}
	if *reloaded.Entracte.AnimIntensity != 0 {
		t.Errorf("Expected AnimIntensity=0 to survive a save -> reload round-trip, got %d (ré-écrasé par ApplyGameSettingsDefaults — le piège B6 documenté au plan)", *reloaded.Entracte.AnimIntensity)
	}
	if reloaded.Entracte.Title != "Pause déjeuner" {
		t.Errorf("Expected title to survive the round-trip, got %q", reloaded.Entracte.Title)
	}

	// Second aller-retour : un save/reload SUPPLÉMENTAIRE (ex. l'utilisateur
	// rouvre /settings et clique Enregistrer sans avoir touché le curseur)
	// ne doit toujours pas réactiver l'animation — "désactivée" doit être un
	// état STABLE, pas seulement survivre un seul cycle par chance.
	req2 := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{
		"entracte": {
			"title": "Pause déjeuner",
			"subtitle": "Retour à 13h30",
			"panel_size": 70,
			"anim_period": 8,
			"anim_intensity": 0
		}
	}`))
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200 on second save, got %d: %s", w2.Code, w2.Body.String())
	}
	reloaded2, err := config.LoadGameSettings(config.GameConfigPath())
	if err != nil {
		t.Fatalf("LoadGameSettings (2nd) failed: %v", err)
	}
	if reloaded2.Entracte.AnimIntensity == nil || *reloaded2.Entracte.AnimIntensity != 0 {
		t.Errorf("Expected AnimIntensity to remain 0 across a SECOND save/reload cycle, got %v", reloaded2.Entracte.AnimIntensity)
	}
}

// TestHTTPServer_GameConfig_POST_EntracteAnimIntensityOmitted_DefaultsTo20 is
// the control case: when the "entracte" section is present in the POST body
// but omits "anim_intensity" entirely (not "anim_intensity": 0, genuinely
// absent), the whole-section-replace semantics (contract http-endpoints.md,
// same rule as "game"/"neon_effect") mean the field IS re-defaulted to 20 —
// this is the intended behavior, not the B6 trap, and this test exists to
// keep the two cases from being confused during future changes.
func TestHTTPServer_GameConfig_POST_EntracteAnimIntensityOmitted_DefaultsTo20(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{"entracte": {"title": "X"}}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	entracte := gs["entracte"].(map[string]interface{})
	if entracte["anim_intensity"] != float64(20) {
		t.Errorf("Expected anim_intensity default (20) when the field is entirely absent from the section, got %v", entracte["anim_intensity"])
	}
}

// TestHTTPServer_GameConfig_GET_EntracteDefaults verifies GET
// /game-config.json returns the contractual defaults (game-state.md
// §ENTRACTE_CONFIG) when no entracte section has ever been saved.
func TestHTTPServer_GameConfig_GET_EntracteDefaults(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/game-config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	entracte, ok := gs["entracte"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected an entracte section in the response, got: %v", gs)
	}

	want := map[string]interface{}{
		"title":          "ENTRACTE",
		"subtitle":       "Retour dans 20mn",
		"panel_size":     float64(65),
		"anim_period":    float64(10),
		"anim_intensity": float64(20),
	}
	for k, v := range want {
		if entracte[k] != v {
			t.Errorf("Expected entracte.%s=%v, got %v", k, v, entracte[k])
		}
	}
}

// TestHTTPServer_GameConfig_POST_EntracteClamped verifies out-of-range
// values are clamped per contract http-endpoints.md (panel_size 20-100,
// anim_period 2-30, anim_intensity 0-100).
func TestHTTPServer_GameConfig_POST_EntracteClamped(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{
		"entracte": {
			"panel_size": 500,
			"anim_period": 1,
			"anim_intensity": 250
		}
	}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	entracte := gs["entracte"].(map[string]interface{})
	if entracte["panel_size"] != float64(100) {
		t.Errorf("Expected panel_size clamped to 100, got %v", entracte["panel_size"])
	}
	if entracte["anim_period"] != float64(2) {
		t.Errorf("Expected anim_period clamped to 2, got %v", entracte["anim_period"])
	}
	if entracte["anim_intensity"] != float64(100) {
		t.Errorf("Expected anim_intensity clamped to 100, got %v", entracte["anim_intensity"])
	}
}
