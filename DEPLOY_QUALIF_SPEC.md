# Deploy QUALIF Specification - Auto-Update v2.50.0

**Date** : 2026-02-01
**Feature** : Auto-Update Server (v2.50.0)
**Target** : QUALIF Environment
**Agent** : deploy
**Branch** : feature/auto-update

---

## Overview

Build, package, and prepare deployment artifacts for QUALIF testing environment.

---

## Part 1 : Build Binaries

### Backend Build (Go)

**Windows Build (amd64)**

```bash
cd server-go
GOOS=windows GOARCH=amd64 go build -o buzzcontrol-v2.50.0-windows-amd64.exe ./cmd/server
```

**Expected** :
- Executable file: `buzzcontrol-v2.50.0-windows-amd64.exe`
- Size: ~45-50 MB (typical)
- No errors or warnings

**Verify** :
```bash
file buzzcontrol-v2.50.0-windows-amd64.exe
# Should show: PE32+ executable
```

---

**Linux ARM64 Build (Raspberry Pi)**

```bash
cd server-go
GOOS=linux GOARCH=arm64 go build -o buzzcontrol-v2.50.0-linux-arm64 ./cmd/server
```

**Expected** :
- Executable file: `buzzcontrol-v2.50.0-linux-arm64`
- Size: ~45-50 MB (typical)
- No errors or warnings

**Verify** :
```bash
file buzzcontrol-v2.50.0-linux-arm64
# Should show: ELF 64-bit LSB executable, ARM aarch64
```

---

### Frontend Build (React)

**Command** :
```bash
cd server-go/web
npm run build
```

**Expected** :
- Build directory: `server-go/web/build/`
- Bundled assets: index.html, static/js/*, static/css/*
- Size: ~3-5 MB (gzipped)

**Verify** :
```bash
ls -la server-go/web/build/
# Should contain: index.html, static/
```

---

### Frontend Bundle Into Binary

The binary must include frontend assets (either embedded or accessible).

**Method 1: Embedded Assets** (Recommended)
```bash
# Use embed directive in Go (1.16+)
//go:embed web/build/*
var frontendAssets embed.FS

# Register in HTTP server
http.FileServer(http.FS(frontendAssets))
```

**Method 2: External Assets**
- Deploy frontend assets separately
- Configure web root in config.json
- Document in QUALIF README

---

## Part 2 : Generate Checksums

**Compute SHA256 for all binaries**

```bash
# Windows
sha256sum buzzcontrol-v2.50.0-windows-amd64.exe > checksums.txt

# Linux ARM64
sha256sum buzzcontrol-v2.50.0-linux-arm64 >> checksums.txt
```

**Expected checksums.txt** :
```
abc1234567890def... buzzcontrol-v2.50.0-windows-amd64.exe
def0987654321abc... buzzcontrol-v2.50.0-linux-arm64
```

**Verify on target system** :
```bash
sha256sum -c checksums.txt
# Should show: OK for each file
```

---

## Part 3 : Create Deployment Archives

### Windows Archive

**Structure** :
```
buzzcontrol-v2.50.0-qualif-windows/
├── buzzcontrol-v2.50.0-windows-amd64.exe
├── config.json (template)
├── QUALIF_README.md
├── DEPLOYMENT_GUIDE.md
├── ROLLBACK.md
└── checksums.txt
```

**Create Archive** :
```bash
mkdir -p buzzcontrol-v2.50.0-qualif-windows
cp buzzcontrol-v2.50.0-windows-amd64.exe buzzcontrol-v2.50.0-qualif-windows/
cp server-go/config.json buzzcontrol-v2.50.0-qualif-windows/config.json.template
cp QUALIF_README.md buzzcontrol-v2.50.0-qualif-windows/
cp DEPLOYMENT_GUIDE.md buzzcontrol-v2.50.0-qualif-windows/
cp ROLLBACK.md buzzcontrol-v2.50.0-qualif-windows/
cp checksums.txt buzzcontrol-v2.50.0-qualif-windows/

zip -r buzzcontrol-v2.50.0-qualif-windows.zip buzzcontrol-v2.50.0-qualif-windows/
```

**Expected** :
- File: `buzzcontrol-v2.50.0-qualif-windows.zip`
- Size: ~45-50 MB
- Extractable on Windows

---

### ARM64 Archive (Raspberry Pi)

**Structure** :
```
buzzcontrol-v2.50.0-qualif-arm64/
├── buzzcontrol-v2.50.0-linux-arm64
├── config.json (template)
├── QUALIF_README.md
├── DEPLOYMENT_GUIDE.md
├── ROLLBACK.md
├── checksums.txt
└── systemd-service.sh (optional: create systemd service)
```

**Create Archive** :
```bash
mkdir -p buzzcontrol-v2.50.0-qualif-arm64
cp buzzcontrol-v2.50.0-linux-arm64 buzzcontrol-v2.50.0-qualif-arm64/
chmod +x buzzcontrol-v2.50.0-qualif-arm64/buzzcontrol-v2.50.0-linux-arm64
cp server-go/config.json buzzcontrol-v2.50.0-qualif-arm64/config.json.template
cp QUALIF_README.md buzzcontrol-v2.50.0-qualif-arm64/
cp DEPLOYMENT_GUIDE.md buzzcontrol-v2.50.0-qualif-arm64/
cp ROLLBACK.md buzzcontrol-v2.50.0-qualif-arm64/
cp checksums.txt buzzcontrol-v2.50.0-qualif-arm64/

tar -czf buzzcontrol-v2.50.0-qualif-arm64.tar.gz buzzcontrol-v2.50.0-qualif-arm64/
```

**Expected** :
- File: `buzzcontrol-v2.50.0-qualif-arm64.tar.gz`
- Size: ~45-50 MB
- Extractable on Linux

---

## Part 4 : Create Deployment Documentation

### QUALIF_README.md

```markdown
# BuzzControl Auto-Update v2.50.0 - QUALIF Deployment

## What's New

- **Feature**: Automatic server updates from GitHub releases
- **Components**: Backend API + Frontend UI + Navbar badge
- **Auto-check**: Configurable updates on startup

## Pre-Deployment Checklist

- [ ] Backup current production data
- [ ] Verify checksums before deployment
- [ ] Plan rollback strategy
- [ ] Notify users of maintenance window

## Deployment Steps (Windows)

1. Extract `buzzcontrol-v2.50.0-qualif-windows.zip`
2. Verify checksums:
   ```bash
   sha256sum -c checksums.txt
   ```
3. Copy config.json.template → config.json
4. Configure settings (port, paths, etc.)
5. Stop current server
6. Run new binary: `./buzzcontrol-v2.50.0-windows-amd64.exe`
7. Verify in browser: http://localhost

## Deployment Steps (Raspberry Pi)

1. Extract archive:
   ```bash
   tar -xzf buzzcontrol-v2.50.0-qualif-arm64.tar.gz
   ```
2. Verify checksums:
   ```bash
   sha256sum -c checksums.txt
   ```
3. Copy config template: `cp config.json.template config.json`
4. Make executable:
   ```bash
   chmod +x buzzcontrol-v2.50.0-linux-arm64
   ```
5. Stop current service (systemd or manual)
6. Run binary:
   ```bash
   ./buzzcontrol-v2.50.0-linux-arm64
   ```
7. Verify in browser: http://raspberry-pi-ip

## Testing Checklist

### Server Startup
- [ ] Server starts without errors
- [ ] Web interface loads
- [ ] Version shows 2.50.0

### Update Feature
- [ ] Navbar badge appears (if update available)
- [ ] UpdatePage loads with versions
- [ ] Can select and download version
- [ ] Download progress shows
- [ ] Can apply and restart
- [ ] Server restarts successfully
- [ ] New version active after restart

### Error Scenarios
- [ ] GitHub unreachable → graceful error
- [ ] Invalid version → proper validation
- [ ] Download fails → retry option
- [ ] Game in progress → warning shown

### Existing Features
- [ ] Game engine works
- [ ] Questions load
- [ ] Teams/players function
- [ ] WebSocket active
- [ ] TV display works
- [ ] Logs accessible

## Rollback Procedure

See ROLLBACK.md for detailed rollback steps.

Quick rollback:
1. Restore previous binary from backup
2. Restart server
3. Verify functionality

## Support

For issues:
1. Check DEPLOYMENT_GUIDE.md
2. Review logs in web interface (/admin/logs)
3. Report issues with details in CHANGELOG.md

## Version History

- v2.50.0: Auto-update feature
- v2.49.0: Player cards styling
- (see CHANGELOG.md for full history)
```

---

### DEPLOYMENT_GUIDE.md

Detailed deployment instructions with:
- System requirements
- Disk space needed
- Network connectivity
- Backup procedures
- Step-by-step deployment
- Verification commands
- Troubleshooting

---

### ROLLBACK.md

Detailed rollback instructions with:
- Pre-rollback checklist
- Step-by-step rollback
- Restore game state if needed
- Verification after rollback
- When to contact support

---

## Part 5 : Build Artifacts Verification

**Checklist** :

- [ ] Windows executable runs (can test on CI)
- [ ] ARM64 executable runs (if cross-compile verified)
- [ ] Frontend bundled correctly
- [ ] Config template valid JSON
- [ ] Checksums computed correctly
- [ ] Archives extract without errors
- [ ] Documentation complete
- [ ] All files readable

**Verification Script** :
```bash
#!/bin/bash
# Verify deployment package

echo "=== Verification ==="

# Check binaries exist
[ -f buzzcontrol-v2.50.0-windows-amd64.exe ] || echo "FAIL: Windows binary"
[ -f buzzcontrol-v2.50.0-linux-arm64 ] || echo "FAIL: ARM64 binary"

# Check sizes are reasonable
windows_size=$(stat -f%z buzzcontrol-v2.50.0-windows-amd64.exe 2>/dev/null || stat -c%s buzzcontrol-v2.50.0-windows-amd64.exe)
arm_size=$(stat -f%z buzzcontrol-v2.50.0-linux-arm64 2>/dev/null || stat -c%s buzzcontrol-v2.50.0-linux-arm64)

[ $windows_size -gt 40000000 ] && echo "OK: Windows size ($windows_size bytes)" || echo "FAIL: Windows size"
[ $arm_size -gt 40000000 ] && echo "OK: ARM64 size ($arm_size bytes)" || echo "FAIL: ARM64 size"

# Verify checksums
sha256sum -c checksums.txt || echo "FAIL: Checksums"

# Check archives
[ -f buzzcontrol-v2.50.0-qualif-windows.zip ] && echo "OK: Windows archive" || echo "FAIL: Windows archive"
[ -f buzzcontrol-v2.50.0-qualif-arm64.tar.gz ] && echo "OK: ARM64 archive" || echo "FAIL: ARM64 archive"

echo "=== Verification Complete ==="
```

---

## Part 6 : Deployment Package Contents

**Final Package Structure** :

```
qualif-deployment/
├── buzzcontrol-v2.50.0-qualif-windows.zip
├── buzzcontrol-v2.50.0-qualif-arm64.tar.gz
├── checksums.txt
├── SHA256SUMS.asc (signed, if applicable)
├── QUALIF_README.md
├── DEPLOYMENT_GUIDE.md
├── ROLLBACK.md
├── DEPLOYMENT_LOG.txt (generated during deploy)
└── manifest.json
    {
      "version": "2.50.0",
      "feature": "auto-update",
      "files": {
        "windows": "buzzcontrol-v2.50.0-qualif-windows.zip",
        "arm64": "buzzcontrol-v2.50.0-qualif-arm64.tar.gz"
      },
      "checksums": "checksums.txt",
      "date": "2026-02-01",
      "tested": true
    }
```

---

## Part 7 : QUALIF Test Plan

Create QUALIF_TEST_PLAN.md with:

### Smoke Tests (15 minutes)
1. Server starts
2. Web UI loads
3. Version shows 2.50.0

### Feature Tests (30 minutes)
1. Update badge appears
2. UpdatePage accessible
3. Download functionality
4. Apply and restart
5. New version active

### Regression Tests (30 minutes)
1. Game engine works
2. Existing UI intact
3. WebSocket functional
4. Data persists

### Load Tests (optional)
1. Multiple clients
2. Download under load
3. Concurrent API calls

### Error Tests
1. GitHub unreachable
2. Corrupted download
3. Invalid version
4. Server restart timeout

---

## Part 8 : Deployment Checklist

**Pre-Deployment** :
- [ ] All tests passing
- [ ] Code reviewed
- [ ] Documentation complete
- [ ] Backups created
- [ ] Rollback procedure verified
- [ ] Communication sent to users

**During Deployment** :
- [ ] Download package from secure location
- [ ] Verify checksums
- [ ] Follow deployment guide step-by-step
- [ ] Monitor logs during startup
- [ ] Verify each component works
- [ ] Document any issues

**Post-Deployment** :
- [ ] Confirm version is 2.50.0
- [ ] Test all update features
- [ ] Monitor for 24 hours
- [ ] Collect user feedback
- [ ] Update deployment log

---

## Part 9 : Release Notes

**Content for Release Notes** :

```markdown
# BuzzControl v2.50.0 - Auto-Update Feature Release

## Release Date
2026-02-01

## New Features
- Automatic server update detection from GitHub releases
- Update management interface in admin panel
- Download version binaries securely
- Apply updates with graceful restart
- Game state preservation during restart

## User Interface
- Notification badge in navbar when update available
- UpdatePage with version list and release notes
- Download progress indicator
- Apply and restart confirmation
- Warning if game in progress

## Technical Details
- Backend: 4 new REST endpoints
- Frontend: React hook and component
- Auto-check configuration option
- GitHub Releases API integration with caching
- Atomic file operations with backup/rollback

## Supported Platforms
- Windows (amd64)
- Linux/Raspberry Pi (arm64)

## Upgrade Path
Simply run the new binary. Game state is preserved.

## Rollback
If issues occur, restore previous binary (included in package).

## Testing
Tested with:
- Unit tests: Go + React
- Manual scenarios: Update workflow
- Regression: Existing features

## Known Issues
- None at release

## Future Enhancements
- Scheduled automatic updates (v2.51)
- GitHub webhooks for instant notifications (v2.51)
- Checksum verification improvements (v2.51)
```

---

## Deployment Execution

**Command** :
```bash
./deploy.sh qualif v2.50.0
```

Or manual:
```bash
cd server-go
go build -o qualif-artifacts/buzzcontrol-v2.50.0-windows-amd64.exe ./cmd/server
# ... (rest of build steps)
```

**Expected Output** :
```
=== Building Artifacts ===
Building Windows binary... OK (45MB)
Building ARM64 binary... OK (46MB)
Building frontend... OK (3MB)
Computing checksums... OK
Creating archives... OK
Generating documentation... OK
=== Deployment Package Ready ===
Location: qualif-deployment/
Size: 95MB
Ready for QUALIF deployment
```

---

## Sign-Off

Deployment ready when:

```
Deployed By: [deploy agent]
Date: [YYYY-MM-DD]
Version: 2.50.0
Status: READY FOR QUALIF
Artifacts:
  - buzzcontrol-v2.50.0-qualif-windows.zip
  - buzzcontrol-v2.50.0-qualif-arm64.tar.gz
  - checksums.txt
  - Documentation
Testing: All scenarios verified
Rollback: Procedure documented and tested
```

---

## Next Steps

1. **QUALIF Testing** : Deploy to QUALIF, execute test plan
2. **User Acceptance** : Manual validation by users
3. **Bug Fixes** : If issues found, fix and create v2.50.1
4. **Sign-Off** : QUALIF team approves
5. **Production Deploy** : Execute `/deploy PROD` (separate manual step)

---

## Timeline

- Build: 10 minutes
- Packaging: 5 minutes
- Documentation: 15 minutes
- Verification: 10 minutes
- **Total** : ~40 minutes

