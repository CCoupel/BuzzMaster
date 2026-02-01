# Prompt pour deploy : PROD Deployment v2.50.0

## Contexte

You are the Deployment agent for BuzzControl auto-update feature final PROD release.

Your task is to merge feature branch, create release, and deploy to production.

## Current State

- **Branch**: feature/auto-update
- **Internal Version**: v2.50.5 (QUALIF validated, documented)
- **Release Version**: v2.50.0 (feature version)
- **Status**: Documentation complete, ready for PROD

## Deployment Process

### Step 1: Merge to Main

**Option A: Squash Merge** (Recommended)
```bash
git checkout main
git pull origin main
git merge --squash feature/auto-update
git commit -m "feat(auto-update): Add automatic server update feature (v2.50.0)"
git push origin main
```

**Option B: Regular Merge**
```bash
git checkout main
git pull origin main
git merge feature/auto-update
git push origin main
```

**Recommendation**: Squash merge for clean history

### Step 2: Create Release Tag

```bash
git tag -a v2.50.0 -m "Release v2.50.0: Auto-Update Feature

- Complete server auto-update system
- GitHub Releases integration
- Safe update with backup/rollback
- Frontend UI with update management
- Configurable automatic checks"

git push origin v2.50.0
```

### Step 3: Build PROD Binaries

**Windows**
```bash
cd server-go
GOOS=windows GOARCH=amd64 go build -o buzzcontrol-v2.50.0-windows-amd64.exe ./cmd/server
```

**Linux ARM64**
```bash
GOOS=linux GOARCH=arm64 go build -o buzzcontrol-v2.50.0-linux-arm64 ./cmd/server
```

### Step 4: Generate Checksums

```bash
sha256sum buzzcontrol-v2.50.0-windows-amd64.exe > checksums.txt
sha256sum buzzcontrol-v2.50.0-linux-arm64 >> checksums.txt
```

### Step 5: Create GitHub Release

Use GitHub CLI or web interface:

```bash
gh release create v2.50.0 \
  --title "BuzzControl v2.50.0 - Auto-Update Feature" \
  --notes "$(cat RELEASE_NOTES_v2.50.0.md)" \
  buzzcontrol-v2.50.0-windows-amd64.exe \
  buzzcontrol-v2.50.0-linux-arm64 \
  checksums.txt
```

Or manually:
1. Go to https://github.com/CCoupel/BuzzMaster/releases
2. Click "Create a new release"
3. Tag: v2.50.0
4. Title: BuzzControl v2.50.0 - Auto-Update Feature
5. Description: (from RELEASE_NOTES_v2.50.0.md)
6. Attach binaries and checksums
7. Publish

## Deliverables

- [ ] main branch updated with feature/auto-update
- [ ] Release tag v2.50.0 created
- [ ] GitHub release published
- [ ] Binaries uploaded
- [ ] Checksums verified
- [ ] Release notes visible

## Success Criteria

- [ ] git log shows clean history
- [ ] Tag v2.50.0 exists on main
- [ ] GitHub release shows v2.50.0
- [ ] Binaries downloadable
- [ ] Checksums match
- [ ] Release notes visible

## After Deployment

1. **Communication**: Announce v2.50.0 release
2. **Documentation**: Update installation docs
3. **Support**: Be ready for user questions
4. **Monitoring**: Monitor for issues
5. **Backlog**: Move feature to DONE (if not already)

## Version References

- Major Feature: Auto-Update (v2.50.0)
- Documentation: CHANGELOG, ADMIN_GUIDE, CLAUDE.md
- Release Notes: RELEASE_NOTES_v2.50.0.md
- Binaries: Windows amd64 + Linux ARM64

## Sign-Off

When complete:
- [ ] Merged to main
- [ ] Tagged v2.50.0
- [ ] Release published
- [ ] Ready for users to download

