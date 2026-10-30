package main

// pushentries_test.go — protected main's push entries are classified one by one (role, user,
// group, deploy key), every entry is checked, and anything this run cannot classify or
// cannot restore faithfully is refused with nothing written: the fail-closed half of the
// #5004 push-entry read. Also the push-token mint outcomes and the flag read-back that the
// fake used to echo back unchanged.

import (
	"encoding/json"
	"strings"
	"testing"
)

// predictedBoardWriterID is the id the fake assigns the board-writer: accounts are created in
// fleetRoles order from nextUserID 1000. Tests that need the id in a PRIOR rule use it, then
// check it with boardWriterIDMatches after the run.
func predictedBoardWriterID(t *testing.T) int64 {
	t.Helper()
	for i, r := range fleetRoles {
		if r.Role == "board-writer" {
			return int64(1000 + i + 1)
		}
	}
	t.Fatal("no board-writer role")
	return 0
}

func boardWriterIDMatches(t *testing.T, f *fakeForge, want int64) {
	t.Helper()
	if got := f.accounts[serviceAccountUsername(fakePrefix, "board-writer")]; got != want {
		t.Fatalf("board-writer id = %d, the fixture predicted %d", got, want)
	}
}

// protectRequests is every protected-branch write the run sent (POST and DELETE).
func protectRequests(f *fakeForge) (deletes int, bodies []map[string]any) {
	for _, r := range f.requests() {
		switch {
		case r.Method == "DELETE" && strings.HasSuffix(r.Path, "/protected_branches/main"):
			deletes++
		case r.Method == "POST" && strings.HasSuffix(r.Path, "/protected_branches"):
			bodies = append(bodies, r.Body)
		}
	}
	return deletes, bodies
}

// SEC-1 / F1: a deploy-key entry carries access_level 40 too. It was read as a Maintainer ROLE
// entry, so a refused re-create restored it as {access_level: 40} — every Maintainer could then
// push main — and the group entry was dropped. The rule must be left as it is instead.
//
// regression: #5004
func TestPushEntryKindsNotRestored(t *testing.T) {
	f := newFakeForge(t)
	f.rule = &fakeRule{merge: mergeAccessLevel, entries: []map[string]any{
		{"access_level": 40, "user_id": 17},
		{"access_level": 40, "deploy_key_id": 5},
		{"access_level": 40, "group_id": 9},
	}}
	f.refuseIntent = true // were the rule replaced, its re-create is refused and restore runs
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
		t.Fatalf("exit %d, want %d\n%s", code, exitFailed, h.out.String())
	}
	deletes, bodies := protectRequests(f)
	for _, b := range bodies {
		push, _ := json.Marshal(b["allowed_to_push"])
		if b["push_access_level"] != nil || strings.Contains(string(push), "access_level") {
			t.Errorf("a protected-branch write grants a ROLE push entry the rule never had: %v", b)
		}
	}
	if deletes != 0 || len(bodies) != 0 {
		t.Errorf("%d DELETE(s), %d POST(s) of protected main — a rule restore cannot reproduce must be left untouched",
			deletes, len(bodies))
	}
	if f.rule == nil || len(f.rule.entries) != 3 {
		t.Fatalf("the previous rule did not survive unchanged: %+v", f.rule)
	}
	out := h.out.String()
	for _, want := range []string{"deploy_key:5", "group:9", "nothing was deleted"} {
		if !strings.Contains(out, want) {
			t.Errorf("the summary does not name %q:\n%s", want, out)
		}
	}
}

// SEC-1: an entry with no field this run recognises (here none at all, and a principal kind it
// does not know) was dropped, so [board-writer, ?] read as "only the board-writer" and the run
// no-op'd over a rule it did not understand.
//
// regression: #5004
func TestPushEntryUnknownFailsClosed(t *testing.T) {
	bw := map[string]any{"access_level": 40, "user_id": predictedBoardWriterID(t)}
	for _, tc := range []struct {
		name, plan, want string
		entries          []map[string]any
	}{
		{"premium: no recognised field", "premium", "unknown", []map[string]any{bw, {"id": 3}}},
		{"premium: unknown principal", "premium", "unknown", []map[string]any{bw, {"access_level": 40, "member_role_id": 12}}},
		{"free: no recognised field", "free", "unknown", []map[string]any{{"id": 3}}},
		{"free: deploy key without a level", "free", "deploy_key:7", []map[string]any{{"deploy_key_id": 7}}},
		{"free: two principals on one entry", "free", "unknown", []map[string]any{{"access_level": 40, "user_id": 17, "group_id": 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			f.plan = tc.plan
			f.rule = &fakeRule{merge: mergeAccessLevel, entries: tc.entries}
			h := newHarness(t, f)
			code := run(h.provisionArgs("--project", fakeProject), h.e)
			boardWriterIDMatches(t, f, predictedBoardWriterID(t))
			if strings.Contains(h.out.String(), "already protected as intended") {
				t.Fatalf("a rule with an unrecognised push entry was read as intended:\n%s", h.out.String())
			}
			if code != exitFailed {
				t.Fatalf("exit %d, want %d\n%s", code, exitFailed, h.out.String())
			}
			if deletes, bodies := protectRequests(f); deletes != 0 || len(bodies) != 0 {
				t.Errorf("%d DELETE(s), %d POST(s) over a rule with an unrecognised entry", deletes, len(bodies))
			}
			if !strings.Contains(h.out.String(), tc.want) {
				t.Errorf("the summary does not name %q:\n%s", tc.want, h.out.String())
			}
		})
	}
}

// SEC-2: the read-back checked entry [0] and the board-writer's presence only, so an extra
// push entry the forge holds beside the intended one passed.
//
// regression: #5004
func TestPushReadbackRejectsExtra(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		extra      map[string]any
	}{
		{"premium: a Developer role beside the board-writer", "premium", map[string]any{"access_level": 30}},
		{"free: Maintainers beside No one", "free", map[string]any{"access_level": 40}},
		{"premium: a deploy key beside the board-writer", "premium", map[string]any{"access_level": 40, "deploy_key_id": 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			f.plan = tc.plan
			f.addPushOnWrite = []map[string]any{tc.extra}
			h := newHarness(t, f)
			if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
				t.Fatalf("exit %d, want %d — an extra push entry in the read-back must be a recorded failure\n%s",
					code, exitFailed, h.out.String())
			}
			if !strings.Contains(h.out.String(), "outside the intended set") {
				t.Fatalf("the summary does not name the extra push entry:\n%s", h.out.String())
			}
		})
	}
}

// SEC-1: admitsRole counted a deploy-key entry (access_level 40) as a Maintainer role, so the
// NOTICE that the regen push is refused was suppressed for a rule no Maintainer can push to.
//
// regression: #5004
func TestPushAdmissionRolesOnly(t *testing.T) {
	f := newFakeForge(t)
	f.plan = "free"
	f.addPushOnWrite = []map[string]any{{"access_level": 40, "deploy_key_id": 5}}
	h := newHarness(t, f)
	run(h.provisionArgs("--project", fakeProject), h.e)
	if !strings.Contains(h.out.String(), "does not admit a Maintainer") {
		t.Fatalf("a deploy-key entry was counted as a Maintainer role (no NOTICE):\n%s", h.out.String())
	}
}

// F2 (M8): a refused re-create restores EVERY role entry beside the user entry. The existing
// restore cases carry a single role entry, so dropping the role loop went unseen.
//
// regression: #5004
func TestRestoreKeepsUserAndRole(t *testing.T) {
	for _, tc := range []struct {
		name string
		rule fakeRule
		want []string
	}{
		{"user and role", fakeRule{push: 40, pushUser: 17, merge: 30, extraPush: []map[string]any{{"access_level": 30}}},
			[]string{`{"user_id":17}`, `{"access_level":30}`}},
		{"two roles", fakeRule{push: 40, merge: 30, extraPush: []map[string]any{{"access_level": 30}}},
			[]string{`{"access_level":40}`, `{"access_level":30}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			rule := tc.rule
			f.rule = &rule
			f.refuseIntent = true
			h := newHarness(t, f)
			if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
				t.Fatalf("exit %d, want %d\n%s", code, exitFailed, h.out.String())
			}
			_, bodies := protectRequests(f)
			if len(bodies) < 2 {
				t.Fatalf("%d protected-branch POST(s), want the refused one and the restore", len(bodies))
			}
			push, _ := json.Marshal(bodies[len(bodies)-1]["allowed_to_push"])
			for _, want := range tc.want {
				if !strings.Contains(string(push), want) {
					t.Errorf("the restore's allowed_to_push %s lacks %s", push, want)
				}
			}
			if !strings.Contains(h.out.String(), "restored to its PREVIOUS rule") {
				t.Errorf("the restore is not reported:\n%s", h.out.String())
			}
		})
	}
}

// F2 (M5) / SEC-6: a mint whose outcome is unknown (5xx, no reply) names the token that may
// exist; a 201 with no usable token is reported by id. None writes a variable.
//
// regression: #5004
func TestPushTokenMintOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*fakeForge)
		want string
	}{
		{"5xx", func(f *fakeForge) { f.accessTokenStatus = 500 }, "may EXIST"},
		{"502", func(f *fakeForge) { f.accessTokenStatus = 502 }, "may EXIST"},
		{"no reply", func(f *fakeForge) { f.accessTokenHangup = true }, "may EXIST"},
		{"201 without a token", func(f *fakeForge) { f.accessTokenBody = `{"id":6001}` }, "without a token value"},
		{"201 not JSON", func(f *fakeForge) { f.accessTokenBody = `not json` }, "without a token value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			tc.set(f)
			h := newHarness(t, f)
			if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
				t.Fatalf("exit %d, want %d", code, exitFailed)
			}
			if n := len(posts(f, "/projects/900/variables")); n != 0 {
				t.Errorf("%d variable write(s) after a failed mint", n)
			}
			out := h.out.String()
			if !strings.Contains(out, tc.want) || !strings.Contains(out, "MANUAL STEP") {
				t.Errorf("the summary does not say %q with the manual step:\n%s", tc.want, out)
			}
			if strings.Contains(out, "cannot be minted here") {
				t.Errorf("an unknown or partial outcome was reported as a definite refusal:\n%s", out)
			}
		})
	}
}

// A3 / SEC-6: every 4xx is a refusal with no variable write, but only 403/404 mean "this
// forge has no project access tokens"; a 400 or 401 has another cause and says so.
//
// regression: #5004
//
// regression: #5004
func TestPushTokenRefusalCause(t *testing.T) {
	for _, tc := range []struct {
		st       int
		want     string
		freeNote bool
	}{
		{403, "not available", true},
		{404, "not available", true},
		{400, "rejected the request", false},
		{401, "credential", false},
		{422, "refused the mint", false},
	} {
		f := newFakeForge(t)
		f.accessTokenStatus = tc.st
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
			t.Fatalf("HTTP %d: exit %d, want %d", tc.st, code, exitFailed)
		}
		if n := len(posts(f, "/projects/900/variables")); n != 0 {
			t.Errorf("HTTP %d: %d variable write(s) after a refused mint", tc.st, n)
		}
		out := h.out.String()
		if !strings.Contains(out, tc.want) || !strings.Contains(out, "MANUAL STEP") {
			t.Errorf("HTTP %d: the summary does not say %q with the manual step:\n%s", tc.st, tc.want, out)
		}
		if got := strings.Contains(out, "Free has none"); got != tc.freeNote {
			t.Errorf("HTTP %d: 'Free has none' present=%v, want %v:\n%s", tc.st, got, tc.freeNote, out)
		}
	}
}

// F2 (M4): the variable's flags are checked as the forge READS THEM BACK, not as they were
// sent. The fake used to echo the POST, so the mismatch branch never ran.
//
// regression: #5004
func TestPushTokenFlagMismatch(t *testing.T) {
	for _, flags := range []map[string]any{{"masked": false}, {"protected": false}, {"masked": "unknown-shape"}} {
		f := newFakeForge(t)
		f.variableReadback = flags
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
			t.Fatalf("flags %v: exit %d, want %d\n%s", flags, code, exitFailed, h.out.String())
		}
		if !strings.Contains(h.out.String(), "intended both true") {
			t.Errorf("flags %v: the mismatch is not named:\n%s", flags, h.out.String())
		}
	}
}
