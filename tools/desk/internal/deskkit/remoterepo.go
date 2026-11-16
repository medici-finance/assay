package deskkit

import (
	"fmt"
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
	// Every refusal below quotes the URL, and a remote URL can carry a credential in its
	// userinfo (`https://user:<secret>@host/...`). The quoted copy is always the REDACTED
	// one: the raw string never reaches an error, so no caller printing it can leak it.
	shown := RedactURLUserinfo(raw)
	if len(segs) < 2 {
		return "", "", fmt.Errorf(
			"cannot parse owner/repo from remote %q: expected a trailing <owner>/<repo> "+
				"(git@host:owner/repo, ssh://host/owner/repo, https://host/owner/repo, or host:owner/repo)", shown)
	}
	owner = segs[len(segs)-2]
	repo = strings.TrimSuffix(segs[len(segs)-1], ".git")
	if !slugSegmentRe.MatchString(owner) || !slugSegmentRe.MatchString(repo) {
		// The rejected pair is the very case where userinfo leaked into a segment. The ':'
		// split above can leave a password alone in a segment (`pw@host`), so any segment
		// holding an '@' is quoted with everything before its last '@' masked.
		return "", "", fmt.Errorf(
			"cannot parse owner/repo from remote %q: the trailing pair %q/%q is not a clean "+
				"<owner>/<repo> (a host, userinfo, or rewrite prefix leaked into it)",
			shown, maskSegmentUserinfo(owner), maskSegmentUserinfo(repo))
	}
	return owner, repo, nil
}

// maskSegmentUserinfo masks everything before the last '@' of one parsed path segment.
func maskSegmentUserinfo(seg string) string {
	if at := strings.LastIndexByte(seg, '@'); at >= 0 {
		return redactedUserinfo + seg[at:]
	}
	return seg
}

// redactedUserinfo is what RedactURLUserinfo puts where a userinfo stood.
const redactedUserinfo = "***"

// RedactURLUserinfo returns s with the userinfo of every URL it contains replaced by "***",
// so a remote URL can be quoted in a refusal or a log line without carrying the credential an
// https remote may embed (`https://user:<secret>@host/owner/repo`). It is the ONE redaction
// a message that echoes an origin or remote URL applies; host and path stay, so the message
// still says which remote it means.
//
// A userinfo is the run of characters before an '@' back to the nearest '/' or whitespace.
// It is redacted when it is the authority of a `scheme://` URL (the whole userinfo, user
// included, because a bare secret is a valid username: `https://<secret>@host/...`), or
// when it carries a ':' (a `user:password` pair anywhere, including one embedded in a
// rewritten hybrid such as `https://host/user:pw@alias:owner/repo`). A run that directly
// continues a redacted userinfo (a password holding a literal '@') is dropped with it. An
// scp-like login with no password (`git@host:owner/repo`) is a login name, not a secret,
// and is kept so the message still names the URL git resolved.
//
// It is string-level and total: it never fails on the malformed URLs it exists to quote, so
// it does not go through net/url (which rejects many shapes git accepts). It is biased to
// over-redact: a masked login costs a look-up, a printed credential is an incident.
func RedactURLUserinfo(s string) string {
	if !strings.Contains(s, "@") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i, prevRedacted := 0, false
	for {
		rel := strings.IndexByte(s[i:], '@')
		if rel < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		at := i + rel
		start := i
		if d := strings.LastIndexAny(s[i:at], "/ \t\r\n"); d >= 0 {
			start = i + d + 1
		}
		tok := s[start:at]
		afterScheme := start >= 3 && s[start-3:start] == "://"
		continues := prevRedacted && start == i
		switch {
		case continues:
			// Still inside the userinfo just masked: drop this run, keep nothing of it.
		case tok != "" && (afterScheme || strings.Contains(tok, ":")):
			b.WriteString(s[i:start])
			b.WriteString(redactedUserinfo)
			b.WriteByte('@')
			prevRedacted = true
			i = at + 1
			continue
		default:
			b.WriteString(s[i : at+1])
			prevRedacted = false
			i = at + 1
			continue
		}
		i = at + 1
	}
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

// OriginRepoSlug reads dir's repository-local `origin` URL through gitcore.RemoteURL
// and parses it to "owner/repo". Repository-local fetch insteadOf rules apply; push-only
// rewrites and global/worktree config do not. err is non-nil when dir is not a git worktree, has no
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
