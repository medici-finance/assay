package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// merge.go — the write path. Everything in this file exists to keep ONE sentence true:
// a desk merge introduces no reviewable content.

// defaultBranchNames are the names deskmerge refuses to push to under any circumstance,
// regardless of what the PR claims its head branch is called. R-5's direction bound is
// negative and absolute — merging a PR INTO main is the human's alone — and this is that bound
// as a compiled-in list rather than as a property that happens to fall out of the
// control flow.
//
// It is belt AND braces: the push refspec is built from the PR's headRefName, which
// should never be a default branch, so this check should never fire. A guard that should
// never fire is exactly the guard worth having, because the day it fires is the day
// something upstream was wrong.
var defaultBranchNames = map[string]bool{
	"main": true, "master": true, "trunk": true, "develop": true, "release": true,
}

// cmdMerge is the write verb. Order is load-bearing: authority, then eligibility, then
// budget, then work. Nothing touches the filesystem before the human gate has answered.
func cmdMerge(args []string, out io.Writer) error {
	o, err := parseArgs(verbMerge, args, true)
	if err != nil {
		return err
	}

	// THE R-5 GATE. A --dry-run is a read and is NOT gated: a desk that cannot yet
	// merge should still be able to find out what it would do, and gating the
	// rehearsal would only push callers toward guessing. Every path that WRITES is
	// below the gate.
	var g grant
	if !o.dryRun {
		if g, err = authorize(o.rulings); err != nil {
			return err
		}
	}

	root, err := resolveRepoRoot(o.repo, o.repoRoot)
	if err != nil {
		return err
	}
	p, err := fetchPR(o.repo, o.pr)
	if err != nil {
		return err
	}
	if err := eligibleForMerge(o.repo, p); err != nil {
		return err
	}
	if err := assertPushableBranch(p); err != nil {
		return err
	}
	defer dropPRHeadRef(root, o.pr)

	t, aerr := assess(root, o.repo, p, o.probe)
	defer t.close()
	if aerr != nil {
		return aerr
	}
	emit(out, t.rep, o.jsonOut)

	if t.UpToDate {
		fmt.Fprintf(out, "noop: %s#%d is already current with %s\n", o.repo, o.pr, p.BaseRefName)
		return auditNoop(o, p, t, "already-current")
	}

	switch t.rep.Mergeability {
	case mergeConflicted:
		// The refusal IS the product: this output is the worker-dispatch signal the
		// brief specifies, and it names the paths so the dispatch does not have to
		// re-derive them.
		_ = auditNoop(o, p, t, "conflicted")
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d has conflicts outside the compiled-in regenerable list (%s):\n  %s\n"+
				"Conflict resolution is AUTHORSHIP, and the desk never approves its own content. "+
				"Dispatch a worker. (Regenerable conflicts also present: %s)",
			deskkit.StripControl(o.repo), o.pr, regenerableList(),
			strings.Join(t.Unlisted, "\n  "), joinOrNone(t.Listed)))
	case mergeRegenerable:
		if err := resolveRegenerable(t); err != nil {
			_ = auditNoop(o, p, t, "regenerator-failed")
			return err
		}
	case mergeClean:
		// nothing to resolve
	default:
		return deskkit.Unverifiable("could-not-check: mergeability was never determined", nil)
	}

	if t.rep.SemanticValidity == stateFailed {
		_ = auditNoop(o, p, t, "probe-failed")
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d — %s", deskkit.StripControl(o.repo), o.pr, t.rep.ProbeDetail))
	}

	if o.dryRun {
		fmt.Fprintf(out, "dry-run: would merge %s (%s) into %s#%d and push; nothing written\n",
			short(t.rep.BaseSHA), p.BaseRefName, o.repo, o.pr)
		return auditDryRun(o, p, t)
	}

	// The budget is charged BEFORE the commit, not after: a run that will not be
	// allowed to push must not first spend a minute producing a commit nobody can use.
	if err := allowWrite(o.repo, o.pr); err != nil {
		return err
	}

	sha, err := commitMerge(t, g, p)
	if err != nil {
		return err
	}
	if err := verifyTwoParent(t, sha, p); err != nil {
		// Nothing has been pushed. The rollback is the worktree teardown the deferred
		// close performs — the malformed commit exists only in a scratch tree that is
		// about to stop existing, and no ref anywhere else ever pointed at it.
		return err
	}
	if err := push(t, p); err != nil {
		return err
	}
	fmt.Fprintf(out, "merged: %s#%d now carries a two-parent merge %s of %s (%s)\n",
		o.repo, o.pr, short(sha), short(t.rep.BaseSHA), p.BaseRefName)
	return auditMerge(o, p, t, sha, g)
}

func joinOrNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, " ")
}

// assertPushableBranch is the compiled-in half of R-5's direction bound.
func assertPushableBranch(p prInfo) error {
	name := strings.TrimSpace(p.HeadRefName)
	if name == "" {
		return deskkit.Unverifiable(
			"could-not-check: the PR reported no head branch name, so deskmerge cannot say WHERE a "+
				"push would land — and it will not find out by trying", nil)
	}
	if defaultBranchNames[strings.ToLower(name)] || strings.EqualFold(name, p.BaseRefName) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the PR's head branch is %q — deskmerge never pushes to a default branch. "+
				"R-5 authorizes merging main INTO a PR branch and nothing else; merging a PR into "+
				"main is the human's alone, and deskmerge holds no merge authority over any branch "+
				"but the PR's own", deskkit.StripControl(name)))
	}
	return nil
}

// resolveRegenerable re-runs each conflicted path's generator and requires the result to
// be a fully-resolved index with NO hunks left anywhere.
//
// The final check is the load-bearing one and it is deliberately about the WHOLE tree,
// not about the listed paths: a generator that resolved its own file while leaving
// another unmerged would otherwise produce a commit that is part transport and part
// abandoned merge.
func resolveRegenerable(t *trial) error {
	if len(t.Unlisted) > 0 {
		return deskkit.Refused(
			"refused: resolveRegenerable called with unlisted conflicts present — refusing rather " +
				"than resolving half a merge")
	}
	for _, path := range t.Listed {
		r, ok := regenerable[path]
		if !ok {
			return deskkit.Refused("refused: " + deskkit.StripControl(path) +
				" is not on the regenerable list")
		}
		if _, err := runCmdIn(t.wt.dir, r.Tool, r.Argv...); err != nil {
			return deskkit.Refused(fmt.Sprintf(
				"refused: the generator for %s (%s) failed, so the conflict is UNRESOLVED. deskmerge "+
					"commits nothing — a partial resolution would be desk-authored content wearing a "+
					"transport commit's clothes: %s",
				deskkit.StripControl(path), r.Tool, deskkit.StripControl(firstLines(err.Error(), 2))))
		}
		// THE MARKER CHECK, and it is not decoration. `git add` on a still-conflicted
		// path marks it RESOLVED with git's conflict markers left in the content, so
		// the "no unmerged paths remain" check below passes for a generator that did
		// nothing at all — and the markers ride into the commit and out to the remote.
		// This exact defect was caught by TestRegenerableCarveout's no-op generator
		// case before it shipped; the case stays as its positive control.
		body, rerr := os.ReadFile(filepath.Join(t.wt.dir, path))
		if rerr != nil {
			return deskkit.Unverifiable("could-not-check: cannot read the regenerated "+
				deskkit.StripControl(path)+" back", rerr)
		}
		if hasConflictMarkers(body) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s still carries git conflict markers after its generator ran, so the "+
					"generator did not regenerate it. Staging that would put merge markers in the "+
					"PR's head under a transport commit's message. Nothing committed.",
				deskkit.StripControl(path)))
		}
		if _, err := runGit(t.wt.dir, "add", "--", path); err != nil {
			return deskkit.Unverifiable(
				"could-not-check: cannot stage the regenerated "+deskkit.StripControl(path), err)
		}
	}
	left, err := runGit(t.wt.dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return deskkit.Unverifiable(
			"could-not-check: cannot confirm the regenerated tree has no conflicts left", err)
	}
	if strings.TrimSpace(left) != "" {
		return deskkit.Refused(
			"refused: hunks remain unmerged after regeneration: " +
				deskkit.StripControl(strings.Join(strings.Fields(left), " ")))
	}
	return nil
}

// hasConflictMarkers reports whether b carries git's conflict markers.
//
// It requires BOTH the opening and the closing marker at a line start. `=======` alone
// is deliberately not enough: it is a setext heading underline in Markdown, and
// STATUS.md — the one file on the regenerable list — is Markdown.
func hasConflictMarkers(b []byte) bool {
	var open, closed bool
	for _, ln := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(ln, "<<<<<<< "):
			open = true
		case strings.HasPrefix(ln, ">>>>>>> "):
			closed = true
		}
	}
	return open && closed
}

// commitMerge writes the merge commit. The message carries the ruling, the authorizing
// artifact and both parent SHAs, so a reader of the commit can audit the authority
// without asking the desk what it thought it was doing.
func commitMerge(t *trial, g grant, p prInfo) (string, error) {
	disposition := "a clean auto-merge with zero conflict hunks"
	if len(t.Listed) > 0 {
		disposition = "conflicts confined to the regenerable list (" +
			strings.Join(t.Listed, " ") + "), resolved by re-running each generator"
	}
	msg := fmt.Sprintf(`Merge %s into %s (merge-currency)

Brought current with %s by deskmerge under issue-flow/01 %s. This commit introduces no
reviewable content: %s.

deskmerge-ruling: %s
deskmerge-authority: %s
deskmerge-parent1: %s
deskmerge-parent2: %s
`, p.BaseRefName, p.HeadRefName, p.BaseRefName, rulingID, disposition,
		rulingID, g.SignOffURL, t.rep.HeadSHA, t.rep.BaseSHA)

	if _, err := runGit(t.wt.dir, "commit", "--no-verify", "-m", msg); err != nil {
		return "", deskkit.Unverifiable(
			"could-not-check: the merge commit could not be written; nothing was pushed", err)
	}
	sha, err := runGit(t.wt.dir, "rev-parse", "HEAD")
	if err != nil {
		return "", deskkit.Unverifiable(
			"could-not-check: the merge commit was written but could not be read back", err)
	}
	return sha, nil
}

// verifyTwoParent inspects the COMMIT, not the intent.
//
// This is the check the brief names as the reason for the whole tool's exec tier: it is
// what makes "the desk performed a transport operation" a verified property of the
// object that will be pushed rather than a description of what the code was trying to
// do. Everything it rejects — a fast-forward, a rebase, a squash, an amended
// single-parent commit, a merge of the wrong base — produces a head that LOOKS current
// and is not (#72).
//
// It runs after the commit and before the push, so a failure costs nothing: no ref
// outside the scratch worktree ever pointed at the commit.
func verifyTwoParent(t *trial, sha string, p prInfo) error {
	line, err := runGit(t.wt.dir, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return deskkit.Unverifiable(
			"could-not-check: cannot read the parents of the commit just written — deskmerge will "+
				"not push a commit whose shape it could not verify", err)
	}
	f := strings.Fields(line)
	if len(f) != 3 {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the resulting commit has %d parent(s), not 2 — this is not a merge. A "+
				"single-parent result is the #72 masquerade: a head that reads as "+
				"current with %s while carrying none of its history. Rolled back; nothing pushed.",
			len(f)-1, deskkit.StripControl(p.BaseRefName)))
	}
	if !strings.EqualFold(f[1], t.rep.HeadSHA) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: parent 1 is %s but the PR head was %s — the first parent must be the PR's own "+
				"history, unchanged. Rolled back; nothing pushed.", short(f[1]), short(t.rep.HeadSHA)))
	}
	if !strings.EqualFold(f[2], t.rep.BaseSHA) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: parent 2 is %s but the fetched %s head was %s — the second parent must be the "+
				"exact base commit this run measured against. Rolled back; nothing pushed.",
			short(f[2]), deskkit.StripControl(p.BaseRefName), short(t.rep.BaseSHA)))
	}
	return nil
}

// push sends the merge to the PR's own branch, and to nothing else.
//
// The refspec is fully qualified on both sides (HEAD:refs/heads/<branch>) so no
// remote-side configuration — a push.default, a refspec in the remote's config, a
// matching-branch rule — can redirect it. assertPushableBranch has already refused
// every default-branch name; this is where that refusal is spent.
func push(t *trial, p prInfo) error {
	refspec := "HEAD:refs/heads/" + p.HeadRefName
	if _, err := runGit(t.wt.dir, "push", "origin", refspec); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the push of %s was rejected or did not complete. The merge exists only "+
				"in a scratch worktree that is about to be removed, so the PR is unchanged; re-run",
			deskkit.StripControl(refspec)), err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// audit
// ---------------------------------------------------------------------------

func auditEntry(o opts, p prInfo, t *trial, result, detail string) deskkit.Entry {
	pr := o.pr
	head := t.rep.HeadSHA
	return deskkit.Entry{
		Tool:       toolName,
		Verb:       verbMerge,
		ArgsDigest: deskkit.ArgsDigest([]string{o.repo, fmt.Sprint(o.pr)}),
		Repo:       o.repo,
		PR:         &pr,
		HeadSHA:    &head,
		Result:     result,
		Detail:     detail,
	}
}

func auditMerge(o opts, p prInfo, t *trial, sha string, g grant) error {
	return auditLog(auditEntry(o, p, t, deskkit.ResultOK, fmt.Sprintf(
		"%s parent1=%s parent2=%s merge=%s disposition=%s authority=%s",
		rulingID, short(t.rep.HeadSHA), short(t.rep.BaseSHA), short(sha),
		dispositionWord(t), g.SignOffURL)))
}

func auditNoop(o opts, p prInfo, t *trial, why string) error {
	return auditLog(auditEntry(o, p, t, deskkit.ResultRefused, fmt.Sprintf(
		"%s no-write=%s parent1=%s parent2=%s unlisted=%s",
		rulingID, why, short(t.rep.HeadSHA), short(t.rep.BaseSHA), joinOrNone(t.Unlisted))))
}

func auditDryRun(o opts, p prInfo, t *trial) error {
	return auditLog(auditEntry(o, p, t, deskkit.ResultDryRun, fmt.Sprintf(
		"%s would-merge parent1=%s parent2=%s disposition=%s",
		rulingID, short(t.rep.HeadSHA), short(t.rep.BaseSHA), dispositionWord(t))))
}

func dispositionWord(t *trial) string {
	if len(t.Listed) > 0 {
		return "regenerable-only(" + strings.Join(t.Listed, ",") + ")"
	}
	return "clean"
}
