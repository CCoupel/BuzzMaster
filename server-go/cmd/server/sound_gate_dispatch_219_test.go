// Suite test-writer pour l'addendum v11.1 « Média indisponible = lancement
// bloqué » (#219/#236/#237, plan _work/reports/plan-20260923-101500.md
// rév. 8 §6 tâche 12, contract sound.md §10.8) : la gate T0 vérifiée par
// dispatch WebSocket RÉEL (handleWebMessage), patron
// cmd/server/rafale_multiteam_start_gate_test.go — reprend son helper
// partagé dispatchAs (entracte_integration_test.go, même paquet).
//
//   - CA18/CA20 : START dispatché sans FORCE sur une question devenue
//     indisponible ENTRE l'affichage et le clic est refusé (la fenêtre de
//     course se ferme dans handleStart lui-même, via
//     refreshQuestionSoundAvailability() juste avant Engine.Start()).
//   - CA22 : FORCE_READY (Ctrl+clic simulé) débloque, puis START démarre
//     réellement, sans son.
//   - CA27 : FORCE_READY sur une question aux participants non conformes
//     reste refusé — le point le plus critique de ce lot (arbitrage #172 B5).
//
// Convention de collision : préfixe tw219d pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// tw219dReadySpeedyWithSound brings a fresh app to PhaseReady for a SPEEDY
// question carrying a sound, sound genuinely enabled/available at that
// point (fake player, no error) — the realistic "available at display
// time" precondition CA20's race window starts from.
func tw219dReadySpeedyWithSound(t *testing.T) (*App, *game.Question) {
	t.Helper()
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster() // broadcastReady/broadcastStart need a non-nil (unstarted is fine) broadcaster
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}
	tw219qSetSoundEnabled(true)

	q := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1234.wav"}
	app.engine.Ready(q.ID, q)
	app.engine.TransitionToReady()
	if app.engine.GetState().Phase != game.PhaseReady {
		t.Fatalf("setup invalide : TransitionToReady() n'a pas atteint PhaseReady, got %s", app.engine.GetState().Phase)
	}
	return app, q
}

// ---------------------------------------------------------------------------
// CA18/CA20 — START refusé, dispatch réel, y compris la fenêtre de course.
// ---------------------------------------------------------------------------

func TestDispatch_START_Refused_WhenMediaBecomesUnavailable_CA18_CA20(t *testing.T) {
	app, _ := tw219dReadySpeedyWithSound(t)

	// La fenêtre de course (CA20) : le bouton était affiché actif
	// (PhaseReady ci-dessus, calculé alors que le son était disponible),
	// puis l'indisponibilité survient JUSTE avant le clic — sans repasser
	// par l'UI, uniquement en changeant l'état serveur.
	tw219qSetSoundEnabled(false)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 20})

	state := app.engine.GetState()
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("CA18/CA20 violés : START a réussi (phase=%s) alors que le média est devenu indisponible avant le clic", state.Phase)
	}
	if state.Phase != game.PhasePrepare {
		t.Errorf("la question doit revenir en PREPARE (refreshQuestionSoundAvailability à l'intérieur de handleStart), got phase=%s", state.Phase)
	}
	if state.QuestionSoundUnavailable != "DISABLED" {
		t.Errorf("QUESTION_SOUND_UNAVAILABLE doit refléter la cause réelle (DISABLED), got %q", state.QuestionSoundUnavailable)
	}
}

func TestDispatch_START_Refused_AnimClientType_CA18(t *testing.T) {
	// Même scénario que le test RAFALE de référence
	// (rafale_multiteam_start_gate_test.go) : reproduit explicitement depuis
	// /anim (ClientTypeAnim), pas seulement /admin — les deux surfaces
	// partagent la même allow-list ActionStart:{Admin,Anim}.
	app, _ := tw219dReadySpeedyWithSound(t)
	tw219qSetSoundEnabled(false)

	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionStart, protocol.StartPayload{Delay: 20})

	state := app.engine.GetState()
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("BUG : START dispatché depuis /anim a réussi (phase=%s) sur une question à média indisponible", state.Phase)
	}
	if state.Phase != game.PhasePrepare {
		t.Errorf("attendu PhasePrepare, got %s", state.Phase)
	}
}

// ---------------------------------------------------------------------------
// CA22 — FORCE_READY débloque, puis START démarre réellement sans son.
// ---------------------------------------------------------------------------

func TestDispatch_ForceReady_ThenStart_CA22(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster() // broadcastReady/broadcastStart need a non-nil (unstarted is fine) broadcaster
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}
	tw219qSetSoundEnabled(false) // indisponible dès le départ

	q := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1234.wav"}
	app.engine.Ready(q.ID, q)
	app.engine.SetQuestionSoundUnavailable("DISABLED") // simule le rafraîchissement déjà survenu (un PONG antérieur)
	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Fatalf("setup invalide : devrait être bloquée en PREPARE, got %s", app.engine.GetState().Phase)
	}

	// D'abord, un START simple doit rester refusé (CA18, confirmé une
	// deuxième fois dans ce scénario précis avant le contournement).
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 20})
	if state := app.engine.GetState(); state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("setup invalide : START a réussi avant même FORCE_READY, phase=%s", state.Phase)
	}

	// Le Ctrl+clic simulé — ADMIN seul (ActionForceReady n'est pas dans
	// l'allow-list anim, vérifié séparément ci-dessous).
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionForceReady, nil)

	if app.engine.GetState().Phase != game.PhaseReady {
		t.Fatalf("CA22 violé : FORCE_READY doit faire passer la question en READY, got phase=%s", app.engine.GetState().Phase)
	}

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 20})

	state := app.engine.GetState()
	if state.Phase != game.PhaseCountdown && state.Phase != game.PhaseStarted {
		t.Errorf("CA22 violé : après FORCE_READY, START doit réellement démarrer la question (sans son) — got phase=%s", state.Phase)
	}
}

func TestDispatch_ForceReady_RefusedFromAnim_CA26(t *testing.T) {
	// CA26 : /anim n'offre aucune échappatoire — ActionForceReady est
	// réservé à l'admin dans l'allow-list entrante
	// (internal/server/inbound_allowlist.go:85, cité par le plan §3.3).
	app, _ := tw219dReadySpeedyWithSound(t)
	tw219qSetSoundEnabled(false)
	app.engine.SetQuestionSoundUnavailable("DISABLED")
	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Fatalf("setup invalide : devrait être bloquée en PREPARE, got %s", app.engine.GetState().Phase)
	}

	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionForceReady, nil)

	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Errorf("CA26 violé : FORCE_READY dispatché depuis /anim ne doit jamais débloquer une question — got phase=%s (l'allow-list doit le refuser avant même d'atteindre le moteur)", app.engine.GetState().Phase)
	}
}

// ---------------------------------------------------------------------------
// CA27 — le point le plus critique : FORCE_READY sur des participants non
// conformes reste refusé, même avec un son également indisponible.
// ---------------------------------------------------------------------------

func TestDispatch_ForceReady_Refused_ParticipantsNonConform_CA27(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster() // broadcastReady/broadcastStart need a non-nil (unstarted is fine) broadcaster
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}
	tw219qSetSoundEnabled(false)

	// MEMORY SOLO, son indisponible ET aucune équipe sélectionnée — les
	// deux gates sont réunies à dessein, exactement le scénario de la
	// recette manuelle (tests/procedures/question-sound-219.md, Scénario
	// 15 étape 5).
	q := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, Answer: "x", Sound: "/question/q1/sound_1234.wav",
		TypedContent: game.TypedContent{MemoryMode: "SOLO"}}
	app.engine.Ready(q.ID, q)
	app.engine.SetQuestionSoundUnavailable("DISABLED")
	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Fatalf("setup invalide : devrait être bloquée en PREPARE, got %s", app.engine.GetState().Phase)
	}

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionForceReady, nil)

	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Fatalf("CA27 violé (arbitrage #172 B5, POINT LE PLUS CRITIQUE DE CE LOT) : FORCE_READY dispatché en réel ne doit JAMAIS faire passer en READY une question MEMORY SOLO sans équipe, même avec un son indisponible — got phase=%s", app.engine.GetState().Phase)
	}

	// Un START qui suivrait quand même doit rester refusé lui aussi —
	// défense en profondeur du même bug #172.
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 20})
	if state := app.engine.GetState(); state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("CA27 violé : START a réussi (phase=%s) après un FORCE_READY resté sans effet — double violation", state.Phase)
	}
}

func TestDispatch_ForceReady_Refused_ParticipantsNonConform_SoundAvailable_CA27(t *testing.T) {
	// Variante : le son EST disponible cette fois (seule la gate
	// participants bloque) — vérifie que CA27 ne dépend pas d'une
	// coïncidence avec la gate son, mais que la branche participants du
	// switch est réellement, indépendamment, toujours appliquée.
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster() // broadcastReady/broadcastStart need a non-nil (unstarted is fine) broadcaster
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}
	tw219qSetSoundEnabled(true)

	q := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, Answer: "x",
		TypedContent: game.TypedContent{MemoryMode: "SOLO"}} // pas de son du tout ici
	app.engine.Ready(q.ID, q)
	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Fatalf("setup invalide : devrait être en PREPARE (participants non conformes), got %s", app.engine.GetState().Phase)
	}

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionForceReady, nil)

	if app.engine.GetState().Phase != game.PhasePrepare {
		t.Errorf("CA27 (non-régression #172, pré-existante) : une question MEMORY SOLO sans équipe et SANS SON reste refusée par FORCE_READY — got phase=%s", app.engine.GetState().Phase)
	}
}
