package tools

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/terbash/terbash/pkg/types"
)

type todoItem struct {
	id    int
	text  string
	done  bool
}

// TodoTool is an in-memory session task list (add/list/complete/remove).
// Pure bookkeeping - never touches disk, never needs confirmation.
type TodoTool struct {
	mu     sync.Mutex
	items  map[int]*todoItem
	nextID int
}

func NewTodoTool() *TodoTool {
	return &TodoTool{items: make(map[int]*todoItem), nextID: 1}
}

func (t *TodoTool) Name() string { return "todo_write" }

func (t *TodoTool) Description() string {
	return "Session task list: add, list, complete, remove tasks"
}

func (t *TodoTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Todo operation",
				Enum:        []string{"add", "list", "complete", "remove", "clear"},
			},
			"text": {Type: "string", Description: "Task text (for add)"},
			"id":   {Type: "integer", Description: "Task id (for complete/remove)"},
		},
		Required: []string{"operation"},
	}
}

func (t *TodoTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)

	t.mu.Lock()
	defer t.mu.Unlock()

	switch op {
	case "add":
		text, _ := args["text"].(string)
		if strings.TrimSpace(text) == "" {
			return &types.ToolResult{Success: false, Error: "text is required for add"}, nil
		}
		id := t.nextID
		t.nextID++
		t.items[id] = &todoItem{id: id, text: strings.TrimSpace(text)}
		return &types.ToolResult{Success: true, Output: fmt.Sprintf("Added #%d: %s", id, text), Metadata: map[string]string{"id": fmt.Sprintf("%d", id)}}, nil
	case "list":
		return &types.ToolResult{Success: true, Output: t.render()}, nil
	case "complete", "remove":
		id := int(floatArg(args, "id"))
		item, ok := t.items[id]
		if !ok || id == 0 {
			return &types.ToolResult{Success: false, Error: fmt.Sprintf("no task #%d", id)}, nil
		}
		if op == "complete" {
			item.done = true
			return &types.ToolResult{Success: true, Output: fmt.Sprintf("Completed #%d: %s", id, item.text)}, nil
		}
		delete(t.items, id)
		return &types.ToolResult{Success: true, Output: fmt.Sprintf("Removed #%d: %s", id, item.text)}, nil
	case "clear":
		t.items = make(map[int]*todoItem)
		return &types.ToolResult{Success: true, Output: "Task list cleared."}, nil
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}

func (t *TodoTool) render() string {
	if len(t.items) == 0 {
		return "No tasks."
	}
	ids := make([]int, 0, len(t.items))
	for id := range t.items {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var b strings.Builder
	for _, id := range ids {
		item := t.items[id]
		box := "[ ]"
		if item.done {
			box = "[x]"
		}
		fmt.Fprintf(&b, "%s #%d %s\n", box, id, item.text)
	}
	return strings.TrimRight(b.String(), "\n")
}

func floatArg(args map[string]interface{}, key string) float64 {
	if n, ok := args[key].(float64); ok {
		return n
	}
	return 0
}
