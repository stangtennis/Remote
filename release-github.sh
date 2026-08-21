#!/bin/bash
# Release to GitHub with SHA256 checksums for auto-update
# Usage: ./release-github.sh v2.x.x

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/builds"

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
    echo "Example: $0 v2.61.5"
    exit 1
fi

echo -e "${GREEN}🚀 GitHub Release Script${NC}"
echo "========================="
echo "Version: $VERSION"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) not installed${NC}"
    exit 1
fi

# Check if logged in
if ! gh auth status &> /dev/null; then
    echo -e "${YELLOW}Please login to GitHub:${NC}"
    gh auth login
fi

# Define versioned file names
AGENT_GUI="remote-agent-${VERSION}.exe"
AGENT_CONSOLE="remote-agent-console-${VERSION}.exe"
SUPPORT="remote-support-${VERSION}.exe"
CONTROLLER="controller-${VERSION}.exe"
AGENT_MAC="RemoteDesktopAgent-macOS-${VERSION}.tar.gz"
CONTROLLER_MAC="RemoteDesktopController-macOS-${VERSION}.tar.gz"

# Check if executables exist
MISSING_FILES=0
for FILE in "$AGENT_GUI" "$AGENT_CONSOLE" "$SUPPORT" "$CONTROLLER" "$AGENT_MAC" "$CONTROLLER_MAC"; do
    if [ ! -f "$BUILD_DIR/$FILE" ]; then
        echo -e "${RED}❌ Missing: $BUILD_DIR/$FILE${NC}"
        MISSING_FILES=1
    fi
done

if [ $MISSING_FILES -eq 1 ]; then
    echo -e "${YELLOW}Please build the executables first with the correct version.${NC}"
    exit 1
fi

# Generate SHA256 checksums
echo -e "${YELLOW}Generating SHA256 checksums...${NC}"
cd "$BUILD_DIR"

for FILE in "$AGENT_GUI" "$AGENT_CONSOLE" "$SUPPORT" "$CONTROLLER" "$AGENT_MAC" "$CONTROLLER_MAC"; do
    SHA256_FILE="${FILE}.sha256"
    sha256sum "$FILE" > "$SHA256_FILE"
    echo "  ✅ $SHA256_FILE"
done

cd "$SCRIPT_DIR"

# Get current date
BUILD_DATE=$(date +%Y-%m-%d)

# Create release notes
RELEASE_NOTES="## Remote Desktop $VERSION

**Build Date:** $BUILD_DATE

### Downloads
| File | Description |
|------|-------------|
| \`$AGENT_GUI\` | Windows Agent (GUI mode) |
| \`$AGENT_CONSOLE\` | Windows Agent (Console mode) |
| \`$SUPPORT\` | Portable AI Support client |
| \`$CONTROLLER\` | Windows Controller |
| \`$AGENT_MAC\` | macOS Agent |
| \`$CONTROLLER_MAC\` | macOS Controller |
| \`*.sha256\` | SHA256 checksums for verification |

### Auto-Update
This release supports auto-update. SHA256 checksums are provided for verification.

### Installation
1. Download the appropriate executable
2. Run as Administrator for first-time setup
3. Follow the on-screen instructions

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

# Create release and upload assets (exe + sha256 files)
gh release create "$VERSION" \
    --title "Remote Desktop $VERSION" \
    --notes "$RELEASE_NOTES" \
    "$BUILD_DIR/$AGENT_GUI" \
    "$BUILD_DIR/$AGENT_CONSOLE" \
    "$BUILD_DIR/$SUPPORT" \
    "$BUILD_DIR/$CONTROLLER" \
    "$BUILD_DIR/$AGENT_MAC" \
    "$BUILD_DIR/$CONTROLLER_MAC" \
    "$BUILD_DIR/${AGENT_GUI}.sha256" \
    "$BUILD_DIR/${AGENT_CONSOLE}.sha256" \
    "$BUILD_DIR/${SUPPORT}.sha256" \
    "$BUILD_DIR/${CONTROLLER}.sha256" \
    "$BUILD_DIR/${AGENT_MAC}.sha256" \
    "$BUILD_DIR/${CONTROLLER_MAC}.sha256"

echo -e "\n${GREEN}✅ Release $VERSION created successfully!${NC}"
echo ""
echo "Assets uploaded:"
echo "  - $AGENT_GUI + .sha256"
echo "  - $AGENT_CONSOLE + .sha256"
echo "  - $SUPPORT + .sha256"
echo "  - $CONTROLLER + .sha256"
echo "  - $AGENT_MAC + .sha256"
echo "  - $CONTROLLER_MAC + .sha256"
echo ""
echo "View release: https://github.com/stangtennis/Remote/releases/tag/$VERSION"
