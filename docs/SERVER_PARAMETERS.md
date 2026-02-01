# Server Parameters Configuration

## Overview

The Server Parameters section in the Configuration page allows administrators to control server behavior and debugging options. These parameters are stored in `config.json` and are loaded on server startup.

## Available Parameters

### auto_open_browsers

**Type**: Boolean
**Default**: `false`
**Scope**: Server startup behavior

When enabled, the server will automatically open web browsers to the admin interface on startup. This is useful for automated deployments or presentations where you want the UI to appear immediately.

**Usage**:
- Toggle the checkbox "Ouvrir les navigateurs automatiquement" in ConfigPage
- Click "Enregistrer" to save
- Changes take effect on server restart

**Example Config**:
```json
{
  "server": {
    "auto_open_browsers": true,
    ...
  }
}
```

### debug

**Type**: Boolean
**Default**: `false`
**Scope**: Server logging and debug endpoints

When enabled, the server enters debug mode which provides additional logging and exposes debug endpoints (e.g., `/logs`). This is valuable for troubleshooting and monitoring server behavior.

**Usage**:
- Toggle the checkbox "Mode debug" in ConfigPage
- Click "Enregistrer" to save
- Debug logging is active immediately (depends on implementation)

**Example Config**:
```json
{
  "server": {
    "debug": true,
    ...
  }
}
```

## Configuration Interface

### Location
- **Page**: Administration → Configuration
- **Section**: "Parametres serveur"
- **Position**: After "Système" section, before "Mode Demo"

### Layout
```
Parametres serveur
Configuration du comportement du serveur au demarrage et mode de fonctionnement.

☐ Ouvrir les navigateurs automatiquement
☐ Mode debug

[Enregistrer]
```

## How It Works

### Loading Parameters

When the Configuration page loads:

1. A GET request is made to `/config.json`
2. Server responds with current configuration
3. `serverParams` state is populated from `data.server`
4. Toggles display the current parameter values

**Network Request**:
```http
GET /config.json HTTP/1.1
Accept: application/json
```

**Response**:
```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 1234,
    "auto_open_browsers": false,
    "debug": false
  },
  "neon_effect": { ... },
  ...
}
```

### Saving Parameters

When the user toggles parameters and clicks "Enregistrer":

1. The button enters loading state
2. A POST request is sent to `/config.json` with updated values
3. Server persists the changes to `config.json`
4. Toggles remain in their updated state
5. On next server restart, new values are loaded

**Network Request**:
```http
POST /config.json HTTP/1.1
Content-Type: application/json

{
  "server": {
    "auto_open_browsers": true,
    "debug": true
  }
}
```

### Persistence

Parameters are saved to `server-go/config.json` in the following structure:

```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 1234,
    "websocket_path": "/ws",
    "auto_open_browsers": false,
    "debug": false
  }
}
```

## Backend Implementation

### Config Structure

See `server-go/internal/config/config.go`:

```go
type ServerConfig struct {
	HTTPPort         int    `json:"http_port"`
	TCPPort          int    `json:"tcp_port"`
	WebSocketPath    string `json:"websocket_path"`
	AutoOpenBrowsers bool   `json:"auto_open_browsers"`
	Debug            bool   `json:"debug"`
}
```

### Endpoint Handler

The `/config.json` endpoint:
- **Method**: POST
- **Content-Type**: application/json
- **Behavior**: Merges partial updates into existing config.json
- **Response**: HTTP 200 on success, HTTP 400+ on error

## Frontend Implementation

### React Component

See `server-go/web/src/pages/ConfigPage.jsx`:

**State Management**:
```javascript
const [serverParams, setServerParams] = useState({
  auto_open_browsers: false,
  debug: false
})
const [savingParams, setSavingParams] = useState(false)
```

**Loading**:
```javascript
useEffect(() => {
  const fetchConfig = async () => {
    const response = await fetch('/config.json')
    if (response.ok) {
      const data = await response.json()
      if (data.server) {
        setServerParams({
          auto_open_browsers: data.server.auto_open_browsers || false,
          debug: data.server.debug || false
        })
      }
    }
  }
  fetchConfig()
}, [])
```

**Saving**:
```javascript
const handleSaveServerParams = async () => {
  setSavingParams(true)
  try {
    const response = await fetch('/config.json', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        server: {
          auto_open_browsers: serverParams.auto_open_browsers,
          debug: serverParams.debug
        }
      })
    })
    if (!response.ok) {
      alert('Erreur: ' + text)
    }
  } catch (error) {
    alert('Erreur: ' + error.message)
  } finally {
    setSavingParams(false)
  }
}
```

## Usage Examples

### Example 1: Enable Auto-Open Browsers

**Scenario**: You want browsers to automatically open when the server starts.

**Steps**:
1. Go to Admin → Configuration
2. Check the box "Ouvrir les navigateurs automatiquement"
3. Click "Enregistrer"
4. Restart the server
5. On restart, the default web browsers will open to the admin interface

### Example 2: Enable Debug Mode

**Scenario**: You're troubleshooting an issue and want detailed logging.

**Steps**:
1. Go to Admin → Configuration
2. Check the box "Mode debug"
3. Click "Enregistrer"
4. Debug logging is active immediately
5. Access debug endpoints if available (e.g., `/logs`)

### Example 3: Disable Debug After Troubleshooting

**Steps**:
1. Go to Admin → Configuration
2. Uncheck the box "Mode debug"
3. Click "Enregistrer"
4. Debug logging is deactivated

## Testing

### Unit Tests
- `server-go/web/src/pages/ConfigPage.test.jsx`
  - Tests for parameter loading
  - Tests for toggle state changes
  - Tests for save functionality

### E2E Tests
- `tests/e2e/config-server-params.md`
  - Full scenario testing with manual validation steps
  - 9 comprehensive test scenarios

### QA Report
- `tests/QA_CONFIG_SERVER_PARAMS_v249.md`
  - Complete test execution results
  - All 9 scenarios validated
  - No blocking issues found

## Troubleshooting

### Parameters not saving

**Problem**: Click "Enregistrer" but changes don't persist

**Solution**:
1. Check browser console for network errors
2. Verify server is running (`http://localhost/`)
3. Check network tab in DevTools to see POST response
4. Verify `config.json` file is writable by the server process
5. Check server logs for error messages

### Changes don't take effect

**Problem**: Toggled a parameter and saved, but behavior didn't change

**Solution**:
- Some parameters (like `auto_open_browsers`) require a server restart
- Restart the server process for changes to take effect
- Some parameters (like `debug`) may take effect immediately

### Loading parameters shows old values

**Problem**: Configuration page shows outdated values

**Solution**:
1. Hard refresh the browser (Ctrl+Shift+R / Cmd+Shift+R)
2. Clear browser cache if necessary
3. Verify `config.json` contains the correct values
4. Check if another browser/instance changed the config

## Future Enhancements

Potential improvements for future versions:

1. **Success Notification**: Add toast notification on successful save instead of silent success
2. **Parameter Hints**: Add tooltips explaining each parameter's effect
3. **Restart Prompt**: Notify when restart is needed for changes to take effect
4. **Reset Button**: Add button to reset parameters to defaults
5. **Parameter Groups**: Organize additional parameters into logical groups
6. **Real-time Effect**: Some parameters could take effect immediately without restart

## Related Documentation

- [CLAUDE.md](../CLAUDE.md) - Project reference
- [GO_SERVER.md](GO_SERVER.md) - Server implementation details
- [REACT_INTERFACE.md](REACT_INTERFACE.md) - Frontend architecture
- [ADMIN_GUIDE.md](ADMIN_GUIDE.md) - Administrator user guide

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 2.49.0 | 2026-02-01 | Initial release: auto_open_browsers and debug parameters |

---

**Last Updated**: 2026-02-01
**Author**: CDP Agent
**Status**: Published
