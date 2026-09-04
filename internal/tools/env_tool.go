package tools

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

// EnvTool reads environment variables. Values that look like secrets are
// redacted unless the caller explicitly opts into reveal=true.
type EnvTool struct{}

func NewEnvTool() *EnvTool { return &EnvTool{} }

func (t *EnvTool) Name() string { return "env_vars" }

func (t *EnvTool) Description() string {
	return "Read environment variables (secrets redacted unless reveal=true)"
}

func (t *EnvTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Env operation",
				Enum:        []string{"get", "list"},
			},
			"name":   {Type: "string", Description: "Variable name (for get)"},
			"prefix": {Type: "string", Description: "Only list variables starting with this (for list)"},
			"reveal": {Type: "boolean", Description: "Show secret values (default false = redacted)"},
		},
		Required: []string{"operation"},
	}
}

func isSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, s := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "_PASS", "AUTH", "CREDENTIAL"} {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

func (t *EnvTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	reveal, _ := args["reveal"].(bool)

	show := func(name, value string) string {
		if value == "" {
			return "(unset)"
		}
		if !reveal && isSecretName(name) {
			return "***redacted (use reveal=true to show)***"
		}
		return value
	}

	switch op {
	case "get":
		name, _ := args["name"].(string)
		if strings.TrimSpace(name) == "" {
			return &types.ToolResult{Success: false, Error: "name is required for get"}, nil
		}
		value, _ := os.LookupEnv(name)
		return &types.ToolResult{
			Success:  true,
			Output:   fmt.Sprintf("%s=%s", name, show(name, value)),
			Metadata: map[string]string{"name": name},
		}, nil
	case "list":
		prefix, _ := args["prefix"].(string)
		var lines []string
		for _, kv := range os.Environ() {
			name := kv
			value := ""
			if i := strings.IndexByte(kv, '='); i >= 0 {
				name = kv[:i]
				value = kv[i+1:]
			}
			if prefix != "" && !strings.HasPrefix(name, prefix) {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s=%s", name, show(name, value)))
		}
		sort.Strings(lines)
		return &types.ToolResult{
			Success:  true,
			Output:   strings.Join(lines, "\n"),
			Metadata: map[string]string{"count": fmt.Sprintf("%d", len(lines))},
		}, nil
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}
