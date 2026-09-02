package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : écriture par lot du réservoir RAFALE — Engine.AppendRafaleQuestions
// (issue #203, milestone v8.1.0, contrat contracts/rafale-ai-generation.md
// §8 "Persistance — écriture par lot, une seule sauvegarde", plan
// _work/reports/plan-20260901-162105.md tâche 1).
//
// Écrit contract-first, en parallèle de dev-backend (Batch 1) — la méthode
// elle-même (server-go/internal/game/rafale_store.go) est livrée par
// dev-backend dans le même batch ; ce fichier vérifie son comportement
// RÉEL au fur et à mesure, pas une spéculation.
//
// Signature attendue (contrat §8) :
//
//	func (e *Engine) AppendRafaleQuestions(qs []RafaleQuestion) ([]RafaleQuestion, error)
//
// Règles vérifiées ici :
//   - IDs "r-NNN" alloués séquentiellement, sans collision avec l'existant ;
//   - ajout PUR : aucune entrée existante n'est modifiée ni supprimée ;
//   - tout ID fourni en entrée est ignoré (l'ID est TOUJOURS alloué serveur,
//     même garantie que UpsertRafaleQuestion(q.ID != "") mais inversée : ici
//     un ID non-vide fourni par l'appelant ne doit jamais être respecté —
//     contrairement à Upsert, ce chemin n'a pas de notion de "mise à jour") ;
//   - UNE SEULE sauvegarde disque par lot — vérifié par une propriété plus
//     forte et directement observable que "compter les appels à SaveRafale"
//     (impossible à instrumenter proprement depuis l'extérieur du paquet) :
//     un lecteur concurrent (LoadRafale sur un second Engine pointé sur le
//     même fichier) ne doit JAMAIS observer un état partiel — soit le compte
//     d'AVANT le lot, soit celui d'APRÈS, jamais un compte intermédiaire.
//     C'est exactement la propriété que l'écriture atomique + le
//     Lock→insérer tout→Unlock→Save unique du contrat sont censés garantir ;
//   - aucun deadlock avec un appel concurrent à UpsertRafaleQuestion (le
//     RWMutex n'est pas réentrant, SaveRafale() prend RLock — contrat §8
//     "garde contre l'inversion Lock/SaveRafale", risque R5 du plan).
// ---------------------------------------------------------------------------

func TestAppendRafaleQuestions_AllocatesSequentialIDsWithoutCollision(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Existing 1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-003", Question: "Existing 3", Answer: "A3", Category: CategoryHistory, Difficulty: 1}, // gap at r-002
	})

	batch := []RafaleQuestion{
		{Question: "New 1", Answer: "NA1", Category: CategoryHistory, Difficulty: 1},
		{Question: "New 2", Answer: "NA2", Category: CategoryHistory, Difficulty: 2},
		{Question: "New 3", Answer: "NA3", Category: CategoryScience, Difficulty: 3},
	}

	stored, err := e.AppendRafaleQuestions(batch)
	if err != nil {
		t.Fatalf("AppendRafaleQuestions failed: %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("expected 3 stored questions, got %d", len(stored))
	}

	seen := make(map[string]bool)
	for _, q := range stored {
		if q.ID == "" {
			t.Fatalf("stored question has empty ID: %+v", q)
		}
		if seen[q.ID] {
			t.Fatalf("duplicate ID allocated within the same batch: %q", q.ID)
		}
		seen[q.ID] = true
		// The highest pre-existing numeric suffix is 3 (r-003) — new IDs must
		// continue strictly above it, never reuse the r-002 gap (contract §8:
		// "nextRafaleIDUnsafe appliqué de façon incrémentale", same scan rule
		// as the existing highest-suffix allocator, not a gap-filling one).
		if q.ID == "r-001" || q.ID == "r-002" || q.ID == "r-003" {
			t.Errorf("newly allocated ID must not reuse an existing or gap ID, got %q", q.ID)
		}
	}

	all, _ := e.SnapshotRafaleReservoir()
	if len(all) != 5 {
		t.Fatalf("expected 5 total questions in reservoir (2 existing + 3 new), got %d", len(all))
	}
}

func TestAppendRafaleQuestions_PureAdd_NeverModifiesOrDeletesExisting(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Untouched", Answer: "UA", Category: CategoryHistory, Difficulty: 2},
	})

	if _, err := e.AppendRafaleQuestions([]RafaleQuestion{
		{Question: "New", Answer: "NA", Category: CategoryScience, Difficulty: 1},
	}); err != nil {
		t.Fatalf("AppendRafaleQuestions failed: %v", err)
	}

	existing, ok := e.GetRafaleQuestion("r-001")
	if !ok {
		t.Fatal("r-001 must still exist after AppendRafaleQuestions")
	}
	if existing.Question != "Untouched" || existing.Answer != "UA" || existing.Category != CategoryHistory || existing.Difficulty != 2 {
		t.Errorf("existing question r-001 was mutated by AppendRafaleQuestions: %+v", existing)
	}

	all, _ := e.SnapshotRafaleReservoir()
	if len(all) != 2 {
		t.Fatalf("expected 2 total questions (1 existing + 1 appended), got %d", len(all))
	}
}

func TestAppendRafaleQuestions_InputIDIsAlwaysIgnored(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Existing", Answer: "EA", Category: CategoryHistory, Difficulty: 1},
	})

	// Caller-supplied ID deliberately collides with an existing one — the
	// contract mandates it is ignored (server always allocates), so this must
	// NOT overwrite r-001.
	stored, err := e.AppendRafaleQuestions([]RafaleQuestion{
		{ID: "r-001", Question: "Attempted overwrite", Answer: "Hacked", Category: CategoryScience, Difficulty: 3},
	})
	if err != nil {
		t.Fatalf("AppendRafaleQuestions failed: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored question, got %d", len(stored))
	}
	if stored[0].ID == "r-001" {
		t.Fatalf("a caller-supplied ID that collides with an existing entry must be ignored and reallocated, got ID=%q reused", stored[0].ID)
	}

	original, ok := e.GetRafaleQuestion("r-001")
	if !ok || original.Question != "Existing" {
		t.Fatalf("r-001 must be untouched, got %+v (ok=%v)", original, ok)
	}

	all, _ := e.SnapshotRafaleReservoir()
	if len(all) != 2 {
		t.Fatalf("expected 2 total questions (original r-001 + reallocated new entry), got %d", len(all))
	}
}

func TestAppendRafaleQuestions_EmptyBatch_NoOp(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Existing", Answer: "EA", Category: CategoryHistory, Difficulty: 1},
	})

	stored, err := e.AppendRafaleQuestions(nil)
	if err != nil {
		t.Fatalf("AppendRafaleQuestions(nil) should not error, got: %v", err)
	}
	if len(stored) != 0 {
		t.Errorf("expected 0 stored questions for an empty batch, got %d", len(stored))
	}
	all, _ := e.SnapshotRafaleReservoir()
	if len(all) != 1 {
		t.Errorf("an empty batch must not alter the reservoir, got %d entries", len(all))
	}
}

// TestAppendRafaleQuestions_ConcurrentReader_NeverObservesPartialWrite is the
// directly-observable counterpart of contract §8's "une seule sauvegarde,
// hors du verrou" rule: a second Engine loading the SAME on-disk file
// concurrently with a batch append must only ever see the count from BEFORE
// the whole batch or the count from AFTER it — never anything in between.
// Counting SaveRafale() invocations from outside the package isn't possible
// (unexported, no test hook) — this test instead proves the property that
// actually matters: no reader-visible partial state, which is exactly what a
// single atomic write (vs. N incremental UpsertRafaleQuestion-style saves)
// guarantees and N separate writes would not (each of the N writes would be
// its own brief window during which a concurrent Load could observe a
// count strictly between before/after).
func TestAppendRafaleQuestions_ConcurrentReader_NeverObservesPartialWrite(t *testing.T) {
	dir := t.TempDir()
	reservoirPath := filepath.Join(dir, "reservoir.json")

	writer := NewEngine()
	writer.SetRafalePath(reservoirPath)
	const preExisting = 3
	seed := make([]RafaleQuestion, 0, preExisting)
	for i := 0; i < preExisting; i++ {
		seed = append(seed, RafaleQuestion{Question: "Existing", Answer: "EA", Category: CategoryHistory, Difficulty: 1})
	}
	seedRafaleReservoir(t, writer, seed)
	if err := writer.SaveRafale(); err != nil {
		t.Fatalf("initial SaveRafale failed: %v", err)
	}

	const batchSize = 25
	batch := make([]RafaleQuestion, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		batch = append(batch, RafaleQuestion{Question: "New", Answer: "NA", Category: CategoryScience, Difficulty: 2})
	}

	var wg sync.WaitGroup
	stopReading := make(chan struct{})
	var badCounts []int
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopReading:
				return
			default:
			}
			reader := NewEngine()
			reader.SetRafalePath(reservoirPath)
			if err := reader.LoadRafale(); err != nil {
				continue // a rename in flight can transiently fail a naive read on some platforms; retry
			}
			all, _ := reader.SnapshotRafaleReservoir()
			n := len(all)
			if n != preExisting && n != preExisting+batchSize {
				mu.Lock()
				badCounts = append(badCounts, n)
				mu.Unlock()
			}
		}
	}()

	if _, err := writer.AppendRafaleQuestions(batch); err != nil {
		t.Fatalf("AppendRafaleQuestions failed: %v", err)
	}
	close(stopReading)
	wg.Wait()

	if len(badCounts) > 0 {
		t.Errorf("concurrent reader observed a partial write (counts other than %d or %d): %v — the batch was not written as a single atomic save", preExisting, preExisting+batchSize, badCounts)
	}
}

// TestAppendRafaleQuestions_ConcurrentWithUpsert_NoDeadlockNoDuplicateID
// exercises risk R5 of the plan directly: AppendRafaleQuestions and
// UpsertRafaleQuestion firing concurrently must never deadlock (the
// RWMutex is not reentrant, and SaveRafale() takes RLock — a bug inverting
// the Lock→insert→Unlock→Save order would deadlock exactly under this kind
// of contention) and must never allocate the same ID twice.
func TestAppendRafaleQuestions_ConcurrentWithUpsert_NoDeadlockNoDuplicateID(t *testing.T) {
	e := NewEngine()

	const upsertCount = 20
	const batchCount = 20

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < upsertCount; i++ {
			if _, err := e.UpsertRafaleQuestion(RafaleQuestion{
				Question: "Upserted", Answer: "UA", Category: CategoryHistory, Difficulty: 1,
			}); err != nil {
				t.Errorf("UpsertRafaleQuestion failed: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		batch := make([]RafaleQuestion, 0, batchCount)
		for i := 0; i < batchCount; i++ {
			batch = append(batch, RafaleQuestion{Question: "Appended", Answer: "AA", Category: CategoryScience, Difficulty: 2})
		}
		if _, err := e.AppendRafaleQuestions(batch); err != nil {
			t.Errorf("AppendRafaleQuestions failed: %v", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK — no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("AppendRafaleQuestions and UpsertRafaleQuestion running concurrently deadlocked (R5: RWMutex not reentrant, SaveRafale takes RLock)")
	}

	all, _ := e.SnapshotRafaleReservoir()
	if len(all) != upsertCount+batchCount {
		t.Fatalf("expected %d total questions (%d upserted + %d appended), got %d", upsertCount+batchCount, upsertCount, batchCount, len(all))
	}
	seen := make(map[string]bool, len(all))
	for _, q := range all {
		if seen[q.ID] {
			t.Errorf("duplicate ID allocated across concurrent Upsert/Append: %q", q.ID)
		}
		seen[q.ID] = true
	}
}

// TestAppendRafaleQuestions_JSONRoundTrip verifies the on-disk shape after an
// append is identical to what UpsertRafaleQuestion would produce — a
// generated question must be indiscernible from a hand-entered one (contract
// §9 "Migration et compatibilité").
func TestAppendRafaleQuestions_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	reservoirPath := filepath.Join(dir, "reservoir.json")

	e := NewEngine()
	e.SetRafalePath(reservoirPath)
	if _, err := e.AppendRafaleQuestions([]RafaleQuestion{
		{Question: "Généré ?", Answer: "Oui", Category: CategoryHistory, Difficulty: 2},
	}); err != nil {
		t.Fatalf("AppendRafaleQuestions failed: %v", err)
	}

	raw, err := os.ReadFile(reservoirPath)
	if err != nil {
		t.Fatalf("failed to read reservoir.json: %v", err)
	}
	var envelope struct {
		Questions []RafaleQuestion `json:"QUESTIONS"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("reservoir.json must unmarshal into {\"QUESTIONS\": [...]}: %v (raw: %s)", err, raw)
	}
	if len(envelope.Questions) != 1 {
		t.Fatalf("expected 1 persisted question, got %d", len(envelope.Questions))
	}
	q := envelope.Questions[0]
	if q.ID == "" || q.Question != "Généré ?" || q.Answer != "Oui" || q.Category != CategoryHistory || q.Difficulty != 2 {
		t.Errorf("persisted shape diverges from RafaleQuestion, got %+v", q)
	}
}
