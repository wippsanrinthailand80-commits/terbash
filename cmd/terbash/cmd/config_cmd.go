package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage terbash config (path, init, set-provider)",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.GetConfigPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file if missing",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.GetConfigPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Config already exists: %s (leaving it alone)\n", path)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		def := types.Config{
			DefaultProvider: types.ProviderOllama,
			Providers: map[types.Provider]types.ProviderConfig{
				types.ProviderOllama: {Model: "llama3.2:3b", BaseURL: "http://localhost:11434", Temperature: 0.7, MaxTokens: 4096},
			},
			Tools: types.ToolsConfig{ConfirmWrites: true, ConfirmCommands: true, SandboxEnabled: true, MaxFileSize: 10485760},
			Godot: types.GodotConfig{BinaryPath: "godot"},
			UI:    types.UIConfig{Theme: "dark", ShowTokens: true, StreamOutput: true},
		}
		data, err := yaml.Marshal(&def)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
		fmt.Printf("Created default config: %s\n", path)
		return nil
	},
}

// Sensible first-run models (same as README examples).
var defaultModels = map[types.Provider]string{
	types.ProviderOpenAI:     "gpt-4o-mini",
	types.ProviderGemini:     "gemini-1.5-flash",
	types.ProviderAnthropic:  "claude-3-haiku-20240307",
	types.ProviderGroq:       "llama-3.1-8b-instant",
	types.ProviderMistral:    "mistral-large-latest",
	types.ProviderCohere:     "command-r-plus",
	types.ProviderTogether:   "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
	types.ProviderPerplexity: "llama-3.1-sonar-large-128k-online",
	types.ProviderDeepSeek:   "deepseek-chat",
	types.ProviderXAI:        "grok-beta",
	types.ProviderOllama:     "llama3.2:3b",
}

var knownProviders = []types.Provider{
	types.ProviderOpenAI, types.ProviderGemini, types.ProviderAnthropic,
	types.ProviderGroq, types.ProviderMistral, types.ProviderCohere,
	types.ProviderTogether, types.ProviderPerplexity, types.ProviderDeepSeek,
	types.ProviderXAI, types.ProviderOllama, types.ProviderCustom,
}

var configSetProviderCmd = &cobra.Command{
	Use:   "set-provider <name>",
	Short: "Set the default provider (openai, gemini, groq, ollama, ...)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := types.Provider(strings.ToLower(strings.TrimSpace(args[0])))
		ok := false
		for _, p := range knownProviders {
			if p == name {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("unknown provider %q (try: openai, gemini, anthropic, groq, mistral, cohere, together, perplexity, deepseek, xai, ollama, custom)", name)
		}
		path, err := config.GetConfigPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		v := viper.New()
		v.SetConfigType("yaml")
		v.SetConfigFile(path)
		// Ignore missing file: a fresh machine just gets the new keys.
		_ = v.ReadInConfig()
		v.Set("default_provider", string(name))
		// Scaffold a minimal entry so validation passes and the
		// provider works once an API key is added (env var or file).
		if !v.IsSet("providers." + string(name) + ".model") {
			if model, ok := defaultModels[name]; ok {
				v.Set("providers."+string(name)+".model", model)
				v.Set("providers."+string(name)+".temperature", 0.7)
				v.Set("providers."+string(name)+".max_tokens", 4096)
			} else {
				return fmt.Errorf("provider %q needs manual config: add providers.%s with base_url and model to %s", name, name, path)
			}
		}
		if err := v.WriteConfigAs(path); err != nil {
			return err
		}
		fmt.Printf("Default provider set to %s (%s)\n", name, path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd, configInitCmd, configSetProviderCmd)
	rootCmd.AddCommand(configCmd)
}
