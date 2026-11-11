package ambiance

import (
	"context"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
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

// v11.7.0, #225 (light-effects.md §2.5): the animateur group is a fixed white,
// untouched by the room override (OFF here) and absent when no such group.
// Not parallel: it swaps the config singleton.
func TestScene_HostGroupFixedWhiteIgnoresOverride_225(t *testing.T) {
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })
	cfg := *prev
	cfg.Lighting.Groups = nil
	config.SetInstance(&cfg)

	c := New(fakeHost{engine: game.NewEngine()})
	hostEntries := func(st lighting.State) []lighting.ZoneState {
		var out []lighting.ZoneState
		for _, z := range st.Zones {
			if z.Group == lighting.GroupHost {
				out = append(out, z)
			}
		}
		return out
	}
	if got := hostEntries(c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(got) != 0 {
		t.Fatalf("sans groupe animateur, aucune entrée GroupHost : %+v", got)
	}

	cfg.Lighting.Groups = []config.LightingGroupEntry{{Name: "Pupitre", Role: config.GroupRoleAnimateur, Lights: []string{"B"}}}
	if err := c.SetLightingMode("OFF"); err != nil {
		t.Fatal(err)
	}
	st := c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if st.Zones[0].Zone != lighting.ZoneGeneral || st.Zones[0].Group != 0 || st.Zones[0].Intensity != 0 {
		t.Fatalf("la salle reste pilotée par l'override OFF, entrée Group 0 en tête : %+v", st.Zones[0])
	}
	got := hostEntries(st)
	if len(got) != 1 || got[0].Zone != lighting.ZoneGeneral || got[0].Color != [3]int{255, 255, 255} || got[0].Intensity != 255 {
		t.Fatalf("blanc fixe attendu malgré OFF : %+v", got)
	}
	if err := c.SetLightingMode("ON"); err != nil {
		t.Fatal(err)
	}
	c.SetLightingFlash(true)
	got = hostEntries(c.Scene(lighting.Event{Kind: lighting.KindRunning}))
	c.SetLightingFlash(false)
	if len(got) != 1 || got[0].Intensity != 255 {
		t.Fatalf("ni ON ni Flash ne touchent le groupe animateur : %+v", got)
	}
}
