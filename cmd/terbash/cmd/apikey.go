package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
	"golang.org/x/term"
)

// readSecret reads one line with hidden input when possible
// (falls back to plain input, e.g. piped stdin).
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	if b, err := term.ReadPassword(int(os.Stdin.Fd())); err == nil {
		fmt.Println()
		return strings.TrimSpace(string(b)), nil
	}
	var s string
	_, err := fmt.Scanln(&s)
	fmt.Println()
	return strings.TrimSpace(s), err
}

func isExitWord(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "exit" || s == "quit" || s == "q"
}

// effectiveKey returns the key the provider would use right now:
// config file first, then its native env var.
func effectiveKey(cfg *types.Config, name types.Provider) string {
	if cfg != nil {
		if e, ok := cfg.Providers[name]; ok && strings.TrimSpace(e.APIKey) != "" {
			return e.APIKey
		}
	}
	if key := types.EnvKey(name); key != "" {
		return strings.TrimSpace(os.Getenv(key))
	}
	return ""
}

// ensureAPIKey prompts for a missing key before interactive chat starts.
// Returns quit=true when the user typed exit/quit.
func ensureAPIKey(cfg *types.Config) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	name := cfg.DefaultProvider
	if !types.RequiresKey(name) || effectiveKey(cfg, name) != "" {
		return false, nil
	}
	envKey := types.EnvKey(name)
	fmt.Printf("No API key found for %s (checked config and %s).\n", name, envKey)
	key, err := readSecret("Enter API key (Enter = skip, 'exit' = quit): ")
	if err != nil || key == "" {
		fmt.Println("Continuing without a key - requests to this provider will fail.")
		return false, nil
	}
	if isExitWord(key) {
		return true, nil
	}
	path, err := config.GetConfigPath()
	if err != nil {
		return false, err
	}
	if err := config.SetProviderAPIKey(path, name, key); err != nil {
		return false, err
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[types.Provider]types.ProviderConfig)
	}
	entry := cfg.Providers[name]
	entry.APIKey = key
	cfg.Providers[name] = entry
	fmt.Println("Saved to config.")
	return false, nil
}

var setKeyValue string

var configSetKeyCmd = &cobra.Command{
	Use:   "set-key <provider>",
	Short: "Save a provider API key to the config file",
	Long: `Save a provider API key to the config file (owner-only permissions).

Without --key it prompts with hidden input. Type 'exit' at the prompt
to cancel, or press Enter to abort without saving.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := types.Provider(strings.ToLower(strings.TrimSpace(args[0])))
		if name == "" {
			return fmt.Errorf("provider name is required")
		}
		key := strings.TrimSpace(setKeyValue)
		if key == "" {
			fmt.Printf("Saving API key for %s.\n", name)
			var err error
			key, err = readSecret("Enter API key ('exit' = cancel): ")
			if err != nil || key == "" || isExitWord(key) {
				fmt.Println("Cancelled - nothing saved.")
				return nil
			}
		}
		path, err := config.GetConfigPath()
		if err != nil {
			return err
		}
		if err := config.SetProviderAPIKey(path, name, key); err != nil {
			return err
		}
		fmt.Printf("API key for %s saved to %s\n", name, path)
		return nil
	},
}

func init() {
	configSetKeyCmd.Flags().StringVar(&setKeyValue, "key", "", "API key value (otherwise prompt with hidden input)")
	configCmd.AddCommand(configSetKeyCmd)
}
