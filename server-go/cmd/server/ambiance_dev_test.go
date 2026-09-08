package main

// Developer-side tests for the ambiance adapter (dev-backend, #205). The
// contract suite (exhaustiveness AST test, CA1..CA7) is test-writer's. These
// TestDev* tests pin the derivation table (§6.2), the scene table (§8), the
// team-colour factoring (§8, "never a second palette") and the nil-writer
// no-op path that makes the 21 unguarded sites safe.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/lighting/hue"
	"buzzcontrol/internal/server"
)

func TestDevAmbianceNilWriterSitesAreNoOps(t *testing.T) {
	app := newTestApp(t)
	if app.ambiance() != nil {
		t.Fatal("test app must start with no lighting writer")
	}
	if app.ambianceIsConfigured() {
		t.Fatal("#205: ambiance must not be configured")
	}
	// Every site calls these unguarded; they must be no-ops on nil.
	app.ambiance().NotifyState()
	app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamA"}, 3, lighting.ScorePulseDuration)
	app.setupAmbiance()
	if app.ambiance() != nil {
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

	// #213: an event concerning teams now ALSO carries one zone per team
	// (TestDevAmbianceTeamZones213 below) — this helper isolates the
	// "general" zone's own colour/intensity, unaffected by that addition
	// (contract §8's table is about the general zone specifically).
	zone := func(ev lighting.Event) lighting.ZoneState {
		st := app.ambianceScene(ev)
		for _, z := range st.Zones {
			if z.Zone == lighting.ZoneGeneral {
				return z
			}
		}
		t.Fatalf("no 'general' zone in %+v", st)
		return lighting.ZoneState{}
	}
	// 2026-09-08 revision (planner-v10-general-theme-toggle-20260908-114420.md
	// §1.3, validated AND EXTENDED by the user): READY/RUNNING/BUZZ/
	// PAUSE_ALL/REVEAL/TEAM_TURN now render the CURRENT QUESTION's theme
	// colour on 'general' (white if none) instead of a fixed colour or the
	// team's own — see ambianceThemeColor's own doc comment. newTestApp sets
	// no httpServer and no live question, so ambianceThemeColor's own nil
	// guard/empty-category path is exercised here, always falling back to
	// white — the SPECIFIC resolution order (RAFALE > host question >
	// unknown/empty ⇒ white) has its own dedicated coverage,
	// TestDevAmbianceThemeColor_ResolutionOrder, below. Only IDLE (plain
	// white, unconditionally) and ENTRACTE (unchanged) stay fixed; SCORE
	// stays the credited team's own colour, untouched by this revision.
	white := [3]int{255, 255, 255}
	tests := []struct {
		ev        lighting.Event
		color     [3]int
		intensity int
	}{
		{lighting.Event{Kind: lighting.KindIdle}, white, 200},
		{lighting.Event{Kind: lighting.KindReady}, white, 200},
		{lighting.Event{Kind: lighting.KindRunning}, white, 200},
		{lighting.Event{Kind: lighting.KindBuzz, Teams: []string{"TeamA"}}, white, 255},
		{lighting.Event{Kind: lighting.KindPauseAll}, white, 120},
		{lighting.Event{Kind: lighting.KindReveal, Teams: []string{"TeamA"}}, white, 255},
		{lighting.Event{Kind: lighting.KindReveal}, white, 255},
		{lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}}, white, 200},
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

// TestDevAmbianceThemeColor_ResolutionOrder covers ambianceThemeColor's own
// 3-step resolution order (contract §8.1, 2026-09-08 revision) against a
// REAL httpServer (ResolveCategoryMeta needs one) and real question/RAFALE
// fixtures: a drawn RAFALE question's OWN category wins over the host
// question's, the host question's category is used otherwise, and an empty/
// unknown category (or no question at all) falls back to white — never a
// panic on the nil-httpServer path exercised by every other test in this
// file (TestDevAmbianceSceneTableAndTeamPalette, above).
func TestDevAmbianceThemeColor_ResolutionOrder(t *testing.T) {
	app := newTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	white := [3]int{255, 255, 255}
	geography := [3]int{0x3b, 0x82, 0xf6} // #3b82f6, hardcodedCategories (internal/server/http.go)

	if got := app.ambianceThemeColor(); got != white {
		t.Fatalf("no question at all: got %v, want white", got)
	}

	app.engine.Ready("q1", &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Category: game.CategoryGeography})
	if got := app.ambianceThemeColor(); got != geography {
		t.Fatalf("host question's own category: got %v, want %v (GEOGRAPHY)", got, geography)
	}

	app.engine.Ready("q2", &game.Question{ID: "q2", Type: game.QuestionTypeSpeedy, Category: ""})
	if got := app.ambianceThemeColor(); got != white {
		t.Fatalf("empty category must fall back to white, got %v", got)
	}

	app.engine.Ready("q3", &game.Question{ID: "q3", Type: game.QuestionTypeSpeedy, Category: "NE_EXISTE_PAS"})
	if got := app.ambianceThemeColor(); got != white {
		t.Fatalf("unknown category must fall back to white, got %v", got)
	}
}

// TestDevAmbianceTeamZones213 pins #213's addition to ambianceScene: one
// dedicated zone per team CURRENTLY ON THE BOARD (Batch A/P1 fix,
// _work/reports/planner-v10-groups-teamcolor-20260907-173831.md §Problème
// 1 — hue-bridge.md §5.2's "état courant" is the LIVE GAME STATE, not the
// event), on top of the unaffected "general" zone (§9) — the driver-side
// routing (zoneFor, internal/lighting/hue/driver.go) already existed
// before this task; this is what actually feeds it.
//
// Batch C/C1a (planner-v10-teamcolor-changes-20260908-092100.md §1):
// updated — a team's own zone is now emitted for EVERY Kind, KindIdle
// included (no more "hors partie ⇒ general only" carve-out), and is at
// FULL intensity outside the three "active game" Kinds
// (RUNNING/PAUSE_ALL/REVEAL — TEAM_TURN/BUZZ/SCORE always have their sole
// concerned team distinguished so they never actually surface the dimmed
// branch in this particular test's shape-only assertions below).
//
// This test covers the STRUCTURAL shape (which zones appear, general's own
// colour untouched, dedup). The distinguished/dimmed INTENSITY matrix has
// its own dedicated coverage:
// TestTeamColor_DistinguishedTeamFullIntensity_OthersAttenuated
// (ambiance_team_color_acceptance_test.go, test-writer/A2).
func TestDevAmbianceTeamZones213(t *testing.T) {
	app := newTestApp(t) // TeamA/TeamB/TeamC configured — see newTestApp's own comment
	wantTeamA := app.teamNameToRGB("TeamA")
	wantTeamB := app.teamNameToRGB("TeamB")
	wantTeamC := app.teamNameToRGB("TeamC")
	if wantTeamA == wantTeamB {
		t.Fatal("setup invalide : TeamA et TeamB doivent avoir des couleurs distinctes pour ce test")
	}

	zonesByName := func(ev lighting.Event) map[string]lighting.ZoneState {
		st := app.ambianceScene(ev)
		out := make(map[string]lighting.ZoneState, len(st.Zones))
		for _, z := range st.Zones {
			out[z.Zone] = z
		}
		return out
	}

	// KindIdle (hors partie), KindReady (PREPARE/READY/COUNTDOWN) and
	// KindEntracte: a team's own zone is now ALWAYS present, at FULL
	// intensity — C1a's "jamais de repli, hors partie comme en partie",
	// with the buzzer-table intensity rule ("STOPPED/PREPARE/READY/
	// COUNTDOWN ⇒ full, always").
	for _, ev := range []lighting.Event{{Kind: lighting.KindIdle}, {Kind: lighting.KindReady}, {Kind: lighting.KindEntracte}} {
		zones := zonesByName(ev)
		if len(zones) != 4 { // general + TeamA + TeamB + TeamC
			t.Fatalf("%s : attendu 4 zones (general + les 3 équipes du plateau, C1a), got %+v", ev.Kind, zones)
		}
		for name, want := range map[string][3]int{"TeamA": wantTeamA, "TeamB": wantTeamB, "TeamC": wantTeamC} {
			z := zones[name]
			if z.Color != want {
				t.Errorf("%s: zone %s doit porter sa propre couleur, got %v want %v", ev.Kind, name, z.Color, want)
			}
			if z.Intensity != 255 {
				t.Errorf("%s: zone %s (hors des 3 phases actives) doit être à pleine intensité, got intensity=%d", ev.Kind, name, z.Intensity)
			}
		}
	}

	// The three "active game" Kinds (STARTED-no-turn/PAUSED-admin/REVEALED
	// with nobody credited) still dim an undistinguished team's own zone —
	// unaffected by C1a, which only removed the fallback, not this rule.
	for _, ev := range []lighting.Event{
		{Kind: lighting.KindRunning}, {Kind: lighting.KindPauseAll}, {Kind: lighting.KindReveal},
	} {
		zones := zonesByName(ev)
		if len(zones) != 4 { // general + TeamA + TeamB + TeamC
			t.Fatalf("%s : attendu 4 zones (general + les 3 équipes du plateau), got %+v", ev.Kind, zones)
		}
		for name, want := range map[string][3]int{"TeamA": wantTeamA, "TeamB": wantTeamB, "TeamC": wantTeamC} {
			z := zones[name]
			if z.Color != want {
				t.Errorf("%s: zone %s doit porter sa propre couleur, got %v want %v", ev.Kind, name, z.Color, want)
			}
			if z.Intensity != dimIntensityFor(want) {
				t.Errorf("%s: zone %s non distinguée doit être atténuée (dimIntensityFor), got intensity=%d", ev.Kind, name, z.Intensity)
			}
		}
	}

	// A single team distinguished (BUZZ/TEAM_TURN/SCORE) ⇒ its own zone at
	// FULL intensity (255, the buzzer's own SOLID/BLINK equivalent) — the
	// two OTHER board teams stay present, dimmed. General keeps its own,
	// unaffected, per-scene colour/intensity (TestDevAmbianceSceneTableAndTeamPalette).
	for _, ev := range []lighting.Event{
		{Kind: lighting.KindBuzz, Teams: []string{"TeamA"}},
		{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}},
		{Kind: lighting.KindScore, Teams: []string{"TeamA"}},
	} {
		zones := zonesByName(ev)
		if len(zones) != 4 {
			t.Fatalf("%s: attendu 4 zones (general + les 3 équipes), got %+v", ev.Kind, zones)
		}
		if got := zones["TeamA"]; got.Color != wantTeamA || got.Intensity != 255 {
			t.Errorf("%s: TeamA (distinguée) doit être à pleine intensité dans sa propre couleur, got %+v", ev.Kind, got)
		}
		if got := zones["TeamB"]; got.Intensity != dimIntensityFor(wantTeamB) {
			t.Errorf("%s: TeamB (non distinguée) doit rester atténuée, got %+v", ev.Kind, got)
		}
	}

	// REVEAL with SEVERAL correct teams: general stays fixed green
	// (unaffected — REVEAL is not UseTeamColor), TeamA/TeamB (both
	// distinguished) at full intensity in their OWN colour — never a
	// shared "success" hue that would erase which team is which — TeamC
	// (on the board, not distinguished) stays present, dimmed.
	zones := zonesByName(lighting.Event{Kind: lighting.KindReveal, Teams: []string{"TeamA", "TeamB"}})
	if len(zones) != 4 {
		t.Fatalf("REVEAL multi-équipe : attendu 4 zones (general + les 3 équipes), got %+v", zones)
	}
	// 2026-09-08 revision: 'general' on REVEAL now carries the question's
	// theme colour, not a fixed green/red — white here since newTestApp
	// leaves httpServer nil (see TestDevAmbianceThemeColor_ResolutionOrder
	// for the dedicated colour-resolution coverage). Unaffected either way
	// by #213's per-team zones, which is this test's own actual point.
	if g := zones[lighting.ZoneGeneral]; g.Color != [3]int{255, 255, 255} || g.Intensity != 255 {
		t.Errorf("REVEAL multi-équipe : general doit porter le thème (blanc ici, non affecté par #213), got %+v", g)
	}
	if zones["TeamA"].Color != wantTeamA || zones["TeamA"].Intensity != 255 {
		t.Errorf("REVEAL multi-équipe : zone TeamA doit porter sa propre couleur à pleine intensité, got %+v", zones["TeamA"])
	}
	if zones["TeamB"].Color != wantTeamB || zones["TeamB"].Intensity != 255 {
		t.Errorf("REVEAL multi-équipe : zone TeamB doit porter sa propre couleur à pleine intensité, got %+v", zones["TeamB"])
	}
	if zones["TeamC"].Intensity != dimIntensityFor(wantTeamC) {
		t.Errorf("REVEAL multi-équipe : TeamC (sur le plateau, non distinguée) doit rester atténuée, got %+v", zones["TeamC"])
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
	// 21 sites in main.go collapse to 20 distinct (func, LED) pairs, plus
	// the T2.1 (#214/R1) fix's own site (onPhaseStarted → sendLEDSetAllEntracteOff,
	// contract §10.5) and the T2.3 (#208) shutdown extinction site
	// (stop → sendLEDSetAllEntracteOff, contract §10.4) — 16 state / 4 pulse / 5 none.
	// handleFlipMemoryCard calls sendLEDSetAllBuzzers three times.
	if state != 16 || pulse != 4 || none != 5 {
		t.Fatalf("registry = %d state / %d pulse / %d none, want 16 / 4 / 5", state, pulse, none)
	}
}

// End-to-end through the real adapter with a fake driver: the writer sees
// the App's live derivation, and a SCORE pulse falls back on its own.
func TestDevAmbianceWriterRendersLiveState(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)

	app.engine.SetPhase(game.PhasePrepare)
	app.ambiance().NotifyState()
	waitForCount(t, fake, 1)
	if last, _ := fake.Last(); last.Zones[0].Color != [3]int{255, 255, 255} {
		t.Fatalf("READY scene expected, got %+v", last)
	}

	// The pulse must outlive the 100 ms throttle that follows the first
	// Apply, otherwise it legitimately expires before being rendered (last
	// state wins). Real SCORE pulses last 4800 ms; 300 ms keeps the test fast.
	app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamB"}, 3, 300*time.Millisecond)
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

// waitForChronoPulseTier polls the LIVE atomics runChronoPulse writes
// (never a value this test computes itself) until they report the wanted
// tier while active, or the timeout elapses.
func waitForChronoPulseTier(t *testing.T, app *App, wantTier int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if app.chronoPulseActive.Load() && app.chronoPulseTier.Load() == wantTier {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("chrono-pulse never reported tier %d (active=%v tier=%d)", wantTier, app.chronoPulseActive.Load(), app.chronoPulseTier.Load())
}

// TestDevChronoPulse_TierSelectionFromLiveTimer covers runChronoPulse's own
// tier arithmetic against the REAL engine timer (StartImmediate(delay) sets
// GameState.CurrentTime = Delay = delay directly, contract §5.4/§8.1's own
// tier boundaries: >10s tier 1, <=10s tier 2, <=5s tier 3) — not a
// synthetic GameState, the actual live path runChronoPulse reads.
func TestDevChronoPulse_TierSelectionFromLiveTimer(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)
	go app.runChronoPulse(ctx) // not wired via a bare Start(ctx) — see startAmbianceWriter's own two call sites

	for _, tt := range []struct {
		delay    int
		wantTier int32
	}{
		{15, 1}, // > 10 s remaining
		{10, 2}, // <= 10 s remaining
		{5, 3},  // <= 5 s remaining
	} {
		qcm := &game.Question{ID: "q1", Type: game.QuestionTypeQCM}
		app.engine.Ready(qcm.ID, qcm)
		app.engine.StartImmediate(tt.delay)
		waitForChronoPulseTier(t, app, tt.wantTier)
		app.engine.Stop()
		// Leaving RUNNING must clear the pulse promptly (belt-and-suspenders
		// NotifyState in runChronoPulse's own "become inactive" branch —
		// see its doc comment on why this isn't load-bearing either way).
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && app.chronoPulseActive.Load() {
			time.Sleep(5 * time.Millisecond)
		}
		if app.chronoPulseActive.Load() {
			t.Fatalf("delay=%d: chrono-pulse still active after Stop()", tt.delay)
		}
	}
}

// TestDevChronoPulse_RenderedZoneCarriesTheFadeAndTheRightPhaseColours
// verifies what actually reaches the driver (not just the internal
// atomics): the 'general' zone alternates between the theme colour (white
// here — newTestApp sets no httpServer/question category) at full
// intensity and, depending on tier, the theme again (tier 1), amber (tier
// 2) or red (tier 3) at a lower intensity — each write carrying
// chronoPulseTransitionMs, never 0, for the fade the whole effect exists
// for.
func TestDevChronoPulse_RenderedZoneCarriesTheFadeAndTheRightPhaseColours(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)
	go app.runChronoPulse(ctx)

	qcm := &game.Question{ID: "q1", Type: game.QuestionTypeQCM}
	app.engine.Ready(qcm.ID, qcm)
	app.engine.StartImmediate(4) // tier 3 immediately: theme <-> red
	defer app.engine.Stop()
	waitForChronoPulseTier(t, app, 3)

	white := [3]int{255, 255, 255}
	sawHighWhiteFull, sawLowRedDim := false, false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !(sawHighWhiteFull && sawLowRedDim) {
		if last, ok := fake.Last(); ok && len(last.Zones) > 0 {
			z := last.Zones[0]
			if z.Zone != lighting.ZoneGeneral {
				t.Fatalf("zone 0 must be 'general', got %+v", last.Zones)
			}
			if z.TransitionMs != chronoPulseTransitionMs {
				t.Fatalf("every chrono-pulse write must carry the half-cycle fade (%d ms), got %+v", chronoPulseTransitionMs, z)
			}
			if z.Color == white && z.Intensity == chronoPulseIntensityFull {
				sawHighWhiteFull = true
			}
			if z.Color == ambianceChronoPulseRed && z.Intensity == chronoPulseIntensityLow {
				sawLowRedDim = true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawHighWhiteFull || !sawLowRedDim {
		t.Fatalf("expected both phases to be rendered within one cycle: high(white/full)=%v low(red/dim)=%v", sawHighWhiteFull, sawLowRedDim)
	}
}

// TestDevChronoPulse_SuppressedBySelector covers "s'efface devant le
// sélecteur ON/AUTO/OFF" (contract §10.1): a forced ON must show the
// selector's own forced scene, never a pulse colour, and never a fade
// (TransitionMs 0 — a forced state must snap).
func TestDevChronoPulse_SuppressedBySelector(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)
	go app.runChronoPulse(ctx)

	qcm := &game.Question{ID: "q1", Type: game.QuestionTypeQCM}
	app.engine.Ready(qcm.ID, qcm)
	app.engine.StartImmediate(15)
	defer app.engine.Stop()
	waitForChronoPulseTier(t, app, 1) // pulse genuinely active underneath

	// OFF (not ON): its forced {0,0,0}/0 can never coincide with a pulse
	// phase's own colour/intensity (white/theme, amber or red, always >0),
	// so a match unambiguously proves the selector actually won — unlike
	// ON's white/255, which happens to equal the pulse's own high-phase
	// value in this test's fixture (no theme configured) and would leave a
	// stale pulse-phase sample indistinguishable from a genuinely forced
	// one.
	app.setLightingMode(lightingModeOff)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		last, ok := fake.Last()
		if ok && len(last.Zones) > 0 {
			z := last.Zones[0]
			if z.Color == [3]int{0, 0, 0} && z.Intensity == 0 {
				if z.TransitionMs != 0 {
					t.Fatalf("a forced OFF must snap, never fade (chrono-pulse's own transitionMs must not leak through), got %+v", z)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("mode OFF never forced 'general' off over the chrono-pulse")
}

// TestDevChronoPulse_SuppressedBySelector_ON is the code-reviewer-flagged
// (v10-3lots, MINEUR 2) companion to the OFF test above — defense in depth,
// not a functional gap: lightingOverrideGeneral treats ON and OFF as two
// structurally symmetric branches of the SAME switch (only the forced
// value differs, never the bypass mechanism), so OFF already proves the
// mechanism works for both. This test guards specifically against a FUTURE
// regression that would give ON its own special-cased path no longer
// running through lightingOverrideGeneral, which the OFF test alone could
// not catch.
//
// Uses a real, NON-white theme (GEOGRAPHY, blue) — unlike the OFF test,
// ON's forced colour (lightingOnColor, white/255) would otherwise coincide
// with the pulse's own tier-1 high-phase value in a themeless fixture
// (white, same as TestDevChronoPulse_SuppressedBySelector's own doc
// comment explains for why THAT test uses OFF), leaving a stale pulse
// sample indistinguishable from a genuinely forced one.
func TestDevChronoPulse_SuppressedBySelector_ON(t *testing.T) {
	app := newTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)
	go app.runChronoPulse(ctx)

	geography := [3]int{0x3b, 0x82, 0xf6} // #3b82f6 (hardcodedCategories) — deliberately NOT white, unlike lightingOnColor
	qcm := &game.Question{ID: "q1", Type: game.QuestionTypeQCM, Category: game.CategoryGeography}
	app.engine.Ready(qcm.ID, qcm)
	app.engine.StartImmediate(15)
	defer app.engine.Stop()
	waitForChronoPulseTier(t, app, 1)
	if got := app.ambianceThemeColor(); got != geography {
		t.Fatalf("setup invalide : thème attendu GEOGRAPHY %v, got %v", geography, got)
	}

	app.setLightingMode(lightingModeOn)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		last, ok := fake.Last()
		if ok && len(last.Zones) > 0 {
			z := last.Zones[0]
			if z.Color == lightingOnColor && z.Intensity == lightingOnIntensity {
				if z.TransitionMs != 0 {
					t.Fatalf("a forced ON must snap, never fade (chrono-pulse's own transitionMs must not leak through), got %+v", z)
				}
				return
			}
			// Anything else (the theme colour, amber, red — a pulse phase
			// sample that predates the mode change) is simply not yet the
			// settled forced state: keep polling rather than fail early.
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("mode ON never forced 'general' to its own scene over the chrono-pulse")
}

// #207 — the App builds/hot-swaps the Hue driver from config.json's
// `lighting` section: disabled ⇒ no writer, no driver; enabled at runtime ⇒
// writer + driver + goroutine; disabled again ⇒ driver nil, writer idles.
func TestDevAmbianceReconfigureFromConfig(t *testing.T) {
	// A minimal Hue v1 fake (config + lights + state) with a fictitious id.
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights"):
			_, _ = w.Write([]byte(`{"8":{"name":"BuzzHue1","state":{"on":false,"bri":1,"xy":[0.3,0.3],"reachable":true}}}`))
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == "PUT":
			_, _ = w.Write([]byte(`[{"success":{"/lights/8/state/on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer bridge.Close()

	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })

	// Not configured: nothing.
	app.setupAmbiance()
	if app.ambiance() != nil || app.LightingDriver() != nil || app.ambianceIsConfigured() {
		t.Fatal("no lighting config ⇒ no writer, no driver")
	}

	// Enabled at runtime through OnConfigUpdate ⇒ driver + writer + goroutine.
	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: true, BridgeIP: bridge.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}}
	config.SetInstance(&cfg)
	app.reconfigureAmbiance()
	if app.ambiance() == nil || !app.ambiance().Enabled() || app.LightingDriver() == nil {
		t.Fatal("enabled config ⇒ writer + driver")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && app.LightingDriver().Status().State != hue.StateOK {
		time.Sleep(5 * time.Millisecond)
	}
	if st := app.LightingDriver().Status(); st.State != hue.StateOK || st.LightsOK != 1 {
		t.Fatalf("driver did not reach ok on the fake bridge: %+v", st)
	}
	if !app.ambiance().Running() {
		t.Fatal("writer goroutine must be running after the first runtime enable")
	}

	// Disabled again ⇒ driver nil (status endpoint says disabled), writer idles.
	cfg2 := cfg
	cfg2.Lighting.Enabled = false
	config.SetInstance(&cfg2)
	app.reconfigureAmbiance()
	if app.LightingDriver() != nil || app.ambiance().Enabled() {
		t.Fatal("disabled config ⇒ no driver, writer disabled")
	}
	if !app.ambiance().Running() {
		t.Fatal("writer goroutine keeps idling (no restart needed)")
	}
	// Key from the environment only, without a stored key: still configured.
	t.Setenv(config.EnvHueAPIKey, "env-key")
	cfg3 := cfg
	cfg3.Lighting.APIKey = ""
	config.SetInstance(&cfg3)
	if !app.ambianceIsConfigured() {
		t.Fatal("BUZZCONTROL_HUE_API_KEY must count as a configured key")
	}
}

// Review #206/#207 (MAJEUR): the first runtime enable publishes the writer
// while the 21 event sites read it from other goroutines, and two config
// updates may race. Exactly one writer/goroutine must result; the race
// detector guards the memory model.
func TestDevAmbianceFirstEnableIsAtomicUnderConcurrency(t *testing.T) {
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights"):
			_, _ = w.Write([]byte(`{"8":{"name":"BuzzHue1","state":{"on":false,"bri":1,"xy":[0.3,0.3],"reachable":true}}}`))
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == "PUT":
			_, _ = w.Write([]byte(`[{"success":{"/lights/8/state/on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer bridge.Close()

	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	app.setupAmbiance()
	if app.ambiance() != nil {
		t.Fatal("not configured at startup ⇒ nil writer")
	}
	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: true, BridgeIP: bridge.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}}
	config.SetInstance(&cfg)

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 4; i++ { // event sites, before/during/after the enable
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					app.ambiance().NotifyState()
					app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamA"}, 3, lighting.ScorePulseDuration)
				}
			}
		}()
	}
	var updates sync.WaitGroup
	for i := 0; i < 8; i++ { // concurrent OnConfigUpdate calls
		updates.Add(1)
		go func() { defer updates.Done(); app.reconfigureAmbiance() }()
	}
	updates.Wait()
	close(stop)
	readers.Wait()

	w := app.ambiance()
	if w == nil || !w.Enabled() || app.LightingDriver() == nil {
		t.Fatal("enable must publish one writer and one driver")
	}
	// The driver the handlers see must be the one the writer drives (not one
	// closed by a later hot-swap), and it must reach ok. Generous deadline:
	// the full package under -race is slow.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && (!w.Running() || app.LightingDriver().Status().State != hue.StateOK) {
		time.Sleep(5 * time.Millisecond)
	}
	if !w.Running() || app.LightingDriver().Status().State != hue.StateOK {
		t.Fatalf("writer running=%v, driver %+v", w.Running(), app.LightingDriver().Status())
	}
	// Disable, then re-enable: still the same single writer handle.
	off := cfg
	off.Lighting.Enabled = false
	config.SetInstance(&off)
	app.reconfigureAmbiance()
	if app.ambiance() != w || w.Enabled() || app.LightingDriver() != nil {
		t.Fatal("disable must keep the writer handle and drop the driver")
	}
	config.SetInstance(&cfg)
	app.reconfigureAmbiance()
	if app.ambiance() != w || !w.Enabled() || app.LightingDriver() == nil {
		t.Fatal("re-enable must reuse the writer handle")
	}
}
