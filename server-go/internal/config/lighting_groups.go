package config

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidateLightingGroups checks a full replacement of lighting.groups against
// the (already normalised) bulb list — contracts/light-effects.md §2.1. It
// returns the canonical groups (role defaulted, slices never nil) or an error
// whose message is readable as-is (it becomes the HTTP 400 body).
//
// Light names are compared as given: callers pass names already normalised
// the same way as LightingLightEntry.Name (hue.NormalizeLightName).
func ValidateLightingGroups(groups []LightingGroupEntry, lights []LightingLightEntry) ([]LightingGroupEntry, error) {
	if len(groups) > MaxLightingGroups {
		return nil, fmt.Errorf("lighting.groups : %d groupes au maximum", MaxLightingGroups)
	}
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
	hosts := 0
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
		case GroupRoleSalle:
		case GroupRoleAnimateur:
			hosts++
			if hosts > 1 {
				return nil, fmt.Errorf("lighting.groups : un seul groupe animateur est autorisé")
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

// HostGroup returns the animateur group, if any.
func (l LightingConfig) HostGroup() (LightingGroupEntry, bool) {
	for _, g := range l.Groups {
		if g.Role == GroupRoleAnimateur {
			return g, true
		}
	}
	return LightingGroupEntry{}, false
}

// GroupAssignment maps each general bulb name to its salle group number
// (1-based rank among ALL "salle" groups in array order — an empty group keeps
// its number, it simply has no bulb) and returns the set of host bulb names.
func (l LightingConfig) GroupAssignment() (num map[string]int, host map[string]bool) {
	num = map[string]int{}
	host = map[string]bool{}
	n := 0
	for _, g := range l.Groups {
		if g.Role == GroupRoleAnimateur {
			for _, ln := range g.Lights {
				host[ln] = true
			}
			continue
		}
		n++
		for _, ln := range g.Lights {
			num[ln] = n
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
