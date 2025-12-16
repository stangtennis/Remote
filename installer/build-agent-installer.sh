#!/bin/bash
# Build Agent installer: ZIP and EXE (NSIS)
# Run on Ubuntu with nsis installed

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/builds"
VERSION="${1:-2.62.5}"

echo "🔨 Building Agent Installer v$VERSION"
echo "====================================="
echo ""

# Create staging directory
STAGE_DIR="$SCRIPT_DIR/staging-agent"
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"

# Build agent GUI if needed
AGENT_EXE="$BUILD_DIR/remote-agent-v$VERSION.exe"
if [ ! -f "$AGENT_EXE" ]; then
    echo "📦 Building agent GUI..."
    cd "$PROJECT_DIR/agent"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
        go build -ldflags '-s -w -H windowsgui' -o "$AGENT_EXE" ./cmd/remote-agent
fi
cp "$AGENT_EXE" "$STAGE_DIR/remote-agent.exe"

# Build agent console if needed
AGENT_CONSOLE_EXE="$BUILD_DIR/remote-agent-console-v$VERSION.exe"
if [ ! -f "$AGENT_CONSOLE_EXE" ]; then
    echo "📦 Building agent console..."
    cd "$PROJECT_DIR/agent"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
        go build -ldflags '-s -w' -o "$AGENT_CONSOLE_EXE" ./cmd/remote-agent
fi
cp "$AGENT_CONSOLE_EXE" "$STAGE_DIR/remote-agent-console.exe"

# Download OpenH264 DLL if not cached (v2.1.1 is latest available)
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
cp "$SCRIPT_DIR/agent-installer.nsi" "$STAGE_DIR/"

cd "$STAGE_DIR"

echo ""
echo "📦 Creating Agent ZIP..."
ZIP_FILE="$BUILD_DIR/RemoteDesktopAgent-$VERSION-win64.zip"
TEMP_ZIP=$(mktemp -d)
mkdir -p "$TEMP_ZIP/RemoteDesktopAgent-$VERSION"
cp remote-agent.exe remote-agent-console.exe openh264-2.1.1-win64.dll "$TEMP_ZIP/RemoteDesktopAgent-$VERSION/"
cat > "$TEMP_ZIP/RemoteDesktopAgent-$VERSION/README.txt" << EOF
Remote Desktop Agent v$VERSION
===================================

INSTALLATION:
1. Extract this ZIP to any folder
2. Run remote-agent.exe

FILER:
- remote-agent.exe        : GUI version (system tray)
- remote-agent-console.exe: Console version (med logs)
- openh264-2.4.1-win64.dll: H.264 encoder

AUTO-OPDATERING:
Agent tjekker automatisk for opdateringer via tray menu.

KRAV:
- Windows 10/11 64-bit
- Admin rettigheder (til firewall regler)

SUPPORT:
https://github.com/stangtennis/Remote
EOF
cd "$TEMP_ZIP"
zip -r "$ZIP_FILE" "RemoteDesktopAgent-$VERSION"
rm -rf "$TEMP_ZIP"
echo "✅ ZIP: $ZIP_FILE"

cd "$STAGE_DIR"

echo ""
echo "📦 Creating NSIS EXE installer..."
# Update version in NSIS script
sed -i "s/!define VERSION \".*\"/!define VERSION \"$VERSION\"/" agent-installer.nsi
sed -i "s/VIProductVersion \".*\"/VIProductVersion \"$VERSION.0\"/" agent-installer.nsi

makensis -V2 agent-installer.nsi
EXE_FILE="$BUILD_DIR/RemoteDesktopAgent-$VERSION-Setup.exe"
mv RemoteDesktopAgent-Setup.exe "$EXE_FILE"
echo "✅ EXE: $EXE_FILE"

# Cleanup
cd "$SCRIPT_DIR"
rm -rf "$STAGE_DIR"

echo ""
echo "=========================================="
echo "✅ Agent installers created!"
echo ""
echo "Files:"
ls -lh "$BUILD_DIR/RemoteDesktopAgent-$VERSION"* 2>/dev/null || true
echo ""
echo "Upload to GitHub release:"
echo "  gh release upload v$VERSION \\"
echo "    \"$BUILD_DIR/RemoteDesktopAgent-$VERSION-win64.zip\" \\"
echo "    \"$BUILD_DIR/RemoteDesktopAgent-$VERSION-Setup.exe\""
