package main

// harness_test.go — the RECORDING fake forge this package's tests drive the verb through.
//
// It stands in for the far side of the wire at the laneForge seam: every call the verb makes
// is recorded with its operation name, so a test can assert ZERO forge requests (a closed or
// refused config), ZERO writes (an unenacted lane), or exactly which writes happened (an
// ejection). The verb's own code path is not stubbed: the config load, the roster, the
// audit log, the rulings register and the kill-signal file are all real, under a private
// HOME and a temp root.
//
// Every fixture value is an example-org placeholder — no deployment's real area, login or
// repository appears in this package.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	fxRepo     = "example-org/tracker"
	fxPR       = 7
	fxHead     = "11111111111111111111"
	fxOldHead  = "00000000000000000000"
	fxReviewer = "example-reviewer-app[bot]"
	fxWorker   = "example-worker-app[bot]"
	fxDesk     = "example-desk-app[bot]"
	fxSignURL  = "https://github.com/example-org/tracker/issues/3#issuecomment-555"
	fxSizeS    = deskkit.SizeLabelPrefix + "S"
	// fxEnactBody is the acceptance artifact's body: the exact enactment line.
	fxEnactBody = "Enact: R-8"
)

// fixtureRosterBase is the trust roster; the lane keys are appended per test.
const fixtureRosterBase = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,reviewer=example-reviewer-app:300000004,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/open:ci:public
`

// fixtureLaneKeysNoThread opts two example areas in and names NO sign-off thread.
const fixtureLaneKeysNoThread = `ASSAY_AUTOAPPROVE_AREAS=example-org/tracker:docs/notes/**:ada,example-org/tracker:BOARD.md:ada
ASSAY_AUTOAPPROVE_EJECT_LINE=0
ASSAY_AUTOAPPROVE_FPY_FLOOR=0.90
ASSAY_AUTOAPPROVE_DAILY_CAP=2
`

// fixtureLaneKeys is the full lane config: the areas plus the one sign-off thread (#3, where
// fxSignURL's comment sits). It mirrors testdata/roster.env.
const fixtureLaneKeys = fixtureLaneKeysNoThread + `ASSAY_AUTOAPPROVE_SIGNOFF_THREAD=3
`

// The time-check fixtures. The register's history (fakeForge.history) is one commit, fxTextSHA,
// merged by PR #fxTextPR at fxTextMerged; the acceptance comment is created at fxAccepted,
// after it.
const (
	fxTextSHA    = "aaaa0001"
	fxTextPR     = 21
	fxTextMerged = "2026-01-01T00:00:00Z"
	fxAccepted   = "2026-01-02T00:00:00Z"
	// fxAbsent is a history entry's content meaning "the register did not exist at this commit".
	fxAbsent = "<absent>"
)

const rulingsSigned = "# Rulings\n\n## R-8 — the auto-approve lane\n\nText.\n\n**Sign-off:** " + fxSignURL + "\n"
const rulingsUnsigned = "# Rulings\n\n## R-8 — the auto-approve lane\n\nText.\n\n**Sign-off:**\n"

type call struct {
	Op   string
	Arg  string
	Body string
}

func (c call) write() bool { return c.Op == "ApplyLabels" || c.Op == "PostComment" }

// fakeForge serves one PR. Every knob defaults to the admit-within-scope, all-green shape.
type fakeForge struct {
	calls []call

	pr         deskkit.PullRequest
	head2      string // what the SECOND PR read reports as head; "" = unchanged
	prReads    int
	files      []deskkit.ChangedFile
	reviews    []deskkit.Review
	checks     *deskkit.ChecksAtHead
	checks2    *deskkit.ChecksAtHead // what every checks read AFTER the first serves; nil = unchanged
	checkReads int
	required   []string
	events     []deskkit.LabelEvent
	comments   map[int][]deskkit.Comment
	surfaces   string
	noSurfaces bool
	// rulings is the register the forge serves at the default branch ("" = absent), and
	// defaultBranch the repo document's default branch ("" = "main").
	rulings       string
	defaultBranch string

	// history is the register's path history at the default branch, newest first; an entry's
	// content "" is the current register (rulings), fxAbsent an absent file, and its date the
	// commit's own committed date (which the gate must never read). commitPRs are the
	// changes behind each commit, and merged the change reads they resolve to.
	history   []histEntry
	commitPRs map[string][]int
	merged    map[int]deskkit.PullRequest

	fail map[string]bool // operation name → answer an error
}

type histEntry struct {
	sha     string
	content string
	date    string // the commit's own committed date, as the forge reports it; "" = none
}

// mergedPR is a change merged into main at the given time.
func mergedPR(n int, at string) deskkit.PullRequest {
	return deskkit.PullRequest{Number: n, State: "closed", Merged: true, MergedAt: at, BaseRef: "main"}
}

func greenForge() *fakeForge {
	return &fakeForge{
		pr: deskkit.PullRequest{
			Number: fxPR, State: "open", Draft: false, HeadSHA: fxHead, BaseRef: "main",
			Mergeable: "MERGEABLE", ChangedFiles: 2,
			Author: deskkit.Account{Login: fxWorker},
			Body:   "Regenerated notes.\n\nIssue: #12\n",
			Labels: []string{deskkit.AutoLaneLabel, "dispatched-model:example-model", "dispatched-tier:strong", labelAfterFlip, fxSizeS},
		},
		files: []deskkit.ChangedFile{
			{Filename: "docs/notes/2026-01-01.md", Status: "added"},
			{Filename: "BOARD.md", Status: "modified"},
		},
		reviews: []deskkit.Review{
			{ID: 1, Author: deskkit.Account{Login: fxReviewer}, State: "APPROVED", CommitID: fxHead},
		},
		checks: &deskkit.ChecksAtHead{CheckRunsTotalCount: 1, CheckRuns: []deskkit.CheckRun{
			{Name: "test", Status: "completed", Conclusion: "success", CompletedAt: "2026-01-01T00:00:00Z"}}},
		events: []deskkit.LabelEvent{
			{Name: "dispatched-model:example-model", AppliedBy: fxDesk},
			{Name: "dispatched-tier:strong", AppliedBy: fxDesk},
			{Name: deskkit.AutoLaneLabel, AppliedBy: fxReviewer},
			{Name: fxSizeS, AppliedBy: fxReviewer},
		},
		comments: map[int][]deskkit.Comment{
			3: {{DatabaseID: 555, Author: deskkit.Account{Login: "ada", ID: 2001, Type: "User"}, Body: fxEnactBody,
				CreatedAt: fxAccepted}},
		},
		surfaces:  "# declared surfaces\n.github/workflows/**\ntools/**\n",
		history:   []histEntry{{sha: fxTextSHA}},
		commitPRs: map[string][]int{fxTextSHA: {fxTextPR}},
		merged:    map[int]deskkit.PullRequest{fxTextPR: mergedPR(fxTextPR, fxTextMerged)},
		fail:      map[string]bool{},
	}
}

func (f *fakeForge) rec(op, arg, body string) error {
	f.calls = append(f.calls, call{Op: op, Arg: arg, Body: body})
	if f.fail[op] {
		return errors.New("HTTP 500 from the fake forge")
	}
	return nil
}

func (f *fakeForge) writes() []call {
	var out []call
	for _, c := range f.calls {
		if c.write() {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeForge) GetPullRequest(r deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	if err := f.rec("GetPullRequest", fmt.Sprint(n), ""); err != nil {
		return nil, err
	}
	if mp, ok := f.merged[n]; ok && n != f.pr.Number {
		return &mp, nil
	}
	f.prReads++
	pr := f.pr
	pr.Labels = append([]string{}, f.pr.Labels...)
	if f.prReads > 1 && f.head2 != "" {
		pr.HeadSHA = f.head2
	}
	return &pr, nil
}

func (f *fakeForge) ListChangedFiles(r deskkit.ForgeRepo, n int) ([]deskkit.ChangedFile, error) {
	if err := f.rec("ListChangedFiles", fmt.Sprint(n), ""); err != nil {
		return nil, err
	}
	return f.files, nil
}

func (f *fakeForge) ReviewsAtHead(r deskkit.ForgeRepo, n int) ([]deskkit.Review, error) {
	if err := f.rec("ReviewsAtHead", fmt.Sprint(n), ""); err != nil {
		return nil, err
	}
	return f.reviews, nil
}

func (f *fakeForge) ChecksAtHead(r deskkit.ForgeRepo, sha string) (*deskkit.ChecksAtHead, error) {
	if err := f.rec("ChecksAtHead", sha, ""); err != nil {
		return nil, err
	}
	f.checkReads++
	if f.checkReads > 1 && f.checks2 != nil {
		return f.checks2, nil
	}
	return f.checks, nil
}

func (f *fakeForge) RequiredStatusChecks(r deskkit.ForgeRepo, branch string) ([]string, error) {
	if err := f.rec("RequiredStatusChecks", branch, ""); err != nil {
		return nil, err
	}
	return f.required, nil
}

func (f *fakeForge) ListLabelEvents(r deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	if err := f.rec("ListLabelEvents", fmt.Sprint(n), ""); err != nil {
		return nil, err
	}
	return f.events, nil
}

func (f *fakeForge) ListComments(r deskkit.ForgeRepo, n int) ([]deskkit.Comment, error) {
	if err := f.rec("ListComments", r.Slug()+"#"+fmt.Sprint(n), ""); err != nil {
		return nil, err
	}
	return f.comments[n], nil
}

func (f *fakeForge) ReadFile(r deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	if err := f.rec("ReadFile", r.Slug()+":"+in.File+"@"+in.Ref, ""); err != nil {
		return nil, err
	}
	switch {
	case in.File == ".assay-surfaces" && !f.noSurfaces:
		return &deskkit.FileContent{Exists: true, Content: []byte(f.surfaces)}, nil
	case in.File == deskkit.AutoLaneRulingsPath && in.Ref != f.branch():
		for _, h := range f.history {
			if h.sha != in.Ref {
				continue
			}
			c := h.content
			if c == "" {
				c = f.rulings
			}
			if c == fxAbsent || c == "" {
				break
			}
			return &deskkit.FileContent{Exists: true, Content: []byte(c)}, nil
		}
	case in.File == deskkit.AutoLaneRulingsPath && f.rulings != "":
		return &deskkit.FileContent{Exists: true, Content: []byte(f.rulings)}, nil
	}
	return &deskkit.FileContent{Exists: false}, nil
}

func (f *fakeForge) branch() string {
	if f.defaultBranch == "" {
		return "main"
	}
	return f.defaultBranch
}

func (f *fakeForge) ListFileCommits(r deskkit.ForgeRepo, ref, file string, limit int) ([]deskkit.RepoCommit, error) {
	if err := f.rec("ListFileCommits", r.Slug()+":"+file+"@"+ref, ""); err != nil {
		return nil, err
	}
	var out []deskkit.RepoCommit
	for _, h := range f.history {
		if len(out) == limit {
			break
		}
		out = append(out, deskkit.RepoCommit{SHA: h.sha, CommittedDate: h.date})
	}
	return out, nil
}

func (f *fakeForge) ListCommitChanges(r deskkit.ForgeRepo, sha string) ([]int, error) {
	if err := f.rec("ListCommitChanges", r.Slug()+"@"+sha, ""); err != nil {
		return nil, err
	}
	return f.commitPRs[sha], nil
}

func (f *fakeForge) RepoHardeningRead(r deskkit.ForgeRepo, kind deskkit.HardeningReadKind) (json.RawMessage, error) {
	if err := f.rec("RepoHardeningRead", r.Slug()+":"+string(kind), ""); err != nil {
		return nil, err
	}
	return json.RawMessage(`{"default_branch":"` + f.branch() + `"}`), nil
}

func (f *fakeForge) ListCommentsTyped(r deskkit.ForgeRepo, n int, kind deskkit.TargetKind) ([]deskkit.Comment, error) {
	if err := f.rec("ListCommentsTyped", r.Slug()+"#"+fmt.Sprint(n)+":"+string(kind), ""); err != nil {
		return nil, err
	}
	return f.comments[n], nil
}

func (f *fakeForge) ApplyLabels(r deskkit.ForgeRepo, n int, ch deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	var add []string
	for _, a := range ch.Add {
		add = append(add, a.Name)
	}
	body := "add=" + strings.Join(add, ",") + " remove=" + strings.Join(ch.Remove, ",")
	if err := f.rec("ApplyLabels", fmt.Sprint(n), body); err != nil {
		return nil, err
	}
	return &deskkit.LabelOutcome{Added: add, Removed: ch.Remove}, nil
}

func (f *fakeForge) PostComment(r deskkit.ForgeRepo, n int, body string) (*deskkit.CommentRef, error) {
	if err := f.rec("PostComment", fmt.Sprint(n), body); err != nil {
		return nil, err
	}
	f.comments[n] = append(f.comments[n], deskkit.Comment{Author: deskkit.Account{Login: fxReviewer}, Body: body})
	return &deskkit.CommentRef{}, nil
}

// env is one test's installed world.
type env struct {
	t    *testing.T
	home string
	root string
	fg   *fakeForge
}

// install plants the roster (base + laneKeys), the fake forge serving the rulings register at
// the default branch, and the DESK_LOOP of the lane's owning window. laneKeys "" is the
// SHIPPED state: no lane key. The register is served by the FORGE only — nothing is planted
// in the local root, which the enactment gate never reads.
func install(t *testing.T, laneKeys, rulings string) *env {
	t.Helper()
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskautolane-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskautolane-test")
	t.Setenv("DESK_LOOP", laneLoop)
	t.Setenv("ASSAY_RISK_CALLOUT", "")
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRosterBase+laneKeys), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	e := &env{t: t, home: home, root: root, fg: greenForge()}
	e.fg.rulings = rulings
	oldMint, oldResolve := mintTokenFn, resolveForgeFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "stub-token", filepath.Join(dir, role+"-token-stub"), nil
	}
	resolveForgeFn = func(fr deskkit.ForgeRepo, role string) (laneForge, error) {
		if role != reviewerRole {
			t.Errorf("the lane resolved a forge as %q, not the reviewer App", role)
		}
		return e.fg, nil
	}
	t.Cleanup(func() { mintTokenFn, resolveForgeFn = oldMint, oldResolve })
	return e
}

// localRulings plants a register in the caller's local root — the tree a PR-head checkout
// would be. The enactment gate must never read it.
func (e *env) localRulings(content string) {
	p := filepath.Join(e.root, "docs", "streams", "issue-flow")
	if err := os.MkdirAll(p, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "rulings.md"), []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// fpy writes a harvested per-class file under root and returns its path.
func (e *env) fpy(content string) string {
	p := filepath.Join(e.root, "fpy-by-class.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
	return p
}

const healthyFPY = `{"auto-lane":{"n":0,"firstPassYield":"could-not-check","note":"no lane merges yet"}}`

// run invokes the verb with --repo and --root filled in, returning exit code, stdout, stderr.
func (e *env) run(args ...string) (int, string, string) {
	e.t.Helper()
	full := append(append([]string{}, args...), "--repo", fxRepo, "--root", e.root)
	var out, errb bytes.Buffer
	code := run(full, &out, &errb)
	return code, out.String(), errb.String()
}

// auditLines returns the audit log's entries for this tool.
func (e *env) audit() []deskkit.Entry {
	e.t.Helper()
	all, err := deskkit.LoadEntries()
	if err != nil {
		e.t.Fatalf("audit log unreadable: %v", err)
	}
	var out []deskkit.Entry
	for _, a := range all {
		if a.Tool == toolName {
			out = append(out, a)
		}
	}
	return out
}

func (e *env) hasAudit(verb, result string) bool {
	for _, a := range e.audit() {
		if a.Verb == verb && a.Result == result {
			return true
		}
	}
	return false
}

func (e *env) seedAudit(entries ...deskkit.Entry) {
	e.t.Helper()
	for _, en := range entries {
		if err := deskkit.Log(en); err != nil {
			e.t.Fatalf("seeding the audit log: %v", err)
		}
	}
}
