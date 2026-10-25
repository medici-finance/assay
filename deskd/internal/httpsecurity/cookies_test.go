package httpsecurity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookiePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, browserURL, target, remote string
		secure                           bool
	}{
		{"secure zero value", "", "http://localhost/", "127.0.0.1:5000", true},
		{"direct TLS", "https://console.example.invalid/auth/callback", "https://console.example.invalid/", "127.0.0.1:5000", true},
		{"TLS proxy", "https://console.example.invalid/auth/callback", "http://localhost/", "127.0.0.1:5000", true},
		{"public HTTP", "http://console.example.invalid/auth/callback", "http://console.example.invalid/", "127.0.0.1:5000", true},
		{"local IPv4 HTTP", "http://127.0.0.1:8787/auth/callback", "http://127.0.0.1:8787/", "127.0.0.1:5000", false},
		{"local IPv6 HTTP", "http://[::1]:8787/auth/callback", "http://[::1]:8787/", "[::1]:5000", false},
		{"localhost HTTP", "http://localhost:8787/auth/callback", "http://localhost:8787/", "127.0.0.1:5000", false},
		{"local TLS", "http://localhost/auth/callback", "https://localhost/", "127.0.0.1:5000", true},
		{"remote client spoofing local host", "http://localhost/auth/callback", "http://localhost/", "192.0.2.1:5000", true},
		{"public host on loopback connection", "http://localhost/auth/callback", "http://console.example.invalid/", "127.0.0.1:5000", true},
		{"localhost suffix", "http://localhost.example.invalid/auth/callback", "http://localhost/", "127.0.0.1:5000", true},
		{"URL credentials", "http://user@localhost/auth/callback", "http://localhost/", "127.0.0.1:5000", true},
		{"invalid URL", "://", "http://localhost/", "127.0.0.1:5000", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.target, nil)
			r.RemoteAddr = tc.remote
			// Untrusted forwarding headers never select the loopback exception.
			r.Header.Set("X-Forwarded-Proto", "http")
			r.Header.Set("X-Forwarded-Host", "localhost")
			c := CookiesForURL(tc.browserURL).New(r, "session", "opaque", http.SameSiteStrictMode)
			if c.Secure != tc.secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != "" {
				t.Fatalf("cookie attributes = %+v, want Secure=%t HttpOnly host-only Strict /", c, tc.secure)
			}
		})
	}
}
