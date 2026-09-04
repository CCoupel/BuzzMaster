package hue

import "testing"

// The guard is the safety-critical piece (contract §4.3): every forbidden
// operation must be rejected at the code level, whatever the caller does.
func TestGuardAllowsOnlyTheFourOperations(t *testing.T) {
	allowed := []struct{ m, p string }{
		{"GET", "/api/abc/lights"},
		{"GET", "/api/abc/lights/7"},
		{"GET", "/api/abc/config"},
		{"PUT", "/api/abc/lights/7/state"},
		{"POST", "/api"},
	}
	for _, a := range allowed {
		if err := guardRequest(a.m, a.p); err != nil {
			t.Errorf("%s %s must be allowed: %v", a.m, a.p, err)
		}
	}
	forbidden := []struct{ m, p string }{
		{"PUT", "/api/abc/groups/0/action"}, // ALL lights
		{"PUT", "/api/abc/groups/3/action"},
		{"GET", "/api/abc/groups"},
		{"POST", "/api/abc/groups"},
		{"PUT", "/api/abc/config"}, // config is read-only here
		{"PUT", "/api/abc/scenes/x"},
		{"POST", "/api/abc/scenes"},
		{"PUT", "/api/abc/rules/1"},
		{"PUT", "/api/abc/schedules/1"},
		{"PUT", "/api/abc/sensors/1/state"},
		{"PUT", "/api/abc/resourcelinks/1"},
		{"DELETE", "/api/abc/lights/7"},
		{"DELETE", "/api/abc/config/whitelist/xyz"},
		{"GET", "/api/abc/config/whitelist"},
		{"PUT", "/api/abc/lights/0/state"}, // id 0
		{"PUT", "/api/abc/lights/7"},       // rename = attribute write
		{"PUT", "/api/abc/lights"},
		{"POST", "/api/abc/lights"}, // search for new lights
		{"PUT", "/api/abc/lights/7/state?x"},
		{"GET", "/api/abc/../config"},
		{"GET", "/api"},
		{"POST", "/api/abc/lights/7/state"},
		{"PUT", "/api/abc/lights/07/state"}, // odd id form
		{"PUT", "/api/abc/lights/-1/state"},
		{"PUT", "/clip/v2/resource/light/x"}, // API v2
		{"GET", "/api/abc/lights/7/state"},
		{"PUT", "/api/abc/lights/7/state#frag"},
	}
	for _, f := range forbidden {
		if err := guardRequest(f.m, f.p); err == nil {
			t.Errorf("%s %s MUST be refused", f.m, f.p)
		}
	}
	for id, ok := range map[string]bool{"1": true, "8": true, "42": true, "0": false, "": false, "07": false, "-1": false, "a": false, "1/2": false} {
		if validLightID(id) != ok {
			t.Errorf("validLightID(%q) = %v, want %v", id, !ok, ok)
		}
	}
}
