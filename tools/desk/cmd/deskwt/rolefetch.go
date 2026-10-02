package main

import (
	"net/url"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fetchRoleBase authenticates the FIRST fetch, before the new worktree exists.
// Wiring its credential after worktree creation is too late: the source tree may
// hold an expired token or another role's helper. Refresh the requested role's
// credential and override helpers for this command only; never edit the source.
func fetchRoleBase(dir, role, repo, username string) error {
	origins, err := originURLs(dir, false)
	if err != nil {
		return err
	}
	if len(origins) != 1 {
		return deskkit.Refused("refused: role-init requires exactly one origin fetch URL")
	}
	origin := origins[0]
	_, networked, err := transportHost(origin)
	if err != nil {
		return deskkit.Refused("refused: cannot resolve role-init fetch transport: " + err.Error())
	}
	if !networked {
		_, err = runGit(dir, "fetch", "--no-tags", "origin", "main")
		return err
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return deskkit.Refused("refused: role-init network fetch requires an HTTPS origin without embedded credentials; configure the role's App transport first")
	}
	path, err := roleCredentialPath(role, repo, origin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return deskkit.Unverifiable("credential resolver returned no token path for role-init fetch", nil)
	}
	if strings.ContainsAny(path, "'\r\n") {
		return deskkit.Refused("refused: role-init token path cannot be quoted into a credential helper")
	}
	args := []string{
		"-c", "credential.helper=",
		"-c", "credential.https://" + u.Host + ".helper=" + deskkit.AppTokenHelper(username, path),
		"-c", "credential.username=" + username,
		"-c", "http.followRedirects=false",
		"fetch", "--no-tags", "--no-recurse-submodules", "origin", "main",
	}
	r := deskkit.Run(deskkit.ToolCall{
		Name: "git", Args: args, Dir: dir, Start: execCommand,
		Env: append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS="),
	})
	if r.Failed() {
		return r.Fail(deskkit.ExitUnverifiable, "role-init fetch as %s", role)
	}
	return nil
}
