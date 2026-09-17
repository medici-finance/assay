// manifest.go — the tier → GitHub App Manifest data (design.md §1 "Tier first, Apps
// second", §2 Screen 0/1, and brief 02's facts). No network, no I/O: this is pure data plus
// the JSON manifest a browser auto-POSTs to GitHub's app-manifest endpoint.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// deskRoles is the six desk roles the "family" tier mints one App per, in the design §2
// run-board order: reviewer, worker, verifier, then desk, issue-loop, intake-loop — chosen
// so a throttled first sitting still leaves a working review loop. It is also the set of
// roles the "team" tier's `<prefix>-act` App is bound to.
var deskRoles = []string{"reviewer", "worker", "verifier", "desk", "issue-loop", "intake-loop"}

// requiredDuties is the permission set every family-tier App carries no matter its role
// (brief 02 facts: "every one carries contents:write, issues:write, pull_requests:write").
var requiredDuties = []string{"contents:write", "issues:write", "pull_requests:write"}

// ciReadDuties is added to reviewer, worker and desk (family tier) — brief facts: "reviewer,
// worker and desk add checks:read, statuses:read, actions:read".
var ciReadDuties = []string{"checks:read", "statuses:read", "actions:read"}

// branchProtectionRead is added to reviewer and desk ONLY (family tier) — brief facts:
// "reviewer and desk additionally carry administration:read". It is READ-ONLY: the legacy
// branch-protection endpoint is the only one that can read a required-status set a ruleset
// with no required_status_checks rule cannot express (#1020); administration:write is never
// granted here because it could rewrite protection itself.
const branchProtectionRead = "administration:read"

// teamReadPerms is the team-tier `<prefix>-read` App's permission set.
var teamReadPerms = []string{
	"metadata", "contents:read", "issues:read", "pull_requests:read",
	"checks:read", "statuses:read", "actions:read", "administration:read",
}

// teamActPerms is the team-tier `<prefix>-act` App's permission set.
var teamActPerms = []string{
	"contents:write", "issues:write", "pull_requests:write",
	"checks:read", "statuses:read", "actions:read", "administration:read",
}

// AppSpec is one App this run will create: its manifest name, its GitHub Manifest
// permission set, and the desk roles bound to it once keyed (records.go writes the
// `<ROLE>_APP=` bindings — brief 01 — from this).
type AppSpec struct {
	Name        string
	Permissions []string // "resource:level" (defaults to "read" when no ":level" is given)
	Roles       []string // desk roles bound to this App once keyed
	ReadOnly    bool     // true for the team-tier `<prefix>-read` App (bound via READ_APP=)
}

// TierManifests returns the App rows a tier creates, in the design §2 run-board order.
func TierManifests(tier, prefix string) ([]AppSpec, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, fmt.Errorf("empty prefix")
	}
	switch tier {
	case "team":
		return []AppSpec{
			{
				Name:        prefix + "-read",
				Permissions: teamReadPerms,
				Roles:       append([]string(nil), deskRoles...),
				ReadOnly:    true,
			},
			{
				Name:        prefix + "-act",
				Permissions: teamActPerms,
				Roles:       append([]string(nil), deskRoles...),
			},
		}, nil
	case "family":
		specs := make([]AppSpec, 0, len(deskRoles))
		for _, role := range deskRoles {
			specs = append(specs, AppSpec{
				Name:        prefix + "-" + role + "-app",
				Permissions: permsForFamilyRole(role),
				Roles:       []string{role},
			})
		}
		return specs, nil
	default:
		return nil, fmt.Errorf("unknown tier %q: want team or family", tier)
	}
}

// permsForFamilyRole returns the permission set one family-tier role's App carries.
func permsForFamilyRole(role string) []string {
	perms := append([]string(nil), requiredDuties...)
	switch role {
	case "reviewer", "worker", "desk":
		perms = append(perms, ciReadDuties...)
	}
	if role == "reviewer" || role == "desk" {
		perms = append(perms, branchProtectionRead)
	}
	return perms
}

// containsPerm reports whether perms names permission p (exact match).
func containsPerm(perms []string, p string) bool {
	for _, x := range perms {
		if x == p {
			return true
		}
	}
	return false
}

// githubManifest is the JSON body a browser auto-POSTs to GitHub's App-manifest new-App
// page (design.md §3): name, url, redirect_url, public:false, default_permissions,
// default_events:[], hook_attributes:{active:false} — the fields brief 02's facts name and
// no others.
type githubManifest struct {
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	RedirectURL        string            `json:"redirect_url"`
	Public             bool              `json:"public"`
	DefaultEvents      []string          `json:"default_events"`
	DefaultPermissions map[string]string `json:"default_permissions"`
	HookAttributes     map[string]bool   `json:"hook_attributes"`
}

// manifestHomepageURL is the App's required homepage URL. Assay is the product these Apps
// belong to, so it points at the public project itself — never a private/internal address.
const manifestHomepageURL = "https://github.com/medici-finance/assay"

// BuildManifestJSON renders spec's GitHub App Manifest JSON, redirecting to redirectURL
// (`http://127.0.0.1:<port>/callback`, the port actually bound — design.md facts).
func BuildManifestJSON(spec AppSpec, redirectURL string) ([]byte, error) {
	perms := make(map[string]string, len(spec.Permissions))
	for _, p := range spec.Permissions {
		resource, level := splitPerm(p)
		perms[resource] = level
	}
	m := githubManifest{
		Name:               spec.Name,
		URL:                manifestHomepageURL,
		RedirectURL:        redirectURL,
		Public:             false,
		DefaultEvents:      []string{},
		DefaultPermissions: perms,
		HookAttributes:     map[string]bool{"active": false},
	}
	return json.Marshal(m)
}

// splitPerm splits "resource:level" into its parts; a bare "resource" (metadata has no
// write level) defaults to "read".
func splitPerm(p string) (resource, level string) {
	if i := strings.IndexByte(p, ':'); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, "read"
}

// newAppURL returns the GitHub URL the manifest form auto-submits to for the given owner
// kind: org-owned or personal-owned (design.md §3).
func newAppURL(ownerKind, org string) string {
	if ownerKind == "me" {
		return "https://github.com/settings/apps/new"
	}
	return fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", org)
}
