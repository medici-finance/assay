package main

// pushtoken_test.go — the board-push credential (STATUSGEN_PUSH_TOKEN) and the per-user push
// entry read-back, per the ruling on issue #5004 (option B). Every case runs against the
// loopback fake forge in fake_test.go.

import (
	"strings"
	"testing"
)

// posts is every request body sent as "POST <api path>" (path after /api/v4).
func posts(f *fakeForge, apiPath string) []map[string]any {
	var out []map[string]any
	for _, r := range f.requests() {
		if r.Method == "POST" && r.Path == "/api/v4"+apiPath {
			out = append(out, r.Body)
		}
	}
	return out
}

// regression: #5004
func TestPushTokenProvisioned(t *testing.T) {
	f := newFakeForge(t)
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitOK {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, h.out.String(), h.err.String())
	}
	mints := posts(f, "/projects/900/access_tokens")
	if len(mints) != 1 {
		t.Fatalf("%d project access-token mint(s), want 1", len(mints))
	}
	m := mints[0]
	if sc, _ := m["scopes"].([]any); len(sc) != 1 || sc[0] != "write_repository" {
		t.Errorf("push token scopes = %v, want exactly [write_repository]", m["scopes"])
	}
	if lvl, _ := m["access_level"].(float64); lvl != 40 {
		t.Errorf("push token access_level = %v, want 40 (Maintainer)", m["access_level"])
	}
	if m["expires_at"] == nil || m["expires_at"] == "" {
		t.Error("push token minted without an expiry")
	}
	vars := posts(f, "/projects/900/variables")
	if len(vars) != 1 {
		t.Fatalf("%d variable write(s), want 1", len(vars))
	}
	v := vars[0]
	if v["key"] != "STATUSGEN_PUSH_TOKEN" || v["masked"] != true || v["protected"] != true {
		t.Errorf("variable = key %v masked %v protected %v, want STATUSGEN_PUSH_TOKEN masked+protected",
			v["key"], v["masked"], v["protected"])
	}
	if v["value"] != fakePushToken {
		t.Errorf("the variable does not hold the dedicated access token")
	}
	// Never a role-fleet PAT: the board-writer's carries `api`.
	for n := 1; n <= len(fleetRoles); n++ {
		if v["value"] == fakeToken(n) {
			t.Errorf("the variable holds role-fleet PAT #%d", n)
		}
	}
	// The board-writer stays Developer.
	for u, id := range f.accounts {
		if strings.Contains(u, "board-writer") && f.members[id] != 30 {
			t.Errorf("board-writer group access_level = %d, want 30 (Developer)", f.members[id])
		}
	}
	if strings.Contains(h.allText(t), fakePushToken) {
		t.Error("the push token's value escaped into the run's output")
	}
}

// regression: #5004
func TestPushTokenRefusedNoFallback(t *testing.T) {
	for _, st := range []int{403, 404} {
		f := newFakeForge(t)
		f.accessTokenStatus = st
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
			t.Fatalf("HTTP %d: exit %d, want %d", st, code, exitFailed)
		}
		if n := len(posts(f, "/projects/900/variables")); n != 0 {
			t.Errorf("HTTP %d: %d variable write(s) after a refused mint — no fallback credential may be stored", st, n)
		}
		out := h.out.String()
		if !strings.Contains(out, "STATUSGEN_PUSH_TOKEN") || !strings.Contains(out, "MANUAL STEP") {
			t.Errorf("HTTP %d: the summary does not name the manual step:\n%s", st, out)
		}
	}
}

// regression: #5004
func TestPushTokenWriteFailReports(t *testing.T) {
	f := newFakeForge(t)
	f.variablePostStatus = 400
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
		t.Fatalf("exit %d, want %d", code, exitFailed)
	}
	for _, r := range f.requests() {
		if r.Method == "DELETE" && strings.Contains(r.Path, "access_tokens") {
			t.Error("the minted token was auto-revoked (the partial-run ruling is report, never revoke)")
		}
	}
	if out := h.out.String(); !strings.Contains(out, "6001") || !strings.Contains(out, "NOT revoked") {
		t.Errorf("the summary does not name the live token by id:\n%s", out)
	}
	if strings.Contains(h.allText(t), fakePushToken) {
		t.Error("the push token's value escaped into the run's output")
	}
}

// regression: #5004
func TestPushTokenExistingNoop(t *testing.T) {
	f := newFakeForge(t)
	f.variables["STATUSGEN_PUSH_TOKEN"] = map[string]any{"key": "STATUSGEN_PUSH_TOKEN",
		"value": fakePushToken, "masked": true, "protected": true}
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitOK {
		t.Fatalf("exit %d\n%s", code, h.err.String())
	}
	if f.accessTokenMints != 0 {
		t.Errorf("an existing variable was re-minted (%d mint(s))", f.accessTokenMints)
	}
	if strings.Contains(h.allText(t), fakePushToken) {
		t.Error("the existing variable's value escaped into the run's output")
	}

	f = newFakeForge(t)
	f.variables["STATUSGEN_PUSH_TOKEN"] = map[string]any{"key": "STATUSGEN_PUSH_TOKEN",
		"value": fakePushToken, "masked": true, "protected": false}
	h = newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
		t.Fatalf("an unprotected existing variable: exit %d, want %d", code, exitFailed)
	}
}

// regression: #5004
func TestPushEntryReadback(t *testing.T) {
	f := newFakeForge(t) // premium: main's push rule names the board-writer
	f.dropPushUser = true
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
		t.Fatalf("exit %d, want %d — a dropped per-user push entry must be a recorded failure\n%s",
			code, exitFailed, h.out.String())
	}
	if !strings.Contains(h.out.String(), "absent from the read-back") {
		t.Fatalf("the summary does not name the absent push entry:\n%s", h.out.String())
	}
}

// The planted second instance of the defect class: the DECIDING read also looked at the
// first push entry only, so a rule whose second entry admits Maintainers read as intended.
// It is not intended, and on the free tier it cannot be re-created and then restored
// faithfully either (two role entries need the allowed_to_push array the scalar form cannot
// carry), so the run records a failure that names the extra entry and deletes nothing.
//
// regression: #5004
func TestPushEntriesAllRead(t *testing.T) {
	f := newFakeForge(t)
	f.plan = "free"
	f.rule = &fakeRule{push: 0, merge: mergeAccessLevel, extraPush: []map[string]any{{"access_level": 40}}}
	h := newHarness(t, f)
	code := run(h.provisionArgs("--project", fakeProject), h.e)
	if strings.Contains(h.out.String(), "already protected as intended") {
		t.Fatalf("a rule with a second push entry was read as intended:\n%s", h.out.String())
	}
	if code != exitFailed || !strings.Contains(h.out.String(), "role:0,role:40") ||
		!strings.Contains(h.out.String(), "nothing was deleted") {
		t.Fatalf("exit %d, want %d with the extra entry named and nothing deleted:\n%s", code, exitFailed, h.out.String())
	}
	if deletes, bodies := protectRequests(f); deletes != 0 || len(bodies) != 0 {
		t.Fatalf("the run wrote the rule (%d DELETE, %d POST) — it must refuse first", deletes, len(bodies))
	}
}
