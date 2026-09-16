package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// allowedRepo is in the fixture roster's ASSAY_ALLOWED_REPOS; outsideRepo is not.
const (
	allowedRepo = "medici-finance/assay"
	outsideRepo = "someone-else/private-thing"
)

// ghStub replaces the exec seam. It RECORDS every argv — the assertions below run
// against the argv this tool actually constructs, not against its stdout — and replies
// from a canned table keyed by a substring of the joined argv.
type ghStub struct {
	calls   [][]string
	replies []stubReply
}

type stubReply struct {
	match  string // matched against the joined argv
	stdout string
	fail   bool
}

func (s *ghStub) install(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "gt05-test")

	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		for _, r := range s.replies {
			if strings.Contains(joined, r.match) {
				if r.fail {
					return exec.Command("/bin/sh", "-c", "echo stub-failure 1>&2; exit 1")
				}
				return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+r.stdout+"\nSTUBEOF")
			}
		}
		// Anything the table does not name succeeds silently: an unexpected WRITE
		// must be caught by the argv assertions, not masked by a stub error.
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })

	oldNow := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = oldNow })
}

// mutating reports every gh verb in the recorded calls that changes remote state.
func (s *ghStub) mutating() []string {
	var out []string
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		for _, verb := range []string{"pr edit", "pr comment", "label create", "pr close",
			"pr merge", "pr ready", "issue close", "pr review"} {
			if strings.Contains(j, verb) {
				out = append(out, verb)
			}
		}
	}
	return out
}

func runVerb(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch args[0] {
	case "set":
		err = cmdSet(args[1:], &buf)
	case "read":
		err = cmdRead(args[1:], &buf)
	case "sweep":
		err = cmdSweep(args[1:], &buf)
	default:
		t.Fatalf("unknown verb %q", args[0])
	}
	return deskkit.ExitCodeOf(err), buf.String() + errText(err)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return "\nERR: " + err.Error()
}

// TestSetRefusalPathsMakeZeroWrites is the fail-closed control: every refusal must be
// decided BEFORE any outward write. A refusal that has already written a label is not a
// refusal.
func TestSetRefusalPathsMakeZeroWrites(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"repo outside the desk-tools set", []string{"set", "-R", outsideRepo, "--pr", "1", "--verdict", "SUPERSEDED", "--evidence", "https://x/1"}},
		{"no repo", []string{"set", "--pr", "1", "--verdict", "SUPERSEDED", "--evidence", "https://x/1"}},
		{"no PR", []string{"set", "-R", allowedRepo, "--verdict", "SUPERSEDED", "--evidence", "https://x/1"}},
		{"verdict outside the closed vocabulary", []string{"set", "-R", allowedRepo, "--pr", "1", "--verdict", "STALE", "--evidence", "https://x/1"}},
		{"terminal verdict with no evidence", []string{"set", "-R", allowedRepo, "--pr", "1", "--verdict", "SUPERSEDED"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &ghStub{}
			s.install(t)
			code, out := runVerb(t, tc.args...)
			if code != deskkit.ExitRefused {
				t.Errorf("want exit 5 refused, got %d: %s", code, out)
			}
			if got := s.mutating(); len(got) != 0 {
				t.Errorf("a refusal path performed outward writes: %v", got)
			}
			if len(s.calls) != 0 {
				t.Errorf("a refusal path called gh at all: %v", s.calls)
			}
		})
	}
}

// TestSetWritesIndexThenRecord pins the write order and the verb set.
func TestSetWritesIndexThenRecord(t *testing.T) {
	s := &ghStub{replies: []stubReply{
		{match: "pr view", stdout: `{"labels":[{"name":"bug"}],"comments":[{"body":"a prose review comment"}]}`},
	}}
	s.install(t)

	code, out := runVerb(t, "set", "-R", allowedRepo, "--pr", "829",
		"--verdict", "SUPERSEDED", "--evidence", "https://github.com/example-org/tracker/pull/223")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}

	var order []string
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		switch {
		case strings.Contains(j, "pr edit"):
			order = append(order, "label")
		case strings.Contains(j, "pr comment"):
			order = append(order, "record")
		}
	}
	if strings.Join(order, ",") != "label,record" {
		t.Fatalf("want the index written before the record (a marker with no label is invisible to the sweep); got %v", order)
	}
	if !strings.Contains(strings.Join(s.calls[len(s.calls)-1], " "), "--body-file") {
		t.Error("the record body must be passed by file, never inline")
	}
	// The tool must never close, merge or flip anything: that is deskclose's act.
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		for _, forbidden := range []string{"pr close", "pr merge", "pr ready", "issue close", "pr review"} {
			if strings.Contains(j, forbidden) {
				t.Errorf("deskdisposition must never construct %q; got %q", forbidden, j)
			}
		}
	}
	if !strings.Contains(out, "deskclose") {
		t.Error("the output must say the close is deskclose's, not this tool's")
	}
	if !strings.Contains(strings.Join(s.calls[0], " "), "pr view") {
		t.Error("set must READ the PR before writing — it refuses on an unknown current state")
	}
}

// TestSetIsIdempotent — the sweep re-runs this verb on every pass; a second identical
// record must cost zero writes.
func TestSetIsIdempotent(t *testing.T) {
	rec := deskkit.Disposition{
		Verdict:    deskkit.DispositionSuperseded,
		Evidence:   "https://github.com/example-org/tracker/pull/223",
		RecordedBy: "earlier-session",
		RecordedAt: "2026-08-09",
	}
	body, err := jsonQuote(rec.Marker())
	if err != nil {
		t.Fatal(err)
	}
	s := &ghStub{replies: []stubReply{
		{match: "pr view", stdout: `{"labels":[{"name":"disposition:superseded"}],"comments":[{"body":` + body + `}]}`},
	}}
	s.install(t)

	code, out := runVerb(t, "set", "-R", allowedRepo, "--pr", "829",
		"--verdict", "SUPERSEDED", "--evidence", "https://github.com/example-org/tracker/pull/223")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	if !strings.Contains(out, "noop") {
		t.Errorf("an identical record must no-op; got: %s", out)
	}
	if got := s.mutating(); len(got) != 0 {
		t.Errorf("a no-op performed writes: %v", got)
	}
}

// TestSetRefusesWhenItCannotReadCurrentState — three-state on the write path: a record
// written over an unknown state could silently contradict an existing one.
func TestSetRefusesWhenItCannotReadCurrentState(t *testing.T) {
	s := &ghStub{replies: []stubReply{{match: "pr view", fail: true}}}
	s.install(t)
	code, out := runVerb(t, "set", "-R", allowedRepo, "--pr", "1", "--verdict", "NEEDS-REBASE")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6 unverifiable, got %d: %s", code, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("the refusal must say could-not-check; got: %s", out)
	}
	if got := s.mutating(); len(got) != 0 {
		t.Errorf("an unverifiable read still wrote: %v", got)
	}
}

// TestSweepClassifiesFromTheIndex is the GitHub regression: the same three-state
// classification, now read through the resolved forge's ListOpenChanges rather than
// `gh pr list` (#1123).
func TestSweepClassifiesFromTheIndex(t *testing.T) {
	s := &ghStub{}
	s.install(t)
	sf := &stubForge{openChanges: &deskkit.OpenChanges{Cap: 100, Changes: []deskkit.OpenChange{
		{Number: 829, Title: "stale tracker work", Labels: []string{"disposition:superseded"}},
		{Number: 900, Title: "live work", Labels: []string{"bug"}},
		{Number: 901, Title: "blocked work", Labels: []string{"disposition:needs-rebase"}},
	}}}
	installStubForge(t, sf)

	code, out := runVerb(t, "sweep", "-R", allowedRepo)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("want one line per open PR, got %d: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "checked-failed") || !strings.Contains(lines[0], "false") {
		t.Errorf("#829 carries SUPERSEDED and must not be dispatch-eligible — this is the 8-of-10 waste; got %q", lines[0])
	}
	if !strings.Contains(lines[1], "checked-clean") || !strings.Contains(lines[1], "true") {
		t.Errorf("#900 has no record and must stay dispatchable; got %q", lines[1])
	}
	if !strings.Contains(lines[2], "true") {
		t.Errorf("NEEDS-REBASE is live work and stays dispatchable; got %q", lines[2])
	}
	if got := s.mutating(); len(got) != 0 {
		t.Errorf("a sweep is read-only; got writes %v", got)
	}
	// One bounded forge read per repo: a per-PR comment fetch across ~80 open PRs is the
	// fan-out that trips GitHub's secondary rate limit.
	if sf.listCalls != 1 {
		t.Errorf("sweep must be ONE open-change read per repo, got %d", sf.listCalls)
	}
	// And NO forge CLI at all: the whole defect (#1123) was that this read went out as
	// `gh pr list` regardless of which forge served the repo.
	if len(s.calls) != 0 {
		t.Errorf("sweep must not shell out at all; got %v", s.calls)
	}
}

// TestSweepReadsGitLabMergeRequests is #1123's reproduction. `sweep -R <gitlab project>`
// shelled `gh pr list -R <owner/name>`, which asks GITHUB about a slug that is not a GitHub
// repository: the answer was `Could not resolve to a Repository with the name …` and the
// verb reported the project's WHOLE queue as could-not-check (exit 6). A GitLab adopter's
// orphan sweep could therefore never look at all.
//
// The forge here is a REAL deskkit.GitLabForge pointed at an httptest instance serving the
// project merge-requests endpoint, so the case exercises the backend's own mapping (MR iid →
// number, flat GitLab labels, draft-prefix strip) rather than a double that would pass
// whatever the code asked for. Pre-fix this test fails: the ambient-gh stub below stands in
// for the GitHub resolution failure and the sweep exits 6 on an empty queue.
func TestSweepReadsGitLabMergeRequests(t *testing.T) {
	const glProject = "medici-finance/assay"
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/merge_requests") {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[`+
				`{"iid":7,"state":"opened","title":"Draft: supersede the thing","sha":"abc123",`+
				`"source_branch":"feat/x","target_branch":"main","created_at":"2026-09-01T09:00:00Z",`+
				`"labels":["disposition:superseded"],"author":{"id":99,"username":"worker-bot"}},`+
				`{"iid":9,"state":"opened","title":"live work","sha":"def456",`+
				`"source_branch":"feat/y","target_branch":"main","created_at":"2026-09-02T09:00:00Z",`+
				`"labels":["bug"],"author":{"id":99,"username":"worker-bot"}}]`)
			return
		}
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	defer srv.Close()

	// The ambient-gh shape a GitLab slug produced: `gh pr list` cannot resolve the project.
	s := &ghStub{replies: []stubReply{{match: "pr list", fail: true}}}
	s.install(t)
	installForge(t, func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return &deskkit.GitLabForge{Token: "test-injected-token-0000", BaseURL: srv.URL, Client: srv.Client()},
			deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	})

	code, out := runVerb(t, "sweep", "-R", glProject)
	if code != deskkit.ExitOK {
		t.Fatalf("a GitLab project's queue must be READABLE, not UNKNOWN (#1123); got exit %d: %s", code, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per open merge request, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "7\t") || !strings.Contains(lines[0], "checked-failed") ||
		!strings.Contains(lines[0], "false") || !strings.Contains(lines[0], "supersede the thing") {
		t.Errorf("!7 carries SUPERSEDED and must not be dispatch-eligible; got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "9\t") || !strings.Contains(lines[1], "checked-clean") ||
		!strings.Contains(lines[1], "true") {
		t.Errorf("!9 has no record and must stay dispatchable; got %q", lines[1])
	}
	// The read went to GitLab's merge-request endpoint, not to any forge CLI.
	if len(gotPaths) == 0 || !strings.HasSuffix(gotPaths[0], "/merge_requests") {
		t.Errorf("want the project merge-requests read; got %v", gotPaths)
	}
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "pr list") {
			t.Fatalf("sweep shelled `gh pr list` against a GitLab project — the #1123 defect: %v", c)
		}
	}
	if got := s.mutating(); len(got) != 0 {
		t.Errorf("a sweep is read-only; got writes %v", got)
	}
}

// TestSweepFailureIsCouldNotCheckNotEmpty — the #777 empty-board failure, guarded. Both
// halves of the read can fail: resolving the forge for the repo, and the open-change read
// itself. Neither may be reported as an empty queue.
func TestSweepFailureIsCouldNotCheckNotEmpty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		install func(t *testing.T)
	}{
		{"the forge cannot be resolved", func(t *testing.T) {
			installForgeError(t, deskkit.Unverifiable("no forge resolved for this repo", nil))
		}},
		{"the open-change read fails", func(t *testing.T) {
			installStubForge(t, &stubForge{failChanges: errors.New("HTTP 403: forbidden")})
		}},
		{"the forge returns no result", func(t *testing.T) {
			installStubForge(t, &stubForge{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &ghStub{}
			s.install(t)
			tc.install(t)
			code, out := runVerb(t, "sweep", "-R", allowedRepo)
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("want exit 6, got %d: %s", code, out)
			}
			if !strings.Contains(out, "could-not-check") || !strings.Contains(out, "UNKNOWN, not empty") {
				t.Errorf("a sweep that could not look must never read as an empty queue; got: %s", out)
			}
		})
	}
}

func TestReadReportsCouldNotCheck(t *testing.T) {
	// `read` now serves from the App-token forge, never `gh` (#984) — the ghStub is
	// installed anyway so a stray `gh` call would still be caught as an unexpected
	// mutation/argv, but the failure this test drives is a forge-level GetIssue error.
	s := &ghStub{}
	s.install(t)
	installStubForge(t, &stubForge{failIssue: errors.New("HTTP 404: not found")})
	code, out := runVerb(t, "read", "-R", allowedRepo, "--pr", "1")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6, got %d: %s", code, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("got: %s", out)
	}
}

func TestReadEmitsTheEvidenceForDeskclose(t *testing.T) {
	rec := deskkit.Disposition{
		Verdict:    deskkit.DispositionSuperseded,
		Evidence:   "https://github.com/example-org/tracker/pull/223",
		RecordedBy: "earlier-session",
		RecordedAt: "2026-08-09",
	}
	s := &ghStub{}
	s.install(t)
	installStubForge(t, &stubForge{
		labels:   []string{"disposition:superseded"},
		comments: []string{rec.Marker()},
	})
	code, out := runVerb(t, "read", "-R", allowedRepo, "--pr", "829")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	for _, want := range []string{"checked-failed", "SUPERSEDED", "dispatch-eligible=false", rec.Evidence} {
		if !strings.Contains(out, want) {
			t.Errorf("deskclose needs %q in the read output; got: %s", want, out)
		}
	}
}

// TestReadNeverShellsToAmbientGH is #984's defect-2 reproduction: `deskclose
// superseded`'s confirm path runs as a child of an already-minted desk session whose
// environment carries no usable ambient `gh` identity. Shelling to `gh pr view` there
// came back `HTTP 401: Requires authentication` even though a valid App-token forge read
// was available — read had no way to use it. This plants exactly that shape (a `gh pr
// view` stub that always 401s) alongside a forge stub carrying a real record, and proves
// `read` answers from the forge and never touches `gh` at all.
func TestReadNeverShellsToAmbientGH(t *testing.T) {
	rec := deskkit.Disposition{
		Verdict:    deskkit.DispositionSuperseded,
		Evidence:   "https://github.com/example-org/tracker/pull/40",
		RecordedBy: "earlier-session",
		RecordedAt: "2026-08-09",
	}
	s := &ghStub{replies: []stubReply{
		// Simulates the isolated-session shape: `gh pr view` always fails as an
		// unauthenticated call would, regardless of what a human's own terminal sees.
		{match: "pr view", fail: true},
	}}
	s.install(t)
	installStubForge(t, &stubForge{
		labels:   []string{"disposition:superseded"},
		comments: []string{rec.Marker()},
	})
	code, out := runVerb(t, "read", "-R", allowedRepo, "--pr", "40")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 (served from the forge, not the broken ambient gh), got %d: %s", code, out)
	}
	if !strings.Contains(out, "SUPERSEDED") {
		t.Errorf("expected the forge-served record in the output; got: %s", out)
	}
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "pr view") {
			t.Fatalf("read shelled out to `gh pr view` — it must read via the App-token forge, "+
				"never ambient gh: %v", c)
		}
	}
}

func TestUnknownSubcommandRefuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	if got := run([]string{"close"}); got != deskkit.ExitRefused {
		t.Fatalf("an unknown subcommand must refuse (exit 5), got %d", got)
	}
	if got := run([]string{}); got != deskkit.ExitRefused {
		t.Fatalf("no args must refuse (exit 5), got %d", got)
	}
}

// jsonQuote renders s as a JSON string literal, for embedding a marker body in a stub
// payload without hand-escaping it.
func jsonQuote(s string) (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

func TestMainHelpIsOK(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	if got := run([]string{"--help"}); got != deskkit.ExitOK {
		t.Fatalf("--help must exit 0, got %d", got)
	}
	// The usage text is the operator's only view of the vocabulary; it must list all
	// of it, or a worker will invent a verdict word.
	var buf bytes.Buffer
	buf.WriteString(usage)
	for _, v := range deskkit.DispositionVerdicts() {
		if !strings.Contains(buf.String(), string(v)) {
			t.Errorf("usage omits the %s verdict", v)
		}
	}
	_ = os.Stdout
}
