package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/terbash/terbash/pkg/types"
)

func Load(configFile string) (*types.Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir := filepath.Join(home, ".config", "terbash")
		v.AddConfigPath(configDir)
		v.SetConfigName("config")
	}

	v.SetEnvPrefix("TERBASH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		// Fresh installs have no config file yet - that is fine, use
		// defaults + env vars. Viper returns different error types
		// depending on whether SetConfigFile or AddConfigPath was used,
		// so check for "not found" broadly (including paths with spaces).
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok &&
			!os.IsNotExist(err) &&
			!strings.Contains(strings.ToLower(err.Error()), "no such file") &&
			!strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	cfg := &types.Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyNativeEnvKeys(cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("default_provider", "ollama")
	v.SetDefault("tools.confirm_writes", true)
	v.SetDefault("tools.confirm_commands", true)
	v.SetDefault("tools.sandbox_enabled", true)
	v.SetDefault("tools.max_file_size", 10485760)
	v.SetDefault("ui.theme", "dark")
	v.SetDefault("ui.show_tokens", true)
	v.SetDefault("ui.stream_output", true)
	v.SetDefault("godot.binary_path", "godot")
}

// EnsureProviderEntry adds a provider section to the config file, but only
// fills in fields that are missing - existing values (especially api_key)
// are never overwritten.
func EnsureProviderEntry(path string, name types.Provider, entry types.ProviderConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	_ = v.ReadInConfig() // ignore missing file on fresh machines
	prefix := "providers." + string(name) + "."
	setIfMissing := func(key, value string) {
		if value != "" && !v.IsSet(prefix+key) {
			v.Set(prefix+key, value)
		}
	}
	setIfMissing("api_key", entry.APIKey)
	setIfMissing("base_url", entry.BaseURL)
	setIfMissing("model", entry.Model)
	if !v.IsSet(prefix+"temperature") && entry.Temperature != 0 {
		v.Set(prefix+"temperature", entry.Temperature)
	}
	if !v.IsSet(prefix+"max_tokens") && entry.MaxTokens != 0 {
		v.Set(prefix+"max_tokens", entry.MaxTokens)
	}
	return v.WriteConfigAs(path)
}

// applyNativeEnvKeys fills empty api_key fields from provider-native env
// vars (OPENAI_API_KEY, GROQ_API_KEY, ...). File values always win, and the
// file is never modified - this is memory-only.
func applyNativeEnvKeys(cfg *types.Config) {
	if cfg.Providers == nil {
		cfg.Providers = make(map[types.Provider]types.ProviderConfig)
	}
	for _, name := range types.SupportedProviders() {
		key := types.EnvKey(name)
		if key == "" {
			continue
		}
		if v := strings.TrimSpace(os.Getenv(key)); v == "" {
			continue
		}
		entry := cfg.Providers[name]
		if entry.APIKey != "" {
			continue
		}
		entry.APIKey = strings.TrimSpace(os.Getenv(key))
		if entry.Model == "" {
			if def, ok := types.DefaultModel(name); ok {
				entry.Model = def
			}
		}
		cfg.Providers[name] = entry
	}
}

// SetProviderAPIKey writes (or overwrites) a provider's api_key in the
// config file and tightens the file to owner-only, since it holds secrets.
func SetProviderAPIKey(path string, name types.Provider, apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API key must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	_ = v.ReadInConfig() // ignore missing file on fresh machines
	v.Set("providers."+string(name)+".api_key", apiKey)
	if err := v.WriteConfigAs(path); err != nil {
		return err
	}
	// Best-effort: secrets file should not be world-readable.
	_ = os.Chmod(path, 0600)
	return nil
}

func Validate(c *types.Config) error {
	if c.DefaultProvider == "" {
		c.DefaultProvider = types.ProviderOllama
	}
	if c.Providers == nil {
		c.Providers = make(map[types.Provider]types.ProviderConfig)
	}
	if _, ok := c.Providers[c.DefaultProvider]; !ok && c.DefaultProvider != types.ProviderOllama {
		return fmt.Errorf("default provider %s not configured", c.DefaultProvider)
	}
	return nil
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "terbash", "config.yaml"), nil
}

func EnsureConfigDir() error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(path), 0755)
}