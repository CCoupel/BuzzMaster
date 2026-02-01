# Prompt pour dev-frontend : Auto-Update UI Implementation

## Contexte

You are the Frontend Developer for BuzzControl auto-update feature (v2.50.0).

Your task is to implement the complete frontend UI for auto-update functionality (Phase 3).

**Backend is COMPLETE** and all 4 REST endpoints are working and ready for consumption.

## Feature Overview

Implement frontend interface for:
1. Update availability notifications
2. Version list and selection
3. Download management with progress
4. Safe server restart with reconnection
5. User-friendly error handling

## Current State

- **Branch** : feature/auto-update
- **Version** : 2.50.0
- **Backend** : COMPLETED (4 endpoints working)
- **Specification** : DEV_FRONTEND_AUTO_UPDATE.md (complete)

## Backend Endpoints (READY TO USE)

### GET /api/updates
Returns list of all available versions with cache 1h

### GET /api/updates/check
Quick version check

### POST /api/updates/download
Download specific version

### POST /api/updates/apply
Apply update and restart

## Implementation Scope

### 1. Create useUpdates Hook
File: `web/src/hooks/useUpdates.js`

Custom React hook with:
- checkForUpdates()
- listAllUpdates()
- downloadUpdate(version)
- applyUpdate(version, path)
- waitForServerRestart()
- Proper error and loading state management

### 2. Create UpdatePage Component
File: `web/src/pages/UpdatePage.jsx`

Full update management page with:
- Current/latest version display
- Version selection dropdown
- Download progress bar
- Apply button with confirmation
- Error handling
- Restart spinner and reconnection

### 3. Modify Navbar
File: `web/src/components/Navbar.jsx`

Add update notification badge:
- Check on page load
- Show badge if update available
- Clickable link to UpdatePage

### 4. Add Route
File: `web/src/App.jsx`

Add route `/admin/updates` for UpdatePage

## Key Requirements

- Fetch API for HTTP calls
- Proper error handling
- Loading and progress states
- Poll for server restart (2s × 30 attempts)
- Auto-reload page when server back
- Warning if game in progress
- Comprehensive component tests

## Testing

Write unit/component tests:
- useUpdates hook functionality
- UpdatePage component rendering
- Navbar badge display
- Error scenarios

Run: `npm test`

## Delivery

- All files created/modified
- Tests passing
- No breaking changes
- Summary of implementation

**Estimated Time** : 4-6 hours

See DEV_FRONTEND_AUTO_UPDATE.md for complete specification.

