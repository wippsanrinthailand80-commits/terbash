package tools

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/terbash/terbash/pkg/types"
)

const maxFetchBytes = 1 << 20 // 1MB response cap

// HTTPTool fetches URLs. Plain GET reads are free; anything that can
// change remote state (POST/PUT/PATCH/DELETE) needs user confirmation.
type HTTPTool struct {
	config *types.Config
	client *http.Client
}

func NewHTTPTool(cfg *types.Config) *HTTPTool {
	return &HTTPTool{config: cfg, client: &http.Client{Timeout: 60 * time.Second}}
}

func (t *HTTPTool) Name() string { return "http_fetch" }

func (t *HTTPTool) Description() string {
	return "Fetch a URL (GET free; POST/PUT/PATCH/DELETE need confirmation)"
}

func (t *HTTPTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"url":     {Type: "string", Description: "http(s) URL to fetch"},
			"method":  {Type: "string", Description: "HTTP method (default GET)", Enum: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}},
			"headers": {Type: "object", Description: "Extra request headers as key/value pairs"},
			"body":    {Type: "string", Description: "Request body (for POST/PUT/PATCH)"},
			"timeout": {Type: "integer", Description: "Timeout in seconds (default 30)"},
		},
		Required: []string{"url"},
	}
}

func (t *HTTPTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	rawURL, _ := args["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return &types.ToolResult{Success: false, Error: "url is required"}, nil
	}
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return &types.ToolResult{Success: false, Error: "only http(s) URLs are allowed"}, nil
	}

	method := "GET"
	if m, ok := args["method"].(string); ok && strings.TrimSpace(m) != "" {
		method = strings.ToUpper(strings.TrimSpace(m))
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unsupported method: %s", method)}, nil
	}
	if method != "GET" && method != "HEAD" {
		if !confirmAction(t.config.Tools.ConfirmCommands, fmt.Sprintf("%s %s?", method, rawURL)) {
			return &types.ToolResult{Success: false, Error: "request cancelled by user"}, nil
		}
	}

	timeout := 30
	if n, ok := args["timeout"].(float64); ok && n > 0 && n <= 300 {
		timeout = int(n)
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	var body io.Reader
	if b, ok := args["body"].(string); ok && b != "" {
		body = bytes.NewBufferString(b)
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", "terbash/1.0")
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	out := string(data)
	if len(data) > maxFetchBytes {
		out = string(data[:maxFetchBytes]) + fmt.Sprintf("\n… truncated at %d bytes", maxFetchBytes)
	}
	return &types.ToolResult{
		Success: resp.StatusCode < 400,
		Output:  out,
		Metadata: map[string]string{
			"status":       fmt.Sprintf("%d", resp.StatusCode),
			"content_type": resp.Header.Get("Content-Type"),
		},
	}, nil
}
