package deskkit

import (
	"fmt"
	"strings"
)

// WriteVerbRoleCredential is the role-credential read every desk WRITE verb takes (#5457):
// deskclose, deskdisposition, deskevidence, deskfile, desklabel, deskpathguard, deskpr,
// deskreply and deskrestamp. It resolves the forge that serves repoSlug ("owner/name") and
// reads the role's credential through ResolveRoleCredential, IN PROCESS.
//
// WHY NOT FORK `desktoken`. Those verbs used to run `desktoken <role> --repo <slug>` without
// `--forge`. On a repository the roster binds to GitLab, desktoken resolves that shape to its
// rotation path, so an ordinary close, label or reply ROTATED the role's GitLab PAT as a side
// effect — a destructive custody change nobody asked for, taken by whichever verb ran first.
// ResolveRoleCredential cannot reach that path: its GitLab arm only reads the already
// provisioned custody file (GitLabRoleToken) and refuses (exit 5) when it is missing or loose;
// its GitHub arm runs the same credential provider the CLI runs, in process, with the same
// on-disk reuse window, so GitHub behaviour is unchanged and nothing new is written or
// exported. Rotation stays an explicit act:
// `desktoken <role> --forge gitlab`.
//
// The token is returned in the RoleCredential only; it is never placed in the environment,
// argv, a file or a log line. The error carries the verb, role and repo — never the value.
// TestWriteVerbsReadCredential (writeverbguard_test.go) enumerates the nine verbs and fails
// if any of them names the desktoken binary again.
func WriteVerbRoleCredential(verb, role, repoSlug string) (RoleCredential, error) {
	slug := strings.TrimSpace(repoSlug)
	owner, name, _ := strings.Cut(slug, "/")
	cred, err := ResolveRoleCredential(role, ForgeRepo{Owner: owner, Name: name}, "")
	if err != nil {
		return RoleCredential{Resolution: cred.Resolution}, &DeskError{
			Code: ExitCodeOf(err),
			Msg:  fmt.Sprintf("%s: read the %s role credential for %s", verb, role, slug),
			Err:  err,
		}
	}
	if strings.TrimSpace(cred.Token) == "" {
		return RoleCredential{Resolution: cred.Resolution}, Unverifiable(
			fmt.Sprintf("%s: the %s role credential for %s is empty", verb, role, slug), nil)
	}
	return cred, nil
}
