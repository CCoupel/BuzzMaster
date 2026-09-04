package hue

// Request guard — the ONLY door to the bridge (contracts/hue-bridge.md §4.3,
// promoted from spike/hue-bridge/guard.go).
//
// Every HTTP request of this package goes through guardRequest, which lets
// exactly four operations through and rejects anything else BEFORE it is
// sent: no groups (never group 0 = "all lights"), no scenes, rules,
// schedules, sensors, resourcelinks, whitelist, firmware, no DELETE, no
// light renaming, no API v2, no query strings or path traversal.

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var (
	reLightsList = regexp.MustCompile(`^/api/[^/]+/lights$`)
	reLightOne   = regexp.MustCompile(`^/api/[^/]+/lights/([1-9][0-9]*)$`)
	reLightState = regexp.MustCompile(`^/api/[^/]+/lights/([1-9][0-9]*)/state$`)
	reConfig     = regexp.MustCompile(`^/api/[^/]+/config$`)
	reRegister   = regexp.MustCompile(`^/api$`)
)

// ErrGuard is returned for any request outside the allow-list.
type ErrGuard struct{ Method, Path string }

func (e ErrGuard) Error() string {
	return fmt.Sprintf("hue guard: refused %s %s — only POST /api, GET lights[/<id>], GET config and PUT lights/<id>/state are allowed", e.Method, e.Path)
}

// guardRequest returns an error unless (method, path) is an allowed
// operation. path is the URL path only (no host, no query).
func guardRequest(method, path string) error {
	if strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
		return ErrGuard{method, path}
	}
	switch method {
	case http.MethodGet:
		if reLightsList.MatchString(path) || reLightOne.MatchString(path) || reConfig.MatchString(path) {
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
