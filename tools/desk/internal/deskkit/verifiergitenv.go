package deskkit

import (
	"os"
	"runtime"
	"strings"
)

// VerifierGitConfigGlobal is the fixed global Git configuration file of a
// verifier run: the null device. It holds no include, so no operator
// includeIf rule reaches a verifier run. Identity, credential helper and
// remote keys are written into the verifier home's own config (deskwt
// role-init, `git config --worktree`), never through an include.path file.
//
// The null device is also the one fixed file a verifyrun admitted row keeps
// (statusgen admittedRowGlobal), so the declared value survives row
// admission without widening that control.
var VerifierGitConfigGlobal = os.DevNull

// verifierGitUnset names the variables a verifier launch REMOVES from the
// environment it inherits. Removed, never set empty: an empty GIT_CONFIG
// names an empty file path to `git config`, which then reports present keys as
// absent (rc 1) and refuses --worktree and --global writes, while transport
// still honours the real configuration. GIT_DIR, GIT_WORK_TREE and
// GIT_COMMON_DIR would redirect every git call away from the verifier home;
// GIT_SSH_COMMAND, GIT_ASKPASS and SSH_ASKPASS would put an operator program
// on the transport and credential path.
var verifierGitUnset = []string{
	"GIT_CONFIG",
	"GIT_CONFIG_PARAMETERS",
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_COMMON_DIR",
	"GIT_SSH_COMMAND",
	"GIT_ASKPASS",
	"SSH_ASKPASS",
}

// VerifierGitUnset returns the variables a verifier launch removes from the
// environment it inherits (a fresh copy).
func VerifierGitUnset() []string { return append([]string(nil), verifierGitUnset...) }

// VerifierGitLaunchEnv is the set of variables a verifier launch DECLARES: the
// operator's global and system configuration are not read, injected
// configuration is off (GIT_CONFIG_COUNT=0), and git never prompts on a
// terminal. It is only half of the launch environment; VerifierGitEnv also
// removes VerifierGitUnset from the inherited environment, and a caller that
// spawns through an API which can only append must remove those itself.
//
// It isolates the INPUT of the origin-authority reader. The reader itself
// stays strict and unchanged: an include that may affect authority, if one is
// still in scope (for example an include.path in the clone's own config), is
// still refused as could-not-check.
func VerifierGitLaunchEnv() []string {
	return []string{
		"GIT_CONFIG_GLOBAL=" + VerifierGitConfigGlobal,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=0",
		"GIT_TERMINAL_PROMPT=0",
	}
}

// VerifierGitEnv returns base with every VerifierGitUnset variable and every
// variable VerifierGitLaunchEnv declares removed, then the declared values
// appended. It is the single choke point for a verifier launch environment.
func VerifierGitEnv(base []string) []string {
	drop := VerifierGitUnset()
	declared := VerifierGitLaunchEnv()
	for _, kv := range declared {
		drop = append(drop, envKey(kv))
	}
	out := make([]string, 0, len(base)+len(declared))
	for _, kv := range base {
		if !envKeyIn(envKey(kv), drop) {
			out = append(out, kv)
		}
	}
	return append(out, declared...)
}

// VerifierGitShell is the POSIX shell form of VerifierGitEnv for a verifier
// shell: an `unset` line, then an `export` line.
func VerifierGitShell() string {
	return "unset " + strings.Join(VerifierGitUnset(), " ") + "\nexport " + strings.Join(VerifierGitLaunchEnv(), " ")
}

func envKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}

func envKeyIn(key string, keys []string) bool {
	for _, k := range keys {
		if key == k || (runtime.GOOS == "windows" && strings.EqualFold(key, k)) {
			return true
		}
	}
	return false
}
