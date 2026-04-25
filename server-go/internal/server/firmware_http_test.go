package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMultipartBin creates a multipart/form-data body with a single "file" field
// containing the provided binary data and the given filename.
// It returns the body reader and the corresponding Content-Type header value.
func buildMultipartBin(t *testing.T, filename string, data []byte) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("failed to write file data: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	return &buf, mw.FormDataContentType()
}

// TestHandleAPIFirmwareVersion_NoFirmware verifies that GET /api/firmware/buzzclick/version
// returns a JSON payload with exists=false when no firmware has been uploaded yet.
func TestHandleAPIFirmwareVersion_NoFirmware(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/version", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// "EXISTS" must be false when no firmware has been stored
	exists, ok := payload["EXISTS"].(bool)
	if !ok {
		t.Fatalf("expected 'EXISTS' field to be a bool, got %T (%v)", payload["EXISTS"], payload["EXISTS"])
	}
	if exists {
		t.Errorf("expected EXISTS=false when no firmware uploaded, got true")
	}
}

// TestHandleAPIFirmwareVersion_WithFirmware verifies that GET /api/firmware/buzzclick/version
// returns exists=true and the correct version after a firmware has been uploaded.
func TestHandleAPIFirmwareVersion_WithFirmware(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Pre-seed the firmware manager with a valid firmware (300 KB)
	firmwareData := makeFirmwareData(300 * 1024)
	expectedVersion := "3.1.0"
	if err := server.firmwareManager.SaveFirmware(firmwareData, expectedVersion); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/version", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	exists, ok := payload["EXISTS"].(bool)
	if !ok {
		t.Fatalf("expected 'EXISTS' field to be a bool, got %T (%v)", payload["EXISTS"], payload["EXISTS"])
	}
	if !exists {
		t.Errorf("expected EXISTS=true after firmware upload, got false")
	}

	version, ok := payload["VERSION"].(string)
	if !ok {
		t.Fatalf("expected 'VERSION' field to be a string, got %T", payload["VERSION"])
	}
	if version != expectedVersion {
		t.Errorf("expected version %q, got %q", expectedVersion, version)
	}
}

// TestHandleAPIFirmwareDownload_NoFirmware verifies that GET /api/firmware/buzzclick/latest.bin
// returns HTTP 404 when no firmware file is available on the server.
func TestHandleAPIFirmwareDownload_NoFirmware(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/latest.bin", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestHandleAPIFirmwareUpload_Valid verifies that POST /api/firmware/buzzclick/upload
// with a valid .bin file of 300 KB returns HTTP 200 and the version extracted from the filename.
func TestHandleAPIFirmwareUpload_Valid(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	firmwareData := makeFirmwareData(300 * 1024) // 300 KB – above 200 KB minimum
	filename := "buzzclick-v3.1.0.bin"

	body, contentType := buildMultipartBin(t, filename, firmwareData)

	req := httptest.NewRequest(http.MethodPost, "/api/firmware/buzzclick/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Fatalf("expected status 200, got %d: %s", w.Code, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", result["status"])
	}

	// Version must be extracted from filename "buzzclick-v3.1.0.bin" → "3.1.0"
	if result["version"] != "3.1.0" {
		t.Errorf("expected version=3.1.0, got %v", result["version"])
	}

	// Verify firmware is now accessible via the manager
	_, _, _, exists := server.firmwareManager.GetInfo()
	if !exists {
		t.Error("expected firmware to exist after successful upload")
	}
}

// TestHandleAPIFirmwareUpload_TooSmall verifies that POST /api/firmware/buzzclick/upload
// with a file below the minimum size (100 KB < 200 KB) returns HTTP 400.
func TestHandleAPIFirmwareUpload_TooSmall(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	firmwareData := makeFirmwareData(100 * 1024) // 100 KB – below 200 KB minimum
	filename := "buzzclick-v3.1.0.bin"

	body, contentType := buildMultipartBin(t, filename, firmwareData)

	req := httptest.NewRequest(http.MethodPost, "/api/firmware/buzzclick/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for firmware that is too small, got %d", w.Code)
	}
}

// TestHandleAPIFirmwareUpload_WrongExtension verifies that POST /api/firmware/buzzclick/upload
// with a non-.bin file returns HTTP 400.
func TestHandleAPIFirmwareUpload_WrongExtension(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Use a .txt extension – should be rejected regardless of content size
	firmwareData := makeFirmwareData(300 * 1024)
	filename := "buzzclick-v3.1.0.txt"

	body, contentType := buildMultipartBin(t, filename, firmwareData)

	req := httptest.NewRequest(http.MethodPost, "/api/firmware/buzzclick/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for wrong file extension, got %d", w.Code)
	}

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, ".bin") {
		t.Errorf("expected error message to mention .bin extension, got: %s", responseBody)
	}
}

// TestBuildFirmwareURL_Format verifies that buildFirmwareURL returns a well-formed
// HTTP URL pointing to the expected firmware download path.
// Non-regression for issue #50: the URL must always be a reachable HTTP endpoint
// so that firmware < 3.1.2 (which reads URL from the OTA_UPDATE message) can
// fall back to it.
func TestBuildFirmwareURL_Format(t *testing.T) {
	url := buildFirmwareURL()

	if !strings.HasPrefix(url, "http://") {
		t.Errorf("expected URL to start with 'http://', got %q", url)
	}
	if !strings.HasSuffix(url, "/api/firmware/buzzclick/latest.bin") {
		t.Errorf("expected URL to end with '/api/firmware/buzzclick/latest.bin', got %q", url)
	}
	// Must have a non-empty host segment (not just "http:///api/...")
	trimmed := strings.TrimPrefix(url, "http://")
	if strings.HasPrefix(trimmed, "/") {
		t.Errorf("expected a host in the URL, got empty host: %q", url)
	}
}

// TestHandleAPIBuzzerUpdate_NotConnected_JSONError verifies that POST /api/buzzer/{mac}/update
// returns a well-formed JSON error (HTTP 200) when the target buzzer is not connected via
// WebSocket, rather than panicking or returning a malformed response.
// Non-regression for issue #50: the handler must remain stable even when OTA cannot be
// triggered (buzzer offline). The OTA URL is only built when the buzzer IS connected.
func TestHandleAPIBuzzerUpdate_NotConnected_JSONError(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Upload valid firmware (so the handler does not fail on missing firmware).
	firmwareData := makeFirmwareData(300 * 1024)
	if err := server.firmwareManager.SaveFirmware(firmwareData, "3.6.1"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	// Target a MAC that is not connected.
	mac := "AA:BB:CC:DD:EE:FF"
	req := httptest.NewRequest(http.MethodPost, "/api/buzzer/"+mac+"/update", nil)
	req.Host = "localhost"
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Fatalf("expected status 200, got %d: %s", w.Code, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Handler must return status=error with a descriptive message (not a crash or empty body).
	if result["status"] != "error" {
		t.Errorf("expected status=error for disconnected buzzer, got: %v", result["status"])
	}
	msg, hasMsg := result["message"].(string)
	if !hasMsg || msg == "" {
		t.Errorf("expected non-empty 'message' field in error response, got: %v", result)
	}
}

// TestHandleAPIBuzzerUpdateAll_NoBuzzers verifies that POST /api/buzzer/update-all
// returns triggered=0 when no buzzers are registered (even with firmware available).
func TestHandleAPIBuzzerUpdateAll_NoBuzzers(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Upload a valid firmware so that the handler does not short-circuit with "no firmware" error
	firmwareData := makeFirmwareData(300 * 1024)
	if err := server.firmwareManager.SaveFirmware(firmwareData, "3.1.0"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/buzzer/update-all", nil)
	// Set a host so the handler can build the firmware URL
	req.Host = "localhost"
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Fatalf("expected status 200, got %d: %s", w.Code, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// With no buzzers registered, triggered must be 0
	triggered, ok := result["triggered"]
	if !ok {
		t.Fatal("expected 'triggered' field in response")
	}

	// JSON numbers are decoded as float64
	triggeredVal := fmt.Sprintf("%v", triggered)
	if triggeredVal != "0" {
		t.Errorf("expected triggered=0, got %v", triggered)
	}
}

// ---------------------------------------------------------------------------
// Non-regression tests: merged binary — firmware download endpoint (issue #63)
// ---------------------------------------------------------------------------

// TestHandleAPIFirmwareDownload_AppOnly_ServesFullContent verifies that
// GET /api/firmware/buzzclick/latest.bin serves the complete binary unchanged
// when the stored firmware is an app-only binary (no merged binary magic).
// Non-regression: app-only OTA must not be truncated or altered.
func TestHandleAPIFirmwareDownload_AppOnly_ServesFullContent(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	firmwareData := makeFirmwareData(300 * 1024) // 300 KB app-only binary
	if err := server.firmwareManager.SaveFirmware(firmwareData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/latest.bin", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Content-Type must be octet-stream
	ct := w.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/octet-stream")
	}

	// Served content must be identical to the original app-only binary
	body := w.Body.Bytes()
	if len(body) != len(firmwareData) {
		t.Errorf("response body size = %d bytes, want %d (full app-only size)", len(body), len(firmwareData))
	}
	if !bytes.Equal(body, firmwareData) {
		t.Error("response body differs from the uploaded app-only firmware")
	}

	// Content-Length header must match the body length
	cl := w.Header().Get("Content-Length")
	if cl != fmt.Sprintf("%d", len(firmwareData)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(firmwareData))
	}
}

// TestHandleAPIFirmwareDownload_Merged_ServesAppOnly verifies that
// GET /api/firmware/buzzclick/latest.bin serves ONLY the app portion (from offset 0x10000)
// when the stored firmware is a merged binary.
// Non-regression for issue #63: buzzers must receive only the app partition for OTA;
// sending the full merged binary causes the OTA to fail because the buzzer expects an
// app image but receives a bootloader header at offset 0.
func TestHandleAPIFirmwareDownload_Merged_ServesAppOnly(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	appSize := 500 * 1024 // 500 KB app portion
	mergedData := makeMergedBinaryData(appSize)
	if err := server.firmwareManager.SaveFirmware(mergedData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/latest.bin", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.Bytes()

	// The served content must be app-only size, NOT the full merged binary size
	if len(body) == len(mergedData) {
		t.Errorf("response body = full merged binary size (%d bytes) — server is serving merged binary to buzzer instead of app-only portion", len(body))
	}
	if len(body) != appSize {
		t.Errorf("response body size = %d bytes, want %d (app-only size, not merged size %d)", len(body), appSize, len(mergedData))
	}

	// The app portion must match exactly what is at offset 0x10000 in the merged binary
	expectedAppData := mergedData[appPartitionOffset:]
	if !bytes.Equal(body, expectedAppData) {
		t.Error("response body differs from expected app portion of merged binary")
	}

	// Content-Length header must advertise the app-only size (not merged binary size)
	cl := w.Header().Get("Content-Length")
	if cl != fmt.Sprintf("%d", appSize) {
		t.Errorf("Content-Length = %q, want %d (app-only size — merged size would be %d)", cl, appSize, len(mergedData))
	}
}

// TestHandleAPIFirmwareVersion_IsMergedFlag verifies that
// GET /api/firmware/buzzclick/version returns IS_MERGED=true for a merged binary
// and IS_MERGED=false for an app-only binary.
// Non-regression: the frontend uses IS_MERGED to show/hide the USB flash button.
func TestHandleAPIFirmwareVersion_IsMergedFlag(t *testing.T) {
	tests := []struct {
		name       string
		firmware   []byte
		wantMerged bool
	}{
		{
			name:       "app-only binary — IS_MERGED=false",
			firmware:   makeFirmwareData(300 * 1024),
			wantMerged: false,
		},
		{
			name:       "merged binary — IS_MERGED=true",
			firmware:   makeMergedBinaryData(500 * 1024),
			wantMerged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := setupTestHTTPServer(t)
			if err := server.firmwareManager.SaveFirmware(tt.firmware, "3.6.2"); err != nil {
				t.Fatalf("SaveFirmware failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/version", nil)
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}

			isMerged, ok := payload["IS_MERGED"].(bool)
			if !ok {
				t.Fatalf("expected 'IS_MERGED' field to be a bool, got %T (%v)", payload["IS_MERGED"], payload["IS_MERGED"])
			}
			if isMerged != tt.wantMerged {
				t.Errorf("IS_MERGED = %v, want %v", isMerged, tt.wantMerged)
			}
		})
	}
}

// TestHandleAPIFirmwareMergedDownload_AppOnly_Returns404 verifies that
// GET /api/firmware/buzzclick/merged.bin returns 404 when the stored firmware is app-only.
// The merged download endpoint is only available for merged binaries.
func TestHandleAPIFirmwareMergedDownload_AppOnly_Returns404(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Store an app-only binary — merged endpoint must return 404
	firmwareData := makeFirmwareData(300 * 1024)
	if err := server.firmwareManager.SaveFirmware(firmwareData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/merged.bin", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for merged.bin when firmware is app-only, got %d", w.Code)
	}
}

// TestHandleAPIFirmwareMergedDownload_Merged_ServesFullBinary verifies that
// GET /api/firmware/buzzclick/merged.bin serves the full merged binary (all bytes)
// when the stored firmware is a merged binary.
// Non-regression: the USB flash endpoint must serve the complete merged binary
// (not just the app portion) for correct esptool-js flashing from address 0x0.
func TestHandleAPIFirmwareMergedDownload_Merged_ServesFullBinary(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	appSize := 500 * 1024
	mergedData := makeMergedBinaryData(appSize)
	if err := server.firmwareManager.SaveFirmware(mergedData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/firmware/buzzclick/merged.bin", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for merged.bin, got %d", w.Code)
	}

	// The merged download serves via http.ServeFile, so body length should match
	body := w.Body.Bytes()
	if len(body) != len(mergedData) {
		t.Errorf("merged.bin response size = %d, want %d (full merged binary)", len(body), len(mergedData))
	}
}
