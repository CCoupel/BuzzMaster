package main

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Confidentialité de GAME.QUIZ_OBJECTIVES sur le chemin de fan-out VPlayer
// "chaud" (#137 Batch 2b, contract game-state.md § "QUIZ_OBJECTIVES — champ
// à diffusion restreinte", 3e site listé).
//
// buildVPlayerPayloads réimplémente le filtrage de SerializeForVPlayer par
// concaténation d'octets (voir son commentaire dans main.go) pour éviter un
// aller-retour map[string]interface{} par destinataire. Ce chemin extrait le
// nœud GAME directement du message source, indépendamment de
// SerializeForWebClient : sans son propre appel à stripVPlayerHiddenGameFields,
// QUIZ_OBJECTIVES fuirait vers tout VJoueur en phase PREPARE/READY, même
// après la correction apportée à internal/protocol/messages.go — c'est
// exactement le "chemin oublié" que contracts/ws-payload-serialization.md
// met en garde contre.
// ---------------------------------------------------------------------------

const quizObjectivesFanoutPlayerID = "vp-quiz-objectives"
const quizObjectivesFanoutSecret = "NE JAMAIS DIFFUSER — objectif de la partie"

// buildQuizObjectivesFanoutMessage builds an UPDATE message via the real
// game.GameData/ToJSON path (not a hand-rolled map) whose GAME node carries
// a distinctive QUIZ_OBJECTIVES value alongside QUIZ_NAME, so the test can
// tell "QUIZ_OBJECTIVES stripped" apart from "the whole GAME node got
// mangled". Field initialization mirrors buildFanoutBenchMessageWithPlayerIDs
// (vplayer_fanout_bytes_test.go) — the slice/map fields GameState needs
// non-nil to serialize safely.
func buildQuizObjectivesFanoutMessage(t *testing.T, phase game.GamePhase) *protocol.Message {
	t.Helper()
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Score: 0, Ready: true},
	}
	bumpers := map[string]*game.Bumper{
		quizObjectivesFanoutPlayerID: {
			Name: "Alice", Team: "TeamA", Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true,
		},
	}
	state := &game.GameState{
		Phase:                    phase,
		QuizName:                 "Quiz confidentiel",
		QuizPopulations:          []string{"Adulte (18-64 ans)"},
		QuizDifficulties:         []string{"Moyen"},
		QuizObjectives:           quizObjectivesFanoutSecret,
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

func TestBuildVPlayerPayloads_StripsQuizObjectives(t *testing.T) {
	msg := buildQuizObjectivesFanoutMessage(t, game.PhaseReady)

	payloads, ok := buildVPlayerPayloads(msg, []server.VPlayerRecipient{
		{ClientID: "c1", PlayerID: quizObjectivesFanoutPlayerID},
	})
	if !ok {
		t.Fatalf("expected reduction to apply (PHASE=READY)")
	}
	data, present := payloads[quizObjectivesFanoutPlayerID]
	if !present {
		t.Fatalf("expected a payload for playerID %q", quizObjectivesFanoutPlayerID)
	}

	var parsed struct {
		Msg struct {
			Game map[string]interface{} `json:"GAME"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v (bytes: %s)", err, data)
	}
	if _, present := parsed.Msg.Game["QUIZ_OBJECTIVES"]; present {
		t.Errorf("buildVPlayerPayloads (fan-out chaud): QUIZ_OBJECTIVES should be stripped from the reduced VPlayer payload, got %v", parsed.Msg.Game["QUIZ_OBJECTIVES"])
	}
	if parsed.Msg.Game["QUIZ_NAME"] != "Quiz confidentiel" {
		t.Errorf("expected QUIZ_NAME to survive untouched (only QUIZ_OBJECTIVES is admin-only), got %v", parsed.Msg.Game["QUIZ_NAME"])
	}

	// Belt-and-braces: the raw bytes must not contain the secret value
	// anywhere in the payload — catches a leak via a field this test doesn't
	// know to look for (e.g. duplicated into another key by a future change).
	if strings.Contains(string(data), quizObjectivesFanoutSecret) {
		t.Errorf("buildVPlayerPayloads: the QUIZ_OBJECTIVES value leaked somewhere in the payload bytes: %s", data)
	}
}
