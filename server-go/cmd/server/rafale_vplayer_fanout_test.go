package main

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Traversée du chemin de fan-out VPlayer "chaud" (buildVPlayerPayloads) par
// les 3 champs indicateur « équipe active » RAFALE (milestone v8.0.0 #16,
// issue #199 task 37, contrat rafale.md §8.2).
//
// buildVPlayerPayloads (main.go) réimplémente le filtrage de
// SerializeForVPlayer par concaténation d'octets, en extrayant le noeud GAME
// directement du message source — indépendamment de
// internal/protocol/messages.go (voir stripVPlayerHiddenGameFields, main.go).
// Le contrat §8.2 met en garde : sans vérification EXPLICITE de ce chemin
// séparé, l'indicateur pourrait fonctionner sur /tv et /anim (couvert par
// internal/protocol/rafale_vplayer_traversal_test.go) tout en restant vide
// sur VPlayer — panne asymétrique. Précédent direct :
// vplayer_fanout_quiz_objectives_test.go (#137 Batch 2b), même structure de
// test, même raison d'être ("chemin oublié").
//
// Résultat de l'analyse (dev-backend) : RAFALE_CURRENT_TEAM/_CURRENT_TEAM_COLOR/
// _PARTICIPATING_TEAMS ne figurent dans AUCUNE des deux listes d'exclusion
// (protocol.AdminOnlyGameFields, protocol.VPlayerOnlyGameFields) que
// stripVPlayerHiddenGameFields applique — ils traversent donc déjà ce chemin
// par construction. Ce test le PROUVE plutôt que de le supposer.
// ---------------------------------------------------------------------------

const rafaleFanoutPlayerID = "vp-rafale-team-indicator"

// buildRafaleTeamFieldsFanoutMessage builds an UPDATE message via the real
// game.GameData/ToJSON path (not a hand-rolled map) whose GAME node carries
// the 3 RAFALE team-indicator fields set to distinctive, non-zero values —
// mirrors buildQuizObjectivesFanoutMessage's construction discipline
// (vplayer_fanout_quiz_objectives_test.go): the slice/map fields GameState
// needs non-nil to serialize safely are all initialized.
func buildRafaleTeamFieldsFanoutMessage(t *testing.T, phase game.GamePhase) *protocol.Message {
	t.Helper()
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Score: 0, Ready: true},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}, ColorName: "bleu", Score: 0, Ready: true},
	}
	bumpers := map[string]*game.Bumper{
		rafaleFanoutPlayerID: {
			Name: "Alice", Team: "TeamA", Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true,
		},
	}
	state := &game.GameState{
		Phase:                    phase,
		RafaleCurrentTeam:        "TeamA",
		RafaleCurrentTeamColor:   []int{255, 0, 0},
		RafaleParticipatingTeams: []string{"TeamA", "TeamB"},
		RafaleTeamCounters:       map[string]int{"TeamA": 3, "TeamB": 1},
		RafaleTeamBest:           map[string]int{},
		RafaleCurrentQuestion:    game.RafaleCurrent{},
		MemoryFlippedCards:       []string{},
		MemoryMatchedPairs:       []int{},
		MemoryTeamPairs:          map[string]int{},
		MemoryTeamErrors:         map[string]int{},
		MemoryParticipatingTeams: []string{},
		MemoryPairOwners:         map[int]string{},
		MemoryCurrentTeamColor:   []int{},
		QcmInvalidated:           []string{},
		MotionCardStates:         map[string]game.MotionCardState{},
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

// assertRafaleTeamFieldsInFanoutPayload is the hot-path counterpart of
// internal/protocol's assertRafaleTeamFieldsPresent — same 3 fields, same
// expected values, different (byte-concatenated) source.
func assertRafaleTeamFieldsInFanoutPayload(t *testing.T, gameNode map[string]interface{}) {
	t.Helper()
	if gameNode["RAFALE_CURRENT_TEAM"] != "TeamA" {
		t.Errorf("RAFALE_CURRENT_TEAM missing or wrong in fan-out payload, got %v (node: %v)", gameNode["RAFALE_CURRENT_TEAM"], gameNode)
	}
	if _, ok := gameNode["RAFALE_CURRENT_TEAM_COLOR"]; !ok {
		t.Errorf("RAFALE_CURRENT_TEAM_COLOR missing in fan-out payload (node: %v)", gameNode)
	}
	if _, ok := gameNode["RAFALE_PARTICIPATING_TEAMS"]; !ok {
		t.Errorf("RAFALE_PARTICIPATING_TEAMS missing in fan-out payload (node: %v)", gameNode)
	}
}

// TestBuildVPlayerPayloads_RafaleTeamFields_Survive_ReducedPath covers the
// per-recipient reduction (PREPARE/READY — the phases buildVPlayerPayloads
// actually reduces; see its own ok=false fallback for every other phase).
func TestBuildVPlayerPayloads_RafaleTeamFields_Survive_ReducedPath(t *testing.T) {
	msg := buildRafaleTeamFieldsFanoutMessage(t, game.PhaseReady)

	payloads, ok := buildVPlayerPayloads(msg, []server.VPlayerRecipient{
		{ClientID: "c1", PlayerID: rafaleFanoutPlayerID},
	})
	if !ok {
		t.Fatalf("expected reduction to apply (PHASE=READY)")
	}
	data, present := payloads[rafaleFanoutPlayerID]
	if !present {
		t.Fatalf("expected a payload for playerID %q", rafaleFanoutPlayerID)
	}

	var parsed struct {
		Msg struct {
			Game map[string]interface{} `json:"GAME"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v (bytes: %s)", err, data)
	}
	assertRafaleTeamFieldsInFanoutPayload(t, parsed.Msg.Game)
}
