package ambiance

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting"
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

// groupTestState is the published state of the running test.
type groupTestState struct {
	addr  int // ZoneState.Group of the tested group: n >= 1 (salle) or -k (animateur)
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

// resolveGroupRef turns "2" (salle group 2) or "host1" (animateur of rank 1) into the
// state address and checks the group exists and is not empty.
func resolveGroupRef(ref string) (int, error) {
	lc := config.Get().Lighting
	if k, ok := strings.CutPrefix(ref, "host"); ok {
		rank, err := strconv.Atoi(k)
		if err != nil || rank < 1 {
			return 0, ErrUnknownGroup
		}
		for _, g := range lc.HostGroups() {
			if g.Rank == rank {
				if len(g.Lights) == 0 {
					return 0, ErrEmptyGroup
				}
				return lighting.HostGroup(rank), nil
			}
		}
		return 0, ErrUnknownGroup
	}
	n, err := strconv.Atoi(ref)
	if err != nil || n < 1 {
		return 0, ErrUnknownGroup
	}
	rank := 0
	for _, g := range lc.Groups {
		if g.Role == config.GroupRoleAnimateur {
			continue
		}
		rank++
		if rank == n {
			if len(g.Lights) == 0 {
				return 0, ErrEmptyGroup
			}
			return n, nil
		}
	}
	return 0, ErrUnknownGroup
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
	z := lighting.ZoneState{Zone: lighting.ZoneGeneral, Group: addr, Color: s.color, Intensity: s.intensity}
	for i := range zones {
		if zones[i].Zone == lighting.ZoneGeneral && zones[i].Group == addr {
			zones[i] = z
			return zones
		}
	}
	// The group has no entry of its own yet (plain room scene): add it after Group 0.
	return append(zones[:1], append([]lighting.ZoneState{z}, zones[1:]...)...)
}
