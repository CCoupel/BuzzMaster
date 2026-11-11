package config

import "testing"

func TestZones_RanksPerZoneEmptyGroupsKeepTheirNumber_Dev(t *testing.T) {
	t.Parallel()
	l := LightingConfig{Groups: []LightingGroupEntry{
		{ID: "a", Role: "salle", Lights: []string{"L1"}},
		{ID: "p", Role: "public", Lights: []string{}},
		{ID: "b", Role: "", Lights: []string{}},
		{ID: "h", Role: "animateur", Lights: []string{"H1"}},
		{ID: "c", Role: "salle", Lights: []string{"L2"}},
		{ID: "q", Role: "public", Lights: []string{"P1"}},
	}}
	room := l.ZoneGroups(ZoneAmbiance)
	if len(room) != 3 || room[2].Num != 3 {
		t.Fatalf("rang par zone, vides compris : %+v", room)
	}
	ne := l.NonEmptyGroups(ZoneAmbiance)
	if len(ne) != 2 || ne[0].Num != 1 || ne[1].Num != 3 {
		t.Fatalf("N = groupes non vides, numéros réels : %+v", ne)
	}
	if g := l.NonEmptyGroups(ZonePublic); len(g) != 1 || g[0].Num != 2 {
		t.Fatalf("zone public : le groupe vide garde le rang 1 : %+v", g)
	}
	refs := l.GroupOfLight()
	if refs["H1"] != (LightRef{Zone: ZoneAnimateur, Num: 1}) || refs["P1"] != (LightRef{Zone: ZonePublic, Num: 2}) || refs["L2"] != (LightRef{Zone: ZoneAmbiance, Num: 3}) {
		t.Fatalf("zone et numéro de chaque ampoule : %+v", refs)
	}
	if !l.ZoneExists(ZoneAmbiance) || !l.ZoneExists(ZoneAnimateur) || !l.ZoneExists(ZonePublic) {
		t.Fatal("zones existantes")
	}
	if (LightingConfig{Groups: []LightingGroupEntry{{Role: "public", Lights: []string{}}}}).ZoneExists(ZonePublic) {
		t.Fatal("une zone sans groupe non vide n'existe pas")
	}
	if !(LightingConfig{}).ZoneExists(ZoneAmbiance) {
		t.Fatal("la salle existe toujours")
	}
}

func TestLoad_LegacyQualifEffectsTablesAndHostKeys_Dev(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	if err := cfg.Lighting.Effects.UnmarshalJSON([]byte(`{"chrono":"pulsation","calm":"x"}`)); err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(cfg)
	if cfg.Lighting.Effects[ZoneAmbiance]["calm"] != "x" || cfg.Lighting.Effects[ZonePublic]["calm"] != "" || cfg.Lighting.Effects[ZoneAmbiance]["chrono"] != "pulsation" {
		t.Fatalf("à plat = ambiance, tables complètes : %+v", cfg.Lighting.Effects)
	}
	if cfg.Lighting.Speeds[ZoneAnimateur]["score"] != 1 {
		t.Fatalf("vitesses ×1 par défaut : %+v", cfg.Lighting.Speeds)
	}
}
