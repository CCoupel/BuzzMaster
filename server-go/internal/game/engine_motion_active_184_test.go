// Tests for #184 B-B4 — GameState.MotionActive, the single active-card slot
// (contract §5). Run: go test ./internal/game/... -run TestMotionActive -v

package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestMotionActive_ZeroValueAfterInit verifies NewEngine()/InitMotionState()
// leave MotionActive at its documented empty shape — never nil maps, so JSON
// serializes {"CARD_ID":"","TYPE":"","STATE":{}}, not {"CARD_ID":"","TYPE":"","STATE":null}.
func TestMotionActive_ZeroValueAfterInit(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	state := e.GetState()
	if state.MotionActive.CardID != "" || state.MotionActive.Type != "" {
		t.Errorf("MotionActive should be empty after init, got %+v", state.MotionActive)
	}
	if state.MotionActive.State == nil {
		t.Error("MotionActive.State should be an empty map, not nil (no omitempty — must serialize {})")
	}

	data, err := json.Marshal(state.MotionActive)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(data) != `{"CARD_ID":"","TYPE":"","STATE":{}}` {
		t.Errorf("MotionActive JSON = %s, want {\"CARD_ID\":\"\",\"TYPE\":\"\",\"STATE\":{}}", data)
	}
}

// TestMotionActive_SetOnSelect verifies SelectMotionCard populates
// MotionActive with the selected card's ID and effective TYPE.
func TestMotionActive_SetOnSelect(t *testing.T) {
	e := NewEngine()
	cards := defaultMotionCards()
	cards[1].Type = QuestionTypeQCM // mc-2
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-2"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}

	state := e.GetState()
	if state.MotionActive.CardID != "mc-2" {
		t.Errorf("MotionActive.CardID = %q, want mc-2", state.MotionActive.CardID)
	}
	if state.MotionActive.Type != QuestionTypeQCM {
		t.Errorf("MotionActive.Type = %q, want QCM", state.MotionActive.Type)
	}
	if len(state.MotionActive.State) != 0 {
		t.Errorf("MotionActive.State should start empty, got %v", state.MotionActive.State)
	}
}

// TestMotionActive_DefaultsToSpeedyType verifies a card with no TYPE (every
// MEMOTION question saved before #184) populates MotionActive.Type=SPEEDY,
// not "".
func TestMotionActive_DefaultsToSpeedyType(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO") // no card sets Type
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}

	if got := e.GetState().MotionActive.Type; got != QuestionTypeSpeedy {
		t.Errorf("MotionActive.Type = %q, want SPEEDY (default for a TYPE-less card)", got)
	}
}

// TestMotionActive_PersistsThroughFlipAndReveal verifies MotionActive is
// NOT re-initialised on FlipMotionCard/RevealMotionCard — contract §5.1:
// reset only "à chaque MEMOTION_SELECT".
func TestMotionActive_PersistsThroughFlipAndReveal(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	before := e.GetState().MotionActive

	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard failed: %v", err)
	}
	if got := e.GetState().MotionActive; !reflect.DeepEqual(got, before) {
		t.Errorf("MotionActive changed on FlipMotionCard: before=%+v after=%+v", before, got)
	}

	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard failed: %v", err)
	}
	if got := e.GetState().MotionActive; !reflect.DeepEqual(got, before) {
		t.Errorf("MotionActive changed on RevealMotionCard: before=%+v after=%+v", before, got)
	}
}

// TestMotionActive_EmptiedOnDoneMotionCard verifies both DoneMotionCard
// paths (normal completion and SELECTED-cancellation) empty MotionActive
// back to its zero shape on return to GRID.
func TestMotionActive_EmptiedOnDoneMotionCard(t *testing.T) {
	t.Run("normal completion", func(t *testing.T) {
		e := NewEngine()
		e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
		q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
		startMEMOTION(t, e, "mq1", q)
		defer e.Stop()

		_ = e.SelectMotionCard("mc-1")
		_ = e.FlipMotionCard()
		_ = e.RevealMotionCard()
		if _, _, err := e.DoneMotionCard("mc-1", "red"); err != nil {
			t.Fatalf("DoneMotionCard failed: %v", err)
		}

		got := e.GetState().MotionActive
		if got.CardID != "" || got.Type != "" || len(got.State) != 0 {
			t.Errorf("MotionActive should be emptied after DoneMotionCard, got %+v", got)
		}
	})

	t.Run("cancelled from SELECTED", func(t *testing.T) {
		e := NewEngine()
		q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
		startMEMOTION(t, e, "mq1", q)
		defer e.Stop()

		_ = e.SelectMotionCard("mc-1")
		if _, _, err := e.DoneMotionCard("mc-1", ""); err != nil {
			t.Fatalf("DoneMotionCard (cancel) failed: %v", err)
		}

		got := e.GetState().MotionActive
		if got.CardID != "" || got.Type != "" || len(got.State) != 0 {
			t.Errorf("MotionActive should be emptied after cancelling from SELECTED, got %+v", got)
		}
	})
}

// TestMotionActive_NotPersisted verifies PersistedGameState (the
// game_state.json envelope) has no field that could carry MotionActive —
// contract §5.2's "Non persisté", structurally guaranteed rather than
// merely conventional: SaveState (state_persistence.go) builds
// PersistedGameState field-by-field from an explicit allowlist, so there is
// no field for MotionActive to leak through even if someone tried.
func TestMotionActive_NotPersisted(t *testing.T) {
	data, err := json.Marshal(PersistedGameState{})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, exists := m["MEMOTION_ACTIVE"]; exists {
		t.Error("PersistedGameState must never carry MEMOTION_ACTIVE — it's ephemeral per-game state (contract §5.2)")
	}
}
