package llm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/terbash/terbash/pkg/providers"
	"github.com/terbash/terbash/pkg/types"
)

type Manager struct {
	providers       map[types.Provider]types.LLMProvider
	defaultProvider types.Provider
	config          *types.Config
}

func NewManager(cfg *types.Config) (*Manager, error) {
	m := &Manager{
		providers:       make(map[types.Provider]types.LLMProvider),
		defaultProvider: cfg.DefaultProvider,
		config:          cfg,
	}

	for providerName, providerConfig := range cfg.Providers {
		p, err := m.createProvider(providerName, providerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", providerName, err)
		}
		m.providers[providerName] = p
	}

	if _, ok := m.providers[m.defaultProvider]; !ok && m.defaultProvider != types.ProviderOllama {
		m.defaultProvider = types.ProviderOllama
	}

	if _, ok := m.providers[types.ProviderOllama]; !ok {
		m.providers[types.ProviderOllama] = providers.NewOllamaProvider(types.ProviderConfig{
			Model: "llama3.2:3b",
		})
		m.defaultProvider = types.ProviderOllama
	}

	return m, nil
}

func (m *Manager) createProvider(name types.Provider, cfg types.ProviderConfig) (types.LLMProvider, error) {
	switch name {
	case types.ProviderOpenAI:
		return providers.NewOpenAIProvider(cfg), nil
	case types.ProviderGroq:
		return providers.NewGroqProvider(cfg), nil
	case types.ProviderMistral:
		return providers.NewMistralProvider(cfg), nil
	case types.ProviderCohere:
		return providers.NewCohereProvider(cfg), nil
	case types.ProviderTogether:
		return providers.NewTogetherProvider(cfg), nil
	case types.ProviderPerplexity:
		return providers.NewPerplexityProvider(cfg), nil
	case types.ProviderDeepSeek:
		return providers.NewDeepSeekProvider(cfg), nil
	case types.ProviderXAI:
		return providers.NewXAIProvider(cfg), nil
	case types.ProviderCustom:
		return providers.NewCustomProvider(cfg), nil
	case types.ProviderAnthropic:
		return providers.NewAnthropicProvider(cfg), nil
	case types.ProviderGemini:
		return providers.NewGeminiProvider(cfg), nil
	case types.ProviderOllama:
		return providers.NewOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func (m *Manager) GetProvider(name string) (types.LLMProvider, error) {
	provider := types.Provider(strings.ToLower(name))
	if provider == "" {
		provider = m.defaultProvider
	}
	p, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not available", provider)
	}
	return p, nil
}

func (m *Manager) GetDefaultProvider() types.LLMProvider {
	return m.providers[m.defaultProvider]
}

// DefaultProviderName returns the active provider's name.
func (m *Manager) DefaultProviderName() string {
	return string(m.defaultProvider)
}

// SetDefaultProvider switches the active provider for this session.
func (m *Manager) SetDefaultProvider(name string) error {
	p := types.Provider(strings.ToLower(strings.TrimSpace(name)))
	if p == "" {
		return fmt.Errorf("provider name is required")
	}
	if _, ok := m.providers[p]; !ok {
		return fmt.Errorf("provider %s not available (use /providers to list)", p)
	}
	m.defaultProvider = p
	return nil
}

// ProviderModel returns the configured model for a provider, or "".
func (m *Manager) ProviderModel(name string) string {
	p, err := m.GetProvider(name)
	if err != nil {
		return ""
	}
	return p.GetConfig().Model
}

func (m *Manager) ListProviders() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, string(name))
	}
	sort.Strings(names)
	return names
}

func (m *Manager) Close() error {
	for _, p := range m.providers {
		p.Close()
	}
	return nil
}

func (m *Manager) GetConfig() *types.Config {
	return m.config
}