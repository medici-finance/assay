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
// over. It is an interface so an explicit dry-run (which performs no network write) can be selected
// in place of the real releasing sink, and so tests assert exactly what Land does with a structured
// Result.
type Sink interface {
	// RecordDispatch appends the landed handle (PR/branch/runner) to the dispatch log. The
	// claim ref, the draft PR, and the deskroster registration are the real dispatch log in
	// production; this is the one structured record the reference build keeps.
	RecordDispatch(r loopengine.Result) error
	// ReleaseDispatchClaim clears the item's dispatch claim ref. The worker's first push
	// created a branch-as-claim that now serves the mutual-exclusion role, so the dispatch ref is
	// released (SKILL § worker prompt essentials: "release the dispatch claim once the branch is
	// pushed — branch-as-claim takes over"). Releasing a missing claim is a no-op.
	ReleaseDispatchClaim(it loopengine.Item) error
}

// dryRunSink performs NO remote write — it prints what the real sink WOULD do. It is what the
// `plan` surface and any explicitly dry-run driver select; it is NOT the fallback for a
// misconfigured deployment, because a sink that silently declines to release claims leaks a
// slot per landing.
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
	fmt.Fprintf(d.out, "[dry-run] release dispatch claim %s%s (branch-as-claim has taken over)\n",
		deskkit.ClaimRefsPrefix, claimKey(it))
	return nil
}

// forgeResolver returns the Forge that serves one repo. It is a seam ONLY so a test can drive
// the release without a live remote; production installs deskkit.ForgeFor, which is the single
// construction site for a backend and the one place the forge/custody/refusal contract lives.
// It takes the repo because a batch can span repos: the forge that serves the item's TARGET is
// the one that must delete its claim, not whichever forge the loop happened to boot under.
type forgeResolver func(repo deskkit.ForgeRepo) (deskkit.Forge, error)

// productionForgeResolver is the wiring the "wired only at cutover" note used to defer. The
// role is the session's own — the same SessionTokenRole lookup every other verb that acts on
// dispatch claims performs — so the release is made by the identity the session was booted
// as, never by an ambient credential (deskkit.ForgeFor refuses to fall back to one).
func productionForgeResolver(repo deskkit.ForgeRepo) (deskkit.Forge, error) {
	role, _, err := deskkit.SessionTokenRole("fanoutloop")
	if err != nil {
		return nil, err
	}
	return deskkit.ForgeFor(repo, role)
}

// forgeDispatchSink is the REAL sink: it releases the dispatch claim through the ENUMERATED forge
// operation `DeleteRef` (the closed-forge-surface brief).
//
// It used to spell that release as `gh api -X DELETE repos/<owner/repo>/git/<the claim ref>`.
// That one line was the stream's live example of a passthrough: an argv carrying a whole REST path,
// so the surface actually reachable from this sink was not "delete a dispatch ref" but "any endpoint
// on the forge, by any method" — and reachable, besides, only on a runner with the `gh` binary
// installed and whatever ambient credential it happened to carry. `DeleteRef(repo, <claim ref>)`
// is the same operation with the reach removed: the ref is validated inside the repo's own namespace
// (deskkit.ValidateRefPath, via deskkit.ClaimRefPath) before a request exists, and the identity is
// the token the Forge was constructed with.
//
// A 404/422 on a missing ref stays a no-op: the claim is already gone, branch-as-claim is live, and
// re-releasing is not a failure. EVERY other outcome — an unresolvable forge, a refused credential,
// a permission failure — is reported, loudly and by claim key. Reporting a still-held claim as
// released is how two workers end up on one item, so this sink has no path that returns nil without
// the forge having answered that the ref is gone.
type forgeDispatchSink struct {
	out     io.Writer
	resolve forgeResolver
}

// newForgeDispatchSink builds the releasing sink. A nil resolver is REFUSED at construction:
// the nil-forge state this sink used to ship in ("no forge wired into the sink", reached only
// because nothing ever constructed one) is not a runtime condition to report, it is a sink
// that must not exist.
func newForgeDispatchSink(out io.Writer, resolve forgeResolver) (*forgeDispatchSink, error) {
	if resolve == nil {
		return nil, deskkit.Refused("refusing to build a dispatch sink with no forge resolver — a sink " +
			"that cannot obtain a forge cannot release a claim, and a claim never released is a slot lost")
	}
	return &forgeDispatchSink{out: out, resolve: resolve}, nil
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
	key := claimKey(it)
	ref, rerr := deskkit.ClaimRefPath(key)
	if rerr != nil {
		return fmt.Errorf("release dispatch claim for %s: %w", it.ID, rerr)
	}
	forgeRepo := deskkit.ForgeRepo{Owner: owner, Name: name}
	forge, ferr := g.resolve(forgeRepo)
	if ferr != nil {
		return fmt.Errorf("release dispatch claim %s in %s: cannot resolve a forge: %w", key, repo, ferr)
	}
	if forge == nil {
		// The resolver answered with neither a forge nor an error. Nothing may be inferred
		// from that, least of all that the claim is free.
		return fmt.Errorf("release dispatch claim %s in %s: the forge resolver returned no forge and no "+
			"error, so the claim is NOT released", key, repo)
	}
	if err := forge.DeleteRef(forgeRepo, ref); err != nil {
		if refAlreadyGone(err) {
			return nil
		}
		return fmt.Errorf("release dispatch claim refs/%s in %s: %w", ref, repo, err)
	}
	// Printed ONLY after the forge has answered that the ref is gone. A release line on any
	// other path would be a record of something that did not happen.
	fmt.Fprintf(g.out, "released dispatch claim refs/%s in %s\n", ref, repo)
	return nil
}

// dispatchRefNamespace is the ref namespace the dispatch claim lives in. It is DERIVED from the
// one definition in deskkit (claimref.go) rather than spelled again here, so the namespace the
// writer deletes from and the namespace the readers list cannot drift apart; and it is a constant
// rather than an interpolation so no caller-supplied value can widen it.
const dispatchRefNamespace = deskkit.ClaimRefNamespace

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

// claimKey sanitizes an item ID into the single claim-key segment under the claim namespace
// (deskkit.ClaimRefsPrefix): "/" cannot appear inside a
// single ref path component, so it is replaced by "--" (the SKILL's `<repo>--<stream>--<NN>` shape).
func claimKey(it loopengine.Item) string {
	return strings.ReplaceAll(strings.ReplaceAll(it.ID, ":", "--"), "/", "--")
}
