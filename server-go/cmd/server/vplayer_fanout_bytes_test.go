package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// #127 T2.4 follow-up — byte-concatenation prototype non-regression
//
// Handoff: _work/handoff/task-dev-backend-20260803-141700.md
//
// buildVPlayerMessageBytes (cmd/server/main.go) assembles the per-recipient
// UPDATE frame by direct []byte concatenation instead of json.Marshal, to
// avoid the O(len(GAME)) compact()-rescan cost json.Marshal pays on every
// json.RawMessage value it touches (identified by BenchmarkVPlayerFanout as
// the dominant cost for MEMOTION questions). This file proves the two
// methods produce byte-for-byte identical output — the ONLY thing allowed to
// change is how the bytes are built, never the bytes themselves.
// ---------------------------------------------------------------------------

// referenceVPlayerMessageBytes builds the same frame via the "slow" path
// (the one BenchmarkVPlayerFanout originally measured, T2.1-T2.3 as
// committed in f9e7b48): unmarshal into map[string]json.RawMessage, marshal
// that map, wrap in a Message, marshal the whole Message. This is the
// ground truth buildVPlayerMessageBytes must match exactly.
func referenceVPlayerMessageBytes(t *testing.T, action, version, playerID string, gameRaw, teamsRaw, strippedBumper json.RawMessage) []byte {
	t.Helper()
	reducedBumpers, err := json.Marshal(map[string]json.RawMessage{playerID: strippedBumper})
	if err != nil {
		t.Fatalf("reference: failed to marshal reduced bumpers: %v", err)
	}
	reducedMsg, err := json.Marshal(map[string]json.RawMessage{
		"GAME":    gameRaw,
		"teams":   teamsRaw,
		"bumpers": reducedBumpers,
	})
	if err != nil {
		t.Fatalf("reference: failed to marshal MSG: %v", err)
	}
	out := &protocol.Message{Action: action, Version: version, Msg: reducedMsg}
	data, err := out.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("reference: failed to marshal Message: %v", err)
	}
	return data
}

func TestBuildVPlayerMessageBytes_MatchesReferencePath(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		version  string
		playerID string
		bumper   map[string]interface{}
		gameRaw  string
		teamsRaw string
	}{
		{
			name: "simple ASCII playerID and bumper", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: "vp-alice",
			bumper:   map[string]interface{}{"NAME": "Alice", "TEAM": "TeamA", "SCORE": 30, "CONNECTED": true},
			gameRaw:  `{"PHASE":"READY","DELAY":30}`,
			teamsRaw: `{"TeamA":{"NAME":"TeamA","SCORE":15}}`,
		},
		{
			name: "empty version (omitempty must drop the VERSION key)", action: protocol.ActionUpdate, version: "",
			playerID: "vp-bob",
			bumper:   map[string]interface{}{"NAME": "Bob", "SCORE": 0},
			gameRaw:  `{"PHASE":"PREPARE"}`,
			teamsRaw: `{}`,
		},
		{
			name: "playerID with double quote and backslash", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: `vp-"weird"\name`,
			bumper:   map[string]interface{}{"NAME": `Weird "Player"`, "SCORE": 5},
			gameRaw:  `{"PHASE":"READY"}`,
			teamsRaw: `{"TeamA":{"NAME":"TeamA"}}`,
		},
		{
			name: "playerID and bumper NAME with unicode / emoji", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: "vp-héllo-🎉",
			bumper:   map[string]interface{}{"NAME": "Chloé 🎉", "SCORE": 12},
			gameRaw:  `{"PHASE":"READY"}`,
			teamsRaw: `{}`,
		},
		{
			name: "playerID with control characters and newline", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: "vp-line\nbreak\ttab",
			bumper:   map[string]interface{}{"NAME": "X"},
			gameRaw:  `{"PHASE":"READY"}`,
			teamsRaw: `{}`,
		},
		{
			name: "bumper with HTML-sensitive characters (<script>&)", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: "vp-html",
			bumper:   map[string]interface{}{"NAME": "<script>alert(1)</script>&amp;"},
			gameRaw:  `{"PHASE":"READY"}`,
			teamsRaw: `{}`,
		},
		{
			name: "large-ish GAME payload (simulated MEMOTION-scale)", action: protocol.ActionUpdate, version: "5.9.1",
			playerID: "vp-motion",
			bumper:   map[string]interface{}{"NAME": "MotionPlayer", "SCORE": 8},
			gameRaw:  buildLargeGameRawForTest(24),
			teamsRaw: `{"TeamA":{"NAME":"TeamA"},"TeamB":{"NAME":"TeamB"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strippedBumper, err := json.Marshal(tc.bumper)
			if err != nil {
				t.Fatalf("failed to marshal test bumper: %v", err)
			}
			playerIDJSON, err := json.Marshal(tc.playerID)
			if err != nil {
				t.Fatalf("failed to marshal test playerID: %v", err)
			}
			var versionJSON []byte
			if tc.version != "" {
				versionJSON, err = json.Marshal(tc.version)
				if err != nil {
					t.Fatalf("failed to marshal test version: %v", err)
				}
			}
			actionJSON, err := json.Marshal(tc.action)
			if err != nil {
				t.Fatalf("failed to marshal test action: %v", err)
			}

			got := buildVPlayerMessageBytes(actionJSON, versionJSON, json.RawMessage(tc.gameRaw), json.RawMessage(tc.teamsRaw), playerIDJSON, strippedBumper)
			want := referenceVPlayerMessageBytes(t, tc.action, tc.version, tc.playerID, json.RawMessage(tc.gameRaw), json.RawMessage(tc.teamsRaw), strippedBumper)

			if !bytes.Equal(got, want) {
				t.Fatalf("byte mismatch:\n got:  %s\n want: %s", got, want)
			}

			// The concatenated bytes must also be valid, semantically correct
			// JSON that round-trips to the expected structure — not just
			// coincidentally byte-equal to a possibly-wrong reference.
			var parsed struct {
				Action string `json:"ACTION"`
				Msg    struct {
					Bumpers map[string]map[string]interface{} `json:"bumpers"`
				} `json:"MSG"`
			}
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("output is not valid JSON: %v (bytes: %s)", err, got)
			}
			if _, ok := parsed.Msg.Bumpers[tc.playerID]; !ok {
				t.Errorf("expected bumpers map to contain key %q, got keys %v", tc.playerID, keysOf(parsed.Msg.Bumpers))
			}
		})
	}
}

// buildLargeGameRawForTest returns a compact JSON object shaped like a real
// GAME node with a MEMOTION-sized MOTION_CARDS array, to exercise the
// concatenation path against something closer to the ~11KB real-world case
// than the tiny fixtures above.
func buildLargeGameRawForTest(nCards int) string {
	cards := make([]game.MotionCard, 0, nCards)
	for i := 0; i < nCards; i++ {
		cards = append(cards, game.MotionCard{
			ID: fmt.Sprintf("mc-%d", i), RectoTheme: "Thème", Difficulty: 2,
			QuestionText: "Une question assez longue pour peser dans le payload, avec des accents éà.",
			AnswerText:   "Une réponse également assez longue, avec guillemets \"internes\" et unicode 🎉.",
		})
	}
	state := struct {
		Phase       string           `json:"PHASE"`
		MotionCards []game.MotionCard `json:"MOTION_CARDS"`
	}{Phase: "READY", MotionCards: cards}
	b, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func keysOf(m map[string]map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// End-to-end sanity: buildVPlayerPayloads (the production entry point) still
// produces JSON that round-trips correctly for a recipient whose playerID
// contains characters that would break naive string concatenation.
// ---------------------------------------------------------------------------

func TestBuildVPlayerPayloads_SpecialCharacterPlayerID_RoundTrips(t *testing.T) {
	weirdID := `vp-"weird"\id` + "\n"
	msg := buildFanoutBenchMessageWithPlayerIDs(t, []string{weirdID}, benchQCMQuestion())

	payloads, ok := buildVPlayerPayloads(msg, []server.VPlayerRecipient{{ClientID: "c1", PlayerID: weirdID}})
	if !ok {
		t.Fatalf("expected reduction to apply")
	}
	data, present := payloads[weirdID]
	if !present {
		t.Fatalf("expected a payload for playerID %q, got keys %v", weirdID, mapKeys(payloads))
	}

	var parsed struct {
		Msg struct {
			Bumpers map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v (bytes: %s)", err, data)
	}
	if len(parsed.Msg.Bumpers) != 1 {
		t.Fatalf("expected exactly 1 bumper entry, got %d: %v", len(parsed.Msg.Bumpers), parsed.Msg.Bumpers)
	}
	if _, ok := parsed.Msg.Bumpers[weirdID]; !ok {
		t.Errorf("expected bumpers map to contain the exact weird playerID as key, got %v", mapKeys2(parsed.Msg.Bumpers))
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapKeys2(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// buildFanoutBenchMessageWithPlayerIDs is a variant of
// buildFanoutBenchMessage (vplayer_fanout_bench_test.go) that uses caller-
// supplied bumper IDs instead of "bench-vp-NN", so a test can exercise a
// specific, deliberately awkward playerID end to end.
func buildFanoutBenchMessageWithPlayerIDs(t *testing.T, ids []string, question *game.Question) *protocol.Message {
	t.Helper()
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Score: 15, Ready: true},
	}
	bumpers := map[string]*game.Bumper{}
	for i, id := range ids {
		bumpers[id] = &game.Bumper{
			Name: fmt.Sprintf("Player%d", i), Team: "TeamA", Score: i * 3,
			Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true,
		}
	}
	state := &game.GameState{
		Phase:                    game.PhaseReady,
		MemoryFlippedCards:       []string{},
		MemoryMatchedPairs:       []int{},
		MemoryTeamPairs:          map[string]int{},
		MemoryTeamErrors:         map[string]int{},
		MemoryParticipatingTeams: []string{},
		MemoryPairOwners:         map[int]string{},
		MemoryCurrentTeamColor:   []int{},
		QcmInvalidated:           []string{},
		MotionCardStates:         map[string]string{},
		MotionCardTeams:          map[string]string{},
		MotionParticipatingTeams: []string{},
		MotionCurrentTeamColor:   []int{},
		ArdoiseAnswers:           map[string]game.ArdoiseAnswer{},
		Question:                 question,
	}
	data, err := (&game.GameData{Game: state, Teams: teams, Bumpers: bumpers}).ToJSON()
	if err != nil {
		t.Fatalf("failed to build GameData JSON: %v", err)
	}
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	msg.Msg = data
	msg.Version = "test"
	return msg
}
