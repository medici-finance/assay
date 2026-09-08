package loopengine

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimnamespace_test.go — the claim-layer brief's Verify row 7.
//
// The reader in this package answers ONE question for the dispatcher: which items are
// already claimed. When it is pointed at a namespace the writer no longer uses, it does not
// error and it does not warn — it returns an empty set, which the dispatcher reads as "every
// slot is free" and acts on. Two machines then work the same brief. That is the worst
// failure this primitive has, and it is invisible to any test of either side alone, so it
// gets its own row: the reader's namespace must be DERIVED from the writer's, not merely
// equal to it today.

// TestClaimReaderNamespaceMatchesWriter proves the derivation two ways: statically, that the
// prefix this package lists is the shared constant itself; and behaviourally, that a ref
// created at the path the WRITER builds (deskkit.ClaimRefPath — the same call the release
// path uses to name what it deletes) is found by this reader, while a ref one namespace over
// is not.
func TestClaimReaderNamespaceMatchesWriter(t *testing.T) {
	// Static half: one definition, not two spellings that agree today.
	if claimRefPrefix != deskkit.ClaimRefsPrefix {
		t.Fatalf("the reader lists %q but the claim namespace is %q — a reader pointed at the wrong "+
			"namespace reports every slot free", claimRefPrefix, deskkit.ClaimRefsPrefix)
	}

	// Behavioural half: the reader must find exactly what the writer's own ref-path builder
	// names, in a real git repo.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; the static half above still holds")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "base")

	const key = "example--example-stream--05"
	ref, err := deskkit.ClaimRefPath(key)
	if err != nil {
		t.Fatalf("ClaimRefPath(%q): %v", key, err)
	}
	// The claim, at the path the WRITER would delete.
	run("update-ref", "refs/"+ref, "HEAD")
	// A ref in the namespace the claim used to live in, plus an ordinary branch. Neither is
	// a claim, and a reader that counted either would over-report claims and stall dispatch.
	run("update-ref", "refs/dispatch/legacy--key", "HEAD")
	run("update-ref", "refs/heads/feat/some-work", "HEAD")

	got := DispatchClaimKeys(root)
	if len(got) != 1 || got[0] != key {
		t.Fatalf("DispatchClaimKeys = %v, want exactly [%s]: the reader and the writer do not agree on "+
			"where a claim lives", got, key)
	}
}
