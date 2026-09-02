package main

// WiFi probe for the 2.4 GHz coexistence protocol. Uses the OS `ping` binary
// so that no raw-socket privilege is needed on either Windows or Linux.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Matches "time=12.3 ms" (Linux), "time=12ms" / "time<1ms" (Windows EN),
// "temps=12 ms" / "temps<1ms" (Windows FR) and a few other locales.
var pingRTTRe = regexp.MustCompile(`(?i)(?:time|temps|tiempo|zeit|tempo)\s*([=<])\s*([0-9]+(?:[.,][0-9]+)?)\s*ms`)

// parsePingOutput extracts the RTT in ms from a single-echo ping output.
// "<1ms" is reported as 0.5 ms.
func parsePingOutput(out string) (float64, bool) {
	m := pingRTTRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", "."), 64)
	if err != nil {
		return 0, false
	}
	if m[1] == "<" {
		return v / 2, true
	}
	return v, true
}

func pingArgs(goos, host string, timeout time.Duration) []string {
	if goos == "windows" {
		return []string{"-n", "1", "-w", strconv.Itoa(int(timeout / time.Millisecond)), host}
	}
	secs := int(math.Ceil(timeout.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return []string{"-c", "1", "-W", strconv.Itoa(secs), host}
}

// pingOnce sends one echo request. ok=false means lost/unreachable/unparsable.
func pingOnce(ctx context.Context, host string, timeout time.Duration) (rttMs float64, ok bool, raw string) {
	cctx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "ping", pingArgs(runtime.GOOS, host, timeout)...)
	out, err := cmd.CombinedOutput()
	raw = strings.TrimSpace(string(out))
	if err != nil && len(out) == 0 {
		return 0, false, err.Error()
	}
	rtt, parsed := parsePingOutput(raw)
	if !parsed {
		return 0, false, raw
	}
	// Windows returns exit 0 even for "Destination host unreachable" but then
	// there is no time= field, which the regexp already rejects.
	return rtt, true, raw
}

// Buzzer mirrors the fields of GET /api/buzzers on the BuzzControl server.
type Buzzer struct {
	MAC      string `json:"mac"`
	Name     string `json:"name"`
	Team     string `json:"team"`
	Protocol string `json:"protocol"`
	IP       string `json:"ip"`
	Version  string `json:"version"`
	Status   string `json:"status"`
}

// fetchBuzzers lists buzzers known by the server. Only entries with an IP are
// returned; the caller decides how to treat disconnected ones.
func fetchBuzzers(serverURL string) ([]Buzzer, error) {
	u := strings.TrimRight(serverURL, "/") + "/api/buzzers"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	var all []Buzzer
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decode %s: %w", u, err)
	}
	var out []Buzzer
	for _, b := range all {
		if strings.TrimSpace(b.IP) != "" {
			out = append(out, b)
		}
	}
	return out, nil
}
