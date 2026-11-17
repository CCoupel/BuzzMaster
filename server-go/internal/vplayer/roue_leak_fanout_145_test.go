package vplayer

import (
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #145 (v12.0.0, LOT-1.1) — NON-FUITE de la phrase ROUE sur le chemin CHAUD des tablettes (BuildPayloads),
// réimplémentation par destinataire du sérialiseur de référence (contracts/roue.md §9) — CRITICAL.
// « Trois sites, une seule liste » : un site oublié laisse la fuite ouverte.

const fanoutPhrase145 = "Combien pour ce chien dans la vitrine"

func buildRoueFanoutMsg145(t *testing.T, phase game.GamePhase) *protocol.Message {
	t.Helper()
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Ready: true},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}, ColorName: "bleu", Ready: true},
	}
	bumpers := map[string]*game.Bumper{
		"p-1": {Name: "Alice", Team: "TeamA", Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true},
		"p-2": {Name: "Bob", Team: "TeamB", Connected: true, IsVirtual: true, IsVPlayer: true, Ready: true},
	}
	state := &game.GameState{
		Phase: phase,
		Question: &game.Question{
			ID: "r1", Question: "Manche roue", Type: game.QuestionTypeRoue,
			TypedContent: game.TypedContent{RouePhrase: fanoutPhrase145},
		},
		RafaleParticipatingTeams: []string{}, RafaleTeamCounters: map[string]int{}, RafaleTeamBest: map[string]int{},
		MemoryFlippedCards: []string{}, MemoryMatchedPairs: []int{}, MemoryTeamPairs: map[string]int{},
		MemoryTeamErrors: map[string]int{}, MemoryParticipatingTeams: []string{}, MemoryPairOwners: map[int]string{},
		MemoryCurrentTeamColor: []int{}, QcmInvalidated: []string{}, MotionCardStates: map[string]game.MotionCardState{},
		MotionCardTeams: map[string]string{}, MotionParticipatingTeams: []string{}, MotionCurrentTeamColor: []int{},
		ArdoiseAnswers: map[string]game.ArdoiseAnswer{},
	}
	data, err := (&game.GameData{Game: state, Teams: teams, Bumpers: bumpers}).ToJSON()
	if err != nil {
		t.Fatalf("GameData.ToJSON: %v", err)
	}
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	msg.Msg = data
	msg.Version = "test"
	return msg
}

func TestRoueLeakFanout145_PhraseAbsentForEveryRecipientAndPhase(t *testing.T) {
	recipients := []server.VPlayerRecipient{{ClientID: "c1", PlayerID: "p-1"}, {ClientID: "c2", PlayerID: "p-2"}}
	for _, phase := range []game.GamePhase{game.PhasePrepare, game.PhaseReady, game.PhaseCountdown, game.PhaseStarted, game.PhasePaused} {
		t.Run(string(phase), func(t *testing.T) {
			payloads, ok := BuildPayloads(buildRoueFanoutMsg145(t, phase), recipients)
			if !ok {
				t.Skip("phase non individualisée : le repli (SerializeForVPlayerCommon) est couvert dans internal/protocol")
			}
			if len(payloads) == 0 {
				t.Fatal("aucun payload construit")
			}
			for id, p := range payloads {
				s := string(p)
				if strings.Contains(s, "ROUE_PHRASE") || strings.Contains(s, fanoutPhrase145) || strings.Contains(s, "VITRINE") || strings.Contains(s, "vitrine") {
					t.Errorf("tablette %s : la phrase fuit sur le chemin chaud : %s", id, s)
				}
			}
		})
	}
}
