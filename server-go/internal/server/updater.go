package server

import (
	"buzzcontrol/internal/game"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	// MinBinarySize is the minimum expected size for a valid BuzzControl binary (40MB)
	MinBinarySize = 40 * 1024 * 1024

	// GitHub repository details
	GitHubOwner = "CCoupel"
	GitHubRepo  = "BuzzMaster"

	// Restart monitoring timeout
	RestartTimeout = 5 * time.Second
)

// UpdateInfo represents information about a version
type UpdateInfo struct {
	Version     string `json:"version"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	Notes       string `json:"notes"`
	DownloadURL string `json:"download_url"`
	Current     bool   `json:"current"`
	Size        int64  `json:"size"`
}

// UpdatesResponse is the response for GET /api/updates
type UpdatesResponse struct {
	Versions []UpdateInfo `json:"versions"`
}

// CheckResponse is the response for GET /api/updates/check
type CheckResponse struct {
	UpdateAvailable bool   `json:"update_available"`
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	ReleaseURL      string `json:"release_url"`
}

// DownloadRequest is the request for POST /api/updates/download
type DownloadRequest struct {
	Version string `json:"version"`
}

// DownloadResponse is the response for POST /api/updates/download
type DownloadResponse struct {
	Success  bool   `json:"success"`
	Version  string `json:"version,omitempty"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"checksum,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ApplyRequest is the request for POST /api/updates/apply
type ApplyRequest struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ApplyResponse is the response for POST /api/updates/apply
type ApplyResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message,omitempty"`
	RestartInSeconds int    `json:"restart_in_seconds,omitempty"`
	Error            string `json:"error,omitempty"`
}

// Updater handles update operations
type Updater struct {
	githubClient  *GitHubClient
	currentVer    string
	tempDir       string
}

// NewUpdater creates a new updater
func NewUpdater(currentVersion string) *Updater {
	tempDir := filepath.Join(os.TempDir(), "buzzcontrol-updates")
	os.MkdirAll(tempDir, 0755)

	return &Updater{
		githubClient: NewGitHubClient(GitHubOwner, GitHubRepo),
		currentVer:   currentVersion,
		tempDir:      tempDir,
	}
}

// extractTitle extracts the first bold text (**...**) from markdown body
func extractTitle(body string) string {
	re := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	matches := re.FindStringSubmatch(body)
	if len(matches) >= 2 {
		// Clean title (remove trailing : or spaces)
		title := strings.TrimSuffix(matches[1], " :")
		title = strings.TrimSuffix(title, ":")
		title = strings.TrimSpace(title)
		return title
	}
	return ""
}

// HandleGetUpdates handles GET /api/updates
func (u *Updater) HandleGetUpdates(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "GET /api/updates")

	// Get releases from GitHub
	releases, err := u.githubClient.GetReleases()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get releases: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch releases: %v"}`, err), http.StatusServiceUnavailable)
		return
	}

	// Filter and build response
	platform := GetPlatformString()
	LogInfo(game.LogComponentUpdater, "Platform: %s", platform)

	var versions []UpdateInfo
	for _, release := range releases {
		asset := FindAssetForPlatform(release, platform)
		if asset == nil {
			LogDebug(game.LogComponentUpdater, "No asset found for platform %s in release %s", platform, release.TagName)
			continue
		}

		version := ParseVersion(release.TagName)
		isCurrent := version == u.currentVer

		// Extract title from first bold text in body
		title := extractTitle(release.Body)
		if title == "" {
			title = release.Name // fallback
		}

		versions = append(versions, UpdateInfo{
			Version:     version,
			Title:       title,
			Date:        release.CreatedAt.Format(time.RFC3339),
			Notes:       release.Body,
			DownloadURL: asset.DownloadURL,
			Current:     isCurrent,
			Size:        asset.Size,
		})
	}

	response := UpdatesResponse{Versions: versions}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	LogInfo(game.LogComponentUpdater, "Returned %d versions for platform %s", len(versions), platform)
}

// HandleCheckUpdates handles GET /api/updates/check
func (u *Updater) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "GET /api/updates/check")

	// Get releases from GitHub
	releases, err := u.githubClient.GetReleases()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get releases: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch releases: %v"}`, err), http.StatusServiceUnavailable)
		return
	}

	// Find latest release for current platform
	platform := GetPlatformString()
	var latestVersion string
	var releaseURL string

	for _, release := range releases {
		asset := FindAssetForPlatform(release, platform)
		if asset == nil {
			continue
		}

		version := ParseVersion(release.TagName)
		if latestVersion == "" || CompareVersions(version, latestVersion) > 0 {
			latestVersion = version
			releaseURL = fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", GitHubOwner, GitHubRepo, release.TagName)
		}
	}

	// Compare with current version
	updateAvailable := false
	if latestVersion != "" {
		updateAvailable = CompareVersions(latestVersion, u.currentVer) > 0
	}

	response := CheckResponse{
		UpdateAvailable: updateAvailable,
		Current:         u.currentVer,
		Latest:          latestVersion,
		ReleaseURL:      releaseURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	LogInfo(game.LogComponentUpdater, "Check result: current=%s, latest=%s, available=%v", u.currentVer, latestVersion, updateAvailable)
}

// HandleDownloadUpdate handles POST /api/updates/download
func (u *Updater) HandleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "POST /api/updates/download")

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogError(game.LogComponentUpdater, "Invalid request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "Invalid request body"})
		return
	}

	LogInfo(game.LogComponentUpdater, "Downloading version: %s", req.Version)

	// Get releases to find download URL
	releases, err := u.githubClient.GetReleases()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get releases: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "Failed to fetch releases"})
		return
	}

	// Find matching release
	platform := GetPlatformString()
	var downloadURL string

	for _, release := range releases {
		version := ParseVersion(release.TagName)
		if version != req.Version {
			continue
		}

		asset := FindAssetForPlatform(release, platform)
		if asset == nil {
			LogError(game.LogComponentUpdater, "No asset found for platform %s", platform)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "Invalid platform"})
			return
		}

		downloadURL = asset.DownloadURL
		break
	}

	if downloadURL == "" {
		LogError(game.LogComponentUpdater, "Version not found: %s", req.Version)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "Version not found"})
		return
	}

	// Download file
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	filename := fmt.Sprintf("buzzcontrol-v%s-%s%s", req.Version, platform, ext)
	destPath := filepath.Join(u.tempDir, filename)

	LogInfo(game.LogComponentUpdater, "Downloading from %s to %s", downloadURL, destPath)

	if err := u.downloadFile(downloadURL, destPath); err != nil {
		LogError(game.LogComponentUpdater, "Download failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: fmt.Sprintf("Download failed: %v", err)})
		return
	}

	// Verify file size
	fileInfo, err := os.Stat(destPath)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to stat downloaded file: %v", err)
		os.Remove(destPath)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "File verification failed"})
		return
	}

	if fileInfo.Size() < MinBinarySize {
		LogError(game.LogComponentUpdater, "Downloaded file too small: %d bytes (min %d)", fileInfo.Size(), MinBinarySize)
		os.Remove(destPath)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadResponse{Success: false, Error: "File too small"})
		return
	}

	// Calculate checksum for verification and audit
	// NOTE: This checksum is returned to the client for future validation.
	// Currently we rely on HTTPS + size validation for integrity.
	// TODO: Fetch official checksums from GitHub releases for stronger validation.
	checksum, err := u.calculateChecksum(destPath)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to calculate checksum: %v", err)
		checksum = ""
	} else {
		LogInfo(game.LogComponentUpdater, "Downloaded file checksum: %s", checksum)
	}

	LogInfo(game.LogComponentUpdater, "Download complete: %d bytes (min: %d), checksum=%s", fileInfo.Size(), MinBinarySize, checksum)

	response := DownloadResponse{
		Success:  true,
		Version:  req.Version,
		Path:     destPath,
		Size:     fileInfo.Size(),
		Checksum: checksum,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleApplyUpdate handles POST /api/updates/apply
func (u *Updater) HandleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "POST /api/updates/apply")

	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogError(game.LogComponentUpdater, "Invalid request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApplyResponse{Success: false, Error: "Invalid request body"})
		return
	}

	LogInfo(game.LogComponentUpdater, "Applying update: version=%s, path=%s", req.Version, req.Path)

	// Verify file exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		LogError(game.LogComponentUpdater, "File not found: %s", req.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApplyResponse{Success: false, Error: "Invalid path"})
		return
	}

	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get executable path: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApplyResponse{Success: false, Error: "Failed to determine executable path"})
		return
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to resolve symlinks: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApplyResponse{Success: false, Error: "Failed to resolve executable path"})
		return
	}

	// Send success response BEFORE restarting
	response := ApplyResponse{
		Success:          true,
		Message:          fmt.Sprintf("Server restarting with version %s...", req.Version),
		RestartInSeconds: 3,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	LogInfo(game.LogComponentUpdater, "Response sent, scheduling restart in 3 seconds...")

	// Schedule restart in a goroutine
	go func() {
		time.Sleep(3 * time.Second)
		u.performRestart(exe, req.Path)
	}()
}

// performRestart performs the actual restart operation
// NOTE: This function uses a copy-based approach instead of atomic rename
// because Windows does not allow renaming an executable while it's running.
// To minimize the risk of inconsistent state:
// 1. Backup is created BEFORE any modification
// 2. Each step is verified before proceeding
// 3. Backup is restored on any failure
// RISK: If the process crashes between backup creation and new file copy,
// the system will be in an inconsistent state. The backup can be manually
// restored in this scenario.
func (u *Updater) performRestart(currentExe, newExe string) {
	LogInfo(game.LogComponentUpdater, "Starting restart procedure...")
	LogInfo(game.LogComponentUpdater, "Current exe: %s", currentExe)
	LogInfo(game.LogComponentUpdater, "New exe: %s", newExe)

	// Step 1: Verify new executable exists
	newInfo, err := os.Stat(newExe)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to stat new executable: %v", err)
		return
	}
	LogInfo(game.LogComponentUpdater, "Step 1/5: New executable verified (%d bytes)", newInfo.Size())

	// Step 2: Create backup BEFORE any modification
	backup := getBackupPath(currentExe)
	LogInfo(game.LogComponentUpdater, "Step 2/5: Creating backup: %s -> %s", currentExe, backup)

	// Remove old backup if exists
	if _, err := os.Stat(backup); err == nil {
		LogInfo(game.LogComponentUpdater, "Removing old backup: %s", backup)
		os.Remove(backup)
	}

	// Copy current exe to backup (safer than rename on Windows as exe is running)
	if err := copyFile(currentExe, backup); err != nil {
		LogError(game.LogComponentUpdater, "CRITICAL: Failed to create backup: %v", err)
		return
	}

	// Verify backup was created successfully
	if _, err := os.Stat(backup); err != nil {
		LogError(game.LogComponentUpdater, "CRITICAL: Backup verification failed: %v", err)
		return
	}
	LogInfo(game.LogComponentUpdater, "Step 2/5: Backup created successfully")

	// Step 3: Make new binary executable (Unix only)
	if runtime.GOOS != "windows" {
		LogInfo(game.LogComponentUpdater, "Step 3/5: Setting executable permissions")
		if err := os.Chmod(newExe, 0755); err != nil {
			LogError(game.LogComponentUpdater, "Failed to make binary executable: %v", err)
			return
		}
	} else {
		LogInfo(game.LogComponentUpdater, "Step 3/5: Skipped (Windows)")
	}

	// Step 4: Replace current executable
	// WARNING: Non-atomic operation - if process crashes here, manual restore needed
	LogInfo(game.LogComponentUpdater, "Step 4/5: Replacing executable: %s <- %s", currentExe, newExe)
	if err := copyFile(newExe, currentExe); err != nil {
		LogError(game.LogComponentUpdater, "CRITICAL: Failed to replace executable: %v", err)
		// Try to restore backup
		LogInfo(game.LogComponentUpdater, "Attempting to restore backup...")
		if restoreErr := copyFile(backup, currentExe); restoreErr != nil {
			LogError(game.LogComponentUpdater, "FATAL: Failed to restore backup: %v", restoreErr)
		} else {
			LogInfo(game.LogComponentUpdater, "Backup restored successfully")
		}
		return
	}
	LogInfo(game.LogComponentUpdater, "Step 4/5: Executable replaced successfully")

	// Step 5: Start new executable
	LogInfo(game.LogComponentUpdater, "Step 5/5: Starting new executable: %s", currentExe)
	cmd := exec.Command(currentExe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		LogError(game.LogComponentUpdater, "CRITICAL: Failed to start new executable: %v", err)
		// Restore backup
		LogInfo(game.LogComponentUpdater, "Attempting to restore backup...")
		if restoreErr := copyFile(backup, currentExe); restoreErr != nil {
			LogError(game.LogComponentUpdater, "FATAL: Failed to restore backup: %v", restoreErr)
		} else {
			LogInfo(game.LogComponentUpdater, "Backup restored successfully")
		}
		return
	}

	LogInfo(game.LogComponentUpdater, "New server started with PID %d", cmd.Process.Pid)
	LogInfo(game.LogComponentUpdater, "Exiting old server in 1 second...")

	// Exit current process
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

// Helper functions

func (u *Updater) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (u *Updater) calculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func getBackupPath(exePath string) string {
	if runtime.GOOS == "windows" {
		return exePath + ".bak"
	}
	return exePath + ".old"
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
