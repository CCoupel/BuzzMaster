package ambiance

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/lighting/hue"
)

// Group test sequence (v11.7.0, #225 — contracts/light-effects.md §2.4): six one-second
// steps played once on one group — off, white 50 %, white 100 %, red, green, blue — then
// back to the live state. It is a state carried by the Controller and read by Scene(),
// exactly like an effect frame: NO direct write to the driver, so there is no cache to
// invalidate and the return to the live state is a plain re-derivation (NotifyState).
//
// The current step is a function of the time elapsed since the start, read on the
// injectable clock (EffectsClock) — Scene() is deterministic; a goroutine only wakes the
// writer at each step boundary.

// GroupTestStepDefault is the duration of one step (contract: groupTestStepMs = 1000).
const GroupTestStepDefault = time.Second

// GroupTestSteps is the length of the sequence.
const GroupTestSteps = 6

// Errors of StartGroupTest (the HTTP layer maps both to 400).
var (
	ErrUnknownGroup = errors.New("groupe inconnu")
	ErrEmptyGroup   = errors.New("groupe vide")
)

type groupTestStep struct {
	color     [3]int
	intensity int
}

var groupTestSequence = [GroupTestSteps]groupTestStep{
	{[3]int{0, 0, 0}, 0},         // off
	{[3]int{255, 255, 255}, 128}, // white 50 %
	{[3]int{255, 255, 255}, 255}, // white 100 %
	{[3]int{255, 0, 0}, 255},     // red
	{[3]int{0, 255, 0}, 255},     // green
	{[3]int{0, 0, 255}, 255},     // blue
}

// testAddr addresses the tested group: its area and its number in that area.
type testAddr struct {
	area  lighting.Area
	group int
}

// groupTestState is the published state of the running test.
type groupTestState struct {
	addr  testAddr
	start time.Time
	step  time.Duration
}

// groupTestRuntime holds the running sequence.
type groupTestRuntime struct {
	state atomic.Pointer[groupTestState]
	epoch atomic.Int64
	// cancel is guarded by Controller.ambianceMu, like lightingFlashCancel.
	cancel context.CancelFunc
}

func (c *Controller) groupTestStepDuration() time.Duration {
	if c.GroupTestStep > 0 {
		return c.GroupTestStep
	}
	return GroupTestStepDefault
}

// GroupTestDurationMs is the total duration of a sequence with the current step.
func (c *Controller) GroupTestDurationMs() int {
	return int(c.groupTestStepDuration() * GroupTestSteps / time.Millisecond)
}

// resolveGroupRef turns "2" (group 2 of the ambiance zone), "host1" (group 1 of the
// animateur zone) or "public1" (group 1 of the public zone) into the state address and checks
// the group exists and is not empty.
func resolveGroupRef(ref string) (testAddr, error) {
	zone, numStr := config.ZoneAmbiance, ref
	if k, ok := strings.CutPrefix(ref, "host"); ok {
		zone, numStr = config.ZoneAnimateur, k
	} else if k, ok := strings.CutPrefix(ref, "public"); ok {
		zone, numStr = config.ZonePublic, k
	}
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 1 {
		return testAddr{}, ErrUnknownGroup
	}
	for _, g := range config.Get().Lighting.ZoneGroups(zone) {
		if g.Num == n {
			if len(g.Lights) == 0 {
				return testAddr{}, ErrEmptyGroup
			}
			return testAddr{area: areaOfZone(zone), group: n}, nil
		}
	}
	return testAddr{}, ErrUnknownGroup
}

// CheckGroupRef validates a group reference ("2", "host1") against the configuration.
func CheckGroupRef(ref string) error {
	_, err := resolveGroupRef(ref)
	return err
}

// StartGroupTest starts the sequence on the group designated by ref ("2" or "host1"),
// replacing a running one, and returns at once. ErrUnknownGroup / ErrEmptyGroup when the
// group does not exist or has no bulb.
func (c *Controller) StartGroupTest(ref string) error {
	addr, err := resolveGroupRef(ref)
	if err != nil {
		return err
	}
	step := c.groupTestStepDuration()
	c.StopEffectTrial()                               // a group test replaces a trial (§7.4)
	c.holdPublicFor(addr.area == lighting.AreaPublic) // public zone: read the bulbs' state first
	c.ambianceMu.Lock()
	if c.groupTest.cancel != nil {
		c.groupTest.cancel()
	}
	parent := c.h.Ctx()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c.groupTest.cancel = cancel
	epoch := c.groupTest.epoch.Add(1)
	st := &groupTestState{addr: addr, start: c.effectsNow(), step: step}
	c.groupTest.state.Store(st)
	c.ambianceMu.Unlock()
	c.Writer().NotifyState()
	c.releasePublicIfIdle() // a replaced public test followed by a non-public one gives the bulbs back
	go c.runGroupTest(ctx, epoch, st)
	return nil
}

// runGroupTest only wakes the writer at each step boundary and clears the state at the
// end; what Scene() shows is computed from the clock.
func (c *Controller) runGroupTest(ctx context.Context, epoch int64, st *groupTestState) {
	for i := 1; i <= GroupTestSteps; i++ {
		select {
		case <-ctx.Done():
			return // replaced or stopped: whoever cancelled already notified
		case <-time.After(st.step):
		}
		if c.groupTest.epoch.Load() != epoch {
			return
		}
		if i == GroupTestSteps {
			// End: back to the live state — only if THIS sequence is still the published one
			// (a replacement or a stop between the epoch check and here must not be erased).
			c.groupTest.state.CompareAndSwap(st, nil)
		}
		c.Writer().NotifyState()
	}
	c.releasePublicIfIdle() // public zone: give the bulbs back their state
}

// StopGroupTest stops the running sequence; the group returns to its live state. Safe
// when nothing runs.
func (c *Controller) StopGroupTest() {
	c.ambianceMu.Lock()
	if c.groupTest.cancel != nil {
		c.groupTest.cancel()
		c.groupTest.cancel = nil
	}
	c.groupTest.epoch.Add(1)
	was := c.groupTest.state.Swap(nil)
	c.ambianceMu.Unlock()
	if was != nil {
		c.Writer().NotifyState()
	}
	c.releasePublicIfIdle()
}

// GroupTestRunning reports whether a sequence is in progress (on the injected clock).
func (c *Controller) GroupTestRunning() bool {
	_, ok := c.currentGroupTest()
	return ok
}

// currentGroupTest returns the running test and its step index, false once it has ended.
func (c *Controller) currentGroupTest() (groupTestStep, bool) {
	return c.currentGroupTestAt(c.effectsNow())
}

func (c *Controller) currentGroupTestAt(now time.Time) (groupTestStep, bool) {
	st := c.groupTest.state.Load()
	if st == nil {
		return groupTestStep{}, false
	}
	i := int(now.Sub(st.start) / st.step)
	if i < 0 {
		i = 0
	}
	if i >= GroupTestSteps {
		return groupTestStep{}, false
	}
	return groupTestSequence[i], true
}

// applyGroupTest overlays the current step of the test on the zones of Scene(): it wins
// over the group's effect, ON / OFF and Flash for that group only.
func (c *Controller) applyGroupTest(zones []lighting.ZoneState) []lighting.ZoneState {
	s, ok := c.currentGroupTest()
	if !ok {
		return zones
	}
	addr := c.groupTest.state.Load().addr
	return overlayGroup(zones, lighting.ZoneState{Zone: lighting.ZoneGeneral, Area: addr.area, Group: addr.group, Color: s.color, Intensity: s.intensity})
}

// overlayGroup replaces the entry of the same (area, group) in zones, or adds it — after the
// room's Group 0 entry, which always stays first.
func overlayGroup(zones []lighting.ZoneState, z lighting.ZoneState) []lighting.ZoneState {
	for i := range zones {
		if zones[i].Zone == lighting.ZoneGeneral && zones[i].Area == z.Area && zones[i].Group == z.Group {
			zones[i] = z
			return zones
		}
	}
	return append(zones[:1], append([]lighting.ZoneState{z}, zones[1:]...)...)
}

// ---------------------------------------------------------------------------
// Public area: state read before a test or a trial, given back afterwards
// (light-effects.md §2.5). The public bulbs are never written by the scene, so after a test
// they would stay on its last colour: their state is read on the bridge when the first test
// or trial starts and written back when the last one ends. A failed read ⇒ switched off.
// ---------------------------------------------------------------------------

type publicHold struct {
	mu    sync.Mutex
	held  bool
	snaps []hue.LightSnapshot
	read  bool // the snapshot was read successfully
	// cancel aborts a restore write in flight (abandonPublic).
	cancel context.CancelFunc
}

func publicLightNames() []string {
	var names []string
	for _, g := range config.Get().Lighting.NonEmptyGroups(config.ZonePublic) {
		names = append(names, g.Lights...)
	}
	return names
}

// holdPublicFor snapshots the public bulbs when needed is true and no hold is active yet.
func (c *Controller) holdPublicFor(needed bool) {
	if !needed {
		return
	}
	c.public.mu.Lock()
	defer c.public.mu.Unlock()
	if c.public.held {
		return
	}
	c.public.held, c.public.read, c.public.snaps = true, false, nil
	drv := c.hueDriver.Load()
	if drv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if snaps, err := drv.SnapshotLights(ctx, publicLightNames()); err == nil {
		c.public.snaps, c.public.read = snaps, true
	}
}

// publicBusy reports whether a test or a trial is still driving the public zone.
func (c *Controller) publicBusy() bool {
	if st := c.groupTest.state.Load(); st != nil && st.addr.area == lighting.AreaPublic {
		if _, running := c.currentGroupTest(); running {
			return true
		}
	}
	return c.trialOnPublic()
}

// releasePublicIfIdle writes the snapshot back once nothing drives the public zone any more,
// unless a live effect now owns it.
func (c *Controller) releasePublicIfIdle() {
	if c.publicBusy() {
		return
	}
	c.public.mu.Lock()
	if !c.public.held {
		c.public.mu.Unlock()
		return
	}
	snaps, read := c.public.snaps, c.public.read
	c.public.held, c.public.snaps = false, nil
	c.public.mu.Unlock()
	if fr := c.effectFrames.Load(); fr != nil && fr.zones[config.ZonePublic].on {
		return // a live effect owns the zone: it keeps its state
	}
	drv := c.hueDriver.Load()
	if drv == nil {
		return
	}
	names := publicLightNames()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	c.public.mu.Lock()
	c.public.cancel = cancel
	c.public.mu.Unlock()
	go func() {
		defer cancel()
		if read {
			_ = drv.RestoreLights(ctx, snaps, nil)
		} else {
			_ = drv.RestoreLights(ctx, nil, names) // unreadable: switched off
		}
	}()
}

// abandonPublic drops the public zone's hold WITHOUT giving the bulbs back their state, and
// aborts a restore already in flight: used by the shutdown and the master switch, where the
// extinction must be the last word on the public bulbs (a late restore could otherwise light
// them again).
func (c *Controller) abandonPublic() {
	c.public.mu.Lock()
	c.public.held, c.public.snaps, c.public.read = false, nil, false
	cancel := c.public.cancel
	c.public.cancel = nil
	c.public.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
