# Prompt pour deploy : QUALIF Deployment

## Contexte

You are the Deployment agent for BuzzControl auto-update feature (v2.50.0).

Your task is to build, package, and prepare deployment to QUALIF environment.

## Current State

- **Branch** : feature/auto-update
- **Version** : 2.50.0 (ready for QUALIF)
- **Backend** : COMPLETED and tested
- **Frontend** : COMPLETED and tested
- **QA** : COMPLETED - all validations passed
- **Documentation** : COMPLETED

## Deployment Tasks

### 1. Build Binaries

**Windows (amd64)**
```bash
cd server-go
GOOS=windows GOARCH=amd64 go build -o buzzcontrol-v2.50.0-windows-amd64.exe ./cmd/server
```

**Linux ARM64 (Raspberry Pi)**
```bash
cd server-go
GOOS=linux GOARCH=arm64 go build -o buzzcontrol-v2.50.0-linux-arm64 ./cmd/server
```

**Frontend**
```bash
cd server-go/web
npm run build
```

### 2. Generate Checksums

```bash
sha256sum buzzcontrol-v2.50.0-windows-amd64.exe > checksums.txt
sha256sum buzzcontrol-v2.50.0-linux-arm64 >> checksums.txt
```

### 3. Create Deployment Archives

**Windows Package**
- buzzcontrol-v2.50.0-qualif-windows.zip
- Contains: binary, config template, documentation

**ARM64 Package**
- buzzcontrol-v2.50.0-qualif-arm64.tar.gz
- Contains: binary, config template, documentation

### 4. Create Documentation

- QUALIF_README.md (deployment guide)
- DEPLOYMENT_GUIDE.md (detailed steps)
- ROLLBACK.md (rollback procedure)
- QUALIF_TEST_PLAN.md (test procedures)

### 5. Verify Artifacts

- [ ] Binaries executable
- [ ] Frontend bundled
- [ ] Checksums valid
- [ ] Archives extractable
- [ ] Documentation complete

## Expected Artifacts

```
qualif-deployment/
├── buzzcontrol-v2.50.0-qualif-windows.zip (~45MB)
├── buzzcontrol-v2.50.0-qualif-arm64.tar.gz (~45MB)
├── checksums.txt
├── QUALIF_README.md
├── DEPLOYMENT_GUIDE.md
├── ROLLBACK.md
└── QUALIF_TEST_PLAN.md
```

## Quality Checks

- [ ] Binaries > 40MB (includes frontend)
- [ ] Checksums match
- [ ] Archives extract cleanly
- [ ] Config templates valid
- [ ] Documentation complete
- [ ] No build warnings

## Deliverables

1. **Deployment Package** - Ready for QUALIF
2. **Build Log** - Build steps executed
3. **Checksums** - SHA256 hashes
4. **Documentation** - Complete deployment guide
5. **Test Plan** - QUALIF testing procedure
6. **Manifest** - Package contents and version info

## Version

- **Version**: 2.50.0
- **Date**: 2026-02-01
- **Target**: QUALIF
- **Branch**: feature/auto-update

## Timeline

- Build: 10 minutes
- Packaging: 5 minutes
- Documentation: 15 minutes
- Verification: 10 minutes
- **Total**: ~40 minutes

## References

- **Full spec**: DEPLOY_QUALIF_SPEC.md
- **Build script**: Can reference existing build.ps1
- **Version**: 2.50.0

## Sign-Off

When complete, provide:
- Build status (SUCCESS)
- Artifacts location
- Checksums
- Deployment instructions
- Ready for QUALIF: YES

---

**Status**: READY FOR DEPLOYMENT

Build artifacts and prepare QUALIF package.

