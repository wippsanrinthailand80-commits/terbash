package llm

import (
	"sort"
	"testing"

	"github.com/terbash/terbash/pkg/types"
)

func testConfig() *types.Config {
	return &types.Config{
		DefaultProvider: types.ProviderOllama,
		Providers: map[types.Provider]types.ProviderConfig{
			types.ProviderGroq: {Model: "llama-3.1-8b-instant"},
			types.ProviderOllama: {Model: "llama3.2:3b"},
		},
	}
}

func TestListProvidersSorted(t *testing.T) {
	m, err := NewManager(testConfig())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	got := m.ListProviders()
	if !sort.StringsAreSorted(got) {
		t.Fatalf("providers not sorted: %v", got)
	}
}

func TestExplicitDefaultNotHijackedByOllamaFallback(t *testing.T) {
	cfg := &types.Config{
		DefaultProvider: types.ProviderGroq,
		Providers: map[types.Provider]types.ProviderConfig{
			types.ProviderGroq: {Model: "llama-3.1-8b-instant"},
		},
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.DefaultProviderName() != "groq" {
		t.Fatalf("expected default groq, got %s", m.DefaultProviderName())
	}
	if _, err := m.GetProvider("ollama"); err != nil {
		t.Fatalf("ollama fallback should still be registered: %v", err)
	}
}

func TestEnsureProvider(t *testing.T) {
	m, err := NewManager(&types.Config{
		DefaultProvider: types.ProviderOllama,
		Providers: map[types.Provider]types.ProviderConfig{
			types.ProviderOllama: {Model: "llama3.2:3b"},
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.IsConfigured("groq") {
		t.Fatal("groq should start unconfigured")
	}
	if err := m.EnsureProvider("groq"); err != nil {
		t.Fatalf("EnsureProvider groq: %v", err)
	}
	if !m.IsConfigured("groq") {
		t.Fatal("groq should be configured after ensure")
	}
	if err := m.SetDefaultProvider("groq"); err != nil {
		t.Fatalf("switch to ensured groq: %v", err)
	}
	if err := m.EnsureProvider("some-random-cloud"); err == nil {
		t.Fatal("expected error for unknown provider without base_url")
	}
}

func TestAllProviderNames(t *testing.T) {
	m, err := NewManager(testConfig())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	names := m.AllProviderNames()
	if len(names) < 12 {
		t.Fatalf("expected all built-ins listed, got %v", names)
	}
	found := false
	for _, n := range names {
		if n == "groq" {
			found = true
		}
	}
	if !found {
		t.Fatalf("groq missing from %v", names)
	}
}

func TestCustomEndpointByName(t *testing.T) {
	cfg := &types.Config{
		DefaultProvider: "mycloud",
		Providers: map[types.Provider]types.ProviderConfig{
			"mycloud": {BaseURL: "https://llm.example.com/v1", Model: "my-model"},
		},
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager with custom endpoint: %v", err)
	}
	if m.DefaultProviderName() != "mycloud" {
		t.Fatalf("expected default mycloud, got %s", m.DefaultProviderName())
	}
}

func TestSetDefaultProvider(t *testing.T) {
	m, err := NewManager(testConfig())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.DefaultProviderName() != "ollama" {
		t.Fatalf("expected default ollama, got %s", m.DefaultProviderName())
	}
	if err := m.SetDefaultProvider("groq"); err != nil {
		t.Fatalf("switch to groq: %v", err)
	}
	if m.DefaultProviderName() != "groq" {
		t.Fatalf("expected default groq, got %s", m.DefaultProviderName())
	}
	if got := m.ProviderModel("groq"); got != "llama-3.1-8b-instant" {
		t.Fatalf("unexpected groq model: %q", got)
	}
	if err := m.SetDefaultProvider("nope"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if m.DefaultProviderName() != "groq" {
		t.Fatalf("failed switch must keep old default, got %s", m.DefaultProviderName())
	}
}
