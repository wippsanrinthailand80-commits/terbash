package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terbash/terbash/pkg/types"
)

func testCfg() *types.Config {
	return &types.Config{
		Tools: types.ToolsConfig{
			ConfirmWrites:   false,
			ConfirmCommands: false,
			SandboxEnabled:  true,
			MaxFileSize:     10485760,
		},
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxRejectsTraversal(t *testing.T) {
	cwd := t.TempDir()
	for _, p := range []string{"../escape", "/abs/path", "..", "a/../../b"} {
		if _, err := resolveSandbox(cwd, p); err == nil {
			t.Fatalf("expected rejection for %q", p)
		}
	}
	if _, err := resolveSandbox(cwd, "sub/dir/file.txt"); err != nil {
		t.Fatalf("valid path rejected: %v", err)
	}
}

func TestSearchTool(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "a.go"), "package main\n// hello world\n")
	writeFile(t, filepath.Join(cwd, "sub", "b.txt"), "say HELLO again\n")
	writeFile(t, filepath.Join(cwd, ".git", "ignored.go"), "hello hidden\n")

	tool := NewSearchTool(testCfg(), cwd)
	res, err := tool.Execute(map[string]interface{}{"query": "hello"})
	if err != nil || !res.Success {
		t.Fatalf("search: %v %v", err, res)
	}
	if !strings.Contains(res.Output, "a.go:2") {
		t.Fatalf("expected a.go hit, got:\n%s", res.Output)
	}
	if strings.Contains(res.Output, "ignored") {
		t.Fatalf(".git should be skipped, got:\n%s", res.Output)
	}

	res, _ = tool.Execute(map[string]interface{}{"query": "hello", "ignore_case": true})
	if !strings.Contains(res.Output, "b.txt") {
		t.Fatalf("expected case-insensitive hit, got:\n%s", res.Output)
	}

	res, _ = tool.Execute(map[string]interface{}{"query": "h.llo", "regex": true})
	if !strings.Contains(res.Output, "a.go") {
		t.Fatalf("expected regex hit, got:\n%s", res.Output)
	}

	res, _ = tool.Execute(map[string]interface{}{"query": "[bad"})
	if !res.Success {
		t.Fatal("literal search must not fail on regex metachars")
	}
	res, _ = tool.Execute(map[string]interface{}{"query": "[bad", "regex": true})
	if res.Success {
		t.Fatal("expected bad-regexp failure")
	}
}

func TestGlobTool(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "cmd", "a", "main.go"), "x")
	writeFile(t, filepath.Join(cwd, "cmd", "b", "main.go"), "x")
	writeFile(t, filepath.Join(cwd, "README.md"), "x")

	tool := NewGlobTool(testCfg(), cwd)
	res, err := tool.Execute(map[string]interface{}{"pattern": "**/main.go"})
	if err != nil || !res.Success {
		t.Fatalf("glob: %v %v", err, res)
	}
	if strings.Count(res.Output, "main.go") != 2 {
		t.Fatalf("expected 2 hits, got:\n%s", res.Output)
	}

	res, _ = tool.Execute(map[string]interface{}{"pattern": "*.md"})
	if !strings.Contains(res.Output, "README.md") {
		t.Fatalf("expected README hit, got:\n%s", res.Output)
	}
}

func TestHTTPTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/echo" {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("method=" + r.Method))
			return
		}
		w.Write([]byte("hello-fetch"))
	}))
	defer srv.Close()

	tool := NewHTTPTool(testCfg())
	res, err := tool.Execute(map[string]interface{}{"url": srv.URL})
	if err != nil || !res.Success || res.Output != "hello-fetch" {
		t.Fatalf("GET: %v %v", err, res)
	}
	if res.Metadata["status"] != "200" {
		t.Fatalf("status meta: %v", res.Metadata)
	}

	res, _ = tool.Execute(map[string]interface{}{"url": "file:///etc/passwd"})
	if res.Success {
		t.Fatal("file:// must be rejected")
	}

	// Non-GET with confirmations disabled still runs (confirmAction passthrough).
	res, err = tool.Execute(map[string]interface{}{"url": srv.URL + "/echo", "method": "POST", "body": "x"})
	if err != nil || !res.Success || res.Output != "method=POST" {
		t.Fatalf("POST: %v %v", err, res)
	}
}

func TestTodoTool(t *testing.T) {
	tool := NewTodoTool()
	res, _ := tool.Execute(map[string]interface{}{"operation": "add", "text": "first task"})
	if !res.Success || res.Metadata["id"] != "1" {
		t.Fatalf("add: %v", res)
	}
	tool.Execute(map[string]interface{}{"operation": "add", "text": "second"})
	res, _ = tool.Execute(map[string]interface{}{"operation": "complete", "id": float64(1)})
	if !res.Success {
		t.Fatalf("complete: %v", res)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "list"})
	if !strings.Contains(res.Output, "[x] #1") || !strings.Contains(res.Output, "[ ] #2") {
		t.Fatalf("list render:\n%s", res.Output)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "complete", "id": float64(99)})
	if res.Success {
		t.Fatal("completing missing id should fail")
	}
}

func TestEnvToolRedaction(t *testing.T) {
	t.Setenv("TERBASH_TEST_API_KEY", "super-secret")
	t.Setenv("TERBASH_TEST_PLAIN", "visible")

	tool := NewEnvTool()
	res, _ := tool.Execute(map[string]interface{}{"operation": "get", "name": "TERBASH_TEST_API_KEY"})
	if !strings.Contains(res.Output, "redacted") || strings.Contains(res.Output, "super-secret") {
		t.Fatalf("secret must be redacted: %s", res.Output)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "get", "name": "TERBASH_TEST_API_KEY", "reveal": true})
	if !strings.Contains(res.Output, "super-secret") {
		t.Fatalf("reveal=true must show value: %s", res.Output)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "get", "name": "TERBASH_TEST_PLAIN"})
	if !strings.Contains(res.Output, "visible") {
		t.Fatalf("plain value must show: %s", res.Output)
	}
}

func TestProcessList(t *testing.T) {
	tool := NewProcessTool(testCfg())
	res, err := tool.Execute(map[string]interface{}{"operation": "list", "filter": "definitely-not-a-real-process-name"})
	if err != nil || !res.Success {
		t.Fatalf("list: %v %v", err, res)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "kill", "pid": float64(1)})
	if res.Success {
		t.Fatal("pid 1 must be rejected")
	}
}

func TestGitOps(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cwd := t.TempDir()
	tool := NewGitTool(testCfg(), cwd)
	run := func(op string) *types.ToolResult {
		res, err := tool.Execute(map[string]interface{}{"operation": op})
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		return res
	}
	// Outside a repo: commands fail gracefully, never crash.
	if res := run("status"); res.Success {
		t.Log("unexpectedly inside a repo - continuing anyway")
	}
}

func TestToolNeedsConfirm(t *testing.T) {
	cfg := testCfg()
	reg := NewRegistry(cfg, t.TempDir())

	needs := func(name, argsJSON string) bool {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			t.Fatal(err)
		}
		tool, ok := reg.Get(name)
		if !ok {
			t.Fatalf("tool %s not registered", name)
		}
		checker, ok := tool.(interface {
			RequiresConfirm(map[string]interface{}) (bool, string)
		})
		if !ok {
			return false
		}
		need, prompt := checker.RequiresConfirm(args)
		if need && prompt == "" {
			t.Fatalf("%s: empty prompt", name)
		}
		return need
	}

	cases := []struct {
		tool string
		args string
		want bool
	}{
		{"file_operations", `{"operation":"read","path":"x"}`, false},
		{"file_operations", `{"operation":"write","path":"x"}`, true},
		{"file_operations", `{"operation":"delete","path":"x"}`, true},
		{"shell_exec", `{"command":"ls"}`, true},
		{"git_ops", `{"operation":"status"}`, false},
		{"git_ops", `{"operation":"commit","message":"m"}`, true},
		{"http_fetch", `{"url":"https://x"}`, false},
		{"http_fetch", `{"url":"https://x","method":"POST"}`, true},
		{"process", `{"operation":"list"}`, false},
		{"process", `{"operation":"kill","pid":123}`, true},
		{"godot_headless", `{"operation":"export"}`, true},
		{"termux_api", `{"operation":"battery"}`, false},
		{"termux_api", `{"operation":"notification"}`, true},
		{"grep_search", `{"query":"x"}`, false},
		{"todo_write", `{"operation":"add"}`, false},
		{"env_vars", `{"operation":"get"}`, false},
	}
	for _, c := range cases {
		if got := needs(c.tool, c.args); got != c.want {
			t.Errorf("%s %s: need=%v want %v", c.tool, c.args, got, c.want)
		}
	}
}

func TestMemoryTool(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tool := NewMemoryTool(testCfg())
	res, _ := tool.Execute(map[string]interface{}{"operation": "save", "text": "user likes tea", "key": "prefs"})
	if !res.Success {
		t.Fatalf("save: %v", res)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "save", "text": "user likes coffee", "key": "prefs"})
	if !res.Success || !strings.Contains(res.Output, "Updated") {
		t.Fatalf("upsert: %v", res)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "search", "query": "coffee"})
	if !res.Success || !strings.Contains(res.Output, "coffee") || strings.Contains(res.Output, "tea") {
		t.Fatalf("search:\n%s", res.Output)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "get", "key": "prefs"})
	if !strings.Contains(res.Output, "coffee") {
		t.Fatalf("get:\n%s", res.Output)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "delete", "key": "prefs"})
	if !res.Success {
		t.Fatalf("delete: %v", res)
	}
	res, _ = tool.Execute(map[string]interface{}{"operation": "list"})
	if res.Output != "No notes." {
		t.Fatalf("expected empty, got:\n%s", res.Output)
	}
}

func TestBrowserExtract(t *testing.T) {
	page := `<html><head><title>Hi</title><script>var x=1</script></head><body><h1>Hello <b>world</b></h1><a href="/next">Next page</a><a href="javascript:void(0)">bad</a></body></html>`
	title, text, links := extractReadable("https://example.com/a", []byte(page))
	if title != "Hi" {
		t.Fatalf("title: %q", title)
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "world") || strings.Contains(text, "var x") {
		t.Fatalf("text: %q", text)
	}
	if len(links) != 1 || links[0].href != "https://example.com/next" || links[0].text != "Next page" {
		t.Fatalf("links: %+v", links)
	}
}

func TestWebSearchParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<div class="result"><h2><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgo&amp;rut=x">Go language</a></h2>
<a class="result__snippet" href="x">A programming language by Google</a></div>
<div class="result"><h2><a class="result__a" href="https://plain.example.com/y">Plain link</a></h2></div>
</body></html>`))
	}))
	defer srv.Close()

	tool := &WebSearchTool{baseURL: srv.URL}
	res, err := tool.Execute(map[string]interface{}{"query": "go", "max_results": float64(5)})
	if err != nil || !res.Success {
		t.Fatalf("search: %v %v", err, res)
	}
	if !strings.Contains(res.Output, "https://example.com/go") || !strings.Contains(res.Output, "A programming language") {
		t.Fatalf("output:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "https://plain.example.com/y") {
		t.Fatalf("plain link missing:\n%s", res.Output)
	}
}
