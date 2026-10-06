package main

// push.go — deskpr's push, IN-PROCESS (desktools-go-git/06).
//
// The push used to be `git push -u origin <branch>`: a git child that authenticated with
// whatever the environment carried — a credential helper, a keychain entry, an insteadOf
// rewrite — which is why the preflight had to PROBE what that transport would do. It is now
// gitcore.Push with the role App's token held in memory and presented to exactly one URL:
//
//   - an https destination (the only forge shape pushDestinationGate admits) is pushed to the
//     RESOLVED forge's canonical https URL for the gated repo, built by
//     deskkit.ForgeGitEndpointForCheckout from the same custody ForgeFor uses (deskpr's
//     already-minted ghToken on GitHub). The configured push URL's host is never the host the
//     token is presented to.
//   - a local destination (a file:// URL or an absolute path — the offline fixtures) reaches no
//     forge, so it is pushed with no credential at all.
//
// NO FORCE IS POSSIBLE. The refspecs carry no "+" and PushOpts.Force is never set, so a push
// that would need force (a rewritten branch, a diverged PR head) is refused by the protocol —
// TestForcePushRejected pins it.
//
// THE PRE-PUSH HOOK STILL RUNS. An in-process push runs no hooks, but the repository's
// configured pre-push hook (core.hooksPath — the desk push guard where it is installed) is a
// guard deskpr's own pushes have always passed through. runPrePushHook (deskkit.PrePushHook)
// resolves it the way git does and runs it with git's hook contract before any byte is sent;
// a non-zero exit stops the push, exactly as it stopped `git push`.

import (
	"fmt"
	"os/exec"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// pushSpec is one in-process push: which local ref goes to which remote branch.
type pushSpec struct {
	dir       string // the worktree pushed from
	repo      string // the owner/name the origin gate decided on
	originURL string // origin's fetch URL as git resolves it — a forge-kind hint only
	srcRef    string // "refs/heads/<branch>" or "HEAD"
	dstBranch string // the remote branch name (no refs/heads/ prefix)
}

// pushFn is the push seam. Production binds pushBranch; a test that needs to observe or fail
// the push substitutes its own.
var pushFn = pushBranch

// endpointFn resolves the https push endpoint (URL + in-memory credential). Production binds
// deskkit.ForgeGitEndpointForCheckout under the session's minted role.
var endpointFn = func(repo, originURL string) (deskkit.ForgeGitEndpoint, error) {
	return deskkit.ForgeGitEndpointForCheckout(repo, mintedRole, originURL)
}

// hookCommand builds the pre-push hook process. It is NOT git: the hook is whatever the
// repository configured. A package var so a test can observe it.
var hookCommand = exec.Command

// pushBranch pushes s.srcRef to refs/heads/<s.dstBranch> in-process. It re-reads git's own
// resolution of origin's push URL (pushDestinationGate already required exactly one https URL
// naming s.repo, or a local destination) and refuses anything else rather than trusting that
// the gate ran.
func pushBranch(s pushSpec) error {
	out, err := git(s.dir, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return deskkit.Unverifiable("cannot resolve origin's push destination", err)
	}
	dests := splitURLList(out)
	if len(dests) != 1 {
		return deskkit.Refused(fmt.Sprintf("refused: origin resolves to %d push destinations; deskpr pushes to exactly one", len(dests)))
	}
	dest := dests[0]

	var url string
	var auth transport.AuthMethod
	switch classifyPushDest(dest) {
	case pushLocal:
		// No forge, no credential.
		url = dest
	case pushHTTPS:
		slug, perr := parseRepo(dest)
		if perr != nil || slug != s.repo {
			return deskkit.Refused("refused: origin's push destination " + redactURL(dest) + " does not name " + s.repo)
		}
		ep, eerr := endpointFn(s.repo, s.originURL)
		if eerr != nil {
			return eerr
		}
		url, auth = ep.Opts.URL, ep.Opts.Auth
	default:
		return deskkit.Refused("refused: origin's push destination " + redactURL(dest) + " is not an https URL or a local repository")
	}

	repo, oerr := gitcore.Open(s.dir)
	if oerr != nil {
		return deskkit.Unverifiable("cannot open "+s.dir+" as a git repository", oerr)
	}
	hash, rerr := repo.Resolve(s.srcRef)
	if rerr != nil {
		return deskkit.Unverifiable("cannot resolve "+s.srcRef, rerr)
	}
	dstRef := "refs/heads/" + s.dstBranch

	// The pre-push hook gets the destination git would have handed it — the configured push
	// URL, not the credentialed endpoint (which carries no userinfo either way).
	if herr := runPrePushHook(s.dir, dest, s.srcRef, hash.String(), dstRef); herr != nil {
		return herr
	}

	// The refspec's source is the RESOLVED commit, never the ref name: go-git skips a
	// symbolic source (an attached HEAD) without error, so "HEAD:<dst>" from a branch
	// checkout would report success having pushed nothing. A hash source is pushed as
	// exactly that commit, fast-forward-checked against the remote's current value — and it
	// is the same sha the pre-push hook was just shown.
	if perr := repo.Push(gitcore.PushOpts{
		URL:      url,
		RefSpecs: []string{hash.String() + ":" + dstRef},
		Auth:     auth,
	}); perr != nil {
		return deskkit.Unverifiable("git push failed", perr)
	}
	return nil
}

// runPrePushHook runs the repository's pre-push hook (deskkit.PrePushHook) for this push:
// remote name "origin" and the configured push URL as argv, the one ref update on stdin. The
// hook path is git's own answer (`rev-parse --git-path hooks/pre-push`, core.hooksPath
// applied) through this package's git seam. A non-zero exit stops the push.
func runPrePushHook(dir, remoteURL, srcRef, srcSHA, dstRef string) error {
	h := deskkit.PrePushHook{
		Dir: dir,
		HookPath: func() (string, error) {
			return git(dir, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push")
		},
		Command: hookCommand,
		Stderr:  deskprStderr,
	}
	if err := h.Run("origin", remoteURL, srcRef, srcSHA, dstRef); err != nil {
		return deskkit.Unverifiable("git push failed", err)
	}
	return nil
}
