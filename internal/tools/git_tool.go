package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/terbash/terbash/pkg/types"
)

// GitTool wraps read-only git inspection plus gated mutating operations.
// Everything runs with `git -C <cwd>` so it never leaves the workspace,
// and add/commit/push need explicit user confirmation.
type GitTool struct {
	config *types.Config
	cwd    string
}

func NewGitTool(cfg *types.Config, cwd string) *GitTool {
	return &GitTool{config: cfg, cwd: cwd}
}

func (t *GitTool) Name() string { return "git_ops" }

func (t *GitTool) Description() string {
	return "Git status/diff/log inspection; add/commit/push need confirmation"
}

func (t *GitTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Git operation",
				Enum:        []string{"status", "diff", "log", "branch", "show", "add", "commit", "push"},
			},
			"args":    {Type: "string", Description: "Extra args, e.g. pathspec for add, ref for show/log"},
			"message": {Type: "string", Description: "Commit message (for commit)"},
		},
		Required: []string{"operation"},
	}
}

var gitMutating = map[string]bool{"add": true, "commit": true, "push": true}

// RequiresConfirm mirrors Execute: add/commit/push need approval.
func (t *GitTool) RequiresConfirm(args map[string]interface{}) (bool, string) {
	op, _ := args["operation"].(string)
	if gitMutating[op] {
		return true, fmt.Sprintf("git %s?", op)
	}
	return false, ""
}

func (t *GitTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	extra, _ := args["args"].(string)
	message, _ := args["message"].(string)

	var gitArgs []string
	switch op {
	case "status":
		gitArgs = []string{"status", "--short", "--branch"}
	case "diff":
		gitArgs = []string{"diff"}
		if extra != "" {
			gitArgs = append(gitArgs, extra)
		}
	case "log":
		gitArgs = []string{"log", "--oneline", "-20"}
		if extra != "" {
			gitArgs = append(gitArgs, extra)
		}
	case "branch":
		gitArgs = []string{"branch", "-vv"}
	case "show":
		if extra == "" {
			extra = "HEAD"
		}
		gitArgs = []string{"show", "--stat", extra}
	case "add":
		if extra == "" {
			return &types.ToolResult{Success: false, Error: "args (pathspec) is required for add"}, nil
		}
		gitArgs = []string{"add", "--", extra}
	case "commit":
		if strings.TrimSpace(message) == "" {
			return &types.ToolResult{Success: false, Error: "message is required for commit"}, nil
		}
		gitArgs = []string{"commit", "-m", message}
	case "push":
		gitArgs = []string{"push"}
		if extra != "" {
			gitArgs = append(gitArgs, strings.Fields(extra)...)
		}
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}

	if gitMutating[op] {
		if !confirmAction(t.config.Tools.ConfirmCommands, fmt.Sprintf("git %s?", strings.Join(gitArgs, " "))) {
			return &types.ToolResult{Success: false, Error: "git operation cancelled by user"}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", t.cwd}, gitArgs...)...)
	out, err := cmd.CombinedOutput()
	result := &types.ToolResult{
		Success:  err == nil,
		Output:   strings.TrimRight(string(out), "\n"),
		Metadata: map[string]string{"operation": op},
	}
	if result.Output == "" && result.Success {
		result.Output = "(no output - clean)"
	}
	if err != nil {
		result.Error = strings.TrimSpace(string(out))
		if result.Error == "" {
			result.Error = err.Error()
		}
	}
	return result, nil
}
