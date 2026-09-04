package types

type Provider string

const (
	ProviderOpenAI      Provider = "openai"
	ProviderGemini      Provider = "gemini"
	ProviderAnthropic   Provider = "anthropic"
	ProviderGroq        Provider = "groq"
	ProviderMistral     Provider = "mistral"
	ProviderCohere      Provider = "cohere"
	ProviderTogether    Provider = "together"
	ProviderPerplexity  Provider = "perplexity"
	ProviderDeepSeek    Provider = "deepseek"
	ProviderXAI         Provider = "xai"
	ProviderOllama      Provider = "ollama"
	ProviderCustom      Provider = "custom"
)

type Config struct {
	DefaultProvider Provider              `mapstructure:"default_provider" yaml:"default_provider"`
	Providers       map[Provider]ProviderConfig `mapstructure:"providers" yaml:"providers"`
	Tools           ToolsConfig           `mapstructure:"tools" yaml:"tools"`
	Godot           GodotConfig           `mapstructure:"godot" yaml:"godot"`
	UI              UIConfig              `mapstructure:"ui" yaml:"ui"`
}

type ProviderConfig struct {
	APIKey      string  `mapstructure:"api_key" yaml:"api_key"`
	BaseURL     string  `mapstructure:"base_url" yaml:"base_url"`
	Model       string  `mapstructure:"model" yaml:"model"`
	Temperature float64 `mapstructure:"temperature" yaml:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens" yaml:"max_tokens"`
}

type ToolsConfig struct {
	ConfirmWrites    bool  `mapstructure:"confirm_writes" yaml:"confirm_writes"`
	ConfirmCommands  bool  `mapstructure:"confirm_commands" yaml:"confirm_commands"`
	SandboxEnabled   bool  `mapstructure:"sandbox_enabled" yaml:"sandbox_enabled"`
	MaxFileSize      int64 `mapstructure:"max_file_size" yaml:"max_file_size"`
}

type GodotConfig struct {
	BinaryPath  string `mapstructure:"binary_path" yaml:"binary_path"`
	ProjectPath string `mapstructure:"project_path" yaml:"project_path"`
}

type UIConfig struct {
	Theme        string `mapstructure:"theme" yaml:"theme"`
	ShowTokens   bool   `mapstructure:"show_tokens" yaml:"show_tokens"`
	StreamOutput bool   `mapstructure:"stream_output" yaml:"stream_output"`
}