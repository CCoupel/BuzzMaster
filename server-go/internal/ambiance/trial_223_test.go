package ambiance

// Essai d'un effet sur UNE zone (#223, contrat light-effects.md §7.4 révision 4) : StartEffectTrial(id, zone, vitesse, secondes)
// (durée en ms) et StopEffectTrial, refus typés (*TrialError : disabled | game | not_applicable | mode), effet inconnu
// (ErrTrialUnknownEffect), un seul essai à la fois, remplace un essai de groupe, ne touche jamais les autres zones, rend la
// zone à son état vivant à la fin. Horloge injectée : aucune attente (la goroutine de réveil n'est qu'un tic d'écriture).

import (
	"errors"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

type tw223TrialRefusal interface{ TrialReason() string }

func tw223TrialEnv(t *testing.T, groups ...config.LightingGroupEntry) *tw223Env {
	t.Helper()
	env := tw223New(t)
	next := *config.Get()
	next.Lighting.Enabled = true
	next.Lighting.Groups = groups
	config.SetInstance(&next)
	t.Cleanup(env.c.StopEffectTrial)
	return env
}

func tw223TrialReason(t *testing.T, err error) string {
	t.Helper()
	var r tw223TrialRefusal
	if !errors.As(err, &r) {
		t.Fatalf("un refus typé (TrialReason) était attendu : %v", err)
	}
	return r.TrialReason()
}

func TestTrial223_UnknownEffect_ErrTrialUnknownEffect(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"))
	if _, err := env.c.StartEffectTrial("n-existe-pas", "ambiance", 1, 10); !errors.Is(err, ErrTrialUnknownEffect) {
		t.Errorf("effet inconnu : %v", err)
	}
}

func TestTrial223_RefusedWhenLightingDisabled(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"))
	next := *config.Get()
	next.Lighting.Enabled = false
	config.SetInstance(&next)
	_, err := env.c.StartEffectTrial("pulsation", "ambiance", 1, 10)
	if err == nil || tw223TrialReason(t, err) != "disabled" {
		t.Errorf("éclairage désactivé ⇒ disabled : %v", err)
	}
}

func TestTrial223_RefusedDuringAQuestion_Game(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"))
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	_, err := env.c.StartEffectTrial("pulsation", "ambiance", 1, 10)
	if err == nil || tw223TrialReason(t, err) != "game" {
		t.Errorf("pendant une question ⇒ game : %v", err)
	}
	if env.c.TrialRunning() {
		t.Error("aucun essai démarré")
	}
}

func TestTrial223_NotApplicable_ZoneWithoutGroup_AndTooFewGroups(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"))
	for _, zone := range []string{"animateur", "public"} {
		_, err := env.c.StartEffectTrial("pulsation", zone, 1, 10)
		if err == nil || tw223TrialReason(t, err) != "not_applicable" {
			t.Errorf("zone %s sans groupe non vide ⇒ not_applicable : %v", zone, err)
		}
	}
	// chenillard : groupes_min 4 ; la zone ambiance n'a que 1 groupe non vide ici
	_, err := env.c.StartEffectTrial("chenillard", "ambiance", 1, 10)
	if err == nil || tw223TrialReason(t, err) != "not_applicable" {
		t.Errorf("moins de groupes que groupes_min ⇒ not_applicable : %v", err)
	}
}

func TestTrial223_Mode_RefusedOnAmbianceOnly(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"), hostGrp("H"))
	if err := env.c.SetLightingMode("OFF"); err != nil {
		t.Fatal(err)
	}
	_, err := env.c.StartEffectTrial("pulsation", "ambiance", 1, 10)
	if err == nil || tw223TrialReason(t, err) != "mode" {
		t.Errorf("zone ambiance en mode OFF ⇒ mode : %v", err)
	}
	if _, err := env.c.StartEffectTrial("pulsation", "animateur", 1, 10); err != nil {
		t.Errorf("la zone animateur n'est pas soumise au mode : %v", err)
	}
}

func TestTrial223_AnimateurZone_PlaysThenReturnsToLive_OtherZonesUntouched(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"), hostGrp("H"), pubGrp("P"))
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "all", 1, "#FF0000"))
	start := env.now
	ms, err := env.c.StartEffectTrial("calm-fx", "animateur", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ms != 10_000 {
		t.Errorf("durée annoncée en ms : %d", ms)
	}
	if !env.c.TrialRunning() {
		t.Error("essai en cours")
	}
	env.now = start.Add(3 * time.Second)
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	z, ok := hostEntry(st)
	if !ok || z.Color != [3]int{255, 0, 0} {
		t.Errorf("la zone animateur joue l'effet pendant l'essai : %+v (%v)", z, ok)
	}
	if es := areaEntries(st, lighting.AreaPublic); len(es) != 0 {
		t.Errorf("la zone public n'est pas touchée : %+v", es)
	}
	if st.Zones[0].Area != lighting.AreaAmbiance || st.Zones[0].Group != 0 {
		t.Errorf("la zone ambiance garde sa propre entrée : %+v", st.Zones[0])
	}
	env.now = start.Add(11 * time.Second)
	if env.c.TrialRunning() {
		t.Error("l'essai est terminé après sa durée")
	}
	z, ok = hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
		t.Errorf("retour à l'état vivant de la zone animateur (blanc fixe) : %+v (%v)", z, ok)
	}
}

func TestTrial223_SpeedFollowsTheChosenMultiplier(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"), hostGrp("H"))
	env.writeUser(t, "clignote.yaml", "id: clignote\nnom: Clignote\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n")
	start := env.now
	if _, err := env.c.StartEffectTrial("clignote", "animateur", 2, 10); err != nil {
		t.Fatal(err)
	}
	red, green := [3]int{255, 0, 0}, [3]int{0, 255, 0}
	at := func(ms int) [3]int {
		env.now = start.Add(time.Duration(ms) * time.Millisecond)
		z, _ := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
		return z.Color
	}
	if c := at(0); c != red {
		t.Errorf("pas 0 : %v", c)
	}
	if c := at(600); c != green {
		t.Errorf("×2 : tick de 500 ms, pas 1 à 0,6 s : %v", c)
	}
	if c := at(1100); c != red {
		t.Errorf("×2 : tick 2 à 1,1 s : %v", c)
	}
}

func TestTrial223_AmbianceZone_UsesRealGroupNumbers_AndStopReturnsToLive(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1), grp(2, "C"))
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#00FF00"))
	if _, err := env.c.StartEffectTrial("calm-fx", "ambiance", 1, 10); err != nil {
		t.Fatal(err)
	}
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if len(g) != 2 || g[0].Group != 1 || g[1].Group != 3 || g[0].Color != [3]int{0, 255, 0} {
		t.Errorf("le groupe vide 2 garde son numéro : l'effet vise les groupes 1 et 3 : %+v", g)
	}
	env.c.StopEffectTrial()
	if env.c.TrialRunning() {
		t.Error("arrêt immédiat")
	}
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("après l'arrêt, plus d'entrée d'essai : %+v", g)
	}
	env.c.StopEffectTrial() // sans effet quand rien ne tourne
}

func TestTrial223_OnlyOneAtATime_ReplacesTheRunningTrialAndAGroupTest(t *testing.T) {
	env := tw223TrialEnv(t, grp(0, "A"), grp(1, "B"), hostGrp("H"), pubGrp("P"))
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "all", 1, "#FF0000"))
	if _, err := env.c.StartEffectTrial("calm-fx", "animateur", 1, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := env.c.StartEffectTrial("calm-fx", "public", 1, 10); err != nil {
		t.Fatal(err)
	}
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if p, ok := areaEntry(st, lighting.AreaPublic, 0); !ok || p.Color != [3]int{255, 0, 0} {
		t.Errorf("le second essai (zone public) joue : %+v (%v)", p, ok)
	}
	if h, _ := hostEntry(st); h.Color != [3]int{255, 255, 255} {
		t.Errorf("le premier essai est remplacé : la zone animateur redevient blanc fixe : %+v", h)
	}
	// un essai de groupe lancé ensuite remplace l'essai d'effet
	tw225SetStep(env.c, tw225Step)
	if err := tw225StartTest(env.c, "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(env.c.StopGroupTest)
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaAmbiance, 1); !ok {
		t.Fatal("l'essai de groupe est en cours")
	}
	if _, err := env.c.StartEffectTrial("calm-fx", "animateur", 1, 10); err != nil {
		t.Fatal(err)
	}
	if _, ok := tw225Entry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaAmbiance, 1); ok {
		t.Error("l'essai d'effet remplace l'essai de groupe (un seul « Tester » à la fois)")
	}
}
