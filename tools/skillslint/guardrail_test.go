package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fixture helpers -------------------------------------------------------

const fixtureSource = `---
name: guardrails
description: fixture
---

# Shared guardrails

## scrub: bundle

- scrub: alpha-repo#133 => #133
- scrub: Robin => human:<name>

## guardrail: hygiene

Prose the parser must ignore.

- site: .claude/skills/one/SKILL.md
- site: plugins/assay/skills/one/SKILL.md (scrub: bundle)

` + "```text" + `
At most once per hour, run the prune.
Script missing (alpha-repo#133 not yet merged) -> skip silently.
` + "```" + `

## guardrail: labels

- site: .claude/skills/one/SKILL.md

` + "```text" + `
- **Escalation labels (Robin 2026-07-12):** label it and say what you need.
` + "```" + `
`

// writeFixture lays out a minimal repo: the declared source plus the two site
// files, each already carrying the correct (scrubbed where declared) copy.
func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	mk := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mk(guardrailSourcePath, fixtureSource)
	mk(".claude/skills/one/SKILL.md", strings.Join([]string{
		"# one",
		"",
		"### Hourly hygiene tick",
		"",
		"At most once per hour, run the prune.",
		"Script missing (alpha-repo#133 not yet merged) -> skip silently.",
		"",
		"## Rules",
		"",
		"- **Escalation labels (Robin 2026-07-12):** label it and say what you need.",
		"- something else entirely",
		"",
	}, "\n"))
	mk("plugins/assay/skills/one/SKILL.md", strings.Join([]string{
		"# one (bundled)",
		"",
		"### Hourly hygiene tick",
		"",
		"At most once per hour, run the prune.",
		"Script missing (#133 not yet merged) -> skip silently.",
		"",
	}, "\n"))
	return root
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- the baseline ----------------------------------------------------------

func TestCheckGuardrails_CleanFixtureIsClean(t *testing.T) {
	root := writeFixture(t)
	rep := CheckGuardrails(root)
	if len(rep.Unchecked) != 0 {
		t.Fatalf("unexpected could-not-check: %v", rep.Unchecked)
	}
	if len(rep.Failed) != 0 {
		t.Fatalf("unexpected drift: %v", rep.Failed)
	}
	if rep.Compared != 3 {
		t.Fatalf("compared %d copies, want 3 (hygiene x2 + labels x1)", rep.Compared)
	}
	if !rep.Clean() {
		t.Fatal("report should be checked-clean")
	}
}

// --- POSITIVE CONTROLS: a check ships with proof it can fail ---------------

// TestCheckGuardrails_BendACanonicalCopyGoesRed is the mutation test for the
// primary contract. Bend ONE line of ONE copy away from the declared source and
// the check must go from clean to a named drift finding on that exact file and
// line. Without this the whole module could be a no-op nobody noticed.
func TestCheckGuardrails_BendACanonicalCopyGoesRed(t *testing.T) {
	root := writeFixture(t)
	if !CheckGuardrails(root).Clean() {
		t.Fatal("precondition: fixture must start clean")
	}

	body := read(t, root, ".claude/skills/one/SKILL.md")
	bent := strings.Replace(body,
		"Script missing (alpha-repo#133 not yet merged) -> skip silently.",
		"Script missing (alpha-repo#133 not yet merged) -> just skip it, it's fine.", 1)
	if bent == body {
		t.Fatal("mutation did not apply — the control proves nothing")
	}
	write(t, root, ".claude/skills/one/SKILL.md", bent)

	rep := CheckGuardrails(root)
	if len(rep.Failed) != 1 {
		t.Fatalf("bending a copy produced %d findings, want exactly 1: %v", len(rep.Failed), rep.Failed)
	}
	got := rep.Failed[0]
	if got.Path != ".claude/skills/one/SKILL.md" {
		t.Errorf("finding named %s, want the bent file", got.Path)
	}
	for _, want := range []string{"hygiene", "drifted", "want[2]", "got [2]"} {
		if !strings.Contains(got.Msg, want) {
			t.Errorf("finding message lacks %q — it must say which rule and which line:\n%s", want, got.Msg)
		}
	}
	if rep.Clean() {
		t.Error("report claims clean after a copy was bent")
	}
}

// TestCheckGuardrails_BendTheBundleTwinGoesRed proves the scrubbed twin is
// DERIVED, not exempted: editing the published copy on its own is caught even
// though it is never byte-equal to the canonical text.
func TestCheckGuardrails_BendTheBundleTwinGoesRed(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, "plugins/assay/skills/one/SKILL.md")
	write(t, root, "plugins/assay/skills/one/SKILL.md",
		strings.Replace(body, "At most once per hour, run the prune.", "Whenever you feel like it, run the prune.", 1))

	rep := CheckGuardrails(root)
	// The first line IS the anchor, so bending it makes the copy unlocatable —
	// which must be could-not-check, never clean.
	if len(rep.Unchecked) == 0 && len(rep.Failed) == 0 {
		t.Fatal("bending the bundle twin was reported as clean")
	}
	if rep.Clean() {
		t.Fatal("report claims clean after the twin was bent")
	}
}

// TestCheckGuardrails_UnscrubbingTheTwinGoesRed is the publication-safety
// control. The bundle copy must carry the SCRUBBED text; pasting the canonical
// house-slug text into the published bundle has to fail, because that is the
// leak the scrub exists to prevent.
func TestCheckGuardrails_UnscrubbingTheTwinGoesRed(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, "plugins/assay/skills/one/SKILL.md")
	write(t, root, "plugins/assay/skills/one/SKILL.md",
		strings.Replace(body, "(#133 not yet merged)", "(alpha-repo#133 not yet merged)", 1))

	rep := CheckGuardrails(root)
	if len(rep.Failed) != 1 {
		t.Fatalf("un-scrubbing the twin produced %d drift findings, want 1: %+v (unchecked: %v)", len(rep.Failed), rep.Failed, rep.Unchecked)
	}
	if !strings.Contains(rep.Failed[0].Msg, "bundle scrub applied") {
		t.Errorf("finding must say the scrub was applied so the reader knows why the texts differ:\n%s", rep.Failed[0].Msg)
	}
}

// TestCheckGuardrails_DeletedCopyIsCouldNotCheckNotClean is the fail-open
// control. Deleting a copy outright must NOT read as "nothing drifted".
func TestCheckGuardrails_DeletedCopyIsCouldNotCheckNotClean(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, ".claude/skills/one/SKILL.md")
	write(t, root, ".claude/skills/one/SKILL.md",
		strings.Replace(body, "At most once per hour, run the prune.\n", "", 1))

	rep := CheckGuardrails(root)
	if len(rep.Unchecked) != 1 {
		t.Fatalf("deleting a copy produced %d could-not-check findings, want 1: %+v", len(rep.Unchecked), rep.Unchecked)
	}
	if !strings.Contains(rep.Unchecked[0].Msg, "could-not-check") {
		t.Errorf("the finding must be labelled could-not-check verbatim:\n%s", rep.Unchecked[0].Msg)
	}
	if rep.Clean() {
		t.Error("a deleted copy was reported as clean — this is the exact fail-open the module exists to remove")
	}
}

// TestCheckGuardrails_UnreadableSiteIsCouldNotCheck — a site file that vanishes
// entirely is could-not-check, not clean.
func TestCheckGuardrails_UnreadableSiteIsCouldNotCheck(t *testing.T) {
	root := writeFixture(t)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash("plugins/assay/skills/one/SKILL.md"))); err != nil {
		t.Fatal(err)
	}
	rep := CheckGuardrails(root)
	if len(rep.Unchecked) != 1 {
		t.Fatalf("want 1 could-not-check for the missing site, got %+v", rep.Unchecked)
	}
	if rep.Clean() {
		t.Error("a missing site file was reported as clean")
	}
}

// TestCheckGuardrails_MissingSourceIsCouldNotCheck — no declared source means
// the check cannot run at all. It must say so, not pass.
func TestCheckGuardrails_MissingSourceIsCouldNotCheck(t *testing.T) {
	root := writeFixture(t)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(guardrailSourcePath))); err != nil {
		t.Fatal(err)
	}
	rep := CheckGuardrails(root)
	if rep.Compared != 0 || len(rep.Unchecked) != 1 {
		t.Fatalf("want 0 compared + 1 could-not-check, got compared=%d unchecked=%+v", rep.Compared, rep.Unchecked)
	}
	if rep.Clean() {
		t.Error("a missing declared source was reported as clean")
	}
}

// TestCheckGuardrails_AmbiguousAnchorIsCouldNotCheck — two identical anchors in
// one file means the checker cannot tell which copy is the copy. Guessing would
// let a second, unmaintained restatement hide behind a correct one.
func TestCheckGuardrails_AmbiguousAnchorIsCouldNotCheck(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, ".claude/skills/one/SKILL.md")
	write(t, root, ".claude/skills/one/SKILL.md", body+"\nAt most once per hour, run the prune.\n")

	rep := CheckGuardrails(root)
	if len(rep.Unchecked) != 1 || !strings.Contains(rep.Unchecked[0].Msg, "occurs 2 times") {
		t.Fatalf("want 1 could-not-check naming the duplicate anchor, got %+v", rep.Unchecked)
	}
}

// TestCheckGuardrails_StaleScrubRuleIsAFinding — a substitution that matches no
// declared text is a claim nobody checks.
func TestCheckGuardrails_StaleScrubRuleIsAFinding(t *testing.T) {
	root := writeFixture(t)
	src := read(t, root, guardrailSourcePath)
	write(t, root, guardrailSourcePath,
		strings.Replace(src, "- scrub: Robin => human:<name>", "- scrub: Robin => human:<name>\n- scrub: NOTHING-MATCHES-THIS => x", 1))

	rep := CheckGuardrails(root)
	found := false
	for _, is := range rep.Failed {
		if strings.Contains(is.Msg, "NOTHING-MATCHES-THIS") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a stale scrub rule was not reported: %+v", rep.Failed)
	}
}

// --- parser refusals -------------------------------------------------------

func TestParseGuardrailSource_Refusals(t *testing.T) {
	cases := []struct {
		name string
		mut  func(string) string
		want string
	}{
		{
			name: "block with no canonical text",
			mut: func(s string) string {
				return strings.Replace(s, "```text\n- **Escalation labels (Robin 2026-07-12):** label it and say what you need.\n```", "", 1)
			},
			want: "declares no canonical text",
		},
		{
			name: "block with no sites",
			mut: func(s string) string {
				return strings.Replace(s, "- site: .claude/skills/one/SKILL.md\n\n```text\n- **Escalation", "\n```text\n- **Escalation", 1)
			},
			want: "declares no copy sites",
		},
		{
			name: "duplicate guardrail id",
			mut: func(s string) string {
				return s + "\n## guardrail: hygiene\n\n- site: .claude/skills/one/SKILL.md\n\n```text\nx\n```\n"
			},
			want: "declared twice",
		},
		{
			name: "malformed scrub rule",
			mut: func(s string) string {
				return strings.Replace(s, "- scrub: Robin => human:<name>", "- scrub: Robin human:<name>", 1)
			},
			want: "malformed scrub rule",
		},
		{
			name: "unknown site qualifier",
			mut: func(s string) string {
				return strings.Replace(s, "(scrub: bundle)", "(scrub: whatever)", 1)
			},
			want: "unknown site qualifier",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFixture(t)
			write(t, root, guardrailSourcePath, tc.mut(read(t, root, guardrailSourcePath)))
			_, err := ParseGuardrailSource(root)
			if err == nil {
				t.Fatalf("parser accepted a malformed source; want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// --- the regenerating half -------------------------------------------------

// TestSyncGuardrails_RewritesADriftedCopy proves the copies are DERIVED, not
// hand-maintained: bend one, run sync, and the check is clean again with the
// canonical text restored — including the scrub on the bundle side.
func TestSyncGuardrails_RewritesADriftedCopy(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, ".claude/skills/one/SKILL.md")
	write(t, root, ".claude/skills/one/SKILL.md",
		strings.Replace(body, "-> skip silently.", "-> shrug and carry on.", 1))
	if CheckGuardrails(root).Clean() {
		t.Fatal("precondition: the bent copy should not be clean")
	}

	changed, rep, err := SyncGuardrails(root, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("sync reported could-not-check: %+v", rep.Unchecked)
	}
	if len(changed) != 1 || changed[0] != ".claude/skills/one/SKILL.md" {
		t.Fatalf("sync rewrote %v, want exactly the bent file", changed)
	}
	if !CheckGuardrails(root).Clean() {
		t.Fatal("check is still not clean after sync — the generator and the diff disagree")
	}
	if !strings.Contains(read(t, root, ".claude/skills/one/SKILL.md"), "-> skip silently.") {
		t.Error("sync did not restore the canonical text")
	}
	if strings.Contains(read(t, root, ".claude/skills/one/SKILL.md"), "shrug and carry on") {
		t.Error("sync left the hand-edit in place")
	}
}

// TestSyncGuardrails_IsIdempotent — running sync on a clean tree changes
// nothing. A generator that rewrites on every run makes every diff noisy and
// hides the real change.
func TestSyncGuardrails_IsIdempotent(t *testing.T) {
	root := writeFixture(t)
	changed, rep, err := SyncGuardrails(root, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("sync reported could-not-check on a clean tree: %+v", rep.Unchecked)
	}
	if len(changed) != 0 {
		t.Fatalf("sync rewrote %v on an already-clean tree", changed)
	}
}

// TestSyncGuardrails_LeavesAnUnlocatableCopyAlone — sync must never guess where
// a rule belongs. A copy whose anchor is gone is reported, not re-inserted at a
// plausible-looking spot.
func TestSyncGuardrails_LeavesAnUnlocatableCopyAlone(t *testing.T) {
	root := writeFixture(t)
	body := read(t, root, ".claude/skills/one/SKILL.md")
	trimmed := strings.Replace(body, "At most once per hour, run the prune.\n", "", 1)
	write(t, root, ".claude/skills/one/SKILL.md", trimmed)

	changed, rep, err := SyncGuardrails(root, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 1 {
		t.Fatalf("want 1 could-not-check, got %+v", rep.Unchecked)
	}
	for _, c := range changed {
		if c == ".claude/skills/one/SKILL.md" && strings.Contains(read(t, root, c), "At most once per hour") {
			t.Fatal("sync re-inserted a block it could not locate — guessing placement is worse than failing")
		}
	}
}

// --- medici-finance/assay#1690: the growth/shrink removal-boundary defect ---
//
// SyncGuardrails used to compute the removal boundary from len(e.lines) — the
// NEW block's own length — instead of the length of the block actually being
// replaced. Whenever the canonical text's line count changed, that either
// swallowed trailing content that was never part of the block (growth: the
// canonical text got LONGER, so the too-long removal ate whatever followed
// the shorter old copy) or left stale old lines behind (shrink: the mirror
// case). Both tests below fail on the pre-fix code — see the PR body's
// "Fail-first" section for the before/after run against the unfixed
// SyncGuardrails(root) single-argument signature.
//
// oldGuardrailSource parses a HAND-WRITTEN previous revision of
// GUARDRAILS.md — standing in for what previousGuardrailSource would fetch
// from git in production — so the fix can be exercised without a real git
// repo in the fixture.
func oldGuardrailSource(t *testing.T, body string) *GuardrailSource {
	t.Helper()
	src, err := parseGuardrailBytes([]byte(body))
	if err != nil {
		t.Fatalf("parsing the fixture's old source: %v", err)
	}
	return src
}

// mkdirWrite is write, but creates the parent directory first — write alone
// assumes writeFixture already laid out the tree, which these two ad hoc
// fixtures do not.
func mkdirWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, root, rel, body)
}

// TestSyncGuardrails_GrowthKeepsTrailingContent reproduces the
// medici-finance/assay#1687 incident: the canonical block grows from 3 lines
// to 5, and the site still carries the old 3-line copy immediately followed
// by two lines that are NOT part of the block. On the unfixed tool, `end` is
// computed from the NEW block's length (5), so the rewrite silently consumes
// both trailing lines.
func TestSyncGuardrails_GrowthKeepsTrailingContent(t *testing.T) {
	root := t.TempDir()

	oldSource := "---\nname: guardrails\ndescription: fixture\n---\n\n## guardrail: widen\n\n- site: .claude/skills/two/SKILL.md\n\n```text\n- **Widen test:** anchor line, stable across edits.\n- old middle line, unrelated wording.\n- old tail line, unrelated wording.\n```\n"
	newSource := "---\nname: guardrails\ndescription: fixture\n---\n\n## guardrail: widen\n\n- site: .claude/skills/two/SKILL.md\n\n```text\n- **Widen test:** anchor line, stable across edits.\n- brand new middle line one.\n- brand new middle line two.\n- brand new middle line three.\n- brand new tail line.\n```\n"
	mkdirWrite(t, root, guardrailSourcePath, newSource)
	old := oldGuardrailSource(t, oldSource)

	site := strings.Join([]string{
		"- **Widen test:** anchor line, stable across edits.",
		"- old middle line, unrelated wording.",
		"- old tail line, unrelated wording.",
		"- something completely unrelated that must survive.",
		"- trailing padding line so a wrong boundary does not just hit EOF.",
	}, "\n")
	mkdirWrite(t, root, ".claude/skills/two/SKILL.md", site)

	changed, rep, err := SyncGuardrails(root, old)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("sync reported could-not-check: %+v", rep.Unchecked)
	}
	if len(changed) != 1 {
		t.Fatalf("sync rewrote %v, want exactly the one site", changed)
	}

	got := read(t, root, ".claude/skills/two/SKILL.md")
	wantBlock := "- **Widen test:** anchor line, stable across edits.\n" +
		"- brand new middle line one.\n" +
		"- brand new middle line two.\n" +
		"- brand new middle line three.\n" +
		"- brand new tail line."
	if !strings.HasPrefix(got, wantBlock) {
		t.Errorf("site does not carry the new (longer) block text:\n%s", got)
	}
	if !strings.Contains(got, "something completely unrelated that must survive") {
		t.Errorf("GROWTH BUG: the unrelated trailing line was swallowed by the rewrite:\n%s", got)
	}
	if !strings.Contains(got, "trailing padding line") {
		t.Errorf("GROWTH BUG: the second trailing line was swallowed by the rewrite:\n%s", got)
	}
}

// TestSyncGuardrails_ShrinkDropsStaleLines is the mirror defect: the canonical
// block shrinks from 5 lines to 2, and the site still carries the old 5-line
// copy. On the unfixed tool, `end` is computed from the NEW (shorter) length,
// leaving 3 stale lines from the old, longer block behind instead of removing
// them.
func TestSyncGuardrails_ShrinkDropsStaleLines(t *testing.T) {
	root := t.TempDir()

	oldSource := "---\nname: guardrails\ndescription: fixture\n---\n\n## guardrail: narrow\n\n- site: .claude/skills/three/SKILL.md\n\n```text\n- **Narrow test:** anchor line, stable across edits.\n- old middle line A, being trimmed away.\n- old middle line B, being trimmed away.\n- old middle line C, being trimmed away.\n- old tail line, being trimmed away.\n```\n"
	newSource := "---\nname: guardrails\ndescription: fixture\n---\n\n## guardrail: narrow\n\n- site: .claude/skills/three/SKILL.md\n\n```text\n- **Narrow test:** anchor line, stable across edits.\n- new tail line after the trim.\n```\n"
	mkdirWrite(t, root, guardrailSourcePath, newSource)
	old := oldGuardrailSource(t, oldSource)

	site := strings.Join([]string{
		"- **Narrow test:** anchor line, stable across edits.",
		"- old middle line A, being trimmed away.",
		"- old middle line B, being trimmed away.",
		"- old middle line C, being trimmed away.",
		"- old tail line, being trimmed away.",
		"- something completely unrelated that must survive.",
	}, "\n")
	mkdirWrite(t, root, ".claude/skills/three/SKILL.md", site)

	changed, rep, err := SyncGuardrails(root, old)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("sync reported could-not-check: %+v", rep.Unchecked)
	}
	if len(changed) != 1 {
		t.Fatalf("sync rewrote %v, want exactly the one site", changed)
	}

	got := read(t, root, ".claude/skills/three/SKILL.md")
	wantBlock := "- **Narrow test:** anchor line, stable across edits.\n- new tail line after the trim."
	if !strings.HasPrefix(got, wantBlock) {
		t.Errorf("site does not carry the new (shorter) block text:\n%s", got)
	}
	if strings.Contains(got, "being trimmed away") {
		t.Errorf("SHRINK BUG: stale lines from the old, longer block were left behind:\n%s", got)
	}
	if !strings.Contains(got, "something completely unrelated that must survive") {
		t.Errorf("the unrelated trailing line should never have been touched either way:\n%s", got)
	}
}
