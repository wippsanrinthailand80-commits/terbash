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
	ProviderLlamaCpp    Provider = "llamacpp"
	ProviderOpenRouter  Provider = "openrouter"
	ProviderCerebras    Provider = "cerebras"
	ProviderSambaNova   Provider = "sambanova"
	ProviderFireworks   Provider = "fireworks"
	ProviderDeepInfra   Provider = "deepinfra"
	ProviderMoonshot    Provider = "moonshot"
	ProviderNovita      Provider = "novita"
	ProviderSiliconFlow Provider = "siliconflow"
	ProviderZhipu       Provider = "zhipu"
	ProviderQwen        Provider = "qwen"
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

// SupportedProviders lists every built-in provider name, sorted.
func SupportedProviders() []Provider {
	return []Provider{
		ProviderAnthropic,
		ProviderCerebras,
		ProviderCohere,
		ProviderCustom,
		ProviderDeepInfra,
		ProviderDeepSeek,
		ProviderFireworks,
		ProviderGemini,
		ProviderGroq,
		ProviderLlamaCpp,
		ProviderMistral,
		ProviderMoonshot,
		ProviderNovita,
		ProviderOllama,
		ProviderOpenRouter,
		ProviderQwen,
		ProviderSambaNova,
		ProviderSiliconFlow,
		ProviderOpenAI,
		ProviderPerplexity,
		ProviderTogether,
		ProviderXAI,
		ProviderZhipu,
	}
}

// IsSupported reports whether name is a built-in provider.
func IsSupported(name Provider) bool {
	for _, p := range SupportedProviders() {
		if p == name {
			return true
		}
	}
	return false
}

// DefaultModel returns a sensible first-run model for a built-in provider.
func DefaultModel(name Provider) (string, bool) {
	models := map[Provider]string{
		ProviderOpenAI:     "gpt-4o-mini",
		ProviderGemini:     "gemini-1.5-flash",
		ProviderAnthropic:  "claude-3-haiku-20240307",
		ProviderGroq:       "llama-3.1-8b-instant",
		ProviderMistral:    "mistral-large-latest",
		ProviderCohere:     "command-r-plus",
		ProviderTogether:   "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
		ProviderPerplexity: "llama-3.1-sonar-large-128k-online",
		ProviderDeepSeek:   "deepseek-chat",
		ProviderXAI:        "grok-beta",
		ProviderOllama:     "llama3.2:3b",
		// llama-server serves whatever GGUF is loaded and echoes this name.
		ProviderLlamaCpp: "default",
		ProviderOpenRouter:  "openai/gpt-4o-mini",
		ProviderCerebras:    "llama3.1-8b",
		ProviderSambaNova:   "Meta-Llama-3.3-70B-Instruct",
		ProviderFireworks:   "accounts/fireworks/models/llama-v3p1-8b-instruct",
		ProviderDeepInfra:   "meta-llama/Meta-Llama-3.1-8B-Instruct",
		ProviderMoonshot:    "kimi-k3",
		ProviderNovita:      "meta-llama/llama-3.1-8b-instruct",
		ProviderSiliconFlow: "Qwen/Qwen3-8B",
		ProviderZhipu:       "glm-4-flash",
		ProviderQwen:        "qwen-plus",
	}
	m, ok := models[name]
	return m, ok
}

// RequiresKey reports whether a provider needs an API key.
// Local providers (ollama, llamacpp) don't; everything else does.
func RequiresKey(name Provider) bool { return EnvKey(name) != "" }

// EnvKey returns the API-key environment variable for a provider, or "".
func EnvKey(name Provider) string {
	keys := map[Provider]string{
		ProviderOpenAI:     "OPENAI_API_KEY",
		ProviderGemini:     "GEMINI_API_KEY",
		ProviderAnthropic:  "ANTHROPIC_API_KEY",
		ProviderGroq:       "GROQ_API_KEY",
		ProviderMistral:    "MISTRAL_API_KEY",
		ProviderCohere:     "COHERE_API_KEY",
		ProviderTogether:   "TOGETHER_API_KEY",
		ProviderPerplexity: "PERPLEXITY_API_KEY",
		ProviderDeepSeek:   "DEEPSEEK_API_KEY",
		ProviderXAI:        "XAI_API_KEY",
		ProviderOpenRouter: "OPENROUTER_API_KEY",
		ProviderCerebras:   "CEREBRAS_API_KEY",
		ProviderSambaNova:  "SAMBANOVA_API_KEY",
		ProviderFireworks:  "FIREWORKS_API_KEY",
		ProviderDeepInfra:  "DEEPINFRA_API_KEY",
		ProviderMoonshot:   "MOONSHOT_API_KEY",
		ProviderNovita:     "NOVITA_API_KEY",
		ProviderSiliconFlow: "SILICONFLOW_API_KEY",
		ProviderZhipu:      "ZAI_API_KEY",
		ProviderQwen:       "DASHSCOPE_API_KEY",
		ProviderCustom:     "CUSTOM_API_KEY",
	}
	return keys[name]
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