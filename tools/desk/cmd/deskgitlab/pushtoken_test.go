package main

// pushtoken_test.go — the board-push credential (STATUSGEN_PUSH_TOKEN) and the per-user push
// entry read-back, per the ruling on issue #5004 (option B). The twin of deskfleet's cases;
// every one runs against the loopback fake GitLab in fake_test.go.

import (
	"strings"
	"testing"
)

// regression: #5004
func TestPushTokenProvisioned(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	mints := f.bodies("POST", "/projects/7/access_tokens")
	if len(mints) != 1 {
		t.Fatalf("%d project access-token mint(s), want 1", len(mints))
	}
	for _, want := range []string{`"scopes":["write_repository"]`, `"access_level":40`, `"expires_at":"`} {
		if !strings.Contains(mints[0], want) {
			t.Errorf("mint body %s lacks %s", mints[0], want)
		}
	}
	vars := f.bodies("POST", "/projects/7/variables")
	if len(vars) != 1 {
		t.Fatalf("%d variable write(s), want 1", len(vars))
	}
	for _, want := range []string{`"key":"STATUSGEN_PUSH_TOKEN"`, `"value":"` + fakePushToken + `"`,
		`"masked":true`, `"protected":true`} {
		if !strings.Contains(vars[0], want) {
			t.Errorf("variable body lacks %s", want)
		}
	}
	if strings.Contains(vars[0], fakeMinted) {
		t.Error("the variable holds a role-fleet PAT")
	}
	if strings.Contains(r.out, fakePushToken) {
		t.Error("the push token's value reached the output")
	}
}

// regression: #5004
func TestPushTokenRefusedNoFallback(t *testing.T) {
	for _, st := range []int{403, 404} {
		f := newFakeGitLab(t)
		f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "POST /projects/7/access_tokens" {
				return st, `{"message":"refused"}`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if r.code == 0 {
			t.Fatalf("HTTP %d: exit 0, want a recorded failure:\n%s", st, r.out)
		}
		if n := len(f.bodies("POST", "/projects/7/variables")); n != 0 {
			t.Errorf("HTTP %d: %d variable write(s) after a refused mint — no fallback credential may be stored", st, n)
		}
		if !r.has("STATUSGEN_PUSH_TOKEN") || !r.has("MANUAL STEP") {
			t.Errorf("HTTP %d: the summary does not name the manual step:\n%s", st, r.out)
		}
	}
}

// regression: #5004
func TestPushTokenWriteFailReports(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
		if m+" "+p == "POST /projects/7/variables" {
			return 400, `{"message":{"value":["is invalid"]}}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.code == 0 {
		t.Fatalf("exit 0, want a recorded failure:\n%s", r.out)
	}
	if n := f.count("DELETE /projects/7/access_tokens"); n != 0 {
		t.Error("the minted token was auto-revoked (the partial-run ruling is report, never revoke)")
	}
	if !r.has("id 61") || !r.has("NOT revoked") {
		t.Errorf("the summary does not name the live token by id:\n%s", r.out)
	}
	if strings.Contains(r.out, fakePushToken) {
		t.Error("the push token's value reached the output")
	}
}

// regression: #5004
func TestPushTokenExistingNoop(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	f.pushVarSet = true
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.out)
	}
	if n := f.count("POST /projects/7/access_tokens"); n != 0 {
		t.Errorf("an existing variable was re-minted (%d mint(s))", n)
	}
	if strings.Contains(r.out, fakePushToken) {
		t.Error("the existing variable's value reached the output")
	}

	f = newFakeGitLab(t)
	f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
		if m+" "+p == "GET /projects/7/variables/STATUSGEN_PUSH_TOKEN" {
			return 200, `{"key":"STATUSGEN_PUSH_TOKEN","masked":true,"protected":false}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
	if r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...); r.code == 0 {
		t.Fatalf("an unprotected existing variable: exit 0, want a recorded failure:\n%s", r.out)
	}
}

// regression: #5004
func TestPushEntryReadback(t *testing.T) {
	f := newFakeGitLab(t)
	f.plan = `,"plan":"premium"`
	f.respond = chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET /projects/7/protected_branches/main":
			if f.bump("pbget") == 1 {
				return 404, `{"message":"404 Not found"}`, true
			}
			// The forge answered the per-user write 201, then dropped the entry.
			return 200, `{"name":"main","push_access_levels":[{"access_level":40}],"merge_access_levels":[{"access_level":40}],"allow_force_push":false}`, true
		case "POST /projects/7/protected_branches":
			return 201, `{}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.code == 0 {
		t.Fatalf("exit 0 — a dropped per-user push entry must be a recorded failure:\n%s", r.out)
	}
	if !r.has("absent from the read-back") {
		t.Fatalf("the summary does not name the absent push entry:\n%s", r.out)
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
	f := newFakeGitLab(t)
	f.respond = chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET /projects/7/protected_branches/main":
			if f.bump("pbget") == 1 {
				return 200, `{"name":"main","push_access_levels":[{"access_level":0},{"access_level":40}],"merge_access_levels":[{"access_level":40}],"allow_force_push":false}`, true
			}
		case "DELETE /projects/7/protected_branches/main":
			return 204, `{}`, true
		case "POST /projects/7/protected_branches":
			return 201, `{}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.has("already protected as intended") {
		t.Fatalf("a rule with a second push entry was read as intended:\n%s", r.out)
	}
	if r.code == 0 || !r.has("role:0,role:40") || !r.has("nothing was deleted") {
		t.Fatalf("exit %d, want a recorded failure naming the extra entry with nothing deleted:\n%s", r.code, r.out)
	}
	if d, posts := f.count("DELETE "+protectedMainPath), f.bodies("POST", protectPath); d != 0 || len(posts) != 0 {
		t.Fatalf("the run wrote the rule (%d DELETE, %d POST) — it must refuse first", d, len(posts))
	}
}
