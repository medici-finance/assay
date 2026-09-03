package filer

// githubadapter.go is the GitHub Issues REFERENCE adapter (spec §9.5): one
// generic filer shipped in-tree so the interface is usable out of the box.
// Other trackers are a different IssueFiler behind the same interface, not a
// fork of this file.
//
// The adapter contacts NO network on its own. The actual Issues API call is a
// single injected Post hook; a nil hook (the zero value) makes the adapter
// dry-run-only, so the reference adapter is safe to construct and exercise
// offline and in tests without ever reaching GitHub. Wiring a real Post is a
// caller's deliberate act.

// GitHubPoster performs the one side-effecting operation the adapter needs: it
// creates an issue for the composed item and returns its tracker reference
// (e.g. the issue URL). It is the ONLY seam that may touch the network, and it
// is injected — the reference adapter carries no HTTP client of its own, which
// is what keeps the package offline-testable and the never-self-dispatch
// posture auditable (there is nothing here that could start work).
type GitHubPoster func(owner, repo string, item RefactorItem) (ref string, err error)

// GitHubFiler is the reference IssueFiler over GitHub Issues.
type GitHubFiler struct {
	// Owner and Repo name the tracker repository issues are filed against.
	Owner string
	Repo  string
	// Post is the injected issue-creation call. When nil, the adapter can only
	// dry-run: File composes the item and returns Filed == false without any
	// side effect. This is the default, so a zero-value GitHubFiler never files.
	Post GitHubPoster
	// ForceDryRun makes every File a dry-run regardless of the caller's flag and
	// regardless of whether Post is wired — a belt-and-braces switch a caller can
	// set to prove composition without any chance of a live write.
	ForceDryRun bool
}

// File composes item and, unless dry-running, files it through Post.
//
// The call dry-runs when ANY of these hold: the caller passed dryRun (e.g. it
// is over budget), the filer has ForceDryRun set, or no Post hook is wired. In
// every dry-run case nothing is written and the returned FiledResult has
// Filed == false, DryRun == true, Ref == "". Only when a real write happens is
// Filed == true with the tracker Ref populated.
func (g *GitHubFiler) File(item RefactorItem, dryRun bool) (FiledResult, error) {
	if err := validateItem(item); err != nil {
		return FiledResult{}, err
	}

	dry := dryRun || g.ForceDryRun || g.Post == nil
	if dry {
		return FiledResult{Item: item, Filed: false, DryRun: true}, nil
	}

	ref, err := g.Post(g.Owner, g.Repo, item)
	if err != nil {
		return FiledResult{}, err
	}
	return FiledResult{Item: item, Filed: true, DryRun: false, Ref: ref}, nil
}

// compile-time assertion the reference adapter satisfies the interface.
var _ IssueFiler = (*GitHubFiler)(nil)
