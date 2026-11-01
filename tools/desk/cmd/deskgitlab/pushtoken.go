package main

// pushtoken.go — the board-push credential the scaffolded statusgen-regen job reads,
// STATUSGEN_PUSH_TOKEN (docs/adopting-assay-gitlab.md §2c), per the recorded ruling on
// issue #5004 (option B). The twin of deskfleet's pushtoken.go; the constants are pinned
// equal by tables_parity_test.go.
//
//   - a DEDICATED project access token, scope write_repository only, role Maintainer. Never
//     a role-fleet PAT: the board-writer's carries `api`, and the board-writer stays Developer.
//   - stored as a project CI/CD variable flagged masked AND protected, and read back by those
//     flags. The read-back GET returns the value too; this run never prints, logs or writes it
//     to disk, and checks only the two flags.
//   - where the forge will not mint a project access token (gitlab.com Free answers 403/404),
//     the step is a recorded failure naming the manual step, and every other refusal names its
//     own cause. It NEVER falls back to the board-writer PAT or any other credential this run
//     holds.
//   - the token expires after --pat-expiry-days (default 7, at most 365) like the role PATs,
//     and nothing here rotates it: an existing variable is a no-op, so rotation is the manual
//     step in docs/adopting-assay-gitlab.md §2c.
//
// An existing variable is a named no-op. A minted token whose variable write fails is
// REPORTED by name and id and left in place (report, never auto-revoke). The step never
// changes who may push `main`; it prints whether the read-back rule admits a Maintainer.

import (
	"net/http"
	"strconv"
)

const pushTokenManualStep = "MANUAL STEP: create a project or group access token (scope write_repository only, " +
	"role Maintainer) and store it as the masked AND protected CI/CD variable " + pushTokenVariable +
	" (docs/adopting-assay-gitlab.md §2c); never a role-fleet token"

// Every reply below is reported by its HTTP status alone: the variable endpoints answer with
// the variable's VALUE and the mint endpoint with the token, so no body is ever echoed.
func (p *provisioner) configurePushToken(mainRule protectedRule, mainRead bool) {
	varPath := "/projects/" + p.projectID + "/variables/" + pushTokenVariable
	st, body := p.api.call(http.MethodGet, varPath, nil)
	switch st {
	case "200":
		v, _ := decodeJSON(body)
		masked, protected := hasString(v, "masked", "unknown"), hasString(v, "protected", "unknown")
		if masked != "true" || protected != "true" {
			p.say("NOTICE: CI/CD variable %s exists with masked=%s protected=%s — both must be true", pushTokenVariable, masked, protected)
			p.record("CI/CD variable %s exists with masked=%s protected=%s — both must be true; fix its flags by hand (Settings > CI/CD > Variables, docs/adopting-assay-gitlab.md §2c)",
				pushTokenVariable, masked, protected)
		} else {
			p.say("no-op: CI/CD variable %s already exists (masked=true, protected=true) — no token minted; a fresh one is a rotation, not a re-provision. Its credential is not inspected: it must be a write_repository-only access token, never a role-fleet PAT",
				pushTokenVariable)
		}
		p.noticePushAdmission(mainRule, mainRead)
		return
	case "404":
		// Absent: mint it below.
	default:
		p.say("NOTICE: could not check for an existing %s (HTTP %s) — could-not-check, nothing minted", pushTokenVariable, st)
		p.record("%s: could not check for an existing variable (HTTP %s) — nothing minted. %s", pushTokenVariable, st, pushTokenManualStep)
		return
	}

	days, _ := strconv.Atoi(p.o.patExpiryDays)
	expires := now().UTC().AddDate(0, 0, days).Format("2006-01-02")
	st, body = p.api.call(http.MethodPost, "/projects/"+p.projectID+"/access_tokens",
		object("name", pushTokenName, "scopes", []string{pushTokenScope}, "access_level", raw(pushTokenAccessLevel), "expires_at", expires))
	switch {
	case st != "201" && !(len(st) == 3 && st[0] == '4'):
		// Only a 4xx is a definite refusal: no answer, a 5xx or another status may mean the
		// forge acted.
		p.say("NOTICE: the project access-token mint's outcome is unknown (HTTP %s) — a token named %s may exist", st, pushTokenName)
		p.record("%s: the project access-token mint was sent but its outcome is unknown (HTTP %s) — a token named %s may EXIST; list Settings > Access tokens and revoke it if present. %s",
			pushTokenVariable, st, pushTokenName, pushTokenManualStep)
		return
	case st != "201":
		p.say("NOTICE: this forge refused to mint a project access token (HTTP %s) — %s not provisioned", st, pushTokenVariable)
		p.record("%s not provisioned: %s. The board-writer PAT is never used in its place. %s",
			pushTokenVariable, mintRefusalCause(st), pushTokenManualStep)
		return
	}
	v, _ := decodeJSON(body)
	id, botID, token := alt(path(v, "id"), "unknown"), alt(path(v, "user_id"), "unknown"), alt(path(v, "token"), "")
	if token == "" {
		p.say("NOTICE: the mint answered HTTP 201 without a token value — %s not provisioned", pushTokenVariable)
		p.record("%s: the mint answered HTTP 201 without a token value — project access token %s (id %s) EXISTS and was NOT revoked; revoke it under Settings > Access tokens. %s",
			pushTokenVariable, pushTokenName, id, pushTokenManualStep)
		return
	}
	st, _ = p.api.call(http.MethodPost, "/projects/"+p.projectID+"/variables",
		object("key", pushTokenVariable, "value", token, "masked", true, "protected", true))
	token = ""
	if st != "201" {
		p.say("NOTICE: storing %s failed (HTTP %s) — the minted token is left in place, by name below", pushTokenVariable, st)
		p.record("%s: the variable write failed (HTTP %s) — project access token %s (id %s, bot user id %s) EXISTS and was NOT revoked; revoke it under Settings > Access tokens. %s",
			pushTokenVariable, st, pushTokenName, id, botID, pushTokenManualStep)
		return
	}
	p.say("minted: project access token %s (id %s, bot user id %s, scopes=%s, access_level=%s Maintainer, expires %s) -> CI/CD variable %s (masked, protected) — value never echoed",
		pushTokenName, id, botID, pushTokenScope, pushTokenAccessLevel, expires, pushTokenVariable)

	st, body = p.api.call(http.MethodGet, varPath, nil)
	if st != "200" {
		p.say("NOTICE: could not read %s back (HTTP %s) — its flags are could-not-check, not a pass", pushTokenVariable, st)
		p.record("%s read-back failed (HTTP %s) — verify its masked/protected flags by hand", pushTokenVariable, st)
	} else if v, _ := decodeJSON(body); hasString(v, "masked", "unknown") != "true" || hasString(v, "protected", "unknown") != "true" {
		p.record("%s read back as masked=%s protected=%s, intended both true — fix its flags by hand",
			pushTokenVariable, hasString(v, "masked", "unknown"), hasString(v, "protected", "unknown"))
	} else {
		p.say("read-back: %s masked=true protected=true", pushTokenVariable)
	}
	p.noticePushAdmission(mainRule, mainRead)
}

// mintRefusalCause names why the forge refused the mint, by its 4xx status (deskfleet's twin):
// only 403/404 is the tier gap; the others are a request, a credential or a validation
// problem, and naming them as the tier would send the reader to the wrong fix.
func mintRefusalCause(st string) string {
	switch st {
	case "403", "404":
		return "project access tokens are not available on this forge or tier (HTTP " + st + ") — gitlab.com Free has none"
	case "400":
		return "the forge rejected the request as invalid (HTTP 400) — for example an expiry past the instance's maximum token lifetime; check --pat-expiry-days"
	case "401":
		return "the run's credential was not accepted for the mint (HTTP 401)"
	}
	return "the forge refused the mint (HTTP " + st + ")"
}

// noticePushAdmission says whether protected main's read-back push rule lets a Maintainer
// token push. It never changes the rule.
func (p *provisioner) noticePushAdmission(rule protectedRule, read bool) {
	switch {
	case !read:
		p.say("NOTICE: protected 'main' was not read back, so whether its push rule admits the Maintainer push token is could-not-check, not a pass")
	case !rule.admitsRole(pushTokenAccessLevel):
		// The rule this verb writes admits no role, so on a provisioned project this NOTICE is
		// the expected state on every run, not a new fault.
		p.say("NOTICE: protected 'main' push rule (role levels=%s, entries=%s) does not admit a Maintainer — the regen job's push is refused (class `scope`) and the board regenerates through a human-merged MR (docs/adopting-assay-gitlab.md §2c). This is the expected state under the rule this verb writes; this run does not widen the push rule — that is a human decision",
			rule.roleLevels(), rule.pushList())
	}
}
