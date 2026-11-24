package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

// #191 (v13.0.0) — PICTO_WORDS never reaches /ws/tv nor the tablets (contracts/picto.md §10), any
// phase; /ws/admin and /ws/anim keep it. Model: roue_leak_test.go.

func buildPictoLeakMsg(t *testing.T, action, phase string) *Message {
	t.Helper()
	q := game.Question{
		ID: "p1", Question: "Dessinez", Type: game.QuestionTypePicto, Category: "ANIMALS", Time: "90",
		TypedContent: game.TypedContent{PictoWords: []string{"Éléphant", "Pont-levis"}},
	}
	raw, err := json.Marshal(map[string]interface{}{
		"GAME":    map[string]interface{}{"PHASE": phase, "QUESTION": q},
		"bumpers": map[string]interface{}{"p-1": map[string]interface{}{"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true}},
		"teams":   map[string]interface{}{"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := NewMessage(action, nil)
	if err != nil {
		t.Fatal(err)
	}
	msg.Msg = raw
	return msg
}

func assertNoPictoWords(t *testing.T, who string, data []byte) {
	t.Helper()
	s := string(data)
	for _, frag := range []string{"PICTO_WORDS", "Éléphant", "Pont-levis"} {
		if strings.Contains(s, frag) {
			t.Errorf("%s : %q ne doit jamais être envoyé (contrat §10) : %s", who, frag, s)
		}
	}
}

func TestPictoLeak191_TVAndTablets_WordsAbsent_AllActionsAndPhases(t *testing.T) {
	for _, action := range roueLeakActions {
		for _, phase := range roueLeakPhases {
			t.Run(action+"/"+phase, func(t *testing.T) {
				tv, err := buildPictoLeakMsg(t, action, phase).SerializeForTV()
				if err != nil {
					t.Fatal(err)
				}
				assertNoPictoWords(t, "TV", tv)
				if q := questionNodeOf(t, tv); q["TYPE"] != "PICTO" || q["TIME"] != "90" {
					t.Errorf("le reste de la question doit survivre sur la TV, got %v", q)
				}
				vc, err := buildPictoLeakMsg(t, action, phase).SerializeForVPlayerCommon()
				if err != nil {
					t.Fatal(err)
				}
				assertNoPictoWords(t, "VPlayer", vc)
			})
		}
	}
}

func TestPictoLeak191_VPlayerIdentified_WordsAbsent_AllPhases(t *testing.T) {
	for _, phase := range roueLeakPhases {
		data, err := buildPictoLeakMsg(t, ActionUpdate, phase).SerializeForVPlayer("p-1")
		if err != nil {
			t.Fatal(err)
		}
		assertNoPictoWords(t, "VPlayer(p-1)/"+phase, data)
	}
}

func TestPictoLeak191_AdminAndAnim_KeepWords(t *testing.T) {
	adm, err := buildPictoLeakMsg(t, ActionUpdate, "REVEALED").SerializeForAdmin()
	if err != nil {
		t.Fatal(err)
	}
	anim, err := buildPictoLeakMsg(t, ActionUpdate, "REVEALED").SerializeForWebClient()
	if err != nil {
		t.Fatal(err)
	}
	for who, data := range map[string][]byte{"admin": adm, "anim": anim} {
		if w, _ := questionNodeOf(t, data)["PICTO_WORDS"].([]interface{}); len(w) != 2 {
			t.Errorf("%s doit recevoir PICTO_WORDS, got %v", who, w)
		}
	}
}
