package tui

import "testing"

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
