package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// resolveSandbox maps a user-supplied relative path into the workspace,
// rejecting absolute paths, directory traversal and sandbox escapes.
// Every file-touching tool must go through this.
func resolveSandbox(cwd, relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", relPath)
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("directory traversal not allowed: %s", relPath)
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(absCwd, clean))
	if err != nil {
		return "", err
	}
	if abs != absCwd && !strings.HasPrefix(abs, absCwd+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes sandbox: %s", relPath)
	}
	return abs, nil
}

// ConfirmFunc performs the actual [y/N] interaction. The default reads
// stdin; interactive UIs override it (e.g. auto-approve because they show
// their own approval overlay before executing).
var ConfirmFunc = func(enabled bool, prompt string) bool {
	if !enabled {
		return true
	}
	fmt.Printf("%s [y/N]: ", prompt)
	var answer string
	fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// confirmAction prints a [y/N] prompt when enabled and reports whether
// the user approved. Destructive operations must gate on this.
func confirmAction(enabled bool, prompt string) bool {
	return ConfirmFunc(enabled, prompt)
}
