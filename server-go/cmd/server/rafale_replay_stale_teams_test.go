package main

// ---------------------------------------------------------------------------
// Régression — retour QUALIF, 3e cycle d'investigation (#199) : "je peux
// encore lancer un START alors que je n'ai pas défini d'équipe".
//
// Chronologie de l'investigation (voir _work/reports/ pour le détail
// complet de chaque cycle) :
//   1. SHA 393c6dc7 — garde participantsConform (CATEGORY/DIFFICULTY/équipe
//      en mode multi). Reviewée, QA validée.
//   2. Retour QUALIF 8.0.0.14 : symptôme persiste. SHA 6b995229 — audit
//      exhaustif des 3 sites Phase=PhaseReady (tous gardés), repro ANIM
//      réelle négative, hypothèse retenue : RAFALE_MODE périmé (question
//      sauvegardée avant le fix http.go 8.0.0.11, jamais re-sauvegardée).
//   3. L'utilisateur a rouvert ET re-sauvegardé sa question, retesté :
//      symptôme TOUJOURS présent. Hypothèse #2 invalidée.
//   4. (CE FICHIER) Cause réelle trouvée et confirmée par reproduction
//      directe (internal/game/rafale_modes_test.go,
//      TestReady_RafaleReplay_ResetsStaleParticipatingTeams) : Ready() ne
//      réinitialisait RAFALE_PARTICIPATING_TEAMS/RAFALE_CURRENT_TEAM que
//      quand isNewQuestion==true (ID différent) — jamais quand la MÊME
//      question RAFALE est rejouée pour une nouvelle manche (même ID,
//      pattern d'usage NORMAL de RAFALE, contrairement à QCM/MEMORY/
//      MEMOTION). Stop() ne touche jamais ces champs non plus (par
//      conception). La sélection d'équipes de la manche PRÉCÉDENTE restait
//      donc en mémoire côté serveur et satisfaisait silencieusement
//      participantsConform à la manche SUIVANTE — alors que l'interface
//      affichait "aucune équipe sélectionnée" (état local frontend,
//      déconnecté du GameState réel, ce qui explique pourquoi le symptôme
//      "sentait" comme un bug malgré une garde backend correcte en
//      isolation). Corrigé dans engine.go (Ready()).
//
// Ce fichier reproduit le scénario EXACT demandé par le CDP à travers le
// VRAI dispatch WS (handleWebMessage, ClientTypeAnim — pas un appel direct
// Engine comme rafale_modes_test.go), de bout en bout : lancer une manche
// multi AVEC équipes, l'arrêter, recharger la MÊME question RAFALE, tenter
// START SANS resélectionner — doit être refusé.
// ---------------------------------------------------------------------------

import (
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

func TestRafaleReplay_AnimDispatch_StaleTeamsFromPreviousManche_StartRefused(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}},
	})
	for i := 1; i <= 20; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "r-" + string(rune('a'+i)), Question: "Q", Answer: "A",
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: %v", err)
		}
	}
	makeRoundQuestion := func() *game.Question {
		return &game.Question{
			ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
			Category: game.CategoryHistory, Points: "10", Time: "120",
			TypedContent: game.TypedContent{
				RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeChacunSonTour),
				RafaleQuestionTime: 3, RafaleMaxQuestions: 100,
			},
		}
	}

	// --- Manche 1 : équipes sélectionnées, jouée jusqu'au bout, arrêtée. ---
	app.engine.Ready("rq1", makeRoundQuestion())
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleSetTeams, protocol.RafaleSetTeamsPayload{Teams: []string{"red", "blue"}})
	app.engine.ForceReady()
	if state := app.engine.GetState(); state.Phase != game.PhaseReady {
		t.Fatalf("sanity: expected READY for manche 1, got %s", state.Phase)
	}
	app.engine.StartImmediate(30)
	if state := app.engine.GetState(); state.Phase != game.PhaseStarted {
		t.Fatalf("sanity: expected STARTED for manche 1, got %s", state.Phase)
	}
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStop, struct{}{})
	if state := app.engine.GetState(); state.Phase != game.PhaseStopped {
		t.Fatalf("sanity: expected STOPPED after ending manche 1, got %s", state.Phase)
	}

	// --- Manche 2 : MÊME question RAFALE rechargée, AUCUNE resélection
	// d'équipe — le scénario réaliste rapporté par l'utilisateur. ---
	app.engine.Ready("rq1", makeRoundQuestion())
	if state := app.engine.GetState(); len(state.RafaleParticipatingTeams) != 0 {
		t.Fatalf("BUG: RAFALE_PARTICIPATING_TEAMS still has %v after replaying the same question (manche 1's stale selection)", state.RafaleParticipatingTeams)
	}

	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionStart, protocol.StartPayload{Delay: 30})

	state := app.engine.GetState()
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("BUG: manche 2 started (phase=%s) via ANIM dispatch without reselecting any team — stale selection from manche 1", state.Phase)
	}
	if state.Phase != game.PhasePrepare {
		t.Errorf("expected the engine to stay in PREPARE for manche 2, got %s", state.Phase)
	}
}
