package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBriefWithContext writes a structurally-valid brief-v1 file with a
// `## Context` (carrying a bulleted `files:` declared-paths line), a `## Task`
// and a `## Verify` section, and returns its path.
func writeBriefWithContext(t *testing.T, dir, num string, declaredPaths []string, task, verifyBody string) string {
	t.Helper()
	var files strings.Builder
	files.WriteString("files:\n")
	for _, p := range declaredPaths {
		files.WriteString("- `" + p + "` — declared\n")
	}
	fm := "---\n" +
		"brief: t/" + num + "\n" +
		"title: fixture\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"authored: 2026-08-25 by test\n" +
		"sources: [\"fixture\"]\n" +
		"schema: brief-v1\n" +
		"---\n\n# Fixture\n\n## Context\n\n" + files.String() +
		"\n## Task\n" + task + "\n\n## Verify\n" + verifyBody + "\n\n## Evidence\n\n## Review\nGate: model.\n"
	path := filepath.Join(dir, "brief-"+num+".md")
	if err := os.WriteFile(path, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// streamForBrief builds a Stream rooted at root with its dir at
// root/docs/streams/t and writes one brief into it, returning (stream, briefRel).
func streamForBrief(t *testing.T, root, num string, declaredPaths []string, task, verifyBody string) (*Stream, string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "t")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeBriefWithContext(t, dir, num, declaredPaths, task, verifyBody)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return &Stream{Name: "t", Dir: dir, Root: root}, filepath.ToSlash(rel)
}

// withBranchChangedSet temporarily overrides the branch-diff helper so a test can
// inject the shape of a change (or a could-not-check) without a git fixture.
func withBranchChangedSet(t *testing.T, fn func(root string) (map[string]bool, bool)) {
	t.Helper()
	orig := branchChangedSet
	branchChangedSet = fn
	t.Cleanup(func() { branchChangedSet = orig })
}

const noObligationRows = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |"

func withObligationRow(cls string) string {
	return "| # | Class | Command | Expect |\n|---|-------|---------|--------|\n| 1 | " + cls + " | `true` | exit 0 |"
}

// TestObligationDerivation is the positive control (Verify row 8): for each
// derived obligation, the triggering diff shape with the row ABSENT reports it,
// and the same shape with the row PRESENT is silent.
func TestObligationDerivation(t *testing.T) {
	type tc struct {
		name       string
		declared   []string
		task       string
		changed    func(root, briefRel string) map[string]bool
		obligation string
		presentRow string // a Class cell that satisfies the obligation
	}
	cases := []tc{
		{
			name:     "mutation",
			declared: []string{"statusgen/foo.go"},
			task:     "Extend the lint.",
			changed: func(root, briefRel string) map[string]bool {
				return map[string]bool{briefRel: true, "statusgen/foo.go": true}
			},
			obligation: classMutation,
			presentRow: "check +mutation",
		},
		{
			name:       "flow",
			declared:   []string{"alpha/one.txt", "beta/two.txt"}, // spans two top-level dirs, neither check-shaped nor .md
			task:       "Wire the two sides.",
			changed:    func(root, briefRel string) map[string]bool { return map[string]bool{briefRel: true} },
			obligation: classFlow,
			presentRow: "check +flow",
		},
		{
			name:       "flow-via-keyword",
			declared:   []string{"alpha/one.txt"}, // single dir — the keyword is what triggers flow
			task:       "This exercises the cross-component path end to end.",
			changed:    func(root, briefRel string) map[string]bool { return map[string]bool{briefRel: true} },
			obligation: classFlow,
			presentRow: "check +flow",
		},
		{
			name:       "dereference",
			declared:   []string{"docs/guide.md"}, // a fact-asserting deliverable, single dir
			task:       "Document the values.",
			changed:    func(root, briefRel string) map[string]bool { return map[string]bool{briefRel: true} },
			obligation: classDereference,
			presentRow: "gate:model +dereference",
		},
	}

	for _, c := range cases {
		t.Run(c.name+"/absent-fires", func(t *testing.T) {
			root := t.TempDir()
			s, briefRel := streamForBrief(t, root, "01", c.declared, c.task, noObligationRows)
			withBranchChangedSet(t, func(string) (map[string]bool, bool) { return c.changed(root, briefRel), true })
			p, n := verifyObligationDerivation(root, []*Stream{s})
			if len(p) != 0 {
				t.Fatalf("advisory phase must not produce PROBLEMs, got %v", p)
			}
			hit := false
			for _, m := range n {
				if strings.Contains(m, "+"+c.obligation) && strings.Contains(m, "["+ruleObligationDerivation+"]") {
					hit = true
				}
			}
			if !hit {
				t.Fatalf("obligation %q owed but absent: got notices %v, want one naming +%s", c.obligation, n, c.obligation)
			}
		})
		t.Run(c.name+"/present-silent", func(t *testing.T) {
			root := t.TempDir()
			s, briefRel := streamForBrief(t, root, "01", c.declared, c.task, withObligationRow(c.presentRow))
			withBranchChangedSet(t, func(string) (map[string]bool, bool) { return c.changed(root, briefRel), true })
			_, n := verifyObligationDerivation(root, []*Stream{s})
			for _, m := range n {
				if strings.Contains(m, "+"+c.obligation) {
					t.Fatalf("obligation %q present but still reported: %q", c.obligation, m)
				}
			}
		})
	}
}

// TestObligationDerivation_UntouchedBriefSilent proves the transition scope: a
// brief this branch did NOT edit is not evaluated, even when it would owe an
// obligation — so the inherited corpus does not flood.
func TestObligationDerivation_UntouchedBriefSilent(t *testing.T) {
	root := t.TempDir()
	s, _ := streamForBrief(t, root, "01", []string{"docs/guide.md"}, "Document.", noObligationRows)
	// The brief's own file is NOT in the diff → out of scope.
	withBranchChangedSet(t, func(string) (map[string]bool, bool) { return map[string]bool{"statusgen/other.go": true}, true })
	_, n := verifyObligationDerivation(root, []*Stream{s})
	if len(n) != 0 {
		t.Fatalf("untouched brief must not be evaluated, got %v", n)
	}
}

// TestObligationDerivationCouldNotCheck (Verify row 9): an unavailable diff
// reports could-not-check, never "nothing is owed".
func TestObligationDerivationCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	s, _ := streamForBrief(t, root, "01", []string{"statusgen/foo.go"}, "Extend the lint.", noObligationRows)
	withBranchChangedSet(t, func(string) (map[string]bool, bool) { return nil, false })
	p, n := verifyObligationDerivation(root, []*Stream{s})
	if len(p) != 0 {
		t.Fatalf("advisory phase must not produce PROBLEMs, got %v", p)
	}
	if len(n) != 1 || !strings.Contains(n[0], "COULD-NOT-CHECK") {
		t.Fatalf("unavailable diff: got %v, want one could-not-check NOTICE", n)
	}
	if strings.Contains(strings.ToLower(n[0]), "owes nothing") && !strings.Contains(n[0], "not a change that owes nothing") {
		t.Fatalf("could-not-check must not round down to nothing-owed: %q", n[0])
	}
}

// TestOwedObligations covers the derivation triggers as a pure function, incl.
// the inverse (not-owed) case for each — the neighbour obligation is validated
// but never derived here.
func TestOwedObligations(t *testing.T) {
	// mutation owed only when a check-shaped path this brief owns is in the diff.
	owed := owedObligations(obligationInputs{
		declaredPaths:      []string{"statusgen/"},
		declaredPathsFound: true,
		changed:            map[string]bool{"statusgen/rowclass.go": true},
	})
	if !owed[classMutation] {
		t.Error("mutation: a check-shaped path under a declared dir in the diff should owe mutation")
	}
	// inverse: same declared path, but the diff does not touch it → not owed.
	owed = owedObligations(obligationInputs{
		declaredPaths:      []string{"statusgen/"},
		declaredPathsFound: true,
		changed:            map[string]bool{"docs/x.md": true},
	})
	if owed[classMutation] {
		t.Error("mutation: no check-shaped path in the diff should NOT owe mutation")
	}
	// flow via span.
	owed = owedObligations(obligationInputs{
		declaredPaths:      []string{"alpha/a.txt", "beta/b.txt"},
		declaredPathsFound: true,
	})
	if !owed[classFlow] {
		t.Error("flow: declared paths spanning two top-level dirs should owe flow")
	}
	// inverse: single top-level dir, no keyword → no flow.
	owed = owedObligations(obligationInputs{
		declaredPaths:      []string{"alpha/a.txt", "alpha/b.txt"},
		declaredPathsFound: true,
	})
	if owed[classFlow] {
		t.Error("flow: a single top-level dir with no keyword should NOT owe flow")
	}
	// dereference via a fact-asserting deliverable; a README/brief is excluded.
	if !owedObligations(obligationInputs{declaredPaths: []string{"docs/guide.md"}, declaredPathsFound: true})[classDereference] {
		t.Error("dereference: a non-README markdown deliverable should owe dereference")
	}
	if owedObligations(obligationInputs{declaredPaths: []string{"docs/streams/t/README.md"}, declaredPathsFound: true})[classDereference] {
		t.Error("dereference: a stream README is scaffolding, not a fact-asserting deliverable")
	}
	// neighbour is never auto-derived.
	all := owedObligations(obligationInputs{
		declaredPaths:      []string{"statusgen/", "alpha/a.txt", "beta/b.txt", "docs/guide.md"},
		declaredPathsFound: true,
		changed:            map[string]bool{"statusgen/rowclass.go": true},
	})
	if all[classNeighbour] {
		t.Error("neighbour must not be auto-derived (validated only, per the header)")
	}
}
