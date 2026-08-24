package main

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// ENTRACTE / ENTRACTE_CONFIG on the VPlayer "hot" fan-out path (v6.5.2,
// #119, contract ws-payload-serialization.md §"ENTRACTE / ENTRACTE_CONFIG":
// "idem, y compris sur le chemin chaud").
//
// buildVPlayerPayloads (main.go) bypasses SerializeForVPlayer entirely for
// PREPARE/READY phases — it extracts GAME as json.RawMessage and splices it
// into every recipient's frame verbatim (see stripVPlayerHiddenGameFields),
// stripping ONLY protocol.AdminOnlyGameFields. ENTRACTE/ENTRACTE_CONFIG are
// deliberately NOT in that list (they are not admin-only — the opposite of
// QUIZ_OBJECTIVES, vplayer_fanout_quiz_objectives_test.go's subject), so
// this test locks down that they survive this path unstripped — the
// "chemin oublié" a future change to AdminOnlyGameFields could otherwise
// silently regress without any test noticing.
// ---------------------------------------------------------------------------

const entracteFanoutPlayerID = "vp-entracte-fanout"

// buildEntracteFanoutMessage mirrors buildQuizObjectivesFanoutMessage
// (vplayer_fanout_quiz_objectives_test.go) — a real game.GameData/ToJSON
// message, this time with Entracte/EntracteConfig set to distinctive
// values, in PREPARE or READY phase to exercise the reduced hot path.
func buildEntracteFanoutMessage(t *testing.T, phase game.GamePhase) *protocol.Message {
	t.Helper()
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Score: 0, Ready: true},
	}
	bumpers := map[string]*game.Bumper{
		entracteFanoutPlayerID: {
			Name: "Alice", Team: "TeamA", Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true,
		},
	}
	state := &game.GameState{
		Phase:    phase,
		Entracte: true,
		EntracteConfig: game.EntracteConfig{
			Title: "Pause déjeuner", Subtitle: "Retour à 13h30",
			ImageIsCustom: true, PanelSize: 70, AnimPeriod: 8, AnimIntensity: 0,
		},
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

func TestBuildVPlayerPayloads_PreservesEntracte(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhasePrepare, game.PhaseReady} {
		t.Run(string(phase), func(t *testing.T) {
			msg := buildEntracteFanoutMessage(t, phase)

			payloads, ok := buildVPlayerPayloads(msg, []server.VPlayerRecipient{
				{ClientID: "c1", PlayerID: entracteFanoutPlayerID},
			})
			if !ok {
				t.Fatalf("expected reduction to apply (PHASE=%s)", phase)
			}
			data, present := payloads[entracteFanoutPlayerID]
			if !present {
				t.Fatalf("expected a payload for playerID %q", entracteFanoutPlayerID)
			}

			var parsed struct {
				Msg struct {
					Game map[string]interface{} `json:"GAME"`
				} `json:"MSG"`
			}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("payload is not valid JSON: %v (bytes: %s)", err, data)
			}

			if parsed.Msg.Game["ENTRACTE"] != true {
				t.Errorf("buildVPlayerPayloads (fan-out chaud, %s): expected ENTRACTE=true to reach VPlayer, got %v", phase, parsed.Msg.Game["ENTRACTE"])
			}
			cfg, ok := parsed.Msg.Game["ENTRACTE_CONFIG"].(map[string]interface{})
			if !ok {
				t.Fatalf("buildVPlayerPayloads (fan-out chaud, %s): expected ENTRACTE_CONFIG object, got %v", phase, parsed.Msg.Game["ENTRACTE_CONFIG"])
			}
			if cfg["TITLE"] != "Pause déjeuner" || cfg["PANEL_SIZE"] != float64(70) || cfg["ANIM_INTENSITY"] != float64(0) {
				t.Errorf("buildVPlayerPayloads (fan-out chaud, %s): ENTRACTE_CONFIG fields not preserved, got %v", phase, cfg)
			}
		})
	}
}
