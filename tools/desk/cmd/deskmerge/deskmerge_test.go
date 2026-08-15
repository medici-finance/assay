package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskmerge_test.go — the Verify table, executed.
//
// Every test below drives the real CLI entry point (dispatch) against a real git world.
// The three assertions repeated everywhere are the ones that matter:
//
//	exit code            — the operator contract
//	*w.pushes            — the argv actually constructed, not the intent
//	remoteBranchSHA      — the BARE REMOTE's branch, because "we did not push" asserted
//	                       against the local side would pass for a push that landed

// stubGenerator points the regenerable list at a generator that exists in a test
// environment. The MAP is the boundary under test; swapping the generator behind one of
// its entries exercises the carve-out without pretending statusgen is installed.
func stubGenerator(t *testing.T, argv ...string) {
	t.Helper()
	prev := regenerable["STATUS.md"]
	regenerable["STATUS.md"] = regenerator{Tool: argv[0], Argv: argv[1:]}
	t.Cleanup(func() { regenerable["STATUS.md"] = prev })
}

// stubProbeTargets replaces the module-discovery seam. Used only by the "no target"
// case; the collision tests below run the REAL discovery against a REAL Go module,
// because a stubbed probe would prove nothing about whether the probe finds anything.
func stubProbeTargets(t *testing.T, fn func(string, []string) []probeStep) {
	t.Helper()
	prev := probeTargets
	probeTargets = fn
	t.Cleanup(func() { probeTargets = prev })
}

// ---------------------------------------------------------------- Verify row 1

// TestConflictRefusal — an UNLISTED conflict and a MIXED conflict are both refused with
// exit 5, zero pushes, and the conflicted paths named in the output.
func TestConflictRefusal(t *testing.T) {
	t.Run("unlisted conflict refuses and names the paths", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"README.md": "pr side\n"},
			map[string]string{"README.md": "main side\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "README.md") {
			t.Fatalf("the refusal must name the conflicted path — that output IS the "+
				"worker-dispatch signal:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("mixed conflict is refused whole — no partial resolution", func(t *testing.T) {
		withScratchTemp(t)
		// A generator that WOULD succeed, to prove the refusal is about the mix and
		// not about the generator being unavailable.
		stubGenerator(t, "/bin/sh", "-c", "printf 'regenerated\\n' > STATUS.md")
		w := newWorld(t,
			map[string]string{"README.md": "pr side\n", "STATUS.md": "generated: pr\n"},
			map[string]string{"README.md": "main side\n", "STATUS.md": "generated: main\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 on a mixed conflict, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "README.md") {
			t.Fatalf("the unlisted half must be named:\n%s", out)
		}
		if !strings.Contains(out, "STATUS.md") {
			t.Fatalf("the listed half must be reported too, so the reader can see the mix:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("check reports the same conflict without writing anything", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"README.md": "pr side\n"},
			map[string]string{"README.md": "main side\n"})
		w.install(t, defaultPR(), false)

		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 from check on a conflicted PR, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "conflicted") || !strings.Contains(out, "README.md") {
			t.Fatalf("check must name the conflict:\n%s", out)
		}
		w.assertNoPush(t)
		for _, c := range *w.ghAll {
			if len(c) > 0 && c[0] == "api" {
				t.Fatalf("check fetched an authorization artifact — reading is not authorship "+
					"and must not consult the ruling gate: %v", c)
			}
		}
	})
}

// ---------------------------------------------------------------- Verify row 2

// TestTwoParentOnly — the pushed commit is verified to have exactly two parents with
// parent 2 the fetched base head, and every other shape is refused with nothing pushed.
func TestTwoParentOnly(t *testing.T) {
	t.Run("a clean merge produces a verified two-parent commit and pushes it", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"pr.txt": "pr work\n"},
			map[string]string{"main.txt": "main work\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		pushed := w.remoteBranchSHA(t, "pr-branch")
		if pushed == w.headSHA {
			t.Fatal("the remote branch did not move — nothing was pushed")
		}
		parents := strings.Fields(git(t, w.dir, "-C", w.remote, "rev-list", "--parents", "-n", "1", pushed))
		if len(parents) != 3 {
			t.Fatalf("the PUSHED commit has %d parent(s), want 2: %v", len(parents)-1, parents)
		}
		if parents[1] != w.headSHA {
			t.Fatalf("parent 1 = %s, want the PR head %s", parents[1], w.headSHA)
		}
		if parents[2] != w.baseSHA {
			t.Fatalf("parent 2 = %s, want the fetched main head %s", parents[2], w.baseSHA)
		}
		// The refspec is the other half of the direction bound.
		for _, p := range *w.pushes {
			if !strings.HasSuffix(strings.Join(p, " "), "HEAD:refs/heads/pr-branch") {
				t.Fatalf("a push targeted something other than the PR's own branch: %v", p)
			}
		}
	})

	t.Run("a synthetic SINGLE-parent result is rolled back and refused", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"pr.txt": "pr work\n"},
			map[string]string{"main.txt": "main work\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		// Inject the #72 shape: the commit step abandons the merge and
		// writes a single-parent commit instead. This is what a rebase, a squash or an
		// amend would leave behind, and it must never reach the remote.
		inner := runGit
		runGit = func(dir string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "commit" {
				_, _ = inner(dir, "merge", "--abort")
				return inner(dir, "commit", "--allow-empty", "--no-verify", "-m", "not a merge")
			}
			return inner(dir, args...)
		}
		t.Cleanup(func() { runGit = inner })

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 on a single-parent result, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "not a merge") && !strings.Contains(out, "parent") {
			t.Fatalf("the refusal must say the commit is not a merge:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("a merge of the WRONG base is rolled back and refused", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"pr.txt": "pr work\n"},
			map[string]string{"main.txt": "main work\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		// The sibling commit is an ancestor of NEITHER side, so merging it yields a
		// well-formed TWO-parent commit — one that a parent-count check accepts. That
		// is exactly why parent 2 is checked by SHA.
		inner := runGit
		runGit = func(dir string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "commit" {
				_, _ = inner(dir, "merge", "--abort")
				return inner(dir, "merge", "--no-ff", "--no-verify", "-m", "wrong base", w.siblingSHA)
			}
			return inner(dir, args...)
		}
		t.Cleanup(func() { runGit = inner })

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 when parent 2 is not the fetched base head, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "parent 2") {
			t.Fatalf("the refusal must name parent 2:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("verifyTwoParent rejects a rebase-shaped result", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"pr.txt": "pr work\n"},
			map[string]string{"main.txt": "main work\n"})
		w.install(t, defaultPR(), false)
		wt, err := newWorktree(w.root, w.headSHA)
		if err != nil {
			t.Fatal(err)
		}
		defer wt.remove()
		git(t, wt.dir, "-c", "user.name=f", "-c", "user.email=f@x", "rebase", w.baseSHA)
		rebased := git(t, wt.dir, "rev-parse", "HEAD")
		tr := &trial{wt: wt, rep: &report{HeadSHA: w.headSHA, BaseSHA: w.baseSHA}}
		if err := verifyTwoParent(tr, rebased, prInfo{BaseRefName: "main"}); err == nil {
			t.Fatal("a rebase produced a single-parent head and verifyTwoParent accepted it")
		} else if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("want a refusal (5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
		}
	})
}

// ---------------------------------------------------------------- Verify row 3

// TestRegenerableCarveout — a conflict confined to the list is resolved by re-running
// the generator and committed; a generator failure refuses with no partial commit.
func TestRegenerableCarveout(t *testing.T) {
	t.Run("STATUS.md-only conflict is regenerated and committed", func(t *testing.T) {
		withScratchTemp(t)
		stubGenerator(t, "/bin/sh", "-c", "printf 'generated: regenerated-by-tool\\n' > STATUS.md")
		w := newWorld(t,
			map[string]string{"STATUS.md": "generated: pr\n"},
			map[string]string{"STATUS.md": "generated: main\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0 on a regenerable-only conflict, got %d\n%s", code, out)
		}
		pushed := w.remoteBranchSHA(t, "pr-branch")
		parents := strings.Fields(git(t, w.dir, "-C", w.remote, "rev-list", "--parents", "-n", "1", pushed))
		if len(parents) != 3 {
			t.Fatalf("the regenerated result is not a two-parent merge: %v", parents)
		}
		// The committed content must be the GENERATOR's output — not either side's
		// hunk. That is what makes the resolution transport rather than authorship.
		got := git(t, w.dir, "-C", w.remote, "show", pushed+":STATUS.md")
		if got != "generated: regenerated-by-tool" {
			t.Fatalf("STATUS.md carries %q — a side was chosen instead of the file being "+
				"regenerated from its declared source", got)
		}
	})

	t.Run("a failing generator refuses with no partial commit", func(t *testing.T) {
		withScratchTemp(t)
		stubGenerator(t, "/bin/sh", "-c", "exit 3")
		w := newWorld(t,
			map[string]string{"STATUS.md": "generated: pr\n"},
			map[string]string{"STATUS.md": "generated: main\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 when the generator fails, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "UNRESOLVED") {
			t.Fatalf("the refusal must say the conflict is unresolved:\n%s", out)
		}
		w.assertNoPush(t)
	})

	// A NO-OP generator is the positive control for the marker check, and it caught a
	// real defect during implementation: `git add` on a still-conflicted path marks it
	// RESOLVED with git's markers left in the content, so the "no unmerged paths
	// remain" check passed and `<<<<<<< HEAD` reached the remote under a transport
	// commit's message. Keep this case.
	t.Run("a no-op generator is refused and no markers reach the remote", func(t *testing.T) {
		withScratchTemp(t)
		stubGenerator(t, "/bin/sh", "-c", "true")
		w := newWorld(t,
			map[string]string{"STATUS.md": "generated: pr\n"},
			map[string]string{"STATUS.md": "generated: main\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 from a no-op generator, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "conflict markers") {
			t.Fatalf("the refusal must name the markers:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("hasConflictMarkers does not fire on a Markdown setext heading", func(t *testing.T) {
		if hasConflictMarkers([]byte("Title\n=======\n\nbody\n")) {
			t.Fatal("a setext heading underline was read as a conflict marker — STATUS.md is " +
				"Markdown, so this false positive would refuse every real regeneration")
		}
		if !hasConflictMarkers([]byte("a\n<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> other\nb\n")) {
			t.Fatal("real conflict markers went undetected")
		}
	})
}

// ---------------------------------------------------------------- Verify row 4

// TestWorktreeHygiene — the scratch worktree is gone after success, after every
// refusal, and after an injected mid-merge error.
func TestWorktreeHygiene(t *testing.T) {
	cases := []struct {
		name   string
		pr     map[string]string
		main   map[string]string
		inject func(t *testing.T)
	}{
		{name: "after success",
			pr:   map[string]string{"pr.txt": "a\n"},
			main: map[string]string{"main.txt": "b\n"}},
		{name: "after a conflict refusal",
			pr:   map[string]string{"README.md": "pr\n"},
			main: map[string]string{"README.md": "main\n"}},
		{name: "after a generator failure",
			pr:     map[string]string{"STATUS.md": "pr\n"},
			main:   map[string]string{"STATUS.md": "main\n"},
			inject: func(t *testing.T) { stubGenerator(t, "/bin/sh", "-c", "exit 9") }},
		{name: "after an injected commit error",
			pr:   map[string]string{"pr.txt": "a\n"},
			main: map[string]string{"main.txt": "b\n"},
			inject: func(t *testing.T) {
				inner := runGit
				runGit = func(dir string, args ...string) (string, error) {
					if len(args) > 0 && args[0] == "commit" {
						return "", errInjected{}
					}
					return inner(dir, args...)
				}
				t.Cleanup(func() { runGit = inner })
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withScratchTemp(t)
			w := newWorld(t, tc.pr, tc.main)
			w.install(t, defaultPR(), true)
			if tc.inject != nil {
				tc.inject(t)
			}
			rul := w.rulingsFile(t, signOffURL)
			_, _ = cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
			if leaked := leakedWorktrees(t, w.root); len(leaked) != 0 {
				t.Fatalf("scratch worktrees survived the run: %v", leaked)
			}
		})
	}

	t.Run("check leaves no worktree either", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), false)
		if _, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root); out == "" {
			t.Fatal("check produced no report")
		}
		if leaked := leakedWorktrees(t, w.root); len(leaked) != 0 {
			t.Fatalf("scratch worktrees survived a read-only run: %v", leaked)
		}
	})
}

type errInjected struct{}

func (errInjected) Error() string { return "injected commit failure" }

// ---------------------------------------------------- the authority boundary

// TestRulingGateRefusesWhileUnsigned pins the state of the world on 2026-08-13: R-5
// carries no sign-off, so `merge` merges nothing — and it establishes that BEFORE it
// fetches, clones, or creates a scratch worktree.
func TestRulingGateRefusesWhileUnsigned(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	rul := w.rulingsFile(t, "")

	code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 on an unsigned R-5, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "UNSIGNED") {
		t.Fatalf("the refusal must say the ruling is unsigned:\n%s", out)
	}
	w.assertNoPush(t)
	if len(*w.gitAll) != 0 {
		t.Fatalf("the gate must fire before any git act; git ran: %v", *w.gitAll)
	}
}

// TestRulingGateRefusesANonHumanSignOff — an App-authored acceptance, and a
// trusted-but-not-blessing human, must both fail to authorize. This is the positive
// control for the identity half of the gate.
func TestRulingGateRefusesANonHumanSignOff(t *testing.T) {
	cases := []struct {
		name  string
		login string
		id    int
		typ   string
	}{
		{"an App artifact", "assay-desk-app[bot]", 300000001, "Bot"},
		{"a trusted automation account reporting type=User", "shared-agent", 2002, "User"},
		{"the right login with the wrong id", blessLogin, 9999, "User"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withScratchTemp(t)
			w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
			w.install(t, defaultPR(), true)
			inner := runGH
			runGH = func(args ...string) (string, error) {
				if len(args) > 0 && args[0] == "api" {
					return `{"id":1,"html_url":"` + signOffURL + `",` +
						`"issue_url":"https://api.github.com/repos/medici-finance/assay/issues/444",` +
						`"body":"accepted","user":{"login":"` + tc.login + `","id":` +
						itoa(tc.id) + `,"type":"` + tc.typ + `"}}`, nil
				}
				return inner(args...)
			}
			t.Cleanup(func() { runGH = inner })
			rul := w.rulingsFile(t, signOffURL)

			code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d\n%s", code, out)
			}
			w.assertNoPush(t)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestNeverMergesToMainOrCallsGhMerge is the direction bound, asserted against the
// argv the run actually constructed. MERGE IS THE HUMAN'S.
func TestNeverMergesToMainOrCallsGhMerge(t *testing.T) {
	t.Run("a PR whose head branch is a default branch is refused", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		pr := defaultPR()
		pr.HeadRefName = "main"
		w.install(t, pr, true)
		rul := w.rulingsFile(t, signOffURL)

		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "never pushes to a default branch") {
			t.Fatalf("the refusal must name the direction bound:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("a successful run makes no gh merge call and pushes only the PR branch", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)

		if code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul); code != 0 {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		for _, c := range *w.ghAll {
			joined := strings.Join(c, " ")
			for _, forbidden := range []string{"merge", "ready", "-X", "--method"} {
				if strings.Contains(joined, forbidden) {
					t.Fatalf("deskmerge made a mutating gh call (%q): %v", forbidden, c)
				}
			}
		}
		if got := w.remoteBranchSHA(t, "main"); got != w.baseSHA {
			t.Fatalf("main moved (%s -> %s) — deskmerge pushed to a default branch", w.baseSHA, got)
		}
		for _, p := range *w.pushes {
			if !strings.Contains(strings.Join(p, " "), ":refs/heads/pr-branch") {
				t.Fatalf("a push targeted something other than the PR's branch: %v", p)
			}
		}
	})
}

// TestFlipBoundAndClosedPRs — R-5's flip bound and the deskpushguard shape.
func TestFlipBoundAndClosedPRs(t *testing.T) {
	cases := []struct {
		name string
		pr   prStub
		want int
	}{
		{"already flipped ready", prStub{State: "OPEN", IsDraft: false, HeadRefName: "pr-branch"}, deskkit.ExitRefused},
		{"merged", prStub{State: "MERGED", IsDraft: true, HeadRefName: "pr-branch"}, deskkit.ExitRefused},
		{"closed", prStub{State: "CLOSED", IsDraft: true, HeadRefName: "pr-branch"}, deskkit.ExitRefused},
		{"an unrecognised state is could-not-check, not open",
			prStub{State: "WEIRD", IsDraft: true, HeadRefName: "pr-branch"}, deskkit.ExitUnverifiable},
		{"a fork head", prStub{State: "OPEN", IsDraft: true, IsCrossRepository: true, HeadRefName: "pr-branch"},
			deskkit.ExitRefused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withScratchTemp(t)
			w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
			w.install(t, tc.pr, true)
			rul := w.rulingsFile(t, signOffURL)
			code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
			if code != tc.want {
				t.Fatalf("want exit %d, got %d\n%s", tc.want, code, out)
			}
			w.assertNoPush(t)
		})
	}
}

// -------------------------------------------- the three-state instrument itself

// TestCouldNotCheckNeverReportsGreen is the core invariant: a checker that could not
// look must never report clean. Each case removes a different thing the instrument
// depends on and requires exit 6 — never 0, and never 5 either, because "I could not
// see" is not "I saw a problem".
func TestCouldNotCheckNeverReportsGreen(t *testing.T) {
	t.Run("an unfetchable origin", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), false)
		// Point origin at a path that does not exist. The URL still NAMES the repo, so
		// the run gets past resolveRepoRoot and dies at the fetch — the case a tool
		// that answered from its stale local refs would report clean.
		dead := filepath.Join(w.dir, "gone", "medici-finance", "assay")
		git(t, w.root, "remote", "set-url", "origin", dead)

		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 when the fetch fails, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "could-not-check") {
			t.Fatalf("the output must say could-not-check in words:\n%s", out)
		}
	})

	t.Run("no local checkout configured", func(t *testing.T) {
		withScratchTemp(t)
		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root",
			filepath.Join(t.TempDir(), "nope"))
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d\n%s", code, out)
		}
	})

	t.Run("the PR moved between the API read and the fetch", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		pr := defaultPR()
		pr.HeadRefOid = strings.Repeat("d", 40) // a head the fetch will not produce
		w.install(t, pr, false)

		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 when the head moved mid-run, got %d\n%s", code, out)
		}
	})

	t.Run("a checkout whose origin names another project is REFUSED, not answered", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), false)
		git(t, w.root, "remote", "set-url", "origin", "https://github.com/someone/else.git")
		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
	})

	t.Run("a repo outside the configured set is refused before any act", func(t *testing.T) {
		withScratchTemp(t)
		code, out := cli(verbCheck, "-R", "someone/else", "7")
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "outside the configured repo set") {
			t.Fatalf("unexpected refusal text:\n%s", out)
		}
	})
}

// TestSemanticValidityIsNeverInferred is the boundary statement, enforced. A clean
// TEXTUAL merge must not produce a semantic-validity verdict — the field says
// "not-checked" and the human output says so in words.
func TestSemanticValidityIsNeverInferred(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)

	code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root, "--json")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 on a clean, behind branch, got %d\n%s", code, out)
	}
	if !strings.Contains(out, `"semanticValidity": "not-checked"`) {
		t.Fatalf("a textually-clean merge reported a semantic verdict it never computed:\n%s", out)
	}
	if strings.Contains(out, `"semanticValidity": "checked-clean"`) {
		t.Fatalf("semantic validity was inferred from textual cleanliness:\n%s", out)
	}

	code, out = cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "did NOT evaluate") {
		t.Fatalf("the human output must state the boundary rather than omit it:\n%s", out)
	}
}

// TestProbeDetectsASemanticCollision is the positive control for the ONLY detector
// deskmerge has for the #912/#913 class: two individually-fine branches that merge with
// zero textual conflicts into a tree that does not build.
//
// The fixture is a REAL Go module compiled by a REAL `go build`, not a stand-in. That
// costs a second and buys the thing a stub cannot: it proves the mechanism end to end,
// including that module discovery actually finds the module. A stubbed probe returning
// "failed" would have passed these assertions while the production discovery found
// nothing at all — which is exactly the defect a live smoke test caught in the first
// cut of this file, where the probe set was a table keyed by repo name and the key
// never matched in production.
func TestProbeDetectsASemanticCollision(t *testing.T) {
	// The collision in miniature, faithful to #912/#913: main changes emit()'s
	// signature; the PR adds a NEW CALLER using the old arity. Neither side touches the
	// other's file, so the merge is textually clean and each side compiles alone.
	// The module skeleton lives in the COMMON ANCESTOR, so neither side has to create
	// the other's file — which is what keeps the merge textually clean.
	baseFiles := map[string]string{
		"go.mod":  "module probefixture\n\ngo 1.21\n",
		"emit.go": "package main\n\nfunc emit(a, b int) int { return a + b }\n",
		"main.go": "package main\n\nfunc main() {}\n",
	}
	// The PR adds a NEW CALLER, in its own file, at the arity it can see.
	prSide := map[string]string{
		"newcaller.go": "package main\n\nfunc callFromPR() int { return emit(1, 2) }\n",
	}
	// main changes the DEFINITION's arity, in emit.go, which the PR never touches.
	mainSide := map[string]string{
		"emit.go": "package main\n\nfunc emit(a, b, c int) int { return a + b + c }\n",
	}

	t.Run("a merged tree that fails the probe is checked-failed, not clean", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorldWithBase(t, baseFiles, prSide, mainSide)
		w.install(t, defaultPR(), false)

		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root, "--probe")
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 when the merged tree does not build, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "MERGED tree does not build") {
			t.Fatalf("the probe failure must be reported as such:\n%s", out)
		}
		// The textual verdict must STILL read clean. The two answers are independent,
		// and collapsing them would hide which one failed — the merge is fine; the
		// code is not.
		if !strings.Contains(out, "mergeability  clean") {
			t.Fatalf("the textual verdict should still read clean:\n%s", out)
		}
	})

	t.Run("without --probe the same collision is invisible", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorldWithBase(t, baseFiles, prSide, mainSide)
		w.install(t, defaultPR(), false)
		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0 — this is the coverage gap the boundary statement "+
				"exists to disclose, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "not-checked") {
			t.Fatalf("the un-probed run must say so in words:\n%s", out)
		}
	})

	t.Run("a merged tree that DOES build is checked-clean", func(t *testing.T) {
		withScratchTemp(t)
		// Same shape, but main's change is arity-compatible, so the merged tree builds.
		w := newWorldWithBase(t, baseFiles, prSide, map[string]string{
			"other.go": "package main\n\nfunc unrelated() int { return 7 }\n",
		})
		w.install(t, defaultPR(), false)
		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root, "--probe")
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0 on a merged tree that builds, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "semantic      checked-clean") {
			t.Fatalf("a successful probe must report checked-clean:\n%s", out)
		}
	})

	t.Run("a tree with no touched buildable module is could-not-check, not clean", func(t *testing.T) {
		withScratchTemp(t)
		// No go.mod anywhere: real discovery finds nothing, exactly as it would on a
		// docs-only repo.
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), false)
		code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root, "--probe")
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 when no probe applies, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "not the same as a tree that builds") {
			t.Fatalf("the could-not-check must not read as a pass:\n%s", out)
		}
	})

	t.Run("the seam returning nothing is could-not-check too", func(t *testing.T) {
		withScratchTemp(t)
		stubProbeTargets(t, func(string, []string) []probeStep { return nil })
		w := newWorldWithBase(t, baseFiles, prSide, mainSide)
		w.install(t, defaultPR(), false)
		if code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root, "--probe"); code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d\n%s", code, out)
		}
	})
}

// TestProbeTargetDiscovery pins the scoping rules discoverProbeTargets applies.
func TestProbeTargetDiscovery(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"go.mod", "sub/go.mod", "sub/nested/go.mod", "vendor/dep/go.mod"} {
		write(t, root, rel, "module x\n\ngo 1.21\n")
	}
	mods := goModuleDirs(root)
	if strings.Join(mods, ",") != ".,sub,sub/nested" {
		t.Fatalf("module discovery = %v — vendor/ must be skipped and the rest found", mods)
	}

	// The NEAREST enclosing module owns a file. Attributing sub/nested/x.go to the
	// root module would build the wrong thing and report on code the toolchain never
	// compiles there.
	if got := owningModule(mods, "sub/nested/x.go"); got != "sub/nested" {
		t.Fatalf("owningModule(sub/nested/x.go) = %q, want sub/nested", got)
	}
	if got := owningModule(mods, "sub/y.go"); got != "sub" {
		t.Fatalf("owningModule(sub/y.go) = %q, want sub", got)
	}
	if got := owningModule(mods, "top.go"); got != "." {
		t.Fatalf("owningModule(top.go) = %q, want .", got)
	}

	// Only TOUCHED modules are built.
	steps := discoverProbeTargets(root, []string{"sub/y.go"})
	if len(steps) != 1 || steps[0].Dir != "sub" {
		t.Fatalf("probe targets = %v, want just sub", steps)
	}
	if len(discoverProbeTargets(root, nil)) != 0 {
		t.Fatal("no changed paths must yield no targets — and callers read that as could-not-check")
	}
}

// TestCIContractDriftIsReportedSeparately — the #898 trap. A branch predating a new
// CI script fails a check that has nothing to do with its content, and a currency tool
// that could not tell those apart would send every such PR to a worker.
func TestCIContractDriftIsReportedSeparately(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t,
		map[string]string{"pr.txt": "a\n"},
		map[string]string{".github/scripts/verify-inscope.sh": "#!/bin/sh\nexit 0\n"})
	w.install(t, defaultPR(), false)

	code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
	if code != deskkit.ExitOK {
		t.Fatalf("CI-contract drift must NOT be a failure — merging is what cures it. "+
			"got %d\n%s", code, out)
	}
	if !strings.Contains(out, "verify-inscope.sh") {
		t.Fatalf("the drifted CI path must be named:\n%s", out)
	}
	if !strings.Contains(out, "not a defect in the branch") {
		t.Fatalf("the report must tell a merge-currency failure apart from a real one:\n%s", out)
	}
}

// TestUpToDateIsANoop — an already-current PR is a no-op, and no worktree is created
// to discover that.
func TestUpToDateIsANoop(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	// Fast-forward the PR branch onto main, so the base is already an ancestor of the
	// head and there is genuinely nothing to merge.
	git(t, w.root, "push", "-q", "--force", "origin", w.baseSHA+":refs/heads/pr-branch")
	git(t, w.dir, "-C", w.remote, "update-ref", "refs/pull/7/head", w.baseSHA)
	w.headSHA = w.baseSHA
	w.install(t, defaultPR(), true)
	rul := w.rulingsFile(t, signOffURL)

	code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "already current") {
		t.Fatalf("want a noop report:\n%s", out)
	}
	w.assertNoPush(t)
	if leaked := leakedWorktrees(t, w.root); len(leaked) != 0 {
		t.Fatalf("a worktree was created to discover there was nothing to do: %v", leaked)
	}
}

// TestDryRunWritesNothing — the brief's triage probe: it runs the whole determination
// and stops before the write, and it is NOT ruling-gated because it is a read.
func TestDryRunWritesNothing(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	rul := w.rulingsFile(t, "") // UNSIGNED

	code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul, "--dry-run")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 from a dry run on a clean merge, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "dry-run: would merge") {
		t.Fatalf("want a dry-run report:\n%s", out)
	}
	w.assertNoPush(t)
	if len(*w.audits) != 1 || (*w.audits)[0].Result != deskkit.ResultDryRun {
		t.Fatalf("a dry run must audit as %q, got %+v", deskkit.ResultDryRun, *w.audits)
	}
}

// TestAuditLineOnEveryDisposition — a merge, a refusal and a dry run each write exactly
// one audit line carrying both parent SHAs and the disposition.
func TestAuditLineOnEveryDisposition(t *testing.T) {
	t.Run("a merge audits ok with both parents", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)
		if code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul); code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		if len(*w.audits) != 1 {
			t.Fatalf("want exactly one audit line, got %d", len(*w.audits))
		}
		e := (*w.audits)[0]
		if e.Result != deskkit.ResultOK || !strings.Contains(e.Detail, "parent1=") ||
			!strings.Contains(e.Detail, "parent2=") || !strings.Contains(e.Detail, rulingID) {
			t.Fatalf("audit line does not carry the ruling and both parents: %+v", e)
		}
	})

	t.Run("a conflict refusal still audits", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"README.md": "pr\n"}, map[string]string{"README.md": "main\n"})
		w.install(t, defaultPR(), true)
		rul := w.rulingsFile(t, signOffURL)
		_, _ = cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if len(*w.audits) != 1 || (*w.audits)[0].Result != deskkit.ResultRefused {
			t.Fatalf("a refusal must leave an audit line: %+v", *w.audits)
		}
	})
}

// TestNoForceEscapeHatch — there is no flag, and no environment variable, that turns a
// refusal into a proceed. Asserted against the SOURCE, because a test of the flags the
// author remembered to wire would prove nothing about the ones they added later.
func TestNoForceEscapeHatch(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{`"force"`, `"yes"`, `"no-verify-parents"`, `"skip-ruling"`, `Getenv("DESKMERGE`}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range banned {
			if strings.Contains(string(b), bad) {
				t.Fatalf("%s contains %s — deskmerge has no escape hatch, and adding one is a "+
					"change to the authority boundary, not a convenience", e.Name(), bad)
			}
		}
	}
}

// TestUnknownVerbRefuses — the verb set is closed.
func TestUnknownVerbRefuses(t *testing.T) {
	code, out := cli("land", "-R", testRepo, "7")
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "CLOSED") {
		t.Fatalf("the refusal must name the closed set:\n%s", out)
	}
}

// TestClassifyConflicts is the unit-level boundary: the list is the list.
func TestClassifyConflicts(t *testing.T) {
	listed, unlisted := classifyConflicts([]string{"STATUS.md", "docs/brief-rules.md", "", "a.go"})
	if strings.Join(listed, ",") != "STATUS.md" {
		t.Fatalf("listed = %v", listed)
	}
	if strings.Join(unlisted, ",") != "a.go,docs/brief-rules.md" {
		t.Fatalf("unlisted = %v", unlisted)
	}
	// docs/brief-rules.md is deliberately NOT regenerable: it carries duplicate rule
	// numbers 25 and 26 on main today, authored in parallel with no conflict at all.
	if _, ok := regenerable["docs/brief-rules.md"]; ok {
		t.Fatal("docs/brief-rules.md must not be auto-resolvable")
	}
}

// ---------------------------------------------------------------------------
// mutation-caught gaps
//
// Every test below was written because a mutation SURVIVED the suite as first
// authored (docs/desk-tools-gate-bar.md §4: a surviving mutation means the guard is
// not load-bearing as written). Each names the mutation it exists to catch.
// ---------------------------------------------------------------------------

// midMergeTrial leaves a scratch worktree stopped mid-merge, for the unit-level
// assertions on resolveRegenerable's internal guards. Those guards are defence in
// depth — cmdMerge routes conflicted merges away before they can be reached — so the
// only way to prove they are load-bearing is to call them directly.
func midMergeTrial(t *testing.T, w *world) *trial {
	t.Helper()
	wt, err := newWorktree(w.root, w.headSHA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wt.remove)
	if _, err := runGit(w.root, "fetch", "--quiet", "origin",
		"+refs/heads/main:refs/remotes/origin/main"); err != nil {
		t.Fatal(err)
	}
	// A conflict is expected here; the error is the point.
	_, _ = runGit(wt.dir, "merge", "--no-ff", "--no-commit", w.baseSHA)
	return &trial{wt: wt, rep: &report{HeadSHA: w.headSHA, BaseSHA: w.baseSHA}}
}

// TestResolveRegenerableInternalGuards catches: "let a MIXED conflict be resolved" and
// "drop the whole-tree residual-conflict check after regeneration".
func TestResolveRegenerableInternalGuards(t *testing.T) {
	t.Run("it refuses outright when unlisted conflicts are present", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"README.md": "pr\n", "STATUS.md": "pr\n"},
			map[string]string{"README.md": "main\n", "STATUS.md": "main\n"})
		tr := midMergeTrial(t, w)
		tr.Listed, tr.Unlisted = []string{"STATUS.md"}, []string{"README.md"}
		stubGenerator(t, "/bin/sh", "-c", "printf 'x\\n' > STATUS.md")
		err := resolveRegenerable(tr)
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("a mixed conflict must be refused whole, got %v", err)
		}
		// The MESSAGE is load-bearing here, not just the code. Without the
		// unlisted-conflicts guard the run still refuses — but downstream, at the
		// whole-tree residual check, AFTER the generator has already rewritten a file
		// and staged it. The refusal must happen BEFORE any resolution is attempted,
		// which is a different refusal with a different message.
		if err == nil || !strings.Contains(err.Error(), "unlisted conflicts present") {
			t.Fatalf("the mixed-conflict guard must fire BEFORE any generator runs; got: %v", err)
		}
	})

	t.Run("it refuses when hunks remain unmerged after regeneration", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t,
			map[string]string{"README.md": "pr\n", "STATUS.md": "pr\n"},
			map[string]string{"README.md": "main\n", "STATUS.md": "main\n"})
		tr := midMergeTrial(t, w)
		// Deliberately MIS-classified: only STATUS.md declared, README.md still
		// conflicted in the tree. This is what a classification bug would look like,
		// and the whole-tree check is the second line that catches it.
		tr.Listed, tr.Unlisted = []string{"STATUS.md"}, nil
		stubGenerator(t, "/bin/sh", "-c", "printf 'regenerated\\n' > STATUS.md")
		err := resolveRegenerable(tr)
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("residual unmerged hunks must refuse, got %v", err)
		}
		if err == nil || !strings.Contains(err.Error(), "remain unmerged") {
			t.Fatalf("the refusal must name the residual: %v", err)
		}
	})
}

// TestParentOneMustBeThePRsOwnHistory catches: "disarm the parent-1 check".
func TestParentOneMustBeThePRsOwnHistory(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), true)
	rul := w.rulingsFile(t, signOffURL)

	// Swap the parents: merge the PR head INTO main. The commit has two parents and
	// parent 2 IS the fetched base — but its first-parent history is main's, not the
	// PR's, so the PR's own line of development stops being the trunk of its branch.
	inner := runGit
	runGit = func(dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "commit" {
			_, _ = inner(dir, "merge", "--abort")
			_, _ = inner(dir, "checkout", "--detach", w.baseSHA)
			return inner(dir, "merge", "--no-ff", "--no-verify", "-m", "swapped", w.headSHA)
		}
		return inner(dir, args...)
	}
	t.Cleanup(func() { runGit = inner })

	code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 when parent 1 is not the PR head, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "parent 1") {
		t.Fatalf("the refusal must name parent 1:\n%s", out)
	}
	w.assertNoPush(t)
}

// TestNeverFastForwards catches: "fast-forward instead of forcing a merge commit".
//
// The case only exists when the PR is BEHIND but not AHEAD — a branch whose own commits
// already landed, or one opened and never pushed to. A fast-forward there produces a
// head that is genuinely current and is NOT a merge, which is the at#72 shape arrived
// at by accident rather than by intent.
func TestNeverFastForwards(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	mergeBase := git(t, w.root, "rev-parse", w.headSHA+"^")
	git(t, w.root, "push", "-q", "--force", "origin", mergeBase+":refs/heads/pr-branch")
	git(t, w.dir, "-C", w.remote, "update-ref", "refs/pull/7/head", mergeBase)
	w.headSHA = mergeBase
	w.install(t, defaultPR(), true)
	rul := w.rulingsFile(t, signOffURL)

	code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	pushed := w.remoteBranchSHA(t, "pr-branch")
	parents := strings.Fields(git(t, w.dir, "-C", w.remote, "rev-list", "--parents", "-n", "1", pushed))
	if len(parents) != 3 {
		t.Fatalf("a fast-forwardable branch was fast-forwarded instead of merged: %v", parents)
	}
}

// TestFetchFailureIsNotAnsweredFromStaleRefs catches: "answer from stale local refs
// when the fetch fails".
//
// The dangerous shape is not "the ref is missing" — that fails loudly on its own. It is
// "the ref is present from an earlier run and the fetch just failed", where a tool that
// ignored the fetch error would answer confidently from data of unknown age.
func TestFetchFailureIsNotAnsweredFromStaleRefs(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	// Plant exactly the refs a previous successful run would have left behind.
	git(t, w.root, "fetch", "-q", "origin", "+refs/pull/7/head:refs/deskmerge/pr-7")
	git(t, w.root, "fetch", "-q", "origin", "+refs/heads/main:refs/remotes/origin/main")
	w.install(t, defaultPR(), false)
	git(t, w.root, "remote", "set-url", "origin",
		filepath.Join(w.dir, "gone", "medici-finance", "assay"))

	code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6 — a failed fetch must be could-not-check even when local "+
			"refs would answer, got %d\n%s", code, out)
	}
}

// TestRulingGateEdgeCases catches: "read a neighbouring ruling's sign-off as this
// one's", "accept an App-authored acceptance", and "accept an anchorless thread link".
func TestRulingGateEdgeCases(t *testing.T) {
	t.Run("a ruling with NO sign-off line does not borrow a later ruling's", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		// signed=true on purpose: the gh stub WILL hand back a valid
		// blessing-authority comment for whatever URL is fetched. So if the section
		// walk leaks past R-5's heading and picks up R-6's signature, this run
		// authorizes and MERGES. Stubbing the fetch to fail would hide the leak behind
		// a could-not-check that looks identical to the correct answer.
		w.install(t, defaultPR(), true)
		rul := filepath.Join(w.dir, "leaky.md")
		if err := os.WriteFile(rul, []byte(
			"## R-5 Desk merge-currency\n\nStatement, and no sign-off line at all.\n\n"+
				"## R-6 Something else\n\n**Sign-off:** "+signOffURL+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 — R-5 has no sign-off line, and R-6's signature is not "+
				"R-5's. got %d\n%s", code, out)
		}
		w.assertNoPush(t)
	})

	t.Run("an anchorless THREAD link is not an authorization", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), false)
		rul := filepath.Join(w.dir, "thread.md")
		if err := os.WriteFile(rul, []byte(
			"## R-5 Desk merge-currency\n\n**Sign-off:** https://github.com/medici-finance/assay/pull/444\n"),
			0o644); err != nil {
			t.Fatal(err)
		}
		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 — a thread is written by whoever shows up. got %d\n%s", code, out)
		}
		if !strings.Contains(out, "permalink") {
			t.Fatalf("the refusal must say why a thread link is not an authorization:\n%s", out)
		}
		w.assertNoPush(t)
	})

	t.Run("an App artifact carrying the AUTHORITY's own login and id is still refused", func(t *testing.T) {
		withScratchTemp(t)
		w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
		w.install(t, defaultPR(), true)
		inner := runGH
		runGH = func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "api" {
				// login AND id are the blessing authority's; only the TYPE says App.
				// Without the type check this authorizes, which is why the two
				// conditions are independent rather than one.
				return `{"id":1,"html_url":"` + signOffURL + `",` +
					`"issue_url":"https://api.github.com/repos/medici-finance/assay/issues/444",` +
					`"body":"accepted","user":{"login":"` + blessLogin + `","id":2001,"type":"Bot"}}`, nil
			}
			return inner(args...)
		}
		t.Cleanup(func() { runGH = inner })
		rul := w.rulingsFile(t, signOffURL)
		code, out := cli(verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "never a human authorization") {
			t.Fatalf("the refusal must be the TYPE arm, not the identity arm:\n%s", out)
		}
		w.assertNoPush(t)
	})
}
