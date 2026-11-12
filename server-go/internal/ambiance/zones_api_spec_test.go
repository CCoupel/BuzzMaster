package ambiance

// SURFACE D'API ATTENDUE (révision 4, trois zones) — contracts/light-effects.md §2.2, §2.5, §5.3, §6.
// Fixé par le contrat : lighting.Area (AreaAmbiance = valeur zéro, AreaHost, AreaPublic), ZoneState{Zone, Area, Group},
// zones de réglage « ambiance » | « animateur » | « public ». NON fixé : la forme des réglages résolus que le lecteur lit.
// Supposé (isolé ICI ; dev-backend n'adapte que ce fichier) :
//
//	EffectsSource.EffectSettings() map[string]map[string]string    // zone → moment → id d'effet
//	EffectsSource.EffectSpeeds()   map[string]map[string]float64   // zone → moment → vitesse (absent = 1)

import (
	"sort"
	"strconv"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/lighting"
)

func (s *tw223Source) EffectsCatalogue() effects.Catalogue          { return s.cat }
func (s *tw223Source) EffectSettings() map[string]map[string]string { return s.eff }
func (s *tw223Source) EffectSpeeds() map[string]map[string]float64  { return s.spd }

func tw223DefaultSource() *tw223Source {
	zero := func() map[string]string { return map[string]string{"chrono": "", "calm": "", "score": ""} }
	s := &tw223Source{
		eff: map[string]map[string]string{"ambiance": zero(), "animateur": zero(), "public": zero()},
		spd: map[string]map[string]float64{},
	}
	s.eff["ambiance"]["chrono"] = "pulsation" // seul défaut non vide : la pulsation du chronomètre de la salle (§6)
	return s
}

// setEffect règle l'effet d'une zone (« ambiance », « animateur », « public ») pour un moment.
func (e *tw223Env) setEffect(zone, moment, effect string) { e.src.eff[zone][moment] = effect }

// setHost / setPublic : raccourcis pour les zones animateur et public.
func (e *tw223Env) setHost(moment, effect string)   { e.setEffect("animateur", moment, effect) }
func (e *tw223Env) setPublic(moment, effect string) { e.setEffect("public", moment, effect) }

// setSpeed règle la vitesse d'une zone pour un moment ; setSpeedAmb : la zone ambiance.
func (e *tw223Env) setSpeedZone(zone, moment string, v float64) {
	if e.src.spd[zone] == nil {
		e.src.spd[zone] = map[string]float64{}
	}
	e.src.spd[zone][moment] = v
}
func (e *tw223Env) setSpeed(moment string, v float64) { e.setSpeedZone("ambiance", moment, v) }

// ---------- groupes de test ----------

var tw223NextID int

func tw223ID() string { tw223NextID++; return "t" + strconv.Itoa(tw223NextID) }

func zoneGrp(role string, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: tw223ID(), Role: role, Lights: lights}
}

// hostGrp : un groupe de la zone animateur (ampoule « H » par défaut) ; pubGrp : un groupe de la zone public.
func hostGrp(lights ...string) config.LightingGroupEntry {
	if len(lights) == 0 {
		lights = []string{"H"}
	}
	return zoneGrp(config.GroupRoleAnimateur, lights...)
}

func pubGrp(lights ...string) config.LightingGroupEntry {
	if len(lights) == 0 {
		lights = []string{"P"}
	}
	return zoneGrp("public", lights...)
}

// ---------- lecture des zones émises par Scene() ----------

// areaEntries : toutes les entrées {Zone:"general"} d'une zone.
func areaEntries(st lighting.State, area lighting.Area) []lighting.ZoneState {
	var out []lighting.ZoneState
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Area == area {
			out = append(out, z)
		}
	}
	return out
}

// areaEntry : l'entrée {Area, Group} (Group 0 = toute la zone).
func areaEntry(st lighting.State, area lighting.Area, group int) (lighting.ZoneState, bool) {
	for _, z := range areaEntries(st, area) {
		if z.Group == group {
			return z, true
		}
	}
	return lighting.ZoneState{}, false
}

// roomGroups : les entrées de groupe (Group ≥ 1) de la zone ambiance.
func roomGroups(st lighting.State) []lighting.ZoneState {
	var out []lighting.ZoneState
	for _, z := range areaEntries(st, lighting.AreaAmbiance) {
		if z.Group >= 1 {
			out = append(out, z)
		}
	}
	// l'ordre des entrées n'est pas contractuel : trié par numéro de groupe
	sort.Slice(out, func(i, j int) bool { return out[i].Group < out[j].Group })
	return out
}

// hostEntry : l'état de toute la zone animateur (entrée Group 0).
func hostEntry(st lighting.State) (lighting.ZoneState, bool) {
	return areaEntry(st, lighting.AreaHost, 0)
}
