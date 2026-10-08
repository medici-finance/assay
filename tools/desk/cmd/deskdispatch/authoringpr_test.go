package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Issue #2355, part 1. A brief-AUTHORING change lives in the brief's tracking repo while the
// brief's deliverable_repo alias resolves to another repo. Reviewing that change needs the change's
// own tree (--repo and --root on the tracking repo) AND --brief, so a `gate: human` brief is still
// detected. Before the fix every flag set was refused: the tracking repo failed the registry's
// deliverable-repo witness, and dropping --brief lost the gate.

// authoringOpts plans a dispatch from a tracking checkout whose brief delivers to example-tool
// (medici-finance/assay) and is human-gated. origin is what --root's `git remote get-url` answers.
func authoringOpts(t *testing.T, origin string, o dispatchOpts) (dispatchPlan, *stub, error) {
	t.Helper()
	s := &stub{}
	s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-tool", "gate: human")
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "https://github.com/" + origin + ".git"},
		{match: "deskwt add", stdout: filepath.Join(t.TempDir(), "home")},
	}
	o.root, o.brief, o.tier, o.dryRun = trk, exampleBriefRel, "any", true
	ro, err := resolveBrief(o)
	if err != nil {
		return dispatchPlan{}, s, err
	}
	plan, err := validateCallerPreconditions(ro)
	return plan, s, err
}

func TestAuthoringPRReviewAccepted(t *testing.T) {
	for name, o := range map[string]dispatchOpts{
		"repo flag":    {item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547},
		"root origin":  {item: "tracker--pr-547", kit: "review", pr: 547},
		"worker --pr":  {item: "example-stream/05", repo: "example-org/tracker", kit: "worker", pr: 547},
		"security key": {item: "tracker--pr-547--security", repo: "example-org/tracker", kit: "review", pr: 547},
	} {
		t.Run(name, func(t *testing.T) {
			plan, s, err := authoringOpts(t, "example-org/tracker", o)
			if err != nil {
				t.Fatalf("the authoring change's own repo must satisfy the registry check: %v", err)
			}
			if plan.repo != "example-org/tracker" {
				t.Errorf("plan repo = %q, want the change's own repo example-org/tracker", plan.repo)
			}
			if !plan.gateHuman {
				t.Error("the brief's `gate: human` was not detected — the acceptance must keep --brief's gate read")
			}
			if o.kit == "review" && plan.claimKey != o.item {
				t.Errorf("review claim key changed: %s -> %s", o.item, plan.claimKey)
			}
			if len(claimCalls(s)) != 0 {
				t.Errorf("planning took a claim: %v", s.calls)
			}
		})
	}
}

// Every other shape of a registry mismatch is still the HARD FAIL: the acceptance is the tracking
// repo with --pr, nothing wider.
func TestAuthoringPRStillRefuses(t *testing.T) {
	for name, c := range map[string]struct {
		origin string
		o      dispatchOpts
	}{
		// A fresh dispatch onto the tracking repo is not an authoring-change review.
		"no --pr": {"example-org/tracker",
			dispatchOpts{item: "example-stream/05", repo: "example-org/tracker", kit: "worker"}},
		"no --pr no repo": {"example-org/tracker",
			dispatchOpts{item: "example-stream/05", kit: "worker"}},
		// --pr onto a third repo, neither tracking nor deliverable.
		"third repo": {"example-org/console",
			dispatchOpts{item: "console--pr-547", repo: "example-org/console", kit: "review", pr: 547}},
		"third origin": {"example-org/console",
			dispatchOpts{item: "console--pr-547", kit: "review", pr: 547}},
		// --repo names the tracking repo but --root is the deliverable's checkout: the reviewer's
		// tree would not be the change's tree.
		"root mismatch": {"medici-finance/assay",
			dispatchOpts{item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547}},
	} {
		t.Run(name, func(t *testing.T) {
			_, s, err := authoringOpts(t, c.origin, c.o)
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "HARD FAIL") {
				t.Fatalf("want the registry HARD FAIL (exit 5), got %v", err)
			}
			if len(claimCalls(s)) != 0 {
				t.Errorf("a hard fail must precede the claim: %v", s.calls)
			}
		})
	}
}
