package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionport_test.go — issue 6131. The decision gate resolves the claim tool's way: the
// pure-Go port deskdecision on PATH first, then tools/decision-issue.sh under the resolved root,
// then a refusal naming both. The field failure: a repo that carries no decision script (every
// consumer repo, since no install path ships it) refused EVERY human-gated dispatch at
// decision-gate with exit 6, even with desk-tools installed.

// withPortsOnPath makes lookPath resolve exactly the named binaries, nothing else.
func withPortsOnPath(t *testing.T, names ...string) {
	t.Helper()
	old := lookPath
	lookPath = func(name string) (string, error) {
		for _, n := range names {
			if name == n {
				return "/opt/desk-tools/bin/" + name, nil
			}
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = old })
}

// decisionCalls returns every recorded child whose argv runs a decision helper, by bare port
// name or by script path.
func decisionCalls(s *stub) []string {
	var out []string
	for _, c := range s.calls {
		if c[0] == goDecisionBinary || strings.HasSuffix(c[0], filepath.FromSlash(decisionScriptRel)) {
			out = append(out, strings.Join(c, " "))
		}
	}
	return out
}

// portArgv is the exact argv the port must receive for a worker-kit dispatch of spec.md.
func portArgv() string {
	return goDecisionBinary + " ensure spec.md --repo " + allowedRepo + " --at start --as " + deskkit.DispatcherRole
}

// POSITIVE. The field shape: neither consumer script in the tree, desk-tools on PATH. The gate
// runs the port by bare name with the script's argv plus --as the dispatching role, and the
// dispatch completes.
//
// regression: #6131
func TestDecisionPortRunsWhenTheScriptIsAbsent(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // the target repo carries no script
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: goDecisionBinary + " ensure", stdout: "created: decision issue #4"})

	if err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md", "--quiet",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatalf("a human-gated dispatch with the port on PATH and no script failed: %v", err)
	}
	got := decisionCalls(s)
	if len(got) != 1 || got[0] != portArgv() {
		t.Fatalf("decision helper calls = %q, want exactly [%q]", got, portArgv())
	}
}

// The port is PREFERRED over a script in the tree, as the claim binary is: a tree that still
// carries the script runs the port, and never both (one gate, one filing).
//
// regression: #6131
func TestDecisionPortIsPreferredOverTheScript(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantOnly(t, root, decisionScriptRel)
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: goDecisionBinary + " ensure", stdout: "exists: decision issue #4"})

	if err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md", "--quiet",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := decisionCalls(s); len(got) != 1 || got[0] != portArgv() {
		t.Fatalf("decision helper calls = %q, want only the port [%q]", got, portArgv())
	}
}

// NEGATIVE. The port's verdict passes through fail-closed: a 6 from it stops the dispatch at
// decision-gate with the port's own words, and no prompt is emitted.
//
// regression: #6131
func TestDecisionPortFailureFailsClosed(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: goDecisionBinary + " ensure", code: deskkit.ExitUnverifiable,
			stderr: "deskdecision: could not check: existing-issue lookup failed"})
	prompt := filepath.Join(t.TempDir(), "p.md")

	err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md", "--quiet", "--prompt-file", prompt})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a failed port gate must fail closed (exit 6): %v", err)
	}
	for _, want := range []string{stepDecisionGate, "existing-issue lookup failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, err.Error())
		}
	}
	if _, serr := os.Stat(emittedPathFor(prompt)); serr == nil {
		t.Error("a prompt was emitted past a failed decision gate")
	}
}

// The port reads no credential from its environment, so stepDecision runs it even when nothing
// was handed over and no GH_TOKEN is exported — the shape that would refuse the script. The
// ambient-auth refusal still binds the script and the ruling corroboration.
//
// regression: #6131
func TestDecisionPortNeedsNoHandedCredential(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	t.Setenv("GH_TOKEN", "")
	s.replies = []reply{{match: goDecisionBinary + " ensure", stdout: "created: decision issue #4"}}
	o := dispatchOpts{root: root, brief: "spec.md", kit: "worker"}

	line, err := stepDecision(o, true, allowedRepo, goDecisionBinary, nil, claimAuth{})
	if err != nil {
		t.Fatalf("the port was refused for a missing handed credential it never reads: %v", err)
	}
	for _, want := range []string{"created: decision issue #4", "the " + deskkit.DispatcherRole + " role credential", goDecisionBinary} {
		if !strings.Contains(line, want) {
			t.Errorf("gate line %q does not name %q", line, want)
		}
	}
	if got := decisionCalls(s); len(got) != 1 || got[0] != portArgv() {
		t.Fatalf("decision helper calls = %q, want [%q]", got, portArgv())
	}

	s.calls = nil
	script := filepath.Join(root, filepath.FromSlash(decisionScriptRel))
	if _, err := stepDecision(o, true, allowedRepo, script, nil, claimAuth{}); err == nil ||
		deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("the script must still refuse on ambient auth (exit 6): %v", err)
	}
	if len(s.calls) != 0 {
		t.Fatalf("the script refusal came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// END TO END. The packet's field shape: a fresh worker dispatch with --attest-worker and a
// --brief whose frontmatter reads `gate: human`, in a tree with no consumer script. The brief's
// gate fires the port with --as the dispatching role BEFORE the attestation is made, and the
// bound prompt is emitted.
//
// regression: #6131
func TestAttestWorkerBriefGateRunsThePortEndToEnd(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	if err := os.WriteFile(filepath.Join(root, "spec.md"),
		[]byte("---\ngate: human\n---\n# example spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(t.TempDir()),
		reply{match: goDecisionBinary + " ensure", stdout: "created: decision issue #4"})
	var gateRanFirst []bool
	old := attestWorkerDispatchFn
	attestWorkerDispatchFn = func(o dispatchOpts, plan dispatchPlan, repo, home string) (deskkit.WorkerReceipt, error) {
		gateRanFirst = append(gateRanFirst, len(decisionCalls(s)) == 1)
		return workerReceiptFixture(o.model), nil
	}
	t.Cleanup(func() { attestWorkerDispatchFn = old })
	prompt := filepath.Join(t.TempDir(), "prompt.md")

	if rc := run(attestArgs(root, prompt)); rc != 0 {
		t.Fatalf("attested human-gated dispatch rc=%d; calls: %v", rc, s.calls)
	}
	if got := decisionCalls(s); len(got) != 1 || got[0] != portArgv() {
		t.Fatalf("decision helper calls = %q, want [%q]", got, portArgv())
	}
	if len(gateRanFirst) != 1 || !gateRanFirst[0] {
		t.Fatalf("the attestation was not made exactly once AFTER the decision gate: %v", gateRanFirst)
	}
	p, err := os.ReadFile(emittedPathFor(prompt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(p), "## Pre-work worker attestation") {
		t.Error("the emitted prompt lacks the attestation section")
	}
}

// CLASS GUARD — the defect class is "a resolver order that names a PATH port no install path
// supplies". Every consumerHelpers pathPort must be a desk-tools command built from
// tools/desk/cmd/<port>, because the Makefile and the release workflow install every cmd/*
// directory; a port named here without one would send a refusal pointing at a binary that
// installing desk-tools never puts on PATH.
//
// regression: #6131
func TestHelperPathPortsAreBuilt(t *testing.T) {
	ports := 0
	for _, h := range consumerHelpers {
		if h.pathPort == "" {
			continue
		}
		ports++
		if _, err := os.Stat(filepath.Join("..", h.pathPort, "main.go")); err != nil {
			t.Errorf("consumerHelpers names %s as the PATH port for %s, but no tools/desk/cmd/%s/main.go "+
				"builds it: %v", h.pathPort, h.rel, h.pathPort, err)
		}
	}
	// Positive control: both helpers have a port since 6131; a matcher that sees none is broken.
	if ports != len(consumerHelpers) {
		t.Fatalf("%d of %d consumer helpers carry a PATH port, want all", ports, len(consumerHelpers))
	}
}
