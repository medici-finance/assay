package main

// provision.go — the provisioning run itself, step for step the flow of the bash script it
// replaces: group resolve and tier detection, the role loop (service account, membership,
// first PAT, own avatar), the --project settings block (protected main, approvals, protected
// tags, merge checks, labels), the --tier ultimate refinements, and the summary.
//
// IDEMPOTENT. Every step reads before it writes: an existing account, membership, protected
// rule, tag rule, label, custom role or status check is a NAMED no-op, never a duplicate.
//
// THE FAILURE LEDGER. A settings step that fails never aborts the steps after it (issue #346:
// one 400 used to skip every later setting). Each failure is recorded, printed under the
// summary's FAILED STEPS, and turns the exit status to 1. Only a failure the run cannot
// continue past — the group, an account or the project cannot be resolved or created — ends
// the run at once with exit 1, as the script's `exit 1` did.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// iconBase is where the default role icons are fetched from (a variable so the scenario
// tests can point it at a local stub).
var iconBase = defaultIconBase

// now is the clock the PAT expiry date is computed from.
var now = time.Now

// errAbort ends a run with exit 1 after its error has been printed.
type errAbort struct{}

func (errAbort) Error() string { return "aborted" }

type provisioner struct {
	o        options
	out, err io.Writer
	api      *apiClient // as the group owner
	avatars  bool
	outDir   string

	failures      []string
	groupID       string
	groupTier     string // premium | free | unknown
	boardWriterID string
	projectID     string
	avatarTmp     string // lazily created, removed at the end of the run
}

// provision runs one validated invocation and returns its exit code.
func provision(o options, stdout, stderr io.Writer) int {
	p := &provisioner{
		o: o, out: stdout, err: stderr,
		api:       newAPIClient(o.gitlabURL, o.token),
		avatars:   !o.noAvatars,
		outDir:    o.outDir,
		groupTier: "unknown",
	}
	defer p.cleanup()
	if err := p.run(); err != nil {
		return 1
	}
	return p.summary()
}

func (p *provisioner) cleanup() {
	if p.avatarTmp != "" {
		_ = os.RemoveAll(p.avatarTmp)
	}
}

func (p *provisioner) say(format string, a ...any)  { fmt.Fprintf(p.out, format+"\n", a...) }
func (p *provisioner) warn(format string, a ...any) { fmt.Fprintf(p.err, format+"\n", a...) }
func (p *provisioner) record(format string, a ...any) {
	p.failures = append(p.failures, fmt.Sprintf(format, a...))
}

// abort prints a fatal error and the server's raw answer (as the script's `cat` did) to
// stderr, and ends the run.
func (p *provisioner) abort(body []byte, format string, a ...any) error {
	p.warn(format, a...)
	_, _ = p.err.Write(body)
	return errAbort{}
}

func (p *provisioner) run() error {
	if !p.o.dryRun {
		if p.outDir == "" {
			d, err := os.MkdirTemp("", "fleet-gitlab-tokens.")
			if err != nil {
				p.warn("error: could not create a token directory: %v", err)
				return errAbort{}
			}
			p.outDir = d
		}
		if err := os.MkdirAll(p.outDir, 0o700); err != nil {
			p.warn("error: could not create --out-dir: %v", err)
			return errAbort{}
		}
	}
	project := p.o.project
	if project == "" {
		project = "<none>"
	}
	p.say("Assay fleet provisioning — group=%s prefix=%s project=%s dry-run=%s avatars-only=%s",
		p.o.group, p.o.prefix, project, bit(p.o.dryRun), bit(p.o.avatarsOnly))

	if p.o.avatarsOnly {
		p.avatarsOnly()
		return nil
	}
	if !p.o.dryRun {
		if err := p.resolveGroup(); err != nil {
			return err
		}
	}
	if !p.avatars {
		p.say("NOTICE: --no-avatars — service accounts keep the default Gravatar identicon; re-run with --avatars-only to set them later")
	}
	for _, r := range fleetRoles {
		if err := p.provisionRole(r); err != nil {
			return err
		}
	}
	if err := p.projectBlock(); err != nil {
		return err
	}
	if p.o.tier == "ultimate" {
		p.configureUltimate()
	}
	return nil
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// --- avatars ---------------------------------------------------------------------------
//
// The owner cannot set a service account's avatar (PUT /users/:id is admin-only) and the
// service-account endpoint has no avatar field, so each account sets its OWN with its own
// PAT — which is why it happens at mint time, the one moment the run holds that credential.

func (p *provisioner) avatarsOnly() {
	if p.o.dryRun {
		for _, r := range fleetRoles {
			u := fleetUsername(p.o.prefix, r.Role)
			p.say("[dry-run] would upload avatar for %s via PUT /user/avatar, as that role's own PAT from %s",
				u, filepath.Join(p.outDir, u+".token"))
		}
		return
	}
	p.say("avatars-only: re-uploading avatars from the token files under %s — no accounts, tokens or project settings are touched", p.outDir)
	for _, r := range fleetRoles {
		u := fleetUsername(p.o.prefix, r.Role)
		p.uploadAvatar(r.Role, u, filepath.Join(p.outDir, u+".token"))
	}
}

// avatarSource returns the local PNG for role, or "" after printing a NOTICE when there is no
// icon to upload. A missing icon is never a failure.
func (p *provisioner) avatarSource(role string) string {
	if p.o.avatarsDir != "" {
		file := filepath.Join(p.o.avatarsDir, role+".png")
		if fi, err := os.Stat(file); err != nil || !fi.Mode().IsRegular() {
			p.say("NOTICE: no avatar for %s — %s does not exist (skipped, not a failure)", role, file)
			return ""
		}
		return file
	}
	src := iconBase + "/app-icon-" + role + ".png"
	notice := func() string {
		p.say("NOTICE: could not fetch the default icon %s for %s (skipped, not a failure)", src, role)
		return ""
	}
	if p.avatarTmp == "" {
		d, err := os.MkdirTemp("", "fleet-gitlab-avatars.")
		if err != nil {
			return notice()
		}
		p.avatarTmp = d
	}
	file := filepath.Join(p.avatarTmp, role+".png")
	if fi, err := os.Stat(file); err == nil && fi.Size() > 0 {
		return file
	}
	data := fetchIcon(src)
	if data == nil || os.WriteFile(file, data, 0o600) != nil {
		_ = os.Remove(file)
		return notice()
	}
	return file
}

// uploadAvatar sets username's avatar as that account itself, with the token in tokenFile.
func (p *provisioner) uploadAvatar(role, username, tokenFile string) {
	tok, err := os.ReadFile(tokenFile)
	if err != nil || len(tok) == 0 {
		p.say("NOTICE: avatar for %s skipped — no token file at %s (skipped, not a failure)", username, tokenFile)
		return
	}
	file := p.avatarSource(role)
	if file == "" {
		return
	}
	data, err := os.ReadFile(file)
	status := transportFailed
	if err == nil {
		self := newAPIClient(p.o.gitlabURL, strings.TrimRight(string(tok), "\n"))
		status, _ = self.putFile("/user/avatar", "avatar", filepath.Base(file), data)
	}
	p.say("avatar: %s <- %s (HTTP %s)", username, file, status)
	if status != "200" {
		p.record("avatar upload for %s returned HTTP %s (expected 200) — re-run with --avatars-only once the cause is fixed", username, status)
	}
}

// --- group and roles -------------------------------------------------------------------

// classifyPlan maps a group's plan to the protected-branch vocabulary it accepts: premium
// (the allowed_to_* arrays), free (scalar fields only), or unknown (probe).
func classifyPlan(plan string) string {
	switch plan {
	case "premium", "ultimate", "gold", "silver":
		return "premium"
	case "free", "default", "bronze", "":
		return "free"
	}
	return "unknown"
}

func (p *provisioner) resolveGroup() error {
	st, body := p.api.call(http.MethodGet, "/groups/"+uriEncode(p.o.group), nil)
	if st != "200" {
		return p.abort(body, "error: could not resolve group '%s' (HTTP %s)", p.o.group, st)
	}
	v, ok := decodeJSON(body)
	if !ok {
		return p.abort(body, "error: the group '%s' answer is not JSON", p.o.group)
	}
	p.groupID = render(path(v, "id"))
	if plan := alt(path(v, "plan"), ""); plan != "" {
		p.groupTier = classifyPlan(plan)
		p.say("tier: group plan='%s' -> %s", plan, p.groupTier)
	} else {
		p.say("tier: group response carries no 'plan' field — will probe (Premium form first, free-tier form on a 400 naming allowed_to_)")
	}
	return nil
}

func (p *provisioner) provisionRole(r fleetRole) error {
	username := fleetUsername(p.o.prefix, r.Role)
	if p.o.dryRun {
		p.say("[dry-run] would create service account %s (role=%s, access=%s, scopes=%s)", username, r.Role, r.AccessName, r.Scopes)
		p.say("[dry-run]   would ensure group membership at access_level=%s", r.AccessLevel)
		p.say("[dry-run]   would mint a PAT (scopes=%s, expires in %sd) if the account is new", r.Scopes, p.o.patExpiryDays)
		switch {
		case !p.avatars:
			p.say("[dry-run]   would skip the avatar upload for %s (--no-avatars)", username)
		case p.o.avatarsDir != "":
			p.say("[dry-run]   would upload avatar for %s <- %s (PUT /user/avatar, as that account's own PAT)",
				username, filepath.Join(p.o.avatarsDir, r.Role+".png"))
		default:
			p.say("[dry-run]   would upload avatar for %s <- %s/app-icon-%s.png (PUT /user/avatar, as that account's own PAT)",
				username, iconBase, r.Role)
		}
		return nil
	}

	// Service account: checked first, so a re-run reads as a named no-op, not a raw 409.
	saPath := "/groups/" + p.groupID + "/service_accounts"
	st, body := p.api.call(http.MethodGet, saPath, nil)
	if st != "200" {
		return p.abort(body, "error: listing the service accounts of group '%s' failed (HTTP %s)", p.o.group, st)
	}
	if v, ok := decodeJSON(body); !ok {
		return p.abort(body, "error: the service-account listing of group '%s' is not a JSON list", p.o.group)
	} else if _, ok := v.([]any); !ok {
		return p.abort(body, "error: the service-account listing of group '%s' is not a JSON list", p.o.group)
	}
	userID := firstMatch(body, "username", username, "id")
	createdNow := false
	if userID != "" {
		p.say("no-op: service account %s already exists (id=%s)", username, userID)
	} else {
		st, body = p.api.call(http.MethodPost, saPath,
			object("name", "Assay "+r.Role+" (fleet bot)", "username", username))
		if st != "201" {
			return p.abort(body, "error: creating service account %s failed (HTTP %s)", username, st)
		}
		v, ok := decodeJSON(body)
		if !ok {
			return p.abort(body, "error: the answer creating service account %s is not JSON", username)
		}
		userID = render(path(v, "id"))
		createdNow = true
		p.say("created: service account %s (id=%s)", username, userID)
	}

	if r.Role == "board-writer" {
		p.boardWriterID = userID
		if err := os.WriteFile(filepath.Join(p.outDir, ".board-writer-id"), []byte(userID+"\n"), 0o600); err != nil {
			p.warn("error: could not record the board-writer id: %v", err)
			return errAbort{}
		}
	}

	// Group membership: an existing one at another level is reported, never changed.
	st, body = p.api.call(http.MethodGet, "/groups/"+p.groupID+"/members/"+userID, nil)
	if st == "200" {
		v, _ := decodeJSON(body)
		if have := render(path(v, "access_level")); have != r.AccessLevel {
			p.say("NOTICE: %s is already a group member at access_level=%s, expected %s — not changed automatically, check by hand",
				username, have, r.AccessLevel)
		} else {
			p.say("no-op: %s already a group member at access_level=%s", username, r.AccessLevel)
		}
	} else {
		st, body = p.api.call(http.MethodPost, "/groups/"+p.groupID+"/members",
			object("user_id", raw(userID), "access_level", raw(r.AccessLevel)))
		if st != "201" {
			return p.abort(body, "error: adding %s to group failed (HTTP %s)", username, st)
		}
		p.say("added: %s to group at access_level=%s", username, r.AccessLevel)
	}

	// PAT: minted only for an account created THIS run. A fresh credential for an existing
	// account is a rotation (tools/renew-fleet-gitlab-tokens.sh), never a re-provision.
	tokenFile := filepath.Join(p.outDir, username+".token")
	if !createdNow {
		p.say("NOTICE: PAT minting skipped for %s (account pre-existing) — renew every role's PAT in one run with tools/renew-fleet-gitlab-tokens.sh, per spec.md §5", username)
		if p.avatars {
			if fi, err := os.Stat(tokenFile); err == nil && fi.Size() > 0 {
				p.uploadAvatar(r.Role, username, tokenFile)
			} else {
				p.say("NOTICE: avatar for %s skipped — pre-existing account and no token file under %s; re-run with --avatars-only --out-dir <dir holding its token> to set it", username, p.outDir)
			}
		}
		return nil
	}
	days, _ := strconv.Atoi(p.o.patExpiryDays)
	expires := now().AddDate(0, 0, days).Format("2006-01-02")
	st, body = p.api.call(http.MethodPost, saPath+"/"+userID+"/personal_access_tokens",
		object("name", fleetPATName(r.Role), "scopes", strings.Split(r.Scopes, ","), "expires_at", expires))
	if st != "201" {
		return p.abort(body, "error: minting PAT for %s failed (HTTP %s)", username, st)
	}
	v, ok := decodeJSON(body)
	if !ok {
		// Never echo this body: it is the answer that carries the new credential.
		p.warn("error: the answer minting a PAT for %s is not JSON", username)
		return errAbort{}
	}
	if err := writeSecretFile(tokenFile, render(path(v, "token"))); err != nil {
		p.warn("error: could not write the token file for %s: %v", username, err)
		return errAbort{}
	}
	p.say("minted: PAT for %s -> %s (0600, expires %s) — path printed, value never echoed", username, tokenFile, expires)
	if p.avatars {
		p.uploadAvatar(r.Role, username, tokenFile)
	}
	return nil
}

// writeSecretFile writes value to path, readable by the owner only, whatever the umask.
func writeSecretFile(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(value)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	if cerr != nil {
		return cerr
	}
	return os.Chmod(path, 0o600)
}

// --- the --project block ---------------------------------------------------------------

func (p *provisioner) projectBlock() error {
	if p.o.project == "" {
		p.say("NOTICE: --project not given — protected-branch and approval settings skipped (could-not-check, not a pass); pass --project to configure them")
		return nil
	}
	if p.o.dryRun {
		p.say("[dry-run] would protect branch 'main' on project %s: allowed_to_push=[board-writer], allowed_to_merge=[Maintainer role]", p.o.project)
		p.say("[dry-run] would set approvals: merge_requests_author_approval=false, merge_requests_disable_committers_approval=true")
		p.say("[dry-run] would protect tags '%s' with create_access_level=%s (Maintainers)", protectedTagGlob, protectedTagCreateLevel)
		p.say("[dry-run] would set only_allow_merge_if_pipeline_succeeds=true and only_allow_merge_if_all_discussions_are_resolved=true")
		for _, l := range fleetLabels {
			p.say("[dry-run] would create project label '%s' (POST /projects/:id/labels, idempotent)", l.Name)
		}
		return nil
	}
	// A missing board-writer id only rules out the Premium push allowlist, which the
	// free-tier form does not use.
	if p.boardWriterID == "" {
		p.warn("NOTICE: board-writer service account id unknown (check %s) — the Premium push allowlist cannot be named; protecting main with the free-tier form instead",
			filepath.Join(p.outDir, ".board-writer-id"))
		p.groupTier = "free"
	}
	st, body := p.api.call(http.MethodGet, "/projects/"+uriEncode(p.o.project), nil)
	if st != "200" {
		return p.abort(body, "error: could not resolve project '%s' (HTTP %s)", p.o.project, st)
	}
	v, ok := decodeJSON(body)
	if !ok {
		return p.abort(body, "error: the project '%s' answer is not JSON", p.o.project)
	}
	p.projectID = render(path(v, "id"))

	// Every step runs even when the one before it failed.
	p.configureProtectedMain()
	p.configureApprovals()
	p.configureProtectedTags()
	p.configureMergeSettings()
	p.configureLabels()
	return nil
}

// trimBody is a response body as the script's $(cat file) read it: trailing newlines removed.
func trimBody(b []byte) string { return strings.TrimRight(string(b), "\n") }

// protectedRule is protected `main` as read before any change.
type protectedRule struct {
	exists                   bool
	push, merge, force, user string
}

func (p *provisioner) readProtectedMain() protectedRule {
	prev := protectedRule{push: "0", merge: mergeAccessLevel, force: "false"}
	st, body := p.api.call(http.MethodGet, "/projects/"+p.projectID+"/protected_branches/main", nil)
	if st != "200" {
		return prev
	}
	v, _ := decodeJSON(body)
	prev.exists = true
	prev.push = alt(path(v, "push_access_levels", 0, "access_level"), "0")
	prev.merge = alt(path(v, "merge_access_levels", 0, "access_level"), "40")
	prev.force = hasString(v, "allow_force_push", "false")
	prev.user = alt(path(v, "push_access_levels", 0, "user_id"), "")
	return prev
}

// protectBody is the POST payload: `premium` names the board-writer in allowed_to_push;
// `free` uses only the three scalar fields Free accepts.
func (p *provisioner) protectBody(form string) []byte {
	if form == "premium" {
		return object("name", "main",
			"allowed_to_push", array(object("user_id", raw(p.boardWriterID))),
			"allowed_to_merge", array(object("access_level", raw(mergeAccessLevel))),
			"allowed_to_unprotect", array(object("access_level", raw("50"))),
			"allow_force_push", false)
	}
	return object("name", "main", "push_access_level", raw("0"), "merge_access_level", raw(mergeAccessLevel),
		"allow_force_push", false)
}

// restoreProtectedMain re-applies exactly what was read, so a refused POST cannot leave
// `main` open. Scalar form unless the previous rule named a user.
func (p *provisioner) restoreProtectedMain(prev protectedRule) bool {
	var body []byte
	if prev.user != "" {
		body = object("name", "main",
			"allowed_to_push", array(object("user_id", raw(prev.user))),
			"allowed_to_merge", array(object("access_level", raw(prev.merge))),
			"allow_force_push", raw(prev.force))
	} else {
		body = object("name", "main", "push_access_level", raw(prev.push), "merge_access_level", raw(prev.merge),
			"allow_force_push", raw(prev.force))
	}
	st, _ := p.api.call(http.MethodPost, "/projects/"+p.projectID+"/protected_branches", body)
	return st == "201"
}

// readbackProtectedMain prints the three decided fields as the server now holds them, so a
// wrong rule is visible at provisioning time.
func (p *provisioner) readbackProtectedMain() {
	st, body := p.api.call(http.MethodGet, "/projects/"+p.projectID+"/protected_branches/main", nil)
	if st != "200" {
		p.say("NOTICE: could not read protected 'main' back (HTTP %s) — the three decided fields are could-not-check, not a pass", st)
		p.record("protected-branch read-back on 'main' failed — verify push/merge/force-push by hand")
		return
	}
	v, _ := decodeJSON(body)
	push := alt(path(v, "push_access_levels", 0, "access_level"), "none")
	merge := alt(path(v, "merge_access_levels", 0, "access_level"), "none")
	// A boolean read-back tests for the KEY: `false` is a value, not an absence.
	force := hasString(v, "allow_force_push", "unknown")
	user := alt(path(v, "push_access_levels", 0, "user_id"), "")
	userNote := ""
	if user != "" {
		userNote = " (user_id=" + user + ")"
	}
	p.say("read-back: main push_access_level=%s%s merge_access_level=%s allow_force_push=%s", push, userNote, merge, force)
	if merge != mergeAccessLevel {
		p.say("NOTICE: merge_access_level is %s, intended %s (Maintainers) — at 30 every Developer service account can merge its own MR", merge, mergeAccessLevel)
		p.record("protected 'main' merge_access_level is %s, intended %s (Maintainers)", merge, mergeAccessLevel)
	}
	if force != "false" {
		p.say("NOTICE: allow_force_push reads %s, intended false", force)
		p.record("protected 'main' allow_force_push is %s, intended false", force)
	}
}

// configureProtectedMain protects `main`, never leaving it unprotected across a failure.
// Preference order: no-op (already as intended) > PATCH (the field PATCH accepts) >
// DELETE+POST with an immediate re-POST of the rule that was read.
func (p *provisioner) configureProtectedMain() {
	prev := p.readProtectedMain()
	wantUser := ""
	if p.groupTier == "premium" {
		wantUser = p.boardWriterID
	}
	pushAsIntended := (wantUser != "" && prev.user == wantUser) ||
		(wantUser == "" && prev.user == "" && prev.push == "0")

	if prev.exists && prev.merge == mergeAccessLevel && prev.force == "false" && pushAsIntended {
		p.say("no-op: main is already protected as intended — nothing deleted, nothing re-created")
		p.readbackProtectedMain()
		return
	}
	if prev.exists && prev.force != "false" && prev.merge == mergeAccessLevel && pushAsIntended {
		st, _ := p.api.call(http.MethodPatch, "/projects/"+p.projectID+"/protected_branches/main",
			[]byte(`{"allow_force_push": false}`))
		if st == "200" {
			p.say("patched: main allow_force_push=false (PATCH — the rule was never removed)")
			p.readbackProtectedMain()
			return
		}
		p.say("NOTICE: PATCH of allow_force_push returned HTTP %s — falling back to a guarded re-create", st)
	}

	var forms []string
	switch p.groupTier {
	case "premium":
		forms = []string{"premium"}
	case "free":
		forms = []string{"free"}
	default:
		forms = []string{"premium", "free"}
	}
	deleted, applied := false, ""
	for _, form := range forms {
		if prev.exists && !deleted {
			p.api.call(http.MethodDelete, "/projects/"+p.projectID+"/protected_branches/main", nil)
			deleted = true
			p.say("unprotected: main (re-applying now; the previous rule goes back immediately if that fails)")
		}
		st, body := p.api.call(http.MethodPost, "/projects/"+p.projectID+"/protected_branches", p.protectBody(form))
		if st == "201" {
			applied = form
			break
		}
		msg := trimBody(body)
		p.say("NOTICE: protect '%s' form refused (HTTP %s): %s", form, st, msg)
		if form == "premium" && st == "400" && strings.Contains(msg, "allowed_to_") {
			p.say("tier: the Premium form was refused for its allowed_to_ arrays — retrying with the free-tier form")
			continue
		}
		break
	}

	if applied == "" {
		urgent := "URGENT: 'main' is UNPROTECTED — protect it by hand: POST /projects/" + p.projectID +
			"/protected_branches with push_access_level=0&merge_access_level=" + mergeAccessLevel + "&allow_force_push=false"
		switch {
		case prev.exists && p.restoreProtectedMain(prev):
			userNote := ""
			if prev.user != "" {
				userNote = " user_id=" + prev.user
			}
			p.warn("error: protecting main failed — protection re-applied to its previous state (push_access_level=%s%s merge_access_level=%s allow_force_push=%s)",
				prev.push, userNote, prev.merge, prev.force)
			p.record("protected-branch step failed; 'main' was restored to its PREVIOUS rule (merge_access_level=%s) — the intended rule is NOT in place", prev.merge)
		case prev.exists:
			p.warn("error: protecting main failed AND the previous rule could not be re-applied — 'main' is UNPROTECTED, fix it by hand now")
			p.record("%s", urgent)
		default:
			p.warn("error: protecting main failed and there was no previous rule to restore — 'main' is UNPROTECTED, fix it by hand now")
			p.record("%s", urgent)
		}
		return
	}
	if applied == "premium" {
		p.say("protected: main (allowed_to_push=board-writer only, allowed_to_merge=Maintainer role (%s), allow_force_push=false)", mergeAccessLevel)
	} else {
		p.say("NOTICE: free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)")
		p.say("protected: main (push_access_level=0 'No one', merge_access_level=%s Maintainers, allow_force_push=false) — every write, board regeneration included, goes through an MR", mergeAccessLevel)
	}
	p.readbackProtectedMain()
}

// configureApprovals writes the two approval settings, then reads them back: on Free the
// write is accepted and the values stay at their defaults, so a 201 is not proof.
func (p *provisioner) configureApprovals() {
	apPath := "/projects/" + p.projectID + "/approvals"
	st, body := p.api.call(http.MethodPost, apPath,
		object("merge_requests_author_approval", false, "merge_requests_disable_committers_approval", true))
	if st != "201" && st != "200" {
		p.warn("error: setting approval rules failed (HTTP %s)", st)
		_, _ = p.err.Write(body)
		p.record("approval settings write failed (HTTP %s)", st)
		return
	}
	st, body = p.api.call(http.MethodGet, apPath, nil)
	if st != "200" {
		p.say("NOTICE: approval settings could not be read back (HTTP %s) — could-not-check, not a pass", st)
		p.record("approval settings read-back failed — verify by hand")
		return
	}
	v, _ := decodeJSON(body)
	author := hasString(v, "merge_requests_author_approval", "unknown")
	committers := hasString(v, "merge_requests_disable_committers_approval", "unknown")
	available := hasString(v, "merge_request_approvers_available", "unknown")
	if author == "false" && committers == "true" && available != "false" {
		p.say("configured: approvals (prevent-author, prevent-committers) — read back and confirmed")
		return
	}
	p.say("read-back: approvals merge_requests_author_approval=%s merge_requests_disable_committers_approval=%s merge_request_approvers_available=%s",
		author, committers, available)
	p.say("NOTICE: approval settings ignored at this tier (failed-at-tier, remediation: Premium)")
	if available == "false" {
		p.say("NOTICE: approval RULES are unavailable here (merge_request_approvers_available=false) — the stored fields can read correct while an MR's own author still approves it; do not count approvals as a server-enforced gate on this tier")
	}
}

// configureProtectedTags protects every release tag with the SCALAR Free-tier field (never
// the Premium allowed_to_create array); an existing rule for the glob is a named no-op.
func (p *provisioner) configureProtectedTags() {
	tagPath := "/projects/" + p.projectID + "/protected_tags"
	st, body := p.api.call(http.MethodGet, tagPath, nil)
	if st == "200" {
		if firstMatch(body, "name", protectedTagGlob, "name") != "" {
			p.say("no-op: tags matching '%s' are already protected (create_access_level unchanged)", protectedTagGlob)
			return
		}
	} else {
		p.say("NOTICE: could not list protected tags (HTTP %s) — could-not-check the existing rule; still attempting to create it", st)
	}
	st, body = p.api.call(http.MethodPost, tagPath,
		object("name", protectedTagGlob, "create_access_level", raw(protectedTagCreateLevel)))
	if st == "201" {
		p.say("protected: tags '%s' (create_access_level=%s Maintainers) — only a human owner can create or move a release tag",
			protectedTagGlob, protectedTagCreateLevel)
		return
	}
	p.warn("error: protecting tags '%s' failed (HTTP %s): %s", protectedTagGlob, st, trimBody(body))
	p.record("protected-tags rule for '%s' not created (HTTP %s) — any Developer bot can create/move a release tag until fixed", protectedTagGlob, st)
}

// configureMergeSettings sets the two Free-tier, server-enforced merge checks in one PUT and
// reads both back off GET /projects/:id.
func (p *provisioner) configureMergeSettings() {
	projPath := "/projects/" + p.projectID
	st, _ := p.api.call(http.MethodPut, projPath,
		object("only_allow_merge_if_pipeline_succeeds", true, "only_allow_merge_if_all_discussions_are_resolved", true))
	if st != "200" {
		p.warn("error: setting merge checks failed (HTTP %s)", st)
		p.record("merge checks (pipeline-succeeds, all-discussions-resolved) could not be set (HTTP %s)", st)
		return
	}
	p.say("configured: pipelines must succeed before merge; all discussions must be resolved before merge")
	st, body := p.api.call(http.MethodGet, projPath, nil)
	if st != "200" {
		p.say("NOTICE: could not read merge checks back (HTTP %s) — could-not-check, not a pass", st)
		p.record("merge-checks read-back failed — verify the pipeline and discussion gates by hand")
		return
	}
	v, _ := decodeJSON(body)
	pipe := hasString(v, "only_allow_merge_if_pipeline_succeeds", "unknown")
	disc := hasString(v, "only_allow_merge_if_all_discussions_are_resolved", "unknown")
	p.say("read-back: only_allow_merge_if_pipeline_succeeds=%s only_allow_merge_if_all_discussions_are_resolved=%s", pipe, disc)
	if pipe != "true" {
		p.say("NOTICE: only_allow_merge_if_pipeline_succeeds reads %s, intended true", pipe)
		p.record("only_allow_merge_if_pipeline_succeeds read back as %s, intended true", pipe)
	}
	if disc != "true" {
		p.say("NOTICE: only_allow_merge_if_all_discussions_are_resolved reads %s, intended true", disc)
		p.record("only_allow_merge_if_all_discussions_are_resolved read back as %s, intended true", disc)
	}
}

// configureLabels ensures every fleet label. A duplicate name answers 409 on some GitLab
// versions and 400 "already exists" on others — both are success for an ensure.
func (p *provisioner) configureLabels() {
	for _, l := range fleetLabels {
		st, body := p.api.call(http.MethodPost, "/projects/"+p.projectID+"/labels",
			object("name", l.Name, "color", "#"+l.Color, "description", l.Description))
		msg := trimBody(body)
		switch {
		case st == "200" || st == "201":
			p.say("label: created '%s' (#%s)", l.Name, l.Color)
		case st == "409",
			st == "400" && (strings.Contains(msg, "already exists") || strings.Contains(msg, "has already been taken")):
			p.say("label: '%s' already exists (no-op)", l.Name)
		default:
			p.warn("error: creating label '%s' failed (HTTP %s): %s", l.Name, st, msg)
			p.record("label '%s' not created (HTTP %s) — the desk write that reaches for it (deskflip queue swap / deskfile --raised-by) degrades silently until fixed", l.Name, st)
		}
	}
}

// --- --tier ultimate ---------------------------------------------------------------------
//
// Two Ultimate-only refinements: a custom reviewer role that cannot push (Reporter base plus
// admin_merge_request), and an external status check the verdict lane posts to. Both
// endpoints answer 403 on a lesser instance; that is RECORDED as could-not-check, never a
// silent downgrade and never "done".

func (p *provisioner) configureUltimate() {
	reviewer := fleetUsername(p.o.prefix, "reviewer")
	roleName := p.o.prefix + "-reviewer-role"
	if p.o.dryRun {
		p.say("[dry-run] --tier ultimate: would create custom role '%s' (base=Reporter/%s, admin_merge_request=true, no push) via POST /groups/:id/member_roles", roleName, customRoleBaseLevel)
		p.say("[dry-run]   would assign the custom role to service account %s via PUT /groups/:id/members/:user_id (member_role_id)", reviewer)
		if p.o.project != "" {
			url := p.o.statusCheckURL
			if url == "" {
				url = "<--status-check-url>"
			}
			p.say("[dry-run] --tier ultimate: would register external status check '%s' -> %s on protected main via POST /projects/:id/external_status_checks", p.o.statusCheckName, url)
		} else {
			p.say("[dry-run] --tier ultimate: would register external status check '%s' — SKIPPED in this enumeration (needs --project); pass --project to include it", p.o.statusCheckName)
		}
		return
	}

	// A. custom reviewer role — reuse one of the same name rather than mint a duplicate.
	rolesPath := "/groups/" + p.groupID + "/member_roles"
	st, body := p.api.call(http.MethodGet, rolesPath, nil)
	roleID := ""
	switch st {
	case "200":
		roleID = firstMatch(body, "name", roleName, "id")
	case "403", "404":
		p.record("custom role: reading /groups/%s/member_roles returned HTTP %s (could-not-check — the instance tier does not expose member roles; Ultimate required)", p.o.group, st)
		return
	}
	if roleID == "" {
		st, body = p.api.call(http.MethodPost, rolesPath,
			object("name", roleName, "base_access_level", raw(customRoleBaseLevel), "admin_merge_request", true))
		if st == "201" || st == "200" {
			v, _ := decodeJSON(body)
			roleID = alt(path(v, "id"), "")
			p.say("custom role: created '%s' (id=%s)", roleName, roleID)
		} else {
			p.record("custom role: creating '%s' returned HTTP %s (could-not-check on a non-Ultimate instance)", roleName, st)
		}
	} else {
		p.say("no-op: custom role '%s' already exists (id=%s)", roleName, roleID)
	}
	if roleID != "" {
		_, body = p.api.call(http.MethodGet, "/groups/"+p.groupID+"/service_accounts", nil)
		if reviewerID := firstMatch(body, "username", reviewer, "id"); reviewerID != "" {
			st, _ = p.api.call(http.MethodPut, "/groups/"+p.groupID+"/members/"+reviewerID,
				object("member_role_id", raw(roleID)))
			if st == "200" {
				p.say("custom role: assigned '%s' to %s (id=%s)", roleName, reviewer, reviewerID)
			} else {
				p.record("custom role: assigning '%s' to %s returned HTTP %s", roleName, reviewer, st)
			}
		} else {
			p.record("custom role: reviewer account %s not found — cannot assign '%s'", reviewer, roleName)
		}
	}

	// B. external status check.
	if p.o.project == "" {
		p.say("NOTICE: --tier ultimate but no --project — external status check registration skipped (could-not-check, not a pass); pass --project to register it")
		return
	}
	if p.o.statusCheckURL == "" {
		p.record("external status check: --status-check-url not given — GitLab requires an external_url to register '%s'", p.o.statusCheckName)
		return
	}
	checksPath := "/projects/" + uriEncode(p.o.project) + "/external_status_checks"
	st, body = p.api.call(http.MethodGet, checksPath, nil)
	if st == "403" || st == "404" {
		p.record("external status check: reading /projects/%s/external_status_checks returned HTTP %s (could-not-check — Ultimate tier required)", p.o.project, st)
		return
	}
	if st == "200" {
		if existing := firstMatch(body, "name", p.o.statusCheckName, "id"); existing != "" {
			p.say("no-op: external status check '%s' already registered (id=%s)", p.o.statusCheckName, existing)
			return
		}
	}
	st, _ = p.api.call(http.MethodPost, checksPath,
		object("name", p.o.statusCheckName, "external_url", p.o.statusCheckURL))
	if st == "201" || st == "200" {
		p.say("external status check: registered '%s' -> %s", p.o.statusCheckName, p.o.statusCheckURL)
		return
	}
	p.record("external status check: registering '%s' returned HTTP %s (could-not-check on a non-Ultimate instance)", p.o.statusCheckName, st)
}

// --- summary ---------------------------------------------------------------------------

const ultimateDoneChecklist = `1. Ultimate-tier settings:
   - Custom reviewer role + external status check: CONFIGURED (or attempted,
     with any 403 recorded as could-not-check) by this run's --tier ultimate
     section above. Verify per docs/adopting-assay-gitlab.md's Ultimate section
     — the custom role must FAIL a push (negative test), the status check must
     be required on protected main.
   - Pipeline execution policy enforcing the ci-config project's pipeline:
     still human-only (not scripted here).`

const ultimateTodoChecklist = `1. Ultimate-tier settings (require an Ultimate license):
   - Custom role for the reviewer service account (Reporter base, no push).
   - External status checks wired to CI verdicts.
   - Pipeline execution policy enforcing the ci-config project's pipeline.
   Re-run with --tier ultimate on an Ultimate instance to script the first two.`

const commonChecklist = `2. Group token-expiry policy: set the group/instance PAT max lifetime to
   %s days or less (Settings > General > Permissions), the
   backstop behind rotate-on-mint (spec.md §5).
3. Create the locked ci-config project: a Maintainer-humans-only project,
   protected main, approval rules on, NO bot membership. Point each fleet
   project's CI/CD configuration file at
   "<path>/.gitlab-ci.yml@<group>/<ci-config-project>" (Settings > CI/CD >
   General pipelines > CI/CD configuration file). See
   docs/adopting-assay-gitlab.md's ci-config runbook section.`

const rule = "============================================================"

// summary prints the human-only remainder and any failed steps, and returns the exit code.
func (p *provisioner) summary() int {
	p.say("")
	p.say(rule)
	p.say("HUMAN-ONLY REMAINDER — this script does not and cannot do these:")
	p.say(rule)
	if p.o.tier == "ultimate" {
		p.say("%s", ultimateDoneChecklist)
	} else {
		p.say("%s", ultimateTodoChecklist)
	}
	p.say(commonChecklist, p.o.patExpiryDays)
	if len(p.failures) > 0 {
		p.say("------------------------------------------------------------")
		p.say("FAILED STEPS — this run did NOT complete cleanly:")
		for _, f := range p.failures {
			p.say("   - %s", f)
		}
		p.say(rule)
		return 1
	}
	p.say(rule)
	return 0
}
