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
