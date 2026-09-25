// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9, contract models.md et
// game-state.md) : sérialisation de Question.Sound/SoundTimerDelayed
// (omitempty — additif, un question.json existant reste inchangé octet
// pour octet) et de GameState.QuestionSoundState/AnswerTimerWaiting
// (JAMAIS omitempty, règle projet CLAUDE.md).
//
// Convention de collision : préfixe tw219g pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package game

import (
	"encoding/json"
	"testing"
)

// TestQuestion_Sound_OmittedWhenEmpty proves an existing question.json with
// no SOUND/SOUND_TIMER_DELAYED round-trips byte-for-byte unaffected by
// #219's new fields — omitempty on both (contract models.md, plan §0.3:
// "additif, un question.json existant round-trippe byte pour byte").
func TestQuestion_Sound_OmittedWhenEmpty(t *testing.T) {
	q := Question{ID: "1", Question: "Q", Answer: "A", Points: "1", Time: "20"}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, present := raw["SOUND"]; present {
		t.Errorf("SOUND must be omitted (omitempty) on a question with no sound, got %v", raw["SOUND"])
	}
	if _, present := raw["SOUND_TIMER_DELAYED"]; present {
		t.Errorf("SOUND_TIMER_DELAYED must be omitted (omitempty, valeur zéro = comportement correct) when false, got %v", raw["SOUND_TIMER_DELAYED"])
	}
}

// TestQuestion_Sound_RoundTripsWhenPresent proves the inverse: a question
// that DOES carry a sound preserves both fields exactly across a
// marshal/unmarshal cycle — the URL path convention (contract models.md:
// "/question/<id>/sound_<rand4>.wav") and the boolean flag.
func TestQuestion_Sound_RoundTripsWhenPresent(t *testing.T) {
	q := Question{
		ID: "1", Question: "Q", Answer: "A", Points: "1", Time: "20",
		Sound: "/question/1/sound_7314.wav", SoundTimerDelayed: true,
	}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var got Question
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.Sound != q.Sound {
		t.Errorf("SOUND round-trip mismatch: got %q, want %q", got.Sound, q.Sound)
	}
	if got.SoundTimerDelayed != true {
		t.Error("SOUND_TIMER_DELAYED round-trip mismatch: got false, want true")
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if raw["SOUND"] != q.Sound {
		t.Errorf(`wire key "SOUND" mismatch: got %v`, raw["SOUND"])
	}
	if raw["SOUND_TIMER_DELAYED"] != true {
		t.Errorf(`wire key "SOUND_TIMER_DELAYED" mismatch: got %v`, raw["SOUND_TIMER_DELAYED"])
	}
}

// TestQuestion_Sound_CommonToEveryType is the executable form of §0.3's
// normative rule: Question.Sound/SoundTimerDelayed live OUTSIDE TypedContent
// and are NEVER dropped for any QuestionType — the server does not gate this
// field by type (that restriction is the v11.1 editor's own choice, never a
// server-side guard). A regression here — someone "helpfully" moving Sound
// into TypedContent, or adding a type check to its marshaling — would make
// re-opening MEMORY/MEMOTION/ENTRACTE for sound a backend change instead of
// the purely-frontend one the contract promises.
func TestQuestion_Sound_CommonToEveryType(t *testing.T) {
	for _, qt := range []QuestionType{
		QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeArdoise,
		QuestionTypeMemory, QuestionTypeMemotion, QuestionTypeEntracte, QuestionTypeRafale,
	} {
		t.Run(string(qt), func(t *testing.T) {
			q := Question{ID: "1", Type: qt, Sound: "/question/1/sound_1234.wav", SoundTimerDelayed: true}
			data, err := json.Marshal(q)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var raw map[string]interface{}
			json.Unmarshal(data, &raw)
			if raw["SOUND"] != q.Sound {
				t.Errorf("type %s: SOUND must serialize regardless of type (§0.3 — structurally common field), got %v", qt, raw["SOUND"])
			}
			if raw["SOUND_TIMER_DELAYED"] != true {
				t.Errorf("type %s: SOUND_TIMER_DELAYED must serialize regardless of type, got %v", qt, raw["SOUND_TIMER_DELAYED"])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GameState — JAMAIS omitempty (CLAUDE.md: "évite réinitialisations
// manquées côté frontend").
// ---------------------------------------------------------------------------

// TestGameState_QuestionSoundFields_NeverOmitted proves both new GameState
// fields are ALWAYS serialized, even at their Go zero value — the opposite
// discipline from Question.Sound above, and the one the project's own
// CLAUDE.md mandates project-wide ("No omitempty on GameState fields").
func TestGameState_QuestionSoundFields_NeverOmitted(t *testing.T) {
	gs := GameState{} // zero value across the board
	data, err := json.Marshal(gs)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, present := raw["QUESTION_SOUND_STATE"]; !present {
		t.Error("QUESTION_SOUND_STATE must always be serialized, even at its zero value (no omitempty, CLAUDE.md)")
	}
	if _, present := raw["ANSWER_TIMER_WAITING"]; !present {
		t.Error("ANSWER_TIMER_WAITING must always be serialized, even when false (no omitempty, CLAUDE.md)")
	}
	if raw["ANSWER_TIMER_WAITING"] != false {
		t.Errorf("ANSWER_TIMER_WAITING zero value should serialize as false, got %v", raw["ANSWER_TIMER_WAITING"])
	}
}

// TestNewEngine_InitializesQuestionSoundStateToIdle proves models.go's own
// requirement is actually honoured at construction: "NewEngine()
// initialise QuestionSoundState: QuestionSoundIdle (jamais la valeur zéro
// Go "")" — the zero value of the underlying string type would otherwise
// serialize as "QUESTION_SOUND_STATE":"" on a freshly-built engine, before
// any question-sound adapter call ever sets it explicitly.
func TestNewEngine_InitializesQuestionSoundStateToIdle(t *testing.T) {
	e := NewEngine()
	state := e.GetState()
	if state.QuestionSoundState != QuestionSoundIdle {
		t.Errorf("a freshly-built Engine must report QUESTION_SOUND_STATE=IDLE, got %q", state.QuestionSoundState)
	}
}
