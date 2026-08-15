package main

import (
	"bytes"
	"encoding/json"
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

func TestSweepClassifiesFromTheIndex(t *testing.T) {
	list := `[
      {"number":829,"title":"stale tracker work","labels":[{"name":"disposition:superseded"}]},
      {"number":900,"title":"live work","labels":[{"name":"bug"}]},
      {"number":901,"title":"blocked work","labels":[{"name":"disposition:needs-rebase"}]}
    ]`
	s := &ghStub{replies: []stubReply{{match: "pr list", stdout: list}}}
	s.install(t)

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
	// One API call per repo: a per-PR comment fetch across ~80 open PRs is the
	// fan-out that trips GitHub's secondary rate limit.
	if len(s.calls) != 1 {
		t.Errorf("sweep must be ONE call per repo, got %d: %v", len(s.calls), s.calls)
	}
}

// TestSweepFailureIsCouldNotCheckNotEmpty — the #777 empty-board failure, guarded.
func TestSweepFailureIsCouldNotCheckNotEmpty(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply stubReply
	}{
		{"list read fails", stubReply{match: "pr list", fail: true}},
		{"list read is unparseable", stubReply{match: "pr list", stdout: "not json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &ghStub{replies: []stubReply{tc.reply}}
			s.install(t)
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
	s := &ghStub{replies: []stubReply{{match: "pr view", fail: true}}}
	s.install(t)
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
	body, err := jsonQuote(rec.Marker())
	if err != nil {
		t.Fatal(err)
	}
	s := &ghStub{replies: []stubReply{
		{match: "pr view", stdout: `{"labels":[{"name":"disposition:superseded"}],"comments":[{"body":` + body + `}]}`},
	}}
	s.install(t)
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
