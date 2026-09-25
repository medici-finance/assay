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
	p.configureProtectedMain(pp)
	p.configureApprovals(pp)
	p.configureProtectedTags(pp)
	p.configureMergeChecks(pp)
	ensureLabels(&gitlabLabels{c: p.gl, projectPath: pp}, p.e.stdout, p.fail)
}

// protectedRule is the part of a protected-branch rule this step decides.
type protectedRule struct {
	exists   bool
	push     int    // levelNone when the read-back carried no push level
	merge    int    // levelNone when the read-back carried no merge level
	force    string // "true" | "false" | "unknown" — `false` is a value, never "absent"
	pushUser int64  // 0 = no user-scoped push entry
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
//   - DECIDING (readback=false) — the rule read before a change. An absent push level reads
//     as 0, an absent merge level as 40, an absent force-push as false (the script's
//     read_protected_main: `// 0`, `// 40`), so the no-op/PATCH/re-create choice and the
//     restore body stay the script's.
//   - READ-BACK (readback=true) — the rule read after a change, to CHECK it. Every absent
//     field reads as unknown (levelNone / "unknown"), never as the intended value, so an
//     absent merge level is a recorded failure (the script's readback_protected_main:
//     `// "none"`).
func parseProtectedRule(body []byte, readback bool) protectedRule {
	push, merge, forceDefault := 0, mergeAccessLevel, "false"
	if readback {
		push, merge, forceDefault = levelNone, levelNone, "unknown"
	}
	var raw struct {
		Push []struct {
			AccessLevel *int   `json:"access_level"`
			UserID      *int64 `json:"user_id"`
		} `json:"push_access_levels"`
		Merge []struct {
			AccessLevel *int `json:"access_level"`
		} `json:"merge_access_levels"`
		Force *bool `json:"allow_force_push"`
	}
	r := protectedRule{exists: true, push: push, merge: merge, force: forceDefault}
	if json.Unmarshal(body, &raw) != nil {
		return r
	}
	if len(raw.Push) > 0 {
		if raw.Push[0].AccessLevel != nil {
			r.push = *raw.Push[0].AccessLevel
		}
		if raw.Push[0].UserID != nil {
			r.pushUser = *raw.Push[0].UserID
		}
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

// restoreBody re-applies exactly the rule that was read.
func restoreBody(prev protectedRule) map[string]any {
	force := prev.force == "true"
	if prev.pushUser != 0 {
		return map[string]any{"name": "main", "allowed_to_push": []map[string]any{{"user_id": prev.pushUser}},
			"allowed_to_merge": []map[string]any{{"access_level": prev.merge}}, "allow_force_push": force}
	}
	return map[string]any{"name": "main", "push_access_level": prev.push, "merge_access_level": prev.merge,
		"allow_force_push": force}
}

func (p *provisioner) configureProtectedMain(pp string) {
	prev := p.readProtectedMain(pp)
	var wantUser int64
	if p.tier == "premium" {
		wantUser = p.boardWriterID
	}
	pushAsIntended := (wantUser != 0 && prev.pushUser == wantUser) ||
		(wantUser == 0 && prev.pushUser == 0 && prev.push == 0)

	// Preference 1 — already as intended: touch nothing.
	if prev.exists && prev.merge == mergeAccessLevel && prev.force == "false" && pushAsIntended {
		p.outf("no-op: main is already protected as intended — nothing deleted, nothing re-created")
		p.readbackProtectedMain(pp)
		return
	}
	// Preference 2 — only allow_force_push is wrong: PATCH it; the rule is never removed.
	if prev.exists && prev.force != "false" && prev.merge == mergeAccessLevel && pushAsIntended {
		resp, err := p.gl.do("PATCH", pp+"/protected_branches/main", map[string]any{"allow_force_push": false})
		if err == nil && resp.Status == 200 {
			p.outf("patched: main allow_force_push=false (PATCH — the rule was never removed)")
			p.readbackProtectedMain(pp)
			return
		}
		p.outf("NOTICE: PATCH of allow_force_push did not succeed (%s) — falling back to a guarded re-create",
			respOrErr(resp, err))
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
				p.readbackProtectedMain(pp)
				return
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
			return
		}
		resp, err := p.gl.do("POST", pp+"/protected_branches", restoreBody(prev))
		if err == nil && resp.Status == 201 {
			p.fail("protected-branch step failed; 'main' was restored to its PREVIOUS rule (merge_access_level=%d) — "+
				"the intended rule is NOT in place", prev.merge)
		} else {
			p.fail("%s (the previous rule could not be re-applied: %s)", urgent, respOrErr(resp, err))
		}
		return
	}
	if applied == "premium" {
		p.outf("protected: main (allowed_to_push=board-writer only, allowed_to_merge=Maintainer role (%d), allow_force_push=false)",
			mergeAccessLevel)
	} else {
		p.outf("NOTICE: free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)")
		p.outf("protected: main (push_access_level=0 'No one', merge_access_level=%d Maintainers, allow_force_push=false)",
			mergeAccessLevel)
	}
	p.readbackProtectedMain(pp)
}

// readbackProtectedMain reads the decided fields BACK, so a wrong rule is visible now.
func (p *provisioner) readbackProtectedMain(pp string) {
	resp, err := p.gl.do("GET", pp+"/protected_branches/main", nil)
	if err != nil || resp.Status != 200 {
		p.fail("protected-branch read-back on 'main' failed (%s) — could-not-check, verify push/merge/force-push by hand",
			respOrErr(resp, err))
		return
	}
	r := parseProtectedRule(resp.Body, true)
	p.outf("read-back: main push_access_level=%s push_user_id=%d merge_access_level=%s allow_force_push=%s",
		levelString(r.push), r.pushUser, levelString(r.merge), r.force)
	if r.merge != mergeAccessLevel {
		p.fail("protected 'main' merge_access_level is %s, intended %d (Maintainers) — at 30 every Developer "+
			"service account can merge its own MR; 'none' means the forge's reply carried no merge level "+
			"(could-not-check, not a pass)", levelString(r.merge), mergeAccessLevel)
	}
	if r.force != "false" {
		p.fail("protected 'main' allow_force_push is %s, intended false", r.force)
	}
}

func (p *provisioner) configureApprovals(pp string) {
	body := map[string]any{"merge_requests_author_approval": false, "merge_requests_disable_committers_approval": true}
	resp, err := p.gl.do("POST", pp+"/approvals", body)
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
