package deskkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/forgeread/mirror"
)

// mirrorResponseLimitError is inspectable through the SDK/DeskError cause chain.
// It carries no response body, header, endpoint or credential.
type mirrorResponseLimitError struct{}

func (*mirrorResponseLimitError) Error() string {
	return "mirror response exceeds decoded JSON byte limit"
}
func (*mirrorResponseLimitError) LimitBytes() int64 { return adapters.MaxUpdateBytes }

// The native guard can reject before either SDK maps an HTTP error. Preserve
// that known status independently of the rejection cause. Added HTTP metadata
// contains status only. Error renders only the original cause.
type mirrorResponseStatusError struct {
	cause  error
	status int
}

func (e *mirrorResponseStatusError) Error() string { return e.cause.Error() }
func (e *mirrorResponseStatusError) Unwrap() []error {
	return []error{e.cause, &ForgeAPIError{Status: e.status}}
}

// mirrorResponseClient copies the admitted transport. Only the two enumerated
// incremental methods use this copy; ordinary reads, logs and writes keep theirs.
func mirrorResponseClient(base *http.Client) *http.Client {
	c := &http.Client{}
	if base != nil {
		*c = *base
	}
	next := c.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	// Keep counting/observation at the actual native leaf, beneath the bounded
	// read. Otherwise overflow would be reported as zero response bytes after
	// this adapter had already consumed the detection prefix. Preserve an
	// existing counted leaf rather than hiding it behind the context wrapper.
	var attempt *mirrorAttemptState
	if contextual, ok := next.(forgeContextTransport); ok {
		attempt = mirrorAttemptContext(contextual.ctx)
		if attempt != nil {
			leaf, covered := mirrorAttemptLeaf(contextual.next, attempt)
			if !covered {
				next = mirrorAttemptRefusal{attempt}
			} else {
				contextual.next = leaf
				next = contextual
			}
		} else {
			contextual.next = countRoundTrips(contextual.next)
			next = contextual
		}
	} else {
		next = countRoundTrips(next)
	}
	c.Transport = countedTransport{next: mirrorResponseTransport{next: next, attempt: attempt}}
	return c
}

func (g *GitHubForge) mirrorAccess(ctx context.Context) adapters.Access {
	cp := WithForgeReadContext(g, ctx).(*GitHubForge)
	cp.Client = mirrorResponseClient(cp.Client)
	return cp.sharedAccess()
}

func (g *GitLabForge) mirrorAccess(ctx context.Context) adapters.Access {
	cp := WithForgeReadContext(g, ctx).(*GitLabForge)
	cp.Client = mirrorResponseClient(cp.Client)
	return cp.sharedAccess()
}

type mirrorAttemptRefusal struct{ state *mirrorAttemptState }

func (t mirrorAttemptRefusal) RoundTrip(*http.Request) (*http.Response, error) {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	return nil, t.state.hold(mirror.AttemptUnknownHeld)
}

type mirrorResponseTransport struct {
	next    http.RoundTripper
	attempt *mirrorAttemptState
}

func (t mirrorResponseTransport) RoundTrip(r *http.Request) (out *http.Response, failure error) {
	var resp *http.Response
	// Run after every later stage, including a failed accounting observation.
	// A known non-success status survives even when the SDK never sees the body.
	defer func() {
		if failure != nil && resp != nil && resp.StatusCode != 0 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
			failure = &mirrorResponseStatusError{cause: failure, status: resp.StatusCode}
		}
	}()
	var consumed int64
	complete := false
	if t.attempt != nil {
		defer func() {
			ctx := context.Background()
			if r != nil {
				ctx = r.Context()
			}
			if err := t.attempt.finish(ctx, consumed, true, complete); err != nil {
				if out != nil && out.Body != nil {
					_ = out.Body.Close()
				}
				out = nil
				failure = errors.Join(failure, err)
			}
		}()
	}
	var err error
	resp, err = t.next.RoundTrip(r)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("mirror response has no body")
	}
	if t.attempt != nil && resp.Header == nil {
		// Lost pagination metadata cannot qualify a complete provider inventory.
		_ = resp.Body.Close()
		t.attempt.mu.Lock()
		err := t.attempt.hold(mirror.AttemptUnknownHeld)
		t.attempt.mu.Unlock()
		return nil, err
	}
	if t.attempt != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		_ = resp.Body.Close()
		t.attempt.mu.Lock()
		err := t.attempt.hold(mirror.AttemptUnknownHeld)
		t.attempt.mu.Unlock()
		return nil, err
	}
	// Finish a bounded read before handing either a success or error response to
	// an SDK. A streaming decoder may accept a short JSON prefix and drain the
	// remainder afterwards; bounding only its Decode call would miss that tail.
	defer resp.Body.Close()
	var decoded io.Reader = resp.Body
	encoding := strings.TrimSpace(resp.Header.Get("Content-Encoding"))
	if strings.EqualFold(encoding, "gzip") {
		z, zerr := gzip.NewReader(resp.Body)
		if zerr != nil {
			return nil, fmt.Errorf("mirror gzip response: %w", zerr)
		}
		defer z.Close()
		decoded = z
	} else if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return nil, fmt.Errorf("mirror response has unsupported content encoding")
	}
	// Default net/http transports have already decoded their automatic gzip
	// response; explicit gzip from an injected transport is decoded above. The
	// cap therefore measures JSON bytes, not Content-Length or compressed bytes.
	body, err := io.ReadAll(io.LimitReader(decoded, adapters.MaxUpdateBytes+1))
	consumed = int64(len(body))
	if int64(len(body)) > adapters.MaxUpdateBytes {
		return nil, &mirrorResponseLimitError{}
	}
	if err != nil {
		return nil, fmt.Errorf("mirror response read did not complete: %w", err)
	}
	complete = true
	cp := *resp
	cp.Header = resp.Header.Clone()
	cp.Header.Del("Content-Encoding")
	cp.Header.Del("Content-Length")
	cp.ContentLength = int64(len(body))
	cp.TransferEncoding = nil
	cp.Uncompressed = resp.Uncompressed || strings.EqualFold(encoding, "gzip")
	cp.Body = io.NopCloser(bytes.NewReader(body))
	return &cp, nil
}
