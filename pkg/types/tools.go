package types

type ToolResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(args map[string]interface{}) (*ToolResult, error)
}

type ToolExecutor interface {
	ExecuteTool(name string, args map[string]interface{}) (*ToolResult, error)
	ListTools() []Tool
	GetTool(name string) Tool
}