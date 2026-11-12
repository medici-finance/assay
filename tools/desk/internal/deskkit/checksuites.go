package deskkit

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
//   - A later-created suite of the same app that concluded WITH runs supersedes an earlier
//     unconcluded one: a re-trigger (close and reopen, a fresh event) re-ran that app's work
//     at this head. Equal or unparseable timestamps never supersede.
//   - A suite whose conclusion is set (stale included: GitHub's own verdict that it will never
//     run) does not hold.
//
// Consumers: cmd/deskflip and cmd/deskautolane checks-green.
func PendingCheckSuites(suites []CheckSuite) []string {
	reporting := map[string]bool{}
	for _, s := range suites {
		if s.RunCount > 0 && strings.TrimSpace(s.App) != "" {
			reporting[s.App] = true
		}
	}
	var held []string
	for _, s := range suites {
		if suiteConcluded(s) || !reporting[s.App] || supersededSuite(s, suites) {
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

// supersededSuite reports whether a LATER-created suite of the same app concluded with runs.
func supersededSuite(s CheckSuite, suites []CheckSuite) bool {
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(s.CreatedAt))
	if err != nil {
		return false
	}
	for _, t := range suites {
		if t.App != s.App || t.RunCount == 0 || !suiteConcluded(t) {
			continue
		}
		tAt, terr := time.Parse(time.RFC3339, strings.TrimSpace(t.CreatedAt))
		if terr == nil && tAt.After(at) {
			return true
		}
	}
	return false
}

func suiteField(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(none)"
	}
	return v
}
