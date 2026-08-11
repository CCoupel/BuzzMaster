package config

// Tests for MigrateGameSettings (#150, option b, plan task 17.3/17.8):
// splitting "game"/"neon_effect" out of config.json into game-config.json
// at startup. Each test isolates both ConfigPath() and GameConfigPath()
// via SetConfigPath/SetGameConfigPath into its own t.TempDir(), restoring
// the previous values on cleanup — configPath/gameConfigPath are
// process-wide globals (same isolation discipline as #143's
// TestSetConfigPath_* in config_test.go).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// isolateMigrationPaths points ConfigPath()/GameConfigPath() at two files
// inside a fresh temp dir and restores the previous values on test cleanup.
// Returns the two resolved paths for convenience.
func isolateMigrationPaths(t *testing.T) (configJSONPath, gameConfigJSONPath string) {
	t.Helper()
	dir := t.TempDir()
	configJSONPath = filepath.Join(dir, "config.json")
	gameConfigJSONPath = filepath.Join(dir, "config", "game-config.json")

	prevConfig, prevGame := ConfigPath(), GameConfigPath()
	t.Cleanup(func() {
		SetConfigPath(prevConfig)
		SetGameConfigPath(prevGame)
	})
	SetConfigPath(configJSONPath)
	SetGameConfigPath(gameConfigJSONPath)
	return configJSONPath, gameConfigJSONPath
}

func TestMigrateGameSettings_NoConfigFile_NoOp(t *testing.T) {
	_, gameConfigJSONPath := isolateMigrationPaths(t)

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("MigrateGameSettings() with no config.json should be a no-op, got error: %v", err)
	}
	if _, err := os.Stat(gameConfigJSONPath); err == nil {
		t.Errorf("game-config.json must not be created when config.json doesn't exist")
	}
}

func TestMigrateGameSettings_NoGameSections_NoOp(t *testing.T) {
	configJSONPath, gameConfigJSONPath := isolateMigrationPaths(t)

	original := `{"version":"6.0.0","server":{"http_port":80}}`
	if err := os.WriteFile(configJSONPath, []byte(original), 0644); err != nil {
		t.Fatalf("could not write fixture config.json: %v", err)
	}

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("MigrateGameSettings() failed: %v", err)
	}

	after, err := os.ReadFile(configJSONPath)
	if err != nil {
		t.Fatalf("could not read config.json after migration: %v", err)
	}
	if string(after) != original {
		t.Errorf("config.json without game/neon_effect sections must be left byte-untouched.\nbefore: %s\nafter:  %s", original, after)
	}
	if _, err := os.Stat(gameConfigJSONPath); err == nil {
		t.Errorf("game-config.json must not be created when config.json never had game/neon_effect sections")
	}
}

func TestMigrateGameSettings_FirstMigration(t *testing.T) {
	configJSONPath, gameConfigJSONPath := isolateMigrationPaths(t)

	original := `{
		"version": "6.0.0",
		"server": {"http_port": 8080},
		"wifi": {"ssid": "MySSID", "password": "MyPassword"},
		"game": {"default_delay": 45},
		"neon_effect": {"enabled": true, "arc_width": 90}
	}`
	if err := os.WriteFile(configJSONPath, []byte(original), 0644); err != nil {
		t.Fatalf("could not write fixture config.json: %v", err)
	}

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("MigrateGameSettings() failed: %v", err)
	}

	// game-config.json must exist and carry the migrated values (with
	// defaults applied to whatever the source didn't specify).
	gs, err := LoadGameSettings(gameConfigJSONPath)
	if err != nil {
		t.Fatalf("could not load migrated game-config.json: %v", err)
	}
	if gs.Game.DefaultDelay != 45 {
		t.Errorf("Expected migrated default_delay=45, got %d", gs.Game.DefaultDelay)
	}
	if !gs.NeonEffect.Enabled {
		t.Errorf("Expected migrated neon_effect.enabled=true, got false")
	}
	if gs.NeonEffect.ArcWidth != 90 {
		t.Errorf("Expected migrated neon_effect.arc_width=90, got %d", gs.NeonEffect.ArcWidth)
	}
	// A field NOT present in the source's neon_effect object must still get
	// its default (contract ai-generation.md §0 semantics, carried over).
	if gs.NeonEffect.BarOffset != 20 {
		t.Errorf("Expected default bar_offset=20 applied to the migrated section, got %d", gs.NeonEffect.BarOffset)
	}

	// config.json must have the two sections stripped, but everything else preserved.
	var afterRaw map[string]json.RawMessage
	afterData, err := os.ReadFile(configJSONPath)
	if err != nil {
		t.Fatalf("could not read config.json after migration: %v", err)
	}
	if err := json.Unmarshal(afterData, &afterRaw); err != nil {
		t.Fatalf("config.json is not valid JSON after migration: %v", err)
	}
	if _, ok := afterRaw["game"]; ok {
		t.Errorf("config.json must no longer have a \"game\" section after migration")
	}
	if _, ok := afterRaw["neon_effect"]; ok {
		t.Errorf("config.json must no longer have a \"neon_effect\" section after migration")
	}
	var afterWiFi WiFiConfig
	if err := json.Unmarshal(afterRaw["wifi"], &afterWiFi); err != nil {
		t.Fatalf("could not decode wifi section after migration: %v", err)
	}
	if afterWiFi.SSID != "MySSID" || afterWiFi.Password != "MyPassword" {
		t.Errorf("Expected wifi section preserved verbatim, got %+v", afterWiFi)
	}
}

// TestMigrateGameSettings_Idempotent is the regression test for R1b (plan
// risk table): a second startup must not rewrite either file.
func TestMigrateGameSettings_Idempotent(t *testing.T) {
	configJSONPath, gameConfigJSONPath := isolateMigrationPaths(t)

	original := `{"version":"6.0.0","game":{"default_delay":45},"neon_effect":{"enabled":true}}`
	if err := os.WriteFile(configJSONPath, []byte(original), 0644); err != nil {
		t.Fatalf("could not write fixture config.json: %v", err)
	}

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("first MigrateGameSettings() failed: %v", err)
	}

	configAfterFirst, err := os.ReadFile(configJSONPath)
	if err != nil {
		t.Fatalf("could not read config.json after first migration: %v", err)
	}
	gameConfigAfterFirst, err := os.ReadFile(gameConfigJSONPath)
	if err != nil {
		t.Fatalf("could not read game-config.json after first migration: %v", err)
	}

	// Simulate a second server startup against the now-migrated files.
	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("second MigrateGameSettings() failed: %v", err)
	}

	configAfterSecond, err := os.ReadFile(configJSONPath)
	if err != nil {
		t.Fatalf("could not read config.json after second migration: %v", err)
	}
	gameConfigAfterSecond, err := os.ReadFile(gameConfigJSONPath)
	if err != nil {
		t.Fatalf("could not read game-config.json after second migration: %v", err)
	}

	if string(configAfterFirst) != string(configAfterSecond) {
		t.Errorf("config.json changed on the second (idempotent) migration run.\nfirst:  %s\nsecond: %s", configAfterFirst, configAfterSecond)
	}
	if string(gameConfigAfterFirst) != string(gameConfigAfterSecond) {
		t.Errorf("game-config.json changed on the second (idempotent) migration run.\nfirst:  %s\nsecond: %s", gameConfigAfterFirst, gameConfigAfterSecond)
	}
}

// TestMigrateGameSettings_BothPresent_GameConfigWins covers the "partially
// migrated or hand-edited config.json" case: game-config.json already
// exists (e.g. from a prior migration or a restore) and is authoritative —
// stray game/neon_effect values still in config.json are discarded, not
// merged or allowed to overwrite it.
func TestMigrateGameSettings_BothPresent_GameConfigWins(t *testing.T) {
	configJSONPath, gameConfigJSONPath := isolateMigrationPaths(t)

	if err := os.MkdirAll(filepath.Dir(gameConfigJSONPath), 0755); err != nil {
		t.Fatalf("could not create game-config.json's directory: %v", err)
	}
	existingGameConfig := `{"game":{"default_delay":99},"neon_effect":{"enabled":false,"arc_width":45}}`
	if err := os.WriteFile(gameConfigJSONPath, []byte(existingGameConfig), 0644); err != nil {
		t.Fatalf("could not write pre-existing game-config.json: %v", err)
	}

	strayConfig := `{"version":"6.0.0","game":{"default_delay":45},"neon_effect":{"enabled":true,"arc_width":90}}`
	if err := os.WriteFile(configJSONPath, []byte(strayConfig), 0644); err != nil {
		t.Fatalf("could not write fixture config.json: %v", err)
	}

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("MigrateGameSettings() failed: %v", err)
	}

	gs, err := LoadGameSettings(gameConfigJSONPath)
	if err != nil {
		t.Fatalf("could not load game-config.json after migration: %v", err)
	}
	if gs.Game.DefaultDelay != 99 {
		t.Errorf("game-config.json's pre-existing default_delay=99 must win over config.json's stray 45, got %d", gs.Game.DefaultDelay)
	}
	if gs.NeonEffect.Enabled {
		t.Errorf("game-config.json's pre-existing neon_effect.enabled=false must win over config.json's stray true")
	}
	if gs.NeonEffect.ArcWidth != 45 {
		t.Errorf("game-config.json's pre-existing arc_width=45 must win over config.json's stray 90, got %d", gs.NeonEffect.ArcWidth)
	}

	// config.json must still be stripped of the stray sections.
	var afterRaw map[string]json.RawMessage
	afterData, err := os.ReadFile(configJSONPath)
	if err != nil {
		t.Fatalf("could not read config.json after migration: %v", err)
	}
	if err := json.Unmarshal(afterData, &afterRaw); err != nil {
		t.Fatalf("config.json is not valid JSON after migration: %v", err)
	}
	if _, ok := afterRaw["game"]; ok {
		t.Errorf("config.json must no longer have a stray \"game\" section after migration")
	}
	if _, ok := afterRaw["neon_effect"]; ok {
		t.Errorf("config.json must no longer have a stray \"neon_effect\" section after migration")
	}
}

// TestMigrateGameSettings_AbsentBoth_Defaults covers the "neither file
// exists" case (fresh install): GetGameSettings() must fall back to
// defaults rather than error.
func TestMigrateGameSettings_AbsentBoth_Defaults(t *testing.T) {
	isolateMigrationPaths(t)

	if err := MigrateGameSettings(); err != nil {
		t.Fatalf("MigrateGameSettings() on a fresh install should be a no-op, got error: %v", err)
	}

	gs, err := LoadGameSettings(GameConfigPath())
	if err == nil {
		t.Fatalf("expected LoadGameSettings to fail (no file yet), got %+v", gs)
	}
	// The actual fresh-install path (GetGameSettings' fallback) is covered
	// by TestSetConfigPath_Save_WritesToConfiguredPath's sibling tests in
	// config_test.go; here we only confirm migration itself didn't
	// fabricate a file out of nothing.
	if _, err := os.Stat(GameConfigPath()); err == nil {
		t.Errorf("game-config.json must not be created when config.json doesn't exist either")
	}
}
