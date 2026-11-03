// Package httpsecurity holds the browser-facing HTTP security policy.
package httpsecurity

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// CookiePolicy defaults to HTTPS-only cookies. Its zero value is secure.
type CookiePolicy struct{ allowLoopbackHTTP bool }

// CookiesForURL permits HTTP only for an explicitly configured loopback browser
// URL. A public HTTPS callback keeps Secure set even behind an HTTP reverse proxy.
func CookiesForURL(browserURL string) CookiePolicy {
	u, err := url.Parse(browserURL)
	return CookiePolicy{allowLoopbackHTTP: err == nil && u.Scheme == "http" && u.User == nil && loopbackHost(u.Host)}
}

// New creates a host-only session cookie. Only a direct loopback HTTP request in
// an explicitly local deployment may omit Secure; forwarded headers grant nothing.
func (p CookiePolicy) New(r *http.Request, name, value string, sameSite http.SameSite) *http.Cookie {
	c := &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: sameSite}
	if p.allowLoopbackHTTP && r.TLS == nil && r.URL.Scheme != "https" && loopbackHost(r.Host) && loopbackHost(r.RemoteAddr) {
		c.Secure = false
	}
	return c
}

func loopbackHost(authority string) bool {
	host := authority
	if h, _, err := net.SplitHostPort(authority); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && ip.IsLoopback()
}
