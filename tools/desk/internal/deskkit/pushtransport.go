package deskkit

// pushtransport.go — the PUSH-transport custody gate.
//
// THE FAULT IT EXISTS TO STOP. A worker worktree cut from a shared checkout inherits that
// checkout's remote. When the remote is an SSH URL (`ssh://git@host/owner/name` or the
// scp-like `git@host:owner/name`), a plain `git push` from that worktree authenticates with
// whatever key the machine's SSH agent happens to hold — in practice a HUMAN's key — even
// though every commit on the branch was authored inline as the role App. The forge then
// records the human as the branch creator, and the App's permission envelope (the
// workflows-scope refusal, an App-scoped ruleset, a bot-author-keyed workflow) is silently
// bypassed. Nothing in the run looks wrong: the push succeeds, the commits carry the App's
// authorship, and only the forge's own record of WHO pushed disagrees.
//
// That is precisely the ambient-identity lane the forge-side custody ruling retired. A tool
// that pushes under whatever key the checkout happens to carry re-opens it, so the refusal
// below is the layer that survives a mis-configured checkout on a machine nobody audited.
//
// WHAT IS GATED, AND WHAT IS NOT.
//   - Only the PUSH transport. Fetch over SSH is untouched: a read carries no identity the
//     forge records against a ref, and gating it would break every offline fixture and every
//     checkout that legitimately fetches over a key.
//   - Only a session that presents a BOT identity — $DESK_LOOP resolving to an App role. A
//     human at a terminal with no loop identity pushes under their own key, which is correct
//     and is what the SSH remote is for. With $DESK_LOOP unset this gate is inert.
//   - An https push URL is allowed, and carries a NOTICE (never a refusal) when no App
//     credential helper is configured for it: https with an ambient keychain helper is the
//     SAME ambient-identity shape one layer along, but the evidence is weaker (a helper this
//     code cannot recognise may still be the App's), so it is could-not-check, not a STOP.
//
// THREE-STATE. A config read that fails, or a remote with no URL at all, is Unverifiable
// (exit 6) — never "no SSH found, carry on". A $DESK_LOOP this process cannot resolve to a
// role is a stderr NOTICE saying the gate DID NOT RUN, never a silent pass.

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// PushTransportInput is everything CheckPushTransport needs. The git read is a SEAM
// (ConfigZ) rather than an exec call of its own so the two call sites keep their single
// argv-recording seam, and so a test drives the gate off a config fixture instead of a
// process.
type PushTransportInput struct {
	// Tool and Verb name the caller in every message ("deskpr create", "deskwt add").
	Tool string
	Verb string
	// Dir is the worktree the push would leave from — quoted in the remedy line.
	Dir string
	// Remote is the remote the push targets; empty means "origin".
	Remote string
	// ConfigZ returns the output of `git config --list -z` run in Dir.
	//
	// ONE read, NUL-delimited, exit 0. `--get-all <key>` would need three calls and an
	// exit-1-means-absent convention at each call site — and a caller that reads exit 1 as
	// "unset" cannot tell it apart from a git that failed, which is the could-not-check
	// rounded up to a pass this file exists to refuse. `-z` also survives a credential
	// helper whose value is a multi-line inline shell function, which the line-oriented
	// `--list` does not.
	ConfigZ func() (string, error)
	// Stderr receives NOTICE lines; nil means discard.
	Stderr io.Writer
}

// CheckPushTransport refuses (exit 5) when the resolved PUSH URL of Remote is an SSH URL
// and this session acts as a role App. It returns nil — after at most one stderr NOTICE —
// in every other reachable state, and Unverifiable (exit 6) when the transport cannot be
// established at all.
func CheckPushTransport(in PushTransportInput) error {
	remote := strings.TrimSpace(in.Remote)
	if remote == "" {
		remote = "origin"
	}
	stderr := in.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	who := strings.TrimSpace(in.Tool + " " + in.Verb)

	// 1. Does this session act as a bot? With no loop identity presented there is no App
	//    identity to protect, and the gate is inert by design (see the header).
	raw := strings.TrimSpace(os.Getenv(loopEnv))
	if raw == "" {
		return nil
	}
	role, loop, rerr := SessionTokenRole(in.Tool)
	if rerr != nil {
		// could-not-check: say the gate did not run rather than letting silence read as a pass.
		fmt.Fprintf(stderr, "%s: NOTICE $%s=%q does not resolve to an App role, so whether this session "+
			"pushes as a bot COULD NOT BE ESTABLISHED and the push-transport gate did NOT run — "+
			"that is could-not-check, never a pass (%v)\n", who, loopEnv, raw, rerr)
		return nil
	}

	// 2. Read the worktree's effective config once.
	if in.ConfigZ == nil {
		return Unverifiable(fmt.Sprintf(
			"%s: no git-config reader wired for the push-transport gate, so the push transport for %q "+
				"cannot be established", who, remote), nil)
	}
	out, cerr := in.ConfigZ()
	if cerr != nil {
		return Unverifiable(fmt.Sprintf(
			"%s: cannot read git config in %s, so whether the push to %q would go out over SSH under the "+
				"%s App's identity cannot be established", who, orDot(in.Dir), remote, role), cerr)
	}
	cfg := parseConfigZ(out)

	// 3. Resolve the PUSH url the way git does: every remote.<name>.pushurl if any is set,
	//    otherwise every remote.<name>.url. Both keys are multi-valued, and a push fans out
	//    to ALL of them — so ANY SSH value is the refusal, not just the first.
	key := "remote." + remote + ".pushurl"
	urls := cfg[key]
	if len(urls) == 0 {
		key = "remote." + remote + ".url"
		urls = cfg[key]
	}
	if len(urls) == 0 {
		return Unverifiable(fmt.Sprintf(
			"%s: remote %q has no url or pushurl configured in %s, so the push transport cannot be "+
				"established", who, remote, orDot(in.Dir)), nil)
	}

	for _, u := range urls {
		if !isSSHTransport(u) {
			continue
		}
		return Refused(fmt.Sprintf(
			"refused: %s would push to %q over an SSH transport (%s = %s), but this session acts as the %s "+
				"App (%s=%s). An SSH push authenticates with whatever key this machine's agent holds — a "+
				"human's key — so the forge records the HUMAN as the branch author and the App's permission "+
				"envelope is bypassed, however the commits are authored. Fetch over SSH stays allowed; only "+
				"the push transport is gated. Remedy (one line, in this worktree):\n"+
				"  git -C %s remote set-url --push %s %s\n"+
				"then configure the %s App's credential helper for that URL.",
			who, remote, key, u, role, loopEnv, loop,
			orDot(in.Dir), remote, orSuggestHTTPS(httpsEquivalent(u), remote), role))
	}

	// 4. Not SSH. An https push is the sanctioned transport — but it only carries the App's
	//    identity if an App credential helper answers for it. A bare keychain helper (or
	//    none at all) hands the push whatever ambient credential the machine holds, which is
	//    the same ambient-identity shape one layer along. The evidence is weaker than an SSH
	//    URL — a helper this code does not recognise may well be the App's — so this is a
	//    NOTICE, never a refusal.
	if !anyHTTPTransport(urls) {
		return nil
	}
	helpers := credentialHelpers(cfg)
	if hasAppCredentialHelper(helpers) {
		return nil
	}
	// An empty value is git's list RESET, not a helper — rendering it would print a bare
	// trailing comma and read as "something is configured" when nothing is.
	var named []string
	for _, h := range helpers {
		if strings.TrimSpace(h) != "" {
			named = append(named, h)
		}
	}
	shown := "none configured"
	if len(named) > 0 {
		shown = strings.Join(named, ", ")
	}
	fmt.Fprintf(stderr, "%s: NOTICE the push URL for %q is https (%s) but no App credential helper is "+
		"configured in %s (credential.helper: %s) — the push will authenticate with whatever ambient "+
		"credential this machine holds, which may not be the %s App. That is could-not-check, not a pass: "+
		"configure the %s App's credential helper for this URL, or confirm the one in place is it.\n",
		who, remote, strings.Join(urls, ", "), orDot(in.Dir), shown, role, role)
	return nil
}

// orSuggestHTTPS falls back to a shaped placeholder when an SSH URL cannot be rewritten
// into its https twin — a remedy line that names no URL at all sends the reader to look one
// up, which is exactly the round trip the message exists to save.
func orSuggestHTTPS(u, remote string) string {
	if u != "" {
		return u
	}
	return "https://<host>/<owner>/<name>.git   # the https URL of " + remote
}

// parseConfigZ parses `git config --list -z` into key → values, preserving git's order.
// Each NUL-delimited record is "key\nvalue"; a record with no newline is a valueless key
// (`[section] flag`), recorded as an empty value so an explicit `credential.helper=` reset
// is visible rather than dropped.
func parseConfigZ(out string) map[string][]string {
	cfg := map[string][]string{}
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		key, val, hasVal := strings.Cut(rec, "\n")
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !hasVal {
			val = ""
		}
		cfg[key] = append(cfg[key], val)
	}
	return cfg
}

// isSSHTransport reports whether a git remote URL is carried over SSH, by git's own rules:
// an explicit ssh-family scheme, or the scp-like `[user@]host:path` shorthand.
//
// The scp-like rule is git's: a colon that appears before any slash. Everything else — an
// https/http/git/file scheme, a bare absolute or relative path — is not SSH. A Windows drive
// path (`C:\src\repo`, `C:/src/repo`) also carries a colon before any slash, so a
// single-letter host is excluded explicitly; git makes the same exception.
func isSSHTransport(u string) bool {
	s := strings.TrimSpace(u)
	if s == "" {
		return false
	}
	if i := strings.Index(s, "://"); i >= 0 {
		switch strings.ToLower(s[:i]) {
		case "ssh", "git+ssh", "ssh+git":
			return true
		default:
			return false
		}
	}
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return false
	}
	if slash := strings.IndexByte(s, '/'); slash >= 0 && slash < colon {
		return false
	}
	host := s[:colon]
	if host == "" {
		return false
	}
	if len(host) == 1 && isDriveLetter(host[0]) {
		return false
	}
	return true
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// anyHTTPTransport reports whether any of the resolved push URLs is an http(s) one — the
// only family the credential-helper NOTICE is about. A file:// or local-path push (every
// offline fixture) consults no credential helper, so it is silent.
func anyHTTPTransport(urls []string) bool {
	for _, u := range urls {
		s := strings.ToLower(strings.TrimSpace(u))
		if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
			return true
		}
	}
	return false
}

// httpsEquivalent rewrites an SSH URL into the https URL that reaches the same repository,
// so the remedy line can be pasted rather than derived. It returns "" when the URL does not
// decompose into a host and a path.
//
// An ssh PORT is dropped: the house's `:443` SSH-over-https-port dodge is a property of the
// SSH transport, not of the https one, and carrying it across produces a URL that does not
// resolve. An IPv6 literal keeps its brackets and its inner colons.
func httpsEquivalent(u string) string {
	s := strings.TrimSpace(u)
	var authority, path string
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if slash := strings.IndexByte(s, '/'); slash >= 0 {
			authority, path = s[:slash], s[slash+1:]
		} else {
			authority = s
		}
	} else {
		colon := strings.IndexByte(s, ':')
		if colon < 0 {
			return ""
		}
		authority, path = s[:colon], s[colon+1:]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		authority = authority[at+1:]
	}
	host := authority
	if strings.HasPrefix(host, "[") {
		if end := strings.IndexByte(host, ']'); end >= 0 {
			host = host[:end+1]
		}
	} else if c := strings.LastIndexByte(host, ':'); c >= 0 {
		host = host[:c]
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "~")
	path = strings.TrimPrefix(path, "/")
	if host == "" || path == "" {
		return ""
	}
	return "https://" + host + "/" + path
}

// credentialHelpers collects every configured credential helper — the global
// `credential.helper` list and any url-scoped `credential.<url>.helper`. The global list
// keeps git's own order; the url-scoped keys are sorted by key, so the NOTICE line this
// feeds is byte-stable across runs rather than riding Go's randomised map order.
func credentialHelpers(cfg map[string][]string) []string {
	out := append([]string(nil), cfg["credential.helper"]...)
	var scoped []string
	for key := range cfg {
		if key == "credential.helper" {
			continue
		}
		if strings.HasPrefix(key, "credential.") && strings.HasSuffix(key, ".helper") {
			scoped = append(scoped, key)
		}
	}
	sort.Strings(scoped)
	for _, key := range scoped {
		out = append(out, cfg[key]...)
	}
	return out
}

// ambientCredentialHelpers are the stock helpers that answer with whatever credential the
// MACHINE holds for a host, rather than with a credential this session minted. A checkout
// configured with only these hands an https push the same ambient identity an SSH agent
// would — one layer along, and the reason the NOTICE fires.
var ambientCredentialHelpers = map[string]bool{
	"osxkeychain":   true,
	"manager":       true,
	"manager-core":  true,
	"wincred":       true,
	"libsecret":     true,
	"gnome-keyring": true,
	"store":         true,
	"cache":         true,
	"netrc":         true,
}

// hasAppCredentialHelper reports whether any configured helper is something OTHER than a
// stock ambient one — which is the closest this code can come to "the App's helper is
// wired", and deliberately errs toward silence: the house pattern is an inline
// `!f(){ …token file… }; f` helper, which no stock name matches, so a real App helper is
// never noticed at all. An empty value is git's list RESET, not a helper, and never counts.
func hasAppCredentialHelper(helpers []string) bool {
	for _, h := range helpers {
		v := strings.TrimSpace(h)
		if v == "" {
			continue
		}
		name := v
		if i := strings.IndexAny(name, " \t"); i >= 0 {
			name = name[:i]
		}
		if !ambientCredentialHelpers[strings.ToLower(name)] {
			return true
		}
	}
	return false
}
