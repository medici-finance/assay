package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitmaintenance"
	"github.com/medici-finance/assay/tools/desk/internal/reviewhome"
)

// The parent observes harness exit and the handoff of any external evidence.
// The receipt predates claim release. Both commands may be retried independently.
func runReviewExited(home, attempt string) (err error) {
	ac := &auditCtx{verb: "review-exited"}
	defer func() { ac.finalize(err) }()
	dir, err := getwd()
	if err != nil {
		return err
	}
	guard, err := newPathGuard(dir)
	if err != nil {
		return err
	}
	rt, err := guard.check(home)
	if err != nil {
		return err
	}
	if cwd := resolvePath(dir); cwd == rt || strings.HasPrefix(cwd, rt+string(filepath.Separator)) {
		return deskkit.Refused("the parent must acknowledge review exit from outside the review checkout")
	}
	if _, err := os.Lstat(rt); os.IsNotExist(err) {
		paths, err := guard.worktreePaths(dir)
		if err != nil {
			return err
		}
		key, _, _ := strings.Cut(attempt, "@")
		if !paths[rt] && deskkit.ValidDispatchRef(key, attempt) {
			ac.successResult = deskkit.ResultNoop
			ac.detail = "review checkout already absent"
			fmt.Fprintln(os.Stdout, ac.detail)
			return nil
		}
	}
	admin, err := ownAdminEntry(guard, rt)
	if err != nil {
		return err
	}
	r, err := reviewhome.Read(admin)
	if err != nil {
		return deskkit.Unverifiable("cannot read confirmed review", err)
	}
	if r.Home != rt || r.Attempt != attempt {
		return deskkit.Refused("review exit does not match the confirmed checkout attempt")
	}
	bound, _, err := r.Identity()
	if err != nil || bound != admin {
		return deskkit.Unverifiable("review checkout identity changed", err)
	}
	if err = r.Acknowledge(admin); err != nil {
		return deskkit.Unverifiable("cannot record reviewer exit", err)
	}
	ac.repo = r.Repo
	why := retireReview(guard, newSweepCtx(dir), dir, rt, admin, attempt, pruneOpts{minIdle: defaultMinIdle})
	if why == "" {
		ac.detail = "review checkout retired"
	} else {
		ac.detail = "reviewer exit recorded; queued for prune: " + why
	}
	fmt.Fprintln(os.Stdout, ac.detail)
	return nil
}

type reviewBacklog struct {
	retired, pending, unknownSize, unknownAge int
	bytes                                     int64
	oldest                                    time.Duration
}

func (b reviewBacklog) summary() string {
	return fmt.Sprintf("review_retired=%d review_backlog=%d review_backlog_bytes=%d review_oldest=%s review_unknown_size=%d review_unknown_age=%d",
		b.retired, b.pending, b.bytes, b.oldest.Truncate(time.Second), b.unknownSize, b.unknownAge)
}

func (b *reviewBacklog) hold(rt string, confirmed fs.FileInfo, now time.Time) {
	b.pending++
	if confirmed == nil {
		b.unknownAge++
	} else if age := now.Sub(confirmed.ModTime()); age >= 0 {
		b.oldest = max(b.oldest, age)
	} else {
		b.unknownAge++
	}
	var size int64
	err := filepath.WalkDir(rt, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err == nil {
			size += fi.Size()
		}
		return err
	})
	if err != nil {
		b.unknownSize++
	} else {
		b.bytes += size
	}
}

// A receipt appearing after the sweep initially classified a tree as ordinary
// transfers custody to review retirement. Leave it for the next pass.
func hasReviewReceipt(admin string) bool {
	if admin == "" {
		return false
	}
	_, err := os.Lstat(filepath.Join(admin, reviewhome.ReceiptFile))
	return !errors.Is(err, fs.ErrNotExist)
}

// checkReviewReceipt takes precedence over ordinary merge pruning and stale-session
// reaping. A failed gate keeps even a merged checkout carrying review evidence.
func checkReviewReceipt(guard *pathGuard, sc *sweepCtx, admin, dir, rt, precondition string, opts pruneOpts, res *pruneResult) (handled, removed bool) {
	if admin == "" {
		return false, false
	}
	info, err := os.Lstat(filepath.Join(admin, reviewhome.ReceiptFile))
	if errors.Is(err, fs.ErrNotExist) {
		return false, false
	}
	hold := precondition
	if hold == "" && (err != nil || !info.Mode().IsRegular()) {
		hold = "unreadable review receipt"
	}
	if hold == "" {
		hold = retireReview(guard, sc, dir, rt, admin, "", opts)
	}
	if hold != "" {
		res.skips = append(res.skips, skipEntry{rt, "review retirement: " + hold})
		res.reviews.hold(rt, info, time.Now())
		return true, false
	}
	res.reviews.retired++
	return true, true
}

func retireReview(guard *pathGuard, sc *sweepCtx, dir, rt, admin, attempt string, opts pruneOpts) string {
	// Serialize the coordinator's first attempt with all periodic retries.
	// The shared lock survives removal of the registration on every platform.
	if !opts.dryRun {
		f, err := gitmaintenance.OpenLock(filepath.Join(guard.commonDir, "assay-review-retirement.lock"))
		if err != nil {
			return err.Error()
		}
		defer f.Close()
		if err := deskkit.TryLockExclusive(f); err != nil {
			return "retirement lock: " + err.Error()
		}
		defer deskkit.UnlockFile(f)
	}
	r, err := reviewhome.Read(admin)
	if err != nil {
		return err.Error()
	}
	if r.Home != rt || (attempt != "" && attempt != r.Attempt) {
		return "confirmed checkout path changed"
	}
	exited, err := os.ReadFile(filepath.Join(admin, reviewhome.ExitFile))
	if err != nil || string(exited) != r.Attempt {
		return "awaiting reviewer exit and evidence handoff"
	}
	verify := func() string {
		current, head, err := r.Identity()
		if err != nil || current != admin || head != r.CheckoutHead {
			return "checkout registration, dispatch or detached HEAD changed"
		}
		if _, err := os.Lstat(filepath.Join(admin, "locked")); !errors.Is(err, fs.ErrNotExist) {
			return "checkout is locked or lock is unreadable"
		}
		if why := activityHold(map[string]string{rt: admin}, nil, rt, opts.minIdle, time.Now()); why != "" {
			return why
		}
		for _, op := range []string{"index.lock", "HEAD.lock", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_START", "rebase-apply", "rebase-merge", "sequencer"} {
			if _, err := os.Lstat(filepath.Join(admin, op)); !errors.Is(err, fs.ErrNotExist) {
				return "Git operation in progress or unreadable: " + op
			}
		}
		// Pin the index to this checkout, include ignored files and override
		// user settings that could hide untracked files or a dirty submodule.
		status, err := reviewInventory(rt, "--git-dir="+admin, "--work-tree="+rt, "--no-optional-locks", "-c", "core.untrackedCache=false", "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all", "--ignore-submodules=none")
		if err != nil {
			return err.Error()
		}
		if status != "" {
			return "tracked, untracked or ignored files require handoff"
		}
		flags, err := reviewInventory(rt, "--git-dir="+admin, "--work-tree="+rt, "ls-files", "-v", "--stage", "-z")
		if err != nil {
			return err.Error()
		}
		for entry := range strings.SplitSeq(flags, "\x00") {
			if strings.HasPrefix(entry, "H 160000 ") {
				return "review submodules require separate preservation"
			}
			if entry != "" && entry[0] != 'H' {
				return "index contains hidden or nonstandard entries"
			}
		}
		return ""
	}
	if why := verify(); why != "" {
		return why
	}
	claim := sc.claimFor(guard, dir, rt)
	if !claim.releasedOwner() || claim.key != r.Key {
		return "cannot prove release of this claim: " + claim.why
	}
	if why := reviewHeadRecoverable(sc, dir, claim.remote, r); why != "" {
		return why
	}
	if opts.dryRun {
		return ""
	}
	if _, err := deskkit.RunHook(deskkit.HookBeforeRemove, deskkit.HookEnv{RunKey: r.Key, Worktree: rt, Repo: r.Repo}); err != nil {
		return "before_remove: " + err.Error()
	}
	if why := claimStillFree(dir, claim.remote, r.Key); why != "" {
		return why
	}
	admissions, err := guard.verifierAdmissions()
	if err != nil {
		return err.Error()
	}
	if why := admissionHoldReason(admissions, rt, time.Now()); why != "" {
		return why
	}
	if why := verify(); why != "" {
		return why
	}
	// Use Git's non-forced removal: it checks the registration, lock and
	// cleanliness again and deregisters just this checkout in the same call.
	if _, err := reviewInventory(guard.sharedCheckout, "--git-dir="+guard.commonDir, "worktree", "remove", "--", rt); err != nil {
		return err.Error()
	}
	return ""
}

// A normal review fetch can populate only FETCH_HEAD. When no persistent remote
// tracking ref covers the commit, prove the PR's repository still publishes this exact PR
// head. remote is where the claim was read (claimRemote): origin when the receipt's
// repository is this checkout's origin, else that repository's derived address. A checkout
// base whose origin is another repository must never vouch for the head under the same PR
// number.
func reviewHeadRecoverable(sc *sweepCtx, dir, remote string, r reviewhome.Receipt) string {
	repo, err := sc.open(sc.historyDir)
	if err != nil {
		return err.Error()
	}
	refs, err := sweepRefsContaining(repo, r.CheckoutHead)
	if err != nil {
		return err.Error()
	}
	if len(refs) > 0 {
		return ""
	}
	if r.CheckoutHead != r.Head {
		return "checkout commit is not reachable from a remote-tracking ref"
	}
	if remote == "" || strings.HasPrefix(remote, "-") {
		return "cannot name the repository that publishes the review head"
	}
	pr := strconv.Itoa(r.PR)
	gh, gl := "refs/pull/"+pr+"/head", "refs/merge-requests/"+pr+"/head"
	ctx, cancel := context.WithTimeout(context.Background(), claimListTimeout)
	defer cancel()
	result := deskkit.Run(deskkit.ToolCall{Context: ctx, Name: "git", Args: []string{"ls-remote", "--refs", remote, gh, gl}, Dir: dir, Env: claimListEnv(), Start: execCommand})
	if result.Failed() {
		return "cannot read the published review head"
	}
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		hash, ref, ok := strings.Cut(line, "\t")
		if ok && hash == r.CheckoutHead && (ref == gh || ref == gl) {
			return ""
		}
	}
	return r.Repo + " no longer advertises the confirmed review commit"
}
