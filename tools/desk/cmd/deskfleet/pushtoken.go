package main

// pushtoken.go — the board-push credential the scaffolded statusgen-regen job reads,
// STATUSGEN_PUSH_TOKEN (docs/adopting-assay-gitlab.md §2c), per the recorded ruling on
// issue #5004 (option B):
//
//   - a DEDICATED project access token, scope write_repository only, role Maintainer. Never
//     a role-fleet PAT: the board-writer's carries `api`, and the board-writer stays Developer.
//   - stored as a project CI/CD variable flagged masked AND protected, and read back by those
//     flags. The value is never read back, printed, or written to disk.
//   - where the forge will not mint a project access token (gitlab.com Free answers 403/404),
//     the step is a recorded failure naming the manual step. It NEVER falls back to the
//     board-writer PAT or any other credential this run holds.
//
// An existing variable is a named no-op: a fresh token for it is a rotation, not a
// re-provision. A minted token whose variable write fails is REPORTED by name and id and left
// in place (the partial-run ruling: report, never auto-revoke).
//
// The step does not change who may push `main`. It prints whether the rule it read back
// admits a Maintainer; where it does not, the regen job's push is refused and the board
// travels as a human-merged MR (§2c). Widening the push rule is not this verb's call.

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// pushTokenManualStep is the step a human takes where this one cannot complete.
const pushTokenManualStep = "MANUAL STEP: create a project or group access token (scope write_repository only, " +
	"role Maintainer) and store it as the masked AND protected CI/CD variable " + pushTokenVariable +
	" (docs/adopting-assay-gitlab.md §2c); never a role-fleet token"

// statusOnly renders a reply by its status alone. The variable endpoints answer with the
// variable's VALUE, and the mint endpoint with the token, so their bodies are never echoed.
func statusOnly(r apiResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("HTTP %d", r.Status)
}

func (p *provisioner) configurePushToken(pp string, mainRule protectedRule, mainRead bool) {
	varPath := pp + "/variables/" + url.PathEscape(pushTokenVariable)
	resp, err := p.gl.do("GET", varPath, nil)
	switch {
	case err == nil && resp.Status == 200:
		masked, protected := boolField(resp.Body, "masked"), boolField(resp.Body, "protected")
		if masked != "true" || protected != "true" {
			p.fail("CI/CD variable %s exists with masked=%s protected=%s — both must be true; fix its flags by hand "+
				"(Settings > CI/CD > Variables, docs/adopting-assay-gitlab.md §2c)", pushTokenVariable, masked, protected)
		} else {
			p.outf("no-op: CI/CD variable %s already exists (masked=true, protected=true) — no token minted; a fresh "+
				"one is a rotation, not a re-provision. Its credential is not inspected: it must be a write_repository-only "+
				"access token, never a role-fleet PAT", pushTokenVariable)
		}
		p.noticePushAdmission(mainRule, mainRead)
		return
	case err == nil && resp.Status == 404:
		// Absent: mint it below.
	default:
		p.fail("%s: could not check for an existing variable (%s) — could-not-check, nothing minted. %s",
			pushTokenVariable, statusOnly(resp, err), pushTokenManualStep)
		return
	}

	expires := p.e.now().UTC().AddDate(0, 0, p.o.patExpiryDays).Format("2006-01-02")
	resp, err = p.gl.do("POST", pp+"/access_tokens", map[string]any{"name": pushTokenName,
		"scopes": []string{pushTokenScope}, "access_level": pushTokenAccessLevel, "expires_at": expires})
	if outcomeUnknown(resp, err, 201) {
		p.fail("%s: the project access-token mint was sent but its outcome is unknown (%s) — a token named %s may "+
			"EXIST; list Settings > Access tokens and revoke it if present. %s",
			pushTokenVariable, statusOnly(resp, err), pushTokenName, pushTokenManualStep)
		return
	}
	if resp.Status != 201 {
		p.fail("%s not provisioned: this forge refused to mint a project access token (%s) — access tokens cannot "+
			"be minted here (gitlab.com Free has none). The board-writer PAT is never used in its place. %s",
			pushTokenVariable, statusOnly(resp, err), pushTokenManualStep)
		return
	}
	var tok struct {
		ID     int64  `json:"id"`
		UserID int64  `json:"user_id"`
		Token  string `json:"token"`
	}
	_ = json.Unmarshal(resp.Body, &tok)
	if tok.Token == "" {
		p.fail("%s: the mint answered HTTP 201 without a token value — project access token %s (id %d) EXISTS and "+
			"was NOT revoked; revoke it under Settings > Access tokens. %s",
			pushTokenVariable, pushTokenName, tok.ID, pushTokenManualStep)
		return
	}
	resp, err = p.gl.do("POST", pp+"/variables", map[string]any{"key": pushTokenVariable, "value": tok.Token,
		"masked": true, "protected": true})
	tok.Token = ""
	if err != nil || resp.Status != 201 {
		p.fail("%s: the variable write failed (%s) — project access token %s (id %d, bot user id %d) EXISTS and was "+
			"NOT revoked; revoke it under Settings > Access tokens. %s",
			pushTokenVariable, statusOnly(resp, err), pushTokenName, tok.ID, tok.UserID, pushTokenManualStep)
		return
	}
	p.outf("minted: project access token %s (id %d, bot user id %d, scopes=%s, access_level=%d Maintainer, expires %s) "+
		"-> CI/CD variable %s (masked, protected) — value never echoed",
		pushTokenName, tok.ID, tok.UserID, pushTokenScope, pushTokenAccessLevel, expires, pushTokenVariable)

	resp, err = p.gl.do("GET", varPath, nil)
	if err != nil || resp.Status != 200 {
		p.fail("%s read-back failed (%s) — could-not-check its masked/protected flags, verify by hand",
			pushTokenVariable, statusOnly(resp, err))
	} else if m, pr := boolField(resp.Body, "masked"), boolField(resp.Body, "protected"); m != "true" || pr != "true" {
		p.fail("%s read back as masked=%s protected=%s, intended both true — fix its flags by hand",
			pushTokenVariable, m, pr)
	} else {
		p.outf("read-back: %s masked=true protected=true", pushTokenVariable)
	}
	p.noticePushAdmission(mainRule, mainRead)
}

// noticePushAdmission says whether protected main's read-back push rule lets a Maintainer
// token push. It never changes the rule.
func (p *provisioner) noticePushAdmission(rule protectedRule, read bool) {
	switch {
	case !read:
		p.outf("NOTICE: protected 'main' was not read back, so whether its push rule admits the Maintainer push " +
			"token is could-not-check, not a pass")
	case !rule.admitsRole(pushTokenAccessLevel):
		p.outf("NOTICE: protected 'main' push rule (push_access_level=%s, user entries=%s) does not admit a Maintainer — "+
			"the regen job's push is refused (class `scope`) and the board regenerates through a human-merged MR "+
			"(docs/adopting-assay-gitlab.md §2c). This run does not widen the push rule; that is a human decision",
			levelString(rule.push), rule.pushUserList())
	}
}
