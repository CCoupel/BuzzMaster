package main

// Developer-side tests for the ambiance adapter (dev-backend, #205). The
// contract suite (exhaustiveness AST test, CA1..CA7) is test-writer's. These
// TestDev* tests pin the derivation table (§6.2), the scene table (§8), the
// team-colour factoring (§8, "never a second palette") and the nil-writer
// no-op path that makes the 21 unguarded sites safe.

import (
	"context"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

func TestDevAmbianceNilWriterSitesAreNoOps(t *testing.T) {
	app := newTestApp(t)
	if app.lighting != nil {
		t.Fatal("test app must start with no lighting writer")
	}
	if app.ambianceIsConfigured() {
		t.Fatal("#205: ambiance must not be configured")
	}
	// Every site calls these unguarded; they must be no-ops on nil.
	app.lighting.NotifyState()
	app.lighting.NotifyPulse(lighting.KindScore, []string{"TeamA"}, lighting.ScorePulseDuration)
	app.setupAmbiance()
	if app.lighting != nil {
		t.Fatal("setupAmbiance must leave lighting nil when not configured")
	}
}

func TestDevAmbianceActiveTeam(t *testing.T) {
	q := func(typ game.QuestionType) *game.Question { return &game.Question{Type: typ} }
	tests := []struct {
		name  string
		state game.GameState
		want  string
	}{
		{"no question", game.GameState{}, ""},
		{"classic question", game.GameState{Question: q(game.QuestionTypeSpeedy), MemoryCurrentTeam: "X"}, ""},
		{"memory", game.GameState{Question: q(game.QuestionTypeMemory), MemoryCurrentTeam: "TeamB"}, "TeamB"},
		{"memotion", game.GameState{Question: q(game.QuestionTypeMemotion), MotionCurrentTeam: "TeamC"}, "TeamC"},
		{"rafale", game.GameState{Question: q(game.QuestionTypeRafale), RafaleCurrentTeam: "TeamA"}, "TeamA"},
		{"memory solo (no current team)", game.GameState{Question: q(game.QuestionTypeMemory)}, ""},
	}
	for _, tt := range tests {
		if got := ambianceActiveTeam(tt.state); got != tt.want {
			t.Errorf("%s: active team = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDevAmbianceDerivationTable(t *testing.T) {
	app := newTestApp(t)
	// setState puts the engine in (phase, question) and installs bumpers
	// with the given press times / answers — through the engine's own
	// exported mutators, no reach into private state.
	type press struct {
		mac, team string
		at        int64
		answer    game.AnswerColor
	}
	setState := func(phase game.GamePhase, question *game.Question, presses ...press) {
		app.engine.SetEntracte(false)
		app.engine.SetPhase(game.PhaseStopped)
		if question != nil {
			app.engine.Ready("q-dev", question)
		}
		bumpers := map[string]*game.Bumper{
			"m1": {Name: "m1", Team: "TeamA"},
			"m2": {Name: "m2", Team: "TeamB"},
			"m3": {Name: "m3", Team: "TeamC"},
		}
		for _, p := range presses {
			bumpers[p.mac].Time = p.at
			bumpers[p.mac].AnswerColor = p.answer
		}
		app.engine.SetBumpers(bumpers)
		app.engine.SetPhase(phase)
	}
	speedy := &game.Question{Type: game.QuestionTypeSpeedy}
	qcmRed := &game.Question{Type: game.QuestionTypeQCM, TypedContent: game.TypedContent{QCMCorrect: "RED"}}

	tests := []struct {
		name  string
		setup func()
		want  lighting.Event
	}{
		{"stopped → idle", func() { setState(game.PhaseStopped, nil) }, lighting.Event{Kind: lighting.KindIdle}},
		{"new game → idle", func() { setState(game.PhaseNewGame, nil) }, lighting.Event{Kind: lighting.KindIdle}},
		{"prepare → ready", func() { setState(game.PhasePrepare, speedy) }, lighting.Event{Kind: lighting.KindReady}},
		{"ready → ready", func() { setState(game.PhaseReady, speedy) }, lighting.Event{Kind: lighting.KindReady}},
		{"countdown → ready (no countdown scene, #212)", func() { setState(game.PhaseCountdown, speedy) }, lighting.Event{Kind: lighting.KindReady}},
		{"started classic → running", func() { setState(game.PhaseStarted, speedy) }, lighting.Event{Kind: lighting.KindRunning}},
		{"paused nobody buzzed → pause all", func() { setState(game.PhasePaused, speedy) }, lighting.Event{Kind: lighting.KindPauseAll}},
		{"paused after buzz → buzz team (latest press)", func() {
			setState(game.PhasePaused, speedy, press{"m1", "TeamA", 1000, ""}, press{"m2", "TeamB", 2000, ""})
		}, lighting.Event{Kind: lighting.KindBuzz, Teams: []string{"TeamB"}}},
		{"revealed classic → reveal, no teams", func() { setState(game.PhaseRevealed, speedy, press{"m1", "TeamA", 10, ""}) }, lighting.Event{Kind: lighting.KindReveal}},
		{"revealed QCM → correct teams by press order, deduped", func() {
			setState(game.PhaseRevealed, qcmRed,
				press{"m3", "TeamC", 500, game.AnswerColorRed},  // right, first
				press{"m1", "TeamA", 700, game.AnswerColorBlue}, // wrong
				press{"m2", "TeamB", 900, game.AnswerColorRed})  // right
		}, lighting.Event{Kind: lighting.KindReveal, Teams: []string{"TeamC", "TeamB"}}},
		{"revealed QCM nobody right → reveal, no teams", func() {
			setState(game.PhaseRevealed, qcmRed, press{"m1", "TeamA", 700, game.AnswerColorBlue})
		}, lighting.Event{Kind: lighting.KindReveal}},
		{"revealed QCM not buzzed but right answer set → ignored", func() {
			setState(game.PhaseRevealed, qcmRed, press{"m1", "TeamA", 0, game.AnswerColorRed})
		}, lighting.Event{Kind: lighting.KindReveal}},
		{"entracte beats the phase", func() {
			setState(game.PhaseReady, speedy)
			if !app.engine.SetEntracte(true) {
				t.Fatal("entracte activation refused in READY")
			}
		}, lighting.Event{Kind: lighting.KindEntracte}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got := app.deriveAmbianceEvent()
			if got.Kind != tt.want.Kind || !equalStrings(got.Teams, tt.want.Teams) {
				t.Fatalf("derive = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDevAmbianceSceneTableAndTeamPalette(t *testing.T) {
	app := newTestApp(t)
	// TeamA is {255,0,0} without ColorName → hue-nearest palette "rouge".
	wantTeamA := app.teamNameToRGB("TeamA")
	if wantTeamA != teamColorPalette["rouge"] {
		t.Fatalf("palette factoring broken: TeamA → %v", wantTeamA)
	}
	if got := app.teamColorToRGB(&game.Bumper{Team: "TeamA"}); got != wantTeamA {
		t.Fatalf("teamColorToRGB must go through teamNameToRGB: %v vs %v", got, wantTeamA)
	}
	gray := [3]int{128, 128, 128}
	if app.teamNameToRGB("") != gray || app.teamNameToRGB("nope") != gray {
		t.Fatal("unknown/no team must stay gray {128,128,128}")
	}

	zone := func(ev lighting.Event) lighting.ZoneState {
		st := app.ambianceScene(ev)
		if len(st.Zones) != 1 || st.Zones[0].Zone != lighting.ZoneGeneral {
			t.Fatalf("#205 renders exactly one 'general' zone, got %+v", st)
		}
		return st.Zones[0]
	}
	tests := []struct {
		ev        lighting.Event
		color     [3]int
		intensity int
	}{
		{lighting.Event{Kind: lighting.KindIdle}, [3]int{255, 214, 170}, 120},
		{lighting.Event{Kind: lighting.KindReady}, [3]int{255, 255, 255}, 200},
		{lighting.Event{Kind: lighting.KindRunning}, [3]int{40, 90, 255}, 160},
		{lighting.Event{Kind: lighting.KindBuzz, Teams: []string{"TeamA"}}, wantTeamA, 255},
		{lighting.Event{Kind: lighting.KindPauseAll}, [3]int{255, 170, 0}, 120},
		{lighting.Event{Kind: lighting.KindReveal, Teams: []string{"TeamA"}}, [3]int{0, 220, 60}, 255},
		{lighting.Event{Kind: lighting.KindReveal}, [3]int{230, 30, 30}, 255},
		{lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}}, wantTeamA, 200},
		{lighting.Event{Kind: lighting.KindScore, Teams: []string{"TeamA"}}, wantTeamA, 255},
		{lighting.Event{Kind: lighting.KindScore}, gray, 255},
		{lighting.Event{Kind: lighting.KindEntracte}, [3]int{255, 214, 170}, 100}, // room stays lit, buzzers go dark
	}
	for _, tt := range tests {
		z := zone(tt.ev)
		if z.Color != tt.color || z.Intensity != tt.intensity {
			t.Errorf("scene(%s %v) = %v/%d, want %v/%d", tt.ev.Kind, tt.ev.Teams, z.Color, z.Intensity, tt.color, tt.intensity)
		}
	}
}

func TestDevAmbianceRegistryShape(t *testing.T) {
	state, pulse, none := 0, 0, 0
	for site, d := range ambianceSiteRegistry {
		switch d.Kind {
		case ambianceNotifyState:
			state++
		case ambianceNotifyPulse:
			pulse++
		case ambianceNoAmbiance:
			none++
		default:
			t.Errorf("%+v: unknown decision %q", site, d.Kind)
		}
		if d.Reason == "" {
			t.Errorf("%+v: every decision needs a reason", site)
		}
		if ambianceIsRenderingLayer(site.Func) && d.Kind != ambianceNoAmbiance {
			t.Errorf("%+v: a rendering-layer function can never emit ambiance (contract §1)", site)
		}
	}
	// 21 sites in main.go collapse to 20 distinct (func, LED) pairs:
	// handleFlipMemoryCard calls sendLEDSetAllBuzzers three times.
	if state != 15 || pulse != 4 || none != 4 {
		t.Fatalf("registry = %d state / %d pulse / %d none, want 15 / 4 / 4", state, pulse, none)
	}
}

// End-to-end through the real adapter with a fake driver: the writer sees
// the App's live derivation, and a SCORE pulse falls back on its own.
func TestDevAmbianceWriterRendersLiveState(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lighting = app.newAmbianceWriter(fake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.lighting.Start(ctx)

	app.engine.SetPhase(game.PhasePrepare)
	app.lighting.NotifyState()
	waitForCount(t, fake, 1)
	if last, _ := fake.Last(); last.Zones[0].Color != [3]int{255, 255, 255} {
		t.Fatalf("READY scene expected, got %+v", last)
	}

	// The pulse must outlive the 100 ms throttle that follows the first
	// Apply, otherwise it legitimately expires before being rendered (last
	// state wins). Real SCORE pulses last 4800 ms; 300 ms keeps the test fast.
	app.lighting.NotifyPulse(lighting.KindScore, []string{"TeamB"}, 300*time.Millisecond)
	waitForCount(t, fake, 2)
	if last, _ := fake.Last(); last.Zones[0].Color != app.teamNameToRGB("TeamB") {
		t.Fatalf("SCORE pulse must use TeamB's palette colour, got %+v", last)
	}
	// After the pulse deadline (+ throttle) the room returns to READY by itself.
	waitForCount(t, fake, 3)
	if last, _ := fake.Last(); last.Zones[0].Color != [3]int{255, 255, 255} {
		t.Fatalf("fallback to READY expected, got %+v", last)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for !fake.Closed() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !fake.Closed() {
		t.Fatal("driver must be closed when the app context is cancelled")
	}
}

func waitForCount(t *testing.T, f *lighting.FakeDriver, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.Count() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("driver received %d state(s), want >= %d", f.Count(), n)
}
