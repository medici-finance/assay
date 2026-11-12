package deskkit

import (
	"strings"
	"testing"
)

// TestPendingCheckSuites pins the scope of the suite hold (#6234): which unconcluded suites
// hold a checks-green verdict and which do not.
func TestPendingCheckSuites(t *testing.T) {
	const app = "github-actions"
	const wfA, wfB = ".github/workflows/changelog-check.yml", ".github/workflows/ci.yml"
	green := CheckSuite{ID: "1", App: app, Status: "completed", Conclusion: "success", RunCount: 1,
		CreatedAt: "2026-11-11T20:00:00Z"}
	cases := []struct {
		name   string
		suites []CheckSuite
		want   []string // fragments of the one held entry; nil = nothing held
	}{
		{"none", nil, nil},
		{"all concluded", []CheckSuite{green}, nil},
		{"zero-run queued after green", []CheckSuite{green,
			{ID: "2", App: app, Status: "queued", CreatedAt: "2026-11-11T20:01:00Z"}},
			[]string{"suite 2 queued, 0 run(s)"}},
		{"completed without conclusion", []CheckSuite{green,
			{ID: "3", App: app, Status: "completed", CreatedAt: "2026-11-11T20:01:00Z"}},
			[]string{"suite 3 completed"}},
		{"stale conclusion", []CheckSuite{green,
			{ID: "4", App: app, Status: "completed", Conclusion: "stale", CreatedAt: "2026-11-11T20:01:00Z"}},
			nil},
		{"idle app", []CheckSuite{green,
			{ID: "5", App: "idle-app", Status: "queued", CreatedAt: "2026-11-11T20:01:00Z"}}, nil},
		{"later rerun of the same workflow supersedes", []CheckSuite{
			{ID: "6", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z", Workflow: wfA},
			{ID: "60", App: app, Status: "completed", Conclusion: "success", RunCount: 1,
				CreatedAt: "2026-11-11T20:00:00Z", Workflow: wfA}}, nil},
		// regression: #6234 (F1) — Actions opens one suite per workflow under one slug, so a
		// later suite of ANOTHER workflow re-ran other work: the stuck one still holds.
		{"later suite of another workflow does not supersede", []CheckSuite{
			{ID: "11", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z", Workflow: wfA},
			{ID: "12", App: app, Status: "completed", Conclusion: "success", RunCount: 1,
				CreatedAt: "2026-11-11T20:00:00Z", Workflow: wfB}},
			[]string{"suite 11 queued, 0 run(s), created 2026-11-11T19:00:00Z, workflow " + wfA}},
		{"unknown workflow never supersedes", []CheckSuite{
			{ID: "13", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z"}, green},
			[]string{"suite 13 queued", "workflow (none)"}},
		{"unknown later workflow never supersedes", []CheckSuite{
			{ID: "14", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z", Workflow: wfA}, green},
			[]string{"suite 14 queued"}},
		{"equal time never supersedes", []CheckSuite{green,
			{ID: "7", App: app, Status: "queued", CreatedAt: green.CreatedAt}},
			[]string{"suite 7 queued"}},
		{"unparseable time never supersedes", []CheckSuite{green,
			{ID: "8", App: app, Status: "queued", CreatedAt: "yesterday"}},
			[]string{"suite 8 queued"}},
		{"zero-run later green does not supersede", []CheckSuite{green,
			{ID: "9", App: app, Status: "queued", CreatedAt: "2026-11-11T20:01:00Z"},
			{ID: "10", App: app, Status: "completed", Conclusion: "success", CreatedAt: "2026-11-11T20:02:00Z"}},
			[]string{"suite 9 queued"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// want lists fragments of the ONE held entry; nil = nothing held.
			got := PendingCheckSuites(c.suites)
			if c.want == nil {
				if len(got) != 0 {
					t.Fatalf("held %q, want nothing held", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("held %q, want exactly 1 entry matching %q", got, c.want)
			}
			for _, w := range c.want {
				if !strings.Contains(got[0], w) {
					t.Errorf("held %q, want it to name %q", got[0], w)
				}
			}
		})
	}
}

// TestSuitesNeedWorkflow pins when a backend pays the workflow-runs read: only for an
// unconcluded GitHub Actions suite, never for a concluded head or an idle third-party app.
func TestSuitesNeedWorkflow(t *testing.T) {
	done := CheckSuite{App: ActionsAppSlug, Status: "completed", Conclusion: "success", RunCount: 1}
	cases := []struct {
		name   string
		suites []CheckSuite
		want   bool
	}{
		{"none", nil, false},
		{"all concluded", []CheckSuite{done}, false},
		{"idle third-party app queued", []CheckSuite{done, {App: "idle-app", Status: "queued"}}, false},
		{"actions queued", []CheckSuite{done, {App: ActionsAppSlug, Status: "queued"}}, true},
		{"actions completed without conclusion", []CheckSuite{{App: ActionsAppSlug, Status: "completed"}}, true},
	}
	for _, c := range cases {
		if got := SuitesNeedWorkflow(c.suites); got != c.want {
			t.Errorf("%s: SuitesNeedWorkflow = %v, want %v", c.name, got, c.want)
		}
	}
}
