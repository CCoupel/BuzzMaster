package ambiance

// #225 (révision du GATE LOT-1) — plusieurs groupes animateur dans Scene() : un blanc fixe par groupe non vide, sur
// Group = HostGroup(k), jamais entraîné par la salle, ON / OFF, Flash. Source : contracts/light-effects.md §2.2, §2.5.
// Réutilise tw223New et les aides de effects_223_test.go.

import (
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting"
)

func tw225Host(name string, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{Name: name, Role: config.GroupRoleAnimateur, Lights: lights, NearTeams: []string{}}
}

func tw225HostEntries(st lighting.State) map[int]lighting.ZoneState {
	out := map[int]lighting.ZoneState{}
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Group < 0 {
			out[z.Group] = z
		}
	}
	return out
}

func TestHosts225_EachNonEmptyHostGetsFixedWhiteOnItsOwnAddress(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw225Host("P", "H1"), grp(0, "A"), tw225Host("Vide"), tw225Host("R", "H2"), tw225Host("S", "H3")})
	for _, k := range tw223Kinds {
		got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}}))
		for _, rank := range []int{1, 3, 4} { // rang 2 = groupe animateur vide : il garde son rang, aucune entrée
			z, ok := got[lighting.HostGroup(rank)]
			if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
				t.Errorf("%s : animateur de rang %d en blanc fixe : %+v (%v)", k, rank, z, ok)
			}
		}
		if _, ok := got[lighting.HostGroup(2)]; ok {
			t.Errorf("%s : un groupe animateur vide est ignoré (aucune entrée)", k)
		}
	}
}

func TestHosts225_NoHostGroupNoEntry(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A")})
	if got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(got) != 0 {
		t.Errorf("aucune entrée animateur : %+v", got)
	}
}

func TestHosts225_NoneTouchedByOnOffFlash(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw225Host("P", "H1"), tw225Host("R", "H2"), grp(0, "A")})
	check := func(label string) {
		t.Helper()
		got := tw225HostEntries(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}))
		for _, rank := range []int{1, 2} {
			if z := got[lighting.HostGroup(rank)]; z.Intensity != 255 || z.Color != [3]int{255, 255, 255} {
				t.Errorf("%s : l'animateur de rang %d reste en blanc fixe : %+v", label, rank, z)
			}
		}
	}
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		check("mode " + mode)
	}
	_ = env.c.SetLightingMode("AUTO")
	env.c.SetLightingFlash(true)
	t.Cleanup(func() { env.c.SetLightingFlash(false) })
	check("Flash")
}

func TestHosts225_RoomGroupNumbersNeverCountHosts(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{tw225Host("P", "H1"), grp(0, "A"), tw225Host("R", "H2"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step()
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if len(g) != 2 || g[0].Group != 1 || g[1].Group != 2 {
		t.Errorf("groupes de salle numérotés 1 et 2, animateurs exclus : %+v", g)
	}
}
