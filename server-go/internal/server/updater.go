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
	// MinBinarySize is the minimum expected size for a valid BuzzControl binary (5MB)
	// BuzzControl binaries (Go + embedded React) are ~8-9MB on Windows, ~8MB on Linux ARM64
	MinBinarySize = 5 * 1024 * 1024

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
	Downloaded  bool   `json:"downloaded"`
	LocalPath   string `json:"local_path,omitempty"`
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
	githubClient *GitHubClient
	currentVer   string
	updatesDir   string
}

// NewUpdater creates a new updater. Downloaded binaries are stored persistently
// in {dataDir}/updates/ so they survive server restarts and can be applied later.
func NewUpdater(currentVersion, dataDir string) *Updater {
	updatesDir := filepath.Join(dataDir, "updates")
	os.MkdirAll(updatesDir, 0755)

	return &Updater{
		githubClient: NewGitHubClient(GitHubOwner, GitHubRepo),
		currentVer:   currentVersion,
		updatesDir:   updatesDir,
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
		// Skip releases that are not ready (draft, prerelease, CI in progress)
		if !IsReleaseReady(release, platform) {
			LogDebug(game.LogComponentUpdater, "Skipping release %s (not ready: draft=%v, prerelease=%v)",
				release.TagName, release.Draft, release.Prerelease)
			continue
		}

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

		// Check if this version is already downloaded locally
		ext := ""
		if runtime.GOOS == "windows" {
			ext = ".exe"
		}
		localFilename := fmt.Sprintf("buzzcontrol-v%s-%s%s", version, platform, ext)
		localPath := filepath.Join(u.updatesDir, localFilename)
		downloaded := false
		if info, err := os.Stat(localPath); err == nil && info.Size() >= MinBinarySize {
			downloaded = true
		}

		info := UpdateInfo{
			Version:     version,
			Title:       title,
			Date:        release.CreatedAt.Format(time.RFC3339),
			Notes:       release.Body,
			DownloadURL: asset.DownloadURL,
			Current:     isCurrent,
			Size:        asset.Size,
			Downloaded:  downloaded,
		}
		if downloaded {
			info.LocalPath = localPath
		}
		versions = append(versions, info)
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
		// Skip releases that are not ready (draft, prerelease, CI in progress)
		if !IsReleaseReady(release, platform) {
			LogDebug(game.LogComponentUpdater, "Skipping release %s in check (not ready: draft=%v, prerelease=%v)",
				release.TagName, release.Draft, release.Prerelease)
			continue
		}

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

// writeJSONError writes a JSON error response with the given HTTP status code.
func writeJSONError(w http.ResponseWriter, status int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// writeJSON writes a successful JSON response (200 OK).
func writeJSON(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleDownloadUpdate handles POST /api/updates/download
func (u *Updater) HandleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "POST /api/updates/download")

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogError(game.LogComponentUpdater, "Invalid request body: %v", err)
		writeJSONError(w, http.StatusBadRequest, DownloadResponse{Success: false, Error: "Invalid request body"})
		return
	}

	if req.Version == "" {
		LogError(game.LogComponentUpdater, "Missing version in request")
		writeJSONError(w, http.StatusBadRequest, DownloadResponse{Success: false, Error: "version is required"})
		return
	}

	LogInfo(game.LogComponentUpdater, "Downloading version: %s", req.Version)

	// Get releases to find download URL
	releases, err := u.githubClient.GetReleases()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get releases: %v", err)
		writeJSONError(w, http.StatusServiceUnavailable, DownloadResponse{Success: false, Error: fmt.Sprintf("Failed to fetch releases: %v", err)})
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
			LogError(game.LogComponentUpdater, "No asset found for platform %s in release %s", platform, release.TagName)
			writeJSONError(w, http.StatusNotFound, DownloadResponse{Success: false, Error: fmt.Sprintf("No binary available for platform %s", platform)})
			return
		}

		downloadURL = asset.DownloadURL
		break
	}

	if downloadURL == "" {
		LogError(game.LogComponentUpdater, "Version not found: %s", req.Version)
		writeJSONError(w, http.StatusNotFound, DownloadResponse{Success: false, Error: fmt.Sprintf("Version %s not found", req.Version)})
		return
	}

	// Download file
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	filename := fmt.Sprintf("buzzcontrol-v%s-%s%s", req.Version, platform, ext)
	destPath := filepath.Join(u.updatesDir, filename)

	LogInfo(game.LogComponentUpdater, "Downloading from %s to %s", downloadURL, destPath)

	if err := u.downloadFile(downloadURL, destPath); err != nil {
		LogError(game.LogComponentUpdater, "Download failed: %v", err)
		os.Remove(destPath)
		writeJSONError(w, http.StatusInternalServerError, DownloadResponse{Success: false, Error: fmt.Sprintf("Download failed: %v", err)})
		return
	}

	// Verify file size
	fileInfo, err := os.Stat(destPath)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to stat downloaded file: %v", err)
		os.Remove(destPath)
		writeJSONError(w, http.StatusInternalServerError, DownloadResponse{Success: false, Error: "File verification failed"})
		return
	}

	if fileInfo.Size() < MinBinarySize {
		LogError(game.LogComponentUpdater, "Downloaded file too small: %d bytes (min %d)", fileInfo.Size(), MinBinarySize)
		os.Remove(destPath)
		writeJSONError(w, http.StatusInternalServerError, DownloadResponse{Success: false, Error: fmt.Sprintf("Downloaded file too small (%d bytes), may be corrupted", fileInfo.Size())})
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

	writeJSON(w, DownloadResponse{
		Success:  true,
		Version:  req.Version,
		Path:     destPath,
		Size:     fileInfo.Size(),
		Checksum: checksum,
	})
}

// HandleApplyUpdate handles POST /api/updates/apply
func (u *Updater) HandleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentUpdater, "POST /api/updates/apply")

	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogError(game.LogComponentUpdater, "Invalid request body: %v", err)
		writeJSONError(w, http.StatusBadRequest, ApplyResponse{Success: false, Error: "Invalid request body"})
		return
	}

	if req.Version == "" || req.Path == "" {
		LogError(game.LogComponentUpdater, "Missing version or path in request")
		writeJSONError(w, http.StatusBadRequest, ApplyResponse{Success: false, Error: "version and path are required"})
		return
	}

	LogInfo(game.LogComponentUpdater, "Applying update: version=%s, path=%s", req.Version, req.Path)

	// Verify downloaded file exists and is valid
	fileInfo, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			LogError(game.LogComponentUpdater, "Downloaded file not found: %s", req.Path)
			writeJSONError(w, http.StatusNotFound, ApplyResponse{Success: false, Error: "Downloaded file not found — please download again"})
		} else {
			LogError(game.LogComponentUpdater, "Cannot access downloaded file: %v", err)
			writeJSONError(w, http.StatusInternalServerError, ApplyResponse{Success: false, Error: fmt.Sprintf("Cannot access file: %v", err)})
		}
		return
	}

	if fileInfo.Size() < MinBinarySize {
		LogError(game.LogComponentUpdater, "Downloaded file too small to apply: %d bytes", fileInfo.Size())
		writeJSONError(w, http.StatusBadRequest, ApplyResponse{Success: false, Error: "Downloaded file appears corrupted — please download again"})
		return
	}

	// Verify the path is inside the expected temp directory (security: prevent path traversal)
	absPath, err := filepath.Abs(req.Path)
	sep := string(os.PathSeparator)
	if err != nil || (!strings.HasPrefix(absPath, u.updatesDir+sep) && absPath != u.updatesDir) {
		LogError(game.LogComponentUpdater, "Suspicious path rejected: %s (expected prefix: %s)", req.Path, u.updatesDir)
		writeJSONError(w, http.StatusBadRequest, ApplyResponse{Success: false, Error: "Invalid file path"})
		return
	}

	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to get executable path: %v", err)
		writeJSONError(w, http.StatusInternalServerError, ApplyResponse{Success: false, Error: "Failed to determine executable path"})
		return
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		LogError(game.LogComponentUpdater, "Failed to resolve symlinks: %v", err)
		writeJSONError(w, http.StatusInternalServerError, ApplyResponse{Success: false, Error: "Failed to resolve executable path"})
		return
	}

	// Send success response BEFORE restarting (client must receive this before the server dies)
	writeJSON(w, ApplyResponse{
		Success:          true,
		Message:          fmt.Sprintf("Server restarting with version %s...", req.Version),
		RestartInSeconds: 3,
	})

	LogInfo(game.LogComponentUpdater, "Response sent, scheduling restart in 3 seconds...")

	// Schedule restart in a goroutine
	go func() {
		time.Sleep(3 * time.Second)
		u.performRestart(exe, req.Path)
	}()
}

// performRestart performs the actual restart operation.
//
// Strategy per OS:
//   - Linux/Mac: chmod + overwrite via copyFile (allowed while running)
//   - Windows: rename current→.bak, rename new→current (os.Rename works on running EXEs
//     because it only modifies the directory entry, not the file content)
//
// If any step fails, the backup is restored and the process does NOT exit.
func (u *Updater) performRestart(currentExe, newExe string) {
	LogInfo(game.LogComponentUpdater, "Starting restart procedure (OS: %s)", runtime.GOOS)
	LogInfo(game.LogComponentUpdater, "Current exe: %s", currentExe)
	LogInfo(game.LogComponentUpdater, "New exe: %s", newExe)

	// Step 1: Verify new executable exists and has valid size
	newInfo, err := os.Stat(newExe)
	if err != nil {
		LogError(game.LogComponentUpdater, "Step 1/5 FAILED: Cannot stat new executable: %v", err)
		return
	}
	if newInfo.Size() < MinBinarySize {
		LogError(game.LogComponentUpdater, "Step 1/5 FAILED: New executable too small (%d bytes)", newInfo.Size())
		return
	}
	LogInfo(game.LogComponentUpdater, "Step 1/5: New executable verified (%d bytes)", newInfo.Size())

	backup := getBackupPath(currentExe)

	// Step 2: Make new binary executable (Unix only), or set up rename approach for Windows
	if runtime.GOOS != "windows" {
		LogInfo(game.LogComponentUpdater, "Step 2/5: Setting executable permissions on new binary")
		if err := os.Chmod(newExe, 0755); err != nil {
			LogError(game.LogComponentUpdater, "Step 2/5 FAILED: Cannot chmod new binary: %v", err)
			return
		}
	} else {
		LogInfo(game.LogComponentUpdater, "Step 2/5: Skipped chmod (Windows)")
	}

	// Step 3: Back up current executable
	LogInfo(game.LogComponentUpdater, "Step 3/5: Creating backup: %s -> %s", currentExe, backup)
	os.Remove(backup) // Remove stale backup if any

	if runtime.GOOS == "windows" {
		// On Windows, os.Rename works on running executables (directory entry rename)
		if err := os.Rename(currentExe, backup); err != nil {
			LogError(game.LogComponentUpdater, "Step 3/5 FAILED: Cannot rename current exe to backup: %v", err)
			return
		}
	} else {
		// On Unix, copyFile is fine since we can overwrite a running binary
		if err := copyFile(currentExe, backup); err != nil {
			LogError(game.LogComponentUpdater, "Step 3/5 FAILED: Cannot copy current exe to backup: %v", err)
			return
		}
	}
	LogInfo(game.LogComponentUpdater, "Step 3/5: Backup created successfully")

	// Step 4: Place new executable at current path
	LogInfo(game.LogComponentUpdater, "Step 4/5: Installing new executable at %s", currentExe)

	var replaceErr error
	if runtime.GOOS == "windows" {
		// On Windows, use rename (atomic directory operation, works since backup freed the name)
		replaceErr = os.Rename(newExe, currentExe)
	} else {
		// On Unix, overwrite in place
		replaceErr = copyFile(newExe, currentExe)
	}

	if replaceErr != nil {
		LogError(game.LogComponentUpdater, "Step 4/5 FAILED: Cannot install new executable: %v", replaceErr)
		// Restore backup
		if runtime.GOOS == "windows" {
			if restoreErr := os.Rename(backup, currentExe); restoreErr != nil {
				LogError(game.LogComponentUpdater, "FATAL: Restore backup also failed: %v — manual restore needed from %s", restoreErr, backup)
			} else {
				LogInfo(game.LogComponentUpdater, "Backup restored successfully")
			}
		} else {
			if restoreErr := copyFile(backup, currentExe); restoreErr != nil {
				LogError(game.LogComponentUpdater, "FATAL: Restore backup also failed: %v", restoreErr)
			} else {
				LogInfo(game.LogComponentUpdater, "Backup restored successfully")
			}
		}
		return
	}
	LogInfo(game.LogComponentUpdater, "Step 4/5: New executable installed successfully")

	// Step 5: Start new process then exit
	LogInfo(game.LogComponentUpdater, "Step 5/5: Launching new process: %s", currentExe)
	cmd := exec.Command(currentExe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		LogError(game.LogComponentUpdater, "Step 5/5 FAILED: Cannot start new process: %v", err)
		// Restore backup
		if runtime.GOOS == "windows" {
			os.Remove(currentExe)
			if restoreErr := os.Rename(backup, currentExe); restoreErr != nil {
				LogError(game.LogComponentUpdater, "FATAL: Restore backup also failed: %v — manual restore needed from %s", restoreErr, backup)
			} else {
				LogInfo(game.LogComponentUpdater, "Backup restored successfully")
			}
		} else {
			if restoreErr := copyFile(backup, currentExe); restoreErr != nil {
				LogError(game.LogComponentUpdater, "FATAL: Restore backup also failed: %v", restoreErr)
			} else {
				LogInfo(game.LogComponentUpdater, "Backup restored successfully")
			}
		}
		return
	}

	LogInfo(game.LogComponentUpdater, "New server started with PID %d — exiting current process in 1s", cmd.Process.Pid)
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

// Helper functions

func (u *Updater) downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
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
