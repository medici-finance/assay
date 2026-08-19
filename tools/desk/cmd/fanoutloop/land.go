package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// Sink is what makes a landed batch dispatch durable. Because Land is a near-no-op (the worker's
// draft PR is the durable artifact — no Evidence, no board flip), a Sink does only two things: record
// the handle in the dispatch log, and release the dispatch claim now that branch-as-claim has taken
// over. It is an interface so the SAFE dry-run default (which performs no network write — the
// autonomous cutover is BLOCKED-ON-HUMAN) can be swapped for the real `gh`-deleting sink at cutover,
// and so tests assert exactly what Land does with a structured Result.
type Sink interface {
	// RecordDispatch appends the landed handle (PR/branch/runner) to the dispatch log. The
	// `refs/dispatch/<id>` claim, the draft PR, and the deskroster registration are the real
	// dispatch log in production; this is the one structured record the reference build keeps.
	RecordDispatch(r loopengine.Result) error
	// ReleaseDispatchClaim clears the item's `refs/dispatch/<id>` claim. The worker's first push
	// created a branch-as-claim that now serves the mutual-exclusion role, so the dispatch ref is
	// released (SKILL § worker prompt essentials: "release the dispatch claim once the branch is
	// pushed — branch-as-claim takes over"). Releasing a missing claim is a no-op.
	ReleaseDispatchClaim(it loopengine.Item) error
}

// dryRunSink is the DEFAULT Sink. It NEVER runs a `gh` mutation — it prints what the real sink WOULD
// do. The dry-run ground rules forbid mutating remote state, and the autonomous cutover is
// BLOCKED-ON-HUMAN, so the safe default is an emitter. The real claim-releasing sink (ghDispatchSink)
// is wired only at cutover, on the owner's call.
type dryRunSink struct{ out io.Writer }

func (d dryRunSink) RecordDispatch(r loopengine.Result) error {
	artifact := r.Artifact
	if artifact == "" {
		artifact = "(no artifact — worker handed back " + r.Verdict + ")"
	}
	fmt.Fprintf(d.out, "[dry-run] dispatch-log: %s -> %s runner=%s verdict=%s\n", r.Item.ID, artifact, r.RunnerID, r.Verdict)
	return nil
}

func (d dryRunSink) ReleaseDispatchClaim(it loopengine.Item) error {
	fmt.Fprintf(d.out, "[dry-run] release dispatch claim refs/dispatch/%s (branch-as-claim has taken over)\n", claimKey(it))
	return nil
}

// ghDispatchSink is the REAL sink used only at cutover. It releases the `refs/dispatch/<id>` claim
// via the documented one-line equivalent `gh api -X DELETE repos/<owner/repo>/git/refs/dispatch/<key>`
// (SKILL § worker prompt essentials). `run` is injectable so the command shape is unit-testable
// without a live remote — and so the reference build's tests never touch the network. A 422/404 on a
// missing ref is a no-op (the claim is already gone), matching ReleaseClaim-of-missing semantics.
type ghDispatchSink struct {
	out io.Writer
	run func(args ...string) (string, error)
}

func newGHDispatchSink(out io.Writer) *ghDispatchSink {
	return &ghDispatchSink{
		out: out,
		run: func(args ...string) (string, error) {
			cmd := exec.Command(args[0], args[1:]...)
			b, err := cmd.CombinedOutput()
			return string(b), err
		},
	}
}

func (g *ghDispatchSink) RecordDispatch(r loopengine.Result) error {
	// The dispatch log in production is the durable GitHub state (the ref, the PR, the roster). The
	// sink records nothing extra beyond a progress line; the handle is the PR itself.
	fmt.Fprintf(g.out, "dispatch-log: %s -> %s runner=%s\n", r.Item.ID, r.Artifact, r.RunnerID)
	return nil
}

func (g *ghDispatchSink) ReleaseDispatchClaim(it loopengine.Item) error {
	repo := strings.TrimSpace(it.Payload["repo"])
	if repo == "" {
		return fmt.Errorf("release dispatch claim for %s: no target repo in payload", it.ID)
	}
	ref := "repos/" + repo + "/git/refs/dispatch/" + claimKey(it)
	out, err := g.run("gh", "api", "-X", "DELETE", ref)
	if err != nil {
		// A missing ref (already released, or never created) is not a failure — branch-as-claim is
		// what matters and it is already live.
		if strings.Contains(out, "Not Found") || strings.Contains(out, "422") || strings.Contains(out, "404") {
			return nil
		}
		return fmt.Errorf("gh api DELETE %s: %v: %s", ref, err, out)
	}
	return nil
}

// claimKey sanitizes an item ID into a `refs/dispatch/<key>` segment: "/" cannot appear inside a
// single ref path component, so it is replaced by "--" (the SKILL's `<repo>--<stream>--<NN>` shape).
func claimKey(it loopengine.Item) string {
	return strings.ReplaceAll(strings.ReplaceAll(it.ID, ":", "--"), "/", "--")
}
