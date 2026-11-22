package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitcore/gittest"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type showForge struct {
	deskkit.Forge
	issue                  *deskkit.Issue
	history                *deskkit.TrustPayload
	comments               []deskkit.Comment
	branchFile, pinnedFile *deskkit.FileContent
	gotRepo                string
	gotNumber              int
}

func (f *showForge) GetIssueTyped(repo deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	if kind != deskkit.TargetIssue {
		panic("wrong kind")
	}
	f.gotRepo, f.gotNumber = repo.Slug(), n
	return f.issue, nil
}
func (f *showForge) IssueTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	return f.history, nil
}
func (f *showForge) ListCommentsTyped(deskkit.ForgeRepo, int, deskkit.TargetKind) ([]deskkit.Comment, error) {
	return f.comments, nil
}
func (f *showForge) RepoDefaultBranch(deskkit.ForgeRepo) (string, error) { return "main", nil }
func (f *showForge) ReadFile(_ deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	if in.Ref == "main" {
		return f.branchFile, nil
	}
	return f.pinnedFile, nil
}

func readyDecisionFixture(t *testing.T) *showForge {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".config", "assay")
	t.Setenv("HOME", filepath.Dir(filepath.Dir(dir)))
	t.Setenv("USERPROFILE", filepath.Dir(filepath.Dir(dir)))
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\nASSAY_TRUSTED_BOT_SLUGS=worker=example-worker:300000006\nASSAY_ALLOWED_REPOS=example-org/tracker:ci:private\n"
	if e := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0600); e != nil {
		t.Fatal(e)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	f := &showForge{history: &deskkit.TrustPayload{BodyHistoryKnown: true, Complete: true}, issue: &deskkit.Issue{Number: 7, Title: "Select the delivery date", State: "open", URL: "https://github.com/example-org/tracker/issues/7", Author: deskkit.Account{Login: "ada", ID: 2001, Type: "User"}, Labels: []string{"needs-decision"}, Body: "## Options\nA. Today\nB. Tomorrow (recommended)"}}
	old := forgeFor
	forgeFor = func(slug string, _ ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
		o, n, _ := strings.Cut(slug, "/")
		return f, deskkit.ForgeRepo{Owner: o, Name: n}, nil
	}
	t.Cleanup(func() { forgeFor = old })
	return f
}

// regression: class decision-routing
func TestShowIssueIdentityAndAuthority(t *testing.T) {
	f := readyDecisionFixture(t)
	call := func(args ...string) (int, string) {
		var out, err bytes.Buffer
		rc := run(args, &out, &err, time.Now())
		return rc, out.String() + err.String()
	}
	if code, out := call("show", "example-org/tracker#7"); code != 0 || !strings.Contains(out, "Next owner: human") || !strings.Contains(out, "B. Tomorrow (recommended)") || f.gotNumber != 7 || f.gotRepo != "example-org/tracker" {
		t.Fatalf("%d %s", code, out)
	}
	f.issue.Labels = append(f.issue.Labels, deskkit.VerifyGateLabel)
	if code, out := call("show", "example-org/tracker#7"); code != 0 || !strings.Contains(out, "Separate the unresolved decision") {
		t.Fatalf("%d %s", code, out)
	}
	f.issue.Labels = []string{"needs-decision"}
	for _, bad := range []string{"7", "example-org/tracker#0", "../repo#7", "org/repo#7#8", "org/repo#+7"} {
		f.gotNumber = 0
		if code, out := call("show", bad); code != 5 || f.gotNumber != 0 {
			t.Fatalf("%s: %d %s", bad, code, out)
		}
	}
	for _, test := range []struct {
		state string
		run   func()
		code  int
	}{
		{"unknown", func() { f.history.BodyHistoryKnown = false }, 6},
		{"prepare", func() { f.history.BodyHistoryKnown = true; f.issue.Author.ID = 999 }, 0},
		{"closed", func() { f.issue.State = "closed" }, 0},
	} {
		test.run()
		if code, out := call("show", "example-org/tracker#7"); code != test.code || !strings.Contains(out, "State: "+test.state) {
			t.Fatalf("%d %s", code, out)
		}
	}
}

// regression: class decision-routing
func TestShowDecisionScreens(t *testing.T) {
	f := readyDecisionFixture(t)
	a := inboxAction{URL: f.issue.URL}
	for _, tc := range []struct{ body, state string }{
		{"## Options\nA. One option", "screened"},
		{"option: one route", "screened"},
		{"default: A\ncaught-by: review", "screened"},
		{"default: A\ncaught-by: review\nclass: irreversible", "prepare"},
		{"~~~\n## Options\nA. Example\n~~~", "prepare"},
		{"<!--\ndefault: A\ncaught-by: foo\n-->", "prepare"},
		{"## Options\nA. Keeps going\n- Another unparsed option", "prepare"},
	} {
		f.issue.Body = tc.body
		if a := screenDecision(f.issue, a, nil); a.State != tc.state {
			t.Errorf("%s: %+v", tc.body, a)
		}
	}
	f.issue.Body = "## Options\nA. Today\nB. Tomorrow"
	stamp := "2026-01-12T10:00:00Z"
	ruling := deskkit.Comment{Author: f.issue.Author, Body: "A.", CreatedAt: stamp, UpdatedAt: stamp}
	bot := deskkit.Comment{Author: deskkit.Account{Login: "example-worker[bot]", ID: 300000006, Type: "Bot"}, Body: "CI refreshed.", CreatedAt: stamp, UpdatedAt: stamp}
	for _, note := range []string{"CI refreshed.", f.issue.Body} {
		bot.Body = note
		if a := screenDecision(f.issue, a, []deskkit.Comment{ruling, bot}); a.State != "screened" {
			t.Fatalf("%s: %+v", note, a)
		}
	}
	bot.Body = "## Options\nA. Monday\nB. Tuesday"
	if a := screenDecision(f.issue, a, []deskkit.Comment{ruling, bot}); a.State != "ready" || !strings.Contains(a.Evidence, "Tuesday") {
		t.Fatal(a)
	}
	ruling.UpdatedAt = ""
	if a := screenDecision(f.issue, a, []deskkit.Comment{ruling}); a.State != "unknown" {
		t.Fatal(a)
	}
}

// regression: class verification-provenance
func TestShowCanonicalGate(t *testing.T) {
	fixture := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(fixture.Dir, ".github/workflows"), 0700); err != nil {
		t.Fatal(err)
	}
	sha := fixture.CommitFile(t, ".github/workflows/verify-gate-close.yml", "name: verify-gate-close\n", "install close workflow")
	p := acceptancePacket{Repository: "example-org/tracker", Purpose: "verification-acceptance", Local: "example/01", Source: sha, Run: "deadbeef", SourceURL: "https://github.com/example-org/tracker/blob/" + sha + "/docs/streams/example/01.md"}
	a := inboxAction{URL: "https://github.com/example-org/tracker/issues/7"}
	body := "generated current card"
	want := body
	oldRunner, oldLook := runReaderFn, lookPathFn
	t.Cleanup(func() { runReaderFn, lookPathFn = oldRunner, oldLook })
	lookPathFn = func(string) (string, error) { return "/fixture/statusgen", nil }
	failClose := false
	runReaderFn = func(_ string, args []string) ([]byte, []byte, error) {
		if strings.Join(args, " ") == "--root "+fixture.Dir+" --verify-issues" {
			b, e := json.Marshal([]any{map[string]any{"body": want, "card": p}})
			return b, nil, e
		}
		if strings.Join(args, " ") != "verify-gate-close --root "+fixture.Dir+" --ref example/01 --dry-run" {
			t.Fatalf("unexpected invocation %v", args)
		}
		if failClose {
			return nil, []byte("required row held"), errors.New("held")
		}
		return []byte("would mark done"), nil, nil
	}
	runGate := func() inboxAction { return inspectGate(a, fixture.Dir, p.Repository, sha, body, p) }
	if got := runGate(); got.State != "ready" || !strings.Contains(got.Act, a.URL) || got.Verify == "" {
		t.Fatal(got)
	}
	failClose = true
	if got := runGate(); got.State != "prepare" || !strings.Contains(got.Act, "held") {
		t.Fatal(got)
	}
	want = "changed evidence"
	if got := runGate(); got.State != "stale" {
		t.Fatal(got)
	}
	if e := os.WriteFile(filepath.Join(fixture.Dir, "dirty"), []byte("unmerged"), 0600); e != nil {
		t.Fatal(e)
	}
	if got := runGate(); got.State != "unknown" {
		t.Fatal(got)
	}
}

// regression: class verified-runsheet
func TestShowRunsheet(t *testing.T) {
	f := readyDecisionFixture(t)
	ref := "https://github.com/example-org/tracker/blob/" + strings.Repeat("b", 40) + "/docs/run.md"
	f.issue.Labels = []string{"help wanted"}
	f.issue.Body = "[Runsheet](" + ref + ")"
	file := &deskkit.FileContent{Exists: true, Kind: deskkit.FileKindRegular, Content: []byte("example-org/tracker#7\n! perform-reviewed-task\nWhy: only the account holder can approve.\nWhat the session did: prepared the approval.\nResume: read the receipt.")}
	f.pinnedFile, f.branchFile = file, file
	check := func(want string) {
		t.Helper()
		a, e := readInboxAction("example-org/tracker", 7)
		if e != nil || a.State != want {
			t.Fatalf("%s: %+v %v", want, a, e)
		}
	}
	check("ready")
	var showOut, showErr bytes.Buffer
	normal := string(file.Content)
	runShow := func() string {
		t.Helper()
		showOut.Reset()
		showErr.Reset()
		if rc := run([]string{"show", "example-org/tracker#7"}, &showOut, &showErr, time.Now()); rc != 0 {
			t.Fatalf("show: %d %s", rc, showErr.String())
		}
		return showOut.String()
	}
	command := "printf '%s' 'two words' > proof.txt  "
	file.Content = []byte(strings.ReplaceAll(normal, "perform-reviewed-task", command))
	f.issue.Body += "\n! command-from-issue"
	if out := runShow(); !strings.Contains(out, "Command:\n"+command+"\n") || !strings.Contains(out, ref+"#L2") || strings.Contains(out, "command-from-issue") {
		t.Fatal(out)
	}
	file.Content = []byte(strings.ReplaceAll(normal, "perform-reviewed-task", "bad\x1bcommand"))
	if out := runShow(); strings.Contains(out, "Command:\n") || strings.Contains(out, "bad") || !strings.Contains(out, ref+"#L2") {
		t.Fatal(out)
	}
	file.Content = []byte(normal)
	f.comments = []deskkit.Comment{{Body: "Status check complete.", Author: deskkit.Account{Login: "example-worker[bot]", ID: 300000006, Type: "Bot"}}}
	check("ready")
	f.branchFile = &deskkit.FileContent{Exists: false}
	check("stale")
	f.issue.Body = "[Runsheet](https://example.invalid/bogus)"
	check("prepare")
	f.issue.Body = "[Runsheet](" + ref + ")"
	f.branchFile = file
	file.Content = append(file.Content, []byte("\n! another-act")...)
	check("prepare")
}
