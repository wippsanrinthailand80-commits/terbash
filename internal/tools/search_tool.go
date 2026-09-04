package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

var defaultSkipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "__pycache__": true,
	".venv": true, "vendor": true, "dist": true, "build": true,
}

const maxSearchFileBytes = 2 << 20 // skip files bigger than 2MB

// SearchTool is a read-only recursive content search constrained to the
// workspace sandbox (ripgrep-style, zero dependencies).
type SearchTool struct {
	config *types.Config
	cwd    string
}

func NewSearchTool(cfg *types.Config, cwd string) *SearchTool {
	return &SearchTool{config: cfg, cwd: cwd}
}

func (t *SearchTool) Name() string { return "grep_search" }

func (t *SearchTool) Description() string {
	return "Search file contents recursively in the workspace (read-only)"
}

func (t *SearchTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"query":       {Type: "string", Description: "Text or pattern to search for"},
			"regex":       {Type: "boolean", Description: "Treat query as a Go regexp (default false = literal)"},
			"ignore_case": {Type: "boolean", Description: "Case-insensitive matching"},
			"include":     {Type: "string", Description: "Only files matching this glob, e.g. *.go"},
			"max_results": {Type: "integer", Description: "Max matches to return (default 50)"},
		},
		Required: []string{"query"},
	}
}

func (t *SearchTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return &types.ToolResult{Success: false, Error: "query is required"}, nil
	}
	useRegex, _ := args["regex"].(bool)
	ignoreCase, _ := args["ignore_case"].(bool)
	include, _ := args["include"].(string)
	maxResults := 50
	if n, ok := args["max_results"].(float64); ok && n > 0 {
		maxResults = int(n)
	}

	match := func(line string) bool {
		if ignoreCase {
			return strings.Contains(strings.ToLower(line), strings.ToLower(query))
		}
		return strings.Contains(line, query)
	}
	if useRegex {
		pattern := query
		if ignoreCase {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return &types.ToolResult{Success: false, Error: fmt.Sprintf("bad regexp: %v", err)}, nil
		}
		match = re.MatchString
	}

	root, err := resolveSandbox(t.cwd, ".")
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	var hits []string
	count := 0
	stop := fmt.Errorf("stop")
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if path != root && defaultSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if include != "" {
			if ok, _ := filepath.Match(include, info.Name()); !ok {
				return nil
			}
		}
		if info.Size() > maxSearchFileBytes {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if err := t.searchFile(path, rel, match, &hits, &count, maxResults); err != nil {
			if err == stop {
				return stop
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != stop {
		return &types.ToolResult{Success: false, Error: walkErr.Error()}, nil
	}

	sort.Strings(hits)
	out := strings.Join(hits, "\n")
	if count >= maxResults {
		out += fmt.Sprintf("\n… truncated at %d results", maxResults)
	}
	if out == "" {
		out = "No matches."
	}
	return &types.ToolResult{Success: true, Output: out, Metadata: map[string]string{"matches": fmt.Sprintf("%d", count)}}, nil
}

func (t *SearchTool) searchFile(path, rel string, match func(string) bool, hits *[]string, count *int, max int) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if strings.IndexByte(line, 0) >= 0 {
			return nil // binary file, skip
		}
		if match(line) {
			*hits = append(*hits, fmt.Sprintf("%s:%d: %s", rel, lineNo, strings.TrimSpace(line)))
			*count++
			if *count >= max {
				return fmt.Errorf("stop")
			}
		}
	}
	return nil
}
