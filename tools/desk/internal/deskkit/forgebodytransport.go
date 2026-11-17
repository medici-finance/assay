package deskkit

// forgebodytransport.go — the byte cap applied in the TRANSPORT of every forge client this
// package builds (restClient, GitLabForge.client, GitLabForge.triggerClient).
//
// ReadCappedForgeBody bounds the reads this package writes itself. The client libraries read
// bodies too: go-gh reads a non-2xx JSON answer whole to build its HTTPError, and the GitLab
// client reads error bodies whole and stream-decodes 2xx answers. None of those reads is a
// call this tree makes, so no source guard can see them. Capping the response body in the
// round-tripper bounds every one of them, and every read a caller makes on a body the
// clients hand back, whatever reader shape it uses.
//
// A body past the cap is an ERROR, never a short body:
//   - a non-2xx answer is read here, under the cap, before the library sees it; over the cap
//     the round-trip itself fails with Unverifiable wrapping *ForgeBodyCapError, so the
//     library reports a failed request, never a ForgeAPIError built from a prefix;
//   - a 2xx answer is streamed through a reader that fails with the same error once more
//     than the cap has arrived, so a decoder sees a read error, never a clean end of input.
//
// TestForgeClientsCapped fails the build when a forge client is constructed without it.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// forgeBodyLimitKey carries a per-request byte limit for a read that keeps its own, larger
// and separately enforced bound (the run-log readers). Only this package can set it.
type forgeBodyLimitKey struct{}

// withForgeBodyLimit returns ctx carrying limit as the transport cap for requests sent with
// it. The caller's own reader must refuse at or below limit, so its own wording wins.
func withForgeBodyLimit(ctx context.Context, limit int64) context.Context {
	return context.WithValue(ctx, forgeBodyLimitKey{}, limit)
}

func forgeBodyLimitFor(req *http.Request) int64 {
	if v, ok := req.Context().Value(forgeBodyLimitKey{}).(int64); ok && v > 0 {
		return v
	}
	return forgeBodyCap
}

// capBodyTransport wraps next so every response body it returns is capped.
type capBodyTransport struct{ next http.RoundTripper }

// capForgeBodies wraps rt (http.DefaultTransport when nil) in the body cap. An already
// capped transport is returned as is.
func capForgeBodies(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	if _, ok := rt.(capBodyTransport); ok {
		return rt
	}
	return capBodyTransport{next: rt}
}

// cappedForgeClient returns a copy of c whose transport caps every response body. c is
// never mutated; a nil c starts from BoundedHTTPClient, so the copy keeps a deadline.
func cappedForgeClient(c *http.Client) *http.Client {
	if c == nil {
		c = BoundedHTTPClient()
	}
	cp := *c
	cp.Transport = capForgeBodies(cp.Transport)
	return &cp
}

func forgeBodyOverCap(req *http.Request, limit int64) error {
	return Unverifiable(fmt.Sprintf("%s %s: response body exceeds the %d-byte forge body cap — refused, never read as a truncated answer",
		req.Method, req.URL.Path, limit), &ForgeBodyCapError{Limit: limit})
}

func (c capBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	limit := forgeBodyLimitFor(req)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, rerr := readCapped(resp.Body, req.Method+" "+req.URL.Path, limit)
		_ = resp.Body.Close()
		if rerr != nil {
			return nil, rerr
		}
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
	resp.Body = &capBody{rc: resp.Body, limit: limit, req: req}
	return resp, nil
}

// capBody fails once more than limit bytes have arrived. It never asks the underlying body
// for more than one byte past the limit, so a hostile body costs at most limit+1 bytes.
type capBody struct {
	rc    io.ReadCloser
	limit int64
	n     int64
	req   *http.Request
	err   error
}

func (b *capBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if room := b.limit - b.n + 1; int64(len(p)) > room {
		p = p[:room]
	}
	m, err := b.rc.Read(p)
	b.n += int64(m)
	if b.n > b.limit {
		b.err = forgeBodyOverCap(b.req, b.limit)
		return m - int(b.n-b.limit), b.err
	}
	return m, err
}

func (b *capBody) Close() error { return b.rc.Close() }
