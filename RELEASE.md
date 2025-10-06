# Release Process

This document explains how to create new releases of the Remote Desktop Agent.

## 🚀 Creating a New Release

Releases are **fully automated** via GitHub Actions. Just push a version tag!

### **Steps:**

1. **Make sure all changes are committed and pushed:**
   ```bash
   git checkout agent  # or main
   git add .
   git commit -m "feat: your changes"
   git push origin agent
   ```

2. **Create and push a version tag:**
   ```bash
   # Merge to main first (if working on agent branch)
   git checkout main
   git merge agent
   git push origin main
   
   # Create version tag (use semantic versioning)
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **GitHub Actions automatically:**
   - ✅ Builds the agent with GCC/MinGW
   - ✅ Creates a GitHub Release
   - ✅ Uploads `remote-agent.exe`
   - ✅ Creates a ZIP with installation scripts
   - ✅ Generates release notes

4. **Release is live!**
   - View at: https://github.com/stangtennis/Remote/releases
   - Direct download link: `https://github.com/stangtennis/Remote/releases/download/v1.0.0/remote-agent.exe`

---

## 📋 Version Naming (Semantic Versioning)

Use the format: `vMAJOR.MINOR.PATCH`

- **MAJOR** (v**1**.0.0) - Breaking changes, major rewrites
- **MINOR** (v1.**1**.0) - New features, non-breaking changes
- **PATCH** (v1.0.**1**) - Bug fixes, small improvements

### Examples:

```bash
# First stable release
git tag v1.0.0

# New feature added
git tag v1.1.0

# Bug fix
git tag v1.0.1

# Major rewrite
git tag v2.0.0
```

---

## 🔄 Pre-releases (Optional)

For testing versions before official release:

```bash
# Beta release
git tag v1.0.0-beta.1
git push origin v1.0.0-beta.1

# Release candidate
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
```

---

## 🧹 Deleting a Tag (If Mistake)

**Local:**
```bash
git tag -d v1.0.0
```

**Remote:**
```bash
git push origin :refs/tags/v1.0.0
```

**Then delete the release manually on GitHub.**

---

## 📦 What Gets Released

Each release includes:

1. **`remote-agent.exe`** - Standalone Windows executable
2. **`remote-agent-windows.zip`** - Complete package with:
   - `remote-agent.exe`
   - `setup-startup.bat`
   - `uninstall-service.bat`
   - `view-logs.bat`
   - `.env.example`

---

## ✅ Checklist Before Release

- [ ] All tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if exists)
- [ ] Version number follows semantic versioning
- [ ] Tested on target Windows version
- [ ] All commits pushed to main branch

---

## 📊 Monitoring Releases

- **View releases:** https://github.com/stangtennis/Remote/releases
- **View workflow runs:** https://github.com/stangtennis/Remote/actions
- **Download statistics:** Available on each release page

---

## 🛠️ Troubleshooting

### Build fails?
- Check the Actions log: https://github.com/stangtennis/Remote/actions
- Ensure Go code compiles locally first
- Verify MinGW/GCC dependencies

### Release not created?
- Check tag format (must be `v*.*.*`)
- Verify GitHub Actions is enabled in repo settings
- Check workflow file syntax

---

## 📝 Release Notes Template

GitHub Actions automatically generates release notes, but you can edit them after creation to add:

```markdown
## 🎉 Version X.Y.Z

### ✨ New Features
- Feature 1
- Feature 2

### 🐛 Bug Fixes
- Fix 1
- Fix 2

### ⚡ Performance
- Optimization 1

### 📚 Documentation
- Doc update 1
```

---

**For questions, see the [GitHub Actions workflow](.github/workflows/release-agent.yml).**
