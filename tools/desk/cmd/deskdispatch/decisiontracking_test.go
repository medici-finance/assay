package main

// decisiontracking_test.go — issue 5525. The human-decision gate files (and dedupes) its
// decision issue in the repo that TRACKS the brief, never in the repo its deliverable lands in.
// The field failure: a `gate: human` brief tracked in one repo and delivered in another (declared
// by `homed-in:`, or dispatched with --cross-repo) ran the decision helper with --repo set to the
// DELIVERABLE repo, so the helper's dedupe never saw the decision issue that already existed in the
// tracking repo, and it either filed a second one there or failed closed.
//
// Every alias, repo and stream below is a synthetic example-* fixture (crossrepo_test.go).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionRepoArg returns the --repo value of every decision-helper call recorded by s.
func decisionRepoArg(s *stub) []string {
	var out []string
	for _, c := range s.calls {
		if c[0] != goDecisionBinary && !strings.HasSuffix(c[0], filepath.FromSlash(decisionScriptRel)) {
			continue
		}
		for i := 0; i+1 < len(c); i++ {
			if c[i] == "--repo" {
				out = append(out, c[i+1])
			}
		}
	}
	return out
}

// regression: #5525
// TestDecisionGateUsesTrackingRepo — both cross-repo shapes the issue names. The deliverable
// lands in medici-finance/assay (--root's origin, alias example-tool); the brief is tracked in
// example-org/tracker (alias example-trk). The decision helper must run against the TRACKING
// repo, exactly once, while the claim and the mint still land in the deliverable repo.
func TestDecisionGateUsesTrackingRepo(t *testing.T) {
	for _, c := range []struct {
		name  string
		front []string
		extra []string
	}{
		{"homed-in", []string{"gate: human", "homed-in: medici-finance/assay"}, nil},
		{"homed-in with brief-v2 id", []string{"gate: human", exampleV2ID, "homed-in: medici-finance/assay"}, nil},
		{"--cross-repo", []string{"gate: human", exampleV2ID},
			[]string{"--repo", "medici-finance/assay", "--cross-repo"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t) // --root's origin is medici-finance/assay
			trk := trackingCheckout(t, exampleRegistry, c.front...)
			withPortsOnPath(t, goClaimBinary, goDecisionBinary)
			s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
				reply{match: goDecisionBinary + " ensure", stdout: "exists: decision issue #4"})
			mints := recordMint(t, home, nil)
			args := append([]string{"example-stream/05", "--root", root, "--claim-root", trk,
				"--brief", exampleBriefRel, "--kit", "worker", "--quiet",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")}, c.extra...)
			if err := cmdDispatch(args); err != nil {
				t.Fatalf("a human-gated cross-repo dispatch must proceed: %v", err)
			}
			if got := decisionRepoArg(s); len(got) != 1 || got[0] != "example-org/tracker" {
				t.Fatalf("decision helper ran with --repo %q, want exactly [example-org/tracker] "+
					"(the tracking repo, never the deliverable repo)", got)
			}
			if len(*mints) != 1 || (*mints)[0] != "medici-finance/assay" {
				t.Errorf("token minted for %v — the claim still belongs to the deliverable repo", *mints)
			}
		})
	}
}

// regression: #5525
// TestDecisionGateUnknownTracking — the tracking alias is named but reserved-unpublished in
// the registry copy this run read, so the tracking repo is unknown. The gate must never fall
// through to the deliverable repo: it refuses BEFORE the claim, the mint and the worktree, and
// names --claim-root as the way out.
func TestDecisionGateUnknownTracking(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "gate: human")
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: goDecisionBinary + " ensure", stdout: "created: decision issue #9"})
	mints := recordMint(t, home, nil)

	err := cmdDispatch([]string{"example-hidden:example-stream/05", "--repo", "medici-finance/assay",
		"--cross-repo", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel,
		"--kit", "worker", "--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("an unknown tracking repo must fail closed (exit 6), never file on the deliverable: %v", err)
	}
	for _, want := range []string{stepDecisionGate, "example-hidden", "--claim-root"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, err.Error())
		}
	}
	if got := decisionRepoArg(s); len(got) != 0 {
		t.Errorf("the decision helper ran against %q with the tracking repo unknown", got)
	}
	if len(claimCalls(s)) != 0 || s.ran("deskwt add") || len(*mints) != 0 {
		t.Errorf("the refusal must precede mint, claim and worktree; calls=%v mints=%v", s.calls, *mints)
	}
}

// regression: #5525
// TestDecisionGateSameRepoUnchanged — the control: an item with no cross-repo declaration
// still gates in its own repo, so the fix moves only the split case.
func TestDecisionGateSameRepoUnchanged(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withPortsOnPath(t, goClaimBinary, goDecisionBinary)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: goDecisionBinary + " ensure", stdout: "exists: decision issue #4"})
	if err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md", "--quiet",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := decisionRepoArg(s); len(got) != 1 || got[0] != allowedRepo {
		t.Fatalf("decision helper ran with --repo %q, want [%s]", got, allowedRepo)
	}
}
