package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetBackupPath(t *testing.T) {
	tests := []struct {
		exePath  string
		expected string
	}{
		{"/usr/bin/buzzcontrol", "/usr/bin/buzzcontrol.old"},
		{"C:\\Program Files\\buzzcontrol.exe", "C:\\Program Files\\buzzcontrol.exe.bak"},
	}

	for _, tt := range tests {
		t.Run(tt.exePath, func(t *testing.T) {
			result := getBackupPath(tt.exePath)

			// Check suffix based on platform
			if runtime.GOOS == "windows" {
				if filepath.Ext(result) != ".bak" {
					t.Errorf("Windows backup should end with .bak, got %s", result)
				}
			} else {
				if filepath.Ext(result) != ".old" {
					t.Errorf("Unix backup should end with .old, got %s", result)
				}
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp directory
	tmpDir := os.TempDir()
	src := filepath.Join(tmpDir, "test_src.txt")
	dst := filepath.Join(tmpDir, "test_dst.txt")

	// Cleanup
	defer os.Remove(src)
	defer os.Remove(dst)

	// Create source file
	content := []byte("test content for copy")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Copy file
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content matches
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: got %s, want %s", string(dstContent), string(content))
	}
}

func TestUpdater_CalculateChecksum(t *testing.T) {
	updater := NewUpdater("2.50.0")

	// Create temp file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "test_checksum.txt")
	defer os.Remove(testFile)

	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate checksum
	checksum, err := updater.calculateChecksum(testFile)
	if err != nil {
		t.Fatalf("calculateChecksum failed: %v", err)
	}

	// Checksum should be non-empty and hex string
	if checksum == "" {
		t.Error("Checksum should not be empty")
	}

	// SHA256 hex string should be 64 characters
	if len(checksum) != 64 {
		t.Errorf("SHA256 checksum should be 64 characters, got %d", len(checksum))
	}

	// Known checksum for "hello world"
	expectedChecksum := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if checksum != expectedChecksum {
		t.Errorf("Checksum mismatch: got %s, want %s", checksum, expectedChecksum)
	}
}

func TestNewUpdater(t *testing.T) {
	version := "2.50.0"
	updater := NewUpdater(version)

	if updater == nil {
		t.Fatal("NewUpdater returned nil")
	}

	if updater.currentVer != version {
		t.Errorf("currentVer = %s, want %s", updater.currentVer, version)
	}

	if updater.githubClient == nil {
		t.Error("githubClient should not be nil")
	}

	if updater.tempDir == "" {
		t.Error("tempDir should not be empty")
	}

	// Verify temp directory was created
	if _, err := os.Stat(updater.tempDir); os.IsNotExist(err) {
		t.Errorf("Temp directory was not created: %s", updater.tempDir)
	}
}

func TestMinBinarySize(t *testing.T) {
	// BuzzControl binaries are ~8-9MB on Windows, ~8MB on Linux ARM64.
	// MinBinarySize must be below actual binary sizes to avoid false rejections.
	const actualBinarySize = 8 * 1024 * 1024 // 8MB conservative lower bound

	if MinBinarySize >= actualBinarySize {
		t.Errorf("MinBinarySize (%d bytes) is >= actual binary size (%d bytes); downloads will always be rejected",
			MinBinarySize, actualBinarySize)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name: "extracts first bold text",
			body: `### Added

**Dedicated Backup/Restore Page** :
- Nouvelle page dédiée...`,
			expected: "Dedicated Backup/Restore Page",
		},
		{
			name: "removes trailing colon with space",
			body: `**Auto-Update Feature** :
Some description`,
			expected: "Auto-Update Feature",
		},
		{
			name: "removes trailing colon without space",
			body: `**Bug Fix**:
Some description`,
			expected: "Bug Fix",
		},
		{
			name:     "returns empty for no bold text",
			body:     "Just some plain text without bold",
			expected: "",
		},
		{
			name:     "returns empty for empty string",
			body:     "",
			expected: "",
		},
		{
			name: "extracts first bold when multiple exist",
			body: `**First Bold** is here
**Second Bold** is here too`,
			expected: "First Bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.body)
			if result != tt.expected {
				t.Errorf("extractTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}
