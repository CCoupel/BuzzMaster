// Suite d'acceptance test-writer pour le correctif "couleur d'ampoule
// d'équipe" (Batch A, A2), milestone v10.0.0, retour QUALIF round 5.
// Diagnostic normatif : `_work/reports/planner-v10-groups-teamcolor-20260907-173831.md`
// §Problème 1. Contrat inchangé (hue-bridge.md §5.2 était déjà correct) :
// c'est `ambianceScene()` (cmd/server/ambiance.go) qui doit rejoindre le
// contrat, pas l'inverse — dev-backend corrige en parallèle (A1).
//
// Modèle transposé du rendu buzzer (sendLEDSetForBuzzerNormal, main.go),
// cité par le rapport : LA COULEUR PORTE L'IDENTITÉ ET NE CHANGE JAMAIS ;
// L'ÉTAT DU JEU NE MODULE QUE L'INTENSITÉ. Une ampoule de rôle `team` rend
// donc TOUJOURS la couleur de son équipe tant que celle-ci est au plateau
// (`Engine.GetTeamsAndBumpersSnapshot().Teams`), avec :
//   - intensité PLEINE (= l'intensité de la scène courante, def.Intensity —
//     même valeur que ce que la zone general affiche déjà pour un événement
//     UseTeamColor à équipe unique, contract §8) quand CETTE équipe est
//     distinguée par l'événement ;
//   - intensité ATTÉNUÉE (dimIntensityFor(sa couleur), main.go — même seuil
//     que les buzzers, pas un second seuil inventé) sinon ;
//   - aucune zone du tout si l'équipe n'est pas/plus au plateau — elle
//     retombe alors dans `general` (hue-bridge.md §5.2), et donc dans le
//     périmètre du sélecteur ON/AUTO/OFF (contract lighting.md §10.1).
//
// Complémentaire de TestDevAmbianceTeamZones213 (ambiance_dev_test.go, #213)
// et TestDevLightingMode_ScopedToGeneralZoneOnly (ambiance_override_dev_test.go,
// #208), qui pinnent le comportement AVANT ce correctif (une zone d'équipe
// n'apparaît que si `ev.Teams` la nomme) — ce fichier-ci pinne le
// comportement APRÈS. Ces deux tests dev-backend devront être mis à jour
// par lui pour refléter la lecture correcte de §5.2 ; ce n'est pas à
// test-writer de les modifier (non-régression), seulement de documenter
// l'attente ici pour la relecture PR/DONE.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Aides préfixées twTC (Team Colour) pour ne jamais entrer en collision.
package main

import (
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

func twTCSetRoster(t *testing.T, app *App, teams map[string]*game.Team) {
	t.Helper()
	app.engine.SetTeams(teams)
}

func twTCZonesByName(app *App, ev lighting.Event) map[string]lighting.ZoneState {
	st := app.ambianceScene(ev)
	out := make(map[string]lighting.ZoneState, len(st.Zones))
	for _, z := range st.Zones {
		out[z.Zone] = z
	}
	return out
}

// TestTeamColor_BulbKeepsOwnColour_WhenNoTeamDistinguishedByEvent — point 1
// de la tâche : RUNNING/READY/PAUSE_ALL ont `ev.Teams` vide, mais les
// équipes sont bien au plateau. Chaque ampoule d'équipe doit montrer SA
// PROPRE couleur, jamais celle de `general` (le bug constaté en QUALIF).
func TestTeamColor_BulbKeepsOwnColour_WhenNoTeamDistinguishedByEvent(t *testing.T) {
	app := newTestApp(t)
	twTCSetRoster(t, app, map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
	})
	wantTeamA := app.teamNameToRGB("TeamA")
	wantTeamB := app.teamNameToRGB("TeamB")

	for _, ev := range []lighting.Event{
		{Kind: lighting.KindRunning},
		{Kind: lighting.KindReady},
		{Kind: lighting.KindPauseAll},
	} {
		zones := twTCZonesByName(app, ev)
		general, teamA, teamB := zones[lighting.ZoneGeneral], zones["TeamA"], zones["TeamB"]
		if teamA.Zone == "" || teamB.Zone == "" {
			t.Fatalf("%s : une ampoule d'équipe au plateau doit toujours avoir sa propre zone, got %+v", ev.Kind, zones)
		}
		if teamA.Color != wantTeamA {
			t.Errorf("%s : ampoule TeamA doit rester sur SA couleur, got %v want %v", ev.Kind, teamA.Color, wantTeamA)
		}
		if teamB.Color != wantTeamB {
			t.Errorf("%s : ampoule TeamB doit rester sur SA couleur, got %v want %v", ev.Kind, teamB.Color, wantTeamB)
		}
		// Le bug constaté en QUALIF : l'ampoule affichait la couleur de la
		// salle. Sanity explicite, pas seulement l'égalité positive ci-dessus.
		if teamA.Color == general.Color && general.Color != wantTeamA {
			t.Errorf("%s : ampoule TeamA a pris la couleur de general (%v) au lieu de la sienne", ev.Kind, general.Color)
		}
		if teamB.Color == general.Color && general.Color != wantTeamB {
			t.Errorf("%s : ampoule TeamB a pris la couleur de general (%v) au lieu de la sienne", ev.Kind, general.Color)
		}
	}
}

// TestTeamColor_DistinguishedTeamFullIntensity_OthersAttenuated — points 2
// et 3 de la tâche, réunis : quand un événement distingue UNE équipe
// (buzz, tour actif, bonne réponse, créditée), elle passe en intensité
// pleine (celle de la scène — def.Intensity, même valeur que general pour
// un événement à équipe unique, cf. TestDevAmbianceTeamZones213) tandis que
// les AUTRES équipes du plateau restent sur leur couleur, mais atténuées
// via dimIntensityFor — jamais une seconde formule d'atténuation.
func TestTeamColor_DistinguishedTeamFullIntensity_OthersAttenuated(t *testing.T) {
	app := newTestApp(t)
	twTCSetRoster(t, app, map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
	})
	wantTeamB := app.teamNameToRGB("TeamB")
	wantDimB := dimIntensityFor(wantTeamB)

	// "Pleine" intensité = 255, FIXE — l'équivalent salle du SOLID/BLINK des
	// buzzers (rapport §Problème 1, table), PAS l'intensité propre de la
	// scène courante (ambianceSceneTeamTurn.Intensity vaut 200, par
	// exemple : la salle et l'ampoule d'équipe distinguée n'ont aucune
	// raison de partager la même valeur — seule leur couleur est liée par
	// la palette commune, jamais leur intensité).
	const twTCFullIntensity = 255

	tests := []struct {
		name string
		ev   lighting.Event
	}{
		{"BUZZ (TeamA a buzzé)", lighting.Event{Kind: lighting.KindBuzz, Teams: []string{"TeamA"}}},
		{"TEAM_TURN (tour de TeamA)", lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}}},
		{"REVEAL (TeamA a bien répondu, seule)", lighting.Event{Kind: lighting.KindReveal, Teams: []string{"TeamA"}}},
		{"SCORE (TeamA créditée)", lighting.Event{Kind: lighting.KindScore, Teams: []string{"TeamA"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones := twTCZonesByName(app, tt.ev)
			teamA, teamB := zones["TeamA"], zones["TeamB"]
			if teamA.Zone == "" || teamB.Zone == "" {
				t.Fatalf("les deux ampoules d'équipe doivent avoir une zone, got %+v", zones)
			}
			if teamA.Intensity != twTCFullIntensity {
				t.Errorf("TeamA (distinguée) doit être en intensité pleine (%d), got %d", twTCFullIntensity, teamA.Intensity)
			}
			if teamB.Intensity != wantDimB {
				t.Errorf("TeamB (non distinguée) doit être atténuée via dimIntensityFor (%d), got %d", wantDimB, teamB.Intensity)
			}
			// L'atténuation ne doit JAMAIS toucher la teinte — seulement
			// l'intensité (c'est tout le sens du correctif).
			if teamB.Color != wantTeamB {
				t.Errorf("TeamB atténuée doit garder SA couleur, got %v want %v", teamB.Color, wantTeamB)
			}
		})
	}
}

// TestTeamColor_TeamOffTheBoard_FallsBackToGeneral_ReachedByOff — point 4 :
// une équipe absente du plateau (roster vide — "hors partie") ne doit
// produire AUCUNE zone dédiée : elle retombe dans `general`, et le
// sélecteur ON/AUTO/OFF (contract §10.1) l'atteint donc pleinement — à
// l'inverse de TestDevLightingMode_ScopedToGeneralZoneOnly (équipe AU
// plateau), qui montre qu'un OFF ne l'atteint PAS dans ce cas.
func TestTeamColor_TeamOffTheBoard_FallsBackToGeneral_ReachedByOff(t *testing.T) {
	app := newTestApp(t)
	twTCSetRoster(t, app, map[string]*game.Team{}) // aucune équipe au plateau

	for _, ev := range []lighting.Event{
		{Kind: lighting.KindIdle},
		{Kind: lighting.KindReady},
		{Kind: lighting.KindRunning},
	} {
		st := app.ambianceScene(ev)
		if len(st.Zones) != 1 || st.Zones[0].Zone != lighting.ZoneGeneral {
			t.Fatalf("%s sans équipe au plateau : une seule zone 'general' attendue, got %+v", ev.Kind, st.Zones)
		}
	}

	// KindIdle est "hors partie" même quand le roster n'a PAS été vidé (une
	// partie précédente a laissé des équipes configurées) : aucune partie en
	// cours ⇒ aucun plateau ⇒ aucune zone d'équipe, quel que soit le roster.
	twTCSetRoster(t, app, map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
	})
	st := app.ambianceScene(lighting.Event{Kind: lighting.KindIdle})
	if len(st.Zones) != 1 || st.Zones[0].Zone != lighting.ZoneGeneral {
		t.Fatalf("KindIdle doit retomber sur 'general' même avec un roster non vide, got %+v", st.Zones)
	}
	twTCSetRoster(t, app, map[string]*game.Team{}) // reset pour la suite du test

	// Le sélecteur OFF doit donc éteindre TOUT (aucune ampoule d'équipe ne
	// le protège, faute d'équipe au plateau) — lighting.md §10.1, nuance du
	// 2026-09-07 sur les installations sans équipe active.
	app.setLightingMode(lightingModeOff)
	st = app.ambianceScene(lighting.Event{Kind: lighting.KindRunning})
	if len(st.Zones) != 1 || st.Zones[0].Intensity != 0 {
		t.Fatalf("OFF sans équipe au plateau doit éteindre la seule zone existante (general), got %+v", st.Zones)
	}
}
