package service

import (
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
)

// MirrorPageSchema opts into bounded snapshot traversal using a separate wire
// envelope. A page never changes the complete-snapshot semantics of Schema.
const MirrorPageSchema = "forge-mirror-page-v1"

type MirrorPageCursor struct {
	Scope      Scope               `json:"scope"`
	Repository mirror.RepositoryID `json:"repository"`
	Kind       mirror.RecordKind   `json:"kind"`
	Version    string              `json:"version"`
	ObservedAt time.Time           `json:"observedAt"`
	After      int64               `json:"after"`
}

type MirrorPageRequest struct {
	Schema string            `json:"schema"`
	Query  Request           `json:"query"`
	Limit  int               `json:"limit"`
	Cursor *MirrorPageCursor `json:"cursor"`
}

type MirrorPage struct {
	Snapshot mirror.Snapshot   `json:"snapshot"`
	Next     *MirrorPageCursor `json:"next"`
}

type MirrorPageResponse struct {
	Schema     string              `json:"schema"`
	ID         string              `json:"id"`
	Scope      Scope               `json:"scope"`
	Repository mirror.RepositoryID `json:"repository"`
	Kind       mirror.RecordKind   `json:"kind"`
	Metadata   Metadata            `json:"metadata"`
	Page       *MirrorPage         `json:"page"`
	Restart    bool                `json:"restart"`
}

func EncodeMirrorPageRequest(q MirrorPageRequest) ([]byte, error) {
	if err := validatePageRequest(q); err != nil {
		return nil, err
	}
	return encode(q)
}

func DecodeMirrorPageRequest(data []byte) (MirrorPageRequest, error) {
	var q MirrorPageRequest
	if err := decode(data, &q); err != nil {
		return MirrorPageRequest{}, err
	}
	if err := validatePageRequest(q); err != nil {
		return MirrorPageRequest{}, err
	}
	return q, nil
}

func EncodeMirrorPageResponse(r MirrorPageResponse, q MirrorPageRequest) ([]byte, error) {
	if err := validatePageResponse(r, q); err != nil {
		return nil, err
	}
	return encode(r)
}

func DecodeMirrorPageResponse(data []byte, q MirrorPageRequest) (MirrorPageResponse, error) {
	var r MirrorPageResponse
	if err := decode(data, &r); err != nil {
		return MirrorPageResponse{}, err
	}
	if err := validatePageResponse(r, q); err != nil {
		return MirrorPageResponse{}, err
	}
	return r, nil
}

func validatePageRequest(r MirrorPageRequest) error {
	q := r.Query
	if r.Schema != MirrorPageSchema || q.Operation != MirrorQuery {
		return ErrSchema
	}
	if err := ValidateRequest(q); err != nil {
		return err
	}
	if len(q.Scope.Repositories) != 1 || q.Mirror.Minimum != nil || q.Diagnostic != nil || r.Limit < 1 || r.Limit > MaxItems {
		return ErrInvalid
	}
	if r.Cursor != nil && !pageCursor(r.Cursor, q) {
		return ErrScope
	}
	return nil
}

func pageCursor(c *MirrorPageCursor, q Request) bool {
	return c.After > 0 && !c.ObservedAt.IsZero() && text(c.Version, 256, false) && SameScope(c.Scope, q.Scope) &&
		c.Repository == q.Repository && c.Kind == q.Mirror.Kind
}

func validatePageResponse(r MirrorPageResponse, req MirrorPageRequest) error {
	if err := validatePageRequest(req); err != nil {
		return err
	}
	q := req.Query
	if r.Schema != MirrorPageSchema || r.ID != q.ID || !SameScope(r.Scope, q.Scope) ||
		r.Repository != q.Repository || r.Kind != q.Mirror.Kind {
		return ErrScope
	}
	if r.Metadata.Observation != nil || r.Metadata.Executor != nil {
		return ErrInvalid
	}
	legacy := Held(q, mirror.Unknown)
	legacy.Metadata = r.Metadata
	switch r.Metadata.State {
	case Permission, Unavailable:
		if r.Page != nil || (r.Restart && (r.Metadata.State != Unavailable || req.Cursor == nil || r.Metadata.Cached != nil || r.Metadata.RetryAt != nil)) {
			return ErrInvalid
		}
	case Confirmed:
		if r.Page == nil || r.Restart {
			return ErrInvalid
		}
		s := &r.Page.Snapshot
		if !text(s.Version, 256, false) || len(s.Records) > req.Limit || r.Metadata.Source == nil ||
			r.Metadata.Source.Revision != s.Version || !r.Metadata.Source.ObservedAt.Equal(s.Coverage.LastComplete) {
			return ErrInvalid
		}
		last := int64(0)
		if req.Cursor != nil {
			if s.Version != req.Cursor.Version || !s.Coverage.LastComplete.Equal(req.Cursor.ObservedAt) {
				return ErrConflict
			}
			last = req.Cursor.After
		}
		for _, row := range s.Records {
			if row.ID.Number <= last {
				return ErrInvalid
			}
			last = row.ID.Number
		}
		if c := r.Page.Next; c != nil && (len(s.Records) == 0 || !pageCursor(c, q) ||
			c.Version != s.Version || !c.ObservedAt.Equal(s.Coverage.LastComplete) || c.After != last) {
			return ErrScope
		}
		legacy.Mirror = s
	default:
		return ErrInvalid
	}
	return ValidateResponse(legacy, q)
}
