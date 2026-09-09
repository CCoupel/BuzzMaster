package hue

// BuzzMaster-managed Hue LightGroups (contracts/hue-bridge.md §5.8, Batch B
// / #208 §5.8, planner report planner-v10-groups-teamcolor-20260907-173831.md
// §Problème 2).
//
// Three groups, all created and reconciled by BuzzMaster, identified by NAME
// (never id — same rule as light resolution, §4.2):
//
//   - buzzmaster-ambiance      — every assigned light (general + all teams)
//   - buzzmaster-general       — the role=general lights
//   - buzzmaster-team-<team>   — a team's own lights, ONLY if it has ≥ 2
//
// Composition depends ONLY on configuration (never on a game event — rule 3:
// mutating a group's membership on every team-turn change would be both slow
// and over the ≤1 write/s budget). reconcileGroups is therefore wired into
// ensureResolved, on the exact same freshness gate as light resolution, so
// it runs at startup and again only when the inventory is re-verified
// (periodic refresh, or a forced RefreshInventory — which #207 already
// triggers right after a configuration save) — never once per Apply.
//
// Everything here is best-effort: groups are an optimisation Apply() may use
// when a target has ≥2 members (§5.8, "on écrit par groupe si et seulement
// si..."), never a prerequisite. Any failure — reading /groups, creating,
// correcting or deleting one — is logged once and leaves d.groups exactly as
// it was; Apply() then simply finds no matching group for that target and
// falls back to writing every light individually (rule 5).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	groupNameAmbiance = "buzzmaster-ambiance"
	groupNameGeneral  = "buzzmaster-general"
	groupNameTeamPfx  = "buzzmaster-team-"
)

func groupNameForTeam(team string) string { return groupNameTeamPfx + team }

// hueGroup is what the driver trusts about one reconciled BuzzMaster group:
// its bridge id and the exact set of light ids it currently holds. Apply()
// only uses a group when this set exactly matches the batch it wants to
// write (never a subset/superset guess).
type hueGroup struct {
	id      string
	members map[string]bool // light id -> true
}

// ---------------------------------------------------------------------------
// client: minimal /groups operations, guarded exactly like the light ones.
// ---------------------------------------------------------------------------

// groupV1 is the subset of GET /groups[/<id>] the driver needs.
type groupV1 struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Lights []string `json:"lights"`
}

func (c *client) groups(ctx context.Context) (map[string]groupV1, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/"+c.key+"/groups", nil)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return nil, herr
		}
		return nil, err
	}
	var m map[string]groupV1
	if jerr := json.Unmarshal(data, &m); jerr != nil {
		if herr := firstHueError(data); herr != nil {
			return nil, herr
		}
		return nil, fmt.Errorf("hue: unexpected /groups response: %w", jerr)
	}
	return m, nil
}

type createGroupRequest struct {
	Name   string   `json:"name"`
	Lights []string `json:"lights"`
	Type   string   `json:"type"`
}

// createGroup creates a LightGroup and returns its new id.
func (c *client) createGroup(ctx context.Context, name string, lightIDs []string) (string, error) {
	data, err := c.do(ctx, http.MethodPost, "/api/"+c.key+"/groups", createGroupRequest{Name: name, Lights: lightIDs, Type: "LightGroup"})
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return "", herr
		}
		return "", err
	}
	succ, errs, perr := parseResultArray(data)
	if perr != nil {
		return "", perr
	}
	if len(errs) > 0 {
		return "", errs[0]
	}
	for _, s := range succ {
		if raw, ok := s["id"]; ok {
			var id string
			if json.Unmarshal(raw, &id) == nil && id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("hue: group creation answered without id: %s", firstLine(string(data)))
}

// updateGroupLights corrects an existing group's membership (composition
// only — never its name, never its state).
func (c *client) updateGroupLights(ctx context.Context, id string, lightIDs []string) error {
	if !validGroupID(id) {
		return ErrGuard{http.MethodPut, "/api/<key>/groups/" + id}
	}
	data, err := c.do(ctx, http.MethodPut, "/api/"+c.key+"/groups/"+id, map[string][]string{"lights": lightIDs})
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return herr
		}
		return err
	}
	_, errs, perr := parseResultArray(data)
	if perr != nil {
		return perr
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// deleteGroup removes a stale BuzzMaster-owned group.
func (c *client) deleteGroup(ctx context.Context, id string) error {
	if !validGroupID(id) {
		return ErrGuard{http.MethodDelete, "/api/<key>/groups/" + id}
	}
	data, err := c.do(ctx, http.MethodDelete, "/api/"+c.key+"/groups/"+id, nil)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return herr
		}
		return err
	}
	_, errs, perr := parseResultArray(data)
	if perr != nil {
		return perr
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// setGroupState is the group equivalent of setState: one write applied to
// every current member of the group in a single Hue command.
func (c *client) setGroupState(ctx context.Context, id string, st stateV1) error {
	if !validGroupID(id) {
		return ErrGuard{http.MethodPut, "/api/<key>/groups/" + id + "/action"}
	}
	data, err := c.do(ctx, http.MethodPut, "/api/"+c.key+"/groups/"+id+"/action", st)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return herr
		}
		return err
	}
	_, errs, perr := parseResultArray(data)
	if perr != nil {
		return perr
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// ---------------------------------------------------------------------------
// Desired composition (config-only) and reconciliation.
// ---------------------------------------------------------------------------

// desiredGroups computes, from CONFIG and the current name→id resolution
// alone (never from a live game event, contract §5.8 rule 3), the light-id
// membership each BuzzMaster group kind should have. A light without a
// resolution (missing/ambiguous) is silently left out, exactly as it is left
// out of Apply()'s own plan. A team's own group is included only when it has
// ≥ 2 resolved lights (rule: table row for buzzmaster-team-<équipe>);
// buzzmaster-general and buzzmaster-ambiance are included with as little as
// one member (lifecycle rule 4's only floor is ≥ 1) — Apply() separately
// decides, per write, whether a target is worth addressing as a group.
func desiredGroups(lights []LightSpec, resolved map[string]resolvedLight) map[string][]string {
	out := map[string][]string{}
	var all, general []string
	byTeam := map[string][]string{}
	for _, lc := range lights {
		rl, ok := resolved[lc.Name]
		if !ok {
			continue
		}
		all = append(all, rl.id)
		if lc.Role == RoleTeam {
			byTeam[lc.Team] = append(byTeam[lc.Team], rl.id)
		} else {
			general = append(general, rl.id)
		}
	}
	if len(all) > 0 {
		out[groupNameAmbiance] = sortedIDs(all)
	}
	if len(general) > 0 {
		out[groupNameGeneral] = sortedIDs(general)
	}
	for team, ids := range byTeam {
		if len(ids) >= 2 {
			out[groupNameForTeam(team)] = sortedIDs(ids)
		}
	}
	return out
}

func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func sameIDSet(a map[string]bool, ids []string) bool {
	if len(a) != len(ids) {
		return false
	}
	for _, id := range ids {
		if !a[id] {
			return false
		}
	}
	return true
}

// reconcileGroups creates missing BuzzMaster groups, corrects the
// composition of ones that drifted, and deletes ones that no longer
// correspond to anything desired (team removed/emptied, light unassigned).
// Called from ensureResolved right after resolve(), under the same
// freshness/force gate — so it sees the exact same light-id snapshot Apply()
// is about to use, and never runs more often than the inventory itself is
// re-verified (contract §5.8 rule 2: startup, and after each config save via
// the forced RefreshInventory #207 already issues then — never per event,
// rule 3).
//
// Best-effort throughout (rule 5): any failure is logged once via d.logf and
// simply leaves the affected group's entry out of d.groups (or, for a group
// whose composition could not be corrected, out of the NEW d.groups) —
// Apply()'s exact-set match then naturally refuses to use it and falls back
// to per-light writes. No error is ever returned to ensureResolved.
func (d *Driver) reconcileGroups(ctx context.Context, c *client) {
	d.mu.Lock()
	disabled := d.disableGroupsForTest
	want := desiredGroups(d.cfg.Lights, d.resolved)
	d.mu.Unlock()
	if disabled {
		return
	}
	live, err := c.groups(ctx)
	if err != nil {
		d.logf("Hue groups: could not read /groups (%v) — group writes unavailable until the next reconciliation", err)
		return
	}
	type liveGroup struct {
		id      string
		members map[string]bool
	}
	liveByName := map[string]liveGroup{}
	for id, g := range live {
		if !strings.HasPrefix(g.Name, "buzzmaster-") {
			continue // never touch a group BuzzMaster did not create
		}
		mm := map[string]bool{}
		for _, lid := range g.Lights {
			mm[lid] = true
		}
		liveByName[g.Name] = liveGroup{id: id, members: mm}
	}

	next := map[string]hueGroup{}
	var changes []string
	for name, ids := range want {
		if lg, ok := liveByName[name]; ok {
			if !sameIDSet(lg.members, ids) {
				if err := c.updateGroupLights(ctx, lg.id, ids); err != nil {
					changes = append(changes, fmt.Sprintf("%s: composition drifted, correction failed (%v)", name, err))
					continue // do not trust a group whose composition we could not fix
				}
				changes = append(changes, fmt.Sprintf("%s: composition corrected", name))
			}
			wantSet := map[string]bool{}
			for _, id := range ids {
				wantSet[id] = true
			}
			next[name] = hueGroup{id: lg.id, members: wantSet}
			continue
		}
		id, err := c.createGroup(ctx, name, ids)
		if err != nil {
			changes = append(changes, fmt.Sprintf("%s: could not be created (%v)", name, err))
			continue
		}
		mm := map[string]bool{}
		for _, lid := range ids {
			mm[lid] = true
		}
		next[name] = hueGroup{id: id, members: mm}
		changes = append(changes, fmt.Sprintf("%s: created", name))
	}
	for name, lg := range liveByName {
		if _, kept := next[name]; kept {
			continue
		}
		if err := c.deleteGroup(ctx, lg.id); err != nil {
			changes = append(changes, fmt.Sprintf("%s: stale group could not be deleted (%v)", name, err))
			continue
		}
		changes = append(changes, fmt.Sprintf("%s: deleted (no longer desired)", name))
	}
	if len(changes) > 0 {
		d.logf("Hue groups reconciled: %s", strings.Join(changes, "; "))
	}
	d.mu.Lock()
	d.groups = next
	d.mu.Unlock()
}
