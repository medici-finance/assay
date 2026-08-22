// Package gitexec is the single audited git-binary fallback seam (desktools-go-git/01).
//
// Every desk tool that still needs the `git` binary will route through this package
// instead of its own per-tool seam. The package owns the two properties every caller
// used to re-implement for itself (issue #1555 finding 1):
//
//   - a scrubbed child environment: only an explicit allowlist of variables is passed,
//     so GIT_SSH_COMMAND / GIT_ASKPASS / GIT_CONFIG_COUNT|KEY_*|VALUE_* (config
//     injection) and every other GIT_* var never reach the child, and
//   - a per-verb + tool allowlist consulted BEFORE anything runs, so a caller cannot
//     smuggle a verb this seam has not been audited for.
//
// The allowlist is seeded with today's full verb set (see
// docs/streams/desktools-go-git/inventory.md for the frozen inventory) and is counted
// DOWNWARD over the course of the stream: later briefs empty it toward a single
// sanctioned entry — deskmerge's trial merge (a three-way merge go-git cannot express;
// human-gated, run only on the desk machine, never by agents). That single entry is the
// stream's documented exception, not a deferral: `allowlist` must be shrunk, not grown,
// and any edit must name the brief that empties the row.
package gitexec

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// verbTool is the allowlist key: one audited (tool, verb) pair.
type verbTool struct {
	tool string // calling tool's command name, e.g. "deskmerge"
	verb string // first git argv token, e.g. "merge"
}

// allowlist is the audited set of permitted git verbs, keyed by calling tool.
//
// Seeded with the union of today's per-tool verbs (inventory.md families 1-25 at
// baseline). The stream's contract: this set shrinks to ONE entry — deskmerge's trial
// merge — by the end of the stream (briefs 02-07 migrate callers; brief 08 flips the
// CI counter to failing outside this list). Until then each entry below documents the
// tool that still uses it; deleting an entry is only valid once that tool's caller has
// migrated to gitcore.
var allowlist = map[verbTool]bool{
	// deskmerge's trial merge — the single sanctioned git-binary caller. go-git's
	// merge is fast-forward-only (no three-way merge, no conflict-stage enumeration,
	// no parent verification), so this one verb cannot migrate and stays fenced here.
	{tool: "deskmerge", verb: "merge"}: true,

	// Remaining deskmerge verbs — migrate to gitcore in brief 06.
	{tool: "deskmerge", verb: "rev-parse"}:  true,
	{tool: "deskmerge", verb: "diff"}:       true,
	{tool: "deskmerge", verb: "worktree"}:   true, // linked worktrees: go-git gap, follow-on stream
	{tool: "deskmerge", verb: "rev-list"}:   true,
	{tool: "deskmerge", verb: "fetch"}:      true,
	{tool: "deskmerge", verb: "update-ref"}: true,
	{tool: "deskmerge", verb: "remote"}:     true,
	{tool: "deskmerge", verb: "push"}:       true,
	{tool: "deskmerge", verb: "merge-base"}: true,
	{tool: "deskmerge", verb: "commit"}:     true,
	{tool: "deskmerge", verb: "add"}:        true,

	// Read-heavy tools (briefs 03-05).
	{tool: "deskgit", verb: "ls-remote"}:         true,
	{tool: "deskgit", verb: "rev-parse"}:         true,
	{tool: "deskgit", verb: "for-each-ref"}:      true,
	{tool: "deskwt", verb: "rev-parse"}:          true,
	{tool: "deskwt", verb: "worktree"}:           true, // linked worktrees: go-git gap, follow-on stream
	{tool: "deskwt", verb: "config"}:             true,
	{tool: "deskwt", verb: "rev-list"}:           true,
	{tool: "deskwt", verb: "status"}:             true,
	{tool: "deskwt", verb: "merge-base"}:         true,
	{tool: "deskscanbody", verb: "merge-base"}:   true,
	{tool: "deskscanbody", verb: "diff"}:         true,
	{tool: "deskscanbody", verb: "clean"}:        true,
	{tool: "deskpr", verb: "rev-parse"}:          true,
	{tool: "deskpr", verb: "push"}:               true,
	{tool: "deskpr", verb: "diff"}:               true,
	{tool: "deskpr", verb: "symbolic-ref"}:       true,
	{tool: "deskpr", verb: "rev-list"}:           true,
	{tool: "deskpr", verb: "config"}:             true,
	{tool: "deskreply", verb: "rev-parse"}:       true,
	{tool: "deskreply", verb: "config"}:          true,
	{tool: "deskpushguard", verb: "rev-parse"}:   true,
	{tool: "deskpushguard", verb: "log"}:         true,
	{tool: "deskpushguard", verb: "show"}:        true,
	{tool: "deskpushguard", verb: "merge-base"}:  true,
	{tool: "deskpushguard", verb: "cat-file"}:    true,
	{tool: "deskpushguard", verb: "branch"}:      true,
	{tool: "deskpushguard", verb: "rev-list"}:    true,
	{tool: "deskpushguard", verb: "remote"}:      true,
	{tool: "deskpushguard", verb: "ls-tree"}:     true,
	{tool: "deskboard", verb: "rev-parse"}:       true,
	{tool: "deskboard", verb: "status"}:          true,
	{tool: "deskboard", verb: "diff"}:            true,
	{tool: "writeguard", verb: "rev-parse"}:      true,
	{tool: "desksourceguard", verb: "rev-parse"}: true,
	{tool: "deskadvisory", verb: "init"}:         true,
	{tool: "deskadvisory", verb: "fetch"}:        true, // third-party-fork fetch; hardening disappears with gitcore transport
	{tool: "deskadvisory", verb: "commit"}:       true,
	{tool: "deskadvisory", verb: "checkout"}:     true,
	{tool: "verifyloop", verb: "push"}:           true, // durable-Evidence push half migrates; pull --rebase is a follow-on gap
	{tool: "verifyloop", verb: "pull"}:           true, // go-git gap: rebase / non-fast-forward pull — follow-on design brief
	{tool: "verifyloop", verb: "commit"}:         true,
	{tool: "verifyloop", verb: "add"}:            true,
	{tool: "deskkit", verb: "remote"}:            true, // preflight ls-remote --get-url probe; replaced by authenticated List
	{tool: "deskkit", verb: "symbolic-ref"}:      true,
	{tool: "deskkit", verb: "push"}:              true, // preflight transport probe
	{tool: "deskkit", verb: "config"}:            true,
}

// envAllowlist is the ONLY set of environment variables passed to the child git
// process (mirrors deskgit's scrubbed env, issue #1555). A fixed argv closes flags but
// NOT the environment: GIT_SSH_COMMAND, GIT_PROXY_COMMAND, GIT_ASKPASS, and the
// GIT_CONFIG_COUNT/GIT_CONFIG_KEY_*/GIT_CONFIG_VALUE_* config-injection trio all name a
// program git will execute or config git will honour. Membership is by exact name or,
// for LC_, by prefix. Everything else — including every GIT_* var — is dropped.
var envAllowlist = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TERM": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"LANG": true, "LANGUAGE": true, "TZ": true,
	// ssh-agent socket — names a socket, not a program; carries no execution surface.
	"SSH_AUTH_SOCK": true,
}

// scrubbedEnv returns the child environment: allowlisted vars from the parent, plus
// GIT_TERMINAL_PROMPT=0 so a scrubbed-away askpass can never become an interactive
// hang. Exposed for tests.
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
	out = append(out, "GIT_TERMINAL_PROMPT=0")
	return out
}

// Allowed reports whether the (tool, verb) pair is on the audited allowlist.
func Allowed(tool, verb string) bool { return allowlist[verbTool{tool: tool, verb: verb}] }

// Run executes `git <args...>` in dir for the named tool and returns trimmed stdout.
// It refuses — before spawning anything — unless the (tool, args[0]) pair is
// allowlisted, and it spawns the child with the scrubbed environment only.
//
// args is an explicit slice built from literal verbs — never a shell string and never
// a raw caller flag — so `--upload-pack`/`--exec`/a refspec cannot be injected.
func Run(tool, dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("gitexec: %s: empty argv", tool)
	}
	if !Allowed(tool, args[0]) {
		return "", fmt.Errorf("gitexec: %s: verb %q is not allowlisted for this tool", tool, args[0])
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = scrubbedEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return strings.TrimSpace(string(out)), fmt.Errorf("gitexec: %s: git %s: %s", tool, args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return strings.TrimSpace(string(out)), fmt.Errorf("gitexec: %s: git %s: %w", tool, args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}
