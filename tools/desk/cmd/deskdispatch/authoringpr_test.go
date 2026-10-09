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
//
// The acceptance needs the tracking alias NAMED for the item (the item-key prefix or the brief's
// brief-v2 id). A tracking alias that only defaults to the registry's own `self` is refused: any
// checkout's registry can name itself, so a third repo would otherwise pass as its own tracker.

// consoleRegistry is the registry a checkout of the THIRD repo (example-org/console) carries: the
// same aliases as exampleRegistry, but its own `self` — the configuration a real third-repo checkout
// produces, as opposed to a console origin carrying the tracker's registry.
const consoleRegistry = `schema: graph-repos-v1
cell: example-cell
self: example-con
repos:
  example-trk:    {cell: example-cell, repo: example-org/tracker}
  example-tool:   {cell: example-cell, repo: medici-finance/assay}
  example-con:    {cell: example-cell, repo: example-org/console}
`

// authoringOpts plans a dispatch from a checkout carrying registry and one human-gated brief that
// delivers to example-tool (medici-finance/assay), plus any extra frontmatter. origin is what
// --root's `git remote get-url` answers.
func authoringOpts(t *testing.T, registry, origin string, o dispatchOpts, front ...string) (dispatchPlan, *stub, error) {
	t.Helper()
	s := &stub{}
	s.install(t)
	trk := trackingCheckout(t, registry, append([]string{"deliverable_repo: example-tool", "gate: human"}, front...)...)
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
	for name, c := range map[string]struct {
		o     dispatchOpts
		front []string
	}{
		"repo flag":    {o: dispatchOpts{itemAlias: "example-trk", item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547}},
		"root origin":  {o: dispatchOpts{itemAlias: "example-trk", item: "tracker--pr-547", kit: "review", pr: 547}},
		"worker --pr":  {o: dispatchOpts{itemAlias: "example-trk", item: "example-stream/05", repo: "example-org/tracker", kit: "worker", pr: 547}},
		"security key": {o: dispatchOpts{itemAlias: "example-trk", item: "tracker--pr-547--security", repo: "example-org/tracker", kit: "review", pr: 547}},
		// No key prefix: the brief's own brief-v2 id names the tracking alias.
		"brief-v2 id": {o: dispatchOpts{item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547},
			front: []string{exampleV2ID}},
	} {
		t.Run(name, func(t *testing.T) {
			plan, s, err := authoringOpts(t, exampleRegistry, "example-org/tracker", c.o, c.front...)
			if err != nil {
				t.Fatalf("the authoring change's own repo must satisfy the registry check: %v", err)
			}
			if plan.repo != "example-org/tracker" {
				t.Errorf("plan repo = %q, want the change's own repo example-org/tracker", plan.repo)
			}
			if !plan.gateHuman {
				t.Error("the brief's `gate: human` was not detected — the acceptance must keep --brief's gate read")
			}
			if c.o.kit == "review" && plan.claimKey != c.o.item {
				t.Errorf("review claim key changed: %s -> %s", c.o.item, plan.claimKey)
			}
			if len(claimCalls(s)) != 0 {
				t.Errorf("planning took a claim: %v", s.calls)
			}
		})
	}
}

// Every other shape of a registry mismatch is still the HARD FAIL: the acceptance is the NAMED
// tracking repo with --pr, nothing wider.
func TestAuthoringPRStillRefuses(t *testing.T) {
	for name, c := range map[string]struct {
		registry string
		origin   string
		o        dispatchOpts
		hint     bool // the refusal must name the fix (prefix the item key)
	}{
		// A fresh dispatch onto the tracking repo is not an authoring-change review.
		"no --pr": {exampleRegistry, "example-org/tracker",
			dispatchOpts{itemAlias: "example-trk", item: "example-stream/05", repo: "example-org/tracker", kit: "worker"}, false},
		"no --pr no repo": {exampleRegistry, "example-org/tracker",
			dispatchOpts{itemAlias: "example-trk", item: "example-stream/05", kit: "worker"}, false},
		// --pr onto a third repo, neither tracking nor deliverable, with the tracking repo named.
		"third repo": {exampleRegistry, "example-org/console",
			dispatchOpts{itemAlias: "example-trk", item: "console--pr-547", repo: "example-org/console", kit: "review", pr: 547}, false},
		"third origin": {exampleRegistry, "example-org/console",
			dispatchOpts{itemAlias: "example-trk", item: "console--pr-547", kit: "review", pr: 547}, false},
		// A real checkout of the third repo: its OWN registry names itself as `self`. With nothing
		// naming the tracking alias, that self is no evidence the checkout tracks the brief.
		"third self repo": {consoleRegistry, "example-org/console",
			dispatchOpts{item: "console--pr-547", repo: "example-org/console", kit: "review", pr: 547}, true},
		"third self origin": {consoleRegistry, "example-org/console",
			dispatchOpts{item: "console--pr-547", kit: "review", pr: 547}, true},
		"third self worker": {consoleRegistry, "example-org/console",
			dispatchOpts{item: "example-stream/05", repo: "example-org/console", kit: "worker", pr: 547}, true},
		// The tracker's own checkout, but its tracking alias is only the defaulted self: refused,
		// and the refusal names the fix.
		"tracker self only": {exampleRegistry, "example-org/tracker",
			dispatchOpts{item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547}, true},
		// --repo names the tracking repo but --root is the deliverable's checkout: the reviewer's
		// tree would not be the change's tree.
		"root mismatch": {exampleRegistry, "medici-finance/assay",
			dispatchOpts{itemAlias: "example-trk", item: "tracker--pr-547", repo: "example-org/tracker", kit: "review", pr: 547}, false},
		// --repo names the deliverable but --root is the tracker's checkout: an explicit --repo is
		// never overridden by the root origin.
		"repo deliverable root tracker": {exampleRegistry, "example-org/tracker",
			dispatchOpts{itemAlias: "example-trk", item: "assay--pr-547", repo: "medici-finance/assay", kit: "review", pr: 547}, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, s, err := authoringOpts(t, c.registry, c.origin, c.o)
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "HARD FAIL") {
				t.Fatalf("want the registry HARD FAIL (exit 5), got %v", err)
			}
			if got := strings.Contains(err.Error(), "named for the item"); got != c.hint {
				t.Errorf("refusal names the item-key-prefix fix = %v, want %v:\n%v", got, c.hint, err)
			}
			if len(claimCalls(s)) != 0 {
				t.Errorf("a hard fail must precede the claim: %v", s.calls)
			}
		})
	}
}
