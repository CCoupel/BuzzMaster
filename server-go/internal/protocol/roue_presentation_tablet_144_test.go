package protocol

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/game"
)

// #144 (v12.0.0) — la tablette affiche l'indice de la manche ROUE : QUESTION.ROUE_PRESENTATION et CATEGORY
// restent diffusés à la TV et à la tablette (publics, contracts/roue.md §2 et §9) ; seule la phrase est retirée.
func TestRoueLeak144_PresentationAndCategoryReachTVAndTablet(t *testing.T) {
	const presentation = "Chanson de Line Renaud"
	q := game.Question{
		ID: "r1", Question: "Manche roue", Type: game.QuestionTypeRoue, Category: "ANIMALS",
		TypedContent: game.TypedContent{RouePhrase: roueSecretPhrase, RoucePresentation: presentation},
	}
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE": "STARTED", "QUESTION": q, "ROUE_SUBPHASE": "SPINNING", "ROUE_CURRENT_TEAM": "TeamA",
		},
		"bumpers": map[string]interface{}{"p-1": map[string]interface{}{"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true}},
		"teams":   map[string]interface{}{"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	m, _ := NewMessage(ActionUpdate, nil)
	m.Msg = raw
	for name, ser := range map[string]func() ([]byte, error){
		"TV":            m.SerializeForTV,
		"VPlayerCommon": m.SerializeForVPlayerCommon,
		"VPlayer(p-1)":  func() ([]byte, error) { return m.SerializeForVPlayer("p-1") },
	} {
		data, err := ser()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertNoPhrase(t, name, data)
		g := gameNodeOf(t, data)
		qn, ok := g["QUESTION"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s : QUESTION absent", name)
		}
		if qn["ROUE_PRESENTATION"] != presentation {
			t.Errorf("%s : ROUE_PRESENTATION doit rester diffusé (indice public), obtenu %v", name, qn["ROUE_PRESENTATION"])
		}
		if qn["CATEGORY"] != "ANIMALS" {
			t.Errorf("%s : CATEGORY doit rester diffusée, obtenu %v", name, qn["CATEGORY"])
		}
	}
}
