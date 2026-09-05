package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeDesktokenSource is compiled once (TestMain) into a temp dir placed FIRST on PATH, so
// the tool's `desktoken worker` invocation hits this canned stand-in instead of a real App
// mint. It appends its argv to FAKEGH_LOG when set; the in-process execCommand recorder is
// the authoritative one used by assertions.
//
// It models the REAL cross-installation failure mode from #562, not just a canned success:
// `desktoken worker` resolves an "owner" from `--repo <slug>` — defaulting to "example-org"
// when the flag is ABSENT, exactly like the production tool — and mints a token whose VALUE
// encodes that owner ("fake-worker-installation-token-for-<owner>"). The fake forge
// (forgeRecorder, below) then checks that the token presented in the Authorization header
// belongs to the owner of the repo the request addresses, and answers a mismatch the way
// GitHub's real API does for an installation token scoped to the wrong org. Without deskreply
// forwarding --repo, that pairing reproduces the bug and
// TestReplyMintsWorkerTokenScopedToOwnRepo / TestReplyMediciFinanceRepoSucceeds fail exactly
// as the real tool did.
//
// (The `gh` half of this fake is gone with the transport: deskreply launches no forge CLI, so
// there is nothing left for a fake CLI to stand in for. The owner-mismatch assertion it
// carried moved to the fake forge, one layer down, where it checks the credential that was
// actually presented rather than an environment variable that was handed over.)
const fakeDesktokenSource = `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if log := os.Getenv("FAKEGH_LOG"); log != "" {
		if f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(f, strings.Join(args, "\x00"))
			f.Close()
		}
	}
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	flagVal := func(name string) string {
		for i, a := range args {
			if a == name && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	owner := func(repoSlug string) string {
		if i := strings.IndexByte(repoSlug, '/'); i >= 0 {
			return repoSlug[:i]
		}
		return repoSlug
	}
	if !has("worker") {
		os.Exit(0)
	}
	// desktoken worker [--repo <slug>]: mirror the REAL desktoken default — owner is parsed
	// from --repo when present, else defaults to "example-org" (the bug #562 fixed was
	// deskreply never passing --repo, so this always resolved "example-org").
	o := "example-org"
	if repo := flagVal("--repo"); repo != "" {
		o = owner(repo)
	}
	f, err := os.CreateTemp("", "fake-worker-token-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp token: %v\n", err)
		os.Exit(1)
	}
	if _, werr := f.WriteString("fake-worker-installation-token-for-" + o + "\n"); werr != nil {
		f.Close()
		os.Remove(f.Name())
		os.Exit(1)
	}
	f.Close()
	if cerr := os.Chmod(f.Name(), 0o600); cerr != nil {
		os.Remove(f.Name())
		os.Exit(1)
	}
	abs, _ := filepath.Abs(f.Name())
	fmt.Println(abs)
}
`

var (
	fakeGHDir string
	origPATH  string
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origPATH = os.Getenv("PATH")
	dir, err := os.MkdirTemp("", "deskreply-fakegh")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.25\n"), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeDesktokenSource), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	buildDT := exec.Command("go", "build", "-o", filepath.Join(dir, "desktoken"), ".")
	buildDT.Dir = dir
	buildDT.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, berr := buildDT.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build fake desktoken: %v\n%s\n", berr, out)
		os.Exit(1)
	}
	fakeGHDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// --- fixtures -------------------------------------------------------------------

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const ghURL = "https://github.com/example-org/tracker.git"

// newBaseFixture builds a scratch worktree on feature/test-branch whose origin remote
// parses to the allowed repo example-org/tracker. deskreply never pushes,
// so no bare/push remote is needed — read-only git (rev-parse / config) is all it runs.
func newBaseFixture(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", ghURL)
	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")
	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	return work
}

const medforURL = "https://github.com/medici-finance/assay.git"

// newMediciFixture is newBaseFixture's twin, but its origin resolves to
// medici-finance/assay — an org OTHER than desktoken's hardcoded "example-org"
// default owner. It exists to reproduce #562: the worker App is installed on BOTH
// example-org and medici-finance, so a worktree whose origin is under medici-finance is
// exactly the case where forgetting to forward --repo to `desktoken worker` silently
// mints a token for the wrong installation instead of failing loudly.
func newMediciFixture(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", medforURL)
	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")
	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	return work
}

// withEnv points deskkit's runtime dir at a fresh HOME, prepends the fake desktoken to PATH,
// binds getwd to work, installs the in-process command recorder AND the fake forge, and
// returns the recorded argv slice. The forge recorder is reachable as forgeRec(t) — it is
// stored per-test so the many existing cases that only need the argv slice keep their
// signature.
func withEnv(t *testing.T, work string) *[][]string {
	t.Helper()
	fixtureHome := t.TempDir()
	t.Setenv("HOME", fixtureHome)
	plantFixtureRoster(t, fixtureHome)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test")
	// Outward verbs refuse with $DESK_LOOP unset (the per-loop stop flag has nothing to
	// match against). Every case below exercises the verb PAST that gate, so the harness
	// presents a loop identity; the refusal itself has its own test.
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("PATH", fakeGHDir+string(os.PathListSeparator)+origPATH)

	oldwd := getwd
	getwd = func() (string, error) { return work, nil }
	t.Cleanup(func() { getwd = oldwd })

	calls := &[][]string{}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })

	oldGate := publicRepoGateFn
	publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error { return nil }
	t.Cleanup(func() { publicRepoGateFn = oldGate })

	currentForge = newForgeRecorder(t)
	t.Cleanup(func() { currentForge = nil })

	return calls
}

// currentForge is the fake forge withEnv installed for the running test. A package var rather
// than a second return value so the cases below keep the signature they had before the
// transport migration — the diff stays about the ASSERTIONS, not about plumbing.
var currentForge *forgeRecorder

func forgeRec(t *testing.T) *forgeRecorder {
	t.Helper()
	if currentForge == nil {
		t.Fatal("no fake forge is installed — call withEnv first")
	}
	return currentForge
}

// bodyFileWith writes body to a temp file and returns its path.
func bodyFileWith(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "reply.md")
	writeFile(t, p, body)
	return p
}

// seedAudit appends n `deskreply` ok entries dated now, SCOPED TO PR `pr`, so AllowWrite
// can be driven to its rate-limit refusal. HOME must already be set by withEnv.
//
// The PR field is not decoration: the budget is per-PR, and checkPRBudget matches on
// (repo, pr). A seed that set only Repo — as this helper did when the constant was renamed
// from RateLimitPerHour to RateLimitPerPRPerHour without the scope following it — seeds a
// bucket the test does not exercise, so the reply under test sees an EMPTY per-PR budget
// and posts. Seed the scope the assertion is about.
func seedAudit(t *testing.T, n, pr int) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		prNum := pr
		e := deskkit.Entry{
			TS:     time.Now().UTC().Format(time.RFC3339),
			Tool:   "deskreply",
			Verb:   "reply",
			Repo:   "example-org/tracker",
			PR:     &prNum,
			Result: deskkit.ResultOK,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
}

// --- argv assertions ------------------------------------------------------------

func gitCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range calls {
		if len(c) > 0 && filepath.Base(c[0]) == "git" {
			out = append(out, c)
		}
	}
	return out
}

func callContainsAll(c []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, a := range c {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func anyCall(calls [][]string, want ...string) bool {
	for _, c := range calls {
		if callContainsAll(c, want...) {
			return true
		}
	}
	return false
}

// forbiddenWritePaths are the forge WRITE surfaces deskreply must never touch: it is a worker
// tool, so it can post nothing a board or desk reads as a reviewer signal, and it never
// opens/edits/merges/closes a change. The only mutating operations it may perform are a
// comment post and — on the --workpad path — an edit of its OWN comment.
//
// This is the successor to the argv-era `forbiddenGHVerbs` list, and it is stricter in the way
// that matters: an argv check could only refuse WORDS a CLI was handed, so a mutation issued
// through some other spelling was invisible to it. This enumerates the write surfaces
// themselves, and the catch-all below refuses any write that is neither of the two allowed.
var forbiddenWritePaths = []string{"/reviews", "/merge", "/labels", "/pulls", "/issues/7$"}

func assertOnlyCommentWrites(t *testing.T, rec *forgeRecorder) {
	t.Helper()
	for _, r := range rec.writes() {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/comments"):
			continue // the comment post
		case r.Path == "/graphql" && strings.Contains(r.Body, "updateIssueComment"):
			continue // the --workpad edit of the tool's own comment
		}
		t.Fatalf("deskreply made a write outside its two allowed operations: %s", r)
	}
	for _, r := range rec.writes() {
		for _, bad := range forbiddenWritePaths {
			if strings.Contains(r.Path, strings.TrimSuffix(bad, "$")) &&
				!strings.HasSuffix(r.Path, "/comments") {
				t.Fatalf("deskreply wrote to a forbidden surface %q: %s", bad, r)
			}
		}
	}
}

// TestPublicRepoGateWired — deskreply refuses (exit 5) when the repo is
// public and the target issue carries no qualifying +1. Uses a custom gate stub that
// simulates a public-repo-without-+1 result, and asserts ZERO write calls were made.
func TestPublicRepoGateWired(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	// Override the gate stub with one that refuses as if the repo is public with no +1.
	publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error {
		return deskkit.Refused("public-repo gate: " + owner + "/" + repo + " is public with no +1")
	}

	body := bodyFileWith(t, "Re-reviewed the delta.")
	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("reply on public repo without +1 rc = %d, want 5 (refused)", rc)
	}

	// Assert ZERO mutating write calls were made.
	assertNoComment(t, forgeRec(t))
}

func assertNoComment(t *testing.T, rec *forgeRecorder) {
	t.Helper()
	if rec.posted() {
		t.Fatalf("a comment was posted on a path that must not post: %v", rec.writes())
	}
}

func assertNoGitPush(t *testing.T, calls [][]string) {
	t.Helper()
	if anyCall(gitCalls(calls), "push") {
		t.Fatalf("deskreply pushed with git — it must never push: %v", gitCalls(calls))
	}
}

func anyDesktokenCall(calls [][]string) bool {
	for _, c := range calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			return true
		}
	}
	return false
}

// --- tests ----------------------------------------------------------------------

func TestReplySuccessPostsOnlyOneComment(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	body := bodyFileWith(t, "Addressed in abc1234: the check is now re-verified in-tool.")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("reply rc = %d, want 0", rc)
	}
	rec := forgeRec(t)
	if !rec.posted() {
		t.Fatalf("expected a comment post; forge writes: %v", rec.writes())
	}
	assertOnlyCommentWrites(t, rec) // never a review, a ready flip, a merge, a label, a create
	assertNoGitPush(t, *calls)
}

func TestReplyMintsWorkerToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	body := bodyFileWith(t, "posted as the worker App")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("reply rc = %d, want 0", rc)
	}
	if !anyDesktokenCall(*calls) {
		t.Fatalf("expected `desktoken worker` call; calls: %v", *calls)
	}
}

// TestReplyMintsWorkerTokenScopedToOwnRepo is the direct regression test for #562:
// deskreply must pass `--repo <this worktree's own repo>` to `desktoken worker`, not
// mint blind. Before the fix, mintWorkerToken called `desktoken worker` with no --repo
// at all, so desktoken (in production) fell back to its hardcoded "example-org" default
// owner regardless of the actual target repo.
func TestReplyMintsWorkerTokenScopedToOwnRepo(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	body := bodyFileWith(t, "desktoken must be told which repo it's minting for")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("reply rc = %d, want 0", rc)
	}
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			if !callContainsAll(c, "--repo", "example-org/tracker") {
				t.Fatalf("desktoken worker call did not forward --repo <this worktree's repo>: %v", c)
			}
		}
	}
	if !found {
		t.Fatalf("expected a `desktoken worker` call; calls: %v", *calls)
	}
}

// TestReplyMediciFinanceRepoSucceeds is the end-to-end regression test for #562: a
// worktree whose origin is under an org OTHER than desktoken's hardcoded "example-org"
// default (medici-finance/assay here — the worker App is installed on both
// accounts) must still mint a token that gh can actually use against that repo.
//
// Before the fix, mintWorkerToken never forwarded --repo, so the fake (and, verified
// against live desktoken/gh during the #562 investigation, the REAL) desktoken
// resolution defaulted to owner "example-org" — minting a token for the WRONG
// installation. The subsequent `gh pr view` then failed exactly as GitHub's real API
// does for a token with no access to the target repo: "GraphQL: Could not resolve to a
// Repository with the name '...'", surfacing as deskreply exit 6. This test fails the
// same way on the pre-fix code path (proved by reverting the fix locally) and passes
// once --repo is forwarded.
func TestReplyMediciFinanceRepoSucceeds(t *testing.T) {
	work := newMediciFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_PR_REPO", "medici-finance/assay")
	body := bodyFileWith(t, "replying on a repo under an org other than example-org")

	rc := run([]string{"medici-finance/assay", "7", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("reply on a medici-finance-origin worktree rc = %d, want 0 (#562 regression: "+
			"desktoken must resolve the medici-finance installation, not silently default to example-org)", rc)
	}
	if !forgeRec(t).posted() {
		t.Fatalf("expected a comment post; forge writes: %v", forgeRec(t).writes())
	}
}

func TestReplyIdempotentNoopSameBodySameHead(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	body := bodyFileWith(t, "same body, same head")

	if rc := run([]string{"example-org/tracker", "7", "--body-file", body}); rc != deskkit.ExitOK {
		t.Fatalf("first reply rc = %d, want 0", rc)
	}
	// Reset the recorders; the SECOND identical invocation must post nothing.
	*calls = nil
	forgeRec(t).requests = nil
	if rc := run([]string{"example-org/tracker", "7", "--body-file", body}); rc != deskkit.ExitOK {
		t.Fatalf("duplicate reply rc = %d, want 0 (noop)", rc)
	}
	assertNoComment(t, forgeRec(t))
}

func TestReplyNotMyOwnBranchRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_PR_HEAD", "someone-elses-branch") // PR head != this worktree's branch
	body := bodyFileWith(t, "trying to reply on a PR that is not mine")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("reply on a not-my-branch PR rc = %d, want 5", rc)
	}
	assertNoComment(t, forgeRec(t))
}

func TestReplyPRNotOpenRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_PR_STATE", "MERGED")
	body := bodyFileWith(t, "reply on a merged PR")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("reply on a non-open PR rc = %d, want 5", rc)
	}
	assertNoComment(t, forgeRec(t))
}

func TestReplyOriginMismatchRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	body := bodyFileWith(t, "replying on a different (allowed) repo than my origin")

	// example-org/agents IS an allowed repo, but it is not THIS worktree's origin.
	rc := run([]string{"example-org/agents", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("origin-mismatch reply rc = %d, want 5", rc)
	}
	assertNoComment(t, forgeRec(t))
}

func TestReplyRepoNotAllowedRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	body := bodyFileWith(t, "x")

	rc := run([]string{"otheruser/otherrepo", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("repo outside the set rc = %d, want 5", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge for a disallowed repo: %v", rec.requests)
	}
}

func TestReplyOversizeBodyRefusesNoCall(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	body := bodyFileWith(t, strings.Repeat("a", maxBodyBytes+1))

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("oversize body rc = %d, want 5", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite an oversize body: %v", rec.requests)
	}
}

func TestReplySecretInBodyRefusesNoCall(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	body := bodyFileWith(t, "here is the token ghp_"+strings.Repeat("a", 36))

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("secret in body rc = %d, want 5", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite a secret in the body: %v", rec.requests)
	}
}

func TestReplyGitSHAInBodyPasses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	// A 40-char lowercase-hex git SHA must NOT trip the secret scan.
	body := bodyFileWith(t, "fixed in "+strings.Repeat("a1b2c3d4", 5))

	if rc := run([]string{"example-org/tracker", "7", "--body-file", body}); rc != deskkit.ExitOK {
		t.Fatalf("git-SHA body rc = %d, want 0 (SHAs must pass the scan)", rc)
	}
}

func TestReplyMissingBodyFileRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	if rc := run([]string{"example-org/tracker", "7"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing --body-file rc = %d, want 5", rc)
	}
}

func TestReplyKillSwitchDisabled(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	body := bodyFileWith(t, "should never post")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("kill switch rc = %d, want 3", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge while the kill switch was armed: %v", rec.requests)
	}
}

func TestReplyRateLimitedNoCall(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	seedAudit(t, deskkit.RateLimitPerPRPerHour, 7) // PR 7's budget already spent this hour
	body := bodyFileWith(t, "one reply too many")

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("rate-limited reply rc = %d, want 4", rc)
	}
	assertNoComment(t, forgeRec(t))
}

// TestNoReviewerAppPath is the mechanical proof of the identity separation (
// updated for the worker-App cutover): the deskreply package must NOT import the
// deskpost reviewer-App mint package and must NOT read any REVIEWER App env vars. The
// WORKER App token IS allowed — deskreply shells out to `desktoken worker` to mint it —
// but the reviewer App voice must remain exclusive to deskpost so the unforgeable review
// gate holds. It walks the AST of every non-test source file (so explanatory comments that
// mention deskpost/REVIEWER_* do not false-positive — comments are not AST nodes here) and
// checks import paths + string literals.
func TestNoAppTokenPath(t *testing.T) {
	// deskpost = reviewer App mint path. deskreply must never import it.
	forbiddenImportSub := []string{"deskpost"}
	// Only REVIEWER App env vars are forbidden; WORKER App identity is accessed
	// via desktoken worker (shell-out), not via env vars or imports.
	forbiddenLiteralSub := []string{"REVIEWER_APP_ID", "REVIEWER_INSTALL_ID", "REVIEWER_APP_PRIVATE_KEY"}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob sources: %v", err)
	}
	scanned := 0
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue // test sources legitimately name the forbidden strings as assertions
		}
		scanned++
		af, perr := parser.ParseFile(fset, f, nil, 0) // no ParseComments: comments are not walked
		if perr != nil {
			t.Fatalf("parse %s: %v", f, perr)
		}
		for _, imp := range af.Imports {
			p := strings.ToLower(strings.Trim(imp.Path.Value, `"`))
			for _, bad := range forbiddenImportSub {
				if strings.Contains(p, bad) {
					t.Fatalf("%s imports %q — deskreply must not import the App token/mint path", f, p)
				}
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			for _, bad := range forbiddenLiteralSub {
				if strings.Contains(val, bad) {
					t.Fatalf("%s contains a string literal %q referencing an App env var — deskreply reads no App identity", f, val)
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("scanned 0 source files — the no-App-path check proved nothing")
	}

	// Best-effort transitive proof: the whole build graph carries no deskpost dependency.
	out, lerr := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if lerr != nil {
		t.Logf("skipping `go list -deps` transitive check: %v", lerr)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(line, "deskpost") {
			t.Fatalf("deskreply's dependency graph includes %q — it must not depend on deskpost", line)
		}
	}
}

func TestParseRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/example-org/tracker.git":              "example-org/tracker",
		"https://github.com/example-org/agents":                   "example-org/agents",
		"git@github.com:example-org/examples.git":                 "example-org/examples",
		"ssh://git@github.com/example-org/example-reconciler.git": "example-org/example-reconciler",
	}
	for in, want := range cases {
		got, err := parseRepo(in)
		if err != nil || got != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// TestGateSeamIsRealInProduction — the seam these tests replace must, in a fresh
// binary, be the real deskkit.PublicRepoGate.
//
// This closes a class the review found twice. publicRepoGateFn is stubbed by every
// other test here, so the tests above prove only that the caller PROPAGATES the gate's
// error — they would pass identically if production had never been bound to the gate
// at all. That is not hypothetical: at head 1b91e146 `deskpr create` returned exit 6
// on EVERY repo, private ones included, and no test noticed, because they all ran
// against a stub. Asserting the production binding is what makes the other assertions
// mean something.
func TestGateSeamIsRealInProduction(t *testing.T) {
	// Package-level vars are initialised before any test runs, so compare against a
	// sentinel captured at init rather than the possibly-stubbed current value.
	if productionGateFn == nil {
		t.Fatal("productionGateFn is nil — the seam has no recorded production binding")
	}
	if fmt.Sprintf("%p", productionGateFn) != fmt.Sprintf("%p", deskkit.PublicRepoGate) {
		t.Fatal("publicRepoGateFn is not bound to deskkit.PublicRepoGate at init — the gate " +
			"is stubbed out in the shipped binary, and every other gate test here is vacuous")
	}
}
