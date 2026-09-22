package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestManifestTierCountsAndDuties — Verify row 3: family produces 6 manifests, team
// produces 2, and every family manifest carries the requiredDuties permission set.
func TestManifestTierCountsAndDuties(t *testing.T) {
	family, err := TierManifests("family", "assay")
	if err != nil {
		t.Fatalf("family tier: %v", err)
	}
	if len(family) != 6 {
		t.Fatalf("family tier produced %d manifests, want 6", len(family))
	}
	t.Logf("family tier: %d manifests", len(family))

	team, err := TierManifests("team", "assay")
	if err != nil {
		t.Fatalf("team tier: %v", err)
	}
	if len(team) != 2 {
		t.Fatalf("team tier produced %d manifests, want 2", len(team))
	}
	t.Logf("team tier: %d manifests", len(team))

	for _, spec := range family {
		for _, d := range requiredDuties {
			if !containsPerm(spec.Permissions, d) {
				t.Fatalf("family App %s is missing requiredDuties permission %s", spec.Name, d)
			}
		}
	}
	t.Log("requiredDuties covered by every family manifest")
}

// TestManifestFamilyNamesAndExtras pins the exact family-tier names and the reviewer/desk
// administration:read carve-out from the brief's facts.
func TestManifestFamilyNamesAndExtras(t *testing.T) {
	specs, err := TierManifests("family", "assay")
	if err != nil {
		t.Fatal(err)
	}
	byRole := map[string]AppSpec{}
	for _, s := range specs {
		if len(s.Roles) != 1 {
			t.Fatalf("family spec %s should bind exactly one role, got %v", s.Name, s.Roles)
		}
		byRole[s.Roles[0]] = s
	}
	for _, role := range []string{"reviewer", "worker", "verifier", "desk", "issue-loop", "intake-loop"} {
		spec, ok := byRole[role]
		if !ok {
			t.Fatalf("no family App for role %s", role)
		}
		wantName := "assay-" + role + "-app"
		if spec.Name != wantName {
			t.Fatalf("role %s App name = %q, want %q", role, spec.Name, wantName)
		}
	}
	for _, role := range []string{"reviewer", "desk"} {
		if !containsPerm(byRole[role].Permissions, "administration:read") {
			t.Fatalf("role %s should carry administration:read", role)
		}
	}
	for _, role := range []string{"worker", "verifier", "issue-loop", "intake-loop"} {
		if containsPerm(byRole[role].Permissions, "administration:read") {
			t.Fatalf("role %s should NOT carry administration:read", role)
		}
		if containsPerm(byRole[role].Permissions, "administration:write") {
			t.Fatalf("role %s must never carry administration:write — read-only is the whole grant", role)
		}
	}
	for _, role := range []string{"verifier", "issue-loop", "intake-loop"} {
		for _, ci := range ciReadDuties {
			if containsPerm(byRole[role].Permissions, ci) {
				t.Fatalf("role %s should not carry CI-read permission %s per the brief's facts", role, ci)
			}
		}
	}
}

// TestManifestNoAdministrationWrite — no manifest, in either tier, ever requests
// administration:write. Read-only is the whole grant (brief facts).
func TestManifestNoAdministrationWrite(t *testing.T) {
	for _, tier := range []string{"team", "family"} {
		specs, err := TierManifests(tier, "assay")
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range specs {
			if containsPerm(spec.Permissions, "administration:write") {
				t.Fatalf("%s tier App %s must never request administration:write", tier, spec.Name)
			}
		}
	}
}

// TestBuildManifestJSON checks the manifest JSON carries exactly the fields the brief's
// facts name, and no `public: true`.
func TestBuildManifestJSON(t *testing.T) {
	spec := AppSpec{Name: "assay-act", Permissions: []string{"contents:write", "metadata"}}
	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "assay-act" {
		t.Fatalf("name = %v", m["name"])
	}
	if m["redirect_url"] != "http://127.0.0.1:41873/callback" {
		t.Fatalf("redirect_url = %v", m["redirect_url"])
	}
	if pub, ok := m["public"].(bool); !ok || pub {
		t.Fatalf("public = %v, want false", m["public"])
	}
	events, ok := m["default_events"].([]any)
	if !ok || len(events) != 0 {
		t.Fatalf("default_events = %v, want []", m["default_events"])
	}
	perms, ok := m["default_permissions"].(map[string]any)
	if !ok || perms["contents"] != "write" || perms["metadata"] != "read" {
		t.Fatalf("default_permissions = %v", m["default_permissions"])
	}
	// assay#1260: a webhook-less spec (no HookExtra at all, this spec's case) posts NO
	// hook_attributes key — see TestBuildManifestJSONOmitsHookAttributesTierPath.
	if _, ok := m["hook_attributes"]; ok {
		t.Fatalf("hook_attributes = %v, want the key absent entirely (assay#1260)", m["hook_attributes"])
	}
	if strings.Contains(string(raw), "\"url\":\"\"") {
		t.Fatal("manifest url must not be empty")
	}
}

// TestBuildManifestJSONOmitsHookAttributesTierPath pins medici-finance/assay#1260: a
// --tier spec never names a webhook (AppSpec.HookExtra is always nil on this path), so the
// posted manifest JSON must carry NO "hook_attributes" key at all — not
// {"hook_attributes": {"active": false}} with no url, which GitHub's own new-App manifest
// page rejects with `"url" wasn't supplied` (reported live against feat/apps-installer-02,
// deskapps init --manifest). Before the fix this test fails: the raw JSON contains
// `"hook_attributes":{"active":false}`.
func TestBuildManifestJSONOmitsHookAttributesTierPath(t *testing.T) {
	spec := AppSpec{Name: "assay-worker-app", Permissions: []string{"contents:write"}}
	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hook_attributes") {
		t.Fatalf("raw manifest JSON must not mention hook_attributes at all: %s", raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["hook_attributes"]; ok {
		t.Fatalf("decoded manifest carries hook_attributes = %v, want the key absent", m["hook_attributes"])
	}
}

// TestBuildManifestJSONEmitsHookAttributesWhenURLSet pins the ONE branch of BuildManifestJSON
// that emits hook_attributes — reached only when a spec's HookExtra names a "url" (N-3). No
// entry path constructs such a spec today (the --tier path never sets HookExtra, and
// LoadManifestFile refuses a manifest hook_attributes.url), so this test drives the branch
// directly with a hand-built AppSpec rather than leaving it unexercised: if the loader's
// refusal is ever relaxed, this is the guard that proves the emitted object carries the url
// AND the defaulted active:false, rather than the branch going live having never run.
func TestBuildManifestJSONEmitsHookAttributesWhenURLSet(t *testing.T) {
	spec := AppSpec{
		Name:        "assay-worker-app",
		Permissions: []string{"contents:write"},
		HookExtra:   map[string]any{"url": "https://example.invalid/hook"},
	}
	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	hook, ok := m["hook_attributes"].(map[string]any)
	if !ok {
		t.Fatalf("hook_attributes absent or not an object: %v", m["hook_attributes"])
	}
	if hook["url"] != "https://example.invalid/hook" {
		t.Fatalf("hook_attributes.url = %v, want the spec's url", hook["url"])
	}
	if active, ok := hook["active"].(bool); !ok || active {
		t.Fatalf("hook_attributes.active = %v, want the defaulted false", hook["active"])
	}
}
