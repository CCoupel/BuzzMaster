package game

// Tests for #141 (plan tasks 18-20, 23): SaveState/LoadState/ClearQuizMeta,
// the first versioned on-disk format in this project, and the H5 rule
// (QUIZ_HIDDEN_FIELDS and siblings survive InitGame/NEW_GAME — now also a
// restart).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestSaveState_LoadState_Roundtrip verifies the persisted subset survives a
// save+load cycle on a fresh Engine (simulating a restart).
func TestSaveState_LoadState_Roundtrip(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "game_state.json")

	e1 := NewEngine()
	e1.SetStatePath(statePath)
	e1.SetQuizMeta("Mon Quiz", "Sciences", "Notes", []string{"Adulte (18-64 ans)"}, []string{"Moyen"}, "Français", "Objectif")
	e1.SetQuizDisplay([]string{"THEME", "LANGUAGE"})
	e1.SetVirtualPlayerLimit(42)

	// Simulate a restart: fresh Engine, same path.
	e2 := NewEngine()
	e2.SetStatePath(statePath)
	if err := e2.LoadState(); err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	state := e2.GetState()
	if state.QuizName != "Mon Quiz" {
		t.Errorf("QuizName not restored, got %q", state.QuizName)
	}
	if state.QuizTheme != "Sciences" {
		t.Errorf("QuizTheme not restored, got %q", state.QuizTheme)
	}
	if state.QuizNotes != "Notes" {
		t.Errorf("QuizNotes not restored, got %q", state.QuizNotes)
	}
	if !reflect.DeepEqual(state.QuizPopulations, []string{"Adulte (18-64 ans)"}) {
		t.Errorf("QuizPopulations not restored, got %v", state.QuizPopulations)
	}
	if !reflect.DeepEqual(state.QuizDifficulties, []string{"Moyen"}) {
		t.Errorf("QuizDifficulties not restored, got %v", state.QuizDifficulties)
	}
	if state.QuizLanguage != "Français" {
		t.Errorf("QuizLanguage not restored, got %q", state.QuizLanguage)
	}
	if state.QuizObjectives != "Objectif" {
		t.Errorf("QuizObjectives not restored, got %q", state.QuizObjectives)
	}
	if !reflect.DeepEqual(state.QuizHiddenFields, []string{"THEME", "LANGUAGE"}) {
		t.Errorf("QuizHiddenFields not restored, got %v", state.QuizHiddenFields)
	}
	if state.VirtualPlayerLimit != 42 {
		t.Errorf("VirtualPlayerLimit not restored, got %d", state.VirtualPlayerLimit)
	}
}

// TestLoadState_NoFile_NoError verifies a fresh install (no game_state.json
// yet) is not an error — NewEngine()'s own defaults apply.
func TestLoadState_NoFile_NoError(t *testing.T) {
	e := NewEngine()
	e.SetStatePath(filepath.Join(t.TempDir(), "does-not-exist.json"))

	if err := e.LoadState(); err != nil {
		t.Fatalf("LoadState on a missing file should not error, got: %v", err)
	}
	if got := e.GetState().VirtualPlayerLimit; got != 20 {
		t.Errorf("Expected NewEngine()'s own default (20) to survive a no-op LoadState, got %d", got)
	}
}

// TestLoadState_NoPathConfigured_NoOp mirrors LoadHistory/LoadTeams's own
// "no path configured" convention (statePath == "").
func TestLoadState_NoPathConfigured_NoOp(t *testing.T) {
	e := NewEngine()
	if err := e.LoadState(); err != nil {
		t.Fatalf("LoadState with no path configured should be a silent no-op, got: %v", err)
	}
}

// TestSaveState_NoPathConfigured_NoOp mirrors SaveHistory/SaveTeams's own
// "no path configured" convention.
func TestSaveState_NoPathConfigured_NoOp(t *testing.T) {
	e := NewEngine()
	if err := e.SaveState(); err != nil {
		t.Fatalf("SaveState with no path configured should be a silent no-op, got: %v", err)
	}
}

// TestSaveState_NoLeftoverTempFile is the atomicity regression test (plan
// task 19 — CreateTemp+Chmod+Rename, same pattern as SaveTeams/SaveBumpers,
// NOT SaveHistory/SaveStatuses' direct os.WriteFile).
func TestSaveState_NoLeftoverTempFile(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "game_state.json")

	e := NewEngine()
	e.SetStatePath(statePath)
	e.SetQuizMeta("Quiz", "", "", nil, nil, "", "")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "game_state.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("Expected exactly one file (game_state.json) after SaveState, got: %v", names)
	}
}

// TestSaveState_WritesFormatVersion verifies the envelope's version field —
// the first versioned on-disk format in this project (plan task 20).
func TestSaveState_WritesFormatVersion(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "game_state.json")
	e := NewEngine()
	e.SetStatePath(statePath)
	e.SetQuizMeta("Quiz", "", "", nil, nil, "", "")

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var onDisk map[string]interface{}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("game_state.json is not valid JSON: %v", err)
	}
	if onDisk["format_version"] != float64(persistedGameStateFormatVersion) {
		t.Errorf("Expected format_version=%d on disk, got %v", persistedGameStateFormatVersion, onDisk["format_version"])
	}
}

// TestLoadState_NewerFormatVersion_DoesNotFail verifies a game_state.json
// written by a FUTURE build (higher format_version) is loaded rather than
// rejected — known fields only, with a log warning (plan task 20's
// forward-compatibility expectation for an envelope that has no migration
// code yet).
func TestLoadState_NewerFormatVersion_DoesNotFail(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "game_state.json")
	future := `{"format_version": 999, "QUIZ_NAME": "From the future"}`
	if err := os.WriteFile(statePath, []byte(future), 0644); err != nil {
		t.Fatalf("could not write fixture: %v", err)
	}

	e := NewEngine()
	e.SetStatePath(statePath)
	if err := e.LoadState(); err != nil {
		t.Fatalf("LoadState should not fail on a newer format_version, got: %v", err)
	}
	if got := e.GetState().QuizName; got != "From the future" {
		t.Errorf("Expected known fields to still load, got QuizName=%q", got)
	}
}

// TestLoadState_MissingVirtualPlayerLimit_DoesNotZeroExistingDefault covers
// the guard in LoadState: a persisted file that omits VIRTUAL_PLAYER_LIMIT
// (e.g. written before that field existed) must not clobber NewEngine()'s
// own 20-default with 0.
func TestLoadState_MissingVirtualPlayerLimit_DoesNotZeroExistingDefault(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "game_state.json")
	noLimit := `{"format_version": 1, "QUIZ_NAME": "Quiz"}`
	if err := os.WriteFile(statePath, []byte(noLimit), 0644); err != nil {
		t.Fatalf("could not write fixture: %v", err)
	}

	e := NewEngine()
	e.SetStatePath(statePath)
	if err := e.LoadState(); err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if got := e.GetState().VirtualPlayerLimit; got != 20 {
		t.Errorf("Expected the pre-existing default (20) preserved when the file omits VIRTUAL_PLAYER_LIMIT, got %d", got)
	}
}

// TestClearQuizMeta_ResetsToDefaults is the #141 counterpart to
// TestNewEngine_QuizHiddenFieldsDefaultsToEmptyNotNil: after populating and
// then clearing, every persisted field is back to its NewEngine() default —
// and slices are non-nil (rule H1).
func TestClearQuizMeta_ResetsToDefaults(t *testing.T) {
	e := NewEngine()
	e.SetQuizMeta("Quiz", "Theme", "Notes", []string{"A"}, []string{"B"}, "Français", "Objectif")
	e.SetQuizDisplay([]string{"THEME"})
	e.SetVirtualPlayerLimit(5)

	e.ClearQuizMeta()

	state := e.GetState()
	if state.QuizName != "" || state.QuizTheme != "" || state.QuizNotes != "" || state.QuizLanguage != "" || state.QuizObjectives != "" {
		t.Errorf("Expected all quiz string fields cleared, got %+v", state)
	}
	if state.QuizPopulations == nil || len(state.QuizPopulations) != 0 {
		t.Errorf("Expected QuizPopulations reset to non-nil empty slice, got %v", state.QuizPopulations)
	}
	if state.QuizDifficulties == nil || len(state.QuizDifficulties) != 0 {
		t.Errorf("Expected QuizDifficulties reset to non-nil empty slice, got %v", state.QuizDifficulties)
	}
	if state.QuizHiddenFields == nil || len(state.QuizHiddenFields) != 0 {
		t.Errorf("Expected QuizHiddenFields reset to non-nil empty slice, got %v", state.QuizHiddenFields)
	}
	if state.VirtualPlayerLimit != 20 {
		t.Errorf("Expected VirtualPlayerLimit reset to the default (20), got %d", state.VirtualPlayerLimit)
	}
}

// TestInitGame_PreservesQuizMetaAndVirtualPlayerLimit is the H5 regression
// test (contract game-state.md rule H5): NEW_GAME (InitGame) must NOT reset
// quiz metadata or the virtual player limit — #141 makes this survive a
// process restart too, but the in-session behavior (already true before
// #141) must not regress.
func TestInitGame_PreservesQuizMetaAndVirtualPlayerLimit(t *testing.T) {
	e := NewEngine()
	e.SetQuizMeta("Mon Quiz", "Sciences", "Notes", []string{"Adulte (18-64 ans)"}, []string{"Moyen"}, "Français", "Objectif")
	e.SetQuizDisplay([]string{"THEME", "LANGUAGE"})
	e.SetVirtualPlayerLimit(42)

	e.InitGame()

	state := e.GetState()
	if state.QuizName != "Mon Quiz" {
		t.Errorf("H5: QuizName must survive InitGame, got %q", state.QuizName)
	}
	if state.QuizTheme != "Sciences" {
		t.Errorf("H5: QuizTheme must survive InitGame, got %q", state.QuizTheme)
	}
	if !reflect.DeepEqual(state.QuizHiddenFields, []string{"THEME", "LANGUAGE"}) {
		t.Errorf("H5: QuizHiddenFields must survive InitGame, got %v", state.QuizHiddenFields)
	}
	if state.VirtualPlayerLimit != 42 {
		t.Errorf("VirtualPlayerLimit must survive InitGame, got %d", state.VirtualPlayerLimit)
	}
}
