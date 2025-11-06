# How to Create GitHub Release v2.0.0

## Steps to Create Release on GitHub

1. **Go to GitHub Releases Page**
   - Navigate to: https://github.com/stangtennis/Remote/releases
   - Click "Draft a new release"

2. **Configure Release**
   - **Tag**: Select `v2.0.0` (already pushed)
   - **Title**: `v2.0.0 - Maximum Quality Update (2025-11-06)`
   - **Description**: Copy content from `RELEASE_NOTES_v2.0.0.md`

3. **Upload Binaries**
   Upload these files:
   - `agent/remote-agent.exe` (Agent executable)
   - `controller/controller.exe` (Controller executable)

4. **Publish**
   - Click "Publish release"

## File Locations

- **Agent**: `F:\#Remote\agent\remote-agent.exe`
- **Controller**: `F:\#Remote\controller\controller.exe`
- **Release Notes**: `F:\#Remote\RELEASE_NOTES_v2.0.0.md`

## Quick Links

- Repository: https://github.com/stangtennis/Remote
- Releases: https://github.com/stangtennis/Remote/releases
- New Release: https://github.com/stangtennis/Remote/releases/new

## Alternative: Using GitHub CLI

If you install GitHub CLI (https://cli.github.com/), you can run:

```bash
gh release create v2.0.0 \
  --title "v2.0.0 - Maximum Quality Update (2025-11-06)" \
  --notes-file RELEASE_NOTES_v2.0.0.md \
  agent\remote-agent.exe \
  controller\controller.exe
```
