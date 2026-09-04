package tools

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

// GlobTool lists workspace files matching a glob pattern with **
// support (read-only, sandbox-constrained).
type GlobTool struct {
	config *types.Config
	cwd    string
}

func NewGlobTool(cfg *types.Config, cwd string) *GlobTool {
	return &GlobTool{config: cfg, cwd: cwd}
}

func (t *GlobTool) Name() string { return "glob_files" }

func (t *GlobTool) Description() string {
	return "List workspace files by glob pattern (** supported, read-only)"
}

func (t *GlobTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"pattern":     {Type: "string", Description: "Glob pattern, e.g. **/*.go or cmd/*/main.go"},
			"max_results": {Type: "integer", Description: "Max paths to return (default 100)"},
		},
		Required: []string{"pattern"},
	}
}

func (t *GlobTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return &types.ToolResult{Success: false, Error: "pattern is required"}, nil
	}
	maxResults := 100
	if n, ok := args["max_results"].(float64); ok && n > 0 {
		maxResults = int(n)
	}

	root, err := resolveSandbox(t.cwd, ".")
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	segs := strings.Split(filepath.ToSlash(pattern), "/")
	var found []string
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if defaultSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if matchSegments(segs, strings.Split(filepath.ToSlash(rel), "/")) {
			found = append(found, rel)
		}
		return nil
	})
	if walkErr != nil {
		return &types.ToolResult{Success: false, Error: walkErr.Error()}, nil
	}

	sort.Strings(found)
	truncated := false
	if len(found) > maxResults {
		found = found[:maxResults]
		truncated = true
	}
	out := strings.Join(found, "\n")
	if out == "" {
		out = "No files matched."
	} else if truncated {
		out += fmt.Sprintf("\n… truncated at %d results", maxResults)
	}
	return &types.ToolResult{Success: true, Output: out, Metadata: map[string]string{"matches": fmt.Sprintf("%d", len(found))}}, nil
}

// matchSegments matches path segments against pattern segments where "**"
// crosses directory boundaries and "*" follows path.Match rules.
func matchSegments(pattern, target []string) bool {
	if len(pattern) == 0 {
		return len(target) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(target); i++ {
			if matchSegments(pattern[1:], target[i:]) {
				return true
			}
		}
		return false
	}
	if len(target) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], target[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], target[1:])
}
