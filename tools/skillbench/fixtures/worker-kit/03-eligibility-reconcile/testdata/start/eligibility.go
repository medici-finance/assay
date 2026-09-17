// Package eligibility is a tiny, standalone stand-in for the mid-run eligibility check a
// worker's supervising process makes — small enough to fix in one sitting, deterministic,
// and offline.
package eligibility

// IsEligible reports whether a claimed item's PR is still eligible for a running worker.
// A merged or closed PR ends eligibility; every other state keeps it.
func IsEligible(prState string) bool {
	// BUG: always eligible — a worker whose PR was merged or closed out from under it
	// keeps running forever.
	return true
}
