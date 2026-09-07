package main

// Regression test for T2.1 (milestone v10.0.0, reprise du 2026-09-07) —
// _work/handoff/task-dev-backend-batch2-20260907.md.
//
// Two bugs, one root cause: a programmed ENTRACTE question (#214) raises
// GameState.Entracte from INSIDE the engine's own actualStart()/
// StartImmediate() transition (startEntracteQuestionUnsafe,
// internal/game/engine.go) — a transition that carries NO LED call of its
// own, so it is structurally invisible to the AST exhaustiveness test
// (contract lighting.md §10.5). Before this fix:
//
//   - Bug R1: nothing notified the ambiance writer when the countdown
//     actually ENDED (broadcastStart() — the only NotifyState() site of the
//     start sequence, contract §6 — fires only when the countdown BEGINS).
//     The room kept showing the previous READY/RUNNING scene instead of
//     KindEntracte.
//   - Bug #2 (asymmetry): the manual voie (ENTRACTE_SET, handleEntracteSet)
//     explicitly turns buzzer LEDs off via sendLEDSetAllEntracteOff() on
//     activation. The programmed voie never called it at all.
//
// Fix: cmd/server/main.go's onPhaseStarted() (wired from
// setupCallbacks/OnStateChange for every transition landing in
// PhaseStarted) now calls sendLEDSetAllEntracteOff() when the engine reports
// an active Entracte, and unconditionally calls a.ambiance().NotifyState().
//
// This test drives the REAL production wiring (setupCallbacks(), not a
// direct call to onPhaseStarted()) so a regression that un-wires the fix
// would be caught, not just a regression in onPhaseStarted() itself.

import (
	"context"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/server"
)

func TestEntracteProgrammed_AmbianceNotifiedAndBuzzersOff_T21(t *testing.T) {
	app := newTestAppWithHub(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.setupCallbacks() // real production wiring — this is what must be exercised

	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)

	app.engine.SetBumpers(map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA"},
	})

	cfg := game.EntracteConfig{Title: "PAUSE", Subtitle: "Retour dans 5mn", PanelSize: 60, AnimPeriod: 5, AnimIntensity: 10, TransitionMs: 500}
	q := &game.Question{
		ID:   "e1",
		Type: game.QuestionTypeEntracte,
		TypedContent: game.TypedContent{
			EntracteConfig: &cfg,
		},
	}
	app.engine.Ready(q.ID, q)
	// StartImmediate mirrors actualStart() exactly for this transition (same
	// startEntracteQuestionUnsafe() call, same callback(PhaseStarted) at the
	// end) — the established pattern for exercising the "end of countdown"
	// transition without waiting on the real 1s countdown ticker (see
	// internal/game/entracte_programme_214_test.go).
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	if !app.engine.IsEntracte() {
		t.Fatal("setup invalide : Entracte non levé après StartImmediate d'une question ENTRACTE — le test ne teste rien")
	}

	// Bug #2 — symmetry with the manual voie (handleEntracteSet): buzzers
	// must be turned off through sendLEDSetAllEntracteOff.
	payload, ok := app.bumperLEDState["m1"]
	if !ok {
		t.Fatal("ENTRACTE programmée : aucun LED_SET envoyé au buzzer — sendLEDSetAllEntracteOff() n'a pas été appelé (asymétrie avec la voie manuelle)")
	}
	if payload.Intensity != 0 || payload.Color != [3]int{0, 0, 0} {
		t.Fatalf("ENTRACTE programmée doit éteindre les buzzers (même payload OFF que handleEntracteSet), got %+v", payload)
	}

	// Bug R1 — the ambiance writer must have been notified at the end of the
	// countdown and re-derive KindEntracte (contract §8: warm white, salle
	// praticable, buzzers noirs — divergence délibérée).
	waitForCount(t, fake, 1)
	last, ok := fake.Last()
	if !ok {
		t.Fatal("l'écrivain d'ambiance n'a reçu aucun état — NotifyState() n'a pas été appelé à la fin du décompte")
	}
	if len(last.Zones) != 1 || last.Zones[0].Zone != lighting.ZoneGeneral {
		t.Fatalf("scène attendue sur la seule zone 'general', got %+v", last)
	}
	if last.Zones[0].Color != ambianceWarmWhite || last.Zones[0].Intensity != 100 {
		t.Fatalf("scène KindEntracte attendue (blanc chaud/100, salle praticable), got %+v", last.Zones[0])
	}
}

// TestEntracteProgrammed_OrdinaryStartAlsoNotifiesAmbiance is R1's OTHER
// half: the fix is not entracte-specific — ANY transition into PhaseStarted
// must notify the ambiance writer (READY/RUNNING scene switch), including
// a question with no ENTRACTE involved at all (broadcastStart() only fires
// when the countdown BEGINS, contract §6 — not when it ends).
func TestEntracteProgrammed_OrdinaryStartAlsoNotifiesAmbiance(t *testing.T) {
	app := newTestAppWithHub(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.setupCallbacks()

	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)

	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	waitForCount(t, fake, 1)
	last, ok := fake.Last()
	if !ok {
		t.Fatal("l'écrivain d'ambiance n'a reçu aucun état à la fin du décompte pour une question normale")
	}
	if last.Zones[0].Color != ambianceSceneRunning.Color || last.Zones[0].Intensity != ambianceSceneRunning.Intensity {
		t.Fatalf("scène KindRunning attendue à la fin du décompte, got %+v", last.Zones[0])
	}

	// Not an entracte: no LED was sent to any buzzer by onPhaseStarted itself
	// (sendLEDSetAllEntracteOff must NOT fire for an ordinary question).
	if _, ok := app.bumperLEDState["m1"]; ok {
		t.Fatal("aucun buzzer configuré dans ce test — un LED_SET inattendu a été envoyé")
	}
}
