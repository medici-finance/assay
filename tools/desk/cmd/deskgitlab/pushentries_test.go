package main

// pushentries_test.go — the twin of deskfleet's: protected main's push entries are classified
// one by one, every entry is checked, and anything this run cannot classify or restore
// faithfully is refused with nothing written. Also the push-token mint outcomes and the flag
// read-back. Every case runs against the loopback fake GitLab in fake_test.go.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	protectedMainPath = "/projects/7/protected_branches/main"
	protectPath       = "/projects/7/protected_branches"
	variablesPath     = "/projects/7/variables"
	pushVarPath       = variablesPath + "/STATUSGEN_PUSH_TOKEN"
	mintPath          = "/projects/7/access_tokens"
)

// ruleJSON is a protected-main answer holding entries, the JSON of its push_access_levels.
func ruleJSON(entries string, merge int) string {
	return fmt.Sprintf(`{"name":"main","push_access_levels":[%s],"merge_access_levels":[{"access_level":%d}],"allow_force_push":false}`, entries, merge)
}

// fixedRule answers every protected-main read with one rule; a re-create is refused once, or
// always when refuseAll, and a restore is otherwise accepted.
func fixedRule(rule string, refuseAll bool) responder {
	return chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET " + protectedMainPath:
			return 200, rule, true
		case "DELETE " + protectedMainPath:
			return 204, `{}`, true
		case "POST " + protectPath:
			if refuseAll || f.bump("protectpost") == 1 {
				return 400, `{"message":"boom"}`, true
			}
			return 201, `{}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
}

// writtenRule answers the first read with no rule, accepts the write, then reads back rule.
func writtenRule(rule string) responder {
	return chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET " + protectedMainPath:
			if f.bump("pbget") == 1 {
				return 404, `{"message":"404 Not found"}`, true
			}
			return 200, rule, true
		case "POST " + protectPath:
			return 201, `{}`, true
		}
		return 0, "", false
	}, mainProtectedAsIntended)
}

// boardWriterID is the id the fake assigns the board-writer: accounts are created in fleetRoles
// order from 101.
func boardWriterID(t *testing.T) int {
	t.Helper()
	for i, r := range fleetRoles {
		if r.Role == "board-writer" {
			return 100 + i + 1
		}
	}
	t.Fatal("no board-writer role")
	return 0
}

// A deploy-key entry carries access_level 40 too. It was read as a Maintainer ROLE entry, so a
// refused re-create restored it as {access_level: 40} — every Maintainer could then push main —
// and the group entry was dropped. The rule must be left as it is instead.
//
// regression: #5004
func TestPushEntryKindsNotRestored(t *testing.T) {
	f := newFakeGitLab(t)
	f.plan = `,"plan":"premium"`
	f.respond = fixedRule(ruleJSON(`{"access_level":40,"user_id":17},{"access_level":40,"deploy_key_id":5},{"access_level":40,"group_id":9}`, 40), true)
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if r.code == 0 {
		t.Fatalf("exit 0, want a recorded failure:\n%s", r.out)
	}
	if d, posts := f.count("DELETE "+protectedMainPath), f.bodies("POST", protectPath); d != 0 || len(posts) != 0 {
		t.Errorf("%d DELETE(s), %d POST(s) of protected main (%q) — a rule restore cannot reproduce must be left untouched", d, len(posts), posts)
	}
	for _, want := range []string{"deploy_key:5", "group:9", "nothing was deleted"} {
		if !r.has(want) {
			t.Errorf("the summary does not name %q:\n%s", want, r.out)
		}
	}
}

// An entry with no field this run recognises was dropped, so [board-writer, ?] read as "only
// the board-writer" and [?] read as "No one": the run no-op'd over a rule it did not understand.
//
// regression: #5004
func TestPushEntryUnknownFailsClosed(t *testing.T) {
	bw := fmt.Sprintf(`{"access_level":40,"user_id":%d}`, boardWriterID(t))
	for _, tc := range []struct{ name, plan, entries, want string }{
		{"premium: no recognised field", "premium", bw + `,{"id":3}`, "unknown"},
		{"premium: unknown principal", "premium", bw + `,{"access_level":40,"member_role_id":12}`, "unknown"},
		{"free: no recognised field", "free", `{"id":3}`, "unknown"},
		{"free: deploy key without a level", "free", `{"deploy_key_id":7}`, "deploy_key:7"},
		{"free: two principals on one entry", "free", `{"access_level":40,"user_id":17,"group_id":9}`, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitLab(t)
			f.plan = `,"plan":"` + tc.plan + `"`
			f.respond = fixedRule(ruleJSON(tc.entries, 40), true)
			r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
			if r.has("already protected as intended") {
				t.Fatalf("a rule with an unrecognised push entry was read as intended:\n%s", r.out)
			}
			if r.code == 0 {
				t.Fatalf("exit 0, want a recorded failure:\n%s", r.out)
			}
			if d, posts := f.count("DELETE "+protectedMainPath), f.bodies("POST", protectPath); d != 0 || len(posts) != 0 {
				t.Errorf("%d DELETE(s), %d POST(s) over a rule with an unrecognised entry", d, len(posts))
			}
			if !r.has(tc.want) {
				t.Errorf("the summary does not name %q:\n%s", tc.want, r.out)
			}
		})
	}
}

// The read-back checked entry [0] and the board-writer's presence only, so an extra push entry
// the forge holds beside the intended one passed, and never appeared in the printed line.
//
// regression: #5004
func TestPushReadbackRejectsExtra(t *testing.T) {
	bw := fmt.Sprintf(`{"access_level":40,"user_id":%d}`, boardWriterID(t))
	for _, tc := range []struct{ name, plan, entries, want string }{
		{"premium: a Developer role beside the board-writer", "premium", bw + `,{"access_level":30}`, "role:30"},
		{"free: Maintainers beside No one", "free", `{"access_level":0},{"access_level":40}`, "role:40"},
		{"premium: a deploy key beside the board-writer", "premium", bw + `,{"access_level":40,"deploy_key_id":5}`, "deploy_key:5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitLab(t)
			f.plan = `,"plan":"` + tc.plan + `"`
			f.respond = writtenRule(ruleJSON(tc.entries, 40))
			r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
			if r.code == 0 {
				t.Fatalf("exit 0 — an extra push entry in the read-back must be a recorded failure:\n%s", r.out)
			}
			if !r.has("outside the intended set") || !r.has(tc.want) {
				t.Fatalf("the summary does not name the extra entry %q:\n%s", tc.want, r.out)
			}
		})
	}
}

// admitsRole counted a deploy-key entry (access_level 40) as a Maintainer role, so the NOTICE
// that the regen push is refused was suppressed for a rule no Maintainer can push to.
//
// regression: #5004
func TestPushAdmissionRolesOnly(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = writtenRule(ruleJSON(`{"access_level":0},{"access_level":40,"deploy_key_id":5}`, 40))
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if !r.has("does not admit a Maintainer") {
		t.Fatalf("a deploy-key entry was counted as a Maintainer role (no NOTICE):\n%s", r.out)
	}
}

// A refused re-create restores EVERY entry of a rule it can reproduce: the existing restore
// case carries a single role entry, so dropping the role or user entries went unseen.
//
// regression: #5004
func TestRestoreKeepsUserAndRole(t *testing.T) {
	for _, tc := range []struct {
		name, entries string
		want          []string
	}{
		{"user and role", `{"access_level":40,"user_id":17},{"access_level":30}`, []string{`{"user_id":17}`, `{"access_level":30}`}},
		{"two roles", `{"access_level":40},{"access_level":30}`, []string{`{"access_level":40}`, `{"access_level":30}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitLab(t)
			f.plan = `,"plan":"premium"`
			f.respond = fixedRule(ruleJSON(tc.entries, 30), false)
			r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
			if r.code == 0 {
				t.Fatalf("exit 0, want a recorded failure:\n%s", r.out)
			}
			posts := f.bodies("POST", protectPath)
			if len(posts) != 2 {
				t.Fatalf("%d protected-branch POST(s), want the refused one and the restore: %q", len(posts), posts)
			}
			// Only the push array: the merge side carries an access_level of its own.
			var restore struct {
				Push json.RawMessage `json:"allowed_to_push"`
			}
			if err := json.Unmarshal([]byte(posts[1]), &restore); err != nil {
				t.Fatalf("the restore %s: %v", posts[1], err)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(restore.Push), want) {
					t.Errorf("the restore's allowed_to_push %s lacks %s", restore.Push, want)
				}
			}
			if !r.has("restored to its PREVIOUS rule") {
				t.Errorf("the restore is not reported:\n%s", r.out)
			}
		})
	}
}

// A mint whose outcome is unknown (5xx, no reply) names the token that may exist; a 201 with no
// usable token is reported by id. None writes a variable.
//
// regression: #5004
func TestPushTokenMintOutcomes(t *testing.T) {
	answers := func(st int, body string) func(*fakeGitLab) {
		return func(f *fakeGitLab) {
			f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
				if m+" "+p == "POST "+mintPath {
					return st, body, true
				}
				return 0, "", false
			}, mainProtectedAsIntended)
		}
	}
	for _, tc := range []struct {
		name string
		set  func(*fakeGitLab)
		want string
	}{
		{"5xx", answers(500, `{}`), "may EXIST"},
		{"502", answers(502, `{}`), "may EXIST"},
		{"no reply", func(f *fakeGitLab) { f.respond = mainProtectedAsIntended; f.dropPath = mintPath }, "may EXIST"},
		{"201 without a token", answers(201, `{"id":61}`), "without a token value"},
		{"201 not JSON", answers(201, `not json`), "without a token value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitLab(t)
			tc.set(f)
			r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
			if r.code == 0 {
				t.Fatalf("exit 0, want a recorded failure:\n%s", r.out)
			}
			if n := len(f.bodies("POST", variablesPath)); n != 0 {
				t.Errorf("%d variable write(s) after a failed mint", n)
			}
			if !r.has(tc.want) || !r.has("MANUAL STEP") {
				t.Errorf("the summary does not say %q with the manual step:\n%s", tc.want, r.out)
			}
			if r.has("not available on this forge") {
				t.Errorf("an unknown or partial outcome was reported as a definite refusal:\n%s", r.out)
			}
		})
	}
}

// Every 4xx is a refusal with no variable write, but only 403/404 mean "this forge has no
// project access tokens"; a 400 or 401 has another cause and says so.
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
		f := newFakeGitLab(t)
		f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "POST "+mintPath {
				return tc.st, `{"message":"refused"}`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if r.code == 0 {
			t.Fatalf("HTTP %d: exit 0, want a recorded failure:\n%s", tc.st, r.out)
		}
		if n := len(f.bodies("POST", variablesPath)); n != 0 {
			t.Errorf("HTTP %d: %d variable write(s) after a refused mint", tc.st, n)
		}
		if !r.has(tc.want) || !r.has("MANUAL STEP") {
			t.Errorf("HTTP %d: the summary does not say %q with the manual step:\n%s", tc.st, tc.want, r.out)
		}
		if got := r.has("Free has none"); got != tc.freeNote {
			t.Errorf("HTTP %d: 'Free has none' present=%v, want %v:\n%s", tc.st, got, tc.freeNote, r.out)
		}
	}
}

// The variable's flags are checked as the forge READS THEM BACK, not as they were sent: the
// fake used to echo the write, so the mismatch branch never ran.
//
// regression: #5004
func TestPushTokenFlagMismatch(t *testing.T) {
	for _, flags := range []string{`"masked":false,"protected":true`, `"masked":true,"protected":false`, `"masked":"odd","protected":true`} {
		f := newFakeGitLab(t)
		f.respond = chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "GET "+pushVarPath && f.pushVarSet {
				return 200, `{"key":"STATUSGEN_PUSH_TOKEN",` + flags + `}`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if r.code == 0 {
			t.Fatalf("flags %s: exit 0, want a recorded failure:\n%s", flags, r.out)
		}
		if !r.has("intended both true") {
			t.Errorf("flags %s: the mismatch is not named:\n%s", flags, r.out)
		}
	}
}
