package main

// transport.go — `deskwt add --role`'s worktree-scoped App TRANSPORT (#861).
//
// THE FAULT. A linked worktree reads its remote from the SHARED .git/config. When that config
// carries an SSH `remote.origin.url` (or an operator's deliberate `remote.origin.pushurl`
// sentinel that points nowhere), every worktree cut from it inherits both: its push goes
// nowhere — or, worse, out over SSH under whatever key the machine's agent holds, a human's —
// and its fetch authenticates with the operator's SSH key. The refusal in
// deskkit/pushtransport.go stops a bot session from pushing over that; this file is the other
// half: a role worktree that `deskwt add --role` cuts is GIVEN an App-only transport of its
// own, so there is nothing to refuse.
//
// WHAT IS WRITTEN, all at WORKTREE scope (extensions.worktreeConfig — the shared checkout's
// config is never touched):
//
//   - remote.origin.pushurl = ""  then  https://<host>:443/<owner>/<name>.git
//   - remote.origin.url     = ""  then  the same URL
//   - the role App's credential helper, host-scoped (wireRoleCredential, role-init's writer —
//     the deskkit.AppTokenHelper shape once that lands, so the preflight's credential-chain
//     check and this writer cannot drift apart).
//
// WHY THE EMPTY ENTRY. Both keys are MULTI-VALUED, and a worktree-scoped value is APPENDED to
// the shared ones rather than replacing them: fetch keeps using the FIRST url (the shared SSH
// one), and a push fans out to EVERY pushurl (the shared sentinel as well). git (2.46+) reads
// an empty value as "reset the list built so far", so an empty entry at worktree scope — the
// last scope git reads — clears the shared values and leaves exactly the one https URL. An
// older git keeps the empty value as a URL of its own; the read-back below catches that and
// refuses, rather than trusting that the reset took.
//
// WHY `:443`. A machine that prefers SSH commonly rewrites `https://<host>/` to SSH with a
// global `url.<ssh>.insteadOf`. The explicit default port leaves the URL git connects to
// unchanged, but it no longer starts with `https://<host>/`, so that rewrite does not catch it.
// The credential helper still matches: git normalises the default port away before matching
// `credential.https://<host>`.
//
// FAIL CLOSED. What git ITSELF resolves is read back afterwards (`git remote get-url [--push]
// --all origin`, insteadOf applied): exactly the one https URL for fetch and for push, or the
// add is REFUSED and the worktree rolled back. A role worktree left on the inherited transport
// is the fault, so "wrote the config" is never taken as "the transport is right".

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// transportHostRe bounds a host this file will write into a URL and hand to `ssh -G`: a DNS
// name, no userinfo, no port, no leading dash (so it can never read as an option).
var transportHostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)

// sshConfigHostname resolves an SSH host ALIAS to the host it connects to, by asking ssh
// itself (`ssh -G`, which prints the evaluated client config and exits — it opens no
// connection). A checkout whose origin is `git@<alias>:owner/name` names a Host block, not a
// host an https URL can reach; the `hostname` line is the real one. Package var ONLY as a test
// seam, so a fixture never depends on the machine's ~/.ssh/config.
var sshConfigHostname = func(alias string) (string, error) {
	out, err := execCommand("ssh", "-G", "--", alias).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), " "); ok && strings.EqualFold(k, "hostname") {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("`ssh -G %s` printed no hostname line", alias)
}

// roleCredUser is the username the role's credential helper answers with: a GitHub App
// installation token authenticates as `x-access-token`, a GitLab token as `oauth2` — read from
// the same roster entry the commit identity was, exactly as role-init reads it.
func roleCredUser(role string) string {
	if ident, bound := deskkit.EffectiveConfig().RoleBotIdentity(role); bound && ident.Forge == deskkit.ForgeGitLab {
		return "oauth2"
	}
	return "x-access-token"
}

// originURLs is what git ITSELF resolves for origin in dir — every fetch url (push=false) or
// every push url (push=true), url.<base>.insteadOf / pushInsteadOf applied.
func originURLs(dir string, push bool) ([]string, error) {
	args := []string{"remote", "get-url", "--all", "origin"}
	if push {
		args = []string{"remote", "get-url", "--push", "--all", "origin"}
	}
	out, err := runGit(dir, args...)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			urls = append(urls, l)
		}
	}
	return urls, nil
}

// transportHost answers which https host reaches the repository origin's fetch url names.
// networked is false for git's local transport (a path, or file://), which carries no key and
// consults no credential helper, so there is no transport to replace.
func transportHost(raw string) (host string, networked bool, err error) {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "file://"):
		return "", false, nil
	case strings.Contains(s, "://"):
		// A scheme URL: https/http/git/ssh. HostOfRemote drops userinfo and port.
	case !deskkit.IsSSHTransport(s):
		return "", false, nil // a bare path (or a Windows drive path): local transport
	}
	h, herr := deskkit.HostOfRemote(s)
	if herr != nil || h == "" {
		return "", true, fmt.Errorf("the origin URL %s names no host", redactURL(s))
	}
	if deskkit.IsSSHTransport(s) && !deskkit.IsWellKnownForgeHost(h) {
		// An SSH host is often an ALIAS (a ~/.ssh/config Host block carrying the key), which
		// no https URL can reach. Resolve it the way ssh would.
		if !transportHostRe.MatchString(h) {
			return "", true, fmt.Errorf("the SSH host %q in the origin URL is not a plain host name", h)
		}
		real, serr := sshConfigHostname(h)
		if serr != nil {
			return "", true, fmt.Errorf("cannot resolve the SSH host alias %q to a host (`ssh -G %s`: %v)", h, h, serr)
		}
		h = strings.ToLower(strings.TrimSpace(real))
	}
	if !transportHostRe.MatchString(h) || !strings.Contains(h, ".") {
		return "", true, fmt.Errorf("the origin host resolves to %q, which is not a qualified host name an https "+
			"URL can reach (an SSH alias with no HostName, or a local name)", h)
	}
	return h, true, nil
}

// redactURL drops any userinfo from an http(s) URL before it is quoted in a message: an https
// origin can carry a credential in it. An SSH URL's user (`git@`) is a login name, not a
// secret, and is kept so the message names the URL git actually resolved.
func redactURL(u string) string {
	lower := strings.ToLower(u)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return u
	}
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		slash := strings.IndexByte(rest, '/')
		auth := rest
		if slash >= 0 {
			auth = rest[:slash]
		}
		if at := strings.LastIndexByte(auth, '@'); at >= 0 {
			return u[:i+3] + "<redacted>@" + rest[at+1:]
		}
	}
	return u
}

// transportRefusal is the ONE refusal shape for a role worktree whose App transport could not
// be established: named, exit 5, and it says the worktree was rolled back.
func transportRefusal(target, role, why string) error {
	return deskkit.Refused(fmt.Sprintf(
		"refused: deskwt add --role %s could not give %s an App-only transport — %s. A role worktree left on "+
			"the transport it inherits from the shared checkout fetches with, and may push under, whatever SSH "+
			"key or ambient credential this machine holds rather than the %s App, so the worktree was ROLLED "+
			"BACK instead of being handed out. Fix the origin remote of the checkout this was cut from (or its "+
			"~/.ssh/config Host block), then re-run.",
		role, target, why, role))
}

// wireRoleTransport gives a freshly created role worktree its own App-only transport (see the
// file header), then proves it by reading back what git itself resolves. It returns a one-line
// detail for the audit line and stderr; any error means the caller must roll the worktree back.
func wireRoleTransport(target, role, repo string) (string, error) {
	fetch, err := originURLs(target, false)
	if err != nil {
		return "", deskkit.Unverifiable("cannot read the origin fetch URL at "+target+" to wire the App transport", err)
	}
	if len(fetch) == 0 {
		return "", deskkit.Unverifiable("origin has no URL at "+target+", so no App transport can be wired", nil)
	}
	host, networked, herr := transportHost(fetch[0])
	if herr != nil {
		return "", transportRefusal(target, role, herr.Error())
	}

	if !networked {
		// git's local transport carries no key and consults no helper — nothing to replace.
		// The PUSH list is judged on its own, though: a local fetch url does not make an SSH
		// pushurl safe.
		push, perr := originURLs(target, true)
		if perr != nil {
			return "", deskkit.Unverifiable("cannot read the origin push URL at "+target, perr)
		}
		for _, u := range push {
			if deskkit.IsSSHTransport(u) {
				return "", transportRefusal(target, role, "origin fetches over git's local transport but pushes to "+
					redactURL(u)+" over SSH, and there is no https host to replace it with")
			}
		}
		if werr := wireRoleCredential(target, role, repo, roleCredUser(role)); werr != nil {
			return "", werr
		}
		return "transport: local origin (no network transport to wire)", nil
	}

	httpsURL := "https://" + host + ":443/" + repo + ".git"
	for _, key := range []string{"remote.origin.pushurl", "remote.origin.url"} {
		if _, werr := runGit(target, "config", "--worktree", "--replace-all", key, ""); werr != nil {
			return "", deskkit.Unverifiable("cannot reset the worktree-scoped "+key+" list at "+target, werr)
		}
		if _, werr := runGit(target, "config", "--worktree", "--add", key, httpsURL); werr != nil {
			return "", deskkit.Unverifiable("cannot set the worktree-scoped "+key+" at "+target, werr)
		}
	}

	// Read back what git ITSELF now resolves. Exactly one URL, the https one, for each
	// direction — anything else (an older git that kept the empty entry, an insteadOf or
	// pushInsteadOf that rewrote the new URL) is the inherited transport still in play.
	for _, push := range []bool{true, false} {
		dir, key := "fetch", "remote.origin.url"
		if push {
			dir, key = "push", "remote.origin.pushurl"
		}
		got, rerr := originURLs(target, push)
		if rerr != nil {
			return "", deskkit.Unverifiable("cannot read back the origin "+dir+" URL at "+target, rerr)
		}
		if len(got) != 1 || got[0] != httpsURL {
			shown := make([]string, len(got))
			for i, u := range got {
				shown[i] = redactURL(u)
			}
			return "", transportRefusal(target, role, fmt.Sprintf(
				"after writing a worktree-scoped %s list reset plus %s, git still resolves the %s URL(s) of "+
					"origin to [%s] rather than exactly that one URL (an empty entry resets the list only on "+
					"git 2.46 or later; a url.*.insteadOf / pushInsteadOf rule can also rewrite it)",
				key, httpsURL, dir, strings.Join(shown, ", ")))
		}
	}

	if werr := wireRoleCredential(target, role, repo, roleCredUser(role)); werr != nil {
		return "", werr
	}
	fmt.Fprintln(os.Stderr, "deskwt: worktree-scoped App transport for origin: fetch and push "+httpsURL)
	return "transport " + httpsURL + " (fetch+push, worktree-scoped)", nil
}
