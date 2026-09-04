#!/usr/bin/env bash
set -euo pipefail

REPO="wippsanrinthailand80-commits/terbash"
BINARY_NAME="terbash"
# $PREFIX is set on Termux, fall back to /usr/local/bin elsewhere.
# NOTE: all paths below are quoted so directories containing spaces work.
INSTALL_DIR="${PREFIX:-/usr/local}/bin"
CONFIG_DIR="${HOME}/.config/terbash"

ARCH=$(uname -m)
case $ARCH in
    aarch64|arm64) GOARCH="arm64" ;;
    x86_64|amd64) GOARCH="amd64" ;;
    armv7l|armhf) GOARCH="arm" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case $OS in
    linux) GOOS="linux" ;;
    darwin) GOOS="darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

echo "Detected: $GOOS/$GOARCH"

ASSET="${BINARY_NAME}-${GOOS}-${GOARCH}"
if [ "$GOOS" = "windows" ]; then
  ASSET="${ASSET}.exe"
fi
LATEST_URL="https://github.com/$REPO/releases/latest/download/${ASSET}"

echo "Downloading from: $LATEST_URL"

mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

try_download() {
    # $1 = url, downloads to $INSTALL_DIR/$BINARY_NAME
    # NOTE: -o/-O must come BEFORE "--" (everything after "--" is a URL).
    if command -v curl >/dev/null 2>&1; then
        curl -fSL --retry 3 --retry-delay 2 -o "$INSTALL_DIR/$BINARY_NAME" -- "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --tries=3 -O "$INSTALL_DIR/$BINARY_NAME" -- "$1"
    else
        echo "Error: curl or wget required"
        return 2
    fi
}

if try_download "$LATEST_URL"; then
    :
else
    echo "Download from latest-redirect failed, resolving exact release tag..."
    TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep -o '"tag_name": *"[^"]*"' | head -n 1 | cut -d'"' -f4 || true)"
    if [ -n "${TAG:-}" ]; then
        PINNED_URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
        echo "Retrying with: $PINNED_URL"
        if ! try_download "$PINNED_URL"; then
            echo "Error: download failed."
            echo "Tried: $LATEST_URL"
            echo "Tried: $PINNED_URL"
            echo "Check https://github.com/$REPO/releases for available files."
            exit 1
        fi
    else
        echo "Error: download failed and could not resolve release tag."
        echo "Tried: $LATEST_URL"
        echo "Check https://github.com/$REPO/releases for available files."
        exit 1
    fi
fi

chmod +x -- "$INSTALL_DIR/$BINARY_NAME"

if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cat > "$CONFIG_DIR/config.yaml" << 'EOF'
default_provider: ollama
providers:
  ollama:
    model: "llama3.2:3b"
    base_url: "http://localhost:11434"
    temperature: 0.7
    max_tokens: 4096
tools:
  confirm_writes: true
  confirm_commands: true
  sandbox_enabled: true
  max_file_size: 10485760
godot:
  binary_path: "godot"
  project_path: ""
ui:
  theme: "dark"
  show_tokens: true
  stream_output: true
EOF
    echo "Created default config at $CONFIG_DIR/config.yaml"
fi

echo "Installed $BINARY_NAME to $INSTALL_DIR"
echo "Config directory: $CONFIG_DIR"
echo ""
echo "Run '$BINARY_NAME' to start"
echo "Run '$BINARY_NAME --help' for options"
echo "Run '$BINARY_NAME update' to self-update to the latest release"