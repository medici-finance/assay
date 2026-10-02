package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// outbound_test.go — the hook half of the push path (desktools-v2/10): a push by ANY route
// meets the same outbound-write check deskpr's pre-push scan runs.

const obPGWithheld = "example-withheld-slug"

// newOutboundPushFixture clones a bare origin and commits one file on branch "mine" whose
// added line is line; it returns the clone and the pushed sha.
func newOutboundPushFixture(t *testing.T, line string) (dir, sha string) {
	t.Helper()
	remoteDir := t.TempDir()
	runGitT(t, remoteDir, "init", "--bare", "-b", "main")
	seed := t.TempDir()
	runGitT(t, seed, "init", "-b", "main")
	runGitT(t, seed, "config", "user.email", "seed@test")
	runGitT(t, seed, "config", "user.name", "seed")
	runGitT(t, seed, "remote", "add", "origin", remoteDir)
	commitEmpty(t, seed, "chore: initial commit on main")
	runGitT(t, seed, "push", "origin", "main")

	dir = t.TempDir()
	runGitT(t, dir, "clone", remoteDir, ".")
	runGitT(t, dir, "config", "user.email", "w@test")
	runGitT(t, dir, "config", "user.name", "w")
	runGitT(t, dir, "checkout", "-b", "mine")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("intro\n"+line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitT(t, dir, "add", "notes.txt")
	runGitT(t, dir, "commit", "-m", "docs: add notes")
	return dir, runGitT(t, dir, "rev-parse", "HEAD")
}

func runOutboundPush(t *testing.T, dir, sha, target string) (int, string) {
	t.Helper()
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")
	defer chdir(t, dir)()
	stdin := stdinString("refs/heads/mine " + sha + " refs/heads/mine 0000000000000000000000000000000000000000\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/" + target + ".git"}, stdin, &stderr)
	return rc, stderr.String()
}

func TestRun_OutboundRefusesPersonalDataOnAddedLine(t *testing.T) {
	addr := "someone.real" + "@" + "corp-mail.dev"
	dir, sha := newOutboundPushFixture(t, "ping "+addr+" for access")

	rc, out := runOutboundPush(t, dir, sha, "example-org/tracker")
	if rc != deskkit.ExitRefused || !strings.Contains(out, deskkit.RulePIIEmail) {
		t.Fatalf("rc=%d, want %d naming %s — personal data is refused on every target\n%s",
			rc, deskkit.ExitRefused, deskkit.RulePIIEmail, out)
	}
	if !strings.Contains(out, "notes.txt:2") {
		t.Errorf("refusal does not point at the added line (notes.txt:2):\n%s", out)
	}

	// The audited override takes a personal-data refusal through.
	t.Setenv(EnvScanOverride, "the address is a published support alias, not a person")
	rc, out = runOutboundPush(t, dir, sha, "example-org/tracker")
	if rc != deskkit.ExitOK || !strings.Contains(out, "scan-override RECORDED") {
		t.Fatalf("override: rc=%d, want 0 with a recorded override\n%s", rc, out)
	}
}

func TestRun_OutboundWithheldIdentifierPublicOnlyAndNotOverridable(t *testing.T) {
	t.Setenv(deskkit.EnvWithheldIdentifiers, obPGWithheld)
	dir, sha := newOutboundPushFixture(t, "mirrors the "+obPGWithheld+" stream")

	if rc, out := runOutboundPush(t, dir, sha, "example-org/tracker"); rc != deskkit.ExitOK {
		t.Fatalf("private target: rc=%d, want 0 — the public layers do not run there\n%s", rc, out)
	}
	rc, out := runOutboundPush(t, dir, sha, "example-org/example-k8s")
	if rc != deskkit.ExitRefused || !strings.Contains(out, deskkit.RuleWithheldIdentifier) {
		t.Fatalf("public target: rc=%d, want %d naming %s\n%s", rc, deskkit.ExitRefused, deskkit.RuleWithheldIdentifier, out)
	}
	t.Setenv(EnvScanOverride, "the operator believes this one is fine")
	if rc, out := runOutboundPush(t, dir, sha, "example-org/example-k8s"); rc != deskkit.ExitRefused {
		t.Fatalf("override took a withheld identifier through: rc=%d\n%s", rc, out)
	}
}

func TestRun_OutboundUnreadableBaseIsCouldNotCheck(t *testing.T) {
	dir, sha := newOutboundPushFixture(t, "plain text")
	runGitT(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
	rc, out := runOutboundPush(t, dir, sha, "example-org/tracker")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc=%d, want 0 (client-side fail-open)\n%s", rc, out)
	}
	if !strings.Contains(out, "COULD-NOT-CHECK") || !strings.Contains(out, "outbound-write check could not read") {
		t.Fatalf("an unreadable range was not announced as could-not-check:\n%s", out)
	}
}
