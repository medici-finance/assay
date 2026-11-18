package deskkit

import (
	"regexp"
	"strings"
)

// Role secrets (issue #6065). A project secret reaches a dispatched
// worker or verifier window as a PATH, never a value: the launcher (cellctl) mounts the
// operator's owner-only file read-only and exports one variable per bound service naming where
// it is. These helpers are the one spelling of those variable names, shared by the launcher
// that sets them and the dispatcher that refuses untrusted work beside them.
//
// The names contain SECRET, so the before_run hook's credential scrub (isSecretEnvName)
// already removes them from every hook environment.

// RoleSecretEnvPrefix begins every role-secret path variable.
const RoleSecretEnvPrefix = "ASSAY_ROLE_SECRET_"

// roleSecretServiceRe is the `<name>` grammar of a `service:<name>` Verify-row Env cell
// (docs/brief-rules.md rule 45): lower-case letters, digits, `.`, `_` and `-`.
var roleSecretServiceRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ValidRoleSecretService reports whether name is a well-formed service name.
func ValidRoleSecretService(name string) bool { return roleSecretServiceRe.MatchString(name) }

// RoleSecretEnvName is the variable that names the path of one service's secret file inside a
// holding window: ASSAY_ROLE_SECRET_<NAME>_FILE, with `.` and `-` spelled `_`. Two services
// that spell the same variable are a configuration error the launcher refuses.
func RoleSecretEnvName(service string) string {
	u := strings.ToUpper(service)
	u = strings.NewReplacer(".", "_", "-", "_").Replace(u)
	return RoleSecretEnvPrefix + u + "_FILE"
}

// IsRoleSecretEnvName reports whether an environment variable NAME is a role-secret path
// variable. The match is case-insensitive, so a lower-case spelling is still recognised.
func IsRoleSecretEnvName(name string) bool {
	return strings.HasPrefix(strings.ToUpper(name), RoleSecretEnvPrefix)
}

// HoldsRoleSecret reports whether an environment (os.Environ form) carries any role-secret
// path variable, set or empty. A window that does is a holding window.
func HoldsRoleSecret(environ []string) bool {
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if IsRoleSecretEnvName(name) {
			return true
		}
	}
	return false
}

// StripRoleSecretEnv returns environ without any role-secret path variable. It never returns
// nil, so a caller handing the result to os/exec replaces the child's environment rather than
// letting it inherit.
func StripRoleSecretEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if IsRoleSecretEnvName(name) {
			continue
		}
		out = append(out, kv)
	}
	return out
}
