package main

import (
	"encoding/json"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// ROUE « Roue de la Fortune » (v12.0.0, #144, contracts/roue.md §8, §12):
// application-layer wiring of the engine — actions, broadcast, LED, ambiance
// and the existing sounds. Kept out of main.go on purpose: the closed
// registries of LED / ambiance / NotifyPulse sites scan main.go.

// setupRoueCallbacks wires the engine's ROUE callbacks. After every
// transition (action, buzz, timer) the UPDATE is broadcast and the LED /
// ambiance follow the team that has the hand; notices carry the existing
// sounds and effects only.
func (a *App) setupRoueCallbacks() {
	a.engine.RoueWheelParamsFn = func() game.RoueWheelParams { // game-config.json `wheel`, read at each START
		w := config.GetGameSettings().Wheel
		return game.RoueWheelParams{
			MaxPoints: w.MaxPoints, Step: w.Step, BankruptCount: w.BankruptCount, LoseTurnCount: w.LoseTurnCount, BonusCount: w.BonusCount, BonusMode: w.BonusMode,
			SpinPeriodMs: w.SpinPeriodMs, SpinMaxMs: w.SpinMaxMs, StopDurationMs: w.StopDurationMs, FlipIntervalMs: w.FlipIntervalMs,
		}
	}
	a.engine.OnRoueChange = func() {
		a.broadcastUpdate()
		a.ledCtl().RoueTeams()
		a.ambiance().NotifyState()
	}
	a.engine.OnRoueNotice = func(n game.RoueNotice) {
		switch n.Kind {
		case game.RoueNoticeBad:
			a.notifySound(audio.CueRafaleMauvaise) // existing "mauvaise réponse" cue
		case game.RoueNoticeCredit:
			a.sendLEDSetComet(n.Team)
			a.ambiance().NotifyPulse(lighting.KindScore, []string{n.Team}, n.Points, lighting.ScorePulseDuration)
			a.notifySound(audio.CueGagne)
			a.startScoreFlash(n.Team, n.Points)
			a.broadcastAwardedTeams()
		case game.RoueNoticeBonus: // #308: no points, so neither score flash nor ambiance pulse
			a.sendLEDSetComet(n.Team)
			a.notifySound(audio.CueRafaleBonne) // existing "bonne réponse" cue
		}
	}
	a.engine.RoueCategoryMeta = func(category string) game.CategoryMeta {
		name, image, color := a.httpServer.ResolveCategoryMeta(category)
		return game.CategoryMeta{Name: name, ImageURL: image, Color: color}
	}
}

// handleRoueAction processes the ROUE_* actions (v12.0.0, #144, contract
// roue.md §8). The engine applies the first valid action for the current
// sub-phase and refuses the next (admin and animateur may both command); a
// refusal answers ROUE_ERROR to the emitter only, state unchanged. The UPDATE
// broadcast and the LED / ambiance refresh come from Engine.OnRoueChange.
func (a *App) handleRoueAction(clientID string, msg *protocol.Message) {
	var err error
	switch msg.Action {
	case protocol.ActionRoueSetTeams:
		var p protocol.RoueSetTeamsPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.RoueError{Code: game.RoueErrInvalidTeams, Message: "payload invalide"}
		} else {
			err = a.engine.RoueSetTeams(p.Teams)
		}
	case protocol.ActionRoueSpin:
		err = a.engine.RoueSpin()
	case protocol.ActionRoueStop:
		err = a.engine.RoueStop()
	case protocol.ActionRoueLetter:
		var p protocol.RoueLetterPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.RoueError{Code: game.RoueErrNotALetter, Message: "payload invalide"}
		} else {
			err = a.engine.RoueLetter(p.Letter)
		}
	case protocol.ActionRoueBuyVowel:
		var p protocol.RoueBuyVowelPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.RoueError{Code: game.RoueErrNotALetter, Message: "payload invalide"}
		} else {
			err = a.engine.RoueBuyVowel(p.Letter)
		}
	case protocol.ActionRoueFlip:
		err = a.engine.RoueFlip()
	case protocol.ActionRoueAnswer:
		err = a.engine.RoueAnswer()
	case protocol.ActionRoueNext:
		var p protocol.RoueNextPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.RoueError{Code: game.RoueErrNotAllowed, Message: "payload invalide"}
		} else {
			err = a.engine.RoueNext(p.Hand)
		}
	case protocol.ActionRouePass:
		err = a.engine.RoucePass()
	case protocol.ActionRoueReveal:
		err = a.engine.RoueReveal()
	}
	if err == nil {
		server.LogInfo(game.LogComponentEngine, "%s", msg.Action)
		return
	}
	code, text := game.RoueErrNotAllowed, err.Error()
	if re, ok := err.(*game.RoueError); ok {
		code, text = re.Code, re.Message
	}
	server.LogWarn(game.LogComponentEngine, "%s refused: %s (%s)", msg.Action, code, text)
	if clientID == "" {
		return
	}
	if rm, merr := protocol.NewMessage(protocol.ActionRoueError, protocol.RoueErrorPayload{Code: code, Message: text}); merr == nil {
		a.wsHub.SendToClient(clientID, rm)
	}
}
