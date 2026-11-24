package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifiertarget_test.go — the dispatch half of the pre-merge verifier target
// (#7623): every malformed, misplaced or opted-out target is refused BEFORE the
// claim, and an admitted target cuts the verifier home at exactly its head.

var tgHead = strings.Repeat("a", 40)

func tgOptIn(t *testing.T, home string) {
	t.Helper()
	path := filepath.Join(home, ".config", "assay", "roster.env")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, []byte("\n"+deskkit.EnvVerifierPremergeRepos+"="+allowedRepo+"\n")...)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

func tgArgs(t *testing.T, root string, extra ...string) []string {
	return append([]string{"example-stream/07", "--root", root, "--repo", allowedRepo, "--kit", "verifier",
		"--model", "gpt-6-astra", "--brief", "spec.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}, extra...)
}

// regression: #7623
// TestTargetRefusedPreClaim: a target on another kit, half a target, an
// abbreviated head, a cross-repository dispatch and an opted-out repository
// are each refused with no child process run.
func TestTargetRefusedPreClaim(t *testing.T) {
	for _, c := range []struct {
		name  string
		optIn bool
		args  func(root string) []string
	}{
		{"worker kit", true, func(root string) []string {
			return []string{"example-stream/07", "--root", root, "--repo", allowedRepo, "--target-change", "12", "--target-head", tgHead}
		}},
		{"change only", true, func(root string) []string { return tgArgs(t, root, "--target-change", "12") }},
		{"head only", true, func(root string) []string { return tgArgs(t, root, "--target-head", tgHead) }},
		{"short head", true, func(root string) []string {
			return tgArgs(t, root, "--target-change", "12", "--target-head", tgHead[:12])
		}},
		{"ref name head", true, func(root string) []string {
			return tgArgs(t, root, "--target-change", "12", "--target-head", "refs/pull/12/head")
		}},
		{"cross repo", true, func(root string) []string {
			return tgArgs(t, root, "--target-change", "12", "--target-head", tgHead, "--cross-repo")
		}},
		{"opted out", false, func(root string) []string { return tgArgs(t, root, "--target-change", "12", "--target-head", tgHead) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			t.Setenv("DESK_LOOP", "verify-desk")
			if c.optIn {
				tgOptIn(t, home)
			}
			s.replies = happyReplies("/private/tmp/verifier-home")
			if rc := run(c.args(root)); rc != deskkit.ExitRefused {
				t.Fatalf("%s: rc = %d, want %d", c.name, rc, deskkit.ExitRefused)
			}
			if len(s.calls) != 0 {
				t.Fatalf("%s refused only after child processes ran: %v", c.name, s.calls)
			}
		})
	}
}

// regression: #7623
// TestTargetHeadNotFetched: an opted-in target whose head is not in the
// checkout is could-not-check before the claim; deskdispatch never fetches.
func TestTargetHeadNotFetched(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	t.Setenv("DESK_LOOP", "verify-desk")
	tgOptIn(t, home)
	s.replies = append([]reply{{match: "cat-file -e", code: 1}}, happyReplies("/private/tmp/verifier-home")...)
	if rc := run(tgArgs(t, root, "--target-change", "12", "--target-head", tgHead)); rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	for _, c := range s.calls {
		if j := strings.Join(c, " "); !strings.Contains(j, "cat-file -e") {
			t.Fatalf("a missing head ran %q", j)
		}
	}
}

// regression: #7623
// TestTargetCutsAtHead: an opted-in target cuts the detached verifier home
// at exactly the named head, and the prompt says the run is pre-merge.
func TestTargetCutsAtHead(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	t.Setenv("DESK_LOOP", "verify-desk")
	tgOptIn(t, home)
	s.replies = happyReplies("/private/tmp/verifier-home")
	prompt := filepath.Join(t.TempDir(), "p.md")
	args := []string{"example-stream/07", "--root", root, "--repo", allowedRepo, "--kit", "verifier",
		"--model", "gpt-6-astra", "--brief", "spec.md", "--prompt-file", prompt, "--target-change", "12", "--target-head", tgHead}
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d", rc)
	}
	if !s.ran("--detach --base " + tgHead) {
		t.Fatalf("the verifier home was not cut at the target head: %v", s.deskwtCalls())
	}
	for _, c := range s.deskwtCalls() {
		if strings.Contains(c, "origin/main") {
			t.Fatalf("a pre-merge home was cut off origin/main: %s", c)
		}
	}
	p, err := os.ReadFile(emittedPathFor(prompt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(p), "UNMERGED head") || !strings.Contains(string(p), "never lands Evidence") {
		t.Fatal("the verifier prompt does not say the run is pre-merge")
	}
}

// regression: #7623
// TestTargetHelpStatesGuards: both use-point modes and the dispatch help name
// the target and its guards.
func TestTargetHelpStatesGuards(t *testing.T) {
	for _, mode := range []string{"", "--check-verifier", "--attest-verifier"} {
		long := newDispatchCLI(mode, dispatchCLI{}).Long
		for _, want := range []string{"--target-head", "ASSAY_VERIFIER_PREMERGE_REPOS", "default off", "APPROVED", "moved head", "never lands Evidence"} {
			if !strings.Contains(long, want) {
				t.Errorf("mode %q help lacks %q", mode, want)
			}
		}
	}
}
