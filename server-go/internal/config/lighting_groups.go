package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Caps (light-effects.md §2.1): 12 salle groups, 4 animateur groups.
const (
	MaxSalleGroups = 12
	MaxHostGroups  = 4
)

var reGroupID = regexp.MustCompile(`^[a-z0-9]{1,12}$`)

// deriveGroupID builds a stable id from a group name ("g" + 6 hex of its SHA-1), made
// unique against used. Deterministic on purpose: a config.json predating the ids gets
// the same ids at every start.
func deriveGroupID(name string, used map[string]bool) string {
	sum := sha1.Sum([]byte(strings.ToLower(name)))
	base := "g" + hex.EncodeToString(sum[:])[:6]
	id := base
	for i := 2; used[id]; i++ {
		id = fmt.Sprintf("%s%d", base, i)
	}
	return id
}

// AssignGroupIDs gives an id to every group that has none (or an invalid/duplicate one is
// left to ValidateLightingGroups to refuse). Used when loading a config.json.
func AssignGroupIDs(groups []LightingGroupEntry) {
	used := map[string]bool{}
	for _, g := range groups {
		if g.ID != "" {
			used[g.ID] = true
		}
	}
	for i := range groups {
		if groups[i].ID == "" {
			groups[i].ID = deriveGroupID(groups[i].Name, used)
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
	names := map[string]bool{}
	usedLight := map[string]string{}
	usedTeam := map[string]string{}
	hosts, salles := 0, 0
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
		g.Name = strings.TrimSpace(g.Name)
		if n := utf8.RuneCountInString(g.Name); n < 1 || n > 40 {
			return nil, fmt.Errorf("lighting.groups : le nom d'un groupe doit compter de 1 à 40 caractères")
		}
		key := strings.ToLower(g.Name)
		if names[key] {
			return nil, fmt.Errorf("lighting.groups : nom de groupe en double (%q)", g.Name)
		}
		names[key] = true
		switch g.Role {
		case "":
			g.Role = GroupRoleSalle
			salles++
		case GroupRoleSalle:
			salles++
		case GroupRoleAnimateur:
			hosts++
			if hosts > MaxHostGroups {
				return nil, fmt.Errorf("lighting.groups : %d groupes animateur au maximum", MaxHostGroups)
			}
			if len(g.NearTeams) > 0 {
				return nil, fmt.Errorf("lighting.groups : le groupe animateur n'a pas d'équipes à proximité (near_teams)")
			}
		default:
			return nil, fmt.Errorf("lighting.groups : rôle de groupe inconnu %q (salle ou animateur)", g.Role)
		}
		lst := make([]string, 0, len(g.Lights))
		for _, ln := range g.Lights {
			r, ok := role[ln]
			switch {
			case !ok:
				return nil, fmt.Errorf("lighting.groups : ampoule inconnue %q dans le groupe %q", ln, g.Name)
			case r != "general":
				return nil, fmt.Errorf("lighting.groups : l'ampoule %q est de rôle équipe, elle ne peut pas être dans un groupe", ln)
			}
			if other, dup := usedLight[ln]; dup {
				return nil, fmt.Errorf("lighting.groups : l'ampoule %q est dans deux groupes (%q et %q)", ln, other, g.Name)
			}
			usedLight[ln] = g.Name
			lst = append(lst, ln)
		}
		g.Lights = lst
		teams := make([]string, 0, len(g.NearTeams))
		for _, t := range g.NearTeams {
			if other, dup := usedTeam[t]; dup {
				return nil, fmt.Errorf("lighting.groups : l'équipe %q est près de deux groupes (%q et %q)", t, other, g.Name)
			}
			usedTeam[t] = g.Name
			teams = append(teams, t)
		}
		g.NearTeams = teams
		if g.ID == "" {
			g.ID = deriveGroupID(g.Name, usedID)
			usedID[g.ID] = true
		}
		out = append(out, g)
	}
	if salles > MaxSalleGroups {
		return nil, fmt.Errorf("lighting.groups : %d groupes de salle au maximum", MaxSalleGroups)
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

// HostGroup is one animateur group with its own rank (A1, A2… among the animateur groups,
// empty ones included) — light-effects.md §2.5.
type HostGroup struct {
	ID     string
	Name   string
	Rank   int // 1-based; the state address is -Rank (lighting.HostGroupAddr)
	Lights []string
}

// HostGroups lists the animateur groups in order.
func (l LightingConfig) HostGroups() []HostGroup {
	var out []HostGroup
	for _, g := range l.Groups {
		if g.Role == GroupRoleAnimateur {
			out = append(out, HostGroup{ID: g.ID, Name: g.Name, Rank: len(out) + 1, Lights: g.Lights})
		}
	}
	return out
}

// GroupAssignment maps each general bulb name to its salle group number (1-based rank
// among ALL "salle" groups — an empty group keeps its number, it simply has no bulb) and
// to the rank k of its animateur group.
func (l LightingConfig) GroupAssignment() (num map[string]int, host map[string]int) {
	num = map[string]int{}
	host = map[string]int{}
	n := 0
	for _, g := range l.Groups {
		if g.Role == GroupRoleAnimateur {
			continue
		}
		n++
		for _, ln := range g.Lights {
			num[ln] = n
		}
	}
	for _, h := range l.HostGroups() {
		for _, ln := range h.Lights {
			host[ln] = h.Rank
		}
	}
	return num, host
}

// RoomGroup is one non-empty salle group: its real number (1-based rank among all
// salle groups, never renumbered) and the teams installed near it.
type RoomGroup struct {
	Num       int
	NearTeams []string
}

// RoomGroups lists the non-empty salle groups in order — the N groups an effect
// renders on (light-effects.md §2.1, §4.5).
func (l LightingConfig) RoomGroups() []RoomGroup {
	var out []RoomGroup
	n := 0
	for _, g := range l.Groups {
		if g.Role == GroupRoleAnimateur {
			continue
		}
		n++
		if len(g.Lights) > 0 {
			out = append(out, RoomGroup{Num: n, NearTeams: g.NearTeams})
		}
	}
	return out
}
