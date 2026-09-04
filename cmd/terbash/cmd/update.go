package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	updateRepo    string
	updateVersion string
	updateCheck   bool
	updateYes     bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Self-update terbash to the latest GitHub release",
	Long: `Download the latest terbash binary for your OS/arch from GitHub
releases and replace the currently running binary.

All file paths are handled with spaces in mind (no shell string
splitting - everything uses filepath + direct file I/O).`,
	RunE:         runUpdate,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// networkHint returns extra guidance when err looks like a local
// network/DNS problem (e.g. Termux with no working resolver).
func networkHint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"no such host", "lookup ", "connection refused",
		"network is unreachable", "temporary failure",
		"i/o timeout", "client.timeout",
	} {
		if strings.Contains(msg, s) {
			return "Hint: this looks like a network/DNS problem on this device, not a terbash bug.\n" +
				"Try: check internet/VPN, then run: getprop net.dns1\n" +
				"If DNS is broken in Termux, fix resolvers with:\n" +
				"  printf 'nameserver 8.8.8.8\\nnameserver 1.1.1.1\\n' > $PREFIX/etc/resolv.conf"
		}
	}
	return ""
}

func downloadErr(op string, err error) error {
	if h := networkHint(err); h != "" {
		return fmt.Errorf("%s: %v\n%s", op, err, h)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func init() {
	updateCmd.Flags().StringVar(&updateRepo, "repo", "wippsanrinthailand80-commits/terbash", "GitHub repo in owner/name form")
	updateCmd.Flags().StringVar(&updateVersion, "version", "latest", "Release tag to install, or 'latest'")
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Only print the download URL, do not install")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(updateCmd)
}

func updateAssetName() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var arch string
	switch goarch {
	case "arm64":
		arch = "arm64"
	case "amd64":
		arch = "amd64"
	case "arm":
		arch = "arm"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	var osName string
	switch goos {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}

	asset := fmt.Sprintf("terbash-%s-%s", osName, arch)
	if osName == "windows" {
		asset += ".exe"
	}
	return asset, nil
}

func updateDownloadURL(repo, version, asset string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, asset)
	}
	version = strings.TrimPrefix(version, "v")
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, version, asset)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	asset, err := updateAssetName()
	if err != nil {
		return err
	}
	url := updateDownloadURL(updateRepo, updateVersion, asset)

	if updateCheck {
		fmt.Printf("Detected: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Asset: %s\n", asset)
		fmt.Printf("URL: %s\n", url)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current binary: %w", err)
	}
	// Resolve symlinks (e.g. $PREFIX/bin/terbash) so we replace the real file.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Printf("Detected: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Downloading from: %s\n", url)

	if !updateYes {
		fmt.Printf("Replace %s? [y/N]: ", exe)
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("update cancelled")
		}
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			return fmt.Errorf("update cancelled")
		}
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return downloadErr("download failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d - check %s", resp.StatusCode, fmt.Sprintf("https://github.com/%s/releases", updateRepo))
	}

	// Write to a temp file in the SAME directory so rename stays on one
	// filesystem (works even when paths contain spaces).
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "terbash-update-*")
	if err != nil {
		return fmt.Errorf("cannot write temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // cleaned up on success too (after rename it is gone, Remove is a no-op)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return downloadErr("download failed", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("cannot replace %s (try re-running with sudo): %w", exe, err)
	}

	fmt.Printf("Updated %s\n", exe)
	fmt.Println("Run 'terbash --help' to verify.")
	return nil
}
