package config

// Tests de spécification v11.7.0 LOT-1 (#225), révision 3 — contracts/light-effects.md §2.1, §2.5 : groupes SANS NOM ni
// `near_teams` (un groupe n'a qu'un rang), `id` stable, plusieurs groupes animateur (4) et 12 groupes de salle,
// retrait automatique, numérotation, lecture tolérante de l'ancienne configuration (name / near_teams ignorés).

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
	cfg.Lighting.Groups = []LightingGroupEntry{{}}
	ApplyDefaults(&cfg)
	if g := cfg.Lighting.Groups[0]; g.Role != "salle" || g.Lights == nil {
		t.Fatalf("rôle absent ⇒ salle, liste jamais nil : %+v", g)
	}
}

func TestTW225_GroupHasNoNameNorNearTeamsKey_InJSON(t *testing.T) {
	b, err := json.Marshal(LightingGroupEntry{ID: "a1", Role: "salle", Lights: []string{"L1"}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, gone := range []string{`"name"`, `"near_teams"`} {
		if strings.Contains(s, gone) {
			t.Errorf("révision 3 : la clé %s n'existe plus dans un groupe : %s", gone, s)
		}
	}
	for _, want := range []string{`"id":"a1"`, `"role":"salle"`, `"lights":["L1"]`} {
		if !strings.Contains(s, want) {
			t.Errorf("clé %s attendue : %s", want, s)
		}
	}
}

func TestTW225_OldGroupsWithNameAndNearTeams_ReadWithoutError_KeysDropped(t *testing.T) {
	// config.json de QUALIF antérieure : name / near_teams présents. Lecture tolérante : ignorés, rien ne casse,
	// et ils disparaissent au prochain enregistrement.
	var cfg Config
	raw := `{"lighting":{"lights":[{"name":"L1","role":"general"}],"groups":[` +
		`{"name":"Scène gauche","role":"salle","lights":["L1"],"near_teams":["Rouges"]}]}}`
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("l'ancienne configuration se lit sans erreur : %v", err)
	}
	ApplyDefaults(&cfg)
	if len(cfg.Lighting.Groups) != 1 || len(cfg.Lighting.Groups[0].Lights) != 1 {
		t.Fatalf("le groupe est conservé : %+v", cfg.Lighting.Groups)
	}
	out, _ := json.Marshal(cfg.Lighting.Groups)
	if strings.Contains(string(out), "Scène") || strings.Contains(string(out), "near_teams") || strings.Contains(string(out), `"name"`) {
		t.Errorf("name et near_teams disparaissent à l'enregistrement : %s", out)
	}
}

func TestTW225_Validate_Accepts(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{
		{Lights: []string{"L1"}},
		{Role: "animateur", Lights: []string{"L2", "L3"}},
		{},
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
	cases := map[string][]LightingGroupEntry{
		"treize groupes de salle":  many,
		"ampoule inconnue":         {{Lights: []string{"Z"}}},
		"ampoule d'équipe":         {{Lights: []string{"T1"}}},
		"ampoule en double":        {{Lights: []string{"L1"}}, {Lights: []string{"L1"}}},
		"ampoule répétée":          {{Lights: []string{"L1", "L1"}}},
		"rôle inconnu":             {{Role: "x"}},
		"cinq animateurs":          {{Role: "animateur"}, {Role: "animateur"}, {Role: "animateur"}, {Role: "animateur"}, {Role: "animateur"}},
		"id majuscules":            {{ID: "ABC"}},
		"id trop long":             {{ID: "abcdefghijklm"}},
		"id avec tiret":            {{ID: "a-b"}},
		"id en double":             {{ID: "x1"}, {ID: "x1"}},
		"id en double salle/anim.": {{ID: "x1"}, {ID: "x1", Role: "animateur"}},
	}
	for name, groups := range cases {
		if _, err := ValidateLightingGroups(groups, tw225Lights()); err == nil {
			t.Errorf("%s : un refus était attendu", name)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s : message vide", name)
		}
	}
}

func TestTW225_Validate_NoNameRules_ManyUnnamedGroupsAccepted(t *testing.T) {
	// plus de nom : ni unicité, ni longueur, ni « nom vide » ne peuvent refuser des groupes
	groups := []LightingGroupEntry{{}, {}, {}, {Role: "animateur"}, {Role: "animateur"}}
	if _, err := ValidateLightingGroups(groups, tw225Lights()); err != nil {
		t.Errorf("des groupes sans nom sont valides : %v", err)
	}
}

func TestTW225_Validate_SeveralHostGroups_UpToFour_CapsIndependent(t *testing.T) {
	hosts := []LightingGroupEntry{{Role: "animateur"}, {Role: "animateur"}, {Role: "animateur"}, {Role: "animateur"}}
	if _, err := ValidateLightingGroups(hosts, tw225Lights()); err != nil {
		t.Fatalf("4 groupes animateur acceptés : %v", err)
	}
	room := make([]LightingGroupEntry, 12)
	if _, err := ValidateLightingGroups(append(append([]LightingGroupEntry{}, room...), hosts...), tw225Lights()); err != nil {
		t.Errorf("12 groupes de salle + 4 animateur : plafonds indépendants : %v", err)
	}
	if _, err := ValidateLightingGroups(append(room, LightingGroupEntry{}), tw225Lights()); err == nil {
		t.Error("13 groupes de salle refusés")
	}
}

func TestTW225_Validate_AssignsWellFormedUniqueIDs_KeepsGivenOnes(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{
		{ID: "keep1"}, {}, {}, {Role: "animateur"}, {Role: "animateur"},
	}, tw225Lights())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[a-z0-9]{1,12}$`)
	seen := map[string]bool{}
	for _, g := range out {
		if !re.MatchString(g.ID) || seen[g.ID] {
			t.Errorf("id [a-z0-9]{1,12} unique (groupes sans nom : plusieurs ids distincts) : %q (%+v)", g.ID, out)
		}
		seen[g.ID] = true
	}
	if out[0].ID != "keep1" {
		t.Errorf("un id fourni est conservé tel quel : %q", out[0].ID)
	}
}

func TestTW225_Validate_IDStableAcrossReorder(t *testing.T) {
	first, err := ValidateLightingGroups([]LightingGroupEntry{{}, {Role: "animateur"}}, tw225Lights())
	if err != nil {
		t.Fatal(err)
	}
	second, err := ValidateLightingGroups([]LightingGroupEntry{first[1], first[0]}, tw225Lights())
	if err != nil {
		t.Fatal(err)
	}
	if second[0].ID != first[1].ID || second[1].ID != first[0].ID {
		t.Errorf("réordonner ne change aucun id : %+v → %+v", first, second)
	}
}

func TestTW225_Prune_RemovesBulbsNoLongerGeneral_KeepsGroups(t *testing.T) {
	groups := []LightingGroupEntry{{Lights: []string{"L1", "L2"}}, {Lights: []string{"L3"}}}
	lights := []LightingLightEntry{{Name: "L1", Role: "team", Team: "R"}, {Name: "L2", Role: "general"}}
	out := PruneLightingGroups(groups, lights)
	if len(out) != 2 {
		t.Fatalf("les groupes sont conservés : %+v", out)
	}
	if len(out[0].Lights) != 1 || out[0].Lights[0] != "L2" {
		t.Errorf("L1 (équipe) doit quitter son groupe : %+v", out[0])
	}
	if len(out[1].Lights) != 0 || out[1].Lights == nil {
		t.Errorf("L3 (supprimée) doit quitter son groupe, liste vide non nil : %+v", out[1])
	}
	if len(groups[0].Lights) != 2 {
		t.Error("l'entrée ne doit pas être modifiée en place")
	}
}

func TestTW225_HostGroups_OwnRankIncludingEmptyOnes_NotInRoomNumbering(t *testing.T) {
	l := LightingConfig{Groups: []LightingGroupEntry{
		{ID: "a1", Role: "animateur", Lights: []string{}},
		{ID: "s1", Role: "salle", Lights: []string{"L1"}},
		{ID: "a2", Role: "animateur", Lights: []string{"H"}},
		{ID: "s2", Lights: []string{"L2"}},
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

func TestTW225_RoomGroups_NonEmptyOnly_RealNumbers(t *testing.T) {
	l := LightingConfig{Groups: []LightingGroupEntry{
		{Lights: []string{"A"}}, {Lights: []string{}}, {Role: "animateur", Lights: []string{"H"}}, {Lights: []string{"C"}},
	}}
	got := l.RoomGroups()
	if len(got) != 2 || got[0].Num != 1 || got[1].Num != 3 {
		t.Errorf("N = groupes de salle non vides, avec leur numéro réel (1 et 3) : %+v", got)
	}
}

func TestTW225_OldConfigWithSingleHostGroupWithoutID_StillValidAndGetsAnID(t *testing.T) {
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
	for k := range cfg.Lighting.HostEffects {
		if k == "chrono" || k == "calm" || k == "score" {
			t.Errorf("l'ancienne forme plate est ignorée, jamais prise pour un id de groupe : %v", cfg.Lighting.HostEffects)
		}
	}
}

func TestTW225_AssignGroupIDs_UnnamedGroupsGetDistinctIDs(t *testing.T) {
	groups := []LightingGroupEntry{{}, {}, {}}
	AssignGroupIDs(groups)
	seen := map[string]bool{}
	for _, g := range groups {
		if g.ID == "" || seen[g.ID] {
			t.Errorf("ids distincts pour des groupes sans nom : %+v", groups)
		}
		seen[g.ID] = true
	}
}
