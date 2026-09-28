package deskkit

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// credhelperchain.go — the transport half of the ambient-identity check (check 6): which
// credential would git ACTUALLY present when this envelope pushes?
//
// `git config --get-urlmatch credential.helper <url>` answers the wrong question. It returns
// ONE value — the last one configured — while git consults EVERY applicable helper in config
// order and uses the first that returns a credential. With a keychain-style helper in the
// system or global config and a token-file helper in the repo config, --get-urlmatch prints
// only the token-file helper, so a probe built on it reads "clean" while a real push
// authenticates with whatever the earlier helper supplies: the probe-green/push-red split
// this check exists to catch, reported as green.
//
// So the chain is rebuilt the way git builds it (credential.c, credential_apply_config):
// every `credential.helper` and `credential.<url>.helper` entry, across every config scope in
// git's own read order (system, global, local, worktree, command line, includes inlined),
// filtered to the entries whose <url> applies to the push URL; an EMPTY value resets the list
// built so far. The check passes only when the resulting chain is non-empty and every entry
// in it IS the desk's App token helper (AppTokenHelper) reading the minted token file — not
// merely a command that mentions that file.
//
// Matching a `credential.<url>` pattern follows git: a pattern git can normalize as a URL is
// decided by git's own URL matcher (--get-urlmatch over a one-entry config file); one it
// cannot normalize — no scheme, or no host: "", "/", "/org/repo.git", "https://" — falls back
// to git's partial-URL match (credential.c, match_partial_url), where every field the pattern
// leaves unset is a wildcard. A pattern with no host therefore applies to every host.
//
// Credentials git uses AHEAD of any helper are judged too: a password embedded in the push
// URL, an `http.extraHeader` Authorization header that applies to it, and a netrc entry for
// its host (git asks libcurl to consult netrc, and curl answers the server's 401 from it
// before any helper runs). A push URL that is not http(s) never consults a helper at all.
// Each of these reads not-clean. askpass needs no separate read: git falls through to it only
// when the helper chain yields no username or password, and every entry of a passing chain
// is the App helper, which always answers both (an empty password when the file is gone).
// That holds because a helper value is matched raw, exactly as git reads it: a leading blank
// would make git run it as `git credential-…`, which answers nothing (isAppTokenHelper).
//
// Every push URL the remote pushes to is judged (`remote get-url --push --all`), since
// `git push` pushes to each; one URL failing fails the check.
//
// What the check cannot read is could-not-check, never clean: a push URL git cannot
// normalize or whose host is empty, a pattern git would skip as unparseable, a netrc file
// that exists but cannot be read, or one holding a NUL byte that nothing else reddens.
//
// Everything here is READ-ONLY: git config reads, file reads and URL parsing. No helper is
// run and the remote is never contacted.
//
// Where exact git semantics would take a re-implementation of git's matcher, the resolver
// errs toward APPLYING an entry (which can only redden the check), never toward skipping it:
// a partial-URL pattern applies when its scheme and host (where given) match, whatever its
// user or path; `http.<url>.extraHeader` entries apply whenever their <url> matches, without
// git's best-match narrowing; the host or `default` anywhere in a netrc file counts, wherever
// it sits and whatever login follows it, and so does any netrc line too long for an old libcurl
// reader to read whole (netrcMatch).

// credTransportMatchesApp judges the credential git would present for a push to rawURL from
// the repository at dir. It returns (true, detail, nil) only when every credential source git
// consults for that URL is the App token helper. An error is could-not-check.
func credTransportMatchesApp(dir, rawURL, appTokenPath string) (bool, string, error) {
	shown := redactURLCredential(rawURL)
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return false, "the push URL " + shown + " is not an http(s) URL, so git never consults a credential " +
			"helper for it and a push cannot authenticate as the minted App token", nil
	}
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return false, "", fmt.Errorf("the push URL %s cannot be parsed, so the credential git presents for it "+
			"cannot be judged", shown)
	}
	if u.Host == "" || u.Hostname() == "" {
		return false, "", fmt.Errorf("the push URL %s has an empty host, so which credential git presents for "+
			"it cannot be judged", shown)
	}
	if err := gitNormalizesURL(dir, rawURL); err != nil {
		return false, "", fmt.Errorf("git cannot normalize the push URL %s (%s), so which credential entries apply "+
			"to it cannot be judged", shown, oneLine(err.Error()))
	}
	if u.User != nil {
		if _, has := u.User.Password(); has {
			return false, "the push URL " + shown + " embeds a credential, which git presents ahead of every " +
				"credential helper", nil
		}
	}

	headers, err := applicableConfigValues(dir, "http", "extraheader", rawURL)
	if err != nil {
		return false, "", err
	}
	for _, h := range headers {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(h)), "authorization") {
			return false, "an http.extraHeader Authorization header applies to " + shown + "; git sends it ahead " +
				"of any credential helper", nil
		}
	}

	entry, nerr := netrcEntryFor(u.Hostname())
	if nerr != nil {
		return false, "", nerr
	}
	if entry != "" {
		return false, entry + " for " + shown + "; a libcurl netrc reader git might link can answer the " +
			"server's authentication challenge from it before any credential helper runs, and this scan does " +
			"not track where a word sits or how an old reader splits a long line, so it cannot rule that out " +
			"(if the word is only in a comment, a macro or a value, edit it out of the netrc file; shorten an " +
			"over-long line)", nil
	}

	chain, err := applicableConfigValues(dir, "credential", "helper", rawURL)
	if err != nil {
		return false, "", err
	}
	if len(chain) == 0 {
		return false, "no credential helper applies to " + shown, nil
	}
	for i, h := range chain {
		if !isAppTokenHelper(h, appTokenPath) {
			return false, fmt.Sprintf("git consults %d credential helper(s) for %s in order, and helper %d of them "+
				"is not the App token helper for %s: git runs %s", len(chain), shown, i+1, appTokenPath,
				helperInvocation(h)), nil
		}
	}
	return true, fmt.Sprintf("every credential helper git consults for %s (%d) is the App token helper",
		shown, len(chain)), nil
}

// AppTokenHelper is the credential helper command the desk installs for a role's minted
// token: it answers with a fixed username and the CONTENTS of the token file, read at auth
// time inside git's own shell, so the token never reaches argv, a URL or config. It is the
// single source of that shape — `deskwt role-init` writes it and the credential-chain check
// recognises it — so the writer and the checker cannot drift apart.
func AppTokenHelper(username, tokenPath string) string {
	return "!f(){ echo username=" + username + "; echo \"password=$(cat '" + tokenPath + "')\"; }; f"
}

// isAppTokenHelper reports whether a helper IS the desk's App token helper
// (AppTokenHelper) reading appTokenPath — the same command, for a plain username, over the
// same file. A helper that merely names the file (a different command around it, `store
// --file <path>`, a same-named file in another directory) is not it: what git presents is
// whatever that command prints, not the token.
//
// The value is matched RAW, never trimmed: git does not trim a helper value either, and a
// leading blank turns the App helper into `git credential- !f(){…`, which answers nothing, so
// git falls through to askpass. Any surrounding whitespace therefore reads not-clean.
func isAppTokenHelper(helper, appTokenPath string) bool {
	want := strings.TrimSpace(appTokenPath)
	if want == "" {
		return false
	}
	name, path, ok := parseAppTokenHelper(helper)
	if !ok || name == "" || !filepath.IsAbs(path) {
		return false // a relative path is read from wherever git runs the helper
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return sameFile(path, want)
}

// parseAppTokenHelper splits a helper built by AppTokenHelper back into its username and token
// path. ok is false for any other command.
func parseAppTokenHelper(h string) (name, path string, ok bool) {
	const sentinelUser, sentinelPath = "\x00user\x00", "\x00path\x00"
	tmpl := AppTokenHelper(sentinelUser, sentinelPath)
	pre, rest, _ := strings.Cut(tmpl, sentinelUser)
	mid, post, _ := strings.Cut(rest, sentinelPath)
	if !strings.HasPrefix(h, pre) || !strings.HasSuffix(h, post) || len(h) < len(pre)+len(post) {
		return "", "", false
	}
	body := h[len(pre) : len(h)-len(post)]
	name, path, found := strings.Cut(body, mid)
	if !found || strings.ContainsAny(name, " \t;'\"$`\\\n") || strings.ContainsAny(path, "'\n") {
		return "", "", false
	}
	return name, path, true
}

// sameFile reports whether two token paths name the same file: equal once cleaned, or equal
// once every symlink is resolved (both must resolve). Anything else is a different file.
func sameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && ra == rb
}

// helperInvocation renders a helper value as the command git would actually run for it
// (credential.c, credential_do): a `!` value is a shell snippet, an absolute path is run as
// given, and anything else is `git credential-<value>`. git reads the value untrimmed, so a
// value with a leading blank is always the `git credential-` form, whatever follows the blank.
func helperInvocation(h string) string {
	switch {
	case strings.TrimLeft(h, " \t\n\r") != h:
		return "`git credential-" + oneLine(h) + "` (the configured value starts with whitespace, which git " +
			"keeps, so it runs `git credential-` with that value rather than the command after the blank)"
	case strings.HasPrefix(h, "!"):
		return "the shell command `" + oneLine(strings.TrimPrefix(h, "!")) + "`"
	case filepath.IsAbs(h) || strings.HasPrefix(h, "/"):
		return "`" + oneLine(h) + "`"
	default:
		return "`git credential-" + oneLine(h) + "`"
	}
}

// applicableConfigValues returns, in git's config read order, the values of
// `<section>.<variable>` and every `<section>.<url>.<variable>` whose <url> applies to rawURL,
// with an empty value resetting the list built so far (git's rule for multi-valued
// credential.helper and http.extraHeader).
func applicableConfigValues(dir, section, variable, rawURL string) ([]string, error) {
	// `(.*\.)?`, not `(.+\.)?`: an EMPTY subsection (`[credential ""]`, key
	// `credential..helper`) is a real entry that git applies to every URL.
	pattern := `^` + section + `\.(.*\.)?` + variable + `$`
	out, err := gitOut(dir, "config", "-z", "--get-regexp", pattern)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil // no such key in any scope
		}
		return nil, fmt.Errorf("git config --get-regexp %s could not be read: %s", pattern, gitErrText(err))
	}
	plain := section + "." + variable
	var vals []string
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		key, val, hasVal := strings.Cut(rec, "\n")
		if !hasVal {
			return nil, fmt.Errorf("%s is set with no value, so what git does with it cannot be judged", key)
		}
		if key != plain {
			sub := strings.TrimSuffix(strings.TrimPrefix(key, section+"."), "."+variable)
			ok, aerr := urlPatternApplies(dir, section, sub, rawURL)
			if aerr != nil {
				return nil, aerr
			}
			if !ok {
				continue
			}
		}
		if val == "" {
			vals = nil
			continue
		}
		vals = append(vals, val)
	}
	return vals, nil
}

// urlPatternApplies reports whether the <url> subsection of a `<section>.<url>.*` entry
// applies to rawURL, the way git decides it (urlmatch.c, urlmatch_config_entry):
//
//   - a pattern git can normalize as a URL is decided by git's OWN URL matcher (a one-entry
//     config file queried with --get-urlmatch), so wildcards, default ports and path prefixes
//     follow git exactly;
//   - a pattern git cannot normalize (no scheme, or a scheme with no host) falls back, in the
//     credential section only, to git's partial-URL match (partialURLApplies), where an unset
//     host or scheme matches anything. git skips such a pattern in the http section; the
//     scheme-less form is still matched on host there, which can only redden the check.
func urlPatternApplies(dir, section, sub, rawURL string) (bool, error) {
	if !strings.Contains(sub, "://") {
		return partialURLApplies(section, sub, rawURL)
	}
	normal, nerr := gitNormalizesPattern(dir, sub)
	if nerr != nil {
		return false, fmt.Errorf("git could not judge whether %s.%s is a URL it can match: %v", section, sub, nerr)
	}
	if !normal {
		if section == "credential" {
			return partialURLApplies(section, sub, rawURL)
		}
		return false, nil
	}
	f, err := os.CreateTemp("", "deskkit-urlmatch-*.cfg")
	if err != nil {
		return false, fmt.Errorf("cannot create a scratch config to match %s.%s: %v", section, sub, err)
	}
	defer os.Remove(f.Name())
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(sub)
	if _, werr := f.WriteString("[" + section + " \"" + esc + "\"]\n\tprobe = match\n"); werr != nil {
		f.Close()
		return false, fmt.Errorf("cannot write a scratch config to match %s.%s: %v", section, sub, werr)
	}
	if cerr := f.Close(); cerr != nil {
		return false, fmt.Errorf("cannot close a scratch config to match %s.%s: %v", section, sub, cerr)
	}
	out, gerr := gitOut(dir, "config", "--file", f.Name(), "--get-urlmatch", section+".probe", rawURL)
	if gerr != nil {
		var ee *exec.ExitError
		if errors.As(gerr, &ee) && ee.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git could not match %s.%s against the push URL: %s", section, sub, gitErrText(gerr))
	}
	return strings.TrimSpace(out) == "match", nil
}

// gitNormalizesURL asks git whether it can normalize rawURL (url_normalize): --get-urlmatch
// normalizes its URL argument first and dies (exit 128) when it cannot; any other outcome
// means it could. It returns nil when git can, and the reason when it cannot.
func gitNormalizesURL(dir, rawURL string) error {
	ok, reason, err := gitURLNormalizes(dir, rawURL)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New(reason)
	}
	return nil
}

// gitNormalizesPattern reports whether git can normalize a `<section>.<url>` pattern. git
// normalizes config patterns allowing `*` globs in the host, which a URL argument does not
// accept, so each `*` in the host is swapped for a plain letter before asking — a swap that
// never changes whether the rest normalizes.
func gitNormalizesPattern(dir, pattern string) (bool, error) {
	probe := pattern
	if i := strings.Index(probe, "://"); i >= 0 {
		rest := probe[i+3:]
		end := strings.IndexAny(rest, "/?#")
		if end < 0 {
			end = len(rest)
		}
		probe = probe[:i+3] + strings.ReplaceAll(rest[:end], "*", "x") + rest[end:]
	}
	ok, _, err := gitURLNormalizes(dir, probe)
	return ok, err
}

func gitURLNormalizes(dir, rawURL string) (bool, string, error) {
	f, err := os.CreateTemp("", "deskkit-urlnorm-*.cfg")
	if err != nil {
		return false, "", fmt.Errorf("cannot create a scratch config: %v", err)
	}
	name := f.Name()
	defer os.Remove(name)
	if cerr := f.Close(); cerr != nil {
		return false, "", fmt.Errorf("cannot close a scratch config: %v", cerr)
	}
	_, gerr := gitOut(dir, "config", "--file", name, "--get-urlmatch", "credential.probe", rawURL)
	if gerr == nil {
		return true, "", nil
	}
	var ee *exec.ExitError
	if errors.As(gerr, &ee) {
		switch ee.ExitCode() {
		case 1:
			return true, "", nil // normalized; nothing matched in the empty file
		case 128:
			return false, gitErrText(gerr), nil
		}
	}
	return false, "", fmt.Errorf("git config --get-urlmatch could not run: %s", gitErrText(gerr))
}

// partialURLApplies is git's partial-URL fallback for a `credential.<pattern>` git cannot
// normalize (credential.c, match_partial_url over credential_from_potentially_partial_url):
// the pattern is split into [scheme://][user@]host[/path], and every field it leaves unset
// matches anything. The scheme and host are compared (case-insensitively, the host with any
// port, as git compares it); user and path are ignored, which can only make the pattern apply
// MORE often than git would — never less. A pattern git would skip as unparseable (a decoded
// field carrying a newline) is could-not-check.
func partialURLApplies(section, pattern, rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true, nil // cannot compare: treat as applying (fail toward red)
	}
	var scheme string
	cp := pattern
	if i := strings.Index(pattern, "://"); i >= 0 {
		scheme, cp = pattern[:i], pattern[i+3:]
	}
	slash := strings.IndexAny(cp, "/?#")
	if slash < 0 {
		slash = len(cp)
	}
	host := cp[:slash]
	if at := strings.Index(cp, "@"); at >= 0 && at < slash {
		host = cp[at+1 : slash]
	}
	for _, field := range []string{scheme, pctDecode(host), pctDecode(pattern)} {
		if strings.ContainsAny(field, "\n\x00") {
			return false, fmt.Errorf("%s.%s cannot be parsed as a partial URL (git skips it with a warning), so "+
				"whether it applies cannot be judged", section, oneLine(pattern))
		}
	}
	if scheme != "" && !strings.EqualFold(scheme, u.Scheme) {
		return false, nil
	}
	if host != "" && !strings.EqualFold(pctDecode(host), u.Host) {
		return false, nil
	}
	return true, nil
}

// pctDecode decodes %XX escapes the way git's url_decode does: a malformed escape is kept
// as-is rather than rejected.
func pctDecode(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// netrcEntryFor returns a description of what in the netrc files could let curl answer an
// authentication challenge for host (netrcMatch), or "" when nothing can. git has libcurl consult netrc
// (CURLOPT_NETRC optional), and curl answers the server's challenge from a matching `machine`
// entry, or from a `default` entry, before git runs any credential helper. The files read
// are the ones curl reads — $HOME/.netrc (on Windows also %USERPROFILE% and _netrc) — plus
// $NETRC when set. A file that exists but cannot be read is could-not-check.
func netrcEntryFor(host string) (string, error) {
	var files []string
	if v := strings.TrimSpace(os.Getenv("NETRC")); v != "" {
		files = append(files, v)
	}
	homes := []string{os.Getenv("HOME")}
	if runtime.GOOS == "windows" {
		homes = append(homes, os.Getenv("USERPROFILE"))
	}
	found := false
	for _, h := range homes {
		if strings.TrimSpace(h) == "" {
			continue
		}
		found = true
		files = append(files, filepath.Join(h, ".netrc"))
		if runtime.GOOS == "windows" {
			files = append(files, filepath.Join(h, "_netrc"))
		}
	}
	if !found {
		if cu, err := user.Current(); err == nil && strings.TrimSpace(cu.HomeDir) != "" {
			files = append(files, filepath.Join(cu.HomeDir, ".netrc"))
		} else {
			return "", fmt.Errorf("no home directory is known, so whether a netrc file answers for %s cannot be judged", host)
		}
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("the netrc file %s exists but cannot be read (%v), so whether it answers for %s "+
				"cannot be judged", f, oneLine(err.Error()), host)
		}
		entry, err := netrcMatch(string(body), host)
		if err != nil {
			return "", fmt.Errorf("the netrc file %s %v, so whether it answers for %s cannot be judged", f, err, host)
		}
		if entry != "" {
			return entry + " in " + f, nil
		}
	}
	return "", nil
}

// netrcMatch decides whether a netrc body could let curl answer an authentication challenge
// for host, per the driver's ruling on arbiter packet #1622
// (https://github.com/medici-finance/assay/issues/1622#issuecomment-5837592241, "Ruling: 1"):
// a state-free scan that can only read red. It returns a non-empty description when it reads
// red, an error for could-not-check, and "" for green.
//
// Why state-free. Every earlier round modelled how one libcurl netrc reader splits the file
// into tokens — keyword case, comments, macro ends, separator bytes, the byte a reader drops
// after a closing quote, and finally which bytes end an unquoted token (SR-1614-2, -3, -4, -6,
// -8). Each model was exact for some readers and wrong for the next one found, because git links
// whichever libcurl the host ships and libcurl has shipped several readers. A quote the model
// opened where a real reader did not let that quote swallow a real entry. So this check keeps no
// quote state and no per-reader separator table at all.
//
// The core rule (netrcWordRule): red when the host or `default` occurs ANYWHERE in the file,
// ASCII case-insensitively, delimited only by the start or end of the file or by a byte that is
// not an ASCII letter, digit, `.`, `-` or `_` (isNetrcWordByte). Three add-only rules follow it,
// each covering one way a reader can form a token whose bytes are not a delimited word in the
// raw file:
//
//   - netrcEscapeFoldedRule: a quoted token may spell the host with backslash escapes (`\.`
//     stands for `.`). The core rule runs again over the body with every backslash removed.
//   - netrcQuoteSkipRule: every libcurl before 8.21 drops the one byte after a closing quote
//     before reading the next token, so in `"a"Xdefault` the word `default` is a token though
//     it follows the letter X. Red when the word ends at a delimiter and starts two bytes after
//     a `"`, whatever the byte between.
//   - netrcLongLineRule: libcurl 7.61.0 through 7.85.0 read a line in pieces of at most 4095
//     bytes and tokenize each piece as a line of its own, so a piece boundary can start or end
//     a token between two word bytes. Red on any line longer than 4095 bytes counting its
//     newline (netrcOldReadPiece), whatever the line holds; it keeps no state but that length.
//     The threshold is exact for those readers: a line of 4095 bytes or fewer, counting its
//     newline, is one read; any longer line is at least two, so no split escapes it.
//
// netrcMatch returns the first red any rule finds, in order, so a later rule only ever runs on a
// body every earlier rule read green: no rule can turn another's red into green or could-not-check.
//
// Why these cover the readers this check knows of. By how each libcurl reads a line (lib/netrc.c):
//
//   - 7.61.0 through 7.85.0: `fgets(netrcbuffer, netrcbuffsize, file)` into `char
//     netrcbuffer[4096]`, the line tokenized with strtok_r (to 7.83.x) or a hand-written
//     tokenizer with quotes (7.84.0, 7.85.0). A line longer than 4095 bytes arrives in pieces:
//     covered by the long-line rule, which reds every such line. A line of at most 4095 bytes
//     is read whole, and the token argument below covers it.
//   - 7.86.0 through 8.6.0: Curl_get_line into the same 4096-byte buffer, which drops a line
//     too long for it; 8.7.0 onward (among them the 8.11-8.12 reader that drops whole-line
//     comments on load, curl-8.13 through curl-8.20 with a signed or an unsigned char, and the
//     curl-8.21+ grammar lexer): Curl_get_line into a capped dynbuf, which stops reading at an
//     over-long line. Neither splits a line, so the token argument covers them.
//
// Every one of them presents a credential only after reading a token equal to host (after
// `machine`) or to `default`, case-insensitively. Within a line it reads whole, such a token is
// one of:
//
//   - unquoted: raw file bytes, ended by end of file or a byte that reader treats as a separator,
//     and started at start of file, after such a byte, right after a closing quote (8.21+), or
//     right after the byte a pre-8.21 reader drops after a closing quote. No reader separates
//     on a letter, digit, `.`, `-` or `_` within a line it reads whole, so every case but the
//     last is a delimited word (core rule) and the last is the quote-skip rule;
//   - quoted: the bytes between two quotes, each optionally preceded by a backslash (\n, \r and
//     \t decode to control bytes a host never holds). With the backslashes removed they are a
//     word delimited by those quotes (escape-folded rule).
//
// The limit of that argument: it holds for readers that split a line only as above, separate only
// on non-word bytes, drop at most that one byte after a quote, and decode only single-byte
// backslash escapes. A reader outside it is NOT covered, and the check may read green there:
//
//   - libcurl 7.60.0 and older, which read with fgets into `char netrcbuffer[256]`, so a line of
//     256 to 4095 bytes arrives in pieces too. git has required libcurl 7.61.0 or later since
//     git 2.48 (git's INSTALL), so only an older git linked against such a libcurl reads that way;
//   - an unbuilt libcurl release, a vendor patch, or a non-libcurl reader.
//
// It cannot tell which libcurl git links, so it never reports could-not-check for that reason.
//
// Cost the ruling accepts (a false red): the host or `default` appearing only in a comment, a
// macro or a value reads red, and so does any line longer than 4095 bytes counting its newline,
// even one holding neither word. The operator clears it by editing their netrc.
//
// Could-not-check: a body with a NUL byte and no red. curl reads the file a line at a time with
// strlen, so a NUL drops the rest of its line and the newline with it, splicing the next line on
// (`def` NUL LF `ault` reads as `default`); no byte scan can see what that forms.
func netrcMatch(body, host string) (string, error) {
	for _, rule := range netrcRedRules {
		if detail := rule.match(body, host); detail != "" {
			return detail, nil
		}
	}
	if strings.IndexByte(body, 0) >= 0 {
		return "", errors.New("holds a NUL byte, which makes curl splice lines together in ways a byte scan cannot read")
	}
	return "", nil
}

// netrcRedRule is one state-free rule of the netrc scan; match returns a description when the
// rule reads red for host, or "".
type netrcRedRule struct {
	name  string
	match func(body, host string) string
}

// netrcRedRules are netrcMatch's rules in order: the core rule first, then the add-only rules.
var netrcRedRules = []netrcRedRule{
	{"core", netrcWordRule},
	{"escape-folded", netrcEscapeFoldedRule},
	{"quote-skip", netrcQuoteSkipRule},
	{"long-line", netrcLongLineRule},
}

// netrcWordRule is the core rule: host or `default` occurs in body as a delimited word.
func netrcWordRule(body, host string) string {
	return netrcWordsFound(body, host, "", netrcWordStartsAfterDelimiter)
}

// netrcEscapeFoldedRule applies the core rule to body with every backslash removed, so a quoted
// token that spells host (or `default`) with backslash escapes still reads red.
func netrcEscapeFoldedRule(body, host string) string {
	if strings.IndexByte(body, '\\') < 0 {
		return ""
	}
	return netrcWordsFound(strings.ReplaceAll(body, `\`, ""), host,
		" once backslash escapes in quoted tokens are decoded", netrcWordStartsAfterDelimiter)
}

// netrcQuoteSkipRule reads red when host or `default` ends at a delimiter and starts two bytes
// after a `"`: a pre-8.21 libcurl drops the byte after a closing quote and reads the word as a
// token whatever that byte was.
func netrcQuoteSkipRule(body, host string) string {
	return netrcWordsFound(body, host, " right after the byte a pre-8.21 libcurl drops after a closing quote",
		func(b string, p int) bool { return p >= 2 && b[p-2] == '"' })
}

// netrcOldReadBuffer is the netrc line buffer of libcurl 7.61.0 through 7.85.0: lib/netrc.c
// declares `char netrcbuffer[4096]` and reads each line with `fgets(netrcbuffer, netrcbuffsize,
// file)`, where netrcbuffsize is sizeof(netrcbuffer). (7.60.0 and older used 256 bytes; 7.86.0
// moved to Curl_get_line, which drops an over-long line instead of splitting it.)
const netrcOldReadBuffer = 4096

// netrcOldReadPiece is the most bytes one such fgets call returns: it stops after a newline or
// one byte short of the buffer, to leave room for the NUL. A line of at most netrcOldReadPiece
// bytes counting its newline is read whole; a longer one is read in pieces.
const netrcOldReadPiece = netrcOldReadBuffer - 1

// netrcLongLineRule reads red when some line of body holds more than netrcOldReadPiece bytes,
// counting its newline (a last line with no newline counts only its bytes). libcurl 7.61.0
// through 7.85.0 read such a line in pieces and tokenize each piece as a line of its own, so a
// piece boundary starts or ends a token wherever it falls, even between two word bytes, and a
// host or `default` glued to other word bytes in the raw file becomes a token. A line of exactly
// 4096 bytes counting its newline is red too: its second piece is the newline alone, which also
// ends a macdef body on those readers. The rule does not look for the host or `default`, and its
// verdict depends on nothing but each line's length (the line number only labels the red, which
// names the line number and length, never its bytes). On Windows curl opens the file in text
// mode ("rt"), which only removes bytes from a line, so counting the raw bytes never undercounts.
func netrcLongLineRule(body, _ string) string {
	for line, start := 1, 0; start < len(body); line++ {
		end := len(body)
		if nl := strings.IndexByte(body[start:], '\n'); nl >= 0 {
			end = start + nl + 1
		}
		if n := end - start; n > netrcOldReadPiece {
			return fmt.Sprintf("netrc line %d of %d bytes, longer than the %d bytes libcurl 7.61.0 through "+
				"7.85.0 read at once (they read it in pieces, and a piece boundary can start or end a token "+
				"inside a word)", line, n, netrcOldReadPiece)
		}
		start = end
	}
	return ""
}

// netrcWordsFound describes the first of host and `default` that occurs in body, ASCII
// case-insensitively, ending at the end of body or at a non-word byte and starting where
// startsOK allows; "" when neither does. The description names only host or `default`, never
// other bytes of the file.
func netrcWordsFound(body, host, how string, startsOK func(body string, p int) bool) string {
	if host != "" && netrcWordOccurs(body, host, startsOK) {
		return fmt.Sprintf("the push host %q as a netrc word (case-insensitively%s)", host, how)
	}
	if netrcWordOccurs(body, "default", startsOK) {
		return fmt.Sprintf("the keyword `default` as a netrc word (case-insensitively%s)", how)
	}
	return ""
}

// netrcWordOccurs reports whether word occurs in body, ASCII case-insensitively, at an offset p
// that startsOK accepts and ending at the end of body or at a non-word byte.
func netrcWordOccurs(body, word string, startsOK func(body string, p int) bool) bool {
	for p := 0; p+len(word) <= len(body); p++ {
		end := p + len(word)
		if (end == len(body) || !isNetrcWordByte(body[end])) && asciiEqualFold(body[p:end], word) && startsOK(body, p) {
			return true
		}
	}
	return false
}

// netrcWordStartsAfterDelimiter is the core rule's start condition: the start of body or a
// non-word byte just before p.
func netrcWordStartsAfterDelimiter(body string, p int) bool {
	return p == 0 || !isNetrcWordByte(body[p-1])
}

// isNetrcWordByte reports whether c can sit inside a host name or `default` without delimiting
// it: an ASCII letter, digit, `.`, `-` or `_`. Every other byte delimits a word, whether or not
// any netrc reader treats it as a separator.
func isNetrcWordByte(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '.' || c == '-' || c == '_'
}

// asciiEqualFold compares two strings ignoring ASCII letter case only, as curl's
// strcasecompare does; no Unicode folding.
func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// redactURLCredential renders a URL with any embedded password removed, for a detail line.
func redactURLCredential(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		// Unparseable: drop any userinfo by hand, since it may carry a password.
		if i := strings.Index(raw, "://"); i >= 0 {
			rest := raw[i+3:]
			end := strings.IndexAny(rest, "/?#")
			if end < 0 {
				end = len(rest)
			}
			if at := strings.LastIndex(rest[:end], "@"); at >= 0 {
				return raw[:i+3] + "<redacted>@" + rest[at+1:]
			}
		}
		return raw
	}
	if u.User == nil {
		return raw
	}
	if _, has := u.User.Password(); has {
		u.User = url.User(u.User.Username())
		return u.String()
	}
	return raw
}

func gitErrText(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return oneLine(string(ee.Stderr))
	}
	return oneLine(err.Error())
}
