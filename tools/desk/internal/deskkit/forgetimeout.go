package deskkit

import (
	"net/http"
	"time"
)

// forgetimeout.go — the one per-request bound every forge client in the desk tools carries.
//
// Why a bound at all: the desk write verbs (deskevidence, deskpost, deskrelease) hold the
// shared audit lock (<state dir>/audit.lock) across forge reads and writes. A client with no
// overall deadline — http.DefaultClient, or a client built on http.DefaultTransport with a zero
// Timeout — lets one stalled connection pin that lock for as long as the TCP stack takes to give
// up, and every other desk writer on the machine waits behind it. The acquire wait (60s in each
// tool's lockWait) bounds the WAITERS; this bounds the HOLDER, one request at a time.
//
// Why 30s: the forge API answers its own requests in seconds (GitHub ends an API request
// server-side well inside that), so a request still open after 30s is stalled, not slow. It is
// per request, not per invocation, and it covers the whole exchange including the response body.
// The largest body a forge client reads is a run-log archive, capped at runLogArchiveCap
// (64 MiB): reading a full-size archive inside 30s needs about 2.2 MB/s. A slower link gets a
// could-not-check naming the timeout — never a partial log, because a body cut off mid-transfer
// is already refused as Unverifiable.
const ForgeRequestTimeout = 30 * time.Second

// BoundedHTTPClient returns a fresh client on the default transport that gives up on any one
// request after ForgeRequestTimeout. It is the replacement for http.DefaultClient wherever a
// forge client would otherwise fall back to it; TestNoUnboundedForgeClient keeps the fallback
// from coming back.
func BoundedHTTPClient() *http.Client {
	return &http.Client{Timeout: ForgeRequestTimeout}
}

// boundedTimeout is the deadline a client built from base should carry: base's own when it
// sets one, else ForgeRequestTimeout. A zero Timeout means "no deadline" to net/http, so it is
// never passed through.
func boundedTimeout(base *http.Client) time.Duration {
	if base != nil && base.Timeout > 0 {
		return base.Timeout
	}
	return ForgeRequestTimeout
}
