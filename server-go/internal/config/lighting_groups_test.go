package config

import (
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
		{"two hosts", []LightingGroupEntry{{Name: "H1", Role: "animateur"}, {Name: "H2", Role: "animateur"}}, "un seul groupe animateur"},
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
	many := make([]LightingGroupEntry, MaxLightingGroups+1)
	for i := range many {
		many[i] = LightingGroupEntry{Name: string(rune('a' + i))}
	}
	if _, err := ValidateLightingGroups(many, lights); err == nil {
		t.Fatal("13 groups must be refused")
	}
}

func TestPruneAndAssignment(t *testing.T) {
	t.Parallel()
	groups := []LightingGroupEntry{
		{Name: "G1", Role: "salle", Lights: []string{"A", "T"}},
		{Name: "H", Role: "animateur", Lights: []string{"B"}},
		{Name: "G2", Role: "salle", Lights: []string{"C"}},
	}
	lights := []LightingLightEntry{{Name: "A", Role: "general"}, {Name: "B", Role: "general"}, {Name: "T", Role: "team", Team: "x"}}
	pruned := PruneLightingGroups(groups, lights)
	if len(pruned[0].Lights) != 1 || pruned[0].Lights[0] != "A" || len(pruned[2].Lights) != 0 {
		t.Fatalf("prune wrong: %+v", pruned)
	}
	num, host := LightingConfig{Groups: pruned}.GroupAssignment()
	if num["A"] != 1 || !host["B"] || len(num) != 1 {
		t.Fatalf("assignment wrong: %v %v", num, host)
	}
	if _, ok := (LightingConfig{Groups: pruned}).HostGroup(); !ok {
		t.Fatal("host group not found")
	}
}
