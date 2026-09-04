package hue

// Minimal Hue API v1 (CLIP v1) client, restricted by guard.go, with the
// three-state error taxonomy of contracts/hue-bridge.md §5.6.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPTimeout is the per-request timeout (contract §5.5): the bridge is on
// the LAN and answers in ~40 ms — past 2 s it is unreachable, not slow.
const HTTPTimeout = 2 * time.Second

// BridgeState is the three-state health of the bridge link (contract §5.6) —
// never collapse refused and unreachable into one "error".
type BridgeState string

const (
	StateOK          BridgeState = "ok"
	StateRefused     BridgeState = "refused"     // key missing/invalid/revoked, link button not pressed
	StateUnreachable BridgeState = "unreachable" // DNS, network, TLS, timeout, bridge off
	StateDisabled    BridgeState = "disabled"    // no bridge configured (reported by the owner, not by a Driver)
)

// Sentinel errors carrying the taxonomy. Wrap them; callers use errors.Is.
var (
	ErrRefused     = errors.New("hue: refused")
	ErrUnreachable = errors.New("hue: unreachable")
)

// hueError is the {"error":{...}} element of a v1 response array.
type hueError struct {
	Type        int    `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

func (e hueError) Error() string {
	return fmt.Sprintf("hue error %d at %q: %s", e.Type, e.Address, e.Description)
}

const (
	hueErrUnauthorized      = 1   // unauthorized user → refused
	hueErrLinkButtonPressed = 101 // link button not pressed → refused (registration)
	hueErrDeviceOff         = 201 // parameter not modifiable, device is set to off
)

// classify wraps a transport/HTTP/Hue error into the taxonomy.
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrRefused) || errors.Is(err, ErrUnreachable) {
		return err
	}
	var he hueError
	if errors.As(err, &he) {
		if he.Type == hueErrUnauthorized || he.Type == hueErrLinkButtonPressed {
			return fmt.Errorf("%w: %v", ErrRefused, err)
		}
		return err // per-light / per-parameter error: neither state
	}
	var hs httpStatusError
	if errors.As(err, &hs) {
		if hs.Code == http.StatusUnauthorized || hs.Code == http.StatusForbidden {
			return fmt.Errorf("%w: %v", ErrRefused, err)
		}
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	var ge ErrGuard
	if errors.As(err, &ge) {
		return err
	}
	// url errors, dial errors, timeouts, TLS, context deadline
	return fmt.Errorf("%w: %v", ErrUnreachable, err)
}

type httpStatusError struct {
	Code int
	Body string
}

func (e httpStatusError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Code, e.Body) }

// client talks to one bridge with one API key.
type client struct {
	base string // scheme://host
	key  string
	http *http.Client
}

// bridgeBase normalises "192.168.1.10", "192.168.1.10:80" or "scheme://host"
// into scheme://host. https is only used when asked for; the self-signed
// bridge certificate is then accepted on this transport only (contract §3).
func bridgeBase(ip string, https bool) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", errors.New("hue: empty bridge address")
	}
	if !strings.Contains(ip, "://") {
		scheme := "http"
		if https {
			scheme = "https"
		}
		ip = scheme + "://" + ip
	}
	u, err := url.Parse(ip)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return "", fmt.Errorf("hue: bridge address must be host or scheme://host, got %q", ip)
	}
	return u.Scheme + "://" + u.Host, nil
}

func newClient(base, key string, timeout time.Duration) *client {
	if timeout <= 0 {
		timeout = HTTPTimeout
	}
	tr := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: timeout}).DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local bridge, self-signed cert, this transport only
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &client{base: base, key: key, http: &http.Client{Timeout: timeout, Transport: tr}}
}

func (c *client) close() {
	if c != nil && c.http != nil {
		c.http.CloseIdleConnections()
	}
}

// do performs one guarded request and returns the body. Errors are raw
// (transport/HTTP/guard); callers classify.
func (c *client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	if err := guardRequest(method, path); err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return data, httpStatusError{Code: resp.StatusCode, Body: firstLine(string(data))}
	}
	return data, nil
}

// parseResultArray splits a v1 result array into successes and errors.
func parseResultArray(data []byte) (successes []map[string]json.RawMessage, errs []hueError, err error) {
	var arr []map[string]json.RawMessage
	if jerr := json.Unmarshal(data, &arr); jerr != nil {
		return nil, nil, fmt.Errorf("hue: unexpected response %q: %w", firstLine(string(data)), jerr)
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

// firstHueError returns the first error of a v1 array response, if the body
// is such an array (an object response yields nil).
func firstHueError(data []byte) error {
	_, errs, perr := parseResultArray(data)
	if perr == nil && len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// ---------------------------------------------------------------------------
// Read operations
// ---------------------------------------------------------------------------

// lightV1 is the subset of GET /lights[/<id>] the driver needs.
type lightV1 struct {
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

func (c *client) lights(ctx context.Context) (map[string]lightV1, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/"+c.key+"/lights", nil)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return nil, herr
		}
		return nil, err
	}
	var m map[string]lightV1
	if jerr := json.Unmarshal(data, &m); jerr != nil {
		if herr := firstHueError(data); herr != nil {
			return nil, herr
		}
		return nil, fmt.Errorf("hue: unexpected /lights response: %w", jerr)
	}
	return m, nil
}

// BridgeInfo is the subset of GET /config used for the status screen.
type BridgeInfo struct {
	BridgeID  string `json:"bridgeid"`
	Name      string `json:"name"`
	ModelID   string `json:"modelid"`
	SWVersion string `json:"swversion"`
	APIVer    string `json:"apiversion"`
}

func (c *client) config(ctx context.Context) (BridgeInfo, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/"+c.key+"/config", nil)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return BridgeInfo{}, herr
		}
		return BridgeInfo{}, err
	}
	var info BridgeInfo
	if jerr := json.Unmarshal(data, &info); jerr != nil {
		if herr := firstHueError(data); herr != nil {
			return BridgeInfo{}, herr
		}
		return BridgeInfo{}, fmt.Errorf("hue: unexpected /config response: %w", jerr)
	}
	// An unauthenticated GET /config still answers a public subset without
	// bridgeid (v1 quirk) — treat that as refused.
	if info.BridgeID == "" {
		return info, hueError{Type: hueErrUnauthorized, Address: "/config", Description: "unauthorized user (public config only)"}
	}
	return info, nil
}

// ---------------------------------------------------------------------------
// The single write
// ---------------------------------------------------------------------------

// stateV1 is the writable subset of /lights/<id>/state.
type stateV1 struct {
	On             *bool       `json:"on,omitempty"`
	Bri            *int        `json:"bri,omitempty"`
	XY             *[2]float64 `json:"xy,omitempty"`
	TransitionTime *int        `json:"transitiontime,omitempty"`
}

// setState writes one light's state. id must already be validated.
func (c *client) setState(ctx context.Context, id string, st stateV1) error {
	if !validLightID(id) {
		return ErrGuard{http.MethodPut, "/api/<key>/lights/" + id + "/state"}
	}
	data, err := c.do(ctx, http.MethodPut, "/api/"+c.key+"/lights/"+id+"/state", st)
	if err != nil {
		if herr := firstHueError(data); herr != nil {
			return herr
		}
		return err
	}
	_, errs, perr := parseResultArray(data)
	if perr != nil {
		return perr
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// ---------------------------------------------------------------------------
// Registration (POST /api) — link-button flow, from the spike
// ---------------------------------------------------------------------------

// ErrLinkButtonNotPressed is the nominal "not yet" of registration (Hue
// error 101). It is a refused-class outcome, not a failure.
var ErrLinkButtonNotPressed = fmt.Errorf("%w: link button not pressed", ErrRefused)

// Register asks the bridge for an API key. It returns ErrLinkButtonNotPressed
// (after wait) while the user has not pressed the button, ErrUnreachable if
// the bridge cannot be reached. Retries every 2 s within wait; wait <= 0
// means a single attempt (the HTTP endpoint of #207 polls itself).
func Register(ctx context.Context, bridgeIP string, https bool, devicetype string, wait time.Duration) (string, error) {
	base, err := bridgeBase(bridgeIP, https)
	if err != nil {
		return "", err
	}
	c := newClient(base, "", HTTPTimeout)
	defer c.close()
	deadline := time.Now().Add(wait)
	for {
		data, err := c.do(ctx, http.MethodPost, "/api", map[string]string{"devicetype": devicetype})
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
			notPressed := false
			for _, e := range errs {
				if e.Type == hueErrLinkButtonPressed {
					notPressed = true
				} else {
					return "", classify(e)
				}
			}
			if !notPressed {
				return "", fmt.Errorf("hue: registration answered without username or error: %s", firstLine(string(data)))
			}
		} else {
			if herr := firstHueError(data); herr != nil {
				err = herr
			}
			cerr := classify(err)
			if !errors.Is(cerr, ErrRefused) {
				return "", cerr
			}
		}
		if wait <= 0 || time.Now().After(deadline) {
			return "", ErrLinkButtonNotPressed
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("%w: %v", ErrUnreachable, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
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
