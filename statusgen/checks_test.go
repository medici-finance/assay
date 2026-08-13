package main

import (
	"strings"
	"testing"
)

func mkStream(name, status, prio string, briefs ...Brief) *Stream {
	return &Stream{Name: name, Dir: "/repo/docs/streams/" + name, Status: status, Priority: prio, Briefs: briefs}
}

func TestCheckCatchesProblems(t *testing.T) {
	streams := []*Stream{
		mkStream("alpha", "done", "P1"),                                    // done but not archived
		mkStream("beta", "active", ""),                                     // active without priority
		{Name: "gamma", Dir: "/repo/docs/streams/delta", Status: "paused"}, // name != dir
		mkStream("epsilon", "active", "P0",
			Brief{Num: "01", Wave: 0, Status: "done"}, // done without Verified/Reviewed
			Brief{Num: "01", Wave: 0, Status: "todo"}, // duplicate num
			Brief{Num: "02", Wave: 0, Status: "wat"},  // bad status
		),
	}
	findings := []Finding{{ID: "F-09", Affects: []string{"nosuch/brief-01"}}}
	problems, _ := check(streams, findings)
	for _, want := range []string{"done streams must", "requires a priority", "!= directory name", "requires Verified", "duplicate brief", "invalid status", "unknown stream"} {
		found := false
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentioning %q in %v", want, problems)
		}
	}
}

func TestCheckHealthy(t *testing.T) {
	streams := []*Stream{mkStream("ok", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done", Verified: "grandfathered", Reviewed: "grandfathered"},
		Brief{Num: "02", Wave: 1, Status: "todo"},
	)}
	if problems, notices := check(streams, nil); len(problems) != 0 || len(notices) != 0 {
		t.Errorf("healthy stream produced problems or notices: %v %v", problems, notices)
	}
}

func TestResolvedFindingHistoricalRefs(t *testing.T) {
	// A resolved finding may reference a stream or brief that has since been
	// archived (F-05 → operator); only unresolved findings need live targets.
	streams := []*Stream{mkStream("live", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})}
	findings := []Finding{
		{ID: "F-05", Affects: []string{"operator/brief-12b"}, Resolved: true},
		{ID: "F-06", Affects: []string{"live/brief-99"}, Resolved: true},
		{ID: "F-07", Affects: []string{"gone/brief-01"}, Resolved: false},
	}
	problems, _ := check(streams, findings)
	for _, p := range problems {
		if strings.Contains(p, "F-05") || strings.Contains(p, "F-06") {
			t.Errorf("resolved finding's historical ref must not error: %v", p)
		}
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "F-07") && strings.Contains(p, "unknown stream") {
			found = true
		}
	}
	if !found {
		t.Errorf("unresolved finding with unknown stream must still error: %v", problems)
	}
}

func TestApplyFindings(t *testing.T) {
	s := mkStream("ok", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	applyFindings([]*Stream{s}, []Finding{
		{ID: "F-01", Affects: []string{"ok/brief-02"}, Resolved: false},
		{ID: "F-02", Affects: []string{"ok/brief-01"}, Resolved: true},
	})
	if s.Briefs[1].StaleRef != "F-01" {
		t.Errorf("brief-02 should be stale-flagged: %+v", s.Briefs[1])
	}
	if s.Briefs[0].StaleRef != "" {
		t.Errorf("resolved finding must not flag brief-01: %+v", s.Briefs[0])
	}
}

// TestApplyFindingsStreamLevelNotBroadcast is the regression: a finding
// whose affects: entry names a bare stream (no /NN) is a STREAM-level
// annotation. Because StaleRef is a hard Next-up exclusion, stamping it on
// every brief in the stream silently removed the whole stream from dispatch —
// one note on issue-loop hid 594 todo placeholders. A bare-stream entry must
// flag no brief; a brief-specific entry must still flag its own brief.
func TestApplyFindingsStreamLevelNotBroadcast(t *testing.T) {
	wide := mkStream("issue-loop", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
	)
	narrow := mkStream("methodology", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	applyFindings([]*Stream{wide, narrow}, []Finding{
		// Bare stream + a brief-specific entry in one finding: the bare half
		// flags nothing, the specific half still flags exactly one brief.
		{ID: "F-guardrails-dup", Affects: []string{"issue-loop", "methodology/brief-02"}},
	})
	for _, b := range wide.Briefs {
		if b.StaleRef != "" {
			t.Errorf("bare-stream affects must not flag issue-loop/%s (got %q)", b.Num, b.StaleRef)
		}
	}
	if narrow.Briefs[1].StaleRef != "F-guardrails-dup" {
		t.Errorf("brief-specific affects must still flag methodology/02: %+v", narrow.Briefs[1])
	}
	if narrow.Briefs[0].StaleRef != "" {
		t.Errorf("unnamed sibling methodology/01 must stay unflagged: %+v", narrow.Briefs[0])
	}

	// The short `<stream>/<NN>` form (no "brief-" prefix) keeps working too.
	short := mkStream("ok", "active", "P1", Brief{Num: "07", Wave: 0, Status: "todo"})
	applyFindings([]*Stream{short}, []Finding{{ID: "F-13", Affects: []string{"ok/07"}}})
	if short.Briefs[0].StaleRef != "F-13" {
		t.Errorf("short <stream>/<NN> affects form must still flag: %+v", short.Briefs[0])
	}
}

func TestCheckMaxConcurrent(t *testing.T) {
	zero := 0
	one := 1
	over := perStreamCap + 1

	// zero -> PROBLEM
	s0 := mkStream("z", "active", "P1")
	s0.MaxConcurrent = &zero
	problems, _ := check([]*Stream{s0}, nil)
	found := false
	for _, p := range problems {
		if strings.Contains(p, "max-concurrent") && strings.Contains(p, "out of range") {
			found = true
		}
	}
	if !found {
		t.Errorf("max-concurrent: 0 should be a PROBLEM, got %v", problems)
	}

	// perStreamCap+1 -> PROBLEM (widens, never restricts)
	sOver := mkStream("over", "active", "P1")
	sOver.MaxConcurrent = &over
	problems, _ = check([]*Stream{sOver}, nil)
	found = false
	for _, p := range problems {
		if strings.Contains(p, "max-concurrent") && strings.Contains(p, "out of range") {
			found = true
		}
	}
	if !found {
		t.Errorf("max-concurrent: %d should be a PROBLEM (exceeds perStreamCap), got %v", over, problems)
	}

	// 1 -> clean
	s1 := mkStream("one", "active", "P1")
	s1.MaxConcurrent = &one
	problems, _ = check([]*Stream{s1}, nil)
	for _, p := range problems {
		if strings.Contains(p, "max-concurrent") {
			t.Errorf("max-concurrent: 1 should be clean, got problem: %s", p)
		}
	}

	// perStreamCap -> clean (equals the global cap)
	eq := perStreamCap
	sEq := mkStream("eq", "active", "P1")
	sEq.MaxConcurrent = &eq
	problems, _ = check([]*Stream{sEq}, nil)
	for _, p := range problems {
		if strings.Contains(p, "max-concurrent") {
			t.Errorf("max-concurrent: %d should be clean, got problem: %s", eq, p)
		}
	}
}
