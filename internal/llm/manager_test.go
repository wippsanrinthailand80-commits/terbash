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
