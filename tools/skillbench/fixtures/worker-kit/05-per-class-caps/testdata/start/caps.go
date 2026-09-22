// Package caps is a tiny, standalone stand-in for the planner's per-class concurrency
// reservation — small enough to fix in one sitting, deterministic, and offline.
package caps

// Allowed reports whether one more worker of the given class may start, given the
// current in-flight counts and the configured per-class caps. A class with no configured
// cap is uncapped.
func Allowed(class string, counts, caps map[string]int) bool {
	// TODO: implement the per-class cap check — currently always allows, so a capped
	// class never actually stops admitting new workers.
	return true
}
