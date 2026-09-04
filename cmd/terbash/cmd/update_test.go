package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "terbash")
	writeTestFile(t, exe, "v1-binary")

	bak, err := backupBinary(exe)
	if err != nil {
		t.Fatalf("backupBinary: %v", err)
	}
	if bak != exe+".bak" {
		t.Fatalf("unexpected backup path: %s", bak)
	}
	if _, err := os.Stat(exe); !os.IsNotExist(err) {
		t.Fatal("exe should be moved aside after backup")
	}
	writeTestFile(t, exe, "v2-binary")

	if err := restoreBackup(exe); err != nil {
		t.Fatalf("restoreBackup: %v", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("read restored exe: %v", err)
	}
	if string(data) != "v1-binary" {
		t.Fatalf("restored wrong content: %q", data)
	}
	// Backup survives restore, so rollback is repeatable.
	if _, err := os.Stat(exe + ".bak"); err != nil {
		t.Fatalf("backup should survive restore: %v", err)
	}
}

func TestRestoreWithoutBackup(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "terbash")
	if err := restoreBackup(exe); err == nil {
		t.Fatal("expected error when no .bak exists")
	}
}

func TestResolveUpdateURL(t *testing.T) {
	asset := "terbash-linux-arm64"
	if got := resolveUpdateURL("o/r", "latest", "", asset); got != "https://github.com/o/r/releases/latest/download/"+asset {
		t.Fatalf("latest URL: %s", got)
	}
	if got := resolveUpdateURL("o/r", "v0.1.1", "", asset); got != "https://github.com/o/r/releases/download/v0.1.1/"+asset {
		t.Fatalf("pinned URL: %s", got)
	}
	if got := resolveUpdateURL("o/r", "latest", "https://mirror.example.com/dir/", asset); got != "https://mirror.example.com/dir/"+asset {
		t.Fatalf("mirror URL: %s", got)
	}
}
