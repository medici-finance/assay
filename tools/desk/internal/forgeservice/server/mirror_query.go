package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/forgeread/service"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
	"github.com/medici-finance/assay/workflow/store"
)

// PostgresSnapshot identifies the scoped, persisted snapshot at its original
// complete observation time. Its revision is not forge or progress evidence.
const PostgresSnapshot = "postgres-mirror-snapshot-v1"

// MirrorQueryConfig comes from trusted composition. The source and admission
// must already have the selected repository and read-only credential bound.
type MirrorQueryConfig struct {
	Connection   store.Config
	Roles        store.RefreshRoleProfile
	Profile      store.RefreshConfig
	Limits       store.MirrorLimits
	Provenance   string
	Verifier     *auth.Verifier
	Admission    mirror.Admission
	Source       *deskkit.MirrorReadSource
	MetadataOnly bool
}

type MirrorQueryHandler struct {
	profile      store.RefreshConfig
	verifier     *auth.Verifier
	admission    mirror.Admission
	source       *deskkit.MirrorReadSource
	metadataOnly bool
	policy       auth.GrantScope
	db           *store.DB
	mirror       *store.MirrorStore
	limits       store.MirrorLimits
	lifetime     context.Context
	stop         context.CancelFunc
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// OpenMirrorQuery is unqualified composition for an owned fixture. Callers must
// supply an actual verifier from the qualified producer before operational use.
// ctx must span the server lifetime because it also governs background fills.
func OpenMirrorQuery(ctx context.Context, c MirrorQueryConfig) (*MirrorQueryHandler, error) {
	if ctx == nil || ctx.Err() != nil || c.Verifier == nil || !presentAdmission(c.Admission) || c.Source == nil ||
		c.Provenance != PostgresSnapshot || c.Profile.Validate() != nil ||
		c.Limits.MaxRecords <= 0 || c.Limits.MaxBytes <= 0 {
		return nil, auth.ErrConfig
	}
	p := c.Verifier.Policy()
	f := c.Profile
	expected := refresh.Caller{ID: p.Caller.ID, AdmissionID: f.AdmissionID,
		AccessEpoch: f.AccessEpoch, ConfigEpoch: f.ConfigEpoch, PolicyClass: f.PolicyClass,
		PolicyEpoch: f.PolicyEpoch, Limits: f.Limits}
	if p.Caller.ID == "" || p.Scope.Validate() != nil || p.Caller != expected ||
		p.Scope.Repository != c.Source.RepositoryID() || p.Scope.AdmissionID != c.Admission.ID() ||
		p.Scope.AdmissionID != f.AdmissionID || p.SourceID != f.SourceID ||
		(c.MetadataOnly && (p.Scope.Kind != mirror.Changes || !c.Source.SupportsGitHubMetadata())) {
		return nil, auth.ErrConfig
	}
	openCtx, cancelOpen := context.WithTimeout(ctx, c.Connection.Timeout)
	defer cancelOpen()
	db, err := store.OpenRefreshRuntime(openCtx, c.Connection, c.Roles)
	if err != nil {
		return nil, err
	}
	observations, err := db.Mirror(c.Limits)
	if err != nil {
		_ = db.Close()
		return nil, auth.ErrConfig
	}
	lifetime, stop := context.WithCancel(ctx)
	return &MirrorQueryHandler{
		profile: c.Profile, verifier: c.Verifier, admission: c.Admission, source: c.Source, metadataOnly: c.MetadataOnly,
		policy: p, db: db, mirror: observations, limits: c.Limits, lifetime: lifetime, stop: stop,
	}, nil
}

func presentAdmission(a mirror.Admission) bool {
	if a == nil {
		return false
	}
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	}
	return true
}

func (h *MirrorQueryHandler) Close() error {
	h.stop()
	return h.db.Close()
}

// Handle returns one encoded mirror-query response. The enclosing transport
// owns the request deadline and carries the signed grant outside the query body.
func (h *MirrorQueryHandler) Handle(r *http.Request, grant auth.SignedGrant) ([]byte, error) {
	q, err := DecodeMirrorQuery(r)
	if err != nil {
		return nil, err
	}
	p := h.policy
	denied := func() ([]byte, error) {
		return service.EncodeResponse(service.Held(q, mirror.Permission), q)
	}
	ctx, cancel := context.WithTimeout(r.Context(), p.Caller.Limits.WaitTimeout)
	defer cancel()
	grant, allowed := h.authorizeMirrorQuery(ctx, q, grant)
	if !allowed {
		return denied()
	}
	maxAge := time.Duration(q.Mirror.MaxAgeMS) * time.Millisecond
	response := service.Held(q, mirror.Unknown)
	// Opaque minimum tokens do not establish progress for this owner.
	if q.Mirror.Minimum == nil && maxAge >= p.Caller.Limits.MinFreshness && maxAge <= p.Caller.Limits.MaxFreshness {
		result, err := h.readMirrorQuery(ctx, grant, maxAge)
		if deniedRead(result, err) {
			return denied()
		}
		response = mirrorReply(q, result, err == nil, time.Now())
	}
	data, err := service.EncodeResponse(response, q)
	if err != nil {
		// An over-size or unrepresentable snapshot must not be cut down to a
		// successful prefix. The codec owns the exact wire and byte ceilings.
		response = service.Held(q, mirror.Unknown)
		data, err = service.EncodeResponse(response, q)
		if err != nil {
			return nil, err
		}
	}
	if !h.releaseMirrorQuery(ctx, grant) {
		return denied()
	}
	if response.Mirror != nil && time.Since(response.Mirror.Coverage.LastComplete) > maxAge {
		return service.EncodeResponse(service.Held(q, mirror.Unknown), q)
	}
	return data, nil
}

func (h *MirrorQueryHandler) authorizeMirrorQuery(ctx context.Context, q service.Request, grant auth.SignedGrant) (auth.SignedGrant, bool) {
	p := h.policy
	if len(q.Scope.Repositories) != 1 || q.Repository != p.Scope.Repository ||
		q.Mirror.Kind != p.Scope.Kind || q.Scope.AdmissionID != p.Scope.AdmissionID ||
		q.Scope.AccessEpoch != p.Caller.AccessEpoch || q.Scope.CallerID != p.Caller.ID ||
		q.Scope.RepresentedActor != p.Actor || len(grant.Payload) > celllaunch.MaxRecordBytes ||
		len(grant.Signature) != ed25519.SignatureSize {
		return auth.SignedGrant{}, false
	}
	grant.Payload, grant.Signature = bytes.Clone(grant.Payload), bytes.Clone(grant.Signature)
	caller, err := h.verifier.Authorize(ctx, grant, p.Scope)
	if err != nil || caller.Caller() != p.Caller {
		return auth.SignedGrant{}, false
	}
	return grant, h.admission.Authorize(ctx, p.Scope.Repository) == nil && h.admission.ID() == p.Scope.AdmissionID
}

func (h *MirrorQueryHandler) readMirrorQuery(ctx context.Context, grant auth.SignedGrant, maxAge time.Duration) (refresh.Result, error) {
	p := h.policy
	authority, err := h.verifier.BindRefresh(grant)
	if err != nil {
		return refresh.Result{}, err
	}
	coordinator, err := h.mirrorCoordinator(authority)
	if err != nil {
		return refresh.Result{}, err
	}
	query := refresh.Query{Query: mirror.Query{Scope: p.Scope, MaxAge: maxAge}}
	return coordinator.Read(ctx, h.admission, query, p.Caller)
}

// WarmMetadata is an in-process entry point for the trusted background runner.
// It consumes the existing signed grant and exposes no snapshot data.
func (h *MirrorQueryHandler) WarmMetadata(ctx context.Context, grant auth.SignedGrant) (bool, error) {
	if h == nil || !h.metadataOnly {
		return false, auth.ErrConfig
	}
	authority, err := h.verifier.BindRefresh(grant)
	if err != nil {
		return false, err
	}
	coordinator, err := h.mirrorCoordinator(authority)
	if err != nil {
		return false, err
	}
	p := h.policy
	return coordinator.Warm(ctx, h.admission, mirror.Query{Scope: p.Scope, MaxAge: p.Caller.Limits.MaxFreshness}, p.Caller)
}

func (h *MirrorQueryHandler) mirrorCoordinator(authority refresh.CallerAuthority) (*refresh.Coordinator, error) {
	clock := wallClock{}
	gate, err := h.db.Refresh(h.mirror, h.profile, authority, clock)
	if err != nil {
		return nil, auth.ErrConfig
	}
	openSource := deskkit.NewMirrorIngestSource
	if h.metadataOnly {
		openSource = deskkit.NewGitHubMetadataMirrorIngestSource
	}
	p := h.policy
	source, err := openSource(h.source, p.Scope.AdmissionID, p.SourceID, clock.Now, gate)
	if err != nil {
		return nil, auth.ErrConfig
	}
	return refresh.New(h.lifetime, h.mirror, source, gate, clock, p.SourceID, authority)
}

func deniedRead(result refresh.Result, err error) bool {
	return result.Reason == mirror.Permission || result.Snapshot.Coverage.Reason == mirror.Permission ||
		errors.Is(err, mirror.ErrPermission) || errors.Is(err, auth.ErrHeld)
}

// Reacquire current proof after all data and coordination I/O and encoding.
func (h *MirrorQueryHandler) releaseMirrorQuery(ctx context.Context, grant auth.SignedGrant) bool {
	p := h.policy
	if err := h.admission.Authorize(ctx, p.Scope.Repository); err != nil || h.admission.ID() != p.Scope.AdmissionID {
		return false
	}
	final, err := h.verifier.Authorize(ctx, grant, p.Scope)
	return err == nil && final.Caller() == p.Caller && h.verifier.CheckRelease(ctx, final) == nil
}

func mirrorReply(q service.Request, result refresh.Result, success bool, now time.Time) service.Response {
	if result.Reason == mirror.Permission || result.Snapshot.Coverage.Reason == mirror.Permission {
		return service.Held(q, mirror.Permission)
	}
	r := service.Held(q, mirror.Unknown)
	switch result.Reason {
	case mirror.Transport, mirror.RateLimited, mirror.Truncated:
		r.Metadata.Reason = result.Reason
	}
	if !result.RetryAt.IsZero() {
		r.Metadata.RetryAt = &result.RetryAt
	}
	s := result.Snapshot
	if s.Version == "" {
		return r
	}
	query := mirror.Query{
		Scope: mirror.Scope{Repository: q.Repository, AdmissionID: q.Scope.AdmissionID, Kind: q.Mirror.Kind},
		Now:   now, MaxAge: time.Duration(q.Mirror.MaxAgeMS) * time.Millisecond,
	}
	evaluated, err := mirror.EvaluateSnapshot(s, query)
	if err != nil {
		return r
	}
	complete := success && result.Fulfilled && evaluated.Available && len(s.Records) <= service.MaxItems
	r.Metadata.Complete = &complete
	if evaluated.Available {
		r.Metadata.Freshness = "fresh"
	} else {
		r.Metadata.Freshness = "stale"
	}
	if complete {
		r.Mirror = &s
		r.Metadata.State, r.Metadata.Reason, r.Metadata.RetryAt = service.Confirmed, "", nil
		r.Metadata.Source = &service.Provenance{
			Source: PostgresSnapshot, Version: "1", Revision: s.Version, ObservedAt: s.Coverage.LastComplete,
		}
		return r
	}
	if len(s.Records) > service.MaxItems {
		r.Metadata.Reason = mirror.Truncated
	}
	r.Metadata.Cached = &service.CachedObservation{Coverage: s.Coverage, EvaluatedAt: now}
	if !s.Coverage.LastComplete.IsZero() {
		age := now.Sub(s.Coverage.LastComplete)
		if s.Coverage.LastComplete.Add(age).Equal(now) {
			ms := age.Milliseconds()
			r.Metadata.Cached.AgeMS = &ms
		}
	}
	return r
}
