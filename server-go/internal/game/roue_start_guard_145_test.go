// #145 (v12.0.0, LOT-1.1) — garde PROVISOIRE de départ (plan tâche 5) : tant que le moteur ROUE n'existe pas
// (LOT-1.2, #144), une question ROUE ne peut pas démarrer. ⚠️ Ce fichier est SUPPRIMÉ par la tâche 18
// (tests LOT-1.2) quand la garde est remplacée par la vraie règle (équipes conformes au mode + phrase non vide).
package game

import "testing"

func TestRoueStartGuard145_RoueQuestionNeverConform(t *testing.T) {
	q := &Question{ID: "r1", Type: QuestionTypeRoue, TypedContent: TypedContent{RouePhrase: "Bonjour"}}
	state := &GameState{}
	if participantsConform(q, state) {
		t.Fatal("une question ROUE ne doit pas pouvoir atteindre READY avant LOT-1.2")
	}
}

// Non-régression : la garde ne touche aucun autre type.
func TestRoueStartGuard145_OtherTypesUnaffected(t *testing.T) {
	for _, qt := range []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeArdoise, QuestionTypeEntracte} {
		q := &Question{ID: "x", Type: qt}
		if !participantsConform(q, &GameState{}) {
			t.Errorf("le type %s doit rester conforme (comportement inchangé)", qt)
		}
	}
}
