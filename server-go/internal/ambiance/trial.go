package ambiance

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// Effect trial (v11.7.0, #223 — contracts/light-effects.md §7.4, revision 4): plays one effect on
// the bulbs of ONE zone for a few seconds, then gives the zone back its live state. Same
// mechanism as the group test: a state carried by the Controller and read by Scene() — no
// direct write to the driver. The tick is computed from the clock, at the chosen speed; the
// context is simulated (theme, team colour, a chrono counting down from 15 s).

// ErrTrialUnknownEffect is returned for an effect id that is not in the catalogue.
var ErrTrialUnknownEffect = errors.New("effet inconnu")

// TrialError is a refusal with the reason of §7.4: "disabled", "game", "not_applicable" or
// "mode".
type TrialError struct{ Reason string }

func (e *TrialError) Error() string { return "essai refusé : " + e.Reason }

// TrialReason is the 409 reason (server.trialRefusal).
func (e *TrialError) TrialReason() string { return e.Reason }

// Simulated context of a trial (§7.4).
var (
	trialTheme = [3]int{0x4F, 0x46, 0xE5}
	trialTeam  = [3]int{0xE5, 0x39, 0x35}
)

const trialChronoStart = 15 * time.Second

type trialState struct {
	zone  string
	en    effects.Entry
	speed float64
	start time.Time
	end   time.Time
	nums  []int
}

type trialRuntime struct {
	state atomic.Pointer[trialState]
	epoch atomic.Int64
	// cancel is guarded by Controller.ambianceMu.
	cancel context.CancelFunc
}

// TrialRunning reports whether a trial is in progress (on the injected clock).
func (c *Controller) TrialRunning() bool {
	st := c.trial.state.Load()
	return st != nil && c.effectsNow().Before(st.end)
}

func (c *Controller) trialOnPublic() bool {
	st := c.trial.state.Load()
	return st != nil && st.zone == config.ZonePublic && c.effectsNow().Before(st.end)
}

// trialEntries returns the entries of the trial for zone at now, when a trial drives it.
func (c *Controller) trialEntries(zone string, now time.Time) ([]lighting.ZoneState, bool) {
	st := c.trial.state.Load()
	if st == nil || st.zone != zone || !now.Before(st.end) {
		return nil, false
	}
	elapsed := now.Sub(st.start)
	ctx := effects.Context{
		Groups: len(st.nums), Nums: st.nums,
		Moment:    effects.Moment(st.en.Effect.Moments[0]),
		Remaining: trialChronoStart - elapsed,
		Total:     trialChronoStart,
		Scene:     [3]int{255, 255, 255},
		Theme:     trialTheme,
		Team:      trialTeam,
	}
	if ctx.Remaining < 0 {
		ctx.Remaining = 0
	}
	tick := int(elapsed / effects.TickDuration(st.en.Effect.TickMs, st.speed))
	f := effects.Render(st.en.Effect, ctx, tick)
	if f.Base == nil && f.Groups == nil {
		return nil, false
	}
	return frameEntries(areaOfZone(zone), f, st.nums), true
}

// StartEffectTrial starts a trial of effect id on zone at the given speed for seconds
// seconds, replacing a running trial or group test, and returns the duration in ms. Errors:
// ErrTrialUnknownEffect, or a *TrialError carrying the reason of the 409.
func (c *Controller) StartEffectTrial(id, zone string, speed float64, seconds int) (int, error) {
	src := c.effectsSource()
	if src == nil {
		return 0, &TrialError{Reason: "disabled"}
	}
	en, ok := src.EffectsCatalogue().Get(id)
	if !ok {
		return 0, ErrTrialUnknownEffect
	}
	lc := config.Get().Lighting
	if !lc.Enabled || c.Writer() == nil || c.effectsQuiet.Load() {
		return 0, &TrialError{Reason: "disabled"}
	}
	if c.h.Engine() != nil {
		switch c.h.Engine().GetState().Phase {
		case game.PhaseStopped, game.PhaseNewGame, game.PhaseEnroll:
		default:
			return 0, &TrialError{Reason: "game"}
		}
	}
	if !lc.ZoneExists(zone) {
		return 0, &TrialError{Reason: "not_applicable"}
	}
	groups := lc.NonEmptyGroups(zone)
	if okApp, _ := en.Effect.Applicable(len(groups)); !okApp {
		return 0, &TrialError{Reason: "not_applicable"}
	}
	if zone == config.ZoneAmbiance && (c.lightingMode() != ModeAuto || c.isLightingFlashOn()) {
		return 0, &TrialError{Reason: "mode"}
	}
	nums := make([]int, len(groups))
	for i, g := range groups {
		nums[i] = g.Num
	}

	c.StopGroupTest() // a trial replaces a group test (§7.4)
	c.holdPublicFor(zone == config.ZonePublic)
	now := c.effectsNow()
	dur := time.Duration(seconds) * time.Second
	st := &trialState{zone: zone, en: en, speed: speed, start: now, end: now.Add(dur), nums: nums}
	c.ambianceMu.Lock()
	if c.trial.cancel != nil {
		c.trial.cancel()
	}
	parent := c.h.Ctx()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c.trial.cancel = cancel
	epoch := c.trial.epoch.Add(1)
	c.trial.state.Store(st)
	c.ambianceMu.Unlock()
	c.Writer().NotifyState()
	c.releasePublicIfIdle() // a previous public trial replaced by a non-public one
	go c.runTrial(ctx, epoch, st, dur)
	return int(dur / time.Millisecond), nil
}

// runTrial wakes the writer every reader period during the trial (the frame is computed
// from the clock) and clears the state at the end.
func (c *Controller) runTrial(ctx context.Context, epoch int64, st *trialState, dur time.Duration) {
	tick := c.EffectsTick
	if tick <= 0 {
		tick = EffectsTickInterval
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	deadline := time.NewTimer(dur)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return // replaced or stopped: whoever cancelled already notified
		case <-deadline.C:
			if c.trial.epoch.Load() == epoch {
				c.trial.state.CompareAndSwap(st, nil)
				c.Writer().NotifyState()
				c.releasePublicIfIdle()
			}
			return
		case <-t.C:
			c.Writer().NotifyState()
		}
	}
}

// StopEffectTrial stops the running trial; the zone returns to its live state. Safe when
// nothing runs.
func (c *Controller) StopEffectTrial() {
	c.ambianceMu.Lock()
	if c.trial.cancel != nil {
		c.trial.cancel()
		c.trial.cancel = nil
	}
	c.trial.epoch.Add(1)
	was := c.trial.state.Swap(nil)
	c.ambianceMu.Unlock()
	if was != nil {
		c.Writer().NotifyState()
	}
	c.releasePublicIfIdle()
}
