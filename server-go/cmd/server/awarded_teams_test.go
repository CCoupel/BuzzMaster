package main

import (
	"buzzcontrol/internal/game"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #170 — AWARDED_TEAMS projection (T1, TEST CENTRAL DU LOT).
//
// getAwardedTeamsPayload is a pure projection of Engine.GetHistory()'s
// POINTS_AWARDED events for the CURRENT question — no new GameState field.
// See its doc comment in main.go for the exact filter/grouping rule; these
// tests are derived directly from contracts/websocket-actions.md
// §"Animateur" AWARDED_TEAMS and from the plan's R1/R2/R3 risk table
// (_work/reports/plan-20260816-125123.md §5.6):
//   - R1: the lock is the PRESENCE of an entry, never the truthiness of its
//     POINTS — a 0-point ("refusal") credit MUST produce a real entry.
//   - R2: replaying an already-credited question must unlock it — the
//     Timestamp >= GameTime floor is what makes that true.
//   - R3: grouping is by TeamName, never WinnerID — WinnerID carries a
//     bumper MAC on SPEEDY's BUMPER_POINTS path, which would never lock
//     anything if grouped on.
// ---------------------------------------------------------------------------

// readyAndStart puts app in PhaseStarted for question id with a real,
// non-zero GameTime (StartImmediate stamps it via time.Now()) — needed to
// exercise the Timestamp >= GameTime floor with explicit before/after event
// timestamps relative to a known baseline, not test-order-dependent.
func readyAndStart(t *testing.T, app *App, id string) int64 {
	t.Helper()
	app.engine.Ready(id, &game.Question{ID: id})
	app.engine.StartImmediate(0)
	return app.engine.GetState().GameTime
}

func TestGetAwardedTeamsPayload_NoQuestion_EmptyQuestionID(t *testing.T) {
	app := newNextQuestionTestApp(t)

	got := app.getAwardedTeamsPayload()
	if got == nil {
		t.Fatal("expected a non-nil payload")
	}
	if got.QuestionID != "" {
		t.Errorf("expected QUESTION_ID=\"\" (no current question), got %q", got.QuestionID)
	}
	if got.Teams == nil {
		t.Error("expected Teams == [] (never nil), got nil")
	}
	if len(got.Teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(got.Teams))
	}
}

func TestGetAwardedTeamsPayload_NoCredit_EmptySliceNotNil(t *testing.T) {
	app := newNextQuestionTestApp(t)
	readyAndStart(t, app, "1")

	got := app.getAwardedTeamsPayload()
	if got.QuestionID != "1" {
		t.Errorf("expected QUESTION_ID=1, got %q", got.QuestionID)
	}
	if got.Teams == nil {
		t.Fatal("expected Teams == [] (never nil) when no credit has happened yet")
	}
	if len(got.Teams) != 0 {
		t.Errorf("expected 0 teams, got %+v", got.Teams)
	}
}

func TestGetAwardedTeamsPayload_TeamPointsCreditPresent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "1", EventType: "POINTS_AWARDED",
		TeamName: "Les Rouges", WinnerType: "TEAM", Points: 10,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 {
		t.Fatalf("expected 1 team, got %+v", got.Teams)
	}
	if got.Teams[0].Team != "Les Rouges" || got.Teams[0].Points != 10 || got.Teams[0].Timestamp != gameTime+1000 {
		t.Errorf("unexpected entry: %+v", got.Teams[0])
	}
}

// R3 — chemin nominal SPEEDY : le crédit vise un joueur (BUMPER_POINTS),
// WinnerID porte une MAC, mais TeamName est toujours renseigné. Le
// regroupement DOIT se faire sur TeamName, jamais WinnerID.
func TestGetAwardedTeamsPayload_BumperPointsCreditPresent_GroupedByTeamName(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "1", EventType: "POINTS_AWARDED",
		WinnerID: "aa:bb:cc:dd:ee:ff", WinnerName: "Alice", WinnerType: "PLAYER",
		TeamName: "Les Rouges", PlayerName: "Alice", Points: 5,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 {
		t.Fatalf("expected 1 team (grouped by TeamName despite a PLAYER-targeted credit), got %+v", got.Teams)
	}
	if got.Teams[0].Team != "Les Rouges" || got.Teams[0].Points != 5 {
		t.Errorf("expected Les Rouges/5, got %+v — grouping must use TeamName, never WinnerID (R3)", got.Teams[0])
	}
}

// R1 — un crédit à 0 point (le geste "0 pt") est une entrée à part entière,
// jamais filtrée pour être "falsy".
func TestGetAwardedTeamsPayload_ZeroPointCreditPresent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "1", EventType: "POINTS_AWARDED",
		TeamName: "Les Bleus", WinnerType: "TEAM", Points: 0,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 {
		t.Fatalf("expected 1 team even with Points=0 (a refusal still locks), got %+v", got.Teams)
	}
	if got.Teams[0].Team != "Les Bleus" || got.Teams[0].Points != 0 {
		t.Errorf("expected Les Bleus/0, got %+v", got.Teams[0])
	}
}

// R1 (variante) — la SOMME peut valoir 0 (deux crédits qui s'annulent, ou
// deux refus) sans que l'entrée disparaisse : `if sum {}` serait le même
// piège que `if points {}` côté client.
func TestGetAwardedTeamsPayload_ZeroSum_EntryStillPresent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "1", EventType: "POINTS_AWARDED",
		TeamName: "Les Verts", WinnerType: "TEAM", Points: 0,
	})
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 2000, QuestionID: "1", EventType: "POINTS_AWARDED",
		TeamName: "Les Verts", WinnerType: "TEAM", Points: 0,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 {
		t.Fatalf("expected 1 team (2 zero-point credits merged), got %+v", got.Teams)
	}
	if got.Teams[0].Points != 0 {
		t.Errorf("expected sum=0, got %d", got.Teams[0].Points)
	}
}

func TestGetAwardedTeamsPayload_OtherQuestion_Absent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "2", EventType: "POINTS_AWARDED", // autre question
		TeamName: "Les Rouges", WinnerType: "TEAM", Points: 10,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 0 {
		t.Errorf("expected 0 teams (credit belongs to a different question), got %+v", got.Teams)
	}
}

// R2 — événement de la MÊME question mais ANTÉRIEUR au dernier départ
// (GameTime) : la question a été rejouée, ce crédit appartient à la
// partie précédente et ne doit plus verrouiller.
func TestGetAwardedTeamsPayload_BeforeLastGameStart_Absent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime - 1000, QuestionID: "1", EventType: "POINTS_AWARDED", // avant le départ courant
		TeamName: "Les Rouges", WinnerType: "TEAM", Points: 10,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 0 {
		t.Errorf("expected 0 teams (credit predates the current GameTime — replayed question), got %+v", got.Teams)
	}
}

func TestGetAwardedTeamsPayload_TwoCreditsOnSameTeam_SummedFirstTimestamp(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 3000, QuestionID: "1", EventType: "POINTS_AWARDED",
		TeamName: "Les Rouges", WinnerType: "TEAM", Points: 10,
	})
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 5000, QuestionID: "1", EventType: "POINTS_AWARDED", // recrédit régie
		TeamName: "Les Rouges", WinnerType: "TEAM", Points: 5,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 {
		t.Fatalf("expected 1 team (merged), got %+v", got.Teams)
	}
	if got.Teams[0].Points != 15 {
		t.Errorf("expected summed Points=15, got %d", got.Teams[0].Points)
	}
	if got.Teams[0].Timestamp != gameTime+3000 {
		t.Errorf("expected Timestamp = FIRST credit's (%d), got %d", gameTime+3000, got.Teams[0].Timestamp)
	}
}

// Non-POINTS_AWARDED events in the same history (e.g. BUZZ) must never be
// mistaken for a credit.
func TestGetAwardedTeamsPayload_IgnoresNonPointsAwardedEvents(t *testing.T) {
	app := newNextQuestionTestApp(t)
	gameTime := readyAndStart(t, app, "1")
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: gameTime + 1000, QuestionID: "1", EventType: "BUZZ",
		TeamName: "Les Rouges", Points: 999,
	})

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 0 {
		t.Errorf("expected 0 teams (BUZZ is not a credit), got %+v", got.Teams)
	}
}
