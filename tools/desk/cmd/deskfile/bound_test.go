package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bound_test.go — workflow-bound filings (`deskfile new --for-pr ... --brief ...`).

// prState is the change-request side of the fake forge: the PRs, their file lists and the
// repository files the binding check reads. installBoundFake resets it per test.
type prState struct {
	prs     map[int]*deskkit.PullRequest
	files   map[int][]deskkit.ChangedFile
	readErr map[int]error
	content map[string]string // path -> file content (any ref)
	// commentErr fails the back-link comment only (the issue create still succeeds).
	commentErr error
}

var prFake *prState

func (f *dfForge) GetPullRequest(repo deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	if err := prFake.readErr[n]; err != nil {
		return nil, err
	}
	pr, ok := prFake.prs[n]
	if !ok {
		return nil, fmt.Errorf("HTTP 404: %s#%d not found", repo.Slug(), n)
	}
	c := *pr
	return &c, nil
}

func (f *dfForge) ListChangedFiles(repo deskkit.ForgeRepo, n int) ([]deskkit.ChangedFile, error) {
	return append([]deskkit.ChangedFile(nil), prFake.files[n]...), nil
}

func (f *dfForge) ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	c, ok := prFake.content[in.File]
	if !ok {
		return &deskkit.FileContent{Exists: false}, nil
	}
	return &deskkit.FileContent{Content: []byte(c), Exists: true}, nil
}

func (f *dfForge) RepoDefaultBranch(repo deskkit.ForgeRepo) (string, error) { return "main", nil }

const (
	bfBrief07 = "docs/streams/demo/brief-07-thing.md"
	bfBrief08 = "docs/streams/demo/brief-08-other.md"
	bfEvil07  = "docs/streams/evil/brief-07-x.md"
	bfPlain03 = "docs/streams/plain/brief-03-x.md"
)

// installBoundFake plants: #42 open carrying `Brief: demo/07`; #46 the same; #43 open with no
// brief; #44 closed; #45 unreadable; #47 open, no trailer, its diff touching brief 07; #48 its
// diff touching an evil brief whose frontmatter id injects a role; #49 its diff touching a
// brief with no frontmatter id; #50 `Cross-repo-brief: tracker:demo/07`; #51 two
// Cross-repo-brief trailers; #52 `Authors: demo/07 demo/08`, its diff touching brief 07 only.
func installBoundFake(t *testing.T) {
	t.Helper()
	open := func(n int, body string, files ...string) {
		cf := make([]deskkit.ChangedFile, 0, len(files))
		for _, p := range files {
			cf = append(cf, deskkit.ChangedFile{Filename: p, Status: "modified"})
		}
		prFake.prs[n] = &deskkit.PullRequest{Number: n, State: "open", Body: body, HeadSHA: "abc123",
			BaseRef: "main", ChangedFiles: len(cf)}
		prFake.files[n] = cf
	}
	prFake = &prState{prs: map[int]*deskkit.PullRequest{}, files: map[int][]deskkit.ChangedFile{},
		readErr: map[int]error{}, content: map[string]string{
			"docs/streams/demo/README.md":   "| [07](brief-07-thing.md) |\n| [08](brief-08-other.md) |\n",
			bfBrief07:                       "---\nbrief: example:demo:07\n---\n# 07\n",
			bfBrief08:                       "---\nbrief: example:demo:08\n---\n# 08\n",
			bfEvil07:                        "---\nbrief: x role nobody | junk:evil:07\n---\n# 07\n",
			bfPlain03:                       "# 03, no frontmatter\n",
			"docs/streams/graph-repos.yaml": "repos:\n  tracker:\n    repo: example-org/tracker\n",
		}}
	open(42, "Change.\n\nBrief: demo/07\n", "tools/a.go")
	open(46, "Change.\n\nBrief: demo/07\n", "tools/b.go")
	open(43, "No brief.\n", "tools/c.go")
	open(47, "Authoring.\n", bfBrief07)
	open(44, "Brief: demo/07\n", "tools/d.go")
	prFake.prs[44].State = "closed"
	open(48, "Authoring.\n", bfEvil07)
	open(49, "Authoring.\n", bfPlain03)
	open(50, "Change.\n\nCross-repo-brief: tracker:demo/07\n", "tools/e.go")
	open(51, "Change.\n\nCross-repo-brief: tracker:demo/08\nCross-repo-brief: tracker:demo/07\n", "tools/f.go")
	open(52, "Authoring.\n\nAuthors: demo/07 demo/08\n", bfBrief07)
	prFake.readErr[45] = deskkit.Unverifiable("HTTP 502", nil)
	t.Cleanup(func() { prFake = nil })
}

func boundEnv(t *testing.T) *[][]string {
	t.Helper()
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	for _, k := range []string{"ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR", "ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER",
		"ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__REVIEWER", "ASSAY_DESK_WRITE_BOUND_REPO_HOUR"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	installBoundFake(t)
	return calls
}

var bfSeq int

// fileBound runs one `deskfile new` with a fresh title and the extra flags.
func fileBound(t *testing.T, extra ...string) (int, string) {
	t.Helper()
	bfSeq++
	body := bodyFileWith(t, "a blocker the review of this change requires")
	args := append([]string{"new", "-R", allowedRepo, "--title",
		fmt.Sprintf("bound blocker number %d for review", bfSeq), "--body-file", body}, extra...)
	return runCapture(args)
}

func boundEntries(t *testing.T) []deskkit.Entry {
	t.Helper()
	var out []deskkit.Entry
	for _, e := range readAudit(t) {
		if e.Tool == "deskfile" && e.Verb == boundVerb {
			out = append(out, e)
		}
	}
	return out
}

// TestBoundFiling: the binding, its refusals and its exemption from the per-session rate.
// regression: #4072
func TestBoundFiling(t *testing.T) {
	t.Run("spent budget, bound files", func(t *testing.T) {
		calls := boundEnv(t)
		seedNewAudit(t, defaultNewRate, allowedRepo, "test")
		// The unbound filing is refused by the spent rate (unchanged)...
		if rc, out := fileBound(t); rc != deskkit.ExitRateLimited {
			t.Fatalf("unbound rc = %d, want 4\n%s", rc, out)
		}
		assertNoIssueCreate(t, *calls)
		// ...and the bound one files.
		rc, out := fileBound(t, "--for-pr", allowedRepo+"#42", "--brief", "demo/07")
		if rc != deskkit.ExitOK {
			t.Fatalf("bound rc = %d, want 0\n%s", rc, out)
		}
		if curForge.filed == nil {
			t.Fatal("bound filing did not create an issue")
		}
		es := boundEntries(t)
		if len(es) != 1 || es[0].Result != deskkit.ResultOK || es[0].PR == nil || *es[0].PR != 42 {
			t.Fatalf("bound audit entry = %+v, want one ok entry with PR 42", es)
		}
		want := "for-pr " + allowedRepo + "#42 role worker brief example:demo:07 | " + createSentMarker
		if !strings.HasPrefix(es[0].Detail, want) {
			t.Fatalf("Detail = %q, want prefix %q", es[0].Detail, want)
		}
		// The back-link: one comment, on the change request, naming the issue.
		if len(curForge.comments) != 1 || curForge.comments[0] != 42 ||
			curForge.commentKinds[0] != deskkit.TargetChange {
			t.Fatalf("back-link comments = %v kinds %v, want one on change 42", curForge.comments, curForge.commentKinds)
		}
		// The per-session count is untouched: an unbound filing is still refused.
		if rc, _ := fileBound(t); rc != deskkit.ExitRateLimited {
			t.Fatalf("after bound, unbound rc = %d, want 4", rc)
		}
	})

	refuse := func(name string, wantRC int, extra ...string) {
		t.Run(name, func(t *testing.T) {
			calls := boundEnv(t)
			rc, out := fileBound(t, extra...)
			if rc != wantRC {
				t.Fatalf("rc = %d, want %d\n%s", rc, wantRC, out)
			}
			assertNoIssueCreate(t, *calls)
			if curForge.filed != nil || len(curForge.comments) != 0 {
				t.Fatal("a refused binding wrote to the forge")
			}
			for _, e := range readAudit(t) {
				if e.Tool == "deskfile" && chargedNewEntry(e) {
					t.Fatalf("a refused binding was charged: %+v", e)
				}
			}
		})
	}
	refuse("brief alone refused", deskkit.ExitRefused, "--brief", "demo/07")
	refuse("for-pr alone, PR has brief", deskkit.ExitRefused, "--for-pr", allowedRepo+"#42")
	refuse("for-pr alone, diff has brief", deskkit.ExitRefused, "--for-pr", allowedRepo+"#47")
	refuse("closed PR refused", deskkit.ExitRefused, "--for-pr", allowedRepo+"#44", "--brief", "demo/07")
	refuse("not the PR's brief", deskkit.ExitRefused, "--for-pr", allowedRepo+"#42", "--brief", "demo/08")
	refuse("brief resolves to no file", deskkit.ExitRefused, "--for-pr", allowedRepo+"#42", "--brief", "demo/09")
	refuse("qualified prefix mismatch", deskkit.ExitRefused, "--for-pr", allowedRepo+"#42", "--brief", "other:demo:07")
	refuse("unreadable PR holds", deskkit.ExitUnverifiable, "--for-pr", allowedRepo+"#45", "--brief", "demo/07")
	refuse("malformed for-pr", deskkit.ExitRefused, "--for-pr", "42", "--brief", "demo/07")
	refuse("force-file with for-pr", deskkit.ExitRefused, "--for-pr", allowedRepo+"#42", "--brief", "demo/07",
		"--force-file", "--reason", "urgent")
	refuse("two cross-repo trailers", deskkit.ExitRefused, "--for-pr", allowedRepo+"#51", "--brief", "demo/07")
	refuse("cross-repo, not its brief", deskkit.ExitRefused, "--for-pr", allowedRepo+"#50", "--brief", "demo/08")
	refuse("authors PR needs brief", deskkit.ExitRefused, "--for-pr", allowedRepo+"#52")
	refuse("authors PR, brief not in diff", deskkit.ExitRefused, "--for-pr", allowedRepo+"#52", "--brief", "demo/08")
	refuse("qualified, no frontmatter id", deskkit.ExitRefused, "--for-pr", allowedRepo+"#49", "--brief", "example:plain:03")

	bound := func(name, ref, brief, wantPrefix string) {
		t.Run(name, func(t *testing.T) {
			boundEnv(t)
			rc, out := fileBound(t, "--for-pr", allowedRepo+ref, "--brief", brief)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0\n%s", rc, out)
			}
			if es := boundEntries(t); len(es) != 1 || !strings.HasPrefix(es[0].Detail, wantPrefix) {
				t.Fatalf("entries = %+v, want Detail prefix %q", es, wantPrefix)
			}
		})
	}
	bound("cross-repo brief binds", "#50", "demo/07", "for-pr "+allowedRepo+"#50 role worker brief example:demo:07 | ")
	bound("authors PR, brief in diff", "#52", "demo/07", "for-pr "+allowedRepo+"#52 role worker brief example:demo:07 | ")
	bound("no frontmatter id, plain", "#49", "plain/03", "for-pr "+allowedRepo+"#49 role worker brief plain/03 | ")

	t.Run("for-pr alone, PR has none", func(t *testing.T) {
		boundEnv(t)
		rc, out := fileBound(t, "--for-pr", allowedRepo+"#43")
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0\n%s", rc, out)
		}
		es := boundEntries(t)
		if len(es) != 1 || !strings.HasPrefix(es[0].Detail, "for-pr "+allowedRepo+"#43 role worker brief none | ") {
			t.Fatalf("entries = %+v", es)
		}
	})

	t.Run("diff touches the brief", func(t *testing.T) {
		boundEnv(t)
		rc, out := fileBound(t, "--for-pr", allowedRepo+"#47", "--brief", "demo:07")
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0\n%s", rc, out)
		}
	})

	t.Run("back-link failure holds", func(t *testing.T) {
		boundEnv(t)
		prFake.commentErr = fmt.Errorf("HTTP 500")
		rc, out := fileBound(t, "--for-pr", allowedRepo+"#42", "--brief", "demo/07")
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc = %d, want 6\n%s", rc, out)
		}
		if curForge.filed == nil || !strings.Contains(out, "issues/101") {
			t.Fatalf("the issue must stay filed and be named; out:\n%s", out)
		}
		// The create was sent, so the line charges the bound count.
		es := boundEntries(t)
		if len(es) != 1 || !chargedNewEntry(deskkit.Entry{Result: es[0].Result, Detail: strings.SplitN(es[0].Detail, " | ", 2)[1]}) {
			t.Fatalf("back-link failure entry = %+v, want a charged line", es)
		}
	})
}

// TestBoundFilingCap: the per-role, per-change cap and how its role is chosen.
// regression: #4072
func TestBoundFilingCap(t *testing.T) {
	boundEnv(t)
	t.Setenv("ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER", "2")
	bind42 := []string{"--for-pr", allowedRepo + "#42", "--brief", "demo/07"}
	for i := 1; i <= 2; i++ {
		if rc, out := fileBound(t, bind42...); rc != deskkit.ExitOK {
			t.Fatalf("filing %d rc = %d, want 0\n%s", i, rc, out)
		}
	}
	rc, out := fileBound(t, bind42...)
	if rc != deskkit.ExitRateLimited || !strings.Contains(out, "retry-after:") {
		t.Fatalf("third filing rc = %d, want 4 with retry-after\n%s", rc, out)
	}
	for _, e := range boundEntries(t) {
		if !strings.HasPrefix(e.Detail, "for-pr "+allowedRepo+"#42 role worker brief ") {
			t.Fatalf("Detail does not record the session-token role: %q", e.Detail)
		}
	}

	// --raised-by another role, and a per-role override for another role, change nothing.
	t.Setenv("ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__REVIEWER", "100")
	if rc, out := fileBound(t, append(bind42, "--raised-by", "reviewer")...); rc != deskkit.ExitRateLimited {
		t.Fatalf("--raised-by reviewer rc = %d, want 4\n%s", rc, out)
	}

	// A different change request is a different count.
	if rc, out := fileBound(t, "--for-pr", allowedRepo+"#46", "--brief", "demo/07"); rc != deskkit.ExitOK {
		t.Fatalf("other PR rc = %d, want 0\n%s", rc, out)
	}

	// A different role (the session's own loop, not a flag) is counted separately.
	t.Setenv("ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__REVIEWER", "2")
	t.Setenv("DESK_LOOP", "pr-review-desk")
	if rc, out := fileBound(t, bind42...); rc != deskkit.ExitOK {
		t.Fatalf("reviewer on PR 42 rc = %d, want 0\n%s", rc, out)
	}
	last := boundEntries(t)
	if d := last[len(last)-1].Detail; !strings.Contains(d, " role reviewer brief ") {
		t.Fatalf("reviewer Detail = %q", d)
	}
	t.Setenv("DESK_LOOP", "worker-desk")
	if rc, _ := fileBound(t, bind42...); rc != deskkit.ExitRateLimited {
		t.Fatalf("worker still capped after reviewer filed: rc = %d, want 4", rc)
	}

	// An unresolvable role holds before anything is created. cmdNew is called directly: run()
	// refuses an unset loop at its own gate first, which is not the path under test.
	for _, loop := range []string{"", "no-such-loop", "conference-scan"} {
		t.Run("role "+loop, func(t *testing.T) {
			created := curForge.filed
			curForge.filed = nil
			t.Setenv("DESK_LOOP", loop)
			body := bodyFileWith(t, "a blocker the review of this change requires")
			err := cmdNew([]string{"-R", allowedRepo, "--title", "unresolvable role filing", "--body-file", body,
				"--for-pr", allowedRepo + "#43"})
			if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
				t.Fatalf("DESK_LOOP=%q: rc = %d (%v), want 6", loop, deskkit.ExitCodeOf(err), err)
			}
			if curForge.filed != nil {
				t.Fatal("an issue was created under an unresolvable role")
			}
			curForge.filed = created
		})
	}
}

// TestBoundFilingOffFixedCeiling: a bound filing is no longer held by the fixed
// 20-an-hour repo-wide ceiling (writes that fill it do not refuse it), but it is still held: by
// the PR-bound backstop (write.bound.repo_hour), which counts the bound class apart and refuses
// at its limit. The backstop's value and source are on the audit line.
// regression: #4072, #4161
func TestBoundFilingOffFixedCeiling(t *testing.T) {
	boundEnv(t)
	bind := []string{"--for-pr", allowedRepo + "#42", "--brief", "demo/07"}

	// Writes that fill the fixed ceiling no longer refuse a bound filing.
	seedRepoWrites(t, deskkit.RateLimitPerPRPerHour)
	if rc, out := fileBound(t, bind...); rc != deskkit.ExitOK {
		t.Fatalf("bound filing after %d fixed-bucket writes rc = %d, want 0\n%s",
			deskkit.RateLimitPerPRPerHour, rc, out)
	}

	// The bound class is bounded by its own tier: seed it to the default limit, and it refuses.
	const dflt = 60
	seedBoundWrites(t, boundVerb, dflt)
	curForge.filed = nil
	rc, out := fileBound(t, bind...)
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("rc = %d, want 4 (PR-bound backstop at its default %d)\n%s", rc, dflt, out)
	}
	if !strings.Contains(out, "PR-bound write budget") || !strings.Contains(out, "write.bound.repo_hour") {
		t.Fatalf("refusal does not name the PR-bound tier and its knob:\n%s", out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was created on a path that must not write")
	}

	// The knob moves the limit, and the line records the value and its source.
	t.Setenv("ASSAY_DESK_WRITE_BOUND_REPO_HOUR", "500")
	if rc, out := fileBound(t, bind...); rc != deskkit.ExitOK {
		t.Fatalf("filing under a raised backstop rc = %d, want 0\n%s", rc, out)
	}
	es := boundEntries(t)
	last := es[len(es)-1].Detail
	if !strings.Contains(last, "write.bound.repo_hour = 500 (env ASSAY_DESK_WRITE_BOUND_REPO_HOUR") {
		t.Fatalf("Detail = %q, want the backstop's value and source", last)
	}
}

// TestBoundTierAuditedAndHeld: the backstop's default and source are on a bound line and on its
// back-link; an out-of-range value, including 0, holds (exit 6) before anything is created.
// regression: #4161
func TestBoundTierAuditedAndHeld(t *testing.T) {
	boundEnv(t)
	bind := []string{"--for-pr", allowedRepo + "#42", "--brief", "demo/07"}
	if rc, out := fileBound(t, bind...); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d\n%s", rc, out)
	}
	const want = "write.bound.repo_hour = 60 (env unset; config unset; default)"
	var links int
	for _, e := range readAudit(t) {
		if e.Tool != "deskfile" || (e.Verb != boundVerb && e.Verb != attachPRVerb) {
			continue
		}
		if e.Verb == attachPRVerb {
			links++
		}
		if !strings.Contains(e.Detail, want) {
			t.Fatalf("%s line Detail = %q, want it to contain %q", e.Verb, e.Detail, want)
		}
	}
	if links != 1 {
		t.Fatalf("back-link lines = %d, want 1", links)
	}
	curForge.filed = nil
	for _, bad := range []string{"0", "-1", "abc", "99999"} {
		t.Setenv("ASSAY_DESK_WRITE_BOUND_REPO_HOUR", bad)
		if rc, _ := fileBound(t, bind...); rc != deskkit.ExitUnverifiable {
			t.Fatalf("ASSAY_DESK_WRITE_BOUND_REPO_HOUR=%s rc = %d, want 6", bad, rc)
		}
	}
	if curForge.filed != nil {
		t.Fatal("an issue was created on a path that must not write")
	}
}

// TestUnboundNewKeepsCeiling: the positive control. An unbound filing is still refused by the
// fixed ceiling, and bound-class writes do not count toward it.
// regression: #4161
func TestUnboundNewKeepsCeiling(t *testing.T) {
	boundEnv(t)
	seedBoundWrites(t, boundVerb, 2*deskkit.RateLimitPerPRPerHour)
	seedBoundWrites(t, attachPRVerb, deskkit.RateLimitPerPRPerHour)
	if rc, out := fileBound(t); rc != deskkit.ExitOK {
		t.Fatalf("unbound filing after bound-class writes rc = %d, want 0\n%s", rc, out)
	}
	seedRepoWrites(t, deskkit.RateLimitPerPRPerHour)
	curForge.filed = nil
	if rc, out := fileBound(t); rc != deskkit.ExitRateLimited {
		t.Fatalf("unbound filing at the fixed ceiling rc = %d, want 4\n%s", rc, out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was created on a path that must not write")
	}
}

// TestAttachToChangeBackstop: an attach to a change request is the attach-pr verb, is counted
// by the PR-bound backstop and not the fixed ceiling, carries the backstop's note, and is
// refused at the limit before anything is posted; an attach to an issue is not held by it.
// regression: #4161
func TestAttachToChangeBackstop(t *testing.T) {
	boundEnv(t)
	t.Setenv("ASSAY_DESK_WRITE_BOUND_REPO_HOUR", "3")
	attach := func(kind string, to int) int {
		body := bodyFileWith(t, "observation on the change")
		rc, _ := runCapture([]string{"attach", "-R", allowedRepo, "--to", fmt.Sprint(to), "--kind", kind, "--body-file", body})
		return rc
	}
	for i := 0; i < 3; i++ {
		if rc := attach("mr", 600+i); rc != deskkit.ExitOK {
			t.Fatalf("attach %d rc = %d, want 0", i, rc)
		}
	}
	var lines []deskkit.Entry
	for _, e := range readAudit(t) {
		if e.Tool == "deskfile" && e.Verb == attachPRVerb {
			lines = append(lines, e)
		}
	}
	if len(lines) != 3 || !strings.Contains(lines[0].Detail, "write.bound.repo_hour = 3 (env ASSAY_DESK_WRITE_BOUND_REPO_HOUR") {
		t.Fatalf("attach-pr lines = %+v, want 3 carrying the backstop note", lines)
	}
	posted := len(curForge.comments)
	if rc := attach("mr", 700); rc != deskkit.ExitRateLimited {
		t.Fatalf("attach at the backstop rc = %d, want 4", rc)
	}
	if len(curForge.comments) != posted {
		t.Fatal("a refused attach posted a comment")
	}
	if rc := attach("issue", 701); rc != deskkit.ExitOK {
		t.Fatalf("attach to an issue at the backstop rc = %d, want 0", rc)
	}
	// Three attach-pr lines do not fill the fixed ceiling for an unbound filing.
	if rc, out := fileBound(t); rc != deskkit.ExitOK {
		t.Fatalf("unbound filing rc = %d\n%s", rc, out)
	}
}

// seedBoundWrites appends n charged deskfile lines of verb on allowedRepo, dated now.
func seedBoundWrites(t *testing.T, verb string, n int) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		num := 800 + i
		line, _ := json.Marshal(deskkit.Entry{TS: time.Now().UTC().Format(time.RFC3339), Tool: "deskfile",
			Verb: verb, Repo: allowedRepo, PR: &num, Result: deskkit.ResultOK, Detail: "seed", SessionTag: "test"})
		b.Write(append(line, '\n'))
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		t.Fatal(err)
	}
}

// seedRepoWrites appends n charged deskfile attach writes on allowedRepo, each on its own
// issue, dated now — counted by the repo-wide tier, not by any one change's.
func seedRepoWrites(t *testing.T, n int) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		num := 500 + i
		line, _ := json.Marshal(deskkit.Entry{TS: time.Now().UTC().Format(time.RFC3339), Tool: "deskfile",
			Verb: "attach", Repo: allowedRepo, PR: &num, Result: deskkit.ResultOK, Detail: "seed", SessionTag: "test"})
		b.Write(append(line, '\n'))
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		t.Fatal(err)
	}
}

// TestBoundBriefIDInjection: a brief file in the change request's own diff supplies its
// frontmatter id, so that id is untrusted. One that would rewrite the audit prefix (spaces,
// a role field, a '|') is refused before any create, and nothing is charged to any role.
// regression: #4072
func TestBoundBriefIDInjection(t *testing.T) {
	calls := boundEnv(t)
	t.Setenv("ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER", "1")
	for i := 1; i <= 2; i++ {
		rc, out := fileBound(t, "--for-pr", allowedRepo+"#48", "--brief", "evil/07")
		if rc != deskkit.ExitRefused {
			t.Fatalf("filing %d rc = %d, want 5 (malformed frontmatter id)\n%s", i, rc, out)
		}
	}
	assertNoIssueCreate(t, *calls)
	if es := boundEntries(t); len(es) != 0 {
		t.Fatalf("a refused injection wrote bound lines: %+v", es)
	}
	// The read-back takes the role from the field after the PR, ahead of the brief, so even a
	// prefix whose brief field carries text cannot move the counted role.
	pr, role, _, err := boundCharged(deskkit.Entry{Result: deskkit.ResultOK,
		Detail: "for-pr " + allowedRepo + "#48 role worker brief x | junk role nobody | " + createSentMarker})
	if err != nil || role != "worker" || pr.N != 48 {
		t.Fatalf("boundCharged = %v %q %v, want #48 role worker", pr, role, err)
	}
	if _, err := boundPrefix(prRef{Repo: allowedRepo, N: 48}, "x role nobody | junk:evil:07", "worker"); err == nil {
		t.Fatal("boundPrefix accepted a brief id with spaces and '|'")
	}
}

// TestBoundCapOverrideAudited: the cap's value and its source are on every bound line, so an
// override from the environment, including one that turns the cap off, is never invisible.
// Out-of-range values still hold (exit 6).
// regression: #4072
func TestBoundCapOverrideAudited(t *testing.T) {
	boundEnv(t)
	const env = "ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER"
	bind42 := []string{"--for-pr", allowedRepo + "#42", "--brief", "demo/07"}
	if rc, out := fileBound(t, bind42...); rc != deskkit.ExitOK {
		t.Fatalf("default-cap filing rc = %d\n%s", rc, out)
	}
	t.Setenv(env, "1")
	if rc, _ := fileBound(t, bind42...); rc != deskkit.ExitRateLimited {
		t.Fatalf("capped filing rc = %d, want 4", rc)
	}
	t.Setenv(env, "0")
	if rc, out := fileBound(t, bind42...); rc != deskkit.ExitOK {
		t.Fatalf("cap-off filing rc = %d\n%s", rc, out)
	}
	es := boundEntries(t)
	if len(es) != 3 {
		t.Fatalf("bound lines = %d, want 3: %+v", len(es), es)
	}
	for i, want := range []string{
		"bound-config: issue.file.bound_per_pr[worker] = 0 (env unset; config unset; default)",
		"bound-config: issue.file.bound_per_pr[worker] = 1 (env[worker] " + env,
		"bound-config: issue.file.bound_per_pr[worker] = 0 (env[worker] " + env,
	} {
		if !strings.Contains(es[i].Detail, want) || !strings.Contains(es[i].Detail, "issue.file.window") {
			t.Fatalf("line %d Detail = %q, want it to contain %q and the window", i+1, es[i].Detail, want)
		}
	}
	for _, bad := range []string{"-1", "abc", "99999"} {
		t.Setenv(env, bad)
		if rc, _ := fileBound(t, bind42...); rc != deskkit.ExitUnverifiable {
			t.Fatalf("%s=%s rc = %d, want 6", env, bad, rc)
		}
	}
}

// TestBoundRepoMustMatchPR: the bound line is charged to (-R, PR number), so -R must be the
// change request's own repository; any other -R is refused before anything is written.
// regression: #4072
func TestBoundRepoMustMatchPR(t *testing.T) {
	calls := boundEnv(t)
	body := bodyFileWith(t, "a blocker the review of this change requires")
	rc, out := runCapture([]string{"new", "-R", "example-org/tracker", "--title", "bound blocker in another repo",
		"--body-file", body, "--for-pr", allowedRepo + "#42", "--brief", "demo/07"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5\n%s", rc, out)
	}
	assertNoIssueCreate(t, *calls)
	if curForge.filed != nil || len(curForge.comments) != 0 || len(boundEntries(t)) != 0 {
		t.Fatal("a mismatched -R wrote to the forge or the ledger")
	}
}

// TestBoundCapUnreadableLedger: with the cap on, a bound line in the window whose ts or
// prefix cannot be read holds (exit 6); it is never read as uncharged.
// regression: #4072
func TestBoundCapUnreadableLedger(t *testing.T) {
	for name, e := range map[string]deskkit.Entry{
		"bad ts":     {TS: "not-a-time", Detail: "for-pr " + allowedRepo + "#42 role worker brief demo/07 | x"},
		"bad prefix": {TS: time.Now().UTC().Format(time.RFC3339), Detail: "garbled"},
	} {
		t.Run(name, func(t *testing.T) {
			calls := boundEnv(t)
			t.Setenv("ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER", "5")
			n := 42
			e.Tool, e.Verb, e.Repo, e.PR, e.Result, e.SessionTag = "deskfile", boundVerb, allowedRepo, &n, deskkit.ResultOK, "test"
			appendAudit(t, e)
			rc, out := fileBound(t, "--for-pr", allowedRepo+"#42", "--brief", "demo/07")
			if rc != deskkit.ExitUnverifiable {
				t.Fatalf("rc = %d, want 6\n%s", rc, out)
			}
			assertNoIssueCreate(t, *calls)
		})
	}
}

// TestBoundFilingWriteCharges pins what a bound filing costs the per-PR outward-write tier: two
// deskfile writes on (repo, PR) — the new-bound line and the back-link attach-pr line — so with
// the cap off that tier (RateLimitPerPRPerHour writes per hour) admits RateLimitPerPRPerHour/2
// bound filings per hour on one change request. The fixed repo-wide ceiling does not count them
// (#4161; the PR-bound backstop does, see TestBoundFilingOffFixedCeiling).
// regression: #4072, #4161
func TestBoundFilingWriteCharges(t *testing.T) {
	boundEnv(t)
	want := deskkit.RateLimitPerPRPerHour / 2
	for i := 0; i < want; i++ {
		if rc, out := fileBound(t, "--for-pr", allowedRepo+"#42", "--brief", "demo/07"); rc != deskkit.ExitOK {
			t.Fatalf("bound filing %d rc = %d, want 0\n%s", i+1, rc, out)
		}
	}
	writes := 0
	for _, e := range readAudit(t) {
		if e.Tool == "deskfile" && e.Repo == allowedRepo && e.Result == deskkit.ResultOK &&
			(e.Verb == boundVerb || e.Verb == attachPRVerb) {
			writes++
		}
	}
	if writes != 2*want {
		t.Fatalf("charged writes = %d, want %d (two per bound filing)", writes, 2*want)
	}
	curForge.filed = nil
	if rc, out := fileBound(t, "--for-pr", allowedRepo+"#42", "--brief", "demo/07"); rc != deskkit.ExitRateLimited {
		t.Fatalf("filing %d rc = %d, want 4 (per-PR tier)\n%s", want+1, rc, out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was created past the per-PR tier")
	}
	// A second change request has its own per-PR count and the fixed ceiling does not see these.
	if rc, out := fileBound(t, "--for-pr", allowedRepo+"#46", "--brief", "demo/07"); rc != deskkit.ExitOK {
		t.Fatalf("filing on another change rc = %d, want 0\n%s", rc, out)
	}
}

// appendAudit appends one entry to the test HOME's audit ledger.
func appendAudit(t *testing.T, e deskkit.Entry) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}
