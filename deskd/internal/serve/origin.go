package serve

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// OriginGuard is the request-origin floor in front of EVERY route deskd serves
// with data or a write behind it. The terminal actions carried this check on
// their own ([refuseCrossSite]); every other state-changing route — the agent
// surface, the legacy answer and session forms, the /v1 writes — did not, so a
// page in a browser on the operator's machine could make a "simple" cross-site
// POST (text/plain or form-encoded, no preflight) to a loopback deskd and have
// it act. The guard closes that class for all of them at once:
//
//   - Host allow-list (DNS rebinding). A request a BROWSER sent (it carries an
//     Origin or a Sec-Fetch-Site header) must address deskd by a loopback name
//     or a host in [OriginGuard.Hosts]. A rebound name is same-origin to the
//     browser, so the Origin check below cannot see it; the Host can.
//   - State-changing methods (anything but GET, HEAD, OPTIONS) refuse a
//     Sec-Fetch-Site other than same-origin/none, and an Origin that is not the
//     request's own Host.
//   - A JSON route ([OriginGuard.WrapJSON]) refuses a state-changing request
//     whose body is not declared application/json: that content type forces
//     a CORS preflight, which deskd never grants.
//
// A request with no browser header at all (a CLI, a loop, the contract client)
// is judged by its content type alone, exactly as the terminal actions were.
type OriginGuard struct {
	// Hosts are the host names (no port) a browser may address deskd by in
	// addition to the loopback names — the public host of a bearer-mode
	// deployment, taken from its OAuth redirect URL. Compared case-insensitively.
	Hosts []string
}

// Wrap guards a route whose state-changing requests may be form-encoded.
func (g OriginGuard) Wrap(next http.Handler) http.Handler { return g.wrap(next, false) }

// WrapJSON guards a route whose state-changing requests must be JSON.
func (g OriginGuard) WrapJSON(next http.Handler) http.Handler { return g.wrap(next, true) }

func (g OriginGuard) wrap(next http.Handler, needJSON bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status, msg := g.refusal(r, needJSON); status != 0 {
			writeJSON(w, status, map[string]string{"error": msg})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refusal returns the status and reason a request is refused with, or 0.
func (g OriginGuard) refusal(r *http.Request, needJSON bool) (int, string) {
	origin := r.Header.Get("Origin")
	site := r.Header.Get("Sec-Fetch-Site")
	if (origin != "" || site != "") && !g.hostAllowed(r.Host) {
		return http.StatusForbidden, "request refused: a browser addressed this service by a host name it does not serve (" +
			"only loopback names and the configured public host are accepted)"
	}
	if isSafeMethod(r.Method) {
		return 0, ""
	}
	if site != "" && site != "same-origin" && site != "none" {
		return http.StatusForbidden, "cross-origin write refused: Sec-Fetch-Site is " + site + "; a write is accepted only from this service's own origin"
	}
	if origin != "" && !isOwnOrigin(origin, r.Host) {
		return http.StatusForbidden, "cross-origin write refused: the request's Origin is not this service's own origin"
	}
	if needJSON {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			return http.StatusUnsupportedMediaType, "this route's request body must be sent as Content-Type: application/json"
		}
	}
	return 0, ""
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// hostAllowed reports whether a Host header names deskd: a loopback name or IP,
// or one of the configured hosts. The port is not compared here — the Origin
// check compares host:port where it matters.
func (g OriginGuard) hostAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	for _, h := range g.Hosts {
		if strings.EqualFold(strings.TrimSpace(h), host) {
			return true
		}
	}
	return false
}

// PublicHosts derives the extra host names a deployment is served under from
// its configured OAuth redirect URL. An unparseable or loopback URL adds none.
func PublicHosts(redirectURL string) []string {
	u, err := url.Parse(strings.TrimSpace(redirectURL))
	if err != nil || u.Hostname() == "" {
		return nil
	}
	h := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(h); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		return nil
	}
	if h == "localhost" {
		return nil
	}
	return []string{h}
}

// ServedHosts is the [OriginGuard.Hosts] list for a deployment: the host of
// the bind address when it names one (not a wildcard or loopback), plus the
// public host of the OAuth redirect URL.
func ServedHosts(bindAddr, redirectURL string) []string {
	var out []string
	if h, _, err := net.SplitHostPort(bindAddr); err == nil && h != "" {
		h = strings.ToLower(h)
		if ip := net.ParseIP(h); ip == nil || !(ip.IsLoopback() || ip.IsUnspecified()) {
			if h != "localhost" {
				out = append(out, h)
			}
		}
	}
	return append(out, PublicHosts(redirectURL)...)
}
