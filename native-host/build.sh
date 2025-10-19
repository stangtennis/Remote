#!/bin/bash
# Build native host for macOS/Linux

echo "Building Remote Desktop Control Native Host..."

# Build the Go executable
go build -ldflags="-s -w" -o remote-desktop-control main.go

if [ $? -eq 0 ]; then
    chmod +x remote-desktop-control
    echo ""
    echo "Build successful! File: remote-desktop-control"
    echo ""
    echo "Run the appropriate install script to install the native host:"
    echo "  - macOS: ./install-macos.sh"
    echo "  - Linux: ./install-linux.sh"
else
    echo ""
    echo "Build failed!"
    exit 1
fi
