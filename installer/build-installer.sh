#!/bin/bash
# Build script for Windows installer
# Run this on Ubuntu to prepare installer files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$SCRIPT_DIR/build"

echo "🔨 Building Remote Desktop Controller Installer"
echo "================================================"

# Create build directory
mkdir -p "$BUILD_DIR"

# Build controller
echo "📦 Building controller..."
cd "$PROJECT_DIR/controller"
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
    go build -ldflags '-s -w -H windowsgui' -o "$BUILD_DIR/controller.exe" .

# Download FFmpeg if not present
FFMPEG_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
FFMPEG_ZIP="$BUILD_DIR/ffmpeg.zip"
FFMPEG_EXE="$BUILD_DIR/ffmpeg.exe"

if [ ! -f "$FFMPEG_EXE" ]; then
    echo "📥 Downloading FFmpeg..."
    curl -L -o "$FFMPEG_ZIP" "$FFMPEG_URL"
    
    echo "📦 Extracting FFmpeg..."
    cd "$BUILD_DIR"
    unzip -o "$FFMPEG_ZIP"
    
    # Find and copy ffmpeg.exe
    find . -name "ffmpeg.exe" -exec cp {} "$FFMPEG_EXE" \;
    
    # Cleanup
    rm -rf ffmpeg-master-latest-win64-gpl
    rm -f "$FFMPEG_ZIP"
fi

# Copy license
cp "$SCRIPT_DIR/LICENSE.txt" "$BUILD_DIR/"

echo ""
echo "✅ Build complete!"
echo ""
echo "Files in $BUILD_DIR:"
ls -la "$BUILD_DIR"
echo ""
echo "📋 Next steps:"
echo "1. Copy files to Windows machine with NSIS installed"
echo "2. Run: makensis controller-installer.nsi"
echo ""
echo "Or create a simple ZIP distribution:"
echo "  cd $BUILD_DIR && zip -r RemoteDesktopController.zip controller.exe ffmpeg.exe"
