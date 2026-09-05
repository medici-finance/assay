package deskkit

// hooks.go — operator-configured lifecycle hooks, loaded from the desk-tools STATE
// DIRECTORY and from nowhere else.
//
// # What this is
//
// The per-run envelope every dispatched agent is asked to remember — export
// KUBECONFIG=/dev/null, set the worktree-local credential helper and commit identity,
// clean up on removal — is prose residue spread across a prompt kit, a skill body and a
// role-init verb, and nothing runs at run-END at all. This makes those four moments a
// declarative, CHECKED configuration (mirroring OpenAI Symphony's hook block) instead of a
// sentence: `after_create`, `before_run`, `after_run`, `before_remove`, each a shell
// snippet the tools run at the matching lifecycle point.
//
// # The single-point-of-failure: the SOURCE RULE
//
// Hooks load from `<StateDir>/hooks.yaml` and from NOWHERE else. The state directory is the
// same $HOME-anchored, deliberately-non-relocatable directory the kill switch and the audit
// log live in (killswitch.go's deskDir): an operator-relocatable hooks path would let a
// caller move — and so bypass — the hooks the same way relocating the state dir would bypass
// the kill switch. So there is NO hooks-path flag anywhere, LoadHooks takes no path
// argument, and a `hooks.yaml` placed inside a worktree or a repo root is NEVER read. The
// negative control — a hooks.yaml in the item's own tree is inert — is the layer behind the
// source rule, and its test (TestHooksIgnoreItemTreeFile) is what proves the layer exists.
// This matters because the item's tree is untrusted content: an agent (or an untrusted PR
// head) that could drop a hooks.yaml the tools would execute is exactly the surface this
// closes.
//
// # Failure classes (mirrors Symphony §5.3.4)
//
//   - after_create : new worktree only; failure ABORTS creation (fatal).
//   - before_run   : each attempt, after worktree preparation; failure ABORTS the attempt (fatal).
//   - after_run    : each attempt end, any outcome; failure is LOGGED, the run's result stands.
//   - before_remove: before deletion; failure is LOGGED, the deletion proceeds.
//
// RunHook itself never decides fatality — it returns (ran, err) and the CALLER applies the
// class above (HookFatalOnFailure reports it). This keeps the four call sites honest: each
// states, at its own site, what it does with a hook failure.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Hook names — the four lifecycle moments, mirroring Symphony exactly. They double as the
// value of the ASSAY_HOOK environment variable a running hook sees, so a single hook script
// dispatched on $ASSAY_HOOK can tell which moment fired it.
const (
	HookAfterCreate  = "after_create"
	HookBeforeRun    = "before_run"
	HookAfterRun     = "after_run"
	HookBeforeRemove = "before_remove"
)

// DefaultHookTimeoutMS is the per-hook wall-clock budget when hooks.yaml sets none (or sets
// a non-positive value). It mirrors Symphony's timeout_ms default of 60000.
const DefaultHookTimeoutMS = 60000

// maxHookOutputBytes bounds how much of a hook's combined stdout+stderr is carried into the
// audit line's detail / the failure error. Hook output is operator-authored and can be
// large; it is a diagnostic, not a payload, so it is truncated rather than streamed.
const maxHookOutputBytes = 2000

// Hooks is the schema of <StateDir>/hooks.yaml. Each field is a shell snippet run at the
// matching lifecycle moment; an empty field is a no-op. Unknown keys in the file are
// ignored (yaml.v3 does not error on them without KnownFields), so a newer file can carry
// keys an older binary does not know without breaking it.
type Hooks struct {
	AfterCreate  string `yaml:"after_create"`
	BeforeRun    string `yaml:"before_run"`
	AfterRun     string `yaml:"after_run"`
	BeforeRemove string `yaml:"before_remove"`
	// TimeoutMS is the per-hook wall-clock budget in milliseconds. Non-positive ⇒
	// DefaultHookTimeoutMS. It applies to every hook.
	TimeoutMS int `yaml:"timeout_ms"`
}

// script returns the shell snippet for a hook name, or "" for an unknown name.
func (h Hooks) script(name string) string {
	switch name {
	case HookAfterCreate:
		return h.AfterCreate
	case HookBeforeRun:
		return h.BeforeRun
	case HookAfterRun:
		return h.AfterRun
	case HookBeforeRemove:
		return h.BeforeRemove
	default:
		return ""
	}
}

// timeout resolves the effective per-hook timeout: the configured value, or the default
// when it is non-positive.
func (h Hooks) timeout() time.Duration {
	if h.TimeoutMS <= 0 {
		return time.Duration(DefaultHookTimeoutMS) * time.Millisecond
	}
	return time.Duration(h.TimeoutMS) * time.Millisecond
}

// Has reports whether a non-empty script is configured for the named hook.
func (h Hooks) Has(name string) bool {
	return strings.TrimSpace(h.script(name)) != ""
}

// HookDryRunLine is the ONE rendering of a hook's dry-run status, so every call site prints
// the same shape: `HOOK <name>: would run` when a script is configured, `HOOK <name>: none`
// otherwise. A hooks file the loader cannot read is Unverifiable — a dry run reports the
// truth or refuses, it never prints a reassuring "none" over a file it could not parse.
func HookDryRunLine(name string) (string, error) {
	h, err := LoadHooks()
	if err != nil {
		return "", err
	}
	if h.Has(name) {
		return "HOOK " + name + ": would run", nil
	}
	return "HOOK " + name + ": none", nil
}

// HookFatalOnFailure reports whether a failure of the named hook aborts the operation
// (after_create, before_run) or is merely logged (after_run, before_remove). The four call
// sites read this so the failure class lives in ONE place rather than being re-decided at
// each site.
func HookFatalOnFailure(name string) bool {
	return name == HookAfterCreate || name == HookBeforeRun
}

// hooksFileName is the ONE file name hooks are ever read from, under the state directory.
const hooksFileName = "hooks.yaml"

// HooksPath returns the ONE path hooks are loaded from — <StateDir>/hooks.yaml — for
// documentation and error messages. It never consults any argument, flag, cwd, worktree, or
// repo root: the source rule is that the state directory is the only home.
func HooksPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", Unverifiable("cannot resolve the desk-tools state directory for hooks.yaml", err)
	}
	return filepath.Join(dir, hooksFileName), nil
}

// LoadHooks reads <StateDir>/hooks.yaml. A MISSING file yields the zero Hooks (every hook a
// no-op — the tools behave exactly as before hooks existed), nil error. An unreadable or
// malformed file is Unverifiable (fail closed): a hooks file the tools cannot positively
// parse is never silently treated as empty, because "no hooks" and "hooks I could not read"
// are different states and only one of them is safe to proceed on.
//
// It takes NO path argument by construction — see the source-rule note at the top of this
// file. The file is read fresh on every call: the desk verbs are one-shot, so an edit
// applies to the next invocation with no restart.
func LoadHooks() (Hooks, error) {
	var h Hooks
	path, err := HooksPath()
	if err != nil {
		return h, err
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return h, nil
		}
		return h, Unverifiable("cannot read "+path, rerr)
	}
	if merr := yaml.Unmarshal(b, &h); merr != nil {
		return h, Unverifiable("cannot parse "+path+" as hooks YAML", merr)
	}
	return h, nil
}

// HookEnv carries the fixed variables every hook receives on top of the (scrubbed) caller
// environment. Worktree is both the ASSAY_WORKTREE value and the working directory the hook
// runs in.
type HookEnv struct {
	RunKey   string // ASSAY_RUN_KEY — the item / claim key this run is for
	Worktree string // ASSAY_WORKTREE — the worktree path (and the hook's cwd)
	Repo     string // ASSAY_REPO — owner/name
	Role     string // ASSAY_ROLE — the dispatched agent class / desk role
}

// RunHook loads the hooks file and runs the named hook. It returns:
//
//   - (false, nil) when no script is configured for the name — a no-op, the common case;
//   - (true, nil)  when the hook ran and exited 0;
//   - (true, err)  when the hook ran and failed (non-zero exit, or timeout);
//   - (false, err) when the hooks file itself could not be read/parsed.
//
// The CALLER applies the per-hook failure class (HookFatalOnFailure): RunHook never decides
// whether a failure aborts. On failure the truncated combined stdout+stderr is folded into
// err so the caller's audit line detail carries what the hook said; hook output NEVER
// reaches an agent's prompt.
func RunHook(name string, he HookEnv) (ran bool, err error) {
	h, lerr := LoadHooks()
	if lerr != nil {
		return false, lerr
	}
	return h.Run(name, he)
}

// Run executes the named hook of an already-loaded Hooks value. Split from RunHook so a
// caller that has already loaded the file (or a test constructing a Hooks directly) can run
// a hook without a second read; RunHook is the load-then-run convenience the call sites use.
func (h Hooks) Run(name string, he HookEnv) (ran bool, err error) {
	script := strings.TrimSpace(h.script(name))
	if script == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout())
	defer cancel()

	// /bin/sh -c is the shell every desk host has; the snippet is operator-authored config
	// from a $HOME-owned file (the same trust root as the kill switch), never agent- or
	// PR-supplied text — the source rule is what guarantees that.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	if he.Worktree != "" {
		cmd.Dir = he.Worktree
	}
	cmd.Env = hookEnviron(name, he)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return true, Unverifiable(fmt.Sprintf("hook %s timed out after %s (killed)%s",
			name, h.timeout(), detailSuffix(out.Bytes())), ctx.Err())
	}
	if runErr != nil {
		return true, Unverifiable(fmt.Sprintf("hook %s failed: %v%s",
			name, runErr, detailSuffix(out.Bytes())), runErr)
	}
	return true, nil
}

// hookEnviron builds the hook's environment: the caller's environment MINUS any variable
// whose name matches a secret shape (contains TOKEN / SECRET / PEM, or begins GH_), PLUS the
// fixed ASSAY_* variables. Scrubbing secrets is the env-scrub control the gate confirms: a
// hook is operator shell, but it must not be the path by which an App token or PEM leaks
// into an operator's log or a child process it spawns.
func hookEnviron(name string, he HookEnv) []string {
	var env []string
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		if isSecretEnvName(kv[:eq]) {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"ASSAY_RUN_KEY="+he.RunKey,
		"ASSAY_WORKTREE="+he.Worktree,
		"ASSAY_REPO="+he.Repo,
		"ASSAY_ROLE="+he.Role,
		"ASSAY_HOOK="+name,
	)
	return env
}

// isSecretEnvName reports whether an environment variable NAME matches one of the scrub
// shapes: contains TOKEN, contains SECRET, contains PEM, or begins with GH_. The match is
// case-insensitive so a lowercase spelling cannot slip a secret past the filter.
func isSecretEnvName(name string) bool {
	u := strings.ToUpper(name)
	if strings.HasPrefix(u, "GH_") {
		return true
	}
	return strings.Contains(u, "TOKEN") ||
		strings.Contains(u, "SECRET") ||
		strings.Contains(u, "PEM")
}

// detailSuffix renders a hook's (truncated) combined output for an error/audit detail, or
// "" when the hook produced none.
func detailSuffix(b []byte) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	if len(s) > maxHookOutputBytes {
		s = s[:maxHookOutputBytes] + "…(truncated)"
	}
	return " — output: " + StripControl(s)
}
