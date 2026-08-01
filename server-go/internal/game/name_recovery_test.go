package game

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : reprise de place assistée par l'animateur (#122, couvre #124)
//
// Plan : _work/reports/plan-20260730-123000.md — tâches B1/B2/B3.
//
// #109 R1 a fait de l'ID de bumper la SEULE preuve de propriété d'un VJoueur.
// Contrepartie : un téléphone qui perd son ID (stockage vidé, changement
// d'appareil, rejet antérieur) ne peut plus jamais reprendre sa place — le
// nom retombe systématiquement sur NAME_TAKEN, y compris pour son propre
// pseudo. #122 ouvre une brèche volontaire et étroite dans cette garantie :
// une autorisation de reprise, à usage unique, bornée dans le temps, et
// décidée par un humain (l'animateur) — jamais automatique.
//
// TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_Rejected et
// TestReconnectOrCreateVirtualPlayer_Case3bis_NameTakenOffline_Disconnected_Rejected
// (reconnect_id_test.go) couvrent B1 lui-même ; ce fichier couvre B2 (le
// marquage ReclaimRequested) et B3 (l'autorisation et le rattachement).
// ---------------------------------------------------------------------------

// withShortReclaimTTL temporarily shrinks reclaimAuthorizationTTL for a test,
// same convention as withShortGreenWindow (engine_connstate_phase2_test.go)
// for connGreenMinDuration.
func withShortReclaimTTL(t *testing.T, d time.Duration) {
	t.Helper()
	original := reclaimAuthorizationTTL
	reclaimAuthorizationTTL = d
	t.Cleanup(func() { reclaimAuthorizationTTL = original })
}

// ---------------------------------------------------------------------------
// B2 — ReclaimRequested : posé sur refus, retiré sur reconnexion normale
// ---------------------------------------------------------------------------

func TestReconnectOrCreateVirtualPlayer_Case3_NameTakenOffline_SetsReclaimRequested(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	before := e.GetBumper(id)
	if before.ReclaimRequested {
		t.Fatal("setup: ReclaimRequested should start false")
	}

	_, _, _, err = e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err == nil {
		t.Fatal("expected a NAME_TAKEN_OFFLINE rejection")
	}

	after := e.GetBumper(id)
	if !after.ReclaimRequested {
		t.Error("expected ReclaimRequested=true on the holder after a failed reclaim attempt (NAME_TAKEN_OFFLINE)")
	}
}

// TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_DoesNotSetReclaimRequested
// is the counterpart: a CONNECTED holder is a genuine homonym, not a failed
// reclaim — ReclaimRequested must never be set for that case.
func TestReconnectOrCreateVirtualPlayer_Case3_NameTaken_Connected_DoesNotSetReclaimRequested(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma") // Connected=true by construction
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}

	_, _, _, err = e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err == nil {
		t.Fatal("expected a NAME_TAKEN rejection")
	}

	after := e.GetBumper(id)
	if after.ReclaimRequested {
		t.Error("a CONNECTED holder is a homonym collision, not a failed reclaim — ReclaimRequested must stay false")
	}
}

func TestReconnectOrCreateVirtualPlayer_Case1_NormalReconnect_ClearsReclaimRequested(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	// Arm ReclaimRequested exactly like a failed reclaim would.
	_, _, _, _ = e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if !e.GetBumper(id).ReclaimRequested {
		t.Fatal("setup: expected ReclaimRequested=true before the normal reconnection")
	}

	// The legitimate owner comes back on their own, with their real ID.
	gotID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer(id, "Emma")
	if err != nil {
		t.Fatalf("expected a normal reconnection to succeed, got: %v", err)
	}
	if !reconnected || gotID != id {
		t.Fatalf("expected a normal case-1 reconnection, got id=%s reconnected=%v", gotID, reconnected)
	}
	if bumper.ReclaimRequested {
		t.Error("expected ReclaimRequested to be cleared once the owner reconnects normally — the seat is no longer being requested")
	}
}

// ---------------------------------------------------------------------------
// B3 — ReleaseBumperName : autorisation, rattachement, usage unique
// ---------------------------------------------------------------------------

func TestReleaseBumperName_GrantsAuthorizationAndClearsReclaimRequested(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.ReconnectOrCreateVirtualPlayer("", "Emma") // arm ReclaimRequested

	if ok := e.ReleaseBumperName(id); !ok {
		t.Fatal("expected ReleaseBumperName to succeed for a valid virtual bumper ID")
	}

	bumper := e.GetBumper(id)
	if bumper.ReclaimRequested {
		t.Error("expected ReclaimRequested to be cleared once the animateur grants a release")
	}
	if bumper.reclaimAuthorizedUntil.IsZero() || !bumper.reclaimAuthorizedUntil.After(time.Now()) {
		t.Error("expected a reclaim authorization set in the future")
	}
}

func TestReleaseBumperName_UnknownID_ReturnsFalse(t *testing.T) {
	e := NewEngine()
	if ok := e.ReleaseBumperName("does-not-exist"); ok {
		t.Error("expected ReleaseBumperName to return false for an unknown ID")
	}
}

func TestReleaseBumperName_PhysicalBuzzer_ReturnsFalse(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false},
	})
	if ok := e.ReleaseBumperName("AA:BB:CC:DD:EE:01"); ok {
		t.Error("expected ReleaseBumperName to return false for a physical (non-virtual) bumper")
	}
}

// TestReconnectOrCreateVirtualPlayer_Reclaim_ReattachesPreservingScoreAndTeam
// is the central B3 test: after a release, a nameless PLAYER_CONNECT for
// that name reattaches to the EXISTING bumper — new ID, but score/team
// preserved — rather than rejecting or creating a blank new enrollment.
func TestReconnectOrCreateVirtualPlayer_Reclaim_ReattachesPreservingScoreAndTeam(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Renards": {Name: "Renards"}})
	e.SetPhase(PhaseEnroll)
	oldID, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(oldID, map[string]interface{}{
		"CONNECTED": false,
		"TEAM":      "Renards",
	})
	e.UpdateBumperScore(oldID, 15) // UpdateBumper itself has no SCORE key — this is the real API for it
	e.ReleaseBumperName(oldID)

	// IDs are timestamped to the second (vjoueur_<name>_<YYYYMMDD_HHMMSS>) —
	// a known, accepted limitation (see the plan's closing notes) means a
	// reclaim within the SAME second as the original enrollment can collide
	// with its own old ID. Harmless in that case (delete-then-reinsert under
	// the same key is a no-op), but this test specifically verifies a fresh
	// ID is generated, so it waits past the second boundary — representative
	// of real usage, where a human takes more than a second to click.
	time.Sleep(1100 * time.Millisecond)

	newID, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err != nil {
		t.Fatalf("expected the reclaim to succeed, got: %v", err)
	}
	if !reconnected {
		t.Error("expected reconnected=true — this is a reattachment, not a fresh enrollment")
	}
	if newID == "" || newID == oldID {
		t.Errorf("expected a freshly generated, DIFFERENT ID, got %q (old was %q)", newID, oldID)
	}
	if bumper.Score != 15 {
		t.Errorf("expected Score=15 preserved, got %d", bumper.Score)
	}
	if bumper.Team != "Renards" {
		t.Errorf("expected Team=Renards preserved, got %q", bumper.Team)
	}
	if !bumper.Connected {
		t.Error("expected Connected=true after reattachment")
	}

	// The old ID must be gone — this is a rename, not a duplication.
	if e.GetBumper(oldID) != nil {
		t.Errorf("expected the old bumper ID %q to no longer exist after reattachment", oldID)
	}
	if e.GetBumper(newID) == nil {
		t.Errorf("expected the new bumper ID %q to exist after reattachment", newID)
	}
}

// TestReconnectOrCreateVirtualPlayer_Reclaim_SingleUse is the central
// usage-once test the plan calls out explicitly: a second nameless attempt
// after the authorization was already consumed must fail normally — the
// reattached bumper is now Connected=true, so it falls through to the
// ordinary (non-offline) NAME_TAKEN homonym rejection.
func TestReconnectOrCreateVirtualPlayer_Reclaim_SingleUse(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	oldID, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(oldID, map[string]interface{}{"CONNECTED": false})
	e.ReleaseBumperName(oldID)

	// First attempt: succeeds, consumes the authorization.
	_, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err != nil || !reconnected {
		t.Fatalf("setup: expected the first reclaim to succeed, got reconnected=%v err=%v", reconnected, err)
	}

	// Second attempt, same name, still no ID: must be rejected — the
	// authorization was single-use, and the reattached bumper is connected.
	_, bumper, reconnected2, err2 := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err2 == nil {
		t.Fatal("expected the second reclaim attempt to be rejected — the authorization was already consumed")
	}
	if reconnected2 || bumper != nil {
		t.Errorf("expected a clean rejection (no bumper, reconnected=false), got bumper=%+v reconnected=%v", bumper, reconnected2)
	}
	enrollErr, ok := err2.(*EnrollmentError)
	if !ok || enrollErr.Reason != "NAME_TAKEN" {
		t.Errorf("expected Reason=NAME_TAKEN (the reattached bumper is now connected), got %v", err2)
	}
}

// TestReconnectOrCreateVirtualPlayer_Reclaim_NoAuthorization_StillRejected is
// the #109 guarantee test: without ANY release from the animateur, a
// nameless PLAYER_CONNECT never reattaches, no matter how disconnected the
// holder is — only an explicit human grant opens the door.
func TestReconnectOrCreateVirtualPlayer_Reclaim_NoAuthorization_StillRejected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	// No ReleaseBumperName call — no authorization exists.

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err == nil {
		t.Fatal("expected a rejection — no reclaim authorization was ever granted")
	}
	if reconnected || bumper != nil {
		t.Errorf("expected a clean rejection, got bumper=%+v reconnected=%v", bumper, reconnected)
	}
}

// TestReconnectOrCreateVirtualPlayer_Reclaim_ExpiredAuthorization_Rejected
// verifies the time bound: an authorization past its TTL no longer allows a
// reattachment — the backdated field is set directly (same package) rather
// than sleeping for real time.
func TestReconnectOrCreateVirtualPlayer_Reclaim_ExpiredAuthorization_Rejected(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.ReleaseBumperName(id)

	// Force the authorization into the past.
	e.mu.Lock()
	e.data.Bumpers[id].reclaimAuthorizedUntil = time.Now().Add(-time.Second)
	e.mu.Unlock()

	_, bumper, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err == nil {
		t.Fatal("expected the expired authorization to be rejected, not honored")
	}
	if reconnected || bumper != nil {
		t.Errorf("expected a clean rejection, got bumper=%+v reconnected=%v", bumper, reconnected)
	}
	enrollErr, ok := err.(*EnrollmentError)
	if !ok || enrollErr.Reason != "NAME_TAKEN_OFFLINE" {
		t.Errorf("expected Reason=NAME_TAKEN_OFFLINE (holder still disconnected), got %v", err)
	}
}

// TestReleaseBumperName_LiftedByNormalReconnectionInTheMeantime verifies the
// third bound: if the bumper reconnects normally (case 1, with its real ID)
// before anyone reclaims it, the authorization is lifted — a later nameless
// attempt must NOT reattach (the seat is legitimately occupied again).
func TestReleaseBumperName_LiftedByNormalReconnectionInTheMeantime(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.ReleaseBumperName(id)

	// The original owner reconnects normally with their real ID before
	// anyone uses the reclaim window.
	_, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer(id, "Emma")
	if err != nil || !reconnected {
		t.Fatalf("setup: expected the normal reconnection to succeed, got reconnected=%v err=%v", reconnected, err)
	}

	// A later nameless attempt under "Emma" must NOT reattach — the seat is
	// legitimately occupied (Connected=true) and the authorization is gone.
	_, bumper, reconnected2, err2 := e.ReconnectOrCreateVirtualPlayer("", "Emma")
	if err2 == nil {
		t.Fatal("expected a rejection — the authorization was lifted by the normal reconnection")
	}
	if reconnected2 || bumper != nil {
		t.Errorf("expected a clean rejection, got bumper=%+v reconnected=%v", bumper, reconnected2)
	}
	enrollErr, ok := err2.(*EnrollmentError)
	if !ok || enrollErr.Reason != "NAME_TAKEN" {
		t.Errorf("expected Reason=NAME_TAKEN (holder is connected again), got %v", err2)
	}
}

// TestReconnectOrCreateVirtualPlayer_Reclaim_ConcurrentAttempts_OnlyOneSucceeds
// is the concurrency test the plan calls out: two goroutines racing the SAME
// authorization must never both succeed — the engine's single mutex makes
// read+consume atomic, same guarantee as the rest of the matrix.
func TestReconnectOrCreateVirtualPlayer_Reclaim_ConcurrentAttempts_OnlyOneSucceeds(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("setup CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.ReleaseBumperName(id)

	const n = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	var ids []string

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newID, _, reconnected, err := e.ReconnectOrCreateVirtualPlayer("", "Emma")
			if err == nil && reconnected {
				mu.Lock()
				successes++
				ids = append(ids, newID)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("expected exactly 1 of %d concurrent reclaim attempts to succeed, got %d (ids: %v)", n, successes, ids)
	}
}

// ---------------------------------------------------------------------------
// Sérialisation — RECLAIM_REQUESTED sans omitempty (règle du projet)
// ---------------------------------------------------------------------------

func TestBumper_ReclaimRequested_SerializedEvenWhenFalse(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"vjoueur_emma": {Name: "Emma", IsVirtual: true, IsVPlayer: true},
	})

	// GetGameJSON is the FULL/UPDATE payload actually diffused to clients
	// (admin sees it in full) — same pattern as
	// TestGetGameJSON_IncludesColorName (team_color_persistence_test.go, #113).
	raw := e.GetGameJSON()
	if !contains(string(raw), `"RECLAIM_REQUESTED":false`) {
		t.Errorf("expected RECLAIM_REQUESTED to serialize as false (not omitted) in GetGameJSON(), got: %s", raw)
	}

	var decoded struct {
		Bumpers map[string]struct {
			ReclaimRequested bool `json:"RECLAIM_REQUESTED"`
		} `json:"bumpers"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GetGameJSON output: %v", err)
	}
	if _, ok := decoded.Bumpers["vjoueur_emma"]; !ok {
		t.Fatal("vjoueur_emma missing from GetGameJSON() bumpers")
	}
}
