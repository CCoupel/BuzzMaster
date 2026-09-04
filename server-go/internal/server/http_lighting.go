package server

// Ambiance lighting endpoints (v10.0.0, #207 — contracts/hue-bridge.md §7).
//
// The bridge itself is driven by internal/lighting/hue; the App owns the
// driver and exposes it through the LightingProvider hook (nil = lighting
// disabled). Every error answer carries the §5.6 taxonomy — "refused" and
// "unreachable" are never merged into one "error".

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting/hue"
)

// LightingProvider is implemented by the App (cmd/server/ambiance.go).
type LightingProvider interface {
	// LightingDriver returns the live driver, or nil when lighting is disabled.
	LightingDriver() *hue.Driver
}

const (
	lightingDiscoverTimeout = 5 * time.Second
	lightingRequestTimeout  = 6 * time.Second
	lightingTestFlashHold   = 400 * time.Millisecond
)

func (h *HTTPServer) lightingDriver() *hue.Driver {
	if h.Lighting == nil {
		return nil
	}
	return h.Lighting.LightingDriver()
}

func writeLightingJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeLightingError maps a driver/client error to the §5.6 taxonomy.
func writeLightingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, hue.ErrLinkButtonNotPressed):
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "link_button_not_pressed"})
	case errors.Is(err, hue.ErrRefused):
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "api_key_refused"})
	case errors.Is(err, hue.ErrUnreachable):
		writeLightingJSON(w, http.StatusServiceUnavailable, map[string]string{"result": "unreachable"})
	default:
		// Security (SSRF audit): the message may carry text from the target
		// (bridge body, Hue error description) — log it, never echo it.
		LogWarn(game.LogComponentHTTP, "Lighting: %v", err)
		writeLightingJSON(w, http.StatusInternalServerError, map[string]string{"result": "error"})
	}
}

// handleLightingStatus — GET /api/lighting/status. Never blocks: it reads the
// driver's known state (contract §7), so the frontend may poll it every 30 s.
func (h *HTTPServer) handleLightingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lc := config.Get().Lighting
	d := h.lightingDriver()
	if d == nil {
		writeLightingJSON(w, http.StatusOK, map[string]any{
			"state": string(hue.StateDisabled), "bridge_id": lc.BridgeID, "bridge_ip": lc.BridgeIP,
			"lights_ok": 0, "lights_total": len(lc.Lights), "enabled": lc.Enabled,
		})
		return
	}
	st := d.Status()
	writeLightingJSON(w, http.StatusOK, map[string]any{
		"state": string(st.State), "reason": st.Reason,
		"bridge_id": st.BridgeID, "bridge_ip": st.BridgeIP, "bridge_model": st.Bridge.ModelID,
		"lights_ok": st.LightsOK, "lights_total": st.LightsTotal, "lights": st.Lights,
		"last_change": st.LastChange, "enabled": lc.Enabled,
	})
}

// handleLightingDiscover — POST /api/lighting/discover: mDNS then SSDP, no cloud.
func (h *HTTPServer) handleLightingDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	release, ok := lightingBusy.acquire(&lightingBusy.discover)
	if !ok {
		writeLightingJSON(w, http.StatusTooManyRequests, map[string]string{"result": "busy", "reason": "discover_in_progress"})
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), lightingDiscoverTimeout+time.Second)
	defer cancel()
	bridges, err := hue.Discover(ctx, lightingDiscoverTimeout/2)
	if err != nil {
		LogWarn(game.LogComponentHTTP, "Lighting discover: %v", err)
	}
	out := make([]map[string]string, 0, len(bridges))
	for _, b := range bridges {
		out = append(out, map[string]string{"ip": b.IP, "id": b.ID, "model": b.Model})
	}
	writeLightingJSON(w, http.StatusOK, map[string]any{"bridges": out})
}

// handleLightingRegister — POST /api/lighting/register {"bridge_ip"}: one
// registration attempt; 409 link_button_not_pressed is the NOMINAL "not yet"
// (the client polls), 200 once the key is obtained and stored server-side —
// the key is never returned to the client.
func (h *HTTPServer) handleLightingRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req struct {
		BridgeIP string `json:"bridge_ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.BridgeIP) == "" {
		http.Error(w, "bridge_ip is required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), lightingRequestTimeout)
	defer cancel()
	// Security (SSRF): the target must be a private-network address BEFORE any
	// outbound request — this endpoint must not become a LAN/Internet prober.
	bridgeAddr, err := validateBridgeAddress(ctx, req.BridgeIP)
	if err != nil {
		LogWarn(game.LogComponentHTTP, "Lighting register refused: %v", err)
		writeLightingJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "reason": "bridge_ip_not_private"})
		return
	}
	release, ok := lightingBusy.acquire(&lightingBusy.register)
	if !ok {
		writeLightingJSON(w, http.StatusTooManyRequests, map[string]string{"result": "busy", "reason": "register_in_progress"})
		return
	}
	defer release()
	key, err := hue.Register(ctx, bridgeAddr, false, lightingDevicetype(), 0)
	if err != nil {
		writeLightingError(w, err)
		return
	}
	// A key handed back by the target is untrusted input: same rule as
	// POST /config.json, applied before anything is persisted.
	if !validHueAPIKey(key) {
		LogWarn(game.LogComponentHTTP, "Lighting register: bridge %s returned a malformed key (%d chars) — not stored", bridgeAddr, len(key))
		writeLightingJSON(w, http.StatusBadGateway, map[string]string{"result": "error", "reason": "invalid_key_from_bridge"})
		return
	}

	// Store the secret (never echoed), plus the bridge identity when readable.
	cfg := *config.Get()
	cfg.Lighting.APIKey = key
	cfg.Lighting.BridgeIP = bridgeAddr
	if info, ierr := hue.BridgeIdentity(ctx, cfg.Lighting.BridgeIP, false, key); ierr == nil && info.BridgeID != "" {
		cfg.Lighting.BridgeID = strings.ToLower(info.BridgeID)
	}
	cfg.Lighting.APIKeyConfigured = false
	cfg.Lighting.ClearAPIKey = false
	config.ApplyDefaults(&cfg)
	if err := config.Save(&cfg); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}
	config.SetInstance(&cfg)
	LogInfo(game.LogComponentHTTP, "Lighting: bridge %s registered (id %s), API key stored", cfg.Lighting.BridgeIP, cfg.Lighting.BridgeID)
	if h.OnConfigUpdate != nil {
		h.OnConfigUpdate() // the App rebuilds its driver from the new config
	}
	if d := h.lightingDriver(); d != nil {
		_ = d.RefreshInventory(ctx) // status/lights are fresh right after registering
	}
	writeLightingJSON(w, http.StatusOK, map[string]string{"result": "ok", "bridge_id": cfg.Lighting.BridgeID})
}

// lightingDevicetype is "buzzmaster#<host>", within Hue's length limits.
func lightingDevicetype() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "server"
	}
	host = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, host)
	if len(host) > 19 {
		host = host[:19]
	}
	return "buzzmaster#" + host
}

// handleLightingLights — GET /api/lighting/lights: the bridge inventory for
// the selection screen. Works as soon as a key is stored, even before the
// module is enabled (a temporary, goroutine-free driver is used then).
func (h *HTTPServer) handleLightingLights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), lightingRequestTimeout)
	defer cancel()
	d := h.lightingDriver()
	if d == nil {
		lc := config.Get().Lighting
		if !lc.EffectiveAPIKeyConfigured() || (lc.BridgeIP == "" && lc.BridgeID == "") {
			writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "not_registered"})
			return
		}
		tmp, err := hue.New(hue.Config{BridgeIP: lc.BridgeIP, BridgeID: lc.BridgeID, APIKey: lc.EffectiveAPIKey()})
		if err != nil {
			writeLightingError(w, err)
			return
		}
		defer tmp.Close()
		d = tmp
	}
	lights, err := d.Inventory(ctx)
	if err != nil {
		writeLightingError(w, err)
		return
	}
	writeLightingJSON(w, http.StatusOK, map[string]any{"lights": lights})
}

// handleLightingTest — POST /api/lighting/test {"name"} (or {} for every
// configured light): brief white flash, then the previous state is restored.
func (h *HTTPServer) handleLightingTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // empty body = all configured lights
	d := h.lightingDriver()
	if d == nil {
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "not_configured"})
		return
	}
	release, ok := lightingBusy.acquire(&lightingBusy.test)
	if !ok {
		writeLightingJSON(w, http.StatusTooManyRequests, map[string]string{"result": "busy", "reason": "test_in_progress"})
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), lightingRequestTimeout)
	defer cancel()
	if err := d.TestFlash(ctx, strings.TrimSpace(req.Name), lightingTestFlashHold, nil); err != nil {
		writeLightingError(w, err)
		return
	}
	writeLightingJSON(w, http.StatusOK, map[string]string{"result": "ok"})
}
