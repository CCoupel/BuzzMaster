package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNeonEffectDefaults moved to GameSettings by #150 (option b): neon
// effect settings now live in game-config.json (LoadGameSettings), not
// config.json (Load) — see gameconfig.go's doc comment.
func TestNeonEffectDefaults(t *testing.T) {
	// Create temp config file with minimal data
	tmpFile, err := os.CreateTemp("", "game-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write minimal game-config.json (no neon_effect section at all).
	minimalConfig := `{
		"game": {"default_delay": 30}
	}`
	tmpFile.WriteString(minimalConfig)
	tmpFile.Close()

	// Load game settings
	gs, err := LoadGameSettings(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load game settings: %v", err)
	}

	// Check neon effect defaults
	if gs.NeonEffect.ArcWidth != 60 {
		t.Errorf("Expected ArcWidth=60, got %d", gs.NeonEffect.ArcWidth)
	}
	if gs.NeonEffect.IntensityGap != 80 {
		t.Errorf("Expected IntensityGap=80, got %d", gs.NeonEffect.IntensityGap)
	}
	if gs.NeonEffect.RotationSpeed != 4.0 {
		t.Errorf("Expected RotationSpeed=4.0, got %.1f", gs.NeonEffect.RotationSpeed)
	}
	if gs.NeonEffect.Enabled != false {
		t.Errorf("Expected Enabled=false, got %v", gs.NeonEffect.Enabled)
	}
}

func TestNeonEffectValidation(t *testing.T) {
	tests := []struct {
		name           string
		input          NeonEffectConfig
		expectedArc    int
		expectedGap    int
		expectedSpeed  float64
	}{
		{
			name:          "Valid values",
			input:         NeonEffectConfig{ArcWidth: 90, IntensityGap: 50, RotationSpeed: 5.0},
			expectedArc:   90,
			expectedGap:   50,
			expectedSpeed: 5.0,
		},
		{
			name:          "ArcWidth too low",
			input:         NeonEffectConfig{ArcWidth: 10, IntensityGap: 50, RotationSpeed: 5.0},
			expectedArc:   30, // Clamped to minimum
			expectedGap:   50,
			expectedSpeed: 5.0,
		},
		{
			name:          "ArcWidth too high",
			input:         NeonEffectConfig{ArcWidth: 200, IntensityGap: 50, RotationSpeed: 5.0},
			expectedArc:   180, // Clamped to maximum
			expectedGap:   50,
			expectedSpeed: 5.0,
		},
		{
			name:          "IntensityGap negative",
			input:         NeonEffectConfig{ArcWidth: 60, IntensityGap: -10, RotationSpeed: 5.0},
			expectedArc:   60,
			expectedGap:   0, // Clamped to minimum
			expectedSpeed: 5.0,
		},
		{
			name:          "IntensityGap too high",
			input:         NeonEffectConfig{ArcWidth: 60, IntensityGap: 150, RotationSpeed: 5.0},
			expectedArc:   60,
			expectedGap:   100, // Clamped to maximum
			expectedSpeed: 5.0,
		},
		{
			name:          "RotationSpeed too low",
			input:         NeonEffectConfig{ArcWidth: 60, IntensityGap: 50, RotationSpeed: 0.5},
			expectedArc:   60,
			expectedGap:   50,
			expectedSpeed: 1.0, // Clamped to minimum
		},
		{
			name:          "RotationSpeed too high",
			input:         NeonEffectConfig{ArcWidth: 60, IntensityGap: 50, RotationSpeed: 15.0},
			expectedArc:   60,
			expectedGap:   50,
			expectedSpeed: 10.0, // Clamped to maximum
		},
		{
			name:          "All values at boundaries (min)",
			input:         NeonEffectConfig{ArcWidth: 30, IntensityGap: 0, RotationSpeed: 1.0},
			expectedArc:   30,
			expectedGap:   0,
			expectedSpeed: 1.0,
		},
		{
			name:          "All values at boundaries (max)",
			input:         NeonEffectConfig{ArcWidth: 180, IntensityGap: 100, RotationSpeed: 10.0},
			expectedArc:   180,
			expectedGap:   100,
			expectedSpeed: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &GameSettings{NeonEffect: tt.input}
			gs.ValidateAndClampNeonEffect()

			if gs.NeonEffect.ArcWidth != tt.expectedArc {
				t.Errorf("ArcWidth: got %d, want %d", gs.NeonEffect.ArcWidth, tt.expectedArc)
			}
			if gs.NeonEffect.IntensityGap != tt.expectedGap {
				t.Errorf("IntensityGap: got %d, want %d", gs.NeonEffect.IntensityGap, tt.expectedGap)
			}
			if gs.NeonEffect.RotationSpeed != tt.expectedSpeed {
				t.Errorf("RotationSpeed: got %.1f, want %.1f", gs.NeonEffect.RotationSpeed, tt.expectedSpeed)
			}
		})
	}
}

// TestApplyDefaults_AIConfig verifies the AI section defaults introduced for
// #8 (contract ai-generation.md §1) — model, timeout and cap — and that a
// stored API key is never touched by ApplyDefaults.
func TestApplyDefaults_AIConfig(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	if cfg.AI.Model != "claude-opus-5" {
		t.Errorf("Expected default model=claude-opus-5, got %q", cfg.AI.Model)
	}
	if cfg.AI.TimeoutSeconds != 300 {
		t.Errorf("Expected default timeout_seconds=300, got %d", cfg.AI.TimeoutSeconds)
	}
	if cfg.AI.MaxQuestions != 200 {
		t.Errorf("Expected default max_questions=200, got %d", cfg.AI.MaxQuestions)
	}
	if cfg.AI.AnthropicAPIKey != "" {
		t.Errorf("Expected no default API key, got %q", cfg.AI.AnthropicAPIKey)
	}

	// ApplyDefaults must not override an existing non-zero value.
	custom := &Config{AI: AIConfig{Model: "claude-sonnet-4", TimeoutSeconds: 60, MaxQuestions: 10, AnthropicAPIKey: "sk-ant-x"}}
	ApplyDefaults(custom)
	if custom.AI.Model != "claude-sonnet-4" || custom.AI.TimeoutSeconds != 60 || custom.AI.MaxQuestions != 10 {
		t.Errorf("ApplyDefaults overrode existing AI values: %+v", custom.AI)
	}
	if custom.AI.AnthropicAPIKey != "sk-ant-x" {
		t.Errorf("ApplyDefaults touched the API key: %q", custom.AI.AnthropicAPIKey)
	}
}

// TestApplyDefaults_StorageDirs is the regression test for the destructive
// bug fixed by #8: questions_dir/files_dir must never be left empty, which is
// exactly how they ended up empty in the shipped server-go/config.json
// (compensated only by a hardcoded fallback in main.go).
func TestApplyDefaults_StorageDirs(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	if cfg.Storage.QuestionsDir == "" {
		t.Error("Expected a non-empty default questions_dir")
	}
	if cfg.Storage.FilesDir == "" {
		t.Error("Expected a non-empty default files_dir")
	}
}

// TestSave_AtomicWriteRoundtrip verifies Save() persists a config that Load()
// can read back identically, and that no stray temp file is left behind
// (contract ai-generation.md §0 — atomic write via temp file + rename).
func TestSave_AtomicWriteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	cfg := &Config{Version: "6.0.0-test"}
	ApplyDefaults(cfg)
	cfg.AI.AnthropicAPIKey = "sk-ant-roundtrip"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if reloaded.Version != "6.0.0-test" {
		t.Errorf("Expected version round-tripped, got %q", reloaded.Version)
	}
	if reloaded.AI.AnthropicAPIKey != "sk-ant-roundtrip" {
		t.Errorf("Expected API key round-tripped, got %q", reloaded.AI.AnthropicAPIKey)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "config.json" {
			t.Errorf("Unexpected leftover file after Save: %s", e.Name())
		}
	}
}

// TestGetSetInstance_ConcurrentAccess exercises the RWMutex added around the
// singleton (contract ai-generation.md §0 — Get/SetInstance were previously
// unlocked) with the race detector (`go test -race`).
func TestGetSetInstance_ConcurrentAccess(t *testing.T) {
	SetInstance(&Config{Version: "race-test"})

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			if n%2 == 0 {
				SetInstance(&Config{Version: "race-test"})
			} else {
				_ = Get()
			}
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestGameSettingsJSONRoundtrip moved from Config to GameSettings by #150 —
// neon_effect is no longer part of Config's own JSON shape at all.
func TestGameSettingsJSONRoundtrip(t *testing.T) {
	// Test that game settings can be marshaled and unmarshaled correctly
	original := &GameSettings{
		NeonEffect: NeonEffectConfig{
			Enabled:       true,
			ArcWidth:      90,
			IntensityGap:  60,
			RotationSpeed: 6.5,
		},
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded GameSettings
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.NeonEffect.Enabled != original.NeonEffect.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", decoded.NeonEffect.Enabled, original.NeonEffect.Enabled)
	}
	if decoded.NeonEffect.ArcWidth != original.NeonEffect.ArcWidth {
		t.Errorf("ArcWidth mismatch: got %d, want %d", decoded.NeonEffect.ArcWidth, original.NeonEffect.ArcWidth)
	}
	if decoded.NeonEffect.IntensityGap != original.NeonEffect.IntensityGap {
		t.Errorf("IntensityGap mismatch: got %d, want %d", decoded.NeonEffect.IntensityGap, original.NeonEffect.IntensityGap)
	}
	if decoded.NeonEffect.RotationSpeed != original.NeonEffect.RotationSpeed {
		t.Errorf("RotationSpeed mismatch: got %.1f, want %.1f", decoded.NeonEffect.RotationSpeed, original.NeonEffect.RotationSpeed)
	}
}

// ---------------------------------------------------------------------------
// EffectiveAnthropicAPIKey / EffectiveGroqAPIKey (security incident
// 2026-08-07 — a Groq key committed in cleartext to a tracked config.json
// blocked the PROD deployment; see docs/ADMIN_GUIDE.md "Configurer les clés
// API IA en production"). Env var takes priority over config.json's stored
// value; config.json remains the fallback for local/dev admin-UI usage.
// ---------------------------------------------------------------------------

func TestEffectiveAnthropicAPIKey_EnvVarTakesPriority(t *testing.T) {
	t.Setenv(EnvAnthropicAPIKey, "sk-ant-from-env")
	cfg := AIConfig{AnthropicAPIKey: "sk-ant-from-file"}
	if got := cfg.EffectiveAnthropicAPIKey(); got != "sk-ant-from-env" {
		t.Errorf("Expected the env var to take priority, got %q", got)
	}
}

func TestEffectiveAnthropicAPIKey_FallsBackToFileWhenEnvUnset(t *testing.T) {
	t.Setenv(EnvAnthropicAPIKey, "") // ensure unset even if the test runner's environment carries one
	cfg := AIConfig{AnthropicAPIKey: "sk-ant-from-file"}
	if got := cfg.EffectiveAnthropicAPIKey(); got != "sk-ant-from-file" {
		t.Errorf("Expected fallback to the config.json value, got %q", got)
	}
}

func TestEffectiveGroqAPIKey_EnvVarTakesPriority(t *testing.T) {
	t.Setenv(EnvGroqAPIKey, "gsk_from_env")
	cfg := AIConfig{GroqAPIKey: "gsk_from_file"}
	if got := cfg.EffectiveGroqAPIKey(); got != "gsk_from_env" {
		t.Errorf("Expected the env var to take priority, got %q", got)
	}
}

func TestEffectiveGroqAPIKey_FallsBackToFileWhenEnvUnset(t *testing.T) {
	t.Setenv(EnvGroqAPIKey, "")
	cfg := AIConfig{GroqAPIKey: "gsk_from_file"}
	if got := cfg.EffectiveGroqAPIKey(); got != "gsk_from_file" {
		t.Errorf("Expected fallback to the config.json value, got %q", got)
	}
}

func TestEffectiveAPIKeyConfigured_TrueFromEnvAloneWithEmptyFile(t *testing.T) {
	// The exact PROD scenario this fix targets: config.json's field is empty
	// (never written to disk), the key comes only from the environment.
	t.Setenv(EnvAnthropicAPIKey, "sk-ant-from-env")
	t.Setenv(EnvGroqAPIKey, "gsk_from_env")
	cfg := AIConfig{} // both file-backed fields empty

	if !cfg.EffectiveAnthropicAPIKeyConfigured() {
		t.Error("Expected EffectiveAnthropicAPIKeyConfigured to be true when only the env var is set")
	}
	if !cfg.EffectiveGroqAPIKeyConfigured() {
		t.Error("Expected EffectiveGroqAPIKeyConfigured to be true when only the env var is set")
	}
}

func TestEffectiveAPIKeyConfigured_FalseWhenNeitherSourceHasAKey(t *testing.T) {
	t.Setenv(EnvAnthropicAPIKey, "")
	t.Setenv(EnvGroqAPIKey, "")
	cfg := AIConfig{}

	if cfg.EffectiveAnthropicAPIKeyConfigured() {
		t.Error("Expected EffectiveAnthropicAPIKeyConfigured to be false with no key anywhere")
	}
	if cfg.EffectiveGroqAPIKeyConfigured() {
		t.Error("Expected EffectiveGroqAPIKeyConfigured to be false with no key anywhere")
	}
}

// TestEffectiveAPIKey_NeverMutatesTheStoredField is the regression guard for
// the actual security property this fix depends on: resolving the effective
// key must NEVER write the env var value back into the AIConfig struct — a
// later config.Save() call (triggered by ANY settings change, not just AI
// ones) persists the struct verbatim, so mutating the field here would leak
// the env-supplied key to config.json on disk the next time an unrelated
// setting was saved.
func TestEffectiveAPIKey_NeverMutatesTheStoredField(t *testing.T) {
	t.Setenv(EnvAnthropicAPIKey, "sk-ant-from-env")
	t.Setenv(EnvGroqAPIKey, "gsk_from_env")
	cfg := AIConfig{AnthropicAPIKey: "", GroqAPIKey: ""}

	_ = cfg.EffectiveAnthropicAPIKey()
	_ = cfg.EffectiveGroqAPIKey()

	if cfg.AnthropicAPIKey != "" {
		t.Errorf("EffectiveAnthropicAPIKey must not mutate AIConfig.AnthropicAPIKey, got %q", cfg.AnthropicAPIKey)
	}
	if cfg.GroqAPIKey != "" {
		t.Errorf("EffectiveGroqAPIKey must not mutate AIConfig.GroqAPIKey, got %q", cfg.GroqAPIKey)
	}
}

// ----------------------------------------------------------------------------------
// SetConfigPath / ConfigPath — path indirection (bugfix #143)
// ----------------------------------------------------------------------------------

// TestSetConfigPath_RoundTrips verifies the setter/getter pair itself,
// independent of Get()/Save(). Restores the previous value afterwards so
// this test cannot influence any other test's Get()/Save() calls, since
// configPath is a process-wide global.
func TestSetConfigPath_RoundTrips(t *testing.T) {
	orig := ConfigPath()
	t.Cleanup(func() { SetConfigPath(orig) })

	SetConfigPath("/tmp/somewhere/custom-config.json")
	if got := ConfigPath(); got != "/tmp/somewhere/custom-config.json" {
		t.Errorf("ConfigPath() = %q, want the value just set", got)
	}
}

// TestSetConfigPath_Save_WritesToConfiguredPath is the regression test for
// #143: Save() must resolve the path SetConfigPath configured, not the
// literal "config.json" that used to be hardcoded — and must NOT fall back
// to writing the default relative path once overridden. This is the
// mechanism internal/server's tests rely on to avoid writing fake API keys
// into the tracked internal/server/config.json fixture.
func TestSetConfigPath_Save_WritesToConfiguredPath(t *testing.T) {
	t.Chdir(t.TempDir()) // isolate the "did it fall back to the default?" check below

	orig := ConfigPath()
	t.Cleanup(func() { SetConfigPath(orig) })

	dir := t.TempDir()
	custom := filepath.Join(dir, "custom-name.json")
	SetConfigPath(custom)

	if err := Save(&Config{Version: "6.1.3-test"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("expected Save() to write to the configured path %s: %v", custom, err)
	}
	var onDisk map[string]interface{}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if onDisk["version"] != "6.1.3-test" {
		t.Errorf("unexpected content at configured path: %v", onDisk)
	}

	if _, err := os.Stat("config.json"); err == nil {
		t.Errorf("Save() also wrote to the default relative config.json even though SetConfigPath was overridden — the override was ignored")
	}
}
