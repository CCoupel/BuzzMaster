package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeFirmwareData generates a byte slice of the given size filled with a repeated pattern.
func makeFirmwareData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func TestFirmwareManager_IsOutdated(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	// Save a firmware with version 3.1.0
	data := makeFirmwareData(300 * 1024) // 300 KB
	if err := fm.SaveFirmware(data, "3.1.0"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	// Buzzer with version 3.0.8 should be outdated
	if !fm.IsOutdated("3.0.8") {
		t.Error("Expected 3.0.8 to be outdated compared to server 3.1.0")
	}
}

func TestFirmwareManager_UpToDate(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	data := makeFirmwareData(300 * 1024)
	if err := fm.SaveFirmware(data, "3.1.0"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	// Buzzer with same version should NOT be outdated
	if fm.IsOutdated("3.1.0") {
		t.Error("Expected 3.1.0 to NOT be outdated compared to server 3.1.0")
	}
}

func TestFirmwareManager_EmptyVersion(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	data := makeFirmwareData(300 * 1024)
	if err := fm.SaveFirmware(data, "3.1.0"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	// Buzzer with empty version (old firmware not reporting version) → not outdated
	if fm.IsOutdated("") {
		t.Error("Expected empty version to NOT be outdated (backward compat)")
	}
}

func TestFirmwareManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	originalData := makeFirmwareData(300 * 1024) // 300 KB
	version := "3.1.0"

	if err := fm.SaveFirmware(originalData, version); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	// GetInfo should return correct version and size
	gotVersion, gotFilename, gotSize, gotExists := fm.GetInfo()
	if !gotExists {
		t.Fatal("Expected firmware to exist after saving")
	}
	if gotVersion != version {
		t.Errorf("Version = %q, want %q", gotVersion, version)
	}
	expectedFilename := "buzzclick-v3.1.0.bin"
	if gotFilename != expectedFilename {
		t.Errorf("Filename = %q, want %q", gotFilename, expectedFilename)
	}
	if gotSize != int64(len(originalData)) {
		t.Errorf("Size = %d, want %d", gotSize, len(originalData))
	}

	// Verify the latest.bin content matches what was saved
	latestPath := fm.GetFirmwarePath()
	savedData, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("Failed to read saved firmware: %v", err)
	}
	if !bytes.Equal(savedData, originalData) {
		t.Error("Saved firmware content does not match original data")
	}

	// Verify versioned file also exists
	versionedPath := filepath.Join(dir, "firmware", "buzzclick-v3.1.0.bin")
	if _, err := os.Stat(versionedPath); err != nil {
		t.Errorf("Versioned firmware file not found: %v", err)
	}
}

func TestFirmwareManager_TooSmall(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	// 100 KB – below the 200 KB minimum
	data := makeFirmwareData(100 * 1024)
	err := fm.SaveFirmware(data, "3.1.0")
	if err == nil {
		t.Error("Expected error for firmware that is too small, got nil")
	}
}

func TestFirmwareManager_TooLarge(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	// 3 MB – above the 2 MB maximum
	data := makeFirmwareData(3 * 1024 * 1024)
	err := fm.SaveFirmware(data, "3.1.0")
	if err == nil {
		t.Error("Expected error for firmware that is too large, got nil")
	}
}

func TestFirmwareManager_NoFirmware(t *testing.T) {
	dir := t.TempDir()
	// Use empty serverVersion to simulate a server with no reference version at all.
	fm := NewFirmwareManager(dir, "")

	// No firmware stored yet
	_, _, _, exists := fm.GetInfo()
	if exists {
		t.Error("Expected exists=false when no firmware has been saved")
	}

	// IsOutdated with no firmware and no server version should return false
	if fm.IsOutdated("3.0.8") {
		t.Error("Expected IsOutdated=false when no firmware is stored on the server and no server version is set")
	}
}

// TestFirmwareManager_PathTraversal verifies that SaveFirmware sanitizes the version string
// to prevent path traversal attacks. A malicious version like "../../../etc/passwd"
// must NOT create any file outside the firmware directory.
func TestFirmwareManager_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.1.0")

	data := makeFirmwareData(300 * 1024) // 300 KB – valid size

	maliciousVersions := []string{
		"../../../etc/passwd",
		"../../tmp/evil",
		"../secret",
		"3.1.0/../../etc/passwd",
		"3.1.0\x00evil",
		"3.1.0; rm -rf /",
	}

	for _, maliciousVersion := range maliciousVersions {
		err := fm.SaveFirmware(data, maliciousVersion)
		// SaveFirmware must succeed (falls back to "unknown"), not return an error
		if err != nil {
			t.Errorf("SaveFirmware(%q) unexpected error: %v", maliciousVersion, err)
			continue
		}

		// The versioned file must be inside the firmware directory only
		firmwareDir := fm.firmwareDir()
		// Check that no file was written outside the firmware directory
		// by verifying that only buzzclick-vunknown.bin and buzzclick-latest.bin exist
		entries, readErr := os.ReadDir(firmwareDir)
		if readErr != nil {
			t.Fatalf("failed to read firmware dir: %v", readErr)
		}
		for _, entry := range entries {
			name := entry.Name()
			if name != "buzzclick-vunknown.bin" && name != "buzzclick-latest.bin" && name != "version.txt" {
				t.Errorf("SaveFirmware(%q) created unexpected file: %s", maliciousVersion, name)
			}
		}
	}
}

// TestFirmwareManager_ValidVersionFormats verifies that SaveFirmware accepts valid version strings.
func TestFirmwareManager_ValidVersionFormats(t *testing.T) {
	validVersions := []string{
		"3.1.0",
		"3.1",
		"10.20.30",
		"1.0",
	}

	for _, version := range validVersions {
		dir := t.TempDir()
		fm := NewFirmwareManager(dir, "3.1.0")
		data := makeFirmwareData(300 * 1024)

		if err := fm.SaveFirmware(data, version); err != nil {
			t.Errorf("SaveFirmware(%q) unexpected error for valid version: %v", version, err)
		}

		gotVersion, _, _, exists := fm.GetInfo()
		if !exists {
			t.Errorf("SaveFirmware(%q) firmware should exist after save", version)
		}
		if gotVersion != version {
			t.Errorf("SaveFirmware(%q) stored version = %q, want %q", version, gotVersion, version)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"3.0.8", "3.1.0", -1},
		{"3.1.0", "3.1.0", 0},
		{"3.1.1", "3.1.0", 1},
		{"2.99.9", "3.0.0", -1},
		{"3.0.0", "2.99.9", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.1", "1.0", 1},
	}

	for _, tt := range tests {
		got := compareSemver(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
