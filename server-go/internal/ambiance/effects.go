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
	// EffectSettings returns the resolved effect id by zone (ambiance, animateur, public) then
	// moment (ambiancecfg Settings.LightEffects).
	EffectSettings() map[string]map[string]string
}

// EffectSpeedSource is optionally implemented next to EffectsSource: the resolved speed
// multiplier by zone then moment (ambiancecfg Settings.LightSpeeds). Absent ⇒ ×1 everywhere.
type EffectSpeedSource interface {
	EffectSpeeds() map[string]map[string]float64
}

// zoneFrame is what a zone's live effect shows after the current tick.
type zoneFrame struct {
	on    bool // an effect frame is live for this zone
	frame effects.Frame
	nums  []int // real numbers of the groups the frame positions stand for
}

// effectFrames is the published result of one reader step.
type effectFrames struct {
	moment effects.Moment
	zones  map[string]zoneFrame
	// post holds, for a short window after the end of a run, the POST frame of the effect that
	// just ended (§4.5 point 5). Only the public zone shows it: the ambiance and animateur
	// zones are repainted at once by the scene that follows.
	post map[string]*postFrame
}

type postFrame struct {
	frame effects.Frame
	nums  []int
	until time.Time
}

// postHold is how long a POST frame is published (long enough for the writer to apply it).
const postHold = 600 * time.Millisecond

// effectRun is the bookkeeping of one running effect (one zone).
type effectRun struct {
	key    string
	start  time.Time
	tick   int
	nextAt time.Time
	eff    effects.Effect // for POST at the end of the run
	ctx    effects.Context
	nums   []int
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
	runs  map[string]*effectRun // by zone
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

// resolveEffect returns the catalogue entry to play for the setting id in a zone, or false:
// absent, not declaring the moment, or not applicable to the zone's n groups (§4.7). "" means
// no effect.
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

// resolveZoneEffect is resolveEffect with the default of the zone (§6): a moment set to an
// absent, invalid or inapplicable effect plays the room's pulsation at the chrono moment and
// no effect anywhere else. A moment explicitly set to "" plays nothing.
func resolveZoneEffect(cat effects.Catalogue, zone, id string, m effects.Moment, n int) (effects.Entry, bool) {
	if en, ok := resolveEffect(cat, id, m, n); ok {
		return en, true
	}
	if zone == config.ZoneAmbiance && id != "" && m == effects.MomentChrono && id != ambiancecfg.PulsationID {
		return resolveEffect(cat, ambiancecfg.PulsationID, m, n)
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

// endPost returns the POST frame of a run that just ended, if its effect has one.
func endPost(r *effectRun, now time.Time) *postFrame {
	if r == nil || !r.eff.HasPost {
		return nil
	}
	if f, ok := effects.RenderPost(r.eff, r.ctx, r.tick); ok {
		return &postFrame{frame: f, nums: r.nums, until: now.Add(postHold)}
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
	// A POST published a moment ago stays for its window.
	out := &effectFrames{zones: map[string]zoneFrame{}, post: map[string]*postFrame{}}
	if prev := c.effectFrames.Load(); prev != nil {
		for z, pf := range prev.post {
			if pf != nil && now.Before(pf.until) {
				out.post[z] = pf
			}
		}
	}
	finish := func() *effectFrames {
		if len(out.zones) == 0 && len(out.post) == 0 {
			return nil
		}
		return out
	}
	if !ok {
		for z, r := range c.effects.runs {
			if pf := endPost(r, now); pf != nil {
				out.post[z] = pf
			}
		}
		c.effects.runs = nil
		return finish()
	}
	out.moment = moment
	cat := src.EffectsCatalogue()
	settings := src.EffectSettings()
	var speeds map[string]map[string]float64
	if ss, okS := src.(EffectSpeedSource); okS {
		speeds = ss.EffectSpeeds()
	}
	lc := config.Get().Lighting

	sceneColor, _ := c.baseSceneColor(ev)
	base := effects.Context{
		Moment:    moment,
		Remaining: time.Duration(state.CurrentTime) * time.Second,
		Total:     time.Duration(state.Delay) * time.Second,
		Theme:     c.ThemeColor(),
	}
	if moment == effects.MomentScore && scoreTeam != "" {
		base.Team = c.h.TeamNameColor(scoreTeam)
	}
	startAt, keySuffix := now, ""
	if moment == effects.MomentScore {
		startAt, keySuffix = scoreStart, "|"+scoreStart.String()
	}

	live := map[string]*effectRun{}
	for _, zone := range config.Zones {
		if !lc.ZoneExists(zone) {
			continue
		}
		// The room obeys ON / OFF / Flash (§5.3 point 2); the other zones never do.
		if zone == config.ZoneAmbiance && (c.lightingMode() != ModeAuto || c.isLightingFlashOn()) {
			continue
		}
		groups := lc.NonEmptyGroups(zone)
		nums := make([]int, len(groups))
		for i, g := range groups {
			nums[i] = g.Num
		}
		en, okE := resolveZoneEffect(cat, zone, settings[zone][string(moment)], moment, len(groups))
		if !okE {
			continue
		}
		ctx := base
		ctx.Groups, ctx.Nums = len(groups), nums
		ctx.Scene = [3]int{255, 255, 255} // "scene" is white outside the room (§2.5)
		if zone == config.ZoneAmbiance {
			ctx.Scene = sceneColor
		}
		key := string(moment) + "|" + en.Effect.ID + "|" + itoa(cat.Generation) + keySuffix
		run := c.effects.runs[zone]
		if run == nil || run.key != key {
			run = &effectRun{key: key, start: startAt}
		}
		run.advance(now, en.Effect.TickMs, speedOf(speeds[zone], string(moment)))
		run.eff, run.ctx, run.nums = en.Effect, ctx, nums
		live[zone] = run
		if f := effects.Render(en.Effect, ctx, run.tick); f.Groups != nil || f.Base != nil {
			out.zones[zone] = zoneFrame{on: true, frame: f, nums: nums}
		}
	}
	for z, old := range c.effects.runs { // a run that stopped or changed: its POST
		if r, still := live[z]; !still || r.key != old.key {
			if pf := endPost(old, now); pf != nil {
				out.post[z] = pf
			}
		}
	}
	c.effects.runs = live
	return finish()
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

// frameEntries turns a zone frame into the entries of its area: the state of g0 on Group 0
// (when written), then one entry per group whose state differs from it (§4.6).
func frameEntries(area lighting.Area, f effects.Frame, nums []int) []lighting.ZoneState {
	var out []lighting.ZoneState
	if b := f.Base; b != nil {
		out = append(out, lighting.ZoneState{Zone: lighting.ZoneGeneral, Area: area, Color: b.Color, Intensity: b.Intensity, TransitionMs: b.FadeMs})
	}
	for i, g := range f.Groups {
		if i >= len(nums) || g.Absent {
			continue
		}
		out = append(out, lighting.ZoneState{Zone: lighting.ZoneGeneral, Area: area, Group: nums[i], Color: g.Color, Intensity: g.Intensity, TransitionMs: g.FadeMs})
	}
	return out
}

// zoneEntries adds to the zones of Scene(ev) what the animateur and public zones show
// (§2.5): the animateur zone its live effect or a fixed white; the public zone only a live
// effect, a trial or a POST being held — otherwise nothing at all.
func (c *Controller) zoneEntries(ev lighting.Event, zones []lighting.ZoneState) []lighting.ZoneState {
	lc := config.Get().Lighting
	fr := c.effectFrames.Load()
	now := c.effectsNow()
	sameMoment := fr != nil && fr.moment == momentOfEvent(ev.Kind)
	for _, zone := range []string{config.ZoneAnimateur, config.ZonePublic} {
		if !lc.ZoneExists(zone) {
			continue
		}
		area := areaOfZone(zone)
		var entries []lighting.ZoneState
		switch tr, trOn := c.trialEntries(zone, now); {
		case trOn:
			entries = tr
		case fr != nil && sameMoment && fr.zones[zone].on:
			entries = frameEntries(area, fr.zones[zone].frame, fr.zones[zone].nums)
		case zone == config.ZonePublic && fr != nil && fr.post[zone] != nil && now.Before(fr.post[zone].until):
			entries = frameEntries(area, fr.post[zone].frame, fr.post[zone].nums)
		}
		if zone == config.ZoneAnimateur && !hasGroupZero(entries, area) {
			// no effect (or an effect that never wrote g0): fixed white under it
			entries = append([]lighting.ZoneState{{Zone: lighting.ZoneGeneral, Area: area, Color: [3]int{255, 255, 255}, Intensity: 255}}, entries...)
		}
		zones = append(zones, entries...)
	}
	return zones
}

func hasGroupZero(entries []lighting.ZoneState, area lighting.Area) bool {
	for _, e := range entries {
		if e.Area == area && e.Group == 0 {
			return true
		}
	}
	return false
}

// applyEffectFrames adds the published room frame to the zones of Scene(ev), only when it was
// computed for ev's moment (or a trial drives the room): the state of g0 over the general
// zone, then one entry per group whose state differs from it, with the real group number.
func (c *Controller) applyEffectFrames(ev lighting.Event, zones []lighting.ZoneState) []lighting.ZoneState {
	if c.lightingMode() != ModeAuto || c.isLightingFlashOn() {
		return zones
	}
	var entries []lighting.ZoneState
	if tr, on := c.trialEntries(config.ZoneAmbiance, c.effectsNow()); on {
		entries = tr
	} else if fr := c.effectFrames.Load(); fr != nil && fr.moment == momentOfEvent(ev.Kind) && fr.zones[config.ZoneAmbiance].on {
		entries = frameEntries(lighting.AreaAmbiance, fr.zones[config.ZoneAmbiance].frame, fr.zones[config.ZoneAmbiance].nums)
	}
	for _, e := range entries {
		if e.Group == 0 {
			zones[0].Color, zones[0].Intensity, zones[0].TransitionMs = e.Color, e.Intensity, e.TransitionMs
			continue
		}
		zones = append(zones[:1], append([]lighting.ZoneState{e}, zones[1:]...)...)
	}
	return zones
}
