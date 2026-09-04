package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
)

type GodotTool struct {
	config *config.Config
	cwd    string
}

func NewGodotTool(cfg *config.Config, cwd string) *GodotTool {
	return &GodotTool{config: cfg, cwd: cwd}
}

func (t *GodotTool) Name() string {
	return "godot_headless"
}

func (t *GodotTool) Description() string {
	return "Run Godot 4.x headless for automated testing, exports, and QA"
}

func (t *GodotTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Operation: run_script, run_tests, export, debug",
				Enum:        []string{"run_script", "run_tests", "export", "debug"},
			},
			"script_path": {
				Type:        "string",
				Description: "Path to GDScript file to execute (for run_script)",
			},
			"project_path": {
				Type:        "string",
				Description: "Godot project path (defaults to config or cwd)",
			},
			"export_preset": {
				Type:        "string",
				Description: "Export preset name (for export operation)",
			},
			"export_path": {
				Type:        "string",
				Description: "Output path for export",
			},
			"args": {
				Type:        "array",
				Description: "Additional arguments to pass to Godot",
				Items: &types.Property{Type: "string"},
			},
			"timeout": {
				Type:        "integer",
				Description: "Timeout in seconds (default 120)",
			},
		},
		Required: []string{"operation"},
	}
}

func (t *GodotTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	if op == "" {
		return &types.ToolResult{Success: false, Error: "operation is required"}, nil
	}

	projectPath := t.getProjectPath(args)
	if projectPath == "" {
		return &types.ToolResult{Success: false, Error: "no Godot project found (need project.godot)"}, nil
	}

	timeout := 120
	if tVal, ok := args["timeout"].(float64); ok {
		timeout = int(tVal)
	}

	godotBin := t.config.Godot.BinaryPath
	if godotBin == "" {
		godotBin = "godot"
	}

	switch op {
	case "run_script":
		return t.runScript(godotBin, projectPath, args, timeout)
	case "run_tests":
		return t.runTests(godotBin, projectPath, args, timeout)
	case "export":
		return t.exportProject(godotBin, projectPath, args, timeout)
	case "debug":
		return t.debugProject(godotBin, projectPath, args, timeout)
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}

func (t *GodotTool) getProjectPath(args map[string]interface{}) string {
	if p, ok := args["project_path"].(string); ok && p != "" {
		return p
	}
	if t.config.Godot.ProjectPath != "" {
		return t.config.Godot.ProjectPath
	}
	return t.findProjectFile(t.cwd)
}

func (t *GodotTool) findProjectFile(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "project.godot")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "project.binary")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func (t *GodotTool) runScript(godotBin, projectPath string, args map[string]interface{}, timeout int) (*types.ToolResult, error) {
	scriptPath, _ := args["script_path"].(string)
	if scriptPath == "" {
		return &types.ToolResult{Success: false, Error: "script_path required for run_script"}, nil
	}

	absScript := scriptPath
	if !filepath.IsAbs(scriptPath) {
		absScript = filepath.Join(projectPath, scriptPath)
	}

	cmdArgs := []string{"--headless", "-s", absScript}
	if extraArgs, ok := args["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	return t.execGodot(godotBin, projectPath, cmdArgs, timeout)
}

func (t *GodotTool) runTests(godotBin, projectPath string, args map[string]interface{}, timeout int) (*types.ToolResult, error) {
	cmdArgs := []string{"--headless", "--verbose", "-s", "res://test_runner.gd"}
	if extraArgs, ok := args["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	return t.execGodot(godotBin, projectPath, cmdArgs, timeout)
}

func (t *GodotTool) exportProject(godotBin, projectPath string, args map[string]interface{}, timeout int) (*types.ToolResult, error) {
	preset, _ := args["export_preset"].(string)
	exportPath, _ := args["export_path"].(string)

	if preset == "" {
		return &types.ToolResult{Success: false, Error: "export_preset required for export"}, nil
	}
	if exportPath == "" {
		return &types.ToolResult{Success: false, Error: "export_path required for export"}, nil
	}

	cmdArgs := []string{"--headless", "--export-release", preset, exportPath}
	if extraArgs, ok := args["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	return t.execGodot(godotBin, projectPath, cmdArgs, timeout)
}

func (t *GodotTool) debugProject(godotBin, projectPath string, args map[string]interface{}, timeout int) (*types.ToolResult, error) {
	cmdArgs := []string{"--headless", "--remote-debug", "localhost:6007"}
	if extraArgs, ok := args["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, godotBin, cmdArgs...)
	cmd.Dir = projectPath
	output, err := cmd.CombinedOutput()

	return &types.ToolResult{
		Success: err == nil,
		Output:  string(output),
		Error:   errToString(err),
		Metadata: map[string]string{
			"project_path": projectPath,
			"command":      godotBin + " " + strings.Join(cmdArgs, " "),
		},
	}, nil
}

func (t *GodotTool) execGodot(godotBin, projectPath string, cmdArgs []string, timeout int) (*types.ToolResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, godotBin, cmdArgs...)
	cmd.Dir = projectPath
	cmd.Env = append(os.Environ(),
		"GODOT_HEADLESS=1",
		"DISPLAY=",
	)

	output, err := cmd.CombinedOutput()

	return &types.ToolResult{
		Success: err == nil,
		Output:  string(output),
		Error:   errToString(err),
		Metadata: map[string]string{
			"project_path": projectPath,
			"command":      godotBin + " " + strings.Join(cmdArgs, " "),
		},
	}, nil
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}