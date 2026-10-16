package main

// kittoolparity_test.go — the kit text and the tools it names must agree.
//
// THE DEFECT CLASS. A dispatched agent follows its prompt as written and treats a refusal as a
// stop. Each place where the printed text and the tool disagree costs it a detour, and some
// read like a guard refusal when they are not: a claim line printed without the credential
// the claim binary needs exits 6 as printed; a packet clause that demands one read cannot be
// met by a packet larger than one read; a one-step verb the kit never names leaves the agent
// running the failing two-step path; and an obligation stated only in the desk's skill never
// reaches the reviewer, who reads only the kit.
//
// WHAT THESE PIN. Every printed claim command the dispatch emits for the pure-Go claim binary
// — the agent's release line, the NOT LAUNCHED progress/release hint, the release-by-hand
// hint, the steal hint — carries --token-file; the legacy script's lines carry none. The
// packet clause allows ranges in all three kits that carry it. The review assignment names
// `deskpost finish` for the lane its claim key names. The review kit states the protected-path
// obligation and how to read deskpathguard's and verifyrun's results.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// credFor is the credential argument a printed Go-binary claim command must carry.
func credFor(role string) string {
	return `--token-file "$(desktoken --no-rotate ` + role + ` --repo ` + allowedRepo + `)"`
}

// lineWith returns the first line of s containing sub, "" when none does.
func lineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

// The agent's release line, for every kit, carries the credential the Go binary needs, under
// the agent's own role.
// regression: #2493
func TestAgentReleaseLineHasCred(t *testing.T) {
	for _, tc := range []struct{ kit, item, role string }{
		{"worker", "example-stream--07", "worker"},
		{"review", "assay--pr-547", "reviewer"},
		{"verifier", "example-stream--07", "verifier"},
	} {
		t.Run(tc.kit, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			withGoClaimOnPath(t) // after install, which resets the PATH seam
			s.replies = []reply{{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"}}
			promptFile := filepath.Join(t.TempDir(), "prompt.md")
			args := []string{tc.item, "--root", root, "--repo", allowedRepo, "--kit", tc.kit,
				"--dry-run", "--prompt-file", promptFile}
			switch tc.kit {
			case "review":
				args = append(args, "--pr", "547")
			case "verifier":
				args = append(args, "--model", "gpt-6-astra", "--brief", "spec.md")
			}
			if rc := run(args); rc != deskkit.ExitOK {
				t.Fatalf("%s dispatch rc = %d, want 0", tc.kit, rc)
			}
			body, err := os.ReadFile(emittedPathFor(promptFile))
			if err != nil {
				t.Fatal(err)
			}
			prompt := string(body)
			line := lineWith(prompt, goClaimBinary+" release")
			if line == "" {
				t.Fatalf("no %s release line in the %s prompt", goClaimBinary, tc.kit)
			}
			if !strings.Contains(line, credFor(tc.role)) {
				t.Errorf("the %s release line fails as printed — no credential:\n%s\nwant it to carry %s",
					tc.kit, line, credFor(tc.role))
			}
		})
	}
}

// The legacy script reads the ambient credential and takes no --token-file: its release line
// must stay as it was.
// regression: #2493
func TestScriptReleaseLineNoCred(t *testing.T) {
	prompt := dispatchPrompt(t, "worker", "example-stream--07")
	line := lineWith(prompt, claimScriptRel+" release")
	if line == "" {
		t.Fatal("no script release line in the worker prompt")
	}
	if strings.Contains(line, "--token-file") {
		t.Errorf("the legacy script's release line carries a flag the script does not take:\n%s", line)
	}
}

// The dispatcher's NOT LAUNCHED line spells both the progress and the release command under
// the dispatcher's role; with the Go binary both need the credential.
// regression: #2493
func TestNotLaunchedHintHasCred(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withGoClaimOnPath(t)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0\n%s", rc, stderr)
	}
	line := lineWith(stderr, "NOT LAUNCHED")
	if got := strings.Count(line, credFor(deskkit.DispatcherRole)); got != 2 {
		t.Errorf("NOT LAUNCHED line carries the credential %d time(s), want 2 (progress and release):\n%s", got, line)
	}
}

// A release that fails after the claim prints a by-hand command; it must run as printed.
// regression: #2493
func TestByHandReleaseHasCred(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root) // the decision script the gate needs
	withGoClaimOnPath(t)  // the claim itself goes through the Go binary
	s.replies = append([]reply{
		{match: "decision-issue.sh ensure", stderr: "refused: no decision surface", code: deskkit.ExitRefused},
		{match: goClaimBinary + " release", stderr: "no forge token", code: deskkit.ExitUnverifiable},
	}, happyReplies(filepath.Join(t.TempDir(), "home"))...)
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human",
		"--brief", "spec.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc == deskkit.ExitOK {
		t.Fatal("a failed decision gate must fail the dispatch")
	}
	line := lineWith(stderr, "release it by hand")
	if line == "" {
		t.Fatalf("no release-by-hand hint after a failed release:\n%s", stderr)
	}
	if !strings.Contains(line, credFor(deskkit.DispatcherRole)) {
		t.Errorf("the release-by-hand hint fails as printed — no credential:\n%s", line)
	}
}

// A stale holder's refusal names the steal command; it must run as printed.
// regression: #2493
func TestStealHintHasCred(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withGoClaimOnPath(t)
	s.replies = append([]reply{
		{match: goClaimBinary + " acquire", stderr: "held", code: deskkit.ExitRefused},
		{match: goClaimBinary + " show", stdout: "HELD example-stream--07 state=claimed age=999m"},
	}, happyReplies(filepath.Join(t.TempDir(), "home"))...)
	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo})
	if err == nil {
		t.Fatal("a held claim must refuse the dispatch")
	}
	if !strings.Contains(err.Error(), "STALE") {
		t.Fatalf("precondition: want the stale-holder refusal, got: %v", err)
	}
	if !strings.Contains(err.Error(), "steal example-stream--07 --repo "+allowedRepo+" --reason <why> "+
		credFor(deskkit.DispatcherRole)) {
		t.Errorf("the steal hint fails as printed — no credential:\n%v", err)
	}
}

// A packet larger than one read can still be read first and whole: every kit that carries the
// packet clause allows ranges sent together, and none demands a single read.
// regression: #2493
func TestPacketReadAllowsRanges(t *testing.T) {
	for _, path := range []string{
		"references/review-prompt.md",
		"references/worker-prompt.md",
		"references/worker-prompt-objective.md",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		kit := foldSpace(string(b))
		if strings.Contains(kit, "**Read it first, whole, in one read**") {
			t.Errorf("%s still demands the packet in one read — unsatisfiable above one read's cap", path)
		}
		if !strings.Contains(kit, "a packet larger than one read returns is taken in ranges, all sent in one request") {
			t.Errorf("%s does not say how a packet larger than one read is read", path)
		}
	}
}

// The review assignment names the one-step post for the lane the claim key names, and only
// for a key `deskpost finish` accepts.
// regression: #2493
func TestReviewNamesFinish(t *testing.T) {
	for _, tc := range []struct{ item, want string }{
		{"assay--pr-547", "deskpost finish review " + allowedRepo + " 547 --verdict approve|request-changes"},
		{"assay--pr-547--security", "deskpost finish security-review " + allowedRepo + " 547 --verdict pass|fail"},
	} {
		asg := assignmentSection(t, dispatchPrompt(t, "review", tc.item, "--pr", "547"))
		if !strings.Contains(asg, tc.want) || !strings.Contains(asg, `--claim "`+tc.item+`"`) {
			t.Errorf("%s: the assignment does not name `%s … --claim %q`:\n%s", tc.item, tc.want, tc.item, asg)
		}
	}
	// No PR number: finish cannot accept the key, so only the release line is printed.
	if asg := assignmentSection(t, dispatchPrompt(t, "review", "review-item-9")); strings.Contains(asg, "deskpost finish") {
		t.Errorf("a key finish would refuse still gets a finish line:\n%s", asg)
	}
}

// The reviewer reads only the kit, so the kit carries the protected-path obligation and how to
// read the two instruments whose exit code is not their result — on every lane.
// regression: #2493
func TestReviewKitStatesToolResults(t *testing.T) {
	for _, lane := range []string{"", deskkit.LaneCorrectness.Name, deskkit.LaneSecurity.Name} {
		kit := foldSpace(reviewCut(t, lane))
		for _, want := range []string{
			"**A PR labelled `wrote-to-the-test` gets no APPROVE without the reason.**",
			"An APPROVE must name the protected path touched and say why the edit is legitimate",
			"its `state=` line is the result",
			"A non-zero exit with no `state=` line means the check did not run",
			"a label write that fails after the state line also exits non-zero and prints no `applied:` line",
			"`could-not-attribute` on stderr",
			"never by writing git config",
			"**Post, confirm and release in one step.**",
			"`verdict-time labels: applied <labels>`",
		} {
			if !strings.Contains(kit, want) {
				t.Errorf("lane %q: the review kit does not carry %q", lane, want)
			}
		}
	}
}
