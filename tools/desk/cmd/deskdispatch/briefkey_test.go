package main

// briefkey_test.go — #6173. A brief-shaped item key dispatched with no --brief never read the
// brief it names, so a `gate: human` brief printed "decision-gate SKIPPED: item is not
// human-gated" and dispatched with no decision surface. The key now names the brief for the
// human-gate decision: the gate fires, a recorded ruling is taken, the decision hold reads it, and
// a key whose brief does not resolve is refused before the claim.
//
// Nothing here reaches a forge: every child is answered by the harness's exec seam.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const skippedLine = "SKIPPED: item is not human-gated"

// boardlessKeyBrief is the harness's resolver: the real one, except that a key whose stream has no
// directory under any base reads as naming no brief — the fixture checkout carries no board.
func boardlessKeyBrief(o dispatchOpts) (string, string, error) {
	p, shown, err := briefFromItemKey(o)
	if err == nil {
		return p, shown, nil
	}
	stream, _, serr := deskkit.SplitBriefKey(briefIDFromItem(o.item))
	if serr != nil {
		return p, shown, err
	}
	for _, base := range []string{o.root, o.claimRoot} {
		if fi, ferr := os.Stat(filepath.Join(base, "docs", "streams", stream)); base != "" && ferr == nil && fi.IsDir() {
			return p, shown, err
		}
	}
	return "", "", nil
}

// strictKeyBrief restores the production resolver for one test.
func strictKeyBrief(t *testing.T) {
	t.Helper()
	old := keyBriefFn
	keyBriefFn = briefFromItemKey
	t.Cleanup(func() { keyBriefFn = old })
}

// bkPlant writes body at rel under root.
func bkPlant(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// bkGated is the fixture gate:human brief with no recorded ruling.
var bkGated = rrBrief("Option 1 or option 2?")

// bkRun dispatches key with no --brief and no --gate-human, returning rc, stderr and the prompt.
func bkRun(t *testing.T, root, key string, extra ...string) (int, string, string) {
	t.Helper()
	promptFile := filepath.Join(t.TempDir(), "p.md")
	args := append([]string{key, "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile}, extra...)
	rc, stderr := runCapturingStderr(t, args)
	body, _ := os.ReadFile(emittedPathFor(promptFile))
	return rc, stderr, string(body)
}

// regression: #6173
func TestBriefKeyGateFires(t *testing.T) {
	for name, key := range map[string]string{
		"claim-key": "assay--example--01",
		"slash-key": "example/01",
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			bkPlant(t, root, rrBriefRel, bkGated)
			s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
				reply{match: "decision-issue.sh ensure", stdout: "created: decision issue #7"})
			rc, stderr, prompt := bkRun(t, root, key)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
			}
			if !s.ran("decision-issue.sh ensure " + rrBriefRel) {
				t.Errorf("the gate:human brief the key names did not fire the decision gate: %v", s.calls)
			}
			if strings.Contains(stderr, skippedLine) {
				t.Errorf("a gate:human brief named by the key printed SKIPPED:\n%s", stderr)
			}
			if !strings.Contains(prompt, "Human-gated item") {
				t.Error("the prompt for a key-named gate:human brief omits the human-gated clause")
			}
		})
	}
}

// A brief found only under --claim-root (the tracking checkout) gates too, and the decision tool
// is handed its absolute path — the same file the detection read.
// regression: #6173
func TestBriefKeyClaimRootGates(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	claimRoot := t.TempDir()
	plantScripts(t, claimRoot)
	bkPlant(t, claimRoot, rrBriefRel, bkGated)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: "decision-issue.sh ensure", stdout: "created: decision issue #7"})
	rc, stderr, _ := bkRun(t, root, "assay--example--01", "--claim-root", claimRoot)
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
	}
	want := filepath.Join(claimRoot, filepath.FromSlash(rrBriefRel))
	if abs, err := filepath.EvalSymlinks(claimRoot); err == nil && !s.ran(want) {
		want = filepath.Join(abs, filepath.FromSlash(rrBriefRel))
	}
	if !s.ran("decision-issue.sh ensure " + want) {
		t.Errorf("the claim-root brief did not fire the gate by its absolute path %s: %v", want, s.calls)
	}
}

// regression: #6173
func TestBriefKeyRecordedRuling(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	rrTree(t, root, rrBrief(rrGoodLine))
	s.replies = append([]reply{{match: "--ruling-record", stdout: rrRecordRel + ": CORROBORATED — example"}},
		happyReplies(filepath.Join(t.TempDir(), "worker-home"))...)
	lookPath = func(name string) (string, error) {
		if name == statusgenBinary {
			return "/example/bin/statusgen", nil
		}
		return "", exec.ErrNotFound
	}
	rc, stderr, _ := bkRun(t, root, "assay--example--01")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
	}
	if !s.ran("--ruling-record " + rrRecordRel + " --ruling-brief " + rrBriefRel) {
		t.Fatalf("the key-named brief's recorded ruling was not corroborated: %v", s.calls)
	}
	if s.ran("decision-issue.sh") {
		t.Fatalf("a decision issue was ensured although the ruling is recorded: %v", s.calls)
	}
}

// The pre-claim decision hold reads the key-named brief too: an unruled open decision holds.
// regression: #6173
func TestBriefKeyDecisionHolds(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	bkPlant(t, root, "docs/streams/example-stream/brief-07-example.md",
		"---\nbrief: example-stream/07\ngate: human\ndecision-issue: 41\n---\n\n# Brief\n")
	useRatifier(t, true)
	useDecisionForge(t, openDecision(41, "", "someone-else"))
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || !strings.Contains(err.Error(), decisionHeldMarker) {
		t.Fatalf("the key-named brief's open decision did not hold the dispatch: %v", err)
	}
	if len(s.calls) != 0 {
		t.Fatalf("the hold came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// regression: #6173
func TestBriefKeyUnresolvedRefused(t *testing.T) {
	for name, files := range map[string][]string{
		"none":      nil,
		"ambiguous": {"docs/streams/example/brief-09-a.md", "docs/streams/example/brief-09-b.md"},
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			strictKeyBrief(t)
			plantScripts(t, root)
			for _, f := range files {
				bkPlant(t, root, f, bkGated)
			}
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			err := cmdDispatch([]string{"assay--example--09", "--root", root, "--repo", allowedRepo,
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want %d (refused): %v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
			}
			for _, want := range []string{"could-not-check", "assay--example--09", filepath.Join("docs", "streams", "example")} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q: %v", want, err)
				}
			}
			if len(s.calls) != 0 {
				t.Fatalf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
			}
		})
	}
}

// A key-named brief that does not gate on a human still SKIPS — and the SKIPPED line says the
// brief was read, so a skip can no longer mean "nothing was read".
// regression: #6173
func TestBriefKeyNonGatedSkips(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	bkPlant(t, root, rrBriefRel, "---\nbrief: example/01\ngate: model\n---\n\n# Example\n")
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr, _ := bkRun(t, root, "assay--example--01")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
	}
	if s.ran("decision-issue.sh") {
		t.Errorf("a gate:model brief fired the decision gate: %v", s.calls)
	}
	if !strings.Contains(stderr, skippedLine) || !strings.Contains(stderr, rrBriefRel) {
		t.Errorf("the SKIPPED line does not name the brief it read (%s):\n%s", rrBriefRel, stderr)
	}
}

// An issue-<N> item names no brief: it SKIPS exactly as before.
func TestIssueKeyStillSkips(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr, _ := bkRun(t, root, "assay--issue-77")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
	}
	if s.ran("decision-issue.sh") || !strings.Contains(stderr, stepDecisionGate+" "+skippedLine+"\n") {
		t.Errorf("an issue item did not SKIP the decision gate unchanged:\n%s", stderr)
	}
}

// An explicit --brief wins over the key, unchanged: its own (non-gating) metadata decides, and a
// key whose brief would not resolve is not consulted at all.
func TestExplicitBriefUnchanged(t *testing.T) {
	for name, key := range map[string]string{
		"key-names-gated": "assay--example--01",
		"key-unresolved":  "assay--example--09",
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			bkPlant(t, root, rrBriefRel, bkGated)
			bkPlant(t, root, "explicit.md", "---\ngate: model\n---\n\n# Explicit\n")
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			rc, stderr, _ := bkRun(t, root, key, "--brief", "explicit.md")
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0:\n%s", rc, stderr)
			}
			if s.ran("decision-issue.sh") || !strings.Contains(stderr, stepDecisionGate+" "+skippedLine+"\n") {
				t.Errorf("an explicit non-gating --brief did not decide the gate unchanged: %v\n%s", s.calls, stderr)
			}
		})
	}
}
