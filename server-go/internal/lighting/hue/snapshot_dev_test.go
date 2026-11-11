package hue

import (
	"context"
	"testing"

	"buzzcontrol/internal/lighting"
)

// Public area (light-effects.md §2.5): a light of the public area with no entry is never
// written; around a test its state is read on the bridge and written back.

func devPublicSpec(name string) LightSpec {
	return LightSpec{Name: name, Role: RoleGeneral, Area: lighting.AreaPublic, Group: 1}
}

func TestDevPublicArea_NoEntryNoWrite_AndSnapshotRestore(t *testing.T) {
	f := newDevBridge(t, "Room", "Pub")
	d, _ := newDevDriver(t, f, LightSpec{Name: "Room", Role: RoleGeneral}, devPublicSpec("Pub"))
	ctx := context.Background()

	// Scene without any public entry: only the room is written.
	room := lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Color: [3]int{255, 255, 255}, Intensity: 255}}}
	if err := d.Apply(ctx, room); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.puts() {
		if p.path == "/api/k/lights/2/state" {
			t.Fatalf("la lampe publique ne reçoit rien sans entrée : %+v", p)
		}
	}

	// Put the public bulb in a known state by hand, snapshot it, drive it, restore it.
	f.mu.Lock()
	f.lights["2"]["state"] = map[string]any{"on": true, "bri": 120, "xy": []float64{0.41, 0.35}, "reachable": true}
	f.mu.Unlock()
	snaps, err := d.SnapshotLights(ctx, []string{"Pub", "Inconnue"})
	if err != nil || len(snaps) != 1 || !snaps[0].On || snaps[0].Bri != 120 || !snaps[0].HasXY || snaps[0].XY[0] != 0.41 {
		t.Fatalf("lecture de l'état : %v %+v", err, snaps)
	}
	pub := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{255, 255, 255}, Intensity: 255},
		{Zone: lighting.ZoneGeneral, Area: lighting.AreaPublic, Color: [3]int{255, 0, 0}, Intensity: 255},
	}}
	if err := d.Apply(ctx, pub); err != nil {
		t.Fatal(err)
	}
	if err := d.RestoreLights(ctx, snaps, nil); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	st := f.lights["2"]["state"].(map[string]any)
	f.mu.Unlock()
	if st["on"] != true || devFloat(st["bri"]) != 120 {
		t.Fatalf("état rétabli : %+v", st)
	}
	// Unreadable state ⇒ switched off.
	if err := d.RestoreLights(ctx, nil, []string{"Pub"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	st = f.lights["2"]["state"].(map[string]any)
	f.mu.Unlock()
	if st["on"] != false {
		t.Fatalf("repli : éteinte : %+v", st)
	}
}

func devFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return -1
}
