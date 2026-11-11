package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLightingGroups(t *testing.T) {
	t.Parallel()
	lights := []LightingLightEntry{{Name: "A", Role: "general"}, {Name: "B", Role: "general"}, {Name: "T", Role: "team", Team: "Rouges"}}
	tests := []struct {
		name    string
		groups  []LightingGroupEntry
		wantErr string
	}{
		{"ok", []LightingGroupEntry{{Name: "G1", Lights: []string{"A"}}, {Name: "Pupitre", Role: "animateur", Lights: []string{"B"}}}, ""},
		{"unknown light", []LightingGroupEntry{{Name: "G1", Lights: []string{"Z"}}}, "inconnue"},
		{"team light", []LightingGroupEntry{{Name: "G1", Lights: []string{"T"}}}, "équipe"},
		{"light twice", []LightingGroupEntry{{Name: "G1", Lights: []string{"A"}}, {Name: "G2", Lights: []string{"A"}}}, "deux groupes"},
		{"dup name", []LightingGroupEntry{{Name: "G1"}, {Name: "g1"}}, "double"},
		{"five hosts", []LightingGroupEntry{{Name: "H1", Role: "animateur"}, {Name: "H2", Role: "animateur"}, {Name: "H3", Role: "animateur"}, {Name: "H4", Role: "animateur"}, {Name: "H5", Role: "animateur"}}, "4 groupes animateur"},
		{"id invalide", []LightingGroupEntry{{ID: "Bad Id", Name: "G1"}}, "identifiant"},
		{"id en double", []LightingGroupEntry{{ID: "x1", Name: "G1"}, {ID: "x1", Name: "G2"}}, "double"},
		{"host near teams", []LightingGroupEntry{{Name: "H1", Role: "animateur", NearTeams: []string{"Rouges"}}}, "near_teams"},
		{"bad role", []LightingGroupEntry{{Name: "G1", Role: "x"}}, "rôle"},
		{"empty name", []LightingGroupEntry{{Name: " "}}, "nom"},
		{"team near two", []LightingGroupEntry{{Name: "G1", NearTeams: []string{"Rouges"}}, {Name: "G2", NearTeams: []string{"Rouges"}}}, "deux groupes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := ValidateLightingGroups(tt.groups, lights)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if out[0].Role != GroupRoleSalle {
					t.Fatalf("role not defaulted: %q", out[0].Role)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
	many := make([]LightingGroupEntry, MaxSalleGroups+1)
	for i := range many {
		many[i] = LightingGroupEntry{Name: string(rune('a' + i))}
	}
	if _, err := ValidateLightingGroups(many, lights); err == nil {
		t.Fatal("13 groupes de salle refusés")
	}
	// Four animateur groups + twelve salle groups are fine; ids are assigned and stable.
	ok := make([]LightingGroupEntry, 0, 16)
	for i := 0; i < MaxSalleGroups; i++ {
		ok = append(ok, LightingGroupEntry{Name: string(rune('a' + i))})
	}
	for i := 0; i < MaxHostGroups; i++ {
		ok = append(ok, LightingGroupEntry{Name: "H" + string(rune('a'+i)), Role: GroupRoleAnimateur})
	}
	out, err := ValidateLightingGroups(ok, lights)
	if err != nil {
		t.Fatalf("12 + 4 groupes : %v", err)
	}
	seen := map[string]bool{}
	for _, g := range out {
		if !reGroupID.MatchString(g.ID) || seen[g.ID] {
			t.Fatalf("id attribué invalide ou en double : %+v", g)
		}
		seen[g.ID] = true
	}
	again, _ := ValidateLightingGroups(out, lights)
	for i := range out {
		if again[i].ID != out[i].ID {
			t.Fatal("un id renvoyé tel quel est conservé")
		}
	}
}

func TestPruneAndAssignment(t *testing.T) {
	t.Parallel()
	groups := []LightingGroupEntry{
		{Name: "G1", Role: "salle", Lights: []string{"A", "T"}},
		{ID: "h1", Name: "H", Role: "animateur", Lights: []string{"B"}},
		{Name: "G2", Role: "salle", Lights: []string{"C"}},
		{ID: "h2", Name: "H2", Role: "animateur", Lights: []string{"D"}},
	}
	lights := []LightingLightEntry{{Name: "A", Role: "general"}, {Name: "B", Role: "general"}, {Name: "D", Role: "general"}, {Name: "T", Role: "team", Team: "x"}}
	pruned := PruneLightingGroups(groups, lights)
	if len(pruned[0].Lights) != 1 || pruned[0].Lights[0] != "A" || len(pruned[2].Lights) != 0 {
		t.Fatalf("prune wrong: %+v", pruned)
	}
	num, host := LightingConfig{Groups: pruned}.GroupAssignment()
	if num["A"] != 1 || host["B"] != 1 || host["D"] != 2 || len(num) != 1 {
		t.Fatalf("assignment wrong: %v %v", num, host)
	}
	hg := (LightingConfig{Groups: pruned}).HostGroups()
	if len(hg) != 2 || hg[0].ID != "h1" || hg[1].Rank != 2 {
		t.Fatalf("groupes animateur : %+v", hg)
	}
}

// A config.json written by the QUALIF 11.7.0.2 build (single animateur group without id,
// flat host_effects) loads without error: ids are assigned, the flat form is ignored.
func TestLoad_LegacyQualifHostGroupAndFlatHostEffects_Tolerated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{"version":"11.7.0.2","lighting":{"enabled":false,"lights":[{"name":"H1","role":"general"}],` +
		`"groups":[{"name":"Pupitre","role":"animateur","lights":["H1"],"near_teams":[]}],` +
		`"effects":{"chrono":"pulsation"},"host_effects":{"chrono":"","calm":"x","score":"couleur-equipe"}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("le serveur doit démarrer : %v", err)
	}
	hg := cfg.Lighting.HostGroups()
	if len(hg) != 1 || !reGroupID.MatchString(hg[0].ID) || hg[0].Rank != 1 {
		t.Fatalf("id attribué à l'ancien groupe animateur : %+v", hg)
	}
	if len(cfg.Lighting.HostEffects) != 0 {
		t.Fatalf("l'ancienne forme plate est ignorée : %+v", cfg.Lighting.HostEffects)
	}
	if cfg.Lighting.Effects["calm"] != "" || cfg.Lighting.Effects["chrono"] != "pulsation" {
		t.Fatalf("effets de salle conservés / complétés : %+v", cfg.Lighting.Effects)
	}
	cfg2, _ := Load(path)
	if cfg2.Lighting.Groups[0].ID != hg[0].ID {
		t.Fatal("l'id attribué est stable d'un démarrage à l'autre")
	}
}
