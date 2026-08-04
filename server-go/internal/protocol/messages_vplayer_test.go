package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// SerializeForVPlayer (#127 T2.1/T2.5)
//
// Contrat : contracts/vplayer-payload-filter.md §2.
// Reference implementation used directly by these tests; the hot fan-out
// path (cmd/server/main.go, buildVPlayerPayloads/buildVPlayerMessageBytes)
// reimplements the same rule for performance (see
// _work/reports/dev-backend-t2-benchmark-*.md) and is covered by its own
// byte-for-byte non-regression tests in cmd/server.
// ---------------------------------------------------------------------------

const vplayerTestPlayerID = "vjoueur-alice"
const vplayerTestOtherBumperID = "AA:BB:CC:DD:EE:01"

// buildVPlayerUpdateMsg builds an UPDATE message with the given GAME.PHASE,
// two bumpers (one keyed by vplayerTestPlayerID with all 5 admin-only fields
// populated, one keyed by vplayerTestOtherBumperID so reduction-to-one-entry
// is actually observable), a "teams" map, and a top-level "config" key.
func buildVPlayerUpdateMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":        phase,
			"CURRENT_TIME": 12,
			"TIME":         int64(1234567890),
		},
		"config": map[string]interface{}{
			"auto_open": true,
		},
		"bumpers": map[string]interface{}{
			vplayerTestPlayerID: map[string]interface{}{
				"NAME":             "Alice",
				"TEAM":             "TeamA",
				"CONNECTED":        true,
				"IS_VIRTUAL":       true,
				"IS_VPLAYER":       true,
				"SCORE":            30,
				"FIRMWARE_VERSION": "3.7.0", // present here on purpose, to prove stripping actually removes it
				"IS_OUTDATED":      true,
				"OTA_STATUS":       "done",
				"OTA_PERCENT":      100,
				"ACK_PENDING":      true,
			},
			vplayerTestOtherBumperID: map[string]interface{}{
				"NAME":      "Buzzer1",
				"TEAM":      "TeamB",
				"CONNECTED": true,
				"SCORE":     10,
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 30},
			"TeamB": map[string]interface{}{"NAME": "TeamB", "SCORE": 10},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	msg.Msg = rawMsg
	msg.Version = "test"
	return msg
}

// ---------------------------------------------------------------------------
// Gating par phase — réduit sur PREPARE/READY, complet ailleurs
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_PhaseGating(t *testing.T) {
	tests := []struct {
		phase   string
		reduced bool
	}{
		{"PREPARE", true},
		{"READY", true},
		{"COUNTDOWN", false},
		{"STARTED", false},
		{"PAUSED", false},
		{"STOPPED", false},
		{"REVEALED", false},
		{"", false}, // malformed/missing phase — never reduce
	}

	for _, tc := range tests {
		t.Run("phase="+tc.phase, func(t *testing.T) {
			msg := buildVPlayerUpdateMsg(t, tc.phase)

			got, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}

			bumpers := parseBumperMap(t, got)
			if tc.reduced {
				if len(bumpers) != 1 {
					t.Errorf("phase %q: expected reduced bumpers map (1 entry), got %d entries: %v", tc.phase, len(bumpers), keysOfMap(bumpers))
				}
				if _, present := bumpers[vplayerTestOtherBumperID]; present {
					t.Errorf("phase %q: expected the OTHER bumper to be absent from the reduced map", tc.phase)
				}
			} else {
				if len(bumpers) != 2 {
					t.Errorf("phase %q: expected complete bumpers map (2 entries, fallback to SerializeForWebClient), got %d: %v", tc.phase, len(bumpers), keysOfMap(bumpers))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// playerID vide -> complet (condition 3 du contrat)
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_EmptyPlayerID_ReturnsComplete(t *testing.T) {
	msg := buildVPlayerUpdateMsg(t, "READY")

	got, err := msg.SerializeForVPlayer("")
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	want, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("empty playerID: expected byte-identical output to SerializeForWebClient (complete payload)\n got:  %s\nwant: %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// Action != UPDATE -> complet, quelle que soit la phase/playerID
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_NonUpdateAction_ReturnsComplete(t *testing.T) {
	startMsg, err := NewMessage(ActionStart, map[string]interface{}{"DELAY": 5})
	if err != nil {
		t.Fatalf("failed to build START message: %v", err)
	}

	got, err := startMsg.SerializeForVPlayer(vplayerTestPlayerID)
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	want, err := startMsg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("non-UPDATE action: expected byte-identical output to SerializeForWebClient\n got:  %s\nwant: %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// GAME et teams identiques octet pour octet au payload de référence
// (SerializeForWebClient, qui ne touche ni GAME ni teams)
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_GameAndTeamsByteIdenticalToReference(t *testing.T) {
	msg := buildVPlayerUpdateMsg(t, "PREPARE")

	reduced, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	reference, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	reducedMsg := parseMsgMap(t, reduced)
	referenceMsg := parseMsgMap(t, reference)

	reducedGame, err := json.Marshal(reducedMsg["GAME"])
	if err != nil {
		t.Fatalf("failed to re-marshal reduced GAME: %v", err)
	}
	referenceGame, err := json.Marshal(referenceMsg["GAME"])
	if err != nil {
		t.Fatalf("failed to re-marshal reference GAME: %v", err)
	}
	if string(reducedGame) != string(referenceGame) {
		t.Errorf("GAME differs between SerializeForVPlayer and SerializeForWebClient:\n reduced:   %s\n reference: %s", reducedGame, referenceGame)
	}

	reducedTeams, err := json.Marshal(reducedMsg["teams"])
	if err != nil {
		t.Fatalf("failed to re-marshal reduced teams: %v", err)
	}
	referenceTeams, err := json.Marshal(referenceMsg["teams"])
	if err != nil {
		t.Fatalf("failed to re-marshal reference teams: %v", err)
	}
	if string(reducedTeams) != string(referenceTeams) {
		t.Errorf("teams differs between SerializeForVPlayer and SerializeForWebClient (contract §2: teams must stay complete):\n reduced:   %s\n reference: %s", reducedTeams, referenceTeams)
	}
}

// ---------------------------------------------------------------------------
// bumpers = 1 entrée, sans les 5 champs OTA/ACK
// ---------------------------------------------------------------------------

func TestSerializeForVPlayer_ReducedBumper_StripsAdminOnlyFields(t *testing.T) {
	msg := buildVPlayerUpdateMsg(t, "READY")

	got, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}

	bumpers := parseBumperMap(t, got)
	if len(bumpers) != 1 {
		t.Fatalf("expected exactly 1 bumper entry, got %d: %v", len(bumpers), keysOfMap(bumpers))
	}
	own, ok := bumpers[vplayerTestPlayerID].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the single entry to be keyed by %q, got keys %v", vplayerTestPlayerID, keysOfMap(bumpers))
	}
	for _, field := range AdminOnlyBumperFields {
		if _, present := own[field]; present {
			t.Errorf("expected admin-only field %q to be stripped from the reduced bumper entry", field)
		}
	}
	// Essential fields must survive.
	for _, field := range []string{"NAME", "TEAM", "CONNECTED", "SCORE"} {
		if _, present := own[field]; !present {
			t.Errorf("expected essential field %q to survive stripping", field)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON invalide -> repli sur SerializeForWebClient (jamais de panique, jamais
// de message silencieusement perdu — même comportement que le repli,
// succès ou erreur, y compris dans les rares cas où même
// SerializeForWebClient ne peut rien produire d'un Msg réellement corrompu)
// ---------------------------------------------------------------------------

// TestSerializeForVPlayer_TrulyMalformedJSON_MatchesWebClientFallback covers
// Msg bytes that aren't valid JSON at all — a case SerializeForWebClient
// itself cannot recover from either (json.RawMessage validates its content
// on Marshal, so even ITS OWN json.Marshal(m) fallback returns an error
// here). SerializeForVPlayer must behave IDENTICALLY to SerializeForWebClient
// for the exact same broken input — never worse (never a panic, never a
// narrower "success" that hides the corruption), never a silently different
// result.
func TestSerializeForVPlayer_TrulyMalformedJSON_MatchesWebClientFallback(t *testing.T) {
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	msg.Msg = []byte(`{not valid json`)

	got, gotErr := msg.SerializeForVPlayer(vplayerTestPlayerID)
	want, wantErr := msg.SerializeForWebClient()

	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("SerializeForVPlayer error-ness diverges from SerializeForWebClient's for the same malformed input: got err=%v, want err=%v", gotErr, wantErr)
	}
	if string(got) != string(want) {
		t.Errorf("malformed JSON: expected byte-identical result to SerializeForWebClient (success or failure alike)\n got:  %s (err=%v)\nwant: %s (err=%v)", got, gotErr, want, wantErr)
	}
}

// TestSerializeForVPlayer_StructurallyUnexpectedJSON_FallsBackCleanly covers
// the realistic "malformed for VPlayer's own parsing" case: the top-level
// Msg IS valid JSON (SerializeForWebClient succeeds normally), but a key
// SerializeForVPlayer specifically needs (GAME, or bumpers) is missing or
// not the expected type — must fall back to the complete payload rather
// than error out or send something narrower/broken.
func TestSerializeForVPlayer_StructurallyUnexpectedJSON_FallsBackCleanly(t *testing.T) {
	tests := []struct {
		name string
		msg  string
	}{
		{"GAME missing", `{"teams":{},"bumpers":{}}`},
		{"GAME wrong type", `{"GAME":"not-an-object","teams":{},"bumpers":{}}`},
		{"bumpers missing", `{"GAME":{"PHASE":"READY"},"teams":{}}`},
		{"bumpers wrong type", `{"GAME":{"PHASE":"READY"},"teams":{},"bumpers":[]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := NewMessage(ActionUpdate, nil)
			if err != nil {
				t.Fatalf("failed to build message: %v", err)
			}
			msg.Msg = []byte(tc.msg)

			got, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
			if err != nil {
				t.Fatalf("SerializeForVPlayer must not error on structurally-unexpected-but-valid JSON: %v", err)
			}
			want, err := msg.SerializeForWebClient()
			if err != nil {
				t.Fatalf("SerializeForWebClient failed: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("%s: expected byte-identical fallback to SerializeForWebClient\n got:  %s\nwant: %s", tc.name, got, want)
			}
		})
	}
}

// TestSerializeForVPlayer_RecipientBumperMissing_FallsBack covers the case
// documented in SerializeForVPlayer's own comment: the recipient's own
// bumper isn't present in this particular snapshot (e.g. evicted the same
// instant this broadcast was built) — must fall back rather than send a
// bumpers map missing the recipient's own entry.
func TestSerializeForVPlayer_RecipientBumperMissing_FallsBack(t *testing.T) {
	msg := buildVPlayerUpdateMsg(t, "READY")

	got, err := msg.SerializeForVPlayer("some-other-unknown-player-id")
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	want, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("unknown playerID: expected byte-identical fallback to SerializeForWebClient\n got:  %s\nwant: %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseBumperMap(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	msgMap := parseMsgMap(t, data)
	bumpersRaw, ok := msgMap["bumpers"]
	if !ok {
		return nil
	}
	bumpersMap, ok := bumpersRaw.(map[string]interface{})
	if !ok {
		return nil
	}
	return bumpersMap
}

// parseMsgMap is defined in messages_test.go (same package) and reused here.

func keysOfMap(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
