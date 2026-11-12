package deskkit

import (
	"fmt"
	"sort"
	"strings"
)

// suiteConcluded reports whether a suite has a verdict. GitHub serves a null conclusion while
// a suite is not done, and a "completed" status with no conclusion is not a verdict either.
func suiteConcluded(s CheckSuite) bool {
	return strings.EqualFold(strings.TrimSpace(s.Status), "completed") && strings.TrimSpace(s.Conclusion) != ""
}

// PendingCheckSuites names the suites at a head that hold a checks-green verdict (#6234): a
// suite with no conclusion holds the forge's merge gate even when it carries no runs, and a
// run-less suite never appears in the check-run rollup. Each entry names the app, suite id,
// status, run count and creation time, sorted.
//
// The hold is scoped so a suite nobody will ever act on cannot hold a head forever:
//
//   - Only a suite from an integration that REPORTS at this head counts: an app owning at
//     least one suite here that carries runs. GitHub opens suites for every installed app with
//     checks permission, and an app that never acts leaves them queued with no runs; such an
//     app reports nothing the merge gate could wait on.
//   - A suite whose conclusion is set (cancelled and stale included: GitHub's own verdict that
//     it will not run) does not hold.
//
// Nothing else releases a held suite. A LATER suite at the same head, even a rerun of the same
// workflow that concluded with runs, supersedes nothing: the forge kept such heads BLOCKED
// after the rerun went green (#6142), so only the stuck suite concluding, or a new head,
// ends the hold.
//
// Consumers: cmd/deskflip checks-green, cmd/deskautolane merge checks-green and cmd/deskpost
// ready precondition (c').
func PendingCheckSuites(suites []CheckSuite) []string {
	reporting := map[string]bool{}
	for _, s := range suites {
		if s.RunCount > 0 && strings.TrimSpace(s.App) != "" {
			reporting[s.App] = true
		}
	}
	var held []string
	for _, s := range suites {
		if suiteConcluded(s) || !reporting[s.App] {
			continue
		}
		status := strings.TrimSpace(s.Status)
		if status == "" {
			status = "no status"
		}
		held = append(held, fmt.Sprintf("%s suite %s %s, %d run(s), created %s",
			s.App, suiteField(s.ID), status, s.RunCount, suiteField(s.CreatedAt)))
	}
	sort.Strings(held)
	return held
}

func suiteField(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(none)"
	}
	return v
}
