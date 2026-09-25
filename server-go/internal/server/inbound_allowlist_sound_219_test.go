// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9, contract sound.md §10.2) :
// QUESTION_SOUND accepté pour admin/anim, refusé pour tv/vplayer/buzzer —
// même "conduite en direct" périmètre que REVEAL/RAFALE_VALIDATE
// (inbound_allowlist.go).
//
// Fichier additionnel, à côté de inbound_allowlist_test.go — ne modifie pas
// sa table existante (règle non-régression du projet).
//
// Convention de collision : préfixe tw219a pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package server

import (
	"buzzcontrol/internal/protocol"
	"testing"
)

func TestIsActionAllowed_QuestionSound(t *testing.T) {
	tests := []struct {
		name       string
		clientType ClientType
		want       bool
	}{
		{"QUESTION_SOUND from admin", ClientTypeAdmin, true},
		{"QUESTION_SOUND from anim", ClientTypeAnim, true},
		{"QUESTION_SOUND from tv", ClientTypeTV, false},
		{"QUESTION_SOUND from vplayer", ClientTypeVPlayer, false},
		{"QUESTION_SOUND from buzzer", ClientTypeBuzzer, false},
		{"QUESTION_SOUND from empty clientType", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsActionAllowed(protocol.ActionQuestionSound, tt.clientType)
			if got != tt.want {
				t.Errorf("IsActionAllowed(QUESTION_SOUND, %q) = %v, want %v (contract sound.md §10.2, même périmètre que REVEAL/RAFALE_VALIDATE)", tt.clientType, got, tt.want)
			}
		})
	}
}

// TestIsActionAllowedDuringEntracte_QuestionSoundRefused re-verifies the
// fix dev-backend applied to entracte_allowlist_test.go's own exhaustiveness
// gate (handoff dev-backend-20260922-122419.md): QUESTION_SOUND is refused
// while GameState.ENTRACTE is true — same compartment as REVEAL/
// RAFALE_VALIDATE, which the entracte allow-list (entracteAllowedActions)
// also excludes.
func TestIsActionAllowedDuringEntracte_QuestionSoundRefused(t *testing.T) {
	if IsActionAllowedDuringEntracte(protocol.ActionQuestionSound) {
		t.Error("QUESTION_SOUND doit être refusé pendant l'entracte (D6, même compartiment que REVEAL/RAFALE_VALIDATE) — une question programmée en pause ne doit pas laisser conduire un son de question sous-jacent")
	}
}
