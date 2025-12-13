#!/bin/bash
# Build all installer formats: ZIP, EXE (NSIS), MSI (WiX)
# Run on Ubuntu with nsis and wixl installed

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/builds"
VERSION="${1:-2.59.0}"

echo "🔨 Building All Installers v$VERSION"
echo "====================================="
echo ""

# Create staging directory
STAGE_DIR="$SCRIPT_DIR/staging"
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"

# Build controller if needed
CONTROLLER_EXE="$BUILD_DIR/controller-v$VERSION.exe"
if [ ! -f "$CONTROLLER_EXE" ]; then
    echo "📦 Building controller..."
    cd "$PROJECT_DIR/controller"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
        go build -ldflags '-s -w -H windowsgui' -o "$CONTROLLER_EXE" .
fi
cp "$CONTROLLER_EXE" "$STAGE_DIR/controller.exe"

# Get FFmpeg if not cached
FFMPEG_CACHE="$BUILD_DIR/ffmpeg-win64.exe"
if [ ! -f "$FFMPEG_CACHE" ]; then
    echo "📥 Downloading FFmpeg..."
    FFMPEG_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
    TEMP_DIR=$(mktemp -d)
    curl -L -o "$TEMP_DIR/ffmpeg.zip" "$FFMPEG_URL"
    cd "$TEMP_DIR"
    unzip -q ffmpeg.zip
    find . -name "ffmpeg.exe" -type f | head -1 | xargs -I {} cp {} "$FFMPEG_CACHE"
    rm -rf "$TEMP_DIR"
fi
cp "$FFMPEG_CACHE" "$STAGE_DIR/ffmpeg.exe"

# Copy installer files
cp "$SCRIPT_DIR/LICENSE.txt" "$STAGE_DIR/"
cp "$SCRIPT_DIR/LICENSE.rtf" "$STAGE_DIR/"
cp "$SCRIPT_DIR/controller-installer.nsi" "$STAGE_DIR/"
cp "$SCRIPT_DIR/controller.wxs" "$STAGE_DIR/"

cd "$STAGE_DIR"

echo ""
echo "📦 Creating ZIP..."
ZIP_FILE="$BUILD_DIR/RemoteDesktopController-$VERSION-win64.zip"
TEMP_ZIP=$(mktemp -d)
mkdir -p "$TEMP_ZIP/RemoteDesktopController-$VERSION"
cp controller.exe ffmpeg.exe "$TEMP_ZIP/RemoteDesktopController-$VERSION/"
cat > "$TEMP_ZIP/RemoteDesktopController-$VERSION/README.txt" << EOF
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
cd "$TEMP_ZIP"
zip -r "$ZIP_FILE" "RemoteDesktopController-$VERSION"
rm -rf "$TEMP_ZIP"
echo "✅ ZIP: $ZIP_FILE"

cd "$STAGE_DIR"

echo ""
echo "📦 Creating NSIS EXE installer..."
# Update version in NSIS script
sed -i "s/!define VERSION \".*\"/!define VERSION \"$VERSION\"/" controller-installer.nsi
sed -i "s/VIProductVersion \".*\"/VIProductVersion \"$VERSION.0\"/" controller-installer.nsi

makensis -V2 controller-installer.nsi
EXE_FILE="$BUILD_DIR/RemoteDesktopController-$VERSION-Setup.exe"
mv RemoteDesktopController-Setup.exe "$EXE_FILE"
echo "✅ EXE: $EXE_FILE"

echo ""
echo "📦 Creating WiX MSI installer..."
# Update version in WXS
sed -i "s/Version=\"[0-9.]*\"/Version=\"$VERSION.0\"/" controller.wxs

# wixl creates MSI directly (no candle/light needed)
MSI_FILE="$BUILD_DIR/RemoteDesktopController-$VERSION-win64.msi"
wixl -v -o "$MSI_FILE" controller.wxs 2>&1 || {
    echo "⚠️ WiX MSI creation failed - trying simplified approach..."
    # Create simplified WXS without UI
    cat > controller-simple.wxs << 'WXSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
    <Product Id="*" 
             Name="Remote Desktop Controller" 
             Language="1033" 
             Version="VERSION_PLACEHOLDER" 
             Manufacturer="StangTennis" 
             UpgradeCode="A1B2C3D4-E5F6-7890-ABCD-EF1234567890">
        
        <Package InstallerVersion="200" Compressed="yes" InstallScope="perMachine"/>
        <MajorUpgrade DowngradeErrorMessage="A newer version is already installed."/>
        <MediaTemplate EmbedCab="yes"/>
        
        <Feature Id="ProductFeature" Title="Remote Desktop Controller" Level="1">
            <ComponentGroupRef Id="ProductComponents"/>
        </Feature>
    </Product>
    
    <Fragment>
        <Directory Id="TARGETDIR" Name="SourceDir">
            <Directory Id="ProgramFiles64Folder">
                <Directory Id="INSTALLFOLDER" Name="Remote Desktop Controller"/>
            </Directory>
        </Directory>
    </Fragment>
    
    <Fragment>
        <ComponentGroup Id="ProductComponents" Directory="INSTALLFOLDER">
            <Component Id="ControllerExe" Guid="*">
                <File Id="controller.exe" Source="controller.exe" KeyPath="yes"/>
            </Component>
            <Component Id="FFmpegExe" Guid="*">
                <File Id="ffmpeg.exe" Source="ffmpeg.exe" KeyPath="yes"/>
            </Component>
        </ComponentGroup>
    </Fragment>
</Wix>
WXSEOF
    sed -i "s/VERSION_PLACEHOLDER/$VERSION.0/" controller-simple.wxs
    wixl -v -o "$MSI_FILE" controller-simple.wxs
}
echo "✅ MSI: $MSI_FILE"

# Cleanup
cd "$SCRIPT_DIR"
rm -rf "$STAGE_DIR"

echo ""
echo "=========================================="
echo "✅ All installers created!"
echo ""
echo "Files:"
ls -lh "$BUILD_DIR/RemoteDesktopController-$VERSION"* 2>/dev/null || true
echo ""
echo "Upload to GitHub release:"
echo "  gh release upload v$VERSION \\"
echo "    \"$BUILD_DIR/RemoteDesktopController-$VERSION-win64.zip\" \\"
echo "    \"$BUILD_DIR/RemoteDesktopController-$VERSION-Setup.exe\" \\"
echo "    \"$BUILD_DIR/RemoteDesktopController-$VERSION-win64.msi\""
