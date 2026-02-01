# Dev Frontend - Auto-Update Implementation Spec (v2.50.0)

**Date** : 2026-02-01
**Feature** : Auto-Update Server
**Agent** : dev-frontend
**Branch** : feature/auto-update
**Phase** : 3 (Frontend UI)

---

## Overview

Implement frontend interface for auto-update feature:
1. **UpdatePage.jsx** - Complete update management UI
2. **Navbar.jsx** - Notification badge for available updates
3. **useUpdates.js** - Hook for backend API integration
4. **Restart handling** - Graceful server restart and reconnection

---

## Backend API Reference (COMPLETED)

The backend has completed all 4 endpoints. Frontend will consume:

### GET /api/updates

**Response** :
```json
{
  "versions": [
    {
      "version": "2.50.0",
      "date": "2026-02-01T10:30:00Z",
      "notes": "Auto-update support, improved performance",
      "download_url": "https://github.com/CCoupel/BuzzMaster/releases/download/v2.50.0/...",
      "current": true,
      "size": 45678900
    },
    {
      "version": "2.49.0",
      "date": "2026-01-31T14:20:00Z",
      "notes": "Player cards neutral gray styling",
      "download_url": "https://github.com/...",
      "current": false,
      "size": 45234567
    }
  ]
}
```

### GET /api/updates/check

**Response** :
```json
{
  "update_available": true,
  "current": "2.49.0",
  "latest": "2.50.0",
  "release_url": "https://github.com/CCoupel/BuzzMaster/releases/tag/v2.50.0"
}
```

### POST /api/updates/download

**Request** :
```json
{
  "version": "2.50.0"
}
```

**Response** :
```json
{
  "success": true,
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-v2.50.0-windows-amd64.exe",
  "size": 45678900,
  "checksum": "abc123def456"
}
```

### POST /api/updates/apply

**Request** :
```json
{
  "version": "2.50.0",
  "path": "/tmp/buzzcontrol-v2.50.0-windows-amd64.exe"
}
```

**Response** :
```json
{
  "success": true,
  "message": "Server restarting with version 2.50.0...",
  "restart_in_seconds": 3
}
```

---

## 1. Hook : useUpdates.js

**Location** : `server-go/web/src/hooks/useUpdates.js`

### Purpose

Custom React hook to manage all update-related API calls and state.

### State Management

```javascript
const [updates, setUpdates] = useState({
  available: [],           // List of all available versions
  current: null,           // Current version (string)
  latest: null,            // Latest version (string)
  updateAvailable: false,  // Boolean: is update available?
  downloadProgress: 0,     // Download progress (0-100)
  isDownloading: false,    // Download in progress?
  isApplying: false,       // Apply/restart in progress?
  downloadedPath: null,    // Path of downloaded binary
  error: null,             // Error message (if any)
  loading: false,          // Initial data loading?
});
```

### Functions

#### checkForUpdates()
```javascript
const checkForUpdates = async () => {
  try {
    setUpdates(prev => ({ ...prev, loading: true, error: null }));

    const response = await fetch('/api/updates/check');
    const data = await response.json();

    if (!response.ok) throw new Error(data.error);

    setUpdates(prev => ({
      ...prev,
      current: data.current,
      latest: data.latest,
      updateAvailable: data.update_available,
      loading: false,
    }));

    return data;
  } catch (err) {
    setUpdates(prev => ({
      ...prev,
      error: err.message,
      loading: false,
    }));
    throw err;
  }
};
```

#### listAllUpdates()
```javascript
const listAllUpdates = async () => {
  try {
    setUpdates(prev => ({ ...prev, loading: true, error: null }));

    const response = await fetch('/api/updates');
    const data = await response.json();

    if (!response.ok) throw new Error(data.error);

    const current = data.versions.find(v => v.current);

    setUpdates(prev => ({
      ...prev,
      available: data.versions,
      current: current?.version || prev.current,
      loading: false,
    }));

    return data.versions;
  } catch (err) {
    setUpdates(prev => ({
      ...prev,
      error: err.message,
      loading: false,
    }));
    throw err;
  }
};
```

#### downloadUpdate(version)
```javascript
const downloadUpdate = async (version) => {
  try {
    setUpdates(prev => ({
      ...prev,
      isDownloading: true,
      error: null,
      downloadProgress: 0,
    }));

    const response = await fetch('/api/updates/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version }),
    });

    const data = await response.json();

    if (!response.ok) throw new Error(data.error);

    setUpdates(prev => ({
      ...prev,
      downloadedPath: data.path,
      downloadProgress: 100,
      isDownloading: false,
    }));

    return data;
  } catch (err) {
    setUpdates(prev => ({
      ...prev,
      error: err.message,
      isDownloading: false,
    }));
    throw err;
  }
};
```

#### applyUpdate(version, path)
```javascript
const applyUpdate = async (version, path) => {
  try {
    setUpdates(prev => ({
      ...prev,
      isApplying: true,
      error: null,
    }));

    const response = await fetch('/api/updates/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version, path }),
    });

    const data = await response.json();

    if (!response.ok) throw new Error(data.error);

    // Server is restarting, start polling for reconnection
    await waitForServerRestart();

    setUpdates(prev => ({
      ...prev,
      isApplying: false,
      current: version,
      updateAvailable: false,
    }));

    return data;
  } catch (err) {
    setUpdates(prev => ({
      ...prev,
      error: err.message,
      isApplying: false,
    }));
    throw err;
  }
};
```

#### waitForServerRestart()
```javascript
const waitForServerRestart = async (maxAttempts = 30, interval = 2000) => {
  // Poll /api/updates/check every 2 seconds
  // Max 30 attempts = 60 seconds
  // Return when server is back online

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    try {
      await new Promise(resolve => setTimeout(resolve, interval));
      const response = await fetch('/api/updates/check');

      if (response.ok) {
        console.log('Server back online after restart');
        return true;
      }
    } catch (err) {
      // Server still restarting
      continue;
    }
  }

  throw new Error('Server did not restart in time');
};
```

#### reset()
```javascript
const reset = () => {
  setUpdates({
    available: [],
    current: null,
    latest: null,
    updateAvailable: false,
    downloadProgress: 0,
    isDownloading: false,
    isApplying: false,
    downloadedPath: null,
    error: null,
    loading: false,
  });
};
```

### Hook Return

```javascript
return {
  // State
  updates,
  current: updates.current,
  latest: updates.latest,
  updateAvailable: updates.updateAvailable,
  isDownloading: updates.isDownloading,
  isApplying: updates.isApplying,
  downloadProgress: updates.downloadProgress,
  error: updates.error,
  loading: updates.loading,

  // Actions
  checkForUpdates,
  listAllUpdates,
  downloadUpdate,
  applyUpdate,
  reset,
};
```

### Usage Example

```javascript
const UpdatePage = () => {
  const {
    updates,
    updateAvailable,
    listAllUpdates,
    downloadUpdate,
    applyUpdate,
  } = useUpdates();

  useEffect(() => {
    listAllUpdates();
  }, []);

  const handleDownload = async (version) => {
    await downloadUpdate(version);
  };

  return (
    // Component JSX
  );
};
```

---

## 2. Component : UpdatePage.jsx

**Location** : `server-go/web/src/pages/UpdatePage.jsx`

### Purpose

Complete update management interface. Can be standalone page or integrated into existing ConfigPage.

### Layout & Sections

#### Section 1 : Current Version Status

```jsx
<div className="update-status">
  <h2>Version</h2>
  <div className="version-info">
    <div className="version-current">
      <label>Version actuelle</label>
      <span className="version-number">{current}</span>
    </div>
    <div className="version-latest">
      <label>Dernière version</label>
      <span className="version-number">{latest}</span>
    </div>
  </div>

  {updateAvailable && (
    <div className="alert alert-info">
      Une nouvelle version est disponible !
    </div>
  )}
</div>
```

#### Section 2 : Available Versions List

```jsx
<div className="versions-list">
  <h2>Versions disponibles</h2>

  <select
    value={selectedVersion}
    onChange={(e) => setSelectedVersion(e.target.value)}
    disabled={isDownloading || isApplying}
  >
    <option value="">-- Sélectionner une version --</option>
    {available.map(v => (
      <option key={v.version} value={v.version}>
        v{v.version} {v.current ? '(actuelle)' : ''}
      </option>
    ))}
  </select>

  {selectedVersion && (
    <div className="version-details">
      <div className="release-notes">
        <h3>Notes de release</h3>
        <p>{releaseNotes}</p>
      </div>
      <div className="version-meta">
        <span>Taille : {formatBytes(versionSize)}</span>
        <span>Date : {releaseDate}</span>
      </div>
    </div>
  )}
</div>
```

#### Section 3 : Download Progress

```jsx
{downloadedPath && (
  <div className="download-status">
    <div className="status-icon">✓</div>
    <span>Téléchargement complété</span>
    <button
      onClick={() => setDownloadedPath(null)}
      disabled={isApplying}
    >
      Télécharger une autre version
    </button>
  </div>
)}

{isDownloading && (
  <div className="download-progress">
    <div className="progress-bar">
      <div
        className="progress-fill"
        style={{ width: `${downloadProgress}%` }}
      />
    </div>
    <span>{downloadProgress}% - Téléchargement en cours...</span>
    <button
      onClick={handleCancel}
      className="btn-cancel"
    >
      Annuler
    </button>
  </div>
)}
```

#### Section 4 : Action Buttons

```jsx
<div className="update-actions">
  {!downloadedPath && !isDownloading && (
    <button
      onClick={() => handleDownload(selectedVersion)}
      disabled={!selectedVersion || updateAvailable === false}
      className="btn btn-primary"
    >
      Télécharger v{selectedVersion}
    </button>
  )}

  {downloadedPath && (
    <button
      onClick={() => handleApply(selectedVersion, downloadedPath)}
      disabled={isApplying}
      className="btn btn-danger"
    >
      {isApplying ? 'Redémarrage en cours...' : 'Appliquer et redémarrer'}
    </button>
  )}

  {isApplying && (
    <div className="restart-message">
      <div className="spinner" />
      <p>Le serveur redémarre avec la version {selectedVersion}...</p>
      <p className="info">Cette page sera rechargée automatiquement.</p>
    </div>
  )}
</div>
```

#### Section 5 : Warnings

```jsx
{gameInProgress && (
  <div className="alert alert-warning">
    ⚠️ Une partie est en cours. La redémarrer interrompra le jeu.
    Êtes-vous sûr ?
  </div>
)}

{error && (
  <div className="alert alert-error">
    Erreur : {error}
  </div>
)}
```

### Component Full Structure

```jsx
import React, { useState, useEffect } from 'react';
import useUpdates from '../hooks/useUpdates';
import './UpdatePage.css';

const UpdatePage = () => {
  const {
    updates,
    updateAvailable,
    isDownloading,
    isApplying,
    downloadProgress,
    error,
    loading,
    listAllUpdates,
    downloadUpdate,
    applyUpdate,
  } = useUpdates();

  const [selectedVersion, setSelectedVersion] = useState('');
  const [downloadedPath, setDownloadedPath] = useState(null);
  const [gameInProgress, setGameInProgress] = useState(false);

  useEffect(() => {
    // Load available versions on mount
    listAllUpdates();

    // Check if game is in progress (query /api/game-state or similar)
    checkGameStatus();
  }, []);

  const handleDownload = async (version) => {
    try {
      const data = await downloadUpdate(version);
      setDownloadedPath(data.path);
    } catch (err) {
      console.error('Download failed:', err);
    }
  };

  const handleApply = async (version, path) => {
    if (gameInProgress && !window.confirm(
      'Une partie est en cours. La redémarrer la terminera. Continuer ?'
    )) {
      return;
    }

    try {
      await applyUpdate(version, path);
      // After restart, page will auto-reload when server is back
      // Show "Reconnecting..." message
      setTimeout(() => {
        window.location.reload();
      }, 5000);
    } catch (err) {
      console.error('Apply failed:', err);
    }
  };

  const checkGameStatus = async () => {
    // Implementation depends on existing game-state API
    // setGameInProgress(...)
  };

  const selectedVersionInfo = updates.available?.find(
    v => v.version === selectedVersion
  );

  if (loading) {
    return <div className="loading">Chargement...</div>;
  }

  return (
    <div className="update-page">
      <h1>Gestion des mises à jour</h1>

      {/* Section 1: Current Status */}
      <div className="update-section">
        <h2>Version actuelle</h2>
        <div className="version-display">
          <div className="version-card">
            <span className="label">Actuelle</span>
            <span className="version">{updates.current}</span>
          </div>
          <div className="version-card">
            <span className="label">Dernière</span>
            <span className="version">{updates.latest}</span>
          </div>
        </div>

        {updateAvailable && (
          <div className="alert alert-info">
            ✓ Nouvelle version disponible !
          </div>
        )}
      </div>

      {/* Section 2: Version Selection */}
      <div className="update-section">
        <h2>Sélectionner une version</h2>
        <select
          value={selectedVersion}
          onChange={(e) => setSelectedVersion(e.target.value)}
          disabled={isDownloading || isApplying}
          className="version-select"
        >
          <option value="">-- Choisir une version --</option>
          {updates.available?.map(v => (
            <option key={v.version} value={v.version}>
              v{v.version} {v.current ? '(actuelle)' : ''}
            </option>
          ))}
        </select>

        {selectedVersionInfo && (
          <div className="version-details">
            <div className="notes">
              <h3>Notes de release</h3>
              <p>{selectedVersionInfo.notes}</p>
            </div>
            <div className="meta">
              <div>
                <strong>Taille :</strong> {(selectedVersionInfo.size / 1024 / 1024).toFixed(1)} MB
              </div>
              <div>
                <strong>Date :</strong> {new Date(selectedVersionInfo.date).toLocaleDateString()}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Section 3: Download Status */}
      {isDownloading && (
        <div className="update-section">
          <h2>Téléchargement</h2>
          <div className="progress">
            <div className="progress-bar">
              <div
                className="progress-fill"
                style={{ width: `${downloadProgress}%` }}
              />
            </div>
            <span>{downloadProgress}% en cours...</span>
          </div>
        </div>
      )}

      {downloadedPath && !isDownloading && (
        <div className="update-section">
          <div className="alert alert-success">
            ✓ Téléchargement complété et vérifié
          </div>
          <button
            onClick={() => setDownloadedPath(null)}
            disabled={isApplying}
            className="btn btn-secondary"
          >
            Télécharger une autre version
          </button>
        </div>
      )}

      {/* Section 4: Actions */}
      <div className="update-section actions">
        {!downloadedPath && !isDownloading && (
          <button
            onClick={() => handleDownload(selectedVersion)}
            disabled={!selectedVersion}
            className="btn btn-primary btn-lg"
          >
            Télécharger v{selectedVersion}
          </button>
        )}

        {downloadedPath && !isApplying && (
          <button
            onClick={() => handleApply(selectedVersion, downloadedPath)}
            className="btn btn-danger btn-lg"
          >
            Appliquer et redémarrer
          </button>
        )}

        {isApplying && (
          <div className="restart-status">
            <div className="spinner" />
            <h3>Redémarrage du serveur...</h3>
            <p>Reconnexion automatique en cours</p>
          </div>
        )}
      </div>

      {/* Section 5: Warnings */}
      {gameInProgress && (
        <div className="alert alert-warning">
          ⚠️ Une partie est actuellement en cours.
          La redémarrer le serveur l'interrompra.
        </div>
      )}

      {error && (
        <div className="alert alert-error">
          Erreur : {error}
        </div>
      )}
    </div>
  );
};

export default UpdatePage;
```

### CSS : UpdatePage.css

```css
.update-page {
  max-width: 600px;
  margin: 0 auto;
  padding: 20px;
}

.update-section {
  background: #f5f5f5;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
  border-left: 4px solid #007bff;
}

.version-display {
  display: flex;
  gap: 20px;
  margin: 15px 0;
}

.version-card {
  flex: 1;
  background: white;
  border-radius: 6px;
  padding: 15px;
  text-align: center;
  border: 1px solid #ddd;
}

.version-card .label {
  display: block;
  font-size: 12px;
  color: #666;
  margin-bottom: 5px;
  text-transform: uppercase;
}

.version-card .version {
  display: block;
  font-size: 24px;
  font-weight: bold;
  color: #333;
  font-family: monospace;
}

.version-select {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  margin-bottom: 15px;
}

.version-details {
  background: white;
  border-radius: 6px;
  padding: 15px;
  margin-top: 15px;
}

.version-details .notes {
  margin-bottom: 15px;
}

.version-details .notes h3 {
  margin: 0 0 10px 0;
  font-size: 14px;
  color: #333;
}

.version-details .notes p {
  margin: 0;
  font-size: 13px;
  color: #666;
  line-height: 1.5;
}

.version-details .meta {
  font-size: 12px;
  color: #666;
}

.version-details .meta div {
  margin-bottom: 5px;
}

.progress {
  margin: 15px 0;
}

.progress-bar {
  height: 8px;
  background: #e0e0e0;
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 10px;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #007bff, #0056b3);
  transition: width 0.3s ease;
}

.actions {
  text-align: center;
}

.btn {
  padding: 10px 20px;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.3s ease;
  margin: 5px;
}

.btn-primary {
  background: #007bff;
  color: white;
}

.btn-primary:hover {
  background: #0056b3;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover {
  background: #c82333;
}

.btn-lg {
  padding: 12px 30px;
  font-size: 16px;
  font-weight: bold;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.alert {
  padding: 12px;
  border-radius: 4px;
  margin-bottom: 15px;
  font-size: 14px;
}

.alert-info {
  background: #d1ecf1;
  color: #0c5460;
  border-left: 4px solid #17a2b8;
}

.alert-warning {
  background: #fff3cd;
  color: #856404;
  border-left: 4px solid #ffc107;
}

.alert-error {
  background: #f8d7da;
  color: #721c24;
  border-left: 4px solid #dc3545;
}

.alert-success {
  background: #d4edda;
  color: #155724;
  border-left: 4px solid #28a745;
}

.restart-status {
  text-align: center;
  padding: 30px;
}

.spinner {
  display: inline-block;
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #007bff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 15px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.loading {
  text-align: center;
  padding: 40px;
  color: #666;
}
```

---

## 3. Component : Navbar Badge

**Location** : `server-go/web/src/components/Navbar.jsx` (modify existing)

### Changes Required

Add notification badge that shows when update is available:

```jsx
import useUpdates from '../hooks/useUpdates';

const Navbar = () => {
  const { updateAvailable, checkForUpdates } = useUpdates();

  useEffect(() => {
    // Check for updates on page load
    checkForUpdates();

    // Optional: Check periodically (every hour)
    const interval = setInterval(checkForUpdates, 60 * 60 * 1000);

    return () => clearInterval(interval);
  }, []);

  return (
    <nav className="navbar">
      {/* ... existing navbar content ... */}

      {/* Add update badge */}
      {updateAvailable && (
        <div className="update-badge">
          <div className="badge-dot" />
          <span className="badge-text">Mise à jour disponible</span>
          <a href="/admin/updates">Mettre à jour</a>
        </div>
      )}

      {/* ... rest of navbar ... */}
    </nav>
  );
};
```

### CSS for Badge

```css
.update-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #ffc107;
  color: #333;
  padding: 8px 12px;
  border-radius: 4px;
  margin-left: auto;
  font-size: 13px;
}

.badge-dot {
  width: 8px;
  height: 8px;
  background: #dc3545;
  border-radius: 50%;
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.badge-text {
  font-weight: 500;
}

.update-badge a {
  color: #0056b3;
  text-decoration: none;
  margin-left: 5px;
  font-weight: 600;
}

.update-badge a:hover {
  text-decoration: underline;
}
```

---

## 4. Routing

### Modify App.jsx

Add route for UpdatePage:

```jsx
import UpdatePage from './pages/UpdatePage';

// In router configuration:
{
  path: '/admin/updates',
  element: <UpdatePage />,
}

// Or add to menu if using navigation structure
```

---

## 5. Component Integration Workflow

### User Workflow

```
1. User opens admin interface
   ↓
2. Navbar hook checks /api/updates/check
   ↓
3. If updateAvailable=true, show badge
   ↓
4. User clicks badge → navigate to UpdatePage
   ↓
5. UpdatePage loads /api/updates
   ↓
6. User selects version and clicks "Download"
   ↓
7. Frontend calls POST /api/updates/download
   ↓
8. Download progress displayed
   ↓
9. When complete, show "Apply" button
   ↓
10. User clicks "Apply" (or cancelled)
    ↓
11. Frontend calls POST /api/updates/apply
    ↓
12. Backend restarts server
    ↓
13. Frontend polls /api/updates/check every 2s
    ↓
14. When server back, auto-reload page
    ↓
15. New version active!
```

---

## 6. Error Handling

Handle all failure modes gracefully:

```javascript
// In components
if (error) {
  return (
    <div className="alert alert-error">
      Erreur lors de la vérification des mises à jour : {error}
      <button onClick={() => retry()}>Réessayer</button>
    </div>
  );
}

// Network failures
try {
  await downloadUpdate(version);
} catch (err) {
  if (err.message === 'GitHub API unavailable') {
    showMessage('GitHub temporairement indisponible. Réessayez plus tard.');
  } else if (err.message.includes('timeout')) {
    showMessage('Téléchargement expiré. Réessayez.');
  } else {
    showMessage(`Erreur : ${err.message}`);
  }
}

// Server restart timeout
try {
  await waitForServerRestart(30, 2000); // 30 attempts × 2s = 60s
} catch (err) {
  showMessage('Le serveur n\'a pas redémarré à temps. Rechargez la page.');
}
```

---

## 7. Testing Requirements

Create React component tests using Testing Library or similar:

### Test Cases

1. **Hook : useUpdates**
   - checkForUpdates() returns correct format
   - listAllUpdates() populates available versions
   - downloadUpdate() shows progress
   - applyUpdate() triggers restart
   - Error handling works

2. **Component : UpdatePage**
   - Renders version display
   - Version selection dropdown works
   - Download button disabled when no version selected
   - Apply button hidden until downloaded
   - Download progress displays
   - Restart spinner shows during apply

3. **Component : Navbar Badge**
   - Badge hidden when no update available
   - Badge shown when update available
   - Badge clickable and navigates to UpdatePage

**Commands** :
```bash
npm test          # Run all tests
npm test -- --coverage  # With coverage report
```

---

## 8. File Checklist

### Files to Create
- [ ] `web/src/hooks/useUpdates.js` (custom hook)
- [ ] `web/src/pages/UpdatePage.jsx` (main page)
- [ ] `web/src/pages/UpdatePage.css` (styles)

### Files to Modify
- [ ] `web/src/components/Navbar.jsx` (add badge)
- [ ] `web/src/App.jsx` (add route)

### Tests to Create
- [ ] `web/src/hooks/__tests__/useUpdates.test.js`
- [ ] `web/src/pages/__tests__/UpdatePage.test.js`

---

## 9. Validation Checkpoints

Before marking complete:

1. **Functionality**
   - [ ] Badge appears when update available
   - [ ] UpdatePage loads all versions
   - [ ] Download works with progress bar
   - [ ] Apply triggers restart
   - [ ] Page auto-reloads when server back
   - [ ] New version shows after reload

2. **Error Handling**
   - [ ] Network errors handled gracefully
   - [ ] Invalid versions handled
   - [ ] Server restart timeout handled

3. **UX**
   - [ ] Clear user feedback at each step
   - [ ] Buttons disabled appropriately
   - [ ] Progress indicators visible
   - [ ] Warning shown if game in progress

4. **Testing**
   - [ ] npm test passes
   - [ ] Components render correctly
   - [ ] Hook functions called appropriately

---

## Summary

Implement complete frontend auto-update system with:
- useUpdates hook for API integration
- UpdatePage for full update workflow
- Navbar badge for notifications
- Graceful restart and reconnection handling
- Comprehensive error handling
- React component tests

**Estimated Time** : 4-6 hours

**Deliverable** : Full working frontend integration with backend

