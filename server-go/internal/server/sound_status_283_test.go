// #283 (v11.6.0, LOT-1) — GET /api/sound/status : diagnostic de la sortie audio.
// Source normative : contracts/http-endpoints.md « GET /api/sound/status — diagnostic (#283, LOT-1) ».
// Champs : active, muted (inchangés) + enabled, output ∈ {ok, unavailable, disabled, restart_required},
// reason (string|null, jamais omis), restart_required, muted_source.
//
// Le fournisseur de diagnostic (interface optionnelle de SoundProvider, détectée par assertion de type) est
// simulé par un double : le serveur HTTP ne fait que sérialiser. Hypothèse de signature à confirmer par
// dev-backend (cmd/server/sound.go : `SoundDiagnostic() (enabled bool, output, reason string, restartRequired bool)`) ;
// si elle change, ce SEUL fichier est à ajuster. Préfixe tw283.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type tw283Provider struct {
	tw230eFakeSoundProvider
	enabled         bool
	output, reason  string
	restartRequired bool
}

func (p *tw283Provider) SoundDiagnostic() (bool, string, string, bool) {
	return p.enabled, p.output, p.reason, p.restartRequired
}

func tw283Status(t *testing.T, s *HTTPServer) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/sound/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return m
}

func TestSoundStatus283_AllFieldsAlwaysPresent_ReasonNullWhenOK(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw283Provider{
		tw230eFakeSoundProvider: tw230eFakeSoundProvider{enabled: true, outputAvailable: true},
		enabled:                 true, output: "ok",
	}
	m := tw283Status(t, s)
	for _, k := range []string{"active", "muted", "enabled", "output", "reason", "restart_required", "muted_source"} {
		if _, ok := m[k]; !ok {
			t.Errorf("champ %q absent (aucun champ omis — contrat)", k)
		}
	}
	if m["output"] != "ok" {
		t.Errorf("output attendu \"ok\", got %v", m["output"])
	}
	if m["reason"] != nil {
		t.Errorf("reason attendu null si output=ok, got %v", m["reason"])
	}
	if m["restart_required"] != false {
		t.Errorf("restart_required attendu false, got %v", m["restart_required"])
	}
	if m["enabled"] != true || m["active"] != true {
		t.Errorf("enabled/active attendus true, got %v / %v", m["enabled"], m["active"])
	}
	if m["muted_source"] != "global" {
		t.Errorf("muted_source attendu \"global\" (tant que LOT-4 n'est pas livré), got %v", m["muted_source"])
	}
}

func TestSoundStatus283_Unavailable_ReasonIsReadableCause(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw283Provider{
		tw230eFakeSoundProvider: tw230eFakeSoundProvider{enabled: true, outputAvailable: false},
		enabled:                 true, output: "unavailable", reason: "oto: no output device",
	}
	m := tw283Status(t, s)
	if m["output"] != "unavailable" || m["reason"] != "oto: no output device" {
		t.Errorf("output=unavailable et reason repris tel quel attendus, got %v / %v", m["output"], m["reason"])
	}
	if m["active"] != false {
		t.Errorf("active=false attendu (sortie indisponible), got %v", m["active"])
	}
	if m["restart_required"] != false {
		t.Errorf("restart_required=false attendu pour unavailable, got %v", m["restart_required"])
	}
}

func TestSoundStatus283_Disabled(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw283Provider{
		tw230eFakeSoundProvider: tw230eFakeSoundProvider{enabled: false, outputAvailable: false},
		enabled:                 false, output: "disabled", reason: "sound.enabled=false",
	}
	m := tw283Status(t, s)
	if m["output"] != "disabled" || m["reason"] != "sound.enabled=false" || m["enabled"] != false {
		t.Errorf("disabled / sound.enabled=false / enabled=false attendus, got %v", m)
	}
	if m["active"] != false || m["restart_required"] != false {
		t.Errorf("active=false et restart_required=false attendus, got %v / %v", m["active"], m["restart_required"])
	}
}

// restart_required : `output == "restart_required"` ET le booléen de commodité sont cohérents.
func TestSoundStatus283_RestartRequired_OutputAndBooleanConsistent(t *testing.T) {
	for _, flip := range []struct {
		name, reason string
		bootEnabled  bool
	}{
		{"false→true", "enabled flipped false→true after startup", false},
		{"true→false", "enabled flipped true→false after startup", true},
	} {
		t.Run(flip.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			s.Sound = &tw283Provider{
				tw230eFakeSoundProvider: tw230eFakeSoundProvider{enabled: !flip.bootEnabled, outputAvailable: flip.bootEnabled},
				enabled:                 flip.bootEnabled, output: "restart_required", reason: flip.reason, restartRequired: true,
			}
			m := tw283Status(t, s)
			if m["output"] != "restart_required" || m["restart_required"] != true {
				t.Errorf("output=restart_required et restart_required=true attendus, got %v / %v", m["output"], m["restart_required"])
			}
			if m["reason"] != flip.reason {
				t.Errorf("reason attendu %q, got %v", flip.reason, m["reason"])
			}
			if m["enabled"] != flip.bootEnabled {
				t.Errorf("enabled = valeur LUE AU DÉMARRAGE (%v), got %v", flip.bootEnabled, m["enabled"])
			}
		})
	}
}

// Rétrocompatibilité : un fournisseur sans diagnostic ne casse pas les champs historiques.
func TestSoundStatus283_ProviderWithoutDiagnostic_KeepsLegacyFields(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}
	m := tw283Status(t, s)
	if m["active"] != true {
		t.Errorf("active=true attendu, got %v", m["active"])
	}
	if _, ok := m["muted"]; !ok {
		t.Error("muted doit rester présent (v11.4.0)")
	}
}

func TestSoundStatus283_RejectsNonGET(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	s.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/sound/status", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("attendu 405, got %d", w.Code)
	}
}
