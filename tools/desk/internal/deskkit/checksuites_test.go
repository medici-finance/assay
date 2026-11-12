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
		want   []string // fragments; nil = nothing held
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
		{"later rerun supersedes", []CheckSuite{
			{ID: "6", App: app, Status: "queued", CreatedAt: "2026-11-11T19:00:00Z"}, green}, nil},
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
			got := PendingCheckSuites(c.suites)
			if len(got) != len(c.want) {
				t.Fatalf("held %q, want %d entr(ies) matching %q", got, len(c.want), c.want)
			}
			for i, w := range c.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("entry %d = %q, want it to name %q", i, got[i], w)
				}
			}
		})
	}
}
