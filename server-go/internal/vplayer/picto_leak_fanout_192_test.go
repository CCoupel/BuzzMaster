package vplayer

import (
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #192 (v13.0.0, LOT-2.1) — NON-FUITE de PICTO_WORDS sur le chemin CHAUD des tablettes (BuildPayloads),
// réimplémentation par destinataire du sérialiseur de référence (contracts/picto.md §10, réserve de revue
// LOT-1) — CRITICAL. Modèle : roue_leak_fanout_145_test.go. « Trois sites, une seule liste ».

var fanoutPictoWords192 = []string{"Éléphant", "Pont-levis"}

func buildPictoFanoutMsg192(t *testing.T, phase game.GamePhase) *protocol.Message {
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
			ID: "p1", Question: "Dessinez", Type: game.QuestionTypePicto, Category: "ANIMALS", Time: "90",
			TypedContent: game.TypedContent{PictoWords: fanoutPictoWords192},
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

func assertNoPictoSecret192(t *testing.T, who string, p []byte) {
	t.Helper()
	s := string(p)
	for _, frag := range append([]string{"PICTO_WORDS"}, fanoutPictoWords192...) {
		if strings.Contains(s, frag) {
			t.Errorf("%s : %q fuit sur le chemin tablette : %s", who, frag, s)
		}
	}
}

func TestPictoLeakFanout192_WordsAbsentForEveryRecipientAndPhase(t *testing.T) {
	recipients := []server.VPlayerRecipient{{ClientID: "c1", PlayerID: "p-1"}, {ClientID: "c2", PlayerID: "p-2"}}
	phases := []game.GamePhase{game.PhasePrepare, game.PhaseReady, game.PhaseCountdown, game.PhaseStarted, game.PhasePaused}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			payloads, ok := BuildPayloads(buildPictoFanoutMsg192(t, phase), recipients)
			if !ok {
				t.Fatalf("phase %s : BuildPayloads doit individualiser cette phase", phase)
			}
			if len(payloads) != len(recipients) {
				t.Fatalf("un payload par destinataire attendu, got %d", len(payloads))
			}
			for id, p := range payloads {
				assertNoPictoSecret192(t, "tablette "+id, p)
			}
		})
	}
}

// Phases non individualisées (STOPPED, REVEALED…) : le repli commun ne fuit pas non plus (§10 : `PICTO_WORDS` est
// retiré dans toutes les phases, REVEALED compris).
func TestPictoLeakFanout192_SharedFallback_WordsAbsent_AllLaterPhases(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStopped, game.PhaseRevealed} {
		t.Run(string(phase), func(t *testing.T) {
			data, err := buildPictoFanoutMsg192(t, phase).SerializeForVPlayerCommon()
			if err != nil {
				t.Fatal(err)
			}
			assertNoPictoSecret192(t, "repli commun/"+string(phase), data)
			for id := range map[string]bool{"p-1": true, "p-2": true} {
				d, err := buildPictoFanoutMsg192(t, phase).SerializeForVPlayer(id)
				if err != nil {
					t.Fatal(err)
				}
				assertNoPictoSecret192(t, "SerializeForVPlayer("+id+")/"+string(phase), d)
			}
		})
	}
}
