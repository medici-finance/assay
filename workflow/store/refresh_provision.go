package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"sort"
	"time"

	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/internal/refreshcodec"
)

// RefreshInitialUsage is an operator's explicitly known charged accounting,
// never a measured HTTP total. Provenance identifies that known baseline; the
// library neither authenticates it nor reconstructs it from absent records.
type RefreshInitialUsage struct {
	Known                  bool
	WindowAt               time.Time
	Fills, ChargedAttempts int64
	Provenance             string
}

type RefreshInitialSource struct {
	Usage      RefreshInitialUsage
	RetryKnown bool
	RetryAt    time.Time
}

type RefreshInitialCaller struct {
	ID    string
	Usage RefreshInitialUsage
}

// RefreshProvision names one admission and its shared source. Repeated source
// or admission identities must carry their original immutable baseline, even
// after runtime usage advances. A new caller never creates another source budget.
type RefreshProvision struct {
	Config    RefreshConfig
	Source    RefreshInitialSource
	Admission RefreshInitialUsage
	Callers   []RefreshInitialCaller
}

// RefreshProvisionReceipt binds exactly the selected immutable initialization
// records. It conveys neither caller authentication nor permission to send HTTP.
type RefreshProvisionReceipt struct {
	Digest  [32]byte
	Records int
}

type refreshInitialRecord struct {
	kind, id       string
	identity, data []byte
	usage          RefreshInitialUsage
}

func initialInstant(t time.Time) bool {
	return !t.IsZero() && t.Year() >= 1 && t.Year() <= 9999 && t.Nanosecond()%1000 == 0
}

func initialUsage(u RefreshInitialUsage, fills, attempts int64) bool {
	return u.Known && initialInstant(u.WindowAt) && u.Provenance != "" && u.Fills >= 0 && u.Fills <= fills && u.ChargedAttempts >= 0 && u.ChargedAttempts <= attempts
}

func prepareRefreshProvision(p RefreshProvision) ([]refreshInitialRecord, RefreshProvisionReceipt, error) {
	c := p.Config
	if !validRefreshConfig(c) || len(p.Callers) == 0 || !initialUsage(p.Source.Usage, 0, c.Native.SourcePerWindow) || !p.Source.RetryKnown || !p.Source.RetryAt.IsZero() && !initialInstant(p.Source.RetryAt) || !initialUsage(p.Admission, int64(c.Limits.AdmissionFillsPerWindow), c.Native.AdmissionPerWindow) {
		return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	profile, e := refreshBytes(c)
	if e != nil {
		return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	budget, e := refreshBytes(c.Native)
	if e != nil {
		return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	callers := append([]RefreshInitialCaller(nil), p.Callers...)
	sort.Slice(callers, func(i, j int) bool { return callers[i].ID < callers[j].ID })
	records := make([]refreshInitialRecord, 0, len(callers)+2)
	add := func(kind, id string, identity []string, encoded []byte, u RefreshInitialUsage, retryKnown bool, retryAt time.Time) error {
		fields := make([][]byte, len(identity))
		for i, field := range identity {
			fields[i] = []byte(field)
		}
		key, data, err := refreshcodec.Initial(refreshcodec.Initialization{Kind: kind, Identity: fields, Profile: encoded, Known: u.Known, WindowMicros: u.WindowAt.UnixMicro(), Fills: u.Fills, Attempts: u.ChargedAttempts, Provenance: []byte(u.Provenance), RetryKnown: retryKnown, HasRetry: !retryAt.IsZero(), RetryMicros: retryAt.UnixMicro()})
		if err != nil {
			return err
		}
		records = append(records, refreshInitialRecord{kind: kind, id: id, identity: key, data: data, usage: u})
		return nil
	}
	if add("source", c.SourceID, []string{c.SourceID}, budget, p.Source.Usage, p.Source.RetryKnown, p.Source.RetryAt) != nil || add("admission", c.AdmissionID, []string{c.AdmissionID}, profile, p.Admission, false, time.Time{}) != nil {
		return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	for i, caller := range callers {
		if caller.ID == "" || i > 0 && caller.ID == callers[i-1].ID || !initialUsage(caller.Usage, int64(c.Limits.CallerFillsPerWindow), c.Native.CallerPerWindow) || add("caller", caller.ID, []string{c.AdmissionID, caller.ID}, profile, caller.Usage, false, time.Time{}) != nil {
			return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
		}
	}
	encoded := make([][]byte, len(records))
	for i, record := range records {
		encoded[i] = record.data
	}
	set, e := refreshcodec.InitialSet(encoded)
	if e != nil {
		return nil, RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	return records, RefreshProvisionReceipt{Digest: sha256.Sum256(set), Records: len(records)}, nil
}

// ProvisionRefresh is an explicit operator operation over this already-open R2
// connection. It creates absent known records only. The declared runtime role
// cannot execute its INSERTs; constructors and runtime queries never call it.
func (d *DB) ProvisionRefresh(ctx context.Context, p RefreshProvision) (RefreshProvisionReceipt, error) {
	records, receipt, e := prepareRefreshProvision(p)
	if e != nil || d == nil || d.db == nil || ctx == nil {
		return RefreshProvisionReceipt{}, refresh.ErrPolicy
	}
	tx, e := d.db.BeginTx(ctx, nil)
	if e != nil {
		return RefreshProvisionReceipt{}, refresh.ErrHeld
	}
	defer tx.Rollback()
	// Operators serialize absent-row initialization by source, independently of
	// runtime lease ownership. Existing admission rows still precede budget locks.
	key := sha256.Sum256([]byte(p.Config.SourceID))
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(key[:8]))); e != nil {
		return RefreshProvisionReceipt{}, refresh.ErrHeld
	}
	var source []byte
	e = tx.QueryRowContext(ctx, `SELECT source_id FROM public.refresh_admissions WHERE admission=$1 FOR UPDATE`, []byte(p.Config.AdmissionID)).Scan(&source)
	if e != nil && !errors.Is(e, sql.ErrNoRows) || e == nil && string(source) != p.Config.SourceID {
		return RefreshProvisionReceipt{}, refresh.ErrHeld
	}
	var now time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return RefreshProvisionReceipt{}, refresh.ErrHeld
	}
	for _, record := range records {
		if e = provisionRefreshRecord(ctx, tx, p, record, now); e != nil {
			return RefreshProvisionReceipt{}, e
		}
	}
	if ctx.Err() != nil || tx.Commit() != nil {
		return RefreshProvisionReceipt{}, refresh.ErrHeld
	}
	return receipt, nil
}

func provisionRefreshRecord(ctx context.Context, tx *sql.Tx, p RefreshProvision, r refreshInitialRecord, now time.Time) error {
	h := sha256.Sum256(r.identity)
	var identity, data []byte
	receiptErr := tx.QueryRowContext(ctx, `SELECT identity,baseline FROM public.refresh_initializations WHERE kind=$1 AND identity_hash=$2`, r.kind, h[:]).Scan(&identity, &data)
	if receiptErr != nil && !errors.Is(receiptErr, sql.ErrNoRows) || receiptErr == nil && (!bytes.Equal(identity, r.identity) || !bytes.Equal(data, r.data)) {
		return refresh.ErrHeld
	}
	var known bool
	var profile []byte
	var rowErr error
	switch r.kind {
	case "source":
		rowErr = tx.QueryRowContext(ctx, `SELECT known,profile FROM public.refresh_budgets WHERE source_id=$1 FOR UPDATE`, []byte(r.id)).Scan(&known, &profile)
	case "admission":
		rowErr = tx.QueryRowContext(ctx, `SELECT known,profile FROM public.refresh_admissions WHERE admission=$1 FOR UPDATE`, []byte(r.id)).Scan(&known, &profile)
	case "caller":
		rowErr = tx.QueryRowContext(ctx, `SELECT known FROM public.refresh_callers WHERE admission=$1 AND caller_id=$2 FOR UPDATE`, []byte(p.Config.AdmissionID), []byte(r.id)).Scan(&known)
	default:
		return refresh.ErrPolicy
	}
	if rowErr == nil {
		if !known || receiptErr != nil {
			return refresh.ErrHeld
		}
		// Full initial profile/baseline equality is independently retained in data.
		// Current counters, holds and windows are intentionally never rewritten.
		return nil
	}
	if !errors.Is(rowErr, sql.ErrNoRows) || receiptErr == nil || now.Before(r.usage.WindowAt) || !now.Before(r.usage.WindowAt.Add(p.Config.Native.Window)) {
		return refresh.ErrHeld
	}
	if _, e := tx.ExecContext(ctx, `INSERT INTO public.refresh_initializations(kind,identity_hash,identity,baseline) VALUES($1,$2,$3,$4)`, r.kind, h[:], r.identity, r.data); e != nil {
		return refresh.ErrHeld
	}
	c, u := p.Config, r.usage
	var e error
	switch r.kind {
	case "source":
		budget, _ := refreshBytes(c.Native)
		var retry any
		if !p.Source.RetryAt.IsZero() {
			retry = p.Source.RetryAt
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit,retry_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, []byte(c.SourceID), budget, u.Known, u.WindowAt, int64(c.Native.Window), u.ChargedAttempts, c.Native.SourcePerWindow, c.Native.Concurrent, c.Native.Reservations, retry)
	case "admission":
		profile, _ := refreshBytes(c)
		_, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_admissions(admission,source_id,profile,known,window_at,window_ns,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit,failure_limit) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, []byte(c.AdmissionID), []byte(c.SourceID), profile, u.Known, u.WindowAt, int64(c.Limits.FillWindow), u.Fills, c.Limits.AdmissionFillsPerWindow, u.ChargedAttempts, c.Native.AdmissionPerWindow, c.Limits.AdmissionFills, c.Limits.AdmissionWaiters, c.Limits.FailureEntries)
	case "caller":
		_, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_callers(admission,caller_id,known,window_at,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, []byte(c.AdmissionID), []byte(r.id), u.Known, u.WindowAt, u.Fills, c.Limits.CallerFillsPerWindow, u.ChargedAttempts, c.Native.CallerPerWindow, c.Limits.ActiveFills, c.Limits.QueuedCallers)
	}
	if e != nil {
		return refresh.ErrHeld
	}
	return nil
}
