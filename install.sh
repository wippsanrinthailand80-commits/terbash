#!/usr/bin/env bash
set -euo pipefail

REPO="wippsanrinthailand80-commits/terbash"
BINARY_NAME="terbash"
# $PREFIX is set on Termux, fall back to /usr/local/bin elsewhere
INSTALL_DIR="${PREFIX:-/usr/local}/bin"
CONFIG_DIR="$HOME/.config/terbash"

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

if command -v curl >/dev/null 2>&1; then
    if ! curl -fsSL "$LATEST_URL" -o "$INSTALL_DIR/$BINARY_NAME"; then
        echo "Error: download failed (404?)."
        echo "The release asset '$ASSET' was not found at $LATEST_URL"
        echo "Check https://github.com/$REPO/releases for available files."
        exit 1
    fi
elif command -v wget >/dev/null 2>&1; then
    if ! wget -q "$LATEST_URL" -O "$INSTALL_DIR/$BINARY_NAME"; then
        echo "Error: download failed. Check https://github.com/$REPO/releases"
        exit 1
    fi
else
    echo "Error: curl or wget required"
    exit 1
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

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