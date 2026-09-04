package types

type ToolResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ToolInterface interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(args map[string]interface{}) (*ToolResult, error)
}

type ToolExecutor interface {
	ExecuteTool(name string, args map[string]interface{}) (*ToolResult, error)
	ListTools() []ToolInterface
	GetTool(name string) ToolInterface
}

// ConfirmChecker is optionally implemented by tools so interactive UIs can
// ask for approval BEFORE executing (instead of the tool prompting on
// stdin mid-run, which fullscreen TUIs cannot do).
type ConfirmChecker interface {
	// RequiresConfirm reports whether these args need approval and the
	// prompt text to show. Pure: must not execute anything or prompt.
	RequiresConfirm(args map[string]interface{}) (bool, string)
}