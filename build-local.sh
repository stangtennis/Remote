#!/bin/bash
# Local build script for Remote Desktop project
# Run from project root: ./build-local.sh [version]

set -e

VERSION=${1:-"dev"}
BUILD_DATE=$(date +%Y-%m-%d)
TIMEOUT=300  # 5 minute timeout per build

echo "🔨 Building Remote Desktop v$VERSION (date: $BUILD_DATE)"
echo "=================================="

# Create output directory
mkdir -p builds

# ldflags for version injection
AGENT_LDFLAGS="-s -w -H windowsgui -X 'github.com/stangtennis/remote-agent/internal/tray.Version=$VERSION' -X 'github.com/stangtennis/remote-agent/internal/tray.BuildDate=$BUILD_DATE'"
AGENT_CONSOLE_LDFLAGS="-s -w -X 'github.com/stangtennis/remote-agent/internal/tray.Version=$VERSION' -X 'github.com/stangtennis/remote-agent/internal/tray.BuildDate=$BUILD_DATE'"
CONTROLLER_LDFLAGS="-s -w -H windowsgui -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE'"

# Build Controller (Windows)
echo ""
echo "📦 Building Controller (Windows)..."
cd controller
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -tags desktop,production -ldflags \"$CONTROLLER_LDFLAGS\" -o ../builds/controller-$VERSION.exe ." && echo "✅ Controller built" || echo "❌ Controller build failed"
cd ..

# Build Agent (Windows) with libjpeg-turbo SIMD encoding
echo ""
echo "📦 Building Agent (Windows) with turbo JPEG..."
echo "   Note: Agent requires Windows SDK headers for DXGI"
echo "   If this fails, build on Windows instead"

# Re-compile manifest+versioninfo to .syso every build. Without this,
# manifest changes (fx DPI awareness flags) ligger ubrugte i kildemappen
# fordi Go bare linker den eksisterende .syso uden at se på .rc/.manifest.
# Bug oplevet: v3.1.10/11 manifest havde PerMonitorV2, men .syso var fra
# marts uden flaget, så DPI-virtualisering blev ved til v3.1.12.
echo "   🔧 Re-compiling manifest resource (.syso)..."
(cd agent/cmd/remote-agent && x86_64-w64-mingw32-windres -i versioninfo.rc -o resource_windows_amd64.syso -O coff) || echo "⚠️  windres failed — manifest may be stale"

cd agent
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -tags turbo -ldflags \"$AGENT_LDFLAGS\" -o ../builds/remote-agent-$VERSION.exe ./cmd/remote-agent" 2>&1 && echo "✅ Agent built (turbo JPEG)" || echo "⚠️  Agent build failed (try on Windows)"
cd ..

# Build Agent Console version
echo ""
echo "📦 Building Agent Console (Windows) with turbo JPEG..."
cd agent
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -tags turbo -ldflags \"$AGENT_CONSOLE_LDFLAGS\" -o ../builds/remote-agent-console-$VERSION.exe ./cmd/remote-agent" 2>&1 && echo "✅ Agent Console built (turbo JPEG)" || echo "⚠️  Agent Console build failed (try on Windows)"
cd ..

# =============================================================================
# macOS Agent (requires osxcross or native macOS)
# =============================================================================
echo ""
echo "📦 Building Agent (macOS arm64)..."
echo "   Note: Requires osxcross or native macOS with Xcode CLI tools"
MACOS_AGENT_LDFLAGS="-s -w -X 'github.com/stangtennis/remote-agent/internal/tray.Version=$VERSION' -X 'github.com/stangtennis/remote-agent/internal/tray.BuildDate=$BUILD_DATE'"
cd agent
if command -v o64-clang &> /dev/null; then
    # Cross-compile via osxcross (no turbo — libjpeg-turbo not in osxcross SDK, use GitHub Actions CI for turbo)
    timeout $TIMEOUT bash -c "GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 CC=oa64-clang CXX=oa64-clang++ go build -ldflags \"$MACOS_AGENT_LDFLAGS\" -o ../builds/remote-agent-macos-arm64-$VERSION ./cmd/remote-agent" 2>&1 && echo "✅ macOS Agent (arm64) built" || echo "⚠️  macOS arm64 build failed (osxcross)"
    timeout $TIMEOUT bash -c "GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 CC=o64-clang CXX=o64-clang++ go build -ldflags \"$MACOS_AGENT_LDFLAGS\" -o ../builds/remote-agent-macos-amd64-$VERSION ./cmd/remote-agent" 2>&1 && echo "✅ macOS Agent (amd64) built" || echo "⚠️  macOS amd64 build failed (osxcross)"
elif [[ "$(uname -s)" == "Darwin" ]]; then
    # Native macOS build with turbo JPEG (brew install libjpeg-turbo)
    timeout $TIMEOUT bash -c "CGO_ENABLED=1 go build -tags turbo -ldflags \"$MACOS_AGENT_LDFLAGS\" -o ../builds/remote-agent-macos-$VERSION ./cmd/remote-agent" 2>&1 && echo "✅ macOS Agent built (native+turbo)" || echo "⚠️  macOS Agent build failed"
else
    echo "⏭️  Skipping macOS build (no osxcross and not on macOS)"
    echo "   macOS builds will be done via GitHub Actions CI"
fi
cd ..

# =============================================================================
# macOS Controller (requires osxcross or native macOS)
# =============================================================================
echo ""
echo "📦 Building Controller (macOS arm64)..."
MACOS_CTRL_LDFLAGS="-s -w -X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE'"
cd controller
if command -v o64-clang &> /dev/null; then
    # Cross-compile via osxcross
    timeout $TIMEOUT bash -c "GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 CC=oa64-clang CXX=oa64-clang++ go build -tags desktop,production -ldflags \"$MACOS_CTRL_LDFLAGS\" -o ../builds/controller-macos-arm64-$VERSION ." 2>&1 && echo "✅ macOS Controller (arm64) built" || echo "⚠️  macOS Controller arm64 build failed"
    timeout $TIMEOUT bash -c "GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 CC=o64-clang CXX=o64-clang++ go build -tags desktop,production -ldflags \"$MACOS_CTRL_LDFLAGS\" -o ../builds/controller-macos-amd64-$VERSION ." 2>&1 && echo "✅ macOS Controller (amd64) built" || echo "⚠️  macOS Controller amd64 build failed"
elif [[ "$(uname -s)" == "Darwin" ]]; then
    timeout $TIMEOUT bash -c "CGO_ENABLED=1 go build -tags desktop,production -ldflags \"$MACOS_CTRL_LDFLAGS\" -o ../builds/controller-macos-$VERSION ." 2>&1 && echo "✅ macOS Controller built (native)" || echo "⚠️  macOS Controller build failed"
else
    echo "⏭️  Skipping macOS Controller build (no osxcross and not on macOS)"
fi
cd ..

# =============================================================================
# macOS Universal Binaries (arm64 + amd64 combined)
# =============================================================================
LIPO=$(command -v /opt/osxcross/target/bin/lipo 2>/dev/null || command -v lipo 2>/dev/null || echo "")
if [ -n "$LIPO" ] && [ -f "builds/remote-agent-macos-arm64-$VERSION" ] && [ -f "builds/remote-agent-macos-amd64-$VERSION" ]; then
    echo ""
    echo "📦 Building macOS Universal Binaries..."
    $LIPO -create builds/remote-agent-macos-arm64-$VERSION builds/remote-agent-macos-amd64-$VERSION \
        -output builds/remote-agent-macos-universal-$VERSION && echo "✅ macOS Agent Universal built" || echo "⚠️  Agent Universal failed"
    $LIPO -create builds/controller-macos-arm64-$VERSION builds/controller-macos-amd64-$VERSION \
        -output builds/controller-macos-universal-$VERSION && echo "✅ macOS Controller Universal built" || echo "⚠️  Controller Universal failed"
fi

# =============================================================================
# NSIS Installers (Windows)
# =============================================================================
echo ""
echo "📦 Building NSIS installers..."

# Strip leading 'v' for NSIS version (e.g. v2.72.2 -> 2.72.2)
NSI_VERSION="${VERSION#v}"

build_installer() {
    local NAME="$1"       # e.g. "Agent"
    local NSI_FILE="$2"   # e.g. "agent-installer.nsi"
    local OUTPUT="$3"     # e.g. "RemoteDesktopAgent"

    local STAGING="installer/staging-${NAME,,}"
    rm -rf "$STAGING"
    mkdir -p "$STAGING"

    # Copy LICENSE
    cp installer/LICENSE.txt "$STAGING/"

    # Copy icon files if present
    for ico in installer/*.ico; do
        [ -f "$ico" ] && cp "$ico" "$STAGING/"
    done

    # Create versioned .nsi copy (sed version + VIProductVersion + OutFile)
    sed -e "s/!define VERSION \"[^\"]*\"/!define VERSION \"${NSI_VERSION}\"/" \
        -e "s/VIProductVersion \"[^\"]*\"/VIProductVersion \"${NSI_VERSION}.0\"/" \
        -e "s/OutFile \"${OUTPUT}-Setup.exe\"/OutFile \"${OUTPUT}-${VERSION}-Setup.exe\"/" \
        "installer/$NSI_FILE" > "$STAGING/$NSI_FILE"
}

# --- Agent installer (GUI + Console + OpenH264 + TurboJPEG) ---
build_installer "Agent" "agent-installer.nsi" "RemoteDesktopAgent"
STAGING="installer/staging-agent"
cp "builds/remote-agent-${VERSION}.exe" "$STAGING/remote-agent.exe"
cp "builds/remote-agent-console-${VERSION}.exe" "$STAGING/remote-agent-console.exe"
cp "installer/openh264-2.1.1-win64.dll" "$STAGING/"
cp "deps/libjpeg-turbo-win64/bin/libturbojpeg.dll" "$STAGING/"
if makensis -V2 "$STAGING/agent-installer.nsi" >/dev/null 2>&1; then
    mv "$STAGING/RemoteDesktopAgent-${VERSION}-Setup.exe" "builds/"
    echo "  ✅ RemoteDesktopAgent-${VERSION}-Setup.exe"
else
    echo "  ❌ Agent installer build failed"
    makensis -V2 "$STAGING/agent-installer.nsi" 2>&1 | tail -5
fi
rm -rf "$STAGING"

# --- Agent Console installer (Console + OpenH264 + TurboJPEG) ---
build_installer "AgentConsole" "agent-console-installer.nsi" "RemoteDesktopAgentConsole"
STAGING="installer/staging-agentconsole"
cp "builds/remote-agent-console-${VERSION}.exe" "$STAGING/remote-agent-console.exe"
cp "installer/openh264-2.1.1-win64.dll" "$STAGING/"
cp "deps/libjpeg-turbo-win64/bin/libturbojpeg.dll" "$STAGING/"
if makensis -V2 "$STAGING/agent-console-installer.nsi" >/dev/null 2>&1; then
    mv "$STAGING/RemoteDesktopAgentConsole-${VERSION}-Setup.exe" "builds/"
    echo "  ✅ RemoteDesktopAgentConsole-${VERSION}-Setup.exe"
else
    echo "  ❌ Agent Console installer build failed"
    makensis -V2 "$STAGING/agent-console-installer.nsi" 2>&1 | tail -5
fi
rm -rf "$STAGING"

# --- Agent Run Once installer (Portable, no service) ---
build_installer "AgentRunOnce" "agent-runonce-installer.nsi" "RemoteDesktopAgent-RunOnce"
STAGING="installer/staging-agentrunonce"
cp "builds/remote-agent-console-${VERSION}.exe" "$STAGING/remote-agent-console.exe"
cp "installer/openh264-2.1.1-win64.dll" "$STAGING/"
cp "deps/libjpeg-turbo-win64/bin/libturbojpeg.dll" "$STAGING/"
if makensis -V2 "$STAGING/agent-runonce-installer.nsi" >/dev/null 2>&1; then
    mv "$STAGING/RemoteDesktopAgent-RunOnce-${VERSION}-Setup.exe" "builds/"
    echo "  ✅ RemoteDesktopAgent-RunOnce-${VERSION}-Setup.exe"
else
    echo "  ❌ Agent Run Once installer build failed"
    makensis -V2 "$STAGING/agent-runonce-installer.nsi" 2>&1 | tail -5
fi
rm -rf "$STAGING"

# --- Controller installer (Controller + FFmpeg) ---
build_installer "Controller" "controller-installer.nsi" "RemoteDesktopController"
STAGING="installer/staging-controller"
cp "builds/controller-${VERSION}.exe" "$STAGING/controller.exe"
cp "installer/ffmpeg.exe" "$STAGING/"
if makensis -V2 "$STAGING/controller-installer.nsi" >/dev/null 2>&1; then
    mv "$STAGING/RemoteDesktopController-${VERSION}-Setup.exe" "builds/"
    echo "  ✅ RemoteDesktopController-${VERSION}-Setup.exe"
else
    echo "  ❌ Controller installer build failed"
    makensis -V2 "$STAGING/controller-installer.nsi" 2>&1 | tail -5
fi
rm -rf "$STAGING"

# =============================================================================
# Generate SHA256 sidecars for ALL build artifacts
# =============================================================================
echo ""
echo "🔐 Genererer SHA256 checksums..."
SHA_MANIFEST="builds/SHA256SUMS-${VERSION}.txt"
> "$SHA_MANIFEST"
for f in builds/*${VERSION}*; do
    # Skip .sha256 files and the manifest itself
    case "$f" in
        *.sha256|*SHA256SUMS*) continue ;;
    esac
    [ -f "$f" ] || continue
    HASH=$(sha256sum "$f" | awk '{print $1}')
    BASENAME=$(basename "$f")
    echo "$HASH  $BASENAME" > "$f.sha256"
    echo "$HASH  $BASENAME" >> "$SHA_MANIFEST"
done
echo "   ✅ $(wc -l < "$SHA_MANIFEST") filer signeret"
echo "   📄 Manifest: $SHA_MANIFEST"

# Convenience variables for version.json
if [ -f "builds/remote-agent-${VERSION}.exe" ]; then
    AGENT_SHA256=$(sha256sum "builds/remote-agent-${VERSION}.exe" | awk '{print $1}')
    echo "   🔑 agent_sha256:      ${AGENT_SHA256}"
fi
if [ -f "builds/controller-${VERSION}.exe" ]; then
    CONTROLLER_SHA256=$(sha256sum "builds/controller-${VERSION}.exe" | awk '{print $1}')
    echo "   🔑 controller_sha256: ${CONTROLLER_SHA256}"
fi

# =============================================================================
# Auto-generer release notes fra git log
# =============================================================================
NOTES_FILE="builds/RELEASE_NOTES-${VERSION}.md"
if command -v git &> /dev/null && git rev-parse --git-dir >/dev/null 2>&1; then
    echo ""
    echo "📝 Genererer release notes..."
    PREV_TAG=$(git tag --sort=-version:refname 2>/dev/null | grep -E '^v[0-9]' | grep -v "^${VERSION}$" | head -1)
    {
        echo "## Remote Desktop ${VERSION}"
        echo ""
        if [ -n "$PREV_TAG" ]; then
            echo "_Ændringer siden ${PREV_TAG} (bygget ${BUILD_DATE})_"
            echo ""
            echo "### Commits"
            git log --pretty=format:"- %s" --no-merges "${PREV_TAG}..HEAD" 2>/dev/null || echo "- (kunne ikke læse git log)"
        else
            echo "_Bygget ${BUILD_DATE} (ingen tidligere tag fundet)_"
            echo ""
            echo "### Seneste commits"
            git log --pretty=format:"- %s" --no-merges -20
        fi
        echo ""
        echo ""
        echo "### SHA256 Checksums"
        echo ""
        echo "\`\`\`"
        cat "$SHA_MANIFEST" 2>/dev/null
        echo "\`\`\`"
    } > "$NOTES_FILE"
    echo "   ✅ ${NOTES_FILE}"
fi

echo ""
echo "=================================="
echo "📁 Build output in ./builds/"
ls -la builds/*$VERSION* 2>/dev/null | grep -v "\.sha256$" || echo "No builds found"
