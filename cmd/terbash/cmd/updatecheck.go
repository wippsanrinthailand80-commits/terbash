package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const githubAPI = "https://api.github.com"

// fetchLatestTag asks GitHub for the newest release tag. Any failure means
// "unknown" - the caller treats that as "no update" and stays silent.
func fetchLatestTag(apiBase, repo string) (string, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(strings.TrimSuffix(apiBase, "/") + "/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var v struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&v); err != nil {
		return "", err
	}
	if strings.TrimSpace(v.TagName) == "" {
		return "", fmt.Errorf("empty tag")
	}
	return v.TagName, nil
}

func parseVer(s string) ([]int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// isNewerAvailable reports whether latest is strictly newer than current.
// Unparseable versions (dev builds) never trigger.
func isNewerAvailable(current, latest string) bool {
	c, ok1 := parseVer(current)
	l, ok2 := parseVer(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := 0; i < len(c) && i < len(l); i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return len(l) > len(c)
}

func promptYesNo(prompt string) bool {
	fmt.Print(prompt)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// autoUpdateOnLogin checks GitHub for a newer release when chat starts.
// Returns a tag for the logout reminder when an update exists but was not
// installed now. Silent on any failure; TERBASH_NO_AUTOUPDATE=1 disables.
func autoUpdateOnLogin() string {
	if os.Getenv("TERBASH_NO_AUTOUPDATE") != "" {
		return ""
	}
	fmt.Println("Checking for updates…")
	latest, err := fetchLatestTag(githubAPI, updateRepo)
	if err != nil || !isNewerAvailable(Version, latest) {
		return ""
	}
	if !promptYesNo(fmt.Sprintf("Update available: %s → %s. Update now? [y/N]: ", Version, latest)) {
		return latest
	}
	asset, err := updateAssetName()
	if err != nil {
		fmt.Printf("Update skipped: %v\n", err)
		return latest
	}
	exe, err := currentExe()
	if err != nil {
		fmt.Printf("Update skipped: %v\n", err)
		return latest
	}
	url := updateDownloadURL(updateRepo, latest, asset)
	fmt.Printf("Downloading from: %s\n", url)
	if err := downloadAndInstall(url, exe, true); err != nil {
		if h := networkHint(err); h != "" {
			fmt.Printf("Update failed: %v\n%s\n", err, h)
		} else {
			fmt.Printf("Update failed: %v\n", err)
		}
		return latest
	}
	fmt.Printf("Updated to %s - restarting into the new version…\n", latest)
	// Replace this process with the fresh binary; on failure (e.g.
	// Windows file locking) keep going and ask for a manual restart.
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		fmt.Println("Restart terbash to use the new version.")
	}
	return ""
}
