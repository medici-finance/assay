package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type stub struct {
	calls   [][]string
	replies []reply
}

type reply struct {
	match  string // matched against the joined argv
	stdout string
	code   int // non-zero exits with this code, so the wrapped scripts' 5/6 contract is exercised
}

// install wires the recording seam and returns (home, root). root is a REAL directory
// containing the consumer scripts this verb wraps, because their PRESENCE is a
// precondition the verb checks on disk.
func (s *stub) install(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	root = t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskdispatch-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskdispatch-test")

	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		for _, r := range s.replies {
			if strings.Contains(joined, r.match) {
				if r.code != 0 {
					return exec.Command("/bin/sh", "-c",
						"echo stub-failure 1>&2; exit "+itoa(r.code))
				}
				return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+r.stdout+"\nSTUBEOF")
			}
		}
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })
	return home, root
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func (s *stub) ran(fragment string) bool {
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), fragment) {
			return true
		}
	}
	return false
}

// plantScripts creates the consumer scripts the verb wraps. They are EMPTY files: this
// verb invokes them through the recording seam and never reads their content, which is
// exactly the wrap-don't-copy boundary under test.
func plantScripts(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{claimScriptRel, decisionScriptRel} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

// allowedRepo is a repo the fixture roster admits. Tests that must keep the executed-process
// count at exactly zero pass it explicitly, so the repo is never resolved by shelling out.
const allowedRepo = "medici-finance/assay"

func happyReplies(worktree string) []reply {
	return []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stdout: worktree},
	}
}

func TestDispatchClaimsFirstThenBuildsTheWorktreeAndEmitsThePrompt(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc := run([]string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if !s.ran("dispatch-claim.sh acquire example-stream--07") {
		t.Error("the durable claim was not acquired — nothing may precede it")
	}
	if !s.ran("deskwt add") {
		t.Error("no worktree was created for the agent")
	}
	// Order: the CLAIM is first. Everything after it is work a second dispatcher must not
	// also be doing.
	claimIdx, wtIdx := -1, -1
	for i, c := range s.calls {
		j := strings.Join(c, " ")
		if claimIdx < 0 && strings.Contains(j, "dispatch-claim.sh acquire") {
			claimIdx = i
		}
		if wtIdx < 0 && strings.Contains(j, "deskwt add") {
			wtIdx = i
		}
	}
	if claimIdx < 0 || wtIdx < 0 || claimIdx > wtIdx {
		t.Errorf("claim at %d, worktree at %d — the claim must come first", claimIdx, wtIdx)
	}

	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt file: %v", err)
	}
	prompt := string(body)
	for _, want := range []string{
		"medici-finance/assay",                    // the target repo, stated
		"/private/tmp/worker-home",                // the home deskwt reported
		"every file operation stays under it",     // the isolation floor, verbatim
		"NEVER re-attempt the same effect",        // the no-evasion clause
		"security-gate removal",                   // the security-gate refusal
		"KUBECONFIG=/dev/null",                    // the offline envelope
		`mktemp "${TMPDIR:-/tmp}/pr-body.XXXXXX"`, // the body-file rule
		"this item requires a strong implementer", // the strong-tier pickup STOP
		"It is not your writable root",            // the checkout base is not writable
		root,                                      // the checkout base, stated ABSOLUTE
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the emitted prompt is missing %q — a dispatched agent without this clause is the "+
				"failure the kit exists to prevent", want)
		}
	}
}

// The tier STOP is emitted ONLY for a strong-tier item: an unconditional line would turn
// every dispatch into a negotiation.
func TestTierStopIsStrongTierOnly(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"item-1", "--root", root, "--tier", "any", "--prompt-file", promptFile}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d", rc)
	}
	body, _ := os.ReadFile(promptFile)
	if strings.Contains(string(body), "requires a strong implementer") {
		t.Error("an `any`-tier dispatch carried the strong-tier pickup STOP")
	}
}

// CONTENTION: the claim tool's exit 5 means a LIVE holder owns the item. deskdispatch
// exits 5, prints who holds it, creates nothing, and NEVER steals.
func TestClaimContentionNamesTheHolderAndNeverSteals(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitRefused},
		{match: "dispatch-claim.sh show", stdout: "state=dispatched owner=other-session branch=feat/x"},
	}

	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"item-1", "--root", root, "--prompt-file", promptFile}); rc != deskkit.ExitRefused {
		t.Fatalf("contention rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if !s.ran("dispatch-claim.sh show") {
		t.Error("the existing holder was never read — a refusal that does not name the holder sends a " +
			"human to find out by hand")
	}
	if s.ran("steal") {
		t.Fatal("deskdispatch invoked a steal on contention — breaking a live claim is never inline")
	}
	if s.ran("deskwt add") {
		t.Error("a worktree was created despite the refused claim")
	}
	if _, err := os.Stat(promptFile); err == nil {
		t.Error("a prompt was emitted despite the refused claim — the agent would have been launched")
	}
}

// An UNREADABLE claim is exit 6, never "assume free". This is the fail-closed direction.
func TestUnreadableClaimIsUnverifiableNotFree(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitUnverifiable},
	}
	if rc := run([]string{"item-1", "--root", root}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable claim rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("deskwt add") {
		t.Error("work proceeded on a claim whose state could not be established")
	}
}

// THE CONFLATION DEFECT. --root used to drive BOTH where the claim script was looked up
// AND which checkout the worker's worktree branches from. Once the consumer scripts were
// centralized out of the consumer repos, those two answers diverged: pointing --root at
// the target repo failed claim-acquire (no script there, exit 6), and pointing --root at
// the checkout that carries the script branched the worker's worktree from the WRONG
// repo. --claim-root separates them; this test is the claim-root ≠ worktree-root case
// that would have caught the conflation.
func TestClaimRootDecouplesTheClaimToolFromTheWorktreeSource(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)  // the ITEM's own repo checkout: carries NO consumer scripts
	claimRoot := t.TempDir() // the central checkout that DOES carry them
	plantScripts(t, claimRoot)
	s.replies = happyReplies("/private/tmp/worker-home")

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc := run([]string{"cross--stream--01", "--root", root, "--repo", allowedRepo,
		"--claim-root", claimRoot, "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("cross-repo dispatch rc = %d, want 0 — the exact shape the conflation broke", rc)
	}

	// The claim tool that ran is the CENTRAL one, and the key/repo still reach it intact.
	wantScript := filepath.Join(claimRoot, filepath.FromSlash(claimScriptRel))
	claimed := false
	for _, c := range s.calls {
		if c[0] == wantScript && len(c) > 2 && c[1] == "acquire" {
			claimed = true
			if c[2] != "cross--stream--01" {
				t.Errorf("claim key %q reached the central tool reshaped", c[2])
			}
		}
		if strings.Contains(strings.Join(c, " "), filepath.Join(root, "tools")) {
			t.Errorf("a consumer script was invoked from --root %v — the item's repo does not carry it", c)
		}
	}
	if !claimed {
		t.Fatalf("the central claim script %s was never invoked; calls: %v", wantScript, s.calls)
	}
	if !s.ran("deskwt add") {
		t.Fatal("no worktree was created for the agent")
	}

	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt file: %v", err)
	}
	prompt := string(body)
	// The worktree source the agent is told about is the ITEM's repo, never the checkout
	// that merely carries the claim tool — an agent branched from the tool checkout
	// recreates the work in a repo that never asked for it.
	if !strings.Contains(prompt, "**Checkout base:** `"+root+"`") {
		t.Errorf("the prompt's checkout base is not the item's own repo root %s", root)
	}
	if strings.Contains(prompt, "**Checkout base:** `"+claimRoot+"`") {
		t.Error("the prompt names the claim-tool checkout as the base — the exact wrong-repo dispatch " +
			"the decoupling exists to prevent")
	}
	// The release instruction must name the claim tool where it actually is: the agent's
	// worktree does not carry it.
	if !strings.Contains(prompt, wantScript+" release") {
		t.Errorf("the prompt's release command does not name the central claim script %s", wantScript)
	}
}

// An explicit --claim-root is AUTHORITATIVE: scripts present in --root do not rescue a
// claim-root that lacks them. A silent fall-back would turn a mispointed flag into a
// dispatch that only looked configured.
func TestClaimRootWithoutTheScriptFailsClosedEvenWhenRootHasIt(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	empty := t.TempDir()
	s.replies = happyReplies("/private/tmp/worker-home")

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--claim-root", empty})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("scriptless --claim-root rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// No claim script means no durable claim can be taken. A machine-local lock would
// serialise two dispatchers on ONE machine and nothing at all across two — which is the
// case that double-dispatches — so this fails closed rather than degrading.
func TestMissingClaimScriptFailsClosed(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	s.replies = happyReplies("/private/tmp/worker-home")

	if rc := run([]string{"item-1", "--root", root}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing claim script rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("deskwt add") {
		t.Error("a worktree was created with no claim held")
	}
}

// A human-gated item must have its decision issue ensured BEFORE the agent starts.
func TestHumanGatedItemEnsuresTheDecisionIssue(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = append(happyReplies("/private/tmp/worker-home"),
		reply{match: "decision-issue.sh ensure", stdout: "created: decision issue #12"})

	rc := run([]string{"item-1", "--root", root, "--gate-human", "--brief", "spec.md",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !s.ran("decision-issue.sh ensure spec.md") {
		t.Error("the human-decision gate was not ensured for a human-gated item")
	}
}

// --gate-human with no specification is refused: the decision issue's content is DERIVED
// from the item's own spec and is never invented by the dispatcher.
func TestHumanGatedItemWithoutASpecIsRefused(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	if rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human"}); rc != deskkit.ExitRefused {
		t.Fatalf("gate-human without --brief rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	// The exit code alone is NOT the property. Asserting only on it is what let this
	// refusal sit downstream of the claim: it exited 5 with the item wedged behind a claim
	// nobody would release and a worktree leaked, and still read as a pass.
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came AFTER %d child process(es) ran: %v — a caller-flag mistake must cost "+
			"nothing durable", len(s.calls), s.calls)
	}
}

// The model stamp is validated at DISPATCH time, not at label time — a malformed slug
// discovered after the agent ran is discovered too late.
func TestMalformedModelSlugIsRefusedBeforeAnythingIsClaimed(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--model", "Not A Slug",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitRefused {
		t.Fatalf("malformed model slug rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	// The name of this test claims "before anything is claimed", so that is what it must
	// assert. Checking only the exit code let the stamp validation live at step 5, where a
	// refusal already had a held claim and a built worktree behind it.
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came AFTER %d child process(es) ran: %v — nothing may be claimed or created "+
			"before the caller's own flags are known good", len(s.calls), s.calls)
	}
}

// TestNoCallerPreconditionIsCheckedAfterTheClaim is the CLASS-CLOSER for the wedged-item
// defect, and it is deliberately a table rather than one more single case.
//
// THE DEFECT IT CLOSES. Two caller-flag validations — the model slug and the
// --gate-human/--brief pairing — lived inside the steps that consumed them, at positions 5
// and 4. Both refused with exit 5, which looked correct, but by then step 1 had taken the
// DURABLE cross-machine claim and step 2 had created a worktree, and neither was released.
// The item was then permanently wedged: every retry, including the operator's corrected
// re-run, was told "already claimed by a LIVE holder" until a human hand-deleted the ref.
// A mistyped flag cost an item nobody could pick up.
//
// WHY A TABLE. The two found instances were not special; they were whatever happened to be
// checked late. The property that actually matters is universal — no caller-controlled
// precondition may be decided after durable state exists — so the test enumerates every
// such input and asserts the same thing about each: the process count is ZERO. A new flag
// validated in the wrong place fails here on the day it is added, rather than on the day it
// wedges an item.
//
// Each case passes --repo so the repo is not resolved by shelling out to git: that read is
// harmless, but keeping the expected count at exactly zero makes the assertion exact
// instead of "zero except the ones we decided were fine".
func TestNoCallerPreconditionIsCheckedAfterTheClaim(t *testing.T) {
	cases := []struct {
		why  string
		args []string
	}{
		{"an item key that would be read as a flag", []string{"--force"}},
		{"an item key with a parent-directory segment", []string{"../escape"}},
		{"a tier outside the closed vocabulary", []string{"item-1", "--tier", "medium"}},
		{"a kit this binary does not carry", []string{"item-1", "--kit", "nonesuch"}},
		{"a malformed model slug", []string{"item-1", "--model", "Not A Slug"}},
		{"a model slug that is empty after trimming", []string{"item-1", "--model", "   x  y  "}},
		{"--gate-human with no specification", []string{"item-1", "--gate-human"}},
		{"a branch name the worktree verb would refuse", []string{"item-1", "--branch", "-dash-leading"}},
		{"a branch name containing a parent-directory segment", []string{"item-1", "--branch", "feat/../x"}},
		{"--prompt-file under a directory that does not exist", []string{"item-1", "--prompt-file", "/nonexistent-dir-for-test/p.md"}},
		{"--claim-root naming a directory that does not exist", []string{"item-1", "--claim-root", "/nonexistent-claim-root-for-test"}},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home")

			args := append([]string{}, c.args...)
			args = append(args, "--root", root, "--repo", allowedRepo)
			rc := run(args)
			if rc == deskkit.ExitOK {
				t.Fatalf("%s was ACCEPTED (rc=0) — it must be refused", c.why)
			}
			if len(s.calls) != 0 {
				t.Fatalf("%s was rejected only AFTER %d child process(es) ran: %v — by then the claim is "+
					"held and the worktree exists, so the item is wedged and the tree leaked",
					c.why, len(s.calls), s.calls)
			}
		})
	}
}

// A human-gated item whose decision script is missing must also refuse before the claim:
// the gate's precondition is knowable from disk, so paying for it with a wedged item is
// the same defect wearing different clothes.
func TestMissingDecisionScriptRefusesBeforeTheClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	// Plant ONLY the claim script, so the decision script is the single thing missing.
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(claimScriptRel)), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	s.replies = happyReplies("/private/tmp/worker-home")

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", "spec.md"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing decision script rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// With no --pr there is no PR to label. The step must SAY so and emit the labels for the
// moment the draft PR opens — never invent a PR number, and never go silent.
func TestStampWithoutAPRIsPendingNotSilent(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	if rc := run([]string{"item-1", "--root", root, "--model", "example-model-1",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d", rc)
	}
	if s.ran("pr edit") {
		t.Fatal("deskdispatch edited a PR without one being named")
	}
}

// An unknown tier must not be recorded as though it meant something.
func TestUnknownTierRefused(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	if rc := run([]string{"item-1", "--root", root, "--tier", "medium"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown tier rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if len(s.calls) != 0 {
		t.Error("an unknown tier ran a child process before refusing")
	}
}

// The tier vocabulary is DERIVED, never a second hand-written list.
func TestTierVocabularyIsTheDeclaredSet(t *testing.T) {
	for _, tier := range deskkit.DispatchTiers() {
		if !validTier(tier) {
			t.Errorf("tier %q is in the declared dispatch-tier set but deskdispatch rejects it", tier)
		}
	}
	if validTier("cheap") {
		t.Error("deskdispatch accepts a tier outside the declared set")
	}
}

// An unknown kit must refuse, not silently produce a prompt with no clauses in it — that
// would LOOK like a successful dispatch.
func TestUnknownKitRefusesRatherThanEmittingAnEmptyPrompt(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	if rc := run([]string{"item-1", "--root", root, "--kit", "nonesuch"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown kit rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if len(s.calls) != 0 {
		t.Error("an unknown kit ran a child process before refusing")
	}
}

// Every advertised kit must actually be embedded and non-trivial.
func TestEveryAdvertisedKitIsEmbedded(t *testing.T) {
	names := kitNames()
	if len(names) != 3 {
		t.Fatalf("kits = %v, want exactly the three dispatched-agent classes", names)
	}
	for _, n := range names {
		text, err := kitText(n)
		if err != nil {
			t.Fatalf("kit %q: %v", n, err)
		}
		if len(text) < 500 {
			t.Errorf("kit %q is %d bytes — too short to be carrying the clauses it advertises", n, len(text))
		}
	}
	common, err := commonKitText()
	if err != nil {
		t.Fatalf("common kit: %v", err)
	}
	if len(common) < 500 {
		t.Errorf("the common kit is %d bytes — too short to carry the isolation floor", len(common))
	}
}

// The common kit is NOT selectable as a class: a dispatch that emitted only the common
// clauses would be one with no class instructions at all.
func TestCommonKitIsNotSelectableAsAClass(t *testing.T) {
	if _, err := kitText("common"); err == nil {
		t.Fatal("--kit common was accepted — the common clauses are not an agent class")
	}
}

// EVERY class gets the common clauses. This is the assertion that stops a class kit's
// "see the common kit" pointer from being a promise nothing keeps.
func TestEveryClassPromptCarriesTheCommonClauses(t *testing.T) {
	for _, kit := range kitNames() {
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		s.replies = happyReplies("/private/tmp/agent-home")

		promptFile := filepath.Join(t.TempDir(), kit+".md")
		if rc := run([]string{"item-1", "--root", root, "--kit", kit, "--prompt-file", promptFile}); rc != deskkit.ExitOK {
			t.Fatalf("kit %q rc = %d, want 0", kit, rc)
		}
		body, err := os.ReadFile(promptFile)
		if err != nil {
			t.Fatalf("kit %q prompt: %v", kit, err)
		}
		for _, want := range []string{
			"every file operation stays under it", // C1, the isolation floor
			"NEVER re-attempt the same effect",    // C2, no-evasion
			"KUBECONFIG=/dev/null",                // C3, offline envelope
			"could-not-check",                     // C4, three-state instruments
		} {
			if !strings.Contains(string(body), want) {
				t.Errorf("the %q prompt is missing the common clause %q", kit, want)
			}
		}
	}
}

// A repo outside the desk set is not somewhere this verb dispatches work.
func TestForeignRepoRefused(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	if rc := run([]string{"item-1", "--root", root, "--repo", "someone-else/thing"}); rc != deskkit.ExitRefused {
		t.Fatalf("foreign repo rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if s.ran("dispatch-claim.sh") {
		t.Error("a claim was attempted in a repo outside the desk set")
	}
}

// The key is passed through UNCHANGED: a key this verb reshaped would not collide with the
// one another desk holds, and a claim that does not collide is not a claim.
func TestItemKeyReachesTheClaimToolUnchanged(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	const key = "example--stream--07"
	_ = run([]string{key, "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "dispatch-claim.sh acquire") {
			if c[2] != key {
				t.Fatalf("the claim tool received key %q, not %q", c[2], key)
			}
			return
		}
	}
	t.Fatal("the claim tool was never invoked")
}

// A key that would be read as a flag, or that carries a parent-directory segment, is
// refused before it reaches any child process.
func TestHostileItemKeysRefused(t *testing.T) {
	for _, key := range []string{"--force", "../escape", "a b", ""} {
		s := &stub{}
		_, root := s.install(t)
		plantScripts(t, root)
		rc := run([]string{key, "--root", root})
		if rc != deskkit.ExitRefused {
			t.Errorf("key %q rc = %d, want %d (refused)", key, rc, deskkit.ExitRefused)
		}
		if len(s.calls) != 0 {
			t.Errorf("key %q reached a child process: %v", key, s.calls)
		}
	}
}

// A deskwt that exits 0 but names no path leaves the agent with no home — the isolation
// floor every other clause rests on.
func TestWorktreeWithNoReportedPathIsUnverifiable(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stdout: ""},
	}
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"item-1", "--root", root, "--prompt-file", promptFile}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("pathless worktree rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if _, err := os.Stat(promptFile); err == nil {
		t.Error("a prompt naming no home worktree was emitted")
	}
}

func TestDryRunTouchesNothingButStillEmitsThePrompt(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"item-1", "--root", root, "--repo", "medici-finance/assay",
		"--dry-run", "--prompt-file", promptFile}); rc != deskkit.ExitOK {
		t.Fatalf("dry-run rc = %d, want 0", rc)
	}
	if len(s.calls) != 0 {
		t.Fatalf("--dry-run ran %d child processes: %v", len(s.calls), s.calls)
	}
	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	// The dry-run prompt must not print a PREDICTED worktree path an operator could paste
	// into a real dispatch: deskwt owns where a worktree lands.
	if !strings.Contains(string(body), homeUnknown) {
		t.Error("the dry-run prompt did not say the worktree path is not yet known")
	}
}

func TestStepListIsTheDocumentedContract(t *testing.T) {
	want := []string{"claim-acquire", "worktree-create", "roster-register", "decision-gate",
		"model-stamp", "prompt-emit"}
	if len(dispatchSteps) != len(want) {
		t.Fatalf("dispatchSteps has %d entries, want %d", len(dispatchSteps), len(want))
	}
	for i := range want {
		if dispatchSteps[i] != want[i] {
			t.Errorf("dispatchSteps[%d] = %q, want %q", i, dispatchSteps[i], want[i])
		}
		if !strings.Contains(usage, want[i]) {
			t.Errorf("step %q is not documented in --help", want[i])
		}
	}
}

func TestHelpNamesTheEngineSeam(t *testing.T) {
	if !strings.Contains(usage, "engine seam: DISPATCH") {
		t.Error("deskdispatch --help does not name its engine seam")
	}
}

func TestVersionKitsAndHelpAreUnguardedReads(t *testing.T) {
	s := &stub{}
	s.install(t)
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
	if rc := run([]string{"--kits"}); rc != deskkit.ExitOK {
		t.Fatalf("--kits rc = %d, want 0", rc)
	}
	if rc := run([]string{"help"}); rc != deskkit.ExitOK {
		t.Fatalf("help rc = %d, want 0", rc)
	}
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no-args rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestKillSwitchIsHonoured(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc := run([]string{"item-1", "--root", root}); rc != deskkit.ExitDisabled {
		t.Fatalf("disabled rc = %d, want %d", rc, deskkit.ExitDisabled)
	}
	if len(s.calls) != 0 {
		t.Error("a disabled deskdispatch still ran a child process")
	}
}
