package game

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : ReconnectOrCreateVirtualPlayer — fix R1 (backend-ID matrix)
//
// Contexte : _work/reports/planner-20260725-143029-r1-fix.md §3.3. Répond au
// blocage code-review CRITIQUE sur l'ancienne version de
// ReconnectOrCreateVirtualPlayer, qui matchait/consolidait par NOM SEUL et
// pouvait donc supprimer silencieusement un bumper homonyme distinct
// (perte de données irréversible). Le fix retient l'ID backend (déjà généré
// et renvoyé au client dans PLAYER_CONNECTED.ID — la clé de la map Bumpers)
// comme SEULE source d'identité pour la reconnexion ; le nom ne sert plus
// qu'au test d'unicité lors d'un nouvel enrôlement.
//
// Matrice testée (5 cas, §3.3) :
//   1. ID fourni et trouvé (IsVirtual)      -> reconnexion, jamais de delete/merge
//   2. ID fourni mais introuvable            -> traité comme sans ID (cas 4/5)
//   3. Pas d'ID, nom déjà pris, détenteur CONNECTÉ    -> REJET NAME_TAKEN
//   3bis. Pas d'ID, nom déjà pris, détenteur DÉCONNECTÉ -> REJET NAME_TAKEN_OFFLINE
//         ([CHANGED] #122/B1 : même rejet strict qu'avant — aucune adoption
//         silencieuse — mais motif distinct, cf. reprise assistée #122)
//   4. Pas d'ID, nom libre                    -> nouvel enrôlement
//   5. ID fourni + introuvable + nom libre    -> nouvel enrôlement (= cas 4)
//
// Plus : le cas contradictoire (2 bumpers homonymes, données différentes)
// et la purge des VJoueurs sur InitGame/NEW_GAME (D-R1b, déplacée depuis
// StartEnrollment — voir le bloc de tests dédié plus bas).
//
// #122 (reprise assistée) ajoute Bumper.ReclaimRequested et le rattachement
// à usage unique après RELEASE_BUMPER_NAME — voir name_recovery_test.go.
// ---------------------------------------------------------------------------

// TestReconnectOrCreateVirtualPlayer_Case1_IDFound_Reconnects covers matrix
// case 1: a resolvable ID is unambiguous and always wins as a genuine
// reconnection — team/score preserved, Connected flips true, ConnState
// transitions via Reconnect (orange/red -> green).
func TestReconnectOrCreateVirtualPlayer_Case1_IDFound_Reconnects(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)

	id, _, err := e.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumperScore(id, 15)
	// Simulate a real disconnect (orange badge) before reconnecting.
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.TransitionConn(id, ConnEventDisconnect)

	// Even outside ENROLL phase, reconnection-by-ID must succeed (per plan:
	// "Autorisé hors phase ENROLL").
	e.SetPhase(PhaseStopped)

	gotID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer(id, "Alice")
	if err != nil {
		t.Fatalf("expected no error reconnecting by ID, got: %v", err)
	}
	if !reconnected {
		t.Errorf("expected reconnected=true for a resolved ID")
	}
	if gotID != id {
		t.Errorf("expected the same bumper ID to be returned, got %q want %q", gotID, id)
	}
	if bumper.Team != "TeamA" {
		t.Errorf("R1: team lost on ID reconnection, got %q", bumper.Team)
	}
	if bumper.Score != 15 {
		t.Errorf("R1: score lost on ID reconnection, got %d", bumper.Score)
	}
	if !bumper.Connected {
		t.Errorf("expected Connected=true after ID reconnection")
	}
	if bumper.ConnState != ConnStateGreen {
		t.Errorf("expected ConnState=green after ID reconnection, got %q", bumper.ConnState)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case1_NameRefreshed verifies that a
// changed display name is refreshed on ID-based reconnection (the ID stays
// authoritative for identity; NAME is just cosmetic).
func TestReconnectOrCreateVirtualPlayer_Case1_NameRefreshed(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)

	id, _, err := e.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer(id, "AliceNouveauPseudo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reconnected {
		t.Errorf("expected reconnected=true")
	}
	if bumper.Name != "AliceNouveauPseudo" {
		t.Errorf("expected name to be refreshed to 'AliceNouveauPseudo', got %q", bumper.Name)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case2_StaleID_FallsBackToNameFree covers
// matrix case 2: an ID that doesn't resolve (or resolves to a non-virtual /
// nonexistent bumper) is treated as if absent — the name-free path (case 4)
// still succeeds normally.
func TestReconnectOrCreateVirtualPlayer_Case2_StaleID_FallsBackToNameFree(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)

	newID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("does-not-exist", "Bob")
	if err != nil {
		t.Fatalf("expected no error, stale ID should fall back to enrollment: %v", err)
	}
	if reconnected {
		t.Errorf("expected reconnected=false (this is a NEW enrollment), got true")
	}
	if newID == "" || bumper == nil {
		t.Fatalf("expected a new bumper to be created")
	}
	if bumper.Name != "Bob" {
		t.Errorf("expected new bumper name 'Bob', got %q", bumper.Name)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case2_IDResolvesToPhysicalBuzzer_Ignored
// verifies that an ID which happens to collide with a non-virtual bumper
// (e.g. a stale localStorage value from a totally different context) is
// never hijacked — it must fall back exactly like an unresolvable ID.
func TestReconnectOrCreateVirtualPlayer_Case2_IDResolvesToPhysicalBuzzer_Ignored(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{"NAME": "PhysicalBuzzer"})

	newID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("AA:BB:CC:DD:EE:01", "Zoe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reconnected {
		t.Errorf("must never 'reconnect' onto a non-virtual bumper")
	}
	if newID == "AA:BB:CC:DD:EE:01" {
		t.Errorf("must not reuse the physical buzzer's ID for the new VJoueur")
	}
	if bumper.Name != "Zoe" {
		t.Errorf("expected a fresh 'Zoe' bumper, got %q", bumper.Name)
	}
	// The physical buzzer itself must be completely untouched.
	physical := e.GetBumper("AA:BB:CC:DD:EE:01")
	if physical.IsVirtual {
		t.Errorf("physical buzzer must not have been mutated into a virtual player")
	}
}

// TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_Rejected covers
// matrix case 3 (connected variant): no ID, name already used by a currently
// CONNECTED VJoueur -> REJECTED, no merge.
func TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_Rejected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	_, _, err := e.CreateVirtualPlayer("Emma") // Connected=true by construction
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")

	assertNameTakenRejection(t, bumper, reconnected, err)
}

// TestReconnectOrCreateVirtualPlayer_Case3bis_NameTakenOffline_Disconnected_Rejected
// covers matrix case 3 (disconnected variant) — the strict rule still holds
// (rejected, no silent adoption), but the REASON changed under #122 (B1):
// a name matching a DISCONNECTED VJoueur now returns NAME_TAKEN_OFFLINE
// instead of the plain NAME_TAKEN returned for a CONNECTED holder (see
// TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_Rejected,
// unchanged just above) — the two situations are opposite in practice (an
// homonym vs. very likely oneself having lost the stored ID) and #122 lets
// the client and admin tell them apart. Renamed and its expected reason
// updated accordingly; the legitimate owner still always has their ID and
// takes the case-1 path instead — no silent adoption is introduced here.
func TestReconnectOrCreateVirtualPlayer_Case3bis_NameTakenOffline_Disconnected_Rejected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")

	assertNameTakenOfflineRejection(t, bumper, reconnected, err)

	// The existing (disconnected) bumper must still exist and stay
	// disconnected — #122 (B2) additionally marks it ReclaimRequested, see
	// TestReconnectOrCreateVirtualPlayer_Case3_NameTakenOffline_SetsReclaimRequested
	// (name_recovery_test.go) for that specific side effect.
	untouched := e.GetBumper(id)
	if untouched == nil {
		t.Fatal("R1: existing disconnected bumper was deleted on a rejected name-only attempt")
	}
	if untouched.Connected {
		t.Errorf("R1: rejected attempt must not have flipped Connected on the existing bumper")
	}
}

// TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_CaseInsensitive verifies
// the uniqueness check is trimmed + case-insensitive (matches the plan's
// "nom normalisé"), so "emma " and "Emma" are the same identity for the
// NAME_TAKEN check.
func TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_CaseInsensitive(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	if _, _, err := e.CreateVirtualPlayer("Emma"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "  emma ")
	if err == nil {
		t.Fatal("expected NAME_TAKEN rejection for a case/whitespace variant of an existing name")
	}
	enrollErr, ok := err.(*EnrollmentError)
	if !ok || enrollErr.Reason != "NAME_TAKEN" {
		t.Errorf("expected EnrollmentError{Reason: NAME_TAKEN}, got %v", err)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case4_NameFree_NewEnrollment covers
// matrix case 4: no ID, free name -> new enrollment, ID returned.
func TestReconnectOrCreateVirtualPlayer_Case4_NameFree_NewEnrollment(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)

	id, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "David")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reconnected {
		t.Errorf("expected reconnected=false for a brand-new enrollment")
	}
	if id == "" || bumper == nil {
		t.Fatal("expected a new bumper with a non-empty ID")
	}
	if bumper.Name != "David" || !bumper.IsVirtual || !bumper.IsVPlayer {
		t.Errorf("unexpected new bumper shape: %+v", bumper)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case4_RespectsEnrollPhaseAndLimit checks
// that the new-enrollment path (cases 4/5) still enforces the existing
// ENROLL-phase and player-limit guards (createVirtualPlayerUnsafe).
func TestReconnectOrCreateVirtualPlayer_Case4_RespectsEnrollPhaseAndLimit(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped) // not ENROLL

	_, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Eve")
	if err == nil {
		t.Fatal("expected ENROLLMENT_CLOSED error outside the ENROLL phase")
	}
	enrollErr, ok := err.(*EnrollmentError)
	if !ok || enrollErr.Reason != "ENROLLMENT_CLOSED" {
		t.Errorf("expected EnrollmentError{Reason: ENROLLMENT_CLOSED}, got %v", err)
	}
}

// TestReconnectOrCreateVirtualPlayer_Case5_StaleIDAndNameFree_NewEnrollment
// covers matrix case 5: ID given + not found + name free -> identical to
// case 4 (new enrollment), just reachable via a slightly different input.
func TestReconnectOrCreateVirtualPlayer_Case5_StaleIDAndNameFree_NewEnrollment(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)

	id, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("vjoueur_stale_id_xyz", "Frank")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reconnected {
		t.Errorf("expected reconnected=false")
	}
	if id == "vjoueur_stale_id_xyz" {
		t.Errorf("must not adopt the stale ID as the new bumper's key")
	}
	if bumper.Name != "Frank" {
		t.Errorf("expected name 'Frank', got %q", bumper.Name)
	}
}

// TestReconnectOrCreateVirtualPlayer_ContradictoryHomonyms_NeverDeleted is the
// "code-reviewer scenario", scoped to a single ongoing game (no cross-session
// legacy bumpers are possible — a game always starts with an empty bumper
// list, per product clarification): two DISTINCT players in the SAME game
// happen to pick the same first name "Emma" — one already enrolled, assigned
// to a team and disconnected mid-game (real team/score data), the other a
// newcomer trying to join under that name without an ID. A no-ID
// PLAYER_CONNECT attempt under that name must reject outright — never pick
// one bumper as "more legitimate" and delete or overwrite the other (that
// destructive tie-break is exactly what this fix removes).
func TestReconnectOrCreateVirtualPlayer_ContradictoryHomonyms_NeverDeleted(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	e.SetPhase(PhaseEnroll)

	e.SetBumpers(map[string]*Bumper{
		// Already enrolled this game, assigned, has real points, currently disconnected.
		"vjoueur_emma_first": {Name: "Emma", IsVirtual: true, IsVPlayer: true, Team: "TeamA", Score: 40, Connected: false, ConnState: ConnStateOrange},
		// A second, distinct player also named Emma, connected and playing on another team.
		"vjoueur_emma_second": {Name: "Emma", IsVirtual: true, IsVPlayer: true, Team: "TeamB", Score: 5, Connected: true, ConnState: ConnStateHidden},
	})

	_, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err == nil {
		t.Fatal("expected a rejection on a homonym collision")
	}
	// [CHANGED, #122/B1] The case-3 loop now returns NAME_TAKEN_OFFLINE for a
	// disconnected holder and NAME_TAKEN for a connected one. With TWO
	// homonyms of different connection states, which one the loop finds
	// FIRST — and hence which reason comes back — depends on Go's
	// (intentionally randomized) map iteration order. Both are a correct,
	// strict rejection; asserting an exact one here would make this test
	// flaky. The invariant this test actually protects (no deletion, no
	// data loss on either homonym) is unaffected and still checked below.
	enrollErr, ok := err.(*EnrollmentError)
	if !ok || (enrollErr.Reason != "NAME_TAKEN" && enrollErr.Reason != "NAME_TAKEN_OFFLINE") {
		t.Errorf("expected EnrollmentError{Reason: NAME_TAKEN or NAME_TAKEN_OFFLINE}, got %v", err)
	}

	bumpers := e.GetTeamsAndBumpers().Bumpers
	if len(bumpers) != 2 {
		t.Fatalf("R1: expected both homonym bumpers to survive untouched, got %d bumpers", len(bumpers))
	}
	first := bumpers["vjoueur_emma_first"]
	if first == nil || first.Team != "TeamA" || first.Score != 40 {
		t.Errorf("R1: first 'Emma' data altered or deleted: %+v", first)
	}
	second := bumpers["vjoueur_emma_second"]
	if second == nil || second.Team != "TeamB" || second.Score != 5 {
		t.Errorf("R1: second 'Emma' data altered or deleted: %+v", second)
	}
}

// assertNameTakenRejection is a shared assertion helper for the NAME_TAKEN
// rejection cases (3/3bis/contradictory).
func assertNameTakenRejection(t *testing.T, bumper *Bumper, reconnected bool, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a NAME_TAKEN rejection, got no error")
	}
	if reconnected {
		t.Errorf("expected reconnected=false on rejection")
	}
	if bumper != nil {
		t.Errorf("expected a nil bumper on rejection, got %+v", bumper)
	}
	enrollErr, ok := err.(*EnrollmentError)
	if !ok {
		t.Fatalf("expected *EnrollmentError, got %T: %v", err, err)
	}
	if enrollErr.Reason != "NAME_TAKEN" {
		t.Errorf("expected Reason=NAME_TAKEN, got %q", enrollErr.Reason)
	}
}

// assertNameTakenOfflineRejection mirrors assertNameTakenRejection for the
// #122 (B1) disconnected-holder variant: same rejection shape (no bumper, no
// reconnection), but Reason=NAME_TAKEN_OFFLINE instead of NAME_TAKEN.
func assertNameTakenOfflineRejection(t *testing.T, bumper *Bumper, reconnected bool, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a NAME_TAKEN_OFFLINE rejection, got no error")
	}
	if reconnected {
		t.Errorf("expected reconnected=false on rejection")
	}
	if bumper != nil {
		t.Errorf("expected a nil bumper on rejection, got %+v", bumper)
	}
	enrollErr, ok := err.(*EnrollmentError)
	if !ok {
		t.Fatalf("expected *EnrollmentError, got %T: %v", err, err)
	}
	if enrollErr.Reason != "NAME_TAKEN_OFFLINE" {
		t.Errorf("expected Reason=NAME_TAKEN_OFFLINE, got %q", enrollErr.Reason)
	}
}

// ---------------------------------------------------------------------------
// Tests : purge des VJoueurs sur InitGame/NEW_GAME (fix R1 follow-up).
//
// [CHANGED] La purge D-R1b a été déplacée de StartEnrollment vers
// InitGame (NEW_GAME) suite à une précision produit de l'utilisateur : une
// partie démarre toujours avec des données vierges, donc il n'existe pas de
// VJoueur "legacy" à nettoyer — la vraie limite de fraîcheur est NEW_GAME,
// pas StartEnrollment (qui peut être rouvert en cours de partie pour inviter
// plus de monde ; y purger les déconnectés évincerait un joueur actif juste
// temporairement coupé). La purge est désormais inconditionnelle (tous les
// VJoueurs, connectés ou non — un nouveau jeu n'hérite d'aucun roster),
// symétrique du reset de score déjà appliqué à tous les bumpers physiques.
// Rappel : ceci reste une hygiène de confort — le rejet NAME_TAKEN à lui
// seul empêche déjà toute perte/fusion de données sur collision de nom.
// ---------------------------------------------------------------------------

func TestInitGame_PurgesAllVirtualPlayers(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"vjoueur_gone":      {Name: "Gone", IsVirtual: true, IsVPlayer: true, Connected: false},
		"vjoueur_here":      {Name: "Here", IsVirtual: true, IsVPlayer: true, Connected: true},
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Connected: false}, // physical, must survive
	})

	e.InitGame()

	if e.GetBumper("vjoueur_gone") != nil {
		t.Errorf("expected disconnected VJoueur to be purged on InitGame (NEW_GAME)")
	}
	if e.GetBumper("vjoueur_here") != nil {
		t.Errorf("expected connected VJoueur to ALSO be purged on InitGame — a new game starts with no VJoueur roster at all")
	}
	if e.GetBumper("AA:BB:CC:DD:EE:01") == nil {
		t.Errorf("physical (non-virtual) buzzer must never be purged by InitGame")
	}
}

func TestInitGame_PurgeFreesUpTheName(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"vjoueur_emma": {Name: "Emma", IsVirtual: true, IsVPlayer: true, Connected: false},
	})

	e.InitGame()
	e.SetPhase(PhaseEnroll)

	// After the purge, "Emma" is no longer taken — a fresh enrollment succeeds.
	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err != nil {
		t.Fatalf("expected 'Emma' to be enrollable again after the purge, got error: %v", err)
	}
	if reconnected {
		t.Errorf("expected a brand-new enrollment, not a reconnection")
	}
	if bumper.Name != "Emma" {
		t.Errorf("expected new bumper named 'Emma', got %q", bumper.Name)
	}
}

// StartEnrollment itself must NOT purge anything anymore (moved to InitGame).
func TestStartEnrollment_DoesNotPurgeVirtualPlayers(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"vjoueur_disconnected": {Name: "Disconnected", IsVirtual: true, IsVPlayer: true, Connected: false},
	})

	e.StartEnrollment(20)

	if e.GetBumper("vjoueur_disconnected") == nil {
		t.Errorf("StartEnrollment must not purge VJoueurs — that's InitGame's job now (fix R1 follow-up)")
	}
}

// ---------------------------------------------------------------------------
// #134 — ReleaseSeat : libérer la place d'un VJoueur, connecté ou non
//
// Contrat : contracts/seat-release.md. Plan : _work/reports/planner-20260804-115318.md
// (T2.1, T2.4, T2.5). CA1, CA4, CA6, CA11 côté moteur — CA2/CA3/CA5/CA7 côté
// handler, testés dans cmd/server/seat_release_test.go.
// ---------------------------------------------------------------------------

// TestReleaseSeat_Disconnected_MatchesReleaseBumperNameExactly is CA1: on a
// disconnected bumper, ReleaseSeat must be behaviorally identical to the
// pre-existing #122 ReleaseBumperName — no eviction, no re-key, same ID,
// same authorization side effects.
func TestReleaseSeat_Disconnected_MatchesReleaseBumperNameExactly(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	newID, wasConnected, ok := e.ReleaseSeat(id)
	if !ok {
		t.Fatal("expected ReleaseSeat to succeed for a valid disconnected virtual bumper")
	}
	if wasConnected {
		t.Error("expected wasConnected=false for a disconnected bumper")
	}
	if newID != "" {
		t.Errorf("expected no new ID for a disconnected release, got %q", newID)
	}

	bumper := e.GetBumper(id)
	if bumper == nil {
		t.Fatal("expected the bumper to still exist under its ORIGINAL id — #122 never re-keys a disconnected bumper")
	}
	if bumper.ReclaimRequested {
		t.Error("expected ReclaimRequested to be cleared")
	}
	if bumper.reclaimAuthorizedUntil.IsZero() || !bumper.reclaimAuthorizedUntil.After(time.Now()) {
		t.Error("expected a reclaim authorization set in the future")
	}
}

// TestReleaseSeat_UnknownID_ReturnsNotOK and
// TestReleaseSeat_PhysicalBuzzer_ReturnsNotOK mirror ReleaseBumperName's own
// not-found contract.
func TestReleaseSeat_UnknownID_ReturnsNotOK(t *testing.T) {
	e := NewEngine()
	if _, _, ok := e.ReleaseSeat("does-not-exist"); ok {
		t.Error("expected ReleaseSeat to return ok=false for an unknown ID")
	}
}

func TestReleaseSeat_PhysicalBuzzer_ReturnsNotOK(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Connected: true},
	})
	if _, _, ok := e.ReleaseSeat("AA:BB:CC:DD:EE:01"); ok {
		t.Error("expected ReleaseSeat to return ok=false for a physical (non-virtual) bumper")
	}
}

// TestReleaseSeat_Connected_ReKeysPreservingScoreTeamAndHistory is the
// central #134 test (CA4): a connected release evicts and re-keys, keeping
// the same struct (score/team survive) under a brand-new, unresolvable-by
// -the-old-id key, with Connected forced false and the badge machine
// reflecting a genuine disconnect (never green — see the "no
// ConnEventReconnect" design note on ReleaseSeat).
func TestReleaseSeat_Connected_ReKeysPreservingScoreTeamAndHistory(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Renards": {Name: "Renards"}})
	e.SetPhase(PhaseEnroll)
	oldID, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(oldID, "Renards", AnswerColorNone); err != nil {
		t.Fatalf("setup AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumperScore(oldID, 42)
	e.UpdateBumper(oldID, map[string]interface{}{"CONNECTED": true})
	// Let the badge settle to Hidden/Green as a real reconnect would — not
	// load-bearing for this test, just representative starting state.

	// IDs are timestamped to the second — wait past the boundary so the new
	// ID is verifiably distinct (same convention as the #122 reclaim test).
	time.Sleep(1100 * time.Millisecond)

	newID, wasConnected, ok := e.ReleaseSeat(oldID)
	if !ok {
		t.Fatal("expected ReleaseSeat to succeed for a connected virtual bumper")
	}
	if !wasConnected {
		t.Error("expected wasConnected=true for a connected bumper")
	}
	if newID == "" || newID == oldID {
		t.Errorf("expected a freshly generated, DIFFERENT ID, got %q (old was %q)", newID, oldID)
	}

	if e.GetBumper(oldID) != nil {
		t.Errorf("expected the old bumper ID %q to no longer resolve after a connected release", oldID)
	}
	bumper := e.GetBumper(newID)
	if bumper == nil {
		t.Fatalf("expected the new bumper ID %q to exist", newID)
	}
	if bumper.Score != 42 {
		t.Errorf("expected Score=42 preserved, got %d", bumper.Score)
	}
	if bumper.Team != "Renards" {
		t.Errorf("expected Team=Renards preserved, got %q", bumper.Team)
	}
	if bumper.Name != "Emma" {
		t.Errorf("expected Name=Emma preserved, got %q", bumper.Name)
	}
	if bumper.Connected {
		t.Error("expected Connected=false after a #134 seat release — the occupant was just evicted, not reconnected")
	}
	if bumper.ConnState == ConnStateGreen {
		t.Error("expected the badge to NEVER read green after a release — that would misleadingly suggest a fresh reconnection (ConnEventReconnect must not fire here)")
	}
	if bumper.reclaimAuthorizedUntil.IsZero() || !bumper.reclaimAuthorizedUntil.After(time.Now()) {
		t.Error("expected a reclaim authorization set in the future on the re-keyed bumper")
	}
}

// TestReleaseSeat_Connected_ThenReclaimByAnyName_PreservesScore is CA6: the
// full round-trip — release while connected, then a nameless PLAYER_CONNECT
// under the SAME name reclaims the seat with its score intact. The plan is
// explicit that this must work for a THIRD PARTY too, not just the original
// occupant — the seat carries the score, not the person — which is
// trivially true here since ReconnectOrCreateVirtualPlayer's reclaim branch
// never checks who is asking, only the name and the authorization.
func TestReleaseSeat_Connected_ThenReclaimByAnyName_PreservesScore(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Renards": {Name: "Renards"}})
	e.SetPhase(PhaseEnroll)
	oldID, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(oldID, "Renards", AnswerColorNone); err != nil {
		t.Fatalf("setup AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumperScore(oldID, 42)
	e.UpdateBumper(oldID, map[string]interface{}{"CONNECTED": true})

	// IDs are timestamped to the second — wait past the boundary at each
	// re-key so the two operations don't collide onto the same generated ID
	// (same convention as the #122 reclaim test).
	time.Sleep(1100 * time.Millisecond)
	releasedID, wasConnected, ok := e.ReleaseSeat(oldID)
	if !ok || !wasConnected {
		t.Fatalf("setup: expected a connected release to succeed, got ok=%v wasConnected=%v", ok, wasConnected)
	}
	time.Sleep(1100 * time.Millisecond)

	// A nameless PLAYER_CONNECT under "Emma" — could be the original player
	// (having purged vplayer_id after PLAYER_EVICTED) or literally anyone
	// else typing the same name; the contract makes no distinction.
	reclaimedID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err != nil {
		t.Fatalf("expected the reclaim to succeed, got: %v", err)
	}
	if !reconnected {
		t.Error("expected reconnected=true — this is a reattachment, not a fresh enrollment")
	}
	if reclaimedID == "" || reclaimedID == releasedID {
		t.Errorf("expected yet another fresh ID for the reclaim, got %q (released bumper was %q)", reclaimedID, releasedID)
	}
	if bumper.Score != 42 {
		t.Errorf("expected Score=42 preserved through the whole release->reclaim cycle, got %d", bumper.Score)
	}
	if bumper.Team != "Renards" {
		t.Errorf("expected Team=Renards preserved, got %q", bumper.Team)
	}
	if !bumper.Connected {
		t.Error("expected Connected=true after the reclaim")
	}
}

// TestReleaseSeat_StaleOldID_RejectedNotResurrected verifies half of CA5 at
// the engine level: once a connected bumper has been released, the OLD id
// is simply gone — a subsequent ReconnectOrCreateVirtualPlayer call with
// that stale ID must fall through to the "unresolvable ID" branch (matrix
// case 2), never resurrect or merge into the released bumper. (The
// PLAYER_REJECTED{SEAT_RELEASED} + eviction-registry side of CA5 is a
// handler-level concern, tested in cmd/server/seat_release_test.go.)
func TestReleaseSeat_StaleOldID_RejectedNotResurrected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	oldID, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(oldID, map[string]interface{}{"CONNECTED": true})
	// ID timestamped to the second — wait past the boundary so the release
	// genuinely generates a DIFFERENT key (same convention as elsewhere in
	// this file); otherwise delete-then-reinsert under the same key would be
	// a no-op and oldID would trivially still resolve, defeating this test.
	time.Sleep(1100 * time.Millisecond)
	if newID, wasConnected, ok := e.ReleaseSeat(oldID); !ok || !wasConnected || newID == oldID {
		t.Fatalf("setup: expected a connected release to succeed with a genuinely different ID, got newID=%q wasConnected=%v ok=%v", newID, wasConnected, ok)
	}

	// The evicted client retries with its now-stale ID (notification lost,
	// or simply hasn't processed it yet).
	_, _, _, err = e.ReconnectOrCreateVirtualPlayer(oldID, "Emma")
	if err == nil {
		t.Fatal("expected a PLAYER_CONNECT with the stale, released ID to be rejected, not silently reconnected")
	}
}

// TestReleaseSeat_DuringPrepare_UnblocksTeamReady is CA11 / T2.4: a bumper
// released while the phase is PREPARE must not permanently block its team
// (and therefore the whole game) from becoming Ready — the mitigation for
// the "partie bloquée" risk identified in the plan.
func TestReleaseSeat_DuringPrepare_UnblocksTeamReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Renards": {Name: "Renards"}})
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "Renards", AnswerColorNone); err != nil {
		t.Fatalf("setup AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": true})
	e.SetPhase(PhasePrepare)

	if e.AreAllTeamsReady() {
		t.Fatal("setup: expected the team to NOT be ready yet — Emma hasn't PONGed")
	}

	if _, wasConnected, ok := e.ReleaseSeat(id); !ok || !wasConnected {
		t.Fatalf("setup: expected a connected release to succeed")
	}

	if !e.AreAllTeamsReady() {
		t.Error("expected the (now-solo) team to become Ready immediately after releasing its only, unresponsive bumper during PREPARE — T2.4 mitigation")
	}
}

// TestReleaseSeat_OutsidePrepare_DoesNotForceReady is the negative
// counterpart: the Ready=true mitigation is scoped strictly to the PREPARE
// phase (per the plan, "portée strictement limitée à ce bumper" — and
// implicitly, to that phase) — releasing a connected bumper in, say,
// PhaseStarted must not fabricate a Ready flag with no phase transition to
// justify it.
func TestReleaseSeat_OutsidePrepare_DoesNotForceReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Renards": {Name: "Renards"}})
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "Renards", AnswerColorNone); err != nil {
		t.Fatalf("setup AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": true})
	e.SetPhase(PhaseStarted)

	newID, _, ok := e.ReleaseSeat(id)
	if !ok {
		t.Fatal("setup: expected the release to succeed")
	}

	if e.GetBumper(newID).Ready {
		t.Error("expected Ready to stay false outside PREPARE — the T2.4 mitigation must not leak to other phases")
	}
}

// TestReleaseSeat_ConcurrentWithReconnect_NoRace is the atomicity guard
// (contract §2, "sous un seul verrou") — a ReleaseSeat racing a concurrent
// ReconnectOrCreateVirtualPlayer for a DIFFERENT identity must never
// corrupt engine state. Run under `go test -race`.
func TestReleaseSeat_ConcurrentWithReconnect_NoRace(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	targetID, _, err := e.CreateVirtualPlayer("Target")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(targetID, map[string]interface{}{"CONNECTED": true})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.ReleaseSeat(targetID)
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			e.ReconnectOrCreateVirtualPlayer("", "Bystander")
		}
	}()
	wg.Wait()
}
