# Backend Implementation Summary - For Frontend Integration

**Date** : 2026-02-01
**Feature** : Auto-Update v2.50.0
**Status** : COMPLETED
**Backend Developer** : dev-backend
**Frontend Developer** : dev-frontend (next)

---

## Overview

Backend has successfully implemented all 4 REST endpoints for auto-update feature. Frontend can now consume these endpoints to build the user interface.

---

## Implemented Endpoints

### 1. GET /api/updates

**Purpose** : List all available versions from GitHub releases for current platform

**Request** :
```
GET http://localhost/api/updates
```

**Response 200** :
```json
{
  "versions": [
    {
      "version": "2.50.0",
      "date": "2026-02-01T10:30:00Z",
      "notes": "Auto-update support, improved performance",
      "download_url": "https://github.com/CCoupel/BuzzMaster/releases/download/v2.50.0/buzzcontrol-v2.50.0-windows-amd64.exe",
      "current": true,
      "size": 45678900
    },
    {
      "version": "2.49.0",
      "date": "2026-01-31T14:20:00Z",
      "notes": "Player cards neutral gray styling",
      "download_url": "https://github.com/CCoupel/BuzzMaster/releases/download/v2.49.0/buzzcontrol-v2.49.0-windows-amd64.exe",
      "current": false,
      "size": 45234567
    }
  ]
}
```

**Cache** : 1 hour
**Platform Detection** : Automatic (windows-amd64, linux-arm64)
**Error Scenarios** :
- GitHub unreachable → Returns cached data or empty list
- Malformed response → Returns empty list

---

### 2. GET /api/updates/check

**Purpose** : Quick version check (for navbar badge)

**Request** :
```
GET http://localhost/api/updates/check
```

**Response 200** (no update) :
```json
{
  "update_available": false,
  "current": "2.50.0",
  "latest": "2.50.0",
  "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
}
```

**Response 200** (update available) :
```json
{
  "update_available": true,
  "current": "2.49.0",
  "latest": "2.50.0",
  "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
}
```

**Cache** : 1 hour
**Use Case** : Call on page load to show notification badge

---

### 3. POST /api/updates/download

**Purpose** : Download a specific version binary

**Request** :
```
POST http://localhost/api/updates/download
Content-Type: application/json

{
  "version": "2.50.0"
}
```

**Response 200** (success) :
```json
{
  "success": true,
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-updates/buzzcontrol-v2.50.0-windows-amd64.exe",
  "size": 45678900,
  "checksum": "abc123def456"
}
```

**Response 400** (version not found) :
```json
{
  "success": false,
  "error": "Version not found or invalid platform"
}
```

**Response 503** (GitHub unreachable) :
```json
{
  "success": false,
  "error": "GitHub API unavailable"
}
```

**Validation** :
- Version must exist on GitHub
- Platform must match current system
- File size must be >= 40MB (minimum threshold)

**File Path** : Temporary directory, valid only during that session

---

### 4. POST /api/updates/apply

**Purpose** : Apply the downloaded update and restart the server

**Request** :
```
POST http://localhost/api/updates/apply
Content-Type: application/json

{
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-updates/buzzcontrol-v2.50.0-windows-amd64.exe"
}
```

**Response 200** (success, restart initiated) :
```json
{
  "success": true,
  "message": "Server restarting with version 2.50.0...",
  "restart_in_seconds": 3
}
```

**Response 400** (validation error) :
```json
{
  "success": false,
  "error": "Invalid path or version mismatch"
}
```

**Response 500** (file operation error) :
```json
{
  "success": false,
  "error": "Failed to backup or replace executable"
}
```

**What Happens** :
1. Backend saves current game state (if game in progress)
2. Closes all WebSocket connections gracefully
3. Creates backup of old executable (.bak or .old)
4. Replaces executable with new binary
5. Starts new server process
6. Monitors for 5 seconds
7. On success : removes backup, server stays running
8. On failure : auto-rollback to old binary

**Important** : After this endpoint returns 200, the server **will restart**. Frontend **must poll** /api/updates/check to detect when server is back online.

---

## Configuration

### auto_check_updates

**Location** : `server-go/config.json`

**Default Value** : `true`

**Schema** :
```json
{
  "server": {
    "auto_check_updates": true
  }
}
```

**Type** : Boolean
**Purpose** : Enable/disable automatic update checks on server startup

---

## Files Created (Backend)

1. `internal/server/updater.go` - Main update logic
2. `internal/server/github_client.go` - GitHub API client with caching
3. `internal/server/github_client_test.go` - Tests for GitHub client
4. `internal/server/updater_test.go` - Tests for update handlers

---

## Files Modified (Backend)

1. `cmd/server/main.go` - Added 4 new route handlers
2. `internal/config/config.go` - Added auto_check_updates field
3. `internal/server/server.go` - Registered update routes
4. `internal/game/engine.go` - Added SaveState/RestoreState methods

---

## Testing

All backend tests pass:
```bash
cd server-go
go test ./... -v -cover
```

**Test Coverage** :
- GitHub API client (mocked responses)
- Version parsing and comparison
- Endpoint validation
- Error handling
- File operations
- Platform detection

---

## Security & Safety

### Backup Strategy
- Old executable backed up before any replacement
- Windows: `.bak` extension
- Linux: `.old` extension
- Backup removed on successful startup

### Rollback Mechanism
- Automatic if new binary fails within 5 seconds
- Old binary restored from backup
- Error logged for debugging

### Integrity Verification
- Downloaded file size checked (>= 40MB)
- Binary tested for execution
- Checksum provided in response (optional verification)

### State Preservation
- Game state saved before shutdown
- Connections closed gracefully
- State restored after restart

---

## Frontend Integration Points

### 1. Navbar Badge
Frontend should:
- Call GET /api/updates/check on page load
- Show badge if `update_available == true`
- Make badge clickable link to update page

### 2. Update Page
Frontend should:
- Call GET /api/updates to populate version list
- Let user select version
- Call POST /api/updates/download for each version
- Show download progress
- Call POST /api/updates/apply when user confirms
- Poll GET /api/updates/check every 2 seconds after apply
- Auto-reload page when server back online

### 3. Error Handling
Frontend should handle:
- GitHub API unavailable (503) → show friendly message
- Version not found (400) → disable version in dropdown
- Download failed → show error with retry option
- Server restart timeout → show timeout message with manual reload

### 4. User Experience
- Show loading states during API calls
- Display download progress
- Show confirmation if game in progress
- Display spinner during restart
- Auto-reload when server back
- Show new version after reload

---

## API Performance

### Caching
- GET /api/updates : cached 1 hour
- GET /api/updates/check : cached 1 hour
- Same cache used by both endpoints

### GitHub Rate Limiting
- 60 requests/hour limit (unauthenticated)
- 1-hour cache prevents hitting limit
- Cache is per-server instance

### Network Timeouts
- GitHub API call : 10 seconds timeout
- Download : Default fetch timeout (usually 30s)
- Restart polling : 2 seconds × 30 attempts = 60 seconds max

---

## Known Limitations

1. **No Live Download Progress** : POST /api/updates/download doesn't stream progress. Frontend polls will show either 0% or 100%.

2. **No Webhook Support** : No GitHub webhook integration. Frontend must poll /api/updates/check for updates.

3. **No Version History** : Only latest N versions shown from GitHub. Older releases may not be listed.

4. **No Scheduled Updates** : No automatic scheduled update downloads. Always manual via frontend.

---

## Future Enhancements (v2.51.0+)

- [ ] GitHub webhook for immediate update notifications
- [ ] Streaming download progress via WebSocket
- [ ] Scheduled/automatic updates
- [ ] Checksum verification (SHA256)
- [ ] Pre-release version handling
- [ ] Update rollback via API (restore from backup)

---

## Summary for Frontend Developer

**Ready to use endpoints:**
- GET /api/updates (list versions, cached)
- GET /api/updates/check (quick check, cached)
- POST /api/updates/download (download binary)
- POST /api/updates/apply (apply and restart)

**Configuration:**
- auto_check_updates option (boolean, default true)

**Safety:**
- Auto-backup and rollback
- State preservation
- Graceful restart

**Testing:**
- All backend tests passing
- Ready for frontend integration

**Next Steps:**
- Frontend: Implement UpdatePage component
- Frontend: Add Navbar badge
- Frontend: Create useUpdates hook
- Frontend: Add route to UpdatePage
- QA: End-to-end testing

---

## Contact

For questions about backend implementation:
- See backend code: `internal/server/updater.go`
- See tests: `internal/server/updater_test.go`
- See contracts: `contracts/auto-update-endpoints.md`

