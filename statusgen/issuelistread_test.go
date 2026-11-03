package main

// issuelistread_test.go — forge-neutral/18: the decision-queue read (--decision-latency,
// --assayscore's Value dimension) and the issue-metrics read (--issues, --self-improvement)
// reach the forge through `deskread issue-list`, never a `gh issue list` shell-out, and every
// list the verb could not serve in full is an error the callers report as could-not-check.
//
// FAIL-FIRST. Before this change the production sources were ghDecisionQueueSource and
// ghIssueMetricLister, which shell `gh issue list --limit 1000`. With `gh` stubbed to fail
// (below), those reads error, so the production assertions here FAIL. The mutation that
// re-reddens them on the fixed tree is pointing either production default back at a `gh`
// shell-out.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ghAnswersShortList is a `gh` stub that answers every call with a one-issue list and exit 0 —
// what `gh issue list --limit 1000` printed for a queue larger than its limit: a short list,
// indistinguishable from a complete one. A read that still reached `gh` takes it as complete.
const ghAnswersShortList = "#!/bin/sh\necho '[{\"number\":1,\"state\":\"OPEN\",\"createdAt\":\"2026-09-01T00:00:00Z\",\"author\":{\"login\":\"someone\"},\"labels\":[]}]'\n"

// ghFails is a `gh` stub that fails every call: the unauthenticated or absent `gh` a read
// must survive.
const ghFails = "#!/bin/sh\necho 'gh: stubbed to fail (issue-list read test)' >&2\nexit 1\n"

// stubIssueListVerbs puts the gh stub and a `deskread` that answers `issue-list` with body
// first on PATH, and returns the file the stub writes its issue-list argv to (one arg a line).
func stubIssueListVerbs(t *testing.T, gh, body string) string {
	t.Helper()
	bin := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "deskread.args")
	writeStubBin(t, bin, "gh", gh)
	writeStubBin(t, bin, "deskread", "#!/bin/sh\n"+
		"[ \"$1\" = issue-list ] || exit 2\n"+
		"printf '%s\\n' \"$@\" > '"+argsFile+"'\n"+
		"cat <<'EOF'\n"+body+"\nEOF\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func readStubArgs(t *testing.T, argsFile string) string {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the deskread stub was never asked for issue-list: %v", err)
	}
	return strings.Join(strings.Fields(string(b)), " ")
}

// regression: the --decision-latency queue read shelled `gh issue list`, so it failed wherever gh was absent or unauthenticated
func TestDecisionQueueReadsThroughDeskread(t *testing.T) {
	argsFile := stubIssueListVerbs(t, ghFails, `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","issues":[`+
		`{"number":7,"state":"closed","createdAt":"2026-08-05T00:00:00Z","closedAt":"2026-08-10T00:00:00Z"},`+
		`{"number":9,"state":"closed","createdAt":"2026-08-06T00:00:00Z","closedAt":"2026-08-08T00:00:00Z"}],`+
		`"incomplete":false,"pageCap":100}],"partial":[]}`)

	got, err := defaultDecisionQueueSource.Issues("example-org/alpha", decisionLabel, "closed")
	if err != nil {
		t.Fatalf("defaultDecisionQueueSource errored with a working deskread and no working gh: %v", err)
	}
	if len(got) != 2 || got[0].Number != 7 || got[0].CreatedAt != "2026-08-05T00:00:00Z" || got[0].ClosedAt != "2026-08-10T00:00:00Z" || got[1].Number != 9 {
		t.Fatalf("decision queue = %+v, want issues 7 and 9 with their envelope timestamps", got)
	}
	if args, want := readStubArgs(t, argsFile), "issue-list --repo example-org/alpha --state closed --label "+decisionLabel; args != want {
		t.Errorf("deskread argv = %q, want %q", args, want)
	}
}

// regression: a decision queue larger than gh's --limit 1000 read as complete; an unservable list must be could-not-check, never short
func TestDecisionQueueUnservedListIsAnError(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"incomplete", `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","issues":[{"number":1,"state":"open"}],"incomplete":true,"pageCap":100}],"partial":[]}`},
		{"no completeness field", `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","issues":[]}],"partial":[]}`},
		{"no issue list", `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","incomplete":false}],"partial":[]}`},
		{"partial", `{"schema":1,"kind":"issue-list","repos":[],"partial":[{"repo":"example-org/alpha","reason":"the App installation cannot read it"}]}`},
		{"repo absent", `{"schema":1,"kind":"issue-list","repos":[],"partial":[]}`},
		{"wrong kind", `{"schema":1,"kind":"issues","repos":[{"repo":"example-org/alpha","issues":[],"incomplete":false}],"partial":[]}`},
		{"no envelope", `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubIssueListVerbs(t, ghAnswersShortList, tc.body)
			got, err := defaultDecisionQueueSource.Issues("example-org/alpha", decisionLabel, "open")
			if err == nil {
				t.Fatalf("defaultDecisionQueueSource returned %+v and no error for an unserved list — it must be could-not-check", got)
			}
		})
	}
}

// regression: the --issues metric read shelled `gh issue list --state all`, so it failed wherever gh was absent or unauthenticated
func TestIssueMetricListerReadsThroughDeskread(t *testing.T) {
	argsFile := stubIssueListVerbs(t, ghFails, `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","issues":[`+
		`{"number":11,"title":"open one","state":"open","authorLogin":"deviant-ozzie","authorType":"User","labels":["bug"," "],"createdAt":"2026-09-01T00:00:00Z"},`+
		`{"number":12,"title":"closed one","state":"closed","authorLogin":"helper-app","authorType":"Bot","labels":["needs-decision"],"createdAt":"2026-09-02T00:00:00Z","closedAt":"2026-09-04T12:00:00Z"}],`+
		`"incomplete":false,"pageCap":100}],"partial":[]}`)

	recs, err := defaultIssueMetricLister("example-org/alpha")
	if err != nil {
		t.Fatalf("defaultIssueMetricLister errored with a working deskread and no working gh: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	open, closed := recs[0], recs[1]
	if open.Number != 11 || open.Repo != "example-org/alpha" || open.OwnerRepo != "example-org/alpha" || open.Title != "open one" ||
		open.Author != "deviant-ozzie" || open.HasClosed || open.CreatedAt.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("open record = %+v", open)
	}
	if len(open.Labels) != 1 || open.Labels[0] != "bug" {
		t.Errorf("open labels = %v, want [bug] (blank names dropped)", open.Labels)
	}
	if !strings.EqualFold(closed.State, "closed") || !closed.HasClosed || closed.ClosedAt.Format("2006-01-02T15") != "2026-09-04T12" {
		t.Errorf("closed record = %+v, want closed with its closedAt", closed)
	}
	// A Bot author served as a bare slug is still an automation identity to the classifier.
	if closed.Author != "helper-app[bot]" || !isBotLogin(closed.Author) {
		t.Errorf("bot author = %q, want helper-app[bot]", closed.Author)
	}
	if args, want := readStubArgs(t, argsFile), "issue-list --repo example-org/alpha --state all"; args != want {
		t.Errorf("deskread argv = %q, want %q", args, want)
	}
}

// regression: a repo with more than 1000 issues was silently truncated by gh --limit 1000 and its metrics read as complete
func TestIssueMetricListerIncompleteIsAnError(t *testing.T) {
	stubIssueListVerbs(t, ghAnswersShortList, `{"schema":1,"kind":"issue-list","repos":[{"repo":"example-org/alpha","issues":[`+
		`{"number":1,"state":"open","createdAt":"2026-09-01T00:00:00Z"}],"incomplete":true,"pageCap":100}],"partial":[]}`)
	recs, err := defaultIssueMetricLister("example-org/alpha")
	if err == nil {
		t.Fatalf("defaultIssueMetricLister returned %d records and no error for a list stopped at its page bound", len(recs))
	}
	if !strings.Contains(err.Error(), "could-not-check") {
		t.Errorf("error %q does not name could-not-check", err)
	}
}
