package deskkit

// #489 — BoardScopeError, the read-side twin of the IsAllowedRepo write refusal.
//
// The two boundaries failed differently on the same empty set. A write verb handed an
// unknown repo refused at exit 5. A board verb handed NO repos swept nothing and
// reported the empty sweep as a clean result at exit 0. These tests pin both axes
// BoardScopeError checks, and — the load-bearing part — pin that it stays quiet once a
// repo is configured, so it cannot be satisfied by refusing everything.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installRoster writes body as the roster under a private HOME and reloads the cache.
func installRoster(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	ReloadConfig()
	t.Cleanup(ReloadConfig)
}

const (
	// rosterTrustButNoScope is the #489 reproduction: trust configured, scope not.
	rosterTrustButNoScope = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
`
	// rosterTrustAndScope adds the one thing the state above is missing.
	rosterTrustAndScope = rosterTrustButNoScope +
		"ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:private\n"
)

// TestBoardScopeError_PartialRosterIsNotAReadableScope is the axis Configured() does
// not cover, and the one that actually reproduced.
func TestBoardScopeError_PartialRosterIsNotAReadableScope(t *testing.T) {
	installRoster(t, rosterTrustButNoScope)

	// The two preconditions that make this state interesting. Asserted, because if
	// either changes the test below stops testing what it claims to.
	if !EffectiveConfig().Configured() {
		t.Fatal("precondition: this roster must report CONFIGURED — Configured() consults " +
			"Problems/Bless/Logins and deliberately not Repos")
	}
	if RosterUnconfiguredError() != nil {
		t.Fatal("precondition: RosterUnconfiguredError must return NIL here — it keys on " +
			"Configured(), which is exactly why wiring it alone would not have fixed #489")
	}

	err := BoardScopeError()
	if err == nil {
		t.Fatal("BoardScopeError() = nil with an EMPTY allowed-repo set. Every board read then " +
			"sweeps zero repos and reports the empty sweep as a result at exit 0.")
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Errorf("exit code = %d, want %d: the caller asked a question this process COULD NOT "+
			"ANSWER (6), which is not the same as one it is not allowed to answer (5)",
			got, ExitUnverifiable)
	}
	// The message has to be actionable: a refusal naming no variable sends the reader
	// looking for a bug instead of a setting.
	for _, want := range []string{"COULD-NOT-CHECK", EnvAllowedRepos, ConfigHomePath()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, err.Error())
		}
	}
}

// TestBoardScopeError_UnconfiguredRosterDefersToTheLoudRefusal pins axis 1 — and is
// the assertion that gives RosterUnconfiguredError its first caller in tools/desk.
// It was documented as "the loud refusal every trust-gated caller emits" while being
// called from nowhere but its own tests (#489 item 4).
func TestBoardScopeError_UnconfiguredRosterDefersToTheLoudRefusal(t *testing.T) {
	home := t.TempDir() // no roster file at all
	t.Setenv("HOME", home)
	ReloadConfig()
	t.Cleanup(ReloadConfig)

	if EffectiveConfig().Configured() {
		t.Fatal("precondition: an absent roster must be unconfigured")
	}
	err := BoardScopeError()
	if err == nil {
		t.Fatal("BoardScopeError() = nil with NO roster at all")
	}
	// Axis 1 must win: its message names the trust variables and the class, which the
	// scope message does not.
	if !strings.Contains(err.Error(), "NOT CONFIGURED") {
		t.Errorf("an absent roster got the SCOPE message rather than the roster-unconfigured "+
			"one; the reader is told to set ASSAY_ALLOWED_REPOS when nothing is set:\n%s", err)
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Errorf("exit code = %d, want %d", got, ExitUnverifiable)
	}
}

// TestBoardScopeError_ConfiguredScopeIsSilent is the POSITIVE CONTROL. Both tests
// above are satisfied by a function that returns an error unconditionally; this is
// what makes them mean something.
func TestBoardScopeError_ConfiguredScopeIsSilent(t *testing.T) {
	installRoster(t, rosterTrustAndScope)

	if n := len(AllowedRepos()); n != 1 {
		t.Fatalf("precondition: want exactly 1 configured repo, got %d", n)
	}
	if err := BoardScopeError(); err != nil {
		t.Fatalf("BoardScopeError() = %v with a configured repo set — the guard is refusing "+
			"reads it must permit", err)
	}
}

// TestEchoNamesAnEmptyRepoSet covers the write path's half of #489, which the read
// guard above cannot reach.
//
// Re-derived rather than taken on report: `deskpost comment medici-finance/assay
// 455` refuses at exit 5 with the line "repo medici-finance/assay is not in the
// fixed desk repo set" in BOTH the absent-roster and the scope-less-roster state. Same
// exit code, byte-identical sentence, two different conditions — one of them "there is
// no set" rather than "this repo is not in it". The refusal is CORRECT in both (an
// empty set admits nothing, which is the fail-closed direction) and is deliberately
// left alone; what was missing is any way to tell the two apart.
//
// The P3 echo is where they become distinguishable, at one site instead of thirteen.
func TestEchoNamesAnEmptyRepoSet(t *testing.T) {
	installRoster(t, rosterTrustButNoScope)

	var line string
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		if strings.HasPrefix(l, "assay-config: "+EnvAllowedRepos+"=") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("the echo carries no %s line at all", EnvAllowedRepos)
	}
	if strings.TrimSpace(line) == "assay-config: "+EnvAllowedRepos+"=" {
		t.Errorf("an EMPTY repo set echoes as a bare %q, which reads as a field nobody filled "+
			"in. It means every repo is out of scope for every tool — the same thing the "+
			"role-bindings line already says in words when nothing is bound.\ngot: %s",
			EnvAllowedRepos+"=", line)
	}
	if !strings.Contains(line, "EMPTY") {
		t.Errorf("the %s line does not name the empty set:\n%s", EnvAllowedRepos, line)
	}

	// POSITIVE CONTROL: a populated set must still echo the VALUES, not a slogan —
	// the echo's whole job (P3) is that a widening or narrowing is visible verbatim.
	installRoster(t, rosterTrustAndScope)
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		if strings.HasPrefix(l, "assay-config: "+EnvAllowedRepos+"=") {
			if !strings.Contains(l, "medici-finance/assay:ci:private") {
				t.Errorf("a populated repo set no longer echoes its values verbatim:\n%s", l)
			}
			if strings.Contains(l, "EMPTY") {
				t.Errorf("a populated repo set echoed the empty-set text:\n%s", l)
			}
		}
	}
}

// TestBoardScopeError_SaysNothingAboutReachability pins the boundary of the guard.
// It answers "is there a repo to read?", never "can that repo be reached?" — the
// latter stays each caller's obligation, and a guard that appeared to cover it
// would invite callers to drop their own error handling.
func TestBoardScopeError_SaysNothingAboutReachability(t *testing.T) {
	installRoster(t, rosterTrustAndScope)
	if err := BoardScopeError(); err != nil {
		t.Fatalf("BoardScopeError() = %v; it must pass on a configured-but-unreached repo — "+
			"it makes no network call and cannot know", err)
	}
}
