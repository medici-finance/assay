package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

// NativeLimits bounds authorized base-transport entries, not provider points or
// scan calls. Values come from trusted composition; there are no runtime defaults.
type NativeLimits struct {
	CallerPerWindow, AdmissionPerWindow, SourcePerWindow int64
	PerFill, Concurrent, Reservations                    int64
	Window                                               time.Duration
}

// RefreshConfig binds a canonical source/admission/policy profile. An existing,
// explicitly initialized SQL profile must exactly match it. This API initializes
// neither policy nor usage and cannot reconstruct missing accounting.
type RefreshConfig struct {
	SourceID, AdmissionID, AccessEpoch, ConfigEpoch, PolicyClass, PolicyEpoch string
	Limits                                                                    refresh.Limits
	Native                                                                    NativeLimits
	PollInterval                                                              time.Duration
}

type RefreshGate struct {
	store                  *MirrorStore
	config                 RefreshConfig
	authority              refresh.CallerAuthority
	profile, budgetProfile []byte
}

var _ refresh.Gate = (*RefreshGate)(nil)
var _ mirror.AttemptAuthority = (*RefreshGate)(nil)

func randomRefreshID() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", refresh.ErrHeld
	}
	return hex.EncodeToString(b[:]), nil
}
func validRefreshConfig(c RefreshConfig) bool {
	l, n := c.Limits, c.Native
	return c.SourceID != "" && c.AdmissionID != "" && c.AccessEpoch != "" && c.ConfigEpoch != "" && c.PolicyClass != "" && c.PolicyEpoch != "" && l.CallerFillsPerWindow > 0 && l.AdmissionFillsPerWindow >= l.CallerFillsPerWindow && l.FillWindow > 0 && l.MinFreshness > 0 && l.MaxFreshness >= l.MinFreshness && l.ActiveFills > 0 && l.QueuedCallers > 0 && l.AdmissionFills >= l.ActiveFills && l.AdmissionWaiters >= l.QueuedCallers && l.FailureEntries > 0 && l.WaitTimeout > 0 && l.FillTimeout > 0 && l.Cooldown > 0 && l.FailureTTL >= l.Cooldown && l.Refresh.Overlap >= 0 && l.Refresh.FullInventoryInterval > 0 && l.Refresh.FailureBackoff >= 0 && n.CallerPerWindow > 0 && n.AdmissionPerWindow >= n.CallerPerWindow && n.SourcePerWindow >= n.AdmissionPerWindow && n.PerFill > 0 && n.Concurrent > 0 && n.Reservations >= n.Concurrent && n.Window > 0 && n.Window == l.FillWindow && c.PollInterval > 0 && c.PollInterval <= l.WaitTimeout
}
func (d *DB) Refresh(s *MirrorStore, c RefreshConfig, a refresh.CallerAuthority) (*RefreshGate, error) {
	if d == nil || d.db == nil || s == nil || s.database != d || s.tx != nil || a == nil || !validRefreshConfig(c) {
		return nil, refresh.ErrPolicy
	}
	profile, e := refreshBytes(c)
	if e != nil {
		return nil, e
	}
	bp, e := refreshBytes(c.Native)
	if e != nil {
		return nil, e
	}
	return &RefreshGate{store: s, config: c, authority: a, profile: profile, budgetProfile: bp}, nil
}
func (g *RefreshGate) caller(id string) refresh.Caller {
	c := g.config
	return refresh.Caller{ID: id, AdmissionID: c.AdmissionID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch, Limits: c.Limits}
}
func (g *RefreshGate) keyOK(k refresh.FillKey) bool {
	c := g.config
	return k.Scope.Validate() == nil && k.Scope.AdmissionID == c.AdmissionID && k.SourceID == c.SourceID && k.AccessEpoch == c.AccessEpoch && k.ConfigEpoch == c.ConfigEpoch && k.PolicyClass == c.PolicyClass && k.PolicyEpoch == c.PolicyEpoch
}
func fillIndex(k refresh.FillKey) ([]byte, []byte, error) {
	b, e := refreshBytes(k)
	h := sha256.Sum256(b)
	return h[:], b, e
}

// Lock order for every mutation: admission anchor, source budget, caller usage,
// fill, waiter/attempt, mirror scope/records. Identity-only lookups before the
// anchor acquire no row locks and are rechecked under it. Native HTTP happens
// after commit and holds no SQL locks or pool connection.
func (g *RefreshGate) start(ctx context.Context) (*sql.Tx, time.Time, error) {
	tx, e := g.store.database.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, time.Time{}, refresh.ErrHeld
	}
	fail := func() (*sql.Tx, time.Time, error) { tx.Rollback(); return nil, time.Time{}, refresh.ErrHeld }
	var p, bp, source []byte
	var known, bknown bool
	var now time.Time
	if tx.QueryRowContext(ctx, `SELECT profile,source_id,known FROM public.refresh_admissions WHERE admission=$1 FOR UPDATE`, []byte(g.config.AdmissionID)).Scan(&p, &source, &known) != nil || !known || !bytes.Equal(p, g.profile) || string(source) != g.config.SourceID {
		return fail()
	}
	if tx.QueryRowContext(ctx, `SELECT profile,known FROM public.refresh_budgets WHERE source_id=$1 FOR UPDATE`, source).Scan(&bp, &bknown) != nil || !bknown || !bytes.Equal(bp, g.budgetProfile) {
		return fail()
	}
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return fail()
	}
	return tx, now, nil
}
func (g *RefreshGate) current(ctx context.Context, id string, scope mirror.Scope) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	return g.authority.Authorize(ctx, g.caller(id), scope)
}
func (g *RefreshGate) lockCaller(ctx context.Context, tx *sql.Tx, id string) error {
	var known bool
	var f, h, active, waiters int64
	e := tx.QueryRowContext(ctx, `SELECT known,fill_limit,http_limit,active_limit,waiter_limit FROM public.refresh_callers WHERE admission=$1 AND caller_id=$2 FOR UPDATE`, []byte(g.config.AdmissionID), []byte(id)).Scan(&known, &f, &h, &active, &waiters)
	if e != nil || !known || f != int64(g.config.Limits.CallerFillsPerWindow) || h != g.config.Native.CallerPerWindow || active != int64(g.config.Limits.ActiveFills) || waiters != int64(g.config.Limits.QueuedCallers) {
		return refresh.ErrHeld
	}
	return nil
}
func (g *RefreshGate) budgetHeld(ctx context.Context, tx *sql.Tx, now time.Time) (bool, *time.Time, error) {
	var held, uncertain bool
	var retry *time.Time
	e := tx.QueryRowContext(ctx, `SELECT unknown_hold,retry_at,EXISTS(SELECT 1 FROM public.refresh_attempts WHERE source_id=$1 AND (unknown_hold OR NOT observed AND expires_at<=$2)) FROM public.refresh_budgets WHERE source_id=$1`, []byte(g.config.SourceID), now).Scan(&held, &retry, &uncertain)
	return held || uncertain || (retry != nil && now.Before(*retry)), retry, e
}
func (g *RefreshGate) rollWindow(ctx context.Context, tx *sql.Tx, now time.Time, id string) error {
	// All three usage owners roll only with zero unresolved observations. SQL
	// triggers independently refuse early/backwards/reset-under-uncertainty writes.
	var unresolved bool
	if e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM public.refresh_attempts WHERE source_id=$1 AND (NOT observed OR unknown_hold))`, []byte(g.config.SourceID)).Scan(&unresolved); e != nil {
		return refresh.ErrHeld
	}
	if unresolved {
		return nil
	}
	args := []any{now, []byte(g.config.SourceID), now.Add(-g.config.Native.Window)}
	if _, e := tx.ExecContext(ctx, `UPDATE public.refresh_budgets SET window_at=$1,http_used=0 WHERE source_id=$2 AND window_at<=$3`, args...); e != nil {
		return refresh.ErrHeld
	}
	if _, e := tx.ExecContext(ctx, `UPDATE public.refresh_admissions SET window_at=$1,fills_used=0,http_used=0 WHERE admission=$2 AND window_at<=$3`, now, []byte(g.config.AdmissionID), now.Add(-g.config.Limits.FillWindow)); e != nil {
		return refresh.ErrHeld
	}
	if _, e := tx.ExecContext(ctx, `UPDATE public.refresh_callers SET window_at=$1,fills_used=0,http_used=0 WHERE admission=$2 AND caller_id=$3 AND window_at<=$4`, now, []byte(g.config.AdmissionID), []byte(id), now.Add(-g.config.Limits.FillWindow)); e != nil {
		return refresh.ErrHeld
	}
	return nil
}

type refreshFill struct {
	reviewOnlyPartial                       bool
	attemptHeld                             bool
	key, identity, scope, generation, owner []byte
	state                                   string
	expires                                 time.Time
	reason                                  mirror.Reason
	retry, base, reviews                    *time.Time
	version, attempts                       int64
}

func readRefreshFill(ctx context.Context, tx *sql.Tx, key []byte) (f refreshFill, e error) {
	f.key = key
	e = tx.QueryRowContext(ctx, `SELECT identity,scope_key,generation,owner_id,state,expires_at,reason,retry_at,base_retry_at,review_retry_at,snapshot_version,attempts_used,attempt_held,review_only_partial FROM public.refresh_fills WHERE fill_key=$1 FOR UPDATE`, key).Scan(&f.identity, &f.scope, &f.generation, &f.owner, &f.state, &f.expires, &f.reason, &f.retry, &f.base, &f.reviews, &f.version, &f.attempts, &f.attemptHeld, &f.reviewOnlyPartial)
	return
}
func (g *RefreshGate) grant(f refreshFill, k refresh.FillKey) mirror.AttemptGrant {
	return mirror.AttemptGrant{ID: string(f.generation), Generation: string(f.generation), SourceID: k.SourceID, Scope: k.Scope, CallerID: string(f.owner), AccessEpoch: k.AccessEpoch, ConfigEpoch: k.ConfigEpoch, PolicyClass: k.PolicyClass, PolicyEpoch: k.PolicyEpoch, MaxHTTPAttempts: g.config.Native.PerFill, ExpiresAt: f.expires}
}
func (g *RefreshGate) Acquire(ctx context.Context, d refresh.Demand) (refresh.Ticket, error) {
	if g == nil || ctx == nil {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.WaitTimeout)
	defer cancel()
	if !g.keyOK(d.Key) || d.CallerID == "" || d.Limits != g.config.Limits || d.Requirements.RequireReviews && d.Key.Scope.Kind != mirror.Changes {
		return refresh.Ticket{}, refresh.ErrPolicy
	}
	if e := g.current(ctx, d.CallerID, d.Key.Scope); e != nil {
		return refresh.Ticket{}, e
	}
	key, identity, e := fillIndex(d.Key)
	if e != nil {
		return refresh.Ticket{}, e
	}
	scope, e := scopeIndex(d.Key.Scope, g.store.limits.MaxBytes)
	if e != nil {
		return refresh.Ticket{}, e
	}
	tx, now, e := g.start(ctx)
	if e != nil {
		return refresh.Ticket{}, e
	}
	defer tx.Rollback()
	if e = g.lockCaller(ctx, tx, d.CallerID); e != nil {
		return refresh.Ticket{}, e
	}
	if e = g.rollWindow(ctx, tx, now, d.CallerID); e != nil {
		return refresh.Ticket{}, e
	}
	held, _, e := g.budgetHeld(ctx, tx, now)
	if e != nil || held {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	f, e := readRefreshFill(ctx, tx, key)
	fresh := errors.Is(e, sql.ErrNoRows)
	if e != nil && !fresh {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if !fresh && d.Requirements.RequireReviews && f.reviews != nil && now.Before(*f.reviews) {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if !fresh && !bytes.Equal(f.identity, identity) {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	var ownwait, allwait int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE caller_id=$3),count(*) FROM public.refresh_waiters WHERE admission=$1 AND NOT released AND expires_at>$2`, []byte(g.config.AdmissionID), now, []byte(d.CallerID)).Scan(&ownwait, &allwait); e != nil {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if ownwait >= g.config.Limits.QueuedCallers || allwait >= g.config.Limits.AdmissionWaiters {
		return refresh.Ticket{}, refresh.ErrQuota
	}
	owner := fresh || f.state == "pending" && !now.Before(f.expires) || f.state != "pending" && !(f.state == "held" || f.retry != nil && now.Before(*f.retry) || f.base != nil && now.Before(*f.base) || d.Requirements.RequireReviews && f.reviews != nil && now.Before(*f.reviews))
	if !fresh && (f.state == "held" || f.state != "pending" && !owner) {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if owner {
		// Only expired terminal entries with no live waiter or unresolved individual
		// reservation are reclaimable. Source retry/unknown state remains on its
		// separate non-evictable budget row.
		if _, e = tx.ExecContext(ctx, `DELETE FROM public.refresh_fills f WHERE admission=$1 AND state='done' AND retire_at<=clock_timestamp() AND COALESCE(retry_at,'-infinity')<=clock_timestamp() AND COALESCE(base_retry_at,'-infinity')<=clock_timestamp() AND COALESCE(review_retry_at,'-infinity')<=clock_timestamp() AND NOT EXISTS(SELECT 1 FROM public.refresh_waiters w WHERE w.fill_key=f.fill_key AND NOT released AND expires_at>clock_timestamp()) AND NOT EXISTS(SELECT 1 FROM public.refresh_attempts a WHERE a.fill_key=f.fill_key AND (NOT observed OR unknown_hold)) AND fill_key<>$2`, []byte(g.config.AdmissionID), key); e != nil {
			return refresh.Ticket{}, refresh.ErrHeld
		}
		var ownactive, active, retained int
		if tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE owner_id=$3 AND state='pending' AND expires_at>$2),count(*) FILTER(WHERE state='pending' AND expires_at>$2),count(*) FROM public.refresh_fills WHERE admission=$1`, []byte(g.config.AdmissionID), now, []byte(d.CallerID)).Scan(&ownactive, &active, &retained) != nil {
			return refresh.Ticket{}, refresh.ErrHeld
		}
		if ownactive >= g.config.Limits.ActiveFills || active >= g.config.Limits.AdmissionFills {
			return refresh.Ticket{}, refresh.ErrQuota
		}
		// Retained failure capacity is a hard bound, never an eviction into allowance.
		if fresh && retained >= g.config.Limits.FailureEntries {
			return refresh.Ticket{}, refresh.ErrHeld
		}
		for _, u := range []struct {
			query string
			args  []any
		}{
			{`UPDATE public.refresh_callers SET fills_used=fills_used+1 WHERE admission=$1 AND caller_id=$2 AND known AND fills_used<fill_limit`, []any{[]byte(g.config.AdmissionID), []byte(d.CallerID)}},
			{`UPDATE public.refresh_admissions SET fills_used=fills_used+1 WHERE admission=$1 AND known AND fills_used<fill_limit`, []any{[]byte(g.config.AdmissionID)}},
		} {
			r, err := tx.ExecContext(ctx, u.query, u.args...)
			if err != nil {
				return refresh.Ticket{}, refresh.ErrHeld
			}
			n, _ := r.RowsAffected()
			if n != 1 {
				return refresh.Ticket{}, refresh.ErrQuota
			}
		}

		generation, e := randomRefreshID()
		if e != nil {
			return refresh.Ticket{}, e
		}
		expires := now.Add(g.config.Limits.FillTimeout)
		if fresh {
			_, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_fills(fill_key,admission,identity,scope_key,generation,owner_id,expires_at,state,attempt_limit,retire_at,owner_reviews) VALUES($1,$2,$3,$4,$5,$6,$7,'pending',$8,$9,$10)`, key, []byte(g.config.AdmissionID), identity, scope, []byte(generation), []byte(d.CallerID), expires, g.config.Native.PerFill, expires.Add(g.config.Limits.FailureTTL), d.Requirements.RequireReviews)
		} else {
			_, e = tx.ExecContext(ctx, `UPDATE public.refresh_fills SET generation=$2,owner_id=$3,expires_at=$4,state='pending',reason='',retry_at=NULL,attempt_held=false,review_only_partial=false,owner_reviews=$5,attempts_used=0,completion_xid=NULL WHERE fill_key=$1`, key, []byte(generation), []byte(d.CallerID), expires, d.Requirements.RequireReviews)
		}
		if e != nil {
			return refresh.Ticket{}, refresh.ErrHeld
		}
		f, e = readRefreshFill(ctx, tx, key)
		if e != nil {
			return refresh.Ticket{}, refresh.ErrHeld
		}
	}
	id, e := randomRefreshID()
	if e != nil {
		return refresh.Ticket{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_waiters(ticket_id,admission,fill_key,generation,caller_id,reviews,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, []byte(id), []byte(g.config.AdmissionID), key, f.generation, []byte(d.CallerID), d.Requirements.RequireReviews, now.Add(g.config.Limits.WaitTimeout)); e != nil {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if e = g.current(ctx, d.CallerID, d.Key.Scope); e != nil {
		return refresh.Ticket{}, e
	}
	if e = tx.Commit(); e != nil {
		return refresh.Ticket{}, refresh.ErrHeld
	}
	if e = g.current(ctx, d.CallerID, d.Key.Scope); e != nil {
		return refresh.Ticket{}, e
	}
	return refresh.Ticket{ID: id, Generation: string(f.generation), Owner: owner, Joined: !owner, AttemptGrant: g.grant(f, d.Key)}, nil
}
func (g *RefreshGate) ticket(ctx context.Context, tx *sql.Tx, t refresh.Ticket) (refreshFill, refresh.FillKey, string, bool, error) {
	var key, generation, caller []byte
	var reviews, released bool
	e := tx.QueryRowContext(ctx, `SELECT fill_key,generation,caller_id,reviews,released FROM public.refresh_waiters WHERE ticket_id=$1 AND admission=$2`, []byte(t.ID), []byte(g.config.AdmissionID)).Scan(&key, &generation, &caller, &reviews, &released)
	if e != nil || string(generation) != t.Generation || released {
		return refreshFill{}, refresh.FillKey{}, "", false, refresh.ErrStale
	}
	f, e := readRefreshFill(ctx, tx, key)
	if e != nil {
		return f, refresh.FillKey{}, "", false, refresh.ErrHeld
	}
	var k refresh.FillKey
	if refreshDecode(f.identity, &k) != nil || !g.keyOK(k) || string(f.generation) != t.Generation {
		return f, k, "", false, refresh.ErrStale
	}
	return f, k, string(caller), reviews, nil
}
func refreshState(f refreshFill, now time.Time, reviews bool) refresh.State {
	retry := f.retry
	if f.base != nil && (retry == nil || retry.Before(*f.base)) {
		retry = f.base
	}
	if reviews && f.reviews != nil && (retry == nil || retry.Before(*f.reviews)) {
		retry = f.reviews
	}
	s := refresh.State{Pending: f.state == "pending", AttemptHeld: f.attemptHeld || f.state == "held", Reason: f.reason}
	if retry != nil && now.Before(*retry) {
		s.RetryAt = *retry
	}
	return s
}
func (g *RefreshGate) Wait(ctx context.Context, t refresh.Ticket) (refresh.State, error) {
	if g == nil || ctx == nil {
		return refresh.State{}, refresh.ErrHeld
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.WaitTimeout)
	defer cancel()
	timer := time.NewTicker(g.config.PollInterval)
	defer timer.Stop()
	for {
		tx, now, e := g.start(ctx)
		if e != nil {
			return refresh.State{}, e
		}
		f, _, _, reviews, e := g.ticket(ctx, tx, t)
		tx.Rollback()
		if e != nil {
			return refresh.State{}, e
		}
		state := refreshState(f, now, reviews)
		if !state.Pending {
			return state, nil
		}
		if !now.Before(f.expires) {
			return refresh.State{}, refresh.ErrStale
		}
		select {
		case <-ctx.Done():
			return refresh.State{}, ctx.Err()
		case <-timer.C:
		}
	}
}
func (g *RefreshGate) Complete(ctx context.Context, t refresh.Ticket, s mirror.Store, a mirror.Admission, c refresh.Completion) error {
	if g == nil || ctx == nil {
		return refresh.ErrHeld
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.FillTimeout)
	defer cancel()
	if s != g.store || !t.Owner {
		return refresh.ErrStale
	}
	tx, now, e := g.start(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = g.lockCaller(ctx, tx, c.Caller.ID); e != nil {
		return e
	}
	f, k, caller, ownerReviews, e := g.ticket(ctx, tx, t)
	if e != nil {
		return e
	}
	if caller != string(f.owner) || caller != c.Caller.ID || c.Caller != g.caller(caller) || c.Query.Scope != k.Scope || c.Query.RequireReviews != ownerReviews || f.state != "pending" || !now.Before(f.expires) {
		return refresh.ErrStale
	}
	// A denied caller may close its ownership without reading/applying rows or
	// manufacturing a shared source failure for another eligible caller.
	if !c.AdmissionDenied {
		if e = g.current(ctx, caller, k.Scope); e != nil {
			return e
		}
		if e = admitMirror(ctx, a, k.Scope); e != nil {
			return e
		}
	}
	fence, e := tx.ExecContext(ctx, `UPDATE public.refresh_fills SET completion_xid=pg_current_xact_id() WHERE fill_key=$1 AND generation=$2 AND state='pending' AND expires_at>clock_timestamp()`, f.key, f.generation)
	if e != nil {
		return refresh.ErrHeld
	}
	fenced, _ := fence.RowsAffected()
	if fenced != 1 {
		return refresh.ErrStale
	}

	var count, observed, unknown, incomplete, measuredHTTP int64
	if tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE observed),count(*) FILTER(WHERE unknown_hold),count(*) FILTER(WHERE NOT response_complete OR status<200 OR status>=300),coalesce(sum(http_count) FILTER(WHERE observed AND http_known),0) FROM public.refresh_attempts WHERE fill_key=$1 AND generation=$2`, f.key, f.generation).Scan(&count, &observed, &unknown, &incomplete, &measuredHTTP) != nil {
		return refresh.ErrHeld
	}
	receipt := c.Attempts
	held := receipt.Held != "" || count != observed || unknown > 0
	if c.Update != nil && (measuredHTTP == 0 || c.Update.CompleteScan && incomplete > 0) {
		held = true
	}
	// Successful native data carries its qualified pre-observer completion bound.
	// It is data age only; canonical lease/window/floor time remains the DB clock.
	if c.Update != nil && c.Failure == "" && (c.Update.CompleteScan || len(c.Update.Upsert) > 0) {
		bound := receipt.ObservedAt
		if bound.IsZero() || bound.After(now) {
			held = true
		} else {
			if c.Update.CompleteScan && (c.Update.Coverage.LastComplete.After(bound) || c.Update.Coverage.LastInventory.After(bound)) {
				held = true
			}
			for _, record := range c.Update.Upsert {
				if record.ObservedAt.After(bound) {
					held = true
				}
			}
		}
	}

	if c.Update != nil || count > 0 || c.Failure != "" {
		held = held || !receipt.Grant.Equal(g.grant(f, k)) || !receipt.HTTP.Known || receipt.HTTP.Value != measuredHTTP || receipt.Retry.Knowledge == mirror.RetryUnknown
	}
	if c.AdmissionDenied {
		held = true
		c.Update = nil
	}
	if held {
		c.Update = nil
	}
	reviewOnlyPartial := c.ReviewOnlyPartial()
	c.Authority = g.authority
	c.Now = now
	c.Query.RequireReviews = false
	state := "done"
	reason := c.Failure
	var retry any
	if held {
		if !c.AdmissionDenied && receipt.Held != mirror.AttemptCallerHeld && receipt.Held != mirror.AttemptAdmissionHeld {
			state = "held"
		}
		reason = ""
	} else {
		bound := &MirrorStore{database: g.store.database, limits: g.store.limits, tx: tx}
		header, err := bound.lockMirrorHeader(ctx, tx, f.scope, k.Scope)
		if err != nil {
			return mirrorFailure(ctx, "refresh-lock", err)
		}
		baseline := int64(0)
		if header.snapshot.Version != "" {
			baseline, err = strconv.ParseInt(header.snapshot.Version, 10, 64)
			if err != nil {
				return refresh.ErrHeld
			}
		}
		// Bind the actual pre-Apply version while holding the canonical scope row.
		// No-update cache completion may observe a version committed before this fill;
		// a changed version after this point must carry this transaction's write_xid.
		fence, err := tx.ExecContext(ctx, `UPDATE public.refresh_fills SET snapshot_version=$3 WHERE fill_key=$1 AND generation=$2 AND state='pending' AND expires_at>clock_timestamp()`, f.key, f.generation, baseline)
		if err != nil {
			return refresh.ErrHeld
		}
		n, _ := fence.RowsAffected()
		if n != 1 {
			return refresh.ErrStale
		}
		_, err = refresh.ApplyCompletion(ctx, bound, a, c)
		if err != nil {
			return err
		}
		if c.Failure != "" {
			retryAt, err := g.retryTime(ctx, tx, now)
			if err != nil {
				return err
			}
			retry = retryAt
		}
	}
	var version int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT version FROM public.mirror_scopes WHERE scope_key=$1),0)`, f.scope).Scan(&version)
	if e != nil {
		return refresh.ErrHeld
	}
	// The expiry/current-generation predicate is deliberately in SQL immediately
	// before publication, not only in an upper cooperative context check.
	r, e := tx.ExecContext(ctx, `UPDATE public.refresh_fills SET state=$3,reason=$4,retry_at=$5,snapshot_version=$6,completion_xid=pg_current_xact_id(),attempt_held=$7,retire_at=$8,review_only_partial=$9 WHERE fill_key=$1 AND generation=$2 AND state='pending' AND expires_at>clock_timestamp()`, f.key, f.generation, state, string(reason), retry, version, held, now.Add(g.config.Limits.FailureTTL), reviewOnlyPartial && !held)
	if e != nil {
		return refresh.ErrHeld
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return refresh.ErrStale
	}
	if e = g.current(ctx, caller, k.Scope); e != nil {
		return e
	}
	if e = admitMirror(ctx, a, k.Scope); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return refresh.ErrHeld
	}
	if e = g.current(ctx, caller, k.Scope); e != nil {
		return e
	}
	return admitMirror(ctx, a, k.Scope)
}
func (g *RefreshGate) Leave(ctx context.Context, t refresh.Ticket, o refresh.Outcome) (refresh.State, error) {
	if g == nil || ctx == nil {
		return refresh.State{}, refresh.ErrHeld
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.WaitTimeout)
	defer cancel()
	tx, now, e := g.start(ctx)
	if e != nil {
		return refresh.State{}, e
	}
	defer tx.Rollback()
	f, k, caller, reviews, e := g.ticket(ctx, tx, t)
	if errors.Is(e, refresh.ErrStale) {
		return refresh.State{}, nil
	}
	if e != nil {
		return refresh.State{}, e
	}
	checked := o.Checked && g.current(ctx, caller, k.Scope) == nil && ctx.Err() == nil && f.state == "done" && !f.attemptHeld && f.reason == ""
	var version int64
	if checked {
		header, err := readMirrorHeader(ctx, tx, f.scope, k.Scope, g.store.limits.MaxBytes, true)
		if err != nil {
			return refresh.State{}, refresh.ErrHeld
		}
		if header.snapshot.Version != "" {
			version, err = strconv.ParseInt(header.snapshot.Version, 10, 64)
			if err != nil {
				return refresh.State{}, refresh.ErrHeld
			}
		}
	}
	if checked && version > 0 && o.SnapshotVersion == strconv.FormatInt(version, 10) && version == f.version {
		column := "base_retry_at"
		if reviews {
			column = "review_retry_at"
		}
		if o.Fulfilled {
			_, e = tx.ExecContext(ctx, `UPDATE public.refresh_fills SET `+column+`=NULL WHERE fill_key=$1 AND generation=$2`, f.key, f.generation)
		} else if reviews || !f.reviewOnlyPartial {
			retryAt, err := g.retryTime(ctx, tx, now)
			if err != nil {
				return refresh.State{}, err
			}
			_, e = tx.ExecContext(ctx, `UPDATE public.refresh_fills SET `+column+`=COALESCE(`+column+`,$3) WHERE fill_key=$1 AND generation=$2`, f.key, f.generation, retryAt)
		}
		if e != nil {
			return refresh.State{}, refresh.ErrHeld
		}
		f, e = readRefreshFill(ctx, tx, f.key)
		if e != nil {
			return refresh.State{}, refresh.ErrHeld
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE public.refresh_waiters SET released=true WHERE ticket_id=$1 AND generation=$2 AND NOT released`, []byte(t.ID), []byte(t.Generation)); e != nil {
		return refresh.State{}, refresh.ErrHeld
	}
	if e = tx.Commit(); e != nil {
		return refresh.State{}, refresh.ErrHeld
	}
	return refreshState(f, now, reviews), nil
}

func (g *RefreshGate) retryTime(ctx context.Context, tx *sql.Tx, now time.Time) (time.Time, error) {
	retry := ceilRefreshTime(now.Add(g.config.Limits.Cooldown))
	var floor *time.Time
	if tx.QueryRowContext(ctx, `SELECT retry_at FROM public.refresh_budgets WHERE source_id=$1`, []byte(g.config.SourceID)).Scan(&floor) != nil {
		return time.Time{}, refresh.ErrHeld
	}
	if floor != nil && retry.Before(*floor) {
		retry = *floor
	}
	return retry, nil
}
