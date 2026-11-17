package deskkit

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

// slugSegmentRe validates ONE owner or repo path segment. It is the same charset the
// verb-local parsers accepted before this consolidation (`[A-Za-z0-9._-]`), spelled once.
// It deliberately excludes '@' and ':' so a userinfo or host token that leaks into the
// owner slot (the ambiguous half of a rewritten/hybrid remote) is REFUSED rather than
// returned as a bogus owner.
var slugSegmentRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidRepoSlug reports whether s is exactly one owner segment, one slash and one repo segment,
// each non-empty, each drawn from slugSegmentRe's charset, and neither a dot-segment ("." or
// ".."). It is the one shape check for an owner/name a caller hands in as a string, so a verb's
// argument check and a constructor's own re-check cannot drift apart.
func ValidRepoSlug(s string) bool {
	owner, name, ok := strings.Cut(s, "/")
	return ok && validSlugSegment(owner) && validSlugSegment(name)
}

func validSlugSegment(seg string) bool {
	return seg != "." && seg != ".." && slugSegmentRe.MatchString(seg)
}

// ParseRemoteRepo extracts the trailing owner/repo pair from a git remote URL in every
// shape git itself accepts, and from the rewritten/hybrid forms an `insteadOf` config or a
// bad URL-composition can bake into `remote.origin.url`:
//
//   - scp-like            git@host:owner/repo(.git)
//   - scp-like, no user   host:owner/repo(.git)                 (ssh alias host)
//   - ssh URL             ssh://git@host[:port]/owner/repo(.git)
//   - https URL           https://host/owner/repo(.git)
//   - bare path           owner/repo
//   - hybrid              https://github.com/git@host:owner/repo(.git)
//     — the shape produced when a base URL is prepended onto an scp string (issue 1470):
//     it is parsed LENIENTLY to owner/repo when the trailing pair is unambiguous, because
//     the rewrite is the operator's own config and the owner/repo tail is not in doubt.
//     A hybrid whose tail does NOT carry a clean owner/repo (e.g. the owner slot still
//     holds a `user@host` token) is REFUSED, naming the string and the expected shape.
//
// The parser reduces every shape to its `/`- and `:`-delimited segments and takes the last
// two as owner/repo — the invariant that holds across all of the above, since a git remote's
// path always ends in owner/repo. A scheme, a userinfo (`user@`), an ssh port, and a host
// alias all sort ahead of that trailing pair and drop out. The forge base URL is applied
// only when a caller BUILDS an API or clone URL from the parsed slug — never to the string
// parsed here, so a rewrite prefix cannot mangle the owner/repo it carries.
func ParseRemoteRepo(raw string) (owner, repo string, err error) {
	work := strings.TrimSpace(raw)
	shown := RedactRemoteUserinfo(raw)
	if work == "" {
		return "", "", fmt.Errorf("empty remote URL")
	}
	// Split the HOST off first, so the host is never mistaken for the owner (the reason a
	// malformed single-segment URL like https://host/repo must REFUSE, not resolve to
	// host/repo). What remains — pathPart — is everything after the host:
	//   - scheme://host[:port]/PATH   → PATH is after the first '/' past the scheme
	//   - [user@]host:PATH            → PATH is after the first ':' (scp-like)
	//   - PATH                        → a bare owner/repo carries no host
	var pathPart string
	if i := strings.Index(work, "://"); i >= 0 {
		rest := work[i+3:]
		if s := strings.Index(rest, "/"); s >= 0 {
			pathPart = rest[s+1:]
		}
	} else if c := strings.Index(work, ":"); c >= 0 {
		pathPart = work[c+1:]
	} else {
		pathPart = work
	}
	// Tokenize the path on BOTH '/' and ':'. The '/' split yields owner/repo; the ':' split
	// additionally rescues the issue-1470 HYBRID, whose path still carries an embedded scp
	// segment (git@host:owner/repo) after a base URL was prepended — reducing it LENIENTLY to
	// its trailing owner/repo. FieldsFunc omits empty fields, so trailing slashes drop away.
	segs := strings.FieldsFunc(pathPart, func(r rune) bool { return r == '/' || r == ':' })
	if len(segs) < 2 {
		return "", "", fmt.Errorf(
			"cannot parse owner/repo from remote %q: expected a trailing <owner>/<repo> "+
				"(git@host:owner/repo, ssh://host/owner/repo, https://host/owner/repo, or host:owner/repo)", shown)
	}
	owner = segs[len(segs)-2]
	repo = strings.TrimSuffix(segs[len(segs)-1], ".git")
	if !slugSegmentRe.MatchString(owner) || !slugSegmentRe.MatchString(repo) {
		// The pair is shown only when the remote carries no userinfo: a userinfo can split
		// across the ':' and '/' this parser cuts on, so a slot can hold part of it with no
		// '@' left for a per-value redaction to find.
		pair := fmt.Sprintf("%q/%q", owner, repo)
		if shown != raw {
			pair = "(not shown: the remote carries userinfo)"
		}
		return "", "", fmt.Errorf(
			"cannot parse owner/repo from remote %q: the trailing pair %s is not a clean "+
				"<owner>/<repo> (a host, userinfo, or rewrite prefix leaked into it)", shown, pair)
	}
	return owner, repo, nil
}

// RedactRemoteUserinfo renders a remote string for an error or log line with every userinfo
// span replaced by "<redacted>": in each '/'-delimited segment, everything up to and including
// the last '@' becomes "<redacted>@". That covers the URL authority
// (https://user:secret@host/...), the scp-like and bare forms (user:secret@host:path), and
// the scp segment a hybrid rewrite embeds in a path. The scheme, host, port and path stay
// visible, so the message still shows the remote's shape; a string with no '@' is returned
// unchanged. Every error ParseRemoteRepo returns names the remote through it, so callers
// surface the parser's error rather than re-echoing the raw string.
func RedactRemoteUserinfo(raw string) string {
	if !strings.Contains(raw, "@") {
		return raw
	}
	segs := strings.Split(raw, "/")
	for i, seg := range segs {
		if at := strings.LastIndexByte(seg, '@'); at >= 0 {
			segs[i] = "<redacted>" + seg[at:]
		}
	}
	return strings.Join(segs, "/")
}

// RemoteRepoSlug is ParseRemoteRepo returning the combined "owner/repo" slug — the form the
// verb call sites (deskwt, deskpr, deskreply) resolve their origin into.
func RemoteRepoSlug(raw string) (string, error) {
	owner, repo, err := ParseRemoteRepo(raw)
	if err != nil {
		return "", err
	}
	return owner + "/" + repo, nil
}

// SlugRemoteURL returns the first repository-local fetch URL of the named remote for a
// caller that decides a repository slug from it. It fails closed, wrapping
// gitcore.ErrRemoteScopeUnmerged, when config that read does not merge could change the
// URL git resolves (gitcore's PinnedRemoteURLs). The one worktree-scope URL it accepts is
// a pin of a single repository-local URL to one URL naming the same host and path (the
// shape a worktree transport repair writes; see samePinLocation), so the slug decided is
// the one git uses.
// Messages name the kind of declaration only, never a configured value.
func SlugRemoteURL(repo *gitcore.Repo, name string) (string, error) {
	local, pinned, err := repo.PinnedRemoteURLs(name)
	if err != nil {
		return "", err
	}
	if len(local) == 0 {
		return "", fmt.Errorf("remote %s: no fetch URL configured", name)
	}
	if pinned == "" {
		return local[0], nil
	}
	refuse := func(why string) error {
		return fmt.Errorf("remote %s: %w: worktree-scope remote.%s.url %s", name, gitcore.ErrRemoteScopeUnmerged, name, why)
	}
	if len(local) != 1 {
		return "", refuse("pins a multi-valued repository-local list")
	}
	want, lerr := RemoteRepoSlug(local[0])
	got, perr := RemoteRepoSlug(pinned)
	if lerr != nil || perr != nil || got != want || !samePinLocation(local[0], pinned) {
		return "", refuse("does not name the repository-local origin's repository")
	}
	return local[0], nil
}

// samePinLocation reports whether pinned reaches the same repository as local: the same
// full path and the same host, compared without userinfo, port, case of the host, or a
// trailing ".git". An ssh host that is not a well-known forge host may be a ~/.ssh/config
// alias, which only ssh can resolve to the https host a pin names; for that one shape the
// path alone is compared.
func samePinLocation(local, pinned string) bool {
	lh, lp, lok := remoteLocation(local)
	ph, pp, pok := remoteLocation(pinned)
	if !lok || !pok || lp != pp || pp == "" {
		return false
	}
	return lh == ph || (IsSSHTransport(local) && lh != "" && !IsWellKnownForgeHost(lh))
}

// remoteLocation splits a remote URL into the host git connects to (lower-cased, without
// userinfo or port; "" for a local path) and its path without surrounding slashes or a
// trailing ".git". ok is false for a scheme URL net/url cannot parse.
func remoteLocation(raw string) (host, path string, ok bool) {
	s := strings.TrimSpace(raw)
	switch slash := strings.IndexByte(s, '/'); {
	case strings.Contains(s, "://"):
		u, err := url.Parse(s)
		if err != nil {
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	case strings.IndexByte(s, ':') >= 0 && (slash < 0 || strings.IndexByte(s, ':') < slash):
		host, path, _ = strings.Cut(s, ":")
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
	default:
		path = s
	}
	return strings.ToLower(host), strings.TrimSuffix(strings.Trim(path, "/"), ".git"), true
}

// OriginRepoSlug reads dir's `origin` URL through SlugRemoteURL and parses it to
// "owner/repo". Repository-local fetch insteadOf rules apply; push-only rewrites and
// global config do not. err is non-nil when dir is not a git worktree, has no origin, the
// URL does not parse, or SlugRemoteURL refuses; callers that must fail closed surface it.
func OriginRepoSlug(dir string) (string, error) {
	repo, err := gitcore.Open(dir)
	if err != nil {
		return "", err
	}
	raw, err := SlugRemoteURL(repo, "origin")
	if err != nil {
		return "", err
	}
	return RemoteRepoSlug(raw)
}
