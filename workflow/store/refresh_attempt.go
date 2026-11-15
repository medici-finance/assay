package store

import (
	"bytes"
	"context"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

func attemptHeld(h mirror.AttemptHold) error { return &mirror.AttemptError{Hold: h} }
func grantKey(grant mirror.AttemptGrant) refresh.FillKey {
	return refresh.FillKey{Scope: grant.Scope, SourceID: grant.SourceID, AccessEpoch: grant.AccessEpoch, ConfigEpoch: grant.ConfigEpoch, PolicyClass: grant.PolicyClass, PolicyEpoch: grant.PolicyEpoch}
}
func (g *RefreshGate) BeforeAttempt(ctx context.Context, grant mirror.AttemptGrant) (mirror.AttemptReservation, error) {
	if g == nil || ctx == nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.WaitTimeout)
	defer cancel()
	k := grantKey(grant)
	if !g.keyOK(k) || grant.CallerID == "" {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptAdmissionHeld)
	}
	if g.current(ctx, grant.CallerID, grant.Scope) != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptCallerHeld)
	}
	tx, now, e := g.start(ctx)
	if e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	defer tx.Rollback()
	if e = g.lockCaller(ctx, tx, grant.CallerID); e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	key, identity, e := fillIndex(k)
	if e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptAdmissionHeld)
	}
	f, e := readRefreshFill(ctx, tx, key)
	if e != nil || !bytes.Equal(f.identity, identity) || !grant.Equal(g.grant(f, k)) || grant.Validate(now) != nil || f.state != "pending" {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptAdmissionHeld)
	}
	var demand bool
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM public.refresh_waiters WHERE fill_key=$1 AND generation=$2 AND NOT released AND expires_at>$3)`, key, f.generation, now).Scan(&demand) != nil || !demand {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptAdmissionHeld)
	}
	held, _, e := g.budgetHeld(ctx, tx, now)
	if e != nil || held {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptSourceHeld)
	}
	if e = g.rollWindow(ctx, tx, now, grant.CallerID); e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	updates := []struct {
		query string
		args  []any
		hold  mirror.AttemptHold
	}{
		{`UPDATE public.refresh_budgets SET http_used=http_used+1 WHERE source_id=$1 AND known AND NOT unknown_hold AND http_used<http_limit AND (retry_at IS NULL OR retry_at<=clock_timestamp()) AND (SELECT count(*) FROM public.refresh_attempts WHERE source_id=$1 AND NOT observed)<inflight_limit AND (SELECT count(*) FROM public.refresh_attempts WHERE source_id=$1)<reservation_limit`, []any{[]byte(g.config.SourceID)}, mirror.AttemptSourceHeld},
		{`UPDATE public.refresh_admissions SET http_used=http_used+1 WHERE admission=$1 AND known AND http_used<http_limit`, []any{[]byte(g.config.AdmissionID)}, mirror.AttemptAdmissionHeld},
		{`UPDATE public.refresh_callers SET http_used=http_used+1 WHERE admission=$1 AND caller_id=$2 AND known AND http_used<http_limit`, []any{[]byte(g.config.AdmissionID), []byte(grant.CallerID)}, mirror.AttemptCallerHeld},
		{`UPDATE public.refresh_fills SET attempts_used=attempts_used+1 WHERE fill_key=$1 AND generation=$2 AND state='pending' AND expires_at>clock_timestamp() AND attempts_used<attempt_limit`, []any{key, f.generation}, mirror.AttemptSourceHeld},
	}
	for _, u := range updates {
		r, err := tx.ExecContext(ctx, u.query, u.args...)
		if err != nil {
			return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return mirror.AttemptReservation{}, attemptHeld(u.hold)
		}
	}
	id, e := randomRefreshID()
	if e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_attempts(reservation_id,admission,source_id,fill_key,generation,caller_id,grant_id,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, []byte(id), []byte(g.config.AdmissionID), []byte(g.config.SourceID), key, f.generation, []byte(grant.CallerID), []byte(grant.ID), grant.ExpiresAt); e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	if g.current(ctx, grant.CallerID, grant.Scope) != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptCallerHeld)
	}
	if e = tx.Commit(); e != nil {
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptUnknownHeld)
	}
	reservation := mirror.AttemptReservation{ID: id, Grant: grant}
	if g.current(ctx, grant.CallerID, grant.Scope) != nil {
		// Commit was acknowledged but no permit has escaped this function. Record
		// certain zero-entry work without refunding the charged reservation. If this
		// bounded bookkeeping is uncertain, its unresolved slot continues to hold.
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), g.config.Limits.WaitTimeout)
		_ = g.ObserveAttempt(cleanup, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now().UTC()})
		stop()
		return mirror.AttemptReservation{}, attemptHeld(mirror.AttemptCallerHeld)
	}
	return reservation, nil
}

// ceilRefreshTime preserves a lower retry bound when PostgreSQL rounds its
// microsecond timestamps. Observation watermarks retain Store52's exact codec.
func ceilRefreshTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	u := t.UTC().Truncate(time.Microsecond)
	if u.Before(t) {
		u = u.Add(time.Microsecond)
	}
	return u
}
func validAttemptObservation(o mirror.AttemptObservation) bool {
	if o.ObservedAt.IsZero() || !o.HTTP.Known || o.DecodedBytes.Value < 0 || o.Status < 0 || o.Status > 599 || o.Retry.Delay < 0 {
		return false
	}
	if o.HTTP.Value == 0 {
		return o.Status == 0 && !o.ResponseComplete && o.DecodedBytes.Known && o.DecodedBytes.Value == 0 && o.Retry.Knowledge == mirror.RetryNoConstraint && o.Retry.NotBefore.IsZero() && !o.Retry.DelayKnown && o.Retry.Delay == 0
	}
	if o.HTTP.Value != 1 {
		return false
	}
	switch o.Retry.Knowledge {
	case mirror.RetryNoResponse:
		return o.Status == 0 && !o.ResponseComplete && o.Retry.NotBefore.IsZero() && !o.Retry.DelayKnown
	case mirror.RetryNoConstraint:
		return o.Status >= 100 && o.Retry.NotBefore.IsZero() && !o.Retry.DelayKnown
	case mirror.RetryNotBefore:
		return o.Status >= 100 && !o.Retry.NotBefore.IsZero()
	case mirror.RetryUnknown:
		return false
	default:
		return false
	}
}
func (g *RefreshGate) ObserveAttempt(ctx context.Context, r mirror.AttemptReservation, o mirror.AttemptObservation) error {
	if g == nil || ctx == nil {
		return attemptHeld(mirror.AttemptUnknownHeld)
	}
	ctx, cancel := context.WithTimeout(ctx, g.config.Limits.WaitTimeout)
	defer cancel()
	k := grantKey(r.Grant)
	if r.ID == "" || !g.keyOK(k) {
		return attemptHeld(mirror.AttemptAdmissionHeld)
	}
	tx, now, e := g.start(ctx)
	if e != nil {
		return attemptHeld(mirror.AttemptUnknownHeld)
	}
	defer tx.Rollback()
	key, identity, ke := fillIndex(k)
	if ke != nil {
		return attemptHeld(mirror.AttemptAdmissionHeld)
	}
	var persistedIdentity []byte
	if tx.QueryRowContext(ctx, `SELECT identity FROM public.refresh_fills WHERE fill_key=$1`, key).Scan(&persistedIdentity) != nil || !bytes.Equal(identity, persistedIdentity) {
		return attemptHeld(mirror.AttemptAdmissionHeld)
	}
	// No current-generation publication occurs here: a late qualified observation
	// can close its own reservation and raise a floor, never complete a stale fill.
	var source, fill, gen, caller, grant []byte
	var expires time.Time
	var observed bool
	e = tx.QueryRowContext(ctx, `SELECT source_id,fill_key,generation,caller_id,grant_id,expires_at,observed FROM public.refresh_attempts WHERE reservation_id=$1 AND admission=$2 FOR UPDATE`, []byte(r.ID), []byte(g.config.AdmissionID)).Scan(&source, &fill, &gen, &caller, &grant, &expires, &observed)
	if e != nil || ke != nil || observed || string(source) != r.Grant.SourceID || !bytes.Equal(fill, key) || string(gen) != r.Grant.Generation || string(caller) != r.Grant.CallerID || string(grant) != r.Grant.ID || !expires.Equal(r.Grant.ExpiresAt) || r.Grant.MaxHTTPAttempts != g.config.Native.PerFill {
		return attemptHeld(mirror.AttemptAdmissionHeld)
	}
	known := validAttemptObservation(o)
	httpCount := max(o.HTTP.Value, 0)
	decoded := max(o.DecodedBytes.Value, 0)
	httpKnown := o.HTTP.Known && o.HTTP.Value >= 0
	decodedKnown := o.DecodedBytes.Known && o.DecodedBytes.Value >= 0
	// SQL anchors and publishes the floor with this individual report. It keeps
	// the positive reported bound even when other receipt knowledge is unknown.
	var floor *time.Time
	retry := string(o.Retry.Knowledge)
	if !known {
		retry = string(mirror.RetryUnknown)
	}
	status := o.Status
	if status < 0 || status > 599 {
		status = 0
	}
	if _, e = tx.ExecContext(ctx, `UPDATE public.refresh_attempts SET observed=true,unknown_hold=$2,http_count=$3,decoded_bytes=$4,response_complete=$5,status=$6,retry_knowledge=$7,retry_at=$8,observed_at=$9,source_observed_at=$10,reported_retry_at=$11,retry_delay_ns=$12,delay_known=$13,http_known=$14,decoded_known=$15 WHERE reservation_id=$1 AND NOT observed`, []byte(r.ID), !known, httpCount, decoded, o.ResponseComplete, status, retry, floor, now, stamp(o.ObservedAt), stamp(o.Retry.NotBefore), max(int64(o.Retry.Delay), 0), o.Retry.DelayKnown, httpKnown, decodedKnown); e != nil {
		return attemptHeld(mirror.AttemptUnknownHeld)
	}

	if e = tx.Commit(); e != nil {
		return attemptHeld(mirror.AttemptUnknownHeld)
	}
	if !known {
		return attemptHeld(mirror.AttemptUnknownHeld)
	}
	return nil
}
