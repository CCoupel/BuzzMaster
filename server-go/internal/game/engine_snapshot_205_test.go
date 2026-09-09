package game

import "testing"

// #205 — the ambiance writer derives the scene on its own goroutine, so it
// must read teams/bumpers through a copy, never the live e.data maps.
func TestGetTeamsAndBumpersSnapshot_IsDeepCopy(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"A": {Name: "A", Color: []int{255, 0, 0}, ColorName: "rouge"}})
	e.SetBumpers(map[string]*Bumper{"m1": {Name: "m1", Team: "A", Time: 42}})

	snap := e.GetTeamsAndBumpersSnapshot()
	if len(snap.Teams) != 1 || len(snap.Bumpers) != 1 {
		t.Fatalf("snapshot sizes = %d/%d", len(snap.Teams), len(snap.Bumpers))
	}
	if snap.Bumpers["m1"].Time != 42 || snap.Teams["A"].ColorName != "rouge" {
		t.Fatalf("snapshot content wrong: %+v %+v", snap.Bumpers["m1"], snap.Teams["A"])
	}

	// Mutating the snapshot must not touch the engine...
	snap.Bumpers["m1"].Time = 0
	snap.Teams["A"].Color[0] = 1
	delete(snap.Bumpers, "m1")
	live := e.GetTeamsAndBumpers()
	if live.Bumpers["m1"] == nil || live.Bumpers["m1"].Time != 42 || live.Teams["A"].Color[0] != 255 {
		t.Fatalf("snapshot mutation leaked into the engine: %+v %+v", live.Bumpers["m1"], live.Teams["A"])
	}

	// ...and mutating the engine must not touch an earlier snapshot.
	snap2 := e.GetTeamsAndBumpersSnapshot()
	e.UpdateBumper("m1", map[string]interface{}{"TEAM": "B"})
	if snap2.Bumpers["m1"].Team != "A" {
		t.Fatalf("engine mutation leaked into the snapshot: %+v", snap2.Bumpers["m1"])
	}
}

func TestGetTeamsAndBumpersSnapshot_EmptyEngine(t *testing.T) {
	snap := NewEngine().GetTeamsAndBumpersSnapshot()
	if snap.Teams == nil || snap.Bumpers == nil || len(snap.Teams)+len(snap.Bumpers) != 0 {
		t.Fatalf("empty snapshot must have non-nil empty maps: %+v", snap)
	}
}
