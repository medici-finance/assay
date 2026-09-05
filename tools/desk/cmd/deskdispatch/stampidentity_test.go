package main

// stampidentity_test.go — the WRITER half of the dispatcher-attestation contract: the
// model stamp must be applied under the DISPATCHER App's identity, because that is the
// only applier the capability floor's reader will accept.
//
// THE DEFECT THESE PIN. The stamp step shelled out to `gh` with no token in the child's
// environment, so the labels landed under whatever credential the calling shell happened
// to hold — a role App for some other role, or the operator's own login. The floor's
// applier-aware reader then read exactly the shape it exists to refuse (a dispatched-*
// label from a non-dispatcher) and refused every correctly-dispatched, genuinely
// strong-tier PR, while an UNSTAMPED PR sailed through on the absent-attestation NOTICE.
// Present-but-untrusted was worse than absent, and the stamp the dispatcher itself wrote
// was the untrusted one.

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stubMint replaces the App-token seam and records what identity was asked for.
type mintCall struct{ role, repo string }

func stubMint(t *testing.T, token string, err error) *[]mintCall {
	t.Helper()
	var calls []mintCall
	old := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		calls = append(calls, mintCall{role: role, repo: repo})
		return token, "/tmp/example-token-path", err
	}
	oldTok := dispatcherToken
	dispatcherToken = ""
	t.Cleanup(func() {
		mintTokenFn = old
		dispatcherToken = oldTok
	})
	return &calls
}

// The stamp is applied as the DISPATCHER App — the identity the floor's reader accepts —
// and NOT as whatever role this session's own loop acts under. The session here presents
// the review loop (the reviewer App), which is precisely the case that produced the field
// failure: a stamp applied by the reviewer App is a stamp the floor cannot trust.
func TestStampAppliedAsDispatcherApp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	t.Setenv("DESK_LOOP", "pr-review-desk")
	calls := stubMint(t, "example-installation-token", nil)

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if len(*calls) == 0 {
		t.Fatal("no App token was minted for the stamp — the labels were applied under the ambient " +
			"credential, which is exactly the applier the floor refuses")
	}
	for _, c := range *calls {
		if c.role != deskkit.DispatcherRole {
			t.Errorf("the stamp authenticated as the %q App; the floor accepts only the dispatcher role %q",
				c.role, deskkit.DispatcherRole)
		}
		if c.repo != allowedRepo {
			t.Errorf("token minted for %q, want the target repo %q (an installation token is per account)",
				c.repo, allowedRepo)
		}
	}
	if dispatcherToken != "example-installation-token" {
		t.Errorf("dispatcherToken = %q — the minted token never reached the gh calls", dispatcherToken)
	}
	edit := "pr edit 77 -R " + allowedRepo + " --add-label "
	if !s.ran(edit + "dispatched-model:example-model-1") {
		t.Errorf("the model half was not applied to the PR: %v", s.calls)
	}
	if !s.ran(edit + "dispatched-tier:strong") {
		t.Errorf("the tier half was not applied to the PR: %v", s.calls)
	}
}

// A stamp that cannot be applied under the dispatcher identity is NOT applied at all. An
// untrusted stamp is worse than no stamp: absent reads UNKNOWN and proceeds with a NOTICE,
// while present-but-untrusted refuses every authority-bearing write on that PR. So a
// failed mint stops before the first label, and says so.
func TestStampNeverUsesAmbientIdentity(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	stubMint(t, "", errors.New("no credential"))

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (a stamp whose identity cannot be established is could-not-check)",
			rc, deskkit.ExitUnverifiable)
	}
	if s.ran("gh pr edit") || s.ran("gh label create") {
		t.Fatalf("a label was applied without the dispatcher token: %v", s.calls)
	}
}

// The fail-closed backstop, independent of the step that is supposed to mint first: a `gh`
// invocation with no dispatcher token does not happen at all. Without this, a future code
// path that reaches the forge before the mint silently re-creates the defect.
func TestForgeCallNeedsDeskToken(t *testing.T) {
	var ran bool
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		ran = true
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	oldTok := dispatcherToken
	dispatcherToken = ""
	t.Cleanup(func() { execCommand = old; dispatcherToken = oldTok })

	r := runCmd("", "gh", "pr", "edit", "1", "--add-label", "dispatched-tier:strong")
	if r.err == nil {
		t.Fatal("gh ran with no dispatcher token — the ambient credential is never a fallback for the stamp")
	}
	if ran {
		t.Fatal("the child process was started before the token check")
	}
	// A non-gh command is unaffected: the guard is about the identity a forge WRITE carries.
	if r := runCmd("", "git", "status"); r.err != nil {
		t.Fatalf("the token guard blocked a non-forge command: %v", r.err)
	}
}

// The token reaches the child process's environment, which is what actually decides the
// identity GitHub records for the label. Asserting the mint alone would pass while the
// token sat in a variable nothing read.
func TestChildEnvCarriesTheToken(t *testing.T) {
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("/bin/sh", "-c", `printf %s "$GH_TOKEN"`)
	}
	oldTok := dispatcherToken
	dispatcherToken = "example-installation-token"
	t.Cleanup(func() { execCommand = old; dispatcherToken = oldTok })

	r := runCmd("", "gh", "pr", "edit", "1")
	if r.err != nil {
		t.Fatalf("gh: %v", r.err)
	}
	if strings.TrimSpace(r.stdout) != "example-installation-token" {
		t.Fatalf("GH_TOKEN in the child environment = %q, want the dispatcher token — the label would be "+
			"applied under the ambient identity", r.stdout)
	}
}

// dispatcherAppLogin is the roster's desk-App login — the identity the floor accepts, read
// from the fixture roster rather than written as a literal.
func dispatcherAppLogin(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin(deskkit.DispatcherRole)
	if !ok {
		t.Fatalf("the fixture roster binds no App to the dispatcher role %q", deskkit.DispatcherRole)
	}
	return login
}

// stampReplies serves the two reads the re-stamp needs: the PR's CURRENT labels (one name
// per line, as the `--jq '.[].name'` filter emits them) and its label timeline (one
// `event\tlabel\tactor` line per event, in order).
func stampReplies(worktree, labelNames, timelineTSV string) []reply {
	return append(happyReplies(worktree),
		reply{match: "issues/77/labels", stdout: labelNames},
		reply{match: "issues/77/timeline", stdout: timelineTSV},
	)
}

// THE NO-OP THIS PINS. Labels are a SET: `--add-label` over a label the PR already carries
// changes nothing. So a PR stamped by some other login — the pre-fix dispatch path, or a
// human — stayed stamped by that login no matter how often the dispatcher re-ran, and every
// authority-bearing write on it kept refusing. A GitHub timeline is append-only, so the only
// repair the forge offers is to REMOVE the label and re-apply it as the dispatcher.
func TestStampReplacesAForeignAppliedStamp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.replies = stampReplies("/private/tmp/worker-home",
		model+"\n"+tier+"\n",
		"labeled\t"+model+"\tsome-other-login\nlabeled\t"+tier+"\tsome-other-login\n")
	stubMint(t, "example-installation-token", nil)

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	edit := "pr edit 77 -R " + allowedRepo + " "
	for _, l := range []string{model, tier} {
		if !s.ran(edit + "--remove-label " + l) {
			t.Errorf("the foreign application of %s was never removed — re-applying it on top is a no-op: %v", l, s.calls)
		}
		if !s.ran(edit + "--add-label " + l) {
			t.Errorf("%s was not re-applied as the dispatcher: %v", l, s.calls)
		}
	}
}

// A stamp already standing under the DISPATCHER is left alone: no removal, no label churn on
// every re-dispatch. Without this the step would remove and re-apply the labels on every run,
// filling the timeline with noise and briefly leaving the PR unstamped.
func TestStampLeavesItsOwnStampAlone(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	d := dispatcherAppLogin(t)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.replies = stampReplies("/private/tmp/worker-home",
		model+"\n"+tier+"\n",
		"labeled\t"+model+"\t"+d+"\nlabeled\t"+tier+"\t"+d+"\n")
	stubMint(t, "example-installation-token", nil)

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if s.ran("--remove-label") {
		t.Errorf("the dispatcher removed its OWN standing stamp — re-dispatch must not churn labels: %v", s.calls)
	}
}

// A SUPERSEDED foreign application is not a reason to remove anything: the dispatcher already
// repaired this PR, and removing the label again would undo its own repair.
func TestStampDoesNotRemoveAnAlreadyRepairedStamp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	d := dispatcherAppLogin(t)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.replies = stampReplies("/private/tmp/worker-home",
		model+"\n"+tier+"\n",
		"labeled\t"+model+"\tsome-other-login\n"+
			"labeled\t"+tier+"\tsome-other-login\n"+
			"unlabeled\t"+model+"\t"+d+"\n"+
			"unlabeled\t"+tier+"\t"+d+"\n"+
			"labeled\t"+model+"\t"+d+"\n"+
			"labeled\t"+tier+"\t"+d+"\n")
	stubMint(t, "example-installation-token", nil)

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if s.ran("--remove-label") {
		t.Errorf("a PR the dispatcher had already re-stamped was stripped again: %v", s.calls)
	}
}

// A read the step cannot complete is UNVERIFIABLE, and NO label is written. Stamping blind
// would report success while adding over a foreign label — the exact no-op this step now
// exists to defeat.
func TestStampRefusesWhenTheLabelReadFails(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = append(happyReplies("/private/tmp/worker-home"),
		reply{match: "issues/77/labels", code: 1, stderr: "labels unreadable"})
	stubMint(t, "example-installation-token", nil)

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (an unreadable label set is could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("--add-label") || s.ran("--remove-label") {
		t.Errorf("a label was written on an unverifiable read: %v", s.calls)
	}
}
