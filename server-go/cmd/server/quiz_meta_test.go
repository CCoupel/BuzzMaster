package main

import (
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: UPDATE_QUIZ_META dispatch — "absent = unchanged" semantics for the
// fields added in v6.0.0 (#8) and extended in v6.1.0 (#137 Batch 2b):
// POPULATIONS, DIFFICULTIES, LANGUAGE, OBJECTIVES.
//
// Contract: contracts/ai-generation.md §7 — a client sending only a subset
// of the form must not wipe the rest, because QuizMetaPayload decodes these
// as pointers (nil = absent from the JSON body, non-nil "" or [] =
// explicitly present and empty). This is the resolution logic in
// cmd/server/main.go's ActionUpdateQuizMeta case, one layer above
// engine.SetQuizMeta itself (already covered by
// internal/game/engine_test.go's TestSetQuizMeta*).
// ---------------------------------------------------------------------------

// sendQuizMeta builds an UPDATE_QUIZ_META IncomingMessage from a raw JSON
// MSG body (so tests can omit fields entirely, which json.Marshal of a Go
// struct cannot easily express) and dispatches it through handleWebMessage.
func sendQuizMeta(t *testing.T, app *App, rawMsg string) {
	t.Helper()
	msg := &protocol.Message{
		Action: protocol.ActionUpdateQuizMeta,
		Msg:    json.RawMessage(rawMsg),
	}
	app.handleWebMessage(&protocol.IncomingMessage{ClientID: "test-admin", ClientType: "admin", Data: msg})
}

func TestUpdateQuizMeta_AbsentFieldsLeaveNewFieldsUnchanged(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	// Seed the fields via a full payload first.
	sendQuizMeta(t, app, `{"NAME":"Quiz v1","THEME":"Cinéma","NOTES":"","POPULATIONS":["Adulte (18-64 ans)"],"DIFFICULTIES":["Moyen"],"LANGUAGE":"Français","OBJECTIVES":"Objectif v1"}`)

	state := app.engine.GetState()
	if !reflect.DeepEqual(state.QuizPopulations, []string{"Adulte (18-64 ans)"}) || !reflect.DeepEqual(state.QuizDifficulties, []string{"Moyen"}) || state.QuizLanguage != "Français" || state.QuizObjectives != "Objectif v1" {
		t.Fatalf("precondition failed, got %+v", state)
	}

	// An older client (or a save that only touches NAME/THEME/NOTES) omits
	// the additive fields entirely — they must survive untouched.
	sendQuizMeta(t, app, `{"NAME":"Quiz v2","THEME":"Cinéma","NOTES":"Nouvelle note"}`)

	state = app.engine.GetState()
	if state.QuizName != "Quiz v2" {
		t.Errorf("QuizName should update to 'Quiz v2', got %q", state.QuizName)
	}
	if state.QuizNotes != "Nouvelle note" {
		t.Errorf("QuizNotes should update to 'Nouvelle note', got %q", state.QuizNotes)
	}
	if !reflect.DeepEqual(state.QuizPopulations, []string{"Adulte (18-64 ans)"}) {
		t.Errorf("QuizPopulations should be preserved (absent from payload), got %v", state.QuizPopulations)
	}
	if !reflect.DeepEqual(state.QuizDifficulties, []string{"Moyen"}) {
		t.Errorf("QuizDifficulties should be preserved (absent from payload), got %v", state.QuizDifficulties)
	}
	if state.QuizLanguage != "Français" {
		t.Errorf("QuizLanguage should be preserved (absent from payload), got %q", state.QuizLanguage)
	}
	if state.QuizObjectives != "Objectif v1" {
		t.Errorf("QuizObjectives should be preserved (absent from payload), got %q", state.QuizObjectives)
	}
}

func TestUpdateQuizMeta_ExplicitEmptyClearsField(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Histoire","NOTES":"","POPULATIONS":["Senior (65+ ans)"],"DIFFICULTIES":["Expert"],"LANGUAGE":"Anglais","OBJECTIVES":"Objectif initial"}`)

	// A field present with an explicit empty value must clear it — distinct
	// from the field being absent entirely (previous test).
	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Histoire","NOTES":"","POPULATIONS":[],"DIFFICULTIES":["Expert"],"LANGUAGE":"Anglais","OBJECTIVES":""}`)

	state := app.engine.GetState()
	if !reflect.DeepEqual(state.QuizPopulations, []string{}) {
		t.Errorf("QuizPopulations should be explicitly cleared, got %v", state.QuizPopulations)
	}
	if !reflect.DeepEqual(state.QuizDifficulties, []string{"Expert"}) {
		t.Errorf("QuizDifficulties should be preserved, got %v", state.QuizDifficulties)
	}
	if state.QuizObjectives != "" {
		t.Errorf("QuizObjectives should be explicitly cleared, got %q", state.QuizObjectives)
	}
}

// ---------------------------------------------------------------------------
// Tests: UPDATE_QUIZ_META.HIDDEN_FIELDS dispatch (v6.1.0, #137 Batch 2b T1.8,
// contract game-state.md §"QUIZ_HIDDEN_FIELDS"). Routed through the dedicated
// Engine.SetQuizDisplay setter, not SetQuizMeta — same absent = unchanged
// semantics, but only when the key was present at all (nil pointer = don't
// call SetQuizDisplay, so an omitted key never resets it even to its own
// current value).
// ---------------------------------------------------------------------------

func TestUpdateQuizMeta_HiddenFieldsAbsentLeavesUnchanged(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Cinéma","NOTES":"","HIDDEN_FIELDS":["DIFFICULTIES","THEME"]}`)

	state := app.engine.GetState()
	if !reflect.DeepEqual(state.QuizHiddenFields, []string{"DIFFICULTIES", "THEME"}) {
		t.Fatalf("precondition failed, got %v", state.QuizHiddenFields)
	}

	// A save that omits HIDDEN_FIELDS entirely must not reset it.
	sendQuizMeta(t, app, `{"NAME":"Quiz v2","THEME":"Cinéma","NOTES":""}`)

	state = app.engine.GetState()
	if !reflect.DeepEqual(state.QuizHiddenFields, []string{"DIFFICULTIES", "THEME"}) {
		t.Errorf("QuizHiddenFields should be preserved (absent from payload), got %v", state.QuizHiddenFields)
	}
}

func TestUpdateQuizMeta_HiddenFieldsExplicitEmptyClearsField(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Histoire","NOTES":"","HIDDEN_FIELDS":["LANGUAGE"]}`)
	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Histoire","NOTES":"","HIDDEN_FIELDS":[]}`)

	state := app.engine.GetState()
	if state.QuizHiddenFields == nil || len(state.QuizHiddenFields) != 0 {
		t.Errorf("QuizHiddenFields should be explicitly cleared to [], got %v", state.QuizHiddenFields)
	}
}

func TestUpdateQuizMeta_HiddenFieldsUnknownValueIgnored(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	sendQuizMeta(t, app, `{"NAME":"Quiz","THEME":"Histoire","NOTES":"","HIDDEN_FIELDS":["THEME","NOT_A_REAL_FIELD"]}`)

	state := app.engine.GetState()
	if !reflect.DeepEqual(state.QuizHiddenFields, []string{"THEME"}) {
		t.Errorf("QuizHiddenFields should drop the unknown value, got %v", state.QuizHiddenFields)
	}
}
