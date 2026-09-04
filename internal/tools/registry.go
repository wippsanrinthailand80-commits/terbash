package tools

import (
	"fmt"

	"github.com/terbash/terbash/pkg/types"
)

type Registry struct {
	tools map[string]types.ToolInterface
}

func NewRegistry(cfg *types.Config, cwd string) *Registry {
	r := &Registry{tools: make(map[string]types.ToolInterface)}
	r.registerBuiltins(cfg, cwd)
	return r
}

func (r *Registry) registerBuiltins(cfg *types.Config, cwd string) {
	r.tools["file_operations"] = NewFileTool(cfg, cwd)
	r.tools["shell_exec"] = NewShellTool(cfg, cwd)
	r.tools["godot_headless"] = NewGodotTool(cfg, cwd)
	r.tools["termux_api"] = NewTermuxTool()
	r.tools["grep_search"] = NewSearchTool(cfg, cwd)
	r.tools["glob_files"] = NewGlobTool(cfg, cwd)
	r.tools["http_fetch"] = NewHTTPTool(cfg)
	r.tools["git_ops"] = NewGitTool(cfg, cwd)
	r.tools["todo_write"] = NewTodoTool()
	r.tools["process"] = NewProcessTool(cfg)
	r.tools["env_vars"] = NewEnvTool()
	r.tools["memory"] = NewMemoryTool(cfg)
	r.tools["browser"] = NewBrowserTool()
	r.tools["web_search"] = NewWebSearchTool()
}

func (r *Registry) Register(name string, tool types.ToolInterface) {
	r.tools[name] = tool
}

func (r *Registry) Get(name string) (types.ToolInterface, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []types.ToolInterface {
	tools := make([]types.ToolInterface, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *Registry) GetSchemas() []types.ToolInterface {
	return r.List()
}

func (r *Registry) Execute(name string, args map[string]interface{}) (*types.ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("tool not found: %s", name)}, nil
	}
	return tool.Execute(args)
}