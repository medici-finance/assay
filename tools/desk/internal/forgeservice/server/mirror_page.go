package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
	"github.com/medici-finance/assay/workflow/store"
)

// HandlePage is the offline companion to Handle. Only a first page can join
// read-through work. Continuations use an exact stored version and keyset.
func (h *MirrorQueryHandler) HandlePage(r *http.Request, grant auth.SignedGrant) ([]byte, error) {
	body, err := readMirrorBody(r)
	if err != nil {
		return nil, err
	}
	request, err := service.DecodeMirrorPageRequest(body)
	if err != nil {
		return nil, err
	}
	q := request.Query
	scope := h.policy.Scope
	ctx, cancel := context.WithTimeout(r.Context(), h.policy.Caller.Limits.WaitTimeout)
	defer cancel()
	denied := func() ([]byte, error) {
		return encodePageHold(request, mirror.Permission, false)
	}
	grant, allowed := h.authorizeMirrorQuery(ctx, q, grant)
	if !allowed {
		return denied()
	}

	age := time.Duration(q.Mirror.MaxAgeMS) * time.Millisecond
	finish := func(data []byte, err error) ([]byte, error) {
		if err != nil {
			return nil, err
		}
		if !h.releaseMirrorQuery(ctx, grant) {
			return denied()
		}
		return data, nil
	}
	if age < h.policy.Caller.Limits.MinFreshness || age > h.policy.Caller.Limits.MaxFreshness {
		return finish(encodePageHold(request, mirror.Unknown, false))
	}
	var after int64
	var version string
	var observed time.Time
	if cursor := request.Cursor; cursor != nil {
		after, version, observed = cursor.After, cursor.Version, cursor.ObservedAt
		if !pageFresh(observed, age, time.Now()) {
			return finish(encodePageHold(request, mirror.Unknown, true))
		}
	} else {
		result, err := h.readMirrorQuery(ctx, grant, age)
		if deniedRead(result, err) {
			return denied()
		}
		if err != nil || !result.Fulfilled {
			meta := mirrorReply(q, result, false, time.Now()).Metadata
			return finish(service.EncodeMirrorPageResponse(pageReply(request, meta), request))
		}
		version, observed = result.Snapshot.Version, result.Snapshot.Coverage.LastComplete
	}
	page, err := h.mirror.ReadPage(ctx, h.admission, scope, version, observed, after,
		min(request.Limit, h.limits.MaxRecords), min(service.MaxMessageBytes, h.limits.MaxBytes))
	if errors.Is(err, mirror.ErrPermission) {
		return denied()
	}
	if err != nil {
		return finish(encodePageHold(request, mirror.Unknown, request.Cursor != nil && errors.Is(err, mirror.ErrConflict)))
	}
	data, err := encodeSnapshotPage(request, page)
	if err != nil {
		return finish(encodePageHold(request, mirror.Truncated, false))
	}
	if !h.releaseMirrorQuery(ctx, grant) {
		return denied()
	}
	if !pageFresh(observed, age, time.Now()) {
		return encodePageHold(request, mirror.Unknown, request.Cursor != nil)
	}
	return data, nil
}

func pageFresh(observed time.Time, age time.Duration, now time.Time) bool {
	return !observed.IsZero() && !observed.After(now) && now.Sub(observed) <= age
}

func pageReply(r service.MirrorPageRequest, metadata service.Metadata) service.MirrorPageResponse {
	q := r.Query
	return service.MirrorPageResponse{
		Schema: service.MirrorPageSchema, ID: q.ID, Scope: q.Scope, Repository: q.Repository,
		Kind: q.Mirror.Kind, Metadata: metadata,
	}
}

func encodePageHold(r service.MirrorPageRequest, reason mirror.Reason, restart bool) ([]byte, error) {
	reply := pageReply(r, service.Held(r.Query, reason).Metadata)
	reply.Restart = restart
	return service.EncodeMirrorPageResponse(reply, r)
}

func encodeSnapshotPage(r service.MirrorPageRequest, p store.MirrorPage) ([]byte, error) {
	complete := true
	observed := p.Snapshot.Coverage.LastComplete
	reply := pageReply(r, service.Metadata{
		State: service.Confirmed, Complete: &complete, Freshness: "fresh",
		Source: &service.Provenance{Source: PostgresSnapshot, Version: "1", Revision: p.Snapshot.Version, ObservedAt: observed},
	})
	records := p.Snapshot.Records
	encodePrefix := func(n int) ([]byte, error) {
		snapshot := p.Snapshot
		snapshot.Records = records[:n]
		reply.Page = &service.MirrorPage{Snapshot: snapshot}
		if n != len(records) || p.More {
			if n == 0 {
				return nil, service.ErrInvalid
			}
			reply.Page.Next = &service.MirrorPageCursor{
				Scope: r.Query.Scope, Repository: r.Query.Repository, Kind: r.Query.Mirror.Kind,
				Version: snapshot.Version, ObservedAt: observed, After: records[n-1].ID.Number,
			}
		}
		return service.EncodeMirrorPageResponse(reply, r)
	}
	// The full page may be smaller than a prefix carrying a cursor.
	if data, err := encodePrefix(len(records)); err == nil {
		return data, nil
	}
	var best []byte
	for lo, hi := 1, len(records)-1; lo <= hi; {
		n := lo + (hi-lo)/2
		data, err := encodePrefix(n)
		if err != nil {
			hi = n - 1
			continue
		}
		best = data
		lo = n + 1
	}
	if best == nil {
		return nil, service.ErrInvalid
	}
	return best, nil
}
