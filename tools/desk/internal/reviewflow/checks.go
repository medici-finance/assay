package reviewflow

import "strings"

// CheckFact is one latest run, selected by deskkit.LatestRunPerName before entry.
type CheckFact struct{ Status, Conclusion, State string }

func CountChecks(checks []CheckFact) (pass, pending, fail, unknown int) {
	// Reduce to the latest run per check NAME first — branch protection's own rule, from
	// the SAME shared reducer deskflip's gate uses — so a superseded run (an older
	// CANCELLED, a stale QUEUED left by a push+pull_request double-trigger) does not count
	// against a PR whose current run for that name is not it. Without this the board would
	// render a double-triggered PR CI-fail while deskflip flips it ready — the divergence
	// #282/#289 set out to end.
	for _, c := range checks {
		switch {
		case c.Status != "": // CheckRun node
			switch {
			case c.Status != "COMPLETED":
				pending++
			case c.Conclusion == "SUCCESS":
				pass++
			case c.Conclusion == "SKIPPED" || c.Conclusion == "NEUTRAL":
				// ignore path-filtered / neutral
			case c.Conclusion == "":
				// #400 N8: COMPLETED with no conclusion at all. This used to fall into
				// `default` and be counted a FAILURE — fail-closed, but the row then said
				// "a check FAILED" about a conclusion nobody could read, which is the same
				// absence-as-verdict shape as R2/R3 with the sign flipped. It is an
				// UNREADABLE entry, and CI-UNKNOWN is what this board calls that; it
				// preempts every approve+green path exactly as a fail does, so nothing
				// gets looser, and the row now names what actually happened.
				unknown++
			default: // FAILURE, CANCELLED, TIMED_OUT, ACTION_REQUIRED, STARTUP_FAILURE
				fail++
			}
		case c.State != "": // StatusContext node (legacy commit status)
			switch strings.ToUpper(c.State) {
			case "SUCCESS":
				pass++
			// #400 T1: `EXPECTED` is NOT a pass. GitHub's StatusState documents it as
			// "Status is expected" — a required legacy status context that has been
			// DECLARED and has not reported yet — as against `SUCCESS`, "Status is
			// successful". Counting it as a pass was this PR's own R2/Q2 defect on the
			// legacy-status axis: one `EXPECTED` context, approved at head, non-draft,
			// CLEAN, rendered `pass=1 ciGreen=true action=MERGE-NOW note="…CI green…"`
			// off a check that had reported nothing. It is a check still owed, which is
			// exactly what `pending` means here — and pending, unlike pass, cannot reach
			// MERGE-NOW or a "CI green" note.
			case "PENDING", "EXPECTED":
				pending++
			case "FAILURE", "ERROR":
				fail++
			default:
				unknown++
			}
		default:
			// Neither shape decoded: a rollup entry we cannot read at all.
			unknown++
		}
	}
	return
}
