package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terbash/terbash/internal/llm"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List configured providers (● = active)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// cfg is loaded by root's PersistentPreRunE.
		m, err := llm.NewManager(cfg)
		if err != nil {
			return err
		}
		defer m.Close()

		active := m.DefaultProviderName()
		for _, n := range m.ListProviders() {
			marker := "○"
			extra := ""
			if n == active {
				marker = "●"
				extra = "  (active)"
			}
			model := m.ProviderModel(n)
			fmt.Printf("%s  %s  %s%s\n", marker, n, model, extra)
		}
		return nil
	},
}

var modelsProvider string

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List models for a provider (default: active provider)",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := llm.NewManager(cfg)
		if err != nil {
			return err
		}
		defer m.Close()

		name := modelsProvider
		if name == "" {
			name = m.DefaultProviderName()
		}
		p, err := m.GetProvider(name)
		if err != nil {
			return err
		}
		models, err := p.GetModels()
		if err != nil {
			return fmt.Errorf("could not list models for %s: %w", name, err)
		}
		if len(models) == 0 {
			fmt.Printf("No models reported by %s\n", name)
			return nil
		}
		for _, mod := range models {
			fmt.Println(mod)
		}
		return nil
	},
}

func init() {
	modelsCmd.Flags().StringVarP(&modelsProvider, "provider", "p", "", "Provider to query (default: active)")
	rootCmd.AddCommand(providersCmd, modelsCmd)
}
