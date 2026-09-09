package main

// Minimal Hue API v1 client (CLIP v1, HTTP/JSON), restricted by guard.go.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to one bridge with one API username.
type Client struct {
	Base string // e.g. "http://192.168.1.10" — scheme + host only
	User string
	http *http.Client
}

// NewClient builds a client. https uses InsecureSkipVerify: the bridge
// presents a self-signed certificate (Signify root, not in system stores).
func NewClient(base, user string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("bridge URL must be scheme://host (got %q)", base)
	}
	tr := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: timeout}).DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec — local bridge, self-signed
		MaxIdleConnsPerHost: 2,
	}
	return &Client{Base: strings.TrimRight(base, "/"), User: user, http: &http.Client{Timeout: timeout, Transport: tr}}, nil
}

// do performs one guarded request and returns the raw body.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, time.Duration, error) {
	if err := guardRequest(method, path); err != nil {
		return nil, 0, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	d := time.Since(start)
	if err != nil {
		return nil, d, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, d, err
	}
	if resp.StatusCode/100 != 2 {
		return data, d, fmt.Errorf("HTTP %d: %s", resp.StatusCode, firstLine(string(data)))
	}
	return data, d, nil
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

// hueError is the {"error":{...}} element of a Hue v1 response array.
type hueError struct {
	Type        int    `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

func (e hueError) Error() string {
	return fmt.Sprintf("hue error %d at %s: %s", e.Type, e.Address, e.Description)
}

// parseResultArray splits a Hue v1 result array into successes and errors.
func parseResultArray(data []byte) (successes []map[string]json.RawMessage, errs []hueError, err error) {
	var arr []map[string]json.RawMessage
	if jerr := json.Unmarshal(data, &arr); jerr != nil {
		return nil, nil, fmt.Errorf("unexpected response %q: %w", firstLine(string(data)), jerr)
	}
	for _, item := range arr {
		if raw, ok := item["error"]; ok {
			var e hueError
			_ = json.Unmarshal(raw, &e)
			errs = append(errs, e)
		}
		if raw, ok := item["success"]; ok {
			var m map[string]json.RawMessage
			if json.Unmarshal(raw, &m) == nil {
				successes = append(successes, m)
			}
		}
	}
	return successes, errs, nil
}

// ---------------------------------------------------------------------------
// Registration (POST /api) — the standard link-button flow
// ---------------------------------------------------------------------------

const errLinkButtonNotPressed = 101

// Register obtains an API username. It retries until the bridge accepts
// (user pressed the link button) or the deadline passes.
func Register(ctx context.Context, base, devicetype string, timeout time.Duration, wait time.Duration, log func(string, ...any)) (string, error) {
	c, err := NewClient(base, "", timeout)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(wait)
	log(">>> Press the LINK BUTTON on the Hue Bridge now (retrying for %s) <<<", wait)
	for {
		data, _, err := c.do(ctx, http.MethodPost, "/api", map[string]string{"devicetype": devicetype})
		if err == nil {
			succ, errs, perr := parseResultArray(data)
			if perr != nil {
				return "", perr
			}
			for _, s := range succ {
				if raw, ok := s["username"]; ok {
					var user string
					if json.Unmarshal(raw, &user) == nil && user != "" {
						return user, nil
					}
				}
			}
			for _, e := range errs {
				if e.Type != errLinkButtonNotPressed {
					return "", e
				}
			}
			// 101: keep waiting
		} else if ctx.Err() != nil {
			return "", ctx.Err()
		} else {
			log("  registration attempt failed: %v", err)
		}
		if time.Now().After(deadline) {
			return "", errors.New("link button not pressed within the wait time")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

const userFile = ".hue-username"

func loadUser() string {
	b, err := os.ReadFile(userFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func saveUser(user string) error {
	return os.WriteFile(userFile, []byte(user+"\n"), 0o600)
}

// ---------------------------------------------------------------------------
// Lights (read) and the single write operation
// ---------------------------------------------------------------------------

// Light is the subset of GET /lights/<id> the spike needs.
type Light struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Model string `json:"modelid"`
	State struct {
		On        bool      `json:"on"`
		Bri       int       `json:"bri"`
		XY        []float64 `json:"xy"`
		Reachable bool      `json:"reachable"`
	} `json:"state"`
}

// Lights lists all lights (read-only). Only names/ids are logged by callers.
func (c *Client) Lights(ctx context.Context) (map[string]Light, time.Duration, error) {
	data, d, err := c.do(ctx, http.MethodGet, "/api/"+c.User+"/lights", nil)
	if err != nil {
		return nil, d, err
	}
	var m map[string]Light
	if err := json.Unmarshal(data, &m); err != nil {
		// A bad username yields an error array instead of an object.
		if _, errs, perr := parseResultArray(data); perr == nil && len(errs) > 0 {
			return nil, d, errs[0]
		}
		return nil, d, fmt.Errorf("unexpected /lights response: %w", err)
	}
	return m, d, nil
}

// Light reads one light by id (read-only).
func (c *Client) Light(ctx context.Context, id string) (Light, time.Duration, error) {
	data, d, err := c.do(ctx, http.MethodGet, "/api/"+c.User+"/lights/"+id, nil)
	if err != nil {
		return Light{}, d, err
	}
	var l Light
	if err := json.Unmarshal(data, &l); err != nil {
		if _, errs, perr := parseResultArray(data); perr == nil && len(errs) > 0 {
			return Light{}, d, errs[0]
		}
		return Light{}, d, fmt.Errorf("unexpected /lights/%s response: %w", id, err)
	}
	return l, d, nil
}

// Target is the one light this spike may write to.
type Target struct {
	ID   string
	Name string
}

var errTargetAmbiguous = errors.New("several lights carry the target name — refusing to guess")

// FindTarget resolves the test light by exact name. Exactly one match is
// required; anything else stops the program (never a fallback).
func (c *Client) FindTarget(ctx context.Context, name string) (Target, map[string]Light, error) {
	lights, _, err := c.Lights(ctx)
	if err != nil {
		return Target{}, nil, err
	}
	var ids []string
	for id, l := range lights {
		if l.Name == name {
			ids = append(ids, id)
		}
	}
	switch len(ids) {
	case 0:
		return Target{}, lights, fmt.Errorf("no light named %q on this bridge (%d lights) — nothing will be written", name, len(lights))
	case 1:
		if err := guardRequest(http.MethodPut, "/api/x/lights/"+ids[0]+"/state"); err != nil {
			return Target{}, lights, fmt.Errorf("light id %q is not a plain positive integer — refusing", ids[0])
		}
		return Target{ID: ids[0], Name: name}, lights, nil
	}
	return Target{}, lights, errTargetAmbiguous
}

// State is the writable subset of /lights/<id>/state.
type State struct {
	On             *bool     `json:"on,omitempty"`
	Bri            *int      `json:"bri,omitempty"`
	XY             []float64 `json:"xy,omitempty"`
	TransitionTime *int      `json:"transitiontime,omitempty"` // in 100 ms units; 0 = instant
}

// SetState is the ONLY write of this program. Before every call it re-reads
// the light and refuses unless its name is still the target name.
func (c *Client) SetState(ctx context.Context, t Target, st State) (time.Duration, error) {
	if t.ID == "" || t.ID == "0" || t.Name == "" {
		return 0, errors.New("SAFETY GUARD: empty or zero light id/name")
	}
	path := "/api/" + c.User + "/lights/" + t.ID + "/state"
	if err := guardRequest(http.MethodPut, path); err != nil {
		return 0, err
	}
	if lightIDFromStatePath(path) != t.ID {
		return 0, errors.New("SAFETY GUARD: path/id mismatch")
	}
	cur, dRead, err := c.Light(ctx, t.ID)
	if err != nil {
		return dRead, fmt.Errorf("pre-write check of light %s failed: %w", t.ID, err)
	}
	if cur.Name != t.Name {
		return dRead, fmt.Errorf("SAFETY GUARD: light %s is now named %q, expected %q — write refused", t.ID, cur.Name, t.Name)
	}
	data, d, err := c.do(ctx, http.MethodPut, path, st)
	if err != nil {
		return d, err
	}
	_, errs, perr := parseResultArray(data)
	if perr != nil {
		return d, perr
	}
	if len(errs) > 0 {
		return d, errs[0]
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Colour helpers (same maths as the BLE spike)
// ---------------------------------------------------------------------------

func rgbToXY(r, g, b uint8) []float64 {
	lin := func(c uint8) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	rl, gl, bl := lin(r), lin(g), lin(b)
	X := 0.4124*rl + 0.3576*gl + 0.1805*bl
	Y := 0.2126*rl + 0.7152*gl + 0.0722*bl
	Z := 0.0193*rl + 0.1192*gl + 0.9505*bl
	sum := X + Y + Z
	if sum == 0 {
		return []float64{0.3127, 0.3290}
	}
	return []float64{round4(X / sum), round4(Y / sum)}
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

var namedColors = map[string][3]uint8{
	"red": {255, 0, 0}, "green": {0, 255, 0}, "blue": {0, 0, 255}, "white": {255, 255, 255},
	"yellow": {255, 220, 0}, "orange": {255, 120, 0}, "magenta": {255, 0, 255}, "cyan": {0, 255, 255},
}

func colorXY(name string) ([]float64, error) {
	rgb, ok := namedColors[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, fmt.Errorf("unknown colour %q", name)
	}
	return rgbToXY(rgb[0], rgb[1], rgb[2]), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func boolp(b bool) *bool { return &b }
func intp(i int) *int    { return &i }
