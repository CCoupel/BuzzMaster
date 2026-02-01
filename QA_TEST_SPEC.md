# QA Test Specification - Auto-Update v2.50.1

**Date** : 2026-02-01
**Feature** : Auto-Update Server (v2.50.1)
**Agent** : QA
**Branch** : feature/auto-update
**Version** : 2.50.1 (with corrections applied)

---

## Overview

Execute complete QA testing of auto-update feature covering unit tests, integration tests, and manual validation scenarios.

---

## Part 1 : Unit Tests Execution

### Backend Tests (Go)

**Command** :
```bash
cd server-go
go test ./... -v -cover
```

**Expected Output** :
```
ok  	buzzcontrol/internal/server	X.XXXs	coverage: Y%
ok  	buzzcontrol/internal/config	X.XXXs	coverage: Y%
...
```

**Pass Criteria** :
- [ ] All tests pass (no FAIL or ERROR)
- [ ] Coverage >= 80% for auto-update packages
  - `internal/server/github_client.go` and tests
  - `internal/server/updater.go` and tests
- [ ] No race conditions detected
- [ ] No timeout failures

**Test Coverage Breakdown** :

| Package | File | Min Coverage |
|---------|------|--------------|
| server | github_client.go | >= 85% |
| server | updater.go | >= 85% |
| config | config.go | >= 75% |
| utils | file_backup.go | >= 80% |

**Specific Tests to Verify** :

1. **GitHub Client Tests** (github_client_test.go)
   - [ ] TestGetReleases - Fetches and parses correctly
   - [ ] TestGetReleasesWithCache - Cache works for 1 hour
   - [ ] TestGetReleasesCacheInvalidation - Cache can be invalidated
   - [ ] TestPlatformDetection - Correct OS/arch detected
   - [ ] TestAssetFiltering - Correct binary selected
   - [ ] TestGitHubAPIError - Error handling (timeout, 403, etc.)
   - [ ] TestMalformedResponse - Graceful handling

2. **Update Handler Tests** (updater_test.go)
   - [ ] TestGetUpdatesEndpoint - Returns correct format
   - [ ] TestGetUpdatesCheckEndpoint - update_available flag correct
   - [ ] TestDownloadEndpointValidation - Version validation
   - [ ] TestDownloadIntegrity - File size check
   - [ ] TestApplyEndpointSafety - Backup before replace
   - [ ] TestApplyRollback - Rollback on failure
   - [ ] TestErrorScenarios - GitHub unavailable, file errors, etc.

**Report Template** :
```
=== BACKEND TESTS ===

Command: go test ./... -v -cover
Status: PASS / FAIL
Total Tests: X
Passed: X
Failed: 0
Coverage: Y%

Test Results:
- github_client_test.go: PASS (coverage: Z%)
- updater_test.go: PASS (coverage: Z%)
- ... other tests ...

Issues Found:
- (none if all pass, or list if failures)
```

---

### Frontend Tests (React/JavaScript)

**Command** :
```bash
cd server-go/web
npm test -- --coverage --watchAll=false
```

**Expected Output** :
```
PASS  src/hooks/__tests__/useUpdates.test.js
PASS  src/pages/__tests__/UpdatePage.test.js
PASS  src/components/__tests__/Navbar.test.js

Test Suites: 3 passed, 3 total
Tests:       X passed, X total
Coverage:    ...
```

**Pass Criteria** :
- [ ] All test suites pass
- [ ] No test failures
- [ ] Coverage >= 75% for new files
  - `hooks/useUpdates.js` >= 80%
  - `pages/UpdatePage.jsx` >= 75%
- [ ] No warnings or errors

**Specific Tests to Verify** :

1. **useUpdates Hook Tests**
   - [ ] checkForUpdates - Calls /api/updates/check
   - [ ] listAllUpdates - Populates versions array
   - [ ] downloadUpdate - Triggers download
   - [ ] applyUpdate - Triggers apply and polls
   - [ ] waitForServerRestart - Polls until online
   - [ ] Error handling - Network errors caught
   - [ ] State management - State updates correctly

2. **UpdatePage Component Tests**
   - [ ] Renders without crashing
   - [ ] Version selection dropdown works
   - [ ] Download button triggers download
   - [ ] Progress displays during download
   - [ ] Apply button appears after download
   - [ ] Confirmation dialog if game in progress
   - [ ] Spinner shows during restart
   - [ ] Error messages display
   - [ ] Auto-reload on reconnection

3. **Navbar Badge Tests**
   - [ ] Badge hidden when no update
   - [ ] Badge shown when update available
   - [ ] Badge clickable
   - [ ] Updates on check

**Report Template** :
```
=== FRONTEND TESTS ===

Command: npm test -- --coverage --watchAll=false
Status: PASS / FAIL
Test Suites: X passed, X total
Tests: Y passed, Y total
Coverage: Z%

Test Results:
- useUpdates.js: PASS (coverage: Z%)
- UpdatePage.jsx: PASS (coverage: Z%)
- Navbar.jsx: PASS (coverage: Z%)

Issues Found:
- (none if all pass, or list if failures)
```

---

## Part 2 : Build Validation

### Backend Build

**Command** :
```bash
cd server-go
go build -o server.exe ./cmd/server
```

**Pass Criteria** :
- [ ] Build succeeds with no errors
- [ ] Build succeeds with no warnings
- [ ] Binary generated
- [ ] Binary is executable

**Report** :
```
=== BACKEND BUILD ===

Command: go build -o server.exe ./cmd/server
Status: SUCCESS / FAILURE
Binary Size: X MB
Build Time: X seconds
Errors: (none)
Warnings: (none)
```

---

### Frontend Build

**Command** :
```bash
cd server-go/web
npm run build
```

**Pass Criteria** :
- [ ] Build succeeds with no errors
- [ ] Build directory created (web/build/)
- [ ] No critical warnings
- [ ] Bundle size reasonable

**Report** :
```
=== FRONTEND BUILD ===

Command: npm run build
Status: SUCCESS / FAILURE
Output Directory: web/build/
Bundle Size: X MB
Build Time: X seconds
Errors: (none)
Warnings: (none or list)
```

---

## Part 3 : Manual Validation Scenarios

If E2E testing is available (MCP claude-in-chrome), execute these scenarios:

### Scenario 1 : Check for Update Available

**Steps** :
1. Start backend server
2. Open http://localhost/admin in browser
3. Check if update badge appears in Navbar

**Expected** :
- Badge visible (yellow/orange background)
- Text: "Mise à jour disponible"
- Pulsing animation on indicator dot

**Validation** :
- [ ] Badge appears
- [ ] Badge text visible
- [ ] Badge clickable
- [ ] Animation present

---

### Scenario 2 : Open Update Page

**Steps** :
1. Click on update badge in Navbar
2. Wait for UpdatePage to load

**Expected** :
- Page displays current version (2.50.0)
- Latest version shown (from GitHub)
- List of available versions in dropdown
- Release notes visible

**Validation** :
- [ ] Page loads
- [ ] Current version correct
- [ ] Latest version correct
- [ ] Versions dropdown populated
- [ ] Release notes displayed

---

### Scenario 3 : Download Version

**Steps** :
1. Select version from dropdown (e.g., 2.49.0)
2. Click "Download v2.49.0" button
3. Observe download progress
4. Wait for completion

**Expected** :
- Download button disabled during download
- Progress bar shows percentage
- After completion: "Download completed" message
- Downloaded file verified

**Validation** :
- [ ] Download starts
- [ ] Progress displays
- [ ] Progress increases
- [ ] Completion message shown
- [ ] Download button becomes active again

---

### Scenario 4 : Apply Update and Restart

**Steps** :
1. After download, click "Apply and Restart" button
2. Observe confirmation dialog (if game in progress)
3. Confirm restart
4. Observe restart spinner

**Expected** :
- Confirmation dialog appears (if game running)
- "Apply and Restart" button disabled
- Spinner displayed with message "Server restarting..."
- Auto-reload after server restarts

**Validation** :
- [ ] Confirmation appears if game active
- [ ] Accept continues
- [ ] Spinner displayed
- [ ] Page reloads automatically
- [ ] No errors

---

### Scenario 5 : Verify New Version

**Steps** :
1. After restart completes and page reloads
2. Check UpdatePage again

**Expected** :
- Current version now shows new version
- update_available = false (no new updates)
- Badge disappears from Navbar

**Validation** :
- [ ] Version updated correctly
- [ ] Badge gone
- [ ] UpdatePage shows current version
- [ ] No update available

---

### Scenario 6 : Error Handling

**Steps** :
1. Simulate GitHub API unavailable
2. Try to get update list
3. Should show error gracefully

**Expected** :
- Error message: "GitHub API temporairement indisponible"
- Retry button available
- Page remains functional
- No crash

**Validation** :
- [ ] Error message clear
- [ ] Retry option available
- [ ] Page functional
- [ ] No console errors

---

### Scenario 7 : Game in Progress Warning

**Steps** :
1. Start a game
2. Try to apply update
3. Observe warning

**Expected** :
- Warning dialog: "Une partie est en cours. Continuer ?"
- Require confirmation before proceeding

**Validation** :
- [ ] Warning displayed
- [ ] Confirmation required
- [ ] Cancel option available
- [ ] Accept proceeds

---

## Part 4 : Coverage Metrics

### Backend Coverage Report

**Command** :
```bash
cd server-go
go test ./... -v -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Expected Metrics** :
- Overall coverage >= 80%
- github_client.go >= 85%
- updater.go >= 85%
- config.go >= 75%
- utils/file_backup.go >= 80%

**Report** :
```
=== COVERAGE REPORT ===

Overall Coverage: X%
Critical Packages:
- internal/server: Y%
- internal/config: Z%
- internal/utils: W%

Uncovered Lines:
- (list if any critical path uncovered)

Recommendation: PASS / NEED MORE COVERAGE
```

---

## Part 5 : Contract Validation

### Endpoint Contract Validation

**GET /api/updates**
- [ ] Response format matches contract
- [ ] All fields present (version, date, notes, download_url, current, size)
- [ ] Data types correct
- [ ] Cache working (no repeated API calls)

**GET /api/updates/check**
- [ ] Response format matches
- [ ] Fields: update_available, current, latest, release_url
- [ ] update_available is boolean
- [ ] Cache working

**POST /api/updates/download**
- [ ] Request accepts { version }
- [ ] Response has success, version, path, size, checksum
- [ ] Path is valid temporary location
- [ ] File size >= 40MB
- [ ] Checksum provided

**POST /api/updates/apply**
- [ ] Request accepts { version, path }
- [ ] Response has success, message, restart_in_seconds
- [ ] Server restarts within stated seconds
- [ ] Old server stops
- [ ] New server starts

**Verdict** : PASS / FAIL

---

## Part 6 : Regression Testing

### Existing Functionality

Verify no breaking changes to existing features:

**Game Engine**
- [ ] Game start/stop works
- [ ] Questions load correctly
- [ ] Scores calculated properly
- [ ] Teams/players managed

**Web Interface**
- [ ] All pages load
- [ ] Navigation works
- [ ] No console errors
- [ ] Styling intact

**WebSocket**
- [ ] Connections work
- [ ] Messages broadcast
- [ ] No leaks on disconnect

**Configuration**
- [ ] Config loads
- [ ] Settings persist
- [ ] Defaults work

**Verdict** : PASS / FAIL

---

## Part 7 : Performance Metrics

**Backend Performance**
- [ ] GET /api/updates response time < 200ms (cached)
- [ ] GET /api/updates/check response time < 100ms
- [ ] Download endpoint handles large files
- [ ] No memory leaks during operations

**Frontend Performance**
- [ ] UpdatePage loads in < 1 second
- [ ] Progress update smooth (no lag)
- [ ] No memory leaks during polling

**System Performance**
- [ ] CPU usage reasonable during download
- [ ] Memory usage stable
- [ ] No resource exhaustion

---

## Part 8 : Security Validation

**File Operations**
- [ ] Backup created before replace
- [ ] Old binary restorable
- [ ] No permission issues
- [ ] Atomic operations

**API Security**
- [ ] No SQL injection
- [ ] No path traversal
- [ ] Input validation present
- [ ] Error messages don't leak info

**State Management**
- [ ] Game state encrypted if sensitive
- [ ] No sensitive data in logs
- [ ] No hardcoded secrets

---

## Final QA Report Template

```markdown
# QA Report - Auto-Update v2.50.1

**Date**: [YYYY-MM-DD]
**Tester**: [QA Agent]
**Version**: 2.50.1

## Test Results Summary

| Category | Status | Details |
|----------|--------|---------|
| Backend Tests | PASS/FAIL | X tests passed |
| Frontend Tests | PASS/FAIL | Y tests passed |
| Backend Build | PASS/FAIL | Binary generated |
| Frontend Build | PASS/FAIL | Build created |
| Manual Scenarios | PASS/FAIL | Z scenarios validated |
| Coverage | PASS/FAIL | X% overall |
| Contracts | PASS/FAIL | All endpoints validated |
| Regression | PASS/FAIL | No breaking changes |

## Detailed Results

### Unit Tests
- Backend: PASS (coverage: X%)
- Frontend: PASS (coverage: Y%)

### Manual Scenarios
1. Update badge: PASS
2. UpdatePage: PASS
3. Download: PASS
4. Apply & Restart: PASS
5. Version verified: PASS
6. Error handling: PASS
7. Game warning: PASS

### Coverage Metrics
- Overall: X%
- Critical paths: >= 80%
- Untested paths: (none)

## Issues Found

### Critical
- (none if all pass)

### High
- (none if all pass)

### Medium
- (none if all pass)

### Low
- (if any)

## Recommendations

- Ready for deployment
- OR Minor issues, proceed with caution
- OR Blocking issues, back to development

## Verdict

**PASSED** - All tests passing, ready for Phase 5
OR
**CONDITIONAL PASS** - Minor issues, non-blocking
OR
**FAILED** - Blocking issues found, back to development

**Signed**: [QA Agent]
**Date**: [YYYY-MM-DD]
```

---

## Execution Checklist

Before starting QA:
- [ ] Branch feature/auto-update checked out
- [ ] All corrections applied (v2.50.1)
- [ ] Dependencies installed (go mod, npm install)
- [ ] Backend server can start
- [ ] Frontend builds

During QA:
- [ ] Run all unit tests
- [ ] Check coverage
- [ ] Build binaries
- [ ] Execute manual scenarios (if E2E available)
- [ ] Document results

After QA:
- [ ] Compile final report
- [ ] List any issues
- [ ] Provide verdict
- [ ] Recommend next steps

---

## Pass Criteria

### PASSED
- All unit tests pass
- Coverage >= 80% (backend), >= 75% (frontend)
- All manual scenarios pass (if executed)
- Contracts validated
- No regression issues
- No blocking security issues
- Ready for Phase 5 (Documentation)

### CONDITIONAL PASS
- All critical tests pass
- Minor issues only (low/medium severity)
- No critical/high issues
- Manual scenarios mostly pass
- Ready for Phase 5 with notes

### FAILED
- Any critical test fails
- Critical/high security issues
- Breaking changes detected
- Manual scenarios fail
- Must return to development

---

## Timeline

**Estimated QA Time**: 2-3 hours
- Backend tests: 30 minutes
- Frontend tests: 30 minutes
- Builds: 20 minutes
- Manual scenarios: 30 minutes
- Report: 30 minutes

