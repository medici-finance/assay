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
// LOOP-ROLE SEPARATION (#6069). The fork ran desktoken's own check that a window takes only
// the credential of the role its loop acts under, so this read applies the same check
// (RequireLoopRole) before any custody is read: a refusal is returned as is. The two verbs
// whose posting identity is fixed by design (deskreply posts as worker, deskevidence as
// verifier, from whichever window calls them) are the exception the fork made through
// --cross-role: for them a refusal is admitted, and the override is written to the audit
// log as a desktoken row carrying the same text the CLI records, on every use. No other
// verb gets an override, exactly as before.
//
// TestWriteVerbsReadCredential (writeverbguard_test.go) enumerates the nine verbs and fails
// if any of them names the desktoken binary again.
func WriteVerbRoleCredential(verb, role, repoSlug string) (RoleCredential, error) {
	slug := strings.TrimSpace(repoSlug)
	owner, name, _ := strings.Cut(slug, "/")
	override, lerr := writeVerbLoopRole(verb, role)
	if lerr != nil {
		return RoleCredential{}, lerr
	}
	cred, err := ResolveRoleCredential(role, ForgeRepo{Owner: owner, Name: name}, "")
	if override != "" {
		logCrossRoleOverride(role, slug, override, err)
	}
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

// logCrossRoleOverride writes the admitted override as a row under the minter's own audit
// tool name, so the ledger reads one minter whichever caller ran it. It is its own function
// so the one place this file names that tool as DATA is the one place the desktoken exec
// guard's allow-list names (TestDesktokenExecConfined): a future exec here still trips it.
func logCrossRoleOverride(role, slug, override string, credErr error) {
	_ = Log(Entry{Tool: "desktoken", Verb: "mint", ArgsDigest: ArgsDigest([]string{role, "--repo", slug}),
		Result: CredentialResult(credErr), Detail: override})
}

// fixedIdentityVerbs are the write verbs that post as one App role whatever window runs them,
// and so may take an audited cross-role override (#6069) where the other verbs may not.
var fixedIdentityVerbs = map[string]string{"deskreply": "worker", "deskevidence": "verifier"}

// writeVerbLoopRole applies RequireLoopRole to verb's credential read. It returns the audit
// text of an admitted cross-role override ("" when none was needed) or the refusal.
func writeVerbLoopRole(verb, role string) (override string, err error) {
	loop, loopRole, err := RequireLoopRole(verb, role)
	if err == nil {
		return "", nil
	}
	if ExitCodeOf(err) != ExitRefused || fixedIdentityVerbs[verb] != role {
		return "", err
	}
	return fmt.Sprintf("cross-role override: loop=%s acts-as=%s role=%s reason=%q",
		loop, loopRole, role, crossRoleText(verb, role, loop, loopRole)), nil
}
