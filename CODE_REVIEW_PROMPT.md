# Prompt pour code-reviewer : Auto-Update Implementation Review

## Contexte

You are the Code Reviewer for BuzzControl auto-update feature (v2.50.0).

Your task is to conduct a comprehensive code review of the completed backend and frontend implementation.

## Current State

- **Branch** : feature/auto-update
- **Version** : 2.50.0
- **Backend** : COMPLETED (4 endpoints, GitHub client, tests)
- **Frontend** : COMPLETED (hook, UpdatePage, Navbar badge)
- **Tests** : ALL PASSING (go test ./..., npm test)

## Review Scope

### Backend Files (Go)

1. **internal/server/github_client.go**
   - GitHub Releases API integration
   - Cache mechanism (1 hour TTL)
   - Platform detection (windows-amd64, linux-arm64)
   - Asset filtering

2. **internal/server/updater.go**
   - 4 HTTP handlers (GET/POST)
   - Download logic
   - Update application logic
   - State preservation

3. **internal/server/github_client_test.go**
   - Unit tests for GitHub client
   - Mock API responses
   - Cache testing
   - Error scenarios

4. **internal/server/updater_test.go**
   - Unit tests for handlers
   - Request/response validation
   - Error path testing
   - Edge cases

5. **internal/server/http.go (modifications)**
   - Route registration
   - Integration with existing server

6. **internal/config/config.go (modifications)**
   - auto_check_updates configuration
   - Default values

### Frontend Files (React)

1. **web/src/hooks/useUpdates.js**
   - Custom React hook
   - State management
   - API integration
   - Error handling

2. **web/src/pages/UpdatePage.jsx**
   - Main update management component
   - User interface
   - Version selection and download
   - Apply and restart workflow

3. **web/src/pages/UpdatePage.css**
   - Styling for UpdatePage
   - Responsive design
   - Progress indicators

4. **web/src/components/Navbar.jsx (modifications)**
   - Update notification badge
   - Check for updates on load

5. **web/src/App.jsx (modifications)**
   - Route to UpdatePage
   - Navigation integration

## Review Criteria (Weighted)

1. **Code Quality** (30%) : Clean, readable, no duplication
2. **Error Handling** (25%) : All error paths covered, clear messages
3. **Security & Safety** (20%) : Safe file ops, backup/rollback, no secrets
4. **Testing** (15%) : Comprehensive coverage, all tests passing
5. **Documentation** (10%) : Comments, API contracts followed

## Contracts to Validate

All implementation must match:
- **contracts/auto-update-endpoints.md** (4 endpoints)
- **contracts/http-endpoints.md** (existing endpoints)
- **PLAN_AUTO_UPDATE_v2.50.0.md** (architecture)

## Pass Criteria

### APPROVED (Proceed to QA)
- Code quality >= 80%
- Error handling >= 85%
- Security & Safety >= 90% (CRITICAL)
- Testing >= 80%
- No critical or high severity issues
- All contracts validated

### APPROVED WITH RESERVATIONS (Fix medium issues, then QA)
- Code quality >= 75%
- Error handling >= 80%
- Security & Safety >= 85%
- Testing >= 75%
- Only medium severity issues (documented, non-blocking)

### REJECTED (Back to development)
- Any criteria below threshold
- Critical/high severity issues present
- Breaking changes detected
- Security concerns

## Deliverables

Provide detailed review report with:

1. **Overall Verdict** : APPROVED / APPROVED WITH RESERVATIONS / REJECTED

2. **Issues Summary**
   - Critical : X
   - High : Y
   - Medium : Z
   - Low : W

3. **Detailed Issues** (for each issue)
   - Issue description
   - File and line number
   - Severity level
   - Recommendation / Code fix

4. **Strengths** (positive aspects)

5. **Risks Assessment** (likelihood, impact, mitigation)

6. **Recommendations** (actionable items)

7. **Blockers for QA** (if any)

## Key Review Points

### Backend Safety
- [ ] Backup always created before file operations
- [ ] Old binary restorable on failure
- [ ] File operations atomic
- [ ] No partial states
- [ ] State saved before shutdown

### Frontend UX
- [ ] Badge visible when update available
- [ ] Clear user feedback at each step
- [ ] Proper button states (enabled/disabled)
- [ ] Error messages helpful
- [ ] Auto-reload on restart

### Testing
- [ ] go test ./... passes (>= 80% coverage)
- [ ] npm test passes
- [ ] Mock data realistic
- [ ] Edge cases covered

### Documentation
- [ ] Code comments clear
- [ ] Implementation matches contracts
- [ ] Complex logic explained

## Timeline

Estimated review time: 2-3 hours

## Questions & References

If unclear, refer to:
- **Detailed specs** : DEV_BACKEND_AUTO_UPDATE.md, DEV_FRONTEND_AUTO_UPDATE.md
- **Code review spec** : CODE_REVIEW_SPEC.md
- **Contracts** : contracts/auto-update-endpoints.md
- **Plan** : PLAN_AUTO_UPDATE_v2.50.0.md

## Sign-Off Template

```
Code Review Completed
Reviewed By: [name]
Date: [YYYY-MM-DD]
Status: APPROVED | APPROVED WITH RESERVATIONS | REJECTED
Critical Issues: X
High Issues: Y
Medium Issues: Z
Low Issues: W
Recommendation: PROCEED TO QA | FIX AND RESUBMIT | ESCALATE
```

---

**Status** : READY FOR CODE REVIEW

Begin comprehensive review of all files. Commit summary when complete.

