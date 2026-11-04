package service

import (
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/medici-finance/assay/forgeread/mirror"
)

var (
	// The unchanged review-observation-v1 profile is narrower than source identity.
	reviewPathProfile = regexp.MustCompile("^[a-z0-9_.-]+(/[a-z0-9_.-]+)+$")
	hostName          = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
	oid               = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digest            = regexp.MustCompile(`^[0-9a-f]{64}$`)
	linkID            = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

func invalid() error { return ErrInvalid }
func text(s string, limit int, empty bool) bool {
	if len(s) > limit || !utf8.ValidString(s) || (!empty && strings.TrimSpace(s) == "") {
		return false
	}
	for _, c := range s {
		if c == 0 || c == '\r' || (c < 32 && c != '\n' && c != '\t') {
			return false
		}
	}
	return true
}
func repository(r mirror.RepositoryID) bool {
	// Provider/path identity belongs to mirror; this boundary only adds wire caps.
	return r.Validate() == nil && hostName.MatchString(r.Host) && !strings.Contains(r.Host, "..") && len(r.Host) <= 253 && text(r.Path, 512, false)
}

func scope(s Scope) bool {
	if len(s.Repositories) == 0 || len(s.Repositories) > MaxItems || !text(s.AdmissionID, 256, false) || !text(s.AccessEpoch, 256, false) || !text(s.CallerID, 256, false) || !text(s.RepresentedActor, 256, true) {
		return false
	}
	seen := map[mirror.RepositoryID]bool{}
	for _, r := range s.Repositories {
		if !repository(r) || seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}
func member(s Scope, r mirror.RepositoryID) bool {
	for _, a := range s.Repositories {
		if a == r {
			return true
		}
	}
	return false
}

// SameScope compares already-explicit sets, never resolves aliases or grants access.
func SameScope(a, b Scope) bool {
	if !scope(a) || !scope(b) || len(a.Repositories) != len(b.Repositories) || a.AdmissionID != b.AdmissionID || a.AccessEpoch != b.AccessEpoch || a.CallerID != b.CallerID || a.RepresentedActor != b.RepresentedActor {
		return false
	}
	for _, r := range a.Repositories {
		if !member(b, r) {
			return false
		}
	}
	return true
}
func token(t *Token, s Scope, r mirror.RepositoryID, kind mirror.RecordKind) bool {
	return repository(r) && member(s, r) && (kind == mirror.Issues || kind == mirror.Changes) && t != nil && SameScope(t.Scope, s) && t.Repository == r && t.Kind == kind && text(t.Value, 512, false)
}
func path(p string) bool {
	if !text(p, MaxTextBytes, false) || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\n\r\t:") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func operationID(id OperationID) bool {
	return text(id.Key, 1024, false) && digest.MatchString(id.PayloadDigest)
}
func diagnostic(d *DiagnosticLink) bool {
	return d == nil || (linkID.MatchString(d.Parent) && linkID.MatchString(d.Link))
}
func knownOperation(op Operation) bool {
	switch op {
	case MirrorQuery, FileAt, Diff, MergeBase, CreateIssue, CreateComment, LookupOperation, ReadObservation, ReviewObservation, IssueProjection:
		return true
	}
	return false
}

func ValidateRequest(r Request) error {
	if r.Schema != Schema || !knownOperation(r.Operation) {
		return ErrSchema
	}
	if !scope(r.Scope) || !text(r.ID, 128, false) || !diagnostic(r.Diagnostic) {
		return invalid()
	}
	selectors := 0
	for _, present := range []bool{r.Mirror != nil, r.Source != nil, r.Effect != nil, r.Lookup != nil, r.Minimum != nil} {
		if present {
			selectors++
		}
	}
	if r.Operation == ReviewObservation || r.Operation == IssueProjection {
		if selectors != 0 || r.Repository != (mirror.RepositoryID{}) {
			return invalid()
		}
		return nil
	}
	if selectors != 1 || !member(r.Scope, r.Repository) {
		return ErrScope
	}
	switch r.Operation {
	case MirrorQuery:
		if r.Mirror == nil || (r.Mirror.Kind != mirror.Issues && r.Mirror.Kind != mirror.Changes) || r.Mirror.MaxAgeMS < 1 || r.Mirror.MaxAgeMS > int64((24*time.Hour)/time.Millisecond) {
			return invalid()
		}
		if r.Mirror.Minimum != nil && !token(r.Mirror.Minimum, r.Scope, r.Repository, r.Mirror.Kind) {
			return ErrScope
		}
	case FileAt, Diff, MergeBase:
		if r.Source == nil || !oid.MatchString(r.Source.Revision) {
			return invalid()
		}
		switch r.Operation {
		case FileAt:
			if !path(r.Source.Path) || r.Source.Other != "" {
				return invalid()
			}
		case Diff, MergeBase:
			if !oid.MatchString(r.Source.Other) || r.Source.Path != "" {
				return invalid()
			}
		}
	case CreateIssue, CreateComment:
		if r.Effect == nil || !operationID(r.Effect.ID) || !text(r.Effect.Body, MaxContentBytes, true) {
			return invalid()
		}
		if r.Operation == CreateIssue && (!text(r.Effect.Title, MaxTextBytes, false) || r.Effect.Number != 0) {
			return invalid()
		}
		if r.Operation == CreateComment && (r.Effect.Number < 1 || r.Effect.Title != "" || r.Effect.Body == "") {
			return invalid()
		}
	case LookupOperation:
		if r.Lookup == nil || !operationID(*r.Lookup) {
			return invalid()
		}
	case ReadObservation:
		if r.Minimum == nil || (r.Minimum.Kind != mirror.Issues && r.Minimum.Kind != mirror.Changes) || !token(r.Minimum, r.Scope, r.Repository, r.Minimum.Kind) {
			return ErrScope
		}
	}
	return nil
}

func provenance(p *Provenance) bool {
	return p != nil && text(p.Source, 256, false) && text(p.Version, 256, false) && text(p.Revision, 512, false) && !p.ObservedAt.IsZero()
}

// cachedFreshness checks only the received wire's internal consistency. The
// serving owner must establish current authorization and original snapshot custody.
func cachedFreshness(c *CachedObservation, q Request, source *Provenance) (string, error) {
	if q.Operation != MirrorQuery || q.Mirror == nil || c.EvaluatedAt.IsZero() || c.Coverage.Reason == mirror.Permission {
		return "", invalid()
	}
	selected := mirror.Scope{Repository: q.Repository, Kind: q.Mirror.Kind, AdmissionID: q.Scope.AdmissionID}
	evaluated, err := mirror.EvaluateSnapshot(mirror.Snapshot{Scope: selected, Coverage: c.Coverage}, mirror.Query{Scope: selected, Now: c.EvaluatedAt, MaxAge: time.Duration(q.Mirror.MaxAgeMS) * time.Millisecond})
	if err != nil {
		return "", invalid()
	}
	if source != nil && (!provenance(source) || source.ObservedAt.After(c.EvaluatedAt)) {
		return "", invalid()
	}
	if c.Coverage.LastComplete.IsZero() {
		if c.AgeMS != nil {
			return "", invalid()
		}
	} else {
		elapsed := c.EvaluatedAt.Sub(c.Coverage.LastComplete)
		if !c.Coverage.LastComplete.Add(elapsed).Equal(c.EvaluatedAt) {
			// time.Sub saturates outside its duration representation. Unknown is explicit.
			if c.AgeMS != nil {
				return "", invalid()
			}
		} else if c.AgeMS == nil || *c.AgeMS < 0 || *c.AgeMS != elapsed.Milliseconds() {
			return "", invalid()
		}
	}
	if evaluated.Available {
		return "fresh", nil
	}
	return "stale", nil
}

func payloadCount(r Response) int {
	n := 0
	for _, p := range []bool{r.Mirror != nil, r.Issues != nil, r.Source != nil, r.Effect != nil, r.Review != nil, r.Projection != nil} {
		if p {
			n++
		}
	}
	return n
}

// ValidateResponse validates wire claims and exact request binding, not current
// authorization, mirror freshness or the producer's classifier conclusions.
func ValidateResponse(r Response, q Request) error {
	if err := ValidateRequest(q); err != nil {
		return err
	}
	if r.Schema != Schema || r.Operation != q.Operation {
		return ErrSchema
	}
	if r.ID != q.ID || !SameScope(r.Scope, q.Scope) || r.Repository != q.Repository {
		return ErrScope
	}
	if !diagnostic(r.Diagnostic) || (r.Diagnostic != nil && (q.Diagnostic == nil || *r.Diagnostic != *q.Diagnostic)) || len(r.Measurements) > 3 {
		return invalid()
	}
	seen := map[string]bool{}
	for _, m := range r.Measurements {
		if m.Boundary != "server-handler" || m.Unit != "handler" || seen[m.Boundary] || (m.Count != nil && *m.Count < 0) {
			return invalid()
		}
		seen[m.Boundary] = true
	}
	m := r.Metadata
	switch m.Reason {
	case "", mirror.Permission, mirror.Transport, mirror.RateLimited, mirror.Truncated, mirror.Unknown:
	default:
		return ErrSchema
	}
	if m.Reason == mirror.Permission && m.State != Permission {
		return invalid()
	}
	if m.RetryAt != nil && (m.RetryAt.IsZero() || (m.State != Older && m.State != Unavailable) || q.Operation == CreateIssue || q.Operation == CreateComment) {
		return invalid()
	}
	cachedStatus := ""
	if m.Cached != nil {
		if m.State != Older && m.State != Unavailable {
			return invalid()
		}
		var err error
		cachedStatus, err = cachedFreshness(m.Cached, q, m.Source)
		if err != nil {
			return err
		}
	}
	switch m.State {
	case Permission:
		if payloadCount(r) != 0 || r.Conflict || m.Source != nil || m.Observation != nil || m.Executor != nil || m.Complete != nil || m.Freshness != "unknown" || m.RetryAt != nil || m.Cached != nil || r.Diagnostic != nil || len(r.Measurements) != 0 || m.Reason != mirror.Permission {
			return invalid()
		}
		return nil
	case Unavailable:
		if payloadCount(r) != 0 || r.Conflict || m.Observation != nil || m.Executor != nil || r.Diagnostic != nil || len(r.Measurements) != 0 {
			return invalid()
		}
		if m.Reason != mirror.Transport && m.Reason != mirror.RateLimited && m.Reason != mirror.Truncated && m.Reason != mirror.Unknown {
			return invalid()
		}
		if m.Cached == nil {
			if m.Source != nil || m.Complete != nil || m.Freshness != "unknown" {
				return invalid()
			}
		} else if m.Complete == nil || *m.Complete || m.Freshness != cachedStatus {
			return invalid()
		}
		return nil
	case Older:
		if payloadCount(r) != 0 || r.Conflict || m.Complete == nil || *m.Complete {
			return invalid()
		}
		if m.Cached == nil {
			if m.Freshness != "stale" || !provenance(m.Source) {
				return invalid()
			}
		} else if m.Freshness != cachedStatus {
			return invalid()
		}
	case Confirmed, Newer:
		if m.Complete == nil || !*m.Complete || m.Freshness != "fresh" || m.Reason != "" || !provenance(m.Source) {
			return invalid()
		}
	case ProjectionPending:
		if m.Complete == nil || *m.Complete || m.Freshness != "unknown" || r.Effect == nil || m.Reason != "" {
			return invalid()
		}
	default:
		return ErrSchema
	}
	if m.Observation != nil {
		kind := m.Observation.Kind
		if q.Mirror != nil {
			kind = q.Mirror.Kind
		}
		if !token(m.Observation, q.Scope, q.Repository, kind) {
			return ErrScope
		}
	}
	minimum := q.Minimum
	if q.Mirror != nil {
		minimum = q.Mirror.Minimum
	}
	if minimum != nil && (m.Observation == nil || (m.State == Confirmed && m.Observation.Value != minimum.Value)) {
		return ErrScope
	}
	if (m.State == Older || m.State == Newer) && (minimum == nil || m.Observation == nil || m.Observation.Value == minimum.Value) {
		return ErrScope
	}
	if m.State == Older {
		return nil
	}
	if r.Conflict {
		if r.Effect != nil || payloadCount(r) != 0 || (q.Operation != CreateIssue && q.Operation != CreateComment && q.Operation != LookupOperation) {
			return invalid()
		}
		return nil // A typed conflict is a valid transport result, never a receipt.
	}
	if q.Operation == ReadObservation {
		if payloadCount(r) != 0 || m.Observation == nil {
			return invalid()
		}
		return nil
	}
	if payloadCount(r) != 1 {
		return invalid()
	}
	switch q.Operation {
	case MirrorQuery:
		return validateMirror(r, q)
	case FileAt, Diff, MergeBase:
		return validateSource(r, q)
	case CreateIssue, CreateComment, LookupOperation:
		return validateEffect(r, q)
	case ReviewObservation:
		return validateReview(r, q)
	case IssueProjection:
		return validateProjection(r, q)
	}
	return ErrSchema
}

// presentWire checks presence in the transport DTO, never recalculates a domain
// conclusion. Scalar pointers make omitted/null distinct from explicit zero.
func presentWire(v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return invalid()
		}
		return presentWire(v.Elem())
	}
	if v.Type() == reflect.TypeFor[time.Time]() {
		return nil
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := presentWire(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if v.IsNil() || v.Len() > MaxItems {
			return invalid()
		}
		it := v.MapRange()
		for it.Next() {
			if err := presentWire(it.Value()); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if v.IsNil() || v.Len() > MaxItems {
			return invalid()
		}
		for i := 0; i < v.Len(); i++ {
			if err := presentWire(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.String:
		if !text(v.String(), MaxContentBytes, true) {
			return invalid()
		}
	}
	return nil
}

func Held(q Request, reason mirror.Reason) Response {
	state := Unavailable
	if reason == mirror.Permission {
		state = Permission
	}
	return Response{Schema: Schema, ID: q.ID, Scope: q.Scope, Repository: q.Repository, Operation: q.Operation, Metadata: Metadata{State: state, Reason: reason, Freshness: "unknown"}, Measurements: []Measurement{}}
}
