package game

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : persistance de Team.COLOR_NAME (#113, tâche B4)
//
// COLOR_NAME est écrit par le frontend à chaque attribution/sélection de couleur
// (contracts/models.md § Team) et sert au backend à résoudre la couleur LED exacte
// du buzzer (teamColorToRGB). Le champ porte `omitempty` — ces tests vérifient
// qu'aucun aller-retour (SetTeams → SaveTeams → LoadTeams) ni la sérialisation de
// l'état diffusé (GetGameJSON, utilisée par /ws/admin) ne le supprime silencieusement.
// ---------------------------------------------------------------------------

// TestTeam_ColorName_SurvivesSaveLoadRoundtrip verifies COLOR_NAME survives a full
// SetTeams → SaveTeams → (fresh engine) LoadTeams round-trip.
func TestTeam_ColorName_SurvivesSaveLoadRoundtrip(t *testing.T) {
	teamsPath := filepath.Join(t.TempDir(), "teams.json")

	e1 := NewEngine()
	e1.SetTeamsPath(teamsPath)
	e1.SetTeams(map[string]*Team{
		"Les Rouges": {Name: "Les Rouges", Color: []int{255, 26, 26}, ColorName: "rouge"},
		"Les Marine": {Name: "Les Marine", Color: []int{0, 54, 179}, ColorName: "bleu-profond"},
	})

	// SetTeams saves asynchronously (go e.SaveTeams()) — call it again synchronously
	// so the round-trip below is deterministic instead of racing a goroutine.
	if err := e1.SaveTeams(); err != nil {
		t.Fatalf("SaveTeams failed: %v", err)
	}

	e2 := NewEngine()
	e2.SetTeamsPath(teamsPath)
	if err := e2.LoadTeams(); err != nil {
		t.Fatalf("LoadTeams failed: %v", err)
	}

	rouge := e2.GetTeam("Les Rouges")
	if rouge == nil {
		t.Fatal("Les Rouges team missing after reload")
	}
	if rouge.ColorName != "rouge" {
		t.Errorf("Les Rouges: COLOR_NAME = %q after reload, want %q", rouge.ColorName, "rouge")
	}

	marine := e2.GetTeam("Les Marine")
	if marine == nil {
		t.Fatal("Les Marine team missing after reload")
	}
	if marine.ColorName != "bleu-profond" {
		t.Errorf("Les Marine: COLOR_NAME = %q after reload, want %q", marine.ColorName, "bleu-profond")
	}
}

// TestTeam_ColorName_AbsentForLegacyTeam verifies a team saved before #113 (no
// COLOR_NAME) still round-trips cleanly — COLOR_NAME stays empty, no panic, no
// spurious value introduced by the save/load path.
func TestTeam_ColorName_AbsentForLegacyTeam(t *testing.T) {
	teamsPath := filepath.Join(t.TempDir(), "teams.json")

	e1 := NewEngine()
	e1.SetTeamsPath(teamsPath)
	e1.SetTeams(map[string]*Team{
		"Legacy": {Name: "Legacy", Color: []int{200, 100, 50}},
	})
	if err := e1.SaveTeams(); err != nil {
		t.Fatalf("SaveTeams failed: %v", err)
	}

	e2 := NewEngine()
	e2.SetTeamsPath(teamsPath)
	if err := e2.LoadTeams(); err != nil {
		t.Fatalf("LoadTeams failed: %v", err)
	}

	legacy := e2.GetTeam("Legacy")
	if legacy == nil {
		t.Fatal("Legacy team missing after reload")
	}
	if legacy.ColorName != "" {
		t.Errorf("Legacy team: COLOR_NAME = %q after reload, want empty (never wrote one)", legacy.ColorName)
	}
}

// TestGetGameJSON_IncludesColorName verifies COLOR_NAME is present in the state
// diffused to clients (GetGameJSON — the FULL/UPDATE payload consumed by /ws/admin
// via SerializeForAdmin, which passes it through unmodified). A silent drop here
// would make the admin UI unable to tell which palette entry a team is using.
func TestGetGameJSON_IncludesColorName(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"Les Rouges": {Name: "Les Rouges", Color: []int{255, 26, 26}, ColorName: "rouge"},
	})

	raw := e.GetGameJSON()

	var decoded struct {
		Teams map[string]struct {
			ColorName string `json:"COLOR_NAME"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GetGameJSON output: %v", err)
	}

	team, ok := decoded.Teams["Les Rouges"]
	if !ok {
		t.Fatal("Les Rouges missing from GetGameJSON() teams")
	}
	if team.ColorName != "rouge" {
		t.Errorf("GetGameJSON teams[\"Les Rouges\"].COLOR_NAME = %q, want %q", team.ColorName, "rouge")
	}
}
