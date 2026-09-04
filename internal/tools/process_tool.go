package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/terbash/terbash/pkg/types"
)

// ProcessTool lists OS processes (Linux /proc, incl. Termux/Android) and
// can terminate one - killing always needs user confirmation.
type ProcessTool struct {
	config *types.Config
}

func NewProcessTool(cfg *types.Config) *ProcessTool {
	return &ProcessTool{config: cfg}
}

func (t *ProcessTool) Name() string { return "process" }

func (t *ProcessTool) Description() string {
	return "List processes; kill by PID needs confirmation (Linux only)"
}

func (t *ProcessTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Process operation",
				Enum:        []string{"list", "kill"},
			},
			"filter": {Type: "string", Description: "Only show commands containing this text (for list)"},
			"pid":    {Type: "integer", Description: "Process id (for kill)"},
			"signal": {Type: "string", Description: "TERM (default) or KILL", Enum: []string{"TERM", "KILL"}},
		},
		Required: []string{"operation"},
	}
}

// RequiresConfirm mirrors Execute: only killing needs approval.
func (t *ProcessTool) RequiresConfirm(args map[string]interface{}) (bool, string) {
	op, _ := args["operation"].(string)
	if op == "kill" {
		return true, fmt.Sprintf("Kill PID %v?", args["pid"])
	}
	return false, ""
}

func (t *ProcessTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("process tool supports linux only (running on %s)", runtime.GOOS)}, nil
	}
	op, _ := args["operation"].(string)
	switch op {
	case "list":
		filter, _ := args["filter"].(string)
		return t.list(filter)
	case "kill":
		pid := int(floatArg(args, "pid"))
		if pid <= 1 {
			return &types.ToolResult{Success: false, Error: "a valid pid (>1) is required for kill"}, nil
		}
		sig := syscall.SIGTERM
		if s, ok := args["signal"].(string); ok && strings.ToUpper(s) == "KILL" {
			sig = syscall.SIGKILL
		}
		name := procName(pid)
		if !confirmAction(t.config.Tools.ConfirmCommands, fmt.Sprintf("Kill PID %d (%s)?", pid, name)) {
			return &types.ToolResult{Success: false, Error: "kill cancelled by user"}, nil
		}
		if err := syscall.Kill(pid, sig); err != nil {
			return &types.ToolResult{Success: false, Error: err.Error()}, nil
		}
		return &types.ToolResult{
			Success:  true,
			Output:   fmt.Sprintf("Sent %s to PID %d (%s)", sigName(sig), pid, name),
			Metadata: map[string]string{"pid": strconv.Itoa(pid)},
		}, nil
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}

func (t *ProcessTool) list(filter string) (*types.ToolResult, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	type row struct {
		pid  int
		name string
	}
	var rows []row
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		name := procName(pid)
		if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			continue
		}
		rows = append(rows, row{pid: pid, name: name})
		if len(rows) >= 200 {
			break
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pid < rows[j].pid })
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-8s %s\n", "PID", "COMMAND"))
	for _, r := range rows {
		fmt.Fprintf(&b, "%-8d %s\n", r.pid, r.name)
	}
	return &types.ToolResult{
		Success:  true,
		Output:   strings.TrimRight(b.String(), "\n"),
		Metadata: map[string]string{"count": strconv.Itoa(len(rows))},
	}, nil
}

func procName(pid int) string {
	if data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline")); err == nil && len(data) > 0 {
		parts := strings.Split(string(data), "\x00")
		var cleaned []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				cleaned = append(cleaned, p)
			}
		}
		if len(cleaned) > 0 {
			joined := strings.Join(cleaned, " ")
			if len(joined) > 120 {
				joined = joined[:120] + "…"
			}
			return joined
		}
	}
	if data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm")); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "?"
}

func sigName(sig syscall.Signal) string {
	if sig == syscall.SIGKILL {
		return "SIGKILL"
	}
	return "SIGTERM"
}
