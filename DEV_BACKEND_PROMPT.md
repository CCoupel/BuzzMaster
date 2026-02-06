# Prompt pour dev-backend : Auto-Update Implementation

## Contexte

You are the Backend Developer for BuzzControl auto-update feature (v2.50.0).

Your task is to implement the complete backend for auto-update functionality covering Phases 1 and 2 of the project plan.

## Feature Overview

Enable BuzzControl server to:
1. Detect new versions available on GitHub
2. Download binaries securely
3. Apply updates with graceful restart
4. Preserve game state across restarts
5. Rollback automatically if update fails

## Current State

- **Branch** : feature/auto-update
- **Version** : 2.49.0 → 2.50.0 (already bumped)
- **Contracts** : contracts/auto-update-endpoints.md (4 REST endpoints defined)
- **Plan** : PLAN_AUTO_UPDATE_v2.50.0.md (full implementation plan)
- **Spec** : DEV_BACKEND_AUTO_UPDATE.md (detailed backend spec)

## Implementation Scope

### Phase 1 : Backend API - Version Management

Implement 2 GET endpoints for version querying:

1. **GET /api/updates**
   - List all available versions from GitHub releases
   - Filter by current platform (windows-amd64 or linux-arm64)
   - Cache 1 hour (mandatory for GitHub rate limiting)
   - Return array of versions with metadata

2. **GET /api/updates/check**
   - Quick version check
   - Return: {update_available, current, latest, release_url}
   - Use same cache as GET /api/updates

### Phase 2 : Backend Download & Apply

Implement 2 POST endpoints for update management:

3. **POST /api/updates/download**
   - Download specific version binary
   - Validate version and platform
   - Verify integrity (minimum 40MB)
   - Return path and checksum

4. **POST /api/updates/apply**
   - Apply downloaded update
   - Save game state (if game in progress)
   - Close WebSocket connections gracefully
   - Backup old executable
   - Replace with atomic rename
   - Start new binary
   - Monitor for 5 seconds
   - Auto-rollback if new binary crashes

## Technical Requirements

### GitHub API Integration

- Endpoint: https://api.github.com/repos/CCoupel/BuzzMaster/releases
- No authentication required (public repo)
- Asset naming: buzzcontrol-vX.Y.Z-{os}-{arch}.(exe|binary)
- Rate limiting: 60 requests/hour without auth
- **MUST implement 1-hour cache** to avoid rate limiting

### Platform Detection

Use Go's runtime package:
```go
os := runtime.GOOS        // "windows" or "linux"
arch := runtime.GOARCH    // "amd64" or "arm64"
```

Expected platforms:
- Windows: windows-amd64
- Raspberry Pi: linux-arm64

### File Safety

- **Windows** : File locked during execution → atomic rename required
- **Backup** : Always backup before replacing (.bak for Windows, .old for Linux)
- **Rollback** : Automatic if new binary fails to start
- **Permissions** : Verify R/W access before operations

### Game State Preservation

- Save game state before shutdown
- Close WebSocket connections gracefully (wait max 2 seconds)
- Restore state after new server starts
- Notify clients of server restart

## Files to Create

1. **internal/server/updater.go** (main update logic)
   - Download logic
   - Apply logic
   - State management
   - Restart monitoring

2. **internal/server/github_client.go** (GitHub API client)
   - Fetch releases from GitHub
   - Cache management
   - Asset filtering

3. **internal/utils/file_backup.go** (file operations)
   - Backup functionality
   - Restore functionality
   - Atomic rename

## Files to Modify

1. **cmd/server/main.go**
   - Add 4 new endpoint handlers
   - Register routes for /api/updates/*

2. **internal/config/config.go**
   - Add auto_check_updates option (bool, default: true)

3. **internal/server/server.go**
   - Register the 4 update routes

4. **internal/game/engine.go**
   - Add SaveState() method
   - Add RestoreState(snapshot) method

## Contracts

All 4 endpoints defined in `contracts/auto-update-endpoints.md`:

### GET /api/updates
```json
Response: { "versions": [ { "version", "date", "notes", "download_url", "current", "size" } ] }
```

### GET /api/updates/check
```json
Response: { "update_available": bool, "current": "X.Y.Z", "latest": "X.Y.Z", "release_url": "" }
```

### POST /api/updates/download
```json
Request: { "version": "X.Y.Z" }
Response: { "success": bool, "version", "path", "size", "checksum" }
```

### POST /api/updates/apply
```json
Request: { "version": "X.Y.Z", "path": "/tmp/..." }
Response: { "success": bool, "message", "restart_in_seconds": 3 }
```

## Testing Requirements

Write comprehensive Go unit tests:

1. **GitHub Client Tests**
   - Mock GitHub API responses
   - Test version parsing
   - Test caching (1 hour TTL)
   - Test error scenarios

2. **Endpoint Tests**
   - GET /api/updates format validation
   - GET /api/updates/check logic
   - POST /api/updates/download validation
   - POST /api/updates/apply safety checks

3. **Integration Tests**
   - Platform detection
   - Asset filtering
   - File operations (backup, rename, restore)

**Run tests** :
```bash
cd server-go
go test ./... -v -cover
```

All tests must pass before marking complete.

## Error Handling

Handle all failure modes gracefully:

- GitHub API unreachable → return cached or error
- Version not found → 400 error
- Download corrupted → 400 error, cleanup temp file
- File operation failed → 500 error, no partial state
- New binary won't start → auto-rollback to old binary

## Logging & Debugging

- Log all GitHub API calls
- Log file operations (backup, rename, delete)
- Log version comparisons
- Log restart attempts and outcomes
- Use appropriate log levels (debug, info, warn, error)

## Code Quality

- Clear variable names
- Functions < 100 lines where practical
- Comprehensive error handling
- Comments for complex logic
- No hardcoded paths (use config)

## Validation Checklist

Before submitting, verify:

- [x] GET /api/updates returns versions with cache
- [x] GET /api/updates/check detects update available
- [x] POST /api/updates/download validates and downloads
- [x] POST /api/updates/apply applies safely
- [x] Rollback works on binary crash
- [x] Game state saved before restart
- [x] Backup created before file replace
- [x] All unit tests pass
- [x] No breaking changes to existing code
- [x] Proper error handling everywhere
- [x] Config option auto_check_updates added

## Summary of Tasks

1. Create github_client.go with caching (1h)
2. Create updater.go with download/apply logic (2-3h)
3. Create file_backup.go utilities (0.5h)
4. Modify main.go to register routes (0.5h)
5. Modify config.go for auto_check_updates (0.5h)
6. Modify engine.go for SaveState/RestoreState (0.5h)
7. Write comprehensive unit tests (1-2h)
8. Validate all endpoints work (0.5h)

**Total estimated time** : 6-9 hours

## Delivery Format

1. Submit branch: feature/auto-update (with all commits)
2. Verify: git log shows all changes
3. Summary: List of new files, modified files, endpoints tested
4. Test results: go test ./... output showing all pass

## Questions?

If ambiguous, refer to:
- Detailed spec: DEV_BACKEND_AUTO_UPDATE.md
- Plan: PLAN_AUTO_UPDATE_v2.50.0.md
- Backlog: backlog/TODO/notification-nouvelle-version.md
- Contracts: contracts/auto-update-endpoints.md

---

**Status** : READY TO IMPLEMENT

Begin implementation and commit regularly. After completion, submit summary for code review.

