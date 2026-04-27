package protocol

import (
	"encoding/json"
	"testing"
)

// ========================================
// Issue #50 — LEDEffectComet constant
// ========================================

// TestLEDEffectComet_Defined verifies the COMET LED effect constant is defined
// and has the correct string value expected by the BuzzClick firmware.
func TestLEDEffectComet_Defined(t *testing.T) {
	if LEDEffectComet == "" {
		t.Fatal("LEDEffectComet must not be empty")
	}
	if LEDEffectComet != "COMET" {
		t.Errorf("LEDEffectComet: got %q, want %q", LEDEffectComet, "COMET")
	}
}

// TestLEDEffectConstants_AllDefined verifies all LED effect constants are defined
// and have non-empty values.
func TestLEDEffectConstants_AllDefined(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"LEDEffectSolid", LEDEffectSolid},
		{"LEDEffectBlink", LEDEffectBlink},
		{"LEDEffectDim", LEDEffectDim},
		{"LEDEffectComet", LEDEffectComet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Errorf("%s must not be empty", tt.name)
			}
		})
	}
}

// TestLEDEffectConstants_Unique verifies all LED effect constants have distinct values.
func TestLEDEffectConstants_Unique(t *testing.T) {
	values := map[string]string{
		"SOLID": LEDEffectSolid,
		"BLINK": LEDEffectBlink,
		"DIM":   LEDEffectDim,
		"COMET": LEDEffectComet,
	}
	seen := map[string]string{}
	for name, val := range values {
		if prev, exists := seen[val]; exists {
			t.Errorf("LEDEffect constants collision: %s and %s both equal %q", prev, name, val)
		}
		seen[val] = name
	}
}

// TestLEDSetPayload_WithComet verifies that a LEDSetPayload can be created and
// serialised with the COMET effect, producing a valid JSON message.
func TestLEDSetPayload_WithComet(t *testing.T) {
	payload := LEDSetPayload{
		Color:     [3]int{255, 215, 0}, // Gold
		Intensity: 255,
		Effect:    LEDEffectComet,
	}

	msg, err := NewMessage(ActionLEDSet, payload)
	if err != nil {
		t.Fatalf("NewMessage with COMET payload failed: %v", err)
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("SerializeForWebSocket failed: %v", err)
	}

	// Deserialise and verify
	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal serialised message: %v", err)
	}

	if parsed.Action != ActionLEDSet {
		t.Errorf("Action: got %q, want %q", parsed.Action, ActionLEDSet)
	}

	var gotPayload LEDSetPayload
	if err := json.Unmarshal(parsed.Msg, &gotPayload); err != nil {
		t.Fatalf("Failed to unmarshal LEDSetPayload: %v", err)
	}

	if gotPayload.Effect != LEDEffectComet {
		t.Errorf("Effect: got %q, want %q", gotPayload.Effect, LEDEffectComet)
	}
	if gotPayload.Color != [3]int{255, 215, 0} {
		t.Errorf("Color: got %v, want [255 215 0]", gotPayload.Color)
	}
	if gotPayload.Intensity != 255 {
		t.Errorf("Intensity: got %d, want 255", gotPayload.Intensity)
	}
}

// TestLEDSetPayload_TableDriven verifies various LED effect payloads round-trip correctly.
func TestLEDSetPayload_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		payload   LEDSetPayload
		wantEffect string
	}{
		{
			name:      "solid red",
			payload:   LEDSetPayload{Color: [3]int{255, 0, 0}, Intensity: 255, Effect: LEDEffectSolid},
			wantEffect: LEDEffectSolid,
		},
		{
			name:      "blink green",
			payload:   LEDSetPayload{Color: [3]int{0, 255, 0}, Intensity: 200, Effect: LEDEffectBlink},
			wantEffect: LEDEffectBlink,
		},
		{
			name:      "dim blue",
			payload:   LEDSetPayload{Color: [3]int{0, 0, 255}, Intensity: 64, Effect: LEDEffectDim},
			wantEffect: LEDEffectDim,
		},
		{
			name:      "comet gold — point award",
			payload:   LEDSetPayload{Color: [3]int{255, 215, 0}, Intensity: 255, Effect: LEDEffectComet},
			wantEffect: LEDEffectComet,
		},
		{
			name:      "comet zero intensity",
			payload:   LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: LEDEffectComet},
			wantEffect: LEDEffectComet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessage(ActionLEDSet, tt.payload)
			if err != nil {
				t.Fatalf("NewMessage failed: %v", err)
			}

			data, err := msg.SerializeForWebSocket()
			if err != nil {
				t.Fatalf("SerializeForWebSocket failed: %v", err)
			}

			var parsed Message
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			var got LEDSetPayload
			if err := json.Unmarshal(parsed.Msg, &got); err != nil {
				t.Fatalf("Unmarshal payload failed: %v", err)
			}

			if got.Effect != tt.wantEffect {
				t.Errorf("Effect: got %q, want %q", got.Effect, tt.wantEffect)
			}
		})
	}
}
