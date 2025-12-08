#!/bin/bash
# Release to GitHub
# Usage: ./release-github.sh v2.x.x

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
    echo -e "${RED}❌ Error: Version required${NC}"
    echo "Usage: $0 v2.x.x"
    echo ""
    echo "Example: $0 v2.8.0"
    exit 1
fi

echo -e "${GREEN}🚀 GitHub Release Script${NC}"
echo "========================="
echo "Version: $VERSION"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${YELLOW}Installing GitHub CLI...${NC}"
    sudo apt-get update && sudo apt-get install -y gh
fi

# Check if logged in
if ! gh auth status &> /dev/null; then
    echo -e "${YELLOW}Please login to GitHub:${NC}"
    gh auth login
fi

# Check if executables exist
if [ ! -f "$BUILD_DIR/remote-agent.exe" ] || [ ! -f "$BUILD_DIR/controller.exe" ]; then
    echo -e "${YELLOW}Building executables first...${NC}"
    "$SCRIPT_DIR/build-windows.sh" all
fi

# Get current date
BUILD_DATE=$(date +%Y-%m-%d)

# Create release notes
RELEASE_NOTES="## Remote Desktop $VERSION

**Build Date:** $BUILD_DATE

### Downloads
- **remote-agent.exe** - Windows Agent (install on remote PCs)
- **controller.exe** - Windows Controller (admin application)

### Installation
1. Download the appropriate executable
2. Run as Administrator for first-time setup
3. Follow the on-screen instructions

### Changes
- Persistent device ID based on Windows MachineGUID
- Improved login dialog with better feedback
- Thread-safe UI updates

See [README.md](https://github.com/stangtennis/Remote/blob/main/README.md) for full documentation.
"

echo -e "${YELLOW}Creating GitHub release $VERSION...${NC}"

# Navigate to repo
cd "$SCRIPT_DIR"

# Create tag if it doesn't exist
if ! git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "Creating git tag $VERSION..."
    git tag -a "$VERSION" -m "Release $VERSION"
    git push origin "$VERSION"
fi

# Create release and upload assets
gh release create "$VERSION" \
    --title "Remote Desktop $VERSION" \
    --notes "$RELEASE_NOTES" \
    "$BUILD_DIR/remote-agent.exe" \
    "$BUILD_DIR/controller.exe"

echo -e "\n${GREEN}✅ Release $VERSION created successfully!${NC}"
echo ""
echo "View release: https://github.com/stangtennis/Remote/releases/tag/$VERSION"
