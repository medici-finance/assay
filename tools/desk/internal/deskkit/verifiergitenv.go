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

// verifierGitDropped reports whether key is removed from the environment a
// verifier Git process inherits: EVERY GIT_* variable (matched case-blind, as
// verifierEnv always has), plus SSH_ASKPASS. A denylist of named GIT_*
// variables leaves the rest open: GIT_OBJECT_DIRECTORY,
// GIT_ALTERNATE_OBJECT_DIRECTORIES, GIT_INDEX_FILE and GIT_NAMESPACE redirect
// what a read resolves; GIT_SSH and GIT_EXEC_PATH put an operator program on
// the transport or subcommand path; GIT_SSL_NO_VERIFY turns TLS verification
// off. Removed, never set empty: an empty GIT_CONFIG names an empty file path
// to `git config`, which then reports present keys as absent (rc 1) and refuses
// --worktree and --global writes, while transport still honours the real
// configuration.
func verifierGitDropped(key string) bool {
	if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
		return true
	}
	return key == "SSH_ASKPASS" || (runtime.GOOS == "windows" && strings.EqualFold(key, "SSH_ASKPASS"))
}

// withoutInheritedGit returns base with every verifierGitDropped variable
// removed. It is the one filter both verifier Git environments start from:
// VerifierGitEnv (the launch environment) and verifierEnv (admission reads).
func withoutInheritedGit(base []string) []string {
	out := make([]string, 0, len(base))
	for _, kv := range base {
		if !verifierGitDropped(envKey(kv)) {
			out = append(out, kv)
		}
	}
	return out
}

// VerifierGitUnset returns the names of the variables in base that a verifier
// launch removes, for a spawn API that can only remove named variables from
// the environment it inherits (pass that inherited environment as base).
func VerifierGitUnset(base []string) []string {
	var keys []string
	for _, kv := range base {
		if k := envKey(kv); verifierGitDropped(k) {
			keys = append(keys, k)
		}
	}
	return keys
}

// VerifierGitLaunchEnv is the set of variables a verifier launch DECLARES: the
// operator's global and system configuration are not read, injected
// configuration is off (GIT_CONFIG_COUNT=0), and git never prompts on a
// terminal. It is only half of the launch environment; VerifierGitEnv also
// removes every inherited GIT_* variable and SSH_ASKPASS, and a caller that
// spawns through an API which can only append must remove those itself
// (VerifierGitUnset names them).
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

// VerifierGitEnv returns base with every inherited GIT_* variable and
// SSH_ASKPASS removed (withoutInheritedGit), then the declared values appended.
// It is the single choke point for a verifier launch environment.
func VerifierGitEnv(base []string) []string {
	return append(withoutInheritedGit(base), VerifierGitLaunchEnv()...)
}

// verifierGitShellUnset is the POSIX shell form of withoutInheritedGit: it
// unsets every GIT_* variable the shell exports (case-blind), whatever its
// name, and SSH_ASKPASS.
const verifierGitShellUnset = `for v in $(env | sed -n 's/^\([Gg][Ii][Tt]_[A-Za-z0-9_]*\)=.*/\1/p') SSH_ASKPASS; do unset "$v"; done`

// VerifierGitShell is the POSIX shell form of VerifierGitEnv for a verifier
// shell: a line that unsets every inherited GIT_* variable and SSH_ASKPASS,
// then an `export` line.
func VerifierGitShell() string {
	return verifierGitShellUnset + "\nexport " + strings.Join(VerifierGitLaunchEnv(), " ")
}

func envKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}
