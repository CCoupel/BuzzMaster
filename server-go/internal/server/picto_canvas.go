package server

import (
	"sync"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// PICTO sheet journal and strokes guard (v13.0.0, #193, contracts/picto.md §8). Lives OUTSIDE the engine, behind
// its own lock; the engine only supplies the drawer, the phase and the epoch (PictoStrokeContext). The relay
// to the displays and the catch-up (PICTO_CANVAS) are driven by the application layer (cmd/server/picto.go).

// Bounds and limits (contracts/picto.md §8.3, §8.5, §8.6, §9.1).
const (
	PictoCoordMax       = 10000 // coordinates are integers in 0..10000 (§8.2)
	PictoMaxStrokes     = 6000  // strokes per epoch; beyond: CANVAS_FULL
	PictoMaxPoints      = 60000 // points per epoch; beyond: CANVAS_FULL
	PictoMaxFrameStroke = 20    // strokes per frame
	PictoMaxFramePoints = 200   // points per frame
	PictoMaxFramesPerS  = 40    // frames per second per connection; beyond: RATE_LIMITED
	PictoMaxSyncsPerS   = 2     // PICTO_CANVAS_SYNC per second per connection (each one serialises the whole sheet)
)

// PictoStrokeContext is what the guard needs to know about the sender and the game (§8.6): the identity comes
// from the CONNECTION (type, bound player ID), never from the payload.
type PictoStrokeContext struct {
	ClientType ClientType
	PlayerID   string // bumper ID bound to the connection (#246), "" if unbound
	Drawer     string // PICTO_DRAWER
	Phase      game.GamePhase
	Epoch      int  // PICTO_CANVAS_EPOCH
	IsPicto    bool // the current question is a PICTO one
	Now        time.Time
}

type pictoRate struct {
	windowStart time.Time
	count       int
	lastErr     time.Time
}

// PictoCanvas is the journal of the sheet of the current epoch.
type PictoCanvas struct {
	mu      sync.Mutex
	epoch   int
	seq     int
	strokes []protocol.PictoStroke
	index   map[int]int // stroke ID → position in strokes (a frame may continue an open stroke)
	points  int
	rate    map[string]*pictoRate // per connection (frames)
	syncs   map[string]*pictoRate // per connection (PICTO_CANVAS_SYNC)
}

// NewPictoCanvas returns an empty journal.
func NewPictoCanvas() *PictoCanvas {
	return &PictoCanvas{index: map[int]int{}, strokes: []protocol.PictoStroke{}, rate: map[string]*pictoRate{}, syncs: map[string]*pictoRate{}}
}

func (c *PictoCanvas) resetLocked(epoch int) {
	c.epoch, c.seq, c.points = epoch, 0, 0
	c.strokes = []protocol.PictoStroke{}
	c.index = map[int]int{}
}

// Reset restarts the journal empty for the given epoch (START, question change, relaunch, NEW_GAME).
func (c *PictoCanvas) Reset(epoch int) {
	c.mu.Lock()
	c.resetLocked(epoch)
	c.mu.Unlock()
}

// ForgetClient drops the rate-limit state of a closed connection.
func (c *PictoCanvas) ForgetClient(clientID string) {
	c.mu.Lock()
	delete(c.rate, clientID)
	delete(c.syncs, clientID)
	c.mu.Unlock()
}

// Snapshot returns the whole sheet of the epoch with the current SEQ (a deep copy). Outside a round it is the
// empty sheet of the last epoch.
func (c *PictoCanvas) Snapshot() protocol.PictoStrokesPayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := protocol.PictoStrokesPayload{Epoch: c.epoch, Seq: c.seq, Strokes: make([]protocol.PictoStroke, len(c.strokes))}
	for i, s := range c.strokes {
		pts := make([][]int, len(s.Pts))
		for j, pt := range s.Pts {
			pts[j] = []int{pt[0], pt[1]}
		}
		out.Strokes[i] = protocol.PictoStroke{ID: s.ID, End: s.End, Pts: pts}
	}
	return out
}

// rateAllowLocked counts one frame of the connection in the current second. Returns (allowed, reportError):
// beyond the limit the frame is refused, and an error is reported at most once per second.
func (c *PictoCanvas) rateAllowLocked(clientID string, now time.Time) (bool, bool) {
	r := c.rate[clientID]
	if r == nil {
		r = &pictoRate{windowStart: now}
		c.rate[clientID] = r
	}
	if now.Sub(r.windowStart) >= time.Second {
		r.windowStart, r.count = now, 0
	}
	r.count++
	if r.count <= PictoMaxFramesPerS {
		return true, false
	}
	if now.Sub(r.lastErr) >= time.Second {
		r.lastErr = now
		return false, true
	}
	return false, false
}

// AllowSync rate-limits PICTO_CANVAS_SYNC per connection (PictoMaxSyncsPerS per second): each call serialises
// the whole sheet (up to ~700 KB) on the single consumer of the hub's Incoming channel. Returns (allowed,
// reportError): beyond the limit the request is refused, with an error reported at most once per second.
func (c *PictoCanvas) AllowSync(clientID string, now time.Time) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.syncs[clientID]
	if r == nil {
		r = &pictoRate{windowStart: now}
		c.syncs[clientID] = r
	}
	if now.Sub(r.windowStart) >= time.Second {
		r.windowStart, r.count = now, 0
	}
	r.count++
	if r.count <= PictoMaxSyncsPerS {
		return true, false
	}
	if now.Sub(r.lastErr) >= time.Second {
		r.lastErr = now
		return false, true
	}
	return false, false
}

func pictoValidShape(p protocol.PictoStrokesPayload) bool {
	if len(p.Strokes) < 1 || len(p.Strokes) > PictoMaxFrameStroke {
		return false
	}
	total := 0
	for _, s := range p.Strokes {
		if s.ID < 1 {
			return false
		}
		for _, pt := range s.Pts {
			if len(pt) != 2 || pt[0] < 0 || pt[0] > PictoCoordMax || pt[1] < 0 || pt[1] > PictoCoordMax {
				return false
			}
		}
		total += len(s.Pts)
		if total > PictoMaxFramePoints {
			return false
		}
	}
	return total >= 1
}

// Accept runs the guard of a PICTO_STROKES frame (§8.6) and, if it passes, appends it to the journal and
// returns the frame to relay (with the SEQ the server assigned). Outcomes:
//   - relay != nil: accepted — relay it to tv / admin / anim, never to the tablets;
//   - perr != nil: refused explicitly — answer PICTO_ERROR to the emitter alone, nothing relayed;
//   - both nil: ignored in silence (stale epoch, or a rate-limited frame whose error was already reported).
func (c *PictoCanvas) Accept(clientID string, p protocol.PictoStrokesPayload, ctx PictoStrokeContext) (*protocol.PictoStrokesPayload, *game.PictoError) {
	c.mu.Lock()
	defer c.mu.Unlock()

	allowed, report := c.rateAllowLocked(clientID, ctx.Now)
	if !allowed {
		if report {
			return nil, &game.PictoError{Code: game.PictoErrRateLimited, Message: "trop de trames par seconde"}
		}
		return nil, nil
	}
	// Identity: the connection's type and bound player ID, never the payload (§8.6).
	if ctx.ClientType != ClientTypeVPlayer || ctx.PlayerID == "" || ctx.PlayerID != ctx.Drawer {
		return nil, &game.PictoError{Code: game.PictoErrNotDrawer, Message: "seul le dessinateur peut dessiner"}
	}
	if !ctx.IsPicto || ctx.Phase != game.PhaseStarted {
		return nil, &game.PictoError{Code: game.PictoErrWrongPhase, Message: "le dessin n'est possible que manche en cours"}
	}
	if p.Epoch != ctx.Epoch {
		return nil, nil // a stroke in flight during a change of round: ignored in silence (§8.4)
	}
	if !pictoValidShape(p) {
		return nil, &game.PictoError{Code: game.PictoErrInvalidStrokes, Message: "trame invalide"}
	}
	if c.epoch != ctx.Epoch { // defensive: the reset callback of this epoch has not run yet
		c.resetLocked(ctx.Epoch)
	}
	newStrokes, newPoints := 0, 0
	seen := map[int]bool{}
	for _, s := range p.Strokes {
		if _, ok := c.index[s.ID]; !ok && !seen[s.ID] {
			newStrokes++
		}
		seen[s.ID] = true
		newPoints += len(s.Pts)
	}
	if len(c.strokes)+newStrokes > PictoMaxStrokes || c.points+newPoints > PictoMaxPoints {
		return nil, &game.PictoError{Code: game.PictoErrCanvasFull, Message: "feuille pleine"}
	}
	for _, s := range p.Strokes {
		if i, ok := c.index[s.ID]; ok {
			c.strokes[i].Pts = append(c.strokes[i].Pts, s.Pts...)
			c.strokes[i].End = c.strokes[i].End || s.End
		} else {
			c.index[s.ID] = len(c.strokes)
			c.strokes = append(c.strokes, protocol.PictoStroke{ID: s.ID, End: s.End, Pts: append([][]int{}, s.Pts...)})
		}
	}
	c.points += newPoints
	c.seq++
	return &protocol.PictoStrokesPayload{Epoch: ctx.Epoch, Seq: c.seq, Strokes: p.Strokes}, nil
}
