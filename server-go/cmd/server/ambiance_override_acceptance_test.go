// Suite d'acceptance test-writer pour #208 (milestone v10.0.0, Batch 2,
// T2.4 item 3) — contracts/lighting.md §10.1 (sélecteur ON/AUTO/OFF + Flash)
// et §10.4 (extinction à l'arrêt), contre l'implémentation réelle de
// dev-backend (cmd/server/ambiance_override.go, commit 881db3c9).
//
// Complémentaire de ambiance_override_dev_test.go (dev-backend), qui couvre
// déjà en détail lightingOverrideGeneral en isolation (toutes les
// combinaisons AUTO/ON/OFF/Flash), la portée general-only face à OFF, la
// re-dérivation à travers le vrai writer (OFF puis retour explicite à AUTO),
// le blink Flash réel avec arrêt de goroutine, la validation de
// SetLightingMode, et l'extinction (buzzers + toutes les zones Hue
// configurées, y compris "aucun pilote configuré"). Ce fichier-ci n'en
// duplique aucun — il ajoute :
//
//   - CA208-1 : la tenue **face à de VRAIS événements de jeu qui se
//     succèdent** pendant qu'un mode ON/OFF est engagé — pas seulement un
//     changement de phase isolé suivi d'un retour AUTO explicite (ce que
//     couvre déjà TestDevLightingMode_ReDerivesThroughTheRealWriter). C'est
//     le garde-fou de non-régression le plus important de ce lot : le design
//     "annulation automatique au premier événement de jeu" a été rédigé PUIS
//     explicitement abandonné le même jour (contract §10.1, historique en
//     tête) — ce test échouerait si ce mécanisme abandonné revenait par
//     erreur dans le code.
//   - CA208-2 : AUTO **hors partie** ne doit jamais plonger la salle dans le
//     noir (contract §10.1.1 point 4) — non testé par dev-backend, qui
//     n'exerce AUTO qu'en cours de partie (retour à KindReady).
//   - CA208-3 : un garde-fou textuel sur (*App).stop() (main.go), à l'image
//     du pattern déjà en place dans ambiance_acceptance_test.go
//     (TestCA2_StartAmbianceLifecycleGuard_IsPresentInSource) — l'ordre
//     extinction-avant-cancelCtx() documenté par le contrat §10.4 et par le
//     commentaire du planner (le piège d'ordonnancement) n'a, à ce jour,
//     aucun test qui échouerait si l'ordre était inversé : les tests
//     existants appellent shutdownExtinguishHueLighting()/
//     sendLEDSetAllEntracteOff() directement, jamais (*App).stop() lui-même
//     (dont l'exécution réelle nécessite les hubs réseau qu'un test unitaire
//     ne monte pas — voir le commentaire de TestDevStop_TurnsBuzzersOffOnShutdown).
//   - CA208-4 : Flash engagé (pas seulement OFF, déjà couvert par
//     TestDevLightingMode_ScopedToGeneralZoneOnly) laisse aussi la zone
//     d'une équipe active intacte.
//
// Hors périmètre de ce fichier, par accord explicite avec le CDP
// (_work/handoff — confirmation du 2026-09-07) : la resynchronisation au
// retour du pont (contract §10.3) n'est pas encore câblée à ce commit —
// dev-backend ferme ce point séparément. Aucun test ci-dessous n'en dépend.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Toutes les aides sont préfixées tw208 pour ne jamais entrer en collision
// avec un nom déclaré ailleurs dans le paquet.
package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// ---------------------------------------------------------------------------
// CA208-1 — tenue indéfinie face à de vrais événements de jeu (contract
// §10.1.1 point 1 : "les événements de jeu ne le recouvrent pas, y compris
// pendant une partie").
// ---------------------------------------------------------------------------

func tw208NewWriterApp(t *testing.T) (*App, *lighting.FakeDriver, func()) {
	t.Helper()
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	go app.ambiance().Start(ctx)
	return app, fake, cancel
}

func TestCA208_Hold_ON_SurvivesRealGameEventsDuringASession(t *testing.T) {
	app, fake, cancel := tw208NewWriterApp(t)
	defer cancel()

	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA"},
		"m2": {Name: "m2", Team: "TeamB"},
	})

	app.setLightingMode(lightingModeOn)
	tw205WaitFor(t, 5*time.Second, func() bool {
		last, ok := fake.Last()
		return ok && last.Zones[0].Color == lightingOnColor && last.Zones[0].Intensity == lightingOnIntensity
	})

	// Une vraie partie qui avance — exactement les sites du registre §6 que
	// le contrat §10.1.2 (dans sa version un temps en vigueur puis
	// abandonnée) aurait branchés sur l'annulation automatique.
	steps := []func(){
		func() { app.engine.SetPhase(game.PhasePrepare); app.ambiance().NotifyState() },
		func() { app.engine.SetPhase(game.PhaseStarted); app.ambiance().NotifyState() },
		func() {
			app.engine.SetBumpers(map[string]*game.Bumper{
				"m1": {Name: "m1", Team: "TeamA", Time: 1000},
				"m2": {Name: "m2", Team: "TeamB"},
			})
			app.engine.SetPhase(game.PhasePaused)
			app.ambiance().NotifyState()
		},
		func() { app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamA"}, 3, 50*time.Millisecond) },
		func() {
			app.engine.SetPhase(game.PhaseReady) // ENTRACTE activation requires an eligible phase
			app.ambiance().NotifyState()
			if !app.engine.SetEntracte(true) {
				t.Fatal("setup invalide : activation ENTRACTE refusée en phase READY")
			}
			app.ambiance().NotifyState()
		},
	}
	for i, step := range steps {
		step()
		// Chaque étape doit se traduire par un nouvel Apply (le writer a
		// bien été notifié) — mais TOUJOURS dans l'état ON forcé.
		tw205WaitFor(t, 5*time.Second, func() bool {
			last, ok := fake.Last()
			return ok && last.Zones[0].Color == lightingOnColor && last.Zones[0].Intensity == lightingOnIntensity
		})
		if got := app.LightingMode(); got != "ON" {
			t.Fatalf("étape %d : le sélecteur a bougé tout seul (mode=%q) — l'annulation automatique abandonnée est-elle revenue ?", i, got)
		}
	}
}

func TestCA208_Hold_OFF_SurvivesRealGameEventsDuringASession(t *testing.T) {
	app, fake, cancel := tw208NewWriterApp(t)
	defer cancel()

	app.setLightingMode(lightingModeOff)
	tw205WaitFor(t, 5*time.Second, func() bool {
		last, ok := fake.Last()
		return ok && last.Zones[0].Intensity == 0
	})

	for i, phase := range []game.GamePhase{game.PhasePrepare, game.PhaseStarted, game.PhasePaused, game.PhaseRevealed, game.PhaseStopped} {
		app.engine.SetPhase(phase)
		app.ambiance().NotifyState()
		tw205WaitFor(t, 5*time.Second, func() bool {
			last, ok := fake.Last()
			return ok && last.Zones[0].Intensity == 0
		})
		if got := app.LightingMode(); got != "OFF" {
			t.Fatalf("étape %d (phase %s) : le sélecteur a bougé tout seul (mode=%q)", i, phase, got)
		}
	}
}

// ---------------------------------------------------------------------------
// CA208-2 — AUTO hors partie ne doit jamais éteindre la salle (contract
// §10.1.1 point 4).
// ---------------------------------------------------------------------------

func TestCA208_AutoOutsideGame_NeverGoesDark(t *testing.T) {
	app := newTestApp(t)
	// Aucune partie en cours : PhaseStopped (défaut du moteur de test) ⇒
	// deriveAmbianceEvent produit KindIdle (cmd/server/ambiance.go §6.2).
	ev := app.deriveAmbianceEvent()
	if ev.Kind != lighting.KindIdle {
		t.Fatalf("setup invalide : attendu KindIdle hors partie, got %s", ev.Kind)
	}

	// AUTO est la position par défaut, mais on la force explicitement pour
	// documenter l'intention du test plutôt que de compter sur le zéro-valeur.
	app.setLightingMode(lightingModeAuto)
	st := app.ambianceScene(ev)
	// Batch C/C1a (planner-v10-teamcolor-changes-20260908-092100.md §1) :
	// une ampoule d'équipe est désormais TOUJOURS présente, y compris hors
	// partie (newTestApp configure TeamA/TeamB/TeamC) — ce test porte sur la
	// zone 'general' uniquement (§10.1.1 point 4), pas sur le nombre total
	// de zones ; on la retrouve par son nom plutôt que de supposer qu'elle
	// est seule.
	var general lighting.ZoneState
	var found bool
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral {
			general, found = z, true
		}
	}
	if !found {
		t.Fatalf("aucune zone 'general' dans la scène, got %+v", st.Zones)
	}
	if general.Intensity == 0 {
		t.Fatalf("AUTO hors partie ne doit JAMAIS éteindre la salle (contract §10.1.1 point 4), got intensité 0")
	}
	if general.Color != ambianceSceneIdle.Color || general.Intensity != ambianceSceneIdle.Intensity {
		t.Fatalf("AUTO hors partie doit rendre la scène IDLE (blanc chaud praticable), got %+v want %+v", general, ambianceSceneIdle)
	}
}

// ---------------------------------------------------------------------------
// CA208-3 — extinction à l'arrêt AVANT cancelCtx() : garde-fou textuel sur
// (*App).stop() (contract §10.4), à l'image de
// TestCA2_StartAmbianceLifecycleGuard_IsPresentInSource
// (ambiance_acceptance_test.go). Ce test échoue si l'ordre est inversé.
// ---------------------------------------------------------------------------

func TestCA208_ShutdownExtinctionOrder_BeforeCancelCtx_SourceGuard(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué")
	}
	path := filepath.Join(filepath.Dir(thisFile), "main.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de %s : %v", path, err)
	}
	src := string(content)

	start := strings.Index(src, "func (a *App) stop() {")
	if start == -1 {
		t.Fatal("CA208-3 : func (a *App) stop() introuvable dans main.go — a-t-elle été renommée ?")
	}
	// Bornée par la déclaration de fonction suivante, au niveau top-level
	// ("\nfunc "), pour ne jamais déborder sur le reste du fichier.
	rest := src[start+len("func (a *App) stop() {"):]
	end := strings.Index(rest, "\nfunc ")
	if end == -1 {
		end = len(rest)
	}
	body := rest[:end]

	idxExtinguish := strings.Index(body, "a.shutdownExtinguishHueLighting()")
	idxBuzzerOff := strings.Index(body, "a.sendLEDSetAllEntracteOff()")
	// The bare substring "a.cancelCtx()" also appears inside the explanatory
	// comment ABOVE the extinction calls ("AVANT a.cancelCtx() — ..."), which
	// would make a naive strings.Index find that comment instead of the real
	// invocation and report a false inversion. Anchor on the actual call
	// site's unique surrounding guard instead — it appears nowhere else.
	idxCancel := strings.Index(body, "if a.cancelCtx != nil {")

	if idxExtinguish == -1 {
		t.Fatal("CA208-3 : (*App).stop() n'appelle plus a.shutdownExtinguishHueLighting() — extinction Hue à l'arrêt disparue (contract §10.4)")
	}
	if idxBuzzerOff == -1 {
		t.Fatal("CA208-3 : (*App).stop() n'appelle plus a.sendLEDSetAllEntracteOff() — extinction des buzzers à l'arrêt disparue (contract §10.4)")
	}
	if idxCancel == -1 {
		t.Fatal("CA208-3 : (*App).stop() ne garde plus 'if a.cancelCtx != nil {' — la garde ou l'appel a été réécrit, le test lui-même doit être révisé")
	}
	if idxExtinguish > idxCancel {
		t.Fatal("CA208-3 : a.shutdownExtinguishHueLighting() est appelée APRÈS a.cancelCtx() — piège d'ordonnancement du contract §10.4 : " +
			"a.ctx porte le client HTTP vers le pont, une extinction émise après cancelCtx() est annulée à l'instant où elle part")
	}
	if idxBuzzerOff > idxCancel {
		t.Fatal("CA208-3 : a.sendLEDSetAllEntracteOff() (extinction buzzers) est appelée APRÈS a.cancelCtx() — piège d'ordonnancement du contract §10.4")
	}
}

// ---------------------------------------------------------------------------
// CA208-4 — Flash engagé laisse la zone d'une équipe active intacte
// (contract §10.1, "Portée" — complète TestDevLightingMode_ScopedToGeneralZoneOnly,
// qui ne teste que OFF, pas Flash).
// ---------------------------------------------------------------------------

func TestCA208_Flash_NeverAffectsAnActiveTeamZone(t *testing.T) {
	app := newTestApp(t)
	app.setLightingFlash(true)
	defer app.setLightingFlash(false)
	app.lightingFlashPhaseOn.Store(true) // phase allumée du blink, déterministe pour ce test

	ev := lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}}
	st := app.ambianceScene(ev)
	var general, teamA lighting.ZoneState
	var teamAFound bool
	for _, z := range st.Zones {
		switch z.Zone {
		case lighting.ZoneGeneral:
			general = z
		case "TeamA":
			teamA, teamAFound = z, true
		}
	}
	if general.Color != lightingFlashColor || general.Intensity != lightingOnIntensity {
		t.Fatalf("Flash doit forcer la zone general, got %+v", general)
	}
	if !teamAFound {
		t.Fatal("la zone TeamA a disparu alors que Flash est engagé — elle doit rester présente et auto-dérivée")
	}
	// Batch A/P1 (planner _work/reports/planner-v10-groups-teamcolor-
	// 20260907-173831.md) : une équipe distinguée par l'événement (ici,
	// TeamA via ev.Teams) est désormais à pleine intensité (255,
	// l'équivalent SOLID/BLINK du buzzer) dans SA zone dédiée, pas
	// ambianceSceneTeamTurn.Intensity (200, qui reste la valeur de la zone
	// general elle-même — inchangée, non affectée par ce fix).
	wantTeamA := app.teamNameToRGB("TeamA")
	if teamA.Color != wantTeamA || teamA.Intensity != 255 {
		t.Fatalf("Flash sur general ne doit jamais affecter la zone d'une équipe active, got %+v want couleur=%v intensité=255",
			teamA, wantTeamA)
	}
}
