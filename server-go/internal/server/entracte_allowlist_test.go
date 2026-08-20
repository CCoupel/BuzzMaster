package server

import (
	"buzzcontrol/internal/protocol"
	"testing"
)

// entracteExpectedAllowed is the hand-authored, human-decided expectation
// for every CLIENT→SERVER action currently known to the protocol (v6.5.2,
// #119, D6, contracts/websocket-actions.md §"Actions refusées pendant
// l'entracte"). true = allowed to proceed while GameState.ENTRACTE is
// active, false = refused.
//
// TestIsActionAllowedDuringEntracte_ExhaustiveOverAllInboundActions below
// sweeps EVERY key of inboundActionAllowlist (the map ENUMERATES every
// action a web client may ever send — see its own doc comment) plus the two
// actions that live outside it (SET_CLIENT_TYPE, gated separately by
// IsSetClientTypeAllowed; ENTRACTE_SET itself). An action present in
// inboundActionAllowlist but MISSING from this map fails the test loudly —
// exactly the "a forgotten action fails, rather than being silently
// permitted" guarantee the plan requires (risk table "Une action ajoutée
// plus tard échappe à la garde").
var entracteExpectedAllowed = map[string]bool{
	// --- Allowed during entracte (D6) ---------------------------------
	protocol.ActionEntracteSet:       true,
	protocol.ActionHello:             true,
	protocol.ActionSetClientType:     true,
	protocol.ActionPlayerConnect:     true,
	protocol.ActionRegieMessageSend:  true,
	protocol.ActionRegieMessageClear: true,
	protocol.ActionPong:              true,

	// --- Refused during entracte (everything else) --------------------
	protocol.ActionReady:                 false,
	protocol.ActionStart:                 false,
	protocol.ActionStop:                  false,
	protocol.ActionPause:                 false,
	protocol.ActionContinue:              false,
	protocol.ActionReveal:                false,
	protocol.ActionBumperPoints:          false,
	protocol.ActionTeamPoints:            false,
	protocol.ActionFull:                  false,
	protocol.ActionUpdate:                false,
	protocol.ActionPoints:                false,
	protocol.ActionRAZ:                   false,
	protocol.ActionRemote:                false,
	protocol.ActionDelete:                false,
	protocol.ActionDeleteBumper:          false,
	protocol.ActionReleaseBumperName:     false,
	protocol.ActionReset:                 false,
	protocol.ActionReboot:                false,
	protocol.ActionReorderQuestions:      false,
	protocol.ActionForceReady:            false,
	protocol.ActionMemorySetTeams:        false,
	protocol.ActionMotionSetTeams:        false,
	protocol.ActionShowQRCode:            false,
	protocol.ActionHideQRCode:            false,
	protocol.ActionSetVirtualPlayerLimit: false,
	protocol.ActionNewGame:               false,
	protocol.ActionUpdateQuizMeta:        false,
	protocol.ActionSetCreditPoints:       false,
	protocol.ActionCancelAIGeneration:    false,
	protocol.ActionButton:                false,
	protocol.ActionFlipMemoryCard:        false,
	protocol.ActionMotionSelect:          false,
	protocol.ActionMotionFlip:            false,
	protocol.ActionMotionStopTimer:       false,
	protocol.ActionMotionReveal:          false,
	protocol.ActionMotionDone:            false,
	protocol.ActionVPlayerQCMAnswer:      false,
	protocol.ActionArdoiseInput:          false,
}

// TestIsActionAllowedDuringEntracte_ExhaustiveOverAllInboundActions is the
// "table exhaustive" the plan requires (B8): it does not hand-pick a subset
// of actions to check, it enumerates inboundActionAllowlist's own keys (the
// map already IS the closed set of every action a web client can send) plus
// the two actions handled outside it, and requires a verdict to be recorded
// for each. A future action added to inboundActionAllowlist without a
// corresponding entry here fails this test immediately, forcing an explicit
// entracte decision before it can ship.
func TestIsActionAllowedDuringEntracte_ExhaustiveOverAllInboundActions(t *testing.T) {
	allActions := make([]string, 0, len(inboundActionAllowlist)+2)
	for action := range inboundActionAllowlist {
		allActions = append(allActions, action)
	}
	// SET_CLIENT_TYPE lives outside inboundActionAllowlist (gated by
	// IsSetClientTypeAllowed instead) but is still a real inbound action the
	// entracte guard must have an opinion on.
	allActions = append(allActions, protocol.ActionSetClientType)

	if len(allActions) != len(entracteExpectedAllowed) {
		t.Fatalf("action set size mismatch: inboundActionAllowlist+SET_CLIENT_TYPE has %d actions, entracteExpectedAllowed has %d — "+
			"a new action was added to the protocol without recording an explicit ENTRACTE decision here (or vice versa)",
			len(allActions), len(entracteExpectedAllowed))
	}

	for _, action := range allActions {
		want, ok := entracteExpectedAllowed[action]
		if !ok {
			t.Errorf("action %q is in inboundActionAllowlist but has no recorded ENTRACTE decision in entracteExpectedAllowed", action)
			continue
		}
		got := IsActionAllowedDuringEntracte(action)
		if got != want {
			t.Errorf("IsActionAllowedDuringEntracte(%q) = %v, want %v", action, got, want)
		}
	}
}

// TestIsActionAllowedDuringEntracte_UnknownActionRefused: an action this
// build has never heard of (e.g. a message from a NEWER client) must be
// refused, not silently allowed through — default-deny at the boundary too.
func TestIsActionAllowedDuringEntracte_UnknownActionRefused(t *testing.T) {
	if IsActionAllowedDuringEntracte("SOME_FUTURE_ACTION_NOBODY_HAS_TAUGHT_THIS_MAP_ABOUT") {
		t.Error("expected an unrecognized action to be refused during entracte, got allowed")
	}
}

// TestIsActionAllowedDuringEntracte_EntracteSetAllowed pins down the single
// most important entry by name: without it, entracte would have no exit.
func TestIsActionAllowedDuringEntracte_EntracteSetAllowed(t *testing.T) {
	if !IsActionAllowedDuringEntracte(protocol.ActionEntracteSet) {
		t.Fatal("ENTRACTE_SET must always be allowed during entracte — otherwise entracte has no way out")
	}
}
