package config

// Tests de spécification v11.7.0 LOT-1 (#225) — contracts/light-effects.md §2.1
// et §2.5 : validation de lighting.groups, rôle animateur unique, near_teams
// refusé sur l'animateur, retrait automatique, numérotation. Aides tw225*.

import (
	"encoding/json"
	"strings"
	"testing"
)

func tw225Lights() []LightingLightEntry {
	return []LightingLightEntry{
		{Name: "L1", Role: "general"}, {Name: "L2", Role: "general"}, {Name: "L3"}, // rôle vide = general
		{Name: "T1", Role: "team", Team: "Rouges"},
	}
}

func TestTW225_ApplyDefaults_GroupsNeverNull_RoleSalle(t *testing.T) {
	var cfg Config
	ApplyDefaults(&cfg)
	b, _ := json.Marshal(cfg.Lighting)
	if !strings.Contains(string(b), `"groups":[]`) {
		t.Fatalf("lighting.groups doit être sérialisé en [] par défaut : %s", b)
	}
	cfg.Lighting.Groups = []LightingGroupEntry{{Name: "A"}}
	ApplyDefaults(&cfg)
	g := cfg.Lighting.Groups[0]
	if g.Role != "salle" || g.Lights == nil || g.NearTeams == nil {
		t.Fatalf("rôle absent ⇒ salle, listes jamais nil : %+v", g)
	}
}

func TestTW225_Validate_Accepts(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{
		{Name: "Scène", Lights: []string{"L1"}, NearTeams: []string{"Rouges"}},
		{Name: "Pupitre", Role: "animateur", Lights: []string{"L2", "L3"}},
		{Name: "Vide"},
	}, tw225Lights())
	if err != nil {
		t.Fatalf("configuration valide refusée : %v", err)
	}
	if len(out) != 3 || out[0].Role != "salle" || out[2].Lights == nil {
		t.Fatalf("groupes canoniques attendus (rôle par défaut, listes non nil) : %+v", out)
	}
}

func TestTW225_Validate_Refuses(t *testing.T) {
	many := make([]LightingGroupEntry, 13)
	for i := range many {
		many[i] = LightingGroupEntry{Name: "G" + string(rune('A'+i))}
	}
	cases := map[string][]LightingGroupEntry{
		"treize groupes":           many,
		"ampoule inconnue":         {{Name: "A", Lights: []string{"Z"}}},
		"ampoule d'équipe":         {{Name: "A", Lights: []string{"T1"}}},
		"ampoule en double":        {{Name: "A", Lights: []string{"L1"}}, {Name: "B", Lights: []string{"L1"}}},
		"ampoule répétée":          {{Name: "A", Lights: []string{"L1", "L1"}}},
		"nom en double":            {{Name: "A"}, {Name: "a"}},
		"nom vide":                 {{Name: "  "}},
		"nom trop long":            {{Name: strings.Repeat("é", 41)}},
		"rôle inconnu":             {{Name: "A", Role: "x"}},
		"deux animateurs":          {{Name: "A", Role: "animateur"}, {Name: "B", Role: "animateur"}},
		"near_teams animateur":     {{Name: "A", Role: "animateur", NearTeams: []string{"Rouges"}}},
		"équipe près de 2 groupes": {{Name: "A", NearTeams: []string{"R"}}, {Name: "B", NearTeams: []string{"R"}}},
	}
	for name, groups := range cases {
		if _, err := ValidateLightingGroups(groups, tw225Lights()); err == nil {
			t.Errorf("%s : un refus était attendu", name)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s : message vide", name)
		}
	}
}

func TestTW225_Validate_NameLengthCountsCharactersNotBytes(t *testing.T) {
	if _, err := ValidateLightingGroups([]LightingGroupEntry{{Name: strings.Repeat("é", 40)}}, tw225Lights()); err != nil {
		t.Fatalf("40 caractères (accentués) acceptés : %v", err)
	}
}

func TestTW225_Prune_RemovesBulbsNoLongerGeneral_KeepsGroups(t *testing.T) {
	groups := []LightingGroupEntry{{Name: "A", Lights: []string{"L1", "L2"}}, {Name: "B", Lights: []string{"L3"}}}
	lights := []LightingLightEntry{{Name: "L1", Role: "team", Team: "R"}, {Name: "L2", Role: "general"}}
	out := PruneLightingGroups(groups, lights)
	if len(out) != 2 {
		t.Fatalf("les groupes sont conservés : %+v", out)
	}
	if len(out[0].Lights) != 1 || out[0].Lights[0] != "L2" {
		t.Errorf("L1 (équipe) doit quitter A : %+v", out[0])
	}
	if len(out[1].Lights) != 0 || out[1].Lights == nil {
		t.Errorf("L3 (supprimée) doit quitter B, liste vide non nil : %+v", out[1])
	}
	if len(groups[0].Lights) != 2 {
		t.Error("l'entrée ne doit pas être modifiée en place")
	}
}

func TestTW225_GroupAssignment_NumberIsRankAmongSalleGroups_HostExcluded(t *testing.T) {
	l := LightingConfig{Groups: []LightingGroupEntry{
		{Name: "Pupitre", Role: "animateur", Lights: []string{"H"}},
		{Name: "A", Role: "salle", Lights: []string{"L1"}},
		{Name: "B", Lights: []string{"L2"}},
	}}
	num, host := l.GroupAssignment()
	if num["L1"] != 1 || num["L2"] != 2 {
		t.Errorf("numéros = rang parmi les groupes de salle : %v", num)
	}
	if !host["H"] || num["H"] != 0 {
		t.Errorf("H est animateur, sans numéro : num=%v host=%v", num, host)
	}
	if g, ok := l.HostGroup(); !ok || g.Name != "Pupitre" {
		t.Errorf("HostGroup : %+v %v", g, ok)
	}
	if _, ok := (LightingConfig{}).HostGroup(); ok {
		t.Error("sans groupe animateur, HostGroup renvoie false")
	}
}
