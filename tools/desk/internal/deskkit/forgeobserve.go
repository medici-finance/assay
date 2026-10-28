package deskkit

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// RequestObserver is an explicitly passed, invocation-owned observation handle.
// It contains no credential, target, or process-global attribution state. Its zero
// value leaves the transport unchanged. The recorder bounds and validates labels.
type RequestObserver struct {
	Sink   perfrec.RequestSink
	Origin perfrec.Origin
}

// RequestObserverFor admits observation only after the actual parser selected
// an opt-in profile. An unselected invocation does no optional provider work.
func RequestObserverFor(ctx perfrec.Context, sink perfrec.RequestSink, origin perfrec.Origin) RequestObserver {
	if ctx.Profile() == "" || sink == nil {
		return RequestObserver{}
	}
	if session, ok := sink.(*perfrec.Session); ok && session == nil {
		return RequestObserver{}
	}
	return RequestObserver{Sink: sink, Origin: origin}
}

type requestScope struct {
	observer RequestObserver
	class    perfrec.RequestClass
	gitlab   bool
}
type requestScopeKey struct{}

type scopedTransport struct {
	next  http.RoundTripper
	scope requestScope
}

func (s scopedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return s.next.RoundTrip(r.WithContext(context.WithValue(r.Context(), requestScopeKey{}, s.scope)))
}

func observedTransport(rt http.RoundTripper, o RequestObserver, class perfrec.RequestClass, gitlab bool) http.RoundTripper {
	if o.Sink == nil {
		return rt
	}
	if o.Origin == "" {
		o.Origin = perfrec.OriginUnknown
	}
	return scopedTransport{next: rt, scope: requestScope{o, class, gitlab}}
}

// ObserveAuthClient copies the caller's client and instruments only its actual
// authentication sends. It neither reads nor changes credential custody.
func ObserveAuthClient(c *http.Client, o RequestObserver) *http.Client {
	if o.Sink == nil {
		return c
	}
	if c == nil {
		c = http.DefaultClient
	}
	cp := *c
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	cp.Transport = observedTransport(authTransport{base}, o, perfrec.RequestAuthentication, false)
	return &cp
}

type authTransport struct{ next http.RoundTripper }

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return observeRoundTrip(a.next, r)
}

func requestLabels(r *http.Request, s requestScope) perfrec.RequestObservation {
	o := perfrec.RequestObservation{Class: s.class, Origin: s.observer.Origin,
		Protocol: "rest", Resource: "core", Points: perfrec.Points{Coverage: perfrec.CoverageUnsupported, Unit: perfrec.UnitNone}}
	if s.class == perfrec.RequestAuthentication {
		o.Protocol, o.Resource = "http", "authentication"
	} else if !s.gitlab && r.URL.Path == "/graphql" || s.gitlab && r.URL.Path == "/api/graphql" {
		o.Protocol, o.Resource = "graphql", "graphql"
		o.Points = perfrec.Points{Coverage: perfrec.CoverageUnknown, Unit: perfrec.UnitGitHubPoints}
		if s.gitlab {
			o.Resource = "complexity"
			o.Points.Unit = perfrec.UnitGitLabComplexity
		}
	} else if strings.HasPrefix(r.URL.Path, "/search/") {
		o.Resource = "search"
	}
	return o
}

func requestOutcome(status int, err error) string {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "cancelled"
		}
		var timeout net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
			return "timeout"
		}
		return "error"
	}
	if status == http.StatusNotModified {
		return "304"
	}
	switch status / 100 {
	case 2:
		return "2xx"
	case 3:
		return "3xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	}
	return "error"
}

// Observation is at the counting leaf, below adapters that can refuse or replay
// locally. There is no additional read, request, retry, or response buffering.
func observeRoundTrip(next http.RoundTripper, r *http.Request) (*http.Response, error) {
	scope, ok := r.Context().Value(requestScopeKey{}).(requestScope)
	if !ok || scope.observer.Sink == nil {
		return next.RoundTrip(r)
	}
	start := time.Now()
	o := requestLabels(r, scope)
	resp, err := next.RoundTrip(r)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	o.Outcome = requestOutcome(status, err)
	b := &observedBody{start: start, sink: scope.observer.Sink, observation: o}
	if err != nil || resp == nil || resp.Body == nil {
		b.finish(err)
		return resp, err
	}
	b.body = resp.Body
	if capture, ok := r.Context().Value(pointCaptureKey{}).(*pointCapture); ok {
		capture.body = b
	}
	// Qualified decoders attach metadata from bytes they already consume. Delay
	// their successful GraphQL observation until Close, retaining only scalars.
	b.deferCost = o.Protocol == "graphql" && status/100 == 2
	cp := *resp
	cp.Body = b
	return &cp, err
}

type observedBody struct {
	body        io.ReadCloser
	start       time.Time
	sink        perfrec.RequestSink
	mu          sync.Mutex
	once        sync.Once
	observation perfrec.RequestObservation
	bytes       int64
	end         time.Time
	deferCost   bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	b.mu.Lock()
	b.bytes += int64(n)
	if err != nil && b.end.IsZero() {
		b.end = time.Now()
	}
	delay := b.deferCost && err == io.EOF
	b.mu.Unlock()
	if err != nil && !delay {
		b.finish(err)
	}
	return n, err
}
func (b *observedBody) Close() error {
	err := b.body.Close()
	b.finish(err)
	return err
}
func (b *observedBody) finish(err error) {
	b.once.Do(func() {
		b.mu.Lock()
		o := b.observation
		end := b.end
		if end.IsZero() {
			end = time.Now()
		}
		ms, count := end.Sub(b.start).Milliseconds(), b.bytes
		o.DurationMs, o.DurationCoverage, o.Bytes = &ms, perfrec.CoverageMeasured, &count
		if err != nil && err != io.EOF {
			o.Outcome = requestOutcome(0, err)
		}
		b.mu.Unlock()
		b.sink.ObserveRequest(o)
	})
}

func attachRequestPoints(body io.ReadCloser, p perfrec.Points) {
	if b, ok := body.(*observedBody); ok {
		b.mu.Lock()
		b.observation.Points = p
		b.mu.Unlock()
	}
}
