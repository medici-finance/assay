package mirror

import "time"

func validateSnapshot(s Snapshot, now time.Time) error {
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if now.IsZero() {
		return invalid("missing observation time")
	}
	c := s.Coverage
	if !validState(c.State) || c.Reason != "" && !validReason(c.Reason) {
		return invalid("coverage state or reason")
	}
	if c.State == Complete && (c.LastComplete.IsZero() || c.Reason != "") {
		return invalid("complete coverage lacks a credible watermark")
	}
	if c.State == Unavailable && c.Reason == "" {
		return invalid("unavailable coverage lacks reason")
	}
	for _, stamp := range []time.Time{c.LastAttempted, c.LastComplete, c.LastInventory, c.ValidatorSince, c.ObservedUpTo, c.RetainedFrom} {
		if stamp.After(now) {
			return invalid("future coverage timestamp")
		}
	}
	if !c.LastComplete.IsZero() && (c.LastAttempted.IsZero() || c.LastComplete.After(c.LastAttempted)) {
		return invalid("watermark after attempt")
	}
	if c.LastInventory.After(c.LastComplete) {
		return invalid("inventory after completeness watermark")
	}
	if c.State == NeverRead && (!c.LastComplete.IsZero() || len(s.Records) > 0) {
		return invalid("never-read scope carries observed rows")
	}
	return validateRecords(s.Scope, s.Records, now)
}
func unavailable(scope Scope, reason Reason) Result {
	return Result{Snapshot: Snapshot{Scope: scope, Coverage: Coverage{State: Unavailable, Reason: reason}}, Stale: true}
}
func snapshotResult(s Snapshot, q Query) Result {
	fresh := s.Coverage.State == Complete && !s.Coverage.LastComplete.IsZero() && !s.Coverage.LastComplete.After(q.Now) && q.Now.Sub(s.Coverage.LastComplete) <= q.MaxAge
	return Result{Snapshot: CloneSnapshot(s), Available: fresh, Stale: !fresh}
}
