#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

REPO="terbash/terbash"
BINARY_NAME="terbash"
INSTALL_DIR="$PREFIX/bin"
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

LATEST_URL="https://github.com/$REPO/releases/latest/download/${BINARY_NAME}_${GOOS}_${GOARCH}"

echo "Downloading from: $LATEST_URL"

mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$LATEST_URL" -o "$INSTALL_DIR/$BINARY_NAME"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$LATEST_URL" -O "$INSTALL_DIR/$BINARY_NAME"
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