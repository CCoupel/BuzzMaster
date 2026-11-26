package main

import (
	"encoding/json"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// PICTO (v13.0.0, #192/#193, contracts/picto.md): application-layer wiring of the engine — preparation
// actions, secret word handover, sheet relay and catch-up. Kept out of main.go on purpose (closed registries
// of LED / ambiance / NotifyPulse sites scan main.go). LED, ambiance and sounds come with LOT-2.2.

// setupPictoCallbacks wires the engine's PICTO callbacks and creates the sheet journal.
func (a *App) setupPictoCallbacks() {
	a.pictoCanvas = server.NewPictoCanvas()
	a.engine.OnPictoChange = func() {
		a.broadcastUpdate()
		a.ledCtl().PictoTeams() // team chosen: its buzzers SOLID, the others DIM (§15)
		a.ambiance().NotifyState()
	}
	a.engine.OnPictoCanvasReset = func(epoch int) { a.pictoCanvas.Reset(epoch) }
	a.engine.OnPictoWord = a.sendPictoWord
}

func pictoWordMessage(w game.PictoWordInfo) *protocol.Message {
	m, err := protocol.NewMessage(protocol.ActionPictoWord, protocol.PictoWordPayload{Word: w.Word, Category: w.Category, Epoch: w.Epoch})
	if err != nil {
		return nil
	}
	return m
}

// sendPictoWord hands the secret word of the round over: to the drawer's tablet alone (SendToPlayerID) and to
// admin + anim ONLY (BroadcastToTypes). NEVER to the TV, the other tablets or the buzzers, and never through
// GameState (contracts/picto.md §2.3, §10) — do not widen this recipient list.
func (a *App) sendPictoWord(w game.PictoWordInfo) {
	m := pictoWordMessage(w)
	if m == nil {
		return
	}
	if w.DrawerID != "" {
		a.wsHub.SendToPlayerID(w.DrawerID, m)
	}
	a.wsHub.BroadcastToTypes(m, server.ClientTypeAdmin, server.ClientTypeAnim)
}

func (a *App) sendPictoError(clientID string, perr *game.PictoError) {
	if clientID == "" || perr == nil {
		return
	}
	if m, err := protocol.NewMessage(protocol.ActionPictoError, protocol.PictoErrorPayload{Code: perr.Code, Message: perr.Message}); err == nil {
		a.wsHub.SendToClient(clientID, m)
	}
}

// handlePictoPrepAction processes PICTO_SET_TEAM / PICTO_SET_DRAWER / PICTO_DRAW_DRAWER (admin and anim).
// A refusal answers PICTO_ERROR to the emitter alone, state unchanged; the UPDATE broadcast comes from
// Engine.OnPictoChange.
func (a *App) handlePictoPrepAction(clientID string, msg *protocol.Message) {
	var err error
	switch msg.Action {
	case protocol.ActionPictoSetTeam:
		var p protocol.PictoSetTeamPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.PictoError{Code: game.PictoErrInvalidTeam, Message: "payload invalide"}
		} else {
			err = a.engine.PictoSetTeam(p.Team)
		}
	case protocol.ActionPictoSetDrawer:
		var p protocol.PictoSetDrawerPayload
		if jerr := json.Unmarshal(msg.Msg, &p); jerr != nil {
			err = &game.PictoError{Code: game.PictoErrNotACandidate, Message: "payload invalide"}
		} else {
			err = a.engine.PictoSetDrawer(p.Player)
		}
	case protocol.ActionPictoDrawDrawer:
		err = a.engine.PictoDrawDrawer()
	}
	if err == nil {
		server.LogInfo(game.LogComponentEngine, "%s", msg.Action)
		return
	}
	perr, ok := err.(*game.PictoError)
	if !ok {
		perr = &game.PictoError{Code: game.PictoErrWrongPhase, Message: err.Error()}
	}
	server.LogWarn(game.LogComponentEngine, "%s refused: %s (%s)", msg.Action, perr.Code, perr.Message)
	a.sendPictoError(clientID, perr)
}

// pictoStepPayload reads the optional EPOCH of PICTO_FOUND / PICTO_PASS ("EPOCH" absent = nil).
func pictoStepPayload(msg *protocol.Message) *int {
	var p protocol.PictoStepPayload
	if msg != nil && len(msg.Msg) > 0 {
		_ = json.Unmarshal(msg.Msg, &p)
	}
	return p.Epoch
}

// pictoStepRefused answers a refused PICTO_FOUND / PICTO_PASS with PICTO_ERROR to the emitter alone.
func (a *App) pictoStepRefused(clientID, action string, err error) {
	perr, ok := err.(*game.PictoError)
	if !ok {
		perr = &game.PictoError{Code: game.PictoErrWrongPhase, Message: err.Error()}
	}
	server.LogWarn(game.LogComponentEngine, "%s refused: %s (%s)", action, perr.Code, perr.Message)
	a.sendPictoError(clientID, perr)
}

// pictoAfterStep runs the application-layer side effects of an accepted PICTO_FOUND / PICTO_PASS from the phase
// the engine ended in: REVEALED (a PICTO "Trouvé", or the gong answer in Rafale Picto) = same side effects as
// STOP then REVEAL — the question sound and the countdown sound end, the REVEAL broadcast (LED, ambiance, sound);
// STOPPED (POOL_END) = the question sound ends and the next question may now differ; STARTED / PAUSED = the round
// goes on, the engine already fired PICTO_WORD, the sheet reset and the UPDATE.
func (a *App) pictoAfterStep(res game.PictoStepResult) {
	switch res.Phase {
	case game.PhaseRevealed:
		a.questionSound().Stop()
		a.countdownSound().Stop()
		a.broadcastReveal("") // the revealed word rides GameState (PICTO_REVEALED_WORD), not this message
		a.broadcastNextQuestion()
	case game.PhaseStopped:
		a.questionSound().Stop()
		a.countdownSound().Stop()
		a.broadcastNextQuestion()
	}
}

// handlePictoFound processes PICTO_FOUND (admin and anim). PICTO (one word): « Trouvé ! » stops the round and
// reveals it in the same action (contracts/picto.md §6.4). RAFALE_PICTO: the word is counted and the round goes
// on with the next word (§17.5); EPOCH is mandatory. A refusal (WRONG_PHASE, STALE_WORD) answers PICTO_ERROR to
// the emitter alone — when admin and anim both click, the first applies and the second is refused, never two
// outcomes.
func (a *App) handlePictoFound(clientID string, msg *protocol.Message) {
	res, err := a.engine.PictoFoundAt(pictoStepPayload(msg))
	if err != nil {
		a.pictoStepRefused(clientID, "PICTO_FOUND", err)
		return
	}
	server.LogInfo(game.LogComponentEngine, "PICTO_FOUND")
	a.pictoAfterStep(res)
}

// handlePictoPass processes PICTO_PASS (admin and anim only — the drawer announces the pass aloud, no tablet
// action exists; the inbound allowlist already refuses any other client type). RAFALE_PICTO only (NOT_RAFALE
// otherwise), STARTED only, EPOCH mandatory, PASS_LIMIT_REACHED when no pass is left.
func (a *App) handlePictoPass(clientID string, msg *protocol.Message) {
	res, err := a.engine.PictoPass(pictoStepPayload(msg))
	if err != nil {
		a.pictoStepRefused(clientID, "PICTO_PASS", err)
		return
	}
	server.LogInfo(game.LogComponentEngine, "PICTO_PASS")
	a.pictoAfterStep(res)
}

// pictoStartRefused is the START guard of a PICTO round (contracts/picto.md §6.1): without a designated and
// connected drawer the emitter gets PICTO_ERROR NO_DRAWER and START is not forwarded to the engine.
func (a *App) pictoStartRefused(clientID string) bool {
	perr := a.engine.PictoStartCheck()
	if perr == nil {
		return false
	}
	server.LogWarn(game.LogComponentEngine, "START refused: %s (%s)", perr.Code, perr.Message)
	a.sendPictoError(clientID, perr)
	return true
}

// handlePictoStrokes processes PICTO_STROKES from a VPlayer (contracts/picto.md §8). The guard takes the
// identity from the CONNECTION (type and bound player ID), never from the payload. An accepted frame is
// journaled then relayed best-effort to tv / admin / anim — NEVER to the tablets, never through an UPDATE.
func (a *App) handlePictoStrokes(incoming *protocol.IncomingMessage) {
	var p protocol.PictoStrokesPayload
	if err := json.Unmarshal(incoming.Data.Msg, &p); err != nil {
		a.sendPictoError(incoming.ClientID, &game.PictoError{Code: game.PictoErrInvalidStrokes, Message: "trame illisible"})
		return
	}
	boundID, connected := a.wsHub.ClientBinding(incoming.ClientID)
	if !connected {
		boundID = incoming.BoundPlayerID
	}
	drawer, phase, epoch, isPicto := a.engine.PictoStrokeContext()
	relay, perr := a.pictoCanvas.Accept(incoming.ClientID, p, server.PictoStrokeContext{
		ClientType: server.ClientType(incoming.ClientType), PlayerID: boundID,
		Drawer: drawer, Phase: phase, Epoch: epoch, IsPicto: isPicto, Now: time.Now(),
	})
	if perr != nil {
		a.sendPictoError(incoming.ClientID, perr)
		return
	}
	if relay == nil {
		return // stale epoch or rate-limited frame already reported: ignored in silence
	}
	m, err := protocol.NewMessage(protocol.ActionPictoStrokes, relay)
	if err != nil {
		return
	}
	a.wsHub.TrySendToTypes(m, server.ClientTypeTV, server.ClientTypeAdmin, server.ClientTypeAnim)
}

// pictoCanvasMessage builds PICTO_CANVAS: the whole sheet of the epoch and the current SEQ.
func (a *App) pictoCanvasMessage() *protocol.Message {
	m, err := protocol.NewMessage(protocol.ActionPictoCanvas, a.pictoCanvas.Snapshot())
	if err != nil {
		return nil
	}
	return m
}

// handlePictoCanvasSync answers PICTO_CANVAS_SYNC to the requester alone (an empty sheet outside a round).
// Contract §9.1: open to tv / admin / anim, and to a vplayer only if it is the drawer (its connection's bound
// player, never the payload); a non-drawer gets PICTO_ERROR NOT_DRAWER. Rate-limited per connection: each
// call serialises the whole sheet on the single consumer of the hub's Incoming channel (#127).
func (a *App) handlePictoCanvasSync(incoming *protocol.IncomingMessage) {
	clientID := incoming.ClientID
	if server.ClientType(incoming.ClientType) == server.ClientTypeVPlayer {
		boundID, connected := a.wsHub.ClientBinding(clientID)
		if !connected {
			boundID = incoming.BoundPlayerID
		}
		drawer, _, _, _ := a.engine.PictoStrokeContext()
		if boundID == "" || boundID != drawer {
			a.sendPictoError(clientID, &game.PictoError{Code: game.PictoErrNotDrawer, Message: "seul le dessinateur peut demander la feuille"})
			return
		}
	}
	allowed, report := a.pictoCanvas.AllowSync(clientID, time.Now())
	if !allowed {
		if report {
			a.sendPictoError(clientID, &game.PictoError{Code: game.PictoErrRateLimited, Message: "trop de demandes de feuille par seconde"})
		}
		return
	}
	if m := a.pictoCanvasMessage(); m != nil {
		a.wsHub.SendToClient(clientID, m)
	}
}

// sendPictoToClient is the PICTO part of the initial state of a (re)connecting client (contracts/picto.md
// §6.7): during a round, a TV / admin / anim receives the sheet (PICTO_CANVAS), admin and anim also the secret
// word (PICTO_WORD). The drawer's tablet is served by sendPictoToPlayer once its connection is bound.
func (a *App) sendPictoToClient(clientID string, clientType server.ClientType) {
	if _, ok := a.engine.PictoWord(); !ok {
		return
	}
	switch clientType {
	case server.ClientTypeTV, server.ClientTypeAdmin, server.ClientTypeAnim:
		if m := a.pictoCanvasMessage(); m != nil {
			a.wsHub.SendToClient(clientID, m)
		}
	}
	if clientType == server.ClientTypeAdmin || clientType == server.ClientTypeAnim {
		if w, ok := a.engine.PictoWord(); ok {
			if m := pictoWordMessage(w); m != nil {
				a.wsHub.SendToClient(clientID, m)
			}
		}
	}
}

// sendPictoToPlayer re-sends the word and the sheet to a VPlayer that just (re)bound its connection, if it is
// the drawer of the round in progress (§6.7): it picks its sheet and its word back up.
func (a *App) sendPictoToPlayer(playerID string) {
	w, ok := a.engine.PictoWord()
	if !ok || w.DrawerID != playerID {
		return
	}
	if m := pictoWordMessage(w); m != nil {
		a.wsHub.SendToPlayerID(playerID, m)
	}
	if m := a.pictoCanvasMessage(); m != nil {
		a.wsHub.SendToPlayerID(playerID, m)
	}
}
