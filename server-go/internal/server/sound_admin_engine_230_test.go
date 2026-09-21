// #230 (v11.0) — POST /api/sounds/{cue}/test et GET /api/sound/status : les
// deux endpoints qui ont besoin d'un accès au moteur (via SoundProvider,
// internal/server/http_sound.go). Séparé de sound_admin_230_test.go
// (purement disque) pour ne pas l'avoir bloqué sur la forme de
// SoundProvider pendant la coordination avec dev-backend.
//
// T2 (planner handoff #230 §1) — LE test prioritaire de ce fichier :
// "Aucune réponse du serveur ne positionne le verdict, y compris
// result:"played"." Ce fichier ne teste PAS le verdict (qui est un état
// React, testé côté frontend — AmbiancePage.sound.test.jsx) : il teste que
// la RÉPONSE SERVEUR elle-même distingue bien les trois issues normatives,
// condition nécessaire pour que le frontend puisse honorer T2 — un serveur
// qui ne distinguerait pas played/disabled/unavailable rendrait le respect
// de T2 impossible côté client.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buzzcontrol/internal/audio"
)

// tw230eFakeSoundProvider is a minimal, fully-controllable SoundProvider
// double — lets each test dial in exactly one of the three taxonomy cases
// without needing a real audio.Engine/Output at all.
type tw230eFakeSoundProvider struct {
	enabled         bool
	outputAvailable bool
	testAccepted    bool
	testCalls       []audio.Cue
}

func (f *tw230eFakeSoundProvider) SoundEnabled() bool         { return f.enabled }
func (f *tw230eFakeSoundProvider) SoundOutputAvailable() bool { return f.outputAvailable }
func (f *tw230eFakeSoundProvider) TestSoundCue(c audio.Cue) bool {
	f.testCalls = append(f.testCalls, c)
	return f.testAccepted
}

// ---------------------------------------------------------------------------
// POST /api/sounds/{cue}/test — taxonomie à trois issues, normative.
// ---------------------------------------------------------------------------

func TestPOSTAPISoundsCueTest_Played(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}
	server.Sound = fake

	w := tw230Post(t, server, "/api/sounds/depart/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Result != "played" {
		t.Errorf("attendu result=played, got %q", resp.Result)
	}
	if len(fake.testCalls) != 1 || fake.testCalls[0] != audio.CueDepart {
		t.Errorf("TestSoundCue doit avoir été appelé exactement une fois avec CueDepart, got %v", fake.testCalls)
	}
}

// TestPOSTAPISoundsCueTest_Disabled_NeverCallsEngine is contract §Sound's
// own emphasis : "disabled est vérifié AVANT tout accès au moteur." A fake
// provider whose TestSoundCue would panic if called proves the check
// happens first, unambiguously.
func TestPOSTAPISoundsCueTest_Disabled_NeverCallsEngine(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	fake := &tw230eFakeSoundProvider{enabled: false, outputAvailable: true, testAccepted: true}
	server.Sound = fake

	w := tw230Post(t, server, "/api/sounds/depart/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Result != "disabled" {
		t.Errorf("attendu result=disabled, got %q", resp.Result)
	}
	if len(fake.testCalls) != 0 {
		t.Errorf("TestSoundCue ne doit JAMAIS être appelé quand sound.enabled=false (contract §Sound: \"disabled est vérifié avant tout accès au moteur\"), got %d appel(s)", len(fake.testCalls))
	}
}

func TestPOSTAPISoundsCueTest_Unavailable_NeutralOutput(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	// enabled=true mais sortie neutre (pilote dégradé) — distinct de disabled.
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: false, testAccepted: true}
	server.Sound = fake

	w := tw230Post(t, server, "/api/sounds/depart/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Result != "unavailable" {
		t.Errorf("attendu result=unavailable (sortie neutre, distincte de disabled), got %q", resp.Result)
	}
}

// TestPOSTAPISoundsCueTest_UnknownCue_Returns404_NeverCallsProvider proves
// the security guard (T6) applies here too, and applies BEFORE any
// SoundProvider access — a nil h.Sound must not panic on an invalid cue.
func TestPOSTAPISoundsCueTest_UnknownCue_Returns404_NeverCallsProvider(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	// h.Sound laissé nil délibérément : si le contrôle de cue n'était pas
	// fait EN PREMIER, ce test paniquerait sur un nil dereference plutôt que
	// de renvoyer 404 proprement.
	w := tw230Post(t, server, "/api/sounds/not-a-real-cue/test", nil, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCueTest_RejectsNonPOST(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	server.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}

	req := httptest.NewRequest("GET", "/api/sounds/depart/test", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestPOSTAPISoundsCueTest_DisabledCue_StillTestable is contract §6.3's own
// emphasis : a cue individually disabled via CuesDisabled must remain
// testable — TestSoundCue calls the engine DIRECTLY, never through
// notifySound, so CuesDisabled (a cmd/server-level concern this fake
// provider does not even model) never enters into it. This test proves the
// HTTP layer itself imposes no such gate — the provider is invoked
// unconditionally once enabled+available.
func TestPOSTAPISoundsCueTest_DisabledCue_StillTestable(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	// Le provider ACCEPTE la cue (simule CuesDisabled["perdu"]=true côté
	// jeu réel, mais TestSoundCue l'ignore délibérément — contract §6.3).
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}
	server.Sound = fake

	w := tw230Post(t, server, "/api/sounds/perdu/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Result string `json:"result"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Result != "played" {
		t.Errorf("une cue individuellement désactivée doit rester testable — attendu played, got %q", resp.Result)
	}
	if len(fake.testCalls) != 1 {
		t.Errorf("TestSoundCue doit être appelé même pour une cue désactivée en jeu, got %d appel(s)", len(fake.testCalls))
	}
}

// ---------------------------------------------------------------------------
// GET /api/sound/status — deux états seulement, fusionnés délibérément.
// ---------------------------------------------------------------------------

func TestGETAPISoundStatus_ActiveOnlyWhenEnabledAndOutputAvailable(t *testing.T) {
	cases := []struct {
		name            string
		enabled         bool
		outputAvailable bool
		wantActive      bool
	}{
		{"enabled+disponible => actif", true, true, true},
		{"désactivé (mais sortie disponible) => inactif", false, true, false},
		{"activé mais sortie indisponible => inactif", true, false, false},
		{"désactivé ET indisponible => inactif", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, dataDir := setupTestHTTPServer(t)
			tw230SeedDefaultSounds(t, dataDir)
			server.Sound = &tw230eFakeSoundProvider{enabled: tc.enabled, outputAvailable: tc.outputAvailable}

			req := httptest.NewRequest("GET", "/api/sound/status", nil)
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var resp struct {
				Active bool `json:"active"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("réponse non JSON valide : %v", err)
			}
			if resp.Active != tc.wantActive {
				t.Errorf("active=%v, attendu %v", resp.Active, tc.wantActive)
			}
		})
	}
}

func TestGETAPISoundStatus_RejectsNonGET(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	server.Sound = &tw230eFakeSoundProvider{enabled: true, outputAvailable: true}

	req := httptest.NewRequest("POST", "/api/sound/status", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}
