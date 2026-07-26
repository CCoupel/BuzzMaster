package game

import "testing"

// ---------------------------------------------------------------------------
// Tests : machine d'état CONN_STATE (badge connexion 4 états)
//
// Contrat testé (écrit avant l'implémentation dev-backend — TDD, voir plan
// _work/reports/planner-20260725-105503-final.md §2/§4) :
//
//   - Bumper.ConnState string `json:"CONN_STATE"` — valeurs "" (HIDDEN),
//     "orange", "red", "green".
//   - type ConnEvent string avec les constantes :
//       ConnEventDisconnect        ConnEvent = "DISCONNECT"
//       ConnEventReconnect        ConnEvent = "RECONNECT"
//       ConnEventMessageLost       ConnEvent = "MESSAGE_LOST"
//       ConnEventDeliveryConfirmed ConnEvent = "DELIVERY_CONFIRMED"
//   - func (e *Engine) TransitionConn(bumperID string, event ConnEvent)
//     applique la table de transitions ci-dessous, thread-safe (lock moteur),
//     et n'a aucun effet si le bumper n'existe pas ou n'est pas participant
//     (Team == "").
//
// IMPORTANT pour dev-backend : ces noms (type/constantes/méthode) sont ceux
// utilisés par ce fichier de test. Si l'implémentation choisit des noms
// différents, ce fichier ne compilera pas — voir le rapport test-writer pour
// la coordination du contrat exact avec le CDP.
//
// Ce batch (Phase 1 du plan) ne couvre QUE la fonction pure de transition et
// les 3 hooks d'écriture du champ TEAM (UpdateBumper/AssignVirtualPlayer/
// SetBumpers). Le déclenchement automatique de MessageLost/DeliveryConfirmed
// depuis les broadcasts/ACK réels et le timer 2s du VERT sont Phase 2 — hors
// scope de ce batch (voir handoff _work/handoff/task-test-writer-20260725-093000.md).
// ---------------------------------------------------------------------------

// newParticipantBumper creates a connected, team-assigned bumper (a "participant"
// per the plan's scope rule) and returns its ID.
func newParticipantBumper(e *Engine) string {
	id := "bumper-conn-1"
	e.UpdateBumper(id, map[string]interface{}{
		"NAME":      "Buzzer1",
		"TEAM":      "TeamA",
		"CONNECTED": true,
	})
	return id
}

// TestTransitionConn_Table exercises every cell of the definitive transition
// table (plan §2): 4 states × 4 events. Cells marked "(n/a)" in the plan are
// treated as defensive no-ops (state unchanged), consistent with a pure,
// idempotent state machine.
func TestTransitionConn_Table(t *testing.T) {
	type cell struct {
		event    ConnEvent
		expected string
	}

	// reachState drives a fresh participant bumper to the target starting
	// state using only legitimate transitions (no direct field injection).
	reachState := func(e *Engine, id string, target string) {
		switch target {
		case "":
			// Default state, nothing to do.
		case "orange":
			e.TransitionConn(id, ConnEventDisconnect)
		case "red":
			e.TransitionConn(id, ConnEventDisconnect)
			e.TransitionConn(id, ConnEventMessageLost)
		case "green":
			e.TransitionConn(id, ConnEventDisconnect)
			e.TransitionConn(id, ConnEventReconnect)
		default:
			t.Fatalf("unknown target state %q in test setup", target)
		}
	}

	table := map[string][]cell{
		"": {
			{ConnEventDisconnect, "orange"},
			{ConnEventReconnect, ""},        // (n/a) — no-op
			{ConnEventMessageLost, ""},      // explicit in plan table
			{ConnEventDeliveryConfirmed, ""}, // explicit in plan table
		},
		"orange": {
			{ConnEventDisconnect, "orange"}, // idempotent
			{ConnEventReconnect, "green"},
			{ConnEventMessageLost, "red"},
			{ConnEventDeliveryConfirmed, "orange"}, // (n/a) — no-op
		},
		"red": {
			{ConnEventDisconnect, "red"}, // idempotent
			{ConnEventReconnect, "green"},
			{ConnEventMessageLost, "red"}, // idempotent — pertes multiples
			{ConnEventDeliveryConfirmed, "red"}, // (n/a) — no-op
		},
		"green": {
			{ConnEventDisconnect, "orange"}, // re-déconnexion pendant vert (D2)
			{ConnEventReconnect, "green"},   // idempotent
			{ConnEventMessageLost, "green"}, // (n/a) — no-op
			{ConnEventDeliveryConfirmed, ""}, // HIDDEN (fenêtre 2s gérée en Phase 2, pas testée ici)
		},
	}

	for startState, cells := range table {
		for _, c := range cells {
			startState, c := startState, c
			t.Run("from="+quoteState(startState)+"/event="+string(c.event), func(t *testing.T) {
				e := NewEngine()
				id := newParticipantBumper(e)
				reachState(e, id, startState)

				// Sanity check: we actually reached the intended starting state.
				if got := e.GetBumper(id).ConnState; got != startState {
					t.Fatalf("setup failed: expected starting state %q, got %q", startState, got)
				}

				e.TransitionConn(id, c.event)

				if got := e.GetBumper(id).ConnState; got != c.expected {
					t.Errorf("TransitionConn(%s, %s): expected ConnState=%q, got %q", startState, c.event, c.expected, got)
				}
			})
		}
	}
}

func quoteState(s string) string {
	if s == "" {
		return "HIDDEN"
	}
	return s
}

// TestTransitionConn_Idempotence_MultipleMessageLost verifies that repeated
// MessageLost events while already RED leave the bumper RED (no further
// degradation, no panic) — explicit acceptance criterion (plan §5).
func TestTransitionConn_Idempotence_MultipleMessageLost(t *testing.T) {
	e := NewEngine()
	id := newParticipantBumper(e)

	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventMessageLost)
	for i := 0; i < 5; i++ {
		e.TransitionConn(id, ConnEventMessageLost)
	}

	if got := e.GetBumper(id).ConnState; got != "red" {
		t.Errorf("expected ConnState to remain 'red' after repeated MessageLost, got %q", got)
	}
}

// TestTransitionConn_RedisconnectDuringGreen verifies D2: a disconnect that
// occurs while the badge is still GREEN (reconnection grace window) reverts
// to ORANGE, not back to HIDDEN or GREEN.
func TestTransitionConn_RedisconnectDuringGreen(t *testing.T) {
	e := NewEngine()
	id := newParticipantBumper(e)

	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)
	if got := e.GetBumper(id).ConnState; got != "green" {
		t.Fatalf("setup failed: expected 'green' after reconnect, got %q", got)
	}

	e.TransitionConn(id, ConnEventDisconnect)

	if got := e.GetBumper(id).ConnState; got != "orange" {
		t.Errorf("expected re-disconnect during GREEN to revert to 'orange', got %q", got)
	}
}

// TestTransitionConn_NonParticipant_NoTransition verifies the scope filter:
// a bumper with Team=="" (not a participant) never transitions, regardless
// of the event received (plan §2 "Filtre périmètre").
func TestTransitionConn_NonParticipant_NoTransition(t *testing.T) {
	e := NewEngine()
	id := "bumper-non-participant"
	e.UpdateBumper(id, map[string]interface{}{
		"NAME":      "Spectator",
		"CONNECTED": true,
		// TEAM intentionally omitted / empty
	})

	events := []ConnEvent{ConnEventDisconnect, ConnEventMessageLost, ConnEventReconnect, ConnEventDeliveryConfirmed}
	for _, ev := range events {
		e.TransitionConn(id, ev)
		if got := e.GetBumper(id).ConnState; got != "" {
			t.Errorf("non-participant bumper: event %s must not transition ConnState, got %q", ev, got)
		}
	}
}

// TestTransitionConn_UnknownBumper_NoPanic verifies TransitionConn is a safe
// no-op when called with an ID that doesn't exist in the engine (defensive:
// hooks may fire in races against a bumper removal).
func TestTransitionConn_UnknownBumper_NoPanic(t *testing.T) {
	e := NewEngine()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TransitionConn on unknown bumper ID must not panic, got: %v", r)
		}
	}()
	e.TransitionConn("does-not-exist", ConnEventDisconnect)
}

// ---------------------------------------------------------------------------
// Tests : hooks d'assignation/désassignation d'équipe (filtre périmètre)
// ---------------------------------------------------------------------------

// TestUpdateBumper_TeamAssignment_DisconnectedBumper_BecomesOrange verifies
// that assigning a team to an already-disconnected bumper immediately
// evaluates its ConnState to "orange" (plan §2: "Assignation ... sur un
// bumper déconnecté → évaluer immédiatement").
func TestUpdateBumper_TeamAssignment_DisconnectedBumper_BecomesOrange(t *testing.T) {
	e := NewEngine()
	id := "bumper-late-assign"

	// Disconnected, non-participant bumper (e.g. a buzzer that HELLO'd once
	// then went silent before ever being assigned to a team).
	e.UpdateBumper(id, map[string]interface{}{
		"NAME":      "Buzzer2",
		"CONNECTED": false,
	})
	if got := e.GetBumper(id).ConnState; got != "" {
		t.Fatalf("setup failed: expected ConnState=='' for unassigned bumper, got %q", got)
	}

	// Now assign it to a team while still disconnected.
	e.UpdateBumper(id, map[string]interface{}{"TEAM": "TeamA"})

	if got := e.GetBumper(id).ConnState; got != "orange" {
		t.Errorf("expected ConnState=='orange' immediately after assigning a disconnected bumper, got %q", got)
	}
}

// TestUpdateBumper_TeamAssignment_ConnectedBumper_StaysHidden verifies that
// assigning a team to a bumper that IS currently connected does not
// spuriously raise a badge.
func TestUpdateBumper_TeamAssignment_ConnectedBumper_StaysHidden(t *testing.T) {
	e := NewEngine()
	id := "bumper-connected-assign"

	e.UpdateBumper(id, map[string]interface{}{
		"NAME":      "Buzzer3",
		"CONNECTED": true,
	})
	e.UpdateBumper(id, map[string]interface{}{"TEAM": "TeamA"})

	if got := e.GetBumper(id).ConnState; got != "" {
		t.Errorf("expected ConnState=='' when assigning an already-connected bumper, got %q", got)
	}
}

// TestUpdateBumper_TeamUnassignment_ForcesConnStateHidden verifies that
// removing a bumper's team assignment forces ConnState back to "" — it
// leaves the badge's scope entirely (plan §2: "Désassignation ... → forcer
// ConnState = ''").
func TestUpdateBumper_TeamUnassignment_ForcesConnStateHidden(t *testing.T) {
	e := NewEngine()
	id := newParticipantBumper(e)

	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventMessageLost)
	if got := e.GetBumper(id).ConnState; got != "red" {
		t.Fatalf("setup failed: expected 'red' before unassignment, got %q", got)
	}

	e.UpdateBumper(id, map[string]interface{}{"TEAM": ""})

	if got := e.GetBumper(id).ConnState; got != "" {
		t.Errorf("expected ConnState=='' after unassigning team from a 'red' bumper, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Tests : hook AssignVirtualPlayer (site engine.go ~2488)
// ---------------------------------------------------------------------------

func TestAssignVirtualPlayer_DisconnectedBumper_BecomesOrange(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)

	id, _, err := e.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	// Simulate the player's socket having dropped before being assigned to a team.
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}

	if got := e.GetBumper(id).ConnState; got != "orange" {
		t.Errorf("expected ConnState=='orange' after assigning a disconnected VJoueur, got %q", got)
	}
}

func TestAssignVirtualPlayer_ConnectedBumper_StaysHidden(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)

	// CreateVirtualPlayer sets Connected=true by construction.
	id, _, err := e.CreateVirtualPlayer("Bob")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}

	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}

	if got := e.GetBumper(id).ConnState; got != "" {
		t.Errorf("expected ConnState=='' for a connected VJoueur just assigned, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Tests : hook SetBumpers (restauration en bloc, site engine.go ~205)
// ---------------------------------------------------------------------------

// TestSetBumpers_RecomputesConnStateForParticipants verifies that a bulk
// bumper restore (e.g. backup restore, SetBumpers) re-evaluates ConnState
// for every participant based on their persisted Connected flag, and leaves
// non-participants hidden regardless of Connected.
func TestSetBumpers_RecomputesConnStateForParticipants(t *testing.T) {
	e := NewEngine()

	bumpers := map[string]*Bumper{
		"disconnected-participant": {Name: "P1", Team: "TeamA", Connected: false},
		"connected-participant":    {Name: "P2", Team: "TeamA", Connected: true},
		"disconnected-spectator":   {Name: "S1", Team: "", Connected: false},
	}
	e.SetBumpers(bumpers)

	if got := e.GetBumper("disconnected-participant").ConnState; got != "orange" {
		t.Errorf("disconnected participant after SetBumpers: expected ConnState=='orange', got %q", got)
	}
	if got := e.GetBumper("connected-participant").ConnState; got != "" {
		t.Errorf("connected participant after SetBumpers: expected ConnState=='', got %q", got)
	}
	if got := e.GetBumper("disconnected-spectator").ConnState; got != "" {
		t.Errorf("non-participant after SetBumpers: expected ConnState=='' regardless of Connected, got %q", got)
	}
}
