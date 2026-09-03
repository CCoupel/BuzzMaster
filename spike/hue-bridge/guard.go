package main

// Safety guard — the ONLY door to the bridge.
//
// The bridge under test is the user's HOME Hue Bridge, with real lights and
// automations. This spike is allowed to do exactly three things and nothing
// else (handoff task-dev-backend-20260903-1945-hue-bridge-spike.md):
//
//   - POST /api                              application registration
//   - GET  /api/<user>/lights[/<id>]         find the test light by name
//   - PUT  /api/<user>/lights/<id>/state     on/off/brightness/colour of THAT light
//
// Every HTTP request goes through guardRequest, which rejects any other
// method/path at the code level: no groups (never group 0 = "all lights"),
// no scenes, rules, schedules, sensors, resourcelinks, config, whitelist,
// firmware, no DELETE — whatever a future edit of this program might try.
// In addition the light id written to must be a positive integer and
// resolveTarget() re-reads the light and checks its name right before every
// write (see hue.go).

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var (
	reLightsList  = regexp.MustCompile(`^/api/[^/]+/lights$`)
	reLightOne    = regexp.MustCompile(`^/api/[^/]+/lights/([1-9][0-9]*)$`)
	reLightState  = regexp.MustCompile(`^/api/[^/]+/lights/([1-9][0-9]*)/state$`)
	reRegister    = regexp.MustCompile(`^/api$`)
	errGuardBlock = "SAFETY GUARD: refused %s %s — only GET /api/<user>/lights[/<id>], PUT /api/<user>/lights/<id>/state and POST /api are allowed on the home bridge"
)

// guardRequest returns an error unless (method, path) is one of the three
// allowed operations. path must be the URL path only (no host, no query).
func guardRequest(method, path string) error {
	if strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
		return fmt.Errorf(errGuardBlock, method, path)
	}
	switch method {
	case http.MethodGet:
		if reLightsList.MatchString(path) || reLightOne.MatchString(path) {
			return nil
		}
	case http.MethodPut:
		if reLightState.MatchString(path) {
			return nil
		}
	case http.MethodPost:
		if reRegister.MatchString(path) {
			return nil
		}
	}
	return fmt.Errorf(errGuardBlock, method, path)
}

// lightIDFromStatePath extracts the light id of an allowed PUT path.
func lightIDFromStatePath(path string) string {
	m := reLightState.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	return m[1]
}
