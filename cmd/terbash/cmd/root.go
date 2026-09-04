package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/internal/tui"
	"github.com/terbash/terbash/pkg/types"
)

var (
	cfgFile string
	cfg     *types.Config
)

var rootCmd = &cobra.Command{
	Use:   "terbash",
	Short: "Terbash - Ultra-lightweight ARM64 AI Agent CLI",
	Long: `A high-performance, ultra-lightweight AI agent CLI for ARM64 devices.
Supports multiple LLM providers (OpenAI, Gemini, Anthropic, Groq, Mistral, Cohere, Together, Perplexity, DeepSeek, xAI, Ollama)
with secure BYOK architecture and native tool calling.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgFile)
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/terbash/config.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	if cfgFile != "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not find home directory: %v\n", err)
		return
	}
	cfgFile = fmt.Sprintf("%s/.config/terbash/config.yaml", home)
}

func runInteractive() error {
	tui.Version = Version
	// cfg is loaded by PersistentPreRunE. Plain-terminal prompts here -
	// the fullscreen TUI cannot do hidden input well.
	quit, err := ensureAPIKey(cfg)
	if err != nil {
		return err
	}
	if quit {
		fmt.Println("Goodbye!")
		return nil
	}
	pendingReminder := autoUpdateOnLogin()
	app := tui.NewApp(cfg)
	runErr := app.Run()
	if pendingReminder != "" {
		fmt.Printf("Update available: %s — run `terbash update` (or `terbash update -y`).\n", pendingReminder)
	}
	return runErr
}