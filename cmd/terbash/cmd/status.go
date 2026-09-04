package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/internal/llm"
	"github.com/terbash/terbash/internal/tools"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show provider, model, tools and config status",
	RunE: func(cmd *cobra.Command, args []string) error {
		// cfg is loaded by root's PersistentPreRunE.
		m, err := llm.NewManager(cfg)
		if err != nil {
			return err
		}
		defer m.Close()

		path, _ := config.GetConfigPath()
		active := m.DefaultProviderName()
		fmt.Printf("terbash %s\n", Version)
		fmt.Printf("config: %s\n", path)
		fmt.Printf("provider: %s (model: %s)\n", active, m.ProviderModel(active))
		fmt.Printf("providers configured: %d\n", len(m.ListProviders()))
		fmt.Printf("tools: %d\n", len(tools.NewRegistry(cfg, ".").List()))
		fmt.Printf("godot: %s\n", cfg.Godot.BinaryPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
