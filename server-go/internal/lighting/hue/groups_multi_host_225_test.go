package hue

// v11.7.0 LOT-1, révision du GATE LOT-1 (#225) — plusieurs groupes animateur dans le pilote Hue.
// Source : contracts/light-effects.md §2.2, §2.3, §2.5 (révisé) ; hue-bridge.md. Rang k ⇒ Group = lighting.HostGroup(k) = -k,
// groupe Hue `buzzmaster-general-host<k>` (≥ 3 ampoules), l'ancien `buzzmaster-general-host` disparaît à la réconciliation,
// `buzzmaster-general` exclut les ampoules de TOUS les groupes animateur. Aides de groupes_ambiance_225_test.go (tw225*).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"buzzcontrol/internal/lighting"
)

func tw225Host(name string, k int) LightSpec { return tw225Spec(name, lighting.HostGroup(k)) }

func TestTW225Multi_HueGroupsPerHostRank_AndGeneralExcludesAllHosts(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "S3", "H1a", "H1b", "H1c", "H2a", "H2b", "H2c")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("S1", 1), tw225Spec("S2", 1), tw225Spec("S3", 1),
		tw225Host("H1a", 1), tw225Host("H1b", 1), tw225Host("H1c", 1),
		tw225Host("H2a", 2), tw225Host("H2b", 2), tw225Host("H2c", 2))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	by := map[string]string{}
	for _, g := range f.groups {
		by[g.name] = fmt.Sprint(twBSortedCopy(g.lights))
	}
	if by["buzzmaster-general-host1"] != "[4 5 6]" {
		t.Errorf("buzzmaster-general-host1 = %s, attendu [4 5 6]", by["buzzmaster-general-host1"])
	}
	if by["buzzmaster-general-host2"] != "[7 8 9]" {
		t.Errorf("buzzmaster-general-host2 = %s, attendu [7 8 9]", by["buzzmaster-general-host2"])
	}
	if by["buzzmaster-general"] != "[1 2 3]" {
		t.Errorf("buzzmaster-general exclut les ampoules de tous les groupes animateur : %s, attendu [1 2 3]", by["buzzmaster-general"])
	}
}

func TestTW225Multi_OldSingleHostGroupDeletedByReconciliation(t *testing.T) {
	f := newDevBridge(t, "S1", "H1a", "H1b", "H1c")
	f.groups["77"] = &devGroup{name: "buzzmaster-general-host", lights: []string{"2", "3", "4"}} // ancienne forme (QUALIF 11.7.0.2)
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("H1a", 1), tw225Host("H1b", 1), tw225Host("H1c", 1))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-host"); ok {
		t.Error("l'ancien groupe Hue buzzmaster-general-host est supprimé (préfixe buzzmaster-)")
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-host1"); !ok {
		t.Error("buzzmaster-general-host1 créé")
	}
}

func TestTW225Multi_HostGroupHueNotCreatedBelowThreshold(t *testing.T) {
	f := newDevBridge(t, "S1", "A1", "A2", "B1", "B2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("A1", 1), tw225Host("A2", 1), tw225Host("B1", 2), tw225Host("B2", 2))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"buzzmaster-general-host1", "buzzmaster-general-host2"} {
		if _, ok := tw225GroupIDByName(f, n); ok {
			t.Errorf("%s : 2 ampoules < SubGroupMinLights, pas de groupe Hue", n)
		}
	}
}

func TestTW225Multi_EachHostWrittenOnlyByItsOwnEntry(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "H1", "H2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Spec("S2", 0), tw225Host("H1", 1), tw225Host("H2", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	// Group 0 seul : aucun des deux animateurs n'est touché.
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200)}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"3", "4"} {
		if tw225Bri(f, id) != 1 {
			t.Errorf("l'ampoule %s d'un groupe animateur ne reçoit jamais Group 0 (bri = %v)", id, tw225Bri(f, id))
		}
	}
	// Entrées dédiées : A1 rouge, A2 bleu, indépendantes.
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225Red, 200),
		tw225Zone(lighting.HostGroup(1), tw225Red, 255),
		tw225Zone(lighting.HostGroup(2), tw225Blue, 255),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	if !tw225IsRed(t, f, "3") || !tw225IsBlue(t, f, "4") {
		t.Error("A1 rouge, A2 bleu : chaque groupe animateur suit sa propre entrée")
	}
	// A2 change seule : A1 n'est pas réécrit.
	before := len(f.puts())
	st.Zones[2] = tw225Zone(lighting.HostGroup(2), tw225White, 255)
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.puts()[before:] {
		if strings.HasSuffix(p.path, "/lights/3/state") {
			t.Errorf("A1 inchangé ne doit pas être réécrit : %s", p.path)
		}
	}
}

func TestTW225Multi_ExplicitOffEntriesSwitchEachHostOff_RoomOffLeavesThemOn(t *testing.T) {
	f := newDevBridge(t, "S1", "H1", "H2")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Host("H1", 1), tw225Host("H2", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	on := lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 255),
		tw225Zone(lighting.HostGroup(1), tw225White, 255), tw225Zone(lighting.HostGroup(2), tw225White, 255)}}
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
	if state("2") != true || state("3") != true {
		t.Error("OFF de la salle (Group 0) laisse les deux groupes animateur allumés")
	}
	off := lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 0),
		tw225Zone(lighting.HostGroup(1), tw225White, 0), tw225Zone(lighting.HostGroup(2), tw225White, 0)}}
	if err := d.Apply(ctx, off); err != nil {
		t.Fatal(err)
	}
	if state("2") != false || state("3") != false {
		t.Error("les entrées explicites éteignent chacun des groupes animateur")
	}
}

func TestTW225Multi_FourHostGroupsAllAddressable(t *testing.T) {
	f := newDevBridge(t, "H1", "H2", "H3", "H4")
	d, _ := newDevGroupsEnabled(t, f, tw225Host("H1", 1), tw225Host("H2", 2), tw225Host("H3", 3), tw225Host("H4", 4))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 255)}}
	cols := [][3]int{tw225Red, tw225Blue, tw225Red, tw225Blue}
	for k := 1; k <= 4; k++ {
		st.Zones = append(st.Zones, tw225Zone(lighting.HostGroup(k), cols[k-1], 255))
	}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	for k, id := range []string{"1", "2", "3", "4"} {
		if cols[k] == tw225Red && !tw225IsRed(t, f, id) || cols[k] == tw225Blue && !tw225IsBlue(t, f, id) {
			t.Errorf("groupe animateur A%d (ampoule %s) : mauvaise couleur", k+1, id)
		}
	}
}
