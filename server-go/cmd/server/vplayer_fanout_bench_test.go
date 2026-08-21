package main

import (
	"fmt"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// BenchmarkVPlayerFanout (#127 T2.4) — CPU cost of buildVPlayerPayloads, the
// core of the individualized VPlayer fan-out (T2.2/T2.3), for 10 and 30
// VPlayers, on a QCM (light) and a MEMOTION (heavy, ~11KB GAME.QUESTION)
// question. No WebSocketHub/network involved — see buildVPlayerPayloads'
// doc comment: it is deliberately factored out so this benchmark measures
// exactly the JSON work, not I/O.
//
// Plan checkpoint (_work/handoff/task-dev-backend-20260802-222654.md):
// "si le fan-out à 30 VJoueurs dépasse ~2ms CPU, ne continue pas T2.5 tel
// quel — envoie [BLOQUE] avec les chiffres du benchmark."
// ---------------------------------------------------------------------------

// buildFanoutBenchMessage constructs a representative UPDATE message (phase
// READY, n virtual bumpers split across 2 teams, plus the given question) —
// same shape as a.engine.GetGameJSON(), built directly (no live engine
// needed) so this benchmark has zero dependency on engine/wsHub setup cost.
func buildFanoutBenchMessage(n int, question *game.Question) *protocol.Message {
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}, ColorName: "rouge", Score: 15, Ready: true},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}, ColorName: "bleu", Score: 12, Ready: true},
	}
	bumpers := map[string]*game.Bumper{}
	for i := 0; i < n; i++ {
		teamName := "TeamA"
		if i%2 == 1 {
			teamName = "TeamB"
		}
		id := fmt.Sprintf("bench-vp-%02d", i)
		bumpers[id] = &game.Bumper{
			Name: fmt.Sprintf("Player%d", i), Team: teamName, Score: i * 3,
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
		MotionCardStates:         map[string]game.MotionCardState{},
		MotionCardTeams:          map[string]string{},
		MotionParticipatingTeams: []string{},
		MotionCurrentTeamColor:   []int{},
		VirtualPlayerCount:       n,
		VirtualPlayerLimit:       n,
		ArdoiseAnswers:           map[string]game.ArdoiseAnswer{},
		Question:                 question,
	}

	data, err := (&game.GameData{Game: state, Teams: teams, Bumpers: bumpers}).ToJSON()
	if err != nil {
		panic(err)
	}
	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		panic(err)
	}
	msg.Msg = data
	msg.Version = "bench"
	return msg
}

func buildFanoutBenchRecipients(n int) []server.VPlayerRecipient {
	recipients := make([]server.VPlayerRecipient, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("bench-vp-%02d", i)
		recipients = append(recipients, server.VPlayerRecipient{ClientID: id + "-client", PlayerID: id})
	}
	return recipients
}

func benchQCMQuestion() *game.Question {
	return &game.Question{
		ID: "bench-qcm", Question: "Quelle est la capitale de la France ?", Answer: "Paris",
		Type: game.QuestionTypeQCM, Category: "Géographie",
		TypedContent: game.TypedContent{
			QCMAnswers: &game.QCMAnswers{Red: "Paris", Green: "Lyon", Yellow: "Marseille", Blue: "Toulouse"},
			QCMCorrect: "RED",
		},
		Points: "10", Time: "20", Status: game.StatusReady,
	}
}

func benchMotionQuestion() *game.Question {
	cards := make([]game.MotionCard, 0, 24)
	for i := 0; i < 24; i++ {
		cards = append(cards, game.MotionCard{
			ID: fmt.Sprintf("mc-%d", i+1), RectoTheme: "Histoire de France",
			RectoImage: "/files/questions/motion/recto-42.jpg", Difficulty: 2,
			QuestionText:  "En quelle année a eu lieu la prise de la Bastille et quelles en furent les conséquences immédiates sur le plan politique ?",
			QuestionImage: "/files/questions/motion/question-42.jpg",
			AnswerText:    "1789 — début de la Révolution française, chute de l'Ancien Régime.",
			AnswerImage:   "/files/questions/motion/answer-42.jpg",
		})
	}
	return &game.Question{
		ID: "bench-motion", Question: "MEMOTION — Histoire de France", Type: game.QuestionTypeMemotion,
		Category: "Histoire", MotionCards: cards, MotionMode: "SOLO",
		Points: "5", Time: "0", Status: game.StatusReady,
	}
}

func runFanoutBenchmark(b *testing.B, n int, question *game.Question) {
	msg := buildFanoutBenchMessage(n, question)
	recipients := buildFanoutBenchRecipients(n)

	// Sanity check outside the timed loop: must actually take the reduction
	// path (ok=true) and produce one payload per recipient — a benchmark
	// silently measuring the "malformed input" fallback would be meaningless.
	payloads, ok := buildVPlayerPayloads(msg, recipients)
	if !ok {
		b.Fatalf("setup failed: message did not qualify for VPlayer reduction")
	}
	if len(payloads) != n {
		b.Fatalf("setup failed: expected %d payloads, got %d", n, len(payloads))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildVPlayerPayloads(msg, recipients)
	}
}

func BenchmarkVPlayerFanout_10_QCM(b *testing.B) { runFanoutBenchmark(b, 10, benchQCMQuestion()) }
func BenchmarkVPlayerFanout_10_MEMOTION(b *testing.B) {
	runFanoutBenchmark(b, 10, benchMotionQuestion())
}
func BenchmarkVPlayerFanout_30_QCM(b *testing.B) { runFanoutBenchmark(b, 30, benchQCMQuestion()) }
func BenchmarkVPlayerFanout_30_MEMOTION(b *testing.B) {
	runFanoutBenchmark(b, 30, benchMotionQuestion())
}

// BenchmarkVPlayerFanout_Baseline_30_MEMOTION measures the PRE-#127 shared
// path this replaces (one json.Marshal via SerializeForWebClient, reused
// as-is for every recipient — no per-recipient reduction) — the comparison
// point for "coût actuel" mentioned in the plan (T2.4).
func BenchmarkVPlayerFanout_Baseline_30_MEMOTION(b *testing.B) {
	msg := buildFanoutBenchMessage(30, benchMotionQuestion())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msg.SerializeForWebClient(); err != nil {
			b.Fatalf("SerializeForWebClient failed: %v", err)
		}
	}
}
