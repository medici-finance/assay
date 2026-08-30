package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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

// dryRunSink is the DEFAULT Sink. It performs NO remote write — it prints what the real sink WOULD
// do. The dry-run ground rules forbid mutating remote state, and the autonomous cutover is
// BLOCKED-ON-HUMAN, so the safe default is an emitter. The real claim-releasing sink
// (forgeDispatchSink) is wired only at cutover, on the owner's call.
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

// forgeDispatchSink is the REAL sink used only at cutover. It releases the `refs/dispatch/<id>`
// claim through the ENUMERATED forge operation `DeleteRef` (the closed-forge-surface brief).
//
// It used to spell that release as `gh api -X DELETE repos/<owner/repo>/git/refs/dispatch/<key>`.
// That one line was the stream's live example of a passthrough: an argv carrying a whole REST path,
// so the surface actually reachable from this sink was not "delete a dispatch ref" but "any endpoint
// on the forge, by any method" — and reachable, besides, only on a runner with the `gh` binary
// installed and whatever ambient credential it happened to carry. `DeleteRef(repo, "dispatch/<key>")`
// is the same operation with the reach removed: the ref is validated inside the repo's own namespace
// (deskkit.ValidateRefPath) before a request exists, and the identity is the token the Forge was
// constructed with.
//
// The Forge is injected so the release is unit-testable without a live remote — the reference build's
// tests never touch the network — and so a GitLab profile substitutes its own backend rather than a
// second sink. A 404/422 on a missing ref stays a no-op: the claim is already gone, branch-as-claim
// is live, and re-releasing is not a failure.
type forgeDispatchSink struct {
	out   io.Writer
	forge deskkit.Forge
}

func newForgeDispatchSink(out io.Writer, f deskkit.Forge) *forgeDispatchSink {
	return &forgeDispatchSink{out: out, forge: f}
}

func (g *forgeDispatchSink) RecordDispatch(r loopengine.Result) error {
	// The dispatch log in production is the durable forge state (the ref, the PR, the roster). The
	// sink records nothing extra beyond a progress line; the handle is the PR itself.
	fmt.Fprintf(g.out, "dispatch-log: %s -> %s runner=%s\n", r.Item.ID, r.Artifact, r.RunnerID)
	return nil
}

func (g *forgeDispatchSink) ReleaseDispatchClaim(it loopengine.Item) error {
	repo := strings.TrimSpace(it.Payload["repo"])
	if repo == "" {
		return fmt.Errorf("release dispatch claim for %s: no target repo in payload", it.ID)
	}
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return fmt.Errorf("release dispatch claim for %s: target repo %q is not owner/name", it.ID, repo)
	}
	if g.forge == nil {
		return fmt.Errorf("release dispatch claim for %s: no forge wired into the sink", it.ID)
	}
	ref := dispatchRefNamespace + "/" + claimKey(it)
	if err := g.forge.DeleteRef(deskkit.ForgeRepo{Owner: owner, Name: name}, ref); err != nil {
		if refAlreadyGone(err) {
			return nil
		}
		return fmt.Errorf("release dispatch claim refs/%s in %s: %w", ref, repo, err)
	}
	return nil
}

// dispatchRefNamespace is the ref namespace the dispatch claim lives in. It is a constant rather
// than an interpolation so no caller-supplied value can widen which namespace this sink deletes from.
const dispatchRefNamespace = "dispatch"

// refAlreadyGone reports whether a DeleteRef error means the ref was not there to delete. GitHub
// answers a missing git ref with 404, and 422 for a ref name it will not resolve; either way the
// claim is not held, which is the only thing this sink cares about. Every other status — a 401
// re-mint, a 403 permission gate — is a real failure and is NOT swallowed, because reporting a
// still-held claim as released is how two workers end up on one item.
func refAlreadyGone(err error) bool {
	if deskkit.IsForgeNotFound(err) {
		return true
	}
	var ae *deskkit.ForgeAPIError
	return errors.As(err, &ae) && ae.Status == http.StatusUnprocessableEntity
}

// claimKey sanitizes an item ID into a `refs/dispatch/<key>` segment: "/" cannot appear inside a
// single ref path component, so it is replaced by "--" (the SKILL's `<repo>--<stream>--<NN>` shape).
func claimKey(it loopengine.Item) string {
	return strings.ReplaceAll(strings.ReplaceAll(it.ID, ":", "--"), "/", "--")
}
