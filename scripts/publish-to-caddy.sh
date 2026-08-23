#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <version> [downloads_dir]"
  echo "Example: $0 v3.1.51 /home/dennis/caddy/downloads"
  exit 1
fi

VERSION="$1"
DOWNLOADS_DIR="${2:-/home/dennis/caddy/downloads}"
BUILDS_DIR="builds"

require_file() {
  local f="$1"
  [[ -f "$f" ]] || { echo "Missing file: $f" >&2; exit 1; }
}

copy_file() {
  local src="$1"
  local dst="$2"
  cp -f "$src" "$dst"
}

mkdir -p "$DOWNLOADS_DIR"

AGENT_EXE="$BUILDS_DIR/remote-agent-${VERSION}.exe"
SUPPORT_EXE="$BUILDS_DIR/remote-support-${VERSION}.exe"
SUPPORT_ZIP="$BUILDS_DIR/RemoteSupport-${VERSION}.zip"
SUPPORT_LAUNCHER="$BUILDS_DIR/RemoteDesktopSupport-${VERSION}-Setup.exe"
AGENT_CONSOLE_EXE="$BUILDS_DIR/remote-agent-console-${VERSION}.exe"
CONTROLLER_EXE="$BUILDS_DIR/controller-${VERSION}.exe"
AGENT_MAC="$BUILDS_DIR/remote-agent-macos-universal-${VERSION}"
CONTROLLER_MAC="$BUILDS_DIR/controller-macos-universal-${VERSION}"
AGENT_MAC_ARCHIVE="$BUILDS_DIR/RemoteDesktopAgent-macOS-${VERSION}.tar.gz"
CONTROLLER_MAC_ARCHIVE="$BUILDS_DIR/RemoteDesktopController-macOS-${VERSION}.tar.gz"

AGENT_SETUP="$BUILDS_DIR/RemoteDesktopAgent-${VERSION}-Setup.exe"
AGENT_CONSOLE_SETUP="$BUILDS_DIR/RemoteDesktopAgentConsole-${VERSION}-Setup.exe"
AGENT_RUNONCE_SETUP="$BUILDS_DIR/RemoteDesktopAgent-RunOnce-${VERSION}-Setup.exe"
CONTROLLER_SETUP="$BUILDS_DIR/RemoteDesktopController-${VERSION}-Setup.exe"

require_file "$AGENT_EXE"
require_file "$SUPPORT_EXE"
require_file "$SUPPORT_ZIP"
require_file "$SUPPORT_LAUNCHER"
require_file "$AGENT_CONSOLE_EXE"
require_file "$CONTROLLER_EXE"
require_file "$AGENT_MAC"
require_file "$CONTROLLER_MAC"
require_file "$AGENT_MAC_ARCHIVE"
require_file "$CONTROLLER_MAC_ARCHIVE"
require_file "$AGENT_SETUP"
require_file "$AGENT_CONSOLE_SETUP"
require_file "$AGENT_RUNONCE_SETUP"
require_file "$CONTROLLER_SETUP"

# Versioned binaries (used by updater/version.json)
copy_file "$AGENT_EXE" "$DOWNLOADS_DIR/remote-agent-${VERSION}.exe"
copy_file "$SUPPORT_EXE" "$DOWNLOADS_DIR/remote-support-${VERSION}.exe"
copy_file "$SUPPORT_ZIP" "$DOWNLOADS_DIR/RemoteSupport-${VERSION}.zip"
copy_file "$SUPPORT_LAUNCHER" "$DOWNLOADS_DIR/RemoteDesktopSupport-${VERSION}-Setup.exe"
copy_file "$AGENT_CONSOLE_EXE" "$DOWNLOADS_DIR/remote-agent-console-${VERSION}.exe"
copy_file "$CONTROLLER_EXE" "$DOWNLOADS_DIR/controller-${VERSION}.exe"
copy_file "$AGENT_MAC" "$DOWNLOADS_DIR/remote-agent-macos-${VERSION}"
copy_file "$CONTROLLER_MAC" "$DOWNLOADS_DIR/controller-macos-${VERSION}"
copy_file "$AGENT_MAC_ARCHIVE" "$DOWNLOADS_DIR/RemoteDesktopAgent-macOS-${VERSION}.tar.gz"
copy_file "$CONTROLLER_MAC_ARCHIVE" "$DOWNLOADS_DIR/RemoteDesktopController-macOS-${VERSION}.tar.gz"

# Versioned installers
copy_file "$AGENT_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgent-${VERSION}-Setup.exe"
copy_file "$AGENT_CONSOLE_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgentConsole-${VERSION}-Setup.exe"
copy_file "$AGENT_RUNONCE_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgent-RunOnce-${VERSION}-Setup.exe"
copy_file "$CONTROLLER_SETUP" "$DOWNLOADS_DIR/RemoteDesktopController-${VERSION}-Setup.exe"

# Stable aliases (used by dashboard/manual download buttons)
copy_file "$AGENT_EXE" "$DOWNLOADS_DIR/remote-agent.exe"
copy_file "$SUPPORT_EXE" "$DOWNLOADS_DIR/remote-support.exe"
copy_file "$SUPPORT_ZIP" "$DOWNLOADS_DIR/RemoteSupport.zip"
copy_file "$SUPPORT_LAUNCHER" "$DOWNLOADS_DIR/RemoteDesktopSupport-Setup.exe"
copy_file "$AGENT_CONSOLE_EXE" "$DOWNLOADS_DIR/remote-agent-console.exe"
copy_file "$CONTROLLER_EXE" "$DOWNLOADS_DIR/controller.exe"
copy_file "$AGENT_MAC" "$DOWNLOADS_DIR/remote-agent-macos"
copy_file "$CONTROLLER_MAC" "$DOWNLOADS_DIR/controller-macos"
copy_file "$AGENT_MAC_ARCHIVE" "$DOWNLOADS_DIR/RemoteDesktopAgent-macOS.tar.gz"
copy_file "$CONTROLLER_MAC_ARCHIVE" "$DOWNLOADS_DIR/RemoteDesktopController-macOS.tar.gz"
copy_file "$AGENT_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgent-Setup.exe"
copy_file "$AGENT_CONSOLE_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgentConsole-Setup.exe"
copy_file "$AGENT_RUNONCE_SETUP" "$DOWNLOADS_DIR/RemoteDesktopAgent-RunOnce-Setup.exe"
copy_file "$CONTROLLER_SETUP" "$DOWNLOADS_DIR/RemoteDesktopController-Setup.exe"

AGENT_SHA256="$(sha256sum "$DOWNLOADS_DIR/remote-agent-${VERSION}.exe" | awk '{print $1}')"
SUPPORT_SHA256="$(sha256sum "$DOWNLOADS_DIR/remote-support-${VERSION}.exe" | awk '{print $1}')"
SUPPORT_IMMUTABLE="remote-support-${VERSION}-${SUPPORT_SHA256:0:8}.exe"
copy_file "$SUPPORT_EXE" "$DOWNLOADS_DIR/$SUPPORT_IMMUTABLE"
SUPPORT_ZIP_SHA256="$(sha256sum "$DOWNLOADS_DIR/RemoteSupport-${VERSION}.zip" | awk '{print $1}')"
SUPPORT_ZIP_IMMUTABLE="RemoteSupport-${VERSION}-${SUPPORT_ZIP_SHA256:0:8}.zip"
copy_file "$SUPPORT_ZIP" "$DOWNLOADS_DIR/$SUPPORT_ZIP_IMMUTABLE"
SUPPORT_LAUNCHER_SHA256="$(sha256sum "$DOWNLOADS_DIR/RemoteDesktopSupport-${VERSION}-Setup.exe" | awk '{print $1}')"
SUPPORT_LAUNCHER_IMMUTABLE="RemoteDesktopSupport-${VERSION}-${SUPPORT_LAUNCHER_SHA256:0:8}-Setup.exe"
copy_file "$SUPPORT_LAUNCHER" "$DOWNLOADS_DIR/$SUPPORT_LAUNCHER_IMMUTABLE"
printf '%s  remote-support.exe\n' "$SUPPORT_SHA256" > "$DOWNLOADS_DIR/remote-support.exe.sha256"
AGENT_MAC_SHA256="$(sha256sum "$DOWNLOADS_DIR/remote-agent-macos-${VERSION}" | awk '{print $1}')"
CONTROLLER_SHA256="$(sha256sum "$DOWNLOADS_DIR/controller-${VERSION}.exe" | awk '{print $1}')"
CONTROLLER_MAC_SHA256="$(sha256sum "$DOWNLOADS_DIR/controller-macos-${VERSION}" | awk '{print $1}')"

cat > "$DOWNLOADS_DIR/version.json" <<EOF
{
  "agent_version": "${VERSION}",
  "controller_version": "${VERSION}",
  "agent_url": "https://updates.hawkeye123.dk/remote-agent-${VERSION}.exe",
  "support_url": "https://updates.hawkeye123.dk/${SUPPORT_LAUNCHER_IMMUTABLE}",
  "support_exe_url": "https://updates.hawkeye123.dk/${SUPPORT_IMMUTABLE}",
  "support_zip_url": "https://updates.hawkeye123.dk/${SUPPORT_ZIP_IMMUTABLE}",
  "agent_url_macos": "https://updates.hawkeye123.dk/remote-agent-macos-${VERSION}",
  "controller_url": "https://updates.hawkeye123.dk/controller-${VERSION}.exe",
  "controller_url_macos": "https://updates.hawkeye123.dk/controller-macos-${VERSION}",
  "agent_sha256": "${AGENT_SHA256}",
  "support_sha256": "${SUPPORT_SHA256}",
  "support_zip_sha256": "${SUPPORT_ZIP_SHA256}",
  "support_launcher_sha256": "${SUPPORT_LAUNCHER_SHA256}",
  "agent_sha256_macos": "${AGENT_MAC_SHA256}",
  "controller_sha256": "${CONTROLLER_SHA256}",
  "controller_sha256_macos": "${CONTROLLER_MAC_SHA256}"
}
EOF

echo "Published ${VERSION} to ${DOWNLOADS_DIR}"
echo "agent_sha256=${AGENT_SHA256}"
echo "support_sha256=${SUPPORT_SHA256}"
echo "support_zip_sha256=${SUPPORT_ZIP_SHA256}"
echo "support_launcher_sha256=${SUPPORT_LAUNCHER_SHA256}"
echo "controller_sha256=${CONTROLLER_SHA256}"
