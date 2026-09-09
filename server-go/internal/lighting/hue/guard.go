package hue

// Request guard — the ONLY door to the bridge (contracts/hue-bridge.md §4.3,
// promoted from spike/hue-bridge/guard.go).
//
// Every HTTP request of this package goes through guardRequest, which lets
// through registration, light inventory/read/write, and — since contracts/
// hue-bridge.md §5.8 (2026-09-07, "clause de sortie DÉCLENCHÉE") — a narrow
// set of BuzzMaster-managed LightGroup operations: list/create groups, read
// group state is never needed so it is not allowed, update the membership of
// or delete ONE group by a strictly positive id, and write that group's
// action (state). Group id 0 ("all lights", Hue's implicit magic group) is
// structurally excluded by the same [1-9][0-9]* pattern used for lights —
// there is no separate carve-out to remember. Still refused: scenes, rules,
// schedules, sensors, resourcelinks, whitelist, firmware, light/group
// renaming, API v2, query strings, path traversal.

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
	reConfig      = regexp.MustCompile(`^/api/[^/]+/config$`)
	reRegister    = regexp.MustCompile(`^/api$`)
	reGroupsList  = regexp.MustCompile(`^/api/[^/]+/groups$`)
	reGroupOne    = regexp.MustCompile(`^/api/[^/]+/groups/([1-9][0-9]*)$`)
	reGroupAction = regexp.MustCompile(`^/api/[^/]+/groups/([1-9][0-9]*)/action$`)
)

// ErrGuard is returned for any request outside the allow-list.
type ErrGuard struct{ Method, Path string }

func (e ErrGuard) Error() string {
	return fmt.Sprintf("hue guard: refused %s %s — only POST /api, GET lights[/<id>], GET config, PUT lights/<id>/state, GET/POST groups, PUT/DELETE groups/<id> and PUT groups/<id>/action are allowed", e.Method, e.Path)
}

// guardRequest returns an error unless (method, path) is an allowed
// operation. path is the URL path only (no host, no query).
func guardRequest(method, path string) error {
	if strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
		return ErrGuard{method, path}
	}
	switch method {
	case http.MethodGet:
		if reLightsList.MatchString(path) || reLightOne.MatchString(path) || reConfig.MatchString(path) || reGroupsList.MatchString(path) {
			return nil
		}
	case http.MethodPut:
		if reLightState.MatchString(path) || reGroupOne.MatchString(path) || reGroupAction.MatchString(path) {
			return nil
		}
	case http.MethodPost:
		if reRegister.MatchString(path) || reGroupsList.MatchString(path) {
			return nil
		}
	case http.MethodDelete:
		if reGroupOne.MatchString(path) {
			return nil
		}
	}
	return ErrGuard{method, path}
}

// GuardRequest is the exported form of guardRequest (test-writer seam; also
// usable by #207's handlers to pre-check a path).
func GuardRequest(method, path string) error { return guardRequest(method, path) }

// LightIDFromStatePath extracts the light id of an allowed PUT state path
// ("" if the path is not one).
func LightIDFromStatePath(path string) string {
	m := reLightState.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	return m[1]
}

// validLightID reports whether id is a strictly positive integer in canonical form.
func validLightID(id string) bool {
	return reLightOne.MatchString("/api/x/lights/" + id)
}

// validGroupID reports whether id is a strictly positive integer in
// canonical form (never "0", the implicit "all lights" group).
func validGroupID(id string) bool {
	return reGroupOne.MatchString("/api/x/groups/" + id)
}
