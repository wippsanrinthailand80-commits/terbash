package llm

import (
	"fmt"
	"os"
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

	// Ollama is always registered as a local fallback, but it must not
	// hijack an explicitly configured default provider.
	if _, ok := m.providers[types.ProviderOllama]; !ok {
		m.providers[types.ProviderOllama] = providers.NewOllamaProvider(types.ProviderConfig{
			Model: "llama3.2:3b",
		})
	}

	if _, ok := m.providers[m.defaultProvider]; !ok {
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
	case types.ProviderLlamaCpp:
		return providers.NewLlamaCppProvider(cfg), nil
	default:
		// Any other name is treated as an OpenAI-compatible custom
		// endpoint, as long as a base_url is configured.
		if strings.TrimSpace(cfg.BaseURL) != "" {
			return providers.NewCustomProvider(cfg), nil
		}
		return nil, fmt.Errorf("unknown provider: %s (add base_url + model via: terbash config add-provider --name %s --base-url https://... --model ...)", name, name)
	}
}

// IsConfigured reports whether a provider is usable right now.
func (m *Manager) IsConfigured(name string) bool {
	_, ok := m.providers[types.Provider(strings.ToLower(strings.TrimSpace(name)))]
	return ok
}

// EnsureProvider registers a provider that is not configured yet.
// Built-ins get sensible defaults; anything else needs base_url + model
// (see: terbash config add-provider).
func (m *Manager) EnsureProvider(name string) error {
	p := types.Provider(strings.ToLower(strings.TrimSpace(name)))
	if p == "" {
		return fmt.Errorf("provider name is required")
	}
	if _, ok := m.providers[p]; ok {
		return nil
	}
	model, ok := types.DefaultModel(p)
	if !ok {
		return fmt.Errorf("provider %s needs manual setup first: terbash config add-provider --name %s --base-url https://... --model ...", p, p)
	}
	entry := types.ProviderConfig{Model: model, Temperature: 0.7, MaxTokens: 4096}
	prov, err := m.createProvider(p, entry)
	if err != nil {
		return err
	}
	m.providers[p] = prov
	if m.config.Providers == nil {
		m.config.Providers = make(map[types.Provider]types.ProviderConfig)
	}
	m.config.Providers[p] = entry
	return nil
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

// EffectiveAPIKey returns the API key a provider will actually use:
// config file first, then its native env var. "" means none found.
func (m *Manager) EffectiveAPIKey(name string) string {
	p := types.Provider(strings.ToLower(strings.TrimSpace(name)))
	if e, ok := m.config.Providers[p]; ok && strings.TrimSpace(e.APIKey) != "" {
		return e.APIKey
	}
	if key := types.EnvKey(p); key != "" {
		return strings.TrimSpace(os.Getenv(key))
	}
	return ""
}

// ProviderModel returns the configured model for a provider, or "".
func (m *Manager) ProviderModel(name string) string {
	p, err := m.GetProvider(name)
	if err != nil {
		return ""
	}
	return p.GetConfig().Model
}

// SetActiveModel switches the model of the active provider for this session.
func (m *Manager) SetActiveModel(model string) {
	m.GetDefaultProvider().SetModel(strings.TrimSpace(model))
}

func (m *Manager) ListProviders() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, string(name))
	}
	sort.Strings(names)
	return names
}

// AllProviderNames returns every selectable provider: all built-ins plus
// any custom endpoints from config. Sorted.
func (m *Manager) AllProviderNames() []string {
	seen := make(map[types.Provider]bool)
	for _, p := range types.SupportedProviders() {
		seen[p] = true
	}
	for name := range m.providers {
		seen[name] = true
	}
	names := make([]string, 0, len(seen))
	for p := range seen {
		names = append(names, string(p))
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