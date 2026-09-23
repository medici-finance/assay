package deskkit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// slugSegmentRe validates ONE owner or repo path segment. It is the same charset the
// verb-local parsers accepted before this consolidation (`[A-Za-z0-9._-]`), spelled once.
// It deliberately excludes '@' and ':' so a userinfo or host token that leaks into the
// owner slot (the ambiguous half of a rewritten/hybrid remote) is REFUSED rather than
// returned as a bogus owner.
var slugSegmentRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

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
				"(git@host:owner/repo, ssh://host/owner/repo, https://host/owner/repo, or host:owner/repo)", raw)
	}
	owner = segs[len(segs)-2]
	repo = strings.TrimSuffix(segs[len(segs)-1], ".git")
	if !slugSegmentRe.MatchString(owner) || !slugSegmentRe.MatchString(repo) {
		return "", "", fmt.Errorf(
			"cannot parse owner/repo from remote %q: the trailing pair %q/%q is not a clean "+
				"<owner>/<repo> (a host, userinfo, or rewrite prefix leaked into it)", raw, owner, repo)
	}
	return owner, repo, nil
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

// OriginRepoSlug reads dir's `origin` remote URL as the RAW configured value and parses it to
// "owner/repo". It reads through gitcore.RemoteURL, which returns the value `git config --get
// remote.origin.url` returns — the RAW config, with NO `insteadOf` expansion — deliberately
// NOT `git ls-remote --get-url`, which expands `url.<base>.insteadOf` locally and would hand
// this parser a rewritten string. err is non-nil when dir is not a git worktree, has no
// origin, or the URL does not parse; callers that must fail closed surface it, and callers
// that tolerate a miss (deriveRepoSlug) map it to "".
func OriginRepoSlug(dir string) (string, error) {
	repo, err := gitcore.Open(dir)
	if err != nil {
		return "", err
	}
	raw, err := repo.RemoteURL("origin")
	if err != nil {
		return "", err
	}
	return RemoteRepoSlug(raw)
}
