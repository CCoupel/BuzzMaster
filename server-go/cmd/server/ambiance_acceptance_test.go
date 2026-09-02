// Suite d'acceptance test-writer pour #205 (milestone v10.0.0), au niveau
// cmd/server — chaque test référence explicitement le critère (CA1-CA7,
// _work/reports/planner-v10-plan-205-20260902-203000.md) qu'il vérifie.
//
// Complémentaire de ambiance_dev_test.go (dev-backend, qui couvre en détail
// la table de dérivation §6.2, la table de scènes §8 et le factoring de la
// palette d'équipe) et de ambiance_sites_test.go (CA3, test-writer). Ce
// fichier-ci n'en duplique aucun test — il couvre CA1, CA2, CA4 (parcours
// réel bout-en-bout), CA5 (au niveau App plutôt que du seul paquet
// internal/lighting), des cas CA6 non couverts par dev-backend, et CA7.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Tous les noms/aides ci-dessous sont préfixés tw205 pour ne jamais entrer
// en collision avec un helper déclaré ailleurs dans le paquet
// (ambiance_dev_test.go, testhelpers_test.go, etc.).
//
// API engine utilisée (vérifiée sur le code réel, pas supposée) : il
// n'existe PAS de Engine.SetState ni de gestion de "TIME"/"ANSWER_COLOR"
// dans Engine.UpdateBumper (seuls NAME/CONNECTED/TEAM/VERSION/IP/PROTOCOL/
// FIRMWARE_VERSION/IS_OUTDATED/OTA_STATUS/OTA_PERCENT/ACK_PENDING y sont
// gérés). La seule façon fiable de poser Bumper.Time/AnswerColor dans un
// test est de construire le champ Go directement et de (re)passer par
// Engine.SetBumpers (remplacement complet de la map, contract vérifié en
// lisant engine.go) — c'est le pattern déjà en place dans
// TestDevAmbianceDerivationTable (ambiance_dev_test.go), repris ici à
// l'identique sous un nom distinct.
package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// ---------------------------------------------------------------------------
// CA1 — non-régression LED stricte.
//
// "cmd/server/led_test.go et cmd/server/led_broadcast_132_test.go passent
// INCHANGÉS." Ceci n'est pas vérifiable par un nouveau test : c'est une
// propriété de DIFF (ces deux fichiers ne sont pas touchés par #205), jamais
// une propriété que du code exécuté à l'intérieur du process de test peut
// observer. La preuve est double et déjà disponible sans ajouter de test
// ici :
//  1. `git diff` sur ces deux fichiers entre la base de la branche
//     milestone/v10.0.0 et HEAD doit être vide — vérification à faire par
//     qa/deployer avant merge (voir tests/procedures/ambiance-lighting-
//     205.md, section "Non-régression").
//  2. Le fait même que `go test ./cmd/server/...` (ce paquet, incluant ces
//     deux fichiers non modifiés) compile et passe DANS CETTE MÊME suite
//     prouve qu'aucune signature qu'ils utilisent n'a bougé.
//
// Un "test" qui réexécuterait leurs assertions ne prouverait rien de plus
// que ce que `go test -run 'TestLEDSet|TestLEDBroadcast132'` prouve déjà en
// les exécutant tels quels — ajouter un troisième fichier qui les
// réimplémenterait irait à l'encontre de la règle de non-régression
// ("ne jamais modifier un test existant").
func TestCA1_NonRegressionLED_SeeProcedureForDiffCheck(t *testing.T) {
	t.Skip("CA1 est une propriété de diff (led_test.go / led_broadcast_132_test.go inchangés), pas une assertion runtime — voir tests/procedures/ambiance-lighting-205.md")
}

// ---------------------------------------------------------------------------
// CA2 — coût nul quand l'éclairage n'est pas configuré (contract §9 :
// "aucune goroutine, aucun appel matériel, aucune ligne de log").
//
// La preuve numérique par comptage de goroutines EST déjà faite, mais au
// niveau internal/lighting (TestCA2_* dans writer_test.go), où elle est
// fiable : appeler la vraie (*App).start() ici spécifiquement pour compter
// des goroutines serait à la fois lourd (ouvre de vrais ports HTTP/UDP/mDNS/
// DNS — aucun test existant du paquet ne le fait, voir testhelpers_test.go)
// et plus bruyant que révélateur (le nombre de goroutines du process entier
// varie pour des raisons sans rapport avec l'ambiance). La preuve
// structurelle ci-dessous est plus précise : elle établit que a.lighting
// reste nil après setupAmbiance(), ce qui — combiné à la garde explicite
// `if a.lighting != nil { go a.lighting.Start(a.ctx) }` de (*App).start()
// (cmd/server/main.go) — rend la ligne `go a.lighting.Start(...)`
// syntaxiquement inatteignable, sans avoir besoin d'exécuter start() pour
// le prouver à l'exécution.
// ---------------------------------------------------------------------------

func TestCA2_SetupAmbiance_NotConfigured_LeavesLightingNilAndCostsNothing(t *testing.T) {
	app := newTestApp(t)
	if app.lighting != nil {
		t.Fatal("setup invalide : une App fraîchement construite ne doit pas avoir de writer d'ambiance")
	}
	if app.ambianceIsConfigured() {
		t.Fatal("#205 : ambianceIsConfigured() doit renvoyer false (contract §9) — aucun pilote réel n'existe encore")
	}

	before := runtime.NumGoroutine()
	app.setupAmbiance()
	runtime.Gosched()
	after := runtime.NumGoroutine()

	if app.lighting != nil {
		t.Fatal("CA2 : setupAmbiance() a construit un writer alors qu'ambianceIsConfigured() est false")
	}
	if after > before {
		t.Errorf("CA2 : le nombre de goroutines a augmenté (%d -> %d) après setupAmbiance() alors que l'éclairage n'est pas configuré", before, after)
	}

	// Les 21 sites appellent a.lighting.Notify*() sans aucune garde `if` —
	// c'est le nil de a.lighting qui absorbe l'appel (contract §4.3). On le
	// vérifie directement ici plutôt que de le supposer.
	app.lighting.NotifyState()
	app.lighting.NotifyPulse(lighting.KindScore, []string{"TeamA"}, lighting.ScorePulseDuration)
}

// TestCA2_StartAmbianceLifecycleGuard_IsPresentInSource is a lightweight,
// text-anchored guard against the specific regression CA2 cares about: the
// `if a.lighting != nil { go a.lighting.Start(...) }` gate in (*App).start()
// being accidentally removed or turned unconditional in a future edit. It
// does not replace TestCA2_SetupAmbiance_NotConfigured_LeavesLightingNilAndCostsNothing
// above (which proves the ACTUAL cost is zero for #205's hardcoded
// unconfigured case) — it guards the OTHER half of CA2's guarantee: that the
// goroutine launch itself stays conditional, which matters once #207 makes
// ambianceIsConfigured() sometimes true.
var tw205LightingGuardRE = regexp.MustCompile(`if\s+a\.lighting\s*!=\s*nil\s*\{[^}]*go\s+a\.lighting\.Start\(`)

func TestCA2_StartAmbianceLifecycleGuard_IsPresentInSource(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué")
	}
	path := filepath.Join(filepath.Dir(thisFile), "main.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de %s : %v", path, err)
	}
	if !tw205LightingGuardRE.Match(content) {
		t.Fatal("CA2 : la garde conditionnelle 'if a.lighting != nil { go a.lighting.Start(...) }' est introuvable dans main.go — " +
			"le lancement de la goroutine d'ambiance doit rester conditionnel, jamais inconditionnel")
	}
}

// ---------------------------------------------------------------------------
// Aide commune CA6/CA4/CA5 : pose (phase, question, bumpers) sur le VRAI
// moteur, en respectant son API réelle (pas de Engine.SetState — vérifié
// inexistant). Repris du pattern de TestDevAmbianceDerivationTable
// (ambiance_dev_test.go), sous un nom distinct pour rester indépendant de ce
// fichier.
// ---------------------------------------------------------------------------

func tw205SetGame(t *testing.T, app *App, phase game.GamePhase, question *game.Question, bumpers map[string]*game.Bumper) {
	t.Helper()
	app.engine.SetEntracte(false)
	app.engine.SetPhase(game.PhaseStopped) // état de départ autorisé pour Ready()
	if question != nil {
		app.engine.Ready("tw205-q", question)
	}
	if bumpers != nil {
		app.engine.SetBumpers(bumpers)
	}
	app.engine.SetPhase(phase)
}

// ---------------------------------------------------------------------------
// CA6 — l'événement porte l'équipe concernée, conformément à la table §6.2.
// Complète TestDevAmbianceDerivationTable et TestDevAmbianceActiveTeam
// (ambiance_dev_test.go), qui ne couvrent pas le comportement actuel pour un
// REVEAL en dehors du QCM, ni l'égalité stricte de temps de buzz.
// ---------------------------------------------------------------------------

// TestCA6_Reveal_NonQCM_NeverCarriesTeams locks in the current, deliberate
// scope of ambianceCorrectTeams (cmd/server/ambiance.go): only QCM has a
// single well-defined "correct answer" concept at the REVEAL instant —
// MEMORY pairs, MEMOTION cards and RAFALE streaks already have their own
// per-event SCORE pulses (handleFlipMemoryCard, handleMotionDone,
// handleBumperPoints/handleTeamPoints). A REVEAL scene for these types
// therefore always shows "no team" (red, contract §8), even if a bumper's
// ANSWER_COLOR happens to equal a locally-meaningless "correct" value. This
// test pins today's behaviour so a future change is a deliberate contract
// amendment, not a silent regression — it does NOT assert this is the only
// valid design.
func TestCA6_Reveal_NonQCM_NeverCarriesTeams(t *testing.T) {
	app := newTestApp(t)
	bumpers := map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA", Time: 700, AnswerColor: game.AnswerColorRed},
	}
	for _, qt := range []game.QuestionType{game.QuestionTypeMemory, game.QuestionTypeMemotion, game.QuestionTypeRafale, game.QuestionTypeArdoise} {
		q := &game.Question{Type: qt, TypedContent: game.TypedContent{QCMCorrect: "RED"}} // même si QCMCorrect matche, le type n'est pas QCM
		tw205SetGame(t, app, game.PhaseRevealed, q, bumpers)

		got := app.deriveAmbianceEvent()
		if got.Kind != lighting.KindReveal || len(got.Teams) != 0 {
			t.Errorf("REVEAL pour %s doit rester sans équipe (Teams vide) — got %+v", qt, got)
		}
	}
}

// TestCA6_BuzzTeam_TieOnPressTime_NeverPanicsAndPicksOneTeam documents the
// behaviour of ambianceBuzzTeam when two bumpers share the exact same press
// Time (a real possibility: Bumper.Time has millisecond-ish resolution and
// two physical buzzers can tie). Strict `>` in the current implementation
// means the tie-break depends on Go's randomised map iteration order — NOT
// a documented rule. This test does not assert WHICH team wins (that would
// be flaky by construction); it asserts the derivation never panics and
// always returns exactly one of the two tied teams.
func TestCA6_BuzzTeam_TieOnPressTime_NeverPanicsAndPicksOneTeam(t *testing.T) {
	app := newTestApp(t)
	bumpers := map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA", Time: 5000},
		"m2": {Name: "m2", Team: "TeamB", Time: 5000},
	}
	tw205SetGame(t, app, game.PhasePaused, &game.Question{Type: game.QuestionTypeSpeedy}, bumpers)

	got := app.deriveAmbianceEvent()
	if got.Kind != lighting.KindBuzz {
		t.Fatalf("deux buzz à égalité doivent quand même produire BUZZ, got %+v", got)
	}
	if len(got.Teams) != 1 || (got.Teams[0] != "TeamA" && got.Teams[0] != "TeamB") {
		t.Fatalf("BUZZ doit désigner exactement une des deux équipes à égalité, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// CA7 — aucune dépendance ajoutée à server-go/go.mod.
//
// Référence figée AVANT #205 (relevé le 2026-09-02 sur milestone/v10.0.0,
// commit de départ 3f340416 "chore(version): Start v10.0.0.0"). #205 ne doit
// ajouter aucun NOUVEAU chemin de module — go/ast, go/parser, sync, time,
// context sont tous stdlib (contract CA7). Une évolution de VERSION d'une
// dépendance déjà présente n'est pas une violation de CA7 ; l'apparition
// d'un nouveau chemin de module l'est.
// ---------------------------------------------------------------------------

var tw205PreExistingModulePaths = map[string]bool{
	"github.com/anthropics/anthropic-sdk-go":                   true,
	"github.com/gorilla/websocket":                             true,
	"github.com/grandcat/zeroconf":                             true,
	"github.com/miekg/dns":                                     true,
	"golang.org/x/sys":                                         true,
	"github.com/bahlo/generic-list-go":                         true,
	"github.com/buger/jsonparser":                              true,
	"github.com/cenkalti/backoff":                              true,
	"github.com/invopop/jsonschema":                            true,
	"github.com/pb33f/ordered-map/v2":                          true,
	"github.com/standard-webhooks/standard-webhooks/libraries": true,
	"github.com/tidwall/gjson":                                 true,
	"github.com/tidwall/match":                                 true,
	"github.com/tidwall/pretty":                                true,
	"github.com/tidwall/sjson":                                 true,
	"go.yaml.in/yaml/v4":                                       true,
	"golang.org/x/mod":                                         true,
	"golang.org/x/net":                                         true,
	"golang.org/x/sync":                                        true,
	"golang.org/x/tools":                                       true,
}

// tw205ModulePathRE matches a require-block line's module path: leading
// whitespace, then a non-whitespace token, up to the version token. Simple
// line-oriented parsing is enough for go.mod's fixed grammar — no need for
// golang.org/x/mod/modfile (itself only an indirect dependency today;
// depending on it directly from a test would blur exactly the "no new
// dependency" signal CA7 checks).
var tw205ModulePathRE = regexp.MustCompile(`^\s*([A-Za-z0-9._\-/]+)\s+v[0-9]`)

func TestCA7_NoNewThirdPartyDependency(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué")
	}
	goModPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		t.Fatalf("lecture de %s : %v", goModPath, err)
	}
	defer f.Close()

	var found []string
	sc := bufio.NewScanner(f)
	inRequire := false
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "require (" {
			inRequire = true
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		if !inRequire {
			continue
		}
		m := tw205ModulePathRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		found = append(found, m[1])
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("lecture de %s : %v", goModPath, err)
	}
	if len(found) == 0 {
		t.Fatal("aucune dépendance trouvée dans go.mod — le parsing de ce test est cassé")
	}

	var newPaths []string
	for _, p := range found {
		if !tw205PreExistingModulePaths[p] {
			newPaths = append(newPaths, p)
		}
	}
	if len(newPaths) > 0 {
		t.Errorf("CA7 violé : nouvelle(s) dépendance(s) dans go.mod absente(s) de la référence pré-#205 : %s\n"+
			"  contracts/lighting.md CA7 : aucune dépendance ajoutée, go/ast/go/parser/sync/time/context suffisent.",
			strings.Join(newPaths, ", "))
	}
}

// ---------------------------------------------------------------------------
// CA4 (parcours réel) + intégration légère — "parcours d'une partie simulée
// (READY → START → buzz → REVEAL → points → STOP)" (plan, Tests Requis),
// asservi à la table de scènes §8, à travers le VRAI adaptateur (Derive =
// a.deriveAmbianceEvent, Scene = a.ambianceScene), pas une closure de test.
// Complémentaire de TestDevAmbianceWriterRendersLiveState
// (ambiance_dev_test.go), qui couvre PREPARE→READY puis un pulse SCORE
// isolé : ce test-ci parcourt le golden path complet jusqu'à STOP.
// ---------------------------------------------------------------------------

func TestIntegration_GoldenPath_SceneSequenceMatchesTable(t *testing.T) {
	app := newTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
	})
	fake := lighting.NewFakeDriver()
	app.lighting = app.newAmbianceWriter(fake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.lighting.Start(ctx)

	// tw205Step attend le CONTENU attendu dans le dernier State appliqué,
	// jamais un compte d'Apply précis : le vrai MinInterval (100 ms, non
	// injecté ici — contrairement à internal/lighting/writer_test.go où il
	// l'est) peut coalescer les notifications de deux étapes rapprochées en
	// un seul Apply, ce qui rendrait un compteur "+1 par étape" flaky. Ce
	// qui compte pour CA4/l'intégration légère est que l'état FINALEMENT
	// rendu, une fois le régime établi, corresponde à la table §8 — pas le
	// nombre exact d'Apply intermédiaires.
	tw205Step := func(t *testing.T, phase game.GamePhase, question *game.Question, bumpers map[string]*game.Bumper, wantColor [3]int, wantIntensity int) {
		t.Helper()
		tw205SetGame(t, app, phase, question, bumpers)
		app.lighting.NotifyState()
		tw205WaitFor(t, 2*time.Second, func() bool {
			last, ok := fake.Last()
			return ok && len(last.Zones) == 1 && last.Zones[0].Color == wantColor && last.Zones[0].Intensity == wantIntensity
		})
	}

	qcm := &game.Question{Type: game.QuestionTypeQCM}
	noBuzz := map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA"},
		"m2": {Name: "m2", Team: "TeamB"},
	}

	// READY (PREPARE)
	tw205Step(t, game.PhasePrepare, qcm, noBuzz, [3]int{255, 255, 255}, 200)

	// RUNNING (STARTED, question classique sans équipe active)
	tw205Step(t, game.PhaseStarted, qcm, noBuzz, [3]int{40, 90, 255}, 160)

	// BUZZ (m1/TeamA presse en premier)
	buzzed := map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA", Time: 1000},
		"m2": {Name: "m2", Team: "TeamB"},
	}
	wantTeamA := app.teamNameToRGB("TeamA")
	tw205Step(t, game.PhasePaused, qcm, buzzed, wantTeamA, 255)

	// REVEAL (m1 a répondu correctement)
	qcmRed := &game.Question{Type: game.QuestionTypeQCM, TypedContent: game.TypedContent{QCMCorrect: "RED"}}
	answered := map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA", Time: 1000, AnswerColor: game.AnswerColorRed},
		"m2": {Name: "m2", Team: "TeamB"},
	}
	tw205Step(t, game.PhaseRevealed, qcmRed, answered, [3]int{0, 220, 60}, 255)

	// Points attribués à TeamA — pulse SCORE. Doit survivre au MinInterval
	// réel de 100 ms qui suit l'Apply de REVEAL, sans quoi il expirerait
	// avant même d'être rendu (même piège documenté par dev-backend dans
	// TestDevAmbianceWriterRendersLiveState : 300 ms, pas 4800 ms, pour
	// garder le test rapide tout en restant sûr).
	app.lighting.NotifyPulse(lighting.KindScore, []string{"TeamA"}, 300*time.Millisecond)
	tw205WaitFor(t, 2*time.Second, func() bool {
		last, ok := fake.Last()
		return ok && len(last.Zones) == 1 && last.Zones[0].Color == wantTeamA && last.Zones[0].Intensity == 255
	})
	// Retombe seul sur son échéance, sans nouvelle notification externe —
	// l'état vivant est toujours PhaseRevealed avec les mêmes réponses, donc
	// la scène de repli est la MÊME scène REVEAL bonne réponse, pas STOP
	// (qui n'arrive qu'à l'étape suivante).
	tw205WaitFor(t, 2*time.Second, func() bool {
		last, ok := fake.Last()
		return ok && len(last.Zones) == 1 && last.Zones[0].Color == [3]int{0, 220, 60} && last.Zones[0].Intensity == 255
	})

	// STOP
	tw205Step(t, game.PhaseStopped, nil, answered, [3]int{255, 214, 170}, 120)

	cancel()
	tw205WaitFor(t, time.Second, fake.Closed)
}

// ---------------------------------------------------------------------------
// CA5 (niveau App) — callbacks Engine invoqués hors lock depuis ~12 sites
// sans sérialisation (contract §5, engine.go:220-241, cause racine #121).
// Complémentaire de TestCA5_ConcurrentNotifyStateAndPulse_RaceFree
// (internal/lighting/writer_test.go, paquet pur) : ce test-ci fait
// concurremment MUTER le GameState vivant (comme le feraient de vrais
// callbacks Engine non sérialisés) EN MÊME TEMPS que des Notify*(), à
// travers le vrai adaptateur qui relit ce même GameState au moment de
// l'Apply — exactement le scénario que le contrat §5 met en garde. À
// exécuter avec `go test -race`.
// ---------------------------------------------------------------------------

func TestCA5_ConcurrentEngineMutationAndNotify_RaceFree(t *testing.T) {
	app := newTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 255, 0}},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA"},
		"m2": {Name: "m2", Team: "TeamB"},
	})
	fake := lighting.NewFakeDriver()
	app.lighting = app.newAmbianceWriter(fake)
	ctx, cancel := context.WithCancel(context.Background())
	go app.lighting.Start(ctx)

	const goroutines, perGoroutine = 8, 150
	var wg sync.WaitGroup
	phases := []game.GamePhase{game.PhasePrepare, game.PhaseStarted, game.PhasePaused, game.PhaseRevealed, game.PhaseStopped}
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				switch (i + g) % 4 {
				case 0:
					app.lighting.NotifyState()
				case 1:
					app.lighting.NotifyPulse(lighting.KindScore, []string{"TeamA"}, 2*time.Millisecond)
				case 2:
					app.engine.SetPhase(phases[(i+g)%len(phases)])
					app.lighting.NotifyState()
				default:
					app.engine.SetBumpers(map[string]*game.Bumper{
						"m1": {Name: "m1", Team: "TeamA", Time: int64(i + g)},
						"m2": {Name: "m2", Team: "TeamB"},
					})
				}
			}
		}(g)
	}
	wg.Wait()

	tw205WaitFor(t, 2*time.Second, func() bool { return fake.Count() >= 1 })
	cancel()
	tw205WaitFor(t, 2*time.Second, fake.Closed)
}

// ---------------------------------------------------------------------------
// Aides propres à ce fichier (préfixées tw205 pour ne jamais entrer en
// collision avec un helper déclaré dans un autre fichier de test du même
// paquet, notamment ambiance_dev_test.go's waitForCount).
// ---------------------------------------------------------------------------

func tw205WaitForCount(t *testing.T, f *lighting.FakeDriver, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.Count() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("le driver a reçu %d état(s), attendu >= %d", f.Count(), n)
}

func tw205WaitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition non atteinte après %s", timeout)
	}
}
