package server

import (
	"testing"

	"buzzcontrol/internal/protocol"
)

// #144 (contracts/roue.md §8.2) — who may send which ROUE action.
func TestRoueAllowlist144(t *testing.T) {
	stepButtons := []string{
		protocol.ActionRoueSpin, protocol.ActionRoueStop, protocol.ActionRoueLetter,
		protocol.ActionRoueFlip, protocol.ActionRoueAnswer, protocol.ActionRoueNext,
	}
	adminOnly := []string{protocol.ActionRoueSetTeams, protocol.ActionRouePass, protocol.ActionRoueReveal}
	for _, a := range stepButtons {
		if !IsActionAllowed(a, ClientTypeAdmin) || !IsActionAllowed(a, ClientTypeAnim) {
			t.Errorf("%s: admin and animateur must be allowed", a)
		}
	}
	for _, a := range adminOnly {
		if !IsActionAllowed(a, ClientTypeAdmin) || IsActionAllowed(a, ClientTypeAnim) {
			t.Errorf("%s: admin only", a)
		}
	}
	for _, a := range append(stepButtons, adminOnly...) {
		for _, ct := range []ClientType{ClientTypeTV, ClientTypeVPlayer} {
			if IsActionAllowed(a, ct) {
				t.Errorf("%s must be refused to client type %v", a, ct)
			}
		}
		if IsActionAllowedDuringEntracte(a) {
			t.Errorf("%s must be refused during an entracte", a)
		}
	}
}
