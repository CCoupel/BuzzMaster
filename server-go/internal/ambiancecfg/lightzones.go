package ambiancecfg

import (
	"encoding/json"
	"sort"
)

// Light effects and speeds by ZONE and moment (light-effects.md §6, revision 4): the room
// ("ambiance"), the animateur zone and the public zone, each with one effect and one speed
// per moment, globally (config.json `lighting.effects|speeds.<zone>.<moment>`) and per round
// (`light.effects|speeds.<zone>.<moment>`).

// Zones of the ambiance, in their normative order.
const (
	ZoneAmbiance  = "ambiance"
	ZoneAnimateur = "animateur"
	ZonePublic    = "public"
)

// Zones lists the three zones.
var Zones = []string{ZoneAmbiance, ZoneAnimateur, ZonePublic}

// ValidZone reports whether z is a known zone.
func ValidZone(z string) bool { return z == ZoneAmbiance || z == ZoneAnimateur || z == ZonePublic }

// DefaultLightEffects is the global default of light.effects: the pulsation for the room's
// chrono moment, no effect anywhere else (a fresh table, the three zones always present).
func DefaultLightEffects() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, z := range Zones {
		out[z] = map[string]string{MomentChrono: "", MomentCalm: "", MomentScore: ""}
	}
	out[ZoneAmbiance][MomentChrono] = PulsationID
	return out
}

// DefaultLightSpeeds is the global default of light.speeds (×1 everywhere, a fresh table).
func DefaultLightSpeeds() map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for _, z := range Zones {
		out[z] = map[string]float64{MomentChrono: 1, MomentCalm: 1, MomentScore: 1}
	}
	return out
}

func cloneEffects(m map[string]map[string]string) map[string]map[string]string {
	if m == nil {
		return nil
	}
	c := make(map[string]map[string]string, len(m))
	for z, mm := range m {
		c[z] = make(map[string]string, len(mm))
		for k, v := range mm {
			c[z][k] = v
		}
	}
	return c
}

func cloneSpeeds(m map[string]map[string]float64) map[string]map[string]float64 {
	if m == nil {
		return nil
	}
	c := make(map[string]map[string]float64, len(m))
	for z, mm := range m {
		c[z] = make(map[string]float64, len(mm))
		for k, v := range mm {
			c[z][k] = v
		}
	}
	return c
}

// completeEffects returns m with the three zones and the three moments always present
// (missing ones take the default).
func completeEffects(m map[string]map[string]string) map[string]map[string]string {
	out := DefaultLightEffects()
	for z, mm := range m {
		if !ValidZone(z) {
			continue
		}
		for mo, v := range mm {
			if ValidMoment(mo) {
				out[z][mo] = v
			}
		}
	}
	return out
}

func completeSpeeds(m map[string]map[string]float64) map[string]map[string]float64 {
	out := DefaultLightSpeeds()
	for z, mm := range m {
		if !ValidZone(z) {
			continue
		}
		for mo, v := range mm {
			if ValidMoment(mo) {
				out[z][mo] = v
			}
		}
	}
	return out
}

// effectsWire / speedsWire decode the zone tables leniently: besides the zone form
// {zone: {moment: value}}, the flat form {moment: value} of a QUALIF file (a string / a
// number directly under a moment key) is read as the "ambiance" zone; anything else is
// ignored, never an error (light-effects.md §6, "lecture d'une configuration antérieure").
type effectsWire map[string]map[string]string

func (w *effectsWire) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		*w = nil
		return nil
	}
	out := effectsWire{}
	for k, v := range raw {
		var obj map[string]string
		if json.Unmarshal(v, &obj) == nil && ValidZone(k) {
			if out[k] == nil {
				out[k] = map[string]string{}
			}
			for m, val := range obj {
				out[k][m] = val
			}
			continue
		}
		var s string
		if ValidMoment(k) && json.Unmarshal(v, &s) == nil {
			if out[ZoneAmbiance] == nil {
				out[ZoneAmbiance] = map[string]string{}
			}
			out[ZoneAmbiance][k] = s
		}
	}
	*w = out
	return nil
}

type speedsWire map[string]map[string]float64

func (w *speedsWire) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		*w = nil
		return nil
	}
	out := speedsWire{}
	for k, v := range raw {
		var obj map[string]float64
		if json.Unmarshal(v, &obj) == nil && ValidZone(k) {
			if out[k] == nil {
				out[k] = map[string]float64{}
			}
			for m, val := range obj {
				out[k][m] = val
			}
			continue
		}
		var f float64
		if ValidMoment(k) && json.Unmarshal(v, &f) == nil {
			if out[ZoneAmbiance] == nil {
				out[ZoneAmbiance] = map[string]float64{}
			}
			out[ZoneAmbiance][k] = f
		}
	}
	*w = out
	return nil
}

func sortedZoneKeys[T any](m map[string]map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
