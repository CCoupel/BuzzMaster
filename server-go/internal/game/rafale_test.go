package game

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : mode RAFALE — réservoir, flag « déjà utilisée », pioche, comptage
// de pool (milestone v8.0.0 #16, issue #197, contrat contracts/rafale.md
// §2.4/§3/§7). Périmètre Batch 1 (test-writer, en parallèle de dev-backend
// sur la même branche) : pioche (§7), persistance du flag (§3.2), comptage
// de pool + estimation du besoin (§7.2). Le CRUD HTTP (§9) et le moteur de
// manche complet (#107, sous-phases/tickers) sont hors périmètre de ce
// fichier — voir internal/server (endpoints) et le futur
// cmd/server/rafale_107_test.go (Batch 2/3).
//
// Écrit à partir du contrat (contract-first) — internal/game/rafale_store.go
// et la partie moteur d'internal/game/engine.go (DrawRafaleQuestion,
// CountRafalePool, reset InitGame) sont la référence de nommage : ce fichier
// vérifie leur comportement RÉEL au moment de l'écriture (Batch 1 dev-backend
// a livré ce module en parallèle), pas une spéculation sur une future API.
// ---------------------------------------------------------------------------

// seedRafaleReservoir upserts each question into e's reservoir via the public
// UpsertRafaleQuestion API — no reservoir path is set, so this never touches
// disk (SaveRafale no-ops when rafalePath == ""), matching e.g.
// setBumpersNoAutoSave's intent elsewhere in this package: give tests full
// control over fixture state without background I/O side effects.
func seedRafaleReservoir(t *testing.T, e *Engine, questions []RafaleQuestion) {
	t.Helper()
	for _, q := range questions {
		if _, err := e.UpsertRafaleQuestion(q); err != nil {
			t.Fatalf("seedRafaleReservoir: UpsertRafaleQuestion(%q) failed: %v", q.ID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Pioche (contrat §7) : pool = catégories ∩ difficulté ∩ non utilisée,
// tirage aléatoire uniforme, marquage immédiat.
// ---------------------------------------------------------------------------

func TestDrawRafaleQuestion_RespectsFilterIntersection(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: CategoryHistory, Difficulty: 2}, // wrong difficulty
		{ID: "r-3", Question: "Q3", Answer: "A3", Category: CategoryScience, Difficulty: 1}, // wrong category
		{ID: "r-4", Question: "Q4", Answer: "A4", Category: CategoryHistory, Difficulty: 1}, // matches
	})

	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		q, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
		if err != nil {
			// Pool of 2 (r-1, r-4) exhausts after 2 draws — expected once seen.
			if !errors.Is(err, ErrRafalePoolEmpty) {
				t.Fatalf("draw %d: unexpected error: %v", i, err)
			}
			break
		}
		if q.Category != CategoryHistory || q.Difficulty != 1 {
			t.Fatalf("draw %d: got %+v, want CATEGORY=HISTORY DIFFICULTY=1 only (contract §7 intersection)", i, q)
		}
		seen[q.ID] = true
	}

	if len(seen) != 2 || !seen["r-1"] || !seen["r-4"] {
		t.Errorf("expected exactly {r-1, r-4} to be drawable (HISTORY ∩ 1), got %v", seen)
	}
	if seen["r-2"] || seen["r-3"] {
		t.Errorf("r-2 (wrong difficulty) and r-3 (wrong category) must never be drawn, got %v", seen)
	}
}

func TestDrawRafaleQuestion_MultiCategory_IsOrNotAnd(t *testing.T) {
	// §7: "q.CATEGORY ∈ RAFALE_CATEGORIES" — a multi-category filter is an
	// OR across categories (any selected category matches), not an AND.
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 2},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: CategoryScience, Difficulty: 2},
		{ID: "r-3", Question: "Q3", Answer: "A3", Category: CategorySports, Difficulty: 2}, // not selected
	})

	drawnIDs := map[string]bool{}
	for i := 0; i < 10; i++ {
		q, err := e.DrawRafaleQuestion([]string{string(CategoryHistory), string(CategoryScience)}, 2)
		if err != nil {
			if errors.Is(err, ErrRafalePoolEmpty) {
				break
			}
			t.Fatalf("draw %d failed: %v", i, err)
		}
		drawnIDs[q.ID] = true
	}
	if len(drawnIDs) != 2 || !drawnIDs["r-1"] || !drawnIDs["r-2"] {
		t.Errorf("expected {r-1, r-2} drawable under a multi-category (OR) filter, got %v", drawnIDs)
	}
	if drawnIDs["r-3"] {
		t.Error("r-3 (SPORTS, not in the selected categories) must never be drawn")
	}
}

func TestDrawRafaleQuestion_MarksUsedImmediately(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	availableBefore, usedBefore, _ := e.CountRafalePool([]string{string(CategoryHistory)}, 1)
	if availableBefore != 1 || usedBefore != 0 {
		t.Fatalf("pre-condition: available=%d used=%d, want 1/0", availableBefore, usedBefore)
	}

	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1); err != nil {
		t.Fatalf("DrawRafaleQuestion failed: %v", err)
	}

	availableAfter, usedAfter, _ := e.CountRafalePool([]string{string(CategoryHistory)}, 1)
	if availableAfter != 0 || usedAfter != 1 {
		t.Errorf("after one draw: available=%d used=%d, want 0/1 (marking must be immediate, not deferred to save)", availableAfter, usedAfter)
	}
}

func TestDrawRafaleQuestion_EmptyPool_ReturnsErrRafalePoolEmpty(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	// Draw the only match, exhausting the pool.
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1); err != nil {
		t.Fatalf("first draw should succeed: %v", err)
	}

	q, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
	if !errors.Is(err, ErrRafalePoolEmpty) {
		t.Errorf("expected ErrRafalePoolEmpty on an exhausted pool, got q=%v err=%v", q, err)
	}
}

func TestDrawRafaleQuestion_EmptyReservoir_ReturnsErrRafalePoolEmpty(t *testing.T) {
	e := NewEngine()
	q, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
	if !errors.Is(err, ErrRafalePoolEmpty) {
		t.Errorf("drawing from an empty reservoir must return ErrRafalePoolEmpty, got q=%v err=%v", q, err)
	}
}

func TestDrawRafaleQuestion_EmptyCategoriesFilter_NeverMatches(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})
	// §3.3: RAFALE_CATEGORIES is "multi-sélection, ≥1" — an empty filter is
	// an invalid/unconfigured manche, and must draw nothing rather than
	// silently matching everything.
	_, err := e.DrawRafaleQuestion([]string{}, 1)
	if !errors.Is(err, ErrRafalePoolEmpty) {
		t.Errorf("an empty categories filter must match nothing, got err=%v", err)
	}
}

// ---------------------------------------------------------------------------
// Persistance du flag « déjà utilisée » (contrat §3.2) : aller-retour
// disque, survie à un redémarrage, reset au NEW_GAME.
// ---------------------------------------------------------------------------

func TestRafaleUsedFlag_SaveRoundTrip_OnDiskShape(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	dir := t.TempDir()
	usedPath := filepath.Join(dir, "rafale_used.json")
	e.SetRafaleUsedPath(usedPath)

	drawn, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
	if err != nil {
		t.Fatalf("DrawRafaleQuestion failed: %v", err)
	}

	// DrawRafaleQuestion fires SaveRafaleUsed asynchronously (safeGo) — call
	// it again here directly (idempotent, same in-memory map) so the
	// assertion below is deterministic instead of racing the background
	// goroutine, mirroring this package's SaveBumpers test discipline.
	if err := e.SaveRafaleUsed(); err != nil {
		t.Fatalf("SaveRafaleUsed failed: %v", err)
	}

	data, err := os.ReadFile(usedPath)
	if err != nil {
		t.Fatalf("rafale_used.json should exist and be readable: %v", err)
	}

	// Contract §3.2 fixes the exact on-disk shape: {"USED": {"<id>": true}}.
	var envelope struct {
		Used map[string]bool `json:"USED"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("rafale_used.json must be valid JSON matching {\"USED\": {...}}: %v (raw: %s)", err, data)
	}
	if !envelope.Used[drawn.ID] {
		t.Errorf("rafale_used.json USED map must contain %q=true, got %v", drawn.ID, envelope.Used)
	}
}

func TestRafaleUsedFlag_SurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	reservoirPath := filepath.Join(dir, "reservoir.json")
	usedPath := filepath.Join(dir, "rafale_used.json")

	// --- "Before restart" engine ---
	e1 := NewEngine()
	e1.SetRafalePath(reservoirPath)
	seedRafaleReservoir(t, e1, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 2},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: CategoryHistory, Difficulty: 2},
	})
	if err := e1.SaveRafale(); err != nil {
		t.Fatalf("SaveRafale failed: %v", err)
	}
	e1.SetRafaleUsedPath(usedPath)

	drawn, err := e1.DrawRafaleQuestion([]string{string(CategoryHistory)}, 2)
	if err != nil {
		t.Fatalf("DrawRafaleQuestion failed: %v", err)
	}
	if err := e1.SaveRafaleUsed(); err != nil { // deterministic flush, see comment above
		t.Fatalf("SaveRafaleUsed failed: %v", err)
	}

	// --- Simulated restart: a brand new Engine loading the same files ---
	e2 := NewEngine()
	e2.SetRafalePath(reservoirPath)
	if err := e2.LoadRafale(); err != nil {
		t.Fatalf("LoadRafale failed: %v", err)
	}
	e2.SetRafaleUsedPath(usedPath)
	if err := e2.LoadRafaleUsed(); err != nil {
		t.Fatalf("LoadRafaleUsed failed: %v", err)
	}

	available, used, total := e2.CountRafalePool([]string{string(CategoryHistory)}, 2)
	if total != 2 {
		t.Fatalf("restart must see the full reservoir, total=%d want 2", total)
	}
	if used != 1 || available != 1 {
		t.Errorf("restart must remember the pre-restart draw: available=%d used=%d, want 1/1", available, used)
	}

	// The only remaining draw must be the OTHER question — never the one
	// drawn (and persisted) before the restart. This is the exact
	// regression contract §7.1 forbids ("jamais de reproposition silencieuse
	// d'une question déjà vue").
	second, err := e2.DrawRafaleQuestion([]string{string(CategoryHistory)}, 2)
	if err != nil {
		t.Fatalf("second draw should still succeed (one question left): %v", err)
	}
	if second.ID == drawn.ID {
		t.Fatalf("restart re-proposed the already-used question %q — the used flag did not survive", drawn.ID)
	}

	if _, err := e2.DrawRafaleQuestion([]string{string(CategoryHistory)}, 2); !errors.Is(err, ErrRafalePoolEmpty) {
		t.Errorf("pool should now be exhausted post-restart, got err=%v", err)
	}
}

func TestInitGame_ResetsRafaleUsedFlag_ButNotTheReservoir(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	drawn, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}

	// Pre-condition: pool now empty.
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1); !errors.Is(err, ErrRafalePoolEmpty) {
		t.Fatalf("pre-condition failed: pool should be empty before InitGame, err=%v", err)
	}

	e.InitGame()

	// Reservoir itself is untouched (contract §3.2: "La réinitialisation du
	// flag ... Réservoir ... contient ce que l'admin a édité — jamais
	// réinitialisé par NEW_GAME").
	available, used, total := e.CountRafalePool([]string{string(CategoryHistory)}, 1)
	if total != 1 {
		t.Errorf("InitGame must not touch the reservoir itself, total=%d want 1", total)
	}
	if used != 0 || available != 1 {
		t.Errorf("InitGame must reset the used flag: available=%d used=%d, want 1/0", available, used)
	}

	again, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1)
	if err != nil {
		t.Fatalf("question should be drawable again after NEW_GAME reset: %v", err)
	}
	if again.ID != drawn.ID {
		t.Errorf("only one question exists in the reservoir — expected %q again, got %q", drawn.ID, again.ID)
	}
}

// ---------------------------------------------------------------------------
// Comptage de pool (contrat §7.2, GET /api/rafale/pool) et formule
// d'estimation du besoin — les 3 états d'alerte avant démarrage.
// ---------------------------------------------------------------------------

func TestCountRafalePool_Filtering(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-3", Question: "Q3", Answer: "A3", Category: CategoryHistory, Difficulty: 2},
		{ID: "r-4", Question: "Q4", Answer: "A4", Category: CategoryScience, Difficulty: 1},
	})
	// Mark r-1 used.
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, 1); err != nil {
		t.Fatalf("setup draw failed: %v", err)
	}

	tests := []struct {
		name          string
		categories    []string
		difficulty    int
		wantAvailable int
		wantUsed      int
		wantTotal     int
	}{
		{"HISTORY/1: one used, one available", []string{string(CategoryHistory)}, 1, 1, 1, 2},
		{"HISTORY/2: single, untouched", []string{string(CategoryHistory)}, 2, 1, 0, 1},
		{"SCIENCE/1: single, untouched", []string{string(CategoryScience)}, 1, 1, 0, 1},
		{"HISTORY/3: no such difficulty", []string{string(CategoryHistory)}, 3, 0, 0, 0},
		{"SPORTS/1: unknown category in reservoir", []string{string(CategorySports)}, 1, 0, 0, 0},
		{"HISTORY+SCIENCE/1: OR across categories", []string{string(CategoryHistory), string(CategoryScience)}, 1, 2, 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available, used, total := e.CountRafalePool(tt.categories, tt.difficulty)
			if available != tt.wantAvailable || used != tt.wantUsed || total != tt.wantTotal {
				t.Errorf("CountRafalePool(%v, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.categories, tt.difficulty, available, used, total,
					tt.wantAvailable, tt.wantUsed, tt.wantTotal)
			}
		})
	}
}

func TestCountRafalePool_EmptyReservoir_AllZero(t *testing.T) {
	e := NewEngine()
	available, used, total := e.CountRafalePool([]string{string(CategoryHistory)}, 1)
	if available != 0 || used != 0 || total != 0 {
		t.Errorf("empty reservoir must count (0,0,0), got (%d,%d,%d)", available, used, total)
	}
}

// rafalePoolAlertLevel mirrors contract §7.2's three-state pre-round alert,
// purely as a spec-level formula check: besoin_estimé = ceil(TIME /
// RAFALE_QUESTION_TIME); disponible==0 → BLOCKING (démarrage refusé),
// disponible<besoin → WARNING (démarrage autorisé, risque de fin
// anticipée), disponible≥besoin → OK. This is NOT a production function —
// it documents the classification this test exercises against
// CountRafalePool's real numbers, so a future change to the formula's
// boundary conditions (e.g. off-by-one on the ceiling) is caught here even
// before the HTTP/GamePage layer computes it (#26, Batch 2).
func rafalePoolAlertLevel(available, roundTimeSeconds, questionTimeSeconds int) string {
	if questionTimeSeconds <= 0 {
		questionTimeSeconds = 1
	}
	neededEstimate := (roundTimeSeconds + questionTimeSeconds - 1) / questionTimeSeconds // ceil
	switch {
	case available == 0:
		return "BLOCKING"
	case available < neededEstimate:
		return "WARNING"
	default:
		return "OK"
	}
}

func TestRafalePoolEstimate_ThreeAlertStates(t *testing.T) {
	// Default manche: TIME=120s, RAFALE_QUESTION_TIME=3s → besoin_estimé = 40.
	const roundTime = 120
	const questionTime = 3
	const neededEstimate = 40 // sanity check for the table below

	if got := rafalePoolAlertLevel(neededEstimate, roundTime, questionTime); got != "OK" {
		t.Fatalf("sanity: available==neededEstimate must be OK (boundary is inclusive), got %s", got)
	}

	tests := []struct {
		name      string
		available int
		want      string
	}{
		{"0 disponible : bloquant, quel que soit le besoin", 0, "BLOCKING"},
		{"moins que le besoin estimé : avertissement", 10, "WARNING"},
		{"juste en dessous du seuil : avertissement (39 < 40)", neededEstimate - 1, "WARNING"},
		{"exactement le besoin estimé : neutre (limite incluse)", neededEstimate, "OK"},
		{"largement au-dessus : neutre", 100, "OK"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			questions := make([]RafaleQuestion, tt.available)
			for i := range questions {
				questions[i] = RafaleQuestion{
					ID: "r-" + string(rune('a'+i)), Question: "Q", Answer: "A",
					Category: CategoryHistory, Difficulty: 1,
				}
			}
			seedRafaleReservoir(t, e, questions)

			available, _, _ := e.CountRafalePool([]string{string(CategoryHistory)}, 1)
			if available != tt.available {
				t.Fatalf("setup: CountRafalePool available=%d, want %d", available, tt.available)
			}
			if got := rafalePoolAlertLevel(available, roundTime, questionTime); got != tt.want {
				t.Errorf("rafalePoolAlertLevel(%d, %d, %d) = %s, want %s", available, roundTime, questionTime, got, tt.want)
			}
		})
	}
}

func TestRafalePoolEstimate_CeilingRoundsUp(t *testing.T) {
	// 121s / 3s = 40.33 → the estimate must round UP to 41 (contract §7.2:
	// "estimation majorante ... à présenter comme un plancher de sécurité").
	// A pool of exactly 40 must therefore be a WARNING, not OK, once
	// rounding is applied correctly.
	if got := rafalePoolAlertLevel(40, 121, 3); got != "WARNING" {
		t.Errorf("121s/3s must ceil to 41 (not floor to 40): available=40 should be WARNING, got %s", got)
	}
	if got := rafalePoolAlertLevel(41, 121, 3); got != "OK" {
		t.Errorf("available=41 should meet the ceil(121/3)=41 estimate and be OK, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// CRUD réservoir — couverture légère (le gros du contrat §9 est testé côté
// HTTP handlers, hors périmètre de ce fichier) : uniquement les invariants
// portés par l'Engine lui-même (assignation d'ID, erreur 404-mappable).
// ---------------------------------------------------------------------------

func TestUpsertRafaleQuestion_CreateAssignsSequentialID(t *testing.T) {
	e := NewEngine()
	q1, err := e.UpsertRafaleQuestion(RafaleQuestion{Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1})
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if q1.ID == "" {
		t.Fatal("create (no ID in input) must assign one")
	}

	q2, err := e.UpsertRafaleQuestion(RafaleQuestion{Question: "Q2", Answer: "A2", Category: CategoryHistory, Difficulty: 1})
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if q2.ID == "" || q2.ID == q1.ID {
		t.Errorf("second create must get a distinct, non-empty ID: q1=%q q2=%q", q1.ID, q2.ID)
	}

	got, ok := e.GetRafaleQuestion(q1.ID)
	if !ok || got.Question != "Q1" {
		t.Errorf("GetRafaleQuestion(%q) = %+v, %v — expected the stored Q1", q1.ID, got, ok)
	}
}

func TestDeleteRafaleQuestion_UnknownID_ReturnsErrRafaleQuestionNotFound(t *testing.T) {
	e := NewEngine()
	err := e.DeleteRafaleQuestion("does-not-exist")
	if !errors.Is(err, ErrRafaleQuestionNotFound) {
		t.Errorf("deleting an unknown ID must return ErrRafaleQuestionNotFound (contract §9, 404), got %v", err)
	}
}
