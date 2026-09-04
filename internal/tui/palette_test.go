package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/terbash/terbash/pkg/types"
)

func TestWindowSelection(t *testing.T) {
	// Short list: everything visible.
	if s, e := windowSelection(5, 2, 8); s != 0 || e != 5 {
		t.Fatalf("short list: got [%d:%d]", s, e)
	}
	// Selection at top: window starts at 0.
	if s, e := windowSelection(13, 0, 8); s != 0 || e != 8 {
		t.Fatalf("top: got [%d:%d]", s, e)
	}
	// Selection in the hidden "more" zone must scroll into view.
	for _, sel := range []int{8, 10, 12} {
		s, e := windowSelection(13, sel, 8)
		if !(s <= sel && sel < e) {
			t.Fatalf("sel %d not visible in [%d:%d]", sel, s, e)
		}
		if e-s != 8 {
			t.Fatalf("sel %d: window size %d, want 8", sel, e-s)
		}
	}
	// Selection at the very bottom: window ends at n.
	if s, e := windowSelection(13, 12, 8); s != 5 || e != 13 {
		t.Fatalf("bottom: got [%d:%d]", s, e)
	}
	// Out-of-range selections clamp instead of producing empty windows.
	if s, e := windowSelection(13, 99, 8); s != 5 || e != 13 {
		t.Fatalf("clamp high: got [%d:%d]", s, e)
	}
	if s, e := windowSelection(13, -3, 8); s != 0 || e != 8 {
		t.Fatalf("clamp low: got [%d:%d]", s, e)
	}
}

func TestPaletteScrollRevealsHiddenCommands(t *testing.T) {
	app := NewApp(&types.Config{})
	app.input = "/"
	app.cursor = 1
	matches := paletteMatches("/")
	if len(matches) <= 8 {
		t.Fatalf("need >8 commands to test scrolling, got %d", len(matches))
	}
	// Park the highlight on the first hidden row, as if scrolled down.
	app.paletteIdx = 8
	view := app.View()
	want := matches[8].name
	if !strings.Contains(view, want) {
		t.Fatalf("scrolled-to command %q missing from view:\n%s", want, view)
	}
	if !strings.Contains(view, "›") {
		t.Fatal("no highlight marker rendered for scrolled selection")
	}
}

func TestProviderPickerScrollRevealsHidden(t *testing.T) {
	app := NewApp(&types.Config{})
	app.openProviderPicker()
	names := app.llmManager.AllProviderNames()
	if len(names) <= 8 {
		t.Fatalf("need >8 providers to test scrolling, got %d", len(names))
	}
	app.providerIdx = len(names) - 1 // scroll to the very bottom
	view := app.View()
	if !strings.Contains(view, names[len(names)-1]) {
		t.Fatalf("bottom provider %q missing from view", names[len(names)-1])
	}
	if !strings.Contains(view, "›") {
		t.Fatal("no highlight marker rendered for scrolled selection")
	}
}

func TestBottomBarPinsVersionRight(t *testing.T) {
	app := NewApp(&types.Config{})
	app.termWidth = 40
	line := app.bottomBar("hi")
	stripped := strings.ReplaceAll(line, "\x1b", "")
	_ = stripped
	// Styled output: check visual width fills the terminal and ends with version.
	if w := lipgloss.Width(line); w != 40 {
		t.Fatalf("bottom bar width %d, want 40: %q", w, line)
	}
	if !strings.HasSuffix(strings.TrimRight(line, " "), "terbash dev") {
		t.Fatalf("version not at right edge: %q", line)
	}
}

func TestBottomBarFallbackWithoutWidth(t *testing.T) {
	app := NewApp(&types.Config{})
	line := app.bottomBar("hi")
	if !strings.Contains(line, "hi • terbash dev") {
		t.Fatalf("fallback should inline version: %q", line)
	}
}

func TestFinishStreamRecordsMeta(t *testing.T) {
	app := NewApp(&types.Config{})
	app.streaming = true
	app.toolIter = 1
	app.msgStart = time.Now().Add(-1500 * time.Millisecond)
	app.streamBuf.WriteString("hello")
	app.fragTools = []*toolFrag{{id: "c1", name: "grep_search"}}
	app.fragTools[0].args.WriteString("{}")

	app.finishStream()

	if len(app.messages) != 1 {
		t.Fatalf("messages: %+v", app.messages)
	}
	m, ok := app.msgMeta[0]
	if !ok {
		t.Fatal("no meta recorded")
	}
	if m.seconds < 1.0 {
		t.Fatalf("seconds too small: %v", m.seconds)
	}
	if len(m.tools) != 1 || m.tools[0] != "grep_search" {
		t.Fatalf("tools: %v", m.tools)
	}
	if !app.streaming {
		t.Fatal("tool loop should continue (streaming must stay on)")
	}
	if len(app.pendingCalls) != 0 {
		t.Fatal("dispatched call should leave the queue")
	}
}

func TestFinishStreamPlainAnswerMeta(t *testing.T) {
	app := NewApp(&types.Config{})
	app.streaming = true
	app.toolIter = 1
	app.msgStart = time.Now()
	app.streamBuf.WriteString("hi")

	app.finishStream()

	m, ok := app.msgMeta[0]
	if !ok {
		t.Fatal("no meta recorded")
	}
	if got := formatReplyMeta(m); !strings.Contains(got, "answer") || !strings.Contains(got, "s") {
		t.Fatalf("meta line: %q", got)
	}
	if app.streaming {
		t.Fatal("plain answer should end the turn")
	}
	view := app.View()
	if !strings.Contains(view, "answer") {
		t.Fatalf("stats line missing from view:\n%s", view)
	}
}

func TestFormatSeconds(t *testing.T) {
	if got := formatSeconds(3.24); got != "3.2s" {
		t.Fatalf("got %q", got)
	}
	if got := formatSeconds(65); got != "1m5s" {
		t.Fatalf("got %q", got)
	}
	if got := formatReplyMeta(replyMeta{seconds: 2, tools: []string{"a", "b"}}); !strings.Contains(got, "a, b") {
		t.Fatalf("got %q", got)
	}
}
