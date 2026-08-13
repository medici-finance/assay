package main

import (
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// These tests are the assay port's strengthening over #1556: they prove each
// transport-exec option is refused BY NAME, not incidentally as "unknown flag" or
// "extra operand". Each case asserts three things together, because any one alone can
// pass for the wrong reason:
//
//	(a) the exit code is 5 (refused) — not 0, not 6;
//	(b) the refusal MESSAGE names the option — so the guard is the named one, not the
//	    FlagSet's generic error;
//	(c) NO `git fetch` was invoked — the refusal happened before any act.
//
// (b) is what makes these incapable of passing if the named guard is deleted: without
// checkTransportExec the FlagSet still refuses, so (a) and (c) still hold, and only the
// message assertion fails.
//
// (b) only holds if the message is the one deskgit ACTUALLY emitted and if it is matched
// on something only the named guard says. Two ways to lose that, both fixed here:
//
//   - Capturing the message from a DIRECT call to checkTransportExec (rather than from
//     run()'s stderr) tests the helper but not its WIRING. Proven: deleting the
//     `checkTransportExec(args)` call from cmdFetch leaves the whole package green,
//     because the FlagSet/operand guard still yields exit 5 and still runs no fetch.
//     Same lesson as TestFetch_ScrubIsWiredToChild (re-review) — assert the wiring,
//     not the helper.
//   - Matching on the option NAME alone does not distinguish the two refusals: cmdFetch
//     wraps the FlagSet error as `refused: bad flags — flag provided but not defined:
//     -upload-pack`, which contains "upload-pack" too. namedGuardMarker is the phrase
//     only checkTransportExec produces, so the pair (marker + name) is what pins the
//     refusal to the named guard.

// namedGuardMarker appears ONLY in checkTransportExec's refusal, never in the FlagSet's
// generic "flag provided but not defined" wrapping. Matching it is what makes the
// message assertion specific to the named guard rather than to any refusal at all.
const namedGuardMarker = "names the transport-exec/config-injection option"

// captureStderr runs fn with os.Stderr redirected to a temp file and returns everything
// written. A file rather than an os.Pipe, so output can never exceed a pipe buffer and
// deadlock. No test in this package calls t.Parallel, so swapping the global is safe.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = f
	fn()
	os.Stderr = old
	if cerr := f.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	b, rerr := os.ReadFile(f.Name())
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(b)
}

// runRefusal executes argv against a scratch allowed-origin repo and returns the exit
// code, the refusal text deskgit ACTUALLY wrote to stderr, and whether a `git fetch`
// argv was ever constructed. The message comes from run() — not from calling
// checkTransportExec directly — so the assertions cover the guard's wiring into cmdFetch
// as well as the guard itself.
func runRefusal(t *testing.T, argv []string) (code int, msg string, fetched bool) {
	t.Helper()
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)

	msg = captureStderr(t, func() { code = run(argv) })
	return code, msg, fetchArgv(*calls) != nil
}

// Every transport-exec / config-injection option, refused by name. The `want` string is
// the denied option the message must name.
func TestTransportExec_RefusedByName(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"upload-pack long", []string{"fetch", "--upload-pack=sh -c ':'"}, "upload-pack"},
		{"upload-pack single dash", []string{"fetch", "-upload-pack=sh -c ':'"}, "upload-pack"},
		{"upload-pack separate value", []string{"fetch", "--upload-pack", "sh -c ':'"}, "upload-pack"},
		{"upload-pack abbreviated", []string{"fetch", "--upload-p=evil"}, "upload-pack"},
		{"exec", []string{"fetch", "--exec=evil"}, "exec"},
		{"receive-pack", []string{"fetch", "--receive-pack=evil"}, "receive-pack"},
		{"upload-archive", []string{"fetch", "--upload-archive=evil"}, "upload-archive"},
		{"exec-path", []string{"fetch", "--exec-path=/tmp/evil"}, "exec-path"},
		{"config-env", []string{"fetch", "--config-env=remote.origin.uploadpack=EVIL"}, "config-env"},
		{"-c config injection", []string{"fetch", "-c", "remote.origin.uploadpack=evil"}, "config-env"},
		{"-u short", []string{"fetch", "-u"}, "upload-pack"},
		{"-e short", []string{"fetch", "-e"}, "exec"},
		// The position that mattered: after a positional operand, Go's flag package has
		// already STOPPED parsing, so the source PR reported only a generic
		// "takes no positional operands" and never named the vector.
		{"after an operand", []string{"fetch", "origin", "--upload-pack=evil"}, "upload-pack"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg, fetched := runRefusal(t, tc.argv)
			if code != deskkit.ExitRefused {
				t.Errorf("exit = %d, want %d (refused)", code, deskkit.ExitRefused)
			}
			// The marker pins the refusal to checkTransportExec; the name pins it to the
			// right option. Both are required: the FlagSet's generic wrapping also
			// contains the option name, so the name alone would pass incidentally.
			if !strings.Contains(msg, namedGuardMarker) {
				t.Errorf("refusal %q lacks the named-guard marker %q — the option was refused\n"+
					"incidentally (flag table / operand guard), or checkTransportExec is no\n"+
					"longer wired into cmdFetch", msg, namedGuardMarker)
			}
			if !strings.Contains(msg, tc.want) {
				t.Errorf("refusal message %q does not name %q", msg, tc.want)
			}
			if fetched {
				t.Error("a `git fetch` argv was constructed for a refused invocation")
			}
		})
	}
}

// -C is refused for a DIFFERENT reason than the exec options — it defeats the repo gate
// rather than executing a program — so it gets its own named refusal and its own case.
func TestTransportExec_RefusesDashCapitalC(t *testing.T) {
	code, msg, fetched := runRefusal(t, []string{"fetch", "-C", "/elsewhere"})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if !strings.Contains(msg, "working directory") {
		t.Errorf("refusal message %q does not explain the -C repo-gate escape", msg)
	}
	if fetched {
		t.Error("git fetch ran despite -C")
	}
}

// The guard must not over-refuse the verb's own flags, or deskgit does nothing.
func TestTransportExec_AllowsLegitimateFlags(t *testing.T) {
	for _, argv := range [][]string{
		{"--prune"}, {"-prune"}, {"--pr", "42"}, {"--branch", "fix/issue-1"}, {},
	} {
		if err := checkTransportExec(argv); err != nil {
			t.Errorf("checkTransportExec(%q) refused a legitimate invocation: %v", argv, err)
		}
	}
}

// optionName is the parsing seam the whole guard rests on; test it directly so a change
// to dash/`=` handling cannot quietly narrow the guard.
func TestOptionName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"--upload-pack=x", "upload-pack"},
		{"-upload-pack", "upload-pack"},
		{"--prune", "prune"},
		{"-c", "c"},
		{"origin", ""},
		{"+refs/heads/x:refs/heads/main", ""},
		{"--", ""},
		{"-", ""},
	}
	for _, c := range cases {
		if got := optionName(c.in); got != c.want {
			t.Errorf("optionName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// An unknown flag that is NOT a transport-exec option must still be refused (by the
// FlagSet) — the named guard is defence in depth, not a replacement for argv strictness.
func TestUnknownFlagStillRefused(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch", "--depth=1"}); code != deskkit.ExitRefused {
		t.Fatalf("unknown flag exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch ran despite an unknown flag")
	}
}

// A transport-exec option must be refused even when the kill switch would ALSO refuse —
// i.e. the kill switch fires first and nothing acts either way.
func TestKillSwitchBeatsTransportExec(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if code := run([]string{"fetch", "--upload-pack=evil"}); code != deskkit.ExitDisabled {
		t.Fatalf("exit = %d, want %d (kill switch is checked FIRST)", code, deskkit.ExitDisabled)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch ran while disabled")
	}
}

// The submodule pin is a port strengthening: config-driven submodule recursion would
// escape the effective-URL repo gate, so recursion is pinned OFF on the CLI.
func TestFetch_PinsNoRecurseSubmodules(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitOK {
		t.Fatalf("fetch exit = %d, want ok", code)
	}
	if got := strings.Join(fetchArgv(*calls), " "); !strings.Contains(got, "--no-recurse-submodules") {
		t.Fatalf("fetch argv %q does not pin --no-recurse-submodules", got)
	}
}
