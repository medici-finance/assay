package mirror

import "time"

// PlanScan forms one version-bound transaction request. The durable adapter must
// independently enforce its admission and completeness constraints. No retention
// action is represented: permission loss requires a separate policy decision.
func PlanScan(s Snapshot, p ScanPlan, scan ScanResult, now time.Time) (ScanUpdate, error) {
	if err := validateSnapshot(s, now); err != nil {
		return ScanUpdate{}, err
	}
	if p.Scope != s.Scope || p.ExpectedVersion != s.Version {
		return ScanUpdate{}, ErrConflict
	}
	if (p.FullInventory && !p.Since.IsZero()) || p.Since.After(now) {
		return ScanUpdate{}, invalid("invalid scan window")
	}
	if now.Before(s.Coverage.LastAttempted) || now.Before(p.NotBefore) {
		return ScanUpdate{}, invalid("attempt before prior attempt or backoff")
	}
	if scan.Failure != "" && !validReason(scan.Failure) {
		return ScanUpdate{}, invalid("scan failure class")
	}
	if scan.Complete && scan.Failure != "" {
		return ScanUpdate{}, invalid("failed scan claims complete")
	}
	if err := validateRecords(s.Scope, scan.Records, now); err != nil {
		return ScanUpdate{}, err
	}
	oldRecords := make(map[RecordID]Observation, len(s.Records))
	for _, r := range s.Records {
		oldRecords[r.ID] = r
	}
	for _, r := range scan.Records {
		if old, ok := oldRecords[r.ID]; ok && ((!old.UpdatedAt.IsZero() && r.UpdatedAt.Before(old.UpdatedAt)) || (!old.ObservedAt.IsZero() && r.ObservedAt.Before(old.ObservedAt))) {
			return ScanUpdate{}, invalid("record timestamp regression")
		}
	}
	if scan.ObservedUpTo.After(now) || !scan.ObservedUpTo.IsZero() && scan.ObservedUpTo.Before(s.Coverage.ObservedUpTo) {
		return ScanUpdate{}, invalid("observed-up-to regression")
	}
	u := ScanUpdate{Scope: s.Scope, ExpectedVersion: s.Version, Coverage: s.Coverage, FullInventory: p.FullInventory}
	u.Coverage.LastAttempted = now
	if scan.Failure == Permission {
		u.Coverage.State = Unavailable
		u.Coverage.Reason = Permission
		u.RetentionDecisionRequired = true
		return u, nil
	}
	// Coverage.Reason is the persisted permission-loss hold. A failed or partial
	// observation cannot establish that source access returned or choose a retention action.
	if s.Coverage.Reason == Permission && !scan.Complete {
		u.Coverage.State = Unavailable
		u.Coverage.Reason = Permission
		u.RetentionDecisionRequired = true
		return u, nil
	}
	// Access restoration needs an inventory newer than the last held attempt;
	// an incremental response cannot account for deletions or moves during the gap.
	if s.Coverage.Reason == Permission && (!p.FullInventory || !now.After(s.Coverage.LastAttempted)) {
		return ScanUpdate{}, invalid("permission recovery requires a fresh full inventory")
	}
	if scan.NotModified {
		if !scan.Complete || scan.Failure != "" || len(scan.Records) > 0 || !scan.ConditionalSupported || s.Coverage.State != Complete || p.Validator == "" || p.Validator != s.Coverage.Validator || scan.Validator != p.Validator || p.FullInventory != s.Coverage.ValidatorFullInventory || !p.Since.Equal(s.Coverage.ValidatorSince) || (scan.Revision != "" && scan.Revision != s.Coverage.Revision) {
			return ScanUpdate{}, invalid("not-modified lacks compatible complete baseline")
		}
		u.Coverage.LastComplete = now
		if p.FullInventory {
			u.Coverage.LastInventory = now
		}
		u.CompleteScan = true
		return u, nil
	}
	u.Upsert = cloneRecords(scan.Records)
	for i := range u.Upsert {
		u.Upsert[i].InInventory = true
	}
	if !scan.Complete {
		u.Coverage.State = Partial
		u.Coverage.Reason = Truncated
		if scan.Failure != "" {
			u.Coverage.State = Unavailable
			u.Coverage.Reason = scan.Failure
		}
		return u, nil
	}
	u.Coverage.State = Complete
	u.Coverage.Reason = ""
	u.Coverage.LastComplete = now
	if !scan.ObservedUpTo.IsZero() {
		u.Coverage.ObservedUpTo = scan.ObservedUpTo
	}
	u.Coverage.Revision = scan.Revision
	u.Coverage.Validator = scan.Validator
	u.Coverage.ValidatorSince = p.Since
	u.Coverage.ValidatorFullInventory = p.FullInventory
	u.CompleteScan = true
	if p.FullInventory {
		u.Coverage.LastInventory = now
		seen := map[RecordID]bool{}
		for _, r := range scan.Records {
			seen[r.ID] = true
		}
		for _, r := range s.Records {
			if !seen[r.ID] {
				u.NotInInventory = append(u.NotInInventory, r.ID)
			}
		}
	}
	return u, nil
}

// Validate checks a transaction's structural contract without deciding authority,
// version currency or durable completeness. Store adapters retain those checks.
func (u ScanUpdate) Validate() error {
	if u.Coverage.State == Unavailable && u.Coverage.Reason == Permission && !u.RetentionDecisionRequired {
		return invalid("permission loss requires retention decision signal")
	}
	if u.Coverage.State == NeverRead {
		return invalid("a scan update cannot publish never-read rows")
	}
	if err := validateSnapshot(Snapshot{Scope: u.Scope, Coverage: u.Coverage}, u.Coverage.LastAttempted); err != nil {
		return err
	}
	if err := validateRecords(u.Scope, u.Upsert, u.Coverage.LastAttempted); err != nil {
		return err
	}
	ids := make([]Observation, len(u.NotInInventory))
	for i, id := range u.NotInInventory {
		ids[i].ID = id
	}
	if err := validateRecords(u.Scope, ids, u.Coverage.LastAttempted); err != nil {
		return err
	}
	seen := map[RecordID]bool{}
	for _, r := range u.Upsert {
		seen[r.ID] = true
	}
	for _, id := range u.NotInInventory {
		if seen[id] {
			return invalid("record both upserted and absent")
		}
	}
	return nil
}
