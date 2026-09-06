package server

import (
	"buzzcontrol/internal/protocol"
	"testing"
)

// Test for #214 — entracte programmée (milestone v9.0.0, Batch 3): the exit
// gesture must be in the entracte allow-list, or a programmed pause would
// have no way out (risk flagged explicitly in dev-backend's own handoff and
// in docs/mockups/entracte-programme-214.html §03 "Le garde-fou à ne pas
// oublier" — "c'est le SEUL point où les deux conceptions [manuelle et
// programmée] entrent réellement en collision").
//
// Maquette §03 "Sortie" : "Même geste que pour l'entracte manuel" — the exit
// gesture for a programmed pause is ENTRACTE_SET, identical to the manual
// button's own exit, NOT a new protocol action. This test asserts against
// entracte_allowlist_test.go's own exhaustive expectation map
// (entracteExpectedAllowed) rather than re-declaring a parallel one — a
// second, independent map would drift the day either one is edited without
// the other, exactly the kind of duplicated-guard mistake #107's own bug
// pattern (rafale.md §7.1 commentary) warns against for THIS package.
func TestEntracteProgrammed_ExitGestureNeverADeadEnd(t *testing.T) {
	if !entracteExpectedAllowed[protocol.ActionEntracteSet] {
		t.Fatal("ActionEntracteSet is not allowed during an active entracte — a programmed pause (same exit gesture as the manual one, maquette §03) would have NO way out")
	}
	if !IsActionAllowedDuringEntracte(protocol.ActionEntracteSet) {
		t.Fatal("IsActionAllowedDuringEntracte(ActionEntracteSet) = false — the runtime gate disagrees with the expectation map (entracte_allowlist_test.go already sweeps this exhaustively; this test exists to name the #214-specific consequence explicitly, not to duplicate that sweep)")
	}
}
