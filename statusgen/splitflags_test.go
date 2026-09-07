package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// splitBrief is a minimal, fully-valid brief-v1 frontmatter for the split-flag
// conservation tests: only the flags this rule reads (gate + the four canonical
// risk answers) and an optional split-from parent vary between fixtures.
type splitBrief struct {
	num       string // "02", "02a"
	gate      string // model | human
	reg       string // regulatory  yes|no
	cust      string // customer     yes|no
	irr       string // irreversible yes|no
	sens      string // sensitive-data yes|no
	splitFrom string // "" or "<stream>/<NN>"
}

// writeSplitBrief materializes one brief file under a stream dir. It writes only
// the fields parseBriefFile requires plus the flags this rule compares, so a
// fixture reads as the flag-shape it is testing and nothing else.
func writeSplitBrief(t *testing.T, dir, stream string, b splitBrief) {
	t.Helper()
	var sf string
	if b.splitFrom != "" {
		sf = fmt.Sprintf("split-from: %s\n", b.splitFrom)
	}
	body := fmt.Sprintf(`---
schema: brief-v1
brief: %s/%s
title: shard %s
wave: 1
depends: []
unblocks: []
effort: M
gate: %s
risk:
  regulatory: %s
  customer: %s
  irreversible: %s
  sensitive-data: %s
issues: []
authored: 2026-07-13
sources:
  - docs/streams/%s/README.md
%s---
## Context
A brief.
`, stream, b.num, b.num, b.gate, b.reg, b.cust, b.irr, b.sens, stream, sf)
	fn := filepath.Join(dir, "brief-"+b.num+"-x.md")
	if err := os.WriteFile(fn, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// splitStream writes a stream dir full of brief fixtures and returns the *Stream
// (Name/Dir only — splitFlagProblems re-parses the files, it does not need a
// README or a loaded row table).
func splitStream(t *testing.T, root, name string, briefs ...splitBrief) *Stream {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, b := range briefs {
		writeSplitBrief(t, dir, name, b)
	}
	return &Stream{Name: name, Dir: dir}
}

// TestSplitFlagConservation is the fail-first proof of the gate-integrity fix: a
// human-gated / risk-yes parent split into a shard that declares model-gate /
// lower-risk is REFUSED, whether the parent is retained, retired (its strictest
// sibling standing in), or named across streams via split-from — while a
// conserving or escalating split, and a plain lettered brief with no split, stay
// silent.
func TestSplitFlagConservation(t *testing.T) {
	const strict = "human"
	const loose = "model"

	t.Run("retired parent — a downgraded shard is caught via its faithful sibling", func(t *testing.T) {
		// The incident: example-app/02 (human, all risk yes) split into 02a
		// (downgraded to model, all no), 02b (faithful), 02c (downgraded). The
		// parent 02 was retired in the same change, so the floor is the strictest
		// sibling (02b).
		root := t.TempDir()
		s := splitStream(t, root, "example-app",
			splitBrief{num: "02a", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no"},
			splitBrief{num: "02b", gate: strict, reg: "yes", cust: "yes", irr: "yes", sens: "yes"},
			splitBrief{num: "02c", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no"},
		)
		problems, _ := splitFlagProblems([]*Stream{s}, []*Stream{s})

		// 02a and 02c are downgrades; 02b (the floor) must be clean.
		if !hasProblem(problems, "brief-02a-x.md", "gate is \"model\"") {
			t.Errorf("want 02a flagged for a gate downgrade; got:\n%s", strings.Join(problems, "\n"))
		}
		if !hasProblem(problems, "brief-02a-x.md", "risk.irreversible") {
			t.Errorf("want 02a flagged for an irreversible downgrade; got:\n%s", strings.Join(problems, "\n"))
		}
		if !hasProblem(problems, "brief-02c-x.md", "gate is \"model\"") {
			t.Errorf("want 02c flagged for a gate downgrade; got:\n%s", strings.Join(problems, "\n"))
		}
		if hasProblem(problems, "brief-02b-x.md") {
			t.Errorf("02b conserves the flags and must be silent; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("retained parent — a downgraded shard is caught against the parent directly", func(t *testing.T) {
		root := t.TempDir()
		s := splitStream(t, root, "example-app",
			splitBrief{num: "02", gate: strict, reg: "yes", cust: "yes", irr: "yes", sens: "yes"},
			splitBrief{num: "02a", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no"},
		)
		problems, _ := splitFlagProblems([]*Stream{s}, []*Stream{s})
		if !hasProblem(problems, "brief-02a-x.md", "is a shard of example-app/02", "gate is \"model\"") {
			t.Errorf("want 02a flagged against the retained parent 02; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("conserving split — an equal-or-stricter shard is silent", func(t *testing.T) {
		root := t.TempDir()
		s := splitStream(t, root, "example-app",
			splitBrief{num: "02", gate: strict, reg: "yes", cust: "yes", irr: "yes", sens: "yes"},
			// 02a conserves every flag; 02b escalates a flag the parent had off.
			splitBrief{num: "02a", gate: strict, reg: "yes", cust: "yes", irr: "yes", sens: "yes"},
		)
		problems, _ := splitFlagProblems([]*Stream{s}, []*Stream{s})
		if hasProblem(problems, "brief-02a-x.md") {
			t.Errorf("a conserving shard must be silent; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("escalation is allowed — a stricter shard than a loose parent is silent", func(t *testing.T) {
		root := t.TempDir()
		s := splitStream(t, root, "gtm",
			splitBrief{num: "05", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no"},
			splitBrief{num: "05a", gate: strict, reg: "yes", cust: "no", irr: "no", sens: "no"},
		)
		problems, _ := splitFlagProblems([]*Stream{s}, []*Stream{s})
		if hasProblem(problems, "brief-05a-x.md") {
			t.Errorf("a shard STRICTER than its parent is an escalation, never a downgrade; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("split-from — a cross-stream downgrade is caught", func(t *testing.T) {
		root := t.TempDir()
		parent := splitStream(t, root, "example-app",
			splitBrief{num: "02", gate: strict, reg: "yes", cust: "yes", irr: "yes", sens: "no"},
		)
		child := splitStream(t, root, "frontend",
			splitBrief{num: "07", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no",
				splitFrom: "example-app/02"},
		)
		problems, _ := splitFlagProblems([]*Stream{parent, child}, []*Stream{parent, child})
		if !hasProblem(problems, "brief-07-x.md", "was split from example-app/02", "risk.customer") {
			t.Errorf("want the split-from child flagged for a customer downgrade; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("split-from to a retired parent is a could-not-check NOTICE, not a pass", func(t *testing.T) {
		root := t.TempDir()
		child := splitStream(t, root, "frontend",
			splitBrief{num: "07", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no",
				splitFrom: "example-app/02"},
		)
		problems, notices := splitFlagProblems([]*Stream{child}, []*Stream{child})
		if hasProblem(problems, "brief-07-x.md") {
			t.Errorf("an unresolved split-from must not be a hard PROBLEM; got:\n%s", strings.Join(problems, "\n"))
		}
		if !hasProblem(notices, "brief-07-x.md", "not a brief in the tree") {
			t.Errorf("want a could-not-check NOTICE for the retired parent; got:\n%s", strings.Join(notices, "\n"))
		}
	})

	t.Run("a lone lettered brief with no sibling and no parent is silent", func(t *testing.T) {
		root := t.TempDir()
		s := splitStream(t, root, "gtm",
			splitBrief{num: "09a", gate: loose, reg: "no", cust: "no", irr: "no", sens: "no"},
		)
		problems, _ := splitFlagProblems([]*Stream{s}, []*Stream{s})
		if hasProblem(problems, "brief-09a-x.md") {
			t.Errorf("a lone shard has nothing to downgrade from and must be silent; got:\n%s", strings.Join(problems, "\n"))
		}
	})
}
