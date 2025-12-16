#!/bin/bash
# Build Agent Console installer: EXE (NSIS)
# Run on Ubuntu with nsis installed

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/builds"
VERSION="${1:-2.62.6}"

echo "🔨 Building Agent Console Installer v$VERSION"
echo "=============================================="
echo ""

# Create staging directory
STAGE_DIR="$SCRIPT_DIR/staging-agent-console"
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"

# Build agent console if needed
AGENT_CONSOLE_EXE="$BUILD_DIR/remote-agent-console-v$VERSION.exe"
if [ ! -f "$AGENT_CONSOLE_EXE" ]; then
    echo "📦 Building agent console..."
    cd "$PROJECT_DIR/agent"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
        go build -ldflags '-s -w' -o "$AGENT_CONSOLE_EXE" ./cmd/remote-agent
fi
cp "$AGENT_CONSOLE_EXE" "$STAGE_DIR/remote-agent-console.exe"

# Use cached OpenH264 DLL
OPENH264_CACHE="$BUILD_DIR/openh264-2.1.1-win64.dll"
if [ ! -f "$OPENH264_CACHE" ]; then
    echo "📥 Downloading OpenH264 DLL..."
    OPENH264_URL="https://github.com/cisco/openh264/releases/download/v2.1.1/openh264-2.1.1-win64.dll.bz2"
    TEMP_DIR=$(mktemp -d)
    curl -L -o "$TEMP_DIR/openh264.dll.bz2" "$OPENH264_URL"
    bunzip2 "$TEMP_DIR/openh264.dll.bz2"
    mv "$TEMP_DIR/openh264.dll" "$OPENH264_CACHE"
    rm -rf "$TEMP_DIR"
fi
cp "$OPENH264_CACHE" "$STAGE_DIR/openh264-2.1.1-win64.dll"

# Copy installer files
cp "$SCRIPT_DIR/LICENSE.txt" "$STAGE_DIR/"
cp "$SCRIPT_DIR/agent-console-installer.nsi" "$STAGE_DIR/"

cd "$STAGE_DIR"

echo ""
echo "📦 Creating NSIS EXE installer..."
# Update version in NSIS script
sed -i "s/!define VERSION \".*\"/!define VERSION \"$VERSION\"/" agent-console-installer.nsi
sed -i "s/VIProductVersion \".*\"/VIProductVersion \"$VERSION.0\"/" agent-console-installer.nsi

makensis -V2 agent-console-installer.nsi
EXE_FILE="$BUILD_DIR/RemoteDesktopAgentConsole-$VERSION-Setup.exe"
mv RemoteDesktopAgentConsole-Setup.exe "$EXE_FILE"
echo "✅ EXE: $EXE_FILE"

# Cleanup
cd "$SCRIPT_DIR"
rm -rf "$STAGE_DIR"

echo ""
echo "=========================================="
echo "✅ Agent Console installer created!"
echo ""
echo "Files:"
ls -lh "$BUILD_DIR/RemoteDesktopAgentConsole-$VERSION"* 2>/dev/null || true
echo ""
echo "Upload to GitHub release:"
echo "  gh release upload v$VERSION \"$BUILD_DIR/RemoteDesktopAgentConsole-$VERSION-Setup.exe\""
