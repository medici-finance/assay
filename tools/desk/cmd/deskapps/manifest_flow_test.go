// manifest_flow_test.go — the --manifest path (assay--issue-1116-manifest): a single
// arbitrary GitHub App registered from a manifest JSON file instead of a tier's fixed App
// set. Mirrors manifest_test.go (manifest → AppSpec/JSON shape), callback_test.go (the
// state-nonce-gated callback) and pem_test.go (the PEM/mismatch writes), scoped to the
// manifest-driven path's own facts: fields read from the file, the two REFUSED fields, and
// the state row keyed by App NAME instead of a desk role.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifestFile writes body (already-marshalled JSON, or a raw string) to a fresh temp
// file and returns its path.
func writeManifestFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLoadManifestFileFields — a manifest carrying every documented field (name, url,
// description, public, default_permissions, default_events, hook_attributes minus url) is
// read back byte-for-byte, none of it silently dropped.
func TestLoadManifestFileFields(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"description": "Runs the public leak-sweep gate.",
		"public": true,
		"default_permissions": {"contents": "read", "metadata": "read"},
		"default_events": ["pull_request"],
		"hook_attributes": {"active": true}
	}`)

	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatalf("LoadManifestFile: %v", err)
	}
	if m.Name != "assay-leaksweep-app" {
		t.Fatalf("Name = %q", m.Name)
	}
	if m.URL != "https://github.com/medici-finance/assay" {
		t.Fatalf("URL = %q", m.URL)
	}
	if m.Description != "Runs the public leak-sweep gate." {
		t.Fatalf("Description = %q", m.Description)
	}
	if !m.Public {
		t.Fatal("Public = false, want true")
	}
	if m.DefaultPermissions["contents"] != "read" || m.DefaultPermissions["metadata"] != "read" {
		t.Fatalf("DefaultPermissions = %v", m.DefaultPermissions)
	}
	if len(m.DefaultEvents) != 1 || m.DefaultEvents[0] != "pull_request" {
		t.Fatalf("DefaultEvents = %v", m.DefaultEvents)
	}
	if active, ok := m.HookAttributes["active"].(bool); !ok || !active {
		t.Fatalf("HookAttributes[active] = %v", m.HookAttributes["active"])
	}
}

// TestLoadManifestFileRequiresNameAndURL — the two fields every manifest must carry.
func TestLoadManifestFileRequiresNameAndURL(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		p := writeManifestFile(t, `{"url": "https://example.invalid"}`)
		if _, err := LoadManifestFile(p); err == nil {
			t.Fatal("expected an error for a manifest with no name")
		}
	})
	t.Run("missing url", func(t *testing.T) {
		p := writeManifestFile(t, `{"name": "assay-leaksweep-app"}`)
		if _, err := LoadManifestFile(p); err == nil {
			t.Fatal("expected an error for a manifest with no url")
		}
	})
}

// TestLoadManifestFileRefusesRedirectURL — Verify: a manifest that specifies its own
// top-level redirect_url is REFUSED with a clear error, never silently stripped.
func TestLoadManifestFileRefusesRedirectURL(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"redirect_url": "https://attacker.invalid/callback"
	}`)
	_, err := LoadManifestFile(p)
	if err == nil {
		t.Fatal("expected LoadManifestFile to refuse a manifest carrying its own redirect_url")
	}
	if !strings.Contains(err.Error(), "redirect_url") {
		t.Fatalf("error does not name redirect_url: %v", err)
	}
}

// TestLoadManifestFileRefusesHookURL — Verify: a manifest that specifies
// hook_attributes.url is REFUSED, the same way and for the same reason as redirect_url.
func TestLoadManifestFileRefusesHookURL(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"hook_attributes": {"url": "https://attacker.invalid/hook", "active": true}
	}`)
	_, err := LoadManifestFile(p)
	if err == nil {
		t.Fatal("expected LoadManifestFile to refuse a manifest carrying hook_attributes.url")
	}
	if !strings.Contains(err.Error(), "hook_attributes.url") {
		t.Fatalf("error does not name hook_attributes.url: %v", err)
	}
}

// TestManifestAppSpecKeyedByName — the one structural difference from the --tier path: a
// manifest-driven AppSpec carries no desk Roles and is not ReadOnly, so its state-machine
// row (and any apps.env bindings) end up keyed by the App's manifest NAME alone.
func TestManifestAppSpecKeyedByName(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"default_permissions": {"contents": "write", "metadata": "read"}
	}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)
	if spec.Name != "assay-leaksweep-app" {
		t.Fatalf("spec.Name = %q", spec.Name)
	}
	if len(spec.Roles) != 0 {
		t.Fatalf("spec.Roles = %v, want empty (manifest Apps are not role-bound)", spec.Roles)
	}
	if spec.ReadOnly {
		t.Fatal("spec.ReadOnly = true, want false")
	}
	if !containsPerm(spec.Permissions, "contents:write") || !containsPerm(spec.Permissions, "metadata:read") {
		t.Fatalf("spec.Permissions = %v", spec.Permissions)
	}
}

// TestBuildManifestJSONFromManifest — the manifest-derived AppSpec's fields (url,
// description, public, default_events) reach the built GitHub Manifest JSON, and the
// tool's OWN redirect_url — never one from the file — is what is sent, the exact machinery
// the --tier path already uses (manifest_test.go's TestBuildManifestJSON is the tier-path
// sibling of this test). A manifest that names hook_attributes.active with no url (the only
// shape LoadManifestFile allows — it refuses hook_attributes.url outright) posts NO
// hook_attributes key at all (assay#1260): see
// TestBuildManifestJSONOmitsHookAttributesManifestPath for the dedicated pin.
func TestBuildManifestJSONFromManifest(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"description": "Runs the public leak-sweep gate.",
		"public": true,
		"default_permissions": {"contents": "read"},
		"default_events": ["pull_request"],
		"hook_attributes": {"active": true}
	}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)

	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "assay-leaksweep-app" {
		t.Fatalf("name = %v", got["name"])
	}
	if got["url"] != "https://github.com/medici-finance/assay" {
		t.Fatalf("url = %v", got["url"])
	}
	if got["description"] != "Runs the public leak-sweep gate." {
		t.Fatalf("description = %v", got["description"])
	}
	// The tool's OWN redirect_url, never anything from the manifest file (which cannot set
	// one — TestLoadManifestFileRefusesRedirectURL).
	if got["redirect_url"] != "http://127.0.0.1:41873/callback" {
		t.Fatalf("redirect_url = %v", got["redirect_url"])
	}
	if pub, ok := got["public"].(bool); !ok || !pub {
		t.Fatalf("public = %v, want true", got["public"])
	}
	events, ok := got["default_events"].([]any)
	if !ok || len(events) != 1 || events[0] != "pull_request" {
		t.Fatalf("default_events = %v", got["default_events"])
	}
	// assay#1260: hook_attributes named active:true but no url — the only shape reachable
	// through this flow — must NOT reach the posted JSON at all (GitHub rejects a url-less
	// hook_attributes regardless of active's value).
	if _, ok := got["hook_attributes"]; ok {
		t.Fatalf("hook_attributes = %v, want the key absent entirely (assay#1260)", got["hook_attributes"])
	}
}

// TestBuildManifestJSONFromManifestOmitsHookAttributesWhenUnset — a manifest that does not
// name hook_attributes at all also gets no hook_attributes key in the posted JSON
// (assay#1260) — never an absent-url {"active": false} that GitHub's schema rejects. (Renamed
// from ...DefaultsHookActiveFalse: there is no active:false default left to assert — the key
// is absent, which is what this test now pins.)
func TestBuildManifestJSONFromManifestOmitsHookAttributesWhenUnset(t *testing.T) {
	p := writeManifestFile(t, `{"name": "assay-leaksweep-app", "url": "https://github.com/medici-finance/assay"}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)
	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["hook_attributes"]; ok {
		t.Fatalf("hook_attributes = %v, want the key absent (assay#1260)", got["hook_attributes"])
	}
}

// TestBuildManifestJSONOmitsHookAttributesManifestPath pins medici-finance/assay#1260 for
// the --manifest path specifically: a manifest file that names hook_attributes.active:false
// and no url (the exact shape Ian's report reproduced — GitHub's new-App page replied
// `"url" wasn't supplied`) must post NO "hook_attributes" key at all in the raw JSON, not
// {"active": false}. Before the fix this test fails: the raw JSON contains
// `"hook_attributes":{"active":false}`.
func TestBuildManifestJSONOmitsHookAttributesManifestPath(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-worker-app",
		"url": "https://github.com/medici-finance/assay",
		"hook_attributes": {"active": false}
	}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)
	raw, err := BuildManifestJSON(spec, "http://127.0.0.1:41873/callback")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hook_attributes") {
		t.Fatalf("raw manifest JSON must not mention hook_attributes at all: %s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["hook_attributes"]; ok {
		t.Fatalf("decoded manifest carries hook_attributes = %v, want the key absent", got["hook_attributes"])
	}
}

// plantManifestPendingRow is plantPendingRow's --manifest sibling: one pending row for a
// manifest-derived spec, keyed by its App name.
func plantManifestPendingRow(t *testing.T, spec AppSpec) (*StateFile, string) {
	t.Helper()
	nonce, err := newStateNonce()
	if err != nil {
		t.Fatal(err)
	}
	sf := &StateFile{Schema: stateSchema, Apps: []AppRow{{
		App:        spec.Name,
		Tier:       "manifest",
		Roles:      spec.Roles,
		ReadOnly:   spec.ReadOnly,
		State:      StatePending,
		StateNonce: nonce,
	}}}
	return sf, nonce
}

// TestManifestCallbackKeysStateByAppNameNotRole — Verify: the manifest path's callback →
// conversion → PEM-write path is the SAME code as --tier's (callback_test.go /
// pem_test.go), reaching "keyed" and writing the PEM 0600 under the manifest App's own
// name; but because the spec carries no Roles, writeBindings writes no
// `<ROLE>_APP=`/`READ_APP=` line for it — the one structural difference from a --tier row.
func TestManifestCallbackKeysStateByAppNameNotRole(t *testing.T) {
	setupTest(t)
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"default_permissions": {"contents": "read"}
	}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)
	sf, nonce := plantManifestPendingRow(t, spec)

	fake := fakeConversionServer(t, conversionResult{ID: 42, ClientID: "cid", WebhookSecret: "whs", PEM: "PEMBYTES-manifest"})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "manifest", spec.Name, "", "me", []AppSpec{spec}, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (after following redirect to /run)", resp.StatusCode)
	}

	row := sf.rowByApp("assay-leaksweep-app")
	if row == nil {
		t.Fatal("no row keyed by the manifest App's name")
	}
	if row.State != StateKeyed {
		t.Fatalf("row state = %s, want keyed", row.State)
	}

	got, err := readPEMFor(t, "assay-leaksweep-app")
	if err != nil {
		t.Fatalf("reading PEM: %v", err)
	}
	if string(got) != "PEMBYTES-manifest" {
		t.Fatalf("pem content mismatch: %q", got)
	}

	content := readAppsEnv()
	if !strings.Contains(content, "ASSAY_LEAKSWEEP_APP_ID=42") {
		t.Fatalf("apps.env missing the App-ID record:\n%s", content)
	}
	for _, unwanted := range []string{"_APP=assay-leaksweep-app", "READ_APP="} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("apps.env carries a role binding for a manifest App (none expected): %q in:\n%s", unwanted, content)
		}
	}
}

// TestManifestCallbackMismatchNoWrite — pem_test.go's TestPemNeverWrittenOnMismatch,
// exercised through the manifest path: design.md §8's mismatch check is the SAME callback
// code, so it applies identically to a manifest-driven row.
func TestManifestCallbackMismatchNoWrite(t *testing.T) {
	setupTest(t)
	p := writeManifestFile(t, `{"name": "assay-leaksweep-app", "url": "https://github.com/medici-finance/assay"}`)
	m, err := LoadManifestFile(p)
	if err != nil {
		t.Fatal(err)
	}
	spec := ManifestAppSpec(m)
	sf, nonce := plantManifestPendingRow(t, spec)

	fake := fakeConversionServer(t, conversionResult{ID: 1, PEM: "PEMBYTES", Owner: struct {
		Login string `json:"login"`
	}{Login: "someone-else"}})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "manifest", spec.Name, "", "me", []AppSpec{spec}, sf)
	srv.identity = ghUser{Login: "the-real-operator"}
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if _, err := readPEMFor(t, "assay-leaksweep-app"); err == nil {
		t.Fatal("a PEM was written despite an identity mismatch")
	}
	if got := sf.rowByApp("assay-leaksweep-app").State; got != StatePending {
		t.Fatalf("row state = %s, want pending (re-armed) after a mismatch", got)
	}
}

// TestRunInitManifestAndTierMutuallyExclusive — Verify: --manifest and --tier refuse
// together, at the flag layer, before any file is read or port bound.
func TestRunInitManifestAndTierMutuallyExclusive(t *testing.T) {
	p := writeManifestFile(t, `{"name": "assay-leaksweep-app", "url": "https://github.com/medici-finance/assay"}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--manifest", p, "--tier", "family", "--dry-run"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected a non-zero exit for --manifest + --tier together; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("stderr does not name the mutual-exclusion refusal: %q", stderr.String())
	}
}

// TestRunInitManifestDryRun — --dry-run for --manifest reports the planned App without
// touching the network (no gh identity call, no listener left bound) — the manifest
// path's sibling of the --tier suite's dry-run behaviour (main.go's runInit).
func TestRunInitManifestDryRun(t *testing.T) {
	p := writeManifestFile(t, `{"name": "assay-leaksweep-app", "url": "https://github.com/medici-finance/assay"}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--manifest", p, "--dry-run", "--no-browser"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "assay-leaksweep-app") {
		t.Fatalf("dry-run output missing the planned App name:\n%s", out)
	}
	if !strings.Contains(out, "tier=manifest") {
		t.Fatalf("dry-run output missing tier=manifest:\n%s", out)
	}
}

// TestRunInitManifestBadFileReportsAndExits — a manifest file that fails validation is
// reported clearly and the process exits non-zero before any port is bound or file
// written — never a silent strip of the offending field.
func TestRunInitManifestBadFileReportsAndExits(t *testing.T) {
	p := writeManifestFile(t, `{
		"name": "assay-leaksweep-app",
		"url": "https://github.com/medici-finance/assay",
		"redirect_url": "https://attacker.invalid/callback"
	}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--manifest", p, "--dry-run"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected a non-zero exit for a manifest carrying redirect_url; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "redirect_url") {
		t.Fatalf("stderr does not name redirect_url: %q", stderr.String())
	}
}
