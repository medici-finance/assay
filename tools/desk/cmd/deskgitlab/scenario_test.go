package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The provisioning scenarios, carried over case for case from the bash suite this command
// replaces (tools/create-fleet-gitlab_test.sh T1–T11, which drove the script with a fake curl
// on PATH). Each runs the real flow in process against the fake GitLab in fake_test.go.

// projectArgs is a live run with --project and no avatars, writing tokens under a temp dir.
func projectArgs(t *testing.T, extra ...string) []string {
	return append([]string{"--group", fakeGroup, "--prefix", fakePrefix, "--project", fakeProject,
		"--out-dir", t.TempDir(), "--no-avatars"}, extra...)
}

// T1 — a 400 from the protect POST re-applies the PREVIOUS rule, and the settings steps after
// it still run.
func TestFleetScenarioProtectFailureRestoresPreviousRule(t *testing.T) {
	f := newFakeGitLab(t)
	f.plan = `,"plan":"premium"`
	f.respond = func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET /projects/7/protected_branches/main":
			return 200, `{"name":"main","push_access_levels":[{"access_level":0}],"merge_access_levels":[{"access_level":30}],"allow_force_push":false}`, true
		case "DELETE /projects/7/protected_branches/main":
			return 204, `{}`, true
		case "POST /projects/7/protected_branches":
			if f.bump("protectpost") == 1 {
				return 400, `{"message":"boom"}`, true
			}
			return 201, `{}`, true
		}
		return mainProtectedAsIntended(f, m, p, "")
	}
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if !r.has("protection re-applied to its previous state") {
		t.Errorf("no re-apply reported:\n%s", r.out)
	}
	posts := f.bodies("POST", "/projects/7/protected_branches")
	if len(posts) < 2 || !strings.Contains(posts[len(posts)-1], `"merge_access_level":30`) {
		t.Errorf("the re-POST does not carry the previously read rule (merge_access_level 30): %q", posts)
	}
	if r.code == 0 {
		t.Errorf("a failed step exited 0")
	}
	if !r.has("configured: pipelines must succeed before merge") {
		t.Errorf("a failed protect step skipped the steps after it:\n%s", r.out)
	}
}

// T2 — --avatars-only uploads and mints NOTHING, as each role, with no token on the wire but
// the role's own header.
func TestFleetScenarioAvatarsOnly(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		if m+" "+p == "PUT /user/avatar" {
			return 200, `{"avatar_url":"/uploads/x.png"}`, true
		}
		return 500, `{"unstubbed":true}`, true
	}
	out, icons := t.TempDir(), t.TempDir()
	for _, r := range fleetRoles {
		tok := "TOKEN-PLACEHOLDER-" + r.Role
		if err := os.WriteFile(filepath.Join(out, fleetUsername(fakePrefix, r.Role)+".token"), []byte(tok), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(icons, r.Role+".png"), []byte("PNGFAKE"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	res := runAgainst(t, f, "", "--avatars-only", "--prefix", fakePrefix, "--out-dir", out, "--avatars-dir", icons)
	if res.code != 0 {
		t.Fatalf("--avatars-only exited %d:\n%s", res.code, res.out)
	}
	if n := f.count("POST "); n != 0 {
		t.Errorf("--avatars-only made %d POSTs (mint/create), want 0", n)
	}
	if n := f.count("PUT /user/avatar"); n != len(fleetRoles) {
		t.Errorf("--avatars-only uploaded %d avatars, want %d", n, len(fleetRoles))
	}
	if n := len(regexp.MustCompile(`(?m)^avatar: exorg-.*\(HTTP 200\)$`).FindAllString(res.out, -1)); n != len(fleetRoles) {
		t.Errorf("%d 'avatar: … (HTTP 200)' lines, want one per role:\n%s", n, res.out)
	}
	for _, q := range f.requests() {
		if !strings.HasPrefix(q.Token, "TOKEN-PLACEHOLDER-") || q.Token == fakeOwnerToken {
			t.Errorf("%s authenticated as %q, want the role's own token", q, q.Token)
		}
	}
	if strings.Contains(res.out, "TOKEN-PLACEHOLDER-reviewer") {
		t.Errorf("a role token was printed:\n%s", res.out)
	}
}

// T3 — unknown tier: the Premium form's 400 falls back to the free-tier form, and main ends up
// protected with merge_access_level 40.
func TestFleetScenarioTierFallback(t *testing.T) {
	f := newFakeGitLab(t)
	f.plan = ""
	f.respond = func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET /projects/7/protected_branches/main":
			if f.bump("pbget") == 1 {
				return 404, `{"message":"404 Not found"}`, true
			}
		case "POST /projects/7/protected_branches":
			if f.bump("protectpost") == 1 {
				return 400, `{"error":"allowed_to_unprotect[access_level] does not have a valid value"}`, true
			}
			return 201, `{}`, true
		}
		return mainProtectedAsIntended(f, m, p, "")
	}
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	if !r.has("free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)") {
		t.Errorf("the free-tier fallback is not reported as failed-at-tier:\n%s", r.out)
	}
	posts := f.bodies("POST", "/projects/7/protected_branches")
	if len(posts) != 2 || !regexp.MustCompile(`"push_access_level":0.*"merge_access_level":40`).MatchString(posts[1]) {
		t.Errorf("the fallback POST is not the free-tier form with merge_access_level 40: %q", posts)
	}
	if !r.has("read-back: main push_access_level=0 merge_access_level=40 allow_force_push=false") {
		t.Errorf("the three decided fields are not read back:\n%s", r.out)
	}
	if r.code != 0 {
		t.Errorf("a tier fallback exited %d, want 0", r.code)
	}
}

// T4 / T9d — a dry run enumerates seven accounts, seven avatar uploads and every label, and
// makes zero network calls.
func TestFleetScenarioDryRun(t *testing.T) {
	f := newFakeGitLab(t)
	r := runAgainst(t, f, "", "--dry-run", "--group", fakeGroup, "--prefix", fakePrefix, "--project", fakeProject)
	if r.code != 0 {
		t.Fatalf("dry run exited %d:\n%s", r.code, r.out)
	}
	if n := strings.Count(r.out, "would create service account"); n != len(fleetRoles) {
		t.Errorf("%d service accounts enumerated, want %d", n, len(fleetRoles))
	}
	if n := strings.Count(r.out, "would upload avatar for"); n != len(fleetRoles) {
		t.Errorf("%d avatar uploads enumerated, want %d", n, len(fleetRoles))
	}
	for _, l := range []string{"authorization-needed", "approval-needed"} {
		if !r.has("would create project label '" + l + "'") {
			t.Errorf("dry run does not enumerate label %s", l)
		}
	}
	if n := f.calls(); n != 0 {
		t.Errorf("dry run made %d network calls, want 0", n)
	}
}

// T5 — approvals: a 201 whose read-back did not take is reported, not trusted.
func TestFleetScenarioApprovalsReadBack(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch m + " " + p {
		case "GET /projects/7/protected_branches/main":
			if f.bump("pbget") == 1 {
				return 404, `{"message":"404 Not found"}`, true
			}
		case "POST /projects/7/protected_branches":
			return 201, `{}`, true
		case "GET /projects/7/approvals":
			return 200, `{"merge_requests_author_approval":true,"merge_requests_disable_committers_approval":false,"merge_request_approvers_available":false}`, true
		}
		return mainProtectedAsIntended(f, m, p, "")
	}
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	for _, want := range []string{
		"approval settings ignored at this tier (failed-at-tier, remediation: Premium)",
		"do not count approvals as a server-enforced gate on this tier",
	} {
		if !r.has(want) {
			t.Errorf("missing %q:\n%s", want, r.out)
		}
	}
	if f.count("GET /projects/7/approvals") == 0 {
		t.Errorf("approvals are not read back after the write")
	}
}

// T6 — default avatars: the public role icons are fetched and self-uploaded; an icon that
// cannot be fetched is a NOTICE, not a failure.
func TestFleetScenarioDefaultAvatars(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // the icon download directory is created, and removed, here
	f := newFakeGitLab(t)
	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", t.TempDir())
	if f.icons != len(fleetRoles) || f.count("PUT /user/avatar") != len(fleetRoles) {
		t.Errorf("icons fetched %d, avatars uploaded %d; want %d each\n%s", f.icons, f.count("PUT /user/avatar"), len(fleetRoles), r.out)
	}
	for _, q := range f.requests() {
		if q.String() == "PUT /user/avatar" && q.Token != fakeMinted {
			t.Errorf("an avatar was uploaded as %q, want the account's own minted PAT", q.Token)
		}
	}
	if r.code != 0 {
		t.Errorf("default avatar run exited %d:\n%s", r.code, r.out)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "fleet-gitlab-avatars.*")); len(left) != 0 {
		t.Errorf("the icon download directory outlived the run: %v", left)
	}

	t.Run("unfetchable-icon", func(t *testing.T) {
		f := newFakeGitLab(t)
		f.iconFail = true
		r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", t.TempDir())
		if !r.has("could not fetch the default icon") || !r.has("skipped, not a failure") || r.code != 0 {
			t.Errorf("an unfetchable icon is not a NOTICE (exit %d):\n%s", r.code, r.out)
		}
	})
}

// T7 / T7b — protected release tags use the SCALAR create_access_level (Free tier), never the
// Premium allowed_to_create array; an existing rule is a named no-op.
func TestFleetScenarioProtectedTags(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	posts := f.bodies("POST", "/projects/7/protected_tags")
	if len(posts) != 1 || !strings.Contains(posts[0], `"create_access_level":40`) || strings.Contains(posts[0], "allowed_to_create") {
		t.Errorf("protected-tags POST is not the scalar create_access_level=40 form: %q", posts)
	}
	if !r.has("protected: tags '*' (create_access_level=40 Maintainers)") {
		t.Errorf("the protected-tags decision is not printed:\n%s", r.out)
	}
	if r.code != 0 {
		t.Errorf("clean run exited %d:\n%s", r.code, r.out)
	}

	t.Run("existing-rule-is-a-no-op", func(t *testing.T) {
		f := newFakeGitLab(t)
		f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "GET /projects/7/protected_tags" {
				return 200, `[{"name":"*","create_access_level":40}]`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if n := f.count("POST /projects/7/protected_tags"); n != 0 || !r.has("already protected") {
			t.Errorf("an existing rule: %d POSTs, want 0 and a named no-op:\n%s", n, r.out)
		}
	})
}

// T8 — merge checks: the PUT sets the discussions gate beside the pipeline gate, and both are
// read back and printed.
func TestFleetScenarioMergeChecks(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	puts := f.bodies("PUT", "/projects/7")
	if len(puts) != 1 || !strings.Contains(puts[0], `"only_allow_merge_if_all_discussions_are_resolved":true`) {
		t.Errorf("merge-checks PUT: %q", puts)
	}
	for _, want := range []string{
		"all discussions must be resolved before merge",
		"read-back: only_allow_merge_if_pipeline_succeeds=true only_allow_merge_if_all_discussions_are_resolved=true",
	} {
		if !r.has(want) {
			t.Errorf("missing %q:\n%s", want, r.out)
		}
	}
	if f.count("GET /projects/7") == 0 {
		t.Errorf("the project settings are not read back")
	}
}

// T9 / T9b / T9c — labels: every row created with GitLab's leading-# color; a duplicate is a
// named no-op; any other refusal is recorded without aborting the run.
func TestFleetScenarioLabels(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	posts := f.bodies("POST", "/projects/7/labels")
	if len(posts) != len(fleetLabels) {
		t.Errorf("%d label POSTs, want %d", len(posts), len(fleetLabels))
	}
	joined := strings.Join(posts, "\n")
	for _, want := range []string{
		`"name":"authorization-needed","color":"#FBCA04"`,
		`"name":"approval-needed","color":"#5319E7"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("no label body carries %s:\n%s", want, joined)
		}
	}
	if !r.has("label: created 'authorization-needed'") || !r.has("label: created 'approval-needed'") || r.code != 0 {
		t.Errorf("label creates not reported, or exit %d:\n%s", r.code, r.out)
	}

	t.Run("duplicate-is-a-no-op", func(t *testing.T) {
		f := newFakeGitLab(t)
		f.respond = chain(func(f *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "POST /projects/7/labels" {
				if f.bump("labelpost") == 1 {
					return 409, `{"message":["Label already exists"]}`, true
				}
				return 400, `{"message":{"title":["has already been taken"]}}`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if !r.has("already exists (no-op)") || r.code != 0 || r.has("not created") {
			t.Errorf("a duplicate label is not a no-op (exit %d):\n%s", r.code, r.out)
		}
	})

	t.Run("forbidden-is-recorded-not-fatal", func(t *testing.T) {
		f := newFakeGitLab(t)
		f.respond = chain(func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
			if m+" "+p == "POST /projects/7/labels" {
				return 403, `{"message":"403 Forbidden"}`, true
			}
			return 0, "", false
		}, mainProtectedAsIntended)
		r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
		if !r.has("label 'authorization-needed' not created (HTTP 403)") || r.code == 0 {
			t.Errorf("a forbidden label is not recorded with a non-zero exit (exit %d):\n%s", r.code, r.out)
		}
		if !r.has("configured: pipelines must succeed before merge") {
			t.Errorf("a failed label step aborted the run:\n%s", r.out)
		}
	})
}

// T10 — owner-PAT custody (issue #786): the owner credential travels only as the
// PRIVATE-TOKEN header — never in a URL, a body or any output — and a minted PAT is written to
// its 0600 file, never printed.
func TestFleetScenarioCredentialCustody(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	out := filepath.Join(t.TempDir(), "tokens") // created by the run
	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--project", fakeProject, "--out-dir", out, "--no-avatars")
	if r.code != 0 {
		t.Fatalf("custody run exited %d:\n%s", r.code, r.out)
	}
	reqs := f.requests()
	if len(reqs) == 0 {
		t.Fatal("no request reached the fake")
	}
	for _, q := range reqs {
		if q.Token != fakeOwnerToken {
			t.Errorf("%s sent PRIVATE-TOKEN %q, want the owner credential", q, q.Token)
		}
		if strings.Contains(q.Path, "TOKEN-PLACEHOLDER") || strings.Contains(q.Body, "TOKEN-PLACEHOLDER") {
			t.Errorf("%s carried a credential in its path or body", q)
		}
	}
	if strings.Contains(r.out, "TOKEN-PLACEHOLDER") {
		t.Errorf("a credential reached the output:\n%s", r.out)
	}
	file := filepath.Join(out, fleetUsername(fakePrefix, "reviewer")+".token")
	fi, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("token file mode %v, want 0600", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(file); string(b) != fakeMinted {
		t.Errorf("token file holds %q, want the minted value", b)
	}
	if d, err := os.Stat(out); err != nil || d.Mode().Perm() != 0o700 {
		t.Errorf("the --out-dir the run created is not owner-only: %v", d)
	}
}

// T11 — transport safety (issue #786): a request that gets no answer is RECORDED as HTTP 000,
// the later settings steps still run, and the summary surfaces the failure before a non-zero
// exit.
func TestFleetScenarioTransportFailureIsRecorded(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	f.dropPath = "/projects/7/approvals"
	r := runAgainst(t, f, fakeOwnerToken, projectArgs(t)...)
	for _, want := range []string{
		"approval settings write failed (HTTP 000",
		"configured: pipelines must succeed before merge",
		"FAILED STEPS — this run did NOT complete cleanly",
		"HUMAN-ONLY REMAINDER",
	} {
		if !r.has(want) {
			t.Errorf("missing %q:\n%s", want, r.out)
		}
	}
	if r.code == 0 {
		t.Errorf("a recorded failure exited 0")
	}
}

// A fatal answer the run cannot continue past (the group cannot be resolved) ends it with
// exit 1 and the server's answer on stderr, before any account is touched.
func TestFleetScenarioUnresolvableGroupIsFatal(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
		if m+" "+p == "GET /groups/"+fakeGroup {
			return 404, `{"message":"404 Group Not Found"}`, true
		}
		return 0, "", false
	}
	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", t.TempDir(), "--no-avatars")
	if r.code != 1 || !r.has("error: could not resolve group 'example' (HTTP 404)") || !r.has("404 Group Not Found") {
		t.Errorf("unresolvable group: exit %d:\n%s", r.code, r.out)
	}
	if n := f.count("POST "); n != 0 {
		t.Errorf("%d writes after a fatal group lookup, want 0", n)
	}
}

// Idempotency: a re-run over accounts that already exist, as members at their level, creates
// and mints nothing and names each no-op.
func TestFleetScenarioRerunIsIdempotent(t *testing.T) {
	f := newFakeGitLab(t)
	f.respond = func(_ *fakeGitLab, m, p, _ string) (int, string, bool) {
		switch {
		case m+" "+p == "GET /groups/1/service_accounts":
			var b strings.Builder
			b.WriteString("[")
			for i, r := range fleetRoles {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(`{"id":` + itoa(200+i) + `,"username":"` + fleetUsername(fakePrefix, r.Role) + `"}`)
			}
			b.WriteString("]")
			return 200, b.String(), true
		case m == "GET" && strings.HasPrefix(p, "/groups/1/members/"):
			id := strings.TrimPrefix(p, "/groups/1/members/")
			for i, r := range fleetRoles {
				if id == itoa(200+i) {
					return 200, `{"access_level":` + r.AccessLevel + `}`, true
				}
			}
		}
		return 0, "", false
	}
	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", t.TempDir(), "--no-avatars")
	if r.code != 0 {
		t.Fatalf("re-run exited %d:\n%s", r.code, r.out)
	}
	if n := f.count("POST "); n != 0 {
		t.Errorf("a re-run made %d POSTs, want 0", n)
	}
	if n := strings.Count(r.out, "no-op: service account"); n != len(fleetRoles) {
		t.Errorf("%d named account no-ops, want %d:\n%s", n, len(fleetRoles), r.out)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
