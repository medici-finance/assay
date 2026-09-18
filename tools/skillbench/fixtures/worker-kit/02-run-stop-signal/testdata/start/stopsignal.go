// Package stopsignal is a tiny, standalone stand-in for the per-run stop check a
// dispatched worker's next desk verb makes — small enough to fix in one sitting,
// deterministic, and offline.
package stopsignal

// ShouldStop reports whether a dispatched run must halt at its next cooperative check:
// true only when a stop flag is armed AND the run is still in the "working" state. A run
// already "handed-off" has nothing left to cooperatively stop.
func ShouldStop(flagArmed bool, state string) bool {
	// TODO: implement — currently never signals a stop, so an armed flag is never honoured.
	return false
}
