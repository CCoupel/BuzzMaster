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
	a.engine.OnPictoChange = func() { a.broadcastUpdate() }
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
func (a *App) handlePictoCanvasSync(clientID string) {
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
