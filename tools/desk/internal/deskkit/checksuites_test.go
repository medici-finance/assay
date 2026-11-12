package deskkit

import (
	"strings"
	"testing"
)

// TestPendingCheckSuites pins the scope of the suite hold (#6234): which unconcluded suites
// hold a checks-green verdict and which do not.
func TestPendingCheckSuites(t *testing.T) {
	const app = "github-actions"
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
		{"cancelled conclusion", []CheckSuite{green,
			{ID: "15", App: app, Status: "completed", Conclusion: "cancelled", CreatedAt: "2026-11-11T20:01:00Z"}},
			nil},
		// regression: #6234 (F1) — a later green suite at the same head, even a rerun of the
		// stuck suite's own workflow, does not release the forge's merge gate (#6142 stayed
		// BLOCKED after its rerun went green): the stuck suite still holds.
		{"later green rerun still holds", []CheckSuite{
			{ID: "6", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z"},
			{ID: "60", App: app, Status: "completed", Conclusion: "success", RunCount: 1,
				CreatedAt: "2026-11-11T20:00:00Z"}},
			[]string{"suite 6 queued"}},
		{"equal time holds", []CheckSuite{green,
			{ID: "7", App: app, Status: "queued", CreatedAt: green.CreatedAt}},
			[]string{"suite 7 queued"}},
		{"unparseable time holds", []CheckSuite{green,
			{ID: "8", App: app, Status: "queued", CreatedAt: "yesterday"}},
			[]string{"suite 8 queued"}},
		{"zero-run later green holds", []CheckSuite{green,
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
