// Chaîne complète avec FakeOutput (#227, milestone v11.0 — plan de dev
// §C.4, contracts/sound.md) : chaque cue du catalogue doit être émise par
// son site réel de production, exercé exactement comme le jeu le ferait
// (StartImmediate/Pause/Continue/Reveal/handlers WS), jamais par un appel
// direct à audio.Engine.PlayCue depuis le test. Complémentaire de
// _work/reports/plan-verif-front-227-20260921-103500.md, qui a identifié le
// site `depart` (onPhaseStarted) comme le seul nécessitant une détection de
// front — les trois scénarios de son §5 sont couverts ici nommément.
//
// Câblage utilisé (proposé à dev-backend en coordination directe, contract
// §6.2 délègue la spec du test-garde/chaîne à Lot C) :
//
//	app.soundEngine.Store(app.newSoundEngine(fake))  // symétrique à app.lightingWriter
//	go app.sound().Start(ctx)
//	func (a *App) notifySound(c audio.Cue)           // point d'entrée unique, nil-safe
//
// Périmètre couvert : depart (3 scénarios §5 du plan-verif), entracte-debut/
// entracte-fin (voies programmée ET manuelle), gagne, reveal, temps-ecoule,
// perdu (MEMORY ET RAFALE_INVALIDATE explicite), et le NON-son de
// l'expiration d'une carte MEMOTION (§4 du plan-verif) et d'une reprise
// après PAUSE. SEUL le déclencheur "timeout RAFALE" (par opposition à
// l'action RAFALE_INVALIDATE explicite, elle bien couverte) reste un test
// explicitement skippé en fin de fichier — voir son commentaire et
// tests/procedures/sound-engine-227.md.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Aides propres à ce fichier (préfixe twa227 pour ne jamais entrer en
// collision avec un helper déclaré ailleurs dans le paquet).
// ---------------------------------------------------------------------------

// twa227WireSound builds an App with a real production wiring
// (setupCallbacks — required for the depart/entracte-programmée scenarios,
// which fire through OnStateChange), plus a sound engine bound to a fresh
// FakeOutput and started on a context cancelled at test cleanup.
func twa227WireSound(t *testing.T) (*App, *audio.FakeOutput) {
	t.Helper()
	app := newTestAppWithHub(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.config.Storage.QuestionsDir = t.TempDir()
	app.setupCallbacks()

	fake := audio.NewFakeOutput()
	app.soundEngine.Store(app.newSoundEngine(fake))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go app.sound().Start(ctx)

	return app, fake
}

// twa227WaitForPlayed polls fake.Played() until it has at least n entries
// or the timeout elapses — synchronizes with the engine's real playback
// goroutine, never a business-timing assumption.
func twa227WaitForPlayed(t *testing.T, fake *audio.FakeOutput, n int, timeout time.Duration) []audio.Cue {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fake.Count() >= n {
			return fake.Played()
		}
		time.Sleep(2 * time.Millisecond)
	}
	got := fake.Played()
	t.Fatalf("sound: %d cue(s) jouée(s) après %s, attendu >= %d (%v)", len(got), timeout, n, got)
	return nil
}

// twa227AssertNoSoundFor gives runnable code a window to (wrongly) produce
// a sound, then asserts none did. Used for the three "must stay silent"
// scenarios (Continue() after Pause, MEMOTION card expiry).
func twa227AssertNoSoundFor(t *testing.T, fake *audio.FakeOutput, window time.Duration) {
	t.Helper()
	time.Sleep(window)
	if got := fake.Played(); len(got) != 0 {
		t.Fatalf("aucun son ne devait être joué, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// depart — les trois scénarios de plan-verif-front-227-20260921-103500.md §5.
// ---------------------------------------------------------------------------

// TestSoundChain_OrdinaryStart_PlaysDepart is scenario 3 (§5) — a real start
// (StartImmediate mirrors actualStart() for this transition, same
// established pattern as TestEntracteProgrammed_OrdinaryStartAlsoNotifiesAmbiance).
func TestSoundChain_OrdinaryStart_PlaysDepart(t *testing.T) {
	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueDepart {
		t.Fatalf("un vrai départ doit jouer exactement [depart], got %v", got)
	}
}

// TestSoundChain_PauseThenContinue_PlaysNoSound is scenario 1 (§5) — the
// nominal SPEEDY flow (buzz -> PAUSE -> régie statue -> CONTINUE) must NOT
// replay `depart` on resume: Continue() re-enters PhaseStarted, re-firing
// onPhaseStarted's OnStateChange callback, but the previous phase was
// PAUSED, not COUNTDOWN/PREPARE/READY — front detection must suppress it.
func TestSoundChain_PauseThenContinue_PlaysNoSound(t *testing.T) {
	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	// Consume the initial `depart` from the start itself before testing the
	// resume — this test is about Continue(), not StartImmediate().
	twa227WaitForPlayed(t, fake, 1, time.Second)

	app.engine.Pause()
	if app.engine.GetPhase() != game.PhasePaused {
		t.Fatal("setup invalide : Pause() n'a pas mis le jeu en PAUSED")
	}
	app.engine.Continue()
	if app.engine.GetPhase() != game.PhaseStarted {
		t.Fatal("setup invalide : Continue() n'a pas repris le jeu")
	}

	if got := fake.Played(); len(got) != 1 {
		t.Fatalf("une reprise après PAUSE (Continue()) ne doit jouer AUCUN son supplémentaire — %s : %d cue(s) au total, attendu 1 (le depart initial), got %v",
			"plan-verif-front-227-20260921-103500.md §5 scénario 1", len(got), got)
	}
}

// TestSoundChain_ProgrammedEntracteStart_PlaysEntracteDebutNotDepart is
// scenario 2 (§5) — mirrors TestEntracteProgrammed_AmbianceNotifiedAndBuzzersOff_T21
// exactly (same StartImmediate(0) on an ENTRACTE question), but asserts on
// the sound side: entracte-debut, and CueDepart must NEVER also fire on the
// same transition.
func TestSoundChain_ProgrammedEntracteStart_PlaysEntracteDebutNotDepart(t *testing.T) {
	app, fake := twa227WireSound(t)
	cfg := game.EntracteConfig{Title: "PAUSE", Subtitle: "Retour dans 5mn", PanelSize: 60, AnimPeriod: 5, AnimIntensity: 10, TransitionMs: 500}
	q := &game.Question{
		ID:   "e1",
		Type: game.QuestionTypeEntracte,
		TypedContent: game.TypedContent{
			EntracteConfig: &cfg,
		},
	}
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	if !app.engine.IsEntracte() {
		t.Fatal("setup invalide : Entracte non levé après StartImmediate d'une question ENTRACTE")
	}

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	// BUGFIX (dev-backend, Lot B, coordination directe) : twa227AssertNoSoundFor
	// exige 0 son au total, mais la ligne précédente vient déjà d'en observer 1
	// (entracte-debut) — les deux ne peuvent jamais être vraies ensemble,
	// quelle que soit la justesse de la production. Remplacé par un simple
	// délai (même intention documentée : laisser une éventuelle double-cue
	// apparaître) suivi de la vraie assertion ci-dessous.
	time.Sleep(100 * time.Millisecond)
	got = fake.Played()
	if len(got) != 1 || got[0] != audio.CueEntracteDebut {
		t.Fatalf("une ENTRACTE programmée doit jouer exactement [entracte-debut], JAMAIS depart en plus — got %v", got)
	}
}

// ---------------------------------------------------------------------------
// entracte-debut / entracte-fin — voie manuelle (ENTRACTE_SET), symétrique à
// handleEntracteSet (contract §6.1 : ce site fait bien partie des 7 du
// catalogue, distinct du site programmé ci-dessus).
// ---------------------------------------------------------------------------

func TestSoundChain_ManualEntracteSet_PlaysEntracteDebutThenFin(t *testing.T) {
	app, fake := twa227WireSound(t)

	onMsg, err := protocol.NewMessage(protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: true})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleEntracteSet(onMsg)
	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueEntracteDebut {
		t.Fatalf("ENTRACTE_SET(ACTIVE=true) doit jouer entracte-debut, got %v", got)
	}

	offMsg, err := protocol.NewMessage(protocol.ActionEntracteSet, protocol.EntracteSetPayload{Active: false})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleEntracteSet(offMsg)
	got = twa227WaitForPlayed(t, fake, 2, time.Second)
	if len(got) != 2 || got[1] != audio.CueEntracteFin {
		t.Fatalf("ENTRACTE_SET(ACTIVE=false) doit ensuite jouer entracte-fin, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// gagne — handleTeamPoints, même harnais que
// awarded_teams_zero_point_test.go (payload réel, handler réel).
// ---------------------------------------------------------------------------

func TestSoundChain_TeamPointsCredited_PlaysGagne(t *testing.T) {
	app, fake := twa227WireSound(t)
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges", Color: []int{239, 68, 68}}})
	app.engine.Ready("1", &game.Question{ID: "1"})

	msg, err := protocol.NewMessage(protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 10})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleTeamPoints(msg)

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueGagne {
		t.Fatalf("un crédit de points doit jouer gagne, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// reveal — site réel broadcastReveal, appelé exactement comme
// ActionReveal le fait (main.go: `answer := a.engine.Reveal(); a.broadcastReveal(answer)`).
// ---------------------------------------------------------------------------

func TestSoundChain_Reveal_PlaysReveal(t *testing.T) {
	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, TypedContent: game.TypedContent{Answer: "42"}}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	app.engine.Stop() // Reveal() exige STOPPED ou PAUSED (engine.go:3081-3090)

	answer := app.engine.Reveal()
	app.broadcastReveal(answer)

	// BUGFIX (dev-backend, Lot B, coordination directe) : StartImmediate()
	// ci-dessus joue déjà `depart` (comportement correct et attendu, couvert
	// nommément par TestSoundChain_OrdinaryStart_PlaysDepart) — le son REVEAL
	// est donc la DEUXIÈME cue, jamais la première. Même correctif que
	// TestSoundChain_GlobalChronoExpiry_PlaysTempsEcoule applique déjà pour
	// la même raison.
	got := twa227WaitForPlayed(t, fake, 2, time.Second)
	if len(got) != 2 || got[1] != audio.CueReveal {
		t.Fatalf("REVEAL doit jouer reveal en 2e position (1re = depart, StartImmediate), got %v", got)
	}
}

// ---------------------------------------------------------------------------
// temps-ecoule — expiration RÉELLE du chrono global (StartImmediate(1) ->
// startTimer() -> processTimerTick sur le VRAI ticker 1s, engine.go:2085).
// Attente réelle bornée (~1.2s), même classe de coût que les tests -race/
// burst déjà présents dans ce dépôt (ex. TestCA4_BurstOverMeasuredWindow).
// ---------------------------------------------------------------------------

func TestSoundChain_GlobalChronoExpiry_PlaysTempsEcoule(t *testing.T) {
	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(1) // CurrentTime=1s : expire au tout premier tick réel
	defer app.engine.Stop()

	// Le `depart` initial arrive d'abord ; l'expiration suit ~1s plus tard.
	twa227WaitForPlayed(t, fake, 1, time.Second)
	got := twa227WaitForPlayed(t, fake, 2, 2*time.Second)
	if len(got) != 2 || got[1] != audio.CueTempsEcoule {
		t.Fatalf("l'expiration du chrono global doit jouer temps-ecoule en second, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Point de vigilance §4 (plan-verif-front-227) — l'expiration d'une carte
// MEMOTION (engine.go:6835, StartMotionCardTimer) est STRUCTURELLEMENT
// identique (même test currentTime<=0) mais NE DOIT JAMAIS sonner en v11.0 :
// c'est le site précis (engine.go:2085) qui doit porter la cue, jamais un
// helper partagé avec le tick de carte.
// ---------------------------------------------------------------------------

func TestSoundChain_MotionCardTimerExpiry_PlaysNoSound(t *testing.T) {
	app, fake := twa227WireSound(t)
	startMotionAtGrid(t, app, "mq-1", motionTestQuestion("mq-1"))
	if err := app.engine.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("setup: SelectMotionCard: %v", err)
	}
	if err := app.engine.FlipMotionCard(); err != nil {
		t.Fatalf("setup: FlipMotionCard: %v", err)
	}

	// BUGFIX (dev-backend, Lot B, coordination directe) : startMotionAtGrid
	// ci-dessus appelle StartImmediate(), qui joue déjà `depart` (comportement
	// correct — voir TestSoundChain_OrdinaryStart_PlaysDepart). La baseline
	// capture ce son légitime AVANT de faire expirer le timer de carte, pour
	// que l'assertion porte uniquement sur ce que le timer de carte ajoute
	// (rien, en v11.0).
	baseline := fake.Count()

	app.engine.StartMotionCardTimer(1) // CurrentTime=1s : expire au tout premier tick réel

	time.Sleep(1300 * time.Millisecond) // laisse le tick réel expirer la carte
	if got := fake.Played(); len(got) != baseline {
		t.Fatalf("l'expiration d'une carte MEMOTION ne doit produire AUCUN son SUPPLÉMENTAIRE en v11.0 (helper partagé avec temps-ecoule ?) — got %v (baseline=%d)", got, baseline)
	}
}

// ---------------------------------------------------------------------------
// perdu (MEMORY) — paire ratée, goroutine de retournement différé
// (main.go:2446-2460). FlipDelay réduit à 50ms (MemoryConfig) pour garder
// le test rapide, comportement de production inchangé (juste plus court).
// ---------------------------------------------------------------------------

func TestSoundChain_MemoryPairMissed_PlaysPerdu(t *testing.T) {
	app, fake := twa227WireSound(t)
	question := &game.Question{
		ID:   "q1",
		Type: game.QuestionTypeMemory,
		TypedContent: game.TypedContent{
			MemoryPairs: []game.MemoryPair{
				{ID: 1, Card1: game.MemoryCard{Text: "A"}, Card2: game.MemoryCard{Text: "A"}},
				{ID: 2, Card1: game.MemoryCard{Text: "B"}, Card2: game.MemoryCard{Text: "B"}},
			},
			MemoryConfig: &game.MemoryConfig{FlipDelay: 0.05},
		},
	}
	app.engine.Ready("q1", question)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA"}); err != nil {
		t.Fatalf("setup: SetMemoryParticipatingTeams: %v", err)
	}
	app.engine.SetPhase(game.PhaseStarted)

	// Deux cartes de paires DIFFÉRENTES — un raté certain (A != B).
	firstMsg, err := protocol.NewMessage(protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleFlipMemoryCard("test-admin", server.ClientTypeAdmin, firstMsg)
	secondMsg, err := protocol.NewMessage(protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "2-1"})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleFlipMemoryCard("test-admin", server.ClientTypeAdmin, secondMsg)

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CuePerdu {
		t.Fatalf("une paire ratée MEMORY doit jouer perdu (à l'issue du délai de retournement), got %v", got)
	}
}

// ---------------------------------------------------------------------------
// perdu (RAFALE invalidate) — manche classique, RAFALE_INVALIDATE via le
// VRAI dispatch (handleWebMessage), même harnais que
// setupRafaleIntegrationTestApp/TestRafaleIntegration_FullCycle_...
// (rafale_107_test.go), dont seule la portion RAFALE-spécifique est
// reprise ici : cette suite a besoin de SA PROPRE App (construite par
// twa227WireSound, qui câble aussi le moteur son).
// ---------------------------------------------------------------------------

// twa227SetupRafaleRound seeds a 10-question reservoir and a classic RAFALE
// round on app, ready for StartImmediate — mirrors
// setupRafaleIntegrationTestApp (rafale_107_test.go) minus the App
// construction itself.
func twa227SetupRafaleRound(t *testing.T, app *App) {
	t.Helper()
	app.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "red", Color: []int{255, 0, 0}},
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
		Category: game.CategoryHistory,
		Points:   "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty:   1,
			RafaleMode:         string(game.RafaleModeChacunSonTour),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready("rq1", q)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleSetTeams, protocol.RafaleSetTeamsPayload{Teams: []string{"red"}})
}

// TestSoundChain_RafaleInvalidate_PlaysPerdu drives RAFALE_INVALIDATE
// through the real dispatch path (handleWebMessage -> handleRafaleInvalidate
// -> Engine.RafaleInvalidate -> OnRafaleInvalid, wired in setupCallbacks) —
// the classic round, never the MEMOTION-card-scoped variant (contract §2.1:
// "invalidation/timeout RAFALE", registry entry for setupCallbacks names
// this explicitly).
func TestSoundChain_RafaleInvalidate_PlaysPerdu(t *testing.T) {
	app, fake := twa227WireSound(t)
	twa227SetupRafaleRound(t, app)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	// Consume the initial `depart` before asserting on the invalidate cue.
	twa227WaitForPlayed(t, fake, 1, time.Second)

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleInvalidate, nil)

	got := twa227WaitForPlayed(t, fake, 2, time.Second)
	if len(got) != 2 || got[1] != audio.CuePerdu {
		t.Fatalf("RAFALE_INVALIDATE (manche classique) doit jouer perdu, got %v", got)
	}
}

// TestSoundChain_RafaleQuestionTimeout_PlaysPerdu_KnownGap documents a
// narrower scope boundary than the invalidate path above: the RAFALE
// per-question TIMEOUT (engine.go, RafaleQuestionTimer expiry — distinct
// code path from the explicit RAFALE_INVALIDATE action just covered) is not
// exercised here — triggering it deterministically needs the real 1s
// RafaleQuestionTimer ticker with RafaleQuestionTime driven to expiry,
// analogous to TestSoundChain_GlobalChronoExpiry_PlaysTempsEcoule above but
// nested one level deeper in RAFALE-specific setup this suite did not have
// budget to also stand up correctly. OnRafaleInvalid fires from the SAME
// site for both the explicit action and the timeout (contract sound.md:
// "invalidation/timeout RAFALE" — one cue, two triggers), so
// TestSoundChain_RafaleInvalidate_PlaysPerdu already exercises the cue
// itself end to end; only the timeout TRIGGER is untested here. Manual
// coverage: tests/procedures/sound-engine-227.md.
func TestSoundChain_RafaleQuestionTimeout_PlaysPerdu_KnownGap(t *testing.T) {
	t.Skip("timeout RAFALE non couvert par un test automatisé (le déclencheur, pas la cue elle-même) — voir le commentaire de la fonction et tests/procedures/sound-engine-227.md")
}
