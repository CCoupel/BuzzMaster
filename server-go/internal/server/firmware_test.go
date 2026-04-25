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

	// FirmwareMaxSize + 1 byte – strictly above the maximum
	data := makeFirmwareData(FirmwareMaxSize + 1)
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

// makeMergedBinaryData creates a fake merged firmware binary with the correct structure:
// - Magic bytes 0xAA 0x50 at offset partitionTableOffset (0x8000)
// - App data starting at offset appPartitionOffset (0x10000), filled with a repeating pattern
// The total binary size is appPartitionOffset + appSize.
// appSize must be large enough so that the total >= FirmwareMinSize (200KB).
func makeMergedBinaryData(appSize int) []byte {
	total := appPartitionOffset + appSize
	data := make([]byte, total)
	// Partition table magic
	data[partitionTableOffset] = 0xAA
	data[partitionTableOffset+1] = 0x50
	// App payload — distinct pattern from the leading zeros so it's identifiable
	for i := 0; i < appSize; i++ {
		data[appPartitionOffset+i] = byte((i + 1) % 256)
	}
	return data
}

// ---------------------------------------------------------------------------
// Tests for IsMergedBinary (non-regression: merged binary detection)
// ---------------------------------------------------------------------------

func TestIsMergedBinary(t *testing.T) {
	// Partition table magic bytes at offset 0x8000
	appSize := 300 * 1024
	mergedData := makeMergedBinaryData(appSize)

	// App-only binary — no magic at 0x8000
	appOnlyData := makeFirmwareData(300 * 1024)

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "merged binary — magic 0xAA 0x50 at 0x8000",
			data: mergedData,
			want: true,
		},
		{
			name: "app-only binary — no magic at 0x8000",
			data: appOnlyData,
			want: false,
		},
		{
			name: "empty data",
			data: []byte{},
			want: false,
		},
		{
			name: "data shorter than partitionTableOffset+2",
			data: make([]byte, partitionTableOffset),
			want: false,
		},
		{
			name: "data exactly at partitionTableOffset+1 (second magic byte missing)",
			data: func() []byte {
				d := make([]byte, partitionTableOffset+1)
				d[partitionTableOffset] = 0xAA
				return d
			}(),
			want: false,
		},
		{
			name: "first magic byte matches but second does not",
			data: func() []byte {
				d := make([]byte, partitionTableOffset+2)
				d[partitionTableOffset] = 0xAA
				d[partitionTableOffset+1] = 0x51 // wrong
				return d
			}(),
			want: false,
		},
		{
			name: "nil data",
			data: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMergedBinary(tt.data)
			if got != tt.want {
				t.Errorf("IsMergedBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for GetAppFirmware (non-regression: app extraction from merged binary)
// ---------------------------------------------------------------------------

func TestFirmwareManager_GetAppFirmware_AppOnly(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	appData := makeFirmwareData(300 * 1024)
	if err := fm.SaveFirmware(appData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	got, err := fm.GetAppFirmware()
	if err != nil {
		t.Fatalf("GetAppFirmware() unexpected error: %v", err)
	}
	if len(got) != len(appData) {
		t.Errorf("GetAppFirmware() returned %d bytes, want %d (full app-only content)", len(got), len(appData))
	}
	if !bytes.Equal(got, appData) {
		t.Error("GetAppFirmware() content differs from original app-only data")
	}
}

func TestFirmwareManager_GetAppFirmware_Merged(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	appSize := 300 * 1024 // 300 KB app portion
	mergedData := makeMergedBinaryData(appSize)
	if err := fm.SaveFirmware(mergedData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware failed: %v", err)
	}

	got, err := fm.GetAppFirmware()
	if err != nil {
		t.Fatalf("GetAppFirmware() unexpected error: %v", err)
	}

	// Must return only the app portion (from offset 0x10000 onwards)
	wantSize := appSize
	if len(got) != wantSize {
		t.Errorf("GetAppFirmware() returned %d bytes, want %d (app-only size)", len(got), wantSize)
	}

	// The returned data must equal the app portion of the merged binary
	wantAppData := mergedData[appPartitionOffset:]
	if !bytes.Equal(got, wantAppData) {
		t.Error("GetAppFirmware() content differs from expected app portion of merged binary")
	}

	// The returned data must NOT start with the merged binary header (no 0xAA 0x50 at 0x8000)
	if len(got) > partitionTableOffset+1 &&
		got[partitionTableOffset] == 0xAA && got[partitionTableOffset+1] == 0x50 {
		t.Error("GetAppFirmware() returned data still looks like a merged binary (partition table magic found)")
	}
}

func TestFirmwareManager_GetAppFirmware_MergedTooSmall(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	// Manually write a "merged binary" that is smaller than appPartitionOffset (0x10000)
	// — bypassing SaveFirmware size checks.
	firmwareDir := filepath.Join(dir, "firmware")
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		t.Fatalf("failed to create firmware dir: %v", err)
	}
	// Build a small binary that has the partition table magic but no app data
	tinyMerged := make([]byte, partitionTableOffset+2)
	tinyMerged[partitionTableOffset] = 0xAA
	tinyMerged[partitionTableOffset+1] = 0x50
	if err := os.WriteFile(fm.GetFirmwarePath(), tinyMerged, 0644); err != nil {
		t.Fatalf("failed to write tiny merged binary: %v", err)
	}

	_, err := fm.GetAppFirmware()
	if err == nil {
		t.Error("GetAppFirmware() expected error for merged binary with no app data, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests for SaveFirmware with merged binaries (non-regression: 3MB size limit)
// ---------------------------------------------------------------------------

func TestFirmwareManager_SaveFirmware_MergedBinary(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	// Typical merged binary: ~1MB total (bootloader + partitions + app)
	appSize := 500 * 1024
	mergedData := makeMergedBinaryData(appSize)

	if err := fm.SaveFirmware(mergedData, "3.6.2"); err != nil {
		t.Fatalf("SaveFirmware() should accept a ~%dKB merged binary, got error: %v", len(mergedData)/1024, err)
	}

	// IsMerged() must return true after saving
	if !fm.IsMerged() {
		t.Error("IsMerged() = false after saving a merged binary")
	}

	// GetInfo size must reflect the full merged binary size (not app-only)
	_, _, storedSize, exists := fm.GetInfo()
	if !exists {
		t.Fatal("firmware should exist after SaveFirmware")
	}
	if storedSize != int64(len(mergedData)) {
		t.Errorf("GetInfo() size = %d, want %d (full merged binary size)", storedSize, len(mergedData))
	}
}

func TestFirmwareManager_SaveFirmware_MergedBinaryAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	// FirmwareMaxSize is exactly 3MB — must be accepted
	data := makeFirmwareData(FirmwareMaxSize)
	// Inject partition table magic to simulate a merged binary of max size
	data[partitionTableOffset] = 0xAA
	data[partitionTableOffset+1] = 0x50

	if err := fm.SaveFirmware(data, "3.6.2"); err != nil {
		t.Errorf("SaveFirmware() should accept firmware at exactly FirmwareMaxSize (%d bytes), got: %v", FirmwareMaxSize, err)
	}
}

func TestFirmwareManager_SaveFirmware_MergedBinaryExceedsMaxSize(t *testing.T) {
	dir := t.TempDir()
	fm := NewFirmwareManager(dir, "3.6.2")

	// FirmwareMaxSize + 1 byte must be rejected even for merged binaries
	data := makeFirmwareData(FirmwareMaxSize + 1)
	data[partitionTableOffset] = 0xAA
	data[partitionTableOffset+1] = 0x50

	err := fm.SaveFirmware(data, "3.6.2")
	if err == nil {
		t.Error("SaveFirmware() should reject firmware larger than FirmwareMaxSize (3MB), got nil error")
	}
}
