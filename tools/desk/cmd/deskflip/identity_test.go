package main

// identity_test.go — the two identity refusals, and the negative-path proof that neither
// of them leaves a forge call behind.
//
// FAIL-FIRST NOTE. The App-token assertions were observed RED against the pre-fix code:
// with the condition and the runCmd backstop removed, TestNoAppTokenRefusesAndNeverTouches
// TheForge returns 0 and the flip mutation shows up in the recorded argv, and
// TestForgeCallRefusedWithNoMintedToken runs gh unauthenticated.
//
// The loop-identity case is DIFFERENT and says so honestly: this verb already refused an
// unset $DESK_LOOP through its caller-role condition, so that test was green before the
// change too. What the change adds is that the refusal is now a property of run() — the
// same one every outward verb makes, in the same words — rather than of one condition
// inside one verb. The test pins the behaviour either way; it is not offered as fail-first
// evidence for this verb. The verbs where that check IS new carry their own tests.

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ghCalls returns every recorded invocation of `gh` — the ones that would have reached the
// forge. A refusal that leaves ANY of these behind has already spent an identity.
func ghCalls(s *stub) [][]string {
	var out [][]string
	for _, c := range s.calls {
		if len(c) > 0 && c[0] == "gh" {
			out = append(out, c)
		}
	}
	return out
}

// With the role's App token unavailable, deskflip refuses — it does NOT proceed on the
// operator's ambient gh credential. This is the whole defect: a flip made on the ambient
// login writes under a human identity and reads afterwards as a human decision.
func TestNoAppTokenRefusesAndNeverTouchesTheForge(t *testing.T) {
	s := &stub{pr: greenPR(), reviews: approvalAtHead(t, headSHA)}
	s.install(t)
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "", "/config/home/" + role + "-token-1", errors.New("private key not found")
	}
	ghToken = ""

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused) — a missing App credential is a settled state, not a "+
			"licence to write as somebody else", rc, deskkit.ExitRefused)
	}
	if calls := ghCalls(s); len(calls) != 0 {
		t.Fatalf("the refusal still made %d forge call(s): %v", len(calls), calls)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("the refusal still mutated: %v", m)
	}
}

// The refusal has to be actionable: it names the ROLE whose token is missing and the PATH
// the token was looked for at, and it never prints a token value.
func TestNoAppTokenRefusalNamesTheRoleAndThePath(t *testing.T) {
	s := &stub{pr: greenPR(), reviews: approvalAtHead(t, headSHA)}
	s.install(t)
	const tokenPath = "/config/home/reviewer-token-1"
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "", tokenPath, errors.New("private key not found")
	}
	ghToken = ""

	err := cmdFlip([]string{"7", "--repo", privateCIRepo})
	if err == nil {
		t.Fatal("no error from a flip with no App token")
	}
	msg := err.Error()
	wantRole, ok := deskkit.TokenRoleForLoop(flipRole)
	if !ok {
		t.Fatalf("loop %s carries no App role in the shared table", flipRole)
	}
	for _, want := range []string{condAppToken, wantRole, tokenPath} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not name %q: %s", want, msg)
		}
	}
}

// The backstop below the condition: even reached directly, a gh call with no minted token
// does not run. This is what makes "never the ambient credential" a property of the code
// rather than of the current call order.
func TestForgeCallRefusedWithNoMintedToken(t *testing.T) {
	prev := ghToken
	t.Cleanup(func() { ghToken = prev })
	ghToken = ""

	var started int
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		started++
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = oldExec })

	r := runCmd("", "gh", "pr", "view", "1")
	if r.err == nil {
		t.Fatal("gh ran with no minted token")
	}
	if started != 0 {
		t.Fatalf("the refusal still started %d subprocess(es) — it must refuse BEFORE exec", started)
	}
	// git is unaffected: the repo resolution runs before any token exists.
	if r := runCmd("", "git", "rev-parse", "HEAD"); r.err != nil && started == 0 {
		t.Fatal("the token guard blocked git as well — only forge calls carry an identity")
	}
}

// And the positive half: when a token IS minted, every gh child gets it in GH_TOKEN. This
// is the assertion that would have caught the original bug, where the value was resolved
// and then never put in the environment.
func TestMintedTokenReachesTheChildEnvironment(t *testing.T) {
	prev := ghToken
	t.Cleanup(func() { ghToken = prev })
	ghToken = "installation-token-for-this-test"

	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("/bin/sh", "-c", `printf %s "$GH_TOKEN"`)
	}
	t.Cleanup(func() { execCommand = oldExec })

	r := runCmd("", "gh", "pr", "view", "1")
	if r.err != nil {
		t.Fatalf("runCmd: %v", r.err)
	}
	if r.stdout != ghToken {
		t.Fatalf("the child saw GH_TOKEN=%q, want the minted token — the read would fall through to "+
			"the ambient keyring", r.stdout)
	}
}

// $DESK_LOOP unset means a STOP.<loop> flag a human is holding can never match this
// session, so the verb refuses before it does anything at all.
func TestDeskLoopUnsetRefusesBeforeAnyWork(t *testing.T) {
	s := &stub{pr: greenPR(), reviews: approvalAtHead(t, headSHA)}
	s.install(t)
	t.Setenv("DESK_LOOP", "")

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if calls := ghCalls(s); len(calls) != 0 {
		t.Fatalf("a session with no loop identity still made %d forge call(s): %v", len(calls), calls)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a session with no loop identity still mutated: %v", m)
	}
}
