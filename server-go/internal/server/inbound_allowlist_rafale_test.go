package server

import (
	"buzzcontrol/internal/protocol"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : allow-list entrante des 3 actions client→serveur du mode RAFALE
// (milestone v8.0.0 #16, issue #107, contrat contracts/rafale.md §5.1) —
// patron inbound_allowlist_test.go (#154).
//
// ⚠️ Écrit en Batch 1 (test-writer), à partir du contrat SEUL — les
// constantes protocol.ActionRafaleValidate / ActionRafaleInvalidate /
// ActionRafaleSetTeams et leurs entrées dans inboundActionAllowlist
// n'existent pas encore au moment où ce fichier est écrit : elles sont
// prévues pour la tâche 18/19 du plan (#107, Batch 2, dev-backend). Ce
// fichier NE COMPILERA PAS tant que ces symboles ne sont pas ajoutés — c'est
// attendu (TDD contract-first, cf. consigne CDP) : dev-backend implémente
// pour faire passer cette suite, il ne l'écrit pas lui-même.
//
// Périmètre fixé par le contrat §5.1 :
//   RAFALE_VALIDATE   {} : admin, anim  — réponse jugée correcte
//   RAFALE_INVALIDATE {} : admin, anim  — réponse jugée incorrecte
//   RAFALE_SET_TEAMS  {TEAMS:[...]} : admin uniquement — équipes + ordre
//
// Allow-list FERMÉE (#154) : toute action absente du map, ou un ClientType
// absent de l'entrée, est rejetée par défaut — donc chaque test positif a
// son miroir négatif pour les types NON listés, exactement comme le reste
// de inbound_allowlist_test.go.
// ---------------------------------------------------------------------------

func TestIsActionAllowed_Rafale(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		clientType ClientType
		want       bool
	}{
		// --- RAFALE_VALIDATE — admin + anim, comme la "conduite en direct"
		// (RAFALE_VALIDATE/INVALIDATE jugent une réponse, au même titre que
		// REVEAL/BUMPER_POINTS/TEAM_POINTS déjà admin+anim dans ce fichier).
		{"RAFALE_VALIDATE from admin", protocol.ActionRafaleValidate, ClientTypeAdmin, true},
		{"RAFALE_VALIDATE from anim", protocol.ActionRafaleValidate, ClientTypeAnim, true},
		{"RAFALE_VALIDATE from tv", protocol.ActionRafaleValidate, ClientTypeTV, false},
		{"RAFALE_VALIDATE from vplayer", protocol.ActionRafaleValidate, ClientTypeVPlayer, false},
		{"RAFALE_VALIDATE from buzzer", protocol.ActionRafaleValidate, ClientTypeBuzzer, false},

		// --- RAFALE_INVALIDATE — même périmètre que RAFALE_VALIDATE.
		{"RAFALE_INVALIDATE from admin", protocol.ActionRafaleInvalidate, ClientTypeAdmin, true},
		{"RAFALE_INVALIDATE from anim", protocol.ActionRafaleInvalidate, ClientTypeAnim, true},
		{"RAFALE_INVALIDATE from tv", protocol.ActionRafaleInvalidate, ClientTypeTV, false},
		{"RAFALE_INVALIDATE from vplayer", protocol.ActionRafaleInvalidate, ClientTypeVPlayer, false},

		// --- RAFALE_SET_TEAMS — admin SEUL (§5.1 : "équipes participantes +
		// ordre de passage"), exactement le même périmètre que
		// MEMORY_SET_TEAMS/MEMOTION_SET_TEAMS (choix des équipes = régie,
		// jamais anim — contrat rafale.md, précédent #159/#160).
		{"RAFALE_SET_TEAMS from admin", protocol.ActionRafaleSetTeams, ClientTypeAdmin, true},
		{"RAFALE_SET_TEAMS from anim", protocol.ActionRafaleSetTeams, ClientTypeAnim, false},
		{"RAFALE_SET_TEAMS from tv", protocol.ActionRafaleSetTeams, ClientTypeTV, false},
		{"RAFALE_SET_TEAMS from vplayer", protocol.ActionRafaleSetTeams, ClientTypeVPlayer, false},

		// --- Default-deny : ClientType vide/inconnu, même pour une action
		// par ailleurs autorisée à quelqu'un — même discipline que
		// TestIsActionAllowed (#154) pour les actions existantes.
		{"RAFALE_VALIDATE from empty clientType", protocol.ActionRafaleValidate, "", false},
		{"RAFALE_SET_TEAMS from unrecognized clientType", protocol.ActionRafaleSetTeams, ClientTypeBuzzer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsActionAllowed(tt.action, tt.clientType); got != tt.want {
				t.Errorf("IsActionAllowed(%q, %q) = %v, want %v", tt.action, tt.clientType, got, tt.want)
			}
		})
	}
}

// TestIsActionAllowed_Rafale_OutboundActionsAreNotInboundEntries is a
// regression guard against a specific, easy mistake: RAFALE_ANSWER and
// RAFALE_TICK are SERVER→CLIENT actions (contract §5.2, delivered via
// BroadcastToTypes — see rafale_leak_test.go), never sent BY a web client.
// They must NOT appear in inboundActionAllowlist at all — if they did, a
// malicious/buggy client could "send" a server-only action and have it
// silently accepted by the (would-be) dispatch switch, defeating the
// closed-allow-list discipline (#154) for an action that was never meant to
// have a client→server direction in the first place.
func TestIsActionAllowed_Rafale_OutboundActionsAreNotInboundEntries(t *testing.T) {
	for _, action := range []string{protocol.ActionRafaleAnswer, protocol.ActionRafaleTick} {
		for _, ct := range []ClientType{ClientTypeAdmin, ClientTypeAnim, ClientTypeTV, ClientTypeVPlayer} {
			if IsActionAllowed(action, ct) {
				t.Errorf("%s (server→client only, contract §5.2) must not be an accepted INBOUND action from %q", action, ct)
			}
		}
	}
}
