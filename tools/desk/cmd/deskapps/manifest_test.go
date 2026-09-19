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
	hook, ok := m["hook_attributes"].(map[string]any)
	if !ok || hook["active"] != false {
		t.Fatalf("hook_attributes = %v, want active:false", m["hook_attributes"])
	}
	if strings.Contains(string(raw), "\"url\":\"\"") {
		t.Fatal("manifest url must not be empty")
	}
}
