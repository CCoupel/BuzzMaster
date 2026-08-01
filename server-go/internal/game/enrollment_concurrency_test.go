package game

import (
	"fmt"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : concurrence à l'inscription VJoueur (#120, couverture de l'analyse)
//
// L'analyse du plan #120 a établi qu'aucun verrou supplémentaire n'est requis
// à l'inscription : les messages WebSocket sont sérialisés par un canal à
// consommateur unique (main.go:290-293, un seul goroutine appelle
// handleWebMessage) et la séquence vérifier-puis-créer de
// ReconnectOrCreateVirtualPlayer est déjà atomique sous e.mu (fix R1, #109).
// Ces tests verrouillent ces deux propriétés contre une régression future en
// appelant directement l'API du moteur depuis de vraies goroutines
// concurrentes — si un futur refactor retirait le verrou englobant, ils
// deviendraient flaky/échoueraient.
// ---------------------------------------------------------------------------

// TestReconnectOrCreateVirtualPlayer_ConcurrentDistinctNames_AllSucceed
// verifies that N goroutines enrolling under N distinct names concurrently
// each get their own bumper with a distinct ID — no lost update, no
// duplicate/merged bumper.
func TestReconnectOrCreateVirtualPlayer_ConcurrentDistinctNames_AllSucceed(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.SetVirtualPlayerLimit(50)

	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make(map[string]bool)
	var errs []error

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", fmt.Sprintf("Player%d", i))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if reconnected {
				errs = append(errs, fmt.Errorf("Player%d: expected a new enrollment, got reconnected=true", i))
				return
			}
			ids[id] = true
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		t.Errorf("unexpected enrollment failure: %v", err)
	}
	if len(ids) != n {
		t.Fatalf("expected %d distinct bumper IDs, got %d: %v", n, len(ids), ids)
	}
	if got := e.CountVirtualPlayers(); got != n {
		t.Errorf("expected %d virtual players in the engine, got %d", n, got)
	}
}

// TestReconnectOrCreateVirtualPlayer_ConcurrentSameName_ExactlyOneSucceeds
// verifies that two goroutines racing to enroll the SAME name (no ID — a
// brand-new device, not a reconnection) never both succeed: the
// check-then-create sequence in ReconnectOrCreateVirtualPlayer is atomic
// under e.mu, so exactly one must win and the other must be rejected with
// NAME_TAKEN, regardless of goroutine scheduling.
func TestReconnectOrCreateVirtualPlayer_ConcurrentSameName_ExactlyOneSucceeds(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.SetVirtualPlayerLimit(50)

	const attempts = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	nameTakenRejections := 0
	var unexpected []error

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Homonym")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && !reconnected:
				successes++
			case err != nil:
				if enrollErr, ok := err.(*EnrollmentError); ok && enrollErr.Reason == "NAME_TAKEN" {
					nameTakenRejections++
				} else {
					unexpected = append(unexpected, err)
				}
			default:
				unexpected = append(unexpected, fmt.Errorf("unexpected outcome: reconnected=%v err=%v", reconnected, err))
			}
		}()
	}
	wg.Wait()

	for _, err := range unexpected {
		t.Errorf("unexpected outcome: %v", err)
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful enrollment for the contested name, got %d", successes)
	}
	if nameTakenRejections != attempts-1 {
		t.Errorf("expected %d NAME_TAKEN rejections, got %d", attempts-1, nameTakenRejections)
	}
	if got := e.CountVirtualPlayers(); got != 1 {
		t.Errorf("expected exactly 1 bumper to have been created for the contested name, got %d", got)
	}
}

// TestReconnectOrCreateVirtualPlayer_ConcurrentLastSlot_ExactlyOneSucceeds
// verifies the same atomicity guarantee for the enrollment-limit check: with
// exactly one free slot, two concurrent new-enrollment attempts must never
// both succeed — one wins the slot, the other is rejected with
// LIMIT_REACHED.
func TestReconnectOrCreateVirtualPlayer_ConcurrentLastSlot_ExactlyOneSucceeds(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.SetVirtualPlayerLimit(1) // exactly one free slot

	const attempts = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	limitReachedRejections := 0
	var unexpected []error

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Distinct names: isolates the limit check from the name-collision
			// check exercised by the sibling test above.
			_, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", fmt.Sprintf("Racer%d", i))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && !reconnected:
				successes++
			case err != nil:
				if enrollErr, ok := err.(*EnrollmentError); ok && enrollErr.Reason == "LIMIT_REACHED" {
					limitReachedRejections++
				} else {
					unexpected = append(unexpected, err)
				}
			default:
				unexpected = append(unexpected, fmt.Errorf("unexpected outcome: reconnected=%v err=%v", reconnected, err))
			}
		}(i)
	}
	wg.Wait()

	for _, err := range unexpected {
		t.Errorf("unexpected outcome: %v", err)
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful enrollment for the last free slot, got %d", successes)
	}
	if limitReachedRejections != attempts-1 {
		t.Errorf("expected %d LIMIT_REACHED rejections, got %d", attempts-1, limitReachedRejections)
	}
	if got := e.CountVirtualPlayers(); got != 1 {
		t.Errorf("expected exactly 1 bumper (the limit), got %d", got)
	}
}
