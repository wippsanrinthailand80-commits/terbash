package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/terbash/terbash/pkg/types"
)

func TestEnsureProviderEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath: %v", err)
	}
	entry := types.ProviderConfig{Model: "llama-3.1-8b-instant", Temperature: 0.7, MaxTokens: 4096}
	if err := EnsureProviderEntry(path, types.ProviderGroq, entry); err != nil {
		t.Fatalf("EnsureProviderEntry: %v", err)
	}

	// Existing values (e.g. api_key) must never be overwritten.
	if err := EnsureProviderEntry(path, types.ProviderGroq, types.ProviderConfig{APIKey: "sk-new", Model: "other-model"}); err != nil {
		t.Fatalf("second EnsureProviderEntry: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.Providers[types.ProviderGroq]
	if got.Model != "llama-3.1-8b-instant" {
		t.Fatalf("model was overwritten: %q", got.Model)
	}
	if got.APIKey != "sk-new" {
		t.Fatalf("api_key not filled: %q", got.APIKey)
	}

	// Sanity: file lives under the temp HOME (space-safe paths work too).
	if _, err := os.Stat(filepath.Join(home, ".config", "terbash", "config.yaml")); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}
