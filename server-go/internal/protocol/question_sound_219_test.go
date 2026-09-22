// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9, contract
// contracts/websocket-actions.md §"QUESTION_SOUND" et
// contracts/ws-payload-serialization.md) : la diffusion de
// GAME.QUESTION_SOUND_STATE/ANSWER_TIMER_WAITING (tableau — Admin✅ / TV+
// VPlayer✅ / Buzzer❌) et le payload ActionQuestionSound lui-même.
//
// Réutilise gameNodeOf/parseMsgMap/NewMessage déjà déclarés dans ce paquet
// (messages_quiz_objectives_test.go / messages_test.go) — même package,
// même discipline que ces suites pour QUIZ_OBJECTIVES/QUIZ_HIDDEN_FIELDS.
//
// Convention de collision : préfixe tw219p pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package protocol

import (
	"encoding/json"
	"testing"
)

const tw219pTestPlayerID = "vjoueur-question-sound"

// tw219pBuildUpdateMsg builds a realistic UPDATE message whose GAME node
// carries QUESTION_SOUND_STATE/ANSWER_TIMER_WAITING alongside a sibling
// field (PHASE) that's known to survive everywhere — same shape convention
// as buildQuizMetaUpdateMsg, so a test can tell "these two fields
// specifically stripped" apart from "the whole GAME node vanished".
func tw219pBuildUpdateMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":                phase,
			"QUESTION_SOUND_STATE": "PLAYING",
			"ANSWER_TIMER_WAITING": true,
			"CURRENT_TIME":         20,
		},
		"bumpers": map[string]interface{}{
			tw219pTestPlayerID: map[string]interface{}{
				"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true, "IS_VIRTUAL": true, "IS_VPLAYER": true,
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("tw219pBuildUpdateMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("tw219pBuildUpdateMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

// ---------------------------------------------------------------------------
// SerializeForAdmin — both fields present (contract table: Admin✅).
// ---------------------------------------------------------------------------

func TestSerializeForAdmin_QuestionSoundFieldsPresent(t *testing.T) {
	msg := tw219pBuildUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if game["QUESTION_SOUND_STATE"] != "PLAYING" {
		t.Errorf("SerializeForAdmin: QUESTION_SOUND_STATE should be present and unchanged, got %v", game["QUESTION_SOUND_STATE"])
	}
	if game["ANSWER_TIMER_WAITING"] != true {
		t.Errorf("SerializeForAdmin: ANSWER_TIMER_WAITING should be present and unchanged, got %v", game["ANSWER_TIMER_WAITING"])
	}
}

// ---------------------------------------------------------------------------
// SerializeForWebClient (TV + /anim fallback) — both fields present
// (contract table: TV✅).
// ---------------------------------------------------------------------------

func TestSerializeForWebClient_QuestionSoundFieldsPresent(t *testing.T) {
	msg := tw219pBuildUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if game["QUESTION_SOUND_STATE"] != "PLAYING" {
		t.Errorf("SerializeForWebClient: QUESTION_SOUND_STATE should reach TV/anim, got %v", game["QUESTION_SOUND_STATE"])
	}
	if game["ANSWER_TIMER_WAITING"] != true {
		t.Errorf("SerializeForWebClient: ANSWER_TIMER_WAITING should reach TV/anim (CA15 depends on it), got %v", game["ANSWER_TIMER_WAITING"])
	}
}

// ---------------------------------------------------------------------------
// SerializeForVPlayer — both fields present in every phase branch (contract
// table: VPlayer✅) — same three phases as the QUIZ_HIDDEN_FIELDS precedent,
// since the reduced PREPARE/READY branch parses GAME independently and could
// regress on its own without the non-reduced branch noticing.
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_QuestionSoundFieldsPresent(t *testing.T) {
	tests := []struct {
		name  string
		phase string
	}{
		{"non-reduced phase (falls back to SerializeForWebClient)", "STARTED"},
		{"reduced phase PREPARE (own GAME-filtering branch)", "PREPARE"},
		{"reduced phase READY (own GAME-filtering branch)", "READY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tw219pBuildUpdateMsg(t, tt.phase)

			data, err := msg.SerializeForVPlayer(tw219pTestPlayerID)
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}

			game := gameNodeOf(t, data)
			if game["QUESTION_SOUND_STATE"] != "PLAYING" {
				t.Errorf("SerializeForVPlayer(phase=%s): QUESTION_SOUND_STATE should reach VPlayer, got %v", tt.phase, game["QUESTION_SOUND_STATE"])
			}
			if game["ANSWER_TIMER_WAITING"] != true {
				t.Errorf("SerializeForVPlayer(phase=%s): ANSWER_TIMER_WAITING should reach VPlayer, got %v", tt.phase, game["ANSWER_TIMER_WAITING"])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SerializeForBuzzer — both fields ABSENT (contract table: Buzzer❌).
// SerializeForBuzzer builds an ALLOW-list (PHASE/TIME/CURRENT_TIME only,
// copied to the TOP LEVEL of the minimal payload, never nested under
// "GAME") — so the assertion here checks the top-level minimal map, not a
// "GAME" node (which SerializeForBuzzer's output doesn't even have).
// ---------------------------------------------------------------------------

func TestSerializeForBuzzer_QuestionSoundFieldsAbsent(t *testing.T) {
	msg := tw219pBuildUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer failed: %v", err)
	}

	minimal := parseMsgMap(t, data)
	if _, present := minimal["QUESTION_SOUND_STATE"]; present {
		t.Errorf("SerializeForBuzzer: QUESTION_SOUND_STATE must never reach a physical buzzer (contract table), got %v", minimal["QUESTION_SOUND_STATE"])
	}
	if _, present := minimal["ANSWER_TIMER_WAITING"]; present {
		t.Errorf("SerializeForBuzzer: ANSWER_TIMER_WAITING must never reach a physical buzzer (contract table), got %v", minimal["ANSWER_TIMER_WAITING"])
	}
	// Sanity: the buzzer allow-list itself isn't broken by this test's own
	// payload shape — CURRENT_TIME must still survive, proving this isn't a
	// coincidental "everything stripped" pass.
	if minimal["CURRENT_TIME"] == nil {
		t.Error("SerializeForBuzzer: setup sanity failed — CURRENT_TIME should survive on the allow-list, got nil (test payload shape problem, not a real regression)")
	}
}

// ---------------------------------------------------------------------------
// ActionQuestionSound / QuestionSoundPayload — wire shape (contract
// websocket-actions.md §"QUESTION_SOUND").
// ---------------------------------------------------------------------------

func TestActionQuestionSound_WireValue(t *testing.T) {
	if ActionQuestionSound != "QUESTION_SOUND" {
		t.Errorf("ActionQuestionSound = %q, want \"QUESTION_SOUND\" (contract websocket-actions.md)", ActionQuestionSound)
	}
}

func TestQuestionSoundCommand_WireValues(t *testing.T) {
	tests := []struct {
		got  QuestionSoundCommand
		want string
	}{
		{QuestionSoundCommandPlay, "PLAY"},
		{QuestionSoundCommandPause, "PAUSE"},
		{QuestionSoundCommandResume, "RESUME"},
		{QuestionSoundCommandStop, "STOP"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("QuestionSoundCommand = %q, want %q (contract §10.2/§10.7)", tt.got, tt.want)
		}
	}
}

// TestQuestionSoundPayload_RoundTrip proves the {"COMMAND": "..."} wire
// shape round-trips exactly as contracts/websocket-actions.md documents —
// the field the frontend's questionSound(command) (useWebSocket.js) emits
// and handleQuestionSound (cmd/server/question_sound.go) decodes.
func TestQuestionSoundPayload_RoundTrip(t *testing.T) {
	raw := []byte(`{"COMMAND":"PAUSE"}`)
	var payload QuestionSoundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if payload.Command != QuestionSoundCommandPause {
		t.Errorf("unmarshal: Command = %q, want PAUSE", payload.Command)
	}

	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var roundtripped map[string]interface{}
	if err := json.Unmarshal(out, &roundtripped); err != nil {
		t.Fatalf("re-unmarshal failed: %v", err)
	}
	if roundtripped["COMMAND"] != "PAUSE" {
		t.Errorf("round-trip: COMMAND = %v, want \"PAUSE\"", roundtripped["COMMAND"])
	}
}
