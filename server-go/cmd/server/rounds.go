package main

import (
	"buzzcontrol/internal/diag"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/rounds"
	"buzzcontrol/internal/server"
	"encoding/json"
)

// Rounds (v11.2, #245) — App side. The logic lives in internal/rounds (#291, Phase B).

func (a *App) questionsDirPath() string {
	d := a.config.Storage.QuestionsDir
	if d == "" {
		d = "./data/files/questions"
	}
	return d
}

// reloadRounds re-reads rounds.json into GameState.ROUNDS (startup, restore).
func (a *App) reloadRounds() { rounds.ReloadRounds(a) }

// currentRounds returns the rounds as stored in rounds.json (see rounds.CurrentRounds).
func (a *App) currentRounds() []game.Round { return rounds.CurrentRounds(a) }

// orderedQuestions returns the questions of the quiz in THE play order (rounds + ORDER).
func (a *App) orderedQuestions() []server.OrderedQuestion {
	raw := make([]map[string]interface{}, 0)
	for _, q := range a.loadQuestions() {
		raw = append(raw, q)
	}
	return server.OrderQuestions(raw, a.currentRounds())
}

// computeRoundContext derives GameState.ROUND_CONTEXT (see rounds.ComputeRoundContext).
func computeRoundContext(ordered []server.OrderedQuestion, rs []game.Round, currentID string) game.RoundContext {
	return rounds.ComputeRoundContext(ordered, rs, currentID)
}

// refreshRoundContext recomputes ROUND_CONTEXT for the engine's current question.
func (a *App) refreshRoundContext() {
	st := a.engine.GetState()
	id := ""
	if st.Question != nil {
		id = st.Question.ID
	}
	a.engine.SetRoundContext(computeRoundContext(a.orderedQuestions(), a.currentRounds(), id))
}

// applyRoundsUpdate validates the WHOLE request, then applies it (see rounds.ApplyUpdate).
func (a *App) applyRoundsUpdate(req protocol.RoundsUpdatePayload) (protocol.RoundsUpdateResultPayload, string) {
	return rounds.ApplyUpdate(a, req)
}

func (a *App) handleRoundsUpdate(clientID string, msg *protocol.Message) {
	var req protocol.RoundsUpdatePayload
	reply := func(res protocol.RoundsUpdateResultPayload) {
		if rm, err := protocol.NewMessage(protocol.ActionRoundsUpdateResult, res); err == nil && clientID != "" {
			a.wsHub.SendToClient(clientID, rm)
		}
	}
	if err := json.Unmarshal(msg.Msg, &req); err != nil {
		server.LogWarn(game.LogComponentApp, "ROUNDS_UPDATE_REFUSED client=%q reason=%q", clientID, "INVALID_NAME")
		reply(rounds.RoundResult("REFUSED", "INVALID_NAME"))
		return
	}
	res, line := a.applyRoundsUpdate(req)
	if res.Status == "REFUSED" && res.Reason == "WRITE_FAILED" {
		server.LogWarn(game.LogComponentApp, "ROUNDS_UPDATE_ROLLBACK client=%q files=%s", clientID, diag.LogValue(res.Questions))
	}
	if res.Status == "REFUSED" {
		server.LogWarn(game.LogComponentApp, "ROUNDS_UPDATE_REFUSED client=%q reason=%q questions=%s", clientID, res.Reason, diag.LogValue(res.Questions))
		reply(res)
		return
	}
	server.LogInfo(game.LogComponentApp, "ROUNDS_UPDATE client=%q %s", clientID, line)
	a.refreshRoundContext()
	a.broadcastUpdate()
	a.broadcastQuestions()
	a.broadcastNextQuestion()
	reply(res)
}
