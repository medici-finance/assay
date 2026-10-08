package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// effFinding builds a resolved finding with an optional effectiveness triple.
func effFinding(id, date string, resolved bool, cmd, ran, by string) Finding {
	return Finding{
		ID: id, Date: date, Title: id + " title", Resolved: resolved,
		Effectiveness: cmd, EffectivenessDate: ran, EffectivenessBy: by,
	}
}

const (
	effAfter  = "2026-10-08" // on the boundary: owes the record as a PROBLEM
	effBefore = "2026-10-07" // the day before: advisory only
)

// All three keys, or none, is silent; one or two is a hard error naming the
// missing ones.
func TestEffectivenessTripleRequiredTogether(t *testing.T) {
	cases := []struct {
		name    string
		f       Finding
		wantSub []string // empty = silent
	}{
		{"none", effFinding("F-none", effAfter, false, "", "", ""), nil},
		{"all three", effFinding("F-all", effAfter, true, "go test ./x", "2026-10-09", "human:ian"), nil},
		{"only command", effFinding("F-c", effAfter, true, "go test ./x", "", ""), []string{"effectiveness-date", "effectiveness-by"}},
		{"only date", effFinding("F-d", effAfter, true, "", "2026-10-09", ""), []string{"effectiveness (the command", "effectiveness-by"}},
		{"only by", effFinding("F-b", effAfter, true, "", "", "human:ian"), []string{"effectiveness (the command", "effectiveness-date"}},
		{"missing by", effFinding("F-mb", effAfter, true, "go test ./x", "2026-10-09", ""), []string{"effectiveness-by"}},
		{"unparseable date", effFinding("F-ud", effAfter, true, "go test ./x", "yesterday", "human:ian"), []string{"parseable effectiveness-date"}},
		{"whitespace counts as absent", effFinding("F-ws", effAfter, true, "go test ./x", "  ", "human:ian"), []string{"effectiveness-date"}},
		{"partial on an unresolved finding", effFinding("F-un", effAfter, false, "go test ./x", "", ""), []string{"effectiveness-date"}},
		{"partial dated before the boundary is still hard", effFinding("F-old", "2025-01-01", true, "go test ./x", "", ""), []string{"effectiveness-date"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectivenessTripleProblems([]Finding{tc.f})
			if len(tc.wantSub) == 0 {
				if len(got) != 0 {
					t.Fatalf("want silent, got %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want exactly 1 PROBLEM, got %v", got)
			}
			for _, sub := range tc.wantSub {
				if !strings.Contains(got[0], sub) {
					t.Errorf("PROBLEM missing %q: %s", sub, got[0])
				}
			}
			if !strings.HasPrefix(got[0], "["+effTagPartial+"]") {
				t.Errorf("PROBLEM must lead with its rule-tag: %s", got[0])
			}
		})
	}
}

// Positive control (rule 16 mutation row): a resolved finding dated on/after the
// boundary with no triple is a PROBLEM; the same finding with the triple is silent.
// Disarming effectivenessClosureMessages turns this RED.
func TestEffectivenessMissingOnResolvedIsAProblem(t *testing.T) {
	bare := effFinding("F-bare", effAfter, true, "", "", "")
	p, n := effectivenessClosureMessages([]Finding{bare}, nil)
	if len(p) != 1 || len(n) != 0 {
		t.Fatalf("want 1 PROBLEM and 0 NOTICE, got problems=%v notices=%v", p, n)
	}
	for _, want := range []string{
		"[" + effTagMissing + "]", "F-bare",
		"effectiveness-date", "effectiveness-by", // names the keys
		"PRESENCE and attribution only", // states which half it covers
		"rule 16",                       // points at the worked example
	} {
		if !strings.Contains(p[0], want) {
			t.Errorf("PROBLEM missing %q: %s", want, p[0])
		}
	}

	withTriple := effFinding("F-done", effAfter, true, "go test ./x -run TestY", "2026-10-09", "human:ian")
	if p, n := effectivenessClosureMessages([]Finding{withTriple}, nil); len(p)+len(n) != 0 {
		t.Fatalf("a finding with the triple must be silent, got problems=%v notices=%v", p, n)
	}

	// An unresolved finding owes nothing yet.
	open := effFinding("F-open", effAfter, false, "", "", "")
	if p, n := effectivenessClosureMessages([]Finding{open}, nil); len(p)+len(n) != 0 {
		t.Fatalf("an unresolved finding must be silent, got problems=%v notices=%v", p, n)
	}

	// A partial record is effectivenessTripleProblems' to report, not reported twice.
	partial := effFinding("F-part", effAfter, true, "go test ./x", "", "")
	if p, n := effectivenessClosureMessages([]Finding{partial}, nil); len(p)+len(n) != 0 {
		t.Fatalf("a partial record must not be double-reported here, got problems=%v notices=%v", p, n)
	}
}

// The promotion is transition-scoped: a finding dated before the boundary yields a
// NOTICE and never a PROBLEM; the boundary day itself is a PROBLEM.
func TestEffectivenessInheritedCorpusStaysAdvisory(t *testing.T) {
	old := effFinding("F-old", effBefore, true, "", "", "")
	p, n := effectivenessClosureMessages([]Finding{old}, nil)
	if len(p) != 0 || len(n) != 1 {
		t.Fatalf("pre-boundary finding: want 0 PROBLEM and 1 NOTICE, got problems=%v notices=%v", p, n)
	}
	if !strings.Contains(n[0], "advisory") || !strings.Contains(n[0], effectivenessBoundary) {
		t.Errorf("NOTICE must say it is advisory and name the boundary: %s", n[0])
	}

	onBoundary := effFinding("F-edge", effectivenessBoundary, true, "", "", "")
	if p, _ := effectivenessClosureMessages([]Finding{onBoundary}, nil); len(p) != 1 {
		t.Fatalf("a finding dated ON the boundary must be a PROBLEM, got %v", p)
	}

	// Three-state: an unreadable date is could-not-check (a NOTICE saying so), neither
	// silently clean nor a fatal the author cannot place.
	undated := effFinding("F-nodate", "sometime", true, "", "", "")
	p, n = effectivenessClosureMessages([]Finding{undated}, nil)
	if len(p) != 0 || len(n) != 1 || !strings.Contains(n[0], "could-not-check") {
		t.Fatalf("unreadable date: want a could-not-check NOTICE, got problems=%v notices=%v", p, n)
	}

	// Same scoping on the recurring-class path.
	streams := streamWith("loop-engine", "04", "done")
	rOld := effFinding("F-rold", effBefore, true, "", "", "")
	rOld.Class, rOld.Control = "recurring", "loop-engine/04"
	p, n = findingControlUnfiredMessages([]Finding{rOld}, streams)
	if len(p) != 0 || len(n) != 1 {
		t.Fatalf("pre-boundary recurring finding: want NOTICE only, got problems=%v notices=%v", p, n)
	}
}

// The findingcontrol promotion: recurring + resolved + landed control + no record is
// a PROBLEM (after the boundary), reported once, and silent with the triple.
func TestFindingControlUnfiredIsAProblem(t *testing.T) {
	streams := streamWith("loop-engine", "04", "done")
	f := effFinding("F-rec", effAfter, true, "", "", "")
	f.Class, f.Control = "recurring", "loop-engine/04"

	p, n := findingControlUnfiredMessages([]Finding{f}, streams)
	if len(p) != 1 || len(n) != 0 {
		t.Fatalf("want 1 PROBLEM, got problems=%v notices=%v", p, n)
	}
	if !strings.Contains(p[0], "["+effTagUnfired+"]") || !strings.Contains(p[0], "PRESENCE and attribution only") {
		t.Errorf("PROBLEM must carry its tag and the presence-not-adequacy statement: %s", p[0])
	}
	// The generic obligation must not also report the same finding.
	if gp, gn := effectivenessClosureMessages([]Finding{f}, streams); len(gp)+len(gn) != 0 {
		t.Errorf("the recurring+landed class is owned by findingcontrol; generic must skip it, got %v %v", gp, gn)
	}

	f.Effectiveness, f.EffectivenessDate, f.EffectivenessBy = "go test ./x -run TestY", "2026-10-09", "human:ian"
	if p, n := findingControlUnfiredMessages([]Finding{f}, streams); len(p)+len(n) != 0 {
		t.Fatalf("with the triple the finding must be silent, got %v %v", p, n)
	}

	// A recurring finding whose control has NOT landed (brief still todo) is not in
	// findingcontrol's class here; the generic obligation covers it instead.
	pending := effFinding("F-pend", effAfter, true, "", "", "")
	pending.Class, pending.Control = "recurring", "loop-engine/04"
	todo := streamWith("loop-engine", "04", "todo")
	if p, n := findingControlUnfiredMessages([]Finding{pending}, todo); len(p)+len(n) != 0 {
		t.Fatalf("an unlanded control is outside the unfired class, got %v %v", p, n)
	}
	if gp, _ := effectivenessClosureMessages([]Finding{pending}, todo); len(gp) != 1 {
		t.Fatalf("generic obligation should own it, got %v", gp)
	}
}

// Neighbour row (rule 17): a finding with no `class:` or `class: one-off` behaves
// exactly as before — findingcontrol neither flags it open nor claims it resolved.
func TestFindingControlOneOffUnchanged(t *testing.T) {
	now := mustTime(t, "2026-10-20")
	streams := streamWith("loop-engine", "04", "done")
	for _, class := range []string{"", "one-off"} {
		open := Finding{ID: "F-open", Date: effAfter, Title: "t", Class: class}
		if got := findingControlNotices([]Finding{open}, streams, now); len(got) != 0 {
			t.Errorf("class %q open: findingControlNotices must stay silent, got %v", class, got)
		}
		resolved := Finding{ID: "F-res", Date: effAfter, Title: "t", Class: class, Control: "loop-engine/04", Resolved: true}
		if p, n := findingControlUnfiredMessages([]Finding{resolved}, streams); len(p)+len(n) != 0 {
			t.Errorf("class %q resolved: findingcontrol path must not flag it, got %v %v", class, p, n)
		}
		// The generic closure obligation still applies to it (as any resolved finding),
		// but with the generic tag — never the findingcontrol one.
		p, _ := effectivenessClosureMessages([]Finding{resolved}, streams)
		if len(p) != 1 || !strings.Contains(p[0], "["+effTagMissing+"]") || strings.Contains(p[0], effTagUnfired) {
			t.Errorf("class %q: want exactly the generic PROBLEM, got %v", class, p)
		}
	}
}

// The three keys round-trip from a real findings entry file into Finding, and the
// whole lint path sees them: a tree with a partial record fails register integrity.
func TestEffectivenessKeysParseFromEntryFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, fm string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\n"+fm+"---\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("F-complete-record.md", `id: F-complete-record
date: "2026-10-09"
title: "complete"
affects: ["x/01"]
resolved: true
effectiveness: "go test ./x -run TestY"
effectiveness-date: "2026-10-10"
effectiveness-by: "human:ian"
`)
	write("F-partial-record.md", `id: F-partial-record
date: "2026-10-09"
title: "partial"
affects: ["x/01"]
resolved: true
effectiveness: "go test ./x -run TestY"
`)
	findings, err := parseFindings(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Finding{}
	for _, f := range findings {
		byID[f.ID] = f
	}
	c := byID["F-complete-record"]
	if c.Effectiveness != "go test ./x -run TestY" || c.EffectivenessDate != "2026-10-10" || c.EffectivenessBy != "human:ian" {
		t.Fatalf("triple did not parse into Finding: %+v", c)
	}
	if p := effectivenessTripleProblems(findings); len(p) != 1 || !strings.Contains(p[0], "F-partial-record") {
		t.Fatalf("want exactly the partial entry flagged, got %v", p)
	}
	var regMsgs []string
	for _, m := range registerIntegrityProblems(root) {
		if strings.Contains(m, effTagPartial) {
			regMsgs = append(regMsgs, m)
		}
	}
	if len(regMsgs) != 1 || !strings.Contains(regMsgs[0], "F-partial-record") {
		t.Fatalf("register integrity must surface the partial record once, got %v", regMsgs)
	}
}
