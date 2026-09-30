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
// THE URL JUDGED IS THE ONE GIT WILL USE. The configured `remote.<name>.pushurl` / `.url` is
// not what leaves the machine: git applies `url.<base>.pushInsteadOf` and
// `url.<base>.insteadOf` before it connects, so an https remote can go out as an SSH push.
// The gate therefore decides from `git remote get-url --push --all <remote>` (the PushURLs
// seam) — git's own resolution, both rewrites applied, no remote contacted — and, when a
// rewrite is what produced the SSH URL, names the rule in the refusal (#884).
//
// THREE-STATE. A config read that fails, or a remote with no URL at all, is Unverifiable
// (exit 6) — never "no SSH found, carry on". "No URL at all" includes the shape real git
// produces for it: no non-blank url/pushurl configured and the remote resolving only to its
// own bare name. A $DESK_LOOP this process cannot resolve to a
// role is a stderr NOTICE saying the gate DID NOT RUN, never a silent pass.

import (
	"fmt"
	"io"
	"os"
	"regexp"
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
	// PushURLs returns the output of `git remote get-url --push --all <Remote>` run in Dir:
	// the URLs a push will ACTUALLY use, one per line, after git has applied
	// `url.<base>.pushInsteadOf` and `url.<base>.insteadOf`. It reads local config only and
	// contacts no remote. The gate decides from these, never from the configured strings —
	// a configured https URL that a rewrite turns into SSH is the false pass it exists to
	// refuse (#884). A nil reader is could-not-check (exit 6), never a fall-back to ConfigZ.
	PushURLs func() (string, error)
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

	// 3. The CONFIGURED push urls: every remote.<name>.pushurl if any is set, otherwise every
	//    remote.<name>.url. They are what a refusal quotes and what a rewrite rule is traced
	//    back to — never what the decision is made from (step 4).
	key := "remote." + remote + ".pushurl"
	configured := cfg[key]
	explicitPush := len(configured) > 0
	if !explicitPush {
		key = "remote." + remote + ".url"
		configured = cfg[key]
	}

	// 4. The EFFECTIVE push urls, as git resolves them: rewrites applied, and every value —
	//    a push fans out to ALL of them, so ANY SSH value is the refusal, not just the first.
	if in.PushURLs == nil {
		return Unverifiable(fmt.Sprintf(
			"%s: no push-URL reader wired for the push-transport gate, so the URL git would push %q to "+
				"(after url.<base>.insteadOf / pushInsteadOf rewrites) cannot be established", who, remote), nil)
	}
	rawURLs, perr := in.PushURLs()
	if perr != nil {
		return Unverifiable(fmt.Sprintf(
			"%s: cannot resolve the push URL of %q in %s (git remote get-url --push --all), so whether the "+
				"push would go out over SSH under the %s App's identity cannot be established",
			who, remote, orDot(in.Dir), role), perr)
	}
	urls := splitURLLines(rawURLs)
	// A remote with no url at all does NOT resolve to nothing with real git: a remote that
	// exists only through another key (a fetch refspec, say) or whose url is set empty is
	// resolved to its bare NAME, which git would then push to as a local path. So an empty
	// resolution is not the only no-url shape — a resolution to exactly [remote] with no
	// non-blank configured value is the same could-not-check, and rounding it to "not SSH,
	// carry on" is the silent pass this gate exists to refuse. (A legacy .git/remotes file
	// has no config values either, but git resolves it to its real url, so it is unaffected.)
	if len(urls) == 0 || (allBlank(configured) && len(urls) == 1 && urls[0] == remote) {
		return Unverifiable(fmt.Sprintf(
			"%s: remote %q has no url or pushurl configured in %s (git resolves it only to its bare name), so "+
				"the push transport cannot be established", who, remote, orDot(in.Dir)), nil)
	}

	for _, u := range urls {
		if !isSSHTransport(u) {
			continue
		}
		rw := traceRewrite(cfg, configured, explicitPush, u)
		if rw.ruleKey == "" {
			// No rewrite involved (or none this code can attribute): the configured value is
			// itself SSH, and pointing the push url at https is the remedy — unless an insteadOf
			// rule would rewrite that https url straight back to SSH (pushURLRemedy checks).
			shown := u
			if !containsString(configured, u) {
				from := strings.Join(configured, ", ")
				if from == "" {
					// Nothing under remote.<name>.url / .pushurl in `git config --list` — e.g. a
					// legacy .git/remotes file. Say so rather than print an empty value.
					from = "(no " + key + " in git config — a legacy remotes file?)"
				}
				shown = fmt.Sprintf("%s, which git resolves to %s through a url rewrite this gate could not "+
					"attribute — run `git -C %s config --show-origin --get-regexp '^url\\.'` to find it",
					from, u, orDot(in.Dir))
			}
			target := u
			if len(configured) == 1 && isSSHTransport(configured[0]) {
				target = configured[0]
			}
			replace := ""
			if containsString(configured, target) {
				replace = target
			}
			return Refused(fmt.Sprintf(
				"refused: %s would push to %q over an SSH transport (%s = %s), but this session acts as the %s "+
					"App (%s=%s). An SSH push authenticates with whatever key this machine's agent holds — a "+
					"human's key — so the forge records the HUMAN as the branch author and the App's permission "+
					"envelope is bypassed, however the commits are authored. Fetch over SSH stays allowed; only "+
					"the push transport is gated. Remedy: %s",
				who, remote, key, shown, role, loopEnv, loop,
				pushURLRemedy(cfg, in.Dir, remote, httpsTargetFor(target, u, remote), replace, role, "", false)))
		}
		return Refused(fmt.Sprintf(
			"refused: %s would push to %q over an SSH transport: %s = %s is rewritten by the rule %s = %s "+
				"into %s, and git pushes to the REWRITTEN url. This session acts as the %s App (%s=%s); an SSH "+
				"push authenticates with whatever key this machine's agent holds — a human's key — so the "+
				"forge records the HUMAN as the branch author and the App's permission envelope is bypassed, "+
				"however the commits are authored. Fetch over SSH stays allowed; only the push transport is "+
				"gated. Remedy: %s\n  (find where the rule is set: git -C %s config --show-origin --get-all %s)",
			who, remote, key, rw.from, rw.ruleKey, rw.prefix, u, role, loopEnv, loop,
			rw.remedy(cfg, in.Dir, remote, u, role), orDot(in.Dir), rw.ruleKey))
	}

	// 5. Not SSH. An https push is the sanctioned transport — but it only carries the App's
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

// splitURLLines splits `git remote get-url --push --all` output into its URLs, one per
// non-blank line.
func splitURLLines(out string) []string {
	var urls []string
	for _, line := range strings.Split(out, "\n") {
		if u := strings.TrimSpace(line); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

// allBlank reports whether no value in list carries anything but whitespace — an empty list
// included.
func allBlank(list []string) bool {
	for _, v := range list {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// rewriteTrace names the url rewrite rule that turned a configured push URL into the
// effective one. A zero ruleKey means no rule could be attributed.
type rewriteTrace struct {
	ruleKey string // url.<base>.insteadOf or url.<base>.pushInsteadOf, git's own spelling
	prefix  string // the rule's value: the URL prefix it replaces
	from    string // the configured URL it rewrote
	push    bool   // pushInsteadOf (push-only) rather than insteadOf
}

// remedy is the one fix that actually clears the rewrite. An explicit https pushurl
// disables every pushInsteadOf alias, and it escapes an insteadOf rule whose prefix matches
// only the configured (SSH) spelling — but NOT an insteadOf rule that matches the https
// pushurl itself, since git applies insteadOf to pushurl values too. Which case this is
// depends on the https target, not on which rule produced the refusal, so the decision is
// pushURLRemedy's: it asks whether git would rewrite that target back to SSH.
func (rw rewriteTrace) remedy(cfg map[string][]string, dir, remote, effective, role string) string {
	lead := "the rule matches only the configured url, and an explicit push url of a spelling it does " +
		"not match escapes it, so set one"
	if rw.push {
		lead = "an explicit push url is never pushInsteadOf-rewritten, so set one"
	}
	target := httpsTargetFor(rw.from, effective, remote)
	// Removing an insteadOf rule that matches target clears the refusal on its own only when
	// that rule is the whole story: an insteadOf rewrite of a configured url that already IS
	// target. A pushInsteadOf alias still applies once the insteadOf rule is gone, so there
	// the push url must be set too.
	ruleAlone := !rw.push && rw.from == target
	return pushURLRemedy(cfg, dir, remote, target, rw.from, role, lead, ruleAlone)
}

// httpsTargetFor is the https push url a remedy proposes. A configured https url is kept
// verbatim (it already names the repository the operator meant); otherwise the https twin of
// the CONFIGURED spelling is preferred over the effective one — the effective url of an
// SSH-to-SSH rewrite is typically an ssh alias host (`ssh.<host>:443`) that is not a web host.
func httpsTargetFor(configured, effective, remote string) string {
	if anyHTTPTransport([]string{configured}) {
		return configured
	}
	if t := httpsEquivalent(configured); t != "" {
		return t
	}
	return orSuggestHTTPS(httpsEquivalent(effective), remote)
}

// pushURLRemedy renders the one fix for an SSH push url: point the push url at target —
// unless a url.<base>.insteadOf rule would rewrite target itself into an SSH url, in which
// case `remote set-url --push` leaves the refusal standing and the remedy is that rule.
// Deciding this per TARGET (not per refusal branch) is what keeps the advice true for every
// shape: an https url an insteadOf rule turns into SSH needs the rule gone, while an SSH url
// an insteadOf rule turns into another SSH url is cleared by an https push url the rule does
// not match.
//
// ruleAlone says whether removing that insteadOf rule is by itself enough — true only when
// the configured url already is target. Otherwise (the configured url is SSH, or a
// pushInsteadOf alias still applies once the rule is gone) the remedy names BOTH steps, so
// the operator is not sent round the gate twice. replace is the configured push-url value
// the remedy swaps out; it makes the set-url line runnable on a multi-valued pushurl.
func pushURLRemedy(cfg map[string][]string, dir, remote, target, replace, role, lead string, ruleAlone bool) string {
	setURL := setPushURLCommand(cfg, dir, remote, target, replace)
	if base, prefix, out, ok := longestRewrite(urlRewriteRules(cfg, false), target); ok && isSSHTransport(out) {
		ruleKey := "url." + base + ".insteadOf"
		msg := fmt.Sprintf("remove or narrow the rule %s = %s in the config file that sets it. An explicit "+
			"https push url does NOT escape it — git applies insteadOf to pushurl values too, and it would "+
			"rewrite %s into %s — so `remote set-url --push` alone would leave this refusal standing "+
			"(find it: git -C %s config --show-origin --get-all %s).",
			ruleKey, prefix, target, out, orDot(dir), ruleKey)
		if ruleAlone {
			return msg
		}
		return fmt.Sprintf("%s Removing the rule alone is not enough either — the push would still go out "+
			"over SSH — so once it is gone, point the push url at https (one line, in this worktree):\n  %s\n"+
			"then configure the %s App's credential helper for that URL.", msg, setURL, role)
	}
	if lead == "" {
		lead = "point the push url at https"
	}
	return fmt.Sprintf("%s (one line, in this worktree):\n  %s\nthen configure the %s App's credential "+
		"helper for that URL.", lead, setURL, role)
}

// setPushURLCommand is the runnable `remote set-url --push` line for the remedy. On a
// multi-valued remote.<name>.pushurl the plain form fails ("remote.<name>.pushurl has
// multiple values"), so it names the value to replace as git's <oldurl> pattern — anchored and
// regex-quoted, so it swaps exactly that value and keeps every other push destination. When
// the value to replace is not one of the configured ones, the only runnable form resets the
// list first.
func setPushURLCommand(cfg map[string][]string, dir, remote, target, replace string) string {
	cmd := fmt.Sprintf("git -C %s remote set-url --push %s %s", orDot(dir), remote, target)
	pushurls := cfg["remote."+remote+".pushurl"]
	if len(pushurls) < 2 {
		return cmd
	}
	if replace != "" && containsString(pushurls, replace) {
		return cmd + " " + shellSingleQuote("^"+regexp.QuoteMeta(replace)+"$")
	}
	return fmt.Sprintf("git -C %s config --unset-all remote.%s.pushurl && %s", orDot(dir), remote, cmd)
}

// shellSingleQuote quotes s for a POSIX shell, so a pasted remedy line passes a regex
// pattern through verbatim.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// urlRewriteRules collects the configured url.<base>.insteadOf (push=false) or
// url.<base>.pushInsteadOf (push=true) rules as base → prefixes. `git config --list`
// lowercases the section and variable names but keeps the subsection (the base) verbatim.
func urlRewriteRules(cfg map[string][]string, push bool) map[string][]string {
	// ".pushinsteadof" does not end in ".insteadof" (an "h" precedes "insteadof"), so the
	// two suffixes never claim each other's keys.
	suffix := ".insteadof"
	if push {
		suffix = ".pushinsteadof"
	}
	rules := map[string][]string{}
	for key, vals := range cfg {
		if !strings.HasPrefix(key, "url.") || !strings.HasSuffix(key, suffix) {
			continue
		}
		base := key[len("url.") : len(key)-len(suffix)]
		rules[base] = append(rules[base], vals...)
	}
	return rules
}

// longestRewrite applies git's rule for one URL: among every rule's prefixes, the LONGEST
// one that prefixes the URL wins, and its base replaces it. Ties break on the base, sorted,
// so the attribution is stable.
func longestRewrite(rules map[string][]string, u string) (base, prefix, out string, ok bool) {
	bases := make([]string, 0, len(rules))
	for b := range rules {
		bases = append(bases, b)
	}
	sort.Strings(bases)
	for _, b := range bases {
		for _, p := range rules[b] {
			if p == "" || !strings.HasPrefix(u, p) {
				continue
			}
			if !ok || len(p) > len(prefix) {
				base, prefix, ok = b, p, true
			}
		}
	}
	if ok {
		out = base + u[len(prefix):]
	}
	return base, prefix, out, ok
}

// traceRewrite finds the rule that turned one of the configured push URLs into effective,
// following git's own order (remote.c): with no explicit pushurl, a url's pushInsteadOf
// alias becomes the push URL when one matches; every configured value (pushurl or url) is
// otherwise insteadOf-rewritten.
func traceRewrite(cfg map[string][]string, configured []string, explicitPush bool, effective string) rewriteTrace {
	pushRules := urlRewriteRules(cfg, true)
	rules := urlRewriteRules(cfg, false)
	for _, c := range configured {
		if !explicitPush {
			if base, prefix, out, ok := longestRewrite(pushRules, c); ok {
				if out == effective {
					return rewriteTrace{ruleKey: "url." + base + ".pushInsteadOf", prefix: prefix, from: c, push: true}
				}
				continue // git pushes to the alias; insteadOf does not apply on top of it
			}
		}
		if base, prefix, out, ok := longestRewrite(rules, c); ok && out == effective {
			return rewriteTrace{ruleKey: "url." + base + ".insteadOf", prefix: prefix, from: c}
		}
	}
	return rewriteTrace{}
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
