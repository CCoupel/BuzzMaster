// Suite contractuelle test-writer pour internal/audio (#227, milestone
// v11.0 — contracts/sound.md). Chaque test référence explicitement la
// section du contrat qu'il vérifie. Calqué sur
// internal/lighting/writer_test.go (#205), même démarche contract-first :
// écrit AVANT que le moteur (Lot B, dev-backend) ne soit livré — voir
// _work/reports/plan-dev-227-20260921-102500.md §6 (Batch 2, dev-backend et
// test-writer en parallèle sur le même répertoire de travail).
//
// Convention de collision : tout nom d'aide propre à ce fichier est préfixé
// twa (test-writer audio), pour ne jamais entrer en collision avec un
// éventuel fichier de tests dev-backend du même paquet (même convention que
// writer_test.go/writer_dev_test.go pour internal/lighting).
//
// Surface d'API ciblée (proposée à dev-backend en coordination directe,
// contract §6.2 délègue la spec du test-garde à Lot C mais pas les noms de
// symboles internes à internal/audio) :
//
//	type Cue string
//	type Output interface { Play(ctx, pcm io.Reader) error; Close() error }
//	type Config struct { Output Output; QueueSize int }
//	type Stats struct { Accepted, Dropped, Played int }
//	type Engine struct{ ... }
//	func NewEngine(cfg Config) *Engine
//	func (e *Engine) PlayCue(c Cue) (accepted bool)   // contract §5.1, signature déjà fixée
//	func (e *Engine) Start(ctx context.Context)        // contract §5.6, calqué AckManager
//	func (e *Engine) Enabled() bool
//	func (e *Engine) Running() bool
//	func (e *Engine) Stats() Stats
//	type FakeOutput struct{ ... }          // knobs Delay/Err/Gate optionnels,
//	                                       // non utilisés par ce fichier (qui
//	                                       // définit ses propres doubles
//	                                       // locaux twa* quand il a besoin de
//	                                       // détecter un appel "en vol")
//	func NewFakeOutput() *FakeOutput
//	func (f *FakeOutput) Count() int
//	func (f *FakeOutput) Closed() bool
//	func (f *FakeOutput) Played() []Cue   // décode le nom de la cue écrit en clair
//	                                       // dans le pcm par le moteur (aucun vrai
//	                                       // catalogue WAV n'existe avant #229) —
//	                                       // c'est le seul moyen pour ce test de
//	                                       // savoir QUELLE cue a atteint la sortie.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go/parser"
	"go/token"
)

// ---------------------------------------------------------------------------
// Aides propres à ce fichier.
// ---------------------------------------------------------------------------

// twaWaitFor interroge cond jusqu'à ce qu'elle soit vraie ou que le délai
// expire. Utilisé uniquement pour synchroniser avec la goroutine réelle du
// moteur — jamais pour fonder une assertion de timing métier.
func twaWaitFor(t *testing.T, timeout time.Duration, cond func() bool) {
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

// twaGatedOutput bloque Play() jusqu'à ce que le test appelle openOnce(), et
// signale started dès l'entrée dans Play() — pour les tests qui doivent
// garantir qu'un Play est bien "en vol" (donc que la file a commencé à se
// remplir derrière lui) avant d'agir. Calqué sur twlGatedDriver
// (internal/lighting/writer_test.go), défini localement pour ne dépendre
// que de l'interface Output fixée par le contrat §4, jamais d'une capacité
// spéculative de FakeOutput.
type twaGatedOutput struct {
	startedOnce sync.Once
	started     chan struct{}
	release     chan struct{}
}

func newTwaGatedOutput() *twaGatedOutput {
	return &twaGatedOutput{started: make(chan struct{}), release: make(chan struct{})}
}

func (g *twaGatedOutput) Play(ctx context.Context, pcm io.Reader) error {
	g.startedOnce.Do(func() { close(g.started) })
	_, _ = io.ReadAll(pcm)
	select {
	case <-g.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (g *twaGatedOutput) Close() error { return nil }
func (g *twaGatedOutput) openOnce()    { close(g.release) }

// twaSingleFlightOutput détecte un recouvrement d'appels Play (contract §5.3).
type twaSingleFlightOutput struct {
	inside int32 // atomic: >0 pendant l'exécution de Play
	calls  int32
	fail   atomic.Bool
}

func (o *twaSingleFlightOutput) Play(ctx context.Context, pcm io.Reader) error {
	if atomic.AddInt32(&o.inside, 1) > 1 {
		o.fail.Store(true)
	}
	atomic.AddInt32(&o.calls, 1)
	_, _ = io.ReadAll(pcm)
	time.Sleep(2 * time.Millisecond) // laisse la fenêtre de recouvrement s'ouvrir si le moteur est fautif
	atomic.AddInt32(&o.inside, -1)
	return nil
}

func (o *twaSingleFlightOutput) Close() error { return nil }

// ---------------------------------------------------------------------------
// contract §5.5 — nil-safety / disabled-safety (aucun goroutine, aucun appel
// matériel tant que le son n'est pas configuré).
// ---------------------------------------------------------------------------

func TestNilEngine_SafeNoOp(t *testing.T) {
	before := runtime.NumGoroutine()

	var e *Engine
	if e.Enabled() {
		t.Fatal("un Engine nil ne doit jamais être considéré comme activé")
	}
	if accepted := e.PlayCue(CueDepart); accepted {
		t.Fatal("PlayCue sur un Engine nil ne doit jamais être accepté")
	}
	e.Start(context.Background()) // doit retourner immédiatement, sans goroutine
	if got := e.Stats(); got.Accepted != 0 || got.Played != 0 || got.Dropped != 0 {
		t.Fatalf("Engine nil : Stats() doit rester à zéro, got %+v", got)
	}

	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("aucune goroutine ne doit être lancée pour un Engine nil (%d -> %d)", before, after)
	}
}

func TestDisabledEngine_NoGoroutineNoOutputCall(t *testing.T) {
	// contract §5.5 : "son désactivé ⇒ aucune goroutine lancée. Pas une
	// goroutine qui tourne à vide : aucune." Output nil est la façon dont ce
	// contrat désactive le moteur (même convention que lighting.Writer,
	// Driver nil ⇒ disabled).
	e := NewEngine(Config{}) // pas d'Output => désactivé
	if e.Enabled() {
		t.Fatal("un Engine sans Output doit être désactivé")
	}

	before := runtime.NumGoroutine()

	done := make(chan struct{})
	go func() { e.Start(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start() sur un Engine désactivé doit retourner immédiatement, sans boucle ni goroutine persistante")
	}

	if accepted := e.PlayCue(CueGagne); accepted {
		t.Fatal("PlayCue doit être refusé sur un Engine désactivé — jamais mis en file pour rien")
	}

	runtime.Gosched()
	time.Sleep(5 * time.Millisecond) // laisse une éventuelle goroutine fautive apparaître avant de mesurer
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("le nombre de goroutines a augmenté (%d -> %d) alors que l'Engine est désactivé", before, after)
	}
	if got := e.Stats(); got.Played != 0 {
		t.Fatalf("%d appel(s) à Output.Play alors que l'Engine est désactivé", got.Played)
	}
}

// ---------------------------------------------------------------------------
// contract §5.5 — pas de son parasite au démarrage : le premier front après
// le lancement ne doit rien jouer. Pour le moteur lui-même (par opposition
// à un site dérivé comme onPhaseStarted, couvert côté cmd/server), cela se
// traduit par : Start() seul, sans aucun PlayCue, ne produit aucun Play.
// ---------------------------------------------------------------------------

func TestStart_NoPlayCueCalled_NoSpuriousOutputCall(t *testing.T) {
	out := NewFakeOutput()
	e := NewEngine(Config{Output: out})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	time.Sleep(20 * time.Millisecond) // laisse le moteur tourner à vide
	if got := out.Count(); got != 0 {
		t.Fatalf("aucun son ne doit être joué avant le premier PlayCue explicite, got %d appel(s)", got)
	}
}

// ---------------------------------------------------------------------------
// contract §5.1 — entrée jamais bloquante, select{case ch<-v: default:}.
// ---------------------------------------------------------------------------

func TestPlayCue_NeverBlocks_EvenWithSaturatedQueueAndStuckOutput(t *testing.T) {
	out := newTwaGatedOutput() // jamais libéré pendant ce test : Play() reste bloqué
	e := NewEngine(Config{Output: out, QueueSize: 4})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	// Le premier PlayCue est consommé par la goroutine unique et s'y bloque
	// (release jamais fermé) : laisse-lui le temps d'entrer dans Play().
	if !e.PlayCue(CueDepart) {
		t.Fatal("le tout premier PlayCue doit être accepté")
	}
	select {
	case <-out.started:
	case <-time.After(time.Second):
		t.Fatal("le premier PlayCue n'a jamais atteint Output.Play — setup invalide")
	}

	// La file (capacité 4) doit maintenant se remplir puis saturer. Aucun de
	// ces appels ne doit jamais attendre : on mesure une borne de latence
	// large (10 ms) par appel — largement au-dessus du pire cas raisonnable
	// pour un simple select/channel, très en dessous d'un blocage réel.
	const attempts = 50
	accepted, rejected := 0, 0
	for i := 0; i < attempts; i++ {
		start := time.Now()
		ok := e.PlayCue(CueGagne)
		if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
			t.Fatalf("PlayCue #%d a mis %s — l'entrée ne doit JAMAIS bloquer (contract §5.1)", i, elapsed)
		}
		if ok {
			accepted++
		} else {
			rejected++
		}
	}
	if rejected == 0 {
		t.Fatal("setup invalide : la file n'a jamais saturé — augmenter `attempts` ou réduire QueueSize")
	}
	stats := e.Stats()
	if stats.Dropped < rejected {
		t.Errorf("Stats().Dropped (%d) doit compter au moins les %d rejets observés par PlayCue", stats.Dropped, rejected)
	}
}

// ---------------------------------------------------------------------------
// contract §5.2 — file FIFO bornée, JAMAIS de coalescence "dernier gagnant"
// (différence délibérée avec lighting.Writer). Deux cues distinctes
// arrivées proches doivent toutes deux ressortir, dans l'ordre.
// ---------------------------------------------------------------------------

func TestFIFOOrder_NoCoalescence_BothCuesSurvive(t *testing.T) {
	out := NewFakeOutput()
	e := NewEngine(Config{Output: out, QueueSize: 8})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	want := []Cue{CuePerdu, CueGagne, CueReveal}
	for _, c := range want {
		if !e.PlayCue(c) {
			t.Fatalf("PlayCue(%s) doit être accepté (file large, aucune saturation attendue)", c)
		}
	}

	twaWaitFor(t, time.Second, func() bool { return out.Count() >= len(want) })
	got := out.Played()
	if len(got) < len(want) {
		t.Fatalf("attendu au moins %d cues jouées, got %d: %v", len(want), len(got), got)
	}
	for i, c := range want {
		if got[i] != c {
			t.Fatalf("ordre FIFO violé (contract §5.2, pas de coalescence dernier-gagnant) : position %d attendu %s, got %s — séquence complète %v", i, c, got[i], got)
		}
	}
}

// ---------------------------------------------------------------------------
// contract §5.3 — une seule goroutine lit le périphérique : Output.Play
// n'est jamais invoqué en recouvrement.
// ---------------------------------------------------------------------------

func TestSingleGoroutine_OutputPlayNeverOverlaps(t *testing.T) {
	out := &twaSingleFlightOutput{}
	e := NewEngine(Config{Output: out, QueueSize: 16})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	var wg sync.WaitGroup
	cues := []Cue{CueDepart, CueGagne, CuePerdu, CueReveal, CueTempsEcoule, CueEntracteDebut, CueEntracteFin}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		c := cues[i%len(cues)]
		go func() {
			defer wg.Done()
			e.PlayCue(c)
		}()
	}
	wg.Wait()

	twaWaitFor(t, 2*time.Second, func() bool { return atomic.LoadInt32(&out.calls) > 0 })
	time.Sleep(50 * time.Millisecond) // laisse toute la file s'écouler
	if out.fail.Load() {
		t.Fatal("Output.Play a été invoqué en RECOUVREMENT — une seule goroutine doit lire le périphérique (contract §5.3)")
	}
}

// ---------------------------------------------------------------------------
// Sûreté d'accès concurrent — à exécuter sous `go test -race`
// (server-go/CLAUDE.md, `go test ./... -v -cover`, complété par -race pour
// cette suite précise vu la sensibilité concurrente du moteur, contract §5.4
// citant la cause racine #121).
// ---------------------------------------------------------------------------

func TestRace_ConcurrentPlayCueBurst(t *testing.T) {
	out := NewFakeOutput()
	e := NewEngine(Config{Output: out, QueueSize: 32})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	const goroutines = 8
	const perGoroutine = 200
	var wg sync.WaitGroup
	var totalAccepted, totalRejected int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if e.PlayCue(CueGagne) {
					atomic.AddInt64(&totalAccepted, 1)
				} else {
					atomic.AddInt64(&totalRejected, 1)
				}
			}
		}()
	}
	wg.Wait()

	sent := int64(goroutines * perGoroutine)
	if got := totalAccepted + totalRejected; got != sent {
		t.Fatalf("chaque PlayCue doit retourner accepted=true ou false, aucun troisième état : %d envoyés, %d comptés", sent, got)
	}
	twaWaitFor(t, 2*time.Second, func() bool {
		s := e.Stats()
		return int64(s.Played)+int64(s.Dropped) >= totalAccepted
	})
	stats := e.Stats()
	if int64(stats.Accepted) != totalAccepted {
		t.Errorf("Stats().Accepted (%d) doit correspondre aux %d PlayCue effectivement acceptés", stats.Accepted, totalAccepted)
	}
}

// ---------------------------------------------------------------------------
// contract §5.6 — cycle de vie calqué sur AckManager : l'annulation du ctx
// arrête la goroutine et ferme l'Output.
// ---------------------------------------------------------------------------

func TestContextCancel_StopsGoroutineAndClosesOutput(t *testing.T) {
	out := NewFakeOutput()
	e := NewEngine(Config{Output: out})
	ctx, cancel := context.WithCancel(context.Background())
	go e.Start(ctx)

	twaWaitFor(t, time.Second, func() bool { return e.Running() })
	cancel()
	twaWaitFor(t, time.Second, func() bool { return !e.Running() })
	twaWaitFor(t, time.Second, func() bool { return out.Closed() })
}

// ---------------------------------------------------------------------------
// contract §2.1 — package testable seul : aucun import de internal/game ni
// internal/lighting (calqué sur TestCA7_PackageImportsNothingFromBuzzcontrolExceptStdlib
// de internal/lighting/writer_test.go).
// ---------------------------------------------------------------------------

func TestPackageImportsNothingFromGameOrLighting(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser internal/audio")
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
			continue // seuls les fichiers non-test comptent (le paquet lui-même
			// doit rester testable seul ; ses tests peuvent importer ce qu'il faut)
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
		t.Fatal("aucun fichier .go non-test trouvé dans internal/audio — le scan est cassé")
	}
	if len(violations) > 0 {
		t.Errorf("contract §2.1 violé — internal/audio doit rester testable seul, sans dépendre du reste du serveur :\n%s",
			strings.Join(violations, "\n"))
	}
}
