package ambiance

import (
	"context"
	"testing"
	"time"

	"buzzcontrol/internal/game"
)

// #295 C5 — tests déplacés de cmd/server (fonctions pures, sans App) + tests du Controller avec un Host factice.
// Les scénarios d'ambiance complets (writer, driver Hue, scènes, flash) passent par l'App (newTestApp, serveur HTTP,
// faux pont) et restent dans cmd/server.

func TestDevAmbianceActiveTeam(t *testing.T) {
	q := func(typ game.QuestionType) *game.Question { return &game.Question{Type: typ} }
	tests := []struct {
		name  string
		state game.GameState
		want  string
	}{
		{"no question", game.GameState{}, ""},
		{"classic question", game.GameState{Question: q(game.QuestionTypeSpeedy), MemoryCurrentTeam: "X"}, ""},
		{"memory", game.GameState{Question: q(game.QuestionTypeMemory), MemoryCurrentTeam: "TeamB"}, "TeamB"},
		{"memotion", game.GameState{Question: q(game.QuestionTypeMemotion), MotionCurrentTeam: "TeamC"}, "TeamC"},
		{"rafale", game.GameState{Question: q(game.QuestionTypeRafale), RafaleCurrentTeam: "TeamA"}, "TeamA"},
		{"memory solo (no current team)", game.GameState{Question: q(game.QuestionTypeMemory)}, ""},
	}
	for _, tt := range tests {
		if got := ActiveTeam(tt.state); got != tt.want {
			t.Errorf("%s: active team = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFlashCadenceConstants_291(t *testing.T) {
	t.Parallel()
	if ScoreFlashGoldPhase != 250*time.Millisecond || ScoreFlashTeamPhase != 500*time.Millisecond {
		t.Errorf("clignotement SCORE = %s or / %s équipe, attendu 250 ms / 500 ms (décision 2026-09-08, jamais 400/400)", ScoreFlashGoldPhase, ScoreFlashTeamPhase)
	}
	if FlashOnPhase != 400*time.Millisecond || FlashOffPhase != 400*time.Millisecond {
		t.Errorf("Flash général = %s / %s, attendu 400 ms / 400 ms", FlashOnPhase, FlashOffPhase)
	}
}

func TestFlashPhaseOverride_ZeroValueIsProduction_291(t *testing.T) {
	t.Parallel()
	var zero FlashPhaseOverride
	if OrDefault(zero.LightingOn, FlashOnPhase) != FlashOnPhase ||
		OrDefault(zero.LightingOff, FlashOffPhase) != FlashOffPhase ||
		OrDefault(zero.ScoreGold, ScoreFlashGoldPhase) != ScoreFlashGoldPhase ||
		OrDefault(zero.ScoreTeam, ScoreFlashTeamPhase) != ScoreFlashTeamPhase {
		t.Error("un FlashPhaseOverride nul doit rendre les cadences de production")
	}
	if got := OrDefault(10*time.Millisecond, time.Second); got != 10*time.Millisecond {
		t.Errorf("une valeur injectée l'emporte : %s", got)
	}
}

type fakeHost struct{ engine *game.Engine }

func (h fakeHost) Engine() *game.Engine                                { return h.engine }
func (h fakeHost) Ctx() context.Context                                { return nil }
func (h fakeHost) TeamNameColor(string) [3]int                         { return [3]int{128, 128, 128} }
func (h fakeHost) HasCategoryMeta() bool                               { return false }
func (h fakeHost) ResolveCategoryMeta(string) (string, string, string) { return "", "", "" }

func TestController_ModeDefaultsToAutoAndRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	c := New(fakeHost{engine: game.NewEngine()})
	if got := c.LightingMode(); got != "AUTO" {
		t.Errorf("mode par défaut = %q, attendu AUTO", got)
	}
	if err := c.SetLightingMode("ON"); err != nil || c.LightingMode() != "ON" {
		t.Errorf("SetLightingMode(ON) : err=%v mode=%q", err, c.LightingMode())
	}
	if err := c.SetLightingMode("blink"); err == nil {
		t.Error("une valeur hors ON/AUTO/OFF est refusée")
	} else if c.LightingMode() != "ON" {
		t.Errorf("un refus ne change pas le mode : %q", c.LightingMode())
	}
}

func TestController_NotConfiguredHasNilWriterAndNilDriver(t *testing.T) {
	t.Parallel()
	c := New(fakeHost{engine: game.NewEngine()})
	if c.Writer() != nil || c.LightingDriver() != nil {
		t.Error("sans configuration, le writer et le driver sont nuls (les 21 sites sont des no-op, contrat §4.3)")
	}
	c.Writer().NotifyState() // no-op sur un writer nul
	if c.CurrentScoreFlashTeam() != "" {
		t.Error("aucun clignotement SCORE au départ")
	}
}

func TestController_FlashToggleIsServerDriven(t *testing.T) {
	t.Parallel()
	c := New(fakeHost{engine: game.NewEngine()})
	c.FlashPhases = FlashPhaseOverride{LightingOn: 5 * time.Millisecond, LightingOff: 5 * time.Millisecond}
	if c.LightingFlash() {
		t.Fatal("Flash éteint au départ")
	}
	c.SetLightingFlash(true)
	if !c.LightingFlash() {
		t.Fatal("Flash engagé")
	}
	c.SetLightingFlash(false)
	if c.LightingFlash() {
		t.Fatal("Flash désengagé")
	}
}
