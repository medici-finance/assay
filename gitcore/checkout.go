package gitcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CheckoutLocalCommit checks out an exact, already fetched commit into an empty
// disposable directory. Only the new repository's HEAD/index/worktree are written.
// Its object alternate retains the original commit objects and their real history;
// no synthetic commit, copied identity configuration, network or credential is used.
// The caller owns dest and its cleanup. Missing history fails rather than falling
// back to the source home's (deliberately frozen) HEAD.
func CheckoutLocalCommit(source, dest, commit string) error {
	if !isFullHex(commit) {
		return fmt.Errorf("gitcore: checkout requires a full commit id")
	}
	common, err := CommonDir(source)
	if err != nil {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(common, "shallow")); err != nil && !os.IsNotExist(err) {
		return err
	} else if len(b) > 0 {
		return fmt.Errorf("gitcore: checkout requires complete local history")
	}
	if strings.ContainsAny(common, "\r\n") {
		return fmt.Errorf("gitcore: unsupported object alternate path")
	}
	src, err := Open(source)
	if err != nil {
		return err
	}
	baseline, err := src.repo.Reference(plumbing.ReferenceName("refs/remotes/origin/main"), true)
	if err != nil || baseline.Type() != plumbing.HashReference {
		return fmt.Errorf("gitcore: landed history requires fetched origin/main baseline: %w", err)
	}
	if base, err := src.MergeBase(baseline.Hash().String(), commit); err != nil || base == "" {
		return fmt.Errorf("gitcore: landed commit has no readable main baseline: %w", err)
	}
	if _, err := git.PlainInit(dest, false); err != nil {
		return err
	}
	info := filepath.Join(dest, ".git", "objects", "info")
	if err := os.MkdirAll(info, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(info, "alternates"), []byte(filepath.Join(common, "objects")+"\n"), 0600); err != nil {
		return err
	}
	r, err := Open(dest)
	if err != nil {
		return err
	}
	if _, err := r.repo.CommitObject(plumbing.NewHash(commit)); err != nil {
		return fmt.Errorf("gitcore: landed commit unavailable locally; fetch the target branch before publishing: %w", err)
	}
	refs, err := src.repo.References()
	if err != nil {
		return err
	}
	defer refs.Close()
	if err := refs.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(string(ref.Name()), "refs/remotes/") {
			return r.repo.Storer.SetReference(ref)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := r.repo.Storer.SetReference(baseline); err != nil {
		return err
	}
	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}
	return wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)})
}
