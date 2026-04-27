package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

// getLocalIP returns the first non-loopback IPv4 address of the host.
// Falls back to "127.0.0.1" if no suitable interface is found.
func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

// buildFirmwareURL constructs the OTA download URL using the real server IP.
// This is provided for backward compatibility with firmware < 3.1.2 which reads
// the URL from the OTA_UPDATE message. Newer firmware ignores this field.
func buildFirmwareURL() string {
	return fmt.Sprintf("http://%s/api/firmware/buzzclick/latest.bin", getLocalIP())
}

// handleAPIFirmwareVersion handles GET /api/firmware/buzzclick/version.
// It returns information about the currently stored BuzzClick firmware.
func (h *HTTPServer) handleAPIFirmwareVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version, filename, size, exists := h.firmwareManager.GetInfo()
	payload := protocol.FirmwareVersionPayload{
		Version:         version,
		Filename:        filename,
		Size:            size,
		Exists:          exists,
		IsMerged:        h.firmwareManager.IsMerged(),
		EmbeddedVersion: h.firmwareManager.GetEmbeddedVersion(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// handleAPIFirmwareDownload handles GET /api/firmware/buzzclick/latest.bin.
// It serves the app-only firmware binary for OTA download by buzzers.
// When the stored binary is a merged binary (bootloader + partitions + app),
// it automatically extracts and serves only the app portion (from offset 0x10000).
// App-only binaries are served as-is for backward compatibility.
func (h *HTTPServer) handleAPIFirmwareDownload(w http.ResponseWriter, r *http.Request) {
	_, _, _, exists := h.firmwareManager.GetInfo()
	if !exists {
		http.Error(w, `{"status":"error","message":"No firmware file available"}`, http.StatusNotFound)
		return
	}

	appData, err := h.firmwareManager.GetAppFirmware()
	if err != nil {
		http.Error(w, `{"status":"error","message":"Failed to read firmware"}`, http.StatusInternalServerError)
		return
	}

	fwVersion, _, _, _ := h.firmwareManager.GetInfo()
	downloadName := "buzzclick-latest.bin"
	if fwVersion != "" && fwVersion != "unknown" {
		downloadName = fmt.Sprintf("buzzclick-v%s.bin", fwVersion)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(appData)))
	w.Write(appData) //nolint:errcheck
}

// handleAPIFirmwareMergedDownload handles GET /api/firmware/buzzclick/merged.bin.
// It serves the full merged firmware binary (bootloader + partitions + boot_app0 + app)
// for a USB flash of new or dead BuzzClick devices (same approach as WLED web installer).
// Only available when the stored binary is a merged binary — returns 404 for app-only binaries.
// The merged binary is written to address 0x0 by esptool-js in a single continuous write,
// which avoids the silent write failures that occur when starting at an app partition address.
func (h *HTTPServer) handleAPIFirmwareMergedDownload(w http.ResponseWriter, r *http.Request) {
	_, _, _, exists := h.firmwareManager.GetInfo()
	if !exists || !h.firmwareManager.IsMerged() {
		http.Error(w, `{"status":"error","message":"No merged firmware available — upload a merged binary to enable USB flash"}`, http.StatusNotFound)
		return
	}

	fwVersion, _, size, _ := h.firmwareManager.GetInfo()
	mergedName := "buzzclick-merged.bin"
	if fwVersion != "" && fwVersion != "unknown" {
		mergedName = fmt.Sprintf("buzzclick-v%s-merged.bin", fwVersion)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, mergedName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	http.ServeFile(w, r, h.firmwareManager.GetFirmwarePath())
}

// handleAPIFirmwareUpload handles POST /api/firmware/buzzclick/upload.
// It accepts a multipart form with a "file" field containing the firmware .bin.
// After a successful upload it broadcasts FIRMWARE_VERSION to all web clients.
func (h *HTTPServer) handleAPIFirmwareUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 4MB to accommodate up to 2MB firmware + overhead)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing 'file' field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".bin" {
		http.Error(w, "Invalid file extension: only .bin files are accepted", http.StatusBadRequest)
		return
	}

	// Read firmware data
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read firmware data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract version from query parameter or form value (optional)
	version := r.FormValue("version")
	if version == "" {
		// Try to derive version from filename (e.g. buzzclick-v3.1.0.bin or buzzclick-v3.0.4-firmware.bin → 3.0.4)
		name := header.Filename
		name = strings.TrimSuffix(name, ".bin")
		if idx := strings.LastIndex(name, "-v"); idx >= 0 {
			candidate := name[idx+2:]
			// Strip any trailing suffix after the version (e.g. "-firmware" in "3.0.4-firmware")
			if end := strings.IndexAny(candidate, "-_"); end >= 0 {
				candidate = candidate[:end]
			}
			version = candidate
		}
	}
	if version == "" {
		version = "unknown"
	}

	// Save firmware via manager (validates size bounds)
	if err := h.firmwareManager.SaveFirmware(data, version); err != nil {
		LogError(game.LogComponentHTTP, "Firmware upload failed: %v", err)
		http.Error(w, "Failed to save firmware: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Recalculate IS_OUTDATED for physical WebSocket buzzers (those with FIRMWARE_VERSION set).
	// Only bumpers that sent firmware_version in their HELLO payload are physical buzzers;
	// TCP-only and simulated demo bumpers only have VERSION and must not be marked outdated.
	allData := h.engine.GetTeamsAndBumpers()
	for mac, bumper := range allData.Bumpers {
		if bumper.FirmwareVersion == "" {
			continue
		}
		newOutdated := h.firmwareManager.IsOutdated(bumper.FirmwareVersion)
		if newOutdated != bumper.IsOutdated {
			h.engine.UpdateBumper(mac, map[string]interface{}{"IS_OUTDATED": newOutdated})
		}
	}

	// Broadcast full state UPDATE so web clients see updated IS_OUTDATED flags immediately.
	if stateMsg, err := protocol.NewMessage(protocol.ActionUpdate, nil); err == nil {
		stateMsg.Msg = h.engine.GetGameJSON()
		h.wsHub.Broadcast(stateMsg)
	}

	// Broadcast FIRMWARE_VERSION to all web clients so the UI can refresh
	versionInfo, filename, size, exists := h.firmwareManager.GetInfo()
	LogInfo(game.LogComponentHTTP, "BuzzClick firmware uploaded: version=%s, size=%d bytes", versionInfo, len(data))
	payload := protocol.FirmwareVersionPayload{
		Version:         versionInfo,
		Filename:        filename,
		Size:            size,
		Exists:          exists,
		IsMerged:        h.firmwareManager.IsMerged(),
		EmbeddedVersion: h.firmwareManager.GetEmbeddedVersion(),
	}
	if msg, err := protocol.NewMessage(protocol.ActionFirmwareVersion, payload); err == nil {
		h.wsHub.Broadcast(msg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"version":   versionInfo,
		"size":      len(data),
		"filename":  filename,
		"is_merged": h.firmwareManager.IsMerged(),
	})
}

// handleAPIFirmwareRestoreEmbedded handles POST /api/firmware/buzzclick/restore-embedded.
// It restores the firmware embedded in the server binary, overwriting any previously uploaded firmware.
// Returns 404 if no embedded firmware is available in this build.
func (h *HTTPServer) handleAPIFirmwareRestoreEmbedded(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.firmwareManager.GetEmbeddedVersion() == "" {
		http.Error(w, `{"status":"error","message":"No embedded firmware available in this build"}`, http.StatusNotFound)
		return
	}

	if err := h.firmwareManager.RestoreEmbedded(); err != nil {
		LogError(game.LogComponentHTTP, "Firmware restore-embedded failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Recalculate IS_OUTDATED for all physical WebSocket buzzers.
	allData := h.engine.GetTeamsAndBumpers()
	for mac, bumper := range allData.Bumpers {
		if bumper.FirmwareVersion == "" {
			continue
		}
		newOutdated := h.firmwareManager.IsOutdated(bumper.FirmwareVersion)
		if newOutdated != bumper.IsOutdated {
			h.engine.UpdateBumper(mac, map[string]interface{}{"IS_OUTDATED": newOutdated})
		}
	}

	// Broadcast full state UPDATE so web clients see updated IS_OUTDATED flags immediately.
	if stateMsg, err := protocol.NewMessage(protocol.ActionUpdate, nil); err == nil {
		stateMsg.Msg = h.engine.GetGameJSON()
		h.wsHub.Broadcast(stateMsg)
	}

	// Broadcast FIRMWARE_VERSION to all web clients so the UI can refresh.
	versionInfo, filename, size, exists := h.firmwareManager.GetInfo()
	LogInfo(game.LogComponentHTTP, "BuzzClick firmware restored from embedded: version=%s, size=%d bytes", versionInfo, size)
	fwPayload := protocol.FirmwareVersionPayload{
		Version:         versionInfo,
		Filename:        filename,
		Size:            size,
		Exists:          exists,
		IsMerged:        h.firmwareManager.IsMerged(),
		EmbeddedVersion: h.firmwareManager.GetEmbeddedVersion(),
	}
	if msg, err := protocol.NewMessage(protocol.ActionFirmwareVersion, fwPayload); err == nil {
		h.wsHub.Broadcast(msg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"version":  versionInfo,
		"size":     size,
		"filename": filename,
	})
}

// handleAPIBuzzerUpdate handles POST /api/buzzer/{mac}/update.
// It sends an OTA_UPDATE message to the specified buzzer via WebSocket.
// If the buzzer is not connected via WebSocket, it returns a JSON error (HTTP 200).
func (h *HTTPServer) handleAPIBuzzerUpdate(mac string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if buzzer is connected via WebSocket
	if !h.buzzerHub.IsClientConnected(mac) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Buzzer not connected via WebSocket",
		})
		return
	}

	// Get firmware info
	version, _, _, exists := h.firmwareManager.GetInfo()
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "No firmware available on server",
		})
		return
	}

	// Advertise the app-only size (what the buzzer actually downloads from /latest.bin).
	// When storage holds a merged binary, GetInfo() returns the merged size (~1MB),
	// but the server only serves the app portion (~500KB) — broadcasting the merged
	// size would make the buzzer expect more bytes than it receives and abort the OTA.
	appData, err := h.firmwareManager.GetAppFirmware()
	if err != nil {
		http.Error(w, "Failed to read firmware: "+err.Error(), http.StatusInternalServerError)
		return
	}
	size := int64(len(appData))

	// Build OTA payload with firmware URL.
	// URL is included for backward compatibility: firmware < 3.1.2 reads the URL from the message.
	// Firmware >= 3.1.2 ignores the URL field and constructs it from its stored server IP.
	firmwareURL := buildFirmwareURL()
	otaPayload := protocol.OTAUpdatePayload{
		Version: version,
		Size:    size,
		URL:     firmwareURL,
	}

	msg, err := protocol.NewMessage(protocol.ActionOTAUpdate, otaPayload)
	if err != nil {
		http.Error(w, "Failed to build OTA message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.buzzerHub.SendToClient(mac, msg); err != nil {
		http.Error(w, "Failed to send OTA message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	LogInfo(game.LogComponentHTTP, "OTA update triggered for buzzer %s: version=%s url=%s", mac, version, firmwareURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"mac":     mac,
		"version": version,
	})
}

// handleAPIBuzzerUpdateAll handles POST /api/buzzer/update-all.
// It sends OTA_UPDATE to all buzzers that are marked as IsOutdated and connected via WebSocket.
func (h *HTTPServer) handleAPIBuzzerUpdateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get firmware info
	version, _, _, exists := h.firmwareManager.GetInfo()
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "No firmware available on server",
		})
		return
	}

	// Advertise the app-only size (same rationale as handleAPIBuzzerUpdate).
	appData, err := h.firmwareManager.GetAppFirmware()
	if err != nil {
		http.Error(w, "Failed to read firmware: "+err.Error(), http.StatusInternalServerError)
		return
	}
	size := int64(len(appData))

	// Build OTA payload with firmware URL (backward compat with firmware < 3.1.2).
	firmwareURL := buildFirmwareURL()
	otaPayload := protocol.OTAUpdatePayload{
		Version: version,
		Size:    size,
		URL:     firmwareURL,
	}

	msg, err := protocol.NewMessage(protocol.ActionOTAUpdate, otaPayload)
	if err != nil {
		http.Error(w, "Failed to build OTA message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Iterate over all bumpers to find outdated ones
	data := h.engine.GetTeamsAndBumpers()
	triggered := 0
	skipped := 0

	for mac, bumper := range data.Bumpers {
		// Only target physical WebSocket buzzers (FirmwareVersion set)
		if bumper.FirmwareVersion == "" {
			skipped++
			continue
		}
		if !bumper.IsOutdated {
			skipped++
			continue
		}
		if !h.buzzerHub.IsClientConnected(mac) {
			skipped++
			continue
		}
		if err := h.buzzerHub.SendToClient(mac, msg); err != nil {
			LogError(game.LogComponentHTTP, "Failed to send OTA to buzzer %s: %v", mac, err)
			skipped++
			continue
		}
		LogInfo(game.LogComponentHTTP, "OTA update triggered for outdated buzzer %s: version=%s", mac, version)
		triggered++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"triggered": triggered,
		"skipped":   skipped,
	})
}
