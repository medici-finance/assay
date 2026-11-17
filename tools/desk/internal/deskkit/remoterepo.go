package deskkit

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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
// span replaced by "<redacted>". For each '@', the masked span is the rest of the
// whitespace-delimited token before it: it runs back to the nearest preceding whitespace (or
// the start of the string), and stops short of that only to keep an opening quote or bracket
// that begins the token and a URL scheme ("https://", at most behind prose ending in an
// opening quote, bracket or '='). Nothing else inside the token is a boundary: a '/', a quote,
// a bracket, a backtick or a "://" that falls inside a userinfo is masked with it, so a
// malformed password holding any of them keeps no part of itself visible, and a list of
// remotes joined by ", " or newlines is masked remote by remote. That covers the URL authority
// (https://user:secret@host/...), a userinfo that is a credential alone
// (https://secret@host/...), the scp-like and bare forms (user:secret@host:path), and the scp
// segment a hybrid rewrite embeds in a path, which is masked back to the scheme together with
// the host before it. The scheme, the host after the '@', the port and the path stay visible,
// so the message still shows the remote's shape; a string with no '@' is returned unchanged.
// Every error ParseRemoteRepo returns names the remote through it, so callers surface the
// parser's error rather than re-echoing the raw string. Prose glued to a remote with no
// whitespace between them is masked with the userinfo: over-redaction is the accepted
// failure direction.
//
// It is the ONE userinfo redactor of the desk tools: redactguard_test.go fails on a second
// definition. A caller rendering a child process's stderr passes it through StripControl
// first, so a terminal escape sequence inside a userinfo cannot move the span.
//
// It redacts USERINFO only. A credential carried anywhere else is out of its scope: in a query
// string or fragment (https://host/o/r?access=<secret>), in a path with no '@', behind a
// look-alike character that is not the ASCII '@' (a fullwidth at sign, say), or in the part of
// a userinfo before an unencoded whitespace character.
func RedactRemoteUserinfo(raw string) string {
	return redactUserinfo(raw, nil)
}

// RedactRemoteArgv joins argv with spaces for an error line, each argument passed through
// RedactRemoteUserinfo, so a remote URL handed to git as an argument (ls-remote, set-url) is
// never echoed with its userinfo when the command fails.
func RedactRemoteArgv(argv []string) string {
	shown := make([]string, len(argv))
	for i, a := range argv {
		shown[i] = RedactRemoteUserinfo(a)
	}
	return strings.Join(shown, " ")
}

// redactUserinfo is RedactRemoteUserinfo with one exception hook: a userinfo span for which
// keep returns true stays visible. A nil keep masks every span.
func redactUserinfo(raw string, keep func(string) bool) string {
	if !strings.Contains(raw, "@") {
		return raw
	}
	mask := make([]bool, len(raw))
	for at := 0; at < len(raw); at++ {
		if raw[at] != '@' {
			continue
		}
		start := userinfoStart(raw, at)
		if start == at || (keep != nil && keep(raw[start:at])) {
			continue
		}
		for j := start; j < at; j++ {
			mask[j] = true
		}
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if !mask[i] {
			b.WriteByte(raw[i])
			continue
		}
		if i == 0 || !mask[i-1] {
			b.WriteString("<redacted>")
		}
	}
	return b.String()
}

// RedactRemoteConfigKey renders a git config key for an error or log line with every userinfo
// span masked as RedactRemoteUserinfo masks it. A url.<base>.insteadOf / .pushInsteadOf key
// carries a remote base INSIDE the key, so a listing of config keys is as much a credential
// surface as a listing of their values. The "url." prefix stays visible, and so does a
// userinfo that is exactly the bare SSH account name "git" (the push gate's RedactRemoteURL rule: it
// names the transport's account and carries no credential), so a rule key still reads as the
// rewrite it is.
func RedactRemoteConfigKey(key string) string {
	if rest, ok := strings.CutPrefix(key, "url."); ok {
		return "url." + RedactRemoteURL(rest)
	}
	return RedactRemoteURL(key)
}

// userinfoStart is where the userinfo ending at s[at] == '@' begins (see
// RedactRemoteUserinfo): the start of its whitespace-delimited token, moved past a leading
// opening quote or bracket, and past a URL scheme when the token carries one.
func userinfoStart(s string, at int) int {
	head := s[:at]
	start := 0
	if k := strings.LastIndexFunc(head, unicode.IsSpace); k >= 0 {
		_, size := utf8.DecodeRuneInString(head[k:])
		start = k + size
	}
	for start < at && strings.IndexByte(tokenOpeners, head[start]) >= 0 {
		start++
	}
	tok := head[start:]
	k := strings.Index(tok, "://")
	if k < 0 {
		return start
	}
	// The text before the FIRST "://" must be a scheme, at most behind prose ending in an
	// opener or '='. Anything else ("user:pa://ss@host") puts the "://" inside the userinfo,
	// and the whole token is masked.
	j := k
	for j > 0 && isSchemeByte(tok[j-1]) {
		j--
	}
	if j == k || (j > 0 && strings.IndexByte(tokenOpeners+"=", tok[j-1]) < 0) {
		return start
	}
	return start + k + 3
}

// tokenOpeners are the characters that may open a quoted or bracketed remote.
const tokenOpeners = "'\"`(<["

func isSchemeByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'
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
// full path and the same host, compared byte for byte after an ASCII-only case fold, without
// userinfo, port or a trailing ".git". A host with a non-ASCII byte never matches: Unicode
// case mapping folds some non-ASCII capitals onto ASCII letters that DNS treats as another
// name. The pin may change transport (ssh to https) but never to a cleartext scheme (http,
// git://). The port is not compared: a pin moves ssh's port 22 to https's 443 on the same
// host, so ports only differ across the transports the pin exists to switch. An ssh host that
// is not a well-known forge host may be a ~/.ssh/config alias, which only ssh can resolve to
// the network host a pin names; for that one shape the path alone is compared, and the pin
// must still name a network host.
func samePinLocation(local, pinned string) bool {
	ls, lh, lp, lok := remoteLocation(local)
	ps, ph, pp, pok := remoteLocation(pinned)
	if !lok || !pok || lp != pp || pp == "" || (ps != ls && (ps == "http" || ps == "git")) {
		return false
	}
	return lh == ph || (IsSSHTransport(local) && lh != "" && ph != "" && !IsWellKnownForgeHost(lh))
}

// remoteLocation splits a remote URL into its lower-cased scheme ("" for scp-like or a local
// path), the host git connects to (ASCII lower-cased, without userinfo or port; "" for a local
// path) and its path without surrounding slashes or a trailing ".git". ok is false for a
// scheme URL net/url cannot parse, and for a host with any byte >= 0x80.
func remoteLocation(raw string) (scheme, host, path string, ok bool) {
	s := strings.Trim(raw, " \t\r\n")
	switch slash := strings.IndexByte(s, '/'); {
	case strings.Contains(s, "://"):
		u, err := url.Parse(s)
		if err != nil {
			return "", "", "", false
		}
		scheme, host, path = u.Scheme, u.Hostname(), u.Path
	case strings.IndexByte(s, ':') >= 0 && (slash < 0 || strings.IndexByte(s, ':') < slash):
		host, path, _ = strings.Cut(s, ":")
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
	default:
		path = s
	}
	host, err := NormalizeForgeHost(host)
	if err != nil {
		return "", "", "", false
	}
	return strings.ToLower(scheme), host, strings.TrimSuffix(strings.Trim(path, "/"), ".git"), true
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
