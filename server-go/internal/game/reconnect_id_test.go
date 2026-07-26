package game

import "testing"

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
//   3. Pas d'ID, nom déjà pris (connecté OU déconnecté) -> REJET NAME_TAKEN
//   4. Pas d'ID, nom libre                    -> nouvel enrôlement
//   5. ID fourni + introuvable + nom libre    -> nouvel enrôlement (= cas 4)
//
// Plus : le cas contradictoire (2 bumpers homonymes, données différentes)
// et la purge des VJoueurs sur InitGame/NEW_GAME (D-R1b, déplacée depuis
// StartEnrollment — voir le bloc de tests dédié plus bas).
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

// TestReconnectOrCreateVirtualPlayer_Case3bis_NameTaken_Disconnected_Rejected
// covers matrix case 3 (disconnected variant) — the strict rule: a name
// matching a DISCONNECTED VJoueur is rejected too, no silent adoption. The
// legitimate owner has their ID and takes the case-1 path instead.
func TestReconnectOrCreateVirtualPlayer_Case3bis_NameTaken_Disconnected_Rejected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")

	assertNameTakenRejection(t, bumper, reconnected, err)

	// The existing (disconnected) bumper must be entirely untouched.
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
		t.Fatal("expected NAME_TAKEN rejection on a homonym collision")
	}
	if enrollErr, ok := err.(*EnrollmentError); !ok || enrollErr.Reason != "NAME_TAKEN" {
		t.Errorf("expected EnrollmentError{Reason: NAME_TAKEN}, got %v", err)
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
