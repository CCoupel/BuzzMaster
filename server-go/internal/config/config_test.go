package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNeonEffectDefaults(t *testing.T) {
	// Create temp config file with minimal data
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write minimal config
	minimalConfig := `{
		"version": "2.46.0"
	}`
	tmpFile.WriteString(minimalConfig)
	tmpFile.Close()

	// Load config
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check neon effect defaults
	if cfg.NeonEffect.ArcWidth != 60 {
		t.Errorf("Expected ArcWidth=60, got %d", cfg.NeonEffect.ArcWidth)
	}
	if cfg.NeonEffect.IntensityGap != 80 {
		t.Errorf("Expected IntensityGap=80, got %d", cfg.NeonEffect.IntensityGap)
	}
	if cfg.NeonEffect.RotationSpeed != 4.0 {
		t.Errorf("Expected RotationSpeed=4.0, got %.1f", cfg.NeonEffect.RotationSpeed)
	}
	if cfg.NeonEffect.Enabled != false {
		t.Errorf("Expected Enabled=false, got %v", cfg.NeonEffect.Enabled)
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
			cfg := &Config{NeonEffect: tt.input}
			cfg.ValidateAndClampNeonEffect()

			if cfg.NeonEffect.ArcWidth != tt.expectedArc {
				t.Errorf("ArcWidth: got %d, want %d", cfg.NeonEffect.ArcWidth, tt.expectedArc)
			}
			if cfg.NeonEffect.IntensityGap != tt.expectedGap {
				t.Errorf("IntensityGap: got %d, want %d", cfg.NeonEffect.IntensityGap, tt.expectedGap)
			}
			if cfg.NeonEffect.RotationSpeed != tt.expectedSpeed {
				t.Errorf("RotationSpeed: got %.1f, want %.1f", cfg.NeonEffect.RotationSpeed, tt.expectedSpeed)
			}
		})
	}
}

func TestConfigJSONRoundtrip(t *testing.T) {
	// Test that config can be marshaled and unmarshaled correctly
	original := &Config{
		Version: "2.46.0",
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
	var decoded Config
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
