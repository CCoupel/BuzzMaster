package ambiance

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// Three zones (light-effects.md §2.5, revision 4): the public zone is never written without an
// effect, a POST or a trial; the animateur zone shows a fixed white; the effect trial plays on
// ONE zone at the chosen speed. No ticker, no sleep: the clock is injected.

type zdSource struct {
	cat effects.Catalogue
	eff map[string]map[string]string
	spd map[string]map[string]float64
}

func (s *zdSource) EffectsCatalogue() effects.Catalogue          { return s.cat }
func (s *zdSource) EffectSettings() map[string]map[string]string { return s.eff }
func (s *zdSource) EffectSpeeds() map[string]map[string]float64  { return s.spd }

const zdSolid = `id: plein
nom: Plein
version: 2
moments: [calm, score]
duree_tick_ms: 1000
pas:
  - { g0: ["#112233", 50] }
post:
  { g0: ["#00FF00", 100] }
`

const zdTour = `id: tour
nom: Tour
version: 2
moments: [calm]
groupes_min: 2
duree_tick_ms: 1000
variables:
  gA: g1; +1
pas:
  - { g0: ["#000000", 0], gA: ["#FF0000", 100] }
`

type zdRig struct {
	c   *Controller
	src *zdSource
	now time.Time
}

func newZdRig(t *testing.T, groups []config.LightingGroupEntry) *zdRig {
	t.Helper()
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })
	cfg := *prev
	cfg.Lighting.Enabled = true
	cfg.Lighting.Groups = groups
	config.SetInstance(&cfg)
	c := New(fakeHost{engine: game.NewEngine()})
	c.LightingWriter.Store(c.NewWriter(lighting.NewFakeDriver()))
	fs := fstest.MapFS{"plein.yaml": {Data: []byte(zdSolid)}, "tour.yaml": {Data: []byte(zdTour)}}
	r := &zdRig{c: c, now: time.Unix(1_700_000_000, 0)}
	r.src = &zdSource{cat: effects.LoadDir(fs, ""), eff: map[string]map[string]string{"ambiance": {}, "animateur": {}, "public": {}}, spd: map[string]map[string]float64{}}
	c.SetEffectsSource(r.src)
	c.EffectsClock = func() time.Time { return r.now }
	t.Cleanup(func() { c.StopEffectTrial(); c.StopGroupTest() })
	return r
}

func zoneEntry(st lighting.State, area lighting.Area, group int) (lighting.ZoneState, bool) {
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Area == area && z.Group == group {
			return z, true
		}
	}
	return lighting.ZoneState{}, false
}

func areaCount(st lighting.State, area lighting.Area) int {
	n := 0
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Area == area {
			n++
		}
	}
	return n
}

var zdIdle = lighting.Event{Kind: lighting.KindIdle}

func TestZones_PublicNeverWrittenWithoutEffect_HostWhiteFixed_Dev(t *testing.T) {
	r := newZdRig(t, []config.LightingGroupEntry{
		{ID: "a", Role: "salle", Lights: []string{"l1"}},
		{ID: "h", Role: "animateur", Lights: []string{"h1"}},
		{ID: "p", Role: "public", Lights: []string{"p1"}},
	})
	_ = r.c.SetLightingMode("OFF")
	r.c.SetLightingFlash(true)
	defer r.c.SetLightingFlash(false)
	st := r.c.Scene(zdIdle)
	if n := areaCount(st, lighting.AreaPublic); n != 0 {
		t.Fatalf("la zone public sans effet n'a AUCUNE entrée (ni scène, ni blanc) : %+v", st.Zones)
	}
	if z, ok := zoneEntry(st, lighting.AreaHost, 0); !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
		t.Fatalf("zone animateur : blanc fixe malgré OFF + Flash : %+v %v", z, ok)
	}
	// Without a public or animateur group nothing of those zones exists.
	cfg := config.Get()
	cfg.Lighting.Groups = cfg.Lighting.Groups[:1]
	st = r.c.Scene(zdIdle)
	if areaCount(st, lighting.AreaHost)+areaCount(st, lighting.AreaPublic) != 0 {
		t.Fatalf("sans groupe animateur ni public, rien ne change : %+v", st.Zones)
	}
}

func TestZones_EffectPerZone_PostHeldOnlyInPublic_Dev(t *testing.T) {
	r := newZdRig(t, []config.LightingGroupEntry{
		{ID: "h", Role: "animateur", Lights: []string{"h1"}},
		{ID: "p", Role: "public", Lights: []string{"p1", "p2"}},
	})
	r.src.eff["animateur"]["calm"] = "plein"
	r.src.eff["public"]["calm"] = "plein"
	r.c.EffectsStep()
	st := r.c.Scene(zdIdle)
	for _, area := range []lighting.Area{lighting.AreaHost, lighting.AreaPublic} {
		if z, ok := zoneEntry(st, area, 0); !ok || z.Color != [3]int{0x11, 0x22, 0x33} || z.Intensity != 128 {
			t.Fatalf("zone %d : l'effet s'applique : %+v %v", area, z, ok)
		}
	}
	// The effect ends: POST is held in the public zone only; the animateur zone is repainted white.
	r.src.eff["animateur"]["calm"], r.src.eff["public"]["calm"] = "", ""
	r.c.EffectsStep()
	st = r.c.Scene(zdIdle)
	if z, ok := zoneEntry(st, lighting.AreaPublic, 0); !ok || z.Color != [3]int{0, 255, 0} {
		t.Fatalf("POST appliqué en zone public : %+v %v", z, ok)
	}
	if z, _ := zoneEntry(st, lighting.AreaHost, 0); z.Intensity != 255 || z.Color != [3]int{255, 255, 255} {
		t.Fatalf("POST sans effet en zone animateur : blanc fixe : %+v", z)
	}
	// After its window nothing is sent to the public zone any more (the bulbs keep the POST state).
	r.now = r.now.Add(2 * time.Second)
	r.c.EffectsStep()
	if n := areaCount(r.c.Scene(zdIdle), lighting.AreaPublic); n != 0 {
		t.Fatalf("la zone public n'est plus écrite : %d entrées", n)
	}
}

func TestZones_EffectTrial_ZoneSpeedRefusalsAndReturn_Dev(t *testing.T) {
	r := newZdRig(t, []config.LightingGroupEntry{
		{ID: "a", Role: "salle", Lights: []string{"l1"}},
		{ID: "a2", Role: "salle", Lights: []string{"l2"}},
		{ID: "p", Role: "public", Lights: []string{"p1"}},
	})
	// Refusals.
	if _, err := r.c.StartEffectTrial("inconnu", "ambiance", 1, 10); err != ErrTrialUnknownEffect {
		t.Fatalf("effet inconnu : %v", err)
	}
	for _, c := range []struct{ id, zone, reason string }{
		{"plein", "animateur", "not_applicable"}, // no animateur group
		{"tour", "public", "not_applicable"},     // needs 2 groups, 1 in the zone
	} {
		_, err := r.c.StartEffectTrial(c.id, c.zone, 1, 10)
		if tr, ok := err.(*TrialError); !ok || tr.Reason != c.reason {
			t.Fatalf("%s/%s : %v, attendu %s", c.id, c.zone, err, c.reason)
		}
	}
	_ = r.c.SetLightingMode("OFF")
	if _, err := r.c.StartEffectTrial("tour", "ambiance", 1, 10); err == nil || err.(*TrialError).Reason != "mode" {
		t.Fatalf("mode OFF ⇒ refus « mode » en zone ambiance : %v", err)
	}
	if _, err := r.c.StartEffectTrial("plein", "public", 1, 10); err != nil {
		t.Fatalf("la zone public ignore ON/OFF/Flash : %v", err)
	}
	r.c.StopEffectTrial()
	_ = r.c.SetLightingMode("AUTO")

	// Trial on the room at ×2: the head moves every 500 ms; the public zone is untouched.
	ms, err := r.c.StartEffectTrial("tour", "ambiance", 2, 10)
	if err != nil || ms != 10000 {
		t.Fatalf("essai : %v %d", err, ms)
	}
	st := r.c.Scene(zdIdle)
	if z, ok := zoneEntry(st, lighting.AreaAmbiance, 1); !ok || z.Color != [3]int{255, 0, 0} {
		t.Fatalf("tick 0 : tête sur le groupe 1 : %+v", st.Zones)
	}
	if areaCount(st, lighting.AreaPublic) != 0 {
		t.Fatal("un essai ne touche que la zone visée")
	}
	r.now = r.now.Add(500 * time.Millisecond)
	if z, ok := zoneEntry(r.c.Scene(zdIdle), lighting.AreaAmbiance, 2); !ok || z.Color != [3]int{255, 0, 0} {
		t.Fatal("×2 : un tick dure 500 ms")
	}
	// Game running ⇒ refused; the end of the trial gives the zone back its live state.
	r.now = r.now.Add(10 * time.Second)
	if _, ok := zoneEntry(r.c.Scene(zdIdle), lighting.AreaAmbiance, 1); ok {
		t.Fatal("après la durée : retour à l'état vivant")
	}
	r.c.StopEffectTrial()
	r.c.StopEffectTrial() // harmless when nothing runs
}

func TestZones_GroupTestAddressesAreaAndGroup_Dev(t *testing.T) {
	r := newZdRig(t, []config.LightingGroupEntry{
		{ID: "a", Role: "salle", Lights: []string{"l1"}},
		{ID: "p", Role: "public", Lights: []string{}},
		{ID: "p2", Role: "public", Lights: []string{"p1"}},
	})
	for ref, want := range map[string]error{"host1": ErrUnknownGroup, "public9": ErrUnknownGroup, "public1": ErrEmptyGroup, "x": ErrUnknownGroup} {
		if err := r.c.StartGroupTest(ref); err != want {
			t.Fatalf("%q : %v, attendu %v", ref, err, want)
		}
	}
	if err := r.c.StartGroupTest("public2"); err != nil {
		t.Fatal(err)
	}
	st := r.c.Scene(zdIdle)
	if z, ok := zoneEntry(st, lighting.AreaPublic, 2); !ok || z.Intensity != 0 {
		t.Fatalf("pas 1 (éteint) sur le groupe 2 de la zone public : %+v %v", z, ok)
	}
	r.now = r.now.Add(3 * time.Second)
	if z, _ := zoneEntry(r.c.Scene(zdIdle), lighting.AreaPublic, 2); z.Color != [3]int{255, 0, 0} {
		t.Fatalf("pas 4 rouge : %+v", z)
	}
	r.c.StopGroupTest()
	if areaCount(r.c.Scene(zdIdle), lighting.AreaPublic) != 0 {
		t.Fatal("après l'arrêt, la zone public n'est plus écrite")
	}
}

// Shutdown / master switch: the public bulbs are NOT given their saved state back (a late restore
// could light them again after the extinction) and a restore already in flight is aborted.
func TestZones_QuiesceAbandonsPublicRestore_Dev(t *testing.T) {
	r := newZdRig(t, []config.LightingGroupEntry{{ID: "p", Role: "public", Lights: []string{"p1"}}})
	r.c.public.mu.Lock()
	r.c.public.held, r.c.public.read = true, true
	r.c.public.mu.Unlock()
	slow, cancel := context.WithTimeout(context.Background(), time.Minute) // a slow restore in flight
	r.c.public.mu.Lock()
	r.c.public.cancel = cancel
	r.c.public.mu.Unlock()

	r.c.quiesceAmbiance()

	if slow.Err() == nil {
		t.Fatal("la restauration en cours est annulée à l'arrêt")
	}
	r.c.public.mu.Lock()
	held := r.c.public.held
	r.c.public.mu.Unlock()
	if held {
		t.Fatal("l'état mémorisé est abandonné : plus aucune restauration possible")
	}
	r.c.releasePublicIfIdle() // nothing left to restore, must not panic nor write
	if _, err := r.c.StartEffectTrial("plein", "public", 1, 10); err == nil {
		t.Fatal("après l'arrêt, plus d'essai (éclairage coupé)")
	}
}
