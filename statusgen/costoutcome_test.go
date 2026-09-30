package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test names stay under 42 characters (the pre-push secret-scan floor the
// requirements_test.go header explains).

// costBrief writes one valid brief-v1 file with the given authored date and extra
// frontmatter lines (spelled verbatim), in a stream dir under root, and returns
// the stream.
func costBrief(t *testing.T, root, authored, extra string) *Stream {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "example-stream")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`---
brief: example-stream/01
title: fixture brief
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: %s by fixture
sources: ["fixture"]
%s---

# Brief 01

## Context
files:
- none

facts:
- k: v
`, authored, extra)
	if err := os.WriteFile(filepath.Join(dir, "brief-01.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Stream{Name: "example-stream", Dir: dir, Root: root}
}

// TestOutcomeUnknownMetric: an outcome that is not a registered requirement id
// is a PROBLEM — whether it is not an id at all or a well-formed id the register
// does not define — while a registered id and the explicit `none` are clean.
func TestOutcomeUnknownMetric(t *testing.T) {
	req := validRequirementFixture() // REQ-evidence-visible
	cases := []struct {
		name, line  string
		wantProblem bool
		echo        string
	}{
		{"not-an-id", "outcome: not-a-metric\n", true, "not-a-metric"},
		{"unregistered", "outcome: REQ-glossary-first\n", true, "REQ-glossary-first"},
		{"registered", "outcome: REQ-evidence-visible\n", false, ""},
		{"none", "outcome: none\n", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := requirementRoot(t, req)
			s := costBrief(t, root, "2026-09-24", c.line)
			problems, _ := checkBriefFiles([]*Stream{s}, []*Stream{s})
			got := mentioning(problems, "outcome")
			if c.wantProblem && (got != 1 || !containsAll(problems, "outcome", c.echo)) {
				t.Errorf("%s must be ONE outcome PROBLEM echoing %q; got:\n%s", c.line, c.echo, joined(problems))
			}
			if !c.wantProblem && got != 0 {
				t.Errorf("%s must be clean; got:\n%s", c.line, joined(problems))
			}
		})
	}
}

// TestOutcomeCrossRepoCouldNotCheck: a cross-repo requirement id names a register
// the offline linter cannot read — a could-not-check NOTICE, never a PROBLEM and
// never silence.
func TestOutcomeCrossRepoCouldNotCheck(t *testing.T) {
	root := requirementRoot(t, validRequirementFixture())
	writeFile(t, root, "docs/streams/graph-repos.yaml",
		"schema: graph-repos-v1\ncell: example\nself: app\nrepos:\n  app: {cell: example, repo: example-org/app}\n")
	s := costBrief(t, root, "2026-09-24", "outcome: app:REQ-evidence-visible\n")
	problems, notices := checkBriefFiles([]*Stream{s}, []*Stream{s})
	if mentioning(problems, "outcome") != 0 {
		t.Errorf("a cross-repo outcome must not be a PROBLEM; got:\n%s", joined(problems))
	}
	if !containsAll(notices, "outcome", "could-not-check") {
		t.Errorf("a cross-repo outcome must be a could-not-check NOTICE; got:\n%s", joined(notices))
	}
}

// TestOutcomeWrongTypeIsParseError: a non-string outcome is a parse error.
func TestOutcomeWrongTypeIsParseError(t *testing.T) {
	s := costBrief(t, t.TempDir(), "2026-09-24", "outcome: [REQ-evidence-visible]\n")
	if _, _, err := parseBriefFile(filepath.Join(s.Dir, "brief-01.md")); err == nil {
		t.Errorf("a list outcome: must be a parse error")
	}
}

// TestBudgetUnitRequired: a budget states what it is counted in. A bare number
// (which YAML parses as an int) is a PROBLEM naming the missing unit; tokens and
// a currency code are clean.
func TestBudgetUnitRequired(t *testing.T) {
	cases := []struct {
		line        string
		wantProblem bool
	}{
		{"budget: 400\n", true},
		{"budget: 400k\n", true},
		{"budget: \"400 dollars\"\n", true},
		{"budget: 0 tokens\n", true},
		{"budget: 1.5m tokens\n", true}, // lower-case m could read as milli; only k and M are multipliers
		{"budget: 400k tokens\n", false},
		{"budget: 1.5M tokens\n", false},
		{"budget: 25 USD\n", false},
	}
	for _, c := range cases {
		s := costBrief(t, t.TempDir(), "2026-09-24", c.line)
		problems, _ := checkBriefFiles([]*Stream{s}, []*Stream{s})
		got := mentioning(problems, "budget")
		if c.wantProblem && got != 1 {
			t.Errorf("%q must be ONE budget PROBLEM; got:\n%s", c.line, joined(problems))
		}
		if !c.wantProblem && got != 0 {
			t.Errorf("%q must be clean; got:\n%s", c.line, joined(problems))
		}
	}
	s := costBrief(t, t.TempDir(), "2026-09-24", "budget: 400\n")
	problems, _ := checkBriefFiles([]*Stream{s}, []*Stream{s})
	if !containsAll(problems, "budget", "no unit") {
		t.Errorf("a unitless budget must say it has no unit; got:\n%s", joined(problems))
	}
}

// TestOutcomeAbsentNotice: a post-cutover brief with no outcome: is a NOTICE;
// `outcome: none` silences it, and a pre-cutover brief is grandfathered.
func TestOutcomeAbsentNotice(t *testing.T) {
	post := costBrief(t, t.TempDir(), "2026-09-24", "")
	if n := outcomeAbsentNotices([]*Stream{post}); len(n) != 1 || !strings.Contains(n[0], tagOutcomeAbsent) {
		t.Errorf("a post-cutover brief with no outcome must raise one [%s] NOTICE; got:\n%s", tagOutcomeAbsent, joined(n))
	}
	none := costBrief(t, t.TempDir(), "2026-09-24", "outcome: none\n")
	if n := outcomeAbsentNotices([]*Stream{none}); len(n) != 0 {
		t.Errorf("outcome: none is a decision and must silence the NOTICE; got:\n%s", joined(n))
	}
	legacy := costBrief(t, t.TempDir(), outcomeLineCutover, "")
	if n := outcomeAbsentNotices([]*Stream{legacy}); len(n) != 0 {
		t.Errorf("a brief authored on/before the cutover is grandfathered; got:\n%s", joined(n))
	}
}
