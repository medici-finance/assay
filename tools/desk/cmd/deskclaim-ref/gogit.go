package main

// gogit.go — the production claimStore: an in-process git-smart-HTTP transport over go-git
// (internal/gitcore), the ONLY forge access this binary makes. There is no `gh`, `glab`, or any
// CLI here and no external `git` process — a dispatch claim is placed, advanced, stolen, read
// and released as library calls that speak the git wire protocol directly. That is what closes
// the forge-surface violation the ban (internal/forgeban) exists to catch, and it is why this
// path runs native on Windows: it is a plain executable making HTTPS calls, with no shell, no
// shebang, and no bash association to depend on.
//
// The forge (github vs gitlab), the host, the owner/name and the credential are all resolved
// WITHOUT minting anything: the forge kind comes from the roster's ASSAY_REPO_FORGES or the
// origin host (deskkit.ForgeKindFromSlugAndHost, the token-free resolver — never ForgeFor/
// ResolveForge, which read App-token custody), and the token comes from --token-file or the
// GH_TOKEN/GITHUB_TOKEN/GITLAB_TOKEN environment, exactly the ambient credential the bash script
// and the old gh-CLI port used. So this changes the TRANSPORT, not WHO the claim is placed as.
//
// CLOCK — a documented behaviour change from the gh-CLI port. That port omitted the tagger so
// GitHub stamped tagger.date server-side, giving one clock across machines. go-git mints the tag
// locally, so the tagger date is now the CLIENT clock. This is sound: the claim's mutual
// exclusion rests on the SERVER-SIDE compare-and-swap (an explicit-old ref update the forge
// accepts or rejects), not on the timestamp — and the script's own header states clock skew "is
// not an authorization boundary". The timestamp drives only the TTL age display and the
// stale-reclaim heuristic, both tolerant of ordinary skew.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// storeTimeout bounds every forge round trip so a hung remote fails closed (as
// could-not-check) rather than wedging a dispatcher.
const storeTimeout = 30 * time.Second

// gogitStore is the live forge seam. url is the full HTTPS git URL built from a
// roster-validated slug + host; auth is the git-basic credential (token in memory only).
type gogitStore struct {
	url  string
	auth transport.AuthMethod
}

// newForgeStore resolves the forge, host, and credential for repo (an "owner/name" slug) and
// returns a claimStore bound to them, or an error (which the caller maps to exit 6,
// could-not-check). tokenFile, when non-empty, is the 0600 file the token is read from instead
// of the environment.
func newForgeStore(repo, tokenFile string) (claimStore, error) {
	owner, name, ok := splitSlug(repo)
	if !ok {
		return nil, fmt.Errorf("repo %q is not an owner/name slug", repo)
	}
	originHost, _ := parseRemote(originRemoteURL())
	kind, host, err := deskkit.ForgeKindFromSlugAndHost(repo, originHost)
	if err != nil {
		return nil, err
	}
	if host == "" {
		return nil, fmt.Errorf("no host known for forge %q serving %s", kind, repo)
	}
	token, err := resolveToken(tokenFile)
	if err != nil {
		return nil, err
	}
	username := gitcore.GitHubGitUsername
	if kind == deskkit.ForgeGitLab {
		username = gitcore.GitLabGitUsername
	}
	return &gogitStore{
		url:  "https://" + host + "/" + owner + "/" + name + ".git",
		auth: gitcore.BasicAuthAs(username, token),
	}, nil
}

func (g *gogitStore) refName(id string) plumbing.ReferenceName {
	return plumbing.ReferenceName(refPrefix + "/" + id)
}

func (g *gogitStore) read(id string) (claimRef, claimStatus) {
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		return claimRef{}, claimUnverifiable
	}
	sha, found := findRef(refs, refPrefix+"/"+id)
	if !found {
		// A clean advertisement with no matching ref is FREE (the brief's rule); only a
		// transport/auth/not-found error above is could-not-check.
		return claimRef{}, claimFree
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	payload, perr := gitcore.FetchTagPayload(ctx, g.url, g.auth, plumbing.NewHash(sha))
	if perr != nil {
		return claimRef{}, claimUnverifiable
	}
	return claimRef{
		sha:  sha,
		msg:  strings.TrimRight(payload.Message, "\n"),
		date: payload.When.UTC().Format(time.RFC3339),
	}, claimHeld
}

func (g *gogitStore) createIfAbsent(id, msg string) writeOutcome {
	return g.mintAndPush(id, msg, plumbing.ZeroHash)
}

func (g *gogitStore) updateFrom(id, oldSHA, msg string) writeOutcome {
	return g.mintAndPush(id, msg, plumbing.NewHash(oldSHA))
}

func (g *gogitStore) mintAndPush(id, msg string, old plumbing.Hash) writeOutcome {
	objs, tagSHA, err := gitcore.MintClaimTag(id, msg, time.Now())
	if err != nil {
		return writeUnverifiable
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	res, perr := gitcore.PushRefUpdate(ctx, gitcore.RefUpdate{
		URL: g.url, Auth: g.auth, Ref: g.refName(id), Old: old, New: tagSHA, Objects: objs,
	})
	if perr != nil {
		return writeUnverifiable
	}
	if res == gitcore.RefUpdateRejected {
		return writeRejected
	}
	return writeApplied
}

func (g *gogitStore) remove(id string) (writeOutcome, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	res, err := gitcore.DeleteRef(ctx, g.url, g.auth, g.refName(id))
	if err != nil {
		return writeUnverifiable, false
	}
	return writeApplied, res == gitcore.DeleteDone
}

func (g *gogitStore) list() ([]string, claimStatus) {
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		return nil, claimUnverifiable
	}
	var ids []string
	for _, r := range refs {
		name := r.Name().String()
		if strings.HasSuffix(name, "^{}") {
			continue // the peeled companion of an annotated tag — the tag ref carries the sha
		}
		if id, ok := strings.CutPrefix(name, refPrefix+"/"); ok {
			ids = append(ids, id)
		}
	}
	return ids, claimHeld
}

func (g *gogitStore) branchExists(branch string) (bool, bool) {
	if branch == "" || branch == "-" {
		return false, true
	}
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		return false, false
	}
	_, found := findRef(refs, "refs/heads/"+branch)
	return found, true
}

// findRef returns the sha of the exact ref name (skipping the peeled `^{}` companions), so an
// annotated tag resolves to its TAG OBJECT sha — the value the CAS `old` must carry — not its
// peeled target.
func findRef(refs []*plumbing.Reference, name string) (string, bool) {
	for _, r := range refs {
		if r.Name().String() == name {
			return r.Hash().String(), true
		}
	}
	return "", false
}

// --- repo / token resolution ------------------------------------------------

// resolveRepo returns owner/name: --repo when given, else the cwd's origin remote (read
// in-process via go-git, never by shelling git or gh).
func resolveRepo(repoFlag string) string {
	if s := strings.TrimSpace(repoFlag); s != "" {
		return s
	}
	_, slug := parseRemote(originRemoteURL())
	return slug
}

// originRemoteURL reads the "origin" remote URL from the current directory's checkout using
// go-git — no external git process, so it works on a native-Windows adopter with no git on PATH.
func originRemoteURL() string {
	r, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return ""
	}
	rem, err := r.Remote("origin")
	if err != nil {
		return ""
	}
	if urls := rem.Config().URLs; len(urls) > 0 {
		return urls[0]
	}
	return ""
}

// parseRemote extracts the host and the owner/name slug from a git remote URL in either
// scheme://host/owner/name(.git) form or the scp-like user@host:owner/name(.git) form. Either
// component may be "" when the URL does not carry it.
func parseRemote(raw string) (host, slug string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	var path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", ""
		}
		host = strings.ToLower(u.Hostname())
		path = u.Path
	} else if at := strings.Index(raw, "@"); at >= 0 {
		// scp-like: user@host:owner/name.git
		rest := raw[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			host = strings.ToLower(rest[:colon])
			path = rest[colon+1:]
		}
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		slug = parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return host, slug
}

func splitSlug(slug string) (owner, name string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(slug), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// resolveToken reads the forge credential from --token-file (a 0600 file) when given, else the
// first non-empty of GH_TOKEN / GITHUB_TOKEN / GITLAB_TOKEN — the same ambient credential the
// script and the gh-CLI port used. It never mints and never falls back to a CLI's cached login.
func resolveToken(tokenFile string) (string, error) {
	if tf := strings.TrimSpace(tokenFile); tf != "" {
		if err := verifyTokenFileMode(tf); err != nil {
			return "", err
		}
		b, err := os.ReadFile(tf)
		if err != nil {
			return "", fmt.Errorf("cannot read --token-file %s: %w", tf, err)
		}
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return "", fmt.Errorf("--token-file %s is empty", tf)
		}
		return tok, nil
	}
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no forge token: pass --token-file <0600 file> or set GH_TOKEN/GITHUB_TOKEN/GITLAB_TOKEN")
}

// verifyTokenFileMode enforces that a --token-file is a regular file locked to its owner (0600),
// so a world- or group-readable credential is refused rather than silently trusted. The
// permission-bit test is skipped on Windows, where os.FileMode's bits are synthetic.
func verifyTokenFileMode(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat --token-file %s: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("--token-file %s is not a regular file (mode %s)", path, fi.Mode())
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("--token-file %s is group/world accessible (mode %s); it must be 0600", path, fi.Mode().Perm())
	}
	return nil
}
