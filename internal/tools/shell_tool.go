package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
)

type ShellTool struct {
	config *config.Config
	cwd    string
}

func NewShellTool(cfg *config.Config, cwd string) *ShellTool {
	return &ShellTool{config: cfg, cwd: cwd}
}

func (t *ShellTool) Name() string {
	return "shell_exec"
}

func (t *ShellTool) Description() string {
	return "Execute shell commands with safety confirmations"
}

func (t *ShellTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"command": {
				Type:        "string",
				Description: "Command to execute",
			},
			"args": {
				Type:        "array",
				Description: "Command arguments",
				Items: &types.Property{Type: "string"},
			},
			"timeout": {
				Type:        "integer",
				Description: "Timeout in seconds (default 30)",
			},
		},
		Required: []string{"command"},
	}
}

func (t *ShellTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	cmdStr, _ := args["command"].(string)
	if cmdStr == "" {
		return &types.ToolResult{Success: false, Error: "command is required"}, nil
	}

	argsList := []string{}
	if argsArray, ok := args["args"].([]interface{}); ok {
		for _, a := range argsArray {
			if s, ok := a.(string); ok {
				argsList = append(argsList, s)
			}
		}
	}

	timeout := 30
	if tVal, ok := args["timeout"].(float64); ok {
		timeout = int(tVal)
	}

	dangerous := []string{"rm -rf", "mkfs", "dd if=", "> /dev/", "chmod 777", "chown -R", ":(){ :|:& };:"}
	cmdLower := strings.ToLower(cmdStr)
	for _, d := range dangerous {
		if strings.Contains(cmdLower, d) {
			return &types.ToolResult{Success: false, Error: fmt.Sprintf("dangerous command blocked: %s", d)}, nil
		}
	}

	if t.config.Tools.ConfirmCommands {
		fmt.Printf("Execute: %s %s? [y/N]: ", cmdStr, strings.Join(argsList, " "))
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return &types.ToolResult{Success: false, Error: "command cancelled by user"}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdStr, argsList...)
	cmd.Dir = t.cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	output, err := cmd.CombinedOutput()
	result := &types.ToolResult{
		Success: err == nil,
		Output:  string(output),
		Metadata: map[string]string{
			"command": cmdStr,
			"args":    strings.Join(argsList, " "),
			"cwd":     t.cwd,
		},
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}