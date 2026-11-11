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
	EffectSettings() (room, host map[string]string)
}

// effectFrames is the published result of one reader step.
type effectFrames struct {
	moment   effects.Moment
	room     effects.Frame // nil = no room effect (ordinary scene)
	roomAll  bool          // target: all — one state for the whole zone
	roomNums []int         // real group numbers of the frame entries (target: groups)
	host     effects.Frame // nil = fixed white
}

type effectRun struct {
	key   string
	start time.Time
}

// effectsState is the reader's own bookkeeping.
type effectsState struct {
	mu    sync.Mutex
	room  effectRun
	host  effectRun
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

// resolveEffect returns the catalogue entry to play for the setting id, or false (default
// of the moment). The legacy pulsation (RunChronoPulse) stays the owner of "pulsation"
// until its migration (Batch 2.3), so it is never played by the reader.
func resolveEffect(cat effects.Catalogue, id string, m effects.Moment, n int) (effects.Entry, bool) {
	if id == "" || id == ambiancecfg.PulsationID {
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
		c.effects.room.key, c.effects.host.key = "", ""
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
		if en, ok := resolveEffect(cat, roomSet[string(moment)], moment, len(groups)); ok {
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
				rctx.Origin = originIndex(groups, scoreTeam)
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

	// Animateur group: independent of mode, Flash and trial (§2.5, §5.3).
	if g, ok := lc.HostGroup(); ok && len(g.Lights) > 0 {
		if en, ok := resolveEffect(cat, hostSet[string(moment)], moment, 1); ok {
			key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation)
			if moment == effects.MomentScore {
				key += "|" + scoreStart.String()
			}
			start := runFor(&c.effects.host, key, now)
			elapsed := now.Sub(start)
			if moment == effects.MomentScore {
				elapsed = now.Sub(scoreStart)
			}
			if en.Effect.DurationMs == 0 || elapsed < time.Duration(en.Effect.DurationMs)*time.Millisecond {
				hctx := ctx
				hctx.Groups = 1
				hctx.Scene = [3]int{255, 255, 255} // "scene" is white for this group (§2.5)
				hctx.Seed = start.UnixMilli()
				hctx.Origin = 1
				out.host = effects.Render(en.Effect, hctx, elapsed)
			}
		}
	}
	if out.host == nil {
		c.effects.host.key = ""
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

// originIndex is the 1-based index, among the rendered groups, of the group installed
// near team (default 1, §4.5).
func originIndex(groups []config.RoomGroup, team string) int {
	if team == "" {
		return 1
	}
	for i, g := range groups {
		for _, t := range g.NearTeams {
			if t == team {
				return i + 1
			}
		}
	}
	return 1
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
func (c *Controller) hostEffectState(ev lighting.Event) (lighting.ZoneState, bool) {
	fr := c.effectFrames.Load()
	if fr == nil || fr.host == nil || fr.moment != momentOfEvent(ev.Kind) {
		return lighting.ZoneState{}, false
	}
	g := fr.host[0]
	if g.Absent {
		return lighting.ZoneState{}, false
	}
	return lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: lighting.GroupHost, Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs}, true
}
