package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Tests for the --open write-confinement gate (openPreflight), the fixes for
// F-sec-open-write-outside-root and F-sec-open-commits-staged-index on PR #1511.
//
// FAIL-FIRST: each of the three refusal tests below REDS against the pre-fix
// openRebaselinePR (no preflight, configured root map preferred over --root, bare
// `git commit -m`). The pre-fix run rewrites the other checkout's brief, commits the
// pre-staged file, and cuts the branch from an ambient feature HEAD. The deskpr seam is
// stubbed in every test, so no test ever runs the real deskpr.

const confineRepo = "example-org/example-repo"

const confineBriefRel = "docs/streams/s/brief-01-x.md"

const confineBrief = "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n# Brief 01\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|-------|---------|--------|\n| 1 | check | `test -f tools/old/widget.go` | exists |\n"

// gitRun runs git in root and returns trimmed stdout, failing the test on error.
func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// staleRenameCheckout builds a checkout whose brief row 1 pins tools/old/widget.go, which git
// records moving by one rename hop to tools/new/widget.go (a safe:rename row). The fetched base
// refs/remotes/origin/main is set to HEAD, so the checkout is at its base and clean.
func staleRenameCheckout(t *testing.T) string {
	t.Helper()
	root := gitRepo(t)
	writeFile(t, root, "tools/old/widget.go", "package old\n")
	writeFile(t, root, confineBriefRel, confineBrief)
	gitCommit(t, root, "add widget and brief")
	if err := os.MkdirAll(filepath.Join(root, "tools/new"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "mv", "tools/old/widget.go", "tools/new/widget.go")
	gitCommit(t, root, "move widget")
	gitRun(t, root, "update-ref", baseRef, "HEAD")
	return root
}

// stubDeskpr replaces the deskpr seam for one test and returns a pointer to its call count.
func stubDeskpr(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := deskprCreate
	deskprCreate = func(root, title, bodyFile string) int {
		calls++
		return deskkit.ExitOK
	}
	t.Cleanup(func() { deskprCreate = orig })
	return &calls
}

// classifyRow loads the brief from root and classifies row 1, asserting it is safe:rename.
func classifyRow(t *testing.T, factsRoot string, lb loadedBrief) (verifyRow, RowFacts, Classification) {
	t.Helper()
	row, ok := lb.row(1)
	if !ok {
		t.Fatalf("fixture brief %s has no Verify row 1", lb.Path)
	}
	f := gatherRowFacts(factsRoot, row, lb.RiskBearing, lb.RiskReason)
	cls := Classify(f)
	if cls.Verdict != SafeRename {
		t.Fatalf("fixture row should classify safe:rename, got %s (%s)\nfacts=%+v", cls.Verdict, cls.Reason, f)
	}
	return row, f, cls
}

func rebaselineBranches(t *testing.T, root string) string {
	t.Helper()
	return gitRun(t, root, "branch", "--list", "rebaseline/*")
}

// TestOpenRefusesBriefOutsideRoot — F-sec-open-write-outside-root. With the configured root map
// pointing at checkout B, a --root A run must (a) resolve the brief under A, and (b) when handed
// a brief that lives in B anyway, refuse before any mutation: B's brief byte-identical and
// clean, A still on its original branch with no rebaseline/* branch, and deskpr never called.
func TestOpenRefusesBriefOutsideRoot(t *testing.T) {
	a := staleRenameCheckout(t)
	b := staleRenameCheckout(t)
	calls := stubDeskpr(t)

	// (a) resolution prefers --root over the configured root map. DESK_ROOTS may only re-point a
	// repo the environment allows (it cannot widen the set), so use the first compiled default
	// whose re-point actually takes effect, and point it at B.
	t.Run("resolution-prefers-root", func(t *testing.T) {
		t.Setenv(deskkit.RootsEnv, "")
		defaults, _ := deskkit.ConfiguredRoots()
		mapped := ""
		for _, d := range defaults {
			t.Setenv(deskkit.RootsEnv, d.Repo+"="+b)
			if deskkit.RootForRepo(d.Repo) == b {
				mapped = d.Repo
				break
			}
		}
		if mapped == "" {
			t.Skip("could-not-check: no compiled default root can be re-pointed in this environment, so the root-map preference is not exercised here")
		}
		lbA, err := loadBrief(mapped, a, "s/01")
		if err != nil {
			t.Fatal(err)
		}
		if rel, _ := filepath.Rel(a, lbA.Path); strings.HasPrefix(rel, "..") {
			t.Fatalf("s/01 with --root %s resolved to %s (the configured root map's checkout) — must resolve under --root", a, lbA.Path)
		}
	})

	// (b) a brief outside --root is refused before any write.
	lbB, err := loadBrief("", b, filepath.Join(b, confineBriefRel))
	if err != nil {
		t.Fatal(err)
	}
	row, f, cls := classifyRow(t, a, lbB)
	branchBefore := gitRun(t, a, "rev-parse", "--abbrev-ref", "HEAD")

	code := openRebaselinePR(a, confineRepo, lbB, row, f, cls)

	if code == deskkit.ExitOK {
		t.Fatalf("--open with a brief outside --root: exit 0, want a refusal")
	}
	got, _ := os.ReadFile(filepath.Join(b, confineBriefRel))
	if string(got) != confineBrief {
		t.Fatalf("the other checkout's brief was MODIFIED — --open wrote outside --root:\n%s", got)
	}
	if st := gitRun(t, b, "status", "--porcelain"); st != "" {
		t.Fatalf("the other checkout was left dirty: %q", st)
	}
	if now := gitRun(t, a, "rev-parse", "--abbrev-ref", "HEAD"); now != branchBefore {
		t.Fatalf("--root was switched to %q (was %q) by a refused --open", now, branchBefore)
	}
	if br := rebaselineBranches(t, a); br != "" {
		t.Fatalf("a refused --open created a branch in --root: %q", br)
	}
	if *calls != 0 {
		t.Fatalf("deskpr create was called %d time(s) on a refused --open", *calls)
	}
}

// TestOpenRefusesDirtyIndex — F-sec-open-commits-staged-index. An unrelated file staged in
// --root must never ride along in the one-row commit: --open refuses before creating a branch,
// the staged file stays staged and uncommitted, and deskpr is never called.
func TestOpenRefusesDirtyIndex(t *testing.T) {
	a := staleRenameCheckout(t)
	calls := stubDeskpr(t)
	writeFile(t, a, "notes.txt", "unrelated\n")
	gitRun(t, a, "add", "notes.txt")
	headBefore := gitRun(t, a, "rev-parse", "HEAD")

	lb, err := loadBrief("", a, filepath.Join(a, confineBriefRel))
	if err != nil {
		t.Fatal(err)
	}
	row, f, cls := classifyRow(t, a, lb)
	code := openRebaselinePR(a, confineRepo, lb, row, f, cls)

	if code == deskkit.ExitOK {
		t.Fatalf("--open with a staged unrelated file: exit 0, want a refusal")
	}
	if now := gitRun(t, a, "rev-parse", "HEAD"); now != headBefore {
		files := gitRun(t, a, "show", "--name-status", "--format=", "HEAD")
		t.Fatalf("a commit was made on a dirty index (HEAD %s -> %s), carrying:\n%s", headBefore, now, files)
	}
	if br := rebaselineBranches(t, a); br != "" {
		t.Fatalf("a refused --open created a branch: %q", br)
	}
	if st := gitRun(t, a, "status", "--porcelain"); st != "A  notes.txt" {
		t.Fatalf("the operator's staged file should be left exactly as it was, got status %q", st)
	}
	if *calls != 0 {
		t.Fatalf("deskpr create was called %d time(s) on a refused --open", *calls)
	}
}

// TestOpenRefusesHeadNotBase — the branch is cut from HEAD, so HEAD must be the fetched base.
// A checkout sitting on an extra, unreviewed commit is refused: that commit would otherwise be
// carried into the one-row PR.
func TestOpenRefusesHeadNotBase(t *testing.T) {
	a := staleRenameCheckout(t)
	calls := stubDeskpr(t)
	writeFile(t, a, "extra.txt", "unreviewed\n")
	gitCommit(t, a, "unreviewed local commit")

	lb, err := loadBrief("", a, filepath.Join(a, confineBriefRel))
	if err != nil {
		t.Fatal(err)
	}
	row, f, cls := classifyRow(t, a, lb)
	code := openRebaselinePR(a, confineRepo, lb, row, f, cls)

	if code == deskkit.ExitOK {
		t.Fatalf("--open with HEAD ahead of %s: exit 0, want a refusal", baseRef)
	}
	if br := rebaselineBranches(t, a); br != "" {
		t.Fatalf("a refused --open created a branch: %q", br)
	}
	if *calls != 0 {
		t.Fatalf("deskpr create was called %d time(s) on a refused --open", *calls)
	}
}

// TestOpenCommitsOnlyTheBriefRow — the happy path the gate must not block. A clean checkout at
// its fetched base gets exactly one commit on rebaseline/s-01-row-1, parented on the base,
// touching only the brief, with the row re-pointed at the rename target, and deskpr is called
// once.
func TestOpenCommitsOnlyTheBriefRow(t *testing.T) {
	a := staleRenameCheckout(t)
	calls := stubDeskpr(t)
	base := gitRun(t, a, "rev-parse", baseRef)

	lb, err := loadBrief("", a, filepath.Join(a, confineBriefRel))
	if err != nil {
		t.Fatal(err)
	}
	row, f, cls := classifyRow(t, a, lb)
	if code := openRebaselinePR(a, confineRepo, lb, row, f, cls); code != deskkit.ExitOK {
		t.Fatalf("--open on a clean checkout at its base: exit %d, want 0", code)
	}
	if *calls != 1 {
		t.Fatalf("deskpr create called %d times, want 1", *calls)
	}
	if br := gitRun(t, a, "rev-parse", "--abbrev-ref", "HEAD"); br != "rebaseline/s-01-row-1" {
		t.Fatalf("current branch %q, want rebaseline/s-01-row-1", br)
	}
	if parent := gitRun(t, a, "rev-parse", "HEAD~1"); parent != base {
		t.Fatalf("re-baseline commit parent %s, want the fetched base %s", parent, base)
	}
	if files := gitRun(t, a, "show", "--name-only", "--format=", "HEAD"); files != confineBriefRel {
		t.Fatalf("re-baseline commit touched %q, want only %s", files, confineBriefRel)
	}
	got, _ := os.ReadFile(filepath.Join(a, confineBriefRel))
	if !strings.Contains(string(got), "`test -f tools/new/widget.go`") || strings.Contains(string(got), "tools/old/widget.go") {
		t.Fatalf("row 1 was not re-pointed at the rename target:\n%s", got)
	}
}
