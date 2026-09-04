package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/terbash/terbash/pkg/types"
	"golang.org/x/net/html"
)

type webHit struct {
	title   string
	url     string
	snippet string
}

// WebSearchTool runs web searches via DuckDuckGo's no-key HTML endpoint.
// DDG rate-limits bots, so blocks surface as a clear retry-later error
// instead of junk. No API key or account needed.
type WebSearchTool struct {
	baseURL string
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{baseURL: "https://html.duckduckgo.com"}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Web search via DuckDuckGo, no API key needed (may rate-limit)"
}

func (t *WebSearchTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"query":       {Type: "string", Description: "Search query (site:example.com works)"},
			"max_results": {Type: "integer", Description: "Max results (default 8, max 20)"},
		},
		Required: []string{"query"},
	}
}

func (t *WebSearchTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return &types.ToolResult{Success: false, Error: "query is required"}, nil
	}
	maxResults := 8
	if n, ok := args["max_results"].(float64); ok && n > 0 {
		maxResults = int(n)
		if maxResults > 20 {
			maxResults = 20
		}
	}

	endpoint := strings.TrimSuffix(t.baseURL, "/") + "/html/?q=" + url.QueryEscape(query)
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; Termux) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("search temporarily blocked (HTTP %d) - try again later", resp.StatusCode)}, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	hits := parseDDGResults(string(data), maxResults)
	if len(hits) == 0 {
		return &types.ToolResult{Success: true, Output: "No results (query may be blocked - try again later or rephrase)."}, nil
	}
	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, h.title, h.url)
		if h.snippet != "" {
			fmt.Fprintf(&b, "   %s\n", h.snippet)
		}
	}
	return &types.ToolResult{
		Success:  true,
		Output:   strings.TrimSpace(b.String()),
		Metadata: map[string]string{"results": fmt.Sprintf("%d", len(hits))},
	}, nil
}

// parseDDGResults extracts result links + snippets from DDG html output.
// Result hrefs are redirect links (/l/?uddg=<actual-url>&...) - unwrap them.
func parseDDGResults(page string, max int) []webHit {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	var hits []webHit
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if len(hits) >= max {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			href := attr(n, "href")
			if target := unwrapDDG(href); target != "" {
				hits = append(hits, webHit{
					title:   nodeText(n),
					url:     target,
					snippet: findSnippet(n),
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if len(hits) >= max {
				return
			}
			walk(c)
		}
	}
	walk(doc)
	return hits
}

// unwrapDDG turns //duckduckgo.com/l/?uddg=<url>&... into the real URL,
// passing through plain http(s) links untouched.
func unwrapDDG(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if !strings.Contains(href, "uddg=") {
		return ""
	}
	if !strings.HasPrefix(href, "http") && !strings.HasPrefix(href, "//") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	target := u.Query().Get("uddg")
	if target == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(target); err == nil {
		target = decoded
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return ""
	}
	return target
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var collect func(x *html.Node)
	collect = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// findSnippet looks for the result snippet near a result link: DDG puts
// <a class="result__snippet"> right after the title link's parent block.
func findSnippet(link *html.Node) string {
	for p := link.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && (p.Data == "div" || p.Data == "td") {
			var best string
			var scan func(x *html.Node)
			scan = func(x *html.Node) {
				if best != "" {
					return
				}
				if x.Type == html.ElementNode && x.Data == "a" && hasClass(x, "result__snippet") {
					best = nodeText(x)
					return
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					scan(c)
				}
			}
			scan(p)
			if best != "" {
				return best
			}
		}
		if p.Type == html.ElementNode && p.Data == "body" {
			break
		}
	}
	return ""
}
