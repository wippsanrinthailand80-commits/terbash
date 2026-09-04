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
	Short: "Manage terbash config (path, init, set-provider, add-provider)",
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

var configSetProviderCmd = &cobra.Command{
	Use:   "set-provider <name>",
	Short: "Set the default provider (openai, gemini, groq, ollama, ...)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := types.Provider(strings.ToLower(strings.TrimSpace(args[0])))
		if !types.IsSupported(name) {
			return fmt.Errorf("unknown provider %q (built-ins: %s; anything else needs: config add-provider)", name, providerNames())
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
			if model, ok := types.DefaultModel(name); ok {
				v.Set("providers."+string(name)+".model", model)
				v.Set("providers."+string(name)+".temperature", 0.7)
				v.Set("providers."+string(name)+".max_tokens", 4096)
			} else {
				return fmt.Errorf("provider %q needs manual config: terbash config add-provider --name %s --base-url https://... --model ...", name, name)
			}
		}
		if err := v.WriteConfigAs(path); err != nil {
			return err
		}
		fmt.Printf("Default provider set to %s (%s)\n", name, path)
		if key := types.EnvKey(name); key != "" {
			fmt.Printf("Add your key via %s env var or providers.%s.api_key in config\n", key, name)
		}
		return nil
	},
}

func providerNames() string {
	names := make([]string, 0)
	for _, p := range types.SupportedProviders() {
		names = append(names, string(p))
	}
	return strings.Join(names, ", ")
}

var (
	addName        string
	addBaseURL     string
	addModel       string
	addAPIKey      string
	addTemperature float64
	addMaxTokens   int
	addSetDefault  bool
)

var configAddProviderCmd = &cobra.Command{
	Use:   "add-provider",
	Short: "Add a provider (built-in or fully custom OpenAI-compatible endpoint)",
	Long: `Add or update a provider entry in the config file. Existing values
(like api_key) are never overwritten.

Examples:
  terbash config add-provider --name groq
  terbash config add-provider --name mycloud --base-url https://llm.example.com/v1 --model my-model --api-key sk-... --set-default`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := types.Provider(strings.ToLower(strings.TrimSpace(addName)))
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if !types.IsSupported(name) && strings.TrimSpace(addBaseURL) == "" {
			return fmt.Errorf("%q is not built-in: --base-url is required for custom endpoints", name)
		}
		model := strings.TrimSpace(addModel)
		if model == "" {
			if def, ok := types.DefaultModel(name); ok {
				model = def
			} else {
				return fmt.Errorf("--model is required for %q", name)
			}
		}
		path, err := config.GetConfigPath()
		if err != nil {
			return err
		}
		entry := types.ProviderConfig{
			APIKey:      addAPIKey,
			BaseURL:     strings.TrimSpace(addBaseURL),
			Model:       model,
			Temperature: addTemperature,
			MaxTokens:   addMaxTokens,
		}
		if err := config.EnsureProviderEntry(path, name, entry); err != nil {
			return err
		}
		if addSetDefault {
			v := viper.New()
			v.SetConfigType("yaml")
			v.SetConfigFile(path)
			_ = v.ReadInConfig()
			v.Set("default_provider", string(name))
			if err := v.WriteConfigAs(path); err != nil {
				return err
			}
		}
		fmt.Printf("Provider %q saved to %s\n", name, path)
		if key := types.EnvKey(name); key != "" && addAPIKey == "" {
			fmt.Printf("Add your key via %s env var or --api-key\n", key)
		}
		return nil
	},
}

func init() {
	configAddProviderCmd.Flags().StringVar(&addName, "name", "", "Provider name (built-in or your own label)")
	configAddProviderCmd.Flags().StringVar(&addBaseURL, "base-url", "", "Base URL for custom OpenAI-compatible endpoints")
	configAddProviderCmd.Flags().StringVar(&addModel, "model", "", "Model name (defaults per built-in provider)")
	configAddProviderCmd.Flags().StringVar(&addAPIKey, "api-key", "", "API key (optional; env var also works)")
	configAddProviderCmd.Flags().Float64Var(&addTemperature, "temperature", 0.7, "Sampling temperature")
	configAddProviderCmd.Flags().IntVar(&addMaxTokens, "max-tokens", 4096, "Max response tokens")
	configAddProviderCmd.Flags().BoolVar(&addSetDefault, "set-default", false, "Also make it the default provider")
	configCmd.AddCommand(configPathCmd, configInitCmd, configSetProviderCmd, configAddProviderCmd)
	rootCmd.AddCommand(configCmd)
}
