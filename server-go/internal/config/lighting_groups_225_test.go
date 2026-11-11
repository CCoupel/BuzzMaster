package config

// Tests de spécification v11.7.0 LOT-1 (#225) — contracts/light-effects.md §2.1
// et §2.5 : validation de lighting.groups, rôle animateur unique, near_teams
// refusé sur l'animateur, retrait automatique, numérotation. Aides tw225*.

import (
	"encoding/json"
	"regexp"
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
		"cinq animateurs":          {{Name: "A", Role: "animateur"}, {Name: "B", Role: "animateur"}, {Name: "C", Role: "animateur"}, {Name: "D", Role: "animateur"}, {Name: "E", Role: "animateur"}},
		"id majuscules":            {{ID: "ABC", Name: "A"}},
		"id trop long":             {{ID: "abcdefghijklm", Name: "A"}},
		"id avec tiret":            {{ID: "a-b", Name: "A"}},
		"id en double":             {{ID: "x1", Name: "A"}, {ID: "x1", Name: "B"}},
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

func TestTW225_Validate_SeveralHostGroups_UpToFour_CapsIndependent(t *testing.T) {
	hosts := []LightingGroupEntry{{Name: "A", Role: "animateur"}, {Name: "B", Role: "animateur"}, {Name: "C", Role: "animateur"}, {Name: "D", Role: "animateur"}}
	if _, err := ValidateLightingGroups(hosts, tw225Lights()); err != nil {
		t.Fatalf("4 groupes animateur acceptés : %v", err)
	}
	room := make([]LightingGroupEntry, 12)
	for i := range room {
		room[i] = LightingGroupEntry{Name: "S" + string(rune('A'+i))}
	}
	if _, err := ValidateLightingGroups(append(append([]LightingGroupEntry{}, room...), hosts...), tw225Lights()); err != nil {
		t.Errorf("12 groupes de salle + 4 animateur : plafonds indépendants : %v", err)
	}
	if _, err := ValidateLightingGroups(append(room, LightingGroupEntry{Name: "Trop"}), tw225Lights()); err == nil {
		t.Error("13 groupes de salle refusés")
	}
}

func TestTW225_Validate_AssignsWellFormedUniqueIDs_KeepsGivenOnes(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{
		{ID: "keep1", Name: "A"}, {Name: "B"}, {Name: "C", Role: "animateur"}, {Name: "D", Role: "animateur"},
	}, tw225Lights())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[a-z0-9]{1,12}$`)
	seen := map[string]bool{}
	for _, g := range out {
		if !re.MatchString(g.ID) || seen[g.ID] {
			t.Errorf("id [a-z0-9]{1,12} unique : %q (%+v)", g.ID, out)
		}
		seen[g.ID] = true
	}
	if out[0].ID != "keep1" {
		t.Errorf("un id fourni est conservé tel quel : %q", out[0].ID)
	}
}

func TestTW225_Validate_IDStableAcrossReorderAndRename(t *testing.T) {
	first, _ := ValidateLightingGroups([]LightingGroupEntry{{Name: "A"}, {Name: "B", Role: "animateur"}}, tw225Lights())
	back := []LightingGroupEntry{first[1], first[0]}
	back[0].Name, back[1].Name = "Régie", "Scène"
	second, err := ValidateLightingGroups(back, tw225Lights())
	if err != nil {
		t.Fatal(err)
	}
	if second[0].ID != first[1].ID || second[1].ID != first[0].ID {
		t.Errorf("renommer et réordonner ne change aucun id : %+v → %+v", first, second)
	}
}

func TestTW225_HostGroups_OwnRankIncludingEmptyOnes_NotInRoomNumbering(t *testing.T) {
	l := LightingConfig{Groups: []LightingGroupEntry{
		{ID: "a1", Name: "Pupitre", Role: "animateur", Lights: []string{}},
		{ID: "s1", Name: "A", Role: "salle", Lights: []string{"L1"}},
		{ID: "a2", Name: "Régie", Role: "animateur", Lights: []string{"H"}},
		{ID: "s2", Name: "B", Lights: []string{"L2"}},
	}}
	hosts := l.HostGroups()
	if len(hosts) != 2 || hosts[0].ID != "a1" || hosts[0].Rank != 1 || hosts[1].ID != "a2" || hosts[1].Rank != 2 {
		t.Errorf("rang propre des animateurs, vides compris : %+v", hosts)
	}
	num, host := l.GroupAssignment()
	if num["L1"] != 1 || num["L2"] != 2 || num["H"] != 0 {
		t.Errorf("numéros de salle : rang parmi les groupes de salle, animateurs exclus : %v", num)
	}
	if host["H"] != 2 {
		t.Errorf("H est dans le groupe animateur de rang 2 : %v", host)
	}
	if len((LightingConfig{}).HostGroups()) != 0 {
		t.Error("sans groupe animateur, liste vide")
	}
}

func TestTW225_OldConfigWithSingleHostGroupWithoutID_StillValidAndGetsAnID(t *testing.T) {
	// config.json de QUALIF 11.7.0.2 : un groupe animateur, aucun `id`.
	var cfg Config
	raw := `{"lighting":{"lights":[{"name":"L1","role":"general"}],"groups":[{"name":"Pupitre","role":"animateur","lights":["L1"],"near_teams":[]}],` +
		`"host_effects":{"chrono":"","calm":"","score":"couleur-equipe"}}}`
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("l'ancienne configuration (host_effects à plat) se lit sans erreur : %v", err)
	}
	ApplyDefaults(&cfg)
	if len(cfg.Lighting.Groups) != 1 || cfg.Lighting.Groups[0].Role != "animateur" {
		t.Fatalf("groupe animateur conservé : %+v", cfg.Lighting.Groups)
	}
	AssignGroupIDs(cfg.Lighting.Groups)
	if cfg.Lighting.Groups[0].ID == "" {
		t.Error("un id est attribué au groupe sans id")
	}
	if len(cfg.Lighting.HostEffects) != 0 {
		for k := range cfg.Lighting.HostEffects {
			if k == "chrono" || k == "calm" || k == "score" {
				t.Errorf("l'ancienne forme plate est ignorée, jamais prise pour un id de groupe : %v", cfg.Lighting.HostEffects)
			}
		}
	}
}
