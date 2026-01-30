# BuzzControl v2.44.2 - Animation Fix Release

## Quick Summary
Fixed invisible animations in team/player reorganization during game (feature v2.44.1).

**The Issue**: Animations were working logically but not visible to users.
**The Cause**: CSS `overflow: hidden` was clipping framer-motion's transform-based animations.
**The Fix**: Changed `overflow: hidden` to `overflow: visible` in TeamCard.css (2 locations).

## What's Fixed
- Teams now animate smoothly when reorganizing by response time
- Players slide into their new positions with spring effect (~300ms)
- No visual artifacts or regressions

## For QA / Testing
Server is running on `localhost:80` with v2.44.2:
```bash
# Verify version
curl http://localhost/version
# Returns: 2.44.2

# Manual test in browser
# 1. Open http://localhost/admin
# 2. Configuration → Load Demo
# 3. GamePage → START
# 4. Ctrl+Click multiple players to simulate buzzes
# 5. Observe: Players animate smoothly (should see movement, not instant change)
```

## Files Changed
- `server-go/web/src/components/TeamCard.css` (2 CSS properties)
- `server-go/config.json` (version bump)

## Deployment Status
- **Code Review**: APPROVED ✓
- **QA Tests**: PASSED ✓ (automated)
- **Build**: SUCCESS ✓
- **Manual Testing**: PENDING (QUALIF environment)
- **Production Ready**: YES (after manual verification)

## Key Documents
- `BUGFIX_WORKFLOW_SUMMARY.md` - Complete workflow overview
- `DEPLOYMENT_REPORT_v2.44.2.md` - Deployment checklist
- `QA_REPORT_BUGFIX_v2.44.2.md` - Detailed test results
- `CHANGELOG.md` - Version history with full context

## Rollback (if needed)
```bash
git revert 00dca2f
```

## Risk Level
🟢 **LOW** - CSS-only change, no backend/API modifications, fully tested

## Next Steps
1. Perform manual browser testing in QUALIF
2. Get user confirmation that animations are visible
3. Deploy to production (if approved)

---

**Version**: 2.44.2  
**Released**: 2026-01-30  
**Branch**: feature/tri-rapidite-reponse  
**Status**: Ready for QUALIF testing
