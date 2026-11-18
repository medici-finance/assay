package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A dispatch only PREPARES the run: claim, worktree, prompt. Launching the agent is the
// caller's separate step, and a caller that never launches leaves the claim held with no
// progress. The closing line on stderr says so, names the grace window, and spells both
// ways out: launch then `progress`, or `release`.
// regression: #2747
func TestDispatchEndsWithANotLaunchedLine(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		name := "verbose"
		if quiet {
			name = "quiet"
		}
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			home := filepath.Join(t.TempDir(), "worker-home")
			s.replies = happyReplies(home)

			args := []string{"example-stream--07", "--root", root, "--kit", "worker", "--tier", "strong",
				"--prompt-file", filepath.Join(t.TempDir(), "prompt.md")}
			if quiet {
				args = append(args, "--quiet") // --quiet drops the per-step OK lines, never a warning
			}
			rc, stderr := runCapturingStderr(t, args)
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0\n%s", rc, stderr)
			}
			var line string
			for _, l := range strings.Split(stderr, "\n") {
				if strings.Contains(l, "NOT LAUNCHED") {
					line = l
				}
			}
			if line == "" {
				t.Fatalf("stderr carries no NOT LAUNCHED line:\n%s", stderr)
			}
			if last := strings.TrimSpace(stderr[strings.LastIndex(strings.TrimRight(stderr, "\n"), "\n")+1:]); !strings.Contains(last, "NOT LAUNCHED") {
				t.Errorf("the NOT LAUNCHED line is not the last thing printed: %q", last)
			}
			for _, want := range []string{
				"example-stream--07", // names the item
				home,                 // names where the agent is to run
				"CLAIMED_TTL_MIN",    // the configurable window
				"progress",           // the launch-side step
				"release",            // the give-up step
				"--repo",             // the commands are complete, not stubs
			} {
				if !strings.Contains(line, want) {
					t.Errorf("NOT LAUNCHED line does not name %q:\n%s", want, line)
				}
			}
			if s.ran("dispatch-claim.sh release") || s.ran("dispatch-claim.sh progress") {
				t.Errorf("the dispatch itself must leave the claim as it stands: %v", s.calls)
			}
		})
	}
}

// A refused dispatch prepared nothing, so it must not claim to have left something unlaunched.
// regression: #2747
func TestRefusedDispatchPrintsNoNotLaunchedLine(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: "refused: no", code: deskkit.ExitRefused},
	}
	rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "prompt.md")})
	if rc == deskkit.ExitOK {
		t.Fatalf("rc = 0, want a refusal")
	}
	if strings.Contains(stderr, "NOT LAUNCHED") {
		t.Errorf("a refused dispatch printed a NOT LAUNCHED line:\n%s", stderr)
	}
}

// A run with no branch (a detached review/verify home) prints `--branch -`. The claim tool
// accepts that vector as printed (TestPrintedBranchlessProgressRuns in cmd/deskclaim-ref runs
// `progress <key> --repo R --branch -`); this pins the printed side of the contract.
// regression: #6896
func TestNotLaunchedBranchlessProgressShape(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	dispatchOpts{item: "stream--b-08"}.notLaunched(dispatchPlan{claimKey: "assay-console--stream--b-08", claimReleaseHint: goClaimBinary}, "medici-finance/assay", "/home/x")
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	want := `progress "assay-console--stream--b-08" --repo medici-finance/assay --branch - --token-file`
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("printed progress command lost its branchless shape %q:\n%s", want, buf.String())
	}
}
