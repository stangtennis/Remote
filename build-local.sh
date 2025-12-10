#!/bin/bash
# Local build script for Remote Desktop project
# Run from project root: ./build-local.sh [version]

set -e

VERSION=${1:-"dev"}
TIMEOUT=300  # 5 minute timeout per build

echo "🔨 Building Remote Desktop v$VERSION"
echo "=================================="

# Create output directory
mkdir -p builds

# Build Controller (Windows)
echo ""
echo "📦 Building Controller (Windows)..."
cd controller
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -ldflags '-s -w -H windowsgui' -o ../builds/controller-$VERSION.exe ." && echo "✅ Controller built" || echo "❌ Controller build failed"
cd ..

# Build Agent (Windows) - requires Windows or proper cross-compile setup
echo ""
echo "📦 Building Agent (Windows)..."
echo "   Note: Agent requires Windows SDK headers for DXGI"
echo "   If this fails, build on Windows instead"
cd agent
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -ldflags '-s -w -H windowsgui' -o ../builds/remote-agent-$VERSION.exe ./cmd/remote-agent" 2>&1 && echo "✅ Agent built" || echo "⚠️  Agent build failed (try on Windows)"
cd ..

# Build Agent Console version
echo ""
echo "📦 Building Agent Console (Windows)..."
cd agent
timeout $TIMEOUT bash -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -ldflags '-s -w' -o ../builds/remote-agent-console-$VERSION.exe ./cmd/remote-agent" 2>&1 && echo "✅ Agent Console built" || echo "⚠️  Agent Console build failed (try on Windows)"
cd ..

echo ""
echo "=================================="
echo "📁 Build output in ./builds/"
ls -la builds/*$VERSION* 2>/dev/null || echo "No builds found"
