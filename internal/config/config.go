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