# Code Review Specification - Auto-Update v2.50.0

**Date** : 2026-02-01
**Feature** : Auto-Update Server (v2.50.0)
**Agent** : code-reviewer
**Branch** : feature/auto-update

---

## Review Scope

Complete code review of auto-update feature implementation covering both backend (Go) and frontend (React).

---

## Backend Files to Review

### 1. internal/server/github_client.go

**Purpose** : GitHub Releases API client with 1-hour caching

**Review Points** :

1. **API Integration**
   - [ ] Correctly fetches from https://api.github.com/repos/CCoupel/BuzzMaster/releases
   - [ ] Handles rate limiting (60 req/hour)
   - [ ] 1-hour cache implemented and working
   - [ ] Cache invalidation logic correct

2. **JSON Parsing**
   - [ ] Properly unmarshals GitHub Release objects
   - [ ] Handles missing/nil fields gracefully
   - [ ] Asset filtering works correctly
   - [ ] Version extraction from filenames correct

3. **Platform Detection**
   - [ ] runtime.GOOS and runtime.GOARCH used correctly
   - [ ] Correct asset selected (windows-amd64 vs linux-arm64)
   - [ ] Asset naming pattern matches: buzzcontrol-vX.Y.Z-{os}-{arch}.*

4. **Error Handling**
   - [ ] GitHub API errors handled (timeout, 403, etc.)
   - [ ] Malformed responses don't crash
   - [ ] Graceful fallback to cache
   - [ ] Error messages clear and logged

5. **Concurrency**
   - [ ] Thread-safe cache access
   - [ ] No race conditions on concurrent requests
   - [ ] Proper locking if used

6. **Code Quality**
   - [ ] Exported functions documented
   - [ ] Private functions reasonably documented
   - [ ] No dead code
   - [ ] Proper variable naming

---

### 2. internal/server/updater.go

**Purpose** : Update logic and HTTP handlers for 4 endpoints

**Review Points** :

1. **GET /api/updates Handler**
   - [ ] Calls GitHub client
   - [ ] Filters by current platform
   - [ ] Returns correct JSON structure
   - [ ] Cache header set properly
   - [ ] CORS headers if needed

2. **GET /api/updates/check Handler**
   - [ ] Calls GitHub client (or uses cached list)
   - [ ] Version comparison logic correct
   - [ ] update_available boolean correct
   - [ ] Response format matches contract

3. **POST /api/updates/download Handler**
   - [ ] Request parsing correct
   - [ ] Version validation
   - [ ] Platform validation
   - [ ] Downloads to temp directory
   - [ ] File integrity checks
   - [ ] Size validation (>= 40MB)
   - [ ] Binary execution test
   - [ ] Checksum calculation
   - [ ] Error responses proper

4. **POST /api/updates/apply Handler**
   - [ ] Request validation
   - [ ] Version vs path matching
   - [ ] Game state save (if applicable)
   - [ ] WebSocket graceful close
   - [ ] Backup creation (atomic)
   - [ ] File replacement atomic
   - [ ] New binary started
   - [ ] Restart monitoring (5 second timeout)
   - [ ] Rollback on failure
   - [ ] Error responses proper

5. **Safety & Security**
   - [ ] Backup always created before file operations
   - [ ] File operations atomic
   - [ ] Old binary restorable on failure
   - [ ] No partial states
   - [ ] Input sanitization
   - [ ] Path traversal prevention

6. **State Management**
   - [ ] Game state saved correctly
   - [ ] Connections closed gracefully
   - [ ] Websocket broadcast works
   - [ ] State restoration path clear

7. **Logging**
   - [ ] All operations logged
   - [ ] Log levels appropriate
   - [ ] Error details captured
   - [ ] Timestamps included

8. **Code Quality**
   - [ ] Functions < 100 lines where practical
   - [ ] Clear variable names
   - [ ] No duplicated code
   - [ ] Proper error propagation

---

### 3. internal/server/github_client_test.go

**Purpose** : Unit tests for GitHub client

**Review Points** :

1. **Test Coverage**
   - [ ] All public functions tested
   - [ ] Normal cases covered
   - [ ] Error cases covered
   - [ ] Edge cases handled
   - [ ] Coverage >= 80%

2. **Mock Data**
   - [ ] Realistic GitHub API responses
   - [ ] Various payload sizes
   - [ ] Missing fields handled
   - [ ] Malformed data tested

3. **Cache Testing**
   - [ ] Cache hit returns same data
   - [ ] Cache TTL respected
   - [ ] Cache invalidation works

4. **Platform Detection**
   - [ ] Windows-amd64 asset selected correctly
   - [ ] Linux-arm64 asset selected correctly
   - [ ] Unsupported platforms handled

5. **Error Scenarios**
   - [ ] GitHub API timeout
   - [ ] Rate limit (403)
   - [ ] Malformed response
   - [ ] Network error

6. **Test Quality**
   - [ ] Tests independent
   - [ ] No side effects
   - [ ] Clear assertions
   - [ ] Good test names

---

### 4. internal/server/updater_test.go

**Purpose** : Unit tests for update handlers

**Review Points** :

1. **Test Coverage**
   - [ ] All 4 endpoints tested
   - [ ] Success paths
   - [ ] Error paths
   - [ ] Edge cases
   - [ ] Coverage >= 80%

2. **Handler Tests**
   - [ ] GET /api/updates returns proper format
   - [ ] GET /api/updates/check detects update
   - [ ] POST /api/updates/download validates version
   - [ ] POST /api/updates/apply applies safely

3. **Mock Data**
   - [ ] Realistic payloads
   - [ ] Various file sizes
   - [ ] Different versions

4. **Error Scenarios**
   - [ ] Invalid version
   - [ ] Invalid path
   - [ ] GitHub unavailable
   - [ ] Download failed
   - [ ] Binary invalid

5. **Test Quality**
   - [ ] Tests isolated
   - [ ] Mocking comprehensive
   - [ ] Assertions clear
   - [ ] No flakiness

---

### 5. internal/server/http.go (Modifications)

**Purpose** : Route registration for update endpoints

**Review Points** :

1. **Route Registration**
   - [ ] All 4 routes registered
   - [ ] Methods correct (GET, POST)
   - [ ] Paths correct (/api/updates/*)
   - [ ] Handlers wired correctly

2. **Integration**
   - [ ] No conflicts with existing routes
   - [ ] Proper middleware applied if needed
   - [ ] CORS headers correct

---

### 6. internal/config/config.go (Modifications)

**Purpose** : Add auto_check_updates config option

**Review Points** :

1. **Config Structure**
   - [ ] Field added to ServerConfig struct
   - [ ] JSON tag correct
   - [ ] Type is bool
   - [ ] Default value reasonable (true)

2. **Initialization**
   - [ ] Default value set if missing
   - [ ] Config persists after load
   - [ ] Used by backend if applicable

---

## Frontend Files to Review

### 1. web/src/hooks/useUpdates.js

**Purpose** : Custom React hook for update management

**Review Points** :

1. **Hook Structure**
   - [ ] Follows React hooks conventions
   - [ ] useEffect cleanup proper
   - [ ] Dependencies arrays correct
   - [ ] No infinite loops

2. **State Management**
   - [ ] All needed state tracked
   - [ ] State updates proper
   - [ ] No unnecessary state
   - [ ] Initial state reasonable

3. **Functions Implementation**
   - [ ] checkForUpdates() works correctly
   - [ ] listAllUpdates() populates versions
   - [ ] downloadUpdate() handles progress
   - [ ] applyUpdate() triggers restart
   - [ ] waitForServerRestart() polls correctly

4. **Error Handling**
   - [ ] Try-catch blocks proper
   - [ ] Error messages clear
   - [ ] State updated on error
   - [ ] No silent failures

5. **API Integration**
   - [ ] Correct endpoints called
   - [ ] Request format correct
   - [ ] Response parsing correct
   - [ ] Headers proper (Content-Type)

6. **Polling Logic**
   - [ ] Poll interval correct (2 seconds)
   - [ ] Max attempts correct (30 = 60 seconds)
   - [ ] Timeout handling
   - [ ] No memory leaks

7. **Code Quality**
   - [ ] Clear variable names
   - [ ] Functions reasonable length
   - [ ] Comments where complex
   - [ ] No dead code

---

### 2. web/src/pages/UpdatePage.jsx

**Purpose** : Main update management page component

**Review Points** :

1. **Component Structure**
   - [ ] Functional component
   - [ ] Props typed if using TypeScript
   - [ ] Hooks used properly
   - [ ] Effect dependencies correct

2. **State Management**
   - [ ] useUpdates hook used
   - [ ] Local state for UI (selected version, etc.)
   - [ ] State updates proper

3. **UI Sections**
   - [ ] Version display section complete
   - [ ] Version selection dropdown works
   - [ ] Release notes displayed
   - [ ] Download section with progress
   - [ ] Apply section with confirmation
   - [ ] Error display
   - [ ] Loading states

4. **User Interactions**
   - [ ] Download button works
   - [ ] Apply button hidden until ready
   - [ ] Confirmation dialog if game in progress
   - [ ] Cancel option if needed
   - [ ] Proper button states (disabled during operations)

5. **Error Handling**
   - [ ] Network errors caught
   - [ ] Error messages displayed
   - [ ] Retry options
   - [ ] No unhandled rejections

6. **Restart Handling**
   - [ ] After apply, shows spinner
   - [ ] Polls for server restart
   - [ ] Auto-reload on success
   - [ ] Timeout handling

7. **Responsive Design**
   - [ ] Mobile friendly
   - [ ] Proper layout
   - [ ] Button sizes accessible

8. **Accessibility**
   - [ ] Labels for inputs
   - [ ] Buttons descriptive
   - [ ] Color not only indicator
   - [ ] Keyboard navigation

9. **Code Quality**
   - [ ] Component < 200 lines where practical
   - [ ] Clear variable names
   - [ ] Comments for complex logic
   - [ ] No console errors/warnings

---

### 3. web/src/pages/UpdatePage.css

**Purpose** : Styling for UpdatePage component

**Review Points** :

1. **Styling**
   - [ ] Responsive design
   - [ ] Proper spacing
   - [ ] Color scheme consistent
   - [ ] Fonts readable

2. **Visual Feedback**
   - [ ] Progress bar visible
   - [ ] Spinner animates
   - [ ] Buttons look clickable
   - [ ] Alerts distinct

3. **Layout**
   - [ ] Sections organized
   - [ ] No horizontal scroll
   - [ ] Mobile layout works
   - [ ] Proper use of flexbox/grid

---

### 4. web/src/components/Navbar.jsx (Modifications)

**Purpose** : Add update notification badge

**Review Points** :

1. **Hook Integration**
   - [ ] useUpdates hook used
   - [ ] checkForUpdates() called on load
   - [ ] Update check interval (optional, hourly)

2. **Badge Display**
   - [ ] Hidden when no update
   - [ ] Visible when update available
   - [ ] Clear messaging
   - [ ] Clickable to UpdatePage

3. **Styling**
   - [ ] Badge visible but not intrusive
   - [ ] Pulsing animation if applicable
   - [ ] Consistent with navbar design
   - [ ] Properly positioned

4. **No Breaking Changes**
   - [ ] Existing navbar functionality intact
   - [ ] No layout broken
   - [ ] Other components still work

---

### 5. web/src/App.jsx (Modifications)

**Purpose** : Add route to UpdatePage

**Review Points** :

1. **Route Addition**
   - [ ] Route path correct (/admin/updates)
   - [ ] Component imported
   - [ ] Route properly integrated
   - [ ] No conflicts

2. **Navigation**
   - [ ] Link from navbar badge works
   - [ ] Navigation to/from page works
   - [ ] History handled properly

---

## Review Criteria

### 1. Code Quality (30%)

- [ ] Clean, readable code
- [ ] Proper naming conventions
- [ ] No code duplication
- [ ] Functions have single responsibility
- [ ] Complexity reasonable

**Pass Threshold** : >= 80% of checks

---

### 2. Error Handling (25%)

- [ ] All error paths covered
- [ ] Error messages clear and actionable
- [ ] Graceful degradation
- [ ] No silent failures
- [ ] Proper error propagation

**Pass Threshold** : >= 85% of checks

---

### 3. Security & Safety (20%)

- [ ] File operations safe
- [ ] Input validation present
- [ ] No hardcoded secrets
- [ ] State managed properly
- [ ] Backup/rollback working
- [ ] No unauthorized access

**Pass Threshold** : >= 90% of checks (CRITICAL)

---

### 4. Testing (15%)

- [ ] Unit tests comprehensive
- [ ] Edge cases covered
- [ ] Mock data realistic
- [ ] Tests independent
- [ ] Coverage adequate (>= 80%)
- [ ] All tests passing

**Pass Threshold** : >= 80% of checks

---

### 5. Documentation (10%)

- [ ] Code comments clear
- [ ] API contracts followed
- [ ] Contracts vs implementation match
- [ ] Complex logic explained

**Pass Threshold** : >= 75% of checks

---

### 6. Backward Compatibility (5%)

- [ ] No breaking changes
- [ ] Existing functionality preserved
- [ ] Config defaults reasonable
- [ ] Migration path if needed

**Pass Threshold** : >= 90% of checks

---

## Review Output Format

The code-reviewer must provide:

### 1. Overall Verdict
```
APPROVED
OR
APPROVED WITH RESERVATIONS
OR
REJECTED
```

### 2. Issues Summary

| Category | Count | Severity |
|----------|-------|----------|
| Critical | X | MUST FIX |
| High | Y | SHOULD FIX |
| Medium | Z | NICE TO FIX |
| Low | W | OPTIONAL |

### 3. Detailed Issues

For each issue:
```
**Issue** : [Description]
**File** : [path/to/file.go or .jsx]
**Line** : [line number]
**Severity** : CRITICAL | HIGH | MEDIUM | LOW
**Recommendation** : [Fix or improvement]
**Code Example** : [Before/After if applicable]
```

### 4. Strengths

List positive aspects:
- Well-structured error handling
- Comprehensive test coverage
- Clean code organization
- Good documentation
- etc.

### 5. Risks Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|-----------|
| ... | ... | ... | ... |

### 6. Recommendations

- [ ] Item 1
- [ ] Item 2
- [ ] etc.

### 7. Blockers for QA

If REJECTED or APPROVED WITH RESERVATIONS:
- List issues that must be addressed before QA
- Estimate effort to fix
- Suggest priority

---

## Review Timeline

- Estimated time: 2-3 hours
- Must check all files listed above
- Must validate against contracts
- Must run tests
- Must test on actual codebase if possible

---

## Pass Criteria

### APPROVED
- Code quality >= 80%
- Error handling >= 85%
- Security & Safety >= 90%
- Testing >= 80%
- No critical or high severity issues

### APPROVED WITH RESERVATIONS
- Code quality >= 75%
- Error handling >= 80%
- Security & Safety >= 85%
- Testing >= 75%
- Medium severity issues only (no critical/high)
- Issues documented and non-blocking for QA

### REJECTED
- Code quality < 75%
- OR Error handling < 80%
- OR Security & Safety < 85%
- OR Testing < 75%
- OR Critical/high severity issues present
- OR Breaking changes detected
- Issues must be fixed before QA

---

## Validation Against Contracts

Review must validate:

1. **GET /api/updates**
   - Response format matches contracts/auto-update-endpoints.md
   - All fields present
   - Data types correct

2. **GET /api/updates/check**
   - Response format matches contract
   - update_available boolean correct
   - Versions accurate

3. **POST /api/updates/download**
   - Request parsing correct
   - Response format matches
   - Path returned usable

4. **POST /api/updates/apply**
   - Request parsing correct
   - Restart initiated properly
   - Frontend can poll after response

---

## Sign-Off

Code review complete when:

```
Reviewed By: [code-reviewer]
Date: [YYYY-MM-DD]
Status: APPROVED | APPROVED WITH RESERVATIONS | REJECTED
Issues: [X critical, Y high, Z medium, W low]
Recommendation: PROCEED TO QA | FIX AND RESUBMIT | ESCALATE
```

---

## Next Steps

**If APPROVED** :
→ Phase 4 : QA Testing

**If APPROVED WITH RESERVATIONS** :
→ Developers fix medium issues
→ Resubmit for verification
→ Phase 4 : QA Testing

**If REJECTED** :
→ Developers fix critical/high issues
→ Resubmit for full review
→ Back to development cycle

