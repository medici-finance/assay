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
// verbs go through runGitWithEnv with an askpass-bearing env instead.
func runGit(dir string, args ...string) (string, error) {
	return runGitWithEnv(dir, scrubbedEnv(os.Environ()), args...)
}

// runGitWithEnv is runGit with the child environment supplied by the caller, for the
// authenticated forms (`push --as`, `fetch --as`). The env MUST be a scrubbedEnv-derived
// slice — the ONLY additions the caller may make are the controlled GIT_ASKPASS and
// DESKGIT_TOKEN that askpassSupply appends, never a passthrough of os.Environ(); every
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

// askpassSupply builds the credential channel for an authenticated git invocation, reusing
// the deskadvisory pattern (advisory.go § writeAskpass) unchanged in every property that
// keeps the token off every durable surface:
//
//   - the token reaches the child through ONE environment variable (DESKGIT_TOKEN) and an
//     ephemeral GIT_ASKPASS script that echoes it; it is never placed in argv, in a URL, in
//     the audit line, or in any file that outlives the call;
//   - the script lives in a private 0700-perms os.MkdirTemp dir removed by the returned
//     cleanup, which the caller defers so it runs on EVERY return path including error;
//   - the argv prefix `-c credential.helper=` clears the helper list on the command line, so
//     no ambient/configured credential helper (the shadowing the transcript sweep observed)
//     is ever consulted — only this askpass answers.
//
// The env is the scrubbed allowlist plus exactly the two variables above; GIT_ASKPASS is
// added AFTER the scrub, the one controlled exception to "every GIT_* var is dropped".
// askpassTempParent is the parent directory the ephemeral askpass dir is created under.
// Empty means os.MkdirTemp's default (the OS temp dir), which is production. A test points
// it at a scratch dir so it can assert the ephemeral dir is REMOVED after the call — the
// leak-on-error-path check — without racing other processes' /tmp entries.
var askpassTempParent = ""

func askpassSupply(token string) (env, argvPrefix []string, cleanup func(), err error) {
	cleanup = func() {}
	dir, derr := os.MkdirTemp(askpassTempParent, "deskgit-askpass-*")
	if derr != nil {
		return nil, nil, cleanup, fmt.Errorf("cannot create askpass temp dir: %w", derr)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	script := filepath.Join(dir, "askpass.sh")
	// The Username prompt is answered `x-access-token` (the GitHub App-token username); every
	// other prompt (the password) gets the token from DESKGIT_TOKEN. The token VALUE never
	// appears in this script — only the variable name does.
	body := "#!/bin/sh\ncase \"$1\" in\n  *Username*) echo \"x-access-token\" ;;\n  *) echo \"$DESKGIT_TOKEN\" ;;\nesac\n"
	if werr := os.WriteFile(script, []byte(body), 0o700); werr != nil {
		cleanup()
		return nil, nil, func() {}, fmt.Errorf("cannot write askpass script: %w", werr)
	}
	env = scrubbedEnv(os.Environ())
	env = append(env, "GIT_ASKPASS="+script, "DESKGIT_TOKEN="+token)
	argvPrefix = []string{"-c", "credential.helper="}
	return env, argvPrefix, cleanup, nil
}
