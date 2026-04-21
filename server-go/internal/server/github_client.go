package server

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Body       string        `json:"body"`
	Assets     []GitHubAsset `json:"assets"`
	CreatedAt  time.Time     `json:"created_at"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
}

// GitHubAsset represents a downloadable asset from a release
type GitHubAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// GitHubClient handles GitHub API communication
type GitHubClient struct {
	RepoOwner string
	RepoName  string
	Client    *http.Client
	token     string // Optional GitHub auth token (GITHUB_TOKEN env var) to avoid rate limiting
	cache     *releasesCache
}

// releasesCache stores cached releases with TTL
type releasesCache struct {
	mu        sync.RWMutex
	releases  []GitHubRelease
	fetchedAt time.Time
	ttl       time.Duration
}

// NewGitHubClient creates a new GitHub API client.
// If the GITHUB_TOKEN environment variable is set, it is used for authentication
// to avoid the 60 req/hour unauthenticated rate limit.
func NewGitHubClient(owner, repo string) *GitHubClient {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		LogInfo(game.LogComponentUpdater, "GitHub client initialized with auth token (rate limit: 5000 req/h)")
	} else {
		LogInfo(game.LogComponentUpdater, "GitHub client initialized without auth token (rate limit: 60 req/h)")
	}
	return &GitHubClient{
		RepoOwner: owner,
		RepoName:  repo,
		Client:    &http.Client{Timeout: 10 * time.Second},
		token:     token,
		cache: &releasesCache{
			ttl: 1 * time.Hour,
		},
	}
}

// GetReleases fetches releases from GitHub API with caching
func (gc *GitHubClient) GetReleases() ([]GitHubRelease, error) {
	// Check cache first
	if releases, valid := gc.cache.get(); valid {
		LogInfo(game.LogComponentUpdater, "Using cached releases (age: %v)", time.Since(gc.cache.fetchedAt))
		return releases, nil
	}

	// Fetch from GitHub
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", gc.RepoOwner, gc.RepoName)
	LogInfo(game.LogComponentUpdater, "Fetching releases from GitHub: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add User-Agent header (required by GitHub API)
	req.Header.Set("User-Agent", "BuzzControl-Server")
	req.Header.Set("Accept", "application/vnd.github+json")

	// Add auth token if available to avoid rate limiting (60 → 5000 req/h)
	if gc.token != "" {
		req.Header.Set("Authorization", "Bearer "+gc.token)
	}

	resp, err := gc.Client.Do(req)
	if err != nil {
		LogError(game.LogComponentUpdater, "GitHub API request failed: %v", err)
		// Return cached data if available, even if expired
		if releases := gc.cache.getExpired(); releases != nil {
			LogWarn(game.LogComponentUpdater, "Using expired cache due to API failure")
			return releases, nil
		}
		return nil, fmt.Errorf("failed to fetch from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		LogError(game.LogComponentUpdater, "GitHub API returned status %d", resp.StatusCode)
		// Return cached data if available
		if releases := gc.cache.getExpired(); releases != nil {
			LogWarn(game.LogComponentUpdater, "Using expired cache due to API error (status %d)", resp.StatusCode)
			return releases, nil
		}
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		LogError(game.LogComponentUpdater, "Failed to decode GitHub response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Update cache
	gc.cache.set(releases)
	LogInfo(game.LogComponentUpdater, "Fetched %d releases from GitHub", len(releases))

	return releases, nil
}

// GetPlatformString returns the current platform identifier
func GetPlatformString() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// FindAssetForPlatform finds the matching asset for the current platform
func FindAssetForPlatform(release GitHubRelease, platform string) *GitHubAsset {
	// Expected pattern: buzzcontrol-vX.Y.Z-{os}-{arch}.exe (Windows) or buzzcontrol-vX.Y.Z-{os}-{arch} (Linux)
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, platform) {
			return &asset
		}
	}
	return nil
}

// ParseVersion extracts version string from tag name (e.g., "v2.50.0" -> "2.50.0")
func ParseVersion(tagName string) string {
	return strings.TrimPrefix(tagName, "v")
}

// CompareVersions compares two semantic version strings
// Returns 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Split by '.'
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		var n1, n2 int
		fmt.Sscanf(parts1[i], "%d", &n1)
		fmt.Sscanf(parts2[i], "%d", &n2)

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	// If all compared parts are equal, longer version is greater
	if len(parts1) > len(parts2) {
		return 1
	}
	if len(parts1) < len(parts2) {
		return -1
	}

	return 0
}

// Cache methods

func (c *releasesCache) get() ([]GitHubRelease, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Since(c.fetchedAt) < c.ttl && c.releases != nil {
		return c.releases, true
	}
	return nil, false
}

func (c *releasesCache) getExpired() []GitHubRelease {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releases
}

func (c *releasesCache) set(releases []GitHubRelease) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases = releases
	c.fetchedAt = time.Now()
}

// IsReleaseReady checks if a release is complete and ready for use
// Returns false if draft, prerelease, or assets not ready
func IsReleaseReady(release GitHubRelease, platform string) bool {
	// Skip drafts
	if release.Draft {
		return false
	}

	// Skip prereleases (beta/alpha versions)
	if release.Prerelease {
		return false
	}

	// Check if asset exists for platform
	asset := FindAssetForPlatform(release, platform)
	if asset == nil {
		return false
	}

	// Check asset size (CI might still be uploading)
	// Assets smaller than 1MB are likely incomplete or corrupted
	const minAssetSize = 1024 * 1024 // 1MB
	if asset.Size < minAssetSize {
		return false
	}

	return true
}
