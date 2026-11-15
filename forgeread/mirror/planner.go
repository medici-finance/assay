package mirror

import "time"

// PlanRefresh selects a full inventory or overlapping incremental scan. It does
// not schedule, coalesce, sleep, retry or call a source. NotBefore is advisory
// input to the later single ingestion owner, not a distributed ownership claim.
func PlanRefresh(s Snapshot, now time.Time, p RefreshPolicy) (ScanPlan, error) {
	if err := validateSnapshot(s, now); err != nil {
		return ScanPlan{}, err
	}
	if p.Overlap < 0 || p.FullInventoryInterval <= 0 || p.FailureBackoff < 0 {
		return ScanPlan{}, invalid("refresh policy")
	}
	plan := ScanPlan{Scope: s.Scope, ExpectedVersion: s.Version, PriorObservedUpTo: s.Coverage.ObservedUpTo}
	c := s.Coverage
	plan.FullInventory = c.Reason == Permission || c.State == NeverRead || c.LastComplete.IsZero() || c.ObservedUpTo.IsZero() || c.LastInventory.IsZero() || now.Sub(c.LastInventory) >= p.FullInventoryInterval
	if !plan.FullInventory {
		plan.Since = c.ObservedUpTo.Add(-p.Overlap)
	}
	if c.State == Complete && c.Validator != "" && plan.FullInventory == c.ValidatorFullInventory && plan.Since.Equal(c.ValidatorSince) {
		plan.Validator = c.Validator
	}
	if c.State == Partial || c.State == Unavailable {
		plan.NotBefore = c.LastAttempted.Add(p.FailureBackoff)
	}
	return plan, nil
}
