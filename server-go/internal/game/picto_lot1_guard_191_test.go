package game

import "testing"

// LOT-1 (#191): until the PICTO round exists (LOT-2.1), a PICTO question never conforms,
// whatever the state — it stays in PREPARE and START is refused.
func TestPictoLot1_ParticipantsNeverConform(t *testing.T) {
	q := &Question{ID: "p", Type: QuestionTypePicto, Time: "90", TypedContent: TypedContent{PictoWords: []string{"chat"}}}
	if participantsConform(q, &GameState{}) || participantsConform(q, &GameState{RoueTeams: []string{"red"}}) {
		t.Error("une question PICTO ne doit pas être conforme en LOT-1")
	}
}
