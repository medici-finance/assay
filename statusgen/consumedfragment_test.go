package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// consumedfragment_test.go — regression for the CONSUMED-RELEASE-FRAGMENT fallback (#722).
//
// THE MEASURED FAILURE. `changelog/README.md` makes a `changelog/<slug>.md` fragment a
// required per-PR deliverable, so briefs list it as a backticked deliverable path and
// assert it in Verify rows. At each release cut the release workflow aggregates every
// fragment into `CHANGELOG.md` and CLEARS the directory. From that commit on the brief's
// backticked path resolved against nothing and the lint went PROBLEM-red on a brief that
// was never wrong — and because the release commit touches only `changelog/` and
// `CHANGELOG.md`, outside the board workflow's paths filter, main reddened silently and
// surfaced weeks later on an unrelated PR.
//
// THE BOUNDARY IS THE OTHER HALF, and is what these tests spend most of their assertions
// on. The fallback adds a resolution base for a fragment that DEMONSTRABLY existed; it is
// not an amnesty for the `changelog/` prefix. A mistyped fragment name, a nested path, the
// directory's own README, and a repo that does not run the convention at all must every
// one of them still PROBLEM.

// writeConsumedFragmentFixture materializes a repo that runs the changelog convention and
// has cut at least one release:
//
//   - `changelog/README.md` (the convention marker) and a root `CHANGELOG.md` (the
//     aggregate a release writes into);
//   - a brief citing FOUR backticked paths: the consumed fragment (must resolve), a
//     never-committed fragment, a nested changelog path, and an unrelated ghost (the last
//     three must all still PROBLEM).
//
// initGit controls whether the tree has git history at all — the could-not-check arm.
func writeConsumedFragmentFixture(t *testing.T, initGit bool) (root, brief string) {
	t.Helper()
	root = t.TempDir()

	clDir := filepath.Join(root, "changelog")
	if err := os.MkdirAll(clDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(clDir, "README.md"), "# changelog fragments\n")
	writeFixtureFile(t, filepath.Join(root, "CHANGELOG.md"), "# Changelog\n\n## v1.0.0 — 2026-09-01\n\n- something\n")

	briefDir := filepath.Join(root, "docs", "streams", "rel")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Brief 15\n\n" +
		// The fragment this brief really delivered, since consumed by the release.
		"Deliverable: `changelog/rel-15-consumed.md`.\n" +
		// CONTROL: a fragment path that was never committed — a typo, and still a PROBLEM.
		"Typo: `changelog/rel-15-nevercommitted.md`.\n" +
		// CONTROL: a NESTED path under changelog/ — not the fragment shape, still a PROBLEM.
		"Nested: `changelog/old/rel-15-consumed.md`.\n" +
		// CONTROL: an unrelated missing path — untouched by this fallback.
		"Ghost: `docs/streams/rel/ghost-nowhere.md`.\n"
	brief = writeTemp(t, briefDir, "brief-15-release.md", content)

	if initGit {
		gitInit(t, root, "Fixture Author", "fixture@example.invalid")
		// The fragment EXISTED and was committed …
		writeFixtureFile(t, filepath.Join(clDir, "rel-15-consumed.md"), "### Added\n- a thing\n")
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-m", "feat(rel): brief 15 and its changelog fragment")
		// … and the release that aggregated it then CLEARED the directory.
		if err := os.Remove(filepath.Join(clDir, "rel-15-consumed.md")); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-m", "release: v1.0.0 — aggregate fragments and clear changelog/")
	}
	return root, brief
}

// TestConsumedChangelogFragmentResolves is the RED-before-GREEN fixture. Against the
// pre-fix code all FOUR backticked paths PROBLEM (the consumed fragment among them);
// with the fallback wired in exactly the three controls remain.
func TestConsumedChangelogFragmentResolves(t *testing.T) {
	root, brief := writeConsumedFragmentFixture(t, true)
	problems := linkProblems(root, []string{brief})

	for _, p := range problems {
		if strings.Contains(p, "rel-15-consumed.md") && !strings.Contains(p, "changelog/old/") {
			t.Errorf("a fragment consumed by a release must resolve, still flagged: %v", p)
		}
	}

	for _, want := range []string{
		"changelog/rel-15-nevercommitted.md",
		"changelog/old/rel-15-consumed.md",
		"docs/streams/rel/ghost-nowhere.md",
	} {
		found := false
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("control %q must still PROBLEM (the fallback must not blanket-suppress changelog/): %v", want, problems)
		}
	}

	if len(problems) != 3 {
		t.Fatalf("got %d problems, want exactly 3 (the three controls, not the consumed fragment): %v", len(problems), problems)
	}
	// A definitively-answered path must not carry the could-not-check note.
	for _, p := range problems {
		if strings.Contains(p, "could-not-check") {
			t.Errorf("git history was readable here; no problem may claim could-not-check: %v", p)
		}
	}
}

// TestConsumedChangelogFragmentUnreadableHistoryIsCouldNotCheck pins the three-state rule:
// a tree with NO git history cannot answer "was this fragment ever tracked?", and that is
// reported AS ITSELF — the PROBLEM still fires (never rounded up to a pass) and its message
// says the exemption could not be evaluated (never rounded down to a clean verdict).
func TestConsumedChangelogFragmentUnreadableHistoryIsCouldNotCheck(t *testing.T) {
	root, brief := writeConsumedFragmentFixture(t, false)
	problems := linkProblems(root, []string{brief})

	if len(problems) != 4 {
		t.Fatalf("got %d problems, want 4 — with no history NOTHING is exempt: %v", len(problems), problems)
	}

	var noted, unnoted []string
	for _, p := range problems {
		if strings.Contains(p, "could-not-check") {
			noted = append(noted, p)
		} else {
			unnoted = append(unnoted, p)
		}
	}
	// Exactly the two FRAGMENT-SHAPED paths are the ones whose exemption was unanswerable.
	if len(noted) != 2 {
		t.Fatalf("want the could-not-check note on exactly the 2 fragment-shaped paths, got %d: %v", len(noted), noted)
	}
	for _, p := range noted {
		if !strings.Contains(p, "changelog/rel-15-consumed.md") && !strings.Contains(p, "changelog/rel-15-nevercommitted.md") {
			t.Errorf("could-not-check note on a path that is not fragment-shaped: %v", p)
		}
	}
	// The nested and unrelated paths were answered definitively and must stay unannotated.
	for _, p := range unnoted {
		if strings.Contains(p, "changelog/rel-15-consumed.md") || strings.Contains(p, "changelog/rel-15-nevercommitted.md") {
			t.Errorf("fragment-shaped path answered definitively without git history: %v", p)
		}
	}
}

// TestConsumedFragmentGates exercises the helper's gates directly. Each is a way the
// exemption could have been made too wide; each must answer "checked, not exempt".
func TestConsumedFragmentGates(t *testing.T) {
	root, _ := writeConsumedFragmentFixture(t, true)
	ix := &consumedFragmentIndex{root: root}

	if exempt, checked := ix.consumed("changelog/rel-15-consumed.md"); !exempt || !checked {
		t.Errorf("the consumed fragment: got exempt=%v checked=%v, want true/true", exempt, checked)
	}
	for _, target := range []string{
		"changelog/README.md",                // the directory's own README is never a fragment
		"changelog/old/rel-15-consumed.md",   // nested — not the one-level fragment shape
		"changelog/rel-15-nevercommitted.md", // fragment-shaped but in no history
		"changelog/rel-15-consumed.txt",      // not markdown
		"docs/streams/rel/ghost-nowhere.md",  // outside changelog/ entirely
		"changelog",                          // the directory itself
	} {
		if exempt, checked := ix.consumed(target); exempt || !checked {
			t.Errorf("%q: got exempt=%v checked=%v, want false/true", target, exempt, checked)
		}
	}

	// A repo that does not RUN the convention gets no exemption: without
	// changelog/README.md a `changelog/x.md` path is just a missing file.
	bare := t.TempDir()
	writeFixtureFile(t, filepath.Join(bare, "CHANGELOG.md"), "# Changelog\n")
	bareIx := &consumedFragmentIndex{root: bare}
	if exempt, checked := bareIx.consumed("changelog/rel-15-consumed.md"); exempt || !checked {
		t.Errorf("convention-absent repo: got exempt=%v checked=%v, want false/true", exempt, checked)
	}

	// A repo that runs the convention but has never CUT a release (no aggregate to
	// have consumed anything) likewise gets no exemption.
	uncut := t.TempDir()
	if err := os.MkdirAll(filepath.Join(uncut, "changelog"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(uncut, "changelog", "README.md"), "# fragments\n")
	uncutIx := &consumedFragmentIndex{root: uncut}
	if exempt, checked := uncutIx.consumed("changelog/rel-15-consumed.md"); exempt || !checked {
		t.Errorf("never-released repo: got exempt=%v checked=%v, want false/true", exempt, checked)
	}
}
