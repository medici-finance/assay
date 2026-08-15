package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// currency.go — the instrument. Everything here is READ-ONLY; the write path is
// merge.go and it consumes this file's verdict rather than re-deriving it.
//
// THE THREE STATES ARE THE POINT (docs/three-state-instrument-rule.md). Every dimension
// below reports checked-clean, checked-failed, or could-not-check, and no dimension can
// reach checked-clean without having looked. An unfetchable ref, an unreadable PR, a
// missing local checkout — each is could-not-check, and could-not-check never rolls up
// to green. A currency tool that reported "current" because it failed to fetch would be
// worse than no tool: it would retire the question.

// state is one dimension's verdict.
type state string

const (
	stateClean    state = "checked-clean"
	stateFailed   state = "checked-failed"
	stateUnknown  state = "could-not-check"
	stateNotAsked state = "not-checked" // a dimension the caller did not ask for
)

// mergeability is what deskmerge's compiled-in boundary says about this merge.
type mergeability string

const (
	mergeClean       mergeability = "clean"            // zero conflict hunks
	mergeRegenerable mergeability = "regenerable-only" // conflicts confined to the list
	mergeConflicted  mergeability = "conflicted"       // at least one unlisted conflict
	mergeUnknown     mergeability = "could-not-check"
)

// report is one PR's currency, in the shape both the human and the JSON consumer read.
// Field names are the JSON contract; a consumer that reads `semanticValidity` gets the
// literal string "not-checked" unless --probe ran, and never gets "checked-clean" from
// a run that did not build the merged tree.
type report struct {
	Repo      string `json:"repo"`
	PR        int    `json:"pr"`
	Base      string `json:"base"`
	Head      string `json:"head"`
	BaseSHA   string `json:"baseSHA"`
	HeadSHA   string `json:"headSHA"`
	MergeBase string `json:"mergeBase"`

	// Behind/Ahead are commit counts relative to the merge base. Behind > 0 is the
	// merge-currency deficit itself.
	Behind      int   `json:"behind"`
	Ahead       int   `json:"ahead"`
	BehindState state `json:"behindState"`

	// Mergeability is the TEXTUAL verdict — and it is only ever textual. See
	// SemanticValidity.
	Mergeability     mergeability `json:"mergeability"`
	ConflictedListed []string     `json:"conflictedListedPaths"`
	ConflictedOther  []string     `json:"conflictedUnlistedPaths"`

	// CIContractDrift names the CI machinery main gained since the merge base. A
	// branch predating such a change fails those checks for reasons that have nothing
	// to do with its content — #898 added .github/scripts/verify-inscope.sh and any
	// older branch fails it with exit 127. Reporting it separately is what lets a
	// reader tell a merge-currency failure from a defect in the branch under test.
	// It is INFORMATIONAL, not a failure: merging is precisely what cures it.
	CIContractDrift      []string `json:"ciContractDriftPaths"`
	CIContractDriftState state    `json:"ciContractDriftState"`

	// SemanticValidity is "not-checked" unless --probe ran. It is NEVER inferred from
	// Mergeability. Two individually-green branches can merge with zero conflicts and
	// still be invalid against each other (#912/#913: emit() grew an 8th
	// parameter on main while a green PR still called it with 7 args) — a textual
	// checker sees nothing there, so this field must not borrow that checker's answer.
	SemanticValidity state  `json:"semanticValidity"`
	ProbeDetail      string `json:"probeDetail,omitempty"`

	// Notes carries could-not-check reasons and boundary statements, verbatim, so the
	// JSON consumer sees the same caveats the human does.
	Notes []string `json:"notes,omitempty"`
}

// exitCode rolls the dimensions up to one process exit status, and the roll-up rule is
// the whole design in three lines:
//
//	could-not-check anywhere            → 6. Never green. Never red.
//	conflicted, or a failed probe       → 5. This is worker work; deskmerge cannot fix it.
//	everything else                     → 0. deskmerge CAN fix this (or there is nothing
//	                                         to fix). "Behind" is not a failure — being
//	                                         behind with a clean merge is the ordinary,
//	                                         deskmerge-shaped state.
//
// CI-contract drift deliberately does not appear: it is cured by the merge, so treating
// it as a failure would mark the fixable case unfixable.
func (r *report) exitCode() int {
	if r.Mergeability == mergeUnknown || r.BehindState == stateUnknown ||
		r.CIContractDriftState == stateUnknown || r.SemanticValidity == stateUnknown {
		return deskkit.ExitUnverifiable
	}
	if r.Mergeability == mergeConflicted || r.SemanticValidity == stateFailed {
		return deskkit.ExitRefused
	}
	return deskkit.ExitOK
}

func (r *report) note(s string) { r.Notes = append(r.Notes, s) }

// render writes the human form. The semantic-validity line is always printed, including
// (especially) when it says NOT CHECKED — an absent line reads as "fine", and the whole
// argument of this tool is that it is not.
func (r *report) render(w *strings.Builder) {
	fmt.Fprintf(w, "%s#%d  %s -> %s\n", r.Repo, r.PR, r.Head, r.Base)
	fmt.Fprintf(w, "  head          %s\n", short(r.HeadSHA))
	fmt.Fprintf(w, "  base          %s (%s)\n", short(r.BaseSHA), r.Base)
	fmt.Fprintf(w, "  merge-base    %s\n", short(r.MergeBase))
	fmt.Fprintf(w, "  currency      %s — behind %d, ahead %d\n", r.BehindState, r.Behind, r.Ahead)
	fmt.Fprintf(w, "  mergeability  %s (TEXTUAL only)\n", r.Mergeability)
	if len(r.ConflictedListed) > 0 {
		fmt.Fprintf(w, "    regenerable conflicts: %s\n", strings.Join(r.ConflictedListed, " "))
	}
	if len(r.ConflictedOther) > 0 {
		fmt.Fprintf(w, "    UNLISTED conflicts:    %s\n", strings.Join(r.ConflictedOther, " "))
	}
	fmt.Fprintf(w, "  ci-contract   %s", r.CIContractDriftState)
	if len(r.CIContractDrift) > 0 {
		fmt.Fprintf(w, " — main gained CI machinery this branch has never run:\n    %s\n",
			strings.Join(r.CIContractDrift, "\n    "))
		fmt.Fprintf(w, "    A check failing on this branch may be THIS, not a defect in the branch.\n")
	} else {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "  semantic      %s", r.SemanticValidity)
	switch r.SemanticValidity {
	case stateNotAsked:
		fmt.Fprintf(w, " — deskmerge did NOT evaluate whether the merged tree is self-consistent.\n"+
			"    A textually-clean merge of two individually-green branches can still be invalid\n"+
			"    (#912/#913). Re-run with --probe to build the merged tree.\n")
	default:
		if r.ProbeDetail != "" {
			fmt.Fprintf(w, " — %s\n", r.ProbeDetail)
		} else {
			fmt.Fprintln(w)
		}
	}
	for _, n := range r.Notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	if sha == "" {
		return "(unknown)"
	}
	return sha
}

// ---------------------------------------------------------------------------
// the local checkout
// ---------------------------------------------------------------------------

// resolveRepoRoot finds the local checkout deskmerge operates from.
//
// Fail-closed twice over: an unresolvable root is could-not-check (6), and a root whose
// `origin` does not name the requested repo is REFUSED (5). The second check is not
// paranoia — the desk keeps several checkouts side by side, and a merge computed in the
// wrong one would be a valid two-parent merge of the wrong project.
func resolveRepoRoot(repo, override string) (string, error) {
	root := strings.TrimSpace(override)
	if root == "" {
		root = deskkit.RootForRepo(repo)
	}
	if root == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: no local checkout is configured for %s — set DESK_ROOTS or pass "+
				"--repo-root <dir>. deskmerge computes the merge locally and cannot answer without one",
			deskkit.StripControl(repo)), nil)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", deskkit.Unverifiable("could-not-check: cannot resolve "+root, err)
	}
	if _, err := runGit(abs, "rev-parse", "--git-common-dir"); err != nil {
		return "", deskkit.Unverifiable(
			"could-not-check: "+abs+" is not a git checkout", err)
	}
	url, err := runGit(abs, "remote", "get-url", "origin")
	if err != nil {
		return "", deskkit.Unverifiable(
			"could-not-check: "+abs+" has no `origin` remote to fetch from", err)
	}
	if !originNames(url, repo) {
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: %s's origin is %s, which does not name %s — deskmerge will not compute a merge "+
				"in one project's checkout and push it to another's",
			abs, deskkit.StripControl(url), deskkit.StripControl(repo)))
	}
	return abs, nil
}

// originNames reports whether a remote URL names owner/repo, across the ssh, https and
// scp-like forms git accepts.
func originNames(url, repo string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return strings.HasSuffix(u, "/"+strings.ToLower(repo)) ||
		strings.HasSuffix(u, ":"+strings.ToLower(repo))
}

// prHeadRef is the namespaced local ref deskmerge fetches a PR head into. It lives
// under refs/deskmerge/ so it can never be mistaken for — or listed among — the
// operator's own branches, and so a stale one cannot shadow a real branch name.
func prHeadRef(pr int) string { return fmt.Sprintf("refs/deskmerge/pr-%d", pr) }

// fetchState fetches the base branch and the PR head, and returns their SHAs.
//
// The head SHA is cross-checked against the one GitHub reported. A mismatch means the
// PR moved between the API read and the fetch, and it is could-not-check rather than a
// silent "well, use the newer one": every verdict below is relative to a specific head,
// and quietly re-basing the question onto a different one would make the answer true of
// a PR nobody asked about.
func fetchState(root, repo string, p prInfo) (baseSHA, headSHA string, err error) {
	if _, ferr := runGit(root, "fetch", "--quiet", "origin",
		"+refs/heads/"+p.BaseRefName+":refs/remotes/origin/"+p.BaseRefName,
		"+refs/pull/"+strconv.Itoa(p.Number)+"/head:"+prHeadRef(p.Number),
	); ferr != nil {
		return "", "", deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot fetch %s's %s and refs/pull/%d/head — an unfetched ref makes "+
				"every currency verdict unverifiable. deskmerge reports could-not-check rather than "+
				"answering from a stale local ref",
			deskkit.StripControl(repo), deskkit.StripControl(p.BaseRefName), p.Number), ferr)
	}
	baseSHA, err = runGit(root, "rev-parse", "refs/remotes/origin/"+p.BaseRefName)
	if err != nil {
		return "", "", deskkit.Unverifiable(
			"could-not-check: cannot resolve origin/"+deskkit.StripControl(p.BaseRefName)+" after fetching it", err)
	}
	headSHA, err = runGit(root, "rev-parse", prHeadRef(p.Number))
	if err != nil {
		return "", "", deskkit.Unverifiable(
			"could-not-check: cannot resolve the fetched PR head", err)
	}
	if !strings.EqualFold(headSHA, p.HeadRefOid) {
		return "", "", deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: GitHub reported head %s but refs/pull/%d/head fetched %s — the PR moved "+
				"mid-run. Every verdict is relative to a head, so deskmerge answers about neither; re-run",
			short(deskkit.StripControl(p.HeadRefOid)), p.Number, short(headSHA)), nil)
	}
	return baseSHA, headSHA, nil
}

// dropPRHeadRef removes the namespaced fetch ref. Best-effort: a leftover ref is
// harmless (it is namespaced out of every branch listing), so its removal must never
// turn a successful run into a failure.
func dropPRHeadRef(root string, pr int) {
	_, _ = runGit(root, "update-ref", "-d", prHeadRef(pr))
}

// ---------------------------------------------------------------------------
// the temp worktree
// ---------------------------------------------------------------------------

// tmpWorktreeBase is the parent of every scratch worktree deskmerge creates. It is
// os.TempDir() and not the repo, because the ONE property the merge must have is that
// it never happens in the desk's own tree.
var tmpWorktreeBase = os.TempDir

// worktree is a detached scratch checkout plus its removal.
type worktree struct {
	root string // the repo root that owns it
	dir  string
}

// newWorktree creates a DETACHED worktree at `at`. Detached on purpose: a worktree on a
// branch could be fast-forwarded, and a fast-forward is not a merge — it is the
// #72 single-parent masquerade, arrived at by accident.
func newWorktree(root, at string) (*worktree, error) {
	dir, err := os.MkdirTemp(tmpWorktreeBase(), "deskmerge-")
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-check: cannot create a scratch directory", err)
	}
	// MkdirTemp created it; `git worktree add` requires the path to be absent or
	// empty, and an existing empty dir is accepted.
	if _, err := runGit(root, "worktree", "add", "--detach", "--quiet", dir, at); err != nil {
		_ = os.RemoveAll(dir)
		return nil, deskkit.Unverifiable(
			"could-not-check: cannot create a scratch worktree at "+short(at), err)
	}
	return &worktree{root: root, dir: dir}, nil
}

// remove tears the worktree down on EVERY exit path, and is written to succeed against
// a tree in any state — mid-merge, mid-conflict, or already gone. Each step is
// best-effort and the last one (RemoveAll) does not depend on git agreeing that the
// worktree exists, because the failure this guards against is a scratch tree surviving
// a crash and being reused by the next run.
func (w *worktree) remove() {
	if w == nil || w.dir == "" {
		return
	}
	_, _ = runGit(w.dir, "merge", "--abort")
	_, _ = runGit(w.root, "worktree", "remove", "--force", w.dir)
	_ = os.RemoveAll(w.dir)
	_, _ = runGit(w.root, "worktree", "prune")
	w.dir = ""
}

// conflictedPaths lists the unmerged paths in a worktree mid-conflict.
func conflictedPaths(dir string) ([]string, error) {
	out, err := runGit(dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, deskkit.Unverifiable(
			"could-not-check: the merge failed and the conflicted path list could not be read either — "+
				"deskmerge cannot say WHY it failed, so it says nothing about it", err)
	}
	var paths []string
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			paths = append(paths, ln)
		}
	}
	if len(paths) == 0 {
		return nil, deskkit.Unverifiable(
			"could-not-check: `git merge` failed but reported ZERO unmerged paths — the failure is "+
				"something other than a conflict (a hook, a lock, a full disk), and deskmerge will not "+
				"report a conflict it cannot name", nil)
	}
	return paths, nil
}
