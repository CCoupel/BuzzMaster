package hue

// v11.7.0 LOT-1, révision 4 (#225) — trois zones (ambiance, animateur, public) dans le pilote Hue.
// Source : contracts/light-effects.md §2.2, §2.3, §2.5 (révision 4) ; hue-bridge.md. Une ampoule porte (Area, Group) ;
// groupes Hue `buzzmaster-general[-n]`, `buzzmaster-host[-n]`, `buzzmaster-public[-n]` (de zone dès 2 ampoules, de groupe
// dès 3), l'ancien `buzzmaster-general-host<k>` disparaît à la réconciliation, aucune écriture croisée entre zones,
// une zone sans entrée n'est jamais écrite. Aides de groupes_ambiance_225_test.go (tw225*).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"buzzcontrol/internal/lighting"
)

func tw225Groups(f *devBridge) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	by := map[string]string{}
	for _, g := range f.groups {
		by[g.name] = fmt.Sprint(twBSortedCopy(g.lights))
	}
	return by
}

func tw225Writes(f *devBridge, from int, id string) int {
	n := 0
	for _, p := range f.puts()[from:] {
		if strings.HasSuffix(p.path, "/lights/"+id+"/state") {
			n++
		}
	}
	return n
}

func TestTW225Zones_HueGroupsPerAreaAndRank_AreasNeverShareAGroup(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "S3", "H1a", "H1b", "H1c", "H2a", "H2b", "H2c", "P1a", "P1b", "P1c")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("S1", 1), tw225Spec("S2", 1), tw225Spec("S3", 1),
		tw225Host("H1a", 1), tw225Host("H1b", 1), tw225Host("H1c", 1),
		tw225Host("H2a", 2), tw225Host("H2b", 2), tw225Host("H2c", 2),
		tw225Pub("P1a", 1), tw225Pub("P1b", 1), tw225Pub("P1c", 1))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	by := tw225Groups(f)
	for name, want := range map[string]string{
		"buzzmaster-general": "[1 2 3]", "buzzmaster-general-1": "[1 2 3]",
		"buzzmaster-host": "[4 5 6 7 8 9]", "buzzmaster-host-1": "[4 5 6]", "buzzmaster-host-2": "[7 8 9]",
		"buzzmaster-public": "[10 11 12]", "buzzmaster-public-1": "[10 11 12]",
	} {
		if by[name] != want {
			t.Errorf("%s = %s, attendu %s", name, by[name], want)
		}
	}
}

func TestTW225Zones_OldHostGroupNameDeletedByReconciliation(t *testing.T) {
	f := newDevBridge(t, "S1", "H1a", "H1b", "H1c")
	f.groups["77"] = &devGroup{name: "buzzmaster-general-host", lights: []string{"2", "3", "4"}}  // QUALIF 11.7.0.2
	f.groups["78"] = &devGroup{name: "buzzmaster-general-host1", lights: []string{"2", "3", "4"}} // QUALIF 11.7.0.3
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("H1a", 1), tw225Host("H1b", 1), tw225Host("H1c", 1))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"buzzmaster-general-host", "buzzmaster-general-host1"} {
		if _, ok := tw225GroupIDByName(f, old); ok {
			t.Errorf("l'ancien groupe Hue %s est supprimé (préfixe buzzmaster-)", old)
		}
	}
	for _, n := range []string{"buzzmaster-host", "buzzmaster-host-1"} {
		if _, ok := tw225GroupIDByName(f, n); !ok {
			t.Errorf("%s créé", n)
		}
	}
}

func TestTW225Zones_GroupHueNotCreatedBelowThresholds(t *testing.T) {
	f := newDevBridge(t, "S1", "A1", "A2", "B1", "B2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("A1", 1), tw225Host("A2", 1), tw225Pub("B1", 1), tw225Pub("B2", 1))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"buzzmaster-host-1", "buzzmaster-public-1"} {
		if _, ok := tw225GroupIDByName(f, n); ok {
			t.Errorf("%s : 2 ampoules < SubGroupMinLights, pas de groupe Hue de groupe", n)
		}
	}
	for _, n := range []string{"buzzmaster-host", "buzzmaster-public"} {
		if _, ok := tw225GroupIDByName(f, n); !ok {
			t.Errorf("%s : groupe Hue de zone dès 2 ampoules", n)
		}
	}
}

func TestTW225Zones_AreaWithoutEntry_NeverWritten(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "H1", "P1")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Spec("S2", 0), tw225Host("H1", 1), tw225Pub("P1", 1))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200)}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"3", "4"} {
		if tw225Bri(f, id) != 1 {
			t.Errorf("l'ampoule %s (animateur / public) ne reçoit jamais l'entrée de l'ambiance (bri = %v)", id, tw225Bri(f, id))
		}
	}
	for _, id := range []string{"3", "4"} {
		if n := tw225Writes(f, 0, id); n != 0 {
			t.Errorf("ampoule %s : %d écriture(s), aucune attendue", id, n)
		}
	}
}

func TestTW225Zones_EachGroupWrittenOnlyByItsOwnEntry_NoCrossWrites(t *testing.T) {
	f := newDevBridge(t, "S1", "H1", "H2", "P1", "P2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("H1", 1), tw225Host("H2", 2), tw225Pub("P1", 1), tw225Pub("P2", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225White, 255),
		tw225ZoneA(lighting.AreaHost, 1, tw225Red, 255),
		tw225ZoneA(lighting.AreaHost, 2, tw225Blue, 255),
		tw225ZoneA(lighting.AreaPublic, 1, tw225Blue, 255),
		tw225ZoneA(lighting.AreaPublic, 2, tw225Red, 255),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	if !tw225IsRed(t, f, "2") || !tw225IsBlue(t, f, "3") || !tw225IsBlue(t, f, "4") || !tw225IsRed(t, f, "5") {
		t.Error("chaque groupe de chaque zone suit sa propre entrée (animateur 1 rouge, 2 bleu ; public 1 bleu, 2 rouge)")
	}
	// Le groupe 2 de la zone animateur change seul : rien d'autre n'est réécrit.
	before := len(f.puts())
	st.Zones[2] = tw225ZoneA(lighting.AreaHost, 2, tw225White, 255)
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "2", "4", "5"} {
		if n := tw225Writes(f, before, id); n != 0 {
			t.Errorf("ampoule %s inchangée : réécrite %d fois", id, n)
		}
	}
	if n := tw225Writes(f, before, "3"); n != 1 {
		t.Errorf("le groupe animateur 2 est écrit une fois : %d", n)
	}
}

func TestTW225Zones_SameGroupNumberInTwoAreas_AreIndependent(t *testing.T) {
	// groupe 1 de l'ambiance, de l'animateur et du public : trois entrées, jamais confondues
	f := newDevBridge(t, "S1", "S2", "H1", "H2", "P1", "P2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 1), tw225Spec("S2", 2), tw225Host("H1", 1), tw225Host("H2", 2), tw225Pub("P1", 1), tw225Pub("P2", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225White, 255), tw225Zone(1, tw225Red, 255), tw225Zone(2, tw225Blue, 255),
		tw225ZoneA(lighting.AreaHost, 1, tw225Blue, 255), tw225ZoneA(lighting.AreaHost, 2, tw225Red, 255),
		tw225ZoneA(lighting.AreaPublic, 1, tw225Red, 255), tw225ZoneA(lighting.AreaPublic, 2, tw225Blue, 255),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, false, true, true, false} // rouge ?
	for i, id := range []string{"1", "2", "3", "4", "5", "6"} {
		if tw225IsRed(t, f, id) != want[i] {
			t.Errorf("ampoule %s : couleur inattendue (la zone fait partie de la clé de l'entrée)", id)
		}
	}
}

func TestTW225Zones_ExplicitOffEntries_SwitchEachAreaOff_RoomOffLeavesOthersOn(t *testing.T) {
	f := newDevBridge(t, "S1", "H1", "P1")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("H1", 1), tw225Pub("P1", 1))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	on := lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 255),
		tw225ZoneA(lighting.AreaHost, 1, tw225White, 255), tw225ZoneA(lighting.AreaPublic, 1, tw225White, 255)}}
	if err := d.Apply(ctx, on); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 0)}}); err != nil {
		t.Fatal(err)
	}
	state := func(id string) any {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.lights[id]["state"].(map[string]any)["on"]
	}
	if state("1") != false || state("2") != true || state("3") != true {
		t.Error("OFF de l'ambiance éteint l'ambiance seule : animateur et public restent allumés")
	}
	off := lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 0),
		tw225ZoneA(lighting.AreaHost, 1, tw225White, 0), tw225ZoneA(lighting.AreaPublic, 1, tw225White, 0)}}
	if err := d.Apply(ctx, off); err != nil {
		t.Fatal(err)
	}
	if state("2") != false || state("3") != false {
		t.Error("les entrées explicites éteignent la zone animateur et la zone public")
	}
}

func TestTW225Zones_TwelveGroupsInAnAreaAllAddressable(t *testing.T) {
	names := make([]string, 12)
	specs := make([]LightSpec, 12)
	for i := range names {
		names[i] = fmt.Sprintf("P%d", i+1)
		specs[i] = tw225Pub(names[i], i+1)
	}
	f := newDevBridge(t, names...)
	d, _ := newDevGroupsEnabled(t, f, specs...)
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{}
	for k := 1; k <= 12; k++ {
		c := tw225Red
		if k%2 == 0 {
			c = tw225Blue
		}
		st.Zones = append(st.Zones, tw225ZoneA(lighting.AreaPublic, k, c, 255))
	}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	for k := 1; k <= 12; k++ {
		id := fmt.Sprint(k)
		if k%2 == 0 && !tw225IsBlue(t, f, id) || k%2 == 1 && !tw225IsRed(t, f, id) {
			t.Errorf("groupe public %d (ampoule %s) : mauvaise couleur", k, id)
		}
	}
}

func TestTW225Zones_SnapshotThenRestore_GivesTheBulbsTheirStateBack(t *testing.T) {
	// §2.5 : après un essai en zone Public sans effet, l'état des lampes lu sur le pont est réécrit
	f := newDevBridge(t, "P1", "P2")
	d, _ := newDevGroupsEnabled(t, f, tw225Pub("P1", 1), tw225Pub("P2", 1))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	for _, id := range []string{"1", "2"} { // l'état de départ : allumées, blanc chaud à mi-intensité
		f.lights[id]["state"] = map[string]any{"on": true, "bri": 127, "xy": []float64{0.45, 0.41}, "reachable": true}
	}
	f.mu.Unlock()
	snaps, err := d.SnapshotLights(ctx, []string{"P1", "P2"})
	if err != nil || len(snaps) != 2 {
		t.Fatalf("lecture de l'état des deux ampoules : %+v %v", snaps, err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225ZoneA(lighting.AreaPublic, 1, tw225Red, 255)}}); err != nil {
		t.Fatal(err)
	}
	if err := d.RestoreLights(ctx, snaps, nil); err != nil {
		t.Fatalf("réécriture de l'état lu : %v", err)
	}
	for _, id := range []string{"1", "2"} {
		if tw225IsRed(t, f, id) || tw225X(t, f, id) > 0.5 || tw225X(t, f, id) < 0.4 {
			t.Errorf("ampoule %s : la couleur d'avant l'essai est rétablie (x = %v)", id, tw225X(t, f, id))
		}
		if b := tw225Bri(f, id); b < 120 || b > 135 {
			t.Errorf("ampoule %s : l'intensité d'avant l'essai est rétablie (bri = %v)", id, b)
		}
	}
	if err := d.RestoreLights(ctx, nil, []string{"P1", "P2"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range []string{"1", "2"} {
		if f.lights[id]["state"].(map[string]any)["on"] != false {
			t.Errorf("lecture impossible ⇒ ampoule %s éteinte", id)
		}
	}
}
