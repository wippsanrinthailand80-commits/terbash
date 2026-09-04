package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/terbash/terbash/internal/config"
	"github.com/terbash/terbash/pkg/types"
)

const maxMemoryBytes = 512 << 10 // refuse writes past 512KB, suggest delete

type memoryEntry struct {
	ID      int    `json:"id"`
	Key     string `json:"key,omitempty"`
	Text    string `json:"text"`
	Created string `json:"created"`
}

type memoryStore struct {
	NextID  int           `json:"next_id"`
	Entries []memoryEntry `json:"entries"`
}

// MemoryTool is a persistent cross-session notebook for the agent:
// save facts, recall them later, forget on request. Stored as JSON next
// to the config file - only clear (wipe) needs confirmation.
type MemoryTool struct {
	config *types.Config
}

func NewMemoryTool(cfg *types.Config) *MemoryTool {
	return &MemoryTool{config: cfg}
}

func (t *MemoryTool) Name() string { return "memory" }

func (t *MemoryTool) Description() string {
	return "Persistent agent notebook: save, list, search, recall and forget notes"
}

func (t *MemoryTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Memory operation",
				Enum:        []string{"save", "list", "search", "get", "delete", "clear"},
			},
			"text":  {Type: "string", Description: "Note text (for save)"},
			"key":   {Type: "string", Description: "Stable label (save upserts by key; get/delete look it up)"},
			"query": {Type: "string", Description: "Substring to find (for search)"},
		},
		Required: []string{"operation"},
	}
}

func (t *MemoryTool) path() (string, error) {
	cfgPath, err := config.GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), "memory.json"), nil
}

func (t *MemoryTool) load() (string, *memoryStore, error) {
	path, err := t.path()
	if err != nil {
		return "", nil, err
	}
	store := &memoryStore{NextID: 1}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, store, nil
		}
		return "", nil, err
	}
	if len(data) == 0 {
		return path, store, nil
	}
	if err := json.Unmarshal(data, store); err != nil {
		return "", nil, fmt.Errorf("memory file is corrupt: %w (delete it or fix the JSON)", err)
	}
	if store.NextID < 1 {
		store.NextID = 1
	}
	return path, store, nil
}

func (t *MemoryTool) save(path string, store *memoryStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (t *MemoryTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	path, store, err := t.load()
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	switch op {
	case "save":
		text, _ := args["text"].(string)
		if strings.TrimSpace(text) == "" {
			return &types.ToolResult{Success: false, Error: "text is required for save"}, nil
		}
		if info, err := os.Stat(path); err == nil && info.Size() > maxMemoryBytes {
			return &types.ToolResult{Success: false, Error: "memory is full - delete old notes first"}, nil
		}
		key, _ := args["key"].(string)
		key = strings.TrimSpace(key)
		for i, e := range store.Entries {
			if key != "" && e.Key == key {
				store.Entries[i].Text = strings.TrimSpace(text)
				store.Entries[i].Created = time.Now().UTC().Format(time.RFC3339)
				if err := t.save(path, store); err != nil {
					return &types.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &types.ToolResult{Success: true, Output: fmt.Sprintf("Updated #%d (%s)", e.ID, key)}, nil
			}
		}
		entry := memoryEntry{ID: store.NextID, Key: key, Text: strings.TrimSpace(text), Created: time.Now().UTC().Format(time.RFC3339)}
		store.NextID++
		store.Entries = append(store.Entries, entry)
		if err := t.save(path, store); err != nil {
			return &types.ToolResult{Success: false, Error: err.Error()}, nil
		}
		return &types.ToolResult{Success: true, Output: fmt.Sprintf("Saved #%d", entry.ID), Metadata: map[string]string{"id": fmt.Sprintf("%d", entry.ID)}}, nil
	case "list", "search":
		query, _ := args["query"].(string)
		var lines []string
		for _, e := range store.Entries {
			if op == "search" && !strings.Contains(strings.ToLower(e.Key+" "+e.Text), strings.ToLower(query)) {
				continue
			}
			label := fmt.Sprintf("#%d", e.ID)
			if e.Key != "" {
				label += fmt.Sprintf(" (%s)", e.Key)
			}
			lines = append(lines, fmt.Sprintf("%s %s", label, e.Text))
		}
		sort.Strings(lines)
		if len(lines) == 0 {
			return &types.ToolResult{Success: true, Output: "No notes."}, nil
		}
		return &types.ToolResult{Success: true, Output: strings.Join(lines, "\n"), Metadata: map[string]string{"count": fmt.Sprintf("%d", len(lines))}}, nil
	case "get", "delete":
		key, _ := args["key"].(string)
		if strings.TrimSpace(key) == "" {
			return &types.ToolResult{Success: false, Error: "key is required"}, nil
		}
		for i, e := range store.Entries {
			if e.Key != key {
				continue
			}
			if op == "get" {
				return &types.ToolResult{Success: true, Output: fmt.Sprintf("#%d (%s) %s", e.ID, e.Key, e.Text)}, nil
			}
			store.Entries = append(store.Entries[:i], store.Entries[i+1:]...)
			if err := t.save(path, store); err != nil {
				return &types.ToolResult{Success: false, Error: err.Error()}, nil
			}
			return &types.ToolResult{Success: true, Output: fmt.Sprintf("Forgot #%d (%s)", e.ID, e.Key)}, nil
		}
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("no note with key %q", key)}, nil
	case "clear":
		if !confirmAction(t.config.Tools.ConfirmWrites, fmt.Sprintf("Forget all %d notes?", len(store.Entries))) {
			return &types.ToolResult{Success: false, Error: "clear cancelled by user"}, nil
		}
		if err := t.save(path, &memoryStore{NextID: 1}); err != nil {
			return &types.ToolResult{Success: false, Error: err.Error()}, nil
		}
		return &types.ToolResult{Success: true, Output: "All notes forgotten."}, nil
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}
}
