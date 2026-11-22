package config

// #309 (v12.1.0, LOT-2) — `wheel.bonus_count` de game-config.json (contracts/roue.md §16.1) : 0 à 4, absent = 0, wheel_default à 0.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeGameConfig309(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "game-config.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// critical — la roue par défaut (« Roue par défaut ») n'a aucun BONUS : une installation v12.0 / LOT-1 garde sa roue.
func TestWheelBonus309_DefaultWheelHasNoBonus(t *testing.T) {
	if got := DefaultWheelConfig().BonusCount; got != 0 {
		t.Fatalf("DefaultWheelConfig().BonusCount = %d, attendu 0", got)
	}
}

// game-config.json sans bonus_count (v12.0 / v12.1 LOT-1) : chargé sans erreur, 0 BONUS, le reste inchangé.
func TestWheelBonus309_AbsentFieldLoadsAsZero(t *testing.T) {
	p := writeGameConfig309(t, `{"wheel":{"max_points":500,"step":50,"bankrupt_count":1,"lose_turn_count":1}}`)
	gs, err := LoadGameSettings(p)
	if err != nil {
		t.Fatalf("chargement refusé : %v", err)
	}
	if gs.Wheel.BonusCount != 0 || gs.Wheel.MaxPoints != 500 || gs.Wheel.BankruptCount != 1 {
		t.Fatalf("wheel = %+v", gs.Wheel)
	}
}

func TestWheelBonus309_SectionAbsentLoadsDefaultWithZeroBonus(t *testing.T) {
	gs, err := LoadGameSettings(writeGameConfig309(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Wheel.BonusCount != 0 || gs.Wheel.MaxPoints != 1000 || gs.Wheel.Step != 50 {
		t.Fatalf("roue par défaut attendue, got %+v", gs.Wheel)
	}
}

// Le BONUS global de 0 à 4 est lu tel quel et survit à un aller-retour d'enregistrement.
func TestWheelBonus309_ValuesRoundTrip(t *testing.T) {
	for n := 0; n <= 4; n++ {
		p := writeGameConfig309(t, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":`+string(rune('0'+n))+`}}`)
		gs, err := LoadGameSettings(p)
		if err != nil {
			t.Fatalf("bonus=%d : %v", n, err)
		}
		if gs.Wheel.BonusCount != n {
			t.Fatalf("bonus=%d : lu %d", n, gs.Wheel.BonusCount)
		}
		raw, _ := json.Marshal(gs)
		var back struct {
			Wheel struct {
				BonusCount *int `json:"bonus_count"`
			} `json:"wheel"`
		}
		_ = json.Unmarshal(raw, &back)
		if back.Wheel.BonusCount == nil || *back.Wheel.BonusCount != n {
			t.Fatalf("bonus=%d : sérialisé %v", n, back.Wheel.BonusCount)
		}
	}
}
