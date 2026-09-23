package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Brief row 3: --dry-run makes ZERO network calls — with --project too, and for both
// forges' label dry-runs. The transport FAILS the test on any request.
func TestFleetDryRunMakesNoNetworkCalls(t *testing.T) {
	h := newHarness(t, nil)
	ft := &failingTransport{t: t}
	h.e.http = &http.Client{Transport: ft}
	h.apiBase = "https://gitlab.example.com/api/v4"

	for _, args := range [][]string{
		h.provisionArgs("--project", fakeProject, "--dry-run"),
		h.provisionArgs("--project", fakeProject, "--avatars-dir", h.dir, "--dry-run"),
		{"provision", "--avatars-only", "--avatars-dir", h.dir, "--prefix", fakePrefix, "--out-dir", h.dir, "--dry-run"},
		{"labels", "--forge", "gitlab", "--project", fakeProject, "--token-file", h.ownerFile, "--dry-run"},
		{"labels", "--forge", "github", "--repo", "example-org/example-repo", "--token-file", h.ownerFile, "--dry-run"},
	} {
		if code := run(args, h.e); code != exitOK {
			t.Fatalf("%v: exit %d, want 0\nstderr:\n%s", args, code, h.err.String())
		}
	}
	if ft.calls != 0 {
		t.Fatalf("dry-run reached the transport %d time(s)", ft.calls)
	}
	if ents, _ := os.ReadDir(h.dir); len(ents) != 0 {
		t.Fatalf("dry-run wrote %d file(s) into the out dir", len(ents))
	}
}

// Brief row 4 (+ row 15's independent copy): the dry run names all seven roles with the
// access level and scopes of the script's ROLE_TABLE, field for field.
func TestFleetDryRunEnumeratesRoleTable(t *testing.T) {
	// An INDEPENDENT copy of tools/create-fleet-gitlab.sh's ROLE_TABLE — not derived from
	// fleetRoles — so a widened scope in tables.go fails here.
	want := []string{
		"reviewer:developer:30:api",
		"worker:developer:30:api,write_repository",
		"verifier:developer:30:api,write_repository",
		"desk:developer:30:api",
		"issue-loop:reporter:20:api",
		"intake-loop:reporter:20:api",
		"board-writer:developer:30:api,write_repository",
	}
	h := newHarness(t, nil)
	h.e.http = &http.Client{Transport: &failingTransport{t: t}}
	if code := run(h.provisionArgs("--dry-run"), h.e); code != exitOK {
		t.Fatalf("exit %d\n%s", code, h.err.String())
	}
	re := regexp.MustCompile(`would create service account example-([a-z-]+)-bot \(role=([a-z-]+), access=([a-z]+) \((\d+)\), scopes=([a-z_,]+)\)`)
	var got []string
	for _, m := range re.FindAllStringSubmatch(h.out.String(), -1) {
		if m[1] != m[2] {
			t.Errorf("username role %q != role %q", m[1], m[2])
		}
		got = append(got, m[2]+":"+m[3]+":"+m[4]+":"+m[5])
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("dry-run role table:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, r := range want {
		role := strings.SplitN(r, ":", 2)[0]
		if !strings.Contains(h.out.String(), "would mint a PAT "+patName(role)) {
			t.Errorf("dry-run does not name the PAT for %s", role)
		}
	}
}

// Brief row 5: a real run with GITLAB_API_BASE unset refuses, naming the variable, before
// the transport is ever reached — for provision and for the GitLab label path.
func TestFleetRefusesWithoutAPIBase(t *testing.T) {
	for _, argsFn := range []func(h *harness) []string{
		func(h *harness) []string { return h.provisionArgs("--project", fakeProject) },
		func(h *harness) []string {
			return []string{"labels", "--forge", "gitlab", "--project", fakeProject, "--token-file", h.ownerFile}
		},
	} {
		h := newHarness(t, nil)
		ft := &failingTransport{t: t}
		h.e.http = &http.Client{Transport: ft}
		h.apiBase = ""
		code := run(argsFn(h), h.e)
		if code != exitRefused {
			t.Fatalf("exit %d, want %d (refused)\n%s", code, exitRefused, h.err.String())
		}
		if !strings.Contains(h.err.String(), "GITLAB_API_BASE") {
			t.Fatalf("refusal does not name GITLAB_API_BASE:\n%s", h.err.String())
		}
		if ft.calls != 0 {
			t.Fatalf("the transport was reached %d time(s) before the refusal", ft.calls)
		}
	}
}

// Brief row 6: across a full run AND a partial run, no token value — the owner's or any
// minted one — appears in stdout, stderr, the partial-run report, the process environment,
// or an exec argv (the package builds none: it imports no os/exec).
func TestFleetTokenNeverEscapes(t *testing.T) {
	sentinels := []string{fakeOwnerToken}
	for n := 1; n <= 7; n++ {
		sentinels = append(sentinels, fakeToken(n))
	}
	check := func(t *testing.T, h *harness) {
		t.Helper()
		text := h.allText(t)
		for _, s := range sentinels {
			if strings.Contains(text, s) {
				t.Errorf("token value %q escaped into the run's output or report", s)
			}
		}
		for _, kv := range os.Environ() {
			for _, s := range sentinels {
				if strings.Contains(kv, s) {
					t.Errorf("token value %q is in the process environment", s)
				}
			}
		}
	}
	t.Run("full run with project", func(t *testing.T) {
		f := newFakeForge(t)
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitOK {
			t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, h.out.String(), h.err.String())
		}
		check(t, h)
	})
	t.Run("partial run", func(t *testing.T) {
		f := newFakeForge(t)
		f.failMintAt = 4
		h := newHarness(t, f)
		if code := run(h.provisionArgs(), h.e); code == exitOK {
			t.Fatal("partial run exited 0")
		}
		check(t, h)
	})
	t.Run("the credential rides a header, never the URL", func(t *testing.T) {
		f := newFakeForge(t)
		h := newHarness(t, f)
		run(h.provisionArgs("--project", fakeProject), h.e)
		for _, r := range f.requests() {
			if r.Auth != fakeOwnerToken {
				t.Errorf("%s %s carried an unexpected credential header", r.Method, r.Path)
			}
			for _, s := range sentinels {
				if strings.Contains(r.Path, s) {
					t.Errorf("token value in request path %s", r.Path)
				}
			}
		}
	})
	t.Run("no subprocess is ever built", func(t *testing.T) {
		fset := token.NewFileSet()
		files, _ := filepath.Glob("*.go")
		for _, fn := range files {
			if strings.HasSuffix(fn, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(fset, fn, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", fn, err)
			}
			for _, im := range af.Imports {
				if im.Path.Value == `"os/exec"` || im.Path.Value == `"syscall"` {
					t.Errorf("%s imports %s — a credential could reach a subprocess argv or env", fn, im.Path.Value)
				}
			}
		}
	})
}

// Brief row 10: the file written for role <r> is gitlab-<r>.token, the name desktoken
// --forge gitlab <role> reads, and it holds exactly the minted value.
func TestFleetWritesGitlabRoleTokenName(t *testing.T) {
	f := newFakeForge(t)
	h := newHarness(t, f)
	if code := run(h.provisionArgs(), h.e); code != exitOK {
		t.Fatalf("exit %d\n%s", code, h.err.String())
	}
	for i, r := range fleetRoles {
		p := filepath.Join(h.dir, "gitlab-"+r.Role+".token")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("role %s: %v", r.Role, err)
		}
		if string(b) != fakeToken(i+1) {
			t.Errorf("role %s: file holds the wrong credential", r.Role)
		}
	}
	ents, _ := os.ReadDir(h.dir)
	if len(ents) != len(fleetRoles) {
		var names []string
		for _, en := range ents {
			names = append(names, en.Name())
		}
		t.Fatalf("out dir holds %v, want exactly the seven gitlab-<role>.token files", names)
	}
}

// The ruling's WARN arm (option 2): an INCONCLUSIVE read-back names the file in a prominent
// warning, repeats it in the closing summary, and the run continues to completion.
func TestFleetWarnsOnInconclusiveCustody(t *testing.T) {
	f := newFakeForge(t)
	h := newHarness(t, f)
	inconclusiveRole := "verifier"
	h.e.classifyCustody = func(path string) deskkit.CustodyVerdict {
		if filepath.Base(path) == tokenFileName(inconclusiveRole) {
			return deskkit.CustodyVerdict{State: deskkit.CustodyInconclusive,
				Err: errors.New("cannot read the Windows owner/DACL (simulated)")}
		}
		return deskkit.ClassifyCustodyOwnerOnly(path)
	}
	code := run(h.provisionArgs(), h.e)
	if code != exitOK {
		t.Fatalf("an inconclusive read-back must WARN and continue, got exit %d\n%s", code, h.err.String())
	}
	file := filepath.Join(h.dir, tokenFileName(inconclusiveRole))
	if !strings.Contains(h.err.String(), "WARNING") || !strings.Contains(h.err.String(), file) {
		t.Fatalf("no prominent WARNING naming %s:\n%s", file, h.err.String())
	}
	if !strings.Contains(h.out.String(), "CUSTODY WARNINGS") || !strings.Contains(h.out.String(), file) {
		t.Fatalf("the closing summary does not repeat the custody warning:\n%s", h.out.String())
	}
	if f.mints != 7 {
		t.Fatalf("run stopped after %d mints; option 2 continues past an inconclusive read-back", f.mints)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the warned token file is gone: %v", err)
	}
}

// Brief row 11 + the dispatch's required row: a run forced to fail after N of 7 mints STOPS,
// reports EXACTLY those N tokens (by role, account, token name and id — never a value), exits
// non-zero, and performs NO revocation. N = 0 included: the account created with no token is
// still reported, and "nothing to revoke" is never claimed for a run that created an account.
func TestFleetPartialRun(t *testing.T) {
	for _, n := range []int{0, 1, 3, 6} {
		t.Run(fmt.Sprintf("fails after %d of 7", n), func(t *testing.T) {
			f := newFakeForge(t)
			f.failMintAt = n + 1
			f.failMintStatus = 403 // a DEFINITE refusal: no token exists for the failing account
			h := newHarness(t, f)
			code := run(h.provisionArgs("--project", fakeProject), h.e)
			if code == exitOK {
				t.Fatal("a partial run exited 0")
			}
			if f.mints != n+1 {
				t.Fatalf("mint attempts = %d, want %d — the run must STOP at the failing role", f.mints, n+1)
			}
			reports, _ := filepath.Glob(filepath.Join(h.dir, "deskfleet-partial-run-*.txt"))
			if len(reports) != 1 {
				t.Fatalf("want exactly one partial-run report file, got %v", reports)
			}
			rb, _ := os.ReadFile(reports[0])
			report := string(rb)
			if !strings.Contains(h.out.String(), report) {
				t.Errorf("the report file and the printed report differ")
			}
			lineRE := regexp.MustCompile(`(?m)^  - role=([a-z-]+) account=(\S+) \(user id \d+\) token=(\S+) \(token id (\d+)\)`)
			var roles, ids []string
			for _, m := range lineRE.FindAllStringSubmatch(report, -1) {
				roles = append(roles, m[1])
				ids = append(ids, m[4])
				if m[2] != serviceAccountUsername(fakePrefix, m[1]) || m[3] != patName(m[1]) {
					t.Errorf("report line names account %s token %s for role %s", m[2], m[3], m[1])
				}
			}
			var wantRoles, wantIDs []string
			for i := 0; i < n; i++ {
				wantRoles = append(wantRoles, fleetRoles[i].Role)
				wantIDs = append(wantIDs, strconv.Itoa(5000+i+1))
			}
			if strings.Join(roles, ",") != strings.Join(wantRoles, ",") || strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
				t.Fatalf("report names roles %v ids %v, want EXACTLY %v ids %v\n%s", roles, ids, wantRoles, wantIDs, report)
			}
			orphan := serviceAccountUsername(fakePrefix, fleetRoles[n].Role)
			if !strings.Contains(report, "Account "+orphan+" was CREATED by this run but holds NO token") {
				t.Errorf("report does not name %s, created by the run with no token\n%s", orphan, report)
			}
			if strings.Contains(h.allText(t), "nothing to revoke") {
				t.Errorf("a run that created an account claims there is nothing to revoke")
			}
			for i := 1; i <= 7; i++ {
				if strings.Contains(h.allText(t), fakeToken(i)) {
					t.Errorf("token value %d appears in the output or report", i)
				}
			}
			for _, r := range f.requests() {
				if r.Method == "DELETE" || strings.Contains(r.Path, "/revoke") ||
					strings.Contains(r.Path, "/rotate") || strings.HasSuffix(r.Path, "/protected_branches") {
					t.Errorf("a partial run made %s %s — it must revoke nothing and touch no settings", r.Method, r.Path)
				}
			}
			for i := n + 1; i < len(fleetRoles); i++ {
				if _, err := os.Stat(filepath.Join(h.dir, tokenFileName(fleetRoles[i].Role))); err == nil {
					t.Errorf("a token file exists for %s, which the stopped run never reached", fleetRoles[i].Role)
				}
			}
		})
	}
	// A mint whose OUTCOME is unknown — a server error, or a connection dropped with no reply —
	// may have created a token. It is reported as could-not-check, naming the account and the
	// token to look for; never as "nothing to revoke", never as definitely tokenless.
	for _, tc := range []struct {
		name   string
		n      int
		status int
		hangup bool
	}{
		{"server error on the first mint", 0, 500, false},
		{"server error after 2 of 7", 2, 502, false},
		{"connection dropped on the first mint", 0, 0, true},
	} {
		t.Run("outcome unknown: "+tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			f.failMintAt, f.failMintStatus, f.failMintHangup = tc.n+1, tc.status, tc.hangup
			h := newHarness(t, f)
			if code := run(h.provisionArgs(), h.e); code == exitOK {
				t.Fatal("a partial run exited 0")
			}
			if f.mints != tc.n+1 {
				t.Fatalf("mint attempts = %d, want %d (never retried, never continued)", f.mints, tc.n+1)
			}
			reports, _ := filepath.Glob(filepath.Join(h.dir, "deskfleet-partial-run-*.txt"))
			if len(reports) != 1 {
				t.Fatalf("want exactly one partial-run report file, got %v\nstderr:\n%s", reports, h.err.String())
			}
			rb, _ := os.ReadFile(reports[0])
			report := string(rb)
			role := fleetRoles[tc.n].Role
			acct := serviceAccountUsername(fakePrefix, role)
			if !strings.Contains(report, "COULD-NOT-CHECK") ||
				!strings.Contains(report, "? role="+role+" account="+acct) || !strings.Contains(report, patName(role)) {
				t.Errorf("report does not name the unknown-outcome mint for %s as could-not-check\n%s", acct, report)
			}
			if strings.Contains(report, "Account "+acct+" was CREATED by this run but holds NO token") {
				t.Errorf("an unknown-outcome mint is reported as DEFINITELY tokenless\n%s", report)
			}
			if strings.Contains(h.allText(t), "nothing to revoke") {
				t.Errorf("an unknown-outcome mint is reported as nothing to revoke")
			}
			if got := strings.Count(report, "\n  - role="); got != tc.n {
				t.Errorf("report lists %d minted token(s), want %d\n%s", got, tc.n, report)
			}
		})
	}
	t.Run("membership refused after the account was created", func(t *testing.T) {
		f := newFakeForge(t)
		f.failMemberPost = true
		h := newHarness(t, f)
		if code := run(h.provisionArgs(), h.e); code == exitOK {
			t.Fatal("a partial run exited 0")
		}
		if f.mints != 0 {
			t.Fatalf("minted %d token(s) past a failed membership step", f.mints)
		}
		reports, _ := filepath.Glob(filepath.Join(h.dir, "deskfleet-partial-run-*.txt"))
		if len(reports) != 1 {
			t.Fatalf("want exactly one partial-run report file, got %v\nstderr:\n%s", reports, h.err.String())
		}
		rb, _ := os.ReadFile(reports[0])
		acct := serviceAccountUsername(fakePrefix, fleetRoles[0].Role)
		if !strings.Contains(string(rb), "Account "+acct+" was CREATED by this run but holds NO token") {
			t.Errorf("report does not name %s, created by the run before its membership failed\n%s", acct, rb)
		}
	})
	t.Run("nothing created and nothing minted is the only nothing-to-revoke", func(t *testing.T) {
		f := newFakeForge(t)
		f.failMemberPost = true
		for i, r := range fleetRoles { // every account pre-exists; none is a member yet
			f.accounts[serviceAccountUsername(fakePrefix, r.Role)] = int64(2000 + i)
		}
		h := newHarness(t, f)
		if code := run(h.provisionArgs(), h.e); code == exitOK {
			t.Fatal("a stopped run exited 0")
		}
		if !strings.Contains(h.err.String(), "nothing to revoke") {
			t.Errorf("a run that created nothing and minted nothing does not say so:\n%s", h.err.String())
		}
		if reports, _ := filepath.Glob(filepath.Join(h.dir, "deskfleet-partial-run-*.txt")); len(reports) != 0 {
			t.Errorf("a report was written for a run that created and minted nothing: %v", reports)
		}
	})
	t.Run("settings steps collect rather than abort", func(t *testing.T) {
		f := newFakeForge(t)
		f.approvalsErr = true
		f.tagsPostErr = true
		h := newHarness(t, f)
		code := run(h.provisionArgs("--project", fakeProject), h.e)
		if code != exitFailed {
			t.Fatalf("exit %d, want %d", code, exitFailed)
		}
		for _, want := range []string{"approval settings write failed", "protected-tags rule"} {
			if !strings.Contains(h.out.String(), want) {
				t.Errorf("summary does not name the failed step %q", want)
			}
		}
		if len(f.labels) != len(fleetLabels) {
			t.Errorf("only %d labels created — a failed earlier settings step aborted the later ones", len(f.labels))
		}
		if f.rule == nil {
			t.Error("main is unprotected at the end of the run")
		}
	})
}

// Brief row 12: a stub that refuses the POST half of a DELETE+POST. The rule the tool read is
// re-applied, and no GET observation ever finds `main` unprotected.
func TestFleetProtectedBranchNeverUnprotected(t *testing.T) {
	t.Run("refused POST re-applies the previous rule", func(t *testing.T) {
		f := newFakeForge(t)
		f.plan = "free"
		f.rule = &fakeRule{push: 30, merge: 30, force: true} // wrong in two fields: forces DELETE+POST
		f.refuseIntent = true
		h := newHarness(t, f)
		code := run(h.provisionArgs("--project", fakeProject), h.e)
		if code != exitFailed {
			t.Fatalf("exit %d, want %d\n%s", code, exitFailed, h.out.String())
		}
		if f.rule == nil {
			t.Fatal("main was left UNPROTECTED")
		}
		if f.rule.push != 30 || f.rule.merge != 30 || !f.rule.force {
			t.Fatalf("restored rule = %+v, want the rule that was read (push 30, merge 30, force true)", *f.rule)
		}
		if f.unprotectedObserved != 0 {
			t.Fatalf("%d GET observation(s) found main unprotected", f.unprotectedObserved)
		}
		if !strings.Contains(h.out.String(), "restored to its PREVIOUS rule") {
			t.Fatalf("summary does not report the restore:\n%s", h.out.String())
		}
	})
	t.Run("refused DELETE leaves the rule and posts nothing", func(t *testing.T) {
		f := newFakeForge(t)
		f.plan = "free"
		f.rule = &fakeRule{push: 30, merge: 30, force: true}
		f.deleteStatus = 403
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
			t.Fatalf("exit %d, want %d", code, exitFailed)
		}
		for _, r := range f.requests() {
			if r.Method == "POST" && strings.HasSuffix(r.Path, "/protected_branches") {
				t.Fatal("a POST followed a refused DELETE")
			}
		}
		if f.rule == nil || f.rule.merge != 30 {
			t.Fatal("the previous rule did not survive a refused DELETE")
		}
	})
	t.Run("only force-push wrong is a PATCH, never a delete", func(t *testing.T) {
		f := newFakeForge(t)
		f.plan = "free"
		f.rule = &fakeRule{push: 0, merge: 40, force: true}
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitOK {
			t.Fatalf("exit %d\n%s", code, h.out.String())
		}
		for _, r := range f.requests() {
			if r.Method == "DELETE" {
				t.Fatal("a PATCH-able rule was deleted")
			}
		}
	})
}

// Brief row 13: ONE table drives BOTH forges; each forge's emitted set carries
// review-request, the six raised-by:* stamps and the authorization-needed/approval-needed pair.
func TestLabelsForgeParity(t *testing.T) {
	required := []string{"review-request", "raised-by:desk", "raised-by:worker", "raised-by:reviewer",
		"raised-by:verifier", "raised-by:issue-loop", "raised-by:intake-loop",
		"authorization-needed", "approval-needed"}
	f := newFakeForge(t)
	h := newHarness(t, f)
	if code := run([]string{"labels", "--forge", "gitlab", "--project", fakeProject, "--token-file", h.ownerFile}, h.e); code != exitOK {
		t.Fatalf("gitlab: exit %d\n%s", code, h.err.String())
	}
	if code := run([]string{"labels", "--forge", "github", "--repo", "example-org/example-repo", "--token-file", h.ownerFile}, h.e); code != exitOK {
		t.Fatalf("github: exit %d\n%s", code, h.err.String())
	}
	names := func(m map[string]bool) []string {
		var s []string
		for k := range m {
			s = append(s, k)
		}
		sort.Strings(s)
		return s
	}
	gl, gh := names(f.labels), names(f.ghLabels)
	if strings.Join(gl, ",") != strings.Join(gh, ",") {
		t.Fatalf("the two forges received different label sets:\ngitlab %v\ngithub %v", gl, gh)
	}
	for _, want := range required {
		if !f.labels[want] || !f.ghLabels[want] {
			t.Errorf("label %q missing on gitlab=%t github=%t", want, f.labels[want], f.ghLabels[want])
		}
	}
	// Colors: GitLab gets the leading '#', GitHub the bare six digits — from the same row.
	for _, r := range f.requests() {
		c, _ := r.Body["color"].(string)
		if strings.HasPrefix(r.Path, "/repos/") && strings.HasPrefix(c, "#") {
			t.Errorf("github label color %q carries a '#'", c)
		}
		if strings.HasPrefix(r.Path, "/api/v4/projects/") && strings.HasSuffix(r.Path, "/labels") && !strings.HasPrefix(c, "#") {
			t.Errorf("gitlab label color %q lacks the '#'", c)
		}
	}
}

// Brief row 14: labels that already exist are a no-op on both forges (GitLab's 409 and its
// 400 "has already been taken" variant; GitHub's 422 already_exists), and the run exits 0.
func TestLabelsIdempotent(t *testing.T) {
	for _, dup400 := range []bool{false, true} {
		f := newFakeForge(t)
		f.gitlab400Dup = dup400
		for _, l := range fleetLabels {
			f.labels[l.Name] = true
			f.ghLabels[l.Name] = true
		}
		h := newHarness(t, f)
		for _, args := range [][]string{
			{"labels", "--forge", "gitlab", "--project", fakeProject, "--token-file", h.ownerFile},
			{"labels", "--forge", "github", "--repo", "example-org/example-repo", "--token-file", h.ownerFile},
		} {
			if code := run(args, h.e); code != exitOK {
				t.Fatalf("%v (400-variant=%t): exit %d\n%s", args[:3], dup400, code, h.err.String())
			}
		}
		if n := strings.Count(h.out.String(), "already exists (no-op)"); n != 2*len(fleetLabels) {
			t.Fatalf("%d no-op lines, want %d\n%s", n, 2*len(fleetLabels), h.out.String())
		}
	}
}

// A re-run against an already-provisioned group mints NOTHING: an existing account's token
// is a rotation, never a second live credential.
func TestFleetRerunMintsNothing(t *testing.T) {
	f := newFakeForge(t)
	for i, r := range fleetRoles {
		id := int64(2000 + i)
		f.accounts[serviceAccountUsername(fakePrefix, r.Role)] = id
		f.members[id] = r.AccessLevel
	}
	h := newHarness(t, f)
	if code := run(h.provisionArgs(), h.e); code != exitOK {
		t.Fatalf("exit %d\n%s", code, h.err.String())
	}
	if f.mints != 0 {
		t.Fatalf("a re-run minted %d token(s)", f.mints)
	}
}

// A non-https GITLAB_API_BASE that is not loopback is refused before contact.
func TestFleetRefusesCleartextAPIBase(t *testing.T) {
	h := newHarness(t, nil)
	ft := &failingTransport{t: t}
	h.e.http = &http.Client{Transport: ft}
	h.apiBase = "http://gitlab.example.com/api/v4"
	if code := run(h.provisionArgs(), h.e); code != exitRefused {
		t.Fatalf("exit %d, want %d", code, exitRefused)
	}
	if ft.calls != 0 {
		t.Fatal("transport reached")
	}
}

// A redirect is never followed: net/http forwards a CUSTOM header such as PRIVATE-TOKEN to the
// redirect target, so following one could hand the credential to another host.
func TestFleetNeverFollowsRedirect(t *testing.T) {
	var leaked []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = append(leaked, r.Header.Get("PRIVATE-TOKEN"))
		w.WriteHeader(200)
	}))
	defer other.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()
	h := newHarness(t, nil)
	h.apiBase = redirector.URL + "/api/v4"
	if code := run(h.provisionArgs(), h.e); code == exitOK {
		t.Fatal("a redirected group lookup succeeded")
	}
	if len(leaked) != 0 {
		t.Fatalf("the redirect target received %d request(s) carrying the credential header", len(leaked))
	}
}

// F2: the protected-main READ-BACK treats an absent level as unknown — never as the intended
// value — and records a failure for it, as the script's `// "none"` does. The deciding read
// keeps the script's defaults.
func TestFleetReadbackAbsentMergeLevel(t *testing.T) {
	body := []byte(`{"merge_access_levels":[],"allow_force_push":false}`)
	if r := parseProtectedRule(body, true); r.merge != levelNone || r.push != levelNone {
		t.Fatalf("read-back parsed an absent level as merge=%d push=%d, want none (%d)", r.merge, r.push, levelNone)
	}
	if r := parseProtectedRule(body, false); r.merge != mergeAccessLevel || r.push != 0 {
		t.Fatalf("deciding read parsed merge=%d push=%d, want the script's defaults 40 / 0", r.merge, r.push)
	}
	f := newFakeForge(t)
	f.plan = "free"
	f.rule = &fakeRule{push: 0, merge: mergeAccessLevel, force: false}
	f.omitMergeLevels = true
	h := newHarness(t, f)
	if code := run(h.provisionArgs("--project", fakeProject), h.e); code != exitFailed {
		t.Fatalf("exit %d, want %d — an absent merge level must be a recorded failure\n%s", code, exitFailed, h.out.String())
	}
	if !strings.Contains(h.out.String(), "merge_access_level=none") ||
		!strings.Contains(h.out.String(), "merge_access_level is none, intended 40") {
		t.Fatalf("the read-back does not print and fail on the absent merge level:\n%s", h.out.String())
	}
}

// F4 / S2: every network-reaching mode prints its target BEFORE its first request. The
// transport snapshots stdout at the first request.
func TestFleetPrintsTargetBeforeFirstContact(t *testing.T) {
	avatarsDir := t.TempDir()
	for _, r := range fleetRoles {
		if err := os.WriteFile(filepath.Join(avatarsDir, r.Role+".png"), []byte("png-"+r.Role), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		args func(h *harness) []string
		want func(f *fakeForge) string
	}{
		{"labels gitlab",
			func(h *harness) []string {
				return []string{"labels", "--forge", "gitlab", "--project", fakeProject, "--token-file", h.ownerFile}
			},
			func(f *fakeForge) string { return "target: " + f.base() + " gitlab " + fakeProject }},
		{"labels github",
			func(h *harness) []string {
				return []string{"labels", "--forge", "github", "--repo", "example-org/example-repo", "--token-file", h.ownerFile}
			},
			func(f *fakeForge) string { return "target: " + f.srv.URL + " github example-org/example-repo" }},
		{"provision",
			func(h *harness) []string { return h.provisionArgs() },
			func(f *fakeForge) string { return "target: " + f.base() + " (GITLAB_API_BASE)" }},
		{"provision --avatars-only",
			func(h *harness) []string {
				if err := os.WriteFile(filepath.Join(h.dir, tokenFileName("reviewer")), []byte(fakeToken(1)), 0o600); err != nil {
					t.Fatal(err)
				}
				return []string{"provision", "--avatars-only", "--avatars-dir", avatarsDir, "--prefix", fakePrefix, "--out-dir", h.dir}
			},
			func(f *fakeForge) string { return "target: " + f.base() + " (GITLAB_API_BASE)" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeForge(t)
			h := newHarness(t, f)
			ft := &firstContactTransport{out: h.out}
			h.e.http = &http.Client{Transport: ft}
			if code := run(tc.args(h), h.e); code != exitOK {
				t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, h.out.String(), h.err.String())
			}
			if ft.requests == 0 {
				t.Fatal("the run made no request — the instrument did not look")
			}
			if want := tc.want(f); !strings.Contains(ft.atFirst, want) {
				t.Fatalf("the first request went out before %q was printed; stdout at first contact:\n%s", want, ft.atFirst)
			}
		})
	}
}

// F3: the avatar step (PUT /user/avatar, as the ROLE's own PAT) is ported behind an explicit
// --avatars-dir; without it the step is skipped, named, and makes no request.
func TestFleetAvatars(t *testing.T) {
	avatarsDir := t.TempDir()
	for _, r := range fleetRoles {
		if r.Role == "desk" {
			continue // a missing icon is a NOTICE, never a failure
		}
		if err := os.WriteFile(filepath.Join(avatarsDir, r.Role+".png"), []byte("png-"+r.Role), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	avatarRequests := func(f *fakeForge) int {
		n := 0
		for _, r := range f.requests() {
			if r.Path == "/api/v4/user/avatar" {
				n++
			}
		}
		return n
	}
	t.Run("each new account sets its own avatar", func(t *testing.T) {
		f := newFakeForge(t)
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--avatars-dir", avatarsDir), h.e); code != exitOK {
			t.Fatalf("exit %d\n%s", code, h.err.String())
		}
		for i, r := range fleetRoles {
			got := f.avatars[fakeToken(i+1)]
			want := r.Role + ".png:png-" + r.Role
			if r.Role == "desk" {
				want = ""
			}
			if got != want {
				t.Errorf("role %s: avatar signed in as its own token = %q, want %q", r.Role, got, want)
			}
		}
		if _, ok := f.avatars[fakeOwnerToken]; ok {
			t.Error("an avatar was set with the OWNER credential — each account must set its own")
		}
		if !strings.Contains(h.out.String(), "NOTICE: no avatar for desk") {
			t.Errorf("the missing desk icon is not named:\n%s", h.out.String())
		}
		if strings.Contains(h.out.String(), "SKIPPED STEP — avatars") {
			t.Error("the summary says avatars were skipped on a run that set them")
		}
	})
	t.Run("without --avatars-dir the step is skipped and named", func(t *testing.T) {
		f := newFakeForge(t)
		h := newHarness(t, f)
		if code := run(h.provisionArgs(), h.e); code != exitOK {
			t.Fatalf("exit %d\n%s", code, h.err.String())
		}
		if n := avatarRequests(f); n != 0 {
			t.Fatalf("%d avatar request(s) without --avatars-dir", n)
		}
		if !strings.Contains(h.out.String(), "SKIPPED STEP — avatars") || !strings.Contains(h.out.String(), "--avatars-only") {
			t.Fatalf("the summary does not name the skipped avatar step and how to run it:\n%s", h.out.String())
		}
	})
	t.Run("a refused upload is a recorded failure and the loop continues", func(t *testing.T) {
		f := newFakeForge(t)
		f.avatarStatus = 403
		h := newHarness(t, f)
		if code := run(h.provisionArgs("--avatars-dir", avatarsDir), h.e); code != exitFailed {
			t.Fatalf("exit %d, want %d", code, exitFailed)
		}
		if f.mints != len(fleetRoles) {
			t.Fatalf("the run stopped after %d mints on an avatar failure", f.mints)
		}
		if !strings.Contains(h.out.String(), "avatar upload for "+serviceAccountUsername(fakePrefix, "reviewer")+" failed") {
			t.Fatalf("the summary does not name the failed avatar upload:\n%s", h.out.String())
		}
	})
	t.Run("avatars-only signs in as each role from its token file and touches nothing else", func(t *testing.T) {
		f := newFakeForge(t)
		h := newHarness(t, f)
		for i, r := range fleetRoles {
			if r.Role == "verifier" {
				continue // no token file: a NOTICE, never a failure
			}
			if err := os.WriteFile(filepath.Join(h.dir, tokenFileName(r.Role)), []byte(fakeToken(i+1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		args := []string{"provision", "--avatars-only", "--avatars-dir", avatarsDir, "--prefix", fakePrefix, "--out-dir", h.dir}
		if code := run(args, h.e); code != exitOK {
			t.Fatalf("exit %d\n%s", code, h.err.String())
		}
		for _, r := range f.requests() {
			if r.Path != "/api/v4/user/avatar" {
				t.Errorf("avatars-only made %s %s", r.Method, r.Path)
			}
		}
		if len(f.avatars) != len(fleetRoles)-2 { // no verifier token, no desk icon
			t.Fatalf("%d avatars set, want %d: %v", len(f.avatars), len(fleetRoles)-2, f.avatars)
		}
		if !strings.Contains(h.out.String(), "avatar for "+serviceAccountUsername(fakePrefix, "verifier")+" skipped — no token file") {
			t.Errorf("the missing verifier token file is not named:\n%s", h.out.String())
		}
	})
	t.Run("avatars-only takes no owner credential and no project", func(t *testing.T) {
		h := newHarness(t, nil)
		h.e.http = &http.Client{Transport: &failingTransport{t: t}}
		for _, extra := range [][]string{{"--owner-token-file", h.ownerFile}, {"--project", fakeProject}} {
			args := append([]string{"provision", "--avatars-only", "--avatars-dir", avatarsDir, "--prefix", fakePrefix}, extra...)
			if code := run(args, h.e); code != exitUsage {
				t.Errorf("%v: exit %d, want %d", extra, code, exitUsage)
			}
		}
	})
}
