package config

// Tests dérivés de contracts/ai-generation.md (#8, v6.0.0) et du plan
// _work/reports/planner-20260805-121900.md, phases 1 et 2.
//
// Scope de ce fichier — concerns du paquet `config` uniquement :
//   - écriture atomique (Save) et protection concurrente du singleton (Get/SetInstance)
//   - application des défauts de la section `ai` (contrat §1)
//   - hygiène de `AIConfig.APIKeyConfigured` : champ dérivé, jamais lu depuis le disque
//     (contrat §1 : "Champ dérivé, jamais persisté : positionné uniquement dans la
//     réponse GET")
//
// Le merge HTTP proprement dit (POST partiel `{neon_effect}` → autres sections
// intactes, `clear_api_key`, validation `sk-ant-`) est testé au niveau HTTP dans
// `internal/server/ai_generation_test.go` (paquet `server`, où vit `handleConfig`) —
// pas ici, pour ne pas dupliquer un comportement qui n'est pas une fonction pure du
// paquet `config`. La non-régression sur `TestHTTPServer_Config_POST` (déjà présent
// dans `internal/server/http_test.go`) est étendue par `dev-backend`.
//
// ⚠️ `config.SetInstance` mute un global : PAS de `t.Parallel()` dans ce fichier.
// Les tests de course (`TestGetSetInstance_ConcurrentAccess`) exercent volontairement
// des goroutines internes, mais le test lui-même reste séquentiel vis-à-vis des autres.

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

// Save(*Config) error — signature imposée par le plan (T1.1) : pas de paramètre de
// chemin, elle écrit sur le même chemin relatif "config.json" que handleConfig
// utilise aujourd'hui en dur (http.go:1056). Chaque test qui l'exerce isole son
// répertoire de travail avec t.Chdir (Go 1.24) pour ne jamais toucher le vrai
// server-go/config.json du dépôt.
const testConfigFileName = "config.json"

// ----------------------------------------------------------------------------------
// T2.1 — section `ai` : structure et défauts (contrat §1)
// ----------------------------------------------------------------------------------

func TestLoad_AIDefaults_AppliedWhenSectionAbsent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-ai-defaults-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(`{"version": "6.0.0"}`)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AI.Model != "claude-opus-5" {
		t.Errorf("Expected default AI.Model=claude-opus-5, got %q", cfg.AI.Model)
	}
	if cfg.AI.TimeoutSeconds != 300 {
		t.Errorf("Expected default AI.TimeoutSeconds=300, got %d", cfg.AI.TimeoutSeconds)
	}
	if cfg.AI.MaxQuestions != 200 {
		t.Errorf("Expected default AI.MaxQuestions=200, got %d", cfg.AI.MaxQuestions)
	}
	if cfg.AI.AnthropicAPIKey != "" {
		t.Errorf("Expected default AI.AnthropicAPIKey empty, got %q", cfg.AI.AnthropicAPIKey)
	}
}

func TestLoad_AIDefaults_DoNotOverrideExplicitValues(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-ai-explicit-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(`{
		"version": "6.0.0",
		"ai": {
			"anthropic_api_key": "sk-ant-abc123",
			"model": "claude-custom",
			"timeout_seconds": 600,
			"max_questions": 50
		}
	}`)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AI.Model != "claude-custom" {
		t.Errorf("Expected explicit AI.Model preserved, got %q", cfg.AI.Model)
	}
	if cfg.AI.TimeoutSeconds != 600 {
		t.Errorf("Expected explicit AI.TimeoutSeconds preserved, got %d", cfg.AI.TimeoutSeconds)
	}
	if cfg.AI.MaxQuestions != 50 {
		t.Errorf("Expected explicit AI.MaxQuestions preserved, got %d", cfg.AI.MaxQuestions)
	}
	if cfg.AI.AnthropicAPIKey != "sk-ant-abc123" {
		t.Errorf("Expected explicit AI.AnthropicAPIKey preserved, got %q", cfg.AI.AnthropicAPIKey)
	}
}

// TestAIConfig_APIKeyConfigured_NotDerivedOnLoad verifies that Load() never computes
// api_key_configured from the presence of a key on disk — contract §1: it is a
// GET-response-only derived field, never read from or written to config.json in this
// form. A config.json that (incorrectly, e.g. hand-edited) contains
// "api_key_configured": true must not let that value leak into the singleton's
// authoritative state — only handleConfig's GET copy is allowed to set it.
func TestAIConfig_APIKeyConfigured_NotDerivedOnLoad(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-ai-derived-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// No key stored, but a stray "api_key_configured": true in the file (should never
	// happen via normal Save(), but Load() must not trust it either way).
	tmpFile.WriteString(`{
		"version": "6.0.0",
		"ai": { "anthropic_api_key": "", "api_key_configured": true }
	}`)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AI.AnthropicAPIKey != "" {
		t.Fatalf("Precondition failed: expected empty key, got %q", cfg.AI.AnthropicAPIKey)
	}
	// The derived flag must not be trusted from disk when it disagrees with the actual
	// key. Callers (handleConfig GET) are responsible for recomputing it from
	// AnthropicAPIKey != "" — Load() itself must not hand out a stale/forged value.
	if cfg.AI.APIKeyConfigured {
		t.Error("Load() must not trust a stray api_key_configured:true from disk when AnthropicAPIKey is empty — it is a derived, GET-only field (contract §1)")
	}
}

// ----------------------------------------------------------------------------------
// T1.1 — Save() : écriture atomique
// ----------------------------------------------------------------------------------

func TestSave_WritesReadableConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg := &Config{
		Version: "6.0.0",
		AI: AIConfig{
			AnthropicAPIKey: "sk-ant-savetest",
			Model:           "claude-opus-5",
			TimeoutSeconds:  300,
			MaxQuestions:    200,
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := Load(testConfigFileName)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if reloaded.AI.AnthropicAPIKey != "sk-ant-savetest" {
		t.Errorf("Expected AnthropicAPIKey to round-trip, got %q", reloaded.AI.AnthropicAPIKey)
	}
	if reloaded.Version != "6.0.0" {
		t.Errorf("Expected Version to round-trip, got %q", reloaded.Version)
	}
}

// TestSave_NoLeftoverTempFile verifies the "fichier temporaire + os.Rename" pattern
// required by contract §0 ("Persistance") does not leave a stray .tmp file behind,
// which would indicate the rename step was skipped or failed silently.
func TestSave_NoLeftoverTempFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := Save(&Config{Version: "6.0.0"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if e.Name() != testConfigFileName {
			t.Errorf("Unexpected leftover file after Save: %s (atomic write should leave only config.json)", e.Name())
		}
	}
}

// TestSave_OverwritesAtomically verifies a second Save() fully replaces the file
// content rather than merging with what was there before (Save is a dumb full
// writer — the merge semantics belong to the HTTP layer, contract §0).
func TestSave_OverwritesAtomically(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := Save(&Config{Version: "6.0.0", WiFi: WiFiConfig{SSID: "FirstSaveSSID"}}); err != nil {
		t.Fatalf("First Save failed: %v", err)
	}
	if err := Save(&Config{Version: "6.0.1"}); err != nil {
		t.Fatalf("Second Save failed: %v", err)
	}

	raw, err := os.ReadFile(testConfigFileName)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var onDisk map[string]interface{}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("config.json is not valid JSON after second Save: %v", err)
	}
	if onDisk["version"] != "6.0.1" {
		t.Errorf("Expected version=6.0.1 after second Save, got %v", onDisk["version"])
	}
	wifi, _ := onDisk["wifi"].(map[string]interface{})
	if wifi != nil && wifi["ssid"] == "FirstSaveSSID" {
		t.Error("Second Save() should not resurrect fields from the first call — Save() is a full overwrite, not a merge")
	}
}

// ----------------------------------------------------------------------------------
// T1.1 — Get()/SetInstance() : sûreté concurrente (sync.RWMutex)
// ----------------------------------------------------------------------------------

// TestGetSetInstance_ConcurrentAccess_Race exercises Get()/SetInstance() from many
// goroutines simultaneously. It only *fails* under `go test -race`, which is how
// `qa` must run it (cf. plan, phase 1 risk R1/R2 precedent). Without the
// `sync.RWMutex` required by contract §0 ("Concurrence"), this reliably trips the
// race detector because `instance` is a bare package-level pointer.
func TestGetSetInstance_ConcurrentAccess_Race(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 200

	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			SetInstance(&Config{Version: "race-test"})
		}(i)
		go func() {
			defer wg.Done()
			_ = Get()
		}()
	}

	wg.Wait()

	// Restore a sane instance so any test running after this one in the same binary
	// (config.SetInstance mutates a package-level global) does not inherit "race-test".
	SetInstance(&Config{Version: "2.0.0-test"})
}

// ----------------------------------------------------------------------------------
// Sécurité — S1 : la clé ne doit jamais apparaître dans une sérialisation destinée
// à être exposée telle quelle (garde-fou de non-régression sur le tag JSON).
// ----------------------------------------------------------------------------------

func TestAIConfig_JSONTags_MatchContract(t *testing.T) {
	cfg := AIConfig{
		AnthropicAPIKey: "sk-ant-tagcheck",
		Model:           "claude-opus-5",
		TimeoutSeconds:  300,
		MaxQuestions:    200,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	body := string(data)

	for _, key := range []string{`"anthropic_api_key"`, `"model"`, `"timeout_seconds"`, `"max_questions"`} {
		if !strings.Contains(body, key) {
			t.Errorf("Expected JSON tag %s in AIConfig marshal (contract §1), got: %s", key, body)
		}
	}

	// api_key_configured is `omitempty`: it must NOT appear when false (zero value),
	// since AIConfig{} above never set it.
	if strings.Contains(body, `"api_key_configured"`) {
		t.Errorf("api_key_configured must be omitted when false (omitempty, contract §1), got: %s", body)
	}
}

func TestAIConfig_APIKeyConfigured_OmittedWhenFalse_PresentWhenTrue(t *testing.T) {
	absent, _ := json.Marshal(AIConfig{})
	if strings.Contains(string(absent), "api_key_configured") {
		t.Errorf("api_key_configured must be omitted (omitempty) when false, got: %s", absent)
	}

	present, _ := json.Marshal(AIConfig{APIKeyConfigured: true})
	if !strings.Contains(string(present), `"api_key_configured":true`) {
		t.Errorf("api_key_configured must be present and true when explicitly set, got: %s", present)
	}
}

// TestConfig_AISectionKey verifies the top-level `ai` key on Config (contract §1:
// `AI AIConfig \`json:"ai"\``), guarding against an accidental rename that would
// silently break every consumer of GET /config.json.
func TestConfig_AISectionKey(t *testing.T) {
	cfg := Config{AI: AIConfig{Model: "claude-opus-5"}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var onWire map[string]interface{}
	if err := json.Unmarshal(data, &onWire); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	ai, ok := onWire["ai"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected top-level \"ai\" key in Config JSON, got: %s", data)
	}
	if ai["model"] != "claude-opus-5" {
		t.Errorf("Expected ai.model=claude-opus-5, got %v", ai["model"])
	}
}
