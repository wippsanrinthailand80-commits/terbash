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

const (
	maxPageBytes = 512 << 10 // 512KB fetch cap
	maxLinks     = 100
)

// BrowserTool renders a web page as readable text plus its links.
// No JavaScript, no cookies, no login: if a page needs those, the tool
// says so instead of returning junk.
type BrowserTool struct{}

func NewBrowserTool() *BrowserTool { return &BrowserTool{} }

func (t *BrowserTool) Name() string { return "browser" }

func (t *BrowserTool) Description() string {
	return "Open a web page as readable text with numbered links (no JavaScript)"
}

func (t *BrowserTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"url":       {Type: "string", Description: "http(s) page URL to open"},
			"max_chars": {Type: "integer", Description: "Max text chars to return (default 6000, max 20000)"},
		},
		Required: []string{"url"},
	}
}

type pageLink struct {
	text string
	href string
}

func (t *BrowserTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	rawURL, _ := args["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return &types.ToolResult{Success: false, Error: "url is required"}, nil
	}
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return &types.ToolResult{Success: false, Error: "only http(s) URLs are allowed"}, nil
	}

	maxChars := 6000
	if n, ok := args["max_chars"].(float64); ok && n > 0 {
		maxChars = int(n)
		if maxChars > 20000 {
			maxChars = 20000
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", "terbash/1.0 (text browser)")
	resp, err := client.Do(req)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("HTTP %d opening %s", resp.StatusCode, rawURL)}, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "html") && !strings.Contains(strings.ToLower(ct), "text") {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("not a readable page (Content-Type: %s)", ct)}, nil
	}

	title, text, links := extractReadable(rawURL, data)
	var b strings.Builder
	if title != "" {
		fmt.Fprintf(&b, "# %s\n\n", title)
	}
	runes := []rune(text)
	if len(runes) > maxChars {
		b.WriteString(string(runes[:maxChars]))
		b.WriteString(fmt.Sprintf("\n… text truncated at %d chars", maxChars))
	} else {
		b.WriteString(text)
	}
	if len(links) > 0 {
		b.WriteString("\n\nLinks:\n")
		for i, l := range links {
			label := l.text
			if label == "" {
				label = l.href
			}
			fmt.Fprintf(&b, "[%d] %s <%s>\n", i+1, label, l.href)
		}
	}
	return &types.ToolResult{
		Success:  true,
		Output:   strings.TrimSpace(b.String()),
		Metadata: map[string]string{"url": rawURL, "links": fmt.Sprintf("%d", len(links))},
	}, nil
}

// extractReadable pulls the title, visible text and links out of HTML.
func extractReadable(base string, data []byte) (string, string, []pageLink) {
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return "", "", nil
	}
	baseURL, _ := url.Parse(base)
	var title string
	var text strings.Builder
	var links []pageLink

	var linkLabel func(n *html.Node) string
	linkLabel = func(n *html.Node) string {
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

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "template", "svg":
				return
			case "head":
				// Title only - skip the rest of <head>.
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "title" && c.FirstChild != nil {
						title = strings.TrimSpace(c.FirstChild.Data)
					}
				}
				return
			case "a":
				href := ""
				for _, a := range n.Attr {
					if a.Key == "href" {
						href = strings.TrimSpace(a.Val)
						break
					}
				}
				if href != "" && !strings.HasPrefix(strings.ToLower(href), "javascript:") && href != "#" {
					if u, err := baseURL.Parse(href); err == nil {
						u.Fragment = ""
						href = u.String()
					}
					if len(links) < maxLinks {
						links = append(links, pageLink{text: linkLabel(n), href: href})
					}
				}
			}
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				text.WriteString(t + "\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Collapse blank lines for a compact readable dump.
	var lines []string
	for _, line := range strings.Split(text.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return title, strings.Join(lines, "\n"), links
}
