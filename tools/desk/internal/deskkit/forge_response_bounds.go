package deskkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/forgeread/adapters"
)

// mirrorResponseLimitError is inspectable through the SDK/DeskError cause chain.
// It carries no response material, endpoint or credential.
type mirrorResponseLimitError struct{}

func (*mirrorResponseLimitError) Error() string {
	return "mirror response exceeds decoded JSON byte limit"
}
func (*mirrorResponseLimitError) LimitBytes() int64 { return adapters.MaxUpdateBytes }

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
	if contextual, ok := next.(forgeContextTransport); ok {
		contextual.next = countRoundTrips(contextual.next)
		next = contextual
	} else {
		next = countRoundTrips(next)
	}
	c.Transport = countedTransport{next: mirrorResponseTransport{next: next}}
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

type mirrorResponseTransport struct{ next http.RoundTripper }

func (t mirrorResponseTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(r)
	if err != nil {
		return resp, err
	}
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("mirror response has no body")
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
	if int64(len(body)) > adapters.MaxUpdateBytes {
		return nil, &mirrorResponseLimitError{}
	}
	if err != nil {
		return nil, fmt.Errorf("mirror response read did not complete: %w", err)
	}
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
