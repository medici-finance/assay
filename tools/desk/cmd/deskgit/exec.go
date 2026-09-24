package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execCommand is the single seam through which every git invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv (so the pinned-argv
// assertions — `--refmap= --upload-pack=git-upload-pack`, the explicit refspec, no
// caller flag — are checked on the real constructed argv) while still delegating to a
// real git process against a scratch fixture. Nothing else in this package constructs
// commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// envAllowlist is the ONLY set of environment variables passed to the child git
// process (issue #1555 security review, finding 1). A fixed argv closes flags but NOT
// the environment: `GIT_SSH_COMMAND`, `GIT_PROXY_COMMAND`, `GIT_ASKPASS`, and the
// `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_*`/`GIT_CONFIG_VALUE_*` config-injection trio all
// name a program git will execute or config git will honour (including
// `remote.origin.uploadpack`, the code-execution vector). deskgit builds the entire git
// invocation itself, so it needs NO `GIT_*` var inherited — we pass a curated allowlist
// and drop everything else. Membership is by exact name or, for `LC_`, by prefix.
var envAllowlist = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TERM": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"LANG": true, "LANGUAGE": true, "TZ": true,
	// ssh-agent socket — needed for key auth on an ssh origin (git@github…). It names
	// a socket, not a program, so it carries no execution surface.
	"SSH_AUTH_SOCK": true,
}

// scrubbedEnv returns the child environment: the allowlisted vars from the parent, plus
// GIT_TERMINAL_PROMPT=0 so a scrubbed-away askpass can never turn into an interactive
// hang. Every GIT_* var (and everything else not allowlisted) is dropped. Exposed as a
// package function so a test can assert the scrub directly.
func scrubbedEnv(parent []string) []string {
	out := make([]string, 0, len(envAllowlist)+1)
	for _, kv := range parent {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		k := kv[:eq]
		if envAllowlist[k] || strings.HasPrefix(k, "LC_") {
			out = append(out, kv)
		}
	}
	// Never block on a credential/askpass prompt (we dropped GIT_ASKPASS/SSH_ASKPASS).
	out = append(out, "GIT_TERMINAL_PROMPT=0")
	return out
}

// runGit executes `git <args...>` in dir and returns trimmed stdout. Two properties make
// it safe against an attacker-influenced repo/environment (issue #1555):
//   - the argv is an explicit slice built from literal verbs — never a shell string and
//     never a raw caller flag, so `--upload-pack`/`--exec`/a refspec cannot be injected;
//   - the child environment is scrubbed to `envAllowlist`, so no inherited `GIT_*` var
//     can name a program to run or inject `remote.origin.uploadpack` via config.
//
// It is the credential-free path (every fetch mode, and every repo probe): it passes the
// bare scrubbed allowlist, in which GIT_ASKPASS is deliberately absent. The authenticated
// verbs go through runGitWithEnv with the credentialSupply env instead.
func runGit(dir string, args ...string) (string, error) {
	return runGitWithEnv(dir, scrubbedEnv(os.Environ()), args...)
}

// runGitWithEnv is runGit with the child environment supplied by the caller, for the
// authenticated forms (`push --as`, `fetch --as`). The env MUST be a scrubbedEnv-derived
// slice — the ONLY addition the caller may make is the controlled DESKGIT_TOKEN that
// credentialSupply appends, never a passthrough of os.Environ(); every
// other guarantee runGit makes (explicit argv, no shell string) is preserved because the
// argv is still an explicit slice built from literal verbs plus validated values.
func runGitWithEnv(dir string, env []string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout := strings.TrimSpace(out.String())
	if err != nil {
		return stdout, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "),
			err, strings.TrimSpace(errb.String()))
	}
	return stdout, nil
}

// credentialHost is the ONE host the `--as` credential is ever answered for. The token is a
// GitHub App installation token; it authenticates nowhere else, so no other host may be offered
// it (sec-1587-S1).
const credentialHost = "github.com"

// credentialHelperKey is the host-scoped config key the ephemeral helper is installed under. Git
// consults a `credential.<url>.helper` only for a credential request whose URL matches <url>, so
// a request for any other host — a submodule's own remote, an http.proxy that asks for its
// password, a destination rewritten after the gate read it — never reaches the helper at all.
// This is the scoping `deskwt role-init` already uses for the worktree helper it persists.
const credentialHelperKey = "credential.https://" + credentialHost + ".helper"

// credentialHelperScript is the ephemeral helper's body. It is the SECOND layer at the answer
// point: even when git does call it, it answers only a `get` whose request says protocol=https and
// host=github.com (or github.com:443), and prints nothing for anything else, so git treats every
// other request as unanswered. The token VALUE never appears in the script — only the variable
// name DESKGIT_TOKEN does.
const credentialHelperScript = `#!/bin/sh
[ "$1" = get ] || exit 0
proto=
host=
while IFS= read -r line; do
  [ -n "$line" ] || break
  case "$line" in
    protocol=*) proto=${line#protocol=} ;;
    host=*) host=${line#host=} ;;
  esac
done
[ "$proto" = https ] || exit 0
case "$host" in
  github.com|github.com:443) ;;
  *) exit 0 ;;
esac
echo username=x-access-token
printf 'password=%s\n' "$DESKGIT_TOKEN"
`

// credentialTempParent is the parent directory the ephemeral helper dir is created under.
// Empty means os.MkdirTemp's default (the OS temp dir), which is production. A test points it
// at a scratch dir so it can assert the ephemeral dir is REMOVED after the call — the
// leak-on-error-path check — without racing other processes' /tmp entries.
var credentialTempParent = ""

// credentialSupply builds the credential channel for an authenticated git invocation. The token
// is bound where it is ANSWERED, not only where destinations are listed (sec-1587-S1, round 3):
// an answer channel that replies to any prompt — the GIT_ASKPASS script this replaced — hands the
// token to every host git talks to during the verb, including hosts no destination list names (a
// recursed submodule's remote, a user-bearing proxy). So:
//
//   - the argv prefix `-c credential.helper=` clears every helper accumulated from system, global,
//     repo and worktree config (an empty value resets the list, and command-line config is read
//     last), so no ambient or configured helper is ever consulted;
//   - `-c credential.https://github.com.helper=!'<script>'` then adds ONE ephemeral helper under
//     the host-scoped key, so git asks it only for https://github.com; the script itself also
//     refuses any request that is not protocol=https, host=github.com (credentialHelperScript);
//   - no GIT_ASKPASS is set, and the env is the scrubbed allowlist with GIT_TERMINAL_PROMPT=0, so
//     a prompt the helper does not answer (a proxy password, a foreign host) fails closed instead
//     of being answered with the token.
//
// Every property that keeps the token off durable surfaces is unchanged: it reaches the child
// through ONE environment variable (DESKGIT_TOKEN), never argv, a URL, the audit line, or any
// file; the script lives in a private 0700-perms os.MkdirTemp dir removed by the returned
// cleanup, which the caller defers so it runs on EVERY return path including error.
func credentialSupply(token string) (env, argvPrefix []string, cleanup func(), err error) {
	cleanup = func() {}
	dir, derr := os.MkdirTemp(credentialTempParent, "deskgit-cred-*")
	if derr != nil {
		return nil, nil, cleanup, fmt.Errorf("cannot create credential helper temp dir: %w", derr)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	script := filepath.Join(dir, "credential-helper.sh")
	// The helper value is run by git through the shell, so the path is single-quoted into it. A
	// path that itself carries a quote cannot be quoted safely: refuse rather than guess.
	if strings.ContainsAny(script, "'\n") {
		cleanup()
		return nil, nil, func() {}, fmt.Errorf("credential helper path %q cannot be quoted into a helper command", script)
	}
	if werr := os.WriteFile(script, []byte(credentialHelperScript), 0o700); werr != nil {
		cleanup()
		return nil, nil, func() {}, fmt.Errorf("cannot write credential helper script: %w", werr)
	}
	env = scrubbedEnv(os.Environ())
	env = append(env, "DESKGIT_TOKEN="+token)
	argvPrefix = []string{"-c", "credential.helper=", "-c", credentialHelperKey + "=!'" + script + "'"}
	return env, argvPrefix, cleanup, nil
}
