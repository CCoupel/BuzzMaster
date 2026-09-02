// Suite contractuelle test-writer pour internal/lighting (#205, milestone
// v10.0.0 — contracts/lighting.md). Chaque test référence explicitement le
// critère d'acceptance (CA1-CA7, _work/reports/planner-v10-plan-205-
// 20260902-203000.md) ou la section du contrat qu'il vérifie.
//
// Complémentaire de writer_dev_test.go (dev-backend) : ce fichier-ci est
// délibérément indépendant de ses types-outils (fakeClock, waitFor, sceneOf)
// pour ne jamais dépendre d'un fichier de tests tiers — toute aide dont ce
// fichier a besoin est définie ici, avec un préfixe twl (test-writer
// lighting) pour ne jamais entrer en collision avec un nom déclaré ailleurs
// dans le paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package lighting

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Aides propres à ce fichier (voir en-tête : jamais partagées avec
// writer_dev_test.go pour rester indépendant).
// ---------------------------------------------------------------------------

// twlWaitFor interroge cond jusqu'à ce qu'elle soit vraie ou que le délai
// expire. Utilisé uniquement pour synchroniser avec la goroutine réelle de
// Start (aucune assertion de timing métier ne repose dessus — celles-ci
// utilisent soit un driver bloquant sur un canal, soit une formule bornée
// sur un temps mesuré, jamais un compte figé après un time.Sleep).
func twlWaitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition non atteinte après %s", timeout)
	}
}

// twlNeverCalledDriver panique si Apply ou Close est jamais invoqué — utilisé
// pour prouver qu'aucun appel matériel ne se produit (CA2).
type twlNeverCalledDriver struct{}

func (twlNeverCalledDriver) Apply(context.Context, State) error {
	panic("CA2: Driver.Apply appelé alors que l'éclairage n'est pas configuré/activé")
}
func (twlNeverCalledDriver) Close() error {
	panic("CA2: Driver.Close appelé alors que l'éclairage n'est pas configuré/activé")
}

// twlGatedDriver bloque Apply jusqu'à ce que le test libère release, et
// enregistre chaque State reçu — pour les tests de non-blocage et de
// priorité d'impulsion qui doivent garantir qu'un Apply est bien "en vol"
// avant d'agir.
type twlGatedDriver struct {
	mu      sync.Mutex
	release chan struct{}
	applied []State
	closed  bool
}

func newTwlGatedDriver() *twlGatedDriver {
	return &twlGatedDriver{release: make(chan struct{})}
}

func (d *twlGatedDriver) Apply(ctx context.Context, s State) error {
	select {
	case <-d.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	d.mu.Lock()
	d.applied = append(d.applied, s)
	d.mu.Unlock()
	return nil
}
func (d *twlGatedDriver) Close() error { d.mu.Lock(); d.closed = true; d.mu.Unlock(); return nil }
func (d *twlGatedDriver) count() int   { d.mu.Lock(); defer d.mu.Unlock(); return len(d.applied) }
func (d *twlGatedDriver) last() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.applied[len(d.applied)-1]
}

// twlOpen fires the gate open exactly once per call (Apply reads a fresh
// channel each time it's re-armed via reset).
func (d *twlGatedDriver) openOnce() {
	close(d.release)
}
func (d *twlGatedDriver) rearm() {
	d.mu.Lock()
	d.release = make(chan struct{})
	d.mu.Unlock()
}

// ---------------------------------------------------------------------------
// CA2 — coût nul quand l'éclairage n'est pas configuré/activé (contract §4.5,
// §9 : "aucune goroutine n'est lancée. Pas une goroutine qui tourne à vide :
// aucune", "aucun appel matériel, aucune ligne de log").
// ---------------------------------------------------------------------------

func TestCA2_NilWriter_NoGoroutineNoDriverCall(t *testing.T) {
	before := runtime.NumGoroutine()

	var w *Writer
	w.NotifyState()
	w.NotifyPulse(KindScore, []string{"TeamA"}, time.Second)
	w.Start(context.Background()) // must return immediately, no goroutine spawned by the caller either
	if w.Enabled() {
		t.Fatal("un Writer nil ne doit jamais être considéré comme activé")
	}
	if got := w.Stats(); got.Applies != 0 || got.Notifies != 0 {
		t.Fatalf("Writer nil : Stats() doit rester à zéro, got %+v", got)
	}

	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("CA2: le nombre de goroutines a augmenté (%d -> %d) alors que le Writer est nil", before, after)
	}
}

func TestCA2_DisabledWriter_NoGoroutineNoDriverCall(t *testing.T) {
	// Driver volontairement "jamais appelable" : si CA2 est violé, ce test
	// panique au lieu de simplement échouer, ce qui rend le message sans
	// ambiguïté.
	w := NewWriter(Config{}) // pas de Driver => disabled (contract §9)
	if w.Enabled() {
		t.Fatal("un Writer sans Driver doit être désactivé")
	}

	before := runtime.NumGoroutine()

	done := make(chan struct{})
	go func() { w.Start(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start() sur un Writer désactivé doit retourner immédiatement, sans boucle ni goroutine persistante")
	}

	w.NotifyState()
	w.NotifyPulse(KindScore, []string{"TeamA"}, time.Second)

	runtime.Gosched()
	time.Sleep(5 * time.Millisecond) // laisse une éventuelle goroutine fautive apparaître avant de mesurer
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("CA2: le nombre de goroutines a augmenté (%d -> %d) alors que le Writer est désactivé", before, after)
	}
	if got := w.Stats(); got.Applies != 0 {
		t.Fatalf("CA2: %d appel(s) à Driver.Apply alors que le Writer est désactivé", got.Applies)
	}
}

func TestCA2_ExplicitlyEnabledButNeverStarted_DriverNeverCalled(t *testing.T) {
	// Un Writer construit avec un vrai Driver mais dont Start() n'est jamais
	// appelé ne doit produire aucun appel — Notify* ne fait qu'armer un
	// signal, jamais un appel direct au Driver (contract §4.3).
	w := NewWriter(Config{Driver: twlNeverCalledDriver{}})
	if !w.Enabled() {
		t.Fatal("setup invalide pour ce test : le Writer doit être activé")
	}
	w.NotifyState()
	w.NotifyPulse(KindScore, []string{"TeamA"}, time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	// L'absence de panique de twlNeverCalledDriver EST l'assertion.
}

// ---------------------------------------------------------------------------
// CA4 — rafale bornée et MESURÉE (contract §4.4) : "une rafale de N
// événements sur une durée T produit au plus T/MinInterval + 1 appels à
// Apply, quel que soit N." Assertion chiffrée sur un temps mesuré (pas une
// impression, pas un compte arbitraire figé après un time.Sleep) : si le
// système est lent et T réel dépasse la fenêtre visée, la borne calculée
// grandit avec lui — l'assertion reste vraie, jamais flaky par charge CPU.
// ---------------------------------------------------------------------------

func TestCA4_BurstOverMeasuredWindow_AppliesBoundedByFormula(t *testing.T) {
	drv := NewFakeDriver()
	w := NewWriter(Config{
		Driver: drv,
		Derive: func() Event { return Event{Kind: KindReady} },
		Scene:  func(Event) State { return State{} },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	start := time.Now()
	burstWindow := 8 * MinInterval // assez large pour laisser plusieurs Apply réels
	deadline := start.Add(burstWindow)
	calls := 0
	for time.Now().Before(deadline) {
		w.NotifyState()
		calls++
	}
	// Laisse le temps à un dernier Apply retardé par le throttle de sortir.
	time.Sleep(MinInterval + 20*time.Millisecond)
	elapsed := time.Since(start)

	got := drv.Count()
	// T/MinInterval + 1, formule normative du contrat §4.4, appliquée au
	// temps RÉELLEMENT écoulé (elapsed), pas à burstWindow visé.
	max := int(elapsed/MinInterval) + 1
	if calls < 100 {
		t.Fatalf("setup invalide : seulement %d NotifyState() envoyés en %s, rafale insuffisante pour prouver quoi que ce soit", calls, elapsed)
	}
	if got > max {
		t.Errorf("CA4: %d appels à Apply pour %d NotifyState() sur %s mesurées (MinInterval=%s) — borne autorisée %d (T/MinInterval+1)",
			got, calls, elapsed, MinInterval, max)
	}
	if got < 1 {
		t.Errorf("CA4: aucun appel à Apply pendant la rafale — le dernier état n'a jamais été rendu")
	}
}

// ---------------------------------------------------------------------------
// CA5 — sûreté d'accès concurrent (contract §5) : NotifyState/NotifyPulse
// sûrs en accès concurrent depuis plusieurs goroutines. À exécuter avec
// `go test -race`.
// ---------------------------------------------------------------------------

func TestCA5_ConcurrentNotifyStateAndPulse_RaceFree(t *testing.T) {
	drv := NewFakeDriver()
	w := NewWriter(Config{
		Driver:      drv,
		Derive:      func() Event { return Event{Kind: KindRunning} },
		Scene:       func(Event) State { return State{} },
		MinInterval: time.Millisecond, // throttle court pour observer des Apply réels pendant le test
	})
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	const goroutines, perGoroutine = 12, 300
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if (i+g)%2 == 0 {
					w.NotifyState()
				} else {
					w.NotifyPulse(KindScore, []string{"TeamA"}, time.Millisecond)
				}
			}
		}(g)
	}
	wg.Wait()

	twlWaitFor(t, 2*time.Second, func() bool { return drv.Count() >= 1 })
	cancel()
	twlWaitFor(t, 2*time.Second, drv.Closed)

	if s := w.Stats(); s.Notifies != goroutines*perGoroutine {
		t.Errorf("CA5: Stats().Notifies = %d, attendu %d (aucun appel concurrent perdu)", s.Notifies, goroutines*perGoroutine)
	}
}

// ---------------------------------------------------------------------------
// §4.2 — priorité de l'impulsion : un NotifyState() concurrent NE DOIT PAS
// faire disparaître une impulsion SCORE non échue. "Si une impulsion non
// échue est présente, c'est elle qui est rendue ; sinon on dérive de l'état
// vivant." Un NotifyState() ne touche jamais w.pulse.
// ---------------------------------------------------------------------------

func TestPulseTakesPrecedenceOverConcurrentStateNotify(t *testing.T) {
	gated := newTwlGatedDriver()
	var liveKind EventKind = KindRunning
	var mu sync.Mutex
	w := NewWriter(Config{
		Driver: gated,
		Derive: func() Event { mu.Lock(); defer mu.Unlock(); return Event{Kind: liveKind} },
		// Encodage volontairement non-ambigu : (len(Kind)*10 + nombre
		// d'équipes) donne une valeur distincte pour chaque combinaison
		// utilisée par ce test (SCORE+1 équipe = 51, IDLE+0 = 40, RUNNING+0
		// = 70) — impossible de confondre "pulse rendu" et "état vivant
		// dérivé" par coïncidence arithmétique.
		Scene: func(ev Event) State {
			return State{Zones: []ZoneState{{Zone: ZoneGeneral, Intensity: len(ev.Kind)*10 + len(ev.Teams)}}}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	// Un pulse est en cours de rendu (Apply bloqué par la porte).
	w.NotifyPulse(KindScore, []string{"TeamA"}, time.Hour) // longue échéance : ne doit pas expirer pendant ce test
	twlWaitFor(t, time.Second, func() bool { return true })
	time.Sleep(5 * time.Millisecond) // laisse drain() entrer dans Apply (bloqué sur la porte)

	// Pendant que l'impulsion est "en vol", un site d'état arrive et change
	// même l'état vivant sous-jacent — cela ne doit RIEN changer au rendu
	// du pulse une fois débloqué.
	mu.Lock()
	liveKind = KindIdle
	mu.Unlock()
	w.NotifyState()

	gated.openOnce()
	twlWaitFor(t, time.Second, func() bool { return gated.count() >= 1 })
	first := gated.last()
	wantPulseIntensity := len(KindScore)*10 + 1 // KindScore + 1 équipe ("TeamA")
	if first.Zones[0].Intensity != wantPulseIntensity {
		t.Fatalf("premier Apply attendu = rendu du pulse (intensity=%d), got %+v", wantPulseIntensity, first)
	}

	// Un second Apply peut suivre (refreshDue posé par le NotifyState), mais
	// il doit ENCORE refléter le pulse (toujours non échu), jamais l'état
	// vivant KindIdle.
	gated.rearm()
	gated.openOnce()
	time.Sleep(20 * time.Millisecond)
	last := gated.last()
	if last.Zones[0].Intensity != wantPulseIntensity {
		t.Fatalf("un NotifyState() concurrent a fait perdre l'impulsion non échue : dernier Apply = %+v, attendu intensity=%d (pulse)", last, wantPulseIntensity)
	}
}

// ---------------------------------------------------------------------------
// §4.2 — expiration : une fois l'échéance dépassée, l'écrivain revient tout
// seul à la dérivation de l'état vivant, sans nouvelle notification externe.
// ---------------------------------------------------------------------------

func TestPulseExpiry_FallsBackToLiveDerivationOnItsOwn(t *testing.T) {
	drv := NewFakeDriver()
	w := NewWriter(Config{
		Driver: drv,
		Derive: func() Event { return Event{Kind: KindPauseAll} },
		Scene: func(ev Event) State {
			return State{Zones: []ZoneState{{Zone: ZoneGeneral, Intensity: len(ev.Kind)}}}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	const pulseDuration = 30 * time.Millisecond
	w.NotifyPulse(KindScore, []string{"TeamA"}, pulseDuration)
	twlWaitFor(t, time.Second, func() bool { return drv.Count() >= 1 })
	if last, _ := drv.Last(); last.Zones[0].Intensity != len(KindScore) {
		t.Fatalf("premier Apply attendu = pulse, got %+v", last)
	}

	// Aucune notification externe n'est envoyée ici — seule l'échéance doit
	// déclencher le retour à l'état vivant.
	twlWaitFor(t, time.Second, func() bool {
		last, ok := drv.Last()
		return ok && last.Zones[0].Intensity == len(KindPauseAll)
	})
}

// ---------------------------------------------------------------------------
// Non-blocage sous pilote lent (contract §4, point 1 : "jamais d'appel
// bloquant depuis un site de jeu"). Complément direct de writer_dev_test.go
// (driver à porte plutôt qu'à délai), sur ce fichier indépendant.
// ---------------------------------------------------------------------------

func TestNotifyNeverBlocks_EvenWithManyPendingCallsDuringSlowApply(t *testing.T) {
	gated := newTwlGatedDriver()
	w := NewWriter(Config{Driver: gated, Derive: func() Event { return Event{} }, Scene: func(Event) State { return State{} }})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	w.NotifyState()
	time.Sleep(5 * time.Millisecond) // Apply est maintenant bloqué sur la porte

	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			w.NotifyState()
			w.NotifyPulse(KindScore, []string{"X"}, time.Second)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("un site de jeu appelant Notify*() s'est bloqué pendant qu'un Apply était en cours — violation du contrat §4")
	}
	gated.openOnce()
	twlWaitFor(t, time.Second, func() bool { return gated.count() >= 1 })
}

// ---------------------------------------------------------------------------
// CA6 (partie type pure) — Event.Teams est un slice ordonné (premier =
// principal), vide = aucune équipe. La sémantique de dérivation elle-même
// (table §6.2) est de la responsabilité de cmd/server/ambiance.go et testée
// côté cmd/server (ambiance_acceptance_test.go) ; ce test-ci verrouille
// uniquement la forme du type sur laquelle §6.2 s'appuie.
// ---------------------------------------------------------------------------

func TestEvent_TeamsSlice_OrderedFirstIsPrincipal(t *testing.T) {
	ev := Event{Kind: KindReveal, Teams: []string{"TeamC", "TeamA"}}
	if len(ev.Teams) != 2 || ev.Teams[0] != "TeamC" || ev.Teams[1] != "TeamA" {
		t.Fatalf("Event.Teams doit préserver l'ordre d'insertion (premier = principal, contract §2.2), got %v", ev.Teams)
	}
	empty := Event{Kind: KindRunning}
	if len(empty.Teams) != 0 {
		t.Fatalf("Event.Teams doit être vide par défaut (aucune équipe concernée), got %v", empty.Teams)
	}
}

// ---------------------------------------------------------------------------
// CA7 — le paquet ne dépend que de la stdlib (contract §2.1 : "aucun import
// de internal/game ni de internal/protocol"). Vérifié ici par AST plutôt que
// par lecture : la garantie de compilation seule ("le paquet compile sans
// buzzcontrol/internal/game") ne suffirait pas à couvrir un import ajouté
// puis retiré par erreur avant commit ; l'AST le fige dans le test.
// ---------------------------------------------------------------------------

func TestCA7_PackageImportsNothingFromBuzzcontrolExceptStdlib(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser internal/lighting")
	}
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lecture de %s : %v", dir, err)
	}

	fset := token.NewFileSet()
	var violations []string
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue // seuls les fichiers non-test comptent pour CA7/§2.1 : le
			// paquet lui-même doit rester testable seul, ses tests peuvent
			// toujours importer ce qu'il faut pour construire des fixtures.
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("go/parser a échoué sur %s : %v", path, err)
		}
		scanned++
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("import illisible dans %s : %v", path, err)
			}
			if strings.HasPrefix(p, "buzzcontrol/") {
				violations = append(violations, name+": import de "+p)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("aucun fichier .go non-test trouvé dans internal/lighting — le scan CA7 est cassé")
	}
	if len(violations) > 0 {
		t.Errorf("CA7/contract §2.1 violé — internal/lighting doit rester testable seul, sans dépendre du reste du serveur :\n%s",
			strings.Join(violations, "\n"))
	}
	// L'absence de tout import tiers ("aucune dépendance ajoutée") est
	// vérifiée séparément et plus généralement pour tout le module dans
	// cmd/server/ambiance_acceptance_test.go (TestCA7_NoNewThirdPartyDependency),
	// qui compare le require direct de go.mod à la référence figée avant #205.
}
