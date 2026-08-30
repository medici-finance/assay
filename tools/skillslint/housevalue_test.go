package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	redTree   = "testdata/plugintree/unresolved"
	greenTree = "testdata/plugintree/neutral"
)

// TestLintPluginTree_RedFixture is the fail-first arm: the fixture tree carries a
// placeholder proper name in all three driver positions, in a REFERENCE and in a
// README — the two file kinds the skill-body-only lint never read. Every one must
// be reported, with file:line and the offending span.
func TestLintPluginTree_RedFixture(t *testing.T) {
	checked, issues, err := LintPluginTree(redTree)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if checked != 2 {
		t.Errorf("checked = %d, want 2 (the reference and the README)", checked)
	}
	if len(issues) == 0 {
		t.Fatal("the red fixture resolves the driver to a proper name in three positions and was reported clean")
	}

	byPath := map[string][]string{}
	for _, is := range issues {
		byPath[is.Path] = append(byPath[is.Path], is.Msg)
	}

	// The widened scope is the point: a reference file AND a README, neither of
	// which matches plugins/assay/skills/*/SKILL.md.
	for _, want := range []string{
		"plugins/assay/references/example-harness.md",
		"plugins/assay/README.md",
	} {
		if len(byPath[want]) == 0 {
			t.Errorf("no issue reported for %s — the widened walk did not read it; got %v", want, keys(byPath))
		}
	}

	ref := strings.Join(byPath["plugins/assay/references/example-harness.md"], "\n")
	// One finding per driver position, and the wrapped attribution is reported on
	// the line the NAME is on, not the line the date is on.
	for _, want := range []string{
		"line 9:",               // dated attribution — reported on the NAME's line, not the date's
		"line 14:",              // possessive
		"line 16:",              // driver lead-in
		"dated attribution",     // the position is named in the message
		"possessive",            //
		"driver lead-in",        //
		`"Alice"`,               // the offending token
		"Alice, 2026-08-26",     // the span, rendered on one line
		"driver-allowlist.txt",  // the message says how a real product name is cleared
		"never a person's name", //
	} {
		if !strings.Contains(ref, want) {
			t.Errorf("reference findings missing %q:\n%s", want, ref)
		}
	}

	// The legitimate capitalised words in the same fixture must NOT be reported —
	// a check that fires on "Cursor's" or "track B's" is unusable.
	all := strings.Join(append(byPath["plugins/assay/references/example-harness.md"],
		byPath["plugins/assay/README.md"]...), "\n")
	for _, forbidden := range []string{`"Cursor"`, `"GitHub"`, `"Claude Code"`, `"App"`, `"The App"`, `"B"`, `"A"`, `"R6"`} {
		if strings.Contains(all, forbidden) {
			t.Errorf("false positive: %s reported as a house value:\n%s", forbidden, all)
		}
	}
}

// TestLintPluginTree_NeutralFixture is the positive control for the red arm: the
// same prose with `human:<name>` in every driver position must be clean. Without
// it, the red above could be firing on the surrounding sentence rather than on
// the name.
func TestLintPluginTree_NeutralFixture(t *testing.T) {
	checked, issues, err := LintPluginTree(greenTree)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if checked != 2 {
		t.Errorf("checked = %d, want 2", checked)
	}
	if len(issues) != 0 {
		t.Fatalf("the neutral fixture uses human:<name> in every driver position and must be clean, got:\n%v", issues)
	}
}

// TestLintPluginTree_RedAndNeutralDifferOnlyInTheToken pins the two fixtures to
// each other: if a later edit lets them drift apart, the pass/fail difference
// stops being attributable to the driver token alone and the control is worthless.
func TestLintPluginTree_RedAndNeutralDifferOnlyInTheToken(t *testing.T) {
	for _, rel := range []string{
		"plugins/assay/references/example-harness.md",
		"plugins/assay/README.md",
	} {
		red := readFixture(t, filepath.Join(redTree, rel))
		green := readFixture(t, filepath.Join(greenTree, rel))
		if strings.Contains(green, "Alice") {
			t.Errorf("%s: the neutral fixture still carries the placeholder name", rel)
		}
		if !strings.Contains(red, "Alice") {
			t.Errorf("%s: the red fixture no longer carries the placeholder name", rel)
		}
		normRed := strings.ReplaceAll(red, "Alice", "<TOKEN>")
		normGreen := strings.ReplaceAll(green, "human:<name>", "<TOKEN>")
		if normRed != normGreen {
			t.Errorf("%s: the two fixtures differ by more than the driver token — the control no longer isolates it\nred:\n%s\ngreen:\n%s",
				rel, normRed, normGreen)
		}
	}
}

// TestLintPluginTree_FailsClosed — a root with no plugin tree, or a plugin tree
// with no markdown in it, is could-not-check, never a quiet pass.
func TestLintPluginTree_FailsClosed(t *testing.T) {
	empty := t.TempDir()
	if _, _, err := LintPluginTree(empty); err == nil {
		t.Error("a root with no plugins/ directory returned no error — nothing to lint must never read as a pass")
	}

	noMD := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noMD, "plugins", "assay"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noMD, "plugins", "assay", "hook.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LintPluginTree(noMD); err == nil {
		t.Error("a plugins/ tree with no *.md returned no error — a check that read nothing proved nothing")
	}
}

// TestDriverPositions_Shapes covers the recognised positions and the shapes that
// are out of scope by construction, without going through the filesystem.
func TestDriverPositions_Shapes(t *testing.T) {
	allow := driverAllowlist()
	if len(allow) == 0 {
		t.Fatal("the embedded driver allowlist parsed to zero entries")
	}

	flagged := []string{
		"ruled by Alice, 2026-08-26 on the record",
		"the merge is Alice's alone",
		"driver Alice; the human decides",
		"the driver is Bartholomew and nobody else",
		"wrapped attribution (Alice,\n2026-08-26) still counts",
		"a typographic apostrophe in Alice\u2019s ruling",
	}
	for _, s := range flagged {
		if got := houseValueIssues(s, allow); len(got) == 0 {
			t.Errorf("expected a finding for %q, got none", s)
		}
	}

	clean := []string{
		"ruled by human:<name>, 2026-08-26 on the record",
		"the merge is human:<name>'s alone",
		"driver human:<name>; the human decides",
		"bound by capability:dispatch-worker, 2026-08-26",
		"Cursor's matrix and GitHub's API and Claude Code's hooks",
		"the App's identity and the PR's head and the README's table",
		"track B's search beat track A's filing",
		"probe vs assertion (R6, 2026-07-10)",
		"the reviewer is the one who decides",
		"Everyone's queue drains eventually",
	}
	for _, s := range clean {
		if got := houseValueIssues(s, allow); len(got) != 0 {
			t.Errorf("false positive on %q: %v", s, got)
		}
	}
}

// TestDriverAllowlist_NoObviousPersonEntry pins the neutrality property the check
// is built on: the allowlist is for product/tool nouns, and the moment it starts
// carrying human-shaped entries the shape check is being laundered rather than
// satisfied. It cannot know which words are people, so it pins what it can — the
// file must document the prohibition, and it must not carry the fixtures' own
// placeholder name.
func TestDriverAllowlist_NoObviousPersonEntry(t *testing.T) {
	if !strings.Contains(driverAllowlistRaw, "NEVER belongs here") {
		t.Error("driver-allowlist.txt no longer states that a person's name never belongs in it")
	}
	if driverAllowlist()["Alice"] {
		t.Error("the fixtures' placeholder name has been allowlisted — the red fixture would silently stop proving anything")
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
