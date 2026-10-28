package serve_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/serve"
)

func originProbe(g serve.OriginGuard, json bool, method, host, ct string, hdr map[string]string) int {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := g.Wrap(next)
	if json {
		h = g.WrapJSON(next)
	}
	req := httptest.NewRequest(method, "/x", strings.NewReader("{}"))
	req.Host = host
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestOriginGuardDecisions pins each refusal the request-origin floor makes,
// and the requests it must let through (status 418 = reached the handler).
func TestOriginGuardDecisions(t *testing.T) {
	g := serve.OriginGuard{Hosts: []string{"desk.example.org"}}
	cases := []struct {
		name   string
		json   bool
		method string
		host   string
		ct     string
		hdr    map[string]string
		want   int
	}{
		{"cross-site fetch metadata", false, "POST", "127.0.0.1:8787", "text/plain", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"same-site is not same-origin", false, "POST", "localhost:8787", "text/plain", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"foreign Origin, no fetch metadata", false, "POST", "127.0.0.1:8787", "text/plain", map[string]string{"Origin": "http://localhost:9999"}, 403},
		{"null Origin", false, "POST", "127.0.0.1:8787", "text/plain", map[string]string{"Origin": "null"}, 403},
		{"rebound host", false, "GET", "rebound.example:8787", "", map[string]string{"Sec-Fetch-Site": "same-origin"}, 403},
		{"json route, form body", true, "POST", "127.0.0.1:8787", "application/x-www-form-urlencoded", nil, 415},
		{"json route, no type", true, "POST", "127.0.0.1:8787", "", nil, 415},
		{"same-origin browser write", false, "POST", "127.0.0.1:8787", "application/x-www-form-urlencoded", map[string]string{"Origin": "http://127.0.0.1:8787", "Sec-Fetch-Site": "same-origin"}, 418},
		{"configured public host", true, "POST", "desk.example.org", "application/json", map[string]string{"Origin": "https://desk.example.org", "Sec-Fetch-Site": "same-origin"}, 418},
		{"ipv6 loopback", false, "POST", "[::1]:8787", "text/plain", map[string]string{"Origin": "http://[::1]:8787"}, 418},
		{"non-browser client", true, "POST", "deskd.svc:8787", "application/json; charset=utf-8", nil, 418},
		{"cross-site GET navigation", false, "GET", "127.0.0.1:8787", "", map[string]string{"Sec-Fetch-Site": "cross-site"}, 418},
	}
	for _, c := range cases {
		if got := originProbe(g, c.json, c.method, c.host, c.ct, c.hdr); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestServedHosts(t *testing.T) {
	got := serve.ServedHosts("0.0.0.0:8787", "https://Desk.Example.org/auth/callback")
	if len(got) != 1 || got[0] != "desk.example.org" {
		t.Fatalf("ServedHosts(wildcard bind, public redirect) = %v", got)
	}
	if got := serve.ServedHosts("127.0.0.1:8787", "http://127.0.0.1:8787/auth/callback"); len(got) != 0 {
		t.Fatalf("a loopback deployment added hosts: %v", got)
	}
}
