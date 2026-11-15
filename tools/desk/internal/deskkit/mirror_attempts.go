package deskkit

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	ghupdates "github.com/medici-finance/assay/forgeread/adapters/github"
	glupdates "github.com/medici-finance/assay/forgeread/adapters/gitlab"
	"github.com/medici-finance/assay/forgeread/mirror"
)

type mirrorAttemptKey struct{}

// Private invocation state is supplied by the admitted source composition. It
// never comes from a request header, query, endpoint or a process-global hook.
type mirrorAttemptState struct {
	mu           sync.Mutex
	ctx          context.Context
	scope        mirror.Scope
	repository   mirror.RepositoryID
	sourceID     string
	admissionID  string
	admission    mirror.Admission
	grant        mirror.AttemptGrant
	authority    mirror.AttemptAuthority
	now          func() time.Time
	origin       url.URL
	path         string
	since        time.Time
	page         int
	receipt      mirror.AttemptReceipt
	pending      *mirrorPendingAttempt
	reservations map[string]bool
}

type mirrorPendingAttempt struct {
	reservation mirror.AttemptReservation
	observation mirror.AttemptObservation
}

func mirrorAttemptContext(ctx context.Context) *mirrorAttemptState {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(mirrorAttemptKey{}).(*mirrorAttemptState)
	return s
}

func newMirrorAttempt(ctx context.Context, s *mirrorIngestSource, a mirror.Admission, scope mirror.Scope, plan mirror.ScanPlan) (*mirrorAttemptState, error) {
	b := s.source.forge
	for {
		switch v := b.(type) {
		case *readOnlyForge:
			b = v.inner
		case *outboundForge:
			b = v.Forge
		default:
			goto unwrapped
		}
	}
unwrapped:
	var origin *url.URL
	var err error
	rel := "issues"
	if scope.Kind == mirror.Changes {
		rel = "pulls"
	}
	switch g := b.(type) {
	case *GitHubForge:
		origin, err = url.Parse(g.baseURL())
		if err == nil {
			origin.Path = strings.TrimRight(origin.Path, "/") + "/repos/" + s.source.repo.Slug() + "/" + rel
			origin.RawPath = ""
		}
	case *GitLabForge:
		// Reuse the selected SDK's actual base-path construction, without any
		// request or custody lookup. The operational read keeps its own fence.
		raw, e := url.Parse(g.baseURL())
		if e != nil || raw.User != nil || raw.RawQuery != "" || raw.Fragment != "" || raw.Opaque != "" {
			return nil, &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
		}
		cp := *g
		cp.cl = nil
		client, e := cp.client()
		if e != nil {
			return nil, e
		}
		origin = client.BaseURL()
		if scope.Kind == mirror.Changes {
			rel = "merge_requests"
		}
		prefix := origin.EscapedPath()
		origin.Path += "projects/" + s.source.repo.Slug() + "/" + rel
		origin.RawPath = prefix + "projects/" + url.PathEscape(s.source.repo.Slug()) + "/" + rel
	default:
		return nil, &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
	}
	if err != nil || origin == nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "https" && origin.Scheme != "http") {
		return nil, &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
	}
	for _, part := range strings.Split(origin.Path, "/") {
		if part == "." || part == ".." {
			return nil, &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
		}
	}
	return &mirrorAttemptState{ctx: ctx, scope: scope, repository: s.repository, sourceID: s.sourceID, admissionID: s.admissionID, admission: a, grant: plan.AttemptGrant, authority: s.authority, now: s.now,
		origin: *origin, path: origin.EscapedPath(), since: plan.Since, page: 1, reservations: map[string]bool{},
		receipt: mirror.AttemptReceipt{Grant: plan.AttemptGrant, HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}}, nil
}

// Unknown opaque wrappers, including a legacy cache, cannot prove fresh provider
// coverage. Never replace them with a different transport/TLS/custody composition.
func mirrorAttemptLeaf(rt http.RoundTripper, state *mirrorAttemptState) (http.RoundTripper, bool) {
	if rt == nil {
		rt = http.DefaultTransport
	}
	switch t := rt.(type) {
	case *http.Transport:
		return countingTransport{next: t, mirror: state}, true
	case countingTransport:
		return mirrorAttemptLeaf(t.next, state)
	case countedTransport:
		next, ok := mirrorAttemptLeaf(t.next, state)
		return countedTransport{next: next}, ok
	case forgeContextTransport:
		next, ok := mirrorAttemptLeaf(t.next, state)
		t.next = next
		return t, ok
	default:
		return nil, false
	}
}

func (s *mirrorAttemptState) query() string {
	var q url.Values
	if s.scope.Repository.Provider == mirror.GitHub {
		q = url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {strconv.Itoa(ghupdates.PerPage)}, "page": {strconv.Itoa(s.page)}}
		if s.scope.Kind == mirror.Issues && !s.since.IsZero() {
			lower := s.since.UTC().Add(-time.Second)
			if lower.Year() >= 1 {
				q.Set("since", lower.Format(time.RFC3339Nano))
			}
		}
	} else {
		q = url.Values{"state": {"all"}, "order_by": {"updated_at"}, "sort": {"desc"}, "per_page": {strconv.Itoa(glupdates.PerPage)}, "page": {strconv.Itoa(s.page)}}
		if !s.since.IsZero() {
			q.Set("updated_after", s.since.UTC().Format(time.RFC3339Nano))
		}
	}
	return q.Encode()
}

func (s *mirrorAttemptState) hold(h mirror.AttemptHold) error {
	if !h.Valid() {
		h = mirror.AttemptUnknownHeld
	}
	// Later refused entries do no new work and cannot widen the original
	// caller-specific denial into a source-wide or uncertain accounting hold.
	if s.receipt.Held == "" {
		s.receipt.Held = h
	}
	return &mirror.AttemptError{Hold: s.receipt.Held}
}

// Called only by the existing native counting leaf, immediately before its base
// RoundTripper. Source-level validation and redirect refusal are independent.
func (s *mirrorAttemptState) roundTrip(base http.RoundTripper, r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	maxPages := ghupdates.MaxPages
	if s.scope.Repository.Provider == mirror.GitLab {
		maxPages = glupdates.MaxPages
	}
	if s.page < 1 || s.page > maxPages {
		return nil, s.hold(mirror.AttemptUnknownHeld)
	}
	if r == nil || r.URL == nil || r.Method != http.MethodGet || r.URL.Scheme != s.origin.Scheme || r.URL.Host != s.origin.Host || (r.Host != "" && r.Host != s.origin.Host) || r.URL.User != nil || r.URL.Opaque != "" || r.URL.Fragment != "" || r.URL.EscapedPath() != s.path || r.URL.RawQuery != s.query() || s.grant.Validate(now) != nil || s.grant.Scope != s.scope || s.scope.Repository != s.repository || s.grant.SourceID != s.sourceID || s.scope.AdmissionID != s.admissionID || s.pending != nil || nilMirrorInput(s.authority) {
		return nil, s.hold(mirror.AttemptUnknownHeld)
	}
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	if s.receipt.Held != "" || s.receipt.Retry.Knowledge == mirror.RetryUnknown || now.Before(s.receipt.Retry.NotBefore) {
		return nil, s.hold(mirror.AttemptSourceHeld)
	}
	if s.receipt.HTTP.Value >= s.grant.MaxHTTPAttempts {
		return nil, s.hold(mirror.AttemptSourceHeld)
	}
	if nilMirrorInput(s.admission) || s.admission.ID() != s.scope.AdmissionID || s.admission.Authorize(r.Context(), s.scope.Repository) != nil || s.admission.ID() != s.scope.AdmissionID {
		return nil, mirror.ErrPermission
	}
	reservation, err := s.authority.BeforeAttempt(r.Context(), s.grant)
	if err != nil {
		var held *mirror.AttemptError
		if errors.As(err, &held) && held != nil {
			return nil, s.hold(held.Hold)
		}
		return nil, s.hold(mirror.AttemptUnknownHeld)
	}
	if reservation.ID == "" || s.reservations[reservation.ID] || !reservation.Grant.Equal(s.grant) {
		return nil, s.hold(mirror.AttemptUnknownHeld)
	}
	s.reservations[reservation.ID] = true
	s.pending = &mirrorPendingAttempt{reservation: reservation, observation: mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: now}}
	// An authority call may block. Recheck current admission/deadline afterwards;
	// a reserved slot is not permission to send a request.
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	if s.admission.ID() != s.scope.AdmissionID || s.admission.Authorize(r.Context(), s.scope.Repository) != nil || s.admission.ID() != s.scope.AdmissionID {
		return nil, mirror.ErrPermission
	}
	if s.grant.Validate(s.now()) != nil {
		return nil, s.hold(mirror.AttemptSourceHeld)
	}
	s.pending.observation.HTTP.Value = 1
	s.receipt.HTTP.Value++
	perfCountForgeCall()
	resp, err := observeRoundTrip(base, r)
	observed := s.now()
	s.pending.observation.ObservedAt = observed
	if observed.IsZero() || observed.Before(now) {
		s.pending.observation.Retry.Knowledge = mirror.RetryUnknown
	} else if resp == nil {
		s.pending.observation.Retry.Knowledge = mirror.RetryNoResponse
		if err == nil {
			s.pending.observation.Retry.Knowledge = mirror.RetryUnknown
		}
	} else {
		s.pending.observation.Status = resp.StatusCode
		s.pending.observation.Retry = mirrorRetryEvidence(s.scope.Repository.Provider, resp.StatusCode, resp.Header, observed)
		if err != nil {
			s.pending.observation.Retry.Knowledge = mirror.RetryUnknown
		}
	}
	if resp == nil && err == nil {
		err = Unverifiable("mirror native transport returned no response or cause", nil)
	}
	return resp, err
}

func (s *mirrorAttemptState) snapshot() mirror.AttemptReceipt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.receipt
}

// The existing bound owns consumption. Report after that read, before any SDK
// response/error mapping. A missing/uncertain report never releases a reservation.
func (s *mirrorAttemptState) finish(ctx context.Context, n int64, known, complete bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return nil
	}
	p := s.pending
	if complete && p.observation.Status >= 200 && p.observation.Status < 300 {
		dataAt := s.now()
		if dataAt.IsZero() || dataAt.Before(p.observation.ObservedAt) || dataAt.Before(s.receipt.ObservedAt) {
			_ = s.hold(mirror.AttemptUnknownHeld)
			p.observation.Retry.Knowledge = mirror.RetryUnknown
			complete = false
		} else {
			s.receipt.ObservedAt = dataAt
		}
	}
	p.observation.DecodedBytes = mirror.WorkCount{Value: n, Known: known}
	p.observation.ResponseComplete = complete
	s.receipt.DecodedBytes.Value += n
	s.receipt.DecodedBytes.Known = s.receipt.DecodedBytes.Known && known
	if p.observation.HTTP.Value == 1 {
		if s.receipt.HTTP.Value == 1 {
			s.receipt.Retry = p.observation.Retry
		} else {
			s.receipt.Retry = mergeMirrorRetry(s.receipt.Retry, p.observation.Retry)
		}
	}
	// The cleanup deadline is bounded by the same finite trusted grant even if
	// caller cancellation prevents an ordinary response observation.
	report, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.grant.ExpiresAt.Sub(s.now()))
	defer cancel()
	if s.ctx != nil {
		if deadline, ok := s.ctx.Deadline(); ok {
			bounded, stop := context.WithDeadline(report, deadline)
			defer stop()
			report = bounded
		}
	}
	if err := s.authority.ObserveAttempt(report, p.reservation, p.observation); err != nil {
		return s.hold(mirror.AttemptUnknownHeld)
	}
	s.pending = nil
	if complete && p.observation.Status >= 200 && p.observation.Status < 300 {
		s.page++
	}
	if s.receipt.Held != "" {
		return &mirror.AttemptError{Hold: s.receipt.Held}
	}
	return nil
}

func mergeMirrorRetry(a, b mirror.RetryEvidence) mirror.RetryEvidence {
	if b.NotBefore.After(a.NotBefore) {
		a.NotBefore = b.NotBefore
	}
	if b.DelayKnown && (!a.DelayKnown || b.Delay > a.Delay) {
		a.Delay, a.DelayKnown = b.Delay, true
	}
	if a.Knowledge == mirror.RetryUnknown || b.Knowledge == mirror.RetryUnknown {
		a.Knowledge = mirror.RetryUnknown
	} else if !a.NotBefore.IsZero() {
		a.Knowledge = mirror.RetryNotBefore
	} else if a.Knowledge == mirror.RetryNoResponse && b.Knowledge == mirror.RetryNoResponse {
		a.Knowledge = mirror.RetryNoResponse
	} else {
		a.Knowledge = mirror.RetryNoConstraint
	}
	return a
}

func mirrorRetryEvidence(provider mirror.Provider, status int, h http.Header, now time.Time) mirror.RetryEvidence {
	if h == nil {
		return mirror.RetryEvidence{Knowledge: mirror.RetryUnknown}
	}
	out := mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}
	unknown := false
	value := func(key string) (string, bool) {
		values := h.Values(key)
		if len(values) == 0 {
			return "", false
		}
		if len(values) != 1 || len(values[0]) > 128 || strings.TrimSpace(values[0]) == "" {
			unknown = true
			return "", true
		}
		return strings.TrimSpace(values[0]), true
	}
	setFloor := func(floor time.Time) {
		if floor.Before(now) {
			unknown = true
		}
		if floor.After(out.NotBefore) {
			out.NotBefore = floor
		}
	}
	retry, hasRetry := value("Retry-After")
	if hasRetry {
		seconds, err := strconv.ParseUint(retry, 10, 64)
		if err == nil && seconds <= math.MaxInt64/uint64(time.Second) {
			out.Delay, out.DelayKnown = time.Duration(seconds)*time.Second, true
			setFloor(now.Add(out.Delay))
		} else if stamp, err := http.ParseTime(retry); err == nil {
			setFloor(stamp)
		} else {
			unknown = true
		}
	}
	remaining, hasRemaining := "", false
	primaryExhausted := false
	if provider == mirror.GitHub {
		remaining, hasRemaining = value("X-RateLimit-Remaining")
	}
	if hasRemaining {
		n, err := strconv.ParseUint(remaining, 10, 64)
		if err != nil {
			unknown = true
		} else if n == 0 {
			primaryExhausted = true
			reset, ok := value("X-RateLimit-Reset")
			epoch, err := strconv.ParseInt(reset, 10, 64)
			if !ok || err != nil || strings.Trim(reset, "0123456789") != "" || epoch < 0 || epoch > 253402300799 {
				unknown = true
			} else {
				setFloor(time.Unix(epoch, 0).UTC())
			}
		}
	}
	if status == http.StatusTooManyRequests && !hasRetry && !primaryExhausted {
		unknown = true
	}
	if status == http.StatusForbidden && provider == mirror.GitHub && !hasRetry && !primaryExhausted {
		unknown = true
	}
	if !out.NotBefore.IsZero() {
		out.Knowledge = mirror.RetryNotBefore
	}
	if unknown || now.IsZero() {
		out.Knowledge = mirror.RetryUnknown
	}
	return out
}
