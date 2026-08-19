package main

// mergecheck.go — `statusgen mergecheck`, the MERGE-TIME RE-CHECK (desk-hardening/05,
// #54, #74).
//
// THE GAP IT OCCUPIES. Review asks "is this PR correct against main?" and answers it
// against the main that existed at review time. Merge lands it in a different main.
// Nothing re-asks the question against the tree the PR actually lands in. The five
// near-misses in #54 all live in that gap, and the ones measured on 2026-08-13 show the
// shape precisely:
//
//   * statusgen/emit.go's emit() grew an 8th parameter on main while another open PR
//     still called it with 7 arguments. Both PRs were green. Neither diff mentioned the
//     other. git merged them without a conflict. main went RED on merge.
//   * docs/brief-rules.md carries duplicate rule numbers 25 and 26 for the same reason:
//     two briefs allocating out of one hand-maintained numbering space in parallel.
//
// Both are SEMANTIC merge collisions: individually valid, jointly invalid, textually
// non-conflicting. A conflict check sees NONE of them, because a conflict check answers
// "can git combine these bytes", and the question is "is the combination still correct".
//
// SO THE ONE LOAD-BEARING IDEA IS THE TREE, NOT THE PARSER: every probe here runs against
// the TRIAL-MERGED tree (`git merge-tree --write-tree`), the tree the PR lands in, and
// each merged-tree result is DIFFED against the same probe on the branch's own tree so
// the verdict can say which of the two produced the finding. That difference is what
// separates the two answers a worker needs and today cannot get:
//
//   MERGE-INTRODUCED  your branch is fine on its own and the combination is not.
//   PRE-EXISTING      it was already like that on your branch; the merge is innocent.
//
// STALE-BASE vs BROKEN, the second discriminator. Measured 2026-08-13: PR #898 added
// .github/scripts/verify-inscope.sh and a CI job that runs it, and 46 of the 52 open PR
// branches predate that commit, so their trees do not contain the script at all and the
// job exits 127. A re-check that reports that as "your branch fails CI" sends 46 workers
// hunting a phantom bug. mergecheck lists the CI-invoked files the BASE has and the HEAD
// lacks and labels it STALE-BASE — a currency fact about the branch, never a defect.
//
// EXIT CODES — the toolkit's three states, and 1 and 2 are never collapsed into "not
// zero" (the same split verifyrun.yml and verify-inscope.sh use):
//
//   0  checked-clean       every probe ran and found nothing MERGE-INTRODUCED
//   1  checked-failed      a probe ran and found a merge-introduced collision
//   2  could-not-check     a probe could NOT run — the base would not resolve, the
//                          trial merge could not be computed, git is too old, a tree
//                          could not be materialised
//
// THREE-STATE IS LOAD-BEARING HERE, not decoration. A merge-time re-check that cannot
// reach its base and answers "current" certifies stale work with a green tick. There is
// no path in this file that turns a failed git invocation into a clean verdict; every
// such path returns 2 and says which probe did not run.
//
// CURRENCY IS ADVISORY AND NEVER FAILS. Measured 2026-08-13: 52 of 52 open PR head
// branches in this repo are behind main — the median is ~200 commits, the worst 1842. A
// gate on "head is current with main" would red the entire open queue on day one, which
// is how a gate teaches people to route around it. mergecheck reports the distance and
// exits on collisions only. Promotion is a later ruling with a drained backlog behind it,
// following mergedstatus.go's precedent.
//
// WHAT THIS IS NOT, so the seams are explicit rather than discovered by collision:
//
//   * NOT a merge. It writes one tree object into the object database (merge-tree does)
//     and updates no ref, checks nothing out, and pushes nothing. Merge is always the
//     human's.
//   * NOT the desk-side merge-currency mechanic. `deskmerge` (issue-flow/09) owns
//     MAKING a PR merge-current — zero-conflict-or-regenerate, resync, the desk's
//     ordering. mergecheck owns ASKING whether the merged result is still correct, and
//     answers for a worktree it does not modify. The seam is: issue-flow/09 changes the
//     branch, desk-hardening/05 reads the combination.
//   * NOT a second Verify gate. ground-truth/02's verifyrun.yml already executes Verify
//     tables in CI and re-runs them on a schedule. mergecheck adds no workflow.
//   * NOT an approval-staleness checker. See the "approval currency" note below — the
//     signal that would carry it is not trustworthy, and that is written down rather
//     than built on.
//
// APPROVAL CURRENCY — DELIBERATELY NOT IMPLEMENTED, and why (this is a finding, not an
// omission). The obvious implementation pins "was this approval given at the head that
// merged" on the GitHub review object's `commit_id`. What is actually established, stated
// at the strength the evidence supports and no further:
//
//   * ONE observation (PR #881, 2026-08-13): a review's `commit_id` reported the current
//     head while that same review's own body described reading an earlier commit. Two
//     probes three seconds apart returned the same thing, so that single disagreement is
//     not per-request noise.
//   * A 10-PR sweep taken afterwards (recorded on PR #940) did NOT establish a direction:
//     4 of the 5 sampled reviews had `commit_id` equal to head with no post-review push,
//     which is consistent with the field being CORRECT, and PR #750's `commit_id`
//     correctly LAGGED head — the field reporting staleness accurately. No review body in
//     the sweep names a SHA, so there is no cross-check to measure a direction with.
//
// So the honest statement is: `commit_id` has been observed to disagree with the head
// named in a review body ONCE, and the direction and frequency of that disagreement are
// could-not-check. It is NOT established that the field under-reports staleness, and it
// is NOT established that it fails open. That is precisely why nothing here is built on
// it: a signal whose error direction is unknown cannot be trusted in EITHER direction,
// and a merge-time claim resting on it would inherit an unmeasured error. This tool
// reports could-not-check for approval currency rather than a number it cannot
// characterise.
//
// Separately and independently established: any resync push invalidates approvals
// outright, so "approval is stale" without a WHY is noise a reviewer cannot act on — two
// PRs lost approvals on 2026-08-13 purely as the cost of becoming mergeable.

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// remoteMainRef is the FULLY-QUALIFIED remote-tracking ref for main, spelled out
// rather than the bare short name `origin/main` (issue #885). A bare `origin/main`
// is not equivalent in this repo: a stale local branch literally named
// `refs/heads/origin/main` exists, so `origin/main` resolves through refs/heads
// first and lands dozens of commits behind the real remote tip — silently, at exit
// 0. Every statusgen git resolution of main's tip/merge-base spells this form so a
// stray decoy cannot shadow it (grandfathering, tombstone/field-gutting bases, the
// UNRUN done-gate, and the --consumers diff base all key on the real remote tip).
const remoteMainRef = "refs/remotes/origin/main"

// defaultMergecheckBase is spelled in FULL and must stay that way: a merge-time
// re-check computed against the wrong base is worse than no re-check — it certifies
// the merge against a tree nobody is merging into. See remoteMainRef (#885).
const defaultMergecheckBase = remoteMainRef

// ciInvokedPrefixes are the tree prefixes whose contents CI invokes by path. A file the
// base has here and the head lacks is the exit-127 shape: the workflow calls a script
// the branch's tree does not contain. Not a general "files added on main" list — that
// would be every merge — but specifically the paths whose ABSENCE produces a CI failure
// that reads as a defect in the branch.
var ciInvokedPrefixes = []string{".github/scripts/", ".github/actions/", ".github/workflows/"}

// blindSpots is printed on EVERY run, clean or not. A check that reports "clean" without
// stating what it did not look at is read as "nothing is wrong", and the collision shapes
// below are exactly the ones that produced the incidents this tool exists for. No silent
// caps: if a shape is not on this list and not probed, that is a bug in this list.
var blindSpots = []string{
	"TYPE- AND SIGNATURE-LEVEL collisions (the emit() arity case) unless --exec runs a build over the merged tree. Without --exec, nothing here compiles anything.",
	"BEHAVIOURAL collisions — two changes that compile and contradict each other at runtime. No probe here executes the merged program's logic beyond whatever --exec runs.",
	"SEMANTIC collisions in any hand-maintained namespace other than docs/brief-rules.md rule numbers: issue/finding IDs, board rows, config keys, generated-artifact contents.",
	"Collisions with a PR that is OPEN but NOT YET MERGED. The trial merge is against the base ref only; two open PRs that collide with each other are invisible until the first one lands.",
	"APPROVAL currency — see the file header. The available signal (review commit_id) has been observed once to disagree with the head named in a review body, and the direction and frequency of that error are unmeasured, so it is not a sound staleness signal in either direction and is reported as could-not-check rather than measured.",
	"PR-BODY and Verify-table claims contradicted by the diff (#74). That is a reviewer-discipline rule here (docs/brief-rules.md, pr-review-desk SKILL.md), not a mechanised probe: deciding whether a body's prose still describes a diff is a judgement.",
	"Anything the base ref does not yet contain. `mergecheck` is only as current as the last fetch; run with --fetch, or fetch first, or the answer is about yesterday's main.",
}

// gitOut runs a git command in root and returns trimmed stdout, or an error carrying
// stderr. Every git failure in this file becomes could-not-check, never a clean verdict.
func gitOut(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// resolveCommit resolves a ref to a commit sha, or errors.
func resolveCommit(root, ref string) (string, error) {
	return gitOut(root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
}

// mergeTreeResult is the outcome of the trial merge.
type mergeTreeResult struct {
	Tree      string   // merged tree OID (empty when conflicted)
	Conflicts []string // conflicted paths, when git reported conflicts
}

// trialMerge computes the merged tree WITHOUT touching the worktree, the index, or any
// ref. `--write-tree` needs git >= 2.38; an older git is could-not-check, never "clean".
func trialMerge(root, base, head string) (mergeTreeResult, error) {
	cmd := exec.Command("git", "-C", root, "merge-tree", "--write-tree", "--name-only", base, head)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		// Clean merge: stdout is the tree OID (and nothing that matters after it).
		if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
			return mergeTreeResult{}, errors.New("git merge-tree reported success but printed no tree oid")
		}
		return mergeTreeResult{Tree: strings.TrimSpace(lines[0])}, nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		// Conflicts. Line 0 is the tree oid of the conflicted merge; the block after
		// the first blank line is informational, the block before it is the file list.
		var conflicts []string
		for _, ln := range lines[1:] {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				break
			}
			conflicts = append(conflicts, ln)
		}
		return mergeTreeResult{Tree: strings.TrimSpace(lines[0]), Conflicts: conflicts}, nil
	default:
		msg := strings.TrimSpace(errb.String())
		if msg == "" && err != nil {
			msg = err.Error()
		}
		return mergeTreeResult{}, fmt.Errorf("git merge-tree --write-tree failed (git >= 2.38 required): %s", msg)
	}
}

// treeFiles lists every path in a tree.
func treeFiles(root, tree string) (map[string]bool, error) {
	out, err := gitOut(root, "ls-tree", "-r", "--name-only", tree)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, ln := range strings.Split(out, "\n") {
		if ln != "" {
			set[ln] = true
		}
	}
	return set, nil
}

// staleBaseFiles are the CI-invoked paths the base carries and the head does not. Sorted
// so the report is stable run to run.
func staleBaseFiles(baseFiles, headFiles map[string]bool) []string {
	var out []string
	for p := range baseFiles {
		if headFiles[p] {
			continue
		}
		for _, pre := range ciInvokedPrefixes {
			if strings.HasPrefix(p, pre) {
				out = append(out, p)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// fileInTree reads one path out of a tree. A missing path is ("", false, nil): not an
// error, because most trees legitimately lack any given file.
func fileInTree(root, tree, path string) (string, bool, error) {
	cmd := exec.Command("git", "-C", root, "cat-file", "-p", tree+":"+path)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), "does not exist") || strings.Contains(errb.String(), "exists on disk, but not in") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("git cat-file %s:%s: %s", tree, path, strings.TrimSpace(errb.String()))
	}
	return out.String(), true, nil
}

// materializeTree extracts a tree into dir so a command can be run over it. Used only by
// --exec; without it nothing is ever written outside the object database.
func materializeTree(root, tree, dir string) error {
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", tree)
	untar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	untar.Stdin = pipe
	var errb bytes.Buffer
	archive.Stderr = &errb
	untar.Stderr = &errb
	if err := untar.Start(); err != nil {
		return err
	}
	if err := archive.Start(); err != nil {
		return err
	}
	if err := archive.Wait(); err != nil {
		untar.Wait()
		return fmt.Errorf("git archive %s: %s", tree, strings.TrimSpace(errb.String()))
	}
	pipe.Close()
	if err := untar.Wait(); err != nil {
		return fmt.Errorf("tar -x: %s", strings.TrimSpace(errb.String()))
	}
	return nil
}

// execOverTree runs cmdline with `sh -c` inside a fresh extraction of tree. Returns the
// exit code and the combined output.
//
// SECURITY. This executes a command the OPERATOR typed on their own command line, over a
// tree they are already merging into their own branch. That is a different exposure from
// verifyrun's (shell lifted out of a markdown file an author controls): there is no
// attacker-authored string on this path. It is still opt-in and never reachable from
// --lint, which is offline and side-effect-free.
func execOverTree(root, tree, cmdline string) (int, string, error) {
	dir, err := os.MkdirTemp("", "statusgen-mergecheck-")
	if err != nil {
		return 0, "", err
	}
	defer os.RemoveAll(dir)
	if err := materializeTree(root, tree, dir); err != nil {
		return 0, "", err
	}
	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), string(out), nil
		}
		return 0, string(out), err
	}
	return 0, string(out), nil
}

// runMergecheck is the subcommand entry point. It returns the process exit code.
func runMergecheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mergecheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root to check")
	base := fs.String("base", defaultMergecheckBase, "base ref the branch will merge into — spell it in full (see defaultMergecheckBase)")
	head := fs.String("head", "HEAD", "the branch head to re-check")
	execCmd := fs.String("exec", "", "command to run over the trial-merged tree (e.g. \"cd statusgen && go build ./...\"); without it, compile-level collisions are invisible")
	doFetch := fs.Bool("fetch", false, "run `git fetch origin` first — otherwise the base is only as current as the last fetch")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	p := func(format string, a ...any) { fmt.Fprintf(stdout, format+"\n", a...) }

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: --root %q could not be resolved: %v\n", *root, err)
		return 2
	}

	if *doFetch {
		if _, err := gitOut(absRoot, "fetch", "--quiet", "origin"); err != nil {
			fmt.Fprintf(stderr, "could-not-check: --fetch failed: %v\n"+
				"The base ref would be stale and the whole re-check would be about an old main. Nothing was checked.\n", err)
			return 2
		}
	}

	headSHA, err := resolveCommit(absRoot, *head)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: the head ref %q would not resolve: %v\nNothing was checked.\n", *head, err)
		return 2
	}
	baseSHA, err := resolveCommit(absRoot, *base)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: the base ref %q would not resolve: %v\n"+
			"This is the fail-open case the check exists to refuse: an unreachable base must never render as \"current\". Nothing was checked.\n", *base, err)
		return 2
	}

	p("statusgen mergecheck — merge-time re-check")
	p("  head: %s (%s)", headSHA, *head)
	p("  base: %s (%s)", baseSHA, *base)

	mergeBase, err := gitOut(absRoot, "merge-base", baseSHA, headSHA)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: no merge-base between %s and %s: %v\n", *base, *head, err)
		return 2
	}
	p("  merge-base: %s", mergeBase)

	behindStr, err := gitOut(absRoot, "rev-list", "--count", headSHA+".."+baseSHA)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: the currency distance could not be counted: %v\n", err)
		return 2
	}
	behind, _ := strconv.Atoi(behindStr)
	p("  currency: head is %d commit(s) behind the base — ADVISORY, this never fails the check.", behind)
	p("            (measured 2026-08-13: 52 of 52 open PRs in this repo are behind main; a gate here reds the whole queue)")
	p("")

	failed := []string{}  // merge-introduced findings → exit 1
	notices := []string{} // pre-existing / advisory → reported, never fails
	unknown := []string{} // could-not-check → exit 2, dominates

	// ── Probe 1: the trial merge itself.
	tm, err := trialMerge(absRoot, baseSHA, headSHA)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: the trial merge could not be computed: %v\n"+
			"No conclusion is drawn about this merge — this is the absence of a check, not a pass.\n", err)
		return 2
	}
	if len(tm.Conflicts) > 0 {
		failed = append(failed, fmt.Sprintf("TEXTUAL CONFLICT in %d path(s): %s", len(tm.Conflicts), strings.Join(tm.Conflicts, ", ")))
	} else {
		p("checked-clean: the trial merge is textually clean (merged tree %s).", tm.Tree)
		p("  This is the WEAKEST result in this report and the one most often over-read. Every")
		p("  incident in #54 merged cleanly. \"No conflict\" means git could combine the bytes.")
	}

	// ── Probe 2: stale base — the CI-invoked files the base has and the head lacks.
	baseFiles, err := treeFiles(absRoot, baseSHA)
	if err != nil {
		unknown = append(unknown, fmt.Sprintf("stale-base probe: the base tree could not be listed: %v", err))
	}
	headFiles, err := treeFiles(absRoot, headSHA)
	if err != nil {
		unknown = append(unknown, fmt.Sprintf("stale-base probe: the head tree could not be listed: %v", err))
	}
	if baseFiles != nil && headFiles != nil {
		missing := staleBaseFiles(baseFiles, headFiles)
		if len(missing) > 0 {
			notices = append(notices, fmt.Sprintf(
				"STALE-BASE: the base carries %d CI-invoked path(s) your head does not have — %s.\n"+
					"    Your branch is STALE here, not BROKEN. A CI job that calls one of these against your\n"+
					"    tree fails with exit 127 (file not found) and the failure is about currency, not your\n"+
					"    work. Measured 2026-08-13: 46 of 52 open PRs predate .github/scripts/verify-inscope.sh\n"+
					"    alone. Resync before you investigate the failure.",
				len(missing), strings.Join(missing, ", ")))
		} else {
			p("checked-clean: no CI-invoked path exists on the base and is missing from your head (no stale-base exit-127 class).")
		}
	}

	// ── Probe 3: the numbering-space collision, read off the MERGED tree.
	// This is the probe that can see what neither branch's own diff shows.
	mergedRules, mergedOK, err := fileInTree(absRoot, tm.Tree, briefRulesRelPath)
	if err != nil {
		unknown = append(unknown, fmt.Sprintf("numbering-space probe: %s could not be read from the merged tree: %v", briefRulesRelPath, err))
	} else if !mergedOK {
		p("checked-clean: %s is not present in the merged tree — numbering-space probe not applicable.", briefRulesRelPath)
	} else {
		headRules, headOK, herr := fileInTree(absRoot, headSHA, briefRulesRelPath)
		if herr != nil {
			unknown = append(unknown, fmt.Sprintf("numbering-space probe: %s could not be read from the head tree, so merge-introduced and pre-existing cannot be told apart: %v", briefRulesRelPath, herr))
		} else {
			mergedFindings := numberSpaceCollisions(mergedRules)
			preExisting := map[string]bool{}
			if headOK {
				for _, f := range numberSpaceCollisions(headRules) {
					preExisting[f] = true
				}
			}
			introduced := 0
			for _, f := range mergedFindings {
				if preExisting[f] {
					notices = append(notices, "PRE-EXISTING numbering collision (already on your branch, the merge did not create it): "+f)
					continue
				}
				introduced++
				failed = append(failed, "MERGE-INTRODUCED numbering collision — invisible in either diff alone, present in the tree you would land: "+f)
			}
			if len(mergedFindings) == 0 {
				p("checked-clean: no rule-number collision in the merged copy of %s.", briefRulesRelPath)
			} else if introduced == 0 {
				p("checked-failed(pre-existing): %d numbering collision(s) in the merged tree, all already present on your head.", len(mergedFindings))
			}
		}
	}

	// ── Probe 4: --exec over the merged tree, with the same merge-introduced /
	// pre-existing split. This is the only probe that can see a compile-level
	// collision such as the emit() arity case.
	if *execCmd == "" {
		notices = append(notices, "no --exec given: NOTHING WAS COMPILED. The emit() arity class of collision is not covered by this run.")
	} else {
		code, out, xerr := execOverTree(absRoot, tm.Tree, *execCmd)
		if xerr != nil {
			unknown = append(unknown, fmt.Sprintf("--exec probe: the merged tree could not be materialised or run: %v", xerr))
		} else if code == 0 {
			p("checked-clean: `%s` exits 0 over the merged tree.", *execCmd)
		} else {
			headTree, terr := gitOut(absRoot, "rev-parse", headSHA+"^{tree}")
			if terr != nil {
				unknown = append(unknown, fmt.Sprintf("--exec probe: the command failed over the merged tree (exit %d) but the head tree could not be resolved, so merge-introduced and pre-existing cannot be told apart: %v", code, terr))
			} else {
				headCode, _, herr := execOverTree(absRoot, headTree, *execCmd)
				switch {
				case herr != nil:
					unknown = append(unknown, fmt.Sprintf("--exec probe: the command failed over the merged tree (exit %d) but could not be run over the head tree, so the two cannot be told apart: %v", code, herr))
				case headCode == 0:
					failed = append(failed, fmt.Sprintf(
						"MERGE-INTRODUCED: `%s` exits 0 on your head and %d over the merged tree.\n"+
							"    Your branch is correct and the COMBINATION is not — this is the emit() arity class.\n"+
							"    Output:\n%s", *execCmd, code, indent(out, "      ")))
				default:
					notices = append(notices, fmt.Sprintf(
						"PRE-EXISTING: `%s` exits %d over the merged tree and %d on your head alone. The merge is not the cause.",
						*execCmd, code, headCode))
				}
			}
		}
	}

	// ── Report.
	p("")
	for _, n := range notices {
		p("notice: %s", n)
	}
	for _, u := range unknown {
		p("could-not-check: %s", u)
	}
	for _, f := range failed {
		p("checked-failed: %s", f)
	}

	p("")
	p("Collision shapes this check CANNOT see — read them before treating a clean run as safe:")
	for _, b := range blindSpots {
		p("  · %s", b)
	}
	p("")
	p("mergecheck does not merge, flip, approve, or push. Merge is always the human's.")

	// 2 dominates 1: "I could not establish this" has to be repaired before a failure
	// count means anything. Same precedence as verifyrun.yml's gate.
	if len(unknown) > 0 {
		p("")
		p("VERDICT: could-not-check — %d probe(s) did not run. This is NOT a pass.", len(unknown))
		return 2
	}
	if len(failed) > 0 {
		p("")
		p("VERDICT: checked-failed — %d merge-introduced finding(s).", len(failed))
		return 1
	}
	p("")
	p("VERDICT: checked-clean for the probes listed above, and only those.")
	return 0
}

// indent prefixes every line of s, for embedding command output in a finding.
func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
