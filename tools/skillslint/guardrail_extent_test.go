package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
//
// "shrink-to-prefix" is deliberately NOT in this table: extentPrefix is a
// PREFIX of extentLong, so shrinking to it is exactly the ambiguous case
// (medici-finance/assay#1692) — the current text and an earlier revision's
// text both match at the anchor — and by default (round 3) that now refuses
// rather than rewriting. See TestSyncGuardrails_AmbiguousExtentRefusesByDefault
// and its allow-ambiguous-extent-opt-in counterpart below for that case.
// "grow-from-prefix" stays here: growing FROM a prefix is unambiguous (only
// the shorter, old text matches at the anchor; the longer, new text does
// not, because the trailer file this fixture writes does not also happen to
// equal the new block's tail), so it must keep succeeding by default.
func TestSyncGuardrails_SecondSyncIsANoOp(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new []string
	}{
		{"shrink", extentLong, extentShort},
		{"grow", extentShort, extentLong},
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
//
// "shrink-to-prefix" is excluded here for the same reason as in
// TestSyncGuardrails_SecondSyncIsANoOp — see that test's comment and
// TestSyncGuardrails_AmbiguousExtentRefusesByDefault.
func TestSyncGuardrails_SourceCommittedFirst(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new []string
	}{
		{"shrink", extentLong, extentShort},
		{"grow", extentShort, extentLong},
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

	changed, rep, err := SyncGuardrails(root, prior, false)
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
// newest first (HEAD, then HEAD~1, ...), and the ambiguous-extent default
// (refuse — allowAmbiguous == false).
func syncWith(root string, prior ...*GuardrailSource) ([]string, GuardrailReport, error) {
	return SyncGuardrails(root, prior, false)
}

// syncAllowAmbiguous is syncWith but with the explicit --allow-ambiguous-extent
// opt-in set, for the tests that check the old (round-2) behaviour is still
// reachable, deliberately, when a caller asks for it.
func syncAllowAmbiguous(root string, prior ...*GuardrailSource) ([]string, GuardrailReport, error) {
	return SyncGuardrails(root, prior, true)
}

// --- medici-finance/assay#1692: the residual longest-match tie ---
//
// matchExtent's longest-known-text-wins rule (above) closed round 1's data
// loss, but review findings F-1692-prefix-extent-ambiguity /
// F-1692-longest-match-ambiguity showed it can still pick the wrong extent
// when the longest known text matching at the anchor is not also the newest
// one matching there (a newer text is a strict prefix of an older one): the
// bytes cannot tell a copy still genuinely at that older, longer text apart
// from a copy at the newer, shorter text, followed by unrelated content —
// possibly a local, site-specific rule — that coincidentally equals the
// longer text's own tail. (Round 4, F-1692-ambiguity-reach-misstated: the
// condition used to be "two lengths matched and the longest is not the
// current text", which also caught every edit after a committed grow; see
// TestSyncGuardrails_GrowHistoryNotAmbiguous.) Round 2 shipped "take the longest match, but note the tie" instead
// of refusing; round 3's review showed the note fires identically on every
// ordinary edit too, so it protects nothing, and a real local rule was
// deleted with only that note to show for it. Round 3 (this round) REFUSES
// the ambiguous case BY DEFAULT — could-not-check, naming the file and the
// exact span, that block not written — and reaches the round-2 behaviour only
// through the explicit --allow-ambiguous-extent opt-in
// (syncAllowAmbiguous in these tests). This default is this desk's own
// choice among the review's options, not a ruling already made — see the PR
// body.

// TestSyncGuardrails_AmbiguousExtentRefusesByDefault covers the two repros
// from the F-1692-longest-match-ambiguity finding and asserts the DEFAULT
// (allowAmbiguous == false) behaviour: could-not-check, the file untouched
// (so the site-local content is never swallowed), and the error names the
// file and the ambiguous span.
func TestSyncGuardrails_AmbiguousExtentRefusesByDefault(t *testing.T) {
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
		if len(changed) != 0 {
			t.Fatalf("changed=%v — an ambiguous extent must refuse, not rewrite", changed)
		}
		if len(rep.Unchecked) != 1 {
			t.Fatalf("want exactly 1 could-not-check naming the ambiguity, got %+v", rep.Unchecked)
		}
		msg := rep.Unchecked[0].Msg
		if !strings.Contains(msg, extentSite) || !strings.Contains(msg, "AMBIGUOUS") {
			t.Fatalf("refusal must name the file and say the extent is ambiguous, got %q", msg)
		}
		if !strings.Contains(msg, "span") {
			t.Fatalf("refusal must name the ambiguous span, got %q", msg)
		}
		if got, want := read(t, root, extentSite), body; got != want {
			t.Fatalf("site was rewritten despite the refusal — the local line must survive intact.\n--- got ---\n%s--- want ---\n%s", got, want)
		}
	})

	t.Run("second-sync-after-trailing-blank-shrink", func(t *testing.T) {
		// Revision 1 (historical): the block's own fence ends in a blank line.
		// Revision 2 (current, want): that trailing blank is dropped from the
		// block. The site separately carries its own genuine separator blank
		// line right after the block, which happens to equal the historical
		// text's dropped tail — the same ambiguity, so this also refuses by
		// default, even on the very first sync.
		anchor := "- **Extent test:** anchor line, stable across edits."
		keep := "- keep this line."
		oldBlock := []string{anchor, keep, ""}
		newBlock := []string{anchor, keep}

		root := t.TempDir()
		mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, newBlock...))
		body := strings.Join([]string{"# ext", "", anchor, keep, "", "", "- NEXT SECTION."}, "\n") + "\n"
		mkdirWrite(t, root, extentSite, body)
		prior := oldGuardrailSource(t, extentSource("extent", extentSite, oldBlock...))

		changed, rep, err := syncWith(root, prior)
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		if len(changed) != 0 {
			t.Fatalf("changed=%v — an ambiguous extent must refuse, not rewrite", changed)
		}
		if len(rep.Unchecked) != 1 {
			t.Fatalf("want exactly 1 could-not-check naming the ambiguity, got %+v", rep.Unchecked)
		}
		if got, want := read(t, root, extentSite), body; got != want {
			t.Fatalf("site was rewritten despite the refusal.\n--- got ---\n%s--- want ---\n%s", got, want)
		}
	})
}

// TestSyncGuardrails_AmbiguousExtentAllowOptIn covers the same two repros
// with --allow-ambiguous-extent set (allowAmbiguous == true), and asserts the
// tool proceeds exactly as round 2 did: the longest match is taken, the
// rewrite happens, and the override is still recorded in rep.Notes rather
// than being silent.
func TestSyncGuardrails_AmbiguousExtentAllowOptIn(t *testing.T) {
	t.Run("dropped-line-kept-locally", func(t *testing.T) {
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

		changed, rep, err := syncAllowAmbiguous(root, prior)
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
			t.Fatal("want an ambiguous-override note — the opt-in must never be silent about what it guessed")
		}
		if got, want := read(t, root, extentSite), strings.Join([]string{"# ext", "", anchor, keep, "- NEXT SECTION."}, "\n")+"\n"; got != want {
			t.Fatalf("site after sync:\n--- got ---\n%s--- want ---\n%s", got, want)
		}
	})

	t.Run("second-sync-after-trailing-blank-shrink", func(t *testing.T) {
		anchor := "- **Extent test:** anchor line, stable across edits."
		keep := "- keep this line."
		oldBlock := []string{anchor, keep, ""}
		newBlock := []string{anchor, keep}

		root := t.TempDir()
		mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, newBlock...))
		body := strings.Join([]string{"# ext", "", anchor, keep, "", "", "- NEXT SECTION."}, "\n") + "\n"
		mkdirWrite(t, root, extentSite, body)
		prior := oldGuardrailSource(t, extentSource("extent", extentSite, oldBlock...))

		changed1, rep1, err := syncAllowAmbiguous(root, prior)
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
			t.Fatal("sync 1: want an ambiguous-override note")
		}

		changed2, rep2, err := syncAllowAmbiguous(root, prior)
		if err != nil {
			t.Fatalf("sync 2: %v", err)
		}
		if len(rep2.Unchecked) != 0 {
			t.Fatalf("sync 2 could-not-check: %+v", rep2.Unchecked)
		}
		if len(changed2) != 0 {
			t.Logf("sync 2 rewrote %v — the residual round-2 ambiguity this fixture pins, reachable only via the opt-in now (medici-finance/assay#1692)", changed2)
		}
		if len(rep2.Notes) == 0 {
			t.Fatal("sync 2: want an ambiguous-override note whether or not it rewrote — the guess must never be silent")
		}
	})
}

// TestSyncGuardrails_ShrinkToPrefixAmbiguousByDefault covers the
// shrink-to-prefix ordering excluded from TestSyncGuardrails_SecondSyncIsANoOp
// and TestSyncGuardrails_SourceCommittedFirst: extentPrefix is a PREFIX of
// extentLong, so once the source has shrunk to it, the current (short) text
// and the historical (long) text both match at the anchor — the ambiguous
// case, not the "common, unambiguous" one an earlier draft of this fix
// mislabelled it as. By default this now refuses; with the opt-in it takes
// the longest match exactly as before (medici-finance/assay#1692).
func TestSyncGuardrails_ShrinkToPrefixAmbiguousByDefault(t *testing.T) {
	root := t.TempDir()
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, extentPrefix...))
	mkdirWrite(t, root, extentSite, extentSiteBody(extentLong))
	head := parsedExtentSource(t, extentLong)

	changed, rep, err := syncWith(root, head)
	if err != nil {
		t.Fatalf("default sync: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("default sync rewrote %v — shrink-to-prefix is ambiguous and must refuse by default", changed)
	}
	if len(rep.Unchecked) != 1 || !strings.Contains(rep.Unchecked[0].Msg, "AMBIGUOUS") {
		t.Fatalf("want 1 ambiguity refusal, got %+v", rep.Unchecked)
	}
	assertSiteIs(t, root, extentLong, "after the default refusal")

	changed, rep, err = syncAllowAmbiguous(root, head)
	if err != nil {
		t.Fatalf("opt-in sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("opt-in sync could-not-check: %+v", rep.Unchecked)
	}
	if len(changed) != 1 {
		t.Fatalf("opt-in sync rewrote %v, want the one site", changed)
	}
	if len(rep.Notes) == 0 {
		t.Fatal("opt-in sync: want an ambiguous-override note")
	}
	assertSiteIs(t, root, extentPrefix, "after the opt-in rewrite")
}

// TestSyncGuardrails_NonPrefixShrinkIsSilent is the explicit negative case
// for the two tests above: a shrink whose new text is NOT a prefix of the old
// one (here 4 lines replaced by 2, the second of them different) leaves only
// one known text matching at the anchor, so it must proceed with no
// could-not-check and no ambiguous-extent note. Note what this is not: it is
// not a plain trailing-line removal. Dropping trailing lines always leaves a
// prefix of the old text, which is the shrink-to-prefix case above and refuses
// by default.
func TestSyncGuardrails_NonPrefixShrinkIsSilent(t *testing.T) {
	root := t.TempDir()
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, extentShort...))
	mkdirWrite(t, root, extentSite, extentSiteBody(extentLong))
	head := parsedExtentSource(t, extentLong)

	changed, rep, err := syncWith(root, head)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Unchecked) != 0 {
		t.Fatalf("could-not-check on the ordinary case: %+v", rep.Unchecked)
	}
	if len(rep.Notes) != 0 {
		t.Fatalf("an ambiguous-extent note fired on the ordinary, unambiguous case: %+v", rep.Notes)
	}
	if len(changed) != 1 {
		t.Fatalf("changed=%v, want the one site rewritten", changed)
	}
	assertSiteIs(t, root, extentShort, "non-prefix shrink")
}

// TestSyncGuardrails_GrowHistoryNotAmbiguous pins the grow-history half of
// #1692's F-1692-ambiguity-reach-misstated. Once a block has had one committed
// append-grow, its older, shorter text is a prefix of the newer, longer one, so
// BOTH match at the anchor of a copy synced to the newer text. That is not the
// ambiguous case: the longest match is also the NEWEST matching revision, which
// is exactly what a synced copy holds. Every later edit of such a block must
// rewrite by default, with no refusal and no note, and a re-sync must be a
// no-op. Before this fix the condition fired whenever two lengths matched and
// the longest was not the current text, so all three cases below refused,
// permanently, and the refusal claimed the current text had matched.
//
// prior is newest first, as priorGuardrailSources returns it.
func TestSyncGuardrails_GrowHistoryNotAmbiguous(t *testing.T) {
	a := "- **Extent test:** anchor line, stable across edits."
	l2, l2x := "- second line.", "- second line, reworded."
	l3, l3x := "- third line, added by the first grow.", "- third line, reworded."
	l4 := "- fourth line, added by a second grow."
	r1 := []string{a, l2}
	r2 := []string{a, l2, l3}
	r3 := []string{a, l2, l3, l4}
	for _, tc := range []struct {
		name   string
		onDisk []string   // the copy, synced to the newest committed revision
		want   []string   // the current canonical text
		prior  [][]string // committed revisions, newest first
	}{
		{"second-grow", r2, r3, [][]string{r2, r1}},
		{"edit-after-grow", r2, []string{a, l2x, l3}, [][]string{r2, r1}},
		{"edit-after-two-grows", r3, []string{a, l2, l3x, l4}, [][]string{r3, r2, r1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.want...))
			mkdirWrite(t, root, extentSite, extentSiteBody(tc.onDisk))
			var prior []*GuardrailSource
			for _, p := range tc.prior {
				prior = append(prior, parsedExtentSource(t, p))
			}
			for run := 1; run <= 2; run++ {
				changed, rep, err := syncWith(root, prior...)
				if err != nil {
					t.Fatalf("run %d: %v", run, err)
				}
				if len(rep.Unchecked) != 0 {
					t.Fatalf("run %d: a grow history must not make the extent ambiguous, got %+v", run, rep.Unchecked)
				}
				if len(rep.Notes) != 0 {
					t.Fatalf("run %d: unexpected note: %+v", run, rep.Notes)
				}
				if wantChanged := map[int]int{1: 1, 2: 0}[run]; len(changed) != wantChanged {
					t.Fatalf("run %d: changed=%v, want %d file(s)", run, changed, wantChanged)
				}
				assertSiteIs(t, root, tc.want, "after run "+string(rune('0'+run)))
			}
		})
	}
}

// TestSyncGuardrails_ShrinkHistoryEditRefuses is the other side of the same
// finding, and the reason the narrowing is by RECENCY rather than only "the
// current text also matches". A block shrinks to a prefix of itself (r1 ->
// r2, committed and synced), the adopter keeps the dropped bullet as
// site-local text right under the block, and then the block is edited again
// (r3). The current text no longer matches, but the newer r2 and the older,
// longer r1 both do, so the longest match is NOT the newest one: taking it
// would delete the site-local line, the harm F-1692-longest-match-ambiguity
// was about. It must still refuse by default, the file byte-identical, and
// the refusal must not claim the current text matched.
func TestSyncGuardrails_ShrinkHistoryEditRefuses(t *testing.T) {
	a := "- **Extent test:** anchor line, stable across edits."
	keep := "- keep this line."
	dropped := "- dropped bullet, kept as site-local text."
	r1 := []string{a, keep, dropped}
	r2 := []string{a, keep}
	r3 := []string{a, keep, "- a new bullet, added after the shrink."}

	root := t.TempDir()
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, r3...))
	body := strings.Join([]string{"# ext", "", a, keep, dropped, "- NEXT SECTION."}, "\n") + "\n"
	mkdirWrite(t, root, extentSite, body)
	prior := []*GuardrailSource{parsedExtentSource(t, r2), parsedExtentSource(t, r1)}

	changed, rep, err := syncWith(root, prior...)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed=%v: the site-local line would be deleted", changed)
	}
	if len(rep.Unchecked) != 1 || !strings.Contains(rep.Unchecked[0].Msg, "AMBIGUOUS") {
		t.Fatalf("want 1 ambiguity refusal, got %+v", rep.Unchecked)
	}
	msg := rep.Unchecked[0].Msg
	if strings.Contains(msg, "current canonical text's") {
		t.Fatalf("the current text did not match here, but the refusal says it did: %q", msg)
	}
	if !strings.Contains(msg, "3 line(s)") || !strings.Contains(msg, "2 line(s)") {
		t.Fatalf("the refusal must name the two lengths that matched (3 and 2), got %q", msg)
	}
	if got := read(t, root, extentSite); got != body {
		t.Fatalf("site was rewritten despite the refusal.\n--- got ---\n%s--- want ---\n%s", got, body)
	}
}

// TestRunSync_AmbiguousCLI runs the built binary, so the parts of the
// ambiguity handling that live in main.go are pinned too: the flag reaching
// SyncGuardrails, the refusal and the opt-in note reaching stderr, the exit
// codes, and --allow-ambiguous-extent being refused without --sync.
func TestRunSync_AmbiguousCLI(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if testing.Short() {
		t.Skip("builds and runs the binary; skipped under -short")
	}
	bin := buildSkillslintBinary(t)
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
	run := func(args ...string) (int, string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("running %v: %v", args, err)
		}
		return code, stderr.String()
	}

	a := "- **Extent test:** anchor line, stable across edits."
	keep := "- keep this line."
	dropped := "- dropped bullet, kept as site-local text."
	git("init", "-q")
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, a, keep, dropped))
	git("add", "-A")
	git("commit", "-q", "-m", "r1")
	mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, a, keep))
	git("commit", "-q", "-am", "r2: shrink to a prefix")
	body := strings.Join([]string{"# ext", "", a, keep, dropped, "- NEXT SECTION."}, "\n") + "\n"
	mkdirWrite(t, root, extentSite, body)

	if code, stderr := run("--root", root, "--allow-ambiguous-extent"); code != 2 || !strings.Contains(stderr, "requires --sync") {
		t.Fatalf("--allow-ambiguous-extent without --sync: exit %d, stderr %q; want exit 2 naming the missing --sync", code, stderr)
	}

	code, stderr := run("--root", root, "--sync")
	if code != 2 || !strings.Contains(stderr, "AMBIGUOUS") || !strings.Contains(stderr, extentSite) {
		t.Fatalf("default sync: exit %d, stderr %q; want exit 2 and the refusal naming %s", code, stderr, extentSite)
	}
	if got := read(t, root, extentSite); got != body {
		t.Fatalf("default sync wrote the site despite refusing:\n%s", got)
	}

	code, stderr = run("--root", root, "--sync", "--allow-ambiguous-extent")
	if !strings.Contains(stderr, "note: ") || !strings.Contains(stderr, "--allow-ambiguous-extent took the longest match") {
		t.Fatalf("opt-in sync: stderr %q; want the override note", stderr)
	}
	if strings.Contains(stderr, "AMBIGUOUS") {
		t.Fatalf("opt-in sync still refused: %q", stderr)
	}
	if want := strings.Join([]string{"# ext", "", a, keep, "- NEXT SECTION."}, "\n") + "\n"; read(t, root, extentSite) != want {
		t.Fatalf("opt-in sync (exit %d) did not take the longest match:\n%s", code, read(t, root, extentSite))
	}
}

// TestSyncGuardrails_RefusesOverlappingProvenExtents pins
// F-1692-overlap-guard-unpinned: the overlap refusal (SyncGuardrails' per-file
// bottom-up edit loop) had no fixture of its own, so a mutation removing it
// left the suite green. A block-split history — one merged block in an
// earlier revision, split into two adjacent blocks at the same site in the
// current source — gives two edits whose PROVEN extents overlap once the
// first (longer, historical) block's extent is matched. The whole file must
// be refused, byte-identical, not partially rewritten.
//
// Block "x"'s own body line is changed (not merely split off) in the current
// source, so its want text does NOT also match the disk's still-merged
// content: only the historical 4-line merged text matches at its anchor —
// ONE known length, so "x" is individually unambiguous — and it is the
// resulting 4-line proven extent, not an ambiguity, that reaches into "y"'s
// anchor and trips the overlap check. (An earlier version of this fixture
// kept "x"'s body line identical to the historical text's, which made "x"
// ambiguous in its own right — medici-finance/assay#1692's round-3 refusal
// then caught it before the overlap check ever ran, unpinning this finding
// again. Changing "x"'s body line keeps the two checks separated.)
func TestSyncGuardrails_RefusesOverlappingProvenExtents(t *testing.T) {
	x1 := "- X anchor, stable across edits."
	xBodyOld, xBodyNew := "- X body line, historical.", "- X body line, replaced entirely in the current source."
	y1, y2 := "- Y anchor, stable across edits.", "- Y body line."

	mergedSrc := extentSource("x", extentSite, x1, xBodyOld, y1, y2)
	splitSrc := "---\nname: guardrails\ndescription: fixture\n---\n\n" +
		"## guardrail: x\n\n- site: " + extentSite + "\n\n```text\n" + strings.Join([]string{x1, xBodyNew}, "\n") + "\n```\n\n" +
		"## guardrail: y\n\n- site: " + extentSite + "\n\n```text\n" + strings.Join([]string{y1, y2}, "\n") + "\n```\n"

	root := t.TempDir()
	mkdirWrite(t, root, guardrailSourcePath, splitSrc)
	body := strings.Join(append([]string{"# ext", "", x1, xBodyOld, y1, y2}, extentTrailer...), "\n") + "\n"
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

// --- medici-finance/assay#1692, round 3: F-1692-prove-apply-race ---
//
// The removal extent used to be proven against ONE read of a site file
// (guardrail.go's first pass, computing each edit), then spliced into a
// SECOND, later read taken only at write time, with nothing checking that
// the two reads still agreed, and the write itself was a plain
// truncate-then-write (os.WriteFile). A reviewer showed this corrupts the
// file under real concurrent `--sync` runs: about 5 of 60 trials lost
// content in one direction (a shrink), and growth duplicated a line in
// another. The fix (guardrail.go's SyncGuardrails + atomicWriteFile) reads
// each file at most once per run, uses that same read as the base for the
// write, re-verifies immediately before writing that the file has not
// changed since (refusing rather than guessing if it has), and writes via a
// temp file plus atomic rename rather than in place.
//
// This test drives real, separate `--sync` PROCESSES concurrently against
// one fixture — the same shape as the reviewer's reproduction, not an
// in-process goroutine race, since the corruption comes from two OS
// processes truncating and writing the same path at once. It was run once,
// by hand, against the pre-fix SyncGuardrails/os.WriteFile code (commit
// fd41e80a; the PR head at the time, d9051bb5, carries the identical
// tools/skillslint tree) to confirm it DOES reproduce
// corruption at roughly the reviewer's ratio before asserting the fix here;
// see the PR body's fail-first section for that run's output. Checked in,
// it runs only against the fixed code and must show zero corruption.
func TestSyncGuardrails_ParallelSyncNoCorruption(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if testing.Short() {
		t.Skip("spawns real subprocesses; skipped under -short")
	}

	bin := buildSkillslintBinary(t)

	// Fewer trials than the reviewer's 60 (kept here for CI speed — see the
	// doc comment above for the manual 60-trial run against the pre-fix
	// code); still enough runners racing enough times to reliably surface
	// the bug if the fix regresses.
	const trials = 20
	const runners = 3

	for _, tc := range []struct {
		name     string
		old, new []string
	}{
		{"shrink", extentLong, extentShort},
		{"grow", extentShort, extentLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gitFixture := func(args ...string) {
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
			gitFixture("init", "-q")
			// Commit 1: the old canonical text. Commit 2: the new one —
			// SOURCE COMMITTED FIRST, so priorGuardrailSources' git-history
			// path supplies `old` as the one known prior revision once the
			// working tree also carries `new`. This is the ordinary,
			// unambiguous shape (not the ambiguous one covered above): the
			// two blocks here never share a prefix relationship.
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.old...))
			gitFixture("add", "-A")
			gitFixture("commit", "-q", "-m", "v1")
			mkdirWrite(t, root, guardrailSourcePath, extentSource("extent", extentSite, tc.new...))
			gitFixture("commit", "-q", "-am", "v2")

			corrupted := 0
			for trial := 0; trial < trials; trial++ {
				// Reset the site to the pre-sync (old) state before every
				// trial. The source stays committed at `new` throughout.
				mkdirWrite(t, root, extentSite, extentSiteBody(tc.old))

				var wg sync.WaitGroup
				for i := 0; i < runners; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						cmd := exec.Command(bin, "--root", root, "--sync")
						_ = cmd.Run() // the file's end state is what this test checks, not any one runner's exit code
					}()
				}
				wg.Wait()

				if got, want := read(t, root, extentSite), extentSiteBody(tc.new); got != want {
					corrupted++
					t.Logf("trial %d: corrupted — got:\n%s\nwant:\n%s", trial, got, want)
				}
			}
			if corrupted != 0 {
				t.Fatalf("%d/%d parallel-sync trials corrupted %s (want 0) — F-1692-prove-apply-race", corrupted, trials, extentSite)
			}
		})
	}
}

// buildSkillslintBinary compiles this package's own binary once into a temp
// directory, for TestSyncGuardrails_ParallelSyncNoCorruption to run as real,
// separate OS processes — the corruption this test guards against is a
// property of concurrent processes truncating and writing one file, which an
// in-process goroutine call of SyncGuardrails would not reproduce faithfully.
func buildSkillslintBinary(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "skillslint-under-test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building skillslint for the parallel-sync test: %v\n%s", err, out)
	}
	return bin
}
