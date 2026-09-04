package tui

import (
	"testing"

	"github.com/terbash/terbash/pkg/types"
)

func TestMergeToolFragSingleCall(t *testing.T) {
	var frags []*toolFrag
	// OpenAI-style: first chunk has id+name, rest only argument fragments.
	mergeToolFrag(&frags, types.ToolCall{ID: "call_1", Type: "function", Function: types.FunctionCall{Name: "grep_search"}})
	mergeToolFrag(&frags, types.ToolCall{ID: "call_1", Type: "function", Function: types.FunctionCall{Arguments: `{"query":`}})
	mergeToolFrag(&frags, types.ToolCall{Type: "function", Function: types.FunctionCall{Arguments: `"hi"}`}})

	calls := fragsToCalls(frags)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "grep_search" {
		t.Fatalf("name: %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"query":"hi"}` {
		t.Fatalf("args: %q", calls[0].Function.Arguments)
	}
}

func TestMergeToolFragTwoCalls(t *testing.T) {
	var frags []*toolFrag
	mergeToolFrag(&frags, types.ToolCall{ID: "a", Type: "function", Function: types.FunctionCall{Name: "todo_write", Arguments: "{}"}})
	mergeToolFrag(&frags, types.ToolCall{ID: "b", Type: "function", Function: types.FunctionCall{Name: "process", Arguments: "{}"}})

	calls := fragsToCalls(frags)
	if len(calls) != 2 || calls[0].ID != "a" || calls[1].ID != "b" {
		t.Fatalf("calls: %+v", calls)
	}
}

func TestMergeToolFragSkipsEmpty(t *testing.T) {
	var frags []*toolFrag
	mergeToolFrag(&frags, types.ToolCall{Type: "function"})
	if calls := fragsToCalls(frags); len(calls) != 0 {
		t.Fatalf("empty frag must be skipped: %+v", calls)
	}
}
