package deskkit

import "os"

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

// VerifierGitLaunchEnv is the declared Git environment of a verifier launch.
// Append it last to the spawned environment so it wins over any inherited
// value: the operator's global and system configuration are not read, and
// injected configuration (GIT_CONFIG_COUNT, GIT_CONFIG_PARAMETERS) is
// cleared.
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
		"GIT_CONFIG_PARAMETERS=",
		"GIT_CONFIG=",
	}
}
