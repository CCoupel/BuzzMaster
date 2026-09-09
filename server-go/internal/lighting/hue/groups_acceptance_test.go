// Suite d'acceptance test-writer pour le cycle de vie des groupes Hue
// (Batch B, B4), contracts/hue-bridge.md §5.8, planner report
// planner-v10-groups-teamcolor-20260907-173831.md §Problème 2. Contre
// l'implémentation réelle de dev-backend (groups.go, commit c0eba926).
//
// Complémentaire de driver_dev_test.go (dev-backend, commit b7278160), qui
// couvre déjà : la création/correction-en-place/suppression d'un groupe
// dérivé d'UNE configuration figée et l'idempotence sur un second passage
// identique (TestDevGroupsReconciliation_CreatesCorrectsDeletes), la règle
// cœur B2 « on écrit par groupe dès 2 membres sales » et le dédup ordinaire
// (TestDevApplyWritesViaGroupWhenTargetHasAtLeastTwoLights), LE piège §5.3
// exact demandé par la tâche — écrire un groupe puis vérifier qu'une
// écriture individuelle suivante n'est pas dédupliquée à tort contre l'état
// pré-groupe (TestDevGroupWriteKeepsPerLightDedupCacheHonest, déjà exhaustif
// : **non dupliqué ici**), et le repli quand un groupe pourtant réconcilié
// disparaît entre la réconciliation et l'écriture
// (TestDevGroupWriteFallsBackToPerLightOnFailure).
//
// Ce fichier-ci ajoute ce qu'aucun des tests ci-dessus n'exerce :
//   - la réconciliation suite à un CHANGEMENT DE CONFIGURATION (ampoule
//     ajoutée/retirée/réaffectée à une équipe) entre deux réconciliations —
//     les tests existants corrigent une dérive côté PONT sur une
//     configuration qui, elle, ne change jamais ;
//   - un groupe encore désiré, supprimé À LA MAIN sur le pont (hors
//     BuzzMaster), recréé à la réconciliation suivante — distinct de la
//     suppression d'un groupe devenu SANS OBJET (déjà couvert) et du
//     scénario de repli à l'écriture (le groupe y disparaît APRÈS la
//     réconciliation, jamais avant/pendant elle) ;
//   - un échec de la réconciliation elle-même (lecture/création de groupe
//     indisponible) dès le départ, sans aucun groupe jamais réconcilié —
//     distinct du scénario de repli existant, où le groupe a d'abord été
//     réconcilié AVEC succès avant de disparaître ;
//   - qu'aucune mutation de composition de groupe (POST/PUT/DELETE
//     /groups) n'est jamais émise par une séquence d'événements de jeu —
//     seules des écritures d'état (PUT .../action ou .../lights/<id>/state)
//     doivent apparaître, conformément à la règle 3 du §5.8.
//
// Le piège §5.3 (dédup après écriture de groupe) n'est PAS répété ici :
// TestDevGroupWriteKeepsPerLightDedupCacheHonest le couvre déjà
// exactement comme demandé par la tâche.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Aides préfixées twB (Batch B) pour ne jamais entrer en collision avec un
// nom déclaré dans driver_dev_test.go.
package hue

import (
	"context"
	"sort"
	"strings"
	"testing"

	"buzzcontrol/internal/lighting"
)

func twBGroupsByName(f *devBridge) map[string]*devGroup {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]*devGroup, len(f.groups))
	for _, g := range f.groups {
		out[g.name] = g
	}
	return out
}

func twBSortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Item 1 — réconciliation suite à une RÉAFFECTATION de configuration
// (ampoule ajoutée/retirée/réaffectée), pas seulement une dérive côté pont
// sur une configuration figée.
// ---------------------------------------------------------------------------

func TestGroups_ReconciliationTracksConfigReassignment_AddRemoveMoveTeamBulb(t *testing.T) {
	f := newDevBridge(t, "G1", "T1a", "T1b", "T2a")
	d, _ := newDevGroupsEnabled(t, f,
		LightSpec{Name: "G1"},
		LightSpec{Name: "T1a", Role: RoleTeam, Team: "A"}, LightSpec{Name: "T1b", Role: RoleTeam, Team: "A"},
		LightSpec{Name: "T2a", Role: RoleTeam, Team: "B"}, // seule ampoule de B : pas de groupe (règle ≥2)
	)
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation initiale : %v", err)
	}
	groups := twBGroupsByName(f)
	if _, ok := groups["buzzmaster-team-B"]; ok {
		t.Fatal("setup invalide : B n'a qu'une ampoule, aucun groupe ne devrait exister pour elle")
	}
	if got := twBSortedCopy(groups["buzzmaster-team-A"].lights); twBJoinIDs(got) != "2,3" {
		t.Fatalf("setup invalide : team-A doit avoir T1a+T1b (ids 2,3), got %v", got)
	}

	// Étape A — RÉAFFECTATION : T2a (équipe B, seule) rejoint l'équipe A.
	// B disparaît (plus aucune ampoule), A passe à 3 membres.
	d.mu.Lock()
	for i := range d.cfg.Lights {
		if d.cfg.Lights[i].Name == "T2a" {
			d.cfg.Lights[i].Team = "A"
		}
	}
	d.mu.Unlock()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation après réaffectation : %v", err)
	}
	groups = twBGroupsByName(f)
	if _, ok := groups["buzzmaster-team-B"]; ok {
		t.Error("l'équipe B n'a plus aucune ampoule après réaffectation : son groupe doit disparaître")
	}
	if g, ok := groups["buzzmaster-team-A"]; !ok || twBSortedCopy(g.lights) == nil {
		t.Fatal("le groupe team-A doit toujours exister")
	} else if got := twBSortedCopy(g.lights); twBJoinIDs(got) != "2,3,4" {
		t.Errorf("team-A doit maintenant compter T1a+T1b+T2a (ids 2,3,4), got %v", got)
	}

	// Étape B — RETRAIT : T1b quitte l'équipe A pour repasser en 'general'.
	// A retombe à 2 membres (T1a+T2a, toujours ≥2, le groupe persiste avec
	// une composition corrigée) ; 'general' gagne T1b.
	d.mu.Lock()
	for i := range d.cfg.Lights {
		if d.cfg.Lights[i].Name == "T1b" {
			d.cfg.Lights[i] = LightSpec{Name: "T1b", Role: RoleGeneral}
		}
	}
	d.mu.Unlock()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation après retrait : %v", err)
	}
	groups = twBGroupsByName(f)
	if got := twBSortedCopy(groups["buzzmaster-general"].lights); twBJoinIDs(got) != "1,3" {
		t.Errorf("'general' doit maintenant compter G1+T1b (ids 1,3), got %v", got)
	}
	if got := twBSortedCopy(groups["buzzmaster-team-A"].lights); twBJoinIDs(got) != "2,4" {
		t.Errorf("team-A doit retomber à T1a+T2a (ids 2,4), got %v", got)
	}

	// Étape C — RÉAFFECTATION vers une équipe NEUVE d'une seule ampoule :
	// T1a rejoint une équipe "C" à elle seule. team-A ne garde que T2a (1
	// membre) : son groupe doit être SUPPRIMÉ (règle ≥2), et aucun groupe
	// team-C ne doit être créé (elle aussi n'a qu'une ampoule).
	d.mu.Lock()
	for i := range d.cfg.Lights {
		if d.cfg.Lights[i].Name == "T1a" {
			d.cfg.Lights[i].Team = "C"
		}
	}
	d.mu.Unlock()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation après réaffectation vers équipe neuve : %v", err)
	}
	groups = twBGroupsByName(f)
	if _, ok := groups["buzzmaster-team-A"]; ok {
		t.Error("team-A n'a plus qu'une ampoule (T2a) : son groupe doit être supprimé")
	}
	if _, ok := groups["buzzmaster-team-C"]; ok {
		t.Error("team-C n'a qu'une ampoule (T1a) : aucun groupe ne doit être créé pour elle")
	}
}

// ---------------------------------------------------------------------------
// Item 2 — groupe encore désiré, supprimé À LA MAIN sur le pont (hors
// BuzzMaster) ⇒ recréé à la réconciliation suivante.
// ---------------------------------------------------------------------------

func TestGroups_ExternallyDeletedGroup_RecreatedAtNextReconciliation(t *testing.T) {
	f := newDevBridge(t, "T1a", "T1b")
	d, _ := newDevGroupsEnabled(t, f, LightSpec{Name: "T1a", Role: RoleTeam, Team: "A"}, LightSpec{Name: "T1b", Role: RoleTeam, Team: "A"})
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation initiale : %v", err)
	}
	before := twBGroupsByName(f)
	g, ok := before["buzzmaster-ambiance"]
	if !ok {
		t.Fatal("setup invalide : buzzmaster-ambiance devrait exister après la réconciliation initiale")
	}
	oldMembers := twBSortedCopy(g.lights)

	// Suppression MANUELLE (hors BuzzMaster) — comme dans l'app Hue.
	var oldID string
	f.mu.Lock()
	for id, gg := range f.groups {
		if gg.name == "buzzmaster-ambiance" {
			oldID = id
		}
	}
	delete(f.groups, oldID)
	f.mu.Unlock()

	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation après suppression manuelle : %v", err)
	}
	after := twBGroupsByName(f)
	recreated, ok := after["buzzmaster-ambiance"]
	if !ok {
		t.Fatal("le groupe supprimé à la main doit être RECRÉÉ à la réconciliation suivante, il a disparu pour de bon")
	}
	if got := twBSortedCopy(recreated.lights); twBJoinIDs(got) != twBJoinIDs(oldMembers) {
		t.Errorf("le groupe recréé doit porter exactement les mêmes membres qu'avant, got %v want %v", got, oldMembers)
	}

	// Il doit être RÉELLEMENT réutilisable ensuite : une écriture générale
	// (2 membres sales) doit passer par LUI en un seul PUT d'action.
	baseline := len(f.puts())
	if err := d.Apply(ctx, devGeneral([3]int{10, 20, 30}, 90)); err != nil {
		t.Fatalf("Apply après recréation : %v", err)
	}
	puts := f.puts()[baseline:]
	if len(puts) != 1 || !strings.Contains(puts[0].path, "/action") {
		t.Fatalf("attendu exactement 1 PUT d'action de groupe après recréation, got %+v", puts)
	}
}

// ---------------------------------------------------------------------------
// Item 3 — échec de résolution/création dès la première réconciliation
// (aucun groupe n'a jamais existé) ⇒ repli par ampoule, sans régression
// fonctionnelle. Distinct de TestDevGroupWriteFallsBackToPerLightOnFailure
// (dev-backend), où le groupe est d'abord réconcilié AVEC succès puis
// disparaît avant l'écriture — ici, la réconciliation elle-même échoue dès
// le départ (bridge/firmware qui ne sert pas /groups du tout, ou erreur
// réseau sur le premier GET), avant même qu'une tentative de création soit
// possible.
// ---------------------------------------------------------------------------

func TestGroups_ReconciliationFailsFromTheStart_FallsBackWithoutRegression(t *testing.T) {
	f := newDevBridge(t, "L1", "L2", "L3")
	f.groupFails = true // /groups répond 404 à toute opération, dès le premier GET
	d, sink := newDevGroupsEnabled(t, f, LightSpec{Name: "L1"}, LightSpec{Name: "L2"}, LightSpec{Name: "L3"})
	ctx := context.Background()

	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("une réconciliation de groupes indisponible ne doit jamais faire échouer la résolution des ampoules : %v", err)
	}
	if len(f.groups) != 0 {
		t.Fatalf("aucun groupe ne peut avoir été créé, got %+v", f.groups)
	}
	if sink.count() == 0 {
		t.Error("l'échec de /groups doit être journalisé au moins une fois")
	}

	// Fonctionnellement : les 3 ampoules doivent quand même toutes recevoir
	// la bonne couleur, une par une (aucun groupe disponible pour les 3
	// membres 'general' pourtant dirty).
	if err := d.Apply(ctx, devGeneral([3]int{200, 50, 10}, 180)); err != nil {
		t.Fatalf("Apply doit réussir via le repli par ampoule : %v", err)
	}
	puts := f.puts()
	if len(puts) != 3 {
		t.Fatalf("attendu 3 écritures individuelles (aucun groupe disponible), got %d: %+v", len(puts), puts)
	}
	for _, p := range puts {
		if strings.Contains(p.path, "groups") {
			t.Errorf("aucune écriture ne doit cibler un groupe quand /groups est indisponible, got %q", p.path)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range []string{"1", "2", "3"} {
		state := f.lights[id]["state"].(map[string]any)
		if on, _ := state["on"].(bool); !on {
			t.Errorf("ampoule %s non atteinte malgré le repli par ampoule : %+v", id, state)
		}
	}
}

// ---------------------------------------------------------------------------
// Item 5 — aucune mutation de composition de groupe sur un événement de jeu
// (contract §5.8 règle 3). Simule une séquence d'événements de jeu (équipe
// active qui change, scène générale qui évolue) via une succession
// d'Apply() — jamais de RefreshInventory entre elles, exactement comme en
// production où Apply() ne redéclenche jamais de réconciliation.
// ---------------------------------------------------------------------------

func TestGroups_NoCompositionMutation_OnGameEventSequence(t *testing.T) {
	f := newDevBridge(t, "G1", "T1a", "T1b", "T2a")
	d, _ := newDevGroupsEnabled(t, f,
		LightSpec{Name: "G1"},
		LightSpec{Name: "T1a", Role: RoleTeam, Team: "A"}, LightSpec{Name: "T1b", Role: RoleTeam, Team: "A"},
		LightSpec{Name: "T2a", Role: RoleTeam, Team: "B"},
	)
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("réconciliation initiale : %v", err)
	}
	baseline := len(f.paths()) // tout ce qui suit vient de la SÉQUENCE DE JEU, pas de la réconciliation

	// Une "partie" qui tourne : l'équipe active alterne, la scène générale
	// change, une équipe est créditée — le genre de rafale d'événements
	// qu'un tour de jeu produit réellement.
	events := []lighting.State{
		{Zones: []lighting.ZoneState{
			{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
			{Zone: "A", Color: [3]int{255, 0, 0}, Intensity: 255},
			{Zone: "B", Color: [3]int{0, 0, 255}, Intensity: 64},
		}},
		{Zones: []lighting.ZoneState{
			{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
			{Zone: "A", Color: [3]int{255, 0, 0}, Intensity: 64},
			{Zone: "B", Color: [3]int{0, 0, 255}, Intensity: 255},
		}},
		{Zones: []lighting.ZoneState{
			{Zone: lighting.ZoneGeneral, Color: [3]int{0, 220, 60}, Intensity: 255}, // REVEAL
			{Zone: "A", Color: [3]int{255, 0, 0}, Intensity: 255},
			{Zone: "B", Color: [3]int{0, 0, 255}, Intensity: 64},
		}},
		{Zones: []lighting.ZoneState{
			{Zone: lighting.ZoneGeneral, Color: [3]int{255, 214, 170}, Intensity: 120}, // retour IDLE
			{Zone: "A", Color: [3]int{255, 0, 0}, Intensity: 64},
			{Zone: "B", Color: [3]int{0, 0, 255}, Intensity: 64},
		}},
	}
	for i, st := range events {
		if err := d.Apply(ctx, st); err != nil {
			t.Fatalf("étape %d : Apply a échoué : %v", i, err)
		}
	}

	sawGroupAction := false
	for _, p := range f.paths()[baseline:] {
		if strings.Contains(p, "/groups") {
			if strings.Contains(p, "/action") && strings.HasPrefix(p, "PUT ") {
				sawGroupAction = true
				continue // écriture d'état — autorisée, c'est tout le sens de B2
			}
			t.Errorf("mutation de composition de groupe détectée pendant une séquence de jeu — interdit par la règle 3 du §5.8 : %q", p)
		}
	}
	// Non-vacuité : la séquence doit avoir RÉELLEMENT emprunté au moins une
	// fois le chemin groupe (team A a 2 membres) — sinon l'absence de
	// mutation de composition ne prouverait rien puisque les groupes
	// n'auraient jamais été sollicités du tout.
	if !sawGroupAction {
		t.Fatal("setup invalide : la séquence n'a jamais écrit via un groupe (team-A, 2 membres) — le test ne prouverait rien")
	}
	if st := d.Status(); st.Stats.GroupWrites == 0 {
		t.Errorf("attendu au moins une écriture de groupe comptabilisée, got %+v", st.Stats)
	}
}

// twBJoinIDs is a tiny local formatter (comma-joined ids) — avoids pulling
// strings.Join at every assertion site above while staying obviously
// distinct from the stdlib name, per this file's twB-prefixing convention
// for anything that could otherwise shadow a package-level helper.
func twBJoinIDs(ids []string) string {
	return strings.Join(ids, ",")
}
