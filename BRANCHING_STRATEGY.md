# Branching Strategy

This repository uses a **simple feature branch workflow** to organize development work.

## Branch Structure

All branches contain the **complete codebase** - just work on the relevant parts!

### **`main`** (Production)
- **Purpose:** Stable, production-ready code
- **Protection:** Should always be stable and tested
- **Merges from:** `agent` and `dashboard` branches

### **`agent`** (Agent Development)
- **Purpose:** Work on the Remote Desktop Agent (Go application)
- **Focus:** Files under `/agent`
- **Contains:** Complete codebase (all files)
- **Usage:**
  ```bash
  git checkout agent
  git pull origin agent
  # Make changes to agent code
  git add agent/
  git commit -m "feat: your agent changes"
  git push origin agent
  ```

### **`dashboard`** (Dashboard Development)
- **Purpose:** Work on the web dashboard and backend
- **Focus:** Files under `/docs` and `/supabase`
- **Contains:** Complete codebase (all files)
- **Usage:**
  ```bash
  git checkout dashboard
  git pull origin dashboard
  # Make changes to dashboard/backend code
  git add docs/ supabase/
  git commit -m "feat: your dashboard changes"
  git push origin dashboard
  ```

### **`controller`** (Controller Application Development) 🆕
- **Purpose:** Work on the standalone controller application
- **Focus:** Files under `/controller`
- **Contains:** Complete codebase (all files)
- **GitHub Actions:** Automatically builds `controller.exe` on push
- **Usage:**
  ```bash
  git checkout controller
  git pull origin controller
  # Make changes to controller code
  git add controller/
  git commit -m "feat: your controller changes"
  git push origin controller
  # GitHub Actions builds EXE automatically
  ```

---

## Workflow

### 1. **Working on Agent**
```bash
git checkout agent
git pull origin agent
# Make your changes
git add agent/
git commit -m "fix: your changes"
git push origin agent
```

### 2. **Working on Dashboard**
```bash
git checkout dashboard
git pull origin dashboard
# Make your changes
git add docs/ supabase/
git commit -m "feat: your changes"
git push origin dashboard
```

### 3. **Working on Controller** 🆕
```bash
git checkout controller
git pull origin controller
# Make your changes
git add controller/
git commit -m "feat: your changes"
git push origin controller
# GitHub Actions builds controller.exe
# Download from Actions → Artifacts
```

### 4. **Merging to Main**

**Option A: Via GitHub Pull Request (Recommended)**
1. Go to https://github.com/stangtennis/Remote
2. Click "Pull Requests" → "New Pull Request"
3. Select `agent` → `main` or `dashboard` → `main`
4. Review changes and merge

**Option B: Command Line**
```bash
# Merge agent to main
git checkout main
git pull origin main
git merge agent
git push origin main

# Merge dashboard to main
git checkout main
git pull origin main
git merge dashboard
git push origin main
```

---

## Commit Message Convention

Use conventional commits format:
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `refactor:` Code refactoring
- `perf:` Performance improvement
- `test:` Tests

**Examples:**
```
feat(agent): add screen capture optimization
fix(dashboard): resolve duplicate device display
docs: update installation instructions
```

---

## Quick Reference

| Task | Command |
|------|---------|
| Switch to agent branch | `git checkout agent` |
| Switch to dashboard branch | `git checkout dashboard` |
| Switch to controller branch | `git checkout controller` 🆕 |
| Switch to main | `git checkout main` |
| View current branch | `git branch` |
| View all branches | `git branch -a` |
| Pull latest changes | `git pull origin <branch-name>` |

---

## Branch URLs

- **Main:** https://github.com/stangtennis/Remote/tree/main
- **Agent:** https://github.com/stangtennis/Remote/tree/agent
- **Dashboard:** https://github.com/stangtennis/Remote/tree/dashboard
- **Controller:** https://github.com/stangtennis/Remote/tree/controller 🆕

---

## GitHub Actions

### Controller Branch
- **Trigger:** Push to `controller` branch
- **Action:** Builds `controller.exe` automatically
- **Download:** Actions tab → Build Controller Application → Artifacts
- **Release:** Tag with `controller-v0.2.0` to create GitHub Release
