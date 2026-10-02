// manifest.go — the tier → GitHub App Manifest data (design.md §1 "Tier first, Apps
// second", §2 Screen 0/1, and brief 02's facts). No network, no I/O: this is pure data plus
// the JSON manifest a browser auto-POSTs to GitHub's app-manifest endpoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
//
// The URL/Description/Public/DefaultEvents/HookExtra fields exist for a --manifest-driven
// spec (see ManifestAppSpec below) only. A tier-derived spec (TierManifests) never sets
// them, and BuildManifestJSON falls back to the tier path's original fixed choices
// (manifestHomepageURL, public:false, default_events:[]) exactly as before — this is
// additive, not a behaviour change for --tier. hook_attributes is posted only when
// HookExtra names a "url" (never reachable today — LoadManifestFile refuses one); a
// webhook-less spec gets no hook_attributes key at all rather than {"active": false} with
// no url, which GitHub's manifest schema rejects (medici-finance/assay#1260).
type AppSpec struct {
	Name        string
	Permissions []string // "resource:level" (defaults to "read" when no ":level" is given)
	Roles       []string // desk roles bound to this App once keyed; empty for a --manifest App
	ReadOnly    bool     // true for the team-tier `<prefix>-read` App (bound via READ_APP=)

	URL           string         // manifest "url" (homepage); "" falls back to manifestHomepageURL
	Description   string         // manifest "description"; "" omits the field
	Public        bool           // manifest "public"
	DefaultEvents []string       // manifest "default_events"; nil falls back to []
	HookExtra     map[string]any // manifest "hook_attributes" minus "url" (refused at load — see LoadManifestFile)
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
// default_events:[] — the fields brief 02's facts name and no others — plus hook_attributes
// only when a webhook url is actually named (omitted entirely otherwise — assay#1260: a
// url-less hook_attributes, even {"active": false}, is rejected by GitHub's own schema).
type githubManifest struct {
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Description        string            `json:"description,omitempty"`
	RedirectURL        string            `json:"redirect_url"`
	Public             bool              `json:"public"`
	DefaultEvents      []string          `json:"default_events"`
	DefaultPermissions map[string]string `json:"default_permissions"`
	HookAttributes     map[string]any    `json:"hook_attributes,omitempty"`
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

	url := spec.URL
	if url == "" {
		url = manifestHomepageURL
	}

	events := spec.DefaultEvents
	if events == nil {
		events = []string{}
	}

	// hook_attributes: start from the manifest's own extra fields (never "url" — refused at
	// load, LoadManifestFile), default "active" to false when the manifest did not name it,
	// exactly the tier path's original fixed value.
	//
	// GitHub's manifest schema requires hook_attributes.url whenever hook_attributes is
	// present at all — posting {"active": false} (or any hook_attributes) with no url gets
	// GitHub's new-App page to reject the whole manifest with `"url" wasn't supplied`
	// (medici-finance/assay#1260, reported live against feat/apps-installer-02). Today
	// hook_attributes.url can never reach this function — LoadManifestFile refuses a
	// manifest that sets it — so a webhook-less App (no hook_attributes at all, or
	// active:false/true with no url) must post NO hook_attributes key rather than a
	// url-less one; omit the field entirely instead of sending it half-built.
	var hook map[string]any
	if _, hasURL := spec.HookExtra["url"]; hasURL {
		hook = make(map[string]any, len(spec.HookExtra)+1)
		for k, v := range spec.HookExtra {
			hook[k] = v
		}
		if _, ok := hook["active"]; !ok {
			hook["active"] = false
		}
	}

	m := githubManifest{
		Name:               spec.Name,
		URL:                url,
		Description:        spec.Description,
		RedirectURL:        redirectURL,
		Public:             spec.Public,
		DefaultEvents:      events,
		DefaultPermissions: perms,
		HookAttributes:     hook,
	}
	return json.Marshal(m)
}

// AppManifestFile is the --manifest JSON contract: a single arbitrary GitHub App's manifest
// fields. Everything else about the flow — the loopback redirect_url, the state nonce, the
// callback → conversion → PEM-write path, the design.md §8 mismatch check — is reused
// unchanged from the --tier path; this file's only job is turning this JSON into the one
// AppSpec that shared machinery drives.
type AppManifestFile struct {
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Description        string            `json:"description"`
	Public             bool              `json:"public"`
	DefaultPermissions map[string]string `json:"default_permissions"`
	DefaultEvents      []string          `json:"default_events"`
	HookAttributes     map[string]any    `json:"hook_attributes"`
}

// LoadManifestFile reads and validates path as a --manifest single-App registration. It
// REFUSES — a clear error, never a silent strip — a manifest that specifies its own
// top-level `redirect_url`, or a `hook_attributes.url`: both are deskapps's to set (the
// loopback callback the tool binds itself, and — for hook_attributes.url — a value this
// flow never sets on the manifest's behalf), never the manifest file's.
func LoadManifestFile(path string) (*AppManifestFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	// Presence-check on the RAW JSON first: unmarshalling straight into AppManifestFile
	// would silently drop an unknown/unwanted redirect_url field rather than refuse it, and
	// a struct field for it would invite exactly the "read it, then ignore it" bug this
	// check exists to prevent.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}
	if _, ok := raw["redirect_url"]; ok {
		return nil, fmt.Errorf("manifest %s sets its own redirect_url — deskapps sets the loopback redirect_url itself; remove redirect_url from the manifest", path)
	}
	if hookRaw, ok := raw["hook_attributes"]; ok {
		var hook map[string]json.RawMessage
		if err := json.Unmarshal(hookRaw, &hook); err != nil {
			return nil, fmt.Errorf("parsing manifest %s hook_attributes: %w", path, err)
		}
		if _, ok := hook["url"]; ok {
			return nil, fmt.Errorf("manifest %s sets hook_attributes.url — deskapps does not set a webhook URL through this flow; remove hook_attributes.url from the manifest", path)
		}
	}

	var m AppManifestFile
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}
	if strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("manifest %s: name is required", path)
	}
	if strings.TrimSpace(m.URL) == "" {
		return nil, fmt.Errorf("manifest %s: url is required", path)
	}
	return &m, nil
}

// ManifestAppSpec converts a validated AppManifestFile into the single AppSpec deskapps
// init's shared machinery drives. This is the one structural difference from the tier
// path: Roles is left empty and ReadOnly false, because a manifest-driven App is not bound
// to a desk role — its apps.state.json row (and every apps.env write) is keyed by this
// App's manifest NAME, not a role name, and writeBindings (records.go) correctly writes no
// `<ROLE>_APP=`/`READ_APP=` line for it as a result.
func ManifestAppSpec(m *AppManifestFile) AppSpec {
	perms := make([]string, 0, len(m.DefaultPermissions))
	for resource, level := range m.DefaultPermissions {
		perms = append(perms, resource+":"+level)
	}
	sort.Strings(perms) // deterministic order — map iteration is not

	hookExtra := make(map[string]any, len(m.HookAttributes))
	for k, v := range m.HookAttributes {
		hookExtra[k] = v
	}

	return AppSpec{
		Name:          m.Name,
		Permissions:   perms,
		URL:           m.URL,
		Description:   m.Description,
		Public:        m.Public,
		DefaultEvents: append([]string(nil), m.DefaultEvents...),
		HookExtra:     hookExtra,
	}
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
