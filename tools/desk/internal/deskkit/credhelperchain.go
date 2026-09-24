package deskkit

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
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
// in it reads the minted App token.
//
// Credentials git uses AHEAD of any helper are judged too: a password embedded in the push
// URL, and an `http.extraHeader` Authorization header that applies to it. A push URL that is
// not http(s) never consults a helper at all. Each of these reads not-clean.
//
// Everything here is READ-ONLY: git config reads and URL parsing. No helper is run and the
// remote is never contacted.
//
// Where exact git semantics would take a re-implementation of git's matcher, the resolver
// errs toward APPLYING an entry (which can only redden the check), never toward skipping it:
// a scheme-less `credential.<host>` pattern applies when its host matches, whatever its user
// or path; `http.<url>.extraHeader` entries apply whenever their <url> matches, without
// git's best-match narrowing.

// credTransportMatchesApp judges the credential git would present for a push to rawURL from
// the repository at dir. It returns (true, detail, nil) only when every credential source git
// consults for that URL is the App token helper.
func credTransportMatchesApp(dir, rawURL, appTokenPath string) (bool, string, error) {
	shown := redactURLCredential(rawURL)
	u, perr := url.Parse(rawURL)
	if perr != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return false, "the push URL " + shown + " is not an http(s) URL, so git never consults a credential " +
			"helper for it and a push cannot authenticate as the minted App token", nil
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

	chain, err := applicableConfigValues(dir, "credential", "helper", rawURL)
	if err != nil {
		return false, "", err
	}
	if len(chain) == 0 {
		return false, "no credential helper applies to " + shown, nil
	}
	for i, h := range chain {
		if !helperReadsAppToken(h, appTokenPath) {
			return false, fmt.Sprintf("git consults %d credential helper(s) for %s in order, and helper %d of them "+
				"does not read the App token cache (%s)", len(chain), shown, i+1, oneLine(h)), nil
		}
	}
	return true, fmt.Sprintf("every credential helper git consults for %s (%d) reads the App token cache",
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

// helperReadsAppToken reports whether a helper command names the minted App token cache, by
// its full path or its file name.
func helperReadsAppToken(helper, appTokenPath string) bool {
	h := strings.TrimSpace(helper)
	p := strings.TrimSpace(appTokenPath)
	if p == "" {
		return false
	}
	if strings.Contains(h, p) {
		return true
	}
	base := baseName(p)
	return base != "" && base != "." && strings.Contains(h, base)
}

func baseName(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// applicableConfigValues returns, in git's config read order, the values of
// `<section>.<variable>` and every `<section>.<url>.<variable>` whose <url> applies to rawURL,
// with an empty value resetting the list built so far (git's rule for multi-valued
// credential.helper and http.extraHeader).
func applicableConfigValues(dir, section, variable, rawURL string) ([]string, error) {
	pattern := `^` + section + `\.(.+\.)?` + variable + `$`
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
// applies to rawURL. A pattern with a scheme is decided by git's OWN URL matcher (a
// one-entry config file queried with --get-urlmatch), so wildcards, default ports and path
// prefixes follow git exactly. A scheme-less pattern is git's partial-URL fallback, matched
// here on host (with port) only — over-inclusive by design.
func urlPatternApplies(dir, section, sub, rawURL string) (bool, error) {
	if !strings.Contains(sub, "://") {
		return partialURLHostApplies(sub, rawURL), nil
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

// partialURLHostApplies matches a scheme-less `[user@]host[:port][/path]` pattern against the
// URL's host (with port), case-insensitively. User and path are ignored, which can only make
// the pattern apply MORE often than git would — never less.
func partialURLHostApplies(pattern, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // cannot compare: treat as applying (fail toward red)
	}
	p := pattern
	if i := strings.LastIndex(p, "@"); i >= 0 {
		p = p[i+1:]
	}
	if i := strings.Index(p, "/"); i >= 0 {
		p = p[:i]
	}
	return strings.EqualFold(p, u.Host)
}

// redactURLCredential renders a URL with any embedded password removed, for a detail line.
func redactURLCredential(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
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
