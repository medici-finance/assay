package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stub is a recording process seam. Every child process deskboot would have started is
// recorded as its real constructed argv, and a table of matchers decides which of them
// succeed. The recording is the point: the assertions that matter here are about WHAT the
// boot ran and IN WHAT ORDER — a boot that reported success without running the preflight
// would pass any assertion made only about its exit code.
type stub struct {
	calls   [][]string
	replies []reply
}

type reply struct {
	match  string // matched against the joined argv
	stdout string
	fail   bool
}

// install wires the stub and returns (home, root). root is a REAL directory: every child
// process runs with cwd=root, so a fictional path would fail every call with a chdir error
// rather than exercising the step under test.
func (s *stub) install(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	root = t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskboot-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskboot-test")
	t.Setenv("DESK_LOOP", "the-desk")

	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		for _, r := range s.replies {
			if strings.Contains(joined, r.match) {
				if r.fail {
					return exec.Command("/bin/sh", "-c", "echo stub-failure 1>&2; exit 1")
				}
				return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+r.stdout+"\nSTUBEOF")
			}
		}
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })
	return home, root
}

func (s *stub) ran(fragment string) bool {
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), fragment) {
			return true
		}
	}
	return false
}

// indexOf returns the position of the first call containing fragment, or -1.
func (s *stub) indexOf(fragment string) int {
	for i, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), fragment) {
			return i
		}
	}
	return -1
}

// happyStub returns the reply table for a boot where every step succeeds: a linked
// worktree (git-dir != common-dir), a mint that names a real token file, and a board.
func happyStub(t *testing.T, tokenPath string) []reply {
	t.Helper()
	return []reply{
		{match: "rev-parse --absolute-git-dir", stdout: "/linked/.git/worktrees/x"},
		{match: "--git-common-dir", stdout: "/repo/.git"},
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "desktoken desk", stdout: tokenPath},
		{match: "show FETCH_HEAD:STATUS.md", stdout: "# Board\n\n## Next up\n\n| item | why |\n|---|---|\n| a | x |\n| b | y |\n"},
	}
}

func writeToken(t *testing.T, home string) string {
	t.Helper()
	p := filepath.Join(home, "token")
	if err := os.WriteFile(p, []byte("not-a-real-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBootCompleteRunsEveryStepInOrder(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))

	if rc := run([]string{"the-desk", "--root", root}); rc != deskkit.ExitOK {
		t.Fatalf("boot rc = %d, want 0", rc)
	}
	// The steps that MUST have run. Asserting each one individually is what stops a
	// "successful" boot that silently skipped the envelope check.
	for _, want := range []string{
		"deskwt prune",
		"git worktree lock",
		"deskroster set --role the-desk",
		"deskroster preflight --role desk",
		"desktoken desk --repo medici-finance/assay",
		"fetch --no-tags origin main",
	} {
		if !s.ran(want) {
			t.Errorf("boot exited 0 without running %q — a boot that skips a step is a boot that "+
				"reports an envelope nobody checked", want)
		}
	}
	// Order: the preflight comes BEFORE the board fetch. A desk with no verified envelope
	// has no business reading a queue it might act on.
	if pre, board := s.indexOf("deskroster preflight"), s.indexOf("fetch --no-tags"); pre < 0 || board < 0 || pre > board {
		t.Errorf("preflight at %d, board fetch at %d — the envelope check must precede the board read", pre, board)
	}
}

// A red preflight is could-not-run for the WHOLE pass, and there is deliberately no
// override: this is the assertion that keeps it that way.
func TestRedPreflightStopsTheBootAndReadsNoBoard(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = append(happyStub(t, writeToken(t, home)),
		reply{match: "deskroster preflight", fail: true})

	if rc := run([]string{"the-desk", "--root", root}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("red preflight rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("desktoken") {
		t.Error("the token mint ran AFTER a red preflight — a red preflight stops the pass")
	}
	if s.ran("fetch --no-tags") {
		t.Error("the board was fetched after a red preflight — nothing proceeds past a red envelope")
	}
}

// $DESK_LOOP unset means every STOP.<name> flag a human is holding would silently fail to
// match. Refusing (5) rather than warning is the whole point.
func TestUnsetLoopIdentityRefusesBeforeTouchingAnything(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "")

	if rc := run([]string{"the-desk"}); rc != deskkit.ExitRefused {
		t.Fatalf("unset DESK_LOOP rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if len(s.calls) != 0 {
		t.Errorf("the identity check ran %d child processes before refusing: %v — it must refuse first",
			len(s.calls), s.calls)
	}
}

// A DESK_LOOP naming ANOTHER role points every stop flag at the wrong window.
func TestLoopIdentityMismatchRefuses(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "verify-desk")

	if rc := run([]string{"the-desk"}); rc != deskkit.ExitRefused {
		t.Fatalf("mismatched DESK_LOOP rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// A RETIRED loop name still resolves: a rename must not orphan a session, in either
// direction. worker-desk's retired spelling is the live example.
func TestRetiredLoopNameStillBoots(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))
	s.replies = append(s.replies, reply{match: "desktoken worker", stdout: writeToken(t, home)})
	t.Setenv("DESK_LOOP", "batch-fanout")

	if rc := run([]string{"worker-desk", "--root", root}); rc != deskkit.ExitOK {
		t.Fatalf("retired loop name rc = %d, want 0 — a rename must not orphan a live session", rc)
	}
}

// An unrecognised DESK_LOOP is could-not-check (6), NOT "no stop held" (which would be the
// conflation the loop roster exists to close) and NOT a plain refusal.
func TestUnknownLoopIdentityIsUnverifiable(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "the-dsek")

	if rc := run([]string{"the-desk"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown DESK_LOOP rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
}

// Booting in the SHARED checkout is refused: git will not lock a main worktree, so a
// "lock" step there could only ever be a no-op reported as done.
func TestSharedCheckoutRefusesTheBoot(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = append([]reply{
		{match: "rev-parse --absolute-git-dir", stdout: "/repo/.git"},
		{match: "--git-common-dir", stdout: "/repo/.git"},
	}, happyStub(t, writeToken(t, home))[2:]...)

	if rc := run([]string{"the-desk", "--root", root}); rc != deskkit.ExitRefused {
		t.Fatalf("shared-checkout boot rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if s.ran("worktree lock") {
		t.Error("deskboot tried to lock the shared checkout — it must refuse before attempting it")
	}
}

// An ALREADY-locked worktree is the idempotent success case: the desired end state holds.
func TestAlreadyLockedWorktreeIsIdempotent(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		if strings.Contains(joined, "worktree lock") {
			return exec.Command("/bin/sh", "-c", `echo "fatal: already locked" 1>&2; exit 128`)
		}
		for _, r := range s.replies {
			if strings.Contains(joined, r.match) {
				return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+r.stdout+"\nSTUBEOF")
			}
		}
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })

	if rc := run([]string{"the-desk", "--root", root}); rc != deskkit.ExitOK {
		t.Fatalf("already-locked rc = %d, want 0 (idempotent)", rc)
	}
}

// An EMPTY token cache authenticates as nobody while every local call still "succeeds".
// The mint proof must reject it rather than boot on it.
func TestEmptyTokenCacheFailsTheMintProof(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	empty := filepath.Join(home, "empty-token")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.replies = happyStub(t, empty)

	if rc := run([]string{"the-desk", "--root", root}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("empty token rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("fetch --no-tags") {
		t.Error("the board was fetched after an unproven mint")
	}
}

// The mint must never echo a token VALUE. This asserts the argv, which is where a value
// would have to appear for it to reach a log.
func TestMintArgvNeverCarriesATokenValue(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))
	_ = run([]string{"the-desk", "--root", root})

	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "not-a-real-token") {
			t.Fatalf("a token VALUE appeared in an argv: %v — deskboot records the PATH, never the value", c)
		}
	}
}

// A repo outside the desk set has no rostered installation to mint against.
func TestForeignRepoRefusesTheMint(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))

	if rc := run([]string{"the-desk", "--root", root, "--repo", "someone-else/private-thing"}); rc != deskkit.ExitRefused {
		t.Fatalf("foreign repo rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if s.ran("desktoken") {
		t.Error("deskboot minted against a repo outside the desk set")
	}
}

// --dry-run touches NOTHING. A plan that ran a prune would not be a plan.
func TestDryRunTouchesNothing(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	if rc := run([]string{"the-desk", "--root", root, "--dry-run"}); rc != deskkit.ExitOK {
		t.Fatalf("dry-run rc = %d, want 0", rc)
	}
	if len(s.calls) != 0 {
		t.Fatalf("--dry-run ran %d child processes: %v", len(s.calls), s.calls)
	}
}

func TestUnknownRoleRefused(t *testing.T) {
	s := &stub{}
	s.install(t)
	if rc := run([]string{"not-a-desk"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown role rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if len(s.calls) != 0 {
		t.Error("an unknown role ran a child process before refusing")
	}
}

// Every role deskboot advertises must map to a token role AND be a name the kill switch
// knows. A role in one list and not the other is a session whose stop flag never matches,
// or a boot with no envelope to check.
func TestEveryKnownRoleIsBothMappedAndStoppable(t *testing.T) {
	roles := knownRoles()
	if len(roles) < 5 {
		t.Fatalf("knownRoles() returned %d roles (%v) — the desk role set is not being derived", len(roles), roles)
	}
	for _, r := range roles {
		if tokenRoleFor(r) == "" {
			t.Errorf("role %q has no App role: there would be no envelope to check", r)
		}
		if !deskkit.IsKnownLoopName(r) {
			t.Errorf("role %q is not a loop name the kill switch knows: a STOP.%s flag would never match", r, r)
		}
	}
	// And the reverse: a mapping entry the kill switch does not know must not be
	// advertised at all.
	for loop := range loopToTokenRole {
		if !deskkit.IsKnownLoopName(loop) {
			t.Errorf("loopToTokenRole carries %q, which the kill switch does not recognise — booting it "+
				"would produce a session no human could halt", loop)
		}
	}
}

// The step list is the verb's published contract; a silent reorder or drop breaks every
// consumer that keys on a step name.
func TestStepListIsTheDocumentedContract(t *testing.T) {
	want := []string{"loop-identity", "worktree-prune", "worktree-lock", "roster-set",
		"roster-preflight", "token-mint", "board-fetch"}
	if len(bootSteps) != len(want) {
		t.Fatalf("bootSteps has %d entries, want %d", len(bootSteps), len(want))
	}
	for i := range want {
		if bootSteps[i] != want[i] {
			t.Errorf("bootSteps[%d] = %q, want %q", i, bootSteps[i], want[i])
		}
	}
	// Every step name must appear in the usage text, or --help documents a contract the
	// binary does not have.
	for _, s := range bootSteps {
		if !strings.Contains(usage, s) {
			t.Errorf("step %q is not documented in --help", s)
		}
	}
}

// --help must name the engine seam. That is how a reader of the CLI learns where the verb
// sits in the loop without reading the loop's source.
func TestHelpNamesTheEngineSeam(t *testing.T) {
	if !strings.Contains(usage, "engine seam: BOOT") {
		t.Error("deskboot --help does not name its engine seam")
	}
}

func TestVersionAndHelpAreUnguardedReads(t *testing.T) {
	s := &stub{}
	s.install(t)
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
	if rc := run([]string{"help"}); rc != deskkit.ExitOK {
		t.Fatalf("help rc = %d, want 0", rc)
	}
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no-args rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestKillSwitchIsHonoured(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc := run([]string{"the-desk"}); rc != deskkit.ExitDisabled {
		t.Fatalf("disabled rc = %d, want %d", rc, deskkit.ExitDisabled)
	}
	if len(s.calls) != 0 {
		t.Error("a disabled deskboot still ran a child process")
	}
}

func TestBoardSummaryCountsNextUpRows(t *testing.T) {
	// The heading spelling is the whole point (assay#333): statusgen emits `## Next up`
	// (a SPACE), so that spelling MUST be counted, not just the hyphenated form. Both are
	// asserted so the two tools stay locked together whichever way the heading is written.
	for _, heading := range []string{"## Next up", "## Next-up"} {
		t.Run(heading, func(t *testing.T) {
			board := "# Status\n\n" + heading + "\n\n| item | why |\n|---|---|\n| a | x |\n| b | y |\n\n## Other\n\n| c | z |\n"
			rows, section := summariseBoard(board)
			if rows != 2 {
				t.Errorf("rows = %d, want 2 (the header and separator are not rows, and the next heading ends "+
					"the section)", rows)
			}
			if !strings.Contains(strings.ToLower(section), "next") {
				t.Errorf("section = %q, want the Next-up heading", section)
			}
		})
	}
}

func TestRepoSlugFromURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:medici-finance/assay.git":       "medici-finance/assay",
		"https://github.com/medici-finance/assay.git":   "medici-finance/assay",
		"https://user@github.com/medici-finance/assay":  "medici-finance/assay",
		"ssh://git@github.com:443/medici-finance/assay": "medici-finance/assay",
		"not-a-url": "",
	}
	for in, want := range cases {
		if got := repoSlugFromURL(in); got != want {
			t.Errorf("repoSlugFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}
