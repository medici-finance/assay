package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The parity corpus under testdata/decisionensure/<case>/ was captured from the repo-local
// shell helper (`decision-issue.sh ensure brief.md --repo o/r --dry-run [flags]`) run against a
// stub forge that answered from the same <case>/forge/*.json files the fake below reads:
//
//	brief.md         the brief (and README.md beside it, for the board-row cases)
//	flags            extra argv, when the case needs any
//	forge/<key>.json what the forge answers for one read; <key>.fail makes that read fail
//	stdout.golden    the helper's stdout, byte for byte
//	stderr.golden    the helper's stderr, where the port's wording is unchanged
//	code.golden      the helper's exit code
//
// Forge keys: `label` is the labelled-population listing; a body-marker search is its query
// text with every non-alphanumeric run folded to `_` (`decision-gate: sample/07` →
// `decision_gate_sample_07`); `view-<N>` is one issue read.

// fakeDecisionForge serves a parity case's forge directory, and records every write.
type fakeDecisionForge struct {
	dir      string
	issues   map[int]fakeDecisionIssue
	labels   []string
	labelErr error
	fileErr  error
	applyErr error
	filed    []IssueInput
	applied  []LabelChange
	searches []string
	nextNum  int
}

type fakeDecisionIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (i fakeDecisionIssue) issue() *Issue {
	out := &Issue{Number: i.Number, Title: i.Title, State: strings.ToLower(i.State), Body: i.Body,
		URL: fmt.Sprintf("https://github.com/o/r/issues/%d", i.Number)}
	for _, l := range i.Labels {
		out.Labels = append(out.Labels, l.Name)
	}
	return out
}

// decisionCorpusRoot is absolute: the cases run from inside their own directories.
var decisionCorpusRoot = func() string {
	p, err := filepath.Abs(filepath.Join("testdata", "decisionensure"))
	if err != nil {
		panic(err)
	}
	return p
}()

var fakeDecisionKeyRe = regexp.MustCompile(`[^A-Za-z0-9]+`)

func newFakeDecisionForge(t *testing.T, dir string) *fakeDecisionForge {
	t.Helper()
	f := &fakeDecisionForge{dir: dir, issues: map[int]fakeDecisionIssue{}, nextNum: 100}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var many []fakeDecisionIssue
		if strings.HasPrefix(e.Name(), "view-") {
			var one fakeDecisionIssue
			if err := json.Unmarshal(raw, &one); err != nil {
				t.Fatalf("%s: %v", e.Name(), err)
			}
			many = append(many, one)
		} else if err := json.Unmarshal(raw, &many); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for _, i := range many {
			f.issues[i.Number] = i
		}
	}
	return f
}

func (f *fakeDecisionForge) read(key string) ([]fakeDecisionIssue, error) {
	if _, err := os.Stat(filepath.Join(f.dir, key+".fail")); err == nil {
		return nil, errors.New("forge read failed: " + key)
	}
	raw, err := os.ReadFile(filepath.Join(f.dir, key+".json"))
	if err != nil {
		return nil, nil
	}
	var out []fakeDecisionIssue
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (f *fakeDecisionForge) GetIssue(_ ForgeRepo, n int) (*Issue, error) {
	i, ok := f.issues[n]
	if !ok {
		return nil, fmt.Errorf("issue #%d: not found", n)
	}
	return i.issue(), nil
}

func (f *fakeDecisionForge) ListIssues(_ ForgeRepo, in IssueListQuery) (*IssueList, error) {
	if in.State != IssueStateAll || in.Label != DecisionLabel {
		return nil, fmt.Errorf("unexpected listing %+v", in)
	}
	got, err := f.read("label")
	if err != nil {
		return nil, err
	}
	out := &IssueList{}
	for _, i := range got {
		out.Issues = append(out.Issues, IssueSummary{Number: i.Number, Title: i.Title})
	}
	return out, nil
}

func (f *fakeDecisionForge) SearchIssues(_ ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	if !in.DecisionGateMarkers {
		return nil, fmt.Errorf("unexpected search %+v", in)
	}
	text := decisionMarkerSearchText(in)
	f.searches = append(f.searches, text)
	got, err := f.read(fakeDecisionKeyRe.ReplaceAllString(text, "_"))
	if err != nil {
		return nil, err
	}
	out := make([]IssueSearchResult, 0, len(got))
	for _, i := range got {
		out = append(out, IssueSearchResult{Number: i.Number, Body: i.Body})
	}
	return out, nil
}

func (f *fakeDecisionForge) ListLabels(ForgeRepo) ([]string, error) { return f.labels, f.labelErr }

func (f *fakeDecisionForge) FileIssue(_ ForgeRepo, in IssueInput) (*IssueRef, error) {
	if f.fileErr != nil {
		return nil, f.fileErr
	}
	f.filed = append(f.filed, in)
	n := f.nextNum
	f.nextNum++
	return &IssueRef{Number: n, URL: fmt.Sprintf("https://github.com/o/r/issues/%d", n)}, nil
}

func (f *fakeDecisionForge) ApplyLabels(_ ForgeRepo, _ int, change LabelChange) (*LabelOutcome, error) {
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	f.applied = append(f.applied, change)
	return &LabelOutcome{}, nil
}

type decisionRun struct {
	res      DecisionEnsureResult
	out, err string
	forged   int
}

// runDecisionCase runs ensure the way the parity capture ran the helper: from the case
// directory, on brief.md, --repo o/r, the case's flags.
func runDecisionCase(t *testing.T, dir string, f *fakeDecisionForge, dryRun bool) decisionRun {
	t.Helper()
	t.Chdir(dir)
	req := DecisionEnsureRequest{BriefFile: "brief.md", Repo: "o/r", At: "start", DryRun: dryRun}
	if raw, err := os.ReadFile("flags"); err == nil {
		args := strings.Fields(string(raw))
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--held":
				req.Held = true
			case "--at":
				i++
				req.At = args[i]
			default:
				t.Fatalf("flags: unhandled %q", args[i])
			}
		}
	}
	var out, errb bytes.Buffer
	req.Out, req.Err = &out, &errb
	run := decisionRun{}
	req.Forge = func(repo ForgeRepo) (DecisionEnsureForge, error) {
		run.forged++
		if repo != (ForgeRepo{Owner: "o", Name: "r"}) {
			t.Fatalf("forge resolved for %+v", repo)
		}
		return f, nil
	}
	run.res = EnsureDecisionIssue(req)
	run.out, run.err = out.String(), errb.String()
	return run
}

// stripCauseLines drops the port's `cause:` detail lines, which the helper does not print.
func stripCauseLines(s string) string {
	var keep []string
	for _, l := range strings.SplitAfter(s, "\n") {
		if !strings.HasPrefix(l, decisionLogPrefix+"cause: ") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "")
}

// regression: #6131
func TestDecisionEnsureParityWithHelper(t *testing.T) {
	root := decisionCorpusRoot
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) < 20 {
		t.Fatalf("parity corpus has %d cases; expected the full captured set", len(ents))
	}
	for _, e := range ents {
		dir := filepath.Join(root, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			wantOut, err := os.ReadFile(filepath.Join(dir, "stdout.golden"))
			if err != nil {
				t.Fatal(err)
			}
			codeRaw, err := os.ReadFile(filepath.Join(dir, "code.golden"))
			if err != nil {
				t.Fatal(err)
			}
			f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
			run := runDecisionCase(t, dir, f, true)
			if got, want := fmt.Sprint(run.res.Code), strings.TrimSpace(string(codeRaw)); got != want {
				t.Errorf("exit = %s, helper exited %s\nstderr:\n%s", got, want, run.err)
			}
			if run.out != string(wantOut) {
				t.Errorf("stdout differs from the helper's\n--- port ---\n%s--- helper ---\n%s", run.out, wantOut)
			}
			if wantErr, err := os.ReadFile(filepath.Join(dir, "stderr.golden")); err == nil {
				if got := stripCauseLines(run.err); got != string(wantErr) {
					t.Errorf("stderr differs from the helper's\n--- port ---\n%s--- helper ---\n%s", got, wantErr)
				}
			}
			if len(f.filed) != 0 || len(f.applied) != 0 {
				t.Errorf("a dry run wrote to the forge: filed=%d applied=%d", len(f.filed), len(f.applied))
			}
		})
	}
}

// The two cases whose stderr the port words neutrally (no house rule ids) still say the
// same thing.
//
// regression: #6131
func TestDecisionEnsureNeutralNotices(t *testing.T) {
	root := decisionCorpusRoot
	for name, want := range map[string]string{
		"dryrun-synth":       "NOTICE: no author default — blocks (no '## Human decision' section",
		"refused-no-default": "has a '## Human decision' section with no parseable default",
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			run := runDecisionCase(t, dir, newFakeDecisionForge(t, filepath.Join(dir, "forge")), true)
			if !strings.Contains(run.err, want) {
				t.Errorf("stderr lacks %q:\n%s", want, run.err)
			}
			if regexp.MustCompile(`\bR-[0-9]`).MatchString(run.err) {
				t.Errorf("stderr cites a rule id the reader cannot resolve:\n%s", run.err)
			}
		})
	}
}

// copyDecisionCase copies a parity case into a temp dir so a test can change it.
func copyDecisionCase(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(decisionCorpusRoot, name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dst, "forge"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

// dryRunBody extracts the composed body from a dry-run's stdout.
func dryRunBody(t *testing.T, out string) (title, body string) {
	t.Helper()
	_, rest, ok := strings.Cut(out, "--- title ---\n")
	if !ok {
		t.Fatalf("no title block in:\n%s", out)
	}
	title, rest, _ = strings.Cut(rest, "\n--- body ---\n")
	body, _, ok = strings.Cut(rest, "\n--- end ---\n")
	if !ok {
		t.Fatalf("no body block in:\n%s", out)
	}
	return title, body
}

// A real run files exactly the body the helper's dry run shows (plus the final newline the
// helper's filing carries), so either side's filing dedupes against the other's.
//
// regression: #6131
func TestDecisionEnsureCreateFilesTheHelperBody(t *testing.T) {
	for _, name := range []string{"dryrun-authored", "dryrun-synth"} {
		t.Run(name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join(decisionCorpusRoot, name, "stdout.golden"))
			if err != nil {
				t.Fatal(err)
			}
			wantTitle, wantBody := dryRunBody(t, string(golden))
			dir := copyDecisionCase(t, name)
			f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
			f.labels = []string{"bug", DecisionLabel}
			run := runDecisionCase(t, dir, f, false)
			if run.res.Code != ExitOK || run.res.State != "created" {
				t.Fatalf("result %+v, stderr:\n%s", run.res, run.err)
			}
			if len(f.filed) != 1 {
				t.Fatalf("filed %d issues, want 1", len(f.filed))
			}
			if f.filed[0].Title != wantTitle || f.filed[0].Body != wantBody+"\n" {
				t.Errorf("filed\n%q\n%q\nwant\n%q\n%q", f.filed[0].Title, f.filed[0].Body, wantTitle, wantBody+"\n")
			}
			if len(f.applied) != 1 || f.applied[0].Target != TargetIssue || len(f.applied[0].Add) != 1 ||
				f.applied[0].Add[0].Name != DecisionLabel {
				t.Errorf("label change %+v, want the %s label on the issue", f.applied, DecisionLabel)
			}
			wantLine := "decision-issue: decision-issue: https://github.com/o/r/issues/100 created for "
			if !strings.HasPrefix(run.out, wantLine) {
				t.Errorf("stdout %q, want prefix %q", run.out, wantLine)
			}
			if run.forged != 1 {
				t.Errorf("forge resolved %d times, want 1", run.forged)
			}

			// The issue just filed carries the marker; the next ensure finds it and files none.
			filed := f.filed[0]
			fi := fakeDecisionIssue{Number: 100, Title: filed.Title, State: "OPEN", Body: filed.Body}
			f.issues[100] = fi
			raw, _ := json.Marshal([]fakeDecisionIssue{fi})
			if err := os.WriteFile(filepath.Join(dir, "forge", "label.json"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			again := runDecisionCase(t, dir, f, false)
			if again.res.State != "exists" || len(f.filed) != 1 {
				t.Errorf("second ensure: %+v filed=%d out=%s err=%s", again.res, len(f.filed), again.out, again.err)
			}
		})
	}
}

// A repo without the label still gets its gate issue, unlabeled, with a notice; a label
// listing that fails reads as missing.
//
// regression: #6131
func TestDecisionEnsureCreateWithoutLabel(t *testing.T) {
	for name, setup := range map[string]func(*fakeDecisionForge){
		"absent":       func(f *fakeDecisionForge) { f.labels = []string{"bug"} },
		"listing-fail": func(f *fakeDecisionForge) { f.labelErr = errors.New("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			dir := copyDecisionCase(t, "dryrun-authored")
			f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
			setup(f)
			run := runDecisionCase(t, dir, f, false)
			if run.res.State != "created" || len(f.filed) != 1 || len(f.applied) != 0 {
				t.Fatalf("result %+v filed=%d applied=%d err=%s", run.res, len(f.filed), len(f.applied), run.err)
			}
			if !strings.Contains(run.err, "filing UNLABELED") {
				t.Errorf("no unlabeled notice:\n%s", run.err)
			}
		})
	}
}

// regression: #6131
func TestDecisionEnsureCreateFailures(t *testing.T) {
	cases := map[string]struct {
		fileErr, applyErr error
		code              int
		state, want       string
	}{
		"create-refused":  {fileErr: Refused("blocked"), code: ExitRefused, state: "refused", want: "issue create was refused on o/r"},
		"create-failed":   {fileErr: errors.New("502"), code: ExitUnverifiable, state: "unverifiable", want: "issue create failed on o/r: 502"},
		"label-unapplied": {applyErr: errors.New("403"), code: ExitOK, state: "created", want: "could not be applied"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := copyDecisionCase(t, "dryrun-authored")
			f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
			f.labels = []string{DecisionLabel}
			f.fileErr, f.applyErr = c.fileErr, c.applyErr
			run := runDecisionCase(t, dir, f, false)
			if run.res.Code != c.code || run.res.State != c.state || !strings.Contains(run.err, c.want) {
				t.Errorf("result %+v stderr:\n%s\nwant code %d state %s containing %q", run.res, run.err, c.code, c.state, c.want)
			}
		})
	}
}

// Every brief that ever had a decision issue — filed by the helper or the port, under the
// current marker, the legacy marker, or the other id spelling, open or closed, or named in
// the frontmatter — gets no second one, and the brief file is never written.
//
// regression: #6131
func TestDecisionEnsureDedupesHelperFilings(t *testing.T) {
	for _, name := range []string{"exists-current", "exists-alt", "exists-legacy", "exists-lowest", "exists-frontmatter"} {
		t.Run(name, func(t *testing.T) {
			dir := copyDecisionCase(t, name)
			before, _ := os.ReadFile(filepath.Join(dir, "brief.md"))
			f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
			f.labels = []string{DecisionLabel}
			run := runDecisionCase(t, dir, f, false)
			if run.res.Code != ExitOK || run.res.State != "exists" {
				t.Fatalf("result %+v stderr:\n%s", run.res, run.err)
			}
			if len(f.filed) != 0 || len(f.applied) != 0 {
				t.Errorf("a brief with a decision issue got a write: filed=%d applied=%d", len(f.filed), len(f.applied))
			}
			after, _ := os.ReadFile(filepath.Join(dir, "brief.md"))
			if !bytes.Equal(before, after) {
				t.Errorf("ensure wrote the brief file")
			}
		})
	}
}

// The labelled sweep covers a fresh filing the search index has not caught up with.
//
// regression: #6131
func TestDecisionEnsureLabelSweepCoversIndexLag(t *testing.T) {
	dir := copyDecisionCase(t, "dryrun-authored")
	issue := `[{"number":7,"title":"Decision: sample/07 — Pick the retention window","state":"OPEN","body":"<!-- decision-gate: assay:ex:sample:07 -->"},` +
		`{"number":3,"title":"Decision: sample/070 — Other","state":"OPEN","body":"<!-- decision-gate: sample/070 -->"}]`
	if err := os.WriteFile(filepath.Join(dir, "forge", "label.json"), []byte(issue), 0o644); err != nil {
		t.Fatal(err)
	}
	f := newFakeDecisionForge(t, filepath.Join(dir, "forge"))
	run := runDecisionCase(t, dir, f, false)
	if run.res.State != "exists" || !strings.Contains(run.out, "exists: #7 (OPEN)") {
		t.Fatalf("result %+v out=%s err=%s", run.res, run.out, run.err)
	}

	// A failing label listing is tolerated: the body searches still run, and nothing found
	// means a filing.
	dir2 := copyDecisionCase(t, "dryrun-authored")
	if err := os.WriteFile(filepath.Join(dir2, "forge", "label.fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f2 := newFakeDecisionForge(t, filepath.Join(dir2, "forge"))
	run2 := runDecisionCase(t, dir2, f2, true)
	if run2.res.State != "dry-run" {
		t.Fatalf("label listing failure was not tolerated: %+v err=%s", run2.res, run2.err)
	}
	want := []string{"decision-gate: assay:ex:sample:07", "decision-gate: sample/07", "needs-decision: assay:ex:sample:07", "needs-decision: sample/07"}
	if strings.Join(f2.searches, "|") != strings.Join(want, "|") {
		t.Errorf("searches %q, want %q", f2.searches, want)
	}
}

// Local outcomes never resolve the forge: no credential is read for a brief that is not
// gated, already done, deferred, or refused before the lookups.
//
// regression: #6131
func TestDecisionEnsureLocalOutcomesSkipTheForge(t *testing.T) {
	root := decisionCorpusRoot
	for _, name := range []string{"not-gated", "not-needed-board-done", "deferred-later-stage", "deferred-spec-placeholder",
		"deferred-spec-absent", "refused-at", "refused-trigger", "refused-no-id", "refused-no-frontmatter",
		"refused-frontmatter-value", "refused-spec-placeholder-at-spec"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			run := runDecisionCase(t, dir, newFakeDecisionForge(t, filepath.Join(dir, "forge")), false)
			if run.forged != 0 {
				t.Errorf("forge resolved %d times for a local outcome %+v", run.forged, run.res)
			}
		})
	}
}

// regression: #6131
func TestDecisionEnsureRepoAndForgeResolution(t *testing.T) {
	dir := copyDecisionCase(t, "dryrun-authored")
	t.Chdir(dir)
	base := func() (DecisionEnsureRequest, *bytes.Buffer, *bytes.Buffer) {
		var out, errb bytes.Buffer
		return DecisionEnsureRequest{BriefFile: "brief.md", At: "start", DryRun: true, Out: &out, Err: &errb,
			Forge: func(ForgeRepo) (DecisionEnsureForge, error) { return newFakeDecisionForge(t, "forge"), nil }}, &out, &errb
	}

	req, out, _ := base()
	req.OriginRepo = func() (string, error) { return "o/r", nil }
	if r := EnsureDecisionIssue(req); r.State != "dry-run" || !strings.Contains(out.String(), "would file on o/r") {
		t.Errorf("origin repo not used: %+v %s", r, out)
	}

	req, _, errb := base()
	req.OriginRepo = func() (string, error) { return "", errors.New("no origin") }
	if r := EnsureDecisionIssue(req); r.Code != ExitUnverifiable || !strings.Contains(errb.String(), "no --repo given") {
		t.Errorf("missing origin: %+v %s", r, errb)
	}

	req, _, errb = base()
	req.Repo = "just-a-name"
	if r := EnsureDecisionIssue(req); r.Code != ExitRefused || !strings.Contains(errb.String(), "--repo must be owner/name") {
		t.Errorf("malformed repo: %+v %s", r, errb)
	}

	req, _, errb = base()
	req.Repo = "o/r"
	req.Forge = func(ForgeRepo) (DecisionEnsureForge, error) { return nil, Unverifiable("no credential", nil) }
	if r := EnsureDecisionIssue(req); r.Code != ExitUnverifiable || !strings.Contains(errb.String(), "cause: no credential") {
		t.Errorf("forge resolution failure: %+v %s", r, errb)
	}

	// The candidate a sweep matched must be readable, or nothing is created.
	dir2 := copyDecisionCase(t, "exists-current")
	t.Chdir(dir2)
	f := newFakeDecisionForge(t, "forge")
	searchOnly := &searchOnlyForge{fakeDecisionForge: f}
	req, _, errb = base()
	req.Repo = "o/r"
	req.DryRun = false
	req.Forge = func(ForgeRepo) (DecisionEnsureForge, error) { return searchOnly, nil }
	if r := EnsureDecisionIssue(req); r.Code != ExitUnverifiable || len(f.filed) != 0 {
		t.Errorf("unreadable candidate: %+v filed=%d %s", r, len(f.filed), errb)
	}
}

// searchOnlyForge finds candidates but cannot read them.
type searchOnlyForge struct{ *fakeDecisionForge }

func (searchOnlyForge) GetIssue(ForgeRepo, int) (*Issue, error) {
	return nil, errors.New("read failed")
}

// The parity corpus covers every exit code and every state the verb can report.
//
// regression: #6131
func TestDecisionEnsureCorpusCoversEveryOutcome(t *testing.T) {
	root := decisionCorpusRoot
	ents, _ := os.ReadDir(root)
	seen := map[string]bool{}
	for _, e := range ents {
		prefix, _, _ := strings.Cut(e.Name(), "-")
		seen[prefix] = true
	}
	var got []string
	for k := range seen {
		got = append(got, k)
	}
	sort.Strings(got)
	for _, want := range []string{"deferred", "dryrun", "exists", "held", "not", "refused", "unverifiable"} {
		if !seen[want] {
			t.Errorf("parity corpus has no %s- case (has %v)", want, got)
		}
	}
}

// The write-budget hook runs once, only on a real filing, and its error class is the exit.
//
// regression: #6131
func TestDecisionEnsureBeforeCreate(t *testing.T) {
	cases := map[string]struct {
		err   error
		code  int
		state string
	}{
		"allowed":      {nil, ExitOK, "created"},
		"refused":      {Refused("bad key"), ExitRefused, "refused"},
		"rate-limited": {RateLimited("budget spent"), ExitRateLimited, "rate-limited"},
		"unreadable":   {errors.New("ledger unreadable"), ExitUnverifiable, "unverifiable"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := copyDecisionCase(t, "dryrun-authored")
			t.Chdir(dir)
			f := newFakeDecisionForge(t, "forge")
			f.labels = []string{DecisionLabel}
			calls := 0
			var out, errb bytes.Buffer
			req := DecisionEnsureRequest{BriefFile: "brief.md", Repo: "o/r", At: "start", Out: &out, Err: &errb,
				Forge:        func(ForgeRepo) (DecisionEnsureForge, error) { return f, nil },
				BeforeCreate: func(repo string) error { calls++; return c.err }}
			r := EnsureDecisionIssue(req)
			if r.Code != c.code || r.State != c.state || calls != 1 || r.Repo != "o/r" {
				t.Errorf("result %+v calls %d; want code %d state %s, one call\n%s", r, calls, c.code, c.state, errb.String())
			}
			if wantFiled := map[bool]int{true: 1, false: 0}[c.err == nil]; len(f.filed) != wantFiled {
				t.Errorf("filed %d, want %d", len(f.filed), wantFiled)
			}
			if c.err == nil && (r.Number != 100 || r.URL != "https://github.com/o/r/issues/100") {
				t.Errorf("created result names %d %q", r.Number, r.URL)
			}

			// Never on a dry run.
			calls = 0
			req.DryRun = true
			if EnsureDecisionIssue(req); calls != 0 {
				t.Errorf("dry run charged the write budget")
			}
		})
	}
}
