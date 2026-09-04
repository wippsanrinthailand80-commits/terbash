# Terbash

Ultra-lightweight, native AI agent CLI for ARM64 devices (Android/Termux, macOS Apple Silicon, Windows on ARM).

## Features

- **Native Performance**: Single static Go binary, <30MB RAM, zero external dependencies
- **Multi-Provider BYOK**: OpenAI, Google Gemini, Anthropic Claude, Groq, Ollama, Mistral, Cohere, Together AI, Perplexity, DeepSeek, xAI
- **Secure Config**: API keys in `~/.config/terbash/config.yaml` with env var fallbacks
- **Human-in-the-Loop Tools**: Read-Ask-Write confirmation for all file/state changes
- **Sandboxed Execution**: Constrained to CWD, no directory traversal
- **Termux API Integration**: Battery, clipboard, notifications, wifi, sensors
- **Godot 4.x Headless QA**: Run tests, execute scripts, export builds
- **TUI with Markdown**: Streaming responses, token tracking, command palette

## Quick Start

### Termux (Android ARM64)
```bash
curl -fsSL https://raw.githubusercontent.com/terbash/terbash/main/install.sh | bash
terbash
```

### Manual Install (Any ARM64)
```bash
# Download latest release for your platform
curl -fsSL https://github.com/terbash/terbash/releases/latest/download/terbash_linux_arm64 -o terbash
chmod +x terbash
sudo mv terbash /usr/local/bin/
```

### Build from Source
```bash
git clone https://github.com/terbash/terbash
cd terbash
make build-arm64  # or make build for current platform
./terbash
```

## Configuration

Config file: `~/.config/terbash/config.yaml`

```yaml
default_provider: ollama
providers:
  openai:
    api_key: "sk-..."  # or OPENAI_API_KEY env var
    model: "gpt-4o-mini"
    temperature: 0.7
    max_tokens: 4096
  gemini:
    api_key: "..."     # or GEMINI_API_KEY
    model: "gemini-1.5-flash"
  anthropic:
    api_key: "..."     # or ANTHROPIC_API_KEY
    model: "claude-3-haiku-20240307"
  groq:
    api_key: "..."     # or GROQ_API_KEY
    model: "llama-3.1-8b-instant"
  mistral:
    api_key: "..."     # or MISTRAL_API_KEY
    model: "mistral-large-latest"
  cohere:
    api_key: "..."     # or COHERE_API_KEY
    model: "command-r-plus"
  together:
    api_key: "..."     # or TOGETHER_API_KEY
    model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
  perplexity:
    api_key: "..."     # or PERPLEXITY_API_KEY
    model: "llama-3.1-sonar-large-128k-online"
  deepseek:
    api_key: "..."     # or DEEPSEEK_API_KEY
    model: "deepseek-chat"
  xai:
    api_key: "..."     # or XAI_API_KEY
    model: "grok-beta"
  ollama:
    model: "llama3.2:3b"
    base_url: "http://localhost:11434"
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
```

### Environment Variable Fallbacks

All providers support environment variable fallbacks for API keys:

```bash
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="..."
export ANTHROPIC_API_KEY="..."
export GROQ_API_KEY="..."
export MISTRAL_API_KEY="..."
export COHERE_API_KEY="..."
export TOGETHER_API_KEY="..."
export PERPLEXITY_API_KEY="..."
export DEEPSEEK_API_KEY="..."
export XAI_API_KEY="..."
```

## Usage

### Interactive TUI
```bash
terbash
```

Commands:
- `/help` - Show commands
- `/providers` - List configured providers
- `/tools` - List available tools
- `/clear` - Clear conversation
- `/config` - Show current config
- `/exit` - Quit

### Tool Calling
The agent can use tools automatically. You'll be prompted for confirmation before:
- Writing/deleting files
- Executing shell commands
- Running Godot exports

### Godot Integration
```bash
# Run a GDScript in headless mode
terbash --tool godot_headless '{"operation": "run_script", "script_path": "res://test.gd"}'

# Run tests
terbash --tool godot_headless '{"operation": "run_tests", "project_path": "/path/to/project"}'

# Export for Android
terbash --tool godot_headless '{"operation": "export", "export_preset": "Android", "export_path": "/tmp/game.apk"}'
```

### Termux API
```bash
# Show notification
terbash --tool termux_api '{"operation": "notification", "args": {"title": "Build Done", "content": "APK ready"}}'

# Get battery status
terbash --tool termux_api '{"operation": "battery"}'

# Scan WiFi
terbash --tool termux_api '{"operation": "wifi_scan"}'
```

## Adding Custom Providers

You can add custom OpenAI-compatible providers by adding to config:

```yaml
providers:
  custom:
    api_key: "..."           # or CUSTOM_API_KEY
    base_url: "https://api.custom.com/v1"
    model: "custom-model"
    temperature: 0.7
    max_tokens: 4096
```

## Architecture

```
terbash/
├── cmd/terbash/            # CLI entrypoint (Cobra)
├── internal/
│   ├── config/             # Configuration management (Viper)
│   ├── llm/                # LLM provider abstraction & manager
│   ├── tools/              # Tool registry & implementations
│   │   ├── file_tool.go    # Read/Write/List/Delete (with confirm)
│   │   ├── shell_tool.go   # Shell execution (with confirm)
│   │   ├── godot_tool.go   # Godot headless operations
│   │   └── termux_tool.go  # Termux:API integration
│   └── tui/                # Bubble Tea TUI with Glamour markdown
├── pkg/
│   ├── providers/          # LLM provider implementations
│   │   ├── base.go              # HTTP client base
│   │   ├── openai_compat.go     # OpenAI, Groq, Mistral, Cohere, Together, Perplexity, DeepSeek, xAI
│   │   ├── anthropic.go         # Anthropic Claude
│   │   ├── gemini.go            # Google Gemini
│   │   └── ollama.go            # Ollama local
│   └── types/              # Shared types & interfaces
├── Makefile
└── install.sh
```

## Requirements

- Go 1.23+ (for building)
- Termux:API app (for Termux features on Android)
- Godot 4.x (for Godot tool)
- Ollama (for local models, optional)

## License

MIT