package ambiance

import (
	"testing"
	"testing/fstest"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// Reader tests (#223, Batch 2.2): no ticker, no sleep — EffectsStep is driven by hand with
// an injected clock. They swap the config singleton, so none is parallel.

type devEffectsSource struct {
	cat        effects.Catalogue
	room, host map[string]string
}

func (s devEffectsSource) EffectsCatalogue() effects.Catalogue { return s.cat }
func (s devEffectsSource) EffectSettings() (map[string]string, map[string]string) {
	return s.room, s.host
}

const devAllEffect = `id: fixe
name: Fixe
version: 1
moments: [calm, score]
target: all
duration_ms: 4800
looks: { a: { color: "#112233", intensity: 50 } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`

const devShiftEffect = `id: tour
name: Tour
version: 1
moments: [calm]
target: groups
min_groups: 2
looks:
  on:  { color: "#FF0000" }
  off: { color: "#000000", intensity: 0 }
pattern: { type: shift, step_ms: 1000, cells: [on], rest: off }
`

type devRig struct {
	c   *Controller
	src *devEffectsSource
	now time.Time
}

func newDevRig(t *testing.T, groups []config.LightingGroupEntry) *devRig {
	t.Helper()
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })
	cfg := *prev
	cfg.Lighting.Groups = groups
	config.SetInstance(&cfg)

	c := New(fakeHost{engine: game.NewEngine()})
	c.LightingWriter.Store(c.NewWriter(lighting.NewFakeDriver()))
	fs := fstest.MapFS{
		"fixe.yaml": {Data: []byte(devAllEffect)},
		"tour.yaml": {Data: []byte(devShiftEffect)},
	}
	r := &devRig{c: c, now: time.Unix(1_700_000_000, 0)}
	r.src = &devEffectsSource{cat: effects.LoadDir(fs, ""), room: map[string]string{}, host: map[string]string{}}
	c.SetEffectsSource(r.src)
	c.EffectsClock = func() time.Time { return r.now }
	return r
}

func zoneFor(st lighting.State, group int) (lighting.ZoneState, bool) {
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Group == group {
			return z, true
		}
	}
	return lighting.ZoneState{}, false
}

func TestEffectsReader_CalmRoomEffect_TargetAll_AndModePrecedence_Dev(t *testing.T) {
	r := newDevRig(t, nil)
	idle := lighting.Event{Kind: lighting.KindIdle}
	before := r.c.Scene(idle)

	r.src.room["calm"] = "fixe"
	r.c.EffectsStep()
	z, _ := zoneFor(r.c.Scene(idle), 0)
	if z.Color != [3]int{0x11, 0x22, 0x33} || z.Intensity != 128 {
		t.Fatalf("effet de salle attendu sur la zone générale : %+v", z)
	}
	// A frame computed for another moment is never applied.
	if z, _ := zoneFor(r.c.Scene(lighting.Event{Kind: lighting.KindRunning}), 0); z.Color == [3]int{0x11, 0x22, 0x33} {
		t.Fatal("une trame calm ne s'applique pas au moment chrono")
	}
	// Mode OFF: no frame, the override applies as before (§5.3 point 2).
	if err := r.c.SetLightingMode("OFF"); err != nil {
		t.Fatal(err)
	}
	r.c.EffectsStep()
	if r.c.effectFrames.Load() != nil {
		t.Fatal("mode OFF ⇒ aucune trame")
	}
	if got := r.c.Scene(idle).Zones[0]; got.Intensity != 0 {
		t.Fatalf("OFF reste prioritaire : %+v", got)
	}
	// No setting ⇒ v11.6.0 scene unchanged.
	_ = r.c.SetLightingMode("AUTO")
	r.src.room["calm"] = ""
	r.c.EffectsStep()
	if after := r.c.Scene(idle); len(after.Zones) != len(before.Zones) || after.Zones[0] != before.Zones[0] {
		t.Fatalf("sans effet, scène inchangée : %+v / %+v", before, after)
	}
}

func TestEffectsReader_Groups_RealNumbersEmptyGroupSkipped_Dev(t *testing.T) {
	r := newDevRig(t, []config.LightingGroupEntry{
		{Name: "A", Role: "salle", Lights: []string{"l1"}},
		{Name: "Vide", Role: "salle", Lights: []string{}},
		{Name: "C", Role: "salle", Lights: []string{"l2"}},
	})
	r.src.room["calm"] = "tour" // min_groups 2 : 2 non-empty groups suffice
	r.c.EffectsStep()
	st := r.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if _, ok := zoneFor(st, 1); !ok {
		t.Fatalf("groupe 1 attendu : %+v", st.Zones)
	}
	if _, ok := zoneFor(st, 2); ok {
		t.Fatal("le groupe vide (numéro 2) ne reçoit aucune entrée")
	}
	if z, ok := zoneFor(st, 3); !ok || z.Intensity != 0 {
		t.Fatalf("groupe 3 = repos du chenillard (rest) : %+v %v", z, ok)
	}
	if z, _ := zoneFor(st, 1); z.Color != [3]int{255, 0, 0} {
		t.Fatalf("la tête est sur le premier groupe non vide : %+v", z)
	}
	// One second later the head moves to the next NON-EMPTY group (3), not the empty one.
	r.now = r.now.Add(time.Second)
	r.c.EffectsStep()
	st = r.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if z, _ := zoneFor(st, 3); z.Color != [3]int{255, 0, 0} {
		t.Fatalf("la tête passe au groupe 3 : %+v", st.Zones)
	}
	// Not applicable with a single group: ordinary scene, no Group entry.
	r.src.room["calm"] = "tour"
	cfg := config.Get()
	cfg.Lighting.Groups = cfg.Lighting.Groups[:1]
	r.c.EffectsStep()
	if r.c.effectFrames.Load() != nil {
		t.Fatal("effet inapplicable ⇒ valeur par défaut du moment (aucune trame)")
	}
}

func TestEffectsReader_HostGroup_OwnEffect_IgnoresModeAndFlash_Dev(t *testing.T) {
	r := newDevRig(t, []config.LightingGroupEntry{{Name: "Pupitre", Role: "animateur", Lights: []string{"h1"}}})
	idle := lighting.Event{Kind: lighting.KindIdle}
	if z, _ := zoneFor(r.c.Scene(idle), lighting.GroupHost); z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
		t.Fatalf("blanc fixe par défaut : %+v", z)
	}
	r.src.host["calm"] = "fixe"
	_ = r.c.SetLightingMode("OFF")
	r.c.SetLightingFlash(true)
	defer r.c.SetLightingFlash(false)
	r.c.EffectsStep()
	z, ok := zoneFor(r.c.Scene(idle), lighting.GroupHost)
	if !ok || z.Color != [3]int{0x11, 0x22, 0x33} || z.Intensity != 128 {
		t.Fatalf("l'effet du groupe animateur s'applique malgré OFF + Flash : %+v %v", z, ok)
	}
	// Unknown effect ⇒ fixed white again.
	r.src.host["calm"] = "inconnu"
	r.c.EffectsStep()
	if z, _ := zoneFor(r.c.Scene(idle), lighting.GroupHost); z.Intensity != 255 {
		t.Fatalf("effet absent ⇒ blanc fixe : %+v", z)
	}
}

func TestEffectsReader_ScoreMoment_DurationAndQuiesce_Dev(t *testing.T) {
	r := newDevRig(t, nil)
	r.src.room["score"] = "fixe"
	score := lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}
	r.c.EffectsStep()
	if r.c.effectFrames.Load() != nil {
		t.Fatal("pas d'impulsion SCORE ⇒ pas de trame")
	}
	r.c.StartScoreFlash("Rouges", 5)
	defer func() { r.c.scoreFlashCancel() }()
	r.c.EffectsStep()
	if z, _ := zoneFor(r.c.Scene(score), 0); z.Color != [3]int{0x11, 0x22, 0x33} {
		t.Fatalf("effet du moment score : %+v", z)
	}
	r.now = r.now.Add(5 * time.Second) // beyond ScorePulseDuration and duration_ms
	r.c.EffectsStep()
	if r.c.effectFrames.Load() != nil {
		t.Fatal("fin de l'impulsion ⇒ plus de trame")
	}
	r.c.StartScoreFlash("Rouges", 5)
	r.c.EffectsStep()
	r.c.quiesceAmbiance()
	r.c.EffectsStep()
	if r.c.effectFrames.Load() != nil {
		t.Fatal("après quiesce, le lecteur ne publie plus")
	}
}
