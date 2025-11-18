# GitHub Release Guide for v0.3.0

## Step 1: Push Commits and Create Tag

```bash
# Push all commits
git push origin main

# Create and push tag
git tag -a v0.3.0 -m "Phase 3: Fix A4/A5 + Complete Result/Option Implementation"
git push origin v0.3.0
```

## Step 2: Create GitHub Release

### Using GitHub CLI (gh)

```bash
gh release create v0.3.0 \
  --title "Dingo v0.3.0 - Phase 3 Release" \
  --notes-file RELEASE_NOTES_v0.3.0.md \
  release/v0.3.0/*
```

### Using GitHub Web UI

1. Go to https://github.com/MadAppGang/dingo/releases/new

2. **Tag**: `v0.3.0`

3. **Title**: `Dingo v0.3.0 - Phase 3 Release`

4. **Description**: Copy content from `RELEASE_NOTES_v0.3.0.md`

5. **Attach Binaries**:
   - Upload files from `release/v0.3.0/`:
     - `dingo-v0.3.0-darwin-amd64`
     - `dingo-v0.3.0-darwin-arm64`
     - `dingo-v0.3.0-linux-amd64`
     - `dingo-v0.3.0-linux-arm64`
     - `dingo-v0.3.0-windows-amd64.exe`

6. **Release Type**:
   - ☑️ Set as the latest release
   - ☐ Set as a pre-release (uncheck - this is a stable release)

7. Click **"Publish release"**

## Step 3: Verify Release

After publishing:

1. Check release page: https://github.com/MadAppGang/dingo/releases/tag/v0.3.0
2. Test download links for each platform
3. Verify release notes display correctly

## Binary Installation Instructions (for users)

### macOS (ARM - M1/M2/M3)
```bash
curl -L https://github.com/MadAppGang/dingo/releases/download/v0.3.0/dingo-v0.3.0-darwin-arm64 -o dingo
chmod +x dingo
sudo mv dingo /usr/local/bin/
```

### macOS (Intel)
```bash
curl -L https://github.com/MadAppGang/dingo/releases/download/v0.3.0/dingo-v0.3.0-darwin-amd64 -o dingo
chmod +x dingo
sudo mv dingo /usr/local/bin/
```

### Linux (AMD64)
```bash
curl -L https://github.com/MadAppGang/dingo/releases/download/v0.3.0/dingo-v0.3.0-linux-amd64 -o dingo
chmod +x dingo
sudo mv dingo /usr/local/bin/
```

### Linux (ARM64)
```bash
curl -L https://github.com/MadAppGang/dingo/releases/download/v0.3.0/dingo-v0.3.0-linux-arm64 -o dingo
chmod +x dingo
sudo mv dingo /usr/local/bin/
```

### Windows
Download `dingo-v0.3.0-windows-amd64.exe` and add to PATH.

## Post-Release Checklist

- [ ] Push commits and tag
- [ ] Create GitHub release
- [ ] Upload all binaries
- [ ] Test download links
- [ ] Update landing page (optional)
- [ ] Announce on social media (optional)
- [ ] Update README with v0.3.0 features (optional)

---

**Ready to ship!** 🚀
