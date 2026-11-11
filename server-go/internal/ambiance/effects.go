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

// Light-effect reader (v11.7.0, #223 — contracts/light-effects.md §5, revision 3): ONE
// goroutine that, at each reader tick (100 ms, never a write cadence), finds the live
// moment, resolves the effect chosen for it, advances that effect's own tick counter
// (duree_tick_ms ÷ speed, never under 250 ms), renders the frame with effects.Render and —
// only when the frame differs from the published one — publishes it and wakes the writer.
// Scene() reads the published frame. The tick counter is cumulative, so a speed change is
// taken at the next tick without restarting the effect.

// EffectsTickInterval is the reader's default period (§5.1).
const EffectsTickInterval = 100 * time.Millisecond

// EffectsSource is what the reader needs from the application: the catalogue and the
// RESOLVED effect settings (global ← round). Optional on Host (type assertion) so the
// minimal test hosts keep compiling; SetEffectsSource overrides it.
type EffectsSource interface {
	EffectsCatalogue() effects.Catalogue
	// EffectSettings returns the resolved effect id per moment for the room and, by
	// animateur group id, for the animateur groups (ambiancecfg LightEffects / LightHostEffects).
	EffectSettings() (room map[string]string, host map[string]map[string]string)
}

// EffectSpeedSource is optionally implemented next to EffectsSource: the resolved speed
// multiplier per moment for the room and, by animateur group id, for the animateur groups
// (ambiancecfg LightSpeeds / LightHostSpeeds). Absent ⇒ ×1 everywhere.
type EffectSpeedSource interface {
	EffectSpeeds() (room map[string]float64, host map[string]map[string]float64)
}

// effectFrames is the published result of one reader step.
type effectFrames struct {
	moment   effects.Moment
	room     effects.Frame         // zero = no room effect (ordinary scene)
	roomOn   bool                  // a room frame is live
	roomNums []int                 // real group numbers of the frame positions
	host     map[int]effects.Frame // by animateur rank k; a missing rank = fixed white
	// post holds, for the one step that follows the end of a run, the POST frame of the
	// effect that just ended (§4.5 point 5). The next scene covers it at once, so Scene()
	// never applies it; it is published so that the end of the moment is observable.
	post *effects.Frame
}

// effectRun is the bookkeeping of one running effect (the room, or one animateur group).
type effectRun struct {
	key    string
	start  time.Time
	tick   int
	nextAt time.Time
	eff    effects.Effect // for POST at the end of the run
	ctx    effects.Context
}

// advance moves the run to `now`: tick 0 at start, then one tick per real tick duration
// (recomputed at each tick, so a speed change is taken at the next one).
func (r *effectRun) advance(now time.Time, tickMs int, speed float64) {
	dur := effects.TickDuration(tickMs, speed)
	if r.nextAt.IsZero() {
		r.nextAt = r.start.Add(dur)
	}
	for guard := 0; !now.Before(r.nextAt) && guard < 10000; guard++ {
		r.tick++
		r.nextAt = r.nextAt.Add(effects.TickDuration(tickMs, speed))
	}
}

// effectsState is the reader's own bookkeeping.
type effectsState struct {
	mu    sync.Mutex
	room  *effectRun
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
// not declaring the moment, or not applicable (the caller then falls back to the default
// of the moment, §3.3). "" means no effect. host selects the animateur rule (§4.7).
func resolveEffect(cat effects.Catalogue, id string, m effects.Moment, n int, host bool) (effects.Entry, bool) {
	if id == "" {
		return effects.Entry{}, false
	}
	en, ok := cat.Get(id)
	if !ok || !en.Effect.HasMoment(m) {
		return effects.Entry{}, false
	}
	if host {
		if !en.Effect.HostApplicable() {
			return effects.Entry{}, false
		}
	} else if okApp, _ := en.Effect.Applicable(n); !okApp {
		return effects.Entry{}, false
	}
	return en, true
}

// resolveRoomEffect is resolveEffect with the fallback of §3.3: a moment set to an absent,
// invalid or inapplicable effect plays its default (the pulsation at the chrono moment, no
// effect elsewhere). A moment explicitly set to "" plays nothing.
func resolveRoomEffect(cat effects.Catalogue, id string, m effects.Moment, n int) (effects.Entry, bool) {
	if en, ok := resolveEffect(cat, id, m, n, false); ok {
		return en, true
	}
	if id != "" && m == effects.MomentChrono && id != ambiancecfg.PulsationID {
		return resolveEffect(cat, ambiancecfg.PulsationID, m, n, false)
	}
	return effects.Entry{}, false
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

func speedOf(m map[string]float64, moment string) float64 {
	if v, ok := m[moment]; ok && v > 0 {
		return v
	}
	return 1
}

// startOrAdvance returns the run for (key), restarting it when the key changed, and
// advances its tick counter to now.
func startOrAdvance(r *effectRun, key string, start, now time.Time, en effects.Entry, ctx effects.Context, speed float64) *effectRun {
	if r == nil || r.key != key {
		r = &effectRun{key: key, start: start}
	}
	r.advance(now, en.Effect.TickMs, speed)
	r.eff, r.ctx = en.Effect, ctx
	return r
}

// endPost returns the POST frame of a run that just ended, if its effect has one.
func endPost(r *effectRun) *effects.Frame {
	if r == nil || !r.eff.HasPost {
		return nil
	}
	if f, ok := effects.RenderPost(r.eff, r.ctx, r.tick); ok {
		return &f
	}
	return nil
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

	c.effects.mu.Lock()
	defer c.effects.mu.Unlock()
	if !ok {
		out := c.endRuns(nil)
		return out
	}
	cat := src.EffectsCatalogue()
	roomSet, hostSet := src.EffectSettings()
	var roomSpeed map[string]float64
	var hostSpeed map[string]map[string]float64
	if ss, okS := src.(EffectSpeedSource); okS {
		roomSpeed, hostSpeed = ss.EffectSpeeds()
	}
	lc := config.Get().Lighting
	groups := lc.RoomGroups()
	nums := make([]int, len(groups))
	for i, g := range groups {
		nums[i] = g.Num
	}

	sceneColor, _ := c.baseSceneColor(ev)
	ctx := effects.Context{
		Groups:    len(groups),
		Nums:      nums,
		Moment:    moment,
		Remaining: time.Duration(state.CurrentTime) * time.Second,
		Total:     time.Duration(state.Delay) * time.Second,
		Scene:     sceneColor,
		Theme:     c.ThemeColor(),
	}
	if moment == effects.MomentScore && scoreTeam != "" {
		ctx.Team = c.h.TeamNameColor(scoreTeam)
	}
	startAt := now
	keySuffix := ""
	if moment == effects.MomentScore {
		startAt, keySuffix = scoreStart, "|"+scoreStart.String()
	}

	out := &effectFrames{moment: moment}
	var post *effects.Frame

	// Room: only in AUTO mode without Flash (§5.3 point 2).
	var roomRun *effectRun
	if c.lightingMode() == ModeAuto && !c.isLightingFlashOn() {
		if en, okE := resolveRoomEffect(cat, roomSet[string(moment)], moment, len(groups)); okE {
			key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation) + keySuffix
			roomRun = startOrAdvance(c.effects.room, key, startAt, now, en, ctx, speedOf(roomSpeed, string(moment)))
			if f := effects.Render(en.Effect, ctx, roomRun.tick); f.Groups != nil || f.Base != nil {
				out.room, out.roomOn, out.roomNums = f, true, nums
			}
		}
	}
	if roomRun != c.effects.room && c.effects.room != nil && (roomRun == nil || roomRun.key != c.effects.room.key) {
		post = endPost(c.effects.room)
	}
	c.effects.room = roomRun

	// Animateur groups: each independent of the others, of mode, Flash and trial (§2.5, §5.3).
	live := map[int]*effectRun{}
	for _, g := range lc.HostGroups() {
		if len(g.Lights) == 0 {
			continue
		}
		en, okE := resolveEffect(cat, hostSet[g.ID][string(moment)], moment, 0, true)
		if !okE {
			continue
		}
		key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation) + keySuffix
		hctx := ctx
		hctx.Groups, hctx.Nums = 0, nil
		hctx.Scene = [3]int{255, 255, 255} // "scene" is white for an animateur group (§2.5)
		run := startOrAdvance(c.effects.host[g.Rank], key, startAt, now, en, hctx, speedOf(hostSpeed[g.ID], string(moment)))
		live[g.Rank] = run
		if f := effects.Render(en.Effect, hctx, run.tick); f.Base != nil {
			if out.host == nil {
				out.host = map[int]effects.Frame{}
			}
			out.host[g.Rank] = f
		}
	}
	for rank, old := range c.effects.host {
		if r, still := live[rank]; (!still || r.key != old.key) && post == nil {
			post = endPost(old)
		}
	}
	c.effects.host = live
	if !out.roomOn && out.host == nil && post == nil {
		return nil
	}
	out.post = post
	return out
}

// endRuns closes every run (the moment ended): the POST of the room's effect, if any, is
// published once.
func (c *Controller) endRuns(_ *effectFrames) *effectFrames {
	var post *effects.Frame
	if c.effects.room != nil {
		post = endPost(c.effects.room)
	}
	if post == nil {
		for _, r := range c.effects.host {
			if post = endPost(r); post != nil {
				break
			}
		}
	}
	c.effects.room, c.effects.host = nil, nil
	if post == nil {
		return nil
	}
	return &effectFrames{post: post}
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

// applyEffectFrames adds the published room frame to the zones of Scene(ev), only when it
// was computed for ev's moment: the state of g0 over the general zone, then one entry per
// group whose state differs from it, with the real group number (§4.6).
func (c *Controller) applyEffectFrames(ev lighting.Event, zones []lighting.ZoneState) []lighting.ZoneState {
	fr := c.effectFrames.Load()
	if fr == nil || !fr.roomOn || fr.moment != momentOfEvent(ev.Kind) {
		return zones
	}
	if c.lightingMode() != ModeAuto || c.isLightingFlashOn() {
		return zones
	}
	if b := fr.room.Base; b != nil {
		zones[0].Color, zones[0].Intensity, zones[0].TransitionMs = b.Color, b.Intensity, b.FadeMs
	}
	extra := make([]lighting.ZoneState, 0, len(fr.room.Groups))
	for i, g := range fr.room.Groups {
		if i >= len(fr.roomNums) || g.Absent {
			continue
		}
		extra = append(extra, lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: fr.roomNums[i], Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs})
	}
	return append(zones[:1], append(extra, zones[1:]...)...)
}

// hostEffectState returns the published animateur frame (its g0) for ev's moment, if any.
func (c *Controller) hostEffectState(ev lighting.Event, rank int) (lighting.ZoneState, bool) {
	fr := c.effectFrames.Load()
	if fr == nil || fr.host == nil || fr.moment != momentOfEvent(ev.Kind) {
		return lighting.ZoneState{}, false
	}
	f, ok := fr.host[rank]
	if !ok || f.Base == nil {
		return lighting.ZoneState{}, false
	}
	g := *f.Base
	return lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: lighting.HostGroup(rank), Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs}, true
}
