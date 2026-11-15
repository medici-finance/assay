// Package client is the separately imported forge-service-v1 HTTP adapter.
// It discovers no credential or endpoint and has no provider fallback or retry.
package client

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

// Config comes from trusted composition, not a request, repository file or URL
// received from a server. Transport must already hold the caller's admission.
type Config struct {
	Endpoint  string
	Transport http.RoundTripper
	Scope     service.Scope
	Timeout   time.Duration
	// DiagnosticConsent means both transport-end policies explicitly admit links.
	// The server must enforce its own consent; this is no automatic header opt-in.
	DiagnosticConsent bool
}
type Client struct {
	endpoint  url.URL
	transport http.RoundTripper
	scope     service.Scope
	timeout   time.Duration
	links     bool
}

// Report measures this client's own invocation only. Server handlers in the
// response remain separate; neither is copied into another process's call total.
type Report struct {
	LogicalCalls, TransportAttempts     int64
	RequestBodyBytes, ResponseBodyBytes *int64
	Duration                            time.Duration
}

func New(c Config) (*Client, error) {
	if c.Transport == nil || c.Timeout <= 0 || c.Timeout > time.Minute {
		return nil, service.ErrInvalid
	}
	v := reflect.ValueOf(c.Transport)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface || v.Kind() == reflect.Func) && v.IsNil() {
		return nil, service.ErrInvalid
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Opaque != "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return nil, service.ErrInvalid
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return nil, service.ErrInvalid
		}
	}
	probe := service.Request{Schema: service.Schema, ID: "binding", Scope: c.Scope, Operation: service.ReviewObservation}
	if err := service.ValidateRequest(probe); err != nil {
		return nil, err
	}
	owned, err := service.EncodeRequest(probe)
	if err != nil {
		return nil, err
	}
	copy, err := service.DecodeRequest(owned)
	if err != nil {
		return nil, err
	}
	return &Client{endpoint: *u, transport: c.Transport, scope: copy.Scope, timeout: c.Timeout, links: c.DiagnosticConsent}, nil
}

func (c *Client) Do(ctx context.Context, q service.Request) (out service.Response, report Report, err error) {
	start := time.Now()
	report.LogicalCalls = 1
	defer func() { report.Duration = time.Since(start) }()
	fail := func(reason mirror.Reason, cause error) (service.Response, Report, error) {
		return service.Held(q, reason), report, cause
	}
	if c == nil || ctx == nil {
		return fail(mirror.Unknown, service.ErrInvalid)
	}
	if !service.SameScope(q.Scope, c.scope) || (q.Diagnostic != nil && !c.links) {
		return fail(mirror.Permission, service.ErrScope)
	}
	b, err := service.EncodeRequest(q)
	if err != nil {
		return fail(mirror.Unknown, err)
	}
	// Own the request before transport/diagnostic code runs. Later caller mutation
	// cannot change the binding against which a response is checked.
	q, err = service.DecodeRequest(b)
	if err != nil {
		return fail(mirror.Unknown, err)
	}
	if !service.SameScope(q.Scope, c.scope) || (q.Diagnostic != nil && !c.links) {
		return fail(mirror.Permission, service.ErrScope)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return fail(mirror.Transport, err)
	}
	u := c.endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return fail(mirror.Unknown, service.ErrInvalid)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	n := int64(len(b))
	report.RequestBodyBytes = &n
	report.TransportAttempts++
	// RoundTrip sends exactly this request; unlike Client.Do it follows no redirect
	// and supplies no default transport, proxy, cookie jar or retry mechanism.
	response, sendErr := c.transport.RoundTrip(req)
	closed := false
	if response != nil && response.Body != nil {
		defer func() {
			if !closed {
				response.Body.Close()
			}
		}()
	}
	if sendErr != nil || response == nil || response.Body == nil {
		if ctx.Err() != nil {
			return fail(mirror.Transport, ctx.Err())
		}
		return fail(mirror.Transport, service.ErrInvalid)
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNotFound {
		return fail(mirror.Permission, service.ErrScope)
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return fail(mirror.RateLimited, service.ErrInvalid)
	}
	if response.StatusCode == http.StatusConflict {
		return fail(mirror.Unknown, service.ErrConflict)
	}
	if response.StatusCode != http.StatusOK {
		return fail(mirror.Transport, service.ErrInvalid)
	}
	media, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || media != "application/json" {
		return fail(mirror.Unknown, service.ErrInvalid)
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, service.MaxMessageBytes+1))
	closeErr := response.Body.Close()
	closed = true
	read := int64(len(raw))
	report.ResponseBodyBytes = &read
	if readErr != nil || closeErr != nil || len(raw) > service.MaxMessageBytes {
		return fail(mirror.Transport, service.ErrInvalid)
	}
	out, err = service.DecodeResponse(raw, q)
	if err != nil {
		return fail(mirror.Unknown, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(mirror.Transport, err)
	}
	if out.Conflict {
		return fail(mirror.Unknown, service.ErrConflict)
	}
	if out.Metadata.State == service.Permission {
		return out, report, service.ErrScope
	}
	if out.Metadata.State == service.Unavailable {
		return out, report, service.ErrInvalid
	}
	return out, report, nil
}
