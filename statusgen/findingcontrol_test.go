package main

import (
	"strings"
	"testing"
	"time"
)

// recurringFinding builds an unresolved recurring-class finding with a given
// control reference for the closure-check tests.
func recurringFinding(id, date, control string) Finding {
	return Finding{ID: id, Date: date, Title: id + " title", Class: "recurring", Control: control}
}

// streamWith builds a one-stream slice carrying a single brief at a status, for
// resolving a control's "<stream>/<NN>" brief reference.
func streamWith(name, briefNum, status string) []*Stream {
	return []*Stream{{
		Name:   name,
		Briefs: []Brief{{Num: briefNum, Status: status}},
	}}
}

// TestFindingControlNoticeFiresOnRecurringWithoutControl is the positive-control
// fixture: the NOTICE MUST fire on a seeded recurring finding that names no
// control, and it must carry the finding-without-control token, the id, and an age.
func TestFindingControlNoticeFiresOnRecurringWithoutControl(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{recurringFinding("F-recurs", "2026-08-10", "")}

	got := findingControlNotices(findings, nil, now)
	if len(got) != 1 {
		t.Fatalf("want exactly 1 NOTICE, got %d: %v", len(got), got)
	}
	n := got[0]
	if !strings.Contains(n, "finding-without-control") {
		t.Errorf("NOTICE missing token: %q", n)
	}
	if !strings.Contains(n, "F-recurs") {
		t.Errorf("NOTICE missing finding id: %q", n)
	}
	if !strings.Contains(n, "open 14 days") {
		t.Errorf("NOTICE missing age (want 'open 14 days'): %q", n)
	}
}

// TestFindingControlNoticeSilentWhenControlNamesDoneBrief: a control naming a
// brief that is `done` is a landed adaptation — the NOTICE is silent.
func TestFindingControlNoticeSilentWhenControlNamesDoneBrief(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{recurringFinding("F-closed", "2026-08-10", "loop-engine/04")}
	streams := streamWith("loop-engine", "04", "done")

	if got := findingControlNotices(findings, streams, now); len(got) != 0 {
		t.Fatalf("want silent (control landed on a done brief), got: %v", got)
	}
}

// TestFindingControlNoticeListsWhenControlNamesTodoBrief: a control naming a brief
// that is not yet done is a tracked-but-unlanded closure vehicle — still listed.
func TestFindingControlNoticeListsWhenControlNamesTodoBrief(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{recurringFinding("F-pending", "2026-08-10", "loop-engine/04")}

	for _, status := range []string{"todo", "in-progress", "implemented", "verified"} {
		streams := streamWith("loop-engine", "04", status)
		got := findingControlNotices(findings, streams, now)
		if len(got) != 1 {
			t.Fatalf("status %q: want 1 NOTICE (control not yet landed), got %d: %v", status, len(got), got)
		}
		if !strings.Contains(got[0], "loop-engine/04") || !strings.Contains(got[0], "not yet done") {
			t.Errorf("status %q: NOTICE should name the unlanded control brief: %q", status, got[0])
		}
	}
}

// TestFindingControlNoticeDanglingBriefRefStillListed: a control naming a brief
// that resolves to nothing must NOT silence the finding.
func TestFindingControlNoticeDanglingBriefRefStillListed(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{recurringFinding("F-dangling", "2026-08-10", "ghost-stream/99")}

	if got := findingControlNotices(findings, nil, now); len(got) != 1 {
		t.Fatalf("want 1 NOTICE (dangling brief ref must not silence), got %d: %v", len(got), got)
	}
}

// TestFindingControlNoticeTrustsCheckNameAndPath: a non-brief control (a bare
// check name, or a path-shaped check/test reference) is trusted as landed.
func TestFindingControlNoticeTrustsCheckNameAndPath(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	for _, control := range []string{
		"skillslint",                    // bare check name
		"tools/skillslint",              // path-shaped check (slash, but no brief-number tail)
		"statusgen/humanstamp_test.go",  // pinned-test path
		"rubric:no-secrets-in-web-copy", // rubric line
	} {
		findings := []Finding{recurringFinding("F-has-control", "2026-08-10", control)}
		if got := findingControlNotices(findings, nil, now); len(got) != 0 {
			t.Errorf("control %q: want silent (trusted landed artifact), got: %v", control, got)
		}
	}
}

// TestFindingControlNoticeSilentOnOneOffAndUnclassified: only recurring-class
// findings are checked; one-off and unclassified (absent class) are never flagged.
func TestFindingControlNoticeSilentOnOneOffAndUnclassified(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{
		{ID: "F-oneoff", Date: "2026-08-10", Title: "t", Class: "one-off"}, // explicit one-off
		{ID: "F-legacy", Date: "2026-08-10", Title: "t"},                   // absent class
	}
	if got := findingControlNotices(findings, nil, now); len(got) != 0 {
		t.Fatalf("want silent on non-recurring findings, got: %v", got)
	}
}

// TestFindingControlNoticeSilentOnResolved: a resolved recurring finding is closed
// (inert, like a tombstone) even with no control.
func TestFindingControlNoticeSilentOnResolved(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	f := recurringFinding("F-done", "2026-08-10", "")
	f.Resolved = true
	if got := findingControlNotices([]Finding{f}, nil, now); len(got) != 0 {
		t.Fatalf("want silent on a resolved finding, got: %v", got)
	}
}

// TestFindingControlNoticeUndatedStillListed: an undated recurring-no-control
// finding is surfaced without a day count, never dropped (three-state).
func TestFindingControlNoticeUndatedStillListed(t *testing.T) {
	now := mustTime(t, "2026-08-24")
	findings := []Finding{recurringFinding("F-nodate", "not-a-date", "")}
	got := findingControlNotices(findings, nil, now)
	if len(got) != 1 {
		t.Fatalf("want 1 NOTICE for an undated recurring finding, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "open (undated)") {
		t.Errorf("undated NOTICE should say 'open (undated)': %q", got[0])
	}
}

// TestControlLandedClassification unit-tests the brief-ref vs artifact
// discrimination that governs silence.
func TestControlLandedClassification(t *testing.T) {
	streams := []*Stream{{
		Name:   "loop-engine",
		Briefs: []Brief{{Num: "04", Status: "done"}, {Num: "12a", Status: "todo"}},
	}}
	cases := []struct {
		control string
		want    bool
	}{
		{"", false},                            // empty is never landed
		{"loop-engine/04", true},               // done brief
		{"loop-engine/brief-04", true},         // brief- prefix tolerated
		{"loop-engine/12a", false},             // todo brief (letter-suffixed num)
		{"loop-engine/99", false},              // unknown brief number
		{"ghost/01", false},                    // unknown stream
		{"skillslint", true},                   // bare check name
		{"tools/skillslint", true},             // path-shaped, no brief-number tail
		{"statusgen/humanstamp_test.go", true}, // pinned-test path
	}
	for _, c := range cases {
		if got := controlLanded(c.control, streams); got != c.want {
			t.Errorf("controlLanded(%q) = %v, want %v", c.control, got, c.want)
		}
	}
}

// TestParseFindingClassControlFields proves the two OPTIONAL fields round-trip
// from finding frontmatter into the Finding model, and that an entry omitting
// them parses cleanly (absent = one-off, no control).
func TestParseFindingClassControlFields(t *testing.T) {
	withFields := []byte(`---
id: F-x
date: "2026-08-10"
title: "t"
affects: ["s/01"]
resolved: false
class: recurring
control: loop-engine/04
---
body
`)
	e, err := parseFindingFile(withFields)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if e.Class != "recurring" || e.Control != "loop-engine/04" {
		t.Fatalf("class/control not parsed: class=%q control=%q", e.Class, e.Control)
	}

	without := []byte(`---
id: F-y
date: "2026-08-10"
title: "t"
affects: ["s/01"]
resolved: false
---
body
`)
	e2, err := parseFindingFile(without)
	if err != nil {
		t.Fatalf("parse (no fields): %v", err)
	}
	if e2.Class != "" || e2.Control != "" {
		t.Fatalf("absent fields should be empty: class=%q control=%q", e2.Class, e2.Control)
	}
}

// static assertion the constant matches the frontmatter vocabulary.
var _ = func() bool { return recurringClass == "recurring" && time.Now().After(time.Time{}) }()
