#!/bin/bash
# Install native messaging host for macOS

echo "Installing Remote Desktop Control Native Host for macOS..."

# Get the directory where the script is located
INSTALL_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
MANIFEST="$INSTALL_DIR/com.remote.desktop.control.json"
MANIFEST_NAME="com.remote.desktop.control.json"

# Update manifest with absolute path
sed -i '' "s|\"path\": \"remote-desktop-control.exe\"|\"path\": \"$INSTALL_DIR/remote-desktop-control\"|g" "$MANIFEST"

# Make the executable runnable
chmod +x "$INSTALL_DIR/remote-desktop-control"

# Chrome native messaging host directory
CHROME_DIR="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts"
mkdir -p "$CHROME_DIR"
cp "$MANIFEST" "$CHROME_DIR/$MANIFEST_NAME"

# Edge native messaging host directory
EDGE_DIR="$HOME/Library/Application Support/Microsoft Edge/NativeMessagingHosts"
mkdir -p "$EDGE_DIR"
cp "$MANIFEST" "$EDGE_DIR/$MANIFEST_NAME"

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Install the browser extension"
echo "2. Open the agent page"
echo "3. Remote control should now work!"
echo ""
