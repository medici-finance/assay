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
	headSeries             [][]deskkit.RepoCommit
	commentsErr            error
	commits                []deskkit.RepoCommit
	commitsErr             error
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
	return f.comments, f.commentsErr
}
func (f *showForge) ListRecentCommits(deskkit.ForgeRepo, int) ([]deskkit.RepoCommit, error) {
	if len(f.headSeries) > 0 {
		head := f.headSeries[0]
		f.headSeries = f.headSeries[1:]
		return head, f.commitsErr
	}
	return f.commits, f.commitsErr
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
		{"option: one route\nclass: one-way", "prepare"},
		{"default: A\ncaught-by: review", "screened"},
		{"default: A\ncaught-by: review\nclass: irreversible", "prepare"},
		{"default: A\ncaught-by: review\n<!--note\n    -->\nclass: one-way", "prepare"},
		{"default: A\ncaught-by: review\n    class: one-way", "prepare"},
		{"default: A\ncaught-by: review\n- note\n\n    class: one-way", "prepare"},
		{"default: A\ncaught-by: review\n`<!--`\n\nclass: one-way", "prepare"},
		{"default: A\ncaught-by: review\ntext <!--\n\nclass: one-way", "prepare"},
		{"default: A\ncaught-by: review\n<!-- class: reserved -->", "prepare"},
		{"default: A\ncaught-by: review\ncla<!-- seam -->ss: high-risk", "prepare"},
		{"default: A\ncaught-by: review\nthis step is one‑way", "prepare"},
		{"default: A\ncaught-by: review\n_class_: reserved", "prepare"},
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
	// Both the original issue and the latest admitted proposal remain veto
	// sources. In particular, filtering either before reading its class can
	// transfer an irreversible choice to the desk.
	for _, tc := range []struct{ issue, proposal, want string }{
		{f.issue.Body, "## Options\nA. Only route\n    class: one-way", "prepare"},
		{f.issue.Body, "## Options\nA. Only route\n<!-- class: one-way -->", "prepare"},
		{f.issue.Body, "## Options\nA. Monday\nB. Tuesday\n\n## Defaults\ndefault: A\ncaught-by: review\n    class: one-way", "ready"},
		{f.issue.Body + "\n<!-- class: one-way -->", "## Options\nA. Only route", "prepare"},
		{f.issue.Body + "\ncla<!-- key -->ss: reserved", "## Options\nA. Only route", "prepare"},
		{f.issue.Body, "## Options\nA. Only route\ncla<!-- key -->ss: reserved", "prepare"},
	} {
		source := *f.issue
		source.Body = tc.issue
		proposal := bot
		proposal.Body = tc.proposal
		if got := screenDecision(&source, a, []deskkit.Comment{proposal}); got.State != tc.want {
			t.Errorf("proposal %q: want %s, got %s", tc.proposal, tc.want, got.State)
		}
	}
	bot.Minimized = true
	if got := screenDecision(f.issue, a, []deskkit.Comment{ruling, bot}); got.State != "screened" {
		t.Fatalf("minimized proposal: %+v", got)
	}
	bot.Minimized = false
	bot.UpdatedAt = "2026-01-12T12:00:00Z"
	if got := screenDecision(f.issue, a, []deskkit.Comment{ruling, bot}); got.State != "unknown" {
		t.Fatalf("edited proposal: %+v", got)
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
	f := readyDecisionFixture(t)
	if _, err := fixture.Git("remote", "add", "origin", "https://github.com/example-org/tracker.git"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+fixture.Dir)
	f.commits = []deskkit.RepoCommit{{SHA: sha}}
	f.issue.Author = deskkit.Account{Login: "github-actions[bot]", ID: 88, Type: "Bot"}
	f.issue.Labels = []string{"verify-gate"}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	body := "<!-- verify-gate: example/01 -->\n\n<!-- assay-card: " + string(raw) + " -->\n\nPacket."
	f.issue.Body = body
	want := body
	oldRunner, oldLook := runReaderFn, lookPathFn
	t.Cleanup(func() { runReaderFn, lookPathFn = oldRunner, oldLook })
	lookPathFn = func(string) (string, error) { return "/fixture/statusgen", nil }
	var closeErr error
	runReaderFn = func(_ string, args []string) ([]byte, []byte, error) {
		if strings.Join(args, " ") == "--root "+fixture.Dir+" --verify-issues" {
			b, e := json.Marshal([]any{map[string]any{"body": want, "card": p}})
			return b, nil, e
		}
		if strings.Join(args, " ") != "verify-gate-close --root "+fixture.Dir+" --ref example/01 --dry-run" {
			t.Fatalf("unexpected invocation %v", args)
		}
		if closeErr != nil {
			return nil, []byte("required row held"), closeErr
		}
		return []byte("would mark done"), nil, nil
	}
	runGate := func() inboxAction {
		t.Helper()
		state, err := readInboxAction(p.Repository, f.issue.Number)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	if got := runGate(); got.State != "ready" || !strings.Contains(got.Act, f.issue.URL) || got.Verify == "" {
		t.Fatal(got)
	}
	f.headSeries = [][]deskkit.RepoCommit{{{SHA: sha}}, {{SHA: strings.Repeat("a", 40)}}}
	if got := runGate(); got.State != "unknown" || !strings.Contains(got.Act, "snapshot changed") {
		t.Fatalf("moved branch: %+v", got)
	}
	closeErr = readerExit(1)
	if got := runGate(); got.State != "prepare" || !strings.Contains(got.Act, "held") {
		t.Fatal(got)
	}
	for _, err := range []error{readerExit(2), readerExit(5), readerExit(6), errors.New("cannot launch reader")} {
		closeErr = err
		if got := runGate(); got.State != "unknown" {
			t.Errorf("close reader %v: %s", err, got.State)
		}
	}
	closeErr = nil
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

type readerExit int

func (e readerExit) Error() string { return "reader exited" }
func (e readerExit) ExitCode() int { return int(e) }

// regression: class decision-routing
func TestShowTrustGuards(t *testing.T) {
	f := readyDecisionFixture(t)
	read := func(want string) inboxAction {
		t.Helper()
		a, err := readInboxAction("example-org/tracker", 7)
		if err != nil || a.State != want {
			t.Fatalf("want %s, got %+v, %v", want, a, err)
		}
		return a
	}
	f.history.BodyEdited = time.Now()
	read("prepare")
	f.history.BodyEdited = time.Time{}
	f.commentsErr = errors.New("comments unavailable")
	read("unknown")
	f.commentsErr = nil
	stamp := "2026-01-12T10:00:00Z"
	ruling := deskkit.Comment{Author: f.issue.Author, Body: "A.", CreatedAt: stamp, UpdatedAt: stamp}
	for name, c := range map[string]deskkit.Comment{
		"wrong identity": {Author: deskkit.Account{Login: "another-user", ID: 99, Type: "User"}, Body: "A.", CreatedAt: stamp, UpdatedAt: stamp},
		"minimized":      {Author: f.issue.Author, Minimized: true, Body: "A.", CreatedAt: stamp, UpdatedAt: stamp},
	} {
		t.Run(name, func(t *testing.T) { f.comments = []deskkit.Comment{c}; read("ready") })
	}
	ruling.Body = "A. Hold until Monday"
	f.comments = []deskkit.Comment{ruling}
	read("prepare")
	ruling.Body = "A."
	f.comments = []deskkit.Comment{ruling, {Author: deskkit.Account{Login: "other[bot]", ID: 12, Type: "Bot"}, Body: "## Options\nA. Supply\nB. Store", CreatedAt: stamp, UpdatedAt: stamp}}
	read("screened")
	blank := t.TempDir()
	t.Setenv("HOME", blank)
	t.Setenv("USERPROFILE", blank)
	deskkit.ReloadConfig()
	if deskkit.EffectiveConfig().Configured() {
		t.Fatal("fixture roster still configured")
	}
	for _, l := range []string{"needs-decision", deskkit.VerifyGateLabel} {
		f.issue.Labels = []string{l}
		var out, diag bytes.Buffer
		if rc := run([]string{"show", "example-org/tracker#7"}, &out, &diag, time.Now()); rc != 6 || !strings.Contains(out.String(), "roster") {
			t.Errorf("no roster %s: exit %d, %s", l, rc, out.String())
		}
	}
}

// regression: class verification-provenance
func TestShowAcceptanceInput(t *testing.T) {
	f := readyDecisionFixture(t)
	issue := f.issue
	issue.Author = deskkit.Account{Login: "github-actions[bot]", ID: 88, Type: "Bot"}
	issue.Labels = []string{"VeRiFy-GaTe"}
	packet := acceptancePacket{Repository: "example-org/tracker", Local: "example/01", Purpose: "verification-acceptance", Source: strings.Repeat("b", 40), Run: "abcd"}
	raw, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	body := "<!-- verify-gate: example/01 -->\n\n<!-- assay-card: " + string(raw) + " -->\n\nPacket."
	issue.Body = body
	check := func(want, why string) {
		t.Helper()
		a, e := readInboxAction("example-org/tracker", 7)
		if e != nil || a.State != want || !strings.Contains(a.Act, why) {
			t.Fatalf("%s %s: %+v %v", want, why, a, e)
		}
	}
	issue.Author.Login = "other[bot]"
	check("prepare", "workflow-issued")
	issue.Author.Login = "github-actions[bot]"
	actualURL := issue.URL
	issue.URL = "https://github.com/example-org/other/issues/7"
	check("prepare", "workflow-issued")
	issue.URL = actualURL
	issue.Body = "## Example\n" + body
	check("prepare", "landed verification")
	issue.Body = body
	f.commitsErr = errors.New("failed to read current revision")
	check("unknown", "default-branch")
	f.commitsErr = nil
	f.commits = []deskkit.RepoCommit{{SHA: strings.Repeat("a", 40)}}
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+t.TempDir())
	check("unknown", "local checkout")
}

// regression: class decision-routing
func TestShowScreenOracle(t *testing.T) {
	requireJQ(t)
	f := readyDecisionFixture(t)
	t.Setenv("ASSAY_INBOX_TRUSTED_JSON", `["ada"]`)
	t.Setenv("ASSAY_INBOX_RATIFIER", "ada")
	t.Setenv("ASSAY_INBOX_APPS_JSON", `["example-worker"]`)
	prog := extractJQFMTProgram(t)
	item := jqItem{Repo: "example-org/tracker", Number: 7, Title: "Decision", URL: f.issue.URL, Labels: []jqLabel{{Name: "needs-decision"}}}
	stamp := "2026-01-12T10:00:00Z"
	for _, test := range []struct{ class, body, ruling, prefix string }{
		{"genuine", "## Options\nA. Today\nB. Tomorrow", "", "Reply on "},
		{"no-fork", "## Options\nA. Only one route", "", "No fork."},
		{"reversible-default", "default: A\ncaught-by: review", "", "Apply the documented reversible default."},
		{"already-ruled", "## Options\nA. Today\nB. Tomorrow", "A.", "Already ruled."},
	} {
		t.Run(test.class, func(t *testing.T) {
			f.issue.Body = test.body
			detail := jqDetail{Body: test.body, Author: &jqCommentAuthor{Login: "ada"}}
			var comments []deskkit.Comment
			if test.ruling != "" {
				comments = []deskkit.Comment{{Author: f.issue.Author, Body: test.ruling, CreatedAt: stamp, UpdatedAt: stamp}}
				detail.Comments = []jqComment{{Author: jqCommentAuthor{Login: "ada"}, Body: test.ruling}}
			}
			oracle := runOracleFormat(t, prog, item, detail, 0, 1)
			action := screenDecision(f.issue, inboxAction{URL: f.issue.URL}, comments)
			if oracle.Class != test.class || !strings.HasPrefix(action.Act, test.prefix) {
				t.Fatalf("oracle %s / show %+v", oracle.Class, action)
			}
		})
	}
	t.Run("untrusted source goes to preparation", func(t *testing.T) {
		// The original walk keeps an unclassified question visible. Show
		// must additionally identify who owns preparing a trustworthy act.
		f.issue.Body = "## Options\nA. Today\nB. Tomorrow"
		f.issue.Author = deskkit.Account{Login: "unknown-author", ID: 123, Type: "User"}
		detail := jqDetail{Body: f.issue.Body, Author: &jqCommentAuthor{Login: "unknown-author"}}
		oracle := runOracleFormat(t, prog, item, detail, 0, 1)
		got, err := readInboxAction(item.Repo, item.Number)
		if err != nil || oracle.Class != "genuine" || got.State != "prepare" || got.Owner != "desk" {
			t.Fatalf("oracle %s / show %+v, %v", oracle.Class, got, err)
		}
	})

}
