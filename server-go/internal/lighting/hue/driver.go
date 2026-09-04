// Package hue implements lighting.Driver for a Philips Hue Bridge over its
// local REST API v1 (contracts/hue-bridge.md, #206).
//
// The driver is only ever called from the single writer goroutine
// (contracts/lighting.md §4), so Apply may block; Status() may however be
// read from HTTP handlers (#207), hence the small mutex around bookkeeping.
// No goroutine is started, ever: when lighting is not configured the owner
// simply does not create a Driver (contract §5.5).
package hue

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"buzzcontrol/internal/lighting"
)

const (
	// RecommendedMinInterval is the writer pacing for this driver: one Apply is
	// up to N HTTP writes at ~40 ms each, where #205 reasoned on a single
	// write (contract §5.4). Wired by #207 into lighting.Config.MinInterval.
	RecommendedMinInterval = 250 * time.Millisecond
	// TransitionTime is sent with every write: 0 = instant. The bridge default
	// (400 ms) would wash out an event flash (contract §5.2).
	TransitionTime = 0
	// DefaultRefreshEvery is how often the light inventory is re-read and the
	// name→id resolution re-verified (contract §4.2: at resolution and
	// periodically, not before every write).
	DefaultRefreshEvery = 5 * time.Minute
	// DefaultDiscoverTimeout bounds the re-discovery by bridge id when the IP
	// changed (contract §4.1).
	DefaultDiscoverTimeout = 3 * time.Second
	// backoff for an unreachable bridge (contract §5.5): 1 s → 60 s.
	backoffMin = time.Second
	backoffMax = 60 * time.Second
)

// LightRole of a configured light.
type LightRole string

const (
	RoleGeneral LightRole = "general"
	RoleTeam    LightRole = "team" // activated by #213
)

// LightSpec is one configured light (contract §6): addressed by NAME.
type LightSpec struct {
	Name string    `json:"name"`
	Role LightRole `json:"role"`
	Team string    `json:"team,omitempty"`
}

// Config of a Driver — filled by #207 from config.json's `lighting` section.
type Config struct {
	BridgeIP string // plain IP/host (http://<ip> unless HTTPS) or scheme://host
	Base     string // full scheme://host[:port] override (tests: httptest.Server.URL); wins over BridgeIP
	BridgeID string // authoritative identity; the IP may change (DHCP)
	APIKey   string
	HTTPS    bool
	Lights   []LightSpec

	Timeout         time.Duration // HTTP, default HTTPTimeout (2 s)
	RefreshEvery    time.Duration // default DefaultRefreshEvery
	DiscoverTimeout time.Duration // default DefaultDiscoverTimeout

	// Logger receives ONE line per state change (ok/unreachable/refused,
	// resolution changes, IP change). Never one line per failure. nil = silent.
	Logger func(format string, args ...any)
	// Now is injectable for tests.
	Now func() time.Time
	// FindBridge overrides re-discovery by id (tests). nil = FindByID.
	FindBridge func(ctx context.Context, id string, timeout time.Duration) (Bridge, bool, error)
}

// Stats counts what the driver did (diagnostics).
type Stats struct {
	Applies     int `json:"applies"`
	Writes      int `json:"writes"`
	Skipped     int `json:"skipped_unchanged"`
	WriteErrors int `json:"write_errors"`
	Inventories int `json:"inventories"`
}

// LightStatus is the per-light part of Status.
type LightStatus struct {
	Name      string    `json:"name"`
	Role      LightRole `json:"role"`
	Team      string    `json:"team,omitempty"`
	ID        string    `json:"id,omitempty"`
	Resolved  bool      `json:"resolved"`
	Ambiguous bool      `json:"ambiguous,omitempty"`
	Reachable bool      `json:"reachable"`
	LastError string    `json:"last_error,omitempty"`
}

// Status is the driver's known state — read without any I/O (contract §7 /status).
type Status struct {
	State       BridgeState   `json:"state"`
	Reason      string        `json:"reason,omitempty"`
	BridgeID    string        `json:"bridge_id"`
	BridgeIP    string        `json:"bridge_ip"`
	Bridge      BridgeInfo    `json:"bridge_info"`
	LightsTotal int           `json:"lights_total"`
	LightsOK    int           `json:"lights_ok"`
	Lights      []LightStatus `json:"lights"`
	LastChange  time.Time     `json:"last_change"`
	NextRetry   time.Time     `json:"next_retry,omitempty"`
	Stats       Stats         `json:"stats"`
}

// applied is the last state effectively written to a light (contract §5.3).
type applied struct {
	on  bool
	bri int
	xy  [2]float64
}

func (a applied) toV1() stateV1 {
	tt := TransitionTime
	if !a.on {
		off := false
		return stateV1{On: &off, TransitionTime: &tt}
	}
	on := true
	bri := a.bri
	xy := a.xy
	return stateV1{On: &on, Bri: &bri, XY: &xy, TransitionTime: &tt}
}

// desired maps a ZoneState to the state to write (contract §5.2).
func desired(z lighting.ZoneState) applied {
	if z.Intensity <= 0 {
		return applied{on: false}
	}
	return applied{on: true, bri: intensityToBri(z.Intensity), xy: rgbToXY(z.Color[0], z.Color[1], z.Color[2])}
}

type resolvedLight struct {
	id        string
	reachable bool
}

// Driver implements lighting.Driver for one bridge.
type Driver struct {
	cfg    Config
	now    func() time.Time
	client *client
	base   string

	mu            sync.Mutex // protects everything below; never held during I/O
	status        BridgeState
	reported      bool // false until the first ok/fail has been logged (initial status is provisional)
	reason        string
	lastChange    time.Time
	failures      int
	nextRetry     time.Time
	bridgeInfo    BridgeInfo
	bridgeChecked bool
	resolved      map[string]resolvedLight // by configured name
	ambiguous     map[string]bool
	lightErr      map[string]string
	lastInventory time.Time
	appliedState  map[string]applied // by configured name
	stats         Stats
	closed        bool
	invalid       error // set by NewDriver on an invalid configuration
}

var _ lighting.Driver = (*Driver)(nil)

// New validates cfg and builds a Driver. It performs NO network I/O.
func New(cfg Config) (*Driver, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("hue: api key is required")
	}
	if strings.TrimSpace(cfg.Base) != "" {
		cfg.BridgeIP = strings.TrimSpace(cfg.Base)
	}
	if strings.TrimSpace(cfg.BridgeIP) == "" && strings.TrimSpace(cfg.BridgeID) == "" {
		return nil, errors.New("hue: bridge ip or bridge id is required")
	}
	if strings.ContainsAny(cfg.APIKey, "/?#") {
		return nil, errors.New("hue: api key contains invalid characters")
	}
	seen := map[string]bool{}
	for i, l := range cfg.Lights {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			return nil, fmt.Errorf("hue: light %d has an empty name", i)
		}
		if seen[name] {
			return nil, fmt.Errorf("hue: light %q is configured twice", name)
		}
		seen[name] = true
		switch l.Role {
		case RoleGeneral, "":
			cfg.Lights[i].Role = RoleGeneral
		case RoleTeam:
			if strings.TrimSpace(l.Team) == "" {
				return nil, fmt.Errorf("hue: light %q has role team but no team", name)
			}
		default:
			return nil, fmt.Errorf("hue: light %q has unknown role %q", name, l.Role)
		}
		cfg.Lights[i].Name = name
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = HTTPTimeout
	}
	if cfg.RefreshEvery <= 0 {
		cfg.RefreshEvery = DefaultRefreshEvery
	}
	if cfg.DiscoverTimeout <= 0 {
		cfg.DiscoverTimeout = DefaultDiscoverTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.FindBridge == nil {
		cfg.FindBridge = FindByID
	}
	d := &Driver{
		cfg:          cfg,
		now:          cfg.Now,
		status:       StateUnreachable, // nothing verified yet; flips to ok on first contact
		reason:       "not contacted yet",
		resolved:     map[string]resolvedLight{},
		ambiguous:    map[string]bool{},
		lightErr:     map[string]string{},
		appliedState: map[string]applied{},
	}
	if strings.TrimSpace(cfg.BridgeIP) != "" {
		base, err := bridgeBase(cfg.BridgeIP, cfg.HTTPS)
		if err != nil {
			return nil, err
		}
		d.base = base
		d.client = newClient(base, cfg.APIKey, cfg.Timeout)
	}
	return d, nil
}

// NewDriver is New without the error return (test-writer seam): an invalid
// configuration yields a Driver that performs no I/O, whose Apply returns
// the validation error and whose Status is StateRefused with the reason.
func NewDriver(cfg Config) *Driver {
	d, err := New(cfg)
	if err == nil {
		return d
	}
	return &Driver{
		cfg: cfg, now: time.Now, status: StateRefused, reason: "invalid config: " + err.Error(), reported: true,
		resolved: map[string]resolvedLight{}, ambiguous: map[string]bool{}, lightErr: map[string]string{}, appliedState: map[string]applied{},
		invalid: err,
	}
}

func (d *Driver) logf(format string, args ...any) {
	if d.cfg.Logger != nil {
		d.cfg.Logger(format, args...)
	}
}

// Close releases the HTTP transport. Idempotent.
func (d *Driver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.client.close()
	return nil
}

// Status returns the known state without any I/O (contract §7).
func (d *Driver) Status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := Status{
		State: d.status, Reason: d.reason, BridgeID: d.cfg.BridgeID, BridgeIP: d.base,
		Bridge: d.bridgeInfo, LightsTotal: len(d.cfg.Lights), LastChange: d.lastChange,
		NextRetry: d.nextRetry, Stats: d.stats,
	}
	if d.bridgeInfo.BridgeID != "" {
		r.BridgeID = d.bridgeInfo.BridgeID
	}
	for _, lc := range d.cfg.Lights {
		ls := LightStatus{Name: lc.Name, Role: lc.Role, Team: lc.Team, Ambiguous: d.ambiguous[lc.Name], LastError: d.lightErr[lc.Name]}
		if rl, ok := d.resolved[lc.Name]; ok {
			ls.ID, ls.Resolved, ls.Reachable = rl.id, true, rl.reachable
			if rl.reachable && ls.LastError == "" {
				r.LightsOK++
			}
		}
		r.Lights = append(r.Lights, ls)
	}
	return r
}

// ---------------------------------------------------------------------------
// Apply
// ---------------------------------------------------------------------------

// Apply writes the zones of st to the configured lights — only the lights
// whose desired state differs from the last applied one (contract §5.3).
// Per-light Hue errors never abort the others and are reported through
// Status(), not as an error (contract §5.5); bridge-level failures return a
// classified error (ErrUnreachable / ErrRefused) and are rate-limited by
// the backoff so the writer is never blocked by a dead bridge.
func (d *Driver) Apply(ctx context.Context, st lighting.State) error {
	d.mu.Lock()
	d.stats.Applies++
	closed, invalid := d.closed, d.invalid
	status, nextRetry := d.status, d.nextRetry
	d.mu.Unlock()
	if invalid != nil {
		return fmt.Errorf("%w: invalid config: %v", ErrRefused, invalid)
	}
	if closed {
		return errors.New("hue: driver closed")
	}
	now := d.now()
	switch {
	case status == StateRefused:
		return ErrRefused // no I/O, no retry loop: a user gesture unblocks (Refresh)
	case status == StateUnreachable && now.Before(nextRetry):
		return ErrUnreachable // fast fail during backoff
	}
	if err := d.ensureResolved(ctx, false); err != nil {
		return d.fail(err)
	}

	d.mu.Lock()
	plan := make([]plannedWrite, 0, len(d.cfg.Lights))
	for _, lc := range d.cfg.Lights {
		rl, ok := d.resolved[lc.Name]
		if !ok {
			continue // missing/ambiguous: logged at resolution, silently skipped here
		}
		zs, ok := zoneFor(lc, st)
		if !ok {
			continue // zone without configured lights or light without zone: non-event
		}
		want := desired(zs)
		if prev, ok := d.appliedState[lc.Name]; ok && prev == want {
			d.stats.Skipped++
			continue
		}
		plan = append(plan, plannedWrite{name: lc.Name, id: rl.id, want: want})
	}
	d.mu.Unlock()

	for _, w := range plan {
		err := d.client.setState(ctx, w.id, w.want.toV1())
		d.mu.Lock()
		d.stats.Writes++
		if err != nil {
			delete(d.appliedState, w.name) // invalidate: retried on the next Apply
			d.stats.WriteErrors++
			d.mu.Unlock()
			cerr := classify(err)
			if errors.Is(cerr, ErrUnreachable) || errors.Is(cerr, ErrRefused) {
				return d.fail(cerr) // bridge-level: stop this Apply
			}
			d.mu.Lock()
			d.lightErr[w.name] = err.Error() // per-light (e.g. 201 device off): others continue
			d.mu.Unlock()
			continue
		}
		d.appliedState[w.name] = w.want
		delete(d.lightErr, w.name)
		d.mu.Unlock()
	}
	d.ok()
	return nil
}

type plannedWrite struct {
	name string
	id   string
	want applied
}

// zoneFor picks the ZoneState a light follows (contract §5.2): a team light
// follows its team's zone when present in st, otherwise the general zone.
func zoneFor(lc LightSpec, st lighting.State) (lighting.ZoneState, bool) {
	if lc.Role == RoleTeam {
		for _, z := range st.Zones {
			if z.Zone == lc.Team {
				return z, true
			}
		}
	}
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral {
			return z, true
		}
	}
	return lighting.ZoneState{}, false
}

// ---------------------------------------------------------------------------
// Resolution, bridge identity, status bookkeeping
// ---------------------------------------------------------------------------

// ensureResolved reads the inventory when it is stale (or forced), verifies
// the bridge identity on first contact, and resolves configured names to
// ids. Returns a raw error (caller classifies through fail()).
func (d *Driver) ensureResolved(ctx context.Context, force bool) error {
	d.mu.Lock()
	fresh := !force && d.resolved != nil && !d.lastInventory.IsZero() && d.now().Sub(d.lastInventory) < d.cfg.RefreshEvery
	checked := d.bridgeChecked
	d.mu.Unlock()
	if fresh {
		return nil
	}
	if d.client == nil { // only an id is known: discover first
		if err := d.rediscover(ctx, "no ip configured"); err != nil {
			return err
		}
	}
	if !checked {
		if err := d.checkBridgeIdentity(ctx); err != nil {
			return err
		}
	}
	lights, err := d.client.lights(ctx)
	if err != nil {
		// IP may have changed (DHCP): one re-discovery by id, then retry once.
		if d.cfg.BridgeID != "" && errors.Is(classify(err), ErrUnreachable) {
			if rerr := d.rediscover(ctx, "inventory unreachable"); rerr == nil {
				lights, err = d.client.lights(ctx)
			}
		}
		if err != nil {
			return err
		}
	}
	d.resolve(lights)
	return nil
}

// checkBridgeIdentity reads /config once; a bridge id mismatch means another
// bridge answers at this IP — re-discover by id (contract §4.1).
func (d *Driver) checkBridgeIdentity(ctx context.Context) error {
	info, err := d.client.config(ctx)
	if err != nil {
		// A bridge (or fake) that does not serve /config at all: identity
		// cannot be verified, but nothing is wrong with the link — skip.
		var hs httpStatusError
		if errors.As(err, &hs) && (hs.Code == 404 || hs.Code == 405) {
			d.mu.Lock()
			d.bridgeChecked = true
			d.mu.Unlock()
			return nil
		}
		if d.cfg.BridgeID != "" && errors.Is(classify(err), ErrUnreachable) {
			if rerr := d.rediscover(ctx, "config unreachable"); rerr == nil {
				info, err = d.client.config(ctx)
			}
		}
		if err != nil {
			return err
		}
	}
	if d.cfg.BridgeID != "" && !strings.EqualFold(info.BridgeID, d.cfg.BridgeID) {
		if rerr := d.rediscover(ctx, fmt.Sprintf("bridge at %s is %s, expected %s", d.base, info.BridgeID, d.cfg.BridgeID)); rerr != nil {
			return fmt.Errorf("%w: bridge id mismatch at %s (%s ≠ %s) and re-discovery failed: %v", ErrUnreachable, d.base, info.BridgeID, d.cfg.BridgeID, rerr)
		}
		info, err = d.client.config(ctx)
		if err != nil {
			return err
		}
		if !strings.EqualFold(info.BridgeID, d.cfg.BridgeID) {
			return fmt.Errorf("%w: discovered bridge %s does not match configured id %s", ErrUnreachable, info.BridgeID, d.cfg.BridgeID)
		}
	}
	d.mu.Lock()
	d.bridgeInfo = info
	d.bridgeChecked = true
	d.mu.Unlock()
	return nil
}

// rediscover finds the configured bridge id on the LAN and switches the
// client to its current IP (contract §4.1). One log line when the IP changes.
func (d *Driver) rediscover(ctx context.Context, why string) error {
	if d.cfg.BridgeID == "" {
		return fmt.Errorf("%w: %s and no bridge id to re-discover", ErrUnreachable, why)
	}
	b, found, err := d.cfg.FindBridge(ctx, d.cfg.BridgeID, d.cfg.DiscoverTimeout)
	if err != nil {
		return fmt.Errorf("%w: re-discovery (%s): %v", ErrUnreachable, why, err)
	}
	if !found {
		return fmt.Errorf("%w: bridge %s not found on the network (%s)", ErrUnreachable, d.cfg.BridgeID, why)
	}
	base, err := bridgeBase(b.IP, d.cfg.HTTPS)
	if err != nil {
		return err
	}
	if base == d.base && d.client != nil {
		return nil
	}
	old := d.base
	d.client.close()
	d.mu.Lock()
	d.client = newClient(base, d.cfg.APIKey, d.cfg.Timeout)
	d.base = base
	d.bridgeChecked = false
	d.mu.Unlock()
	d.logf("Hue bridge %s moved from %s to %s (%s)", d.cfg.BridgeID, old, base, why)
	return nil
}

// resolve maps configured names to ids: exactly one match required
// (contract §4.2). Logs resolution changes once.
func (d *Driver) resolve(lights map[string]lightV1) {
	byName := map[string][]string{}
	for id, l := range lights {
		byName[l.Name] = append(byName[l.Name], id)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stats.Inventories++
	d.lastInventory = d.now()
	var changes []string
	for _, lc := range d.cfg.Lights {
		ids := byName[lc.Name]
		prev, had := d.resolved[lc.Name]
		switch len(ids) {
		case 1:
			id := ids[0]
			if !validLightID(id) {
				delete(d.resolved, lc.Name)
				d.ambiguous[lc.Name] = false
				d.lightErr[lc.Name] = "invalid light id " + id
				changes = append(changes, fmt.Sprintf("%s: invalid id %q", lc.Name, id))
				continue
			}
			rl := resolvedLight{id: id, reachable: lights[id].State.Reachable}
			if !had || prev.id != id {
				delete(d.appliedState, lc.Name) // unknown state on a (re)resolved light
				changes = append(changes, fmt.Sprintf("%s → id %s", lc.Name, id))
			}
			d.resolved[lc.Name] = rl
			delete(d.ambiguous, lc.Name)
			if d.lightErr[lc.Name] == "not found" || strings.HasPrefix(d.lightErr[lc.Name], "ambiguous") || strings.HasPrefix(d.lightErr[lc.Name], "invalid light id") {
				delete(d.lightErr, lc.Name)
			}
		case 0:
			if had || d.lightErr[lc.Name] != "not found" {
				changes = append(changes, lc.Name+": not found")
			}
			delete(d.resolved, lc.Name)
			delete(d.appliedState, lc.Name)
			delete(d.ambiguous, lc.Name)
			d.lightErr[lc.Name] = "not found"
		default:
			sort.Strings(ids)
			if !d.ambiguous[lc.Name] {
				changes = append(changes, fmt.Sprintf("%s: ambiguous (ids %s) — refusing to guess", lc.Name, strings.Join(ids, ",")))
			}
			delete(d.resolved, lc.Name)
			delete(d.appliedState, lc.Name)
			d.ambiguous[lc.Name] = true
			d.lightErr[lc.Name] = "ambiguous: " + strings.Join(ids, ",")
		}
	}
	if len(changes) > 0 {
		d.logf("Hue lights resolved: %s", strings.Join(changes, "; "))
	}
}

// fail records a bridge-level failure: status + backoff, one log line on change.
func (d *Driver) fail(err error) error {
	cerr := classify(err)
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	var st BridgeState
	switch {
	case errors.Is(cerr, ErrRefused):
		st = StateRefused
		d.nextRetry = time.Time{} // no retry loop
	default:
		st = StateUnreachable
		d.failures++
		wait := backoffMin << uint(min(d.failures-1, 6)) // 1,2,4,…,64 → capped
		if wait > backoffMax {
			wait = backoffMax
		}
		d.nextRetry = now.Add(wait)
	}
	if d.status != st || !d.reported {
		d.status, d.reason, d.lastChange, d.reported = st, cerr.Error(), now, true
		d.logf("Hue bridge %s: %v", st, cerr)
	} else {
		d.reason = cerr.Error()
	}
	return cerr
}

// ok records a successful contact: back to StateOK, one log line on change.
func (d *Driver) ok() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failures = 0
	d.nextRetry = time.Time{}
	if d.status != StateOK || !d.reported {
		d.status, d.reason, d.lastChange, d.reported = StateOK, "", d.now(), true
		d.logf("Hue bridge ok (%s, %s)", d.base, d.bridgeInfo.BridgeID)
	}
}

// ---------------------------------------------------------------------------
// Operations for #207 (inventory, forced refresh, test flash)
// ---------------------------------------------------------------------------

// LightInfo is one bridge light as returned by Inventory (contract §7 /lights).
type LightInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Model     string `json:"model,omitempty"`
	Reachable bool   `json:"reachable"`
	On        bool   `json:"on"`
}

// Inventory reads all lights (GET only), refreshes the resolution and
// returns the list sorted by id. It also lifts a refused state if the key
// works again (user gesture: re-registration).
func (d *Driver) Inventory(ctx context.Context) ([]LightInfo, error) {
	d.mu.Lock()
	if d.status == StateRefused {
		d.status, d.reason = StateUnreachable, "re-checking after refused"
	}
	d.mu.Unlock()
	if d.client == nil {
		if err := d.rediscover(ctx, "no ip configured"); err != nil {
			return nil, d.fail(err)
		}
	}
	lights, err := d.client.lights(ctx)
	if err != nil {
		return nil, d.fail(err)
	}
	if err := d.checkBridgeIdentity(ctx); err != nil {
		return nil, d.fail(err)
	}
	d.resolve(lights)
	d.ok()
	out := make([]LightInfo, 0, len(lights))
	for id, l := range lights {
		out = append(out, LightInfo{ID: id, Name: l.Name, Type: l.Type, Model: l.Model, Reachable: l.State.Reachable, On: l.State.On})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].ID) != len(out[j].ID) {
			return len(out[i].ID) < len(out[j].ID)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// RefreshInventory forces a new inventory/resolution (configuration change, user
// gesture after refused). Equivalent to Inventory without the list.
func (d *Driver) RefreshInventory(ctx context.Context) error {
	_, err := d.Inventory(ctx)
	return err
}

// TestFlash flashes one configured light (or all, name == "") bright white
// for hold, then restores the state read from the bridge beforehand
// (contract §7 /test). Uses the same guarded write as Apply.
func (d *Driver) TestFlash(ctx context.Context, name string, hold time.Duration, sleep func(time.Duration)) error {
	if sleep == nil {
		sleep = time.Sleep
	}
	if err := d.ensureResolved(ctx, false); err != nil {
		return d.fail(err)
	}
	lights, err := d.client.lights(ctx)
	if err != nil {
		return d.fail(err)
	}
	d.mu.Lock()
	var targets []plannedWrite
	for _, lc := range d.cfg.Lights {
		if name != "" && lc.Name != name {
			continue
		}
		if rl, ok := d.resolved[lc.Name]; ok {
			targets = append(targets, plannedWrite{name: lc.Name, id: rl.id})
		}
	}
	d.mu.Unlock()
	if len(targets) == 0 {
		return fmt.Errorf("hue: no resolved light matches %q", name)
	}
	white := applied{on: true, bri: 254, xy: rgbToXY(255, 255, 255)}
	var firstErr error
	for _, t := range targets {
		if err := d.client.setState(ctx, t.id, white.toV1()); err != nil && firstErr == nil {
			firstErr = classify(err)
		}
	}
	sleep(hold)
	for _, t := range targets {
		prev := lights[t.id]
		restore := applied{on: prev.State.On, bri: clamp(prev.State.Bri, 1, 254)}
		if len(prev.State.XY) == 2 {
			restore.xy = [2]float64{prev.State.XY[0], prev.State.XY[1]}
		} else {
			restore.xy = rgbToXY(255, 214, 170)
		}
		if err := d.client.setState(ctx, t.id, restore.toV1()); err != nil && firstErr == nil {
			firstErr = classify(err)
		}
		d.mu.Lock()
		delete(d.appliedState, t.name) // the writer's view is unknown now
		d.mu.Unlock()
	}
	if firstErr != nil {
		if errors.Is(firstErr, ErrUnreachable) || errors.Is(firstErr, ErrRefused) {
			return d.fail(firstErr)
		}
		return firstErr
	}
	d.ok()
	return nil
}
