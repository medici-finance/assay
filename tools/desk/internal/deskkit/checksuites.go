package deskkit

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ActionsAppSlug is the GitHub App slug GitHub Actions opens its check suites under: one suite
// per workflow run, all under this one slug.
const ActionsAppSlug = "github-actions"

// suiteConcluded reports whether a suite has a verdict. GitHub serves a null conclusion while
// a suite is not done, and a "completed" status with no conclusion is not a verdict either.
func suiteConcluded(s CheckSuite) bool {
	return strings.EqualFold(strings.TrimSpace(s.Status), "completed") && strings.TrimSpace(s.Conclusion) != ""
}

// SuitesNeedWorkflow reports whether any unconcluded suite belongs to GitHub Actions — the one
// case where a suite's workflow decides whether a later suite superseded it, so the only case a
// backend needs the extra workflow-runs read for. A head with nothing unconcluded, or only an
// idle third-party app's queued suite, costs no read.
func SuitesNeedWorkflow(suites []CheckSuite) bool {
	for _, s := range suites {
		if s.App == ActionsAppSlug && !suiteConcluded(s) {
			return true
		}
	}
	return false
}

// PendingCheckSuites names the suites at a head that hold a checks-green verdict (#6234): a
// suite with no conclusion holds the forge's merge gate even when it carries no runs, and a
// run-less suite never appears in the check-run rollup. Each entry names the app, suite id,
// status, run count, creation time and workflow, sorted.
//
// The hold is scoped so a suite nobody will ever act on cannot hold a head forever:
//
//   - Only a suite from an integration that REPORTS at this head counts: an app owning at
//     least one suite here that carries runs. GitHub opens suites for every installed app with
//     checks permission, and an app that never acts leaves them queued with no runs; such an
//     app reports nothing the merge gate could wait on.
//   - A later-created suite of the same app AND the same workflow that concluded WITH runs
//     supersedes an earlier unconcluded one: that workflow re-ran at this head (close and
//     reopen, a fresh event), and its newest suite is what the merge gate reads. A later
//     suite of a DIFFERENT workflow re-ran other work and supersedes nothing, and a suite
//     whose workflow is unknown neither supersedes nor is superseded. Equal or unparseable
//     timestamps never supersede.
//   - A suite whose conclusion is set (stale included: GitHub's own verdict that it will never
//     run) does not hold.
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
		if suiteConcluded(s) || !reporting[s.App] || supersededSuite(s, suites) {
			continue
		}
		status := strings.TrimSpace(s.Status)
		if status == "" {
			status = "no status"
		}
		held = append(held, fmt.Sprintf("%s suite %s %s, %d run(s), created %s, workflow %s",
			s.App, suiteField(s.ID), status, s.RunCount, suiteField(s.CreatedAt), suiteField(s.Workflow)))
	}
	sort.Strings(held)
	return held
}

// supersededSuite reports whether a LATER-created suite of the same app and the same known
// workflow concluded with runs.
func supersededSuite(s CheckSuite, suites []CheckSuite) bool {
	wf := strings.TrimSpace(s.Workflow)
	if wf == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(s.CreatedAt))
	if err != nil {
		return false
	}
	for _, t := range suites {
		if t.App != s.App || strings.TrimSpace(t.Workflow) != wf || t.RunCount == 0 || !suiteConcluded(t) {
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
