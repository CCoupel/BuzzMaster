package server

import (
	"testing"
	"time"
)

func TestGetPlatformString(t *testing.T) {
	platform := GetPlatformString()
	if platform == "" {
		t.Error("Platform string should not be empty")
	}
	// Should be in format "os-arch" like "windows-amd64" or "linux-arm64"
	if len(platform) < 5 {
		t.Errorf("Platform string too short: %s", platform)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v2.50.0", "2.50.0"},
		{"v1.0.0", "1.0.0"},
		{"2.49.0", "2.49.0"},
		{"v10.20.30", "10.20.30"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseVersion(tt.input)
			if result != tt.expected {
				t.Errorf("ParseVersion(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"2.50.0", "2.49.0", 1},
		{"2.49.0", "2.50.0", -1},
		{"2.50.0", "2.50.0", 0},
		{"v2.50.0", "v2.49.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"10.0.0", "9.0.0", 1},
		{"2.50.1", "2.50.0", 1},
		{"2.50.0", "2.50.1", -1},
		{"3.0.0", "2.99.99", 1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("CompareVersions(%s, %s) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestFindAssetForPlatform(t *testing.T) {
	release := GitHubRelease{
		TagName: "v2.50.0",
		Assets: []GitHubAsset{
			{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", DownloadURL: "https://example.com/win.exe", Size: 50000000},
			{Name: "buzzcontrol-v2.50.0-linux-arm64", DownloadURL: "https://example.com/linux", Size: 48000000},
		},
	}

	tests := []struct {
		platform string
		found    bool
	}{
		{"windows-amd64", true},
		{"linux-arm64", true},
		{"darwin-amd64", false},
		{"linux-amd64", false},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			asset := FindAssetForPlatform(release, tt.platform)
			if tt.found && asset == nil {
				t.Errorf("Expected to find asset for platform %s, but got nil", tt.platform)
			}
			if !tt.found && asset != nil {
				t.Errorf("Expected no asset for platform %s, but found one", tt.platform)
			}
		})
	}
}

func TestReleasesCache(t *testing.T) {
	// Use a large TTL so the cache does not expire during the test.
	// 100 ns was too short: by the time set() returns and get() is called,
	// more than 100 ns has elapsed on any modern machine.
	cache := &releasesCache{
		ttl: 1 * time.Minute,
	}

	releases := []GitHubRelease{
		{TagName: "v2.50.0"},
	}

	// Test empty cache
	if _, valid := cache.get(); valid {
		t.Error("Empty cache should not be valid")
	}

	// Set cache
	cache.set(releases)

	// Immediately get - should be valid (TTL is 1 minute, so it can't expire yet)
	result, valid := cache.get()
	if !valid {
		t.Error("Fresh cache should be valid")
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 release, got %d", len(result))
	}

	// getExpired should always return data regardless of TTL
	expired := cache.getExpired()
	if len(expired) != 1 {
		t.Errorf("getExpired should return cached data, got %d releases", len(expired))
	}
}

func TestIsReleaseReady(t *testing.T) {
	tests := []struct {
		name     string
		release  GitHubRelease
		platform string
		expected bool
		reason   string
	}{
		{
			name: "draft release",
			release: GitHubRelease{
				Draft:      true,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", Size: 50000000},
				},
			},
			platform: "windows-amd64",
			expected: false,
			reason:   "Draft releases should not be ready",
		},
		{
			name: "prerelease",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: true,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-beta-windows-amd64.exe", Size: 50000000},
				},
			},
			platform: "windows-amd64",
			expected: false,
			reason:   "Prereleases should not be ready",
		},
		{
			name: "no assets",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets:     []GitHubAsset{},
			},
			platform: "windows-amd64",
			expected: false,
			reason:   "Releases without assets should not be ready",
		},
		{
			name: "wrong platform",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-linux-arm64", Size: 50000000},
				},
			},
			platform: "windows-amd64",
			expected: false,
			reason:   "Assets for wrong platform should not match",
		},
		{
			name: "small asset (CI in progress)",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", Size: 100},
				},
			},
			platform: "windows-amd64",
			expected: false,
			reason:   "Assets smaller than 1MB should not be ready (CI in progress)",
		},
		{
			name: "valid asset at 1MB threshold",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", Size: 1024 * 1024},
				},
			},
			platform: "windows-amd64",
			expected: true,
			reason:   "Assets at exactly 1MB should be ready",
		},
		{
			name: "ready release",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", Size: 50000000},
				},
			},
			platform: "windows-amd64",
			expected: true,
			reason:   "Valid releases with complete assets should be ready",
		},
		{
			name: "ready release with multiple assets",
			release: GitHubRelease{
				Draft:      false,
				Prerelease: false,
				Assets: []GitHubAsset{
					{Name: "buzzcontrol-v2.50.0-linux-arm64", Size: 48000000},
					{Name: "buzzcontrol-v2.50.0-windows-amd64.exe", Size: 50000000},
				},
			},
			platform: "windows-amd64",
			expected: true,
			reason:   "Should find correct asset among multiple platforms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReleaseReady(tt.release, tt.platform)
			if result != tt.expected {
				t.Errorf("IsReleaseReady() = %v, want %v. Reason: %s", result, tt.expected, tt.reason)
			}
		})
	}
}
