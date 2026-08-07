package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Confidentialité de GAME.QUIZ_OBJECTIVES (v6.1.0, #137 Batch 2b)
//
// Contrat : contracts/game-state.md § "QUIZ_OBJECTIVES — champ à diffusion
// restreinte", contracts/ws-payload-serialization.md tableau 2. L'objectif
// de la partie est une consigne d'animation/génération IA — il ne doit
// JAMAIS être lisible depuis un client TV ou VJoueur, même dans les outils
// réseau du navigateur, alors que le reste des champs QUIZ_* est diffusé et
// affiché sur l'écran TV NEW_GAME.
//
// Trois chemins de sérialisation vivent dans ce package (AdminOnlyGameFields,
// messages.go) et sont couverts ci-dessous ; un 4e (fan-out chaud VPlayer,
// cmd/server/main.go:buildVPlayerPayloads, qui réimplique le filtrage par
// concaténation d'octets plutôt que d'appeler SerializeForVPlayer) est
// couvert séparément par cmd/server/vplayer_fanout_quiz_objectives_test.go.
// Les quatre sont nécessaires — un chemin non testé est précisément celui
// qui peut fuir silencieusement (plan
// _work/reports/planner-20260806-145021-plan-137-batch2b.md §7/§8).
// ---------------------------------------------------------------------------

const quizObjectivesTestPlayerID = "vjoueur-quiz-meta"

// buildQuizMetaUpdateMsg builds a realistic UPDATE message whose GAME node
// carries QUIZ_OBJECTIVES alongside other QUIZ_* fields that ARE meant to
// reach TV/VPlayer, so a test can tell "QUIZ_OBJECTIVES specifically
// stripped" apart from "the whole GAME node vanished by accident". phase
// drives SerializeForVPlayer's reduction gate
// (contracts/vplayer-payload-filter.md §2): PREPARE/READY take the reduced
// path, anything else falls back to SerializeForWebClient.
func buildQuizMetaUpdateMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":              phase,
			"QUIZ_NAME":          "Quiz ciné",
			"QUIZ_THEME":         "Cinéma français",
			"QUIZ_POPULATIONS":   []string{"Adulte (18-64 ans)"},
			"QUIZ_DIFFICULTIES":  []string{"Moyen"},
			"QUIZ_OBJECTIVES":    "Réviser le chapitre 3 avant le contrôle — ne jamais afficher aux joueurs",
			"QUIZ_HIDDEN_FIELDS": []string{"DIFFICULTIES"},
		},
		"bumpers": map[string]interface{}{
			quizObjectivesTestPlayerID: map[string]interface{}{
				"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true, "IS_VIRTUAL": true, "IS_VPLAYER": true,
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildQuizMetaUpdateMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("buildQuizMetaUpdateMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

// gameNodeOf extracts MSG.GAME from a serialized message as a map, for
// field-presence assertions.
func gameNodeOf(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	msgMap := parseMsgMap(t, data)
	game, ok := msgMap["GAME"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a GAME node in MSG, got %v", msgMap)
	}
	return game
}

// -----------------------------------------------------------------------------
// SerializeForAdmin — QUIZ_OBJECTIVES present (admin edits it in QuestionsPage)
// -----------------------------------------------------------------------------

func TestSerializeForAdmin_QuizObjectivesPresent(t *testing.T) {
	msg := buildQuizMetaUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if _, present := game["QUIZ_OBJECTIVES"]; !present {
		t.Error("SerializeForAdmin: QUIZ_OBJECTIVES should be present (admin edits it in the Quiz section) but is missing")
	}
	// Sanity: a sibling QUIZ_* field genuinely survives too — proves this
	// isn't a coincidental empty-GAME-node pass.
	if game["QUIZ_NAME"] != "Quiz ciné" {
		t.Errorf("SerializeForAdmin: expected QUIZ_NAME to survive untouched, got %v", game["QUIZ_NAME"])
	}
}

// -----------------------------------------------------------------------------
// SerializeForWebClient (TV + VPlayer fallback) — QUIZ_OBJECTIVES absent
// -----------------------------------------------------------------------------

func TestSerializeForWebClient_StripsQuizObjectives(t *testing.T) {
	msg := buildQuizMetaUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	game := gameNodeOf(t, data)
	if _, present := game["QUIZ_OBJECTIVES"]; present {
		t.Errorf("SerializeForWebClient: QUIZ_OBJECTIVES should be stripped (confidentiality — TV must never receive it), got %v", game["QUIZ_OBJECTIVES"])
	}
	// The other QUIZ_* fields ARE shown on the TV NEW_GAME screen (contract
	// game-state.md) — they must survive, otherwise this test would trivially
	// pass by stripping the whole GAME node instead of just the one field.
	if game["QUIZ_NAME"] != "Quiz ciné" {
		t.Errorf("SerializeForWebClient: expected QUIZ_NAME to survive (only QUIZ_OBJECTIVES is admin-only), got %v", game["QUIZ_NAME"])
	}
	pops, ok := game["QUIZ_POPULATIONS"].([]interface{})
	if !ok || len(pops) != 1 || pops[0] != "Adulte (18-64 ans)" {
		t.Errorf("SerializeForWebClient: expected QUIZ_POPULATIONS to survive untouched, got %v", game["QUIZ_POPULATIONS"])
	}
}

// -----------------------------------------------------------------------------
// SerializeForVPlayer — QUIZ_OBJECTIVES absent in BOTH the non-reduced
// (falls back to SerializeForWebClient) and reduced (PREPARE/READY, own
// GAME-filtering branch) variants — contract §7 of the handoff explicitly
// calls out both, since the reduced branch parses GAME independently and
// could regress on its own without the non-reduced branch noticing.
// -----------------------------------------------------------------------------

func TestSerializeForVPlayer_StripsQuizObjectives(t *testing.T) {
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
			msg := buildQuizMetaUpdateMsg(t, tt.phase)

			data, err := msg.SerializeForVPlayer(quizObjectivesTestPlayerID)
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}

			game := gameNodeOf(t, data)
			if _, present := game["QUIZ_OBJECTIVES"]; present {
				t.Errorf("SerializeForVPlayer(phase=%s): QUIZ_OBJECTIVES should be stripped (confidentiality — a VJoueur's dev tools must never show it), got %v", tt.phase, game["QUIZ_OBJECTIVES"])
			}
			if game["QUIZ_NAME"] != "Quiz ciné" {
				t.Errorf("SerializeForVPlayer(phase=%s): expected QUIZ_NAME to survive, got %v", tt.phase, game["QUIZ_NAME"])
			}
		})
	}
}

// -----------------------------------------------------------------------------
// QUIZ_HIDDEN_FIELDS (v6.1.0, #137 Batch 2b T1.8) — display preference, NOT
// confidentiality: unlike QUIZ_OBJECTIVES it must be transmitted to TV/VPlayer
// so the client can apply the preference itself (contract game-state.md
// §"Diffusion — préférence d'affichage ≠ confidentialité",
// ws-payload-serialization.md tableau 2). Same three sites as QUIZ_OBJECTIVES,
// opposite expected outcome: present everywhere, never stripped.
// -----------------------------------------------------------------------------

func TestSerializeForWebClient_KeepsQuizHiddenFields(t *testing.T) {
	msg := buildQuizMetaUpdateMsg(t, "STARTED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}

	game := gameNodeOf(t, data)
	hidden, ok := game["QUIZ_HIDDEN_FIELDS"].([]interface{})
	if !ok || len(hidden) != 1 || hidden[0] != "DIFFICULTIES" {
		t.Errorf("SerializeForWebClient: QUIZ_HIDDEN_FIELDS should survive untouched (TV applies the preference client-side), got %v", game["QUIZ_HIDDEN_FIELDS"])
	}
}

func TestSerializeForVPlayer_KeepsQuizHiddenFields(t *testing.T) {
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
			msg := buildQuizMetaUpdateMsg(t, tt.phase)

			data, err := msg.SerializeForVPlayer(quizObjectivesTestPlayerID)
			if err != nil {
				t.Fatalf("SerializeForVPlayer failed: %v", err)
			}

			game := gameNodeOf(t, data)
			hidden, ok := game["QUIZ_HIDDEN_FIELDS"].([]interface{})
			if !ok || len(hidden) != 1 || hidden[0] != "DIFFICULTIES" {
				t.Errorf("SerializeForVPlayer(phase=%s): QUIZ_HIDDEN_FIELDS should survive untouched, got %v", tt.phase, game["QUIZ_HIDDEN_FIELDS"])
			}
		})
	}
}
