package main

// pushdest.go — deskpr's origin reads AS GIT ITSELF RESOLVES THEM (#1623).
//
// THE DEFECT CLASS. preflight used to decide the repo from go-git's RemoteURL("origin"), which
// reads only the repository's own config file, and then `git push -u origin` pushed wherever
// GIT resolved origin to: worktree- and global-scope values, empty-value list resets,
// url.<base>.insteadOf and pushInsteadOf rewrites, and every remote.origin.pushurl value. The two
// reads can disagree, so the gate passed an allowed repo while git pushed somewhere else.
//
// Both reads below are `git remote get-url [--push] --all origin`: a config read that contacts
// no remote and applies exactly the resolution the push itself applies, through the same argv
// seam (so the same environment) as the push.
//
// THE PUSH-DESTINATION RULE. create and update push, and deskpr pushes only as a role App. The
// list git resolves for the push must be EXACTLY ONE destination, and that destination must be
// an https URL naming the repo the origin gate decided on. Everything else is refused before
// the token mint, naming every value, where git read it from, and a one-line remedy scoped to
// this worktree:
//
//   - more than one value: `git push` pushes to EVERY value. An inherited value (from the
//     shared checkout's config) plus a worktree-scoped one is the common shape, and one of the
//     two can go out over SSH under the operator's key while the other goes out as the App.
//   - an SSH or scp-like destination (including one an insteadOf / pushInsteadOf rule
//     produced from an https value): authenticates with the machine's SSH agent, not the App.
//   - a non-URL value, such as a disabled-push sentinel: git would fail late with "does not
//     appear to be a git repository". The refusal names it instead.
//   - cleartext http, a remote-helper `<transport>::<address>`, or any other transport.
//   - an https URL naming a different repo than the fetch origin.
//
// The ONE other admitted shape is a LOCAL destination (a `file://` URL or an absolute path).
// It reaches no forge and presents no identity or credential to anyone, so the custody question
// this gate answers does not arise; the offline test fixtures push to a local bare repository
// through exactly that shape.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// splitURLList parses `git remote get-url --all` output: one URL per line, blank lines dropped.
func splitURLList(out string) []string {
	var urls []string
	for _, line := range strings.Split(out, "\n") {
		if u := strings.TrimSpace(line); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

// effectiveOriginURL returns origin's FETCH url as git resolves it (`git remote get-url --all
// origin`). A git error is Unverifiable (exit 6); a multi-valued list is Refused (exit 5): no
// single URL can stand for the list, so there is no one repo to gate on.
func effectiveOriginURL(dir string) (string, error) {
	out, err := git(dir, "remote", "get-url", "--all", "origin")
	if err != nil {
		return "", deskkit.Unverifiable("cannot resolve origin's URL as git does (`git remote get-url --all origin`)", err)
	}
	urls := splitURLList(out)
	switch len(urls) {
	case 0:
		return "", deskkit.Unverifiable("git resolved no URL for origin", nil)
	case 1:
		return urls[0], nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "refused: origin resolves to %d fetch URLs (a multi-valued remote.origin.url list), "+
			"so no single URL names the repo this PR belongs to — set exactly one:\n", len(urls))
		for i, u := range urls {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, redactURL(u))
		}
		b.WriteString(originConfigSources(dir))
		return "", deskkit.Refused(strings.TrimRight(b.String(), "\n"))
	}
}

// pushDestKind classifies one resolved push destination.
type pushDestKind int

const (
	pushHTTPS pushDestKind = iota
	pushLocal
	pushSSH
	pushCleartext
	pushHelper
	pushNotURL
	pushOtherTransport
)

func (k pushDestKind) String() string {
	switch k {
	case pushHTTPS:
		return "https"
	case pushLocal:
		return "a local repository (no forge, no credential)"
	case pushSSH:
		return "an SSH transport — authenticates with this machine's SSH agent key, not the App's credential"
	case pushCleartext:
		return "cleartext http — refused"
	case pushHelper:
		return "a remote-helper transport (<transport>::<address>) — refused"
	case pushNotURL:
		return "not an absolute URL or path (a disabled-push sentinel, a relative path, or a typo); git " +
			"would fail late with \"does not appear to be a git repository\""
	default:
		return "a transport deskpr does not push over"
	}
}

// classifyPushDest sorts a destination into the shape the gate decides on. It is string-level
// and total — it never fails on the hostile values it exists to classify.
func classifyPushDest(u string) pushDestKind {
	s := strings.TrimSpace(u)
	lower := strings.ToLower(s)
	if i := strings.Index(s, "::"); i > 0 && !strings.Contains(s[:i], "/") {
		return pushHelper
	}
	if i := strings.Index(s, "://"); i >= 0 {
		switch lower[:i] {
		case "https":
			if urlHost(s) == "" {
				return pushNotURL
			}
			return pushHTTPS
		case "http":
			return pushCleartext
		case "file":
			return pushLocal
		case "ssh", "git+ssh", "ssh+git":
			return pushSSH
		default:
			return pushOtherTransport
		}
	}
	if filepath.IsAbs(s) || strings.HasPrefix(s, "/") {
		return pushLocal
	}
	// scp-like [user@]host:path — git's rule: a colon before any slash. A single letter before
	// the colon is a Windows drive path, which git also treats as local.
	if colon := strings.IndexByte(s, ':'); colon > 0 {
		if slash := strings.IndexByte(s, '/'); slash < 0 || colon < slash {
			if colon == 1 {
				return pushLocal
			}
			return pushSSH
		}
	}
	return pushNotURL
}

// urlHost returns the host of a scheme://[userinfo@]host[:port]/path URL, lower-cased, or "".
func urlHost(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return ""
	}
	rest := u[i+3:]
	if s := strings.IndexByte(rest, '/'); s >= 0 {
		rest = rest[:s]
	}
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		rest = rest[at+1:]
	}
	if strings.HasPrefix(rest, "[") {
		if end := strings.IndexByte(rest, ']'); end >= 0 {
			return strings.ToLower(rest[:end+1])
		}
		return ""
	}
	if c := strings.LastIndexByte(rest, ':'); c >= 0 {
		rest = rest[:c]
	}
	return strings.ToLower(rest)
}

// redactURL strips userinfo from a URL before it reaches a message: an https remote can carry
// `user:token@`. Only the userinfo goes; host and path stay, so the line still says where.
func redactURL(raw string) string {
	u := strings.TrimSpace(raw)
	i := strings.Index(u, "://")
	if i < 0 {
		return u
	}
	scheme, rest := u[:i+3], u[i+3:]
	end := len(rest)
	if s := strings.IndexByte(rest, '/'); s >= 0 {
		end = s
	}
	if at := strings.LastIndexByte(rest[:end], '@'); at >= 0 {
		return scheme + "<redacted>@" + rest[at+1:]
	}
	return u
}

// pushDestinationGate is the push-destination gate for create and update. It runs after
// preflight (which decided repo from the fetch URL) and before the token mint, so a
// mis-configured remote costs no network round trip. It is not wired into `edit`, which pushes
// nothing.
func pushDestinationGate(dir, verb, repo, fetchURL string) error {
	out, err := git(dir, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"deskpr %s: cannot resolve where `git push origin` would push (`git remote get-url --push --all "+
				"origin` failed), so the push destination cannot be established", verb), err)
	}
	dests := splitURLList(out)
	if len(dests) == 0 {
		return deskkit.Unverifiable(fmt.Sprintf(
			"deskpr %s: git resolved no push URL for origin, so the push destination cannot be established", verb), nil)
	}

	kinds := make([]pushDestKind, len(dests))
	for i, d := range dests {
		kinds[i] = classifyPushDest(d)
	}
	var reason string
	switch {
	case len(dests) > 1:
		reason = fmt.Sprintf("git resolves origin's push URL list to %d values, and `git push` pushes to EVERY "+
			"one of them — deskpr pushes to exactly one", len(dests))
	case kinds[0] == pushLocal:
		return nil
	case kinds[0] != pushHTTPS:
		reason = "the one push destination git resolves is not an https URL"
	default:
		slug, perr := parseRepo(dests[0])
		if perr != nil {
			reason = "the push destination does not parse to an owner/repo (" + perr.Error() + ")"
		} else if slug != repo {
			reason = fmt.Sprintf("the push destination names %s, but origin's fetch URL names %s — deskpr pushes "+
				"only to the repo the origin gate decided on", slug, repo)
		} else {
			return nil
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "refused: deskpr %s will not push: %s. deskpr pushes only as a role App, so git's "+
		"resolved push URL list for origin must be exactly one https URL naming %s.\n", verb, reason, repo)
	b.WriteString("  git would push to:\n")
	for i, d := range dests {
		fmt.Fprintf(&b, "    %d. %s — %s\n", i+1, redactURL(d), kinds[i])
	}
	b.WriteString(originConfigSources(dir))
	b.WriteString(pushRemedy(dir, repo, fetchURL))
	return deskkit.Refused(strings.TrimRight(b.String(), "\n"))
}

// originConfigSources renders every config entry git builds origin's URL lists from — the url
// and pushurl keys and every insteadOf / pushInsteadOf rewrite rule — with the SCOPE and FILE
// each came from, so the refusal says which key to fix and where. One read (`git config
// --show-scope --show-origin --get-regexp`). It is a diagnostic inside a refusal that has
// already been decided; a failed read is reported as itself, never as "nothing set".
func originConfigSources(dir string) string {
	out, err := git(dir, "config", "--show-scope", "--show-origin", "--get-regexp",
		`^remote\.origin\.(push)?url$|^url\..*\.(push)?insteadof$`)
	if err != nil {
		if strings.TrimSpace(out) == "" {
			return "  where git reads it from: COULD NOT BE READ (" + err.Error() + ")\n"
		}
	}
	var b strings.Builder
	b.WriteString("  where git reads it from (scope, key = value, file):\n")
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			fmt.Fprintf(&b, "    %s\n", line)
			continue
		}
		scope, origin, kv := parts[0], parts[1], parts[2]
		key, val, _ := strings.Cut(kv, " ")
		shown := redactURL(val)
		if val == "" {
			shown = "(empty — resets the list inherited from the scopes above)"
		}
		fmt.Fprintf(&b, "    %-8s %s = %s   (%s)\n", scope, key, shown, origin)
	}
	return b.String()
}

// pushRemedy is the one-line fix, scoped to THIS worktree: it resets the inherited pushurl list
// with an empty value and adds the one https URL, both in the worktree's own config
// (extensions.worktreeConfig), so it never rewrites the shared checkout's config that every
// sibling worktree inherits.
func pushRemedy(dir, repo, fetchURL string) string {
	target := suggestHTTPSURL(repo, fetchURL)
	d := dir
	if d == "" {
		d = "."
	}
	enable := ""
	if v, err := git(dir, "config", "--bool", "--get", "extensions.worktreeConfig"); err != nil || strings.TrimSpace(v) != "true" {
		enable = fmt.Sprintf("git -C %s config extensions.worktreeConfig true && ", d)
	}
	var b strings.Builder
	b.WriteString("  Remedy (one line; worktree-scoped — it never touches the shared checkout's remote config):\n")
	fmt.Fprintf(&b, "    %sgit -C %s config --worktree --replace-all remote.origin.pushurl '' && "+
		"git -C %s config --worktree --add remote.origin.pushurl %s\n", enable, d, d, target)
	b.WriteString("  then configure the role App's credential helper for that URL in the same worktree scope.\n")
	for _, p := range insteadOfPrefixes(dir) {
		if strings.HasPrefix(target, p) {
			fmt.Fprintf(&b, "  NOTE: a url.<base>.insteadOf rule rewrites URLs starting %q, which includes that "+
				"URL — git would still not push to it as written. Use a spelling the rule does not match, or "+
				"remove the rule.\n", p)
		}
	}
	return b.String()
}

// suggestHTTPSURL builds the https URL the remedy names: the fetch URL itself when it is
// already https, else the https URL on the fetch URL's host, else a shaped placeholder (an ssh
// host alias is not a web host, so it is not guessed at).
func suggestHTTPSURL(repo, fetchURL string) string {
	if classifyPushDest(fetchURL) == pushHTTPS && !strings.Contains(redactURL(fetchURL), "<redacted>") {
		if slug, err := parseRepo(fetchURL); err == nil && slug == repo {
			return fetchURL
		}
	}
	host := ""
	s := strings.TrimSpace(fetchURL)
	if strings.Contains(s, "://") {
		host = urlHost(s)
	} else if colon := strings.IndexByte(s, ':'); colon > 0 {
		host = s[:colon]
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
	}
	if host == "" || !strings.Contains(host, ".") {
		host = "<forge-host>"
	}
	return "https://" + host + "/" + repo + ".git"
}

// insteadOfPrefixes returns the prefixes every url.<base>.insteadOf rule rewrites. (An explicit
// pushurl is exempt from pushInsteadOf, but not from insteadOf — so only insteadOf can undo the
// remedy.) Best-effort: a failed read yields none, and the remedy line still stands.
func insteadOfPrefixes(dir string) []string {
	out, err := git(dir, "config", "--get-regexp", `^url\..*\.insteadof$`)
	if err != nil {
		return nil
	}
	var ps []string
	for _, line := range strings.Split(out, "\n") {
		if i := strings.LastIndex(line, ".insteadof "); i >= 0 {
			if p := strings.TrimSpace(line[i+len(".insteadof "):]); p != "" {
				ps = append(ps, p)
			}
		}
	}
	return ps
}
