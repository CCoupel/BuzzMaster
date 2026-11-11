package hue

// Tests de spécification v11.7.0 LOT-1 (#225) — groupes d'ambiance et groupe
// animateur dans le pilote Hue. Écrits depuis contracts/light-effects.md §2.2,
// §2.3, §2.5 et contracts/hue-bridge.md §11, AVANT l'implémentation (TDD) :
// ils sont ROUGES (compilation comprise : LightSpec.Group, lighting.HostGroup(1),
// ZoneState.Group, SubGroupMinLights) tant que dev-backend n'a pas livré.
//
// Réutilise les aides de driver_dev_test.go (devBridge, newDevGroupsEnabled…).
// Toutes les aides ajoutées sont préfixées tw225 pour éviter toute collision.
// Aucune attente réelle : le pont est un faux pont httptest.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"buzzcontrol/internal/lighting"
)

var (
	tw225Red   = [3]int{255, 0, 0}
	tw225Blue  = [3]int{0, 0, 255}
	tw225White = [3]int{255, 255, 255}
)

// tw225Spec construit une ampoule générale rattachée au groupe n.
func tw225Spec(name string, group int) LightSpec {
	return LightSpec{Name: name, Role: RoleGeneral, Group: group}
}

// tw225Zone construit une entrée de zone « general » pour un groupe.
func tw225Zone(group int, color [3]int, intensity int) lighting.ZoneState {
	return lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: group, Color: color, Intensity: intensity}
}

// tw225XY lit la chromaticité x actuelle d'une ampoule du faux pont.
func tw225X(t *testing.T, f *devBridge, id string) float64 {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.lights[id]["state"].(map[string]any)
	switch xy := st["xy"].(type) {
	case []float64:
		return xy[0]
	case []any:
		v, _ := xy[0].(float64)
		return v
	}
	t.Fatalf("ampoule %s : xy illisible (%T)", id, st["xy"])
	return 0
}

func tw225Bri(f *devBridge, id string) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.lights[id]["state"].(map[string]any)
	switch b := st["bri"].(type) {
	case int:
		return float64(b)
	case float64:
		return b
	}
	return -1
}

// tw225IsRed / IsBlue : le rouge pur a x ≈ 0,70 ; le bleu pur x ≈ 0,14.
func tw225IsRed(t *testing.T, f *devBridge, id string) bool  { return tw225X(t, f, id) > 0.55 }
func tw225IsBlue(t *testing.T, f *devBridge, id string) bool { return tw225X(t, f, id) < 0.25 }

func tw225GroupAction(f *devBridge) []devRecorded {
	var out []devRecorded
	for _, p := range f.puts() {
		if strings.Contains(p.path, "/groups/") && strings.HasSuffix(p.path, "/action") {
			out = append(out, p)
		}
	}
	return out
}

func tw225GroupIDByName(f *devBridge, name string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, g := range f.groups {
		if g.name == name {
			return id, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// §2.3 — seuil des groupes Hue : SubGroupMinLights
// ---------------------------------------------------------------------------

func TestTW225_SubGroupMinLights_IsThreeByDefault(t *testing.T) {
	if SubGroupMinLights != 3 {
		t.Fatalf("SubGroupMinLights = %d, le contrat §2.3 fixe 3 (amendable seulement avec le chiffre mesuré en QUALIF)", SubGroupMinLights)
	}
}

// ---------------------------------------------------------------------------
// §2.3 — groupes Hue buzzmaster-general-<n> (≥ 3 ampoules) et repli par ampoule
// ---------------------------------------------------------------------------

func TestTW225_HueGroups_CreatedOnlyFromThreeLights_NamedByRank(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "A3", "B1", "B2", "C1", "C2", "C3", "C4")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("A3", 1), // groupe 1 : 3 ampoules → groupe Hue
		tw225Spec("B1", 2), tw225Spec("B2", 2), // groupe 2 : 2 ampoules → aucun groupe Hue
		tw225Spec("C1", 3), tw225Spec("C2", 3), tw225Spec("C3", 3), tw225Spec("C4", 3), // groupe 3 : 4 ampoules
	)
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("réconciliation : %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	byName := map[string][]string{}
	for _, g := range f.groups {
		byName[g.name] = twBSortedCopy(g.lights)
	}
	if got := fmt.Sprint(byName["buzzmaster-general-1"]); got != "[1 2 3]" {
		t.Errorf("buzzmaster-general-1 = %v, attendu [1 2 3]", byName["buzzmaster-general-1"])
	}
	if _, ok := byName["buzzmaster-general-2"]; ok {
		t.Error("le groupe d'ambiance 2 n'a que 2 ampoules (< SubGroupMinLights) : aucun groupe Hue buzzmaster-general-2")
	}
	if got := fmt.Sprint(byName["buzzmaster-general-3"]); got != "[6 7 8 9]" {
		t.Errorf("buzzmaster-general-3 = %v, attendu [6 7 8 9]", byName["buzzmaster-general-3"])
	}
	if _, ok := byName["buzzmaster-general"]; !ok {
		t.Error("buzzmaster-general (toute la zone générale) doit toujours exister")
	}
}

func TestTW225_HueGroups_StaleSubGroupDeletedWhenGroupShrinks(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "A3")
	f.groups["42"] = &devGroup{name: "buzzmaster-general-2", lights: []string{"1"}} // groupe d'une ancienne configuration
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("A3", 1))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-2"); ok {
		t.Error("un groupe Hue buzzmaster-general-<n> sans groupe d'ambiance correspondant doit être supprimé (§5.8)")
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-1"); !ok {
		t.Error("buzzmaster-general-1 doit être créé")
	}
}

func TestTW225_SubGroupWrite_OneGroupPUT_NotPerLight(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "A3", "B1", "B2", "B3")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("A3", 1),
		tw225Spec("B1", 2), tw225Spec("B2", 2), tw225Spec("B3", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225Red, 200),
		tw225Zone(1, tw225Blue, 200),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	// Groupe 1 bleu, groupe 2 (pas d'entrée Group 2) retombe sur Group 0 = rouge.
	for _, id := range []string{"1", "2", "3"} {
		if !tw225IsBlue(t, f, id) {
			t.Errorf("ampoule %s (groupe 1) devrait être bleue", id)
		}
	}
	for _, id := range []string{"4", "5", "6"} {
		if !tw225IsRed(t, f, id) {
			t.Errorf("ampoule %s (groupe 2, sans entrée dédiée) devrait suivre Group 0 = rouge", id)
		}
	}
	for _, p := range f.puts() {
		if strings.Contains(p.path, "/lights/") {
			t.Fatalf("des groupes de ≥ 3 ampoules doivent s'écrire par groupe Hue, pas par ampoule : %s", p.path)
		}
	}
	if n := len(tw225GroupAction(f)); n != 2 {
		t.Errorf("attendu 2 écritures de groupe (un état différent par groupe), obtenu %d", n)
	}
}

func TestTW225_BelowThreshold_FallsBackToPerLightWrites_NoGroupPUT(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "B1", "B2")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("B1", 2), tw225Spec("B2", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225Red, 200),
		tw225Zone(1, tw225Blue, 200),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	if !tw225IsBlue(t, f, "1") || !tw225IsBlue(t, f, "2") {
		t.Error("les ampoules du groupe 1 (2 ampoules) doivent être bleues, écrites ampoule par ampoule")
	}
	if !tw225IsRed(t, f, "3") || !tw225IsRed(t, f, "4") {
		t.Error("les ampoules du groupe 2 doivent suivre Group 0 = rouge")
	}
	for _, p := range f.puts() {
		if strings.Contains(p.path, "/groups/") && strings.HasSuffix(p.path, "/action") &&
			!strings.Contains(p.path, "/groups/"+mustGroupID(f, "buzzmaster-general")+"/") {
			t.Errorf("sous le seuil, aucune écriture de sous-groupe attendue : %s", p.path)
		}
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-1"); ok {
		t.Error("aucun groupe Hue buzzmaster-general-1 sous le seuil")
	}
}

func mustGroupID(f *devBridge, name string) string {
	id, _ := tw225GroupIDByName(f, name)
	if id == "" {
		return "-"
	}
	return id
}

// ---------------------------------------------------------------------------
// §2.3 — état identique partout ⇒ UNE écriture buzzmaster-general (non-régression de débit)
// ---------------------------------------------------------------------------

func TestTW225_AllStatesEqual_SingleBuzzmasterGeneralWrite(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "A3", "B1", "B2", "B3")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("A3", 1),
		tw225Spec("B1", 2), tw225Spec("B2", 2), tw225Spec("B3", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	// Entrées de groupe explicites mais identiques à Group 0.
	st := lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225Red, 180),
		tw225Zone(1, tw225Red, 180),
		tw225Zone(2, tw225Red, 180),
	}}
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	acts := tw225GroupAction(f)
	if len(acts) != 1 {
		t.Fatalf("états égaux : exactement 1 écriture de groupe attendue, obtenu %d (%v)", len(acts), f.paths())
	}
	gid := mustGroupID(f, "buzzmaster-general")
	if !strings.Contains(acts[0].path, "/groups/"+gid+"/action") {
		t.Errorf("l'écriture unique doit viser buzzmaster-general (id %s), obtenu %s", gid, acts[0].path)
	}
	// Aucun changement : aucune écriture supplémentaire.
	if err := d.Apply(ctx, st); err != nil {
		t.Fatal(err)
	}
	if n := len(f.puts()); n != 1 {
		t.Errorf("état inchangé : aucune nouvelle écriture, total %d", n)
	}
}

func TestTW225_NoGroups_BehaviourIdenticalToV1160(t *testing.T) {
	// Non-régression critique : sans groupe (LightSpec.Group = 0 partout), la
	// suite d'écritures est celle de v11.6.0 — 1 écriture de groupe par
	// changement d'état, rien quand l'état ne change pas.
	f := newDevBridge(t, "G1", "G2")
	d, _ := newDevGroupsEnabled(t, f, LightSpec{Name: "G1"}, LightSpec{Name: "G2"})
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	seq := []lighting.State{
		devGeneral(tw225Red, 200), devGeneral(tw225Red, 200), devGeneral(tw225Blue, 100),
		devGeneral(tw225Blue, 100), devGeneral(tw225White, 255),
	}
	for _, s := range seq {
		if err := d.Apply(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(f.puts()); n != 3 {
		t.Fatalf("3 changements d'état ⇒ 3 écritures exactement (v11.6.0), obtenu %d : %v", n, f.paths())
	}
	if st := d.Status(); st.Stats.GroupWrites != 3 {
		t.Errorf("les 3 écritures passent par buzzmaster-general, GroupWrites = %d", st.Stats.GroupWrites)
	}
	for _, name := range []string{"buzzmaster-general-1", "buzzmaster-general-host1"} {
		if _, ok := tw225GroupIDByName(f, name); ok {
			t.Errorf("sans groupe d'ambiance, le groupe Hue %s ne doit pas exister", name)
		}
	}
}

// ---------------------------------------------------------------------------
// §2.3 — cache : une écriture de sous-groupe ne laisse pas un cache périmé
// ---------------------------------------------------------------------------

func TestTW225_SubGroupWrite_KeepsPerLightCacheHonest(t *testing.T) {
	f := newDevBridge(t, "A1", "A2", "A3", "B1", "B2", "B3")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("A1", 1), tw225Spec("A2", 1), tw225Spec("A3", 1),
		tw225Spec("B1", 2), tw225Spec("B2", 2), tw225Spec("B3", 2))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	// 1) tout rouge, 2) groupe 1 bleu, 3) retour tout rouge : le groupe 1 doit
	// REDEVENIR rouge physiquement (un cache resté à « rouge » l'ignorerait).
	steps := []lighting.State{
		{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200)}},
		{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200), tw225Zone(1, tw225Blue, 200)}},
		{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200)}},
	}
	for _, s := range steps {
		if err := d.Apply(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"1", "2", "3", "4", "5", "6"} {
		if !tw225IsRed(t, f, id) {
			t.Errorf("ampoule %s doit être rouge à la fin (cache d'ampoule périmé après écriture de sous-groupe ?)", id)
		}
	}
}

// ---------------------------------------------------------------------------
// §2.5 — groupe animateur
// ---------------------------------------------------------------------------

func TestTW225_Host_HostGroupIsMinusRank(t *testing.T) {
	// §2.2 (révision du GATE LOT-1) : A1 ⇒ -1, A2 ⇒ -2… ; HostGroup(1) reste la valeur de l'ancien GroupHost.
	for k := 1; k <= 4; k++ {
		if got := lighting.HostGroup(k); got != -k {
			t.Errorf("HostGroup(%d) = %d, attendu %d", k, got, -k)
		}
	}
}

func TestTW225_Host_NeverReceivesGroupZeroEntry(t *testing.T) {
	// Salle (2 ampoules) + animateur (1 ampoule). L'état n'émet que Group 0 :
	// l'ampoule de l'animateur ne doit JAMAIS être touchée (contrat §2.5).
	f := newDevBridge(t, "S1", "S2", "H1")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 1), tw225Spec("S2", 1), tw225Spec("H1", lighting.HostGroup(1)))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200)}}); err != nil {
		t.Fatal(err)
	}
	if !tw225IsRed(t, f, "1") || !tw225IsRed(t, f, "2") {
		t.Error("les ampoules de salle suivent Group 0")
	}
	if got := tw225Bri(f, "3"); got != 1 {
		t.Errorf("l'ampoule de l'animateur ne doit pas être écrite par Group 0 (bri = %v, initial 1)", got)
	}
	for _, p := range f.puts() {
		if strings.HasSuffix(p.path, "/lights/3/state") {
			t.Errorf("écriture inattendue de l'ampoule animateur : %s", p.path)
		}
	}
}

func TestTW225_Host_WrittenOnlyByItsOwnEntry_WhiteStaysWhenRoomChanges(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "H1")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 1), tw225Spec("S2", 1), tw225Spec("H1", lighting.HostGroup(1)))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	host := tw225Zone(lighting.HostGroup(1), tw225White, 255)
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225Red, 200), host}}); err != nil {
		t.Fatal(err)
	}
	if got := tw225Bri(f, "3"); got < 250 {
		t.Fatalf("animateur en blanc fixe : bri = %v, attendu 254-255", got)
	}
	puts := len(f.puts())
	// La salle change, l'animateur garde la même entrée : aucune réécriture de l'animateur.
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225Blue, 100), host}}); err != nil {
		t.Fatal(err)
	}
	hostWrites := 0
	for _, p := range f.puts()[puts:] {
		if strings.HasSuffix(p.path, "/lights/3/state") {
			hostWrites++
		}
	}
	if hostWrites != 0 {
		t.Errorf("l'animateur (état inchangé) ne doit pas être réécrit quand la salle change, %d écriture(s)", hostWrites)
	}
	if got := tw225Bri(f, "3"); got < 250 {
		t.Errorf("l'animateur doit rester blanc fixe, bri = %v", got)
	}
}

func TestTW225_Host_HueGroupFromThreeLights_ExcludedFromGeneral(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "S3", "H1", "H2", "H3")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("S1", 1), tw225Spec("S2", 1), tw225Spec("S3", 1),
		tw225Spec("H1", lighting.HostGroup(1)), tw225Spec("H2", lighting.HostGroup(1)), tw225Spec("H3", lighting.HostGroup(1)))
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	byName := map[string][]string{}
	for _, g := range f.groups {
		byName[g.name] = twBSortedCopy(g.lights)
	}
	if got := fmt.Sprint(byName["buzzmaster-general-host1"]); got != "[4 5 6]" {
		t.Errorf("buzzmaster-general-host = %v, attendu [4 5 6]", byName["buzzmaster-general-host1"])
	}
	if got := fmt.Sprint(byName["buzzmaster-general"]); got != "[1 2 3]" {
		t.Errorf("buzzmaster-general doit EXCLURE les ampoules de l'animateur : %v, attendu [1 2 3]", byName["buzzmaster-general"])
	}
}

func TestTW225_Host_UnderThreshold_NoHueGroup_PerLightWrite(t *testing.T) {
	f := newDevBridge(t, "S1", "S2", "H1", "H2")
	d, _ := newDevGroupsEnabled(t, f,
		tw225Spec("S1", 0), tw225Spec("S2", 0),
		tw225Spec("H1", lighting.HostGroup(1)), tw225Spec("H2", lighting.HostGroup(1)))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := tw225GroupIDByName(f, "buzzmaster-general-host1"); ok {
		t.Error("2 ampoules < SubGroupMinLights : pas de groupe Hue buzzmaster-general-host1")
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225Red, 200), tw225Zone(lighting.HostGroup(1), tw225White, 255)}}); err != nil {
		t.Fatal(err)
	}
	if tw225Bri(f, "3") < 250 || tw225Bri(f, "4") < 250 {
		t.Error("les deux ampoules de l'animateur doivent être en blanc fixe (écriture par ampoule)")
	}
}

func TestTW225_Host_NoHostEntry_LeavesHostBulbsUntouched_AtShutdownStyleGeneralOff(t *testing.T) {
	// Un état « tout éteint » Group 0 (ex. OFF) n'éteint pas l'animateur :
	// seul l'interrupteur général / l'arrêt le font, en émettant EXPLICITEMENT
	// une entrée GroupHost éteinte (§2.5).
	f := newDevBridge(t, "S1", "S2", "H1")
	d, _ := newDevGroupsEnabled(t, f, tw225Spec("S1", 0), tw225Spec("S2", 0), tw225Spec("H1", lighting.HostGroup(1)))
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225White, 255), tw225Zone(lighting.HostGroup(1), tw225White, 255)}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{tw225Zone(0, tw225White, 0)}}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	on := f.lights["3"]["state"].(map[string]any)["on"]
	f.mu.Unlock()
	if on != true {
		t.Errorf("OFF de la salle (Group 0 seul) ne doit pas éteindre l'animateur, on = %v", on)
	}
	// Extinction explicite du groupe animateur.
	if err := d.Apply(ctx, lighting.State{Zones: []lighting.ZoneState{
		tw225Zone(0, tw225White, 0), tw225Zone(lighting.HostGroup(1), tw225White, 0)}}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	on = f.lights["3"]["state"].(map[string]any)["on"]
	f.mu.Unlock()
	if on != false {
		t.Errorf("une entrée GroupHost d'intensité 0 doit éteindre l'animateur, on = %v", on)
	}
}
