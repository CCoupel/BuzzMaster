package ambiance

import (
	"context"
	"reflect"
	"sync"
	"time"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// Light-effect reader (v11.7.0, #223 — contracts/light-effects.md §5): ONE goroutine
// that, at each tick, finds the live moment, resolves the effect chosen for it, renders
// the frame with effects.Render, and — only when the frame differs from the published
// one — publishes it and wakes the writer. Scene() reads the published frame. Same
// pattern as RunChronoPulse; never a write cadence of its own (the writer keeps its 250 ms).

// EffectsTickInterval is the reader's default tick (§5.1).
const EffectsTickInterval = 100 * time.Millisecond

// EffectsSource is what the reader needs from the application: the catalogue and the
// RESOLVED effect settings (global ← round). Optional on Host (type assertion) so the
// minimal test hosts keep compiling; SetEffectsSource overrides it.
type EffectsSource interface {
	EffectsCatalogue() effects.Catalogue
	// EffectSettings returns the resolved effect id per moment for the room and for the
	// animateur group (ambiancecfg Settings.LightEffects / LightHostEffects).
	// host is a table by animateur group id, then moment.
	EffectSettings() (room map[string]string, host map[string]map[string]string)
}

// effectFrames is the published result of one reader step.
type effectFrames struct {
	moment   effects.Moment
	room     effects.Frame         // nil = no room effect (ordinary scene)
	roomAll  bool                  // target: all — one state for the whole zone
	roomNums []int                 // real group numbers of the frame entries (target: groups)
	host     map[int]effects.Frame // by animateur rank k; a missing rank = fixed white
}

type effectRun struct {
	key   string
	start time.Time
}

// effectsState is the reader's own bookkeeping.
type effectsState struct {
	mu    sync.Mutex
	room  effectRun
	host  map[int]*effectRun // by animateur rank
	score struct {
		start time.Time
		team  string
	}
	src EffectsSource
}

// SetEffectsSource overrides the host's source (tests).
func (c *Controller) SetEffectsSource(s EffectsSource) {
	c.effects.mu.Lock()
	c.effects.src = s
	c.effects.mu.Unlock()
}

func (c *Controller) effectsSource() EffectsSource {
	c.effects.mu.Lock()
	s := c.effects.src
	c.effects.mu.Unlock()
	if s != nil {
		return s
	}
	if hs, ok := c.h.(EffectsSource); ok {
		return hs
	}
	return nil
}

func (c *Controller) effectsNow() time.Time {
	if c.EffectsClock != nil {
		return c.EffectsClock()
	}
	return time.Now()
}

// noteScorePulse records the start of a SCORE pulse and the credited team (called from
// StartScoreFlash, itself invoked at every NotifyPulse(KindScore) site).
func (c *Controller) noteScorePulse(team string) {
	c.effects.mu.Lock()
	c.effects.score.start, c.effects.score.team = c.effectsNow(), team
	c.effects.mu.Unlock()
}

// RunEffects is the reader goroutine. Idempotent (effectsStarted), stopped by ctx.
func (c *Controller) RunEffects(ctx context.Context) {
	if !c.effectsStarted.CompareAndSwap(false, true) {
		return
	}
	tick := c.EffectsTick
	if tick <= 0 {
		tick = EffectsTickInterval
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		c.EffectsStep()
	}
}

// liveMoment determines the moment from the live state (§5.2), never from a stored state
// of its own — except the SCORE pulse, which has no state to derive from.
func (c *Controller) liveMoment(ev lighting.Event, st game.GameState, now time.Time) (m effects.Moment, scoreStart time.Time, team string, ok bool) {
	c.effects.mu.Lock()
	ss, sTeam := c.effects.score.start, c.effects.score.team
	c.effects.mu.Unlock()
	if !ss.IsZero() && now.Sub(ss) < lighting.ScorePulseDuration && now.After(ss.Add(-time.Second)) {
		return effects.MomentScore, ss, sTeam, true
	}
	switch ev.Kind {
	case lighting.KindRunning:
		if st.Delay > 0 {
			return effects.MomentChrono, time.Time{}, "", true
		}
	case lighting.KindIdle, lighting.KindPauseAll, lighting.KindEntracte:
		return effects.MomentCalm, time.Time{}, "", true
	}
	return "", time.Time{}, "", false
}

// momentOfEvent is the moment a Scene(ev) call belongs to (to apply only a frame computed
// for that same moment).
func momentOfEvent(k lighting.EventKind) effects.Moment {
	switch k {
	case lighting.KindRunning:
		return effects.MomentChrono
	case lighting.KindIdle, lighting.KindPauseAll, lighting.KindEntracte:
		return effects.MomentCalm
	case lighting.KindScore:
		return effects.MomentScore
	}
	return ""
}

// resolveEffect returns the catalogue entry to play for the setting id, or false: absent,
// not declaring the moment, or not applicable to n groups (the caller then falls back to
// the default of the moment, §3.3). "" means no effect.
func resolveEffect(cat effects.Catalogue, id string, m effects.Moment, n int) (effects.Entry, bool) {
	if id == "" {
		return effects.Entry{}, false
	}
	en, ok := cat.Get(id)
	if !ok || !en.Effect.HasMoment(m) {
		return effects.Entry{}, false
	}
	if okApp, _ := en.Effect.Applicable(n); !okApp {
		return effects.Entry{}, false
	}
	return en, true
}

// resolveRoomEffect is resolveEffect with the fallback of §3.3: a moment set to an absent,
// invalid or inapplicable effect plays its default (the pulsation at the chrono moment, no
// effect elsewhere). A moment explicitly set to "" plays nothing.
func resolveRoomEffect(cat effects.Catalogue, id string, m effects.Moment, n int) (effects.Entry, bool) {
	if en, ok := resolveEffect(cat, id, m, n); ok {
		return en, true
	}
	if id != "" && m == effects.MomentChrono && id != ambiancecfg.PulsationID {
		return resolveEffect(cat, ambiancecfg.PulsationID, m, n)
	}
	return effects.Entry{}, false
}

// runFor returns the start of the run of key, restarting it when the key changes.
func runFor(r *effectRun, key string, now time.Time) time.Time {
	if r.key != key {
		r.key, r.start = key, now
	}
	return r.start
}

// EffectsStep computes and, when changed, publishes the frames once. Exported so tests
// drive it with an injected clock instead of waiting for the ticker.
func (c *Controller) EffectsStep() {
	next := c.computeFrames()
	prev := c.effectFrames.Load()
	if reflect.DeepEqual(prev, next) || (prev == nil && next == nil) {
		return
	}
	c.effectFrames.Store(next)
	c.Writer().NotifyState() // nil-safe
}

func (c *Controller) computeFrames() *effectFrames {
	if c.effectsQuiet.Load() || c.Writer() == nil {
		return nil
	}
	src := c.effectsSource()
	if src == nil || c.h.Engine() == nil {
		return nil
	}
	now := c.effectsNow()
	ev := c.DeriveEvent()
	state := c.h.Engine().GetState()
	moment, scoreStart, scoreTeam, ok := c.liveMoment(ev, state, now)
	if !ok {
		c.effects.mu.Lock()
		c.effects.room.key = ""
		c.effects.host = nil
		c.effects.mu.Unlock()
		return nil
	}
	cat := src.EffectsCatalogue()
	roomSet, hostSet := src.EffectSettings()
	lc := config.Get().Lighting
	groups := lc.RoomGroups()

	// Context shared by both renders.
	sceneColor, _ := c.baseSceneColor(ev)
	ctx := effects.Context{
		Moment:    moment,
		Remaining: time.Duration(state.CurrentTime) * time.Second,
		Total:     time.Duration(state.Delay) * time.Second,
		Scene:     sceneColor,
		Theme:     c.ThemeColor(),
		WallMs:    now.UnixMilli(),
	}
	if moment == effects.MomentScore && scoreTeam != "" {
		ctx.Team = c.h.TeamNameColor(scoreTeam)
	}

	out := &effectFrames{moment: moment}
	c.effects.mu.Lock()
	defer c.effects.mu.Unlock()

	// Room: only in AUTO mode without Flash (§5.3 point 2).
	if c.lightingMode() == ModeAuto && !c.isLightingFlashOn() {
		if en, ok := resolveRoomEffect(cat, roomSet[string(moment)], moment, len(groups)); ok {
			key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation)
			if moment == effects.MomentScore {
				key += "|" + scoreStart.String()
			}
			start := runFor(&c.effects.room, key, now)
			elapsed := now.Sub(start)
			if moment == effects.MomentScore {
				elapsed = now.Sub(scoreStart)
			}
			if en.Effect.DurationMs == 0 || elapsed < time.Duration(en.Effect.DurationMs)*time.Millisecond {
				rctx := ctx
				rctx.Groups = len(groups)
				rctx.Seed = start.UnixMilli()
				rctx.Origin = 1
				if f := effects.Render(en.Effect, rctx, elapsed); f != nil {
					out.room = f
					out.roomAll = en.Effect.Target == effects.TargetAll
					if !out.roomAll {
						out.roomNums = make([]int, len(groups))
						for i, g := range groups {
							out.roomNums[i] = g.Num
						}
					}
				}
			}
		}
	}
	if out.room == nil {
		c.effects.room.key = ""
	}

	// Animateur groups: each independent of the others, of mode, Flash and trial (§2.5, §5.3).
	live := map[int]bool{}
	for _, g := range lc.HostGroups() {
		if len(g.Lights) == 0 {
			continue
		}
		live[g.Rank] = true
		en, ok := resolveEffect(cat, hostSet[g.ID][string(moment)], moment, 1)
		if !ok {
			delete(c.effects.host, g.Rank)
			continue
		}
		if c.effects.host == nil {
			c.effects.host = map[int]*effectRun{}
		}
		run := c.effects.host[g.Rank]
		if run == nil {
			run = &effectRun{}
			c.effects.host[g.Rank] = run
		}
		key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation)
		if moment == effects.MomentScore {
			key += "|" + scoreStart.String()
		}
		start := runFor(run, key, now)
		elapsed := now.Sub(start)
		if moment == effects.MomentScore {
			elapsed = now.Sub(scoreStart)
		}
		if en.Effect.DurationMs != 0 && elapsed >= time.Duration(en.Effect.DurationMs)*time.Millisecond {
			continue
		}
		hctx := ctx
		hctx.Groups = 1
		hctx.Scene = [3]int{255, 255, 255} // "scene" is white for an animateur group (§2.5)
		hctx.Seed = start.UnixMilli()
		hctx.Origin = 1
		if f := effects.Render(en.Effect, hctx, elapsed); f != nil {
			if out.host == nil {
				out.host = map[int]effects.Frame{}
			}
			out.host[g.Rank] = f
		}
	}
	for rank := range c.effects.host { // a group that disappeared drops its run
		if !live[rank] {
			delete(c.effects.host, rank)
		}
	}
	if out.room == nil && out.host == nil {
		return nil
	}
	return out
}

func itoa(u uint64) string {
	var b [20]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
		if u == 0 {
			break
		}
	}
	return string(b[i:])
}

// applyEffectFrames adds the published frames to the zones of Scene(ev): the room frame
// (only when it was computed for ev's moment) over the general zone, and the animateur
// group's own entry. Returns the zones, with the host entry already included.
func (c *Controller) applyEffectFrames(ev lighting.Event, zones []lighting.ZoneState) []lighting.ZoneState {
	fr := c.effectFrames.Load()
	if fr == nil || fr.moment != momentOfEvent(ev.Kind) {
		return zones
	}
	if fr.room != nil && c.lightingMode() == ModeAuto && !c.isLightingFlashOn() {
		if fr.roomAll {
			g := fr.room[0]
			zones[0].Color, zones[0].Intensity, zones[0].TransitionMs = g.Color, g.Intensity, g.FadeMs
		} else {
			// the frame refines the plain Group 0 entry kept first (§2.2)
			extra := make([]lighting.ZoneState, 0, len(fr.room))
			for i, g := range fr.room {
				if i >= len(fr.roomNums) {
					break
				}
				if g.Absent { // no state for this group: it keeps the plain room scene (§4.3.1)
					continue
				}
				extra = append(extra, lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: fr.roomNums[i], Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs})
			}
			zones = append(zones[:1], append(extra, zones[1:]...)...)
		}
	}
	return zones
}

// hostEffectState returns the published animateur frame for ev's moment, if any.
func (c *Controller) hostEffectState(ev lighting.Event, rank int) (lighting.ZoneState, bool) {
	fr := c.effectFrames.Load()
	if fr == nil || fr.host == nil || fr.moment != momentOfEvent(ev.Kind) {
		return lighting.ZoneState{}, false
	}
	f, ok := fr.host[rank]
	if !ok || len(f) == 0 {
		return lighting.ZoneState{}, false
	}
	g := f[0]
	if g.Absent {
		return lighting.ZoneState{}, false
	}
	return lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: lighting.HostGroup(rank), Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs}, true
}
