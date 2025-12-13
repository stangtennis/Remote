#!/bin/bash
# Create ZIP distribution with FFmpeg bundled
# This is simpler than NSIS installer

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/builds"
VERSION="${1:-2.59.0}"

echo "🔨 Creating ZIP distribution v$VERSION"
echo "======================================="

# Create temp directory
TEMP_DIR=$(mktemp -d)
RELEASE_DIR="$TEMP_DIR/RemoteDesktopController-$VERSION"
mkdir -p "$RELEASE_DIR"

# Build controller if not exists
CONTROLLER_EXE="$BUILD_DIR/controller-v$VERSION.exe"
if [ ! -f "$CONTROLLER_EXE" ]; then
    echo "📦 Building controller..."
    cd "$PROJECT_DIR/controller"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
        go build -ldflags '-s -w -H windowsgui' -o "$CONTROLLER_EXE" .
fi

# Copy controller
cp "$CONTROLLER_EXE" "$RELEASE_DIR/controller.exe"

# Download FFmpeg if not cached
FFMPEG_CACHE="$BUILD_DIR/ffmpeg-win64.exe"
if [ ! -f "$FFMPEG_CACHE" ]; then
    echo "📥 Downloading FFmpeg (one-time)..."
    FFMPEG_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
    FFMPEG_ZIP="$TEMP_DIR/ffmpeg.zip"
    
    curl -L -o "$FFMPEG_ZIP" "$FFMPEG_URL"
    
    echo "📦 Extracting FFmpeg..."
    cd "$TEMP_DIR"
    unzip -q "$FFMPEG_ZIP"
    
    # Find and cache ffmpeg.exe
    find . -name "ffmpeg.exe" -type f | head -1 | xargs -I {} cp {} "$FFMPEG_CACHE"
    
    echo "✅ FFmpeg cached at $FFMPEG_CACHE"
fi

# Copy FFmpeg
cp "$FFMPEG_CACHE" "$RELEASE_DIR/ffmpeg.exe"

# Create README
cat > "$RELEASE_DIR/README.txt" << 'EOF'
Remote Desktop Controller v$VERSION
===================================

INSTALLATION:
1. Extract this ZIP to any folder
2. Run controller.exe

REQUIREMENTS:
- Windows 10/11 64-bit
- FFmpeg is bundled (ffmpeg.exe)

H.264 VIDEO:
The bundled ffmpeg.exe enables H.264 video decoding.
Keep ffmpeg.exe in the same folder as controller.exe.

SUPPORT:
https://github.com/stangtennis/Remote
EOF

sed -i "s/\$VERSION/$VERSION/g" "$RELEASE_DIR/README.txt"

# Create ZIP
ZIP_FILE="$BUILD_DIR/RemoteDesktopController-$VERSION-win64.zip"
cd "$TEMP_DIR"
zip -r "$ZIP_FILE" "RemoteDesktopController-$VERSION"

# Cleanup
rm -rf "$TEMP_DIR"

echo ""
echo "✅ ZIP created: $ZIP_FILE"
ls -lh "$ZIP_FILE"
