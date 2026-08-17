package server

import (
	"buzzcontrol/internal/protocol"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #155/#156 (v6.2.0) — inbound allow-list entries for ClientTypeAnim
// (plan tâche B3, contracts/websocket-actions.md §"Sécurité — Allow-list
// entrante").
//
// Design intent unchanged from #154 (inbound_allowlist.go's own doc comment):
// a new ClientType only needs new map entries, never a change to
// IsActionAllowed/IsSetClientTypeAllowed themselves. These tests exercise
// the POLICY only (pure map lookup, no WebSocket wiring) — see
// cmd/server/inbound_allowlist_anim_test.go for the end-to-end
// handleWebMessage exercise (rejection + logging, B6).
// ---------------------------------------------------------------------------

func TestIsActionAllowed_Anim(t *testing.T) {
	tests := []struct {
		name   string
		action string
		want   bool
	}{
		// Handshake.
		{"HELLO", protocol.ActionHello, true},

		// "Conduite en direct" — the exact périmètre acté for the animateur
		// (contracts/websocket-actions.md, plan §3 B3).
		{"START", protocol.ActionStart, true},
		{"STOP", protocol.ActionStop, true},
		{"PAUSE", protocol.ActionPause, true},
		{"CONTINUE", protocol.ActionContinue, true},
		{"REVEAL", protocol.ActionReveal, true},
		{"READY", protocol.ActionReady, true},
		{"BUMPER_POINTS", protocol.ActionBumperPoints, true},
		{"TEAM_POINTS", protocol.ActionTeamPoints, true},

		// Explicitly NOT opened to anim in #155/#156 — régie/configuration/
		// destructive scope (contracts/websocket-actions.md "Ne PAS ouvrir").
		{"NEW_GAME", protocol.ActionNewGame, false},
		{"RAZ", protocol.ActionRAZ, false},
		{"REMOTE", protocol.ActionRemote, false},
		{"DELETE", protocol.ActionDelete, false},
		{"DELETE_BUMPER", protocol.ActionDeleteBumper, false},
		{"RELEASE_BUMPER_NAME", protocol.ActionReleaseBumperName, false},
		{"RESET", protocol.ActionReset, false},
		{"REBOOT", protocol.ActionReboot, false},
		{"REORDER_QUESTIONS", protocol.ActionReorderQuestions, false},
		{"FORCE_READY", protocol.ActionForceReady, false},
		{"UPDATE_QUIZ_META", protocol.ActionUpdateQuizMeta, false},
		{"SET_VIRTUAL_PLAYER_LIMIT", protocol.ActionSetVirtualPlayerLimit, false},
		{"SHOW_QR_CODE", protocol.ActionShowQRCode, false},
		{"HIDE_QR_CODE", protocol.ActionHideQRCode, false},
		{"CANCEL_AI_GENERATION", protocol.ActionCancelAIGeneration, false},
		{"FULL", protocol.ActionFull, false},
		{"UPDATE", protocol.ActionUpdate, false},
		{"POINTS", protocol.ActionPoints, false},

		// MEMORY/MEMOTION — hors périmètre #155/#156 (contract note explicite),
		// SAUF FLIP_MEMORY_CARD depuis #159 (B1) et, depuis #160 (B1), les
		// cinq actions MEMOTION (Select/Flip/StopTimer/Reveal/Done) : l'anim
		// conduit désormais une manche MEMOTION de bout en bout depuis sa
		// tablette (sélection, retournement, arrêt du chrono, révélation,
		// crédit), exactement comme #159 l'a fait pour MEMORY — capability
		// widening délibéré, signalé au rapport dev-backend, pas noyé dans
		// le diff. MEMORY_SET_TEAMS et MEMOTION_SET_TEAMS restent hors
		// périmètre (admin uniquement), non touchés par #159/#160.
		{"MEMORY_SET_TEAMS", protocol.ActionMemorySetTeams, false},
		{"FLIP_MEMORY_CARD", protocol.ActionFlipMemoryCard, true}, // #159/B1 — était `false` avant ce lot
		{"MEMOTION_FLIP", protocol.ActionMotionFlip, true},        // #160/B1 — était `false` avant ce lot
		{"MEMOTION_STOP_TIMER", protocol.ActionMotionStopTimer, true}, // #160/B1 — était `false` avant ce lot
		{"MEMOTION_REVEAL", protocol.ActionMotionReveal, true},    // #160/B1 — était `false` avant ce lot
		{"MEMOTION_DONE", protocol.ActionMotionDone, true},        // #160/B1 — était `false` avant ce lot
		{"MEMOTION_SET_TEAMS", protocol.ActionMotionSetTeams, false},
		{"MEMOTION_SELECT", protocol.ActionMotionSelect, true}, // #160/B1 — était `false` avant ce lot

		// Enrôlement VJoueur — hors périmètre animateur.
		{"PLAYER_CONNECT", protocol.ActionPlayerConnect, false},
		{"VPLAYER_QCM_ANSWER", protocol.ActionVPlayerQCMAnswer, false},
		{"ARDOISE_INPUT", protocol.ActionArdoiseInput, false},
		{"BUTTON", protocol.ActionButton, false},
		{"PONG", protocol.ActionPong, false},

		// SET_CLIENT_TYPE is never a static map entry, for any type.
		{"SET_CLIENT_TYPE not in the static map", protocol.ActionSetClientType, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsActionAllowed(tt.action, ClientTypeAnim); got != tt.want {
				t.Errorf("IsActionAllowed(%q, ClientTypeAnim) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

// TestIsActionAllowed_ConduiteActions_UnchangedForOtherTypes guards against a
// B3 implementation that (incorrectly) widens the "conduite en direct" entries
// to TV/VPlayer while adding ClientTypeAnim — the contract only adds a THIRD
// value to each of these entries' list, admin's and TV/VPlayer's existing
// (denied) behavior must stay exactly as #154 left it.
func TestIsActionAllowed_ConduiteActions_UnchangedForOtherTypes(t *testing.T) {
	conduiteActions := []string{
		protocol.ActionStart, protocol.ActionStop, protocol.ActionPause,
		protocol.ActionContinue, protocol.ActionReveal, protocol.ActionReady,
		protocol.ActionBumperPoints, protocol.ActionTeamPoints,
	}
	for _, action := range conduiteActions {
		if !IsActionAllowed(action, ClientTypeAdmin) {
			t.Errorf("IsActionAllowed(%q, ClientTypeAdmin) = false, want true (must remain admin-allowed)", action)
		}
		if IsActionAllowed(action, ClientTypeTV) {
			t.Errorf("IsActionAllowed(%q, ClientTypeTV) = true, want false (B3 must not accidentally open this to TV)", action)
		}
		if IsActionAllowed(action, ClientTypeVPlayer) {
			t.Errorf("IsActionAllowed(%q, ClientTypeVPlayer) = true, want false (B3 must not accidentally open this to VPlayer)", action)
		}
	}
}

// TestIsSetClientTypeAllowed_Anim is #155's application of #154 E3's closed
// escalation path: a client connected on the dedicated /ws/anim endpoint
// (fixed type at connection time, like /ws/tv and /ws/player) has no
// legitimate reason to send SET_CLIENT_TYPE — IsSetClientTypeAllowed only
// permits it from the Admin state, which the plan states is "already
// satisfied by construction" (rev.1 §0.3) but flags for a dedicated test
// rather than relying on re-reading alone.
func TestIsSetClientTypeAllowed_Anim(t *testing.T) {
	if IsSetClientTypeAllowed(ClientTypeAnim) {
		t.Error("IsSetClientTypeAllowed(ClientTypeAnim) = true, want false — an anim-typed connection must not be able to self-declare a different type")
	}
}
