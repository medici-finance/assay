package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/medici-finance/assay/forgeread/mirror"
)

type MirrorLimits struct {
	MaxRecords int
	MaxBytes   int
}

// MirrorStore is scoped durable observation storage. Naked Apply has no refresh-
// generation fence: a future joined gate transaction is required before production
// coordinator use. It grants no retention, effect or execution authority.
type MirrorStore struct {
	database *DB
	limits   MirrorLimits
	// tx is supplied only by the canonical refresh completion owner.
	tx *sql.Tx
}

var _ mirror.Store = (*MirrorStore)(nil)

func (d *DB) Mirror(limits MirrorLimits) (*MirrorStore, error) {
	if d == nil || d.db == nil || limits.MaxRecords <= 0 || limits.MaxRecords == math.MaxInt || limits.MaxBytes <= 0 {
		return nil, mirror.ErrInvalid
	}
	return &MirrorStore{database: d, limits: limits}, nil
}
func admitMirror(ctx context.Context, a mirror.Admission, scope mirror.Scope) error {
	if a == nil || a.ID() == "" || a.ID() != scope.AdmissionID {
		return mirror.ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil || a.ID() != scope.AdmissionID {
		return mirror.ErrPermission
	}
	return nil
}
func scopeIndex(scope mirror.Scope, limit int) ([]byte, error) {
	fields := []string{string(scope.Repository.Provider), scope.Repository.Host, scope.Repository.Path, string(scope.Kind), scope.AdmissionID}
	hash := sha256.New()
	left := limit
	for _, field := range fields {
		if len(field) > left {
			return nil, mirror.ErrInvalid
		}
		left -= len(field)
		hash.Write(binary.BigEndian.AppendUint64(nil, uint64(len(field))))
		hash.Write([]byte(field))
	}
	return hash.Sum(nil), nil
}
func mirrorFailure(ctx context.Context, operation string, err error) error {
	if errors.Is(err, mirror.ErrInvalid) || errors.Is(err, mirror.ErrPermission) || errors.Is(err, mirror.ErrConflict) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if pg.Code == "40001" || pg.Code == "40P01" {
			return mirror.ErrConflict
		}
		if pg.Code == "P0001" || pg.Code == "P0002" || len(pg.Code) >= 2 && (pg.Code[:2] == "23" || pg.Code[:2] == "22") {
			return mirror.ErrInvalid
		}
	}
	return failure(ctx, operation, Unavailable)
}
func emptyMirror(scope mirror.Scope) mirror.Snapshot {
	return mirror.Snapshot{Scope: scope, Coverage: mirror.Coverage{State: mirror.NeverRead}, Records: []mirror.Observation{}}
}

const mirrorHeaderColumns = `provider,host,repository,kind,admission,version,state,reason,last_attempted::text,last_complete::text,last_inventory::text,observed_up_to::text,retained_from::text,validator_since::text,validator,revision,validator_full,complete_scan,full_inventory,retention_required`

type mirrorHeader struct {
	snapshot                  mirror.Snapshot
	complete, full, retention bool
}

func readMirrorHeader(ctx context.Context, tx *sql.Tx, key []byte, scope mirror.Scope, limit int, lock bool) (mirrorHeader, error) {
	sizeQuery := "SELECT octet_length(host)::bigint+octet_length(repository)+octet_length(admission)+octet_length(validator)+octet_length(revision)+octet_length(provider)+octet_length(kind) FROM public.mirror_scopes WHERE scope_key=$1"
	if lock {
		sizeQuery += " FOR UPDATE"
	}
	var size int64
	if err := tx.QueryRowContext(ctx, sizeQuery, key).Scan(&size); errors.Is(err, sql.ErrNoRows) {
		return mirrorHeader{snapshot: emptyMirror(scope)}, nil
	} else if err != nil {
		return mirrorHeader{}, err
	}
	if size > int64(limit) {
		return mirrorHeader{}, mirror.ErrInvalid
	}
	query := "SELECT " + mirrorHeaderColumns + " FROM public.mirror_scopes WHERE scope_key=$1"
	if lock {
		query += " FOR UPDATE"
	}
	h := mirrorHeader{snapshot: emptyMirror(scope)}
	var provider, kind string
	var host, repo, admission, validator, revision []byte
	var version int64
	stamps := make([]*string, 6)
	err := tx.QueryRowContext(ctx, query, key).Scan(&provider, &host, &repo, &kind, &admission, &version, &h.snapshot.Coverage.State, &h.snapshot.Coverage.Reason, &stamps[0], &stamps[1], &stamps[2], &stamps[3], &stamps[4], &stamps[5], &validator, &revision, &h.snapshot.Coverage.ValidatorFullInventory, &h.complete, &h.full, &h.retention)
	if errors.Is(err, sql.ErrNoRows) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	if provider != string(scope.Repository.Provider) || string(host) != scope.Repository.Host || string(repo) != scope.Repository.Path || kind != string(scope.Kind) || string(admission) != scope.AdmissionID {
		return h, mirror.ErrInvalid
	}
	c := &h.snapshot.Coverage
	destinations := []*time.Time{&c.LastAttempted, &c.LastComplete, &c.LastInventory, &c.ObservedUpTo, &c.RetainedFrom, &c.ValidatorSince}
	for i, raw := range stamps {
		value, e := parseStamp(raw)
		if e != nil {
			return h, mirror.ErrInvalid
		}
		*destinations[i] = value
	}
	c.Validator = string(validator)
	c.Revision = string(revision)
	if version > 0 {
		h.snapshot.Version = strconv.FormatInt(version, 10)
	} else if c.State != mirror.NeverRead {
		return h, mirror.ErrInvalid
	}
	return h, nil
}
func (s *MirrorStore) readMirrorRecords(ctx context.Context, tx *sql.Tx, key []byte, h *mirrorHeader) error {
	var count, total, max int64
	if err := tx.QueryRowContext(ctx, "SELECT count(*),coalesce(sum(octet_length(payload)),0),coalesce(max(octet_length(payload)),0) FROM public.mirror_records WHERE scope_key=$1", key).Scan(&count, &total, &max); err != nil {
		return err
	}
	budget := s.limits.MaxBytes
	for _, field := range []string{string(h.snapshot.Scope.Repository.Provider), h.snapshot.Scope.Repository.Host, h.snapshot.Scope.Repository.Path, string(h.snapshot.Scope.Kind), h.snapshot.Scope.AdmissionID, h.snapshot.Coverage.Validator, h.snapshot.Coverage.Revision} {
		if len(field) > budget {
			return mirror.ErrInvalid
		}
		budget -= len(field)
	}
	if count > int64(s.limits.MaxRecords) || total > int64(budget) || max > int64(budget) {
		return mirror.ErrInvalid
	}
	if len(h.snapshot.Coverage.Validator) > s.limits.MaxBytes || len(h.snapshot.Coverage.Revision) > s.limits.MaxBytes-len(h.snapshot.Coverage.Validator) {
		return mirror.ErrInvalid
	}
	rows, err := tx.QueryContext(ctx, "SELECT record_kind,number,payload,in_inventory,updated_at::text,observed_at::text FROM public.mirror_records WHERE scope_key=$1 ORDER BY record_kind,number LIMIT $2", key, s.limits.MaxRecords+1)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var number int64
		var payload []byte
		var inventory bool
		var updated, observed *string
		if err = rows.Scan(&kind, &number, &payload, &inventory, &updated, &observed); err != nil {
			return err
		}
		r, e := decodeFact(payload, s.limits.MaxBytes)
		if e != nil {
			return e
		}
		r.ID = mirror.RecordID{Repository: h.snapshot.Scope.Repository, Kind: kind, Number: number}
		r.InInventory = inventory
		updatedAt, e := parseStamp(updated)
		if e != nil || !r.UpdatedAt.Equal(updatedAt) {
			return mirror.ErrInvalid
		}
		observedAt, e := parseStamp(observed)
		if e != nil || !r.ObservedAt.Equal(observedAt) {
			return mirror.ErrInvalid
		}
		h.snapshot.Records = append(h.snapshot.Records, r)
		if len(h.snapshot.Records) > s.limits.MaxRecords {
			return mirror.ErrInvalid
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if h.snapshot.Coverage.State == mirror.NeverRead {
		if len(h.snapshot.Records) != 0 {
			return mirror.ErrInvalid
		}
		return nil
	}
	u := mirror.ScanUpdate{Scope: h.snapshot.Scope, Coverage: h.snapshot.Coverage, Upsert: h.snapshot.Records, CompleteScan: h.complete, FullInventory: h.full, RetentionDecisionRequired: h.retention}
	return u.Validate()
}
func (s *MirrorStore) Read(ctx context.Context, a mirror.Admission, scope mirror.Scope) (result mirror.Snapshot, err error) {
	if err := scope.Validate(); err != nil {
		return mirror.Snapshot{}, err
	}
	if err := admitMirror(ctx, a, scope); err != nil {
		return mirror.Snapshot{}, err
	}
	defer func() {
		if current := admitMirror(ctx, a, scope); current != nil {
			result = mirror.Snapshot{}
			err = current
		}
	}()
	key, err := scopeIndex(scope, s.limits.MaxBytes)
	if err != nil {
		return mirror.Snapshot{}, err
	}
	tx, owned, err := s.mirrorTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-read", err)
	}
	if owned {
		defer tx.Rollback()
	}
	h, err := readMirrorHeader(ctx, tx, key, scope, s.limits.MaxBytes, false)
	if err == nil {
		err = s.readMirrorRecords(ctx, tx, key, &h)
	}
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-read", err)
	}
	if owned {
		if err = tx.Commit(); err != nil {
			return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-read", err)
		}
	}
	return h.snapshot, nil
}

const mirrorCAS = `UPDATE public.mirror_scopes SET version=version+1,state=$3,reason=$4,last_attempted=$5,last_complete=$6,last_inventory=$7,observed_up_to=$8,retained_from=$9,validator_since=$10,validator=$11,revision=$12,validator_full=$13,complete_scan=$14,full_inventory=$15,retention_required=$16 WHERE scope_key=$1 AND version=$2 RETURNING version`

func (s *MirrorStore) Apply(ctx context.Context, a mirror.Admission, u mirror.ScanUpdate) (result mirror.Snapshot, err error) {
	if len(u.Upsert) > s.limits.MaxRecords || len(u.NotInInventory) > s.limits.MaxRecords {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if err := u.Validate(); err != nil {
		return mirror.Snapshot{}, err
	}
	if err := admitMirror(ctx, a, u.Scope); err != nil {
		return mirror.Snapshot{}, err
	}
	defer func() {
		if current := admitMirror(ctx, a, u.Scope); current != nil {
			result = mirror.Snapshot{}
			err = current
		}
	}()
	if !u.Coverage.RetainedFrom.IsZero() || u.RetentionDecisionRequired && (len(u.Upsert) != 0 || len(u.NotInInventory) != 0 || u.Coverage.State != mirror.Unavailable || u.Coverage.Reason != mirror.Permission) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	// Absence needs full inventory proof even when SQL would update no row.
	if len(u.NotInInventory) > 0 && (!u.CompleteScan || !u.FullInventory) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if len(u.Upsert) > s.limits.MaxRecords || len(u.NotInInventory) > s.limits.MaxRecords {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	for _, record := range u.Upsert {
		if !record.InInventory {
			return mirror.Snapshot{}, mirror.ErrInvalid
		}
	}
	expected := int64(0)
	if u.ExpectedVersion != "" {
		n, e := strconv.ParseInt(u.ExpectedVersion, 10, 64)
		if e != nil || n <= 0 || strconv.FormatInt(n, 10) != u.ExpectedVersion {
			return mirror.Snapshot{}, mirror.ErrConflict
		}
		expected = n
	}
	key, err := scopeIndex(u.Scope, s.limits.MaxBytes)
	if err != nil {
		return mirror.Snapshot{}, err
	}
	tx, owned, err := s.mirrorTx(ctx, nil)
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
	}
	if owned {
		defer tx.Rollback()
	}
	old, err := s.lockMirrorHeader(ctx, tx, key, u.Scope)
	if err == nil {
		err = s.readMirrorRecords(ctx, tx, key, &old)
	}
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
	}
	if err = admitMirror(ctx, a, u.Scope); err != nil {
		return mirror.Snapshot{}, err
	}
	// Bound the complete target before staging row writes, including replacements.
	payloads := make(map[mirror.RecordID][]byte, len(old.snapshot.Records)+len(u.Upsert))
	budget := s.limits.MaxBytes
	for _, field := range []string{string(u.Scope.Repository.Provider), u.Scope.Repository.Host, u.Scope.Repository.Path, string(u.Scope.Kind), u.Scope.AdmissionID, u.Coverage.Validator, u.Coverage.Revision} {
		if len(field) > budget {
			return mirror.Snapshot{}, mirror.ErrInvalid
		}
		budget -= len(field)
	}
	for _, r := range old.snapshot.Records {
		payload, e := encodeFact(r, s.limits.MaxBytes)
		if e != nil {
			return mirror.Snapshot{}, e
		}
		payloads[r.ID] = payload
	}
	for _, r := range u.Upsert {
		payload, e := encodeFact(r, s.limits.MaxBytes)
		if e != nil {
			return mirror.Snapshot{}, e
		}
		payloads[r.ID] = payload
	}
	if len(payloads) > s.limits.MaxRecords {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	for _, payload := range payloads {
		if len(payload) > budget {
			return mirror.Snapshot{}, mirror.ErrInvalid
		}
		budget -= len(payload)
	}
	// The SQL predicate is the CAS authority; no in-process version comparison or mutex.
	c := u.Coverage
	var version int64
	err = tx.QueryRowContext(ctx, mirrorCAS, key, expected, string(c.State), string(c.Reason), stamp(c.LastAttempted), stamp(c.LastComplete), stamp(c.LastInventory), stamp(c.ObservedUpTo), stamp(c.RetainedFrom), stamp(c.ValidatorSince), []byte(c.Validator), []byte(c.Revision), c.ValidatorFullInventory, u.CompleteScan, u.FullInventory, u.RetentionDecisionRequired).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return mirror.Snapshot{}, mirror.ErrConflict
	}
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
	}
	for _, r := range u.Upsert {
		payload := payloads[r.ID]
		_, err = tx.ExecContext(ctx, `INSERT INTO public.mirror_records(scope_key,record_kind,number,payload,in_inventory,updated_at,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(scope_key,record_kind,number) DO UPDATE SET payload=excluded.payload,in_inventory=excluded.in_inventory,updated_at=excluded.updated_at,observed_at=excluded.observed_at`, key, r.ID.Kind, r.ID.Number, payload, r.InInventory, stamp(r.UpdatedAt), stamp(r.ObservedAt))
		if err != nil {
			return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
		}
	}
	for _, id := range u.NotInInventory {
		if _, err = tx.ExecContext(ctx, "UPDATE public.mirror_records SET in_inventory=false WHERE scope_key=$1 AND record_kind=$2 AND number=$3", key, id.Kind, id.Number); err != nil {
			return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
		}
	}
	final, err := readMirrorHeader(ctx, tx, key, u.Scope, s.limits.MaxBytes, false)
	if err == nil {
		err = s.readMirrorRecords(ctx, tx, key, &final)
	}
	if err != nil {
		return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
	}
	if err = admitMirror(ctx, a, u.Scope); err != nil {
		return mirror.Snapshot{}, err
	}
	if owned {
		if err = tx.Commit(); err != nil {
			return mirror.Snapshot{}, mirrorFailure(ctx, "mirror-apply", err)
		}
	}
	return final.snapshot, nil
}

// mirrorTx never exports a SQL handle or accepts arbitrary transaction callbacks.
// The refresh gate alone creates a tx-bound clone; standalone Read/Apply retain
// their original transaction ownership and final admission checks.
func (s *MirrorStore) mirrorTx(ctx context.Context, options *sql.TxOptions) (*sql.Tx, bool, error) {
	if s.tx != nil {
		return s.tx, false, nil
	}
	tx, err := s.database.db.BeginTx(ctx, options)
	return tx, true, err
}

// lockMirrorHeader is the same canonical scope-row lock used by standalone Apply
// and joined refresh completion. It prevents a tx-bound read from mixing an old
// header with concurrently committed records under READ COMMITTED.
func (s *MirrorStore) lockMirrorHeader(ctx context.Context, tx *sql.Tx, key []byte, scope mirror.Scope) (mirrorHeader, error) {
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.mirror_scopes(scope_key,provider,host,repository,kind,admission) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", key, string(scope.Repository.Provider), []byte(scope.Repository.Host), []byte(scope.Repository.Path), string(scope.Kind), []byte(scope.AdmissionID)); err != nil {
		return mirrorHeader{}, err
	}
	return readMirrorHeader(ctx, tx, key, scope, s.limits.MaxBytes, true)
}
