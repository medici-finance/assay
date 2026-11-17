package deskkit

import (
	"fmt"
	"io"
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

// ReadForgeBody reads a forge response body to the end, for a client whose caller decodes the
// result when want is true. The per-request deadline above is what makes this necessary: a
// body that stalls past it ends in a read error with only a prefix in hand, and a prefix such as
// `[]` decodes cleanly into an empty list. So:
//
//   - a read error is a could-not-check (Unverifiable), never a short body taken as complete;
//   - a 2xx with an EMPTY body, when the caller wants a result, is a could-not-check too —
//     except 204 No Content and 205 Reset Content, which carry no body by definition.
//
// what names the request for the error ("GET /repos/..."). A non-2xx body is returned as read;
// the caller's own status handling decides what it means.
func ReadForgeBody(resp *http.Response, what string, want bool) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Unverifiable(fmt.Sprintf("%s: response body not read in full", what), err)
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	noBody := resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusResetContent
	if want && ok && !noBody && len(raw) == 0 {
		return nil, Unverifiable(fmt.Sprintf("%s: HTTP %d with an empty body where a result was expected", what, resp.StatusCode), nil)
	}
	return raw, nil
}

// boundedCopy returns a copy of base carrying boundedTimeout(base) — a fresh bounded client
// when base is nil. The caller's client is never mutated. Every place the forge library
// copies an injected client to adjust its transport or redirect policy starts from this, so
// no copy can drop the deadline (TestNoUnboundedForgeClient refuses a bare http.Client literal).
func boundedCopy(base *http.Client) *http.Client {
	if base == nil {
		return BoundedHTTPClient()
	}
	cp := *base
	cp.Timeout = boundedTimeout(base)
	return &cp
}
