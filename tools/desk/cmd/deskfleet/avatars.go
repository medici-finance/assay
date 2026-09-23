package main

// avatars.go — the script's avatar step (PUT /user/avatar, as the ROLE's own PAT), ported.
//
// A group Owner cannot set a service account's avatar (PUT /users/:id is admin-only), so each
// account sets its OWN with its own token. The role token is read back from its
// gitlab-<role>.token file through the same read-side custody check every desk verb applies,
// and is carried as a request header only.
//
// One deliberate divergence from the script: there is NO default remote fetch. The script,
// given no --avatars-dir, downloads public role icons from a fixed web host; this verb adds no
// network default, so without --avatars-dir the step is SKIPPED — named in a NOTICE and in the
// closing summary, never silently.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// avatarsSkippedNotice tells the operator the avatar step did not run and exactly how to run
// it later.
func avatarsSkippedNotice(o provisionOpts) string {
	return fmt.Sprintf("--avatars-dir was not given, so the avatar step (PUT /user/avatar) is SKIPPED and the "+
		"service accounts keep the forge's default avatar. To set them later, run: deskfleet provision "+
		"--avatars-only --avatars-dir <dir holding <role>.png> --prefix %s --out-dir %s (it signs in as each "+
		"role from its gitlab-<role>.token there)", o.prefix, o.outDir)
}

// uploadAvatar sets <avatarsDir>/<role>.png as user's avatar, signed in as that role. A missing
// icon or token file is a NOTICE (skipped, not a failure), as in the script; a forge refusal
// is a recorded failure that does not stop the run.
func (p *provisioner) uploadAvatar(role, user string) {
	tokPath := filepath.Join(p.o.outDir, tokenFileName(role))
	if _, err := os.Stat(tokPath); err != nil {
		p.outf("NOTICE: avatar for %s skipped — no token file at %s (skipped, not a failure)", user, tokPath)
		return
	}
	img := filepath.Join(p.o.avatarsDir, role+".png")
	data, err := os.ReadFile(img)
	if err != nil {
		p.outf("NOTICE: no avatar for %s — %s could not be read (%v) (skipped, not a failure)", role, img, err)
		return
	}
	tok, err := readCredentialFile("token file", tokPath)
	if err != nil {
		p.outf("NOTICE: avatar for %s skipped — its token file did not pass the read-side custody check (%v); "+
			"fix the file's access, then re-run with --avatars-only", user, err)
		return
	}
	c := newGitLabClient(p.base, p.e.http, tok)
	resp, err := c.doFile("PUT", "/user/avatar", "avatar", role+".png", data)
	if err != nil || resp.Status != 200 {
		p.fail("avatar upload for %s failed (%s) — re-run with --avatars-only once the cause is fixed",
			user, respOrErr(resp, err))
		return
	}
	p.outf("avatar: %s <- %s (HTTP 200)", user, img)
}

// runAvatarsOnly is `provision --avatars-only`: for each role whose token file exists under
// --out-dir, set that account's avatar as the account itself. It creates no account, mints no
// token and touches no project setting, so it needs no owner credential.
func (p *provisioner) runAvatarsOnly() int {
	if p.o.dryRun {
		for _, r := range fleetRoles {
			p.outf("[dry-run] would upload avatar for %s <- %s (PUT /user/avatar, as that role's own PAT from %s)",
				serviceAccountUsername(p.o.prefix, r.Role), filepath.Join(p.o.avatarsDir, r.Role+".png"),
				filepath.Join(p.o.outDir, tokenFileName(r.Role)))
		}
		return exitOK
	}
	base, err := gitlabAPIBase(p.e)
	if err != nil {
		p.errf("refused: %v", err)
		return exitRefused
	}
	p.base = base
	p.outf("target: %s (GITLAB_API_BASE) — avatars-only: signing in as each role from the token files in %s; "+
		"no account, token or project setting is touched", base, p.o.outDir)
	for _, r := range fleetRoles {
		p.uploadAvatar(r.Role, serviceAccountUsername(p.o.prefix, r.Role))
	}
	if len(p.failures) > 0 {
		p.outf("%s", ruleLine)
		p.outf("FAILED STEPS — this run did NOT complete cleanly:")
		for _, f := range p.failures {
			p.outf("   - %s", f)
		}
		return exitFailed
	}
	return exitOK
}

func idOrUnknown(id int64) string {
	if id == 0 {
		return "unknown"
	}
	return strconv.FormatInt(id, 10)
}
