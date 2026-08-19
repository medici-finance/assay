package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// allowedSlug is in the allowed-repo set; deniedSlug is not. The test upstream is a bare repo
// whose PATH ends in the slug, so the effective-URL gate (git ls-remote --get-url, which
// deskgit now uses per finding 3) resolves it to that owner/repo.
const allowedSlug = "example-org/tracker"
const deniedSlug = "someone/random-repo"

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

// newRepo builds a scratch checkout whose origin is a real bare upstream (so `git fetch
// origin` succeeds offline) placed at <root>/<slug>.git — so the effective origin URL
// parses to <slug>. Returns the work dir.
func newRepo(t *testing.T, slug string) string {
	t.Helper()
	root := t.TempDir()

	upstream := filepath.Join(root, filepath.FromSlash(slug)+".git")
	if err := os.MkdirAll(filepath.Dir(upstream), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "", "init", "--bare", "-b", "main", upstream)

	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", upstream)
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")
	mustGit(t, work, "push", "-u", "origin", "main")

	// The upstream is a BARE LOCAL PATH origin. Since #215 a bare local path is
	// admitted only when it matches a configured local root, so register this test upstream
	// as slug's trusted root — the same act a real operator performs via DESK_ROOTS to trust
	// a local checkout. (For an out-of-set slug this entry is itself refused by
	// ConfiguredRoots, which is exactly why such a fetch is still gated.)
	t.Setenv(deskkit.RootsEnv, slug+"="+upstream)
	return work
}

// withEnv gives the tool a fresh HOME (audit/kill-switch dir), binds getwd to work, and
// installs the argv recorder. Returns the recorded git argv slice.
func withEnv(t *testing.T, work string) *[][]string {
	t.Helper()
	calls, _ := withEnvCmds(t, work)
	return calls
}

// withEnvCmds is withEnv but also captures the *exec.Cmd of every invocation, so a test
// can assert the child's ACTUAL .Env after run() — the wiring `runGit` sets after
// execCommand returns (issue #1555 re-review). runGit assigns cmd.Env before Run, so
// the captured pointer reflects the scrubbed env once run() completes.
func withEnvCmds(t *testing.T, work string) (*[][]string, *[]*exec.Cmd) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test")

	oldwd := getwd
	getwd = func() (string, error) { return work, nil }
	t.Cleanup(func() { getwd = oldwd })

	calls := &[][]string{}
	cmds := &[]*exec.Cmd{}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		c := oldExec(name, args...)
		*cmds = append(*cmds, c)
		return c
	}
	t.Cleanup(func() { execCommand = oldExec })
	return calls, cmds
}

// fetchArgv returns the argv of the `git fetch …` call the run made, or nil.
func fetchArgv(calls [][]string) []string {
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "git" && c[1] == "fetch" {
			return c
		}
	}
	return nil
}

// The pinned hardening flags must appear in every fetch (issue #1555 finding 1).
func assertHardened(t *testing.T, argv []string) {
	t.Helper()
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--upload-pack=git-upload-pack", "--refmap=", "--no-recurse-submodules"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fetch argv %q missing hardening flag %q", joined, want)
		}
	}
}

func TestFetch_Bare_HardenedArgv(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)

	if code := run([]string{"fetch"}); code != deskkit.ExitOK {
		t.Fatalf("fetch exit = %d, want %d", code, deskkit.ExitOK)
	}
	argv := fetchArgv(*calls)
	if argv == nil {
		t.Fatal("no `git fetch` was invoked")
	}
	assertHardened(t, argv)
	want := "git fetch --refmap= --upload-pack=git-upload-pack --no-recurse-submodules origin +refs/heads/*:refs/remotes/origin/*"
	if strings.Join(argv, " ") != want {
		t.Fatalf("bare fetch argv = %q\n want %q", strings.Join(argv, " "), want)
	}
}

func TestFetch_Prune(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "--prune"}); code != deskkit.ExitOK {
		t.Fatalf("fetch --prune exit = %d, want %d", code, deskkit.ExitOK)
	}
	want := "git fetch --refmap= --upload-pack=git-upload-pack --no-recurse-submodules --prune origin +refs/heads/*:refs/remotes/origin/*"
	if got := strings.Join(fetchArgv(*calls), " "); got != want {
		t.Fatalf("fetch --prune argv = %q\n want %q", got, want)
	}
}

func TestFetch_PR_BuildsPullRefspec(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	// origin has no pull/* ref, so git fetch will fail; we assert the ARGV regardless.
	run([]string{"fetch", "--pr", "42"})
	argv := fetchArgv(*calls)
	if argv == nil {
		t.Fatal("no `git fetch` invoked for --pr")
	}
	assertHardened(t, argv)
	joined := strings.Join(argv, " ")
	if !strings.HasSuffix(joined, "origin refs/pull/42/head:refs/heads/pr42") {
		t.Fatalf("--pr argv = %q, want pull refspec suffix", joined)
	}
}

func TestFetch_PR_RejectsNonDigits(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "--pr", "42; rm -rf x"}); code != deskkit.ExitRefused {
		t.Fatalf("--pr non-digit exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must not run for a non-digit --pr")
	}
}

func TestFetch_Branch_BuildsRefspec(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	run([]string{"fetch", "--branch", "fix/issue-1"})
	joined := strings.Join(fetchArgv(*calls), " ")
	if !strings.HasSuffix(joined, "origin refs/heads/fix/issue-1:refs/heads/fix/issue-1") {
		t.Fatalf("--branch argv = %q, want branch refspec suffix", joined)
	}
}

// Security review blocker: the main/master guard was case-SENSITIVE while the
// ref namespace it protects is case-INSENSITIVE on darwin/APFS, so `--branch Main` passed
// the guard and git — believing it was CREATING a ref — replaced local `main` with an
// unrelated commit at exit 0, no fast-forward check, audit line reading `ok`. The mixed-case
// spellings are the regression pins; `main`/`master` are the controls that were already green.
func TestFetch_Branch_RefusesMainMaster(t *testing.T) {
	for _, b := range []string{"main", "master", "Main", "MAIN", "mAiN", "Master", "MASTER", "mASTEr"} {
		work := newRepo(t, allowedSlug)
		calls := withEnv(t, work)
		if code := run([]string{"fetch", "--branch", b}); code != deskkit.ExitRefused {
			t.Fatalf("--branch %s exit = %d, want %d (refused)", b, code, deskkit.ExitRefused)
		}
		if fetchArgv(*calls) != nil {
			t.Fatalf("git fetch must not run for --branch %s", b)
		}
	}
}

// The magic names are only the instance; the defect class is a filesystem ref-NAMESPACE
// collision, so any existing local branch must be unrewritable by naming another spelling
// of it. An EXACT-spelling match is NOT a collision — that is the ordinary update the mode
// exists for, and blocking it would break the feature to fix the bug.
func TestFetch_Branch_RefusesCaseCollisionWithExistingRef(t *testing.T) {
	t.Run("differing case is refused before any fetch", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		mustGit(t, work, "branch", "feature-x")
		calls := withEnv(t, work)
		if code := run([]string{"fetch", "--branch", "Feature-X"}); code != deskkit.ExitRefused {
			t.Fatalf("--branch Feature-X exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if fetchArgv(*calls) != nil {
			t.Fatal("git fetch must not run when --branch collides by case with an existing ref")
		}
	})

	t.Run("exact spelling of an existing branch still fetches", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		mustGit(t, work, "branch", "feature-x")
		calls := withEnv(t, work)
		// Upstream has no feature-x, so git itself fails — but the tool must have got far
		// enough to CONSTRUCT the fetch, proving the collision check did not refuse it.
		run([]string{"fetch", "--branch", "feature-x"})
		if fetchArgv(*calls) == nil {
			t.Fatal("exact-spelling --branch must not be treated as a case-collision")
		}
	})

	t.Run("no collision when no such branch exists", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		calls := withEnv(t, work)
		run([]string{"fetch", "--branch", "Feature-X"})
		if fetchArgv(*calls) == nil {
			t.Fatal("--branch must be allowed to construct a fetch when nothing collides")
		}
	})

	// Correctness review: the check is keyed on the DESTINATION ref, so it must cover
	// --pr too. --pr N writes refs/heads/pr<N>, which is not the string the caller typed —
	// gating on the flag value covered only --branch while the doc claimed the general
	// principle. Reproduced before the fix: a differently-cased local pr<N> was replaced by
	// an unrelated commit at exit 0, non-fast-forward — the same shape as the blocker.
	t.Run("--pr destination is covered too", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		mustGit(t, work, "branch", "PR99")
		calls := withEnv(t, work)
		if code := run([]string{"fetch", "--pr", "99"}); code != deskkit.ExitRefused {
			t.Fatalf("--pr 99 against existing PR99 exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if fetchArgv(*calls) != nil {
			t.Fatal("git fetch must not run when the --pr destination collides by case")
		}
	})

	t.Run("--pr still fetches when nothing collides", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		calls := withEnv(t, work)
		run([]string{"fetch", "--pr", "99"})
		if fetchArgv(*calls) == nil {
			t.Fatal("--pr must be allowed to construct a fetch when nothing collides")
		}
	})

	// Correctness review: the fail-CLOSED branch had no test — mutating it to fail OPEN
	// left the whole suite green. Enumeration failing must produce a refusal, never a
	// silent "no collision".
	t.Run("fails closed when ref enumeration fails", func(t *testing.T) {
		why := branchCollision(filepath.Join(t.TempDir(), "does-not-exist"), "anything")
		if why == "" {
			t.Fatal("branchCollision must fail CLOSED when it cannot enumerate local refs")
		}
	})
}

func TestFetch_Branch_RejectsInjection(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	// leading '+' (force), ':' (2nd refspec), and leading '-' (flag) must all be refused.
	for _, bad := range []string{"+refs/heads/x", "x:refs/heads/main", "--upload-pack=evil"} {
		if code := run([]string{"fetch", "--branch", bad}); code != deskkit.ExitRefused {
			t.Fatalf("--branch %q exit = %d, want refused", bad, code)
		}
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must not run for an injected --branch")
	}
}

func TestFetch_ModesMutuallyExclusive(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "--pr", "1", "--branch", "x"}); code != deskkit.ExitRefused {
		t.Fatalf("combined modes exit = %d, want refused", code)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must not run when modes conflict")
	}
}

// The core of issue #1555: --upload-pack / --exec / a raw refspec cannot be smuggled
// through the verb. flag parsing refuses the unknown flag; the operand guard refuses the
// refspec. In neither case does the string reach git.
func TestFetch_RejectsUploadPackFlag(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "--upload-pack=sh -c 'touch /tmp/PROOF'"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch --upload-pack exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when a flag is refused")
	}
}

func TestFetch_RejectsRefspecOperand(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "origin", "+refs/heads/x:refs/heads/main"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch with refspec exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when operands are refused")
	}
}

func TestFetch_RefusesRepoOutsideSet(t *testing.T) {
	work := newRepo(t, deniedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch on out-of-set origin exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run for a repo outside the set")
	}
}

// finding 3 / re-review: the DECISION must be driven by the EFFECTIVE URL, not the
// recorded one. Here the recorded origin is an allowed slug, but an insteadOf rewrites the
// effective URL to a DENIED slug — deskgit must refuse and never run `git fetch`. If the
// gate reverted to the configured string, this would proceed (the mutation).
func TestFetch_EffectiveURLDrivesDecision(t *testing.T) {
	work := newRepo(t, allowedSlug) // recorded origin resolves to the allowed slug
	// Rewrite the EFFECTIVE URL (what ls-remote --get-url returns) to a denied slug.
	recorded := mustGit(t, work, "config", "--get", "remote.origin.url")
	denied := filepath.Join(t.TempDir(), filepath.FromSlash(deniedSlug)+".git")
	mustGit(t, work, "config", "url."+denied+".insteadOf", recorded)

	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (effective URL is denied)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when the EFFECTIVE URL is not allowed")
	}
}

// re-review: assert the scrub is WIRED, not just that scrubbedEnv() is correct. A
// hostile GIT_* in the process env must be absent from the CHILD git's actual .Env.
// Deleting `cmd.Env = scrubbedEnv(...)` in runGit makes this fail.
func TestFetch_ScrubIsWiredToChild(t *testing.T) {
	work := newRepo(t, allowedSlug)
	t.Setenv("GIT_SSH_COMMAND", "sh -c 'touch /tmp/should-not-run'")
	_, cmds := withEnvCmds(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitOK {
		t.Fatalf("fetch exit = %d, want ok", code)
	}
	var fetchCmd *exec.Cmd
	for _, c := range *cmds {
		if len(c.Args) >= 2 && c.Args[1] == "fetch" {
			fetchCmd = c
		}
	}
	if fetchCmd == nil {
		t.Fatal("no git fetch cmd captured")
	}
	if fetchCmd.Env == nil {
		t.Fatal("runGit did not set cmd.Env (scrub not wired) — child inherited os.Environ()")
	}
	for _, kv := range fetchCmd.Env {
		if strings.HasPrefix(kv, "GIT_SSH_COMMAND=") {
			t.Fatalf("child git inherited GIT_SSH_COMMAND — scrub not wired: %q", kv)
		}
	}
}

// re-review: the gate rejects remote-helper transport forms and padded paths, and
// requires an exact owner/repo path for host-bearing URLs.
func TestParseRepo_GateShape(t *testing.T) {
	allow := []struct{ url, want string }{
		{"https://github.com/example-org/tracker.git", "example-org/tracker"},
		{"git@github-example:example-org/tracker.git", "example-org/tracker"},
	}
	for _, c := range allow {
		got, err := parseRepo(c.url)
		if err != nil || got != c.want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", c.url, got, err, c.want)
		}
	}
	refuse := []string{
		"ext::sh -c touch /tmp/x",                             // remote-helper transport
		"transport::address",                                  // remote-helper transport
		"https://github.com/attacker/pad/example-org/tracker", // padded path
	}
	for _, u := range refuse {
		if got, err := parseRepo(u); err == nil {
			t.Errorf("parseRepo(%q) = %q, nil; want error", u, got)
		}
	}
}

// Security review / correctness review: the exact-owner/repo rule was sound, but two
// URL shapes never REACHED it — the bypass was in parseRepo's ROUTING, not in the check.
// TestParseRepo_GateShape above covers only shapes that were already closed, so neither
// bypass had a failing case; these are those cases.
//
// The `want` side is as load-bearing as the refusals: both fixes work by routing MORE input
// into the strict branch, so a fix that simply refused everything would be indistinguishable
// from a correct one. The allow list pins the legitimate shapes that must keep working —
// including `host:owner/repo`, the userless scp-like form that bypass A turned on, which git
// accepts and which must now parse correctly rather than fall to the lenient branch.
func TestParseRepo_AuthorityParsing(t *testing.T) {
	const slug = "example-org/tracker"

	t.Run("refuses", func(t *testing.T) {
		for _, c := range []struct{ url, why string }{
			{"https://evil.example.com/pad@github.com/example-org/tracker",
				"bypass B: '@' in the PATH — the real authority must not be consumed as userinfo"},
			{"ssh://evil.example.com/pad@github.com/example-org/tracker",
				"bypass B over ssh://"},
			{"evil.example.com:pad/example-org/tracker",
				"bypass A: host-bearing scp-like with NO user@ must not fall to the lenient branch"},
			{"git@evil.example.com:pad/example-org/tracker",
				"bypass A's twin WITH a user — was already refused; asserted so the asymmetry cannot return"},
			{"https://evil.example.com/a@b/example-org/tracker",
				"bypass B with the '@' in a leading path segment"},
		} {
			got, err := parseRepo(c.url)
			if err == nil {
				t.Errorf("parseRepo(%q) = %q, nil; want error\n  %s", c.url, got, c.why)
			}
		}
	})

	t.Run("still allows legitimate shapes", func(t *testing.T) {
		for _, c := range []struct{ url, want, why string }{
			{"https://github.com/example-org/tracker", slug, "plain https"},
			{"https://user@github.com/example-org/tracker", slug,
				"REAL userinfo, in the authority, must still be stripped"},
			{"ssh://git@github.com/example-org/tracker", slug, "ssh:// with userinfo"},
			{"git@github-example:example-org/tracker.git", slug, "ssh host alias, scp-like"},
			{"github.com:example-org/tracker", slug,
				"userless scp-like — git accepts it; it must now parse strictly, not leniently"},
		} {
			got, err := parseRepo(c.url)
			if err != nil || got != c.want {
				t.Errorf("parseRepo(%q) = %q, %v; want %q\n  %s", c.url, got, err, c.want, c.why)
			}
		}
	})
}

// readAudit returns the parsed audit lines written under the test's HOME.
func readAudit(t *testing.T) []deskkit.Entry {
	t.Helper()
	path := filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	var out []deskkit.Entry
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var e deskkit.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("bad audit line %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

// Security review: with residual 2 open, a legitimate refresh and a smuggled fetch used to
// produce byte-identical audit lines, so the smuggle was undetectable after the fact. The
// success line must now carry the EFFECTIVE origin URL — the value the gate actually decided
// on — which is the only compensating control available while the residual stands.
func TestFetch_AuditRecordsEffectiveOriginURL(t *testing.T) {
	work := newRepo(t, allowedSlug)
	withEnv(t, work)
	if code := run([]string{"fetch"}); code != 0 {
		t.Fatalf("fetch exit = %d, want 0", code)
	}
	entries := readAudit(t)
	last := entries[len(entries)-1]
	if last.Result != "ok" {
		t.Fatalf("audit result = %q, want ok", last.Result)
	}
	wantURL := mustGit(t, work, "ls-remote", "--get-url", "origin")
	if !strings.Contains(last.Detail, wantURL) {
		t.Errorf("audit detail = %q, want it to contain the effective origin URL %q", last.Detail, wantURL)
	}
}

// Correctness review: a URL the parser positively rejects is a REFUSAL (exit 5), not an
// "unverifiable" (exit 6). Filing smuggling probes in the same bucket as network faults is
// exactly the operator confusion the named transport-exec guard exists to prevent.
func TestFetch_UnparseableOriginIsRefusedNotUnverifiable(t *testing.T) {
	work := newRepo(t, allowedSlug)
	mustGit(t, work, "remote", "set-url", "origin",
		"https://evil.example.com/pad@github.com/example-org/tracker")
	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("unparseable origin exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must not run when the origin URL cannot be parsed")
	}
}

// The audit ledger is durable, and an https remote can legitimately carry `user:token@`.
// Recording the effective URL must not turn the audit ledger into a credential store.
func TestRedactURL(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://user:ghp_SECRETTOKEN@github.com/o/r", "https://<redacted>@github.com/o/r"},
		{"https://x-access-token:ghs_SECRET@github.com/o/r", "https://<redacted>@github.com/o/r"},
		{"ssh://git@github.com/o/r", "ssh://<redacted>@github.com/o/r"},
		{"git@github.com:o/r", "<redacted>@github.com:o/r"},
		{"https://github.com/o/r", "https://github.com/o/r"},
		{"/srv/local/o/r", "/srv/local/o/r"},
	} {
		if got := redactURL(c.in); got != c.want {
			t.Errorf("redactURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Total on hostile input: it exists to log exactly these, so it must never panic.
	for _, u := range []string{"", "://", "@", ":", "a://@", "ext::sh -c x", "https://"} {
		_ = redactURL(u)
	}
}

// Security review hygiene. Two parser defects that were NOT exploitable at the time they
// were found — the first was fail-closed only because real git rejects the resulting protocol
// name, the second bought nothing beyond the already-documented unbound host — but both would
// become load-bearing the moment a host allowlist lands. Pinned now so the fix cannot regress
// silently before then.
func TestParseRepo_PathBoundaries(t *testing.T) {
	const slug = "example-org/tracker"

	t.Run("query or fragment is never read as the path", func(t *testing.T) {
		for _, u := range []string{
			"https://evil.example.com?x=/example-org/tracker",
			"https://evil.example.com#/example-org/tracker",
			"https://evil.example.com?/example-org/tracker",
		} {
			if got, err := parseRepo(u); err == nil {
				t.Errorf("parseRepo(%q) = %q, nil; want error — the authority ended at '?'/'#', so there is no path", u, got)
			}
		}
	})

	t.Run("scheme detection is anchored to the start", func(t *testing.T) {
		// `://` buried in an scp-like path must NOT route the URL into the host-bearing
		// branch, where its trailing components would be taken as an exact owner/repo.
		for _, u := range []string{
			"evil.example.com:pad://x/example-org/tracker",
			"evil.example.com:a://b/c/example-org/tracker",
		} {
			if got, err := parseRepo(u); err == nil {
				t.Errorf("parseRepo(%q) = %q, nil; want error — scp-like, must take the strict branch", u, got)
			}
		}
	})

	t.Run("real schemes and authorities still parse", func(t *testing.T) {
		for _, c := range []struct{ url, why string }{
			{"ssh://git@[fd00::1]:22/example-org/tracker", "IPv6 literal + port"},
			{"ssh://github.com:2222/example-org/tracker", "non-default port"},
			{"https://a@b@github.com/example-org/tracker", "multi-@ authority, stripped at the last"},
		} {
			got, err := parseRepo(c.url)
			if err != nil || got != slug {
				t.Errorf("parseRepo(%q) = %q, %v; want %q — %s", c.url, got, err, slug, c.why)
			}
		}
	})
}

// CHARACTERIZATION, NOT ENDORSEMENT (correctness review). The shape below still reaches the
// gate with an allowed slug, and it is a documented residual in parseRepo's doc comment,
// main.go and README.md. It is pinned so the three documents and the code cannot drift apart
// silently: a future change that CLOSES it must delete this case and the matching residual
// paragraph in the same commit — and a change that accidentally loosens it will fail loudly
// instead of quietly widening the gate.
//
//   - Residual 1: the host is not bound (ssh host ALIASES make a hard host requirement refuse
//     the desk's real remotes), so an allowed slug on an unexpected host passes.
//
// (The former residual 2 — bare local paths took the lenient last-two-components branch — is
// CLOSED as of #215; its cases moved to TestParseRepo_LocalPathGated, which
// proves they are now refused.)
func TestParseRepo_DocumentedResiduals(t *testing.T) {
	const slug = "example-org/tracker"
	for _, c := range []struct{ url, residual string }{
		{"https://evil.example.com/example-org/tracker", "1: host is not bound"},
		{"git@evil.example.com:example-org/tracker", "1: host is not bound (scp-like)"},
	} {
		got, err := parseRepo(c.url)
		if err != nil || got != slug {
			t.Errorf("parseRepo(%q) = %q, %v; want %q\n  residual %s changed — update the residual docs in "+
				"parseRepo, main.go and README.md in THIS commit, or revert", c.url, got, err, slug, c.residual)
		}
	}
}

// #215: a BARE LOCAL PATH is no longer read leniently off its last two
// components — the exact defect that let a `url.<base>.insteadOf` rewrite point origin at any
// directory NAMED to spell an allowed slug and land foreign content on the desk's tracking
// refs. Its identity now comes from the configured local-roots allowlist (DESK_ROOTS /
// deskkit.RepoForLocalPath): only a path that IS a trusted root is admitted — by equality,
// not descendant containment — and the repo is the ROOT's, not the path's.
//
// This is the proof that the bare local path is now gated the SAME as a full path: the two
// URLs whose last two components spell the allowed slug — one absolute, one relative, exactly
// the pair that used to pass as the former residual 2 — are refused, a bare repo planted
// UNDER a trusted root is refused (equality, not descendant — #215 blocker), while a real
// trusted root parses to its configured repo (identity from the root, not the spelling).
func TestParseRepo_LocalPathGated(t *testing.T) {
	const slug = "example-org/tracker"

	// The roster must name example-org/tracker as an allowed repo, or ConfiguredRoots refuses
	// the DESK_ROOTS entry itself. ConfiguredRoots reads it from the config HOME, so plant it
	// there and point HOME at that same dir.
	rosterHome := t.TempDir()
	plantFixtureRoster(t, rosterHome)
	t.Setenv("HOME", rosterHome)

	// A trusted local checkout the operator has declared. Point DESK_ROOTS at it; only paths
	// that resolve into this root are admitted.
	trusted := t.TempDir()
	t.Setenv(deskkit.RootsEnv, slug+"="+trusted)

	t.Run("a path that IS the trusted root parses to the root's repo", func(t *testing.T) {
		got, err := parseRepo(trusted)
		if err != nil || got != slug {
			t.Errorf("parseRepo(%q) = %q, %v; want %q — a path resolving to a trusted root must parse to the root's repo",
				trusted, got, err, slug)
		}
	})

	t.Run("a path UNDER the trusted root is REFUSED (equality, not descendant — #215 blocker)", func(t *testing.T) {
		// A bare repo a caller who controls .git/config can plant inside a trusted root
		// (e.g. <root>/vendor/cache/x.git) must NOT inherit the root's identity: admitting
		// descendants is exactly what turned the compiled "." home root (== the worktree
		// deskgit fetches into) into a wildcard trust anchor. Only an EXACT root is admitted.
		sub := filepath.Join(trusted, "nested", "checkout")
		if err := os.MkdirAll(sub, 0o755); err != nil { // exists, so this is not merely a fail-closed non-existent path
			t.Fatal(err)
		}
		if got, err := parseRepo(sub); err == nil {
			t.Errorf("parseRepo(%q) = %q, nil; want error — a descendant of a trusted root must be refused, "+
				"not admitted with the root's identity (#215 blocker)", sub, got)
		}
	})

	t.Run("the former residual-2 smuggle paths are now REFUSED", func(t *testing.T) {
		// Both spell the allowed slug in their last two components and both sit OUTSIDE every
		// configured root — the exact insteadOf-reachable bypass #215 closes.
		for _, u := range []string{
			"/srv/anywhere/example-org/tracker",
			"../../example-org/tracker",
		} {
			if got, err := parseRepo(u); err == nil {
				t.Errorf("parseRepo(%q) = %q, nil; want error — a bare local path spelling an allowed slug "+
					"but outside every trusted root must be refused (#215)", u, got)
			}
		}
	})
}

// #215 blocker: under the SHIPPED DEFAULT configuration (DESK_ROOTS unset), the compiled
// home root is "." — which canonicalises to the process working directory, i.e. the very
// worktree deskgit binds its fetch to. The pre-fix code admitted any DESCENDANT of a
// configured root, so the whole worktree subtree became a trust anchor: a bare repo planted
// under it (reachable via an insteadOf rewrite) parsed to the "." root's allowed repo and
// landed foreign content on the tracking refs the desk loops merge. Nothing covered the
// default config because newRepo and TestParseRepo_LocalPathGated both set DESK_ROOTS.
//
// The fix skips a root that resolves to the process CWD AND requires equality, not
// descendant containment — so a path under the CWD is refused with DESK_ROOTS unset.
func TestParseRepo_DefaultRoots_RejectDescendantOfCWD(t *testing.T) {
	// Force the shipped default: no DESK_ROOTS, so ConfiguredRoots returns the compiled
	// topology roots (which include the "." home root that canonicalises to this CWD).
	t.Setenv(deskkit.RootsEnv, "")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// A directory planted under the worktree, named to spell an allowed slug — the exact
	// insteadOf-reachable smuggle. It need not exist: canonicalPath falls back to the cleaned
	// absolute form, so the verdict does not depend on the attacker having created it yet.
	planted := filepath.Join(cwd, "vendor", "cache", "example-org", "tracker.git")
	if got, err := parseRepo(planted); err == nil {
		t.Errorf("parseRepo(%q) = %q, nil; want error — with DESK_ROOTS unset a path under the "+
			"process working directory must NOT parse to the compiled '.' root's repo (#215 blocker)", planted, got)
	}
}

// #215: file:// URLs name a purely LOCAL path git reads off the filesystem, so they must be
// gated by the local-roots allowlist exactly like a bare local path — not by the lenient
// host-bearing exact-path rule, which would admit file:///owner/repo on its trailing
// components. An insteadOf rewrite can choose this spelling, so the gap was attacker-reachable.
func TestParseRepo_FileURLGatedByLocalRoots(t *testing.T) {
	const slug = "example-org/tracker"

	rosterHome := t.TempDir()
	plantFixtureRoster(t, rosterHome)
	t.Setenv("HOME", rosterHome)

	trusted := t.TempDir()
	t.Setenv(deskkit.RootsEnv, slug+"="+trusted)

	t.Run("file:// naming the trusted root parses to the root's repo", func(t *testing.T) {
		got, err := parseRepo("file://" + trusted)
		if err != nil || got != slug {
			t.Errorf("parseRepo(file://%s) = %q, %v; want %q — a file:// URL at a trusted root is admitted",
				trusted, got, err, slug)
		}
	})

	t.Run("file:// spelling the slug but outside every root is REFUSED", func(t *testing.T) {
		for _, u := range []string{
			"file:///example-org/tracker",
			"file://localhost/example-org/tracker",
			"file:///example-org/tracker.git",
		} {
			if got, err := parseRepo(u); err == nil {
				t.Errorf("parseRepo(%q) = %q, nil; want error — a file:// path spelling an allowed slug "+
					"outside every trusted root must be refused (#215)", u, got)
			}
		}
	})
}

// Security review: values that branchRe's character class admits but git's
// check-ref-format rejects must be the TOOL's refusal (exit 5) before delegation, not git's
// failure (exit 6) after it — so a caller mistake stays distinguishable from a network or
// tool fault in the audit line.
func TestFetch_BranchRejectsInvalidRefSequences(t *testing.T) {
	for _, bad := range []string{"a..b", "a//b", "a/", "a.", "a.lock"} {
		work := newRepo(t, allowedSlug)
		calls := withEnv(t, work)
		if code := run([]string{"fetch", "--branch", bad}); code != deskkit.ExitRefused {
			t.Errorf("--branch %q exit = %d, want %d (refused by deskgit, not by git)",
				bad, code, deskkit.ExitRefused)
		}
		if fetchArgv(*calls) != nil {
			t.Errorf("git fetch must not run for --branch %q", bad)
		}
	}
	// The guard must not over-refuse: a normal branch with dots and slashes still works.
	if err := branchRejectReason("release/v1.2.3"); err != "" {
		t.Errorf("branchRejectReason(release/v1.2.3) = %q, want ok", err)
	}
}

func TestFetch_KillSwitchDisables(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if code := run([]string{"fetch"}); code != deskkit.ExitDisabled {
		t.Fatalf("fetch with kill switch exit = %d, want %d", code, deskkit.ExitDisabled)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when disabled")
	}
}

func TestUnknownSubcommandRefused(t *testing.T) {
	work := newRepo(t, allowedSlug)
	withEnv(t, work)
	if code := run([]string{"pull"}); code != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestNoArgsRefused(t *testing.T) {
	if code := run(nil); code != deskkit.ExitRefused {
		t.Fatalf("no-args exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// finding 1 env vector: the child env must carry no GIT_* var (upload-pack via
// GIT_CONFIG_*, GIT_SSH_COMMAND, GIT_ASKPASS, …) and must force GIT_TERMINAL_PROMPT=0.
func TestScrubbedEnv_DropsGitAndDangerous(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "HOME=/home/x", "SSH_AUTH_SOCK=/tmp/agent", "LC_ALL=C",
		"GIT_SSH_COMMAND=evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=remote.origin.uploadpack",
		"GIT_CONFIG_VALUE_0=pwn", "GIT_ASKPASS=evil", "GIT_DIR=/elsewhere", "SSH_ASKPASS=evil",
		"AWS_SECRET_ACCESS_KEY=leak",
		// re-review: an unrelated, NOT-explicitly-denied var. An allowlist drops it; a
		// denylist of only the named vars would keep it. This is what makes the shape testable.
		"TOTALLY_UNRELATED_VAR=x",
	}
	got := scrubbedEnv(parent)
	kept := map[string]bool{}
	for _, kv := range got {
		kept[strings.SplitN(kv, "=", 2)[0]] = true
	}
	for _, k := range []string{"PATH", "HOME", "SSH_AUTH_SOCK", "LC_ALL"} {
		if !kept[k] {
			t.Errorf("scrubbedEnv dropped allowlisted %q", k)
		}
	}
	for _, k := range []string{"GIT_SSH_COMMAND", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0", "GIT_ASKPASS", "GIT_DIR", "SSH_ASKPASS", "AWS_SECRET_ACCESS_KEY",
		"TOTALLY_UNRELATED_VAR"} {
		if kept[k] {
			t.Errorf("scrubbedEnv KEPT non-allowlisted %q (allowlist, not denylist, required)", k)
		}
	}
	if !kept["GIT_TERMINAL_PROMPT"] {
		t.Error("scrubbedEnv must set GIT_TERMINAL_PROMPT=0")
	}
}
