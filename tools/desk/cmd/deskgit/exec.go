package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
func runGit(dir string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = scrubbedEnv(os.Environ())
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
