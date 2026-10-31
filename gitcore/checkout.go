package gitcore

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// originMain is where a landed checkout records its repository's main.
const originMain = plumbing.ReferenceName("refs/remotes/origin/main")

// CheckoutLocalCommit checks out an exact, already fetched commit into an empty
// disposable directory. Only the new repository's HEAD/index/worktree are written.
// Its object alternate retains the original commit objects and their real history;
// no synthetic commit, copied identity configuration, network or credential is used.
// Only the source origin's repository URL survives, with URL credentials, query and
// fragment removed. Approval readers still require their explicit network opt-in.
// The caller owns dest and its cleanup. Missing history fails rather than falling
// back to the source home's (deliberately frozen) HEAD.
//
// baseline names the source ref holding the landed repository's own main. The
// commit must share history with it, and dest records it as origin/main. A
// caller whose source tracks a different repository fetches that main into a
// private ref first and names it here, rather than the source's origin/main.
func CheckoutLocalCommit(source, dest, commit, baseline string) error {
	if !isFullHex(commit) {
		return fmt.Errorf("gitcore: checkout requires a full commit id")
	}
	if !strings.HasPrefix(baseline, "refs/") || plumbing.ReferenceName(baseline).Validate() != nil {
		return fmt.Errorf("gitcore: checkout requires a full baseline ref name")
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
	// An absent commit is a missing fetch, never a history mismatch: name it first.
	if _, err := src.repo.CommitObject(plumbing.NewHash(commit)); err != nil {
		return fmt.Errorf("gitcore: landed commit unavailable locally; fetch the target branch before publishing: %w", err)
	}
	mainRef, err := src.repo.Reference(plumbing.ReferenceName(baseline), true)
	if err != nil || mainRef.Type() != plumbing.HashReference {
		return fmt.Errorf("gitcore: landed history requires fetched main baseline %s: %w", baseline, err)
	}
	if base, err := src.MergeBase(mainRef.Hash().String(), commit); err != nil || base == "" {
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
	// Preserve repository authority for every consumer of this history snapshot.
	// Never copy the source config: helpers, headers, hooks and identity stay home.
	cfg, err := src.repo.Config()
	if err != nil {
		return err
	}
	if origin := cfg.Remotes["origin"]; origin != nil && len(origin.URLs) > 0 {
		raw := origin.URLs[0]
		if strings.Contains(raw, "://") {
			u, err := url.Parse(raw)
			if err != nil {
				return fmt.Errorf("gitcore: cannot parse checkout origin")
			}
			u.User, u.RawQuery, u.Fragment = nil, "", ""
			raw = u.String()
		} else if strings.Contains(raw, "@") {
			// Use the existing git URL parser for scp-style SSH coordinates.
			ep, err := transport.NewEndpoint(raw)
			if err != nil {
				return fmt.Errorf("gitcore: cannot parse checkout origin")
			}
			ep.User, ep.Password = "", ""
			raw = ep.String()
		}
		if _, err := r.repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{raw}}); err != nil {
			return err
		}
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
	if err := r.repo.Storer.SetReference(plumbing.NewHashReference(originMain, mainRef.Hash())); err != nil {
		return err
	}
	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}
	return wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)})
}
