// Package heartbeat is a tiny, standalone stand-in for the liveness check a real
// ObservableProbe makes — small enough to fix in one sitting, deterministic, and offline.
package heartbeat

import "time"

// IsStale reports whether a worker has gone silent for longer than the heartbeat gap.
// A worker last seen exactly one gap ago, or longer, is stale; strictly inside the gap is
// not.
func IsStale(lastSeen, now time.Time, gap time.Duration) bool {
	// BUG: strict "greater than" never treats an exactly-gap-old heartbeat as stale, so a
	// worker sitting precisely on the boundary reads alive forever.
	return now.Sub(lastSeen) > gap
}
