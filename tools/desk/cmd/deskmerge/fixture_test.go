package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fixture_test.go — a REAL git world, on purpose.
//
// deskmerge's whole claim is about the shape of a commit object: exactly two parents,
// parent 2 the fetched base head, produced without a rebase or a fast-forward. A mock
// of git can only ever confirm that the code called the functions the author intended
// to call. So every test here drives real `git` against real repositories in t.TempDir():
//
//	remote/  a bare repo standing in for origin, carrying refs/pull/<N>/head exactly as
//	         GitHub serves it
//	root/    a clone — the desk's local checkout
//
// Only the two things that genuinely cannot be local are stubbed: `gh` (the PR-state
// read and the authorization fetch) and deskkit's write meter.

func TestMain(m *testing.M) {
	cleanup, err := installFixtureRoster()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot install the fixture roster:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// testRepo is in the fixture roster's allowed set.
const testRepo = "medici-finance/assay"

const testPR = 7

// blessing identity, matching the fixture roster (ASSAY_BLESS_LOGIN=ada:2001).
const (
	blessLogin = "ada"
	blessID    = 2001
	signOffURL = "https://github.com/medici-finance/assay/pull/444#issuecomment-5206838120"
)

// ---------------------------------------------------------------------------
// the git world
// ---------------------------------------------------------------------------

type world struct {
	dir    string // parent
	remote string // bare origin
	root   string // the desk's checkout
	// headSHA / baseSHA as they stand after setup; siblingSHA is a commit that is an
	// ancestor of neither, for the wrong-parent-2 control.
	headSHA    string
	baseSHA    string
	siblingSHA string
	// pushes records every `git push` argv the run constructed. It is asserted on
	// rather than a "did it decide to push" flag, so a refusal path that reaches the
	// push by some route the author did not imagine still fails the test.
	pushes *[][]string
	gitAll *[][]string
	ghAll  *[][]string
	audits *[]deskkit.Entry
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newWorld builds: main with one base commit; a PR branch off it; then `mutate` runs
// against a checkout of main to advance it (that is what makes the PR behind). The PR
// tip is published as refs/pull/<N>/head in the bare remote, exactly as GitHub does.
func newWorld(t *testing.T, prFiles, mainFiles map[string]string) *world {
	t.Helper()
	return newWorldWithBase(t, nil, prFiles, mainFiles)
}

// newWorldWithBase seeds baseFiles into the COMMON ancestor before branching.
//
// A semantic collision needs a shared starting point: the caller and the definition must
// already exist together, so that one side can change the definition and the other add a
// caller WITHOUT either touching the other's file. Creating both files on both branches
// instead produces a textual conflict, which is the case the collision is defined not to
// be.
func newWorldWithBase(t *testing.T, baseFiles, prFiles, mainFiles map[string]string) *world {
	t.Helper()
	base := t.TempDir()
	// The bare remote's PATH must name the repo: deskmerge refuses a checkout whose
	// origin points somewhere other than the repo it was asked about, and that refusal
	// is one of the behaviours under test — so the fixture has to satisfy it honestly
	// rather than be exempted from it.
	remote := filepath.Join(base, filepath.FromSlash(testRepo)+".git")
	seed := filepath.Join(base, "seed")

	if err := os.MkdirAll(filepath.Dir(remote), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, base, "init", "--bare", "--initial-branch=main", remote)
	git(t, base, "init", "--initial-branch=main", seed)
	git(t, seed, "config", "user.name", "fixture")
	git(t, seed, "config", "user.email", "fixture@example.invalid")
	write(t, seed, "README.md", "seed\n")
	write(t, seed, "STATUS.md", "generated: v0\n")
	for rel, body := range baseFiles {
		write(t, seed, rel, body)
	}
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "-m", "base")
	git(t, seed, "remote", "add", "origin", remote)
	git(t, seed, "push", "-q", "origin", "main")

	// the PR branch
	git(t, seed, "checkout", "-q", "-b", "pr-branch")
	for rel, body := range prFiles {
		write(t, seed, rel, body)
	}
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "-m", "pr work")
	prSHA := git(t, seed, "rev-parse", "HEAD")
	git(t, seed, "push", "-q", "origin", "pr-branch")
	// GitHub publishes the PR tip at refs/pull/<N>/head; reproduce it so the fetch
	// refspec under test is the production one.
	git(t, base, "-C", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", testPR), prSHA)

	// A SIBLING branch off the base. It is not an ancestor of either side, so merging
	// it produces a well-formed TWO-parent commit whose parent 2 is simply the wrong
	// commit — which is what makes it the positive control for the parent-2 SHA check.
	// Merging an ancestor would create no commit at all and would only re-test the
	// parent-COUNT arm.
	git(t, seed, "checkout", "-q", "-b", "sibling", "main")
	write(t, seed, "sibling.txt", "sibling\n")
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "-m", "sibling")
	siblingSHA := git(t, seed, "rev-parse", "HEAD")
	git(t, seed, "push", "-q", "origin", "sibling")

	// advance main
	git(t, seed, "checkout", "-q", "main")
	for rel, body := range mainFiles {
		write(t, seed, rel, body)
	}
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "-m", "main moves")
	mainSHA := git(t, seed, "rev-parse", "HEAD")
	git(t, seed, "push", "-q", "origin", "main")

	root := filepath.Join(base, "root")
	git(t, base, "clone", "-q", remote, root)
	git(t, root, "config", "user.name", "assay-desk-app[bot]")
	git(t, root, "config", "user.email", "desk@example.invalid")

	return &world{dir: base, remote: remote, root: root,
		headSHA: prSHA, baseSHA: mainSHA, siblingSHA: siblingSHA}
}

// rulingsFile writes a rulings register. signOff empty == R-5 unsigned, which is the
// state of the world today.
func (w *world) rulingsFile(t *testing.T, signOff string) string {
	t.Helper()
	p := filepath.Join(w.dir, "rulings.md")
	body := "# Issue-Flow Rulings\n\n## R-1 Close lanes\n\n**Sign-off:** " + signOff + "\n\n" +
		"## R-5 Desk merge-currency\n\nStatement.\n\n**Sign-off:** " + signOff + "\n"
	if signOff == "" {
		// The unsigned register carries a SIGNED R-1 above the unsigned R-5 on
		// purpose. That is the live shape of the real register — several rulings in one
		// file, signed independently — and it is the only shape that can catch a
		// section walk which fails to LEAVE on the next heading and reads the
		// neighbour's signature as R-5's.
		body = "# Issue-Flow Rulings\n\n## R-1 Close lanes\n\n**Sign-off:** " + signOffURL +
			"\n\n## R-5 Desk merge-currency\n\nStatement.\n\n" +
			"**Sign-off:** _(empty — the blessing authority fills with an acceptance/amendment URL)_\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// ---------------------------------------------------------------------------
// the seams
// ---------------------------------------------------------------------------

type prStub struct {
	State             string
	IsDraft           bool
	IsCrossRepository bool
	HeadRefName       string
	HeadRefOid        string // "" -> the world's real head
}

func defaultPR() prStub {
	return prStub{State: "OPEN", IsDraft: true, HeadRefName: "pr-branch"}
}

// install wires the exec seams for one run and restores them afterwards. It records
// EVERY git and gh argv, so the "zero pushes" assertions inspect what was actually
// constructed rather than what the code meant.
func (w *world) install(t *testing.T, pr prStub, signed bool) {
	t.Helper()
	prevGit, prevGH, prevAllow, prevAudit := runGit, runGH, allowWrite, auditLog
	var gitCalls, ghCalls, pushCalls [][]string
	var audits []deskkit.Entry
	w.gitAll, w.ghAll, w.pushes, w.audits = &gitCalls, &ghCalls, &pushCalls, &audits

	runGit = func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, append([]string{dir}, args...))
		if len(args) > 0 && args[0] == "push" {
			pushCalls = append(pushCalls, append([]string{dir}, args...))
		}
		return runCmdIn(dir, "git", args...)
	}
	head := pr.HeadRefOid
	if head == "" {
		head = w.headSHA
	}
	runGH = func(args ...string) (string, error) {
		ghCalls = append(ghCalls, args)
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			b, _ := json.Marshal(prInfo{
				Number: testPR, State: pr.State, IsDraft: pr.IsDraft,
				HeadRefName: pr.HeadRefName, HeadRefOid: head, BaseRefName: "main",
				IsCrossRepository: pr.IsCrossRepository,
			})
			return string(b), nil
		case len(args) > 0 && args[0] == "api":
			if !signed {
				return "", fmt.Errorf("no comment fetch expected on an unsigned world")
			}
			c := map[string]any{
				"id": 5206838120, "html_url": signOffURL,
				"issue_url": "https://api.github.com/repos/medici-finance/assay/issues/444",
				"body":      "accepted",
				"user":      map[string]any{"login": blessLogin, "id": blessID, "type": "User"},
			}
			b, _ := json.Marshal(c)
			return string(b), nil
		}
		return "", fmt.Errorf("unexpected gh call: %v", args)
	}
	allowWrite = func(string, int) error { return nil }
	auditLog = func(e deskkit.Entry) error { audits = append(audits, e); return nil }

	t.Cleanup(func() { runGit, runGH, allowWrite, auditLog = prevGit, prevGH, prevAllow, prevAudit })
}

// cli runs a verb and returns (exit code, stdout).
func cli(args ...string) (int, string) {
	var out bytes.Buffer
	err := dispatch(args, &out)
	return deskkit.ExitCodeOf(err), out.String() + errText(err)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return "\n" + err.Error()
}

// remoteBranchSHA reads a branch from the bare remote — the ONLY honest way to assert
// "nothing was pushed", because a local-side check would pass for a push that landed.
func (w *world) remoteBranchSHA(t *testing.T, branch string) string {
	t.Helper()
	return git(t, w.dir, "-C", w.remote, "rev-parse", "refs/heads/"+branch)
}

func (w *world) assertNoPush(t *testing.T) {
	t.Helper()
	if len(*w.pushes) != 0 {
		t.Fatalf("a push was constructed on a no-write path: %v", *w.pushes)
	}
	if got := w.remoteBranchSHA(t, "pr-branch"); got != w.headSHA {
		t.Fatalf("the remote PR branch moved (%s -> %s) on a no-write path", w.headSHA, got)
	}
}

// leakedWorktrees lists scratch worktrees git still knows about, plus any deskmerge-*
// directory left behind in TMPDIR.
func leakedWorktrees(t *testing.T, root string) []string {
	t.Helper()
	out := git(t, root, "worktree", "list", "--porcelain")
	var leaked []string
	for _, ln := range strings.Split(out, "\n") {
		if !strings.HasPrefix(ln, "worktree ") {
			continue
		}
		p := strings.TrimPrefix(ln, "worktree ")
		if strings.Contains(filepath.Base(p), "deskmerge-") {
			leaked = append(leaked, p)
		}
	}
	entries, err := os.ReadDir(os.TempDir())
	if err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "deskmerge-") {
				leaked = append(leaked, filepath.Join(os.TempDir(), e.Name()))
			}
		}
	}
	return leaked
}

// withScratchTemp points every scratch worktree at a private directory so
// leakedWorktrees's TMPDIR sweep cannot see another test's (or another developer's)
// leftovers and call them this test's leak.
func withScratchTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	prev := tmpWorktreeBase
	tmpWorktreeBase = func() string { return dir }
	t.Cleanup(func() { tmpWorktreeBase = prev })
	prevTMP, had := os.LookupEnv("TMPDIR")
	_ = os.Setenv("TMPDIR", dir)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("TMPDIR", prevTMP)
		} else {
			_ = os.Unsetenv("TMPDIR")
		}
	})
}
