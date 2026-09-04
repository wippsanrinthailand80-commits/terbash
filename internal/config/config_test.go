package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestNativeEnvKeyFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GROQ_API_KEY", "env-groq-key")

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath: %v", err)
	}
	// File entry has a model but no key - env must fill it in memory only.
	if err := EnsureProviderEntry(path, types.ProviderGroq, types.ProviderConfig{Model: "llama-3.1-8b-instant"}); err != nil {
		t.Fatalf("EnsureProviderEntry: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Providers[types.ProviderGroq].APIKey; got != "env-groq-key" {
		t.Fatalf("env fallback not applied: %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "env-groq-key") {
		t.Fatal("env key must not be written to the file")
	}
}

func TestSetProviderAPIKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath: %v", err)
	}
	if err := SetProviderAPIKey(path, types.ProviderGroq, "sk-first"); err != nil {
		t.Fatalf("SetProviderAPIKey: %v", err)
	}
	if err := SetProviderAPIKey(path, types.ProviderGroq, "sk-second"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Providers[types.ProviderGroq].APIKey; got != "sk-second" {
		t.Fatalf("key not overwritten: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secrets file should be 0600, got %o", info.Mode().Perm())
	}
	if err := SetProviderAPIKey(path, types.ProviderGroq, "  "); err == nil {
		t.Fatal("expected error for empty key")
	}
}
