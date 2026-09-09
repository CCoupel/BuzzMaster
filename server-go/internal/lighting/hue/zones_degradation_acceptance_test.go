// Suite d'acceptance test-writer pour #213 (milestone v10.0.0, Batch 2,
// T2.4) — contracts/hue-bridge.md §5.2 (résolution de zone) et §5.7 (les
// quatre règles de dégradation normatives).
//
// Complémentaire de driver_test.go (test-writer, #206), qui couvre déjà au
// niveau unitaire zoneFor() (TestZoneFor_GeneralLight_AlwaysFollowsGeneralZone,
// TestZoneFor_TeamLight_FollowsItsTeamZoneWhenPresent,
// TestZoneFor_TeamLight_FallsBackToGeneralWhenTeamNotNamed,
// TestZoneFor_NoMatchingZoneAtAll_IsASilentNonEvent) et une ampoule
// injoignable en zone general (TestDriver_OneUnreachableLight_OthersStillWritten).
// Ce fichier-ci n'en duplique aucun — il ajoute :
//   - la nuance §5.7 « une équipe dépourvue d'ampoule ne repeint jamais
//     general », non testée explicitement (les tests zoneFor existants ne
//     placent jamais une zone d'équipe NON pourvue en concurrence d'une zone
//     general dans le même State) ;
//   - les 4 règles de §5.7 exercées via le VRAI Apply() + un faux pont
//     httptest (pas seulement la fonction pure zoneFor), avec des scénarios
//     à plusieurs équipes ;
//   - une ampoule D'ÉQUIPE injoignable spécifiquement (le test existant porte
//     sur des ampoules non affectées à un rôle team) — §5.7 note explicitement
//     que « c'est sur une ampoule d'équipe qu'on l'oublie le plus facilement » ;
//   - un scénario combinant les 4 règles à la fois, comme demandé par la
//     tâche T2.4.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Toutes les aides ci-dessous sont préfixées tw213 pour ne jamais entrer en
// collision avec un nom déclaré ailleurs dans le paquet (driver_test.go).
package hue

import (
	"context"
	"testing"

	"buzzcontrol/internal/lighting"
)

// tw213LightState reads back what the fake bridge actually stored for one
// light — white-box (package hue), like the rest of this test package.
func tw213LightState(t *testing.T, fb *fakeBridge, id string) (on bool, bri int, xy [2]float64) {
	t.Helper()
	fb.mu.Lock()
	defer fb.mu.Unlock()
	l, ok := fb.lights[id]
	if !ok {
		t.Fatalf("fausse ampoule %q introuvable sur le faux pont", id)
	}
	return l.on, l.bri, l.xy
}

func tw213AssertLightColor(t *testing.T, fb *fakeBridge, id, label string, color [3]int, intensity int) {
	t.Helper()
	on, bri, xy := tw213LightState(t, fb, id)
	wantOn := intensity > 0
	wantBri := intensityToBri(intensity)
	wantXY := rgbToXY(color[0], color[1], color[2])
	if on != wantOn || bri != wantBri || xy != wantXY {
		t.Errorf("%s (%s) = on=%v bri=%d xy=%v, want on=%v bri=%d xy=%v", label, id, on, bri, xy, wantOn, wantBri, wantXY)
	}
}

// ---------------------------------------------------------------------------
// §5.2 — résolution de zone : general reste general MÊME quand des zones
// d'équipe (pourvues ou non) sont présentes dans le même State.
// ---------------------------------------------------------------------------

// TestCA213_Zone_GeneralNeverAffectedByDecoyTeamZones renforce
// TestZoneFor_GeneralLight_AlwaysFollowsGeneralZone (driver_test.go) : cette
// dernière ne place qu'UNE seule zone dans le State (general seule), donc ne
// prouve pas que general résiste à la PRÉSENCE d'autres zones. Ici, trois
// zones d'équipe (dont deux non pourvues d'ampoule) sont présentes à côté de
// general — zoneFor pour une ampoule 'general' doit rester insensible à leur
// contenu (contract §5.7 : « ne repeint pas general »).
func TestCA213_Zone_GeneralNeverAffectedByDecoyTeamZones(t *testing.T) {
	lc := LightSpec{Name: "Salle", Role: RoleGeneral}
	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
		{Zone: "Rouges", Color: [3]int{220, 20, 20}, Intensity: 255},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255},
		{Zone: "Verts", Color: [3]int{20, 220, 20}, Intensity: 255},
	}}
	z, ok := zoneFor(lc, st)
	if !ok {
		t.Fatal("une ampoule general doit toujours trouver la zone general")
	}
	if z.Color != [3]int{40, 90, 255} || z.Intensity != 160 {
		t.Fatalf("une ampoule general a suivi une zone d'équipe au lieu de general : %+v", z)
	}
}

// ---------------------------------------------------------------------------
// §5.7 — les 4 règles de dégradation, une par test, via le VRAI Apply()
// ---------------------------------------------------------------------------

// TestCA213_Degradation_FewerLightsThanTeams — règle 1 (« le cas le plus
// fréquent en soirée réelle ») : 3 équipes dans l'état de jeu, seulement 2
// pourvues d'une ampoule. La 3e (Verts) n'est écrite nulle part ; les deux
// autres sont rendues correctement ; general n'est jamais repeint à leur
// couleur.
func TestCA213_Degradation_FewerLightsThanTeams(t *testing.T) {
	fb := newFakeBridge(t, "fffe0000deadbeef")
	fb.addLight("1", "Salle")
	fb.addLight("2", "AmpouleRouge")
	fb.addLight("3", "AmpouleBleue")
	// Aucune ampoule pour "Verts" : ni id ni LightSpec.

	d := newTestDriver(t, fb, []LightSpec{
		{Name: "Salle", Role: RoleGeneral},
		{Name: "AmpouleRouge", Role: RoleTeam, Team: "Rouges"},
		{Name: "AmpouleBleue", Role: RoleTeam, Team: "Bleus"},
	}, nil)

	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
		{Zone: "Rouges", Color: [3]int{220, 20, 20}, Intensity: 255},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255},
		{Zone: "Verts", Color: [3]int{20, 220, 20}, Intensity: 255},
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatalf("Apply a échoué : %v", err)
	}

	if got := fb.writeCount(); got != 3 {
		t.Fatalf("seules les 3 ampoules configurées doivent recevoir une écriture (jamais 'Verts', sans ampoule), got %d", got)
	}
	tw213AssertLightColor(t, fb, "1", "Salle (general)", [3]int{40, 90, 255}, 160)
	tw213AssertLightColor(t, fb, "2", "AmpouleRouge", [3]int{220, 20, 20}, 255)
	tw213AssertLightColor(t, fb, "3", "AmpouleBleue", [3]int{20, 20, 220}, 255)
}

// TestCA213_Degradation_TeamWithoutBulb_NeverRepaintsGeneral — règle 2, cas
// minimal isolé (une seule équipe, non pourvue) pour vérifier précisément la
// clause « ne repeint pas general à la couleur de cette équipe » — la salle
// ne doit JAMAIS dépendre de quelle équipe se trouve dépourvue d'ampoule.
func TestCA213_Degradation_TeamWithoutBulb_NeverRepaintsGeneral(t *testing.T) {
	fb := newFakeBridge(t, "fffe0000deadbeef")
	fb.addLight("1", "Salle")

	d := newTestDriver(t, fb, []LightSpec{{Name: "Salle", Role: RoleGeneral}}, nil)

	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255}, // "Bleus" n'a aucune ampoule
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatalf("Apply a échoué : %v", err)
	}
	if got := fb.writeCount(); got != 1 {
		t.Fatalf("seule 'Salle' doit être écrite (aucune ampoule pour 'Bleus'), got %d écriture(s)", got)
	}
	tw213AssertLightColor(t, fb, "1", "Salle (general)", [3]int{40, 90, 255}, 160)
}

// TestCA213_Degradation_NoTeamBulbsAtAll_FullFallbackToGeneral — règle 3 :
// une installation SANS aucune ampoule d'équipe (uniquement des ampoules
// 'general') doit se comporter exactement comme avant #213, quel que soit le
// nombre de zones d'équipe présentes dans le State — aucune d'entre elles ne
// trouve jamais de destinataire.
func TestCA213_Degradation_NoTeamBulbsAtAll_FullFallbackToGeneral(t *testing.T) {
	fb := newFakeBridge(t, "fffe0000deadbeef")
	fb.addLight("1", "Salle")

	d := newTestDriver(t, fb, []LightSpec{{Name: "Salle", Role: RoleGeneral}}, nil)

	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{220, 20, 20}, Intensity: 255}, // TEAM_TURN "Rouges" porté par general
		{Zone: "Rouges", Color: [3]int{220, 20, 20}, Intensity: 255},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255},
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatalf("Apply a échoué : %v", err)
	}
	if got := fb.writeCount(); got != 1 {
		t.Fatalf("aucune ampoule d'équipe configurée : une seule écriture ('Salle') attendue, got %d", got)
	}
	tw213AssertLightColor(t, fb, "1", "Salle (general, portant la couleur d'équipe active pré-#213)", [3]int{220, 20, 20}, 255)
}

// TestCA213_Degradation_UnreachableTeamBulb_NeverBlocksOthers — règle 4,
// spécifiquement sur une ampoule de RÔLE TEAM (le test existant,
// TestDriver_OneUnreachableLight_OthersStillWritten, ne porte que sur des
// ampoules de rôle general implicite) : contract §5.7 note que c'est sur une
// ampoule d'équipe qu'on oublie le plus facilement cette règle.
func TestCA213_Degradation_UnreachableTeamBulb_NeverBlocksOthers(t *testing.T) {
	fb := newFakeBridge(t, "fffe0000deadbeef")
	fb.addLight("1", "Salle")
	fb.addLight("2", "AmpouleRouge")
	bleue := fb.addLight("3", "AmpouleBleue") // "éteinte au mur"
	bleue.reachable = false
	bleue.putErr = &hueError{Type: 201, Description: "device is off"}

	d := newTestDriver(t, fb, []LightSpec{
		{Name: "Salle", Role: RoleGeneral},
		{Name: "AmpouleRouge", Role: RoleTeam, Team: "Rouges"},
		{Name: "AmpouleBleue", Role: RoleTeam, Team: "Bleus"},
	}, nil)

	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
		{Zone: "Rouges", Color: [3]int{220, 20, 20}, Intensity: 255},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255},
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatalf("une ampoule d'équipe injoignable ne doit jamais faire échouer Apply : %v", err)
	}

	report := d.Status()
	var salle, rouge, bleu LightStatus
	for _, l := range report.Lights {
		switch l.Name {
		case "Salle":
			salle = l
		case "AmpouleRouge":
			rouge = l
		case "AmpouleBleue":
			bleu = l
		}
	}
	if salle.LastError != "" || rouge.LastError != "" {
		t.Errorf("general et l'équipe joignable ne doivent porter aucune erreur : general=%+v rouge=%+v", salle, rouge)
	}
	if bleu.LastError == "" {
		t.Error("l'ampoule d'équipe injoignable doit porter une erreur visible")
	}
	tw213AssertLightColor(t, fb, "1", "Salle (general)", [3]int{40, 90, 255}, 160)
	tw213AssertLightColor(t, fb, "2", "AmpouleRouge", [3]int{220, 20, 20}, 255)
}

// TestCA213_Degradation_CombinedScenario_AllFourRulesAtOnce — demandé
// explicitement par la tâche T2.4 (« plus un test combinant plusieurs cas
// simultanément ») : 3 équipes, 2 pourvues (une joignable, une injoignable),
// 1 non pourvue, aux côtés de general. Les 4 règles du §5.7 s'appliquent en
// même temps sans qu'aucune n'en bloque une autre.
func TestCA213_Degradation_CombinedScenario_AllFourRulesAtOnce(t *testing.T) {
	fb := newFakeBridge(t, "fffe0000deadbeef")
	fb.addLight("1", "Salle")
	fb.addLight("2", "AmpouleRouge")
	bleue := fb.addLight("3", "AmpouleBleue") // règle 4 : injoignable
	bleue.reachable = false
	bleue.putErr = &hueError{Type: 201, Description: "device is off"}
	// "Verts" (règles 1+2) : aucune ampoule configurée du tout.

	d := newTestDriver(t, fb, []LightSpec{
		{Name: "Salle", Role: RoleGeneral},
		{Name: "AmpouleRouge", Role: RoleTeam, Team: "Rouges"},
		{Name: "AmpouleBleue", Role: RoleTeam, Team: "Bleus"},
	}, nil)

	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{40, 90, 255}, Intensity: 160},
		{Zone: "Rouges", Color: [3]int{220, 20, 20}, Intensity: 255},
		{Zone: "Bleus", Color: [3]int{20, 20, 220}, Intensity: 255},
		{Zone: "Verts", Color: [3]int{20, 220, 20}, Intensity: 255}, // règle 3 partielle : pas de repli "tout general" ici (d'autres équipes SONT pourvues) — seule Verts n'a nulle part où aller
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatalf("aucune des 4 situations dégradées ne doit faire échouer Apply : %v", err)
	}

	// Règle 1+2 : "Verts" n'a jamais été écrite (aucun id ne lui correspond),
	// et n'a strictement aucune trace dans le rapport (pas de LightSpec).
	report := d.Status()
	for _, l := range report.Lights {
		if l.Name == "Verts" || l.Team == "Verts" {
			t.Fatalf("'Verts' ne doit apparaître nulle part dans le rapport, aucune ampoule ne lui est affectée : %+v", l)
		}
	}
	// 3 écritures tentées : Salle, AmpouleRouge, AmpouleBleue (jamais Verts,
	// qui n'a pas d'ampoule).
	if got := fb.writeCount(); got != 3 {
		t.Fatalf("3 écritures attendues (Salle, AmpouleRouge, AmpouleBleue), got %d", got)
	}
	// Règle 2+3 (ici combinée à 1) : general garde SA couleur, jamais celle
	// de Rouges, Bleus ou Verts.
	tw213AssertLightColor(t, fb, "1", "Salle (general)", [3]int{40, 90, 255}, 160)
	// L'équipe pourvue et joignable est rendue normalement.
	tw213AssertLightColor(t, fb, "2", "AmpouleRouge", [3]int{220, 20, 20}, 255)
	// Règle 4 : l'ampoule injoignable ne bloque ni general ni l'autre équipe
	// (déjà vérifié ci-dessus par les deux assertions précédentes qui
	// n'auraient pas pu réussir si Apply s'était arrêté en cours de route),
	// et porte sa propre erreur sans faire tomber les autres.
	var bleu LightStatus
	for _, l := range report.Lights {
		if l.Name == "AmpouleBleue" {
			bleu = l
		}
	}
	if bleu.LastError == "" {
		t.Error("l'ampoule d'équipe injoignable doit porter une erreur visible même dans ce scénario combiné")
	}
}
