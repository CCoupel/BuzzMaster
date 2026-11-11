package config

// Tests de spécification v11.7.0 LOT-1 (#225), révision 4 — contracts/light-effects.md §2.1, §2.5 : trois zones (ambiance = rôle salle,
// animateur, public), groupes SANS NOM ni `near_teams`, `id` stable, 12 groupes par zone (plafonds indépendants),
// rang par zone (vides compris), zone sans groupe non vide inexistante, lecture tolérante de l'ancienne configuration.

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
		"treize groupes animateur": repeatRole("animateur", 13),
		"treize groupes public":    repeatRole("public", 13),
		"ampoule inconnue":         {{Lights: []string{"Z"}}},
		"ampoule d'équipe":         {{Lights: []string{"T1"}}},
		"ampoule en double":        {{Lights: []string{"L1"}}, {Lights: []string{"L1"}}},
		"ampoule répétée":          {{Lights: []string{"L1", "L1"}}},
		"rôle inconnu":             {{Role: "x"}},
		"id majuscules":            {{ID: "ABC"}},
		"id trop long":             {{ID: "abcdefghijklm"}},
		"id avec tiret":            {{ID: "a-b"}},
		"id en double":             {{ID: "x1"}, {ID: "x1"}},
		"id en double salle/anim.": {{ID: "x1"}, {ID: "x1", Role: "animateur"}},
		"id en double anim/public": {{ID: "x1", Role: "animateur"}, {ID: "x1", Role: "public"}},
		"ampoule dans deux zones":  {{Lights: []string{"L1"}}, {Role: "public", Lights: []string{"L1"}}},
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

func repeatRole(role string, n int) []LightingGroupEntry {
	out := make([]LightingGroupEntry, n)
	for i := range out {
		out[i].Role = role
	}
	return out
}

func TestTW225_Validate_TwelveGroupsPerZone_CapsIndependent(t *testing.T) {
	all := append(append(repeatRole("salle", 12), repeatRole("animateur", 12)...), repeatRole("public", 12)...)
	if _, err := ValidateLightingGroups(all, tw225Lights()); err != nil {
		t.Fatalf("12 groupes dans chacune des trois zones (36 au total) acceptés : %v", err)
	}
	for _, role := range []string{"salle", "animateur", "public"} {
		over := append(append([]LightingGroupEntry{}, all...), LightingGroupEntry{Role: role})
		if _, err := ValidateLightingGroups(over, tw225Lights()); err == nil {
			t.Errorf("un 13e groupe de la zone %s est refusé", role)
		}
	}
}

func TestTW225_Validate_PublicRoleAccepted(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{{Role: "public", Lights: []string{"L1"}}}, tw225Lights())
	if err != nil || len(out) != 1 || out[0].Role != "public" {
		t.Fatalf("rôle public valide : %+v %v", out, err)
	}
}

func TestTW225_Validate_AssignsWellFormedUniqueIDs_KeepsGivenOnes(t *testing.T) {
	out, err := ValidateLightingGroups([]LightingGroupEntry{
		{ID: "keep1"}, {}, {}, {Role: "animateur"}, {Role: "animateur"}, {Role: "public"},
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

func tw225ThreeZones() LightingConfig {
	return LightingConfig{Groups: []LightingGroupEntry{
		{ID: "a1", Role: "animateur", Lights: []string{}},
		{ID: "s1", Role: "salle", Lights: []string{"L1"}},
		{ID: "p1", Role: "public", Lights: []string{"P"}},
		{ID: "a2", Role: "animateur", Lights: []string{"H"}},
		{ID: "s2", Lights: []string{}},
		{ID: "s3", Lights: []string{"L2"}},
	}}
}

func TestTW225_ZoneGroups_OwnRankPerZone_EmptyOnesKeepTheirNumber(t *testing.T) {
	l := tw225ThreeZones()
	host := l.ZoneGroups("animateur")
	if len(host) != 2 || host[0].ID != "a1" || host[0].Num != 1 || host[1].ID != "a2" || host[1].Num != 2 {
		t.Errorf("rang propre à la zone animateur, vide compris : %+v", host)
	}
	room := l.ZoneGroups("ambiance")
	if len(room) != 3 || room[2].ID != "s3" || room[2].Num != 3 {
		t.Errorf("rang propre à la zone ambiance (rôle salle), vide compris : %+v", room)
	}
	pub := l.ZoneGroups("public")
	if len(pub) != 1 || pub[0].Num != 1 {
		t.Errorf("zone public : %+v", pub)
	}
	if len((LightingConfig{}).ZoneGroups("animateur")) != 0 {
		t.Error("sans groupe, liste vide")
	}
}

func TestTW225_NonEmptyGroups_RealNumbers_PerZone(t *testing.T) {
	l := tw225ThreeZones()
	room := l.NonEmptyGroups("ambiance")
	if len(room) != 2 || room[0].Num != 1 || room[1].Num != 3 {
		t.Errorf("N = groupes non vides de la zone, avec leur numéro réel (1 et 3) : %+v", room)
	}
	host := l.NonEmptyGroups("animateur")
	if len(host) != 1 || host[0].Num != 2 {
		t.Errorf("le groupe animateur vide est écarté, le suivant garde le numéro 2 : %+v", host)
	}
}

func TestTW225_ZoneExists_NeedsANonEmptyGroup_AmbianceAlways(t *testing.T) {
	var none LightingConfig
	if !none.ZoneExists("ambiance") {
		t.Error("la zone ambiance existe toujours (ampoules générales hors groupe)")
	}
	if none.ZoneExists("animateur") || none.ZoneExists("public") {
		t.Error("sans groupe, ni animateur ni public n'existent")
	}
	onlyEmpty := LightingConfig{Groups: []LightingGroupEntry{{Role: "public", Lights: []string{}}}}
	if onlyEmpty.ZoneExists("public") {
		t.Error("un groupe vide ne fait pas exister la zone")
	}
	l := tw225ThreeZones()
	if !l.ZoneExists("animateur") || !l.ZoneExists("public") {
		t.Error("une zone avec un groupe non vide existe")
	}
}

func TestTW225_GroupOfLight_ZoneAndNumber(t *testing.T) {
	got := tw225ThreeZones().GroupOfLight()
	if r := got["L2"]; r.Zone != "ambiance" || r.Num != 3 {
		t.Errorf("L2 : %+v", r)
	}
	if r := got["H"]; r.Zone != "animateur" || r.Num != 2 {
		t.Errorf("H : %+v", r)
	}
	if r := got["P"]; r.Zone != "public" || r.Num != 1 {
		t.Errorf("P : %+v", r)
	}
	if _, ok := got["Z"]; ok {
		t.Error("une ampoule hors groupe est absente (zone ambiance, groupe 0)")
	}
}

func TestTW225_ZoneOfRole(t *testing.T) {
	for role, want := range map[string]string{"": "ambiance", "salle": "ambiance", "animateur": "animateur", "public": "public"} {
		if got := ZoneOfRole(role); got != want {
			t.Errorf("ZoneOfRole(%q) = %q, attendu %q", role, got, want)
		}
	}
}

func TestTW225_OldConfigWithSingleHostGroupWithoutID_StillValidAndGetsAnID_HostEffectsIgnored(t *testing.T) {
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
	if cfg.Lighting.Effects["animateur"]["score"] != "" || cfg.Lighting.Effects["ambiance"]["score"] != "" {
		t.Errorf("l'ancien `host_effects` est ignoré (jamais repris dans une zone) : %v", cfg.Lighting.Effects)
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

// ── Réglages par zone (contrats §6) : formes complètes, défauts, lecture tolérante ──

func TestTW225_ApplyDefaults_EffectsAndSpeeds_ThreeZonesByThreeMoments(t *testing.T) {
	var cfg Config
	ApplyDefaults(&cfg)
	for _, z := range []string{"ambiance", "animateur", "public"} {
		for _, m := range []string{"chrono", "calm", "score"} {
			id, ok := cfg.Lighting.Effects[z][m]
			if !ok {
				t.Errorf("effects.%s.%s absent : la forme est toujours complète", z, m)
			}
			want := ""
			if z == "ambiance" && m == "chrono" {
				want = "pulsation"
			}
			if id != want {
				t.Errorf("effects.%s.%s = %q, attendu %q", z, m, id, want)
			}
			if sp, ok := cfg.Lighting.Speeds[z][m]; !ok || sp != 1 {
				t.Errorf("speeds.%s.%s = %v (%v), attendu 1", z, m, sp, ok)
			}
		}
	}
}

func TestTW225_OldFlatEffectsAndSpeeds_ReadAsAmbianceZone_HostKeysIgnored(t *testing.T) {
	var cfg Config
	raw := `{"lighting":{"effects":{"chrono":"x","calm":"y","score":""},"speeds":{"chrono":2,"calm":0.5,"score":1},` +
		`"host_effects":{"g1":{"calm":"z"}},"host_speeds":{"g1":{"calm":2}}}}`
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("l'ancienne forme plate se lit sans erreur : %v", err)
	}
	ApplyDefaults(&cfg)
	if cfg.Lighting.Effects["ambiance"]["chrono"] != "x" || cfg.Lighting.Effects["ambiance"]["calm"] != "y" {
		t.Errorf("valeurs plates ⇒ zone ambiance : %v", cfg.Lighting.Effects)
	}
	if cfg.Lighting.Speeds["ambiance"]["chrono"] != 2 || cfg.Lighting.Speeds["ambiance"]["calm"] != 0.5 {
		t.Errorf("vitesses plates ⇒ zone ambiance : %v", cfg.Lighting.Speeds)
	}
	if cfg.Lighting.Effects["animateur"]["calm"] != "" || cfg.Lighting.Speeds["animateur"]["calm"] != 1 {
		t.Errorf("host_effects / host_speeds ignorés : %v %v", cfg.Lighting.Effects, cfg.Lighting.Speeds)
	}
}

func TestTW225_NewEffectsAndSpeeds_ByZone_RoundTrip(t *testing.T) {
	var cfg Config
	raw := `{"lighting":{"effects":{"animateur":{"score":"couleur-equipe"},"public":{"calm":"chenillard"}},` +
		`"speeds":{"public":{"calm":1.5},"zone-inconnue":{"calm":2}}}}`
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(&cfg)
	if cfg.Lighting.Effects["animateur"]["score"] != "couleur-equipe" || cfg.Lighting.Effects["public"]["calm"] != "chenillard" {
		t.Errorf("effets par zone conservés : %v", cfg.Lighting.Effects)
	}
	if cfg.Lighting.Effects["ambiance"]["chrono"] != "pulsation" || cfg.Lighting.Effects["animateur"]["chrono"] != "" {
		t.Errorf("le reste reçoit les défauts : %v", cfg.Lighting.Effects)
	}
	if cfg.Lighting.Speeds["public"]["calm"] != 1.5 {
		t.Errorf("vitesse de la zone public : %v", cfg.Lighting.Speeds)
	}
	if _, ok := cfg.Lighting.Speeds["zone-inconnue"]; ok {
		t.Error("une zone inconnue est écartée")
	}
}
