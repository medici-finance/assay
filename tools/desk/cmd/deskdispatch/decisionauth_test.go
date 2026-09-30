package main

// decisionauth_test.go — issue 1146. deskdispatch handed the dispatching role's credential
// only to the claim child (issue 1151); the DECISION-GATE child — the consumer repo's
// tools/decision-issue.sh, a script that shells out to the forge CLI itself — was started
// through runCmd with a nil environment. It therefore ran on whatever forge login happened to
// be ambient in the dispatcher's shell: nothing in a sandboxed desk window, or a human's own
// login, so the decision issue a human-gated item puts in front of the human was filed under
// the wrong identity (or not at all).
//
// THE FIX. The claim step's single role-token resolution (resolveClaimAuth) now also carries
// the ENVIRONMENT-shaped hand-over of the same credential (claimAuth.scriptEnv), whatever
// shape the claim tool itself takes it in. The decision child runs under that environment,
// the credential is resolved before the claim (so a mint failure stops the dispatch with
// nothing durable taken), and stepDecision refuses to start the script at all when it has
// neither a handed-over credential nor an explicit GH_TOKEN export — never ambient.
//
// These tests drive a REAL fake decision script through the exec seam and assert on the
// credential the child OBSERVED, the same technique claimauth_test.go uses for the claim
// child. Nothing here reaches a forge.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionChildScript is the fake decision-issue.sh. Each invocation appends one record to
// the file named by DECISION_CHILD_RECORD — its argv and the GH_TOKEN / GITLAB_TOKEN it could
// see — then prints the line the real script prints on success.
const decisionChildScript = `#!/bin/sh
{
  printf 'argv=%s\n' "$*"
  printf 'GH_TOKEN=%s\n' "${GH_TOKEN-}"
  printf 'GITLAB_TOKEN=%s\n' "${GITLAB_TOKEN-}"
  printf -- '--\n'
} >> "$DECISION_CHILD_RECORD"
echo "created: decision issue #12"
exit 0
`

type decisionChildRecord struct {
	argv, ghToken, gitlabToken string
}

// installDecisionChild plants the fake as root's tools/decision-issue.sh and wraps the exec
// seam so that script (and only it) really runs; every other child keeps the harness's canned
// reply. It returns the record path the child appends to.
func installDecisionChild(t *testing.T, s *stub, root string) (record string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(decisionScriptRel)), []byte(decisionChildScript), 0o700); err != nil {
		t.Fatal(err)
	}
	record = filepath.Join(t.TempDir(), "decision-child.record")
	t.Setenv("DECISION_CHILD_RECORD", record)
	prev := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if strings.HasSuffix(name, filepath.FromSlash(decisionScriptRel)) {
			s.calls = append(s.calls, append([]string{name}, args...))
			return exec.Command(name, args...)
		}
		return prev(name, args...)
	}
	t.Cleanup(func() { execCommand = prev })
	return record
}

// ensureRecord reads the record of the decision child's `ensure` invocation; it fails the test
// when the child never ran, so no assertion below can pass vacuously.
func ensureRecord(t *testing.T, record string) decisionChildRecord {
	t.Helper()
	b, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the fake decision child left no record — it never ran: %v", err)
	}
	for _, block := range strings.Split(string(b), "--\n") {
		var r decisionChildRecord
		for _, ln := range strings.Split(block, "\n") {
			k, v, _ := strings.Cut(ln, "=")
			switch k {
			case "argv":
				r.argv = v
			case "GH_TOKEN":
				r.ghToken = v
			case "GITLAB_TOKEN":
				r.gitlabToken = v
			}
		}
		if strings.HasPrefix(r.argv, "ensure ") {
			return r
		}
	}
	t.Fatalf("the fake decision child recorded no ensure invocation:\n%s", b)
	return decisionChildRecord{}
}

// planClaimTool puts the claim tool in the requested shape. The claim child itself answers
// from the harness's canned replies (exit 0); only its SHAPE matters here, because the shape
// decides how the claim credential is carried (--token-file vs GH_TOKEN) and the decision
// child must receive the credential in the environment either way.
func planClaimTool(t *testing.T, root, tool string) {
	t.Helper()
	if tool == "go-binary" {
		plantGoClaimChild(t)
		return
	}
	plantScripts(t, root)
}

// FRESH HUMAN-GATED DISPATCH, no GH_TOKEN exported: the decision child — a script that shells
// out to the forge CLI — must see the dispatching role's minted token in GH_TOKEN, for BOTH
// claim-tool shapes. On the unfixed code it ran with a nil environment and saw nothing (the
// harness clears GH_TOKEN), i.e. it would have run on the ambient login. The token is minted
// ONCE and shared with the claim, so both writes carry one identity.
func TestDecisionChildReceivesTheMintedRoleTokenInItsEnvironment(t *testing.T) {
	for _, tool := range []string{"go-binary", "legacy-script"} {
		t.Run(tool, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			planClaimTool(t, root, tool)
			record := installDecisionChild(t, s, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			mints := stubMint(t, stubMintedToken, nil)

			rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", "spec.md",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0", rc)
			}
			r := ensureRecord(t, record)
			if r.ghToken != stubMintedToken {
				t.Fatalf("the decision script saw GH_TOKEN=%q, want the dispatching role's minted token — its "+
					"forge-CLI calls would have run on the ambient login (argv: %s)", r.ghToken, r.argv)
			}
			if strings.Contains(r.argv, stubMintedToken) {
				t.Errorf("the token VALUE appeared on the decision child's command line: %s", r.argv)
			}
			if len(*mints) != 1 {
				t.Errorf("the role token was minted %d time(s), want exactly 1 — the claim and the decision "+
					"gate must share ONE resolution so both writes carry one identity: %v", len(*mints), *mints)
			}
			if os.Getenv("GH_TOKEN") != "" {
				t.Errorf("GH_TOKEN leaked into the dispatcher's own environment: %q", os.Getenv("GH_TOKEN"))
			}
		})
	}
}

// An explicit GH_TOKEN export WINS for the decision child exactly as it does for the claim:
// nothing is minted and the child sees the operator's export.
func TestDecisionChildSeesTheExplicitGHTokenAndNothingIsMinted(t *testing.T) {
	for _, tool := range []string{"go-binary", "legacy-script"} {
		t.Run(tool, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			planClaimTool(t, root, tool)
			record := installDecisionChild(t, s, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			t.Setenv("GH_TOKEN", "example-explicit-export")
			mints := stubMint(t, stubMintedToken, nil)
			stubTokenIdentity(t, dispatcherAppIdentity, nil) // issue 1631: the export IS the role's App

			rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", "spec.md",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0", rc)
			}
			if r := ensureRecord(t, record); r.ghToken != "example-explicit-export" {
				t.Errorf("the decision child saw GH_TOKEN=%q, want the explicit export", r.ghToken)
			}
			if len(*mints) != 0 {
				t.Errorf("the role token was minted %d time(s) although an explicit GH_TOKEN was exported", len(*mints))
			}
		})
	}
}

// FAIL CLOSED, BEFORE ANYTHING DURABLE: when the role token cannot be minted, a human-gated
// dispatch stops with exit 6 before the claim — so neither the claim child nor the decision
// child ever runs, and no child falls back to the ambient login.
func TestHumanGatedDispatchFailsClosedOnMintRefusalBeforeAnyChild(t *testing.T) {
	for _, tool := range []string{"go-binary", "legacy-script"} {
		t.Run(tool, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			planClaimTool(t, root, tool)
			record := installDecisionChild(t, s, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			stubMint(t, "", errors.New("example mint refusal: no App key for this role"))

			err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", "spec.md",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if err == nil {
				t.Fatal("a human-gated dispatch whose role token could not be minted returned nil")
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
				t.Fatalf("rc = %d, want %d (unverifiable): %v", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable, err)
			}
			if _, statErr := os.Stat(record); statErr == nil {
				b, _ := os.ReadFile(record)
				t.Fatalf("the decision child RAN despite the mint refusal — the ambient-auth fallback:\n%s", b)
			}
			if s.ran("acquire") || s.ran("decision-issue.sh") {
				t.Fatalf("a child ran despite the mint refusal: %v", s.calls)
			}
		})
	}
}

// THE BACKSTOP. stepDecision itself refuses to start the decision script when it was handed no
// credential and no GH_TOKEN is exported: a caller that forgets to thread the credential gets
// an exit-6 refusal naming the gate, never a silent run on the ambient login.
func TestStepDecisionRefusesToRunTheScriptOnAmbientAuth(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // clears GH_TOKEN
	record := installDecisionChild(t, s, root)
	script := filepath.Join(root, filepath.FromSlash(decisionScriptRel))

	_, err := stepDecision(dispatchOpts{root: root, brief: "spec.md"}, true, allowedRepo, script, claimAuth{})
	if err == nil {
		t.Fatal("stepDecision ran the decision script with no credential handed over and none exported")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d: %v", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable, err)
	}
	if !strings.Contains(err.Error(), stepDecisionGate) {
		t.Errorf("the refusal does not name the %s step: %v", stepDecisionGate, err)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		b, _ := os.ReadFile(record)
		t.Fatalf("the decision child RAN on ambient auth:\n%s", b)
	}
}

// The decision-gate report line names HOW the child authenticated, never the token value.
func TestDecisionGateLineNamesTheCredentialSourceNotTheToken(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantGoClaimChild(t)
	installDecisionChild(t, s, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldErr := os.Stderr
	os.Stderr = w
	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", "spec.md",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	w.Close()
	os.Stderr = oldErr
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, rerr := r.Read(buf)
		sb.Write(buf[:n])
		if rerr != nil {
			break
		}
	}
	out := sb.String()
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0:\n%s", rc, out)
	}
	line := ""
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, stepDecisionGate+" OK:") {
			line = ln
		}
	}
	if line == "" {
		t.Fatalf("no %s OK line was printed:\n%s", stepDecisionGate, out)
	}
	for _, want := range []string{"GH_TOKEN", stubMintedTokenPath(t, home)} {
		if !strings.Contains(line, want) {
			t.Errorf("the decision-gate line does not carry %q: %s", want, line)
		}
	}
	if strings.Contains(out, stubMintedToken) {
		t.Errorf("the token VALUE was printed:\n%s", out)
	}
}

// GITLAB: on a GitLab-served project the decision child gets the role's GitLab PAT in
// GH_TOKEN and GITLAB_TOKEN — the same custody the claim uses — and the GitHub App minter is
// never called.
func TestGitLabDecisionChildGetsTheGitLabPATInItsEnvironment(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	installGLStamp(t, home)
	plantGoClaimChild(t)
	record := installDecisionChild(t, s, root)
	mints := stubMint(t, stubMintedToken, nil)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@gitlab.com:" + glProject + ".git"},
		{match: "deskwt add", stdout: "/private/tmp/worker-home"},
	}
	rc := run([]string{"item-1", "--root", root, "--repo", glProject, "--gate-human", "--brief", "spec.md",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab human-gated dispatch rc = %d, want 0", rc)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitLab-served project: %v", len(*mints), *mints)
	}
	r := ensureRecord(t, record)
	if r.ghToken != glPATPlaceholder || r.gitlabToken != glPATPlaceholder {
		t.Fatalf("the decision child saw GH_TOKEN=%q GITLAB_TOKEN=%q, want the role's GitLab PAT %q in both",
			r.ghToken, r.gitlabToken, glPATPlaceholder)
	}
}
