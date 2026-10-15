package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisiongate.go — the decision-gate hold's network layer on an evidence landing
// (spec/lifecycle-v1.md §4.5).
//
// A gate: human brief moves to implemented, verified or done only once a human has
// ruled on its decision issue. The offline layer of that hold is part of
// `statusgen --lint`, so the PROBLEM-diff guard (lintdiff.go) already refuses a
// landing that adds a status move with no well-formed ruling: link. It cannot tell
// who wrote the linked comment. An evidence landing rides the Contents API straight
// to the default branch, with no pull request, so the pull-request check that reads
// the comment's author never sees it. This file runs that check here instead:
// `statusgen --decision-gate` before and after staging the write, refusing only the
// REFUSED lines the write adds.
//
// Same shape and the same rules as lintdiff.go: the stage is always reverted (the
// real write rides the Contents API); statusgen not on PATH, or one too old to know
// --decision-gate, is could-not-check, never a silent pass.

// decisionGateDiffFn is the seam cmdEvidence calls. setupFake stubs it to
// "introduces nothing"; decisiongate_test.go drives decisionGateDiffAt directly.
var decisionGateDiffFn = decisionGateDiffAt

// statusgenDecisionGateFn is ONE `statusgen --decision-gate` shell, judging the
// working tree at root against the commit base and returning the REFUSED lines it
// printed. Production: statusgenDecisionGateAt.
var statusgenDecisionGateFn = statusgenDecisionGateAt

// decisionGateReplacedBaseFn builds the commit the landing is judged against: the
// worktree's HEAD with targetRepoPath holding the content the write REPLACES. A seam
// so a test can pin what the gate is given; production: replacedBaseCommit.
var decisionGateReplacedBaseFn = replacedBaseCommit

// decisionGateDiffAt judges one pending write against the content it REPLACES, never
// against whatever the landing worktree happens to hold.
//
// The write replaces the target's content on the remote branch (replaced, present
// when replacedExists). The landing worktree may already carry the very edit being
// landed — the documented direct-write flow edits the tracked file in place and then
// lands it — uncommitted, or even committed locally. Judging "the tree as it stands"
// against "the tree with the write staged" then compares two copies of the same bytes
// and introduces nothing, so an unruled gate: human move would land with exit 0.
//
// So both runs share one base: the worktree's HEAD with the target replaced by the
// content the write replaces (a dangling commit; no ref moves). The "before" run
// stages the replaced content over the target, the "after" run stages newContent, and
// only the REFUSED lines present after and absent before are this landing's. Lines
// both runs print come from other, uncommitted edits in the worktree, not from this
// write. The stage is always reverted.
//
// board is the target branch's version of every board file it changed since the home's
// commit (decisionGateBranchBoard): applied to the base and to the worktree of both
// runs, so the judge reads the board the write lands on, not the home's older one.
func decisionGateDiffAt(root, targetRepoPath string, replaced []byte, replacedExists bool, newContent []byte, board []boardEdit) (introduced []string, err error) {
	base, err := decisionGateReplacedBaseFn(root, targetRepoPath, replaced, replacedExists, board)
	if err != nil {
		return nil, err
	}
	// Every path this stages must be a plain file (or absent) under real directories in
	// the home: a link there would carry the staged bytes through to whatever it names,
	// and a non-file entry is not the record the board reads (S-2266-7). stageBoardEdits
	// holds the board's paths to the same rule before it writes any of them.
	if lerr := refuseLinkedHomePath(root, targetRepoPath); lerr != nil {
		return nil, lerr
	}
	abs := filepath.Join(root, filepath.FromSlash(targetRepoPath))
	orig, rerr := os.ReadFile(abs)
	existed := rerr == nil
	if rerr != nil && !os.IsNotExist(rerr) {
		return nil, deskkit.Unverifiable("cannot read local "+abs+" to stage the decision-gate check", rerr)
	}
	created, merr := mkdirTracked(root, filepath.Dir(abs))
	defer removeCreatedDirs(created)
	if merr != nil {
		return nil, deskkit.Unverifiable("cannot create local directories under "+root+" to stage the decision-gate check", merr)
	}
	defer func() {
		if existed {
			_ = os.WriteFile(abs, orig, 0o644)
		} else {
			_ = os.Remove(abs)
		}
	}()
	// The branch's board (every changed file but the target) stands in the worktree for
	// both runs, as it does in the base, so only this write's own difference is judged.
	restoreBoard, berr := stageBoardEdits(root, board)
	if berr != nil {
		return nil, berr
	}
	defer restoreBoard()
	// Before: the replaced content in place, the same base.
	if replacedExists {
		if werr := os.WriteFile(abs, replaced, 0o644); werr != nil {
			return nil, deskkit.Unverifiable("cannot stage the decision-gate check at "+abs, werr)
		}
	} else if rmerr := os.Remove(abs); rmerr != nil && !os.IsNotExist(rmerr) {
		return nil, deskkit.Unverifiable("cannot clear "+abs+" to stage the decision-gate check", rmerr)
	}
	before, err := statusgenDecisionGateFn(root, base)
	if err != nil {
		return nil, err
	}
	if werr := os.WriteFile(abs, newContent, 0o644); werr != nil {
		return nil, deskkit.Unverifiable("cannot stage the decision-gate check at "+abs, werr)
	}
	after, aerr := statusgenDecisionGateFn(root, base)
	if aerr != nil {
		return nil, aerr
	}
	had := make(map[string]bool, len(before))
	for _, l := range before {
		had[l] = true
	}
	seen := map[string]bool{}
	for _, l := range after {
		if had[l] || seen[l] {
			continue
		}
		seen[l] = true
		introduced = append(introduced, l)
	}
	return introduced, nil
}

// replacedBaseCommit writes (without touching any ref, the index or the worktree) a
// commit whose tree is root's HEAD tree with targetRepoPath holding replaced — or
// absent when !replacedExists — and returns its id. It uses a throwaway index file.
func replacedBaseCommit(root, targetRepoPath string, replaced []byte, replacedExists bool, board []boardEdit) (string, error) {
	fail := func(what string, err error) (string, error) {
		return "", deskkit.Unverifiable("cannot build the decision-gate base ("+what+") in "+root, err)
	}
	tmp, err := os.MkdirTemp("", "deskevidence-gate-index-")
	if err != nil {
		return fail("temp dir", err)
	}
	defer os.RemoveAll(tmp)
	env := append(os.Environ(),
		"GIT_INDEX_FILE="+filepath.Join(tmp, "index"),
		"GIT_AUTHOR_NAME=deskevidence", "GIT_AUTHOR_EMAIL=deskevidence@invalid",
		"GIT_COMMITTER_NAME=deskevidence", "GIT_COMMITTER_EMAIL=deskevidence@invalid",
	)
	git := func(stdin []byte, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = env
		cmd.Dir = os.TempDir()
		if stdin != nil {
			cmd.Stdin = bytes.NewReader(stdin)
		}
		out, cerr := cmd.Output()
		return strings.TrimSpace(string(out)), cerr
	}
	head, err := git(nil, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil || head == "" {
		return fail("HEAD does not resolve", err)
	}
	if _, err := git(nil, "read-tree", head); err != nil {
		return fail("read-tree", err)
	}
	// The base replaces each staged path with a plain blob: refuse when the home's HEAD
	// holds that path (or a directory above it) as anything else, a link or a submodule
	// the board would not read as a record (S-2266-7).
	paths := []string{targetRepoPath}
	for _, e := range board {
		paths = append(paths, e.Path)
	}
	for _, p := range paths {
		if err := headEntriesPlain(git, head, p); err != nil {
			return "", deskkit.Unverifiable("cannot build the decision-gate base in "+root+": "+err.Error()+" (decision-gate hold, lifecycle-v1 §4.5)", nil)
		}
	}
	if replacedExists {
		blob, err := git(replaced, "hash-object", "-w", "--stdin")
		if err != nil || blob == "" {
			return fail("hash-object", err)
		}
		if _, err := git(nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+targetRepoPath); err != nil {
			return fail("update-index", err)
		}
	} else if _, err := git(nil, "update-index", "--force-remove", "--", targetRepoPath); err != nil {
		return fail("update-index --force-remove", err)
	}
	for _, e := range board {
		if e.Delete {
			if _, err := git(nil, "update-index", "--force-remove", "--", e.Path); err != nil {
				return fail("update-index --force-remove "+e.Path, err)
			}
			continue
		}
		blob, err := git(e.Content, "hash-object", "-w", "--stdin")
		if err != nil || blob == "" {
			return fail("hash-object "+e.Path, err)
		}
		if _, err := git(nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+e.Path); err != nil {
			return fail("update-index "+e.Path, err)
		}
	}
	tree, err := git(nil, "write-tree")
	if err != nil || tree == "" {
		return fail("write-tree", err)
	}
	commit, err := git([]byte("deskevidence decision-gate base\n"), "commit-tree", tree, "-p", head)
	if err != nil || commit == "" {
		return fail("commit-tree", err)
	}
	return commit, nil
}

// boardEdit is one board file the target branch holds differently from the landing
// home: Content is the branch's bytes, or Delete is true when the branch has no such
// file. The judge is run on the home's tree with these applied, so it reads the board
// the write lands on (S-2266-6).
type boardEdit struct {
	Path    string
	Content []byte
	Delete  bool
}

// decisionGateBranchBoardFn is the seam cmdEvidence calls before the judge. setupFake
// stubs it to "no difference"; decisiongate_test.go drives decisionGateBranchBoard
// directly and the whole-verb tests restore the production binding.
var decisionGateBranchBoardFn = decisionGateBranchBoard

// decisionGateCompareFileCap is the number of files the forge's compare read serves
// before it stops listing (GitHub: 300). A listing that reaches it may be truncated.
const decisionGateCompareFileCap = 300

// decisionGateBoardRe selects the paths whose change on the target branch the judge
// would not see from a stale landing home: the board's files under docs/streams and
// docs/archive, less the registers the board never reads (findings, intake,
// verify-outcomes), which change on most days and bear on no brief's gate.
var (
	decisionGateBoardRe    = regexp.MustCompile(`^docs/(streams|archive)/`)
	decisionGateRegisterRe = regexp.MustCompile(`^docs/(streams|archive)/(findings|intake|verify-outcomes)/`)
)

// decisionGateBranchBoard returns the board files the target branch holds differently
// from the landing home, so the judge can read the board the write lands on.
//
// decisionGateDiffAt reads every board record except the write's own target from the
// landing home: its HEAD for the base, its working tree for the after side. The home
// is bound to the commit fixed at verifier dispatch, which sits behind the target
// branch once anything lands there (including this verb's own earlier landing of the
// same brief). A brief relabelled gate: human on the branch after dispatch is then
// invisible to the judge, and an unruled move of it would land. The write lands on the
// branch (lifecycle-v1 §4.5), so the branch is what it must be judged against: every
// board file the branch changed since the home's HEAD, other than the target the write
// replaces, is read at the branch tip and applied to both sides of the judge. The home
// is never moved.
//
// Fail closed throughout: a home HEAD the forge does not know, a compare that cannot be
// shown complete (or a forge that serves no compare), or a changed board file that
// cannot be read at the branch tip is could-not-check, never a pass.
func decisionGateBranchBoard(fg deskkit.Forge, fr deskkit.ForgeRepo, root, branch, targetRepoPath string) ([]boardEdit, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	head := strings.TrimSpace(string(out))
	if err != nil || head == "" {
		return nil, deskkit.Unverifiable("cannot resolve the landing home's HEAD in "+root+" to compare it with "+branch+" (decision-gate hold, lifecycle-v1 §4.5)", err)
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")
	// The branch tip is resolved ONCE, and both the compare and every read below are made
	// at that commit, so a push to the branch between them cannot pair one tip's file list
	// with another tip's bytes.
	tipCommit, err := fg.GetCommit(fr, branch)
	if err != nil || tipCommit == nil || strings.TrimSpace(tipCommit.SHA) == "" {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot resolve the tip of %s on %s, so the board the write lands on cannot be judged (decision-gate hold, lifecycle-v1 §4.5)", branch, fr.Slug()), err)
	}
	tip := tipCommit.SHA
	cmp, err := fg.CompareRefs(fr, head, tip)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot compare the landing home's HEAD %s with %s (%s) on %s, so the board the write lands on cannot be shown to match the one judged (decision-gate hold, lifecycle-v1 §4.5)", head, branch, tip, fr.Slug()), err)
	}
	if cmp == nil || !cmp.CommitsComplete || len(cmp.Files) >= decisionGateCompareFileCap {
		return nil, deskkit.Unverifiable(fmt.Sprintf("the forge's compare of the landing home's HEAD %s with %s (%s) on %s cannot be shown complete, so a board change on the branch cannot be ruled out (decision-gate hold, lifecycle-v1 §4.5)", head, branch, tip, fr.Slug()), nil)
	}
	// The compare lists what the branch changed since the commits' merge base. Only when
	// the home's HEAD is an ancestor of the tip is that every difference between the board
	// judged and the board landed on; a home carrying commits the branch lacks would keep
	// its own board edits in the judge.
	if cmp.BehindBy != 0 || (cmp.Status != "ahead" && cmp.Status != "identical") {
		return nil, deskkit.Unverifiable(fmt.Sprintf("the landing home's HEAD %s is not an ancestor of %s (%s) on %s (compare status %q, behind by %d), so the board judged cannot be shown to be the board the write lands on (decision-gate hold, lifecycle-v1 §4.5)", head, branch, tip, fr.Slug(), cmp.Status, cmp.BehindBy), nil)
	}
	isBoard := func(p string) bool {
		return p != "" && p != targetRepoPath && decisionGateBoardRe.MatchString(p) && !decisionGateRegisterRe.MatchString(p)
	}
	var edits []boardEdit
	seen := map[string]bool{}
	for _, f := range cmp.Files {
		// A previous name is gone from the branch only when the file was renamed; a copy
		// keeps it.
		if f.Status == "renamed" && isBoard(f.PreviousFilename) && !seen[f.PreviousFilename] {
			seen[f.PreviousFilename] = true
			edits = append(edits, boardEdit{Path: f.PreviousFilename, Delete: true})
		}
		if !isBoard(f.Filename) || seen[f.Filename] {
			continue
		}
		seen[f.Filename] = true
		if f.Status == "removed" {
			edits = append(edits, boardEdit{Path: f.Filename, Delete: true})
			continue
		}
		fc, rerr := fg.ReadFile(fr, deskkit.ReadFileInput{File: f.Filename, Ref: tip})
		if rerr != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("cannot read %s at %s (%s) on %s, so the board the write lands on cannot be judged (decision-gate hold, lifecycle-v1 §4.5)", f.Filename, branch, tip, fr.Slug()), rerr)
		}
		if fc == nil || !fc.Exists {
			return nil, deskkit.Unverifiable(fmt.Sprintf("%s is listed as changed on %s but is absent at its tip, so the board the write lands on cannot be judged (decision-gate hold, lifecycle-v1 §4.5)", f.Filename, branch), nil)
		}
		// The board reads a record only from a plain file. A link (served as its target's
		// bytes, or as nothing) or a submodule is an unreadable record to the board, so
		// staging it as a plain file would judge a board the write does not land on.
		if fc.Kind != deskkit.FileKindRegular {
			kind := fc.Kind
			if kind == "" {
				kind = "an entry the forge did not type"
			}
			return nil, deskkit.Unverifiable(fmt.Sprintf("%s at %s (%s) on %s is %s, not a plain file, so the board the write lands on cannot be judged (decision-gate hold, lifecycle-v1 §4.5)", f.Filename, branch, tip, fr.Slug(), kind), nil)
		}
		edits = append(edits, boardEdit{Path: f.Filename, Content: fc.Content})
	}
	return edits, nil
}

// stageBoardEdits applies edits to the working tree at root and returns the function
// that puts every touched file back as it was.
func stageBoardEdits(root string, edits []boardEdit) (restore func(), err error) {
	type saved struct {
		abs     string
		content []byte
		existed bool
	}
	var undo []saved
	var dirs [][]string
	restore = func() {
		for i := len(undo) - 1; i >= 0; i-- {
			u := undo[i]
			if u.existed {
				_ = os.WriteFile(u.abs, u.content, 0o644)
			} else {
				_ = os.Remove(u.abs)
			}
		}
		for i := len(dirs) - 1; i >= 0; i-- {
			removeCreatedDirs(dirs[i])
		}
	}
	for _, e := range edits {
		if lerr := refuseLinkedHomePath(root, e.Path); lerr != nil {
			restore()
			return func() {}, lerr
		}
		abs := filepath.Join(root, filepath.FromSlash(e.Path))
		orig, rerr := os.ReadFile(abs)
		if rerr != nil && !os.IsNotExist(rerr) {
			restore()
			return func() {}, deskkit.Unverifiable("cannot read local "+abs+" to stage the branch's board for the decision-gate check", rerr)
		}
		undo = append(undo, saved{abs: abs, content: orig, existed: rerr == nil})
		if e.Delete {
			if rmerr := os.Remove(abs); rmerr != nil && !os.IsNotExist(rmerr) {
				restore()
				return func() {}, deskkit.Unverifiable("cannot clear "+abs+" to stage the branch's board for the decision-gate check", rmerr)
			}
			continue
		}
		created, merr := mkdirTracked(root, filepath.Dir(abs))
		dirs = append(dirs, created)
		if merr != nil {
			restore()
			return func() {}, deskkit.Unverifiable("cannot create local directories under "+root+" to stage the branch's board for the decision-gate check", merr)
		}
		if werr := os.WriteFile(abs, e.Content, 0o644); werr != nil {
			restore()
			return func() {}, deskkit.Unverifiable("cannot stage the branch's board at "+abs+" for the decision-gate check", werr)
		}
	}
	return restore, nil
}

// refuseLinkedHomePath is could-not-check unless rel names, under root, a plain file or
// nothing, reached through real directories only: no component is a symbolic link, no
// directory position holds a file, and the path itself is not a directory, device or
// other non-file. rel must be a local, slash-separated repo path.
func refuseLinkedHomePath(root, rel string) error {
	local := filepath.FromSlash(rel)
	if rel == "" || !filepath.IsLocal(local) {
		return deskkit.Unverifiable(fmt.Sprintf("%q is not a path inside the landing home, so the decision-gate check cannot stage it (lifecycle-v1 §4.5)", rel), nil)
	}
	parts := strings.Split(filepath.Clean(local), string(filepath.Separator))
	cur := root
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return deskkit.Unverifiable("cannot inspect local "+cur+" to stage the decision-gate check", err)
		}
		last := i == len(parts)-1
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			return deskkit.Unverifiable(fmt.Sprintf("local %s is a symbolic link; the decision-gate check stages %s only through real directories onto a plain file (lifecycle-v1 §4.5)", cur, rel), nil)
		case !last && !fi.IsDir():
			return deskkit.Unverifiable(fmt.Sprintf("local %s is not a directory, so %s cannot be staged for the decision-gate check (lifecycle-v1 §4.5)", cur, rel), nil)
		case last && !fi.Mode().IsRegular():
			return deskkit.Unverifiable(fmt.Sprintf("local %s is not a plain file, so the decision-gate check cannot stage it (lifecycle-v1 §4.5)", cur), nil)
		}
	}
	return nil
}

// headEntriesPlain checks the home HEAD's entries along rel: each directory above it is a
// tree (040000) and rel itself a plain blob (100644 or 100755), where present. git runs
// one git command in the home (replacedBaseCommit's runner).
//
// `ls-tree -r -t` on rel lists the trees on the way down and rel itself, stopping at the
// first component that is not a tree; that component, when present, is then listed on its
// own, so a link or submodule standing where a directory belongs is seen too.
func headEntriesPlain(git func([]byte, ...string) (string, error), head, rel string) error {
	entries := func(args ...string) (map[string]string, error) {
		out, err := git(nil, append([]string{"ls-tree", "-z"}, args...)...)
		if err != nil {
			return nil, fmt.Errorf("cannot list %s in the home's HEAD: %v", rel, err)
		}
		modes := map[string]string{}
		for _, rec := range strings.Split(out, "\x00") {
			if rec == "" {
				continue
			}
			meta, name, ok := strings.Cut(rec, "\t")
			f := strings.Fields(meta)
			if !ok || len(f) != 3 {
				return nil, fmt.Errorf("cannot read the home's HEAD entry %q", rec)
			}
			modes[name] = f[0]
		}
		return modes, nil
	}
	chain, err := entries("-r", "-t", head, "--", rel)
	if err != nil {
		return err
	}
	parts := strings.Split(rel, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		last := i == len(parts)-1
		mode, ok := chain[p]
		if !ok {
			// Absent from the chain: either nothing is there (a new path) or something that
			// is not a tree stands where a directory belongs.
			own, err := entries(head, "--", p)
			if err != nil {
				return err
			}
			if m, there := own[p]; there {
				return fmt.Errorf("the home's HEAD holds %s as mode %s, not a directory", p, m)
			}
			return nil
		}
		switch {
		case !last && mode != "040000":
			return fmt.Errorf("the home's HEAD holds %s as mode %s, not a directory", p, mode)
		case last && mode != "100644" && mode != "100755":
			return fmt.Errorf("the home's HEAD holds %s as mode %s, not a plain file", p, mode)
		}
	}
	return nil
}

// mkdirTracked creates dir (under root) and every missing directory above it, returning
// the ones it created, deepest first, so removeCreatedDirs can take them away again.
func mkdirTracked(root, dir string) ([]string, error) {
	var missing []string
	for d := dir; d != root && len(d) > len(root); d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		missing = append(missing, d)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return missing, err
	}
	return missing, nil
}

// removeCreatedDirs removes the directories mkdirTracked created, deepest first; a
// directory something else has since written into is left in place.
func removeCreatedDirs(created []string) {
	for _, d := range created {
		_ = os.Remove(d)
	}
}

// decisionGateRefusedMark is the separator statusgen prints between a board id and
// the reason on a refused line ("<board> REFUSED — <reason>").
const decisionGateRefusedMark = " REFUSED — "

// decisionGateWholeChange is the board label statusgen uses when it fails closed on
// the change as a whole rather than on one brief.
const decisionGateWholeChange = "(board)" + decisionGateRefusedMark

// statusgenDecisionGateAt shells `statusgen --root <root> --decision-gate
// --decision-gate-base <base>` from a neutral working directory: the working tree
// judged against base (decisionGateDiffAt passes the commit that holds the content
// the write replaces).
//
//   - exit 0                     → nothing refused.
//   - exit 1 with ≥1 REFUSED line → those lines.
//   - anything else (exit 1 with no REFUSED line, exit 2, an unknown flag from a
//     statusgen that predates the hold) → could-not-check.
func statusgenDecisionGateAt(root, base string) ([]string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return nil, deskkit.Unverifiable("statusgen is not on PATH — cannot run the decision-gate check (lifecycle-v1 §4.5) on this landing", err)
	}
	cmd := exec.Command("statusgen", "--root", root, "--decision-gate", "--decision-gate-base", base)
	cmd.Dir = os.TempDir()
	out, cerr := cmd.CombinedOutput()
	return parseDecisionGateRun(root, string(out), cerr)
}

// parseDecisionGateRun reads one `statusgen --decision-gate` run (split out so the
// exit/line reading is testable without a statusgen binary).
func parseDecisionGateRun(root, out string, cerr error) ([]string, error) {
	var refused []string
	for _, ln := range strings.Split(out, "\n") {
		t := strings.TrimSpace(ln)
		if strings.Contains(t, decisionGateRefusedMark) {
			refused = append(refused, t)
		}
	}
	if cerr == nil {
		return nil, nil
	}
	ee, ok := cerr.(*exec.ExitError)
	if !ok {
		return nil, deskkit.Unverifiable("could not run statusgen --decision-gate at "+root, cerr)
	}
	if strings.Contains(out, "flag provided but not defined") {
		return nil, deskkit.Unverifiable("the statusgen on PATH predates the decision-gate hold (no --decision-gate flag) — upgrade statusgen to the release that ships this deskevidence; a landing is never let through unchecked", cerr)
	}
	if ee.ExitCode() == 1 && len(refused) > 0 {
		// A "(board)" refusal is statusgen failing closed on the whole change (the base
		// does not resolve, or the board cannot be read at one end): no brief was judged.
		// Diffing it before/after would cancel it out and let the landing through, so it
		// is could-not-check here, never a pass.
		for _, l := range refused {
			if strings.HasPrefix(l, decisionGateWholeChange) {
				return nil, deskkit.Unverifiable("statusgen --decision-gate could not judge "+root+": "+l, cerr)
			}
		}
		return refused, nil
	}
	msg := "statusgen --decision-gate could not evaluate " + root + " (exit nonzero with no REFUSED line)"
	if t := strings.TrimSpace(out); t != "" {
		if i := strings.LastIndex(t, "\n"); i >= 0 {
			t = t[i+1:]
		}
		msg += ": " + t
	}
	return nil, deskkit.Unverifiable(msg, cerr)
}
