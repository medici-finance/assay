package deskkit

import (
	"fmt"
	"net/http"
	"path"
	"strings"
)

// TreeEntry is one non-directory entry of a tree listing: its slash-separated path from the
// repository root, its git mode as the forge serves it ("100644", "120000", "160000", ...) and
// its object id. Two listings of one path at two commits hold the same entry exactly when the
// mode and the object id are equal.
type TreeEntry struct {
	Path string
	Mode string
	OID  string
}

// ReadTreeListing lists every non-directory entry under dir at commit, recursively, and
// answers only with the whole listing or an error. A dir the commit does not hold is an empty
// listing (the commit is first shown to exist, so a missing commit is never read as an empty
// directory). Every other uncertain shape is could-not-check:
//
//   - commit not a full lowercase object id (an abbreviation or branch name could move), or
//     dir not a clean relative path;
//   - the forge resolves the commit to anything but that commit, or dir to anything but a tree;
//   - the forge reports the recursive listing truncated, serves a different tree than the one
//     asked for, or serves an entry with no path, mode or object id, or a path twice.
//
// It exists because a forge compare lists at most a fixed number of changed files (GitHub:
// 300): a caller that needs every difference under a few directories reads those directories
// at both commits instead of trusting a capped list. Only the GitHub backend serves it.
// Consumer: cmd/deskevidence's decision-gate board read when the compare is truncated.
func ReadTreeListing(f Forge, repo ForgeRepo, commit, dir string) ([]TreeEntry, error) {
	g, ok := backendOf(f).(*GitHubForge)
	if !ok {
		return nil, Unverifiable("could-not-check: ReadTreeListing is served only by the GitHub backend for "+repo.Slug(), nil)
	}
	return g.treeListing(repo, commit, dir)
}

// ghTreeObjectQuery resolves the commit and the directory under it in one read: the commit's
// own object (so an absent directory is told apart from an absent commit) and the object at
// "<commit>:<dir>".
const ghTreeObjectQuery = `query TreeObject($owner:String!,$name:String!,$rev:String!,$expr:String!){
 repository(owner:$owner,name:$name){
  commit:object(expression:$rev){__typename oid}
  dir:object(expression:$expr){__typename oid}
 }
}`

// ghTreeWire is the git-trees read shape (only the fields consumed).
type ghTreeWire struct {
	SHA       string `json:"sha"`
	Truncated *bool  `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
		Type string `json:"type"`
		SHA  string `json:"sha"`
	} `json:"tree"`
}

func (g *GitHubForge) treeListing(repo ForgeRepo, commit, dir string) ([]TreeEntry, error) {
	if !IsFullSHA(commit) {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: the tree listing on %s needs a full commit id, got %q", repo.Slug(), commit), nil)
	}
	if dir == "" || path.Clean(dir) != dir || strings.HasPrefix(dir, "/") || dir == "." || strings.HasPrefix(dir, "../") || dir == ".." {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: the tree listing on %s needs a clean relative directory, got %q", repo.Slug(), dir), nil)
	}
	type object struct {
		Typename string `json:"__typename"`
		OID      string `json:"oid"`
	}
	var env struct {
		Data struct {
			Repository *struct {
				Commit *object `json:"commit"`
				Dir    *object `json:"dir"`
			} `json:"repository"`
		} `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	in := map[string]any{"query": ghTreeObjectQuery, "variables": map[string]any{
		"owner": repo.Owner, "name": repo.Name, "rev": commit, "expr": commit + ":" + dir,
	}}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &env); err != nil {
		return nil, err
	}
	if len(env.Errors) > 0 {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: the forge answered the tree read of %s at %s on %s with errors", dir, commit, repo.Slug()), nil)
	}
	r := env.Data.Repository
	if r == nil || r.Commit == nil || r.Commit.Typename != "Commit" || !strings.EqualFold(r.Commit.OID, commit) {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: %s does not resolve to that commit on %s", commit, repo.Slug()), nil)
	}
	if r.Dir == nil {
		return []TreeEntry{}, nil
	}
	if r.Dir.Typename != "Tree" || r.Dir.OID == "" {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: %s at %s on %s is a %s, not a directory", dir, commit, repo.Slug(), r.Dir.Typename), nil)
	}
	var w ghTreeWire
	p := fmt.Sprintf("/repos/%s/%s/git/trees/%s?recursive=1", repo.Owner, repo.Name, r.Dir.OID)
	if err := g.doJSON(http.MethodGet, p, nil, &w); err != nil {
		return nil, err
	}
	if !strings.EqualFold(w.SHA, r.Dir.OID) {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: the forge served tree %q for %s at %s on %s, not %s", w.SHA, dir, commit, repo.Slug(), r.Dir.OID), nil)
	}
	if w.Truncated == nil || *w.Truncated || w.Tree == nil {
		return nil, Unverifiable(fmt.Sprintf("could-not-check: the forge's listing of %s at %s on %s is truncated or incomplete", dir, commit, repo.Slug()), nil)
	}
	out := make([]TreeEntry, 0, len(w.Tree))
	seen := map[string]bool{}
	for _, e := range w.Tree {
		if e.Path == "" || e.Mode == "" || e.SHA == "" || seen[e.Path] {
			return nil, Unverifiable(fmt.Sprintf("could-not-check: the forge's listing of %s at %s on %s serves an entry with no path, mode or object id, or a path twice", dir, commit, repo.Slug()), nil)
		}
		seen[e.Path] = true
		if e.Type == "tree" {
			continue
		}
		out = append(out, TreeEntry{Path: dir + "/" + e.Path, Mode: e.Mode, OID: strings.ToLower(e.SHA)})
	}
	return out, nil
}
