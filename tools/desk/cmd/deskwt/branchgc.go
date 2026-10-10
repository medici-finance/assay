package main

// branchgc.go — prune's LOCAL-BRANCH GC pass (Step C).
//
// The defect this pass exists for: `deskwt prune` reaps worktrees and the forge reaps
// remote branches on merge, but nothing reaps the LOCAL branches a worktree leaves behind
// (git worktree remove does not delete the branch it was checked out on). They accumulate
// without bound — thousands per stream-root checkout, the overwhelming majority of them
// proven residue: work that is already on the remote mainline, kept only because no tool
// ever asked the question.
//
// The pass collects a branch ONLY when its content is provably on refs/remotes/origin/main,
// by one of two proofs:
//
//	merged — the branch tip is an ancestor of the remote mainline (every commit it ever
//	  carried is reachable from main). This is the same question Step B asks of a
//	  worktree's HEAD, answered from the sweep's ONE already-walked ancestor set, and it
//	  covers the "fresh handle, never used" case exactly: a branch zero commits ahead of
//	  its fork point has its tip == merge-base(tip, main), which is precisely "tip is an
//	  ancestor of main" — no separate check can distinguish it, so none is made.
//	cherry-clean — the squash-merge case: the tip is NOT an ancestor, but every unique
//	  commit's patch-id is already present in the mainline's history, so each change the
//	  branch made landed under a different commit sha. (See the patch-id note below for
//	  how this differs from `git cherry` and why.)
//
// The NEVER list is absolute: `main` and `master` are never candidates; a branch checked
// out in ANY registered worktree is never a candidate (git would refuse the delete anyway —
// a refusal is treated as a skip, not an error); a branch carrying at least one patch the
// mainline lacks is KEPT, always — this GC targets proven residue, not judgement calls; and
// any branch whose proof cannot be COMPLETED (an unreadable mainline, an uncountable patch
// set, a delete that fails) is kept and reported, because an unreadable gate is not a
// passed one.
//
// The delete is `git update-ref -d <ref> <sha-at-proof>` — a compare-and-delete against the
// sha the proof was taken on, so a branch that moved between proof and delete is left alone.
// It is deliberately NOT `git branch -d` (that refuses the cherry-clean class it just proved
// safe, because -d asks about ancestry and a squash-merged branch is not an ancestor) and
// NEVER `-D`: this tool has no force verb anywhere, and the tool's OWN proof is the safety
// gate, exactly as on the add-collision reclaim path (branchcollision.go).
//
// Patch-id proof, and why not `git cherry`. `git cherry origin/main <branch>` answers the
// right question but pays for it per branch: it computes patch-ids over the mainline
// history SINCE THE FORK for every candidate, so a checkout carrying thousands of
// never-merged branches re-diffs the same mainline thousands of times per sweep. This pass
// computes the mainline's patch-id set ONCE per sweep (`git log -p --no-merges
// refs/remotes/origin/main | git patch-id --stable`, lazily, only when a cherry candidate
// exists) and then diffs only each candidate's OWN unique commits against it. The one
// semantic difference from `git cherry` is deliberate and in the safe direction for the
// stated invariant: the comparison set is the WHOLE mainline history rather than
// since-the-fork, so a branch whose commit re-applies a change main landed before the fork
// point reads as clean — the patch IS provably in origin/main, which is exactly the
// content-preservation bar the pass holds itself to.
//
// Two fail-closed guards sit inside the cherry proof:
//
//   - a branch with a MERGE commit ahead of the mainline is held: merge commits have no
//     patch-id, so a patch-id proof cannot see content a merge introduced (an evil merge,
//     or a merge whose conflict resolution added content). One `git rev-list --parents`
//     per candidate answers "any merges ahead?" and lists the non-merge commits in the
//     same call.
//   - the branch-side patch-id count must EQUAL its unique non-merge commit count. A
//     commit whose diff yields no patch-id (an empty commit, an unparseable hunk) breaks
//     the equality and the branch is held: "every commit's patch-id is present" cannot be
//     proven for a commit whose patch-id could not be computed.
//
// Output discipline follows the desk noise-floor contract: a live sweep prints COUNTS per
// class on the one summary line (and any warnings); the per-branch plan — every candidate,
// COLLECT and KEEP alike, with the reason — is printed only by --dry-run, which deletes
// nothing.

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mainlineRef is the remote mainline, spelled FULLY QUALIFIED everywhere for the reason
// issue #885 gives at length: refs/heads/ wins gitrevisions disambiguation, so the bare
// short name `origin/main` can silently resolve to a stray local decoy branch.
const mainlineRef = "refs/remotes/origin/main"

// branchGCVerdict is the pass's judgement for ONE branch, and the unit the --dry-run plan
// is printed from. reason is filled in BOTH directions — a KEEP with no reason is
// indistinguishable from a branch the sweep never looked at.
type branchGCVerdict struct {
	name    string
	collect bool
	class   string // "merged" or "cherry-clean" — the proof that made it collectible
	reason  string
}

// planLine is one row of the --dry-run plan: verdict, branch, why.
func (v branchGCVerdict) planLine() string {
	verdict := "KEEP"
	if v.collect {
		verdict = "COLLECT"
	}
	return fmt.Sprintf("  %s branch %s — %s", verdict, v.name, v.reason)
}

// branchGCResult is the branch-GC pass's half of a sweep result. The counts are per class
// by design: the summary line is how an operator tells a draining refs store from a stuck
// one, and a single merged number could not say WHICH proof is doing the work.
type branchGCResult struct {
	ran          bool // false when --no-branches skipped the pass — a zero count would mislead
	only         bool // --branches-only: the worktree arms did not run this sweep
	merged       int  // collected: tip an ancestor of the remote mainline
	cherry       int  // collected: every unique commit's patch-id already on the mainline
	heldUnique   int  // kept: at least one patch the mainline does not have
	heldUnproven int  // kept: a proof could not be completed (fail closed)
	protected    int  // never candidates: main / master
	checkedOut   int  // never candidates: held by a registered worktree
	deleted      []string
	plan         []branchGCVerdict
	warns        []string
}

// collected is the total number of branches the pass dropped (or, under --dry-run, would
// drop) — the number the noop verdict and the summary headline need.
func (r branchGCResult) collected() int { return r.merged + r.cherry }

// held is every branch the pass looked at and KEPT, for whatever reason.
func (r branchGCResult) held() int {
	return r.heldUnique + r.heldUnproven + r.protected + r.checkedOut
}

// localBranch is one refs/heads/ entry: full ref, short name, and the sha it pointed at
// when enumerated (the sha every proof — and the compare-and-delete — is taken against).
type localBranch struct {
	ref  string
	name string
	sha  string
}

// listLocalBranches enumerates every local branch in ONE git call. A checkout that needs
// this pass carries thousands of them; per-branch resolution would be a process per ref.
// The enumerate runs through the package's single argv seam like every other git call.
func listLocalBranches(dir string) ([]localBranch, error) {
	out, err := runGit(dir, "for-each-ref", "--format=%(refname)%09%(objectname)", "refs/heads/")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot enumerate local branches (git for-each-ref refs/heads/ failed)", err)
	}
	var branches []localBranch
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ref, sha, _ := strings.Cut(line, "\t")
		branches = append(branches, localBranch{
			ref:  ref,
			name: strings.TrimPrefix(ref, "refs/heads/"),
			sha:  sha,
		})
	}
	return branches, nil
}

// branchCommitsAhead lists the commits in <mainline>..<ref> with their parents in ONE call
// (`git rev-list --parents`), returning the non-merge commit shas and whether ANY merge
// commit is ahead. The merge answer is a gate, not a statistic: a merge commit carries no
// patch-id, so a patch-id proof over the non-merge commits says nothing about content a
// merge introduced — a branch with one ahead is held, however clean its other commits are.
func branchCommitsAhead(dir, ref string) (nonMerge []string, hasMerge bool, err error) {
	out, err := runGit(dir, "rev-list", "--parents", mainlineRef+".."+ref)
	if err != nil {
		return nil, false, deskkit.Unverifiable(
			"cannot list "+ref+"'s commits ahead of "+mainlineRef, err)
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) > 2 { // commit + two or more parents = a merge
			hasMerge = true
			continue
		}
		nonMerge = append(nonMerge, fields[0])
	}
	return nonMerge, hasMerge, nil
}

// patchIDsForRange computes the set of patch-ids of every non-merge commit in <rangeSpec>
// by piping `git log -p` into `git patch-id --stable` — WITHOUT a shell: the two processes
// are joined by an OS pipe directly, so no shell string and no intermediate file ever
// exists. Both processes go through the package's single argv seam, so the recording tests
// (and the no-force-flag assertion) see them exactly as they see every other git call.
//
// --stable is spelled explicitly on every call: patch-id's default stability changed across
// git versions, and a set built under one algorithm compared against ids computed under the
// other would read every patch as absent — the fail-SAFE direction here (everything held),
// but a silently inert proof is still a defect, so the algorithm is pinned.
//
// The `--format=commit %H` line is LOAD-BEARING, not cosmetic. git patch-id ends one patch
// and starts the next at a `commit <sha>` line, not at a `diff --git` line: with an empty
// format the diffs of every commit in the range are summed into ONE patch-id, and the
// count check below (ids must equal the commit count) turns a formatting slip into every
// branch being held. This was found by a test, not by reading git's source — do not
// "simplify" the format away.
func patchIDsForRange(dir, rangeSpec string) (map[string]struct{}, error) {
	logCmd := execCommand("git", "log", "--no-merges", "-p", "--format=commit %H", rangeSpec)
	logCmd.Dir = dir
	idCmd := execCommand("git", "patch-id", "--stable")
	idCmd.Dir = dir

	pr, pw := io.Pipe()
	logCmd.Stdout = pw
	idCmd.Stdin = pr
	var idOut, logErrBuf, idErrBuf bytes.Buffer
	idCmd.Stdout = &idOut
	logCmd.Stderr = &logErrBuf
	idCmd.Stderr = &idErrBuf

	// The consumer starts first: writes into an io.Pipe block until a reader takes them,
	// and patch-id's stdin-copy goroutine is that reader.
	if err := idCmd.Start(); err != nil {
		return nil, deskkit.Unverifiable("cannot start git patch-id", err)
	}
	if err := logCmd.Start(); err != nil {
		_ = pr.Close()
		_ = idCmd.Wait()
		return nil, deskkit.Unverifiable("cannot start git log for the patch-id pass", err)
	}
	logErr := logCmd.Wait()
	// CloseWithError hands the consumer the producer's failure as its read error, so a
	// failed log can never masquerade as a complete (truncated) diff stream.
	_ = pw.CloseWithError(logErr)
	idErr := idCmd.Wait()
	if logErr != nil {
		return nil, deskkit.Unverifiable("git log for the patch-id pass failed: "+
			strings.TrimSpace(logErrBuf.String()), logErr)
	}
	if idErr != nil {
		return nil, deskkit.Unverifiable("git patch-id failed: "+
			strings.TrimSpace(idErrBuf.String()), idErr)
	}

	ids := make(map[string]struct{})
	for _, line := range strings.Split(idOut.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ids[fields[0]] = struct{}{}
	}
	return ids, nil
}

// deleteProvenBranch drops a local branch whose content was JUST proven redundant, by
// compare-and-delete: `git update-ref -d <ref> <sha-at-proof>`. See the file header for why
// this is neither `git branch -d` (refuses the cherry-clean class it just proved safe) nor
// ever `-D` (no force verb exists in this tool).
func deleteProvenBranch(dir, ref, sha string) error {
	if _, err := runGit(dir, "update-ref", "-d", ref, sha); err != nil {
		return deskkit.Unverifiable("git update-ref -d "+ref+" failed (the branch may have "+
			"moved since its proof, or been checked out)", err)
	}
	return nil
}

// gcLocalBranches is the whole pass: enumerate, classify, delete the proven. It deletes
// nothing under dryRun and never fails the sweep — a branch it cannot prove safe is KEPT,
// a branch it cannot delete is a warning, and a mainline it cannot read holds everything.
// sc supplies the sweep's ONE already-walked mainline ancestor set; when that set could not
// be built the pass holds every branch (fail closed, said out loud once).
func gcLocalBranches(sc *sweepCtx, dir string, dryRun bool) branchGCResult {
	res := branchGCResult{ran: true}

	branches, err := listLocalBranches(dir)
	if err != nil {
		res.warns = append(res.warns, "could-not-check: "+errText(err)+
			" — no branch can be classified, so the branch GC collected nothing this sweep")
		return res
	}
	// The holder set is the checked-out guard: a branch any registered worktree has checked
	// out is never a candidate. A failure to read it fails CLOSED for the whole pass — an
	// unknown holder set is never read as "nobody holds anything", because that reading is
	// the one that would delete a branch out from under a live worktree.
	holders, herr := branchHolders(dir)
	if herr != nil {
		res.warns = append(res.warns, "could-not-check: "+errText(herr)+
			" — which branches are checked out is unknown, so the branch GC collected nothing this sweep")
		return res
	}
	if sc.mainAncestors == nil {
		res.warns = append(res.warns,
			"could-not-check: "+mainlineRef+" history unreadable, so no branch can be proven merged "+
				"— every branch is held and the branch GC collected nothing this sweep: "+errText(sc.ancestorErr))
		return res
	}

	// The mainline patch-id set is built LAZILY, on the first cherry candidate: a sweep
	// whose branches are all ancestry-merged (the drained steady state) never pays for it.
	var upstreamIDs map[string]struct{}
	upstreamErr := error(nil)
	upstreamTried := false

	for _, b := range branches {
		if b.name == "main" || b.name == "master" {
			res.protected++
			res.plan = append(res.plan, branchGCVerdict{name: b.name,
				reason: "protected name (main/master are never branch-GC candidates)"})
			continue
		}
		if wt, held := holders[b.ref]; held {
			res.checkedOut++
			res.plan = append(res.plan, branchGCVerdict{name: b.name,
				reason: "checked out in worktree " + wt})
			continue
		}

		class := ""
		if _, ok := sc.mainAncestors[b.sha]; ok {
			class = "merged"
		} else {
			// Cherry-clean proof: squash-merge-visible residue. Any merge commit ahead
			// holds the branch outright — patch-ids cannot see what a merge introduced.
			nonMerge, hasMerge, lerr := branchCommitsAhead(dir, b.ref)
			if lerr != nil {
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "could-not-check: " + errText(lerr)})
				continue
			}
			switch {
			case hasMerge:
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "merge commit(s) ahead of the mainline — a patch-id proof cannot see " +
						"merge-introduced content, so the branch is kept"})
				continue
			case len(nonMerge) == 0:
				// Unreachable in principle (a branch with NOTHING ahead is an ancestor and
				// took the merged arm); held rather than collected, because an empty proof
				// set proving anything would be a bug, not a case.
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "could-not-check: no commits ahead of the mainline yet not an ancestor of it"})
				continue
			}
			if !upstreamTried {
				upstreamTried = true
				upstreamIDs, upstreamErr = patchIDsForRange(dir, mainlineRef)
				if upstreamErr != nil {
					res.warns = append(res.warns, "could-not-check: the mainline patch-id set could "+
						"not be built, so no branch can be proven cherry-clean this sweep: "+errText(upstreamErr))
				}
			}
			if upstreamErr != nil {
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "could-not-check: the mainline patch-id set is unavailable"})
				continue
			}
			branchIDs, perr := patchIDsForRange(dir, mainlineRef+".."+b.ref)
			if perr != nil {
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "could-not-check: " + errText(perr)})
				continue
			}
			if len(branchIDs) != len(nonMerge) {
				res.heldUnproven++
				res.plan = append(res.plan, branchGCVerdict{name: b.name, reason: fmt.Sprintf(
					"could-not-check: %d unique commit(s) ahead but only %d patch-id(s) computed — "+
						"a commit whose patch-id cannot be computed cannot be proven redundant",
					len(nonMerge), len(branchIDs))})
				continue
			}
			clean := true
			for id := range branchIDs {
				if _, ok := upstreamIDs[id]; !ok {
					clean = false
					break
				}
			}
			if !clean {
				res.heldUnique++
				res.plan = append(res.plan, branchGCVerdict{name: b.name,
					reason: "unique patch(es) the mainline does not have — kept, always"})
				continue
			}
			class = "cherry-clean"
		}

		// Proven redundant by `class`. --dry-run stops here exactly as the worktree arms
		// do: record the verdict, delete nothing.
		if dryRun {
			switch class {
			case "merged":
				res.merged++
			default:
				res.cherry++
			}
			res.plan = append(res.plan, branchGCVerdict{name: b.name, collect: true, class: class,
				reason: class + " [dry-run: not deleted]"})
			continue
		}
		if derr := deleteProvenBranch(dir, b.ref, b.sha); derr != nil {
			// A skip, never a sweep failure: the branch stays, the warning says why.
			res.heldUnproven++
			res.warns = append(res.warns, "branch "+b.name+" was proven "+class+
				" but its ref could not be deleted (left in place): "+errText(derr))
			res.plan = append(res.plan, branchGCVerdict{name: b.name,
				reason: "proven " + class + " but the delete failed: " + errText(derr)})
			continue
		}
		switch class {
		case "merged":
			res.merged++
		default:
			res.cherry++
		}
		res.deleted = append(res.deleted, b.name)
		res.plan = append(res.plan, branchGCVerdict{name: b.name, collect: true, class: class,
			reason: class})
	}
	return res
}
