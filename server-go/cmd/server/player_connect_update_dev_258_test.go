package main

import (
	"testing"
	"time"
)

// #258 review (mineur 2): after PLAYER_CONNECT binds a tablette, a targeted UPDATE
// (filtered for it) follows, so the badges of its team are not empty until the next UPDATE.
func TestPlayerConnect_TargetedUpdateFollowsTheBinding(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")

	e.connect(t, cid, "Paul", e.paul)

	readActionMatchingWithin(t, conn, "PLAYER_CONNECTED", 2*time.Second)
	readActionMatchingWithin(t, conn, "UPDATE", 2*time.Second)
}
