# Dev Backend - Auto-Update Implementation Spec (v2.50.0)

**Date** : 2026-02-01
**Feature** : Auto-Update Server
**Agent** : dev-backend
**Branch** : feature/auto-update
**Phases** : 1 + 2 (API + Download/Apply)

---

## Overview

Implement backend support for auto-update feature:
1. **Phase 1** : REST API endpoints to list and check for available updates from GitHub
2. **Phase 2** : Download and apply update mechanism with graceful restart and rollback

---

## Specification References

- **Contracts** : contracts/auto-update-endpoints.md (4 REST endpoints)
- **Plan** : PLAN_AUTO_UPDATE_v2.50.0.md (full project plan)
- **Backlog** : backlog/TODO/notification-nouvelle-version.md (original spec)

---

## Phase 1 : Backend API - Version Management

### Endpoint 1 : GET /api/updates

**Purpose** : List all available versions from GitHub releases for current platform

**Implementation Details** :

1. **Platform Detection**
   ```go
   os := runtime.GOOS        // "windows" or "linux"
   arch := runtime.GOARCH    // "amd64" or "arm64"
   platform := fmt.Sprintf("%s-%s", os, arch)
   // Expected: "windows-amd64" or "linux-arm64"
   ```

2. **GitHub Releases API Call**
   ```
   GET https://api.github.com/repos/CCoupel/BuzzMaster/releases
   ```
   - No authentication required (public repo)
   - Returns array of Release objects
   - Each release has Assets array with downloadable binaries

3. **Asset Filtering**
   - Pattern to match : `buzzcontrol-v*-{os}-{arch}.exe` (Windows) or `buzzcontrol-v*-{os}-{arch}` (Linux)
   - Extract version from filename
   - Map release body to "notes"
   - Include current version flag for each asset

4. **Caching Strategy**
   ```
   Cache key: "github_releases_list"
   TTL: 1 hour (3600 seconds)
   Cache hit: Return cached data
   Cache miss: Fetch from GitHub, cache result
   ```

5. **Response Structure**
   ```json
   {
     "versions": [
       {
         "version": "2.50.0",
         "date": "2026-02-01T10:30:00Z",
         "notes": "Auto-update support, improved performance",
         "download_url": "https://github.com/...",
         "current": true,
         "size": 45678900
       },
       ...
     ]
   }
   ```

**Handler Signature** :
```go
func (s *Server) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
    // Implement above logic
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

**Error Handling** :
- GitHub unreachable (503) : Return cached data if available, else error
- Malformed response : Log and return empty list
- No matching assets : Return empty versions list

---

### Endpoint 2 : GET /api/updates/check

**Purpose** : Quick version check (used by frontend for badge notification)

**Implementation Details** :

1. **Logic**
   - Get latest release from GitHub (or use cached list)
   - Compare current version (from config) with latest
   - Determine if update_available

2. **Version Comparison**
   ```go
   // Implement semantic version comparison
   // Current: 2.49.0, Latest: 2.50.0
   // update_available = Latest > Current
   ```

3. **Response Structure**
   ```json
   {
     "update_available": false,
     "current": "2.50.0",
     "latest": "2.50.0",
     "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
   }
   ```

4. **Caching**
   - Same as GET /api/updates (1 hour)
   - Re-parse cached list to get latest version

**Handler Signature** :
```go
func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
    // Implement above logic
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

---

### Config Option : auto_check_updates

**Location** : server-go/config.json

**Schema** :
```json
{
  "server": {
    "auto_check_updates": true
  }
}
```

**Type** : `bool`
**Default** : `true`
**Usage** : Can be used for background checks on startup (optional optimization)

**Code Changes** :
1. Add field to Config struct in `internal/config/config.go` :
   ```go
   type ServerConfig struct {
       ...
       AutoCheckUpdates bool `json:"auto_check_updates"`
   }
   ```

2. Initialize in config loading :
   ```go
   if cfg.Server.AutoCheckUpdates == nil {
       cfg.Server.AutoCheckUpdates = true
   }
   ```

---

## Phase 2 : Backend Download & Apply

### Endpoint 3 : POST /api/updates/download

**Purpose** : Download a specific version binary

**Implementation Details** :

1. **Request Parsing**
   ```json
   {
     "version": "2.50.0"
   }
   ```

2. **Download Logic**
   a. Get download URL from cached releases list (or call GET /api/updates)
   b. Verify version matches current platform
   c. Create temp directory (e.g., `/tmp/buzzcontrol-updates/`)
   d. Download binary to temp file
   e. Verify download integrity

3. **Integrity Verification**
   ```go
   // After download:
   - Check file size >= 40MB (minimum threshold)
   - Test if executable is valid (Unix: execute test, Windows: PE header)
   - Calculate SHA256 checksum
   ```

4. **Response Structure**
   ```json
   {
     "success": true,
     "version": "2.50.0",
     "path": "/tmp/buzzcontrol-updates/buzzcontrol-v2.50.0-windows-amd64.exe",
     "size": 45678900,
     "checksum": "abc123def456"
   }
   ```

5. **Error Handling**
   - Version not found (400)
   - Invalid platform (400)
   - Download failed (503)
   - File too small (400)
   - GitHub API unavailable (503)

**Handler Signature** :
```go
func (s *Server) handleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Version string `json:"version"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Implement download logic
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

**Files to Create** :
- `internal/server/updater.go` : Download logic
- `internal/utils/file_backup.go` : File operations utilities

---

### Endpoint 4 : POST /api/updates/apply

**Purpose** : Apply downloaded update and restart server

**Implementation Details** :

1. **Request Parsing**
   ```json
   {
     "version": "2.50.0",
     "path": "/tmp/buzzcontrol-updates/buzzcontrol-v2.50.0-windows-amd64.exe"
   }
   ```

2. **Pre-Apply Operations**
   a. Validate path and version match
   b. Verify file exists and is executable
   c. Save game state (if game in progress)
      ```go
      state := engine.SaveState() // New method in engine.go
      stateFile := filepath.Join(config.StorageDir, ".update_state.json")
      // Save to file
      ```
   d. Close all WebSocket connections gracefully
      ```go
      server.broadcastMessage("SERVER_RESTART", map[string]interface{}{
          "reason": "update_apply",
          "version": version,
      })
      // Wait 2 seconds for clients to disconnect
      ```

3. **File Replacement (Atomic)**
   a. Get current executable path
      ```go
      exe, _ := os.Executable()
      ```
   b. Create backup
      ```go
      // Windows: exe.bak
      // Linux: exe.old
      backup := exe + ".bak"
      os.Rename(exe, backup)
      ```
   c. Move new binary to executable location
      ```go
      os.Rename(newBinaryPath, exe)
      ```

4. **Start New Binary**
   ```go
   cmd := exec.Command(exe)
   cmd.Start()

   // Give 5 seconds for new server to start
   // Monitor if process exits (failure)
   ```

5. **Restart Monitoring**
   - Wait 5 seconds
   - Test if new server is responding
   - If OK: Remove backup, exit gracefully
   - If FAIL: Restore from backup, return error

6. **Rollback on Failure**
   ```go
   // If new server doesn't respond after 5s:
   backup := exe + ".bak"
   os.Rename(backup, exe)
   cmd := exec.Command(exe)
   cmd.Start()
   ```

7. **Response Structure**
   ```json
   {
     "success": true,
     "message": "Server restarting with version 2.50.0...",
     "restart_in_seconds": 3
   }
   ```

**Error Handling** :
- Invalid path (400)
- Version mismatch (400)
- File operations failed (500)
- New binary failed to start (500 with automatic rollback)

**Handler Signature** :
```go
func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Version string `json:"version"`
        Path    string `json:"path"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Implement apply logic
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

---

## Game State Preservation

### New Methods in internal/game/engine.go

**SaveState()** :
```go
func (e *Engine) SaveState() (*GameStateSnapshot, error) {
    // Return current game state as JSON-serializable struct
    return &GameStateSnapshot{
        Teams: e.teams,
        CurrentQuestion: e.currentQuestion,
        GamePhase: e.gamePhase,
        // ... all relevant state
    }, nil
}
```

**RestoreState(snapshot)** :
```go
func (e *Engine) RestoreState(snapshot *GameStateSnapshot) error {
    // Restore engine to saved state
    e.teams = snapshot.Teams
    e.currentQuestion = snapshot.CurrentQuestion
    e.gamePhase = snapshot.GamePhase
    // ... all fields
    return nil
}
```

### New File : internal/utils/file_backup.go

```go
package utils

import (
    "os"
    "runtime"
)

// GetBackupPath returns platform-specific backup path
func GetBackupPath(exePath string) string {
    if runtime.GOOS == "windows" {
        return exePath + ".bak"
    }
    return exePath + ".old"
}

// BackupFile creates a backup of the given file
func BackupFile(filePath string) error {
    backup := GetBackupPath(filePath)
    return os.Rename(filePath, backup)
}

// RestoreFromBackup restores a file from backup
func RestoreFromBackup(exePath string) error {
    backup := GetBackupPath(exePath)
    return os.Rename(backup, exePath)
}
```

---

## GitHub API Client

### New File : internal/server/github_client.go

```go
package server

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type GitHubRelease struct {
    TagName string       `json:"tag_name"`
    Name    string       `json:"name"`
    Body    string       `json:"body"`
    Assets  []GitHubAsset `json:"assets"`
    CreatedAt time.Time  `json:"created_at"`
}

type GitHubAsset struct {
    Name        string `json:"name"`
    DownloadURL string `json:"browser_download_url"`
    Size        int64  `json:"size"`
}

type GitHubClient struct {
    RepoOwner string
    RepoName  string
    Client    *http.Client
    Cache     map[string]interface{}
    CacheTTL  time.Duration
}

func NewGitHubClient(owner, repo string) *GitHubClient {
    return &GitHubClient{
        RepoOwner: owner,
        RepoName:  repo,
        Client:    &http.Client{Timeout: 10 * time.Second},
        Cache:     make(map[string]interface{}),
        CacheTTL:  1 * time.Hour,
    }
}

func (gc *GitHubClient) GetReleases() ([]GitHubRelease, error) {
    // Implement: check cache, fetch from API, cache result
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases",
        gc.RepoOwner, gc.RepoName)

    resp, err := gc.Client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var releases []GitHubRelease
    json.NewDecoder(resp.Body).Decode(&releases)
    return releases, nil
}
```

---

## Route Registration

### Modify cmd/server/main.go

```go
func (s *Server) setupUpdateRoutes() {
    s.router.HandleFunc("GET", "/api/updates", s.handleGetUpdates)
    s.router.HandleFunc("GET", "/api/updates/check", s.handleCheckUpdates)
    s.router.HandleFunc("POST", "/api/updates/download", s.handleDownloadUpdate)
    s.router.HandleFunc("POST", "/api/updates/apply", s.handleApplyUpdate)
}

func init() {
    // In server setup:
    s.setupUpdateRoutes()
}
```

---

## Testing Requirements

### Unit Tests (Go)

Create `internal/server/updater_test.go` and `internal/server/github_client_test.go`

**Test Cases** :

1. **GitHub Client**
   - Mock GitHub API responses
   - Test version parsing
   - Test caching logic
   - Test error handling

2. **Endpoints**
   - GET /api/updates returns correct format
   - GET /api/updates/check detects update available
   - POST /api/updates/download validates version
   - POST /api/updates/apply requires valid path

3. **Platform Detection**
   - Correctly identify Windows vs Linux
   - Filter correct binary asset

4. **File Operations**
   - Backup functionality
   - Restore on error
   - Atomic rename

**Commands** :
```bash
go test ./internal/server/... -v -cover
go test ./internal/utils/... -v -cover
```

---

## Error Handling Strategy

| Scenario | Status Code | Response | Action |
|----------|-------------|----------|--------|
| GitHub unreachable | 503 | Return cached data or empty | Log error |
| Version not found | 400 | Error message | None |
| Invalid platform | 400 | Error message | None |
| Download failed | 503 | Error message | Cleanup temp file |
| File too small | 400 | Error message | Delete bad file |
| Backup failed | 500 | Error message | Don't proceed |
| New binary invalid | 400 | Error message | Rollback |
| New binary crash | 500 | Error message | Auto-rollback |

---

## File Changes Summary

### Files to Create

| File | Purpose |
|------|---------|
| `internal/server/updater.go` | Main update logic (phases 1-2) |
| `internal/server/github_client.go` | GitHub API client with caching |
| `internal/utils/file_backup.go` | File backup/restore utilities |

### Files to Modify

| File | Changes |
|------|---------|
| `cmd/server/main.go` | Add 4 new endpoints |
| `internal/config/config.go` | Add auto_check_updates field |
| `internal/server/server.go` | Register routes |
| `internal/game/engine.go` | Add SaveState/RestoreState methods |

---

## Implementation Checklist

### Phase 1 : API (GET endpoints)
- [ ] GitHub client with caching
- [ ] Platform detection
- [ ] GET /api/updates endpoint
- [ ] GET /api/updates/check endpoint
- [ ] Config option auto_check_updates

### Phase 2 : Download/Apply (POST endpoints)
- [ ] POST /api/updates/download endpoint
- [ ] POST /api/updates/apply endpoint
- [ ] Game state save/restore
- [ ] Backup/restore utilities
- [ ] Graceful shutdown notification
- [ ] Atomic file replacement
- [ ] Restart monitoring and rollback

### Testing
- [ ] Unit tests for all endpoints
- [ ] Unit tests for GitHub client
- [ ] Unit tests for file operations
- [ ] All tests pass (go test ./...)

### Documentation
- [ ] Code comments
- [ ] Error messages clear
- [ ] Logging comprehensive

---

## Validation Checkpoints

Before marking complete :

1. **Functionality**
   - [ ] GET /api/updates returns versions with cache
   - [ ] GET /api/updates/check detects latest
   - [ ] POST /api/updates/download validates and downloads
   - [ ] POST /api/updates/apply safely restarts
   - [ ] Rollback works if new binary crashes

2. **Safety**
   - [ ] Backup created before any file change
   - [ ] Game state saved before restart
   - [ ] Connections closed gracefully
   - [ ] Rollback automatic on failure

3. **Testing**
   - [ ] go test ./... passes with coverage
   - [ ] All error paths tested
   - [ ] Mock GitHub API tested

4. **Documentation**
   - [ ] Contracts validated
   - [ ] Code comments complete
   - [ ] Error handling documented

---

## Summary

Implement robust backend auto-update system with :
- 4 REST endpoints for version management and updates
- GitHub API integration with smart caching
- Safe download and atomic file replacement
- Graceful restart with state preservation
- Automatic rollback on failure
- Comprehensive error handling and logging

**Estimated Time** : 4-6 hours (depending on testing depth)

**Deliverable** : Working backend with tests, ready for frontend integration

