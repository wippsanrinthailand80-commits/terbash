package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewerAvailable(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.12", "v0.1.13", true},
		{"v0.1.12", "v0.1.12", false},
		{"v0.1.13", "v0.1.12", false},
		{"v0.1.9", "v0.1.12", true},
		{"0.1.12", "v0.1.13", true},
		{"dev", "v0.1.13", false},     // dev builds never nag
		{"unknown", "v0.1.13", false}, //
		{"", "v0.1.13", false},        //
		{"v0.1.12", "", false},
		{"v0.1.12", "garbage", false},
	}
	for _, c := range cases {
		if got := isNewerAvailable(c.current, c.latest); got != c.want {
			t.Errorf("isNewerAvailable(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestFetchLatestTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/latest" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()

	tag, err := fetchLatestTag(srv.URL, "o/r")
	if err != nil || tag != "v9.9.9" {
		t.Fatalf("tag=%q err=%v", tag, err)
	}
	if _, err := fetchLatestTag(srv.URL, "nope/nope"); err == nil {
		t.Fatal("expected error for 404")
	}
}
