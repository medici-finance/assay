package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
)

// MirrorPage is a bounded portion of one complete persisted snapshot.
type MirrorPage struct {
	Snapshot mirror.Snapshot
	More     bool
}

// ReadPage uses the mirror's scoped key index. It verifies the version and
// original observation time in the same snapshot as the returned records.
func (s *MirrorStore) ReadPage(ctx context.Context, a mirror.Admission, scope mirror.Scope, version string, observedAt time.Time, after int64, limit, maxBytes int) (result MirrorPage, err error) {
	if s == nil || s.database == nil || s.tx != nil || scope.Validate() != nil ||
		version == "" || observedAt.IsZero() || after < 0 || limit < 1 || limit > s.limits.MaxRecords ||
		maxBytes < 1 || maxBytes > s.limits.MaxBytes {
		return MirrorPage{}, mirror.ErrInvalid
	}
	if err := admitMirror(ctx, a, scope); err != nil {
		return MirrorPage{}, err
	}
	defer func() {
		if current := admitMirror(ctx, a, scope); current != nil {
			result, err = MirrorPage{}, current
		}
	}()
	key, err := scopeIndex(scope, s.limits.MaxBytes)
	if err != nil {
		return MirrorPage{}, err
	}
	tx, err := s.database.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
	}
	defer tx.Rollback()
	h, err := readMirrorHeader(ctx, tx, key, scope, s.limits.MaxBytes, false)
	if err != nil {
		return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
	}
	if h.snapshot.Coverage.Reason == mirror.Permission {
		return MirrorPage{}, mirror.ErrPermission
	}
	if h.snapshot.Version != version || !h.snapshot.Coverage.LastComplete.Equal(observedAt) ||
		h.snapshot.Coverage.State != mirror.Complete {
		return MirrorPage{}, mirror.ErrConflict
	}
	kind := "issue"
	if scope.Kind == mirror.Changes {
		kind = "pull-request"
		if scope.Repository.Provider == mirror.GitLab {
			kind = "merge-request"
		}
	}
	size, more, err := pagePayloadCount(ctx, tx, key, kind, after, limit, maxBytes)
	if err != nil {
		return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
	}
	h.snapshot.Records = []mirror.Observation{}
	if size != 0 {
		rows, err := tx.QueryContext(ctx, `
			SELECT record_kind,number,payload,in_inventory,updated_at::text,observed_at::text
			FROM public.mirror_records
			WHERE scope_key=$1 AND record_kind=$2 AND number>$3
			ORDER BY number LIMIT $4`, key, kind, after, size)
		if err != nil {
			return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
		}
		err = readMirrorRecordRows(rows, &h, size, maxBytes)
		rows.Close()
		if err != nil {
			return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
		}
		if len(h.snapshot.Records) != size {
			return MirrorPage{}, mirror.ErrInvalid
		}
	}
	if err := tx.Commit(); err != nil {
		return MirrorPage{}, mirrorFailure(ctx, "mirror-page", err)
	}
	return MirrorPage{Snapshot: h.snapshot, More: more}, nil
}

// Count a fitting prefix before fetching payloads. The extra row distinguishes
// the end of the snapshot from a page bounded by rows or raw bytes.
func pagePayloadCount(ctx context.Context, tx *sql.Tx, key []byte, kind string, after int64, limit, budget int) (int, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT octet_length(payload) FROM public.mirror_records
		WHERE scope_key=$1 AND record_kind=$2 AND number>$3
		ORDER BY number LIMIT $4`, key, kind, after, limit+1)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var bytes int
		if err := rows.Scan(&bytes); err != nil {
			return 0, false, err
		}
		if bytes < 1 || (count == 0 && bytes > budget) {
			return 0, false, mirror.ErrInvalid
		}
		if count == limit || bytes > budget {
			return count, true, nil
		}
		count++
		budget -= bytes
	}
	return count, false, rows.Err()
}
