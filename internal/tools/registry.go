package tools

import (
	"fmt"

	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
)

type Registry struct {
	tools map[string]types.Tool
}

func NewRegistry(cfg *config.Config, cwd string) *Registry {
	r := &Registry{tools: make(map[string]types.Tool)}
	r.registerBuiltins(cfg, cwd)
	return r
}

func (r *Registry) registerBuiltins(cfg *config.Config, cwd string) {
	r.tools["file_operations"] = NewFileTool(cfg, cwd)
	r.tools["shell_exec"] = NewShellTool(cfg, cwd)
	r.tools["godot_headless"] = NewGodotTool(cfg, cwd)
	r.tools["termux_api"] = NewTermuxTool()
}

func (r *Registry) Register(name string, tool types.Tool) {
	r.tools[name] = tool
}

func (r *Registry) Get(name string) (types.Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []types.Tool {
	tools := make([]types.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *Registry) GetSchemas() []types.Tool {
	return r.List()
}

func (r *Registry) Execute(name string, args map[string]interface{}) (*types.ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("tool not found: %s", name)}, nil
	}
	return tool.Execute(args)
}