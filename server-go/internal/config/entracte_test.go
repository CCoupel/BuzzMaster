package config

import (
	"path/filepath"
	"testing"
)

// isolateGameConfigPath points GameConfigPath() at a fresh temp file and
// restores the previous value on cleanup — same isolation discipline as
// gameconfig_migration_test.go's isolateMigrationPaths.
func isolateGameConfigPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "game-config.json")
	prev := GameConfigPath()
	t.Cleanup(func() { SetGameConfigPath(prev) })
	SetGameConfigPath(path)
	return path
}

// TestApplyGameSettingsDefaults_Entracte covers the plain-default path: a
// zero-value GameSettings gets every ENTRACTE field filled in, per contract
// game-state.md §"ENTRACTE_CONFIG".
func TestApplyGameSettingsDefaults_Entracte(t *testing.T) {
	var gs GameSettings
	ApplyGameSettingsDefaults(&gs)

	if gs.Entracte.Title != "ENTRACTE" {
		t.Errorf("Title default: got %q, want %q", gs.Entracte.Title, "ENTRACTE")
	}
	if gs.Entracte.Subtitle != "Retour dans 20mn" {
		t.Errorf("Subtitle default: got %q, want %q", gs.Entracte.Subtitle, "Retour dans 20mn")
	}
	if gs.Entracte.PanelSize != 65 {
		t.Errorf("PanelSize default: got %d, want 65", gs.Entracte.PanelSize)
	}
	if gs.Entracte.AnimPeriod != 10 {
		t.Errorf("AnimPeriod default: got %d, want 10", gs.Entracte.AnimPeriod)
	}
	if gs.Entracte.AnimIntensity == nil {
		t.Fatal("AnimIntensity: expected a non-nil default to be filled in")
	}
	if *gs.Entracte.AnimIntensity != 20 {
		t.Errorf("AnimIntensity default: got %d, want 20", *gs.Entracte.AnimIntensity)
	}
}

// TestApplyGameSettingsDefaults_AnimIntensityZero_Survives is THE critical
// regression test for the plan's documented pitfall (B6, risk table
// "ANIM_INTENSITY = 0 ré-écrasé par ApplyGameSettingsDefaults"): an
// explicitly-disabled animation (a non-nil pointer to 0) must NOT be
// silently reset to the default 20 on the next call — which is exactly what
// a naive `if gs.Entracte.AnimIntensity == 0` check would do.
func TestApplyGameSettingsDefaults_AnimIntensityZero_Survives(t *testing.T) {
	zero := 0
	gs := GameSettings{Entracte: EntracteConfig{AnimIntensity: &zero}}

	ApplyGameSettingsDefaults(&gs)

	if gs.Entracte.AnimIntensity == nil {
		t.Fatal("AnimIntensity became nil — must stay a non-nil pointer to 0")
	}
	if *gs.Entracte.AnimIntensity != 0 {
		t.Errorf("AnimIntensity: got %d, want 0 (explicitly disabled must survive defaulting)", *gs.Entracte.AnimIntensity)
	}
}

// TestApplyGameSettingsDefaults_Entracte_IdempotentAcrossMultipleCalls
// simulates the real production sequence (LoadGameSettings calls
// ApplyGameSettingsDefaults once; handleGameConfig calls it again on every
// POST) — the defaulting logic must be a true no-op on a value it already
// filled in, disabled animation included.
func TestApplyGameSettingsDefaults_Entracte_IdempotentAcrossMultipleCalls(t *testing.T) {
	zero := 0
	gs := GameSettings{Entracte: EntracteConfig{AnimIntensity: &zero, PanelSize: 80}}

	ApplyGameSettingsDefaults(&gs)
	ApplyGameSettingsDefaults(&gs)
	ApplyGameSettingsDefaults(&gs)

	if gs.Entracte.AnimIntensity == nil || *gs.Entracte.AnimIntensity != 0 {
		t.Errorf("AnimIntensity drifted after repeated ApplyGameSettingsDefaults calls: %v", gs.Entracte.AnimIntensity)
	}
	if gs.Entracte.PanelSize != 80 {
		t.Errorf("PanelSize drifted after repeated ApplyGameSettingsDefaults calls: got %d, want 80 (explicit value preserved)", gs.Entracte.PanelSize)
	}
}

// TestValidateAndClampEntracte_Clamps covers the numeric bounds from
// contract http-endpoints.md §"Mode ENTRACTE".
func TestValidateAndClampEntracte_Clamps(t *testing.T) {
	tooLow, tooHigh := -5, 500
	tests := []struct {
		name          string
		in            EntracteConfig
		wantPanelSize int
		wantAnimPer   int
		wantIntensity int
	}{
		{
			name:          "below range clamps up",
			in:            EntracteConfig{PanelSize: 5, AnimPeriod: 0, AnimIntensity: &tooLow},
			wantPanelSize: 20,
			wantAnimPer:   2,
			wantIntensity: 0,
		},
		{
			name:          "above range clamps down",
			in:            EntracteConfig{PanelSize: 500, AnimPeriod: 999, AnimIntensity: &tooHigh},
			wantPanelSize: 100,
			wantAnimPer:   30,
			wantIntensity: 100,
		},
		{
			name:          "in range untouched",
			in:            EntracteConfig{PanelSize: 65, AnimPeriod: 10, AnimIntensity: intPtr(20)},
			wantPanelSize: 65,
			wantAnimPer:   10,
			wantIntensity: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := GameSettings{Entracte: tt.in}
			gs.ValidateAndClampEntracte()

			if gs.Entracte.PanelSize != tt.wantPanelSize {
				t.Errorf("PanelSize: got %d, want %d", gs.Entracte.PanelSize, tt.wantPanelSize)
			}
			if gs.Entracte.AnimPeriod != tt.wantAnimPer {
				t.Errorf("AnimPeriod: got %d, want %d", gs.Entracte.AnimPeriod, tt.wantAnimPer)
			}
			if gs.Entracte.AnimIntensity == nil || *gs.Entracte.AnimIntensity != tt.wantIntensity {
				t.Errorf("AnimIntensity: got %v, want %d", gs.Entracte.AnimIntensity, tt.wantIntensity)
			}
		})
	}
}

// TestValidateAndClampEntracte_TruncatesLongText verifies Title/Subtitle are
// rune-truncated (not byte-truncated — multi-byte UTF-8 must not be cut
// mid-character).
func TestValidateAndClampEntracte_TruncatesLongText(t *testing.T) {
	longText := ""
	for i := 0; i < entracteTextMaxRunes+50; i++ {
		longText += "é" // 2-byte UTF-8 rune — would corrupt on a byte-slice truncation
	}
	gs := GameSettings{Entracte: EntracteConfig{Title: longText, Subtitle: longText, AnimIntensity: intPtr(20)}}
	gs.ValidateAndClampEntracte()

	if got := []rune(gs.Entracte.Title); len(got) != entracteTextMaxRunes {
		t.Errorf("Title: got %d runes, want %d", len(got), entracteTextMaxRunes)
	}
	if got := []rune(gs.Entracte.Subtitle); len(got) != entracteTextMaxRunes {
		t.Errorf("Subtitle: got %d runes, want %d", len(got), entracteTextMaxRunes)
	}
}

// TestSaveLoadGameSettings_Entracte_AnimIntensityZero_RoundTrips is the
// full persistence round-trip (B8 "Config: ... persistance"): save a
// GameSettings with the animation explicitly disabled, reload it from disk,
// and confirm it is STILL disabled — the scenario a naive zero-check would
// silently break the very first time someone reloads game-config.json.
func TestSaveLoadGameSettings_Entracte_AnimIntensityZero_RoundTrips(t *testing.T) {
	isolateGameConfigPath(t)

	zero := 0
	gs := GameSettings{Entracte: EntracteConfig{
		Title: "Pause", Subtitle: "On revient", PanelSize: 70, AnimPeriod: 8, AnimIntensity: &zero,
	}}
	ApplyGameSettingsDefaults(&gs)
	gs.ValidateAndClampEntracte()

	if err := SaveGameSettings(&gs); err != nil {
		t.Fatalf("SaveGameSettings failed: %v", err)
	}

	loaded, err := LoadGameSettings(GameConfigPath())
	if err != nil {
		t.Fatalf("LoadGameSettings failed: %v", err)
	}
	if loaded.Entracte.AnimIntensity == nil || *loaded.Entracte.AnimIntensity != 0 {
		t.Errorf("AnimIntensity did not survive a save/load round-trip: got %v, want 0", loaded.Entracte.AnimIntensity)
	}
	if loaded.Entracte.Title != "Pause" || loaded.Entracte.PanelSize != 70 {
		t.Errorf("other Entracte fields did not survive the round-trip: %+v", loaded.Entracte)
	}
}

func intPtr(v int) *int { return &v }
