# Deployment Guide - QUALIF v2.49.4

**Feature** : Backup/Restore Page
**Release Date** : 2026-02-01
**Version** : v2.49.4
**Branch** : feature/admin-joueur-card-style

---

## Pre-Deployment Checklist

### Build Verification
- [x] Go build: `go build -o server.exe ./cmd/server` ✅ SUCCESS
- [x] React build: `npm run build` ✅ SUCCESS (452KB)
- [x] No compilation errors ✅
- [x] Version updated to 2.49.4 ✅

### Code Quality
- [x] Code review: APPROVED ✅
- [x] QA testing: VALIDATED ✅
- [x] No regressions detected ✅
- [x] CHANGELOG updated ✅

### Commits
```
6aade52 docs(changelog): Add v2.49.4 release notes
501d1cb test(qa): QA test report - VALIDATED
7ca64c8 docs(review): Code review report
48c8b3b test(backup): Add unit tests and E2E scenarios
d6fff8e feat(backup): Create dedicated Backup/Restore page
```

---

## Deployment Steps

### Step 1: Build Artifacts

```bash
# Go to project directory
cd C:\Users\cyril\Documents\VScode\buzzcontrol\server-go

# Build Go server
go build -o server.exe ./cmd/server

# Build React frontend
cd web
npm run build
cd ..
```

**Expected Output:**
- `server.exe` : Binary executable
- `web/dist/` : Static files (HTML, CSS, JS)
- Build time: ~2 minutes

### Step 2: Deployment to QUALIF

Option A: Docker Container (recommended)
```bash
# Build portable container
./build.ps1

# Push to QUALIF registry
docker push registry.qualif/buzzcontrol:2.49.4
```

Option B: Direct Binary (QUALIF machine)
```bash
# Copy files
scp server.exe qualif@qualif.local:/app/buzzcontrol/
scp -r web/dist/* qualif@qualif.local:/app/buzzcontrol/web/

# Update config
scp server-go/config.json qualif@qualif.local:/app/buzzcontrol/

# Restart service
ssh qualif@qualif.local "systemctl restart buzzcontrol"
```

Option C: Manual QUALIF (Windows/Raspberry)
```bash
# Stop current service
systemctl stop buzzcontrol
# OR manually kill process

# Wait 2 seconds
timeout 2

# Copy new binary and files
cp server.exe /path/to/qualif/
cp -r web/dist/* /path/to/qualif/web/

# Start new service
./server.exe

# Verify startup
curl http://localhost/admin
```

### Step 3: Verification on QUALIF

```bash
# 1. Check server is running
curl -s http://qualif.local/admin | grep -i "BuzzControl"

# 2. Test admin interface loads
curl -s http://qualif.local/admin | head -20

# 3. Test new Backup/Restore route
curl -s http://qualif.local/admin/backup | grep -i "title"

# 4. Check version
curl -s http://localhost/config.json | grep version
# Expected: "version": "2.49.4"
```

### Step 4: Manual QA on QUALIF

**Test 1: Menu Navigation**
1. Open http://qualif.local/admin
2. Click bee logo (🐝)
3. Verify menu has: Config, Backup/Restaure, Logs
4. Click "Backup/Restaure"
5. Verify URL = http://qualif.local/admin/backup

**Test 2: Page Display**
1. Verify page title: "Sauvegarde et Restauration"
2. Verify 3 sections visible:
   - Sauvegarde (💾)
   - Restauration (📂)
   - Réinitialisation (🔄)
3. Verify checkboxes present in each section

**Test 3: Backup Operation**
1. Select some checkboxes in Sauvegarde section
2. Click "Sauvegarder" button
3. Verify file downloads: `buzzcontrol-backup-YYYY-MM-DD.tar`
4. Check file size > 1KB

**Test 4: Restore Navigation**
1. Verify "Selectionner un fichier" button present
2. Click to verify file picker opens
3. Verify accepts .tar files

**Test 5: Reset Navigation**
1. Verify 5 checkboxes in reset section (unchecked by default)
2. Click checkbox to toggle state
3. Verify state change works

**Test 6: ConfigPage Check**
1. Navigate to /admin/settings
2. Verify Backup/Reset sections are GONE
3. Verify Neon, Server Params, Demo still present
4. Page should load without errors

**Test 7: Responsive Design**
1. Open DevTools (F12)
2. Set viewport to 768px width
3. Navigate to /admin/backup
4. Verify layout changes to single column
5. Verify all elements are readable

### Step 5: Load Testing (Optional)

```bash
# Test with Apache Bench
ab -n 100 -c 10 http://qualif.local/admin/backup

# Expected: <200ms response time
# Expected: 0 errors
```

---

## Rollback Procedure

**If issues detected on QUALIF:**

```bash
# Stop service
systemctl stop buzzcontrol

# Restore previous binary
cp server.exe.backup server.exe

# Restore previous dist
rm -rf web/dist
cp -r web/dist.backup web/dist

# Restart
./server.exe

# Verify old version works
curl http://localhost/config.json | grep version
# Expected: "version": "2.49.3"
```

---

## Features to Test

| Feature | Test URL | Status |
|---------|----------|--------|
| New Backup page | /admin/backup | ✅ Ready |
| Bee menu item | /admin (click bee) | ✅ Ready |
| ConfigPage cleanup | /admin/settings | ✅ Ready |
| Backup checkboxes | /admin/backup | ✅ Ready |
| Reset checkboxes | /admin/backup | ✅ Ready |
| Restore button | /admin/backup | ✅ Ready |

---

## Known Issues & Limitations

### None Critical

**Minor Notes:**
- No toast notifications on success (acceptable for MVP)
- Tests could be more comprehensive (v2.50+)
- Accessibility labels could be enhanced (WCAG AAA standard v2.50+)

---

## Post-Deployment

### 1. Monitor Logs
```bash
tail -f /var/log/buzzcontrol.log

# Look for:
# - "Listening on" messages
# - No "error" or "panic" messages
# - WebSocket connections successful
```

### 2. Check User Reports
- Monitor for issues reported by QUALIF users
- Check backup/restore functionality in production usage
- Monitor file sizes and bandwidth

### 3. Performance Metrics
- Monitor API response times
- Check memory usage (should be stable)
- Check disk space usage

---

## PROD Deployment (After QUALIF Sign-off)

### Timeline
1. QUALIF validation: 1-2 days
2. User acceptance: 1 day
3. PROD deployment: Scheduled maintenance window
4. Monitoring: 24 hours post-deployment

### PROD Process
```bash
# Same steps as QUALIF, but target production servers
# Requires approval from release manager
# Backup old binaries before deployment
```

---

## Checklist - Ready for QUALIF

- [x] Code compiled and tested
- [x] QA validation completed
- [x] CHANGELOG updated
- [x] No regressions detected
- [x] Documentation complete
- [x] Rollback procedure documented
- [x] Test scenarios defined
- [x] All commits reviewed

**Status: READY FOR QUALIF DEPLOYMENT**

---

## Contact & Support

**Questions or Issues:**
- Check CLAUDE.md for project context
- Review CODE_REVIEW_BACKUP_FEATURE.md for technical details
- Check QA_TEST_REPORT_BACKUP.md for test results

**Release Manager**: Contact for PROD approval
