package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type FileTool struct {
	config *types.Config
	cwd    string
}

func NewFileTool(cfg *types.Config, cwd string) *FileTool {
	return &FileTool{config: cfg, cwd: cwd}
}

func (t *FileTool) Name() string {
	return "file_operations"
}

func (t *FileTool) Description() string {
	return "Read, write, and list files in the workspace with safety confirmations"
}

func (t *FileTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Operation to perform: read, write, list, delete",
				Enum:        []string{"read", "write", "list", "delete"},
			},
			"path": {
				Type:        "string",
				Description: "Relative path to file or directory",
			},
			"content": {
				Type:        "string",
				Description: "Content to write (for write operation)",
			},
		},
		Required: []string{"operation", "path"},
	}
}

// RequiresConfirm mirrors Execute: writes and deletes need approval,
// reads and listings never do.
func (t *FileTool) RequiresConfirm(args map[string]interface{}) (bool, string) {
	op, _ := args["operation"].(string)
	path, _ := args["path"].(string)
	switch op {
	case "write":
		return true, fmt.Sprintf("Write to %s?", path)
	case "delete":
		return true, fmt.Sprintf("Delete %s?", path)
	default:
		return false, ""
	}
}

func (t *FileTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	path, _ := args["path"].(string)

	if path == "" {
		return &types.ToolResult{Success: false, Error: "path is required"}, nil
	}

	absPath, err := resolveSandbox(t.cwd, path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	switch op {
	case "read":
		return t.readFile(absPath)
	case "write":
		content, _ := args["content"].(string)
		return t.writeFile(absPath, content)
	case "list":
		return t.listDir(absPath)
	case "delete":
		return t.deleteFile(absPath)
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}

func (t *FileTool) readFile(path string) (*types.ToolResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	if info.Size() > t.config.Tools.MaxFileSize {
		return &types.ToolResult{Success: false, Error: "file too large"}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	relPath, _ := filepath.Rel(t.cwd, path)
	return &types.ToolResult{
		Success: true,
		Output:  string(content),
		Metadata: map[string]string{"path": relPath, "size": fmt.Sprintf("%d", info.Size())},
	}, nil
}

func (t *FileTool) writeFile(path, content string) (*types.ToolResult, error) {
	if relPath, err := filepath.Rel(t.cwd, path); err == nil {
		if !confirmAction(t.config.Tools.ConfirmWrites, fmt.Sprintf("Write to %s?", relPath)) {
			return &types.ToolResult{Success: false, Error: "write cancelled by user"}, nil
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	relPath, _ := filepath.Rel(t.cwd, path)
	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Written %d bytes to %s", len(content), relPath),
		Metadata: map[string]string{"path": relPath, "bytes": fmt.Sprintf("%d", len(content))},
	}, nil
}

func (t *FileTool) listDir(path string) (*types.ToolResult, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	var output strings.Builder
	for _, e := range entries {
		prefix := "  "
		if e.IsDir() {
			prefix = "📁 "
		} else {
			prefix = "📄 "
		}
		output.WriteString(fmt.Sprintf("%s%s\n", prefix, e.Name()))
	}

	relPath, _ := filepath.Rel(t.cwd, path)
	return &types.ToolResult{
		Success: true,
		Output:  output.String(),
		Metadata: map[string]string{"path": relPath},
	}, nil
}

func (t *FileTool) deleteFile(path string) (*types.ToolResult, error) {
	if relPath, err := filepath.Rel(t.cwd, path); err == nil {
		if !confirmAction(t.config.Tools.ConfirmWrites, fmt.Sprintf("Delete %s?", relPath)) {
			return &types.ToolResult{Success: false, Error: "delete cancelled by user"}, nil
		}
	}

	if err := os.Remove(path); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	relPath, _ := filepath.Rel(t.cwd, path)
	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted %s", relPath),
		Metadata: map[string]string{"path": relPath},
	}, nil
}