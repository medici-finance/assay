package deskkit

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// regression: #2701, F-bundle-fetch-size
func TestCompositionHTTPBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fetch Fetcher
		limit int
	}{
		{"metadata", HTTPFetch, 1 << 20},
		{"cursor", HTTPFetchCursorBundle, CursorBundleMaxBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, extra := range []int{0, 1} {
				t.Run([]string{"at-limit", "over-limit"}[extra], func(t *testing.T) {
					payload := bytes.Repeat([]byte("a"), tc.limit+extra)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.(http.Flusher).Flush() // no Content-Length; bound the actual streamed bytes
						_, _ = w.Write(payload)
					}))
					defer server.Close()
					got, err := tc.fetch(server.URL)
					if extra == 0 {
						if err != nil || !bytes.Equal(got, payload) {
							t.Fatalf("at limit: len=%d err=%v", len(got), err)
						}
					} else if err == nil || !strings.Contains(err.Error(), "exceeds") || got != nil {
						t.Fatalf("over limit returned truncated success: len=%d err=%v", len(got), err)
					}
				})
			}
			server := httptest.NewServer(http.NotFoundHandler())
			defer server.Close()
			if _, err := tc.fetch(server.URL); !errors.Is(err, ErrNotFound) {
				t.Fatalf("404 classification lost: %v", err)
			}
		})
	}
}

// regression: #2701, F-bundle-fetch-size
func TestCursorFetchIsolation(t *testing.T) {
	src := CompositionSource{ReleasesDir: t.TempDir(), Announce: io.Discard, Fetch: func(string) ([]byte, error) {
		t.Fatal("metadata fetch must not opt into archive network access")
		return nil, nil
	}}
	if _, err := src.CursorBundle("v1.2.3"); err == nil || !strings.Contains(err.Error(), "explicitly allow --fetch") {
		t.Fatalf("archive without opt-in: %v", err)
	}
}
