package config

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
)

// MaxGroupsPerZone is the cap of groups in one zone (light-effects.md §2.1, revision 4).
const MaxGroupsPerZone = 12

// Zones of the ambiance (light-effects.md §2.5): the room ("ambiance", role salle), the
// animateur zone and the public zone.
const (
	ZoneAmbiance  = "ambiance"
	ZoneAnimateur = "animateur"
	ZonePublic    = "public"
)

// Zones lists the three zones in their normative order.
var Zones = []string{ZoneAmbiance, ZoneAnimateur, ZonePublic}

// ZoneOfRole maps a group role to its zone ("" and salle ⇒ ambiance).
func ZoneOfRole(role string) string {
	switch role {
	case GroupRoleAnimateur:
		return ZoneAnimateur
	case GroupRolePublic:
		return ZonePublic
	}
	return ZoneAmbiance
}

var reGroupID = regexp.MustCompile(`^[a-z0-9]{1,12}$`)

// deriveGroupID builds a stable id from a seed ("g" + 6 hex of its SHA-1), made unique
// against used. Deterministic on purpose: a config.json that predates the ids gets the
// same ids at every start (seed = role and position, groups have no name any more).
func deriveGroupID(seed string, used map[string]bool) string {
	sum := sha1.Sum([]byte(seed))
	base := "g" + hex.EncodeToString(sum[:])[:6]
	id := base
	for i := 2; used[id]; i++ {
		id = fmt.Sprintf("%s%d", base, i)
	}
	return id
}

// randomGroupID gives a fresh id to a group created through the API.
func randomGroupID(used map[string]bool) string {
	for {
		var b [3]byte
		_, _ = rand.Read(b[:])
		id := "g" + hex.EncodeToString(b[:])
		if !used[id] {
			return id
		}
	}
}

// AssignGroupIDs gives an id to every group that has none. Used when loading a config.json.
func AssignGroupIDs(groups []LightingGroupEntry) {
	used := map[string]bool{}
	for _, g := range groups {
		if g.ID != "" {
			used[g.ID] = true
		}
	}
	for i := range groups {
		if groups[i].ID == "" {
			role := groups[i].Role
			if role == "" {
				role = GroupRoleSalle
			}
			groups[i].ID = deriveGroupID(fmt.Sprintf("%s#%d", role, i), used)
			used[groups[i].ID] = true
		}
	}
}

// ValidateLightingGroups checks a full replacement of lighting.groups against
// the (already normalised) bulb list — contracts/light-effects.md §2.1. It
// returns the canonical groups (role defaulted, slices never nil) or an error
// whose message is readable as-is (it becomes the HTTP 400 body).
//
// Light names are compared as given: callers pass names already normalised
// the same way as LightingLightEntry.Name (hue.NormalizeLightName).
func ValidateLightingGroups(groups []LightingGroupEntry, lights []LightingLightEntry) ([]LightingGroupEntry, error) {
	role := make(map[string]string, len(lights))
	for _, l := range lights {
		r := l.Role
		if r == "" {
			r = "general"
		}
		role[l.Name] = r
	}
	out := make([]LightingGroupEntry, 0, len(groups))
	usedLight := map[string]string{}
	perZone := map[string]int{}
	usedID := map[string]bool{}
	for _, g := range groups {
		if g.ID != "" {
			if !reGroupID.MatchString(g.ID) {
				return nil, fmt.Errorf("lighting.groups : identifiant de groupe invalide %q (1 à 12 caractères a-z, 0-9)", g.ID)
			}
			if usedID[g.ID] {
				return nil, fmt.Errorf("lighting.groups : identifiant de groupe en double (%q)", g.ID)
			}
			usedID[g.ID] = true
		}
	}
	for _, g := range groups {
		switch g.Role {
		case "":
			g.Role = GroupRoleSalle
		case GroupRoleSalle, GroupRoleAnimateur, GroupRolePublic:
		default:
			return nil, fmt.Errorf("lighting.groups : rôle de groupe inconnu %q (salle, animateur ou public)", g.Role)
		}
		zone := ZoneOfRole(g.Role)
		perZone[zone]++
		if perZone[zone] > MaxGroupsPerZone {
			return nil, fmt.Errorf("lighting.groups : %d groupes au maximum dans la zone %s", MaxGroupsPerZone, zone)
		}
		lst := make([]string, 0, len(g.Lights))
		for _, ln := range g.Lights {
			r, ok := role[ln]
			switch {
			case !ok:
				return nil, fmt.Errorf("lighting.groups : ampoule inconnue %q", ln)
			case r != "general":
				return nil, fmt.Errorf("lighting.groups : l'ampoule %q est de rôle équipe, elle ne peut pas être dans un groupe", ln)
			}
			if _, dup := usedLight[ln]; dup {
				return nil, fmt.Errorf("lighting.groups : l'ampoule %q est dans deux groupes", ln)
			}
			usedLight[ln] = g.ID
			lst = append(lst, ln)
		}
		g.Lights = lst
		if g.ID == "" {
			g.ID = randomGroupID(usedID)
			usedID[g.ID] = true
		}
		out = append(out, g)
	}
	return out, nil
}

// PruneLightingGroups drops from every group the bulbs that are no longer
// general bulbs of the list (removed, or switched to role=team) — §2.1, "dans
// la même écriture". It never fails and returns a fresh slice.
func PruneLightingGroups(groups []LightingGroupEntry, lights []LightingLightEntry) []LightingGroupEntry {
	general := make(map[string]bool, len(lights))
	for _, l := range lights {
		if l.Role == "" || l.Role == "general" {
			general[l.Name] = true
		}
	}
	out := make([]LightingGroupEntry, 0, len(groups))
	for _, g := range groups {
		ng := g
		ng.Lights = make([]string, 0, len(g.Lights))
		for _, ln := range g.Lights {
			if general[ln] {
				ng.Lights = append(ng.Lights, ln)
			}
		}
		out = append(out, ng)
	}
	return out
}

// ZoneGroup is one group of a zone: its stable id, its real number (1-based rank among ALL
// the groups of the zone, empty ones included — an empty group keeps its number) and its
// bulbs.
type ZoneGroup struct {
	ID     string
	Num    int
	Lights []string
}

// ZoneGroups lists every group of a zone in order (empty ones included).
func (l LightingConfig) ZoneGroups(zone string) []ZoneGroup {
	var out []ZoneGroup
	for _, g := range l.Groups {
		if ZoneOfRole(g.Role) == zone {
			out = append(out, ZoneGroup{ID: g.ID, Num: len(out) + 1, Lights: g.Lights})
		}
	}
	return out
}

// NonEmptyGroups lists the non-empty groups of a zone in order, with their real numbers —
// the N groups an effect renders on (light-effects.md §2.1, §4.3).
func (l LightingConfig) NonEmptyGroups(zone string) []ZoneGroup {
	var out []ZoneGroup
	for _, g := range l.ZoneGroups(zone) {
		if len(g.Lights) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// ZoneExists reports whether a zone exists: the ambiance zone always does (it owns every
// general bulb that is in no group); animateur and public exist only with at least one
// non-empty group (§2.5).
func (l LightingConfig) ZoneExists(zone string) bool {
	if zone == ZoneAmbiance {
		return true
	}
	return len(l.NonEmptyGroups(zone)) > 0
}

// LightRef says where a bulb sits: its zone and group number (0 = in no group).
type LightRef struct {
	Zone string
	Num  int
}

// GroupOfLight maps each grouped bulb name to its zone and group number. A general bulb that
// is in no group is absent (it belongs to the ambiance zone, group 0).
func (l LightingConfig) GroupOfLight() map[string]LightRef {
	out := map[string]LightRef{}
	for _, zone := range Zones {
		for _, g := range l.ZoneGroups(zone) {
			for _, ln := range g.Lights {
				out[ln] = LightRef{Zone: zone, Num: g.Num}
			}
		}
	}
	return out
}
