#!/bin/bash
# Build Windows executables from Ubuntu
# Usage: ./build-windows.sh [agent|controller|all]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🔨 Remote Desktop Windows Build Script${NC}"
echo "========================================"

# Create build directory
mkdir -p "$BUILD_DIR"

build_agent() {
    echo -e "\n${YELLOW}📦 Building Agent...${NC}"
    cd "$SCRIPT_DIR/agent"
    
    # Build for Windows AMD64
    echo "  Compiling for Windows AMD64..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        CC=x86_64-w64-mingw32-gcc \
        CXX=x86_64-w64-mingw32-g++ \
        go build -ldflags "-s -w" -o "$BUILD_DIR/remote-agent.exe" ./cmd/remote-agent 2>&1 | grep -v "merge failure" || true
    
    if [ -f "$BUILD_DIR/remote-agent.exe" ]; then
        SIZE=$(du -h "$BUILD_DIR/remote-agent.exe" | cut -f1)
        echo -e "  ${GREEN}✅ Agent built successfully: $BUILD_DIR/remote-agent.exe ($SIZE)${NC}"
    else
        echo -e "  ${RED}❌ Agent build failed${NC}"
        exit 1
    fi
}

build_controller() {
    echo -e "\n${YELLOW}📦 Building Controller...${NC}"
    cd "$SCRIPT_DIR/controller"
    
    # Build for Windows AMD64 (GUI mode - no console window)
    echo "  Compiling for Windows AMD64..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        CC=x86_64-w64-mingw32-gcc \
        CXX=x86_64-w64-mingw32-g++ \
        go build -ldflags "-H windowsgui -s -w" -o "$BUILD_DIR/controller.exe" .
    
    if [ -f "$BUILD_DIR/controller.exe" ]; then
        SIZE=$(du -h "$BUILD_DIR/controller.exe" | cut -f1)
        echo -e "  ${GREEN}✅ Controller built successfully: $BUILD_DIR/controller.exe ($SIZE)${NC}"
    else
        echo -e "  ${RED}❌ Controller build failed${NC}"
        exit 1
    fi
}

show_results() {
    echo -e "\n${GREEN}========================================"
    echo "Build Complete!"
    echo "========================================${NC}"
    echo ""
    echo "Output files in: $BUILD_DIR/"
    ls -lh "$BUILD_DIR"/*.exe 2>/dev/null || echo "No executables found"
    echo ""
    echo "To upload to GitHub, run:"
    echo "  ./release-github.sh v2.x.x"
}

# Parse arguments
case "${1:-all}" in
    agent)
        build_agent
        ;;
    controller)
        build_controller
        ;;
    all)
        build_agent
        build_controller
        ;;
    *)
        echo "Usage: $0 [agent|controller|all]"
        exit 1
        ;;
esac

show_results
