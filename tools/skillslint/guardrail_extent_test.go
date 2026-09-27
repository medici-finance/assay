package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- medici-finance/assay#1690 round 2: the removal extent must be PROVEN ---
//
// Round 1 took the removal length from HEAD's GUARDRAILS.md without checking
// that the lines about to be removed actually ARE that block. Three orderings
// broke it (review findings F-1692-sync-rerun-boundary / F-1690-rerun-dataloss,
// F-1690-commit-first-unfixed, F-1692-old-source-fail-open):
//
//   - sync run twice before the source edit is committed: the second run
//     removed HEAD's (old) length from a copy already rewritten to the new one;
//   - the source edit committed BEFORE the sync: HEAD == new, so the old
//     same-length arithmetic came straight back;
//   - HEAD's source unparseable (or no git at all): a silent fall back to the
//     same-length guess.
//
// Every fixture below asserts the one property that closes the class: a sync
// removes exactly the lines of a block text it has matched byte-for-byte at
// the anchor — the current text (already synced: no write) or a known prior
// revision of it — and otherwise refuses with could-not-check and writes
// nothing. Trailing content is never touched.

// extentSource renders a one-block GUARDRAILS.md whose block is `lines`.
func extentSource(id, site string, lines ...string) string {
	return "---\nname: guardrails\ndescription: fixture\n---\n\n## guardrail: " + id +
		"\n\n- site: " + site + "\n\n```text\n" + strings.Join(lines, "\n") + "\n```\n"
}

const extentSite = ".claude/skills/ext/SKILL.md"

var (
	extentShort = []string{
		"- **Extent test:** anchor line, stable across edits.",
		"- short tail line.",
	}
	extentLong = []string{
		"- **Extent test:** anchor line, stable across edits.",
		"- long middle line one.",
		"- long middle line two.",
		"- long tail line.",
	}
	// extentPrefix is extentLong's first two lines: a block that grows or
	// shrinks by appending/trimming lines, so one text is a prefix of the other
	// and a match on the shorter one alone would be wrong.
	extentPrefix  = extentLong[:2]
	extentTrailer = []string{
		"- UNRELATED-1 must survive.",
		"- UNRELATED-2 must survive.",
	}
)

func extentSiteBody(block []string) string {
	return strings.Join(append(append([]string{"# ext", ""}, block...), extentTrailer...), "\n") + "\n"
}

func parsedExtentSource(t *testing.T, block []string) *GuardrailSource {
	t.Helper()
	return oldGuardrailSource(t, extentSource("extent", extentSite, block...))
}

// assertSiteIs fails unless the site holds exactly `block` followed by the
// untouched trailer.
func assertSiteIs(t *testing.T, root string, block []string, what string) {
	t.Helper()
	if got, want := read(t, root, extentSite), extentSiteBody(block); got != want {
		t.Fatalf("%s: site is corrupted.\n--- got ---\n%s--- want ---\n%s", what, got, want)
	}
}

// TestSyncGuardrails_SecondSyncIsANoOp covers the edit → sync → sync-again
// loop with the source edit still uncommitted (HEAD holds the old text), in
// both directions. The second run must be a byte-for-byte no-op.
func TestSyncGuardrails_SecondSyncIsANoOp(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new []string
	}{
		{"shrink", extentLong, extentShort},
		{"grow", extentShort, extentLong},
		{"shrink-to-prefix", extentLong, extentPrefix},
		{"grow-from-prefix", extentPrefix, extentLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.new...))
			mkdirWrite(t, root, extentSite, extentSiteBody(tc.old))
			head := parsedExtentSource(t, tc.old)

			for run := 1; run <= 2; run++ {
				changed, rep, err := syncWith(root, head)
				if err != nil {
					t.Fatalf("run %d: %v", run, err)
				}
				if len(rep.Unchecked) != 0 {
					t.Fatalf("run %d: could-not-check: %+v", run, rep.Unchecked)
				}
				if run == 2 && len(changed) != 0 {
					t.Errorf("run 2 rewrote %v — a second sync over an already-synced copy must be a no-op", changed)
				}
				assertSiteIs(t, root, tc.new, "after run "+string(rune('0'+run)))
			}
			if !CheckGuardrails(root).Clean() {
				t.Fatal("check not clean after sync")
			}
		})
	}
}

// TestSyncGuardrails_SourceCommittedFirst covers the ordering where the
// GUARDRAILS.md edit is already in HEAD when sync runs (commit-first, or a
// merge that brought the source change in). HEAD == new, so the copy's real
// extent can only come from an EARLIER revision (HEAD~1 here). Given that
// history the rewrite must be exact; the trailer must survive either way.
func TestSyncGuardrails_SourceCommittedFirst(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new []string
	}{
		{"shrink", extentLong, extentShort},
		{"grow", extentShort, extentLong},
		{"shrink-to-prefix", extentLong, extentPrefix},
		{"grow-from-prefix", extentPrefix, extentLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.new...))
			mkdirWrite(t, root, extentSite, extentSiteBody(tc.old))
			head := parsedExtentSource(t, tc.new) // HEAD already carries the edit
			prev := parsedExtentSource(t, tc.old) // HEAD~1 is what the copy holds

			changed, rep, err := syncWith(root, head, prev)
			if err != nil {
				t.Fatalf("sync: %v", err)
			}
			if len(rep.Unchecked) != 0 {
				t.Fatalf("could-not-check: %+v", rep.Unchecked)
			}
			if len(changed) != 1 {
				t.Fatalf("rewrote %v, want the one site", changed)
			}
			assertSiteIs(t, root, tc.new, "commit-first sync")
		})
	}
}

// TestSyncGuardrails_UnprovableExtentRefuses: when no known text of the block
// matches what is on disk at the anchor — no history at all (nil), history
// that does not include the copy's text (HEAD == new), or a hand-drifted
// copy — the extent cannot be established, so sync must report could-not-check
// and leave the file byte-identical. Guessing a length is the defect class.
func TestSyncGuardrails_UnprovableExtentRefuses(t *testing.T) {
	for _, tc := range []struct {
		name            string
		current, onDisk []string
		history         func(t *testing.T) []*GuardrailSource
	}{
		{"grow-no-history", extentLong, extentShort, func(*testing.T) []*GuardrailSource { return nil }},
		{"shrink-no-history", extentShort, extentLong, func(*testing.T) []*GuardrailSource { return nil }},
		{"grow-head-is-new", extentLong, extentShort, func(t *testing.T) []*GuardrailSource {
			return []*GuardrailSource{parsedExtentSource(t, extentLong)}
		}},
		{"shrink-head-is-new", extentShort, extentLong, func(t *testing.T) []*GuardrailSource {
			return []*GuardrailSource{parsedExtentSource(t, extentShort)}
		}},
		{"hand-drifted", extentLong, []string{extentLong[0], "- a hand edit.", extentLong[2], extentLong[3]}, func(t *testing.T) []*GuardrailSource {
			return []*GuardrailSource{parsedExtentSource(t, extentShort)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.current...))
			mkdirWrite(t, root, extentSite, extentSiteBody(tc.onDisk))

			changed, rep, err := syncWith(root, tc.history(t)...)
			if err != nil {
				t.Fatalf("sync: %v", err)
			}
			if len(rep.Unchecked) != 1 {
				t.Errorf("want exactly 1 could-not-check, got %+v", rep.Unchecked)
			}
			if len(changed) != 0 {
				t.Errorf("rewrote %v with an unprovable extent", changed)
			}
			assertSiteIs(t, root, tc.onDisk, "refused sync")
		})
	}
}

// TestPriorGuardrailSources_FromGit exercises the production history path
// against a real repository (review advisory A-2): an unparseable HEAD
// revision is skipped (and noted), an earlier parseable revision still
// supplies the copy's extent, and the end-to-end sync is exact.
func TestPriorGuardrailSources_FromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main",
		}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	// Commit 1: the short block, synced into the site.
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, extentShort...))
	mkdirWrite(t, root, extentSite, extentSiteBody(extentShort))
	git("add", "-A")
	git("commit", "-q", "-m", "v1")
	// Commit 2: a broken source (unterminated fence) — must be skipped, never
	// a reason to guess.
	mkdirWrite(t, root, guardrailSourcePath, strings.TrimSuffix(extentSource("extent", extentSite, extentLong...), "```\n"))
	git("commit", "-q", "-am", "broken")
	// Working tree: the grown block, not committed.
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, extentLong...))

	prior, notes := priorGuardrailSources(root)
	if len(prior) != 1 {
		t.Fatalf("want 1 parseable prior revision, got %d (notes %v)", len(prior), notes)
	}
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, "\n"), "skipped") {
		t.Errorf("the unparseable revision must be noted, got %v", notes)
	}

	changed, rep, err := SyncGuardrails(root, prior)
	if err != nil || len(rep.Unchecked) != 0 || len(changed) != 1 {
		t.Fatalf("sync: changed=%v unchecked=%+v err=%v", changed, rep.Unchecked, err)
	}
	assertSiteIs(t, root, extentLong, "git-history sync")

	// A root that is not a git checkout yields no history and says so.
	bare := t.TempDir()
	if p, n := priorGuardrailSources(filepath.Join(bare)); len(p) != 0 || len(n) == 0 {
		t.Errorf("non-git root: want no history plus a note, got %d revisions, notes %v", len(p), n)
	}
}

// syncWith is SyncGuardrails with the prior revisions as variadic arguments,
// newest first (HEAD, then HEAD~1, ...).
func syncWith(root string, prior ...*GuardrailSource) ([]string, GuardrailReport, error) {
	return SyncGuardrails(root, prior)
}

// --- medici-finance/assay#1692 round 2: the residual longest-match tie ---
//
// matchExtent's longest-known-text-wins rule (above) closed round 1's data
// loss, but review findings F-1692-prefix-extent-ambiguity /
// F-1692-longest-match-ambiguity showed it can still pick the wrong extent
// when MORE THAN ONE known length matches at the anchor and the longest is
// not the current canonical text: the bytes cannot tell a copy still
// genuinely at that longer, earlier text (the ordinary case this rule
// serves — see TestSyncGuardrails_SourceCommittedFirst/shrink-to-prefix,
// which needs exactly this rewrite to pass) apart from a copy already at the
// current text, followed by unrelated content that coincidentally equals the
// earlier text's own tail. Refusing outright would also refuse that ordinary
// case, so the rewrite still happens; what closes the finding is that the
// tie is no longer silent — GuardrailReport.Notes carries it. Both fixtures
// below assert a note is present for the ambiguous rewrite; mutating
// matchExtent's `ambiguous` computation to always false turns them red.

// TestSyncGuardrails_AmbiguousLongestMatchIsNoted covers the two repros from
// the F-1692-longest-match-ambiguity finding.
func TestSyncGuardrails_AmbiguousLongestMatchIsNoted(t *testing.T) {
	t.Run("dropped-line-kept-locally", func(t *testing.T) {
		// Revision 1 (historical): the block carries a trailing bullet.
		// Revision 2 (current, want): that bullet is dropped from the block.
		// The adopter keeps the exact same sentence as SITE-LOCAL text right
		// after the block — a natural way to demote a shared rule to a local
		// one — so a later sync, with no further source change, still finds
		// the historical (longer) text matching at the anchor.
		anchor := "- **Extent test:** anchor line, stable across edits."
		keep := "- keep this line."
		dropped := "- dropped bullet, once part of the block."
		oldBlock := []string{anchor, keep, dropped}
		newBlock := []string{anchor, keep}

		root := t.TempDir()
		mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, newBlock...))
		body := strings.Join([]string{"# ext", "", anchor, keep, dropped, "- NEXT SECTION."}, "\n") + "\n"
		mkdirWrite(t, root, extentSite, body)
		prior := oldGuardrailSource(t, extentSource("extent", extentSite, oldBlock...))

		changed, rep, err := syncWith(root, prior)
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		if len(rep.Unchecked) != 0 {
			t.Fatalf("could-not-check: %+v", rep.Unchecked)
		}
		if len(changed) != 1 {
			t.Fatalf("changed=%v, want the one site rewritten", changed)
		}
		if len(rep.Notes) == 0 {
			t.Fatal("want an ambiguous-match note — the longest match (the historical, 3-line text) was not the current canonical text, and the current text also matched")
		}
		if got, want := read(t, root, extentSite), strings.Join([]string{"# ext", "", anchor, keep, "- NEXT SECTION."}, "\n")+"\n"; got != want {
			t.Fatalf("site after sync:\n--- got ---\n%s--- want ---\n%s", got, want)
		}
	})

	t.Run("second-sync-after-trailing-blank-shrink", func(t *testing.T) {
		// Revision 1 (historical): the block's own fence ends in a blank line.
		// Revision 2 (current, want): that trailing blank is dropped from the
		// block. The site separately carries its own genuine separator blank
		// line right after the block, which happens to equal the historical
		// text's dropped tail. The first sync correctly drops the stale
		// in-block blank; the second, with nothing left to fix, must not also
		// eat the site's own separator blank — and if it still does (this is
		// the SAME ambiguity, not a new one), that must be noted, not silent.
		anchor := "- **Extent test:** anchor line, stable across edits."
		keep := "- keep this line."
		oldBlock := []string{anchor, keep, ""}
		newBlock := []string{anchor, keep}

		root := t.TempDir()
		mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, newBlock...))
		body := strings.Join([]string{"# ext", "", anchor, keep, "", "", "- NEXT SECTION."}, "\n") + "\n"
		mkdirWrite(t, root, extentSite, body)
		prior := oldGuardrailSource(t, extentSource("extent", extentSite, oldBlock...))

		changed1, rep1, err := syncWith(root, prior)
		if err != nil {
			t.Fatalf("sync 1: %v", err)
		}
		if len(rep1.Unchecked) != 0 {
			t.Fatalf("sync 1 could-not-check: %+v", rep1.Unchecked)
		}
		if len(changed1) != 1 {
			t.Fatalf("sync 1 changed=%v, want the one site rewritten (the stale in-block blank dropped)", changed1)
		}
		if len(rep1.Notes) == 0 {
			t.Fatal("sync 1: want an ambiguous-match note")
		}

		changed2, rep2, err := syncWith(root, prior)
		if err != nil {
			t.Fatalf("sync 2: %v", err)
		}
		if len(rep2.Unchecked) != 0 {
			t.Fatalf("sync 2 could-not-check: %+v", rep2.Unchecked)
		}
		if len(changed2) != 0 {
			t.Logf("sync 2 rewrote %v — the residual ambiguity this fixture pins (medici-finance/assay#1692)", changed2)
		}
		if len(rep2.Notes) == 0 {
			t.Fatal("sync 2: want an ambiguous-match note whether or not it rewrote — the tie must never be silent")
		}
	})
}

// TestSyncGuardrails_RefusesOverlappingProvenExtents pins
// F-1692-overlap-guard-unpinned: the overlap refusal (SyncGuardrails' per-file
// bottom-up edit loop) had no fixture of its own, so a mutation removing it
// left the suite green. A block-split history — one merged block in an
// earlier revision, split into two adjacent blocks at the same site in the
// current source — gives two edits whose PROVEN extents overlap once the
// first (longer, historical) block's extent is matched. The whole file must
// be refused, byte-identical, not partially rewritten.
func TestSyncGuardrails_RefusesOverlappingProvenExtents(t *testing.T) {
	x1, x2 := "- X anchor, stable across edits.", "- X body line."
	y1, y2 := "- Y anchor, stable across edits.", "- Y body line."

	mergedSrc := extentSource("x", extentSite, x1, x2, y1, y2)
	splitSrc := "---\nname: guardrails\ndescription: fixture\n---\n\n" +
		"## guardrail: x\n\n- site: " + extentSite + "\n\n```text\n" + strings.Join([]string{x1, x2}, "\n") + "\n```\n\n" +
		"## guardrail: y\n\n- site: " + extentSite + "\n\n```text\n" + strings.Join([]string{y1, y2}, "\n") + "\n```\n"

	root := t.TempDir()
	mkdirWrite(t, root, guardrailSourcePath, splitSrc)
	body := strings.Join(append([]string{"# ext", "", x1, x2, y1, y2}, extentTrailer...), "\n") + "\n"
	mkdirWrite(t, root, extentSite, body)
	prior := oldGuardrailSource(t, mergedSrc)

	changed, rep, err := syncWith(root, prior)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("rewrote %v — an overlapping proven extent must refuse the whole file", changed)
	}
	foundOverlap := false
	for _, is := range rep.Unchecked {
		if strings.Contains(is.Msg, "overlap") {
			foundOverlap = true
		}
	}
	if !foundOverlap {
		t.Fatalf("want an overlap-refusal could-not-check, got %+v", rep.Unchecked)
	}
	if got := read(t, root, extentSite); got != body {
		t.Fatalf("site was rewritten despite the overlap refusal.\n--- got ---\n%s--- want ---\n%s", got, body)
	}
}
