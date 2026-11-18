package main

// bodycap_test.go — a forge answer over maxResponseBytes is refused, never read as its first
// 4 MiB. The served body is valid JSON padded with trailing spaces, so its first
// maxResponseBytes bytes decode as a complete, empty answer: a truncating read passes it on.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6755
func TestSendRefusesOverCapBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		size   int
		refuse bool
	}{
		{"over the cap", maxResponseBytes + 1, true},
		{"at the cap", maxResponseBytes, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]byte("[]"), bytes.Repeat([]byte(" "), tc.size-2)...)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(body)
			}))
			defer srv.Close()
			c := newGitHubClient(srv.URL, srv.Client(), "example-credential")
			r, err := c.do("GET", "/orgs/example/teams", nil)
			if !tc.refuse {
				var v []any
				if err != nil || json.Unmarshal(r.Body, &v) != nil {
					t.Fatalf("a body exactly at the cap was not read whole: err=%v len=%d", err, len(r.Body))
				}
				return
			}
			if err == nil {
				t.Fatalf("a %d-byte answer read as complete (%d bytes handed on); want a refusal", tc.size, len(r.Body))
			}
			if !deskkit.IsForgeBodyOverCap(err) || !deskkit.IsUnverifiable(err) {
				t.Errorf("the refusal is not an over-cap could-not-check: %v", err)
			}
			if r.Body != nil {
				t.Errorf("a refused answer still handed on %d bytes", len(r.Body))
			}
			if strings.Contains(err.Error(), "example-credential") {
				t.Errorf("the refusal carries the credential: %v", err)
			}
		})
	}
}
