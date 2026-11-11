package ambiance

// #223 (révision du GATE LOT-1) — une trame indépendante par groupe animateur, écrite sur Group = -k, réglée par
// `light.host_effects.<id du groupe>` (§5.3, §6). Réutilise tw223New / env.setHost de effects_223_test.go.

import (
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting"
)

func tw223Host2(id, name string, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: id, Name: name, Role: config.GroupRoleAnimateur, Lights: lights, NearTeams: []string{}}
}

func TestHostsEffects223_EachHostHasItsOwnEffect_OnItsOwnAddress(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), grp(0, "A"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.writeUser(t, "bleu.yaml", tw223Effect("bleu", "calm", "all", 1, "#0000FF"))
	env.setHost("h1", "calm", "vert")
	env.setHost("h2", "calm", "bleu")
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if z := got[lighting.HostGroup(1)]; z.Color != [3]int{0, 255, 0} {
		t.Errorf("A1 (id h1) joue « vert » sur Group -1 : %+v", z)
	}
	if z := got[lighting.HostGroup(2)]; z.Color != [3]int{0, 0, 255} {
		t.Errorf("A2 (id h2) joue « bleu » sur Group -2 : %+v", z)
	}
}

func TestHostsEffects223_OneHostWithEffect_OtherStaysFixedWhite(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.setHost("h2", "calm", "vert")
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if z := got[lighting.HostGroup(1)]; z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
		t.Errorf("A1 sans réglage : blanc fixe : %+v", z)
	}
	if z := got[lighting.HostGroup(2)]; z.Color != [3]int{0, 255, 0} {
		t.Errorf("A2 : effet « vert » : %+v", z)
	}
}

func TestHostsEffects223_HostWithoutEntryInTable_FixedWhiteAtEveryMoment(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("nouveau", "Pupitre", "P")}) // aucun réglage pour cet id
	env.step()
	for _, k := range tw223Kinds {
		z, ok := tw225HostEntries(env.c.Scene(lighting.Event{Kind: k}))[lighting.HostGroup(1)]
		if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
			t.Errorf("%s : groupe animateur sans entrée de réglage ⇒ blanc fixe : %+v (%v)", k, z, ok)
		}
	}
}

func TestHostsEffects223_UnknownIdInSettings_IgnoredNoError(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.setHost("disparu", "calm", "vert") // groupe supprimé : sa clé survit dans les réglages
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if len(got) != 1 {
		t.Fatalf("une seule entrée animateur (h1) : %+v", got)
	}
	if z := got[lighting.HostGroup(1)]; z.Color != [3]int{255, 255, 255} {
		t.Errorf("un id inconnu est ignoré à la résolution : %+v", z)
	}
}

func TestHostsEffects223_SettingFollowsTheGroupIdAfterReorder(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "vert")
	env.step()
	if z := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))[lighting.HostGroup(1)]; z.Color != [3]int{0, 255, 0} {
		t.Fatalf("avant : A1 = h1 = vert : %+v", z)
	}
	// l'utilisateur inverse les deux groupes : h1 devient A2, son réglage le suit
	env.setup([]config.LightingGroupEntry{tw223Host2("h2", "Régie", "R"), tw223Host2("h1", "Pupitre", "P")})
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if got[lighting.HostGroup(2)].Color != [3]int{0, 255, 0} || got[lighting.HostGroup(1)].Color != [3]int{255, 255, 255} {
		t.Errorf("la clé est l'id, pas le rang : h1 (maintenant A2) garde « vert » : %+v", got)
	}
}

func TestHostsEffects223_EmptyHostGroupIgnoredByReader_KeepsItsRank(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Vide"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "vert")
	env.setHost("h2", "calm", "vert")
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if _, ok := got[lighting.HostGroup(1)]; ok {
		t.Error("groupe animateur vide : aucune trame")
	}
	if got[lighting.HostGroup(2)].Color != [3]int{0, 255, 0} {
		t.Errorf("A2 garde son rang 2 : %+v", got)
	}
}

func TestHostsEffects223_ScoreMoment_TeamColourOnEveryHostWithThatSetting(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), tw223Host2("h2", "Régie", "R")})
	env.setHost("h1", "score", "couleur-equipe")
	env.setHost("h2", "score", "couleur-equipe")
	env.step()
	env.c.StartScoreFlash("Rouges", 5)
	t.Cleanup(func() { env.c.StartScoreFlash("", 0) })
	env.step()
	want := (fakeHost{}).TeamNameColor("Rouges")
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	for rank := 1; rank <= 2; rank++ {
		if got[lighting.HostGroup(rank)].Color != want {
			t.Errorf("A%d : couleur de l'équipe : %+v, attendu %v", rank, got[lighting.HostGroup(rank)], want)
		}
	}
}

func TestHostsEffects223_NoneAffectedByOnOffFlash_WhileEachKeepsItsEffect(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.writeUser(t, "bleu.yaml", tw223Effect("bleu", "calm", "all", 1, "#0000FF"))
	env.setHost("h1", "calm", "vert")
	env.setHost("h2", "calm", "bleu")
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		env.step()
		got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
		if got[lighting.HostGroup(1)].Color != [3]int{0, 255, 0} || got[lighting.HostGroup(2)].Color != [3]int{0, 0, 255} {
			t.Errorf("mode %s : chaque animateur garde son effet : %+v", mode, got)
		}
	}
}

func TestHostsEffects223_QuiesceStopsEveryHostFrame(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw223Host2("h1", "Pupitre", "P"), tw223Host2("h2", "Régie", "R")})
	env.writeUser(t, "vert.yaml", tw223Effect("vert", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "vert")
	env.setHost("h2", "calm", "vert")
	env.step()
	env.c.quiesceAmbiance()
	env.step()
	got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	for rank, z := range map[int]lighting.ZoneState{1: got[lighting.HostGroup(1)], 2: got[lighting.HostGroup(2)]} {
		if z.Color == [3]int{0, 255, 0} {
			t.Errorf("A%d : plus de trame d'effet après l'arrêt : %+v", rank, z)
		}
	}
}
