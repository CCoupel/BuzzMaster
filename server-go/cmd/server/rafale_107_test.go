package main

import (
	"fmt"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Intégration — flux complet RAFALE à travers le VRAI dispatch
// (handleWebMessage), pas seulement les méthodes moteur unitaires déjà
// couvertes par internal/game/rafale_modes_test.go (les 4 modes, compteurs,
// meilleur score) et internal/game/rafale_test.go (pioche/pool/persistance
// du flag) — milestone v8.0.0 #16, issue #107, contrat contracts/rafale.md.
//
// Plan _work/reports/plan-20260828-161558.md, section "Tests Requis" :
//
//	Intégration (Go) — cmd/server/rafale_107_test.go
//	[ ] Flux complet START → questions → validations → fin de manche → TEAM_POINTS
//
// Ce que ce fichier prouve, que les tests moteur ne prouvent PAS : que les
// handlers WS (task 23 — handleRafaleValidate/handleRafaleInvalidate/
// handleRafaleSetTeams, main.go) sont réellement câblés dans le switch de
// handleWebMessage ET passent l'allow-list (task 19) pour un client admin —
// risque nommé du plan ("Action oubliée dans l'allow-list → rejet
// silencieux ... symptôme : le bouton ne fait rien, sans erreur visible").
// Un appel direct à e.RafaleValidate() (comme rafale_modes_test.go) ne
// peut PAS détecter un oubli dans inboundActionAllowlist ou dans le switch
// de handleWebMessage — seul un test passant par handleWebMessage le peut.
//
// Harnais : mêmes conventions que entracte_integration_test.go — dispatchAs
// (défini dans ce fichier-là, réutilisé tel quel ici, même package `main`)
// construit un IncomingMessage et l'envoie au VRAI handleWebMessage.
//
// ⚠️ Démarrage via StartImmediate, pas via un ACTION START dispatché : le
// dispatch réel de START (handleStart -> Engine.Start) traverse une phase
// COUNTDOWN réelle (3s pour RAFALE, aucune branche spéciale contrairement à
// MEMOTION/MEMORY — Start(), engine.go) avant d'appeler actualStart(). Ce
// délai est un comportement GÉNÉRIQUE du moteur (toute question passe par
// ce countdown), pas une particularité RAFALE, et n'est pas le risque que ce
// fichier cherche à couvrir — l'attendre ici ajouterait ~3s réelles à la
// suite pour zéro couverture supplémentaire. StartImmediate appelle le
// MÊME startRafaleRoundUnsafe() qu'actualStart() (engine.go, commentaire
// "mirrors actualStart(); StartImmediate bypasses actualStart entirely")
// — c'est exactement le patron déjà suivi par rafale_modes_test.go et
// cmd/server/awarded_teams_test.go (readyAndStart).
// ---------------------------------------------------------------------------

// setupRafaleIntegrationTestApp wires an App with 2 teams, a 10-question
// reservoir (HISTORY/1, largement assez pour ne jamais croiser
// ErrRafalePoolEmpty pendant ce test), et une question de configuration de
// manche CHACUN_SON_TOUR prête (PhaseReady n'est pas requis avant
// StartImmediate, qui bypass tous les gardes de phase — mais
// SetRafaleParticipatingTeams, appelé via dispatch juste après, exige
// PREPARE/READY : Ready() suffit).
func setupRafaleIntegrationTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	// STOP's broadcastStop() also fans out over UDP (legacy ESP32 v1
	// protocol compat) — without a real broadcaster here, that call panics
	// on a nil pointer; handleWebMessage recovers it silently (unrelated to
	// RAFALE), but it's needless log noise and masks a real regression in
	// broadcastStop if one ever appeared. Same fix as next_question_triggers_test.go.
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{
		"red":  {Name: "red", Color: []int{255, 0, 0}},
		"blue": {Name: "blue", Color: []int{0, 0, 255}},
	})

	for i := 1; i <= 10; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: fmt.Sprintf("r-%d", i), Question: fmt.Sprintf("Q%d", i), Answer: fmt.Sprintf("A%d", i),
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
		}
	}

	q := &game.Question{
		ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
		Points: "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleCategories:   []string{string(game.CategoryHistory)},
			RafaleDifficulty:   1,
			RafaleMode:         string(game.RafaleModeChacunSonTour),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready("rq1", q)

	return app
}

// TestRafaleIntegration_FullCycle_StartValidateInvalidateStopTeamPoints is
// the plan's "flux complet" line item, in one linear sequence: RAFALE_SET_
// TEAMS -> démarrage -> RAFALE_VALIDATE -> RAFALE_INVALIDATE -> STOP (fin de
// manche) -> TEAM_POINTS (action existante réutilisée, contrat §6.2 —
// AUCUNE action d'attribution dédiée n'est créée pour RAFALE). Toutes les
// actions "réelles" (celles qu'un client web enverrait) passent par
// dispatchAs -> handleWebMessage, pas par un appel direct à une méthode
// Engine.
func TestRafaleIntegration_FullCycle_StartValidateInvalidateStopTeamPoints(t *testing.T) {
	app := setupRafaleIntegrationTestApp(t)

	// --- RAFALE_SET_TEAMS via le VRAI dispatch (handleRafaleSetTeams, tâche 23) ---
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleSetTeams, protocol.RafaleSetTeamsPayload{Teams: []string{"red", "blue"}})

	state := app.engine.GetState()
	if len(state.RafaleParticipatingTeams) != 2 {
		t.Fatalf("RAFALE_SET_TEAMS via dispatch: expected 2 participating teams, got %v", state.RafaleParticipatingTeams)
	}
	if state.RafaleCurrentTeam != "red" {
		t.Fatalf("RAFALE_SET_TEAMS via dispatch: expected 'red' (first in TEAMS) to become current, got %q", state.RafaleCurrentTeam)
	}

	// --- Démarrage de la manche (StartImmediate — voir commentaire d'en-tête) ---
	app.engine.StartImmediate(0)

	state = app.engine.GetState()
	if state.Phase != game.PhaseStarted {
		t.Fatalf("expected PhaseStarted after round start, got %s", state.Phase)
	}
	if state.RafaleSubPhase != game.RafaleSubPhaseQuestion {
		t.Fatalf("expected RafaleSubPhaseQuestion after round start, got %q", state.RafaleSubPhase)
	}
	firstQuestionID := state.RafaleCurrentQuestion.ID
	if firstQuestionID == "" {
		t.Fatal("expected a question drawn from the reservoir on round start, got an empty RAFALE_CURRENT_QUESTION.ID")
	}

	// --- RAFALE_VALIDATE via le VRAI dispatch (handleRafaleValidate, tâche 23) ---
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleValidate, nil)

	state = app.engine.GetState()
	if got := state.RafaleTeamCounters["red"]; got != 1 {
		t.Fatalf("RAFALE_VALIDATE via dispatch: expected red counter=1, got %d (counters=%v)", got, state.RafaleTeamCounters)
	}
	if state.RafaleCurrentTeam != "blue" {
		t.Fatalf("CHACUN_SON_TOUR: expected rotation to 'blue' after red's correct answer, got %q", state.RafaleCurrentTeam)
	}
	secondQuestionID := state.RafaleCurrentQuestion.ID
	if secondQuestionID == "" || secondQuestionID == firstQuestionID {
		t.Fatalf("expected a NEW question to be posed after RAFALE_VALIDATE, got %q (was %q)", secondQuestionID, firstQuestionID)
	}

	// --- RAFALE_INVALIDATE via le VRAI dispatch (handleRafaleInvalidate, tâche 23) ---
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleInvalidate, nil)

	state = app.engine.GetState()
	if got := state.RafaleTeamCounters["blue"]; got != 0 {
		t.Fatalf("RAFALE_INVALIDATE via dispatch: expected blue counter to stay 0 (incorrect answer), got %d", got)
	}
	if state.RafaleCurrentTeam != "red" {
		t.Fatalf("expected rotation back to 'red' after blue's incorrect answer, got %q", state.RafaleCurrentTeam)
	}

	// --- Fin de manche : STOP manuel via le VRAI dispatch (contrat §7.1
	//     "STOP manuel" ; handleStop est une action EXISTANTE, sa branche
	//     RAFALE vit dans Engine.Stop(), engine.go) ---
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStop, nil)

	state = app.engine.GetState()
	if state.RafaleSubPhase != game.RafaleSubPhaseRoundEnd {
		t.Fatalf("expected RafaleSubPhaseRoundEnd after STOP, got %q", state.RafaleSubPhase)
	}

	// --- Attribution des points en fin de manche : TEAM_POINTS, action
	//     EXISTANTE réutilisée (contrat §6.2 : "Aucune action d'attribution
	//     de points n'est créée") ; valeur suggérée = compteur retenu ×
	//     barème de manche (Question.POINTS="10") — l'animateur reste libre
	//     de l'ajuster, ici on envoie exactement la valeur suggérée. ---
	suggestedPoints := state.RafaleTeamCounters["red"] * 10
	if suggestedPoints != 10 {
		t.Fatalf("sanity: expected red counter=1 x barème 10 = 10 suggested points, got %d", suggestedPoints)
	}
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "red", Points: suggestedPoints})

	redTeam := app.engine.GetTeam("red")
	if redTeam == nil {
		t.Fatal("expected team 'red' to still exist after TEAM_POINTS")
	}
	if redTeam.Score != suggestedPoints {
		t.Errorf("TEAM_POINTS via dispatch: expected red.Score=%d, got %d", suggestedPoints, redTeam.Score)
	}

	// Non-régression : blue, jamais crédité, reste à 0.
	blueTeam := app.engine.GetTeam("blue")
	if blueTeam == nil || blueTeam.Score != 0 {
		t.Errorf("expected blue.Score=0 (never credited), got %+v", blueTeam)
	}
}

// TestRafaleIntegration_ValidateInvalidate_RejectedFromTV is the allow-list
// half of the same risk (task 19): even though handleRafaleValidate is
// wired into the dispatch switch, a client type NOT in RAFALE_VALIDATE's
// allow-list entry (contract §5.1: admin+anim only) must never reach it —
// covered in isolation by internal/server/inbound_allowlist_rafale_test.go
// (IsActionAllowed, pure policy), reproduced here end-to-end through the
// real handleWebMessage to prove the gate is actually CONSULTED at dispatch
// time, not just correctly defined in the policy map.
func TestRafaleIntegration_ValidateInvalidate_RejectedFromTV(t *testing.T) {
	app := setupRafaleIntegrationTestApp(t)
	if err := app.engine.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("setup: SetRafaleParticipatingTeams failed: %v", err)
	}
	app.engine.StartImmediate(0)

	before := app.engine.GetState().RafaleTeamCounters["red"]

	// TV is not in RAFALE_VALIDATE's allow-list entry (admin+anim only).
	dispatchAs(t, app, server.ClientTypeTV, protocol.ActionRafaleValidate, nil)

	after := app.engine.GetState().RafaleTeamCounters["red"]
	if after != before {
		t.Errorf("RAFALE_VALIDATE from a TV client must be rejected by the allow-list (never reach the engine), counter changed %d -> %d", before, after)
	}
}
