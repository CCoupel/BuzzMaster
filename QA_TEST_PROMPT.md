# Prompt pour QA : Auto-Update Feature Testing

## Contexte

You are the QA agent for BuzzControl auto-update feature (v2.50.1).

Your task is to execute comprehensive testing of the implemented feature.

## Current State

- **Branch** : feature/auto-update
- **Version** : 2.50.1 (with corrections applied)
- **Backend** : Code reviewed and corrected
- **Frontend** : Code reviewed and corrected
- **Code Review** : PASSED with corrections

## Testing Scope

### Part 1 : Unit Tests

**Backend (Go)**
```bash
cd server-go
go test ./... -v -cover
```

Expected: All tests pass, coverage >= 80%

**Frontend (React)**
```bash
cd server-go/web
npm test -- --coverage --watchAll=false
```

Expected: All tests pass, coverage >= 75%

### Part 2 : Build Validation

**Backend Build**
```bash
cd server-go
go build -o server.exe ./cmd/server
```

Expected: Binary created, no errors

**Frontend Build**
```bash
cd server-go/web
npm run build
```

Expected: Build directory created, no errors

### Part 3 : Manual Scenarios (if E2E available)

Use MCP claude-in-chrome if available:

1. Check update badge appears
2. Open UpdatePage
3. Download version
4. Apply and restart
5. Verify new version active
6. Error handling
7. Game in progress warning

### Part 4 : Coverage Metrics

Generate coverage reports:
- Backend >= 80%
- Frontend >= 75%
- Critical paths >= 85%

### Part 5 : Contract Validation

Verify all endpoints match contracts:
- GET /api/updates
- GET /api/updates/check
- POST /api/updates/download
- POST /api/updates/apply

### Part 6 : Regression Testing

Ensure no breaking changes:
- Game engine works
- Web interface works
- WebSocket works
- Configuration works

## Pass Criteria

### PASSED
- All unit tests pass
- Coverage >= 80% (backend), >= 75% (frontend)
- All builds succeed
- All manual scenarios pass (if executed)
- Contracts validated
- No regression issues
- Ready for Phase 5

### CONDITIONAL PASS
- Critical tests pass
- Minor issues only (non-blocking)
- Ready for Phase 5 with notes

### FAILED
- Any critical test fails
- Blocking issues found
- Back to development

## Deliverables

Provide QA report with:

1. **Test Execution Results**
   - Backend: X tests passed
   - Frontend: Y tests passed

2. **Coverage Metrics**
   - Overall coverage
   - Critical packages

3. **Build Status**
   - Backend build success
   - Frontend build success

4. **Manual Scenarios**
   - Scenario results (if executed)

5. **Issues Found**
   - Critical: X
   - High: Y
   - Medium: Z
   - Low: W

6. **Verdict**
   - PASSED / CONDITIONAL PASS / FAILED

7. **Recommendations**
   - Proceed to Phase 5
   - OR Fix issues and retest
   - OR Back to development

## Key Test Points

- [ ] All unit tests pass
- [ ] Coverage >= thresholds
- [ ] Builds succeed
- [ ] Manual scenarios work
- [ ] Contracts validated
- [ ] No regressions
- [ ] No security issues

## Timeline

Estimated: 2-3 hours

## References

- **Full spec**: QA_TEST_SPEC.md
- **Contracts**: contracts/auto-update-endpoints.md
- **Version**: 2.50.1

---

**Status** : READY FOR QA TESTING

Execute all tests and provide detailed report.

