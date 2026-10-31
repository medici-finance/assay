package main

// project.go — the --project settings steps, ported from the script's configure_* functions.
//
// Two properties are inherited and must not be lost in translation:
//
//  1. EVERY settings step runs even when an earlier one failed. Each failure is recorded
//     (provisioner.fail) and named in the closing summary, and the run exits non-zero.
//  2. `main` is NEVER left unprotected across a failure. Preference order: no-op (already as
//     intended) > PATCH (the one field PATCH accepts on every tier) > DELETE + POST, where a
//     refused POST immediately re-applies the rule that was read. One tightening over the
//     script: a DELETE the forge refuses is detected, and the step stops there with the old
//     rule still in place, instead of POSTing over a rule that was never removed.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (p *provisioner) configureProject() {
	resp, err := p.gl.do("GET", "/projects/"+url.PathEscape(p.o.project), nil)
	if err != nil {
		p.fail("could not resolve project %s: %v — no project setting was applied", p.o.project, err)
		return
	}
	if resp.Status != 200 {
		p.fail("could not resolve project %s (%s) — no project setting was applied", p.o.project, statusText(resp))
		return
	}
	var pr struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(resp.Body, &pr) != nil || pr.ID == 0 {
		p.fail("project %s: response carries no id — no project setting was applied", p.o.project)
		return
	}
	pp := "/projects/" + strconv.FormatInt(pr.ID, 10)

	if p.boardWriterID == 0 && p.tier != "free" {
		p.errf("NOTICE: board-writer service account id unknown — the Premium push allowlist cannot be named; " +
			"protecting main with the free-tier form instead")
		p.tier = "free"
	}
	mainRule, mainRead := p.configureProtectedMain(pp)
	p.configureApprovals(pp)
	p.configureProtectedTags(pp)
	p.configureMergeChecks(pp)
	ensureLabels(&gitlabLabels{c: p.gl, projectPath: pp}, p.e.stdout, p.fail)
	p.configurePushToken(pp, mainRule, mainRead)
}

// protectedRule is the part of a protected-branch rule this step decides. Its push side is
// EVERY push_access_levels entry, each classified on its own (pushEntry): a forge may hold
// several, and a check of entry [0] misses both a dropped per-user entry and an extra one.
type protectedRule struct {
	exists bool
	merge  int    // levelNone when the read-back carried no merge level
	force  string // "true" | "false" | "unknown" — `false` is a value, never "absent"
	push   []pushEntry
}

// The kinds of push entry. A deploy-key, group or user entry carries an access_level too, so
// an entry is a ROLE entry only when it names no principal at all. An entry this run cannot
// classify is pushUnknown, and every decision treats it as an entry it may not drop, widen or
// restore: fail closed.
const (
	pushRole      = "role"
	pushUser      = "user"
	pushGroup     = "group"
	pushDeployKey = "deploy_key"
	pushUnknown   = "unknown"
)

// pushEntry is one push_access_levels entry: a role entry's level, or a principal's id.
type pushEntry struct {
	kind  string
	id    int64 // the user, group or deploy key; 0 for a role or unknown entry
	level int   // a role entry's access level
}

func (e pushEntry) String() string {
	switch e.kind {
	case pushRole:
		return pushRole + ":" + strconv.Itoa(e.level)
	case pushUnknown:
		return pushUnknown
	}
	return e.kind + ":" + strconv.FormatInt(e.id, 10)
}

// principalKinds maps each principal field this run knows to its kind. Any OTHER non-null
// `*_id` field names a principal it does not know (the entry's own `id` excepted).
var principalKinds = map[string]string{"user_id": pushUser, "group_id": pushGroup, "deploy_key_id": pushDeployKey}

// classifyPushEntry classifies one entry as decoded from JSON. Exactly one non-null principal
// id makes it that principal's entry; none, with a numeric access_level, makes it a role
// entry; anything else (two principals, an unknown principal, a non-numeric value, no field
// at all, not an object) is pushUnknown.
func classifyPushEntry(raw any) pushEntry {
	m, ok := raw.(map[string]any)
	if !ok {
		return pushEntry{kind: pushUnknown}
	}
	var found []pushEntry
	for k, v := range m {
		if k == "id" || !strings.HasSuffix(k, "_id") || v == nil {
			continue
		}
		n, isNum := v.(float64)
		if isNum && n == 0 {
			continue // GitLab answers an absent principal as null; 0 names no one either
		}
		kind, known := principalKinds[k]
		if !known || !isNum || n != float64(int64(n)) {
			return pushEntry{kind: pushUnknown}
		}
		found = append(found, pushEntry{kind: kind, id: int64(n)})
	}
	switch len(found) {
	case 1:
		return found[0]
	case 0:
		if n, ok := m["access_level"].(float64); ok && n == float64(int(n)) {
			return pushEntry{kind: pushRole, level: int(n)}
		}
	}
	return pushEntry{kind: pushUnknown}
}

// hasPushUser reports whether a user-scoped push entry names id.
func (r protectedRule) hasPushUser(id int64) bool {
	for _, e := range r.push {
		if e.kind == pushUser && e.id == id {
			return true
		}
	}
	return false
}

// pushIsOnlyUser reports whether the push side is exactly one entry, a user entry naming id.
func (r protectedRule) pushIsOnlyUser(id int64) bool {
	return len(r.push) == 1 && r.push[0].kind == pushUser && r.push[0].id == id
}

// pushIsNoOne reports whether every push entry is a role entry at 0 ("No one"). An empty push
// side reads as No one, the deciding read's default.
func (r protectedRule) pushIsNoOne() bool {
	for _, e := range r.push {
		if e.kind != pushRole || e.level != 0 {
			return false
		}
	}
	return true
}

// admitsRole reports whether a ROLE entry lets a member at level push (30 admits Developers
// and Maintainers, 40 Maintainers; 0 admits no one). A user, group or deploy-key entry
// admits that principal only, never a role.
func (r protectedRule) admitsRole(level int) bool {
	for _, e := range r.push {
		if e.kind == pushRole && e.level > 0 && e.level <= level {
			return true
		}
	}
	return false
}

// pushOutside lists every push entry outside the intended set: wantUser's user entry (when
// wantUser is not 0) and role entries at 0, which admit no one. Anything else — another
// user, a role that admits someone, a group, a deploy key, an unrecognised entry — is extra.
func (r protectedRule) pushOutside(wantUser int64) []string {
	var out []string
	for _, e := range r.push {
		switch {
		case e.kind == pushRole && e.level == 0:
		case wantUser != 0 && e.kind == pushUser && e.id == wantUser:
		default:
			out = append(out, e.String())
		}
	}
	return out
}

// unrestorable names the entries a restore could not re-apply exactly, or "" when it can. A
// restore sends user entries by id and role entries by level; it never re-sends a group, a
// deploy key or an unrecognised entry (turning one into a role grant would WIDEN push). With
// no user entry it sends the scalar push_access_level, which carries one role entry only;
// more than that needs the allowed_to_push array, which only the Premium form is known to
// accept.
func (r protectedRule) unrestorable(premium bool) string {
	var bad []string
	users, roles := 0, 0
	for _, e := range r.push {
		switch e.kind {
		case pushUser:
			users++
		case pushRole:
			roles++
		default:
			bad = append(bad, e.String())
		}
	}
	if len(bad) > 0 {
		return strings.Join(bad, ",")
	}
	if users == 0 && roles > 1 && !premium {
		return r.pushList() + " (several role entries; the scalar form re-applies one)"
	}
	return ""
}

// roleLevels is every role entry's level, or "none".
func (r protectedRule) roleLevels() string {
	var s []string
	for _, e := range r.push {
		if e.kind == pushRole {
			s = append(s, strconv.Itoa(e.level))
		}
	}
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ",")
}

func (r protectedRule) pushUserList() string {
	var s []string
	for _, e := range r.push {
		if e.kind == pushUser {
			s = append(s, strconv.FormatInt(e.id, 10))
		}
	}
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ",")
}

// pushList is every push entry, in reply order, or "none".
func (r protectedRule) pushList() string {
	if len(r.push) == 0 {
		return "none"
	}
	s := make([]string, len(r.push))
	for i, e := range r.push {
		s[i] = e.String()
	}
	return strings.Join(s, ",")
}

// levelNone is an access level the forge's reply did not carry. It is a sentinel, never an
// access level GitLab uses, so the read-back reports it as "none" and never mistakes an
// ABSENT merge level for the intended one (the script's `// "none"`).
const levelNone = -1

func levelString(n int) string {
	if n == levelNone {
		return "none"
	}
	return strconv.Itoa(n)
}

// parseProtectedRule reads a protected-branch rule. It has two modes, as the script does:
//
//   - DECIDING (readback=false) — the rule read before a change. An absent merge level reads
//     as 40 and an absent force-push as false (the script's read_protected_main: `// 40`); an
//     absent push side is no entries, which reads as No one (`// 0`).
//   - READ-BACK (readback=true) — the rule read after a change, to CHECK it. Every absent
//     field reads as unknown (levelNone / "unknown"), never as the intended value, so an
//     absent merge level is a recorded failure (the script's readback_protected_main:
//     `// "none"`), and so is an absent push side.
func parseProtectedRule(body []byte, readback bool) protectedRule {
	merge, forceDefault := mergeAccessLevel, "false"
	if readback {
		merge, forceDefault = levelNone, "unknown"
	}
	var raw struct {
		Push  []any `json:"push_access_levels"`
		Merge []struct {
			AccessLevel *int `json:"access_level"`
		} `json:"merge_access_levels"`
		Force *bool `json:"allow_force_push"`
	}
	r := protectedRule{exists: true, merge: merge, force: forceDefault}
	if json.Unmarshal(body, &raw) != nil {
		return r
	}
	for _, e := range raw.Push {
		r.push = append(r.push, classifyPushEntry(e))
	}
	if len(raw.Merge) > 0 && raw.Merge[0].AccessLevel != nil {
		r.merge = *raw.Merge[0].AccessLevel
	}
	if raw.Force != nil {
		r.force = strconv.FormatBool(*raw.Force)
	}
	return r
}

func (p *provisioner) readProtectedMain(pp string) protectedRule {
	resp, err := p.gl.do("GET", pp+"/protected_branches/main", nil)
	if err != nil || resp.Status != 200 {
		return protectedRule{merge: mergeAccessLevel, force: "false"}
	}
	return parseProtectedRule(resp.Body, false)
}

func (p *provisioner) protectBody(form string) map[string]any {
	if form == "premium" {
		return map[string]any{
			"name":                 "main",
			"allowed_to_push":      []map[string]any{{"user_id": p.boardWriterID}},
			"allowed_to_merge":     []map[string]any{{"access_level": mergeAccessLevel}},
			"allowed_to_unprotect": []map[string]any{{"access_level": unprotectAccessLevel}},
			"allow_force_push":     false,
		}
	}
	return map[string]any{"name": "main", "push_access_level": 0, "merge_access_level": mergeAccessLevel,
		"allow_force_push": false}
}

// restoreBody re-applies exactly the rule that was read: every user and role push entry, not
// the first one alone. It refuses (ok=false) a rule with any other entry, so a group, a deploy
// key or an unrecognised entry is never re-sent as a role grant; the caller checks
// unrestorable before it deletes anything, so this is the second line, not the first.
func restoreBody(prev protectedRule) (map[string]any, bool) {
	force := prev.force == "true"
	var users, roles []map[string]any
	scalar := 0
	for _, e := range prev.push {
		switch e.kind {
		case pushUser:
			users = append(users, map[string]any{"user_id": e.id})
		case pushRole:
			roles = append(roles, map[string]any{"access_level": e.level})
			scalar = e.level
		default:
			return nil, false
		}
	}
	if len(users) > 0 || len(roles) > 1 {
		return map[string]any{"name": "main", "allowed_to_push": append(users, roles...),
			"allowed_to_merge": []map[string]any{{"access_level": prev.merge}}, "allow_force_push": force}, true
	}
	return map[string]any{"name": "main", "push_access_level": scalar, "merge_access_level": prev.merge,
		"allow_force_push": force}, true
}

// configureProtectedMain returns the rule as read BACK after the step, and whether that
// read-back succeeded.
func (p *provisioner) configureProtectedMain(pp string) (protectedRule, bool) {
	prev := p.readProtectedMain(pp)
	var wantUser int64
	if p.tier == "premium" {
		wantUser = p.boardWriterID
	}
	pushAsIntended := (wantUser != 0 && prev.pushIsOnlyUser(wantUser)) ||
		(wantUser == 0 && prev.pushIsNoOne())

	// Preference 1 — already as intended: touch nothing.
	if prev.exists && prev.merge == mergeAccessLevel && prev.force == "false" && pushAsIntended {
		p.outf("no-op: main is already protected as intended — nothing deleted, nothing re-created")
		return p.readbackProtectedMain(pp, wantUser, true)
	}
	// Preference 2 — only allow_force_push is wrong: PATCH it; the rule is never removed.
	if prev.exists && prev.force != "false" && prev.merge == mergeAccessLevel && pushAsIntended {
		resp, err := p.gl.do("PATCH", pp+"/protected_branches/main", map[string]any{"allow_force_push": false})
		if err == nil && resp.Status == 200 {
			p.outf("patched: main allow_force_push=false (PATCH — the rule was never removed)")
			return p.readbackProtectedMain(pp, wantUser, true)
		}
		p.outf("NOTICE: PATCH of allow_force_push did not succeed (%s) — falling back to a guarded re-create",
			respOrErr(resp, err))
	}

	// Refuse before any DELETE: a rule whose push side this run cannot re-apply exactly (a
	// group, a deploy key, an entry it cannot classify, or a shape the tier's restore form
	// cannot carry) is left untouched. Deleting it would leave no faithful way back, and
	// turning such an entry into a role grant would widen who may push.
	if prev.exists {
		if bad := prev.unrestorable(p.tier == "premium"); bad != "" {
			p.fail("protected-branch step refused to replace the existing rule on 'main': its push side holds "+
				"entries this run cannot restore faithfully (%s; all entries: %s) — nothing was deleted, the "+
				"PREVIOUS rule is still in place and the intended rule is NOT. Set it by hand (Settings > "+
				"Repository > Protected branches) and re-run to confirm", bad, prev.pushList())
			return p.readbackProtectedMain(pp, 0, false)
		}
	}

	// Preference 3 — guarded DELETE + POST.
	var forms []string
	switch p.tier {
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
			resp, err := p.gl.do("DELETE", pp+"/protected_branches/main", nil)
			if err != nil || (resp.Status != 204 && resp.Status != 200) {
				p.fail("protected-branch step could not replace the existing rule on 'main' (DELETE: %s) — the "+
					"PREVIOUS rule is still in place (merge_access_level=%d); the intended rule is NOT",
					respOrErr(resp, err), prev.merge)
				// The intended rule is not in place and that is already recorded: the read-back
				// prints the rule that remains, without a second failure for its push entry.
				return p.readbackProtectedMain(pp, 0, false)
			}
			deleted = true
			p.outf("unprotected: main (re-applying now; the previous rule goes back immediately if that fails)")
		}
		resp, err := p.gl.do("POST", pp+"/protected_branches", p.protectBody(form))
		if err == nil && resp.Status == 201 {
			applied = form
			break
		}
		p.outf("NOTICE: protect '%s' form refused (%s)", form, respOrErr(resp, err))
		if err == nil && form == "premium" && resp.Status == 400 && strings.Contains(string(resp.Body), "allowed_to_") {
			p.outf("tier: the Premium form was refused for its allowed_to_ arrays — retrying with the free-tier form")
			continue
		}
		break
	}

	if applied == "" {
		urgent := fmt.Sprintf("URGENT: 'main' is UNPROTECTED — protect it by hand: POST %s/protected_branches with "+
			"push_access_level=0 merge_access_level=%d allow_force_push=false", pp, mergeAccessLevel)
		if !prev.exists {
			p.fail("%s", urgent)
			return protectedRule{}, false
		}
		body, ok := restoreBody(prev)
		if !ok {
			// Unreachable while the refusal above stands; kept so a restore never widens.
			p.fail("%s (the previous rule's push entries %s cannot be re-applied faithfully; nothing was re-sent)",
				urgent, prev.pushList())
			return protectedRule{}, false
		}
		resp, err := p.gl.do("POST", pp+"/protected_branches", body)
		if err == nil && resp.Status == 201 {
			p.fail("protected-branch step failed; 'main' was restored to its PREVIOUS rule (merge_access_level=%d) — "+
				"the intended rule is NOT in place", prev.merge)
		} else {
			p.fail("%s (the previous rule could not be re-applied: %s)", urgent, respOrErr(resp, err))
		}
		return protectedRule{}, false
	}
	if applied == "premium" {
		p.outf("protected: main (allowed_to_push=board-writer only, allowed_to_merge=Maintainer role (%d), allow_force_push=false)",
			mergeAccessLevel)
	} else {
		p.outf("NOTICE: free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)")
		p.outf("protected: main (push_access_level=0 'No one', merge_access_level=%d Maintainers, allow_force_push=false)",
			mergeAccessLevel)
	}
	wantUser = 0
	if applied == "premium" {
		wantUser = p.boardWriterID
	}
	return p.readbackProtectedMain(pp, wantUser, true)
}

// readbackProtectedMain reads the decided fields BACK, so a wrong rule is visible now.
// wantUser is the user a per-user push entry was written for (0: none was): a forge can answer
// that write 2xx and then not hold the entry (seen on GitLab 19.5.3), so its absence from the
// read-back is a recorded failure, never a pass. With check, EVERY push entry is held to the
// intended set — wantUser's user entry, and role entries at 0 (No one) — and any other entry
// is a recorded failure. Without check (the intended rule was never written, and that is
// already recorded) it prints the rule that remains and records nothing more for its push side.
func (p *provisioner) readbackProtectedMain(pp string, wantUser int64, check bool) (protectedRule, bool) {
	resp, err := p.gl.do("GET", pp+"/protected_branches/main", nil)
	if err != nil || resp.Status != 200 {
		p.fail("protected-branch read-back on 'main' failed (%s) — could-not-check, verify push/merge/force-push by hand",
			respOrErr(resp, err))
		return protectedRule{}, false
	}
	r := parseProtectedRule(resp.Body, true)
	p.outf("read-back: main push_access_level=%s push_user_ids=%s merge_access_level=%s allow_force_push=%s push_entries=%s",
		r.roleLevels(), r.pushUserList(), levelString(r.merge), r.force, r.pushList())
	if check && len(r.push) == 0 {
		p.fail("protected 'main' read back with no push entries — could-not-check who may push, verify by hand")
	}
	if extra := r.pushOutside(wantUser); check && len(extra) > 0 {
		p.fail("protected 'main' push side holds entries outside the intended set (%s; intended: %s) — remove "+
			"them by hand (Settings > Repository > Protected branches) and re-run to confirm",
			strings.Join(extra, ","), intendedPush(wantUser))
	}
	if check && wantUser != 0 && !r.hasPushUser(wantUser) {
		p.fail("protected 'main' has no per-user push entry for the board-writer (user id %d): the forge accepted "+
			"the write, but the entry is absent from the read-back — add it by hand (Settings > Repository > "+
			"Protected branches) and re-run to confirm", wantUser)
	}
	if r.merge != mergeAccessLevel {
		p.fail("protected 'main' merge_access_level is %s, intended %d (Maintainers) — at 30 every Developer "+
			"service account can merge its own MR; 'none' means the forge's reply carried no merge level "+
			"(could-not-check, not a pass)", levelString(r.merge), mergeAccessLevel)
	}
	if r.force != "false" {
		p.fail("protected 'main' allow_force_push is %s, intended false", r.force)
	}
	return r, true
}

// intendedPush names the intended push side for a failure message.
func intendedPush(wantUser int64) string {
	if wantUser != 0 {
		return "user:" + strconv.FormatInt(wantUser, 10) + " only"
	}
	return "role:0 (No one) only"
}

func (p *provisioner) configureApprovals(pp string) {
	body := map[string]any{"merge_requests_author_approval": false, "merge_requests_disable_committers_approval": true}
	resp, err := p.gl.do("POST", pp+"/approvals", body)
	// This project has already resolved; only this endpoint's CE 404 is a tier gap.
	// Authentication, permission and server errors remain failures.
	if err == nil && resp.Status == 404 {
		p.outf("NOTICE: approval settings unavailable at this tier (failed-at-tier, remediation: Premium)")
		p.outf("NOTICE: do not count approvals as a server-enforced gate on this tier")
		return
	}
	if err != nil || (resp.Status != 200 && resp.Status != 201) {
		p.fail("approval settings write failed (%s)", respOrErr(resp, err))
		return
	}
	// A 201 is not proof: on Free the write is accepted and the values stay at defaults.
	resp, err = p.gl.do("GET", pp+"/approvals", nil)
	if err != nil || resp.Status != 200 {
		p.fail("approval settings read-back failed (%s) — could-not-check, verify by hand", respOrErr(resp, err))
		return
	}
	author := boolField(resp.Body, "merge_requests_author_approval")
	committers := boolField(resp.Body, "merge_requests_disable_committers_approval")
	available := boolField(resp.Body, "merge_request_approvers_available")
	if author == "false" && committers == "true" && available != "false" {
		p.outf("configured: approvals (prevent-author, prevent-committers) — read back and confirmed")
		return
	}
	p.outf("read-back: approvals merge_requests_author_approval=%s merge_requests_disable_committers_approval=%s "+
		"merge_request_approvers_available=%s", author, committers, available)
	p.outf("NOTICE: approval settings ignored at this tier (failed-at-tier, remediation: Premium)")
	if available == "false" {
		p.outf("NOTICE: approval RULES are unavailable here — do not count approvals as a server-enforced gate on this tier")
	}
}

func (p *provisioner) configureProtectedTags(pp string) {
	resp, err := p.gl.do("GET", pp+"/protected_tags", nil)
	if err == nil && resp.Status == 200 {
		var tags []struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(resp.Body, &tags)
		for _, t := range tags {
			if t.Name == protectedTagGlob {
				p.outf("no-op: tags matching '%s' are already protected (create_access_level unchanged)", protectedTagGlob)
				return
			}
		}
	} else {
		p.outf("NOTICE: could not list protected tags (%s) — could-not-check the existing rule; still attempting to create it",
			respOrErr(resp, err))
	}
	resp, err = p.gl.do("POST", pp+"/protected_tags",
		map[string]any{"name": protectedTagGlob, "create_access_level": protectedTagCreateLevel})
	if err == nil && resp.Status == 201 {
		p.outf("protected: tags '%s' (create_access_level=%d Maintainers)", protectedTagGlob, protectedTagCreateLevel)
		return
	}
	p.fail("protected-tags rule for '%s' not created (%s) — any Developer bot can create/move a release tag until fixed",
		protectedTagGlob, respOrErr(resp, err))
}

func (p *provisioner) configureMergeChecks(pp string) {
	body := map[string]any{"only_allow_merge_if_pipeline_succeeds": true,
		"only_allow_merge_if_all_discussions_are_resolved": true}
	resp, err := p.gl.do("PUT", pp, body)
	if err != nil || resp.Status != 200 {
		p.fail("merge checks (pipeline-succeeds, all-discussions-resolved) could not be set (%s)", respOrErr(resp, err))
		return
	}
	p.outf("configured: pipelines must succeed before merge; all discussions must be resolved before merge")
	resp, err = p.gl.do("GET", pp, nil)
	if err != nil || resp.Status != 200 {
		p.fail("merge-checks read-back failed (%s) — could-not-check, verify the pipeline and discussion gates by hand",
			respOrErr(resp, err))
		return
	}
	pv := boolField(resp.Body, "only_allow_merge_if_pipeline_succeeds")
	dv := boolField(resp.Body, "only_allow_merge_if_all_discussions_are_resolved")
	p.outf("read-back: only_allow_merge_if_pipeline_succeeds=%s only_allow_merge_if_all_discussions_are_resolved=%s", pv, dv)
	if pv != "true" {
		p.fail("only_allow_merge_if_pipeline_succeeds read back as %s, intended true", pv)
	}
	if dv != "true" {
		p.fail("only_allow_merge_if_all_discussions_are_resolved read back as %s, intended true", dv)
	}
}

// boolField reads a boolean by KEY presence: `false` is a value, and an absent key is
// "unknown" — never read as false.
func boolField(body []byte, key string) string {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return "unknown"
	}
	v, ok := m[key]
	if !ok {
		return "unknown"
	}
	if b, ok := v.(bool); ok {
		return strconv.FormatBool(b)
	}
	return fmt.Sprint(v)
}

func respOrErr(r apiResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	return statusText(r)
}
