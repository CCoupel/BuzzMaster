# Endpoints Auto-Update (v2.50.0)

> **Base URL** : `http://localhost` (port 80)
> **Version** : 2.50.0
> **Date** : 2026-02-01
> **Status** : Draft (awaiting implementation planning)

---

## Update Management

### GET /api/updates

List all available versions from GitHub releases for current platform.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Response  | application/json |
| Cache     | 1 heure |

#### Response 200

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

---

### GET /api/updates/check

Quick check for available updates.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Response  | application/json |
| Cache     | 1 heure |

#### Response 200

```json
{
  "update_available": false,
  "current": "2.50.0",
  "latest": "2.50.0",
  "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
}
```

#### Response 200 (with update)

```json
{
  "update_available": true,
  "current": "2.49.0",
  "latest": "2.50.0",
  "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
}
```

---

### POST /api/updates/download

Download a specific version binary.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Content-Type | application/json |

#### Request Body

```json
{
  "version": "2.50.0"
}
```

#### Response 200

```json
{
  "success": true,
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-v2.50.0-windows-amd64.exe",
  "size": 45678900,
  "checksum": "abc123def456"
}
```

#### Response 400

```json
{
  "success": false,
  "error": "Version not found or invalid platform"
}
```

#### Response 503

```json
{
  "success": false,
  "error": "GitHub API unavailable"
}
```

---

### POST /api/updates/apply

Apply the downloaded update and restart server.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Content-Type | application/json |

#### Request Body

```json
{
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-v2.50.0-windows-amd64.exe"
}
```

#### Response 200

```json
{
  "success": true,
  "message": "Server restarting with version 2.50.0...",
  "restart_in_seconds": 3
}
```

#### Response 400

```json
{
  "success": false,
  "error": "Invalid path or version mismatch"
}
```

#### Response 500

```json
{
  "success": false,
  "error": "Failed to backup or replace executable"
}
```

---

## Config Option

### auto_check_updates (config.json)

```json
{
  "server": {
    "auto_check_updates": true
  }
}
```

- Type: `bool`
- Default: `true`
- Description: Check for updates on server startup
- Backend: Triggers background check if enabled

---

## Notes

### Platform Detection
The server automatically detects platform using:
- `runtime.GOOS` (windows, linux)
- `runtime.GOARCH` (amd64, arm64)

Asset name format: `buzzcontrol-vX.Y.Z-{os}-{arch}.(exe|binary)`

### GitHub Rate Limiting
- 60 requests/hour without authentication
- All endpoints cache responses for 1 hour to avoid hitting limits
- Cache invalidation: manual `/api/updates/check` call forces fresh fetch

### Security
- Downloaded binary verified for minimum size (> 40MB)
- Binary tested for execution before applying
- Old executable backed up before replacement (Windows: `.bak`, Linux: `.old`)
- Automatic rollback if new server fails to start within 5 seconds

### Restart Behavior
1. Save game state (if game in progress)
2. Close all WebSocket connections gracefully
3. Rename old executable to `.bak`/`.old`
4. Move new executable to active location
5. Start new executable
6. On successful startup: remove backup
7. On failure: restore old executable from backup

---

## Webhook (Optional - v2.51.0)

Future: GitHub webhook for immediate notifications on new releases (requires public endpoint).

