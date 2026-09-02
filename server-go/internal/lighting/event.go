// Package lighting is the ambiance-lighting vocabulary and writer for
// BuzzControl (#205, milestone v10.0.0 — contracts/lighting.md).
//
// It deliberately imports neither internal/game nor internal/protocol: the
// package must compile and be testable on its own. The adapter that turns a
// live GameState into an Event, and an Event into a State, lives in
// cmd/server/ambiance.go.
package lighting

import (
	"context"
	"time"
)

// EventKind is the ambiance event vocabulary. The list is CLOSED for
// v10.0.0 (contract §2.1): a new need adds a kind to the contract, it never
// overloads the meaning of an existing one.
type EventKind string

const (
	KindIdle     EventKind = "IDLE"      // no game in progress
	KindReady    EventKind = "READY"     // ready to start (PREPARE, READY, COUNTDOWN)
	KindRunning  EventKind = "RUNNING"   // question in progress (STARTED)
	KindBuzz     EventKind = "BUZZ"      // a buzz interrupted the question
	KindPauseAll EventKind = "PAUSE_ALL" // admin pause, no buzz
	KindReveal   EventKind = "REVEAL"    // answer revealed
	KindTeamTurn EventKind = "TEAM_TURN" // active team changed (MEMORY/MEMOTION/RAFALE)
	KindEntracte EventKind = "ENTRACTE"  // intermission active
	KindScore    EventKind = "SCORE"     // points awarded — a pulse, see contract §2.3
)

// Event is one ambiance event. Teams holds team NAMES (game.Team has no ID —
// contract §2.2), first = principal; empty = no team concerned. It is a slice
// because a QCM REVEAL concerns several teams at once.
type Event struct {
	Kind  EventKind
	Teams []string
}

// State is the desired lighting state at a given instant. In #205 there is
// always exactly one zone, "general"; #213 adds one zone per team.
type State struct {
	Zones []ZoneState
}

// ZoneState is the colour of one zone. Color and Intensity deliberately use
// the exact format and scale of protocol.LEDSetPayload (RGB 0-255, intensity
// 0-255) so that the room and the buzzers show the SAME colour for the same
// team (contract §3). Conversion to hardware formats belongs to the driver.
type ZoneState struct {
	Zone      string
	Color     [3]int
	Intensity int
}

// ZoneGeneral is the single zone name used by #205.
const ZoneGeneral = "general"

// Driver applies a lighting state on hardware.
type Driver interface {
	// Apply is called ONLY from the writer's single goroutine (contract §4).
	// It may therefore block and does NOT need to be safe for concurrent use.
	Apply(ctx context.Context, s State) error

	// Close releases resources. Idempotent, callable even if Apply never
	// succeeded.
	Close() error
}

// MinInterval is the minimum delay between two Driver.Apply calls (contract
// §4.4). PROVISIONAL: 100 ms until the #204 spike measures real BLE write
// latency and inter-bulb desync; recalibrate here, nowhere else.
const MinInterval = 100 * time.Millisecond

// ScorePulseDuration is how long a SCORE pulse is rendered before the room
// falls back to the derived state. Aligned on the 4800 ms time.AfterFunc that
// restores buzzer LEDs at the end of sendLEDSetComet (cmd/server/main.go), so
// room and buzzers return to normal at the same instant by construction
// (contract §4.2).
const ScorePulseDuration = 4800 * time.Millisecond
