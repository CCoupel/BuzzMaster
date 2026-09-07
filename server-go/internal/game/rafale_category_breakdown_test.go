package game

// Tests for Lot A+1 (retour QUALIF v9.0.0.4, réouverture #216) —
// répartition des points RAFALE par catégorie, plan
// _work/reports/plan-v900-correctifs-qualif-20260906-104500.md §2.
//
// RafaleCategoryBreakdown/largestRemainderSplit (rafale_category_breakdown.go)
// are pure functions — no engine needed, exercised directly.

import (
	"testing"
)

func rafaleRoundQuestion(categories []string) *Question {
	return &Question{
		ID: "rq", Type: QuestionTypeRafale,
		TypedContent: TypedContent{RafaleCategories: categories, RafaleDifficulties: []int{1}},
	}
}

func sumMap(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// ---------------------------------------------------------------------------
// Non-divisible cases — la somme des parts doit être EXACTEMENT égale aux
// points attribués, méthode du plus fort reste.
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_SumAlwaysExactlyEqualsPoints(t *testing.T) {
	tests := []struct {
		name    string
		points  int
		correct map[string]int
	}{
		{"7 points sur 3 catégories (2,1,1)", 7, map[string]int{"A": 2, "B": 1, "C": 1}},
		{"10 points sur 3 catégories (5,3,2)", 10, map[string]int{"A": 5, "B": 3, "C": 2}},
		{"1 point sur 4 catégories (1,1,1,1)", 1, map[string]int{"A": 1, "B": 1, "C": 1, "D": 1}},
		{"3 points sur 3 catégories à parts égales (1,1,1)", 3, map[string]int{"A": 1, "B": 1, "C": 1}},
		{"grand nombre de points, poids très inégaux", 97, map[string]int{"A": 1, "B": 1, "C": 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cats := make([]string, 0, len(tt.correct))
			for c := range tt.correct {
				cats = append(cats, c)
			}
			q := rafaleRoundQuestion(cats)
			got := RafaleCategoryBreakdown(q, tt.points, tt.correct)
			if sum := sumMap(got); sum != tt.points {
				t.Errorf("sum(breakdown) = %d, want exactly %d (breakdown=%v) — jamais une unité perdue ni créée", sum, tt.points, got)
			}
			for cat, v := range got {
				if v <= 0 {
					t.Errorf("breakdown[%q] = %d, want > 0 — une catégorie sans part ne doit pas apparaître (jamais un 0 explicite)", cat, v)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Répartition proportionnelle aux bonnes réponses par catégorie.
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_ProportionalToCorrectAnswersPerCategory(t *testing.T) {
	q := rafaleRoundQuestion([]string{"A", "B"})
	// 3x plus de bonnes réponses en A qu'en B -> A doit recevoir ~3x plus.
	got := RafaleCategoryBreakdown(q, 12, map[string]int{"A": 3, "B": 1})
	if got["A"] != 9 || got["B"] != 3 {
		t.Errorf("breakdown = %v, want A=9 B=3 (12 points, poids 3:1, division exacte)", got)
	}
}

// ---------------------------------------------------------------------------
// Repli à parts égales quand aucune bonne réponse n'est enregistrée
// (attribution manuelle par l'admin).
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_FallsBackToEqualSharesWhenNoCorrectAnswersRecorded(t *testing.T) {
	q := rafaleRoundQuestion([]string{"A", "B", "C"})
	got := RafaleCategoryBreakdown(q, 10, map[string]int{}) // aucune bonne réponse tracée
	if sum := sumMap(got); sum != 10 {
		t.Fatalf("sum(breakdown) = %d, want 10", sum)
	}
	if len(got) != 3 {
		t.Fatalf("breakdown = %v, want une entrée pour chacune des 3 catégories effectives (repli à parts égales, jamais de catégorie vide)", got)
	}
	for _, cat := range []string{"A", "B", "C"} {
		if got[cat] == 0 {
			t.Errorf("breakdown[%q] = 0, want une part non nulle (repli à parts égales)", cat)
		}
	}
}

func TestRafaleCategoryBreakdown_FallbackNilCounters_SameAsEmptyMap(t *testing.T) {
	q := rafaleRoundQuestion([]string{"A", "B"})
	got := RafaleCategoryBreakdown(q, 5, nil) // teamCategoryCounters nil-safe, contrat de la doc
	if sum := sumMap(got); sum != 5 {
		t.Errorf("sum(breakdown) = %d, want 5 (nil counters -> repli à parts égales)", sum)
	}
}

// ---------------------------------------------------------------------------
// Une seule catégorie tirée -> une seule entrée, résultat identique à
// l'actuel (non-régression).
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_SingleCategory_OneEntryEqualToPoints(t *testing.T) {
	q := rafaleRoundQuestion([]string{"HISTORY"})
	got := RafaleCategoryBreakdown(q, 15, map[string]int{"HISTORY": 4})
	if len(got) != 1 || got["HISTORY"] != 15 {
		t.Errorf("breakdown = %v, want exactement {HISTORY: 15}", got)
	}
}

// ---------------------------------------------------------------------------
// Non-régression : types/valeurs hors périmètre -> nil (GameEvent sans
// CategoryBreakdown, comportement actuel inchangé, omitempty).
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_NonRafaleQuestion_ReturnsNil(t *testing.T) {
	q := &Question{ID: "q", Type: QuestionTypeSpeedy, Category: CategoryHistory}
	if got := RafaleCategoryBreakdown(q, 10, map[string]int{"HISTORY": 1}); got != nil {
		t.Errorf("breakdown = %v, want nil pour un type non-RAFALE (aucune régression sur les autres types)", got)
	}
}

func TestRafaleCategoryBreakdown_NilQuestion_ReturnsNil(t *testing.T) {
	if got := RafaleCategoryBreakdown(nil, 10, map[string]int{"HISTORY": 1}); got != nil {
		t.Errorf("breakdown = %v, want nil pour une question nil", got)
	}
}

func TestRafaleCategoryBreakdown_ZeroOrNegativePoints_ReturnsNil(t *testing.T) {
	q := rafaleRoundQuestion([]string{"HISTORY"})
	if got := RafaleCategoryBreakdown(q, 0, map[string]int{"HISTORY": 1}); got != nil {
		t.Errorf("breakdown(points=0) = %v, want nil", got)
	}
	if got := RafaleCategoryBreakdown(q, -5, map[string]int{"HISTORY": 1}); got != nil {
		t.Errorf("breakdown(points=-5) = %v, want nil", got)
	}
}

func TestRafaleCategoryBreakdown_NoConfiguredCategory_ReturnsNil(t *testing.T) {
	q := rafaleRoundQuestion(nil) // round mal configuré, défensif (contrat §7.5 empêche normalement ce cas)
	if got := RafaleCategoryBreakdown(q, 10, map[string]int{}); got != nil {
		t.Errorf("breakdown = %v, want nil quand aucune catégorie effective n'existe", got)
	}
}

// ---------------------------------------------------------------------------
// Retro-compat : une question RAFALE mono (CATEGORY générique, pas
// RAFALE_CATEGORIES) doit fonctionner via EffectiveRafaleCategories.
// ---------------------------------------------------------------------------

func TestRafaleCategoryBreakdown_MonoLegacyQuestion_FallsBackViaEffectiveCategories(t *testing.T) {
	q := &Question{ID: "rq-mono", Type: QuestionTypeRafale, Category: CategoryHistory, TypedContent: TypedContent{RafaleDifficulty: 1}}
	got := RafaleCategoryBreakdown(q, 8, map[string]int{}) // aucune bonne réponse tracée -> repli
	if len(got) != 1 || got["HISTORY"] != 8 {
		t.Errorf("breakdown = %v, want {HISTORY: 8} via la rétro-compatibilité mono (EffectiveRafaleCategories)", got)
	}
}
