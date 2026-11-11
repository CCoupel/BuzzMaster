package ambiance

// #225 (v11.7.0, LOT-1, révision du GATE LOT-1) — essai d'un groupe en séquence visible.
// Source : contracts/light-effects.md §2.4 : six pas de 1000 ms — éteint, blanc 50 %, blanc 100 %, rouge, vert, bleu —
// puis retour à l'état vivant ; aucune écriture directe au pilote (état lu par Scene()) ; un seul essai à la fois
// (un nouvel essai remplace le précédent) ; arrêt explicite ; l'essai prime sur effet / ON / OFF / Flash pour CE seul
// groupe ; quiesceAmbiance l'annule. Horloge injectée (EffectsClock) : aucune attente réelle. Non parallèles (config).
// Réutilise tw223New / grp / hostGrp / roomGroups de effects_223_test.go.

import (
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting"
)

const tw225Step = time.Second

type tw225Colour struct {
	color     [3]int
	intensity int
}

var tw225Sequence = []tw225Colour{
	{[3]int{0, 0, 0}, 0},         // 1 : éteint
	{[3]int{255, 255, 255}, 128}, // 2 : blanc 50 %
	{[3]int{255, 255, 255}, 255}, // 3 : blanc 100 %
	{[3]int{255, 0, 0}, 255},     // 4 : rouge
	{[3]int{0, 255, 0}, 255},     // 5 : vert
	{[3]int{0, 0, 255}, 255},     // 6 : bleu
}

func tw225Near(a, b int) bool { d := a - b; return d >= -1 && d <= 1 }

func tw225Entry(st lighting.State, group int) (lighting.ZoneState, bool) {
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Group == group {
			return z, true
		}
	}
	return lighting.ZoneState{}, false
}

func tw225TestEnv(t *testing.T, groups ...config.LightingGroupEntry) *tw223Env {
	t.Helper()
	env := tw223New(t)
	env.setup(groups)
	tw225SetStep(env.c, tw225Step)
	t.Cleanup(env.c.StopGroupTest) // la goroutine de pas ne survit pas au test
	return env
}

func tw225Mid(env *tw223Env, start time.Time, step int) {
	env.now = start.Add(time.Duration(step)*tw225Step + 100*time.Millisecond) // milieu de pas (+100 ms)
}

func TestGroupTest225_RoomGroup_SixStepsInOrder_ThenLive(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"), grp(2, "C"))
	start := env.now
	if err := tw225StartTest(env.c, "2"); err != nil {
		t.Fatalf("essai du groupe 2 : %v", err)
	}
	for i, want := range tw225Sequence {
		tw225Mid(env, start, i)
		st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
		z, ok := tw225Entry(st, 2)
		if !ok {
			t.Fatalf("pas %d : une entrée {general, Group 2} est émise par Scene()", i+1)
		}
		if z.Color != want.color || !tw225Near(z.Intensity, want.intensity) {
			t.Errorf("pas %d : %v @%d, attendu %v @%d", i+1, z.Color, z.Intensity, want.color, want.intensity)
		}
		if z.TransitionMs != 0 {
			t.Errorf("pas %d : fondu 0, obtenu %d", i+1, z.TransitionMs)
		}
		// les autres groupes ne sont pas touchés
		for _, g := range []int{1, 3} {
			if _, has := tw225Entry(st, g); has {
				t.Errorf("pas %d : le groupe %d n'est pas concerné par l'essai", i+1, g)
			}
		}
	}
	// après 6 pas : retour à l'état vivant (plus d'entrée d'essai)
	env.now = start.Add(6*tw225Step + 100*time.Millisecond)
	if z, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 2); ok {
		t.Errorf("fin de séquence : retour à l'état vivant, plus d'entrée Group 2 : %+v", z)
	}
}

func TestGroupTest225_HostGroup_SequenceOnItsOwnAddress_ThenFixedWhite(t *testing.T) {
	env := tw225TestEnv(t, hostGrp(), grp(0, "A"))
	start := env.now
	if err := tw225StartTest(env.c, "host1"); err != nil {
		t.Fatalf("essai du groupe animateur : %v", err)
	}
	for i, want := range tw225Sequence {
		tw225Mid(env, start, i)
		z, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.HostGroup(1))
		if !ok || z.Color != want.color || !tw225Near(z.Intensity, want.intensity) {
			t.Errorf("pas %d : %+v (%v), attendu %v @%d", i+1, z, ok, want.color, want.intensity)
		}
	}
	env.now = start.Add(6*tw225Step + 100*time.Millisecond)
	z, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.HostGroup(1))
	if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
		t.Errorf("retour à l'état vivant de l'animateur = blanc fixe : %+v (%v)", z, ok)
	}
}

func TestGroupTest225_SecondHostGroup_UsesAddressMinusTwo(t *testing.T) {
	env := tw225TestEnv(t, hostGrp(), config.LightingGroupEntry{Role: config.GroupRoleAnimateur, Lights: []string{"R"}})
	start := env.now
	if err := tw225StartTest(env.c, "host2"); err != nil {
		t.Fatal(err)
	}
	tw225Mid(env, start, 3) // pas 4 : rouge
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if z, ok := tw225Entry(st, lighting.HostGroup(2)); !ok || z.Color != [3]int{255, 0, 0} {
		t.Errorf("host2 ⇒ Group -2, rouge au pas 4 : %+v (%v)", z, ok)
	}
	if z, _ := tw225Entry(st, lighting.HostGroup(1)); z.Color != [3]int{255, 255, 255} {
		t.Errorf("host1 non concerné, reste blanc fixe : %+v", z)
	}
}

func TestGroupTest225_NewTestReplacesTheRunningOne_PreviousGroupBackToLive(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"), grp(2, "C"))
	start := env.now
	_ = tw225StartTest(env.c, "1")
	tw225Mid(env, start, 1)
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 1); !ok {
		t.Fatal("essai du groupe 1 en cours")
	}
	// remplacement par le groupe 3 : le groupe 1 revient aussitôt à son état vivant, la séquence repart du pas 1
	restart := env.now
	if err := tw225StartTest(env.c, "3"); err != nil {
		t.Fatal(err)
	}
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if _, ok := tw225Entry(st, 1); ok {
		t.Error("un nouvel essai remplace le précédent : le groupe 1 revient aussitôt à l'état vivant")
	}
	z, ok := tw225Entry(st, 3)
	if !ok || z.Intensity != 0 {
		t.Errorf("la séquence du groupe 3 repart du pas 1 (éteint) : %+v (%v)", z, ok)
	}
	env.now = restart.Add(2*tw225Step + 100*time.Millisecond) // pas 3 : blanc 100 %
	if z, _ := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 3); z.Intensity != 255 || z.Color != [3]int{255, 255, 255} {
		t.Errorf("pas 3 du nouvel essai : %+v", z)
	}
}

func TestGroupTest225_Stop_GroupBackToLiveImmediately_StopWithoutTestIsHarmless(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"))
	tw225StopTest(env.c) // aucun essai : sans effet, sans panique
	start := env.now
	_ = tw225StartTest(env.c, "2")
	tw225Mid(env, start, 2)
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 2); !ok {
		t.Fatal("essai en cours")
	}
	tw225StopTest(env.c)
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 2); ok {
		t.Error("arrêt : le groupe revient aussitôt à son état vivant")
	}
	tw225StopTest(env.c)
}

func TestGroupTest225_UnknownOrEmptyGroup_Refused_NothingStarted(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1), hostGrp(), config.LightingGroupEntry{Role: config.GroupRoleAnimateur, Lights: []string{}})
	for _, ref := range []string{"9", "0", "2", "host3", "host2", "host", "salle", ""} {
		if err := tw225StartTest(env.c, ref); err == nil {
			t.Errorf("groupe %q inconnu ou vide : refus attendu", ref)
		}
	}
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 0 {
		t.Error("un refus ne démarre rien")
	}
}

func TestGroupTest225_PrimesOverRoomEffect_ForThatGroupOnly(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"), grp(2, "C"))
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step()
	start := env.now
	_ = tw225StartTest(env.c, "2")
	tw225Mid(env, start, 4) // pas 5 : vert
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	z2, _ := tw225Entry(st, 2)
	if z2.Color != [3]int{0, 255, 0} {
		t.Errorf("l'essai prime sur l'effet pour le groupe 2 : %+v", z2)
	}
	for _, g := range []int{1, 3} {
		z, ok := tw225Entry(st, g)
		if !ok || z.Color != [3]int{255, 0, 0} {
			t.Errorf("les autres groupes gardent l'effet de salle (rouge) : groupe %d = %+v (%v)", g, z, ok)
		}
	}
}

func TestGroupTest225_PrimesOverOffAndFlash_ForThatGroupOnly(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"))
	start := env.now
	_ = tw225StartTest(env.c, "1")
	tw225Mid(env, start, 3) // pas 4 : rouge
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
		if z, ok := tw225Entry(st, 1); !ok || z.Color != [3]int{255, 0, 0} {
			t.Errorf("mode %s : la séquence reste visible sur le groupe testé : %+v (%v)", mode, z, ok)
		}
		if _, ok := tw225Entry(st, 2); ok {
			t.Errorf("mode %s : l'autre groupe n'est pas concerné", mode)
		}
	}
	_ = env.c.SetLightingMode("AUTO")
	env.c.SetLightingFlash(true)
	t.Cleanup(func() { env.c.SetLightingFlash(false) })
	if z, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 1); !ok || z.Color != [3]int{255, 0, 0} {
		t.Errorf("Flash : la séquence reste visible : %+v (%v)", z, ok)
	}
}

func TestGroupTest225_QuiesceCancelsTheTest(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), grp(1, "B"))
	start := env.now
	_ = tw225StartTest(env.c, "1")
	tw225Mid(env, start, 2)
	env.c.quiesceAmbiance()
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), 1); ok {
		t.Error("l'arrêt du serveur / la désactivation annule l'essai")
	}
}

func TestGroupTest225_DoesNotChangeStateWithoutTest_NoRegression(t *testing.T) {
	env := tw225TestEnv(t, grp(0, "A"), hostGrp())
	for _, k := range tw223Kinds {
		st := env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}, Points: 5})
		if g := roomGroups(st); len(g) != 0 {
			t.Errorf("%s : sans essai, aucune entrée de groupe de salle : %+v", k, g)
		}
		if z, ok := tw225Entry(st, lighting.HostGroup(1)); !ok || z.Color != [3]int{255, 255, 255} {
			t.Errorf("%s : l'animateur reste en blanc fixe : %+v", k, z)
		}
	}
}
