// Package led drives the LED_SET frames sent to the physical buzzers (v3.4.0+,
// docs/LED_SET_PROTOCOL.md): per-phase rules, buzz states, COMET, ACK-tracked
// sends and the reconnect replay. It reaches the application through the
// narrow Host interface. Extracted from cmd/server (#291/#295, LOT-4).
package led

import (
	"strings"
	"sync"
	"time"

	"buzzcontrol/internal/colors"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// Host is what the LED controller needs from the application.
type Host interface {
	Engine() *game.Engine
	BuzzerHub() *server.BuzzerWebSocketHub
	AckManager() *server.AckManager
	BroadcastUpdateTo(types ...server.ClientType)
}

// Controller owns the LED rules and sends. The last-sent-payload and buzz-state
// maps and their mutex are owned by the App (tests and the HELLO replay read
// them directly) and shared here by reference: ledStateMu guards
// bumperLEDState (sendLEDSet runs from several goroutines); bumperBuzzState is
// only touched from the single dispatch goroutine (same discipline as before).
type Controller struct {
	h               Host
	ledStateMu      *sync.RWMutex
	bumperLEDState  *map[string]protocol.LEDSetPayload
	bumperBuzzState *map[string]game.BuzzState
}

// New builds a Controller over the given Host and shared state.
func New(h Host, mu *sync.RWMutex, ledState *map[string]protocol.LEDSetPayload, buzzState *map[string]game.BuzzState) *Controller {
	return &Controller{h: h, ledStateMu: mu, bumperLEDState: ledState, bumperBuzzState: buzzState}
}

// TeamColor returns the LED RGB color for a bumper's team, or gray [128,128,128] if no team.
// Resolution order:
//  1. team.ColorName (explicit name set by frontend) → direct palette lookup
//  2. Hue-based nearest palette color (more robust than Euclidean distance)
func (c *Controller) TeamColor(bumper *game.Bumper) [3]int {
	return c.TeamNameColor(bumper.Team)
}

// TeamNameColor is the shared trunk of teamColorToRGB: team NAME → palette
// RGB (game.Team has no ID). Factored out for the ambiance scene table (#205,
// contracts/lighting.md §8), which only knows team names and must show the
// SAME colour as the buzzers — never a second palette. Gray {128,128,128}
// for no/unknown team, unchanged.
func (c *Controller) TeamNameColor(teamName string) [3]int {
	if teamName == "" {
		return [3]int{128, 128, 128}
	}
	team := c.h.Engine().GetTeam(teamName)
	if team == nil {
		return [3]int{128, 128, 128}
	}
	// 1. Named color lookup (explicit, highest priority)
	if team.ColorName != "" {
		if rgb, ok := colors.TeamColorPalette[strings.ToLower(team.ColorName)]; ok {
			return rgb
		}
	}
	// 2. Hue-based nearest palette color
	if len(team.Color) < 3 {
		return [3]int{128, 128, 128}
	}
	return colors.NearestPaletteColorByHue(team.Color[0], team.Color[1], team.Color[2])
}

// Send sends a LED_SET message to a specific buzzer and stores the state for reconnect.
// A MSG_ID is attached so the buzzer can ACK receipt (v3.8.0 ACK protocol).
func (c *Controller) Send(mac string, payload protocol.LEDSetPayload) {
	c.ledStateMu.Lock()
	(*c.bumperLEDState)[mac] = payload
	c.ledStateMu.Unlock()
	if !c.h.BuzzerHub().IsClientConnected(mac) {
		// #109 Phase 2 (D4): a LED_SET that should have reached this buzzer while
		// it's disconnected counts as a lost message (orange -> red).
		c.h.Engine().TransitionConn(mac, game.ConnEventMessageLost)
		return
	}
	msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to create LED_SET message for %s: %v", mac, err)
		return
	}
	// Attach MSG_ID for ACK tracking
	msgID := server.GenerateMsgID()
	msg.MsgID = msgID

	if err := c.h.BuzzerHub().SendToClient(mac, msg); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to send LED_SET to buzzer %s: %v", mac, err)
	} else {
		server.LogInfo(game.LogComponentApp, "LED_SET sent to %s (msgID=%s): RGB(%d,%d,%d) intensity=%d effect=%s",
			mac, msgID, payload.Color[0], payload.Color[1], payload.Color[2], payload.Intensity, payload.Effect)
		// Register for ACK tracking; set AckPending on bumper.
		// NOTE: broadcastUpdate() is intentionally NOT called here — callers are responsible for
		// a single broadcast after the full loop completes, avoiding N redundant UPDATE messages.
		c.h.AckManager().Register(mac, msgID, protocol.ActionLEDSet)
		c.h.Engine().UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": true})
	}
}

// Broadcast sends the same LED_SET payload to all connected buzzers.
//
// #132 audit: currently DEAD CODE — grep confirms zero call sites anywhere
// in the codebase (including tests). Fixed anyway, for the same reason
// dead code still gets its imports checked: if this is ever wired up, it
// should not silently reintroduce the exact #127/#129 bug class. Flagged
// separately in the audit report as its own finding (candidate for removal
// — not done here, out of this bugfix's scope).
func (c *Controller) Broadcast(payload protocol.LEDSetPayload) {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		c.Send(mac, payload)
	}
	// One broadcast for all AckPending state changes set by the loop above.
	// #132 (same pattern/justification as sendLEDSetAllBuzzers #127/#129-T1.6
	// and sendLEDSetPause #129 T3.1-adjacent): the loop above unconditionally
	// skips IsVPlayer bumpers, and ACK_PENDING — the only field this call
	// exists to announce — is always stripped from TV's payload too
	// (protocol.AdminOnlyBumperFields, via SerializeForWebClient). Neither
	// client type can ever see anything this call carries.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// ResendOnReconnect re-sends the last known LED state to a buzzer that just reconnected (HELLO).
func (c *Controller) ResendOnReconnect(mac string) {
	c.ledStateMu.RLock()
	payload, ok := (*c.bumperLEDState)[mac]
	c.ledStateMu.RUnlock()
	if !ok {
		// No stored state — derive from current game state
		c.sendLEDSetForBuzzer(mac)
		return
	}
	msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
	if err != nil {
		return
	}
	if err := c.h.BuzzerHub().SendToClient(mac, msg); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to resend LED_SET to %s: %v", mac, err)
	} else {
		server.LogInfo(game.LogComponentApp, "LED_SET resent on reconnect to %s", mac)
	}
}

// ResetBuzzStates resets BuzzState to NONE for all known buzzers.
// Called at the start of each question round (READY phase).
func (c *Controller) ResetBuzzStates() {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac := range tb.Bumpers {
		(*c.bumperBuzzState)[mac] = game.BuzzStateNone
	}
}

// UpdateBuzzStates updates BuzzState for all buzzers after buzzingMAC has pressed.
//
// Rules (from BUZZER_LED_STATE_MACHINE.md):
//   - The buzzing bumper becomes MOI (first buzz of its team).
//   - All other bumpers of the same team become EQUIPE.
//   - Bumpers of other teams that had no buzz yet become AUTRE.
//   - Existing MOI/EQUIPE/AUTRE states of already-buzzed teams are preserved.
func (c *Controller) UpdateBuzzStates(buzzingMAC string) {
	buzzingBumper := c.h.Engine().GetBumper(buzzingMAC)
	if buzzingBumper == nil {
		return
	}
	buzzingTeam := buzzingBumper.Team

	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		current := (*c.bumperBuzzState)[mac]

		if mac == buzzingMAC {
			// The buzzing bumper itself: MOI (first of its team)
			(*c.bumperBuzzState)[mac] = game.BuzzStateMoi
		} else if bumper.Team == buzzingTeam {
			// Same team as the buzzer: EQUIPE (unless already MOI — impossible here
			// since the engine only allows one buzz per team, but defensive check)
			if current != game.BuzzStateMoi {
				(*c.bumperBuzzState)[mac] = game.BuzzStateEquipe
			}
		} else {
			// Different team: promote to AUTRE only if still NONE
			if current == game.BuzzStateNone {
				(*c.bumperBuzzState)[mac] = game.BuzzStateAutre
			}
			// If already MOI/EQUIPE/AUTRE, leave unchanged (their team buzzed earlier)
		}
	}
}

// BuzzStateFor returns the current BuzzState for a given buzzer MAC.
// Defaults to NONE if not tracked yet.
func (c *Controller) BuzzStateFor(mac string) game.BuzzState {
	if bs, ok := (*c.bumperBuzzState)[mac]; ok {
		return bs
	}
	return game.BuzzStateNone
}

// sendLEDSetForBuzzer derives and sends the correct LED state for one buzzer based on current game phase.
// Used on reconnect when no stored LED state is available.
func (c *Controller) sendLEDSetForBuzzer(mac string) {
	bumper := c.h.Engine().GetBumper(mac)
	if bumper == nil || bumper.IsVPlayer {
		return
	}

	// #119 (B5): a buzzer connecting for the first time during ENTRACTE (no
	// prior stored LED state — resendLEDOnReconnect's fallback path, the
	// sole caller of this function) must come up OFF, not its team color.
	// The reconnect-with-remembered-state path is already covered: entering
	// entracte overwrites bumperLEDState via sendLEDSetAllEntracteOff, so
	// resendLEDOnReconnect's "stored state" branch already resends OFF.
	if c.h.Engine().IsEntracte() {
		c.Send(mac, protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"})
		return
	}

	state := c.h.Engine().GetState()
	phase := c.h.Engine().GetPhase()

	switch {
	case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
		c.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
	case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
		c.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
	case state.Question != nil && state.Question.Type == game.QuestionTypeMemotion:
		// MEMOTION: buzzers display team color (same as NORMAL stopped state)
		c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
	default:
		c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
	}
}

func (c *Controller) sendLEDSetForBuzzerNormal(mac string, bumper *game.Bumper, phase game.GamePhase) {
	rgb := c.TeamColor(bumper)
	dimIntensity := colors.DimIntensityFor(rgb)
	bs := c.BuzzStateFor(mac)
	switch phase {
	case game.PhaseStopped, game.PhasePrepare, game.PhaseReady, game.PhaseCountdown:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	case game.PhaseStarted:
		switch bs {
		case game.BuzzStateMoi:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	case game.PhasePaused:
		switch bs {
		case game.BuzzStateMoi:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	case game.PhaseRevealed:
		switch bs {
		case game.BuzzStateMoi:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	default:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
}

func (c *Controller) sendLEDSetForBuzzerQCM(mac string, bumper *game.Bumper, phase game.GamePhase, state game.GameState) {
	answerRGB := colors.AnswerColorToRGB(bumper.AnswerColor)
	teamRGB := c.TeamColor(bumper)
	bs := c.BuzzStateFor(mac)

	switch phase {
	case game.PhaseStopped, game.PhasePrepare:
		c.Send(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
	case game.PhaseReady, game.PhaseCountdown:
		c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
	case game.PhaseStarted:
		if bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe {
			// My team buzzed → show team color (hides the answer)
			c.Send(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
		} else {
			// NONE or AUTRE → show answer color
			c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
		}
	case game.PhasePaused:
		if bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe {
			c.Send(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
		} else {
			c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
		}
	case game.PhaseRevealed:
		// Revealed logic uses firstBuzzTeam from engine state (team.Time > 0 && team.Bumper == mac or same team)
		c.ForBuzzerQCMReveal(mac, bumper, state)
	default:
		c.Send(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
	}
}

// ForBuzzerQCMReveal handles the REVEALED phase LED for a single QCM buzzer.
func (c *Controller) ForBuzzerQCMReveal(mac string, bumper *game.Bumper, state game.GameState) {
	answerRGB := colors.AnswerColorToRGB(bumper.AnswerColor)
	bs := c.BuzzStateFor(mac)
	myTeamBuzzed := bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe

	if !myTeamBuzzed {
		// No buzz from my team: DIM 25% answer color
		c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 64, Effect: "DIM"})
		return
	}

	// My team buzzed — determine correct answer from current question
	correctAnswer := ""
	if state.Question != nil {
		correctAnswer = state.Question.QCMCorrect
	}
	myAnswerCorrect := correctAnswer != "" && string(bumper.AnsweredColor) == correctAnswer // #255: answered, not assigned

	if !myAnswerCorrect {
		// Wrong answer (or no correct defined): DIM 25%
		c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 64, Effect: "DIM"})
		return
	}

	// Correct answer — check if my team was the first to buzz (team.Time == smallest among buzzed teams)
	isFirstBuzz := c.isFirstBuzzTeam(bumper.Team)
	if isFirstBuzz {
		// Good answer + first buzz: BLINK
		c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "BLINK"})
	} else {
		// Good answer + not first buzz: SOLID 100%
		c.Send(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
	}
}

// isFirstBuzzTeam returns true if the given team was the first team to buzz in this round.
// Uses bumper.Time values stored by the engine to find the earliest buzz.
func (c *Controller) isFirstBuzzTeam(teamName string) bool {
	if teamName == "" {
		return false
	}
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return false
	}

	// Find the earliest buzz time across all teams
	var minTime int64 = 0
	teamEarliestTime := int64(0)

	for _, bumper := range tb.Bumpers {
		if bumper.IsVPlayer || bumper.Time <= 0 {
			continue
		}
		if minTime == 0 || bumper.Time < minTime {
			minTime = bumper.Time
		}
		if bumper.Team == teamName {
			if teamEarliestTime == 0 || bumper.Time < teamEarliestTime {
				teamEarliestTime = bumper.Time
			}
		}
	}

	return teamEarliestTime > 0 && teamEarliestTime == minTime
}

func (c *Controller) sendLEDSetForBuzzerMemory(mac string, bumper *game.Bumper, phase game.GamePhase, state game.GameState) {
	rgb := c.TeamColor(bumper)
	dimIntensity := colors.DimIntensityFor(rgb)
	memoryMode := ""
	if state.Question != nil {
		memoryMode = state.Question.MemoryMode
	}

	switch phase {
	case game.PhaseStopped, game.PhasePrepare, game.PhaseReady, game.PhaseRevealed:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	case game.PhasePaused:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
	case game.PhaseStarted:
		if memoryMode == string(game.MemoryModeSolo) || memoryMode == "" {
			// SOLO: active team = SOLID 100%, inactive = DIM 25%
			if bumper.Team == state.MemoryCurrentTeam {
				c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
			} else {
				c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
			}
		} else {
			// Multi-team modes: active=SOLID 100%, next=SOLID 50%, others participating=DIM 25%, not selected=OFF
			c.sendLEDSetMemoryMultiTeam(mac, bumper, state)
		}
	default:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
}

// sendLEDSetMemoryMultiTeam computes the LED for a buzzer in a multi-team Memory round.
func (c *Controller) sendLEDSetMemoryMultiTeam(mac string, bumper *game.Bumper, state game.GameState) {
	c.sendLEDSetMultiTeam(mac, bumper, state.MemoryCurrentTeam, c.nextMemoryTeam(state), state.MemoryParticipatingTeams)
}

// sendLEDSetMultiTeam computes and sends the LED for one buzzer in a
// multi-team round — the "active/next/other-participant/non-participant"
// grid shared by MEMORY (sendLEDSetMemoryMultiTeam) and RAFALE
// (sendLEDSetRafaleTeams, v8.0.0, #199 task 35): active team SOLID 100%
// (INTENSITY=255), next team SOLID 50% (INTENSITY=128), other participating
// teams DIM at a tone-relative intensity (colors.DimIntensityFor — #113 B3, a flat
// value nearly extinguishes deep-toned team colors), non-participants OFF.
// Color is always resolved via bumper.Team's COLOR_NAME (teamColorToRGB),
// never a hard-coded RGB triplet (contract LED_SET_PROTOCOL.md §11).
//
// Deliberately parameterized by (activeTeam, nextTeam, participatingTeams)
// rather than a game.GameState — the caller decides which mode's fields
// those come from (Memory* vs Rafale*), this function only knows the grid
// itself, exactly the same separation team_rotation.go's rotateTeam keeps
// between rotation MECHANISM and per-mode rotation POLICY.
func (c *Controller) sendLEDSetMultiTeam(mac string, bumper *game.Bumper, activeTeam, nextTeam string, participatingTeams []string) {
	rgb := c.TeamColor(bumper)

	// Not in participating teams → OFF
	participating := false
	for _, t := range participatingTeams {
		if t == bumper.Team {
			participating = true
			break
		}
	}
	if !participating {
		c.Send(mac, protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"})
		return
	}

	// Active team
	if bumper.Team == activeTeam {
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		return
	}

	// Next team: SOLID 50% (INTENSITY=128)
	if nextTeam != "" && bumper.Team == nextTeam {
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 128, Effect: "SOLID"})
		return
	}

	// Other participating teams: DIM 25% (tone-relative, #113 B3 fast-follow —
	// same rationale as sendLEDSetForBuzzerNormal: a flat 64 nearly extinguishes
	// deep-toned team colors).
	c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: colors.DimIntensityFor(rgb), Effect: "DIM"})
}

// nextMemoryTeam returns the name of the team that plays after MemoryCurrentTeam,
// cycling through MemoryParticipatingTeams in order.
func (c *Controller) nextMemoryTeam(state game.GameState) string {
	teams := state.MemoryParticipatingTeams
	if len(teams) < 2 {
		return ""
	}
	for i, t := range teams {
		if t == state.MemoryCurrentTeam {
			return teams[(i+1)%len(teams)]
		}
	}
	return ""
}

// nextRafaleTeam returns the name of the team that plays after
// RafaleCurrentTeam, cycling through RafaleParticipatingTeams in order —
// RAFALE's counterpart to nextMemoryTeam above (v8.0.0, #199 task 35/36).
func (c *Controller) nextRafaleTeam(state game.GameState) string {
	teams := state.RafaleParticipatingTeams
	if len(teams) < 2 {
		return ""
	}
	for i, t := range teams {
		if t == state.RafaleCurrentTeam {
			return teams[(i+1)%len(teams)]
		}
	}
	return ""
}

// sendLEDSetForBuzzerRafale computes and sends the LED for ONE buzzer during
// a RAFALE question — mirrors sendLEDSetForBuzzerMemory's per-buzzer phase
// dispatch (STOPPED/PREPARE/READY/REVEALED → plain team color, PAUSED →
// DIM, STARTED → the mode-specific grid). Extracted (post-review MINEUR-3,
// code-review-20260828-183037.md) so sendLEDSetAllBuzzers can call it
// per-buzzer like every other question type's case in that switch — without
// this, RAFALE fell into the `default:` branch there (plain per-buzzer team
// color, no active/next/dim/off grid at all), so an admin action that
// re-triggers sendLEDSetAllBuzzers mid-round (e.g. editing bumpers/teams)
// transiently overwrote the "active team" grid until the next RAFALE
// advance self-corrected it via OnRafaleTeamsChanged — never a state/data
// loss, but a visible flicker.
//
// STARTED/SOLO is OFF for everyone (contract §8.3's own table: "Mode SOLO |
// Éteint pour tous") — deliberately NOT the same as MEMORY's own SOLO
// branch (active=SOLID/inactive=DIM): RAFALE has no notion of "my turn" LED
// feedback in SOLO, there is exactly one team and nothing to distinguish.
func (c *Controller) sendLEDSetForBuzzerRafale(mac string, bumper *game.Bumper, phase game.GamePhase, state game.GameState) {
	rgb := c.TeamColor(bumper)
	dimIntensity := colors.DimIntensityFor(rgb)

	switch phase {
	case game.PhaseStopped, game.PhasePrepare, game.PhaseReady, game.PhaseRevealed:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	case game.PhasePaused:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
	case game.PhaseStarted:
		mode := ""
		if state.Question != nil {
			mode = state.Question.RafaleMode
		}
		if mode == "" || mode == string(game.RafaleModeSolo) {
			c.Send(mac, protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"})
			return
		}
		c.sendLEDSetMultiTeam(mac, bumper, state.RafaleCurrentTeam, c.nextRafaleTeam(state), state.RafaleParticipatingTeams)
	default:
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
}

// RafaleTeams refreshes buzzer LEDs for the active RAFALE round —
// contract §8.3. Wired to Engine.OnRafaleTeamsChanged (setupCallbacks
// below) — fired on round start, RAFALE_SET_TEAMS, and every advance (task
// 36: "rafraîchi à chaque rotation d'équipe et chaque changement de
// question").
//
// Phase-gated to STARTED, mirroring sendLEDSetForBuzzerRafale's own
// dispatch: the multi-team grid is a "round in progress" concept — a no-op
// during PREPARE/READY (see SetRafaleParticipatingTeams' doc comment) and
// during ROUND_END (whatever was lit for the last question stays lit; no
// LED behavior is specified past the round ending, and the next START —
// any question type — resets buzzer LEDs via sendLEDSetAllBuzzers as
// usual, which now also carries a dedicated RAFALE case — see it below).
//
// ⚠️ This function only ever drives LED OUTPUT — buzzer presses stay
// ignored regardless of what's lit (engine.go ProcessButtonPress, #107 task
// 25, untouched by this batch).
func (c *Controller) RafaleTeams() {
	state := c.h.Engine().GetState()
	if state.Phase != game.PhaseStarted || state.Question == nil || state.Question.Type != game.QuestionTypeRafale {
		return
	}

	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		c.sendLEDSetForBuzzerRafale(mac, bumper, state.Phase, state)
	}
	// One broadcast for all AckPending state changes set by the loop above —
	// same rationale as sendLEDSetAllBuzzers' own trailing call (#127/#129).
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// AllBuzzers sends per-buzzer LED_SET based on current game state (READY/START/STOPPED phase).
// Called after READY, START, and FULL updates.
func (c *Controller) AllBuzzers() {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := c.h.Engine().GetState()
	phase := c.h.Engine().GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			c.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			c.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeRafale:
			// Post-review MINEUR-3 (code-review-20260828-183037.md): without
			// this case, RAFALE fell into `default:` below (plain per-buzzer
			// team color) — any admin action that re-triggers
			// sendLEDSetAllBuzzers mid-round (editing bumpers/teams) would
			// transiently overwrite the active-team grid.
			c.sendLEDSetForBuzzerRafale(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemotion:
			// MEMOTION: buzzers display team color (same as NORMAL)
			c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		default:
			c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending state changes set by the loop above.
	// #127: the loop above unconditionally skips IsVPlayer bumpers (line
	// 3077), so this broadcast never carries a change relevant to any
	// VJoueur, at any of this function's call sites — targeting VPlayer here
	// was always a needless full-GameState resend to every connected
	// VJoueur. Found while verifying #127 CA1 empirically: it was firing an
	// extra unconditional UPDATE at every PREPARE->READY transition (via
	// broadcastReady -> sendLEDSetAllBuzzers), on top of the legitimate one
	// from TransitionToReady's OnStateChange.
	//
	// #129: TV removed too, at all 11 call sites, for a stronger reason than
	// the VPlayer case above — ACK_PENDING (the very field this broadcast
	// exists to announce) is one of the 5 fields SerializeForWebClient always
	// strips from TV's payload (protocol.AdminOnlyBumperFields). TV can never
	// see the content this call carries, regardless of phase or call site —
	// unlike the VPlayer case, this isn't conditional on the loop skipping
	// virtual bumpers, it's the serializer itself making the field invisible
	// to TV unconditionally. Found while verifying #129 CA12 empirically: TV
	// was still receiving 3 UPDATE in the PREPARE->READY window instead of
	// the 2 the contract promises (§1), this call being the 3rd, redundant
	// with the legitimate READY-transition broadcast from
	// TransitionToReady's OnStateChange. Admin still needs it (ACK_PENDING
	// spinner UI); Buzzer still needs it (LED sync, its own fields).
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// Stop broadcasts team color SOLID 100% to all buzzers (game stopped/prepare).
func (c *Controller) Stop() {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		rgb := c.TeamColor(bumper)
		c.Send(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
	// One broadcast for all AckPending changes.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. Sole call site (broadcastStop) already sends its own
	// Admin+TV+VPlayer ActionStop broadcast right before this — TV/VPlayer
	// already have everything they need from that call; this one only ever
	// carried ACK_PENDING, which neither can see anyway.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// Pause sends per-buzzer LED state when a specific buzzer has buzzed (PAUSED phase).
// Uses BuzzState to determine each buzzer's LED.
func (c *Controller) Pause(bumperID string) {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := c.h.Engine().GetState()
	phase := c.h.Engine().GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			c.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			c.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		default:
			c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending changes.
	// #129 (found while verifying Phase 3/CA10 empirically — same pattern
	// and justification as sendLEDSetAllBuzzers's #127/#129-T1.6 narrowing):
	// the loop above unconditionally skips IsVPlayer bumpers, and
	// ACK_PENDING (the field this broadcast exists to announce) is always
	// stripped from TV's payload too (protocol.AdminOnlyBumperFields, via
	// SerializeForWebClient) — neither client type can ever see anything
	// this call carries, at either of sendLEDSetPause's two call sites
	// (broadcastPause, broadcastPauseAll → sendLEDSetPauseAll). Without this,
	// every buzz/QCM-answer/etc. still sent a full, un-targeted UPDATE to
	// every VPlayer right next to it, defeating T3.1's targeting entirely —
	// this is what main_broadcast_129_phase3_test.go's CA10 tests caught.
	// broadcastPause's OWN direct PAUSE broadcast (the one the plan says to
	// leave untouched — it still reaches Admin/TV/VPlayer) is a separate
	// call, unaffected by this line.
	//
	// The 5 sibling sendLEDSet* functions with the identical pattern
	// (broadcastLEDSet, sendLEDSetStop, sendLEDSetReveal, sendLEDSetToTeam,
	// sendLEDSetComet) were audited and fixed the same way under #132 —
	// see their own doc comments and _work/reports/dev-backend-132-audit-*.md.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// PauseAll sends LED state for PAUSE ALL (admin-initiated pause, no specific buzzer).
func (c *Controller) PauseAll() {
	c.Pause("")
}

// Continue sends LED state after the game resumes from pause (back to STARTED).
// Uses the same per-buzzer state machine as sendLEDSetAllBuzzers but preserves BuzzStates.
func (c *Controller) Continue() {
	c.AllBuzzers()
}

// Reveal sends per-buzzer LED feedback at REVEALED phase.
func (c *Controller) Reveal(correctAnswer string) {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := c.h.Engine().GetState()
	phase := c.h.Engine().GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			c.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			c.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		default:
			c.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending changes.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. Sole call site (broadcastReveal) already sends its own
	// Admin+TV+VPlayer ActionReveal broadcast right before this.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// ToTeam sends a LED_SET payload to all physical buzzers belonging to teamID.
// If teamID is empty, sends to ALL physical buzzers (excluding VPlayers).
//
// #132 audit: currently DEAD CODE — grep confirms zero call sites anywhere
// in the codebase (including tests). Fixed anyway — see broadcastLEDSet's
// doc comment for the reasoning; both are flagged together in the audit
// report as candidates for removal in a separate cleanup, not done here.
func (c *Controller) ToTeam(teamID string, payload protocol.LEDSetPayload) {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		if teamID != "" && bumper.Team != teamID {
			continue
		}
		c.Send(mac, payload)
	}
	// One broadcast for all AckPending changes.
	// #132: same pattern/justification as the other sendLEDSet* functions —
	// loop always skips IsVPlayer, ACK_PENDING always stripped from TV.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// cometBandColor returns the best-contrasting band color for a COMET animation over bgColor.
// Uses gold [255,215,0] by default; switches to white [255,255,255] when the background is
// too close to gold (squared Euclidean distance < 8000, ~89 units).
func cometBandColor(bg [3]int) [3]int {
	gold := [3]int{255, 215, 0}
	dr := bg[0] - gold[0]
	dg := bg[1] - gold[1]
	db := bg[2] - gold[2]
	if dr*dr+dg*dg+db*db < 8000 {
		return [3]int{255, 255, 255} // white for yellow/gold teams
	}
	return gold
}

// Comet sends a COMET LED effect to all physical buzzers of the given team.
// COLOR = team background color; COMET_COLOR = contrasting band color (gold or white).
// Duration: 2 laps × 23 steps × 100 ms = 4600 ms + 200 ms margin = 4800 ms.
func (c *Controller) Comet(teamID string) {
	tb := c.h.Engine().GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		if teamID != "" && bumper.Team != teamID {
			continue
		}
		teamColor := c.TeamColor(bumper)
		band := cometBandColor(teamColor)
		c.Send(mac, protocol.LEDSetPayload{
			Color:      teamColor,
			Intensity:  255,
			Effect:     protocol.LEDEffectComet,
			CometColor: &band,
		})
	}
	// One broadcast for all AckPending changes from the loop above.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. All 4 call sites (handlePoints, MEMOTION_DONE, BUMPER_POINTS,
	// handleTeamPoints) already call their own unconditional broadcastUpdate()
	// right after this returns, to announce the score change itself — by
	// then ACK_PENDING is already set (sendLEDSet sets it synchronously), so
	// Admin still sees it on that later broadcast; nothing is lost, only the
	// redundant TV/VPlayer leg of THIS broadcast is removed.
	c.h.BroadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
	// Restore normal LED state after firmware COMET animation completes.
	time.AfterFunc(4800*time.Millisecond, func() {
		c.AllBuzzers()
	})
}
