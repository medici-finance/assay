package deskkit

// runcredential.go — WHO may start a workflow run or clear a deployment gate on a repo
// (forge-neutral brief 14). The answer is the per-repo run-credential binding in the roster
// (ASSAY_RUN_CREDENTIALS, rosterconfig.go), read here BEFORE any token is minted.
//
// WHY A BINDING OF ITS OWN. GitHub grants dispatching a workflow and approving a pending
// deployment through ONE permission, `actions: write`, and the same permission also cancels
// any run, deletes run logs and disables workflows repo-wide. None of the desk's role Apps
// holds it and none should acquire it by accident, so the credential that starts a release
// is a DEDICATED one — the `release-runner` role — which an operator creates and binds on
// purpose, per repo. A repo nobody automated is bound to a human instead, and that is a
// state this resolver refuses on, never a gap it routes around.
//
// THREE DISTINCT OUTCOMES, never conflated:
//
//	unbound (or the key is invalid)  a CONFIGURATION gap — could-not-check (exit 6) naming
//	                                  the missing key, the unconfigured-forge refusal's shape
//	human:<name>                     a DELIBERATE state — Refused (exit 5) naming the human
//	                                  and the repo; nothing is minted, no ambient credential
//	                                  is read
//	release-runner                   proceeds; the caller then obtains the role's token
//	                                  through ForgeFor's ordinary custody path, whose own
//	                                  refusal of an unminted token is the second, independent
//	                                  layer
//
// SINGLE-POINT-OF-FAILURE. The binding is the one control between "a run starts / a gate
// clears" and the wrong actor doing either. The layer behind it fails on a different signal
// in a different component: the backends refuse to reach the forge without an explicitly
// minted token (GitHubForge.restClient, GitLabForge.client/triggerClient), so a resolver bug
// that lets a request through without a real credential still cannot reach the forge.

import (
	"fmt"
	"regexp"
	"strings"
)

// ReleaseRunnerRole is the App/custody role a run credential binds. It is a role like
// worker/reviewer/verifier — minted by `desktoken release-runner` on GitHub, read from the
// `gitlab-release-runner.token` custody file on GitLab — and it is never one of those roles'
// credentials repurposed (ResolveRunCredential refuses a binding that shares another role's
// App).
const ReleaseRunnerRole = "release-runner"

// humanRunBindingPrefix is the identity-layer token for a human (the same `human:<slug>`
// class verifyrun's executing-runner fallback emits) read here as a run-credential value.
const humanRunBindingPrefix = "human:"

// RunCredential is one repo's resolved run-credential binding. Exactly one of Human and Role
// is set.
type RunCredential struct {
	// Repo is the lowercased owner/name the binding names.
	Repo string
	// Human is the bound human's name when the binding is `human:<name>`.
	Human string
	// Role is ReleaseRunnerRole when the binding is the release-runner role.
	Role string
	// GateShape is the declared gate shape (release-runner bindings only); empty when the
	// entry declared none.
	GateShape GateShape
}

// runBindingNameRe is the shape of a bound human's name.
var runBindingNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// parseRunCredentials parses ASSAY_RUN_CREDENTIALS. A malformed entry, an unknown value, a
// bare basename or a repo bound twice marks the key invalid AND returns an empty map: a
// partially parsed binding set would let the entries that happened to parse dispatch while
// the operator believes the file is the configuration in force.
func parseRunCredentials(raw string, issue *extAccumulator) map[string]RunCredential {
	out := map[string]RunCredential{}
	for _, entry := range splitList(raw) {
		key, val, hasEq := strings.Cut(entry, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if !hasEq || key == "" || val == "" {
			issue.bad("%s: cannot parse entry %q — expected owner/name=human:<name> or "+
				"owner/name=%s[+environment|+manual-job]", EnvRunCredentials, entry, ReleaseRunnerRole)
			continue
		}
		if strings.Count(key, "/") != 1 || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") ||
			strings.Contains(key, "*") {
			issue.bad("%s: entry %q's repo %q is not a full owner/name slug — this key chooses who may start a "+
				"release, so a basename or pattern that could cover more than one repo is refused", EnvRunCredentials, entry, key)
			continue
		}
		if _, dup := out[key]; dup {
			issue.bad("%s: repo %q is bound more than once", EnvRunCredentials, key)
			continue
		}
		cred := RunCredential{Repo: key}
		switch {
		case strings.HasPrefix(strings.ToLower(val), humanRunBindingPrefix):
			name := strings.TrimSpace(val[len(humanRunBindingPrefix):])
			if !runBindingNameRe.MatchString(name) {
				issue.bad("%s: entry %q names no human — expected human:<name> with a plain name", EnvRunCredentials, entry)
				continue
			}
			cred.Human = name
		default:
			role, shape, hasShape := strings.Cut(val, "+")
			if strings.ToLower(strings.TrimSpace(role)) != ReleaseRunnerRole {
				issue.bad("%s: entry %q binds %q, which is neither human:<name> nor the %s role — no other "+
					"role may start a run", EnvRunCredentials, entry, role, ReleaseRunnerRole)
				continue
			}
			cred.Role = ReleaseRunnerRole
			if hasShape {
				gs, err := ParseGateShape(shape)
				if err != nil || gs == "" {
					issue.bad("%s: entry %q declares gate shape %q — the shapes are %q and %q", EnvRunCredentials,
						entry, shape, GateShapeEnvironment, GateShapeManualJob)
					continue
				}
				cred.GateShape = gs
			}
		}
		out[key] = cred
	}
	if issue.invalid {
		return map[string]RunCredential{}
	}
	return out
}

// ResolveRunCredential resolves repo's run-credential binding and applies the identity rule.
// It reads configuration only — it mints nothing and contacts nothing — so a caller calls it
// BEFORE any credential exists:
//
//   - unbound, or ASSAY_RUN_CREDENTIALS invalid → Unverifiable (exit 6) naming the key;
//   - `human:<name>` → Refused (exit 5) naming the human and the repo, with the
//     RunCredential returned alongside so the caller can report it;
//   - `release-runner` whose roster App binding is SHARED with another desk role → Refused:
//     the release credential is never a worker/reviewer/verifier identity repurposed;
//   - `release-runner` otherwise → the binding, nil error.
func ResolveRunCredential(repo ForgeRepo) (RunCredential, error) {
	cfg := EffectiveConfig()
	key := strings.ToLower(strings.TrimSpace(repo.Slug()))
	if ext, ok := cfg.Ext["run-credentials"]; ok && ext.Status == ExtInvalid {
		return RunCredential{}, Unverifiable(fmt.Sprintf(
			"could-not-check: %s is invalid (%s), so no repo has a run-credential binding — %s cannot be "+
				"dispatched or gate-approved until the key is fixed", EnvRunCredentials, ext.Reason, repo.Slug()), nil)
	}
	cred, ok := cfg.RunCredentials[key]
	if !ok {
		return RunCredential{}, Unverifiable(fmt.Sprintf(
			"could-not-check: no %s entry binds %s, so who may start a run or clear a gate on it is unconfigured. "+
				"Add %s=%s (a dedicated release-runner credential) or %s=human:<name> (dispatching stays a human "+
				"action) to the roster — never a flag or environment variable", EnvRunCredentials, repo.Slug(),
			repo.Slug(), ReleaseRunnerRole, repo.Slug()), nil)
	}
	if cred.Human != "" {
		return cred, Refused(fmt.Sprintf(
			"refused: %s's run credential is bound to human:%s — dispatching a run or approving a gate on %s is a "+
				"human action today, done by that human in their own identity. Nothing was minted and no ambient "+
				"credential was read. To automate it, an operator binds a dedicated %s credential in %s",
			repo.Slug(), cred.Human, repo.Slug(), ReleaseRunnerRole, EnvRunCredentials))
	}
	if slug, bound := cfg.RoleBots[ReleaseRunnerRole]; bound && strings.TrimSpace(slug) != "" {
		for role, other := range cfg.RoleBots {
			if role != ReleaseRunnerRole && strings.EqualFold(other, slug) {
				return cred, Refused(fmt.Sprintf(
					"refused: the %s role is bound to App %q, which is also the %s role's App — the release credential "+
						"must be a dedicated identity, never a desk role's App repurposed (it would carry actions:write "+
						"for every write that role makes). Fix the %s entry", ReleaseRunnerRole, slug, role, EnvTrustedBotSlugs))
			}
		}
	}
	return cred, nil
}
