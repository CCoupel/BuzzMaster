// Suite test-writer pour #228 (milestone v11.0 — plan de dev §1.6,
// _work/reports/plan-dev-228-229-20260921-114500.md Partie 0/1.6).
//
// Partie 0 du plan documente le trou réel de #227 : l'interface Output
// disait « Play MAY block » — une implémentation conforme pouvait rendre la
// main dès que les octets sont confiés au lecteur (démarrage asynchrone),
// exactement le bug du spike qui refermait le lecteur avant que le son ne
// soit rendu. #228 amende le contrat en « Play DOIT bloquer jusqu'à la fin
// du rendu » (contracts/sound.md, tâche A.1, dev-backend). Ce fichier
// verrouille la conséquence côté MOTEUR — déjà livré en #227, jamais
// modifié ici — qui est ce qui rend cette exigence vérifiable SANS
// matériel : si Play bloque D, le moteur (lecture strictement séquentielle,
// une seule goroutine, contract §5.3) ne doit jamais jouer la cue suivante
// avant que D ne soit écoulé — sinon la lecture séquentielle décrite en
// Partie 0 ("une rafale de cues s'étale au lieu de se superposer") ne tient
// plus, et le trou du spike redevient invisible à ce niveau aussi.
//
// Convention de collision : préfixe tw228 pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun ;
// il n'utilise que la surface publique déjà livrée par #227
// (Engine/Config/FakeOutput), jamais un champ interne, et ne modifie pas
// fake.go.
package audio

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

// tw228TimestampedOutput records the [start,end) interval of each Play call
// and blocks for Delay before returning — like FakeOutput.Delay, but with
// timestamps, which FakeOutput deliberately does not expose (its own
// contract is "records every Play call", not "records when"). Defined
// locally rather than by extending FakeOutput, so #227's own file/tests
// stay untouched.
type tw228TimestampedOutput struct {
	Delay time.Duration

	mu     sync.Mutex
	starts []time.Time
	ends   []time.Time
}

func (o *tw228TimestampedOutput) Play(ctx context.Context, pcm io.Reader) error {
	start := time.Now()
	o.mu.Lock()
	o.starts = append(o.starts, start)
	o.mu.Unlock()

	_, _ = io.ReadAll(pcm)
	select {
	case <-time.After(o.Delay):
	case <-ctx.Done():
		o.mu.Lock()
		o.ends = append(o.ends, time.Now())
		o.mu.Unlock()
		return ctx.Err()
	}

	o.mu.Lock()
	o.ends = append(o.ends, time.Now())
	o.mu.Unlock()
	return nil
}

func (o *tw228TimestampedOutput) Close() error { return nil }

// intervals returns a copy of the recorded [start,end) pairs, in call order.
func (o *tw228TimestampedOutput) intervals() (starts, ends []time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]time.Time(nil), o.starts...), append([]time.Time(nil), o.ends...)
}

// tw228WaitForStarts polls until Play has been ENTERED n times (started,
// not necessarily returned) or the timeout elapses.
func tw228WaitForStarts(t *testing.T, o *tw228TimestampedOutput, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s, _ := o.intervals()
		if len(s) >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	s, _ := o.intervals()
	t.Fatalf("Play entré %d fois après %s, attendu >= %d", len(s), timeout, n)
}

// tw228WaitForEnds polls until Play has RETURNED n times or the timeout
// elapses — unlike tw228WaitForStarts, this waits for completion.
func tw228WaitForEnds(t *testing.T, o *tw228TimestampedOutput, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, e := o.intervals()
		if len(e) >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	_, e := o.intervals()
	t.Fatalf("Play a retourné %d fois après %s, attendu >= %d", len(e), timeout, n)
}

// TestPlayContract_EngineNeverStartsNextCueBeforeCurrentOneFinishes is the
// #228 verification of Partie 0's exigence: a slow (but eventually
// returning) Output.Play must fully elapse before the engine's single
// playback goroutine starts the NEXT queued cue. This is exactly what makes
// "Play DOIT bloquer" a testable contract without any real audio hardware
// (plan §1.6, point 1) — a driver that returned early (the spike's bug)
// would show overlapping or back-to-back-without-gap intervals here.
func TestPlayContract_EngineNeverStartsNextCueBeforeCurrentOneFinishes(t *testing.T) {
	const delay = 40 * time.Millisecond
	out := &tw228TimestampedOutput{Delay: delay}
	e := NewEngine(Config{Output: out, QueueSize: 8})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Start(ctx)

	const n = 4
	for i := 0; i < n; i++ {
		if !e.PlayCue(CueGagne) {
			t.Fatalf("PlayCue #%d rejeté — file trop petite pour ce test (QueueSize=8)", i)
		}
	}

	tw228WaitForEnds(t, out, n, time.Second)
	starts, ends := out.intervals()
	if len(starts) != n || len(ends) != n {
		t.Fatalf("setup invalide : %d débuts / %d fins enregistrés, attendu %d", len(starts), len(ends), n)
	}

	for i := 1; i < n; i++ {
		gap := starts[i].Sub(ends[i-1])
		if gap < 0 {
			t.Fatalf("cue #%d a démarré %s AVANT la fin de la cue #%d — lecture séquentielle violée (contract Partie 0 : le moteur ne doit jamais chevaucher deux Play), starts=%v ends=%v",
				i, -gap, i-1, starts, ends)
		}
	}
	// Borne globale : le temps total ne peut pas être inférieur à n*delay si
	// chaque Play a réellement bloqué jusqu'au bout (à la latence de
	// dispatch près, largement dominée par `delay`).
	total := ends[n-1].Sub(starts[0])
	if total < time.Duration(n)*delay {
		t.Fatalf("durée totale mesurée %s < %d × %s — au moins un Play n'a pas bloqué jusqu'au bout (régression du trou #227 Partie 0)", total, n, delay)
	}
}

// TestPlayContract_ContextCancelDuringSlowPlayStopsWaitingPromptly proves
// the OTHER half of the same requirement: blocking is not unconditional —
// it MUST remain sensitive to context cancellation (plan §1.2 "L'attente
// doit rester sensible à l'annulation du contexte, pour que l'arrêt du
// serveur ne soit pas retardé"), otherwise "Play DOIT bloquer" would
// conflict with #227's own shutdown guarantee. Uses a Play that would
// otherwise take much longer than the engine's context lifetime.
func TestPlayContract_ContextCancelDuringSlowPlayStopsWaitingPromptly(t *testing.T) {
	const longDelay = 5 * time.Second
	out := &tw228TimestampedOutput{Delay: longDelay}
	e := NewEngine(Config{Output: out, QueueSize: 1})
	ctx, cancel := context.WithCancel(context.Background())
	go e.Start(ctx)

	if !e.PlayCue(CueDepart) {
		t.Fatal("setup invalide : PlayCue refusé")
	}
	tw228WaitForStarts(t, out, 1, time.Second) // laisse Play() entrer et s'engager dans son attente longue

	stopped := make(chan struct{})
	go func() {
		cancel()
		// Running() doit retomber à false rapidement une fois le contexte
		// annulé, sans attendre longDelay.
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if !e.Running() {
				close(stopped)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	select {
	case <-stopped:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("l'arrêt du moteur a été retardé au-delà de 500ms alors qu'un Play de %s était en cours — l'annulation du contexte doit interrompre l'attente immédiatement (plan §1.2)", longDelay)
	}
}
