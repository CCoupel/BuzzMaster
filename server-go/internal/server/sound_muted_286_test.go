// #286 (v11.4.0, LOT-1) — `sound.muted` : son coupé, persisté, écrit par POST /config.json.
// Contrat : contracts/sound.md §11, contracts/http-endpoints.md §Sound.
//
// Tests par la FRONTIÈRE (JSON/HTTP/réflexion par nom), jamais par un symbole pas encore livré
// (docs/TEST_PROCEDURE.md §2.3) : le fichier compile avant le dev et échoue par assertion.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"buzzcontrol/internal/config"
)

// tw286PostConfig POSTe un patch partiel sur /config.json.
func tw286PostConfig(t *testing.T, s *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /config.json %s : attendu 200, got %d : %s", body, w.Code, w.Body.String())
	}
	return w
}

// tw286SoundSection renvoie la section "sound" d'un JSON de config (GET /config.json ou fichier).
func tw286SoundSection(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("JSON de configuration invalide : %v\n%s", err, raw)
	}
	sec, ok := parsed["sound"].(map[string]interface{})
	if !ok {
		t.Fatalf("section \"sound\" absente ou mal formée : %s", raw)
	}
	return sec
}

func tw286GetConfigSound(t *testing.T, s *HTTPServer) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /config.json : attendu 200, got %d", w.Code)
	}
	return tw286SoundSection(t, w.Body.Bytes())
}

// tw286StatusMuted lit `muted` et `active` de GET /api/sound/status.
func tw286Status(t *testing.T, s *HTTPServer) (active bool, muted interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/sound/status", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/sound/status : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	a, _ := resp["active"].(bool)
	return a, resp["muted"]
}

// §11.2 « Lecture » : GET /api/sound/status → {active, muted}, le champ `muted` est toujours présent.
func TestSoundMuted286_Status_ExposesMuted_DefaultFalse(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	active, muted := tw286Status(t, s)
	if !active {
		t.Error("active doit rester vrai (sortie disponible), indépendant de muted")
	}
	if muted != false {
		t.Errorf("muted doit être présent et false par défaut dans /api/sound/status, got %v", muted)
	}
}

// §11.2 « Écriture » : POST /config.json {"sound":{"muted":true}} puis lecture par /api/sound/status.
func TestSoundMuted286_PostConfig_Muted_ReflectedInStatus(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	if _, muted := tw286Status(t, s); muted != true {
		t.Fatalf("après POST muted:true, /api/sound/status.muted doit être true, got %v", muted)
	}
	// Le rétablissement est immédiat et ne nécessite aucun redémarrage.
	tw286PostConfig(t, s, `{"sound":{"muted":false}}`)
	if _, muted := tw286Status(t, s); muted != false {
		t.Fatalf("après POST muted:false, /api/sound/status.muted doit être false, got %v", muted)
	}
}

// §11.2 : `muted` est indépendant d'`active` — sortie indisponible ET coupé = (false, true).
func TestSoundMuted286_Status_MutedIndependentOfActive(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: false}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	active, muted := tw286Status(t, s)
	if active {
		t.Error("sortie indisponible ⇒ active=false")
	}
	if muted != true {
		t.Errorf("muted doit refléter la config même si active=false, got %v", muted)
	}
}

// §11.2 « Persisté » : l'écriture atteint config.json sur disque.
func TestSoundMuted286_PostConfig_Muted_PersistedToDisk(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)

	raw, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatalf("config.json non écrit (%s) : %v", config.ConfigPath(), err)
	}
	if sec := tw286SoundSection(t, raw); sec["muted"] != true {
		t.Fatalf("sound.muted doit être persisté à true dans config.json, got %v", sec["muted"])
	}
}

// §11.2 « Persisté : un redémarrage conserve le son coupé » — rechargement depuis le fichier.
func TestSoundMuted286_Muted_SurvivesRestart_LoadFromFile(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}
	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)

	loaded, err := config.Load(config.ConfigPath())
	if err != nil {
		t.Fatalf("config.Load(%s) : %v", config.ConfigPath(), err)
	}
	// Frontière : on re-sérialise la config rechargée, sans référencer le champ.
	raw, err := json.Marshal(loaded)
	if err != nil {
		t.Fatalf("marshal : %v", err)
	}
	if sec := tw286SoundSection(t, raw); sec["muted"] != true {
		t.Fatalf("après redémarrage (Load), sound.muted doit rester true, got %v", sec["muted"])
	}
}

// §11.1 / convention `CuesDisabled` : valeur zéro = actif, clé absente (omitempty) — aucune migration.
func TestSoundMuted286_Default_KeyAbsentFromSavedConfig(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	tw286PostConfig(t, s, `{"sound":{"muted":false}}`)

	raw, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatalf("config.json non écrit : %v", err)
	}
	if sec := tw286SoundSection(t, raw); sec["muted"] != nil {
		t.Errorf("muted=false doit être omis du fichier (omitempty, valeur zéro = actif), got %v", sec["muted"])
	}
}

// §6.3 « Fusion champ par champ » : un patch sans `muted` préserve la valeur ; et réciproquement.
func TestSoundMuted286_MergeFieldByField_MutedPreservedByOtherSoundPatch(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	tw286PostConfig(t, s, `{"sound":{"cues_disabled":{"perdu":true}}}`)

	sec := tw286GetConfigSound(t, s)
	if sec["muted"] != true {
		t.Errorf("un patch sans muted ne doit pas effacer muted, got %v", sec["muted"])
	}
	cd, _ := sec["cues_disabled"].(map[string]interface{})
	if cd["perdu"] != true {
		t.Errorf("cues_disabled doit être appliqué, got %v", sec["cues_disabled"])
	}

	// Réciproque : écrire muted ne touche pas cues_disabled.
	tw286PostConfig(t, s, `{"sound":{"muted":false}}`)
	sec = tw286GetConfigSound(t, s)
	cd, _ = sec["cues_disabled"].(map[string]interface{})
	if cd["perdu"] != true {
		t.Errorf("écrire muted ne doit pas effacer cues_disabled, got %v", sec["cues_disabled"])
	}
}

// §11.2 : un patch d'une AUTRE section ne touche pas `sound.muted`.
func TestSoundMuted286_OtherSectionPatch_KeepsMuted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	tw286PostConfig(t, s, `{"lighting":{"enabled":false}}`)

	if sec := tw286GetConfigSound(t, s); sec["muted"] != true {
		t.Errorf("un POST de la section lighting ne doit pas réinitialiser sound.muted, got %v", sec["muted"])
	}
}

// §11.2 : POST /api/sounds/{cue}/test ignore `muted` (geste explicite de l'utilisateur).
func TestSoundMuted286_CueTest_StillPlaysWhenMuted(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}
	s.Sound = fake

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	if _, muted := tw286Status(t, s); muted != true {
		t.Fatalf("pré-requis : muted doit être true, got %v", muted)
	}

	w := tw230Post(t, s, "/api/sounds/depart/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("test de cue coupé : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp struct {
		Result string `json:"result"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Result != "played" || len(fake.testCalls) != 1 {
		t.Errorf("/test doit jouer même coupé : result=%q, appels=%d", resp.Result, len(fake.testCalls))
	}
}

// §11.2 « Effet de bord » : le passage à true appelle OnSoundMutedChange(true) ; le passage à false
// l'appelle avec false (mais ne reprend rien — côté App). Le champ est cherché PAR NOM (réflexion)
// pour que ce fichier compile avant la livraison du dev.
func TestSoundMuted286_PostConfig_CallsOnSoundMutedChange_OnlyOnChange(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	field := reflect.ValueOf(s).Elem().FieldByName("OnSoundMutedChange")
	if !field.IsValid() {
		t.Fatal("HTTPServer.OnSoundMutedChange absent : le rappel de §11.2 n'est pas livré")
	}
	var calls []bool
	cb := func(muted bool) { calls = append(calls, muted) }
	if !reflect.TypeOf(cb).AssignableTo(field.Type()) {
		t.Fatalf("OnSoundMutedChange doit avoir la signature func(bool), got %s", field.Type())
	}
	field.Set(reflect.ValueOf(cb))

	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	if len(calls) != 1 || calls[0] != true {
		t.Fatalf("muted false→true : un appel OnSoundMutedChange(true) attendu, got %v", calls)
	}
	// Réécrire la même valeur n'est pas un changement.
	tw286PostConfig(t, s, `{"sound":{"muted":true}}`)
	if len(calls) != 1 {
		t.Errorf("muted inchangé : aucun nouvel appel attendu, got %v", calls)
	}
	// Un patch sans muted n'est pas un changement.
	tw286PostConfig(t, s, `{"sound":{"cues_disabled":{"depart":true}}}`)
	if len(calls) != 1 {
		t.Errorf("patch sans muted : aucun appel attendu, got %v", calls)
	}
	tw286PostConfig(t, s, `{"sound":{"muted":false}}`)
	if len(calls) != 2 || calls[1] != false {
		t.Errorf("muted true→false : OnSoundMutedChange(false) attendu, got %v", calls)
	}
}

// §11.4 : interrupteur général de l'éclairage — `enabled=false` ⇒ commandes d'ambiance refusées 409 not_configured.
// (La coupure effective des écritures Hue et l'extinction sont testées côté App : cmd/server/lighting_master_switch_286_test.go.)
func TestLightingMasterSwitch286_PostEnabledFalse_RoundTripAndStatus(t *testing.T) {
	s, _ := setupTestHTTPServer(t)

	tw286PostConfig(t, s, `{"lighting":{"enabled":true,"bridge_ip":"192.168.1.101","bridge_id":"001788fffea0591e"}}`)
	tw286PostConfig(t, s, `{"lighting":{"enabled":false}}`)

	if config.Get().Lighting.Enabled {
		t.Fatal("lighting.enabled doit être false après POST {\"lighting\":{\"enabled\":false}}")
	}
	// Un patch partiel d'`enabled` ne doit pas effacer l'association au pont.
	if got := config.Get().Lighting; got.BridgeIP != "192.168.1.101" || got.BridgeID != "001788fffea0591e" {
		t.Errorf("le patch enabled seul ne doit pas effacer le pont (fusion champ par champ), got %+v", got)
	}

	req := httptest.NewRequest("GET", "/api/lighting/status", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	var st map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("/api/lighting/status non JSON : %v", err)
	}
	if st["enabled"] != false {
		t.Errorf("/api/lighting/status.enabled doit être false (l'en-tête grise sa section), got %v", st["enabled"])
	}

	for _, ep := range []struct{ url, body string }{
		{"/api/lighting/mode", `{"mode":"ON"}`},
		{"/api/lighting/flash", `{"on":true}`},
	} {
		req := httptest.NewRequest("POST", ep.url, strings.NewReader(ep.body))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "not_configured") {
			t.Errorf("%s avec éclairage désactivé : attendu 409 not_configured, got %d %s", ep.url, w.Code, w.Body.String())
		}
	}
}

// §11.4 : l'état « activé » est diffusé aux clients via /api/lighting/status.enabled=true une fois ré-activé.
func TestLightingMasterSwitch286_PostEnabledTrue_StatusEnabled(t *testing.T) {
	s, _ := setupTestHTTPServer(t)

	tw286PostConfig(t, s, `{"lighting":{"enabled":false}}`)
	tw286PostConfig(t, s, `{"lighting":{"enabled":true}}`)

	req := httptest.NewRequest("GET", "/api/lighting/status", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	var st map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if st["enabled"] != true {
		t.Errorf("/api/lighting/status.enabled doit être true après réactivation, got %v", st["enabled"])
	}
}
