// aggregate_test.go — the regression suite for the cross-domain reducer.
//
// The eight TestDefectN_* tests below are one-per-defect against the prior
// reducer (issue #1001), which measured them by RUNNING that reducer rather
// than reading it. Each asserts the DISTINCTION the fix creates, so that
// removing the fix reddens the named test — a test that cannot fail is the
// defect to avoid. Each was demonstrated fail-first: the fix was broken, the
// named test went red, and the fix was restored.
//
// The grouping names here (product-a/b/c, org) and repo owners (example-org,
// example-net) are fabricated fixtures — the taxonomy is config, not code.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fixtures -------------------------------------------------------------

const testDate = "2026-08-13"

// fullConfig is a healthy config: three product groupings plus org.
const fullConfig = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
    - example-org/beta
  product-b:
    - example-net/gamma
  product-c:
    - example-org/delta
  org:
    - example-org/orgrepo
`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type harness struct {
	root    string
	cfgPath string
	snapDir string
}

func newHarness(t *testing.T, cfgBody string) *harness {
	t.Helper()
	base := t.TempDir()
	h := &harness{
		root:    filepath.Join(base, "repo"),
		cfgPath: filepath.Join(base, "domains.yaml"),
		snapDir: filepath.Join(base, "snapshots"),
	}
	writeFile(t, h.cfgPath, cfgBody)
	if err := os.MkdirAll(h.snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(h.root, 0o755); err != nil {
		t.Fatal(err)
	}
	return h
}

// snapshot writes one grouping's artifact verbatim, so a test controls
// exactly which keys are present, absent, or null — the distinction the whole
// gap model rests on.
func (h *harness) snapshot(t *testing.T, grouping, date, body string) {
	t.Helper()
	writeFile(t, filepath.Join(h.snapDir, "metrics-snapshot-"+grouping+"-"+date+".json"), body)
}

func (h *harness) run(t *testing.T, date string, extra ...string) int {
	t.Helper()
	args := append([]string{
		"--config", h.cfgPath,
		"--root", h.root,
		"--snapshots", h.snapDir,
		"--date", date,
	}, extra...)
	return runAggregate(args)
}

func (h *harness) top(t *testing.T, date string) *topRollup {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.root, "reports", "daily", date, "rollup.json"))
	if err != nil {
		t.Fatalf("reading top rollup: %v", err)
	}
	var top topRollup
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parsing top rollup: %v", err)
	}
	return &top
}

func (h *harness) markdown(t *testing.T, date string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.root, "reports", "daily", date, "rollup.md"))
	if err != nil {
		t.Fatalf("reading rollup.md: %v", err)
	}
	return string(data)
}

// headlineRow returns the headline table row whose first cell is label.
func headlineRow(t *testing.T, md, label string) string {
	t.Helper()
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "| "+label+" |") {
			return line
		}
	}
	t.Fatalf("no headline row for %q in:\n%s", label, md)
	return ""
}

// repoEntry builds a healthy snapshot entry. declareTruncation controls
// whether the source says anything about its upstream cap.
func repoEntry(spec string, draft, ready int, labels string, unlabeled int, declareTruncation bool) string {
	trunc := ""
	if declareTruncation {
		trunc = `,"truncated":{"prsOpenByState":false,"issuesOpenByLabel":false,"issuesOpenUnlabeled":false}`
	}
	return `{"repo":"` + spec + `","prsOpenByState":{"draft":` + itoa(draft) + `,"ready":` + itoa(ready) +
		`},"issuesOpenByLabel":` + labels + `,"issuesOpenUnlabeled":` + itoa(unlabeled) + trunc + `}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func snapshotBody(grouping, date string, entries ...string) string {
	return `{"grouping":"` + grouping + `","date":"` + date + `","repos":[` +
		strings.Join(entries, ",") + `]}`
}

// healthy writes a complete, truncation-declaring snapshot set for
// fullConfig. Figures: product-a PRs 7 (3 draft / 4 ready), unlabelled 2;
// product-b PRs 1 (1/0), unlabelled 3; product-c PRs 2 (0/2), unlabelled 1;
// org PRs 100.
func (h *harness) healthy(t *testing.T, date string) {
	t.Helper()
	// One CI run downloads exactly its own four artifacts, so the dir is
	// re-made rather than added to — a second day's snapshots alongside the
	// first would be the two-artifacts-per-grouping ambiguity the reducer
	// refuses (see TestTwoSnapshotsOneGroupingRefused).
	if err := os.RemoveAll(h.snapDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(h.snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h.snapshot(t, "product-a", date, snapshotBody("product-a", date,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4,"P1":2}`, 1, true),
		repoEntry("example-org/beta", 1, 2, `{"bug":1}`, 1, true)))
	h.snapshot(t, "product-b", date, snapshotBody("product-b", date,
		repoEntry("example-net/gamma", 1, 0, `{"bug":1}`, 3, true)))
	h.snapshot(t, "product-c", date, snapshotBody("product-c", date,
		repoEntry("example-org/delta", 0, 2, `{}`, 1, true)))
	h.snapshot(t, "org", date, snapshotBody("org", date,
		repoEntry("example-org/orgrepo", 50, 50, `{"bug":9}`, 7, true)))
}

func mustValue(t *testing.T, name string, m measure) int {
	t.Helper()
	if m.State != stateMeasured {
		t.Fatalf("%s: want state %q, got %q (note: %s)", name, stateMeasured, m.State, m.Note)
	}
	if m.Value == nil {
		t.Fatalf("%s: state is measured but Value is nil", name)
	}
	return *m.Value
}

func wantState(t *testing.T, name string, m measure, want string) {
	t.Helper()
	if m.State != want {
		t.Fatalf("%s: want state %q, got %q (note: %s)", name, want, m.State, m.Note)
	}
	if want != stateMeasured && m.Value != nil {
		t.Fatalf("%s: state is %q but a value (%d) was published anyway — a could-not-check "+
			"must never render as a measured value", name, want, *m.Value)
	}
}

// --- control: the instrument is live and the arithmetic is right ----------

func TestControl_HealthyRunSumsCorrectly(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)

	if rc := h.run(t, testDate, "--run-id", "12345"); rc != exitOK {
		t.Fatalf("want exit %d on a fully measured day, got %d", exitOK, rc)
	}
	top := h.top(t, testDate)

	prodA := top.Products["product-a"].figures
	if got := mustValue(t, "product-a prsOpenByState.total", prodA.PRsOpenByState.Total); got != 7 {
		t.Errorf("product-a open PRs: want 7, got %d", got)
	}
	if got := mustValue(t, "product-a prsOpenByState.draft", prodA.PRsOpenByState.Draft); got != 3 {
		t.Errorf("product-a draft PRs: want 3, got %d", got)
	}
	if got := mustValue(t, "product-a prsOpenByState.ready", prodA.PRsOpenByState.Ready); got != 4 {
		t.Errorf("product-a ready PRs: want 4, got %d", got)
	}
	if got := mustValue(t, "product-a issuesOpenUnlabeled", prodA.IssuesOpenUnlabeled); got != 2 {
		t.Errorf("product-a unlabelled issues: want 2, got %d", got)
	}
	if got := prodA.IssuesOpenByLabel.Labels["bug"]; got != 5 {
		t.Errorf("product-a bug label: want 5, got %d", got)
	}

	// allProducts = product-a + product-b + product-c, and org is NOT folded in.
	if got := mustValue(t, "allProducts total", top.AllProducts.PRsOpenByState.Total); got != 10 {
		t.Errorf("allProducts open PRs: want 10 (7+1+2), got %d", got)
	}
	if got := mustValue(t, "allProducts unlabelled", top.AllProducts.IssuesOpenUnlabeled); got != 6 {
		t.Errorf("allProducts unlabelled issues: want 6 (2+3+1), got %d", got)
	}
	if got := mustValue(t, "org total", top.Org.PRsOpenByState.Total); got != 100 {
		t.Errorf("org open PRs: want 100, got %d", got)
	}
	if top.AllProducts.PRsOpenByState.Total.Value != nil && *top.AllProducts.PRsOpenByState.Total.Value >= 100 {
		t.Errorf("org was folded into allProducts — it must stay a distinct section")
	}
	if top.RunID != "12345" {
		t.Errorf("runID: want 12345, got %q", top.RunID)
	}
}

// --- defect 1 -------------------------------------------------------------

// #1001 defect 1: "An unreadable input tree publishes 0, exit 0." The
// headline table rendered `| product-a | 0 (0/0) | 0 | 0 | 0 |` for a day on
// which nothing at all was measured.
func TestDefect1_UnreadableInputIsNotZero(t *testing.T) {
	h := newHarness(t, fullConfig)
	// No snapshots at all: every configured source is unreadable.

	rc := h.run(t, testDate)
	if rc != exitPublished {
		t.Fatalf("want exit %d (published with a gap), got %d — an all-unreadable day must not "+
			"exit 0", exitPublished, rc)
	}

	top := h.top(t, testDate)
	prodA := top.Products["product-a"].figures
	wantState(t, "product-a prsOpenByState.total", prodA.PRsOpenByState.Total, stateCouldNotCheck)
	wantState(t, "product-a issuesOpenUnlabeled", prodA.IssuesOpenUnlabeled, stateCouldNotCheck)
	wantState(t, "allProducts total", top.AllProducts.PRsOpenByState.Total, stateCouldNotCheck)

	md := h.markdown(t, testDate)
	row := headlineRow(t, md, "product-a")
	if !strings.Contains(row, "could-not-check (0/2 sources)") {
		t.Errorf("headline row does not say could-not-check where the number is read:\n%s", row)
	}
	for _, cell := range strings.Split(row, "|") {
		if strings.TrimSpace(cell) == "0" {
			t.Errorf("headline row publishes a measured 0 for an unmeasured day:\n%s", row)
		}
	}
}

// --- defect 2 -------------------------------------------------------------

// #1001 defect 2: "Not-configured, could-not-check, and genuine-zero are
// byte-identical in the table", and the empty-config grouping never appeared
// in the gaps section at all — so a typo dropping a repo list read as a clean
// measured zero with no trace anywhere in the output.
func TestDefect2_ThreeStatesAreDistinct(t *testing.T) {
	const cfg = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
    - example-org/beta
  product-b:
    - example-net/gamma
  product-c: []
  org:
    - example-org/orgrepo
`
	h := newHarness(t, cfg)
	// product-b: a genuine, fully measured zero. product-a: no snapshot at all.
	// product-c: nothing configured.
	h.snapshot(t, "product-b", testDate, snapshotBody("product-b", testDate,
		repoEntry("example-net/gamma", 0, 0, `{}`, 0, true)))

	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}

	top := h.top(t, testDate)
	wantState(t, "product-a total", top.Products["product-a"].PRsOpenByState.Total, stateCouldNotCheck)
	wantState(t, "product-c total", top.Products["product-c"].PRsOpenByState.Total, stateNotConfigured)
	if got := mustValue(t, "product-b total", top.Products["product-b"].PRsOpenByState.Total); got != 0 {
		t.Fatalf("product-b open PRs: want a measured 0, got %d", got)
	}

	md := h.markdown(t, testDate)
	aCells := strings.Split(headlineRow(t, md, "product-a"), "|")
	cCells := strings.Split(headlineRow(t, md, "product-c"), "|")
	bCells := strings.Split(headlineRow(t, md, "product-b"), "|")

	got := []string{
		strings.TrimSpace(aCells[2]),
		strings.TrimSpace(cCells[2]),
		strings.TrimSpace(bCells[2]),
	}
	if got[0] == got[1] || got[0] == got[2] || got[1] == got[2] {
		t.Fatalf("three distinct states collapsed to the same glyphs in the headline table: "+
			"product-a=%q product-c=%q product-b=%q", got[0], got[1], got[2])
	}
	if got[1] != "not-configured" {
		t.Errorf("empty-config grouping renders %q, not %q", got[1], "not-configured")
	}
	if got[2] != "0" {
		t.Errorf("genuine zero renders %q, not %q", got[2], "0")
	}
	// The empty-config case must leave a trace, not vanish.
	if !strings.Contains(md, "Configured sources (0)") {
		t.Errorf("the empty-config grouping leaves no trace in the coverage section")
	}
}

// --- defect 3 -------------------------------------------------------------

// #1001 defect 3: "A partial gap silently undercounts, and one flat gap label
// covers four different facts." One shared per-repo boolean could not express
// "excluded from one figure, included in three", so a 43% PR undercount rode
// alongside three complete figures in the same row with one gap label.
func TestDefect3_GapsArePerFigure(t *testing.T) {
	h := newHarness(t, fullConfig)
	// beta's PR read failed (the collector's null shape); its ISSUE reads
	// succeeded. The two figures must therefore disagree about coverage.
	h.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4}`, 1, true),
		`{"repo":"example-org/beta","prsOpenByState":{"draft":null,"ready":null},`+
			`"issuesOpenByLabel":{"bug":1},"issuesOpenUnlabeled":1,`+
			`"truncated":{"prsOpenByState":false,"issuesOpenByLabel":false,"issuesOpenUnlabeled":false}}`))

	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}
	prodA := h.top(t, testDate).Products["product-a"].figures

	// The gapped figure refuses to publish a count rather than publishing a
	// smaller one (the old reducer published 4 against a true 7).
	wantState(t, "product-a prsOpenByState.total", prodA.PRsOpenByState.Total, stateWithheld)
	prCov := prodA.PRsOpenByState.Total.Cov
	if len(prCov.Measured) != 1 || prCov.Measured[0] != "example-org/alpha" {
		t.Errorf("PR figure measured set: want [example-org/alpha], got %v", prCov.Measured)
	}
	if len(prCov.Failed) != 1 || prCov.Failed[0] != "example-org/beta" {
		t.Errorf("PR figure failed set: want [example-org/beta], got %v", prCov.Failed)
	}

	// The ungapped figures in the SAME row keep their full coverage.
	if got := mustValue(t, "product-a issuesOpenUnlabeled", prodA.IssuesOpenUnlabeled); got != 2 {
		t.Errorf("unlabelled issues: want 2 (both repos), got %d", got)
	}
	issCov := prodA.IssuesOpenUnlabeled.Cov
	if len(issCov.Measured) != 2 {
		t.Errorf("issue figure measured set: want both repos, got %v", issCov.Measured)
	}
	if sameSources(prCov.Measured, issCov.Measured) {
		t.Fatalf("the PR figure and the issue figure report the SAME source set (%v) on a day "+
			"they were measured over different ones — that is the single-boolean gap model "+
			"#1001 defect 3 measured", prCov.Measured)
	}
	// And the two states are visible where the numbers are read.
	row := headlineRow(t, h.markdown(t, testDate), "product-a")
	if !strings.Contains(row, "withheld (1/2 sources)") || !strings.Contains(row, "| 2 |") {
		t.Errorf("row does not carry two different coverages side by side:\n%s", row)
	}
}

// --- defect 4 -------------------------------------------------------------

// #1001 defect 4: "Day-over-day trend deltas are computed across differing
// gap sets, fabricating movement." Nothing changed upstream; one repo's
// capture failed; the trend rendered -3 / -2 / -5 / -2 as clean change.
func TestDefect4_TrendNeedsMatchedCoverage(t *testing.T) {
	const prevDay = "2026-08-12"

	// Day 1: both repos measured, and the delta IS computed — so this test
	// can fail in both directions.
	h := newHarness(t, fullConfig)
	h.healthy(t, prevDay)
	if rc := h.run(t, prevDay); rc != exitPublished {
		// prevDay has no day-before, so the trend is no-prior-day; figures
		// are all measured, hence exit 0.
		if rc != exitOK {
			t.Fatalf("day 1: unexpected exit %d", rc)
		}
	}
	h.healthy(t, testDate)
	if rc := h.run(t, testDate); rc != exitOK {
		t.Fatalf("day 2 (identical coverage): want exit %d, got %d", exitOK, rc)
	}
	d := h.top(t, testDate).Trend.Products["product-a"]
	if d.PRsOpenTotal.State != deltaComputed || *d.PRsOpenTotal.Value != 0 {
		t.Fatalf("matched coverage must compute a delta: state=%q note=%s",
			d.PRsOpenTotal.State, d.PRsOpenTotal.Note)
	}

	// Day 2 again, this time with beta's PR capture failed and NOTHING
	// changed upstream. The -3 the old reducer published was the capture
	// failure, not the world.
	h.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4,"P1":2}`, 1, true),
		`{"repo":"example-org/beta","prsOpenByState":{"draft":null,"ready":null},`+
			`"issuesOpenByLabel":{"bug":1},"issuesOpenUnlabeled":1,`+
			`"truncated":{"prsOpenByState":false,"issuesOpenByLabel":false,"issuesOpenUnlabeled":false}}`))
	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}
	trend := h.top(t, testDate).Trend
	if trend.State != trendComputed {
		t.Fatalf("trend state: want %q, got %q", trendComputed, trend.State)
	}
	d = trend.Products["product-a"]
	if d.PRsOpenTotal.State != deltaWithheld {
		t.Fatalf("PR delta across differing coverage: want %q, got %q (value %v) — a delta "+
			"between a 2-source day and a 1-source day is movement in the instrument",
			deltaWithheld, d.PRsOpenTotal.State, deltaCell(d.PRsOpenTotal))
	}
	if d.PRsOpenTotal.Value != nil {
		t.Fatalf("withheld delta still published a value: %d", *d.PRsOpenTotal.Value)
	}
	// The issue figure's coverage DID match on both days, so its delta stands.
	if d.IssuesOpenUnlabeled.State != deltaComputed {
		t.Errorf("issue delta: coverage matched on both days, so it must be computed; got %q (%s)",
			d.IssuesOpenUnlabeled.State, d.IssuesOpenUnlabeled.Note)
	}
	md := h.markdown(t, testDate)
	if !strings.Contains(md, "| product-a | withheld |") {
		t.Errorf("the trend table does not render the withheld PR delta:\n%s", md)
	}
}

// The load-bearing half of #1001 defect 4: BOTH days are fully measured, and
// they still are not comparable, because the source set itself changed (a
// repo left the grouping). Withholding only when a figure is unmeasured is
// not enough — the previous reducer subtracted field-by-field without ever
// comparing the two days' coverage, so a config change rendered as clean
// day-over-day movement.
func TestDefect4_BothMeasuredStillNeedsMatch(t *testing.T) {
	const prevDay = "2026-08-12"
	const twoRepos = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
    - example-org/beta
  product-b: []
  product-c: []
  org: []
`
	const oneRepo = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
  product-b: []
  product-c: []
  org: []
`
	h := newHarness(t, twoRepos)

	// Day 1: both repos, fully measured.
	h.snapshot(t, "product-a", prevDay, snapshotBody("product-a", prevDay,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4}`, 1, true),
		repoEntry("example-org/beta", 1, 2, `{"bug":1}`, 1, true)))
	h.run(t, prevDay)
	if got := mustValue(t, "day 1 product-a total",
		h.top(t, prevDay).Products["product-a"].PRsOpenByState.Total); got != 7 {
		t.Fatalf("day 1 product-a open PRs: want 7, got %d", got)
	}

	// Day 2: beta has left the grouping. Nothing about alpha changed.
	writeFile(t, h.cfgPath, oneRepo)
	if err := os.RemoveAll(h.snapDir); err != nil {
		t.Fatal(err)
	}
	h.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4}`, 1, true)))
	h.run(t, testDate)

	prodA := h.top(t, testDate).Products["product-a"].figures
	if got := mustValue(t, "day 2 product-a total", prodA.PRsOpenByState.Total); got != 4 {
		t.Fatalf("day 2 product-a open PRs: want a measured 4, got %d", got)
	}
	d := h.top(t, testDate).Trend.Products["product-a"]
	if d.PRsOpenTotal.State != deltaWithheld {
		t.Fatalf("both days measured, but over different source sets ([alpha] vs [alpha beta]) — "+
			"want delta %q, got %q with value %v. Subtracting these two published counts yields "+
			"-3, which is beta leaving the config, not three PRs closing (#1001 defect 4).",
			deltaWithheld, d.PRsOpenTotal.State, d.PRsOpenTotal.Value)
	}
	if !strings.Contains(d.PRsOpenTotal.Note, "different source sets") {
		t.Errorf("withheld delta does not say why: %q", d.PRsOpenTotal.Note)
	}
}

// --- defect 5 -------------------------------------------------------------

// #1001 defect 5: "Checked-failed is laundered into could-not-check, then
// asserted as fact." A corrupt 18-byte prior rollup.json produced the
// sentence "No prior day's roll-up found" — a positive claim of absence about
// a file sitting on disk.
func TestDefect5_ParseFailureIsNotAbsence(t *testing.T) {
	const prevDay = "2026-08-12"
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)

	// Control: genuinely absent. Checked, and it really is not there.
	if rc := h.run(t, testDate); rc != exitOK {
		t.Fatalf("absent prior day: want exit %d, got %d", exitOK, rc)
	}
	trend := h.top(t, testDate).Trend
	if trend.State != trendNoPriorDay {
		t.Fatalf("absent prior day: want state %q, got %q", trendNoPriorDay, trend.State)
	}

	// Now the defect's own fixture: present on disk, 18 bytes, not JSON.
	writeFile(t, filepath.Join(h.root, "reports", "daily", prevDay, "rollup.json"), "{ this is not json")
	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("unparseable prior day: want exit %d (loud), got %d", exitPublished, rc)
	}
	trend = h.top(t, testDate).Trend
	if trend.State != trendPriorUnreadable {
		t.Fatalf("want state %q, got %q — a parse failure is not an absence",
			trendPriorUnreadable, trend.State)
	}
	if !strings.Contains(trend.Note, "did NOT parse") {
		t.Errorf("trend note does not name the parse failure: %q", trend.Note)
	}
	md := h.markdown(t, testDate)
	if strings.Contains(md, "No prior day's roll-up") {
		t.Errorf("rollup.md asserts the prior day is absent about a file that exists:\n%s", md)
	}
	if !strings.Contains(md, "could not be read") {
		t.Errorf("rollup.md does not report the read failure:\n%s", md)
	}
}

// --- defect 6 -------------------------------------------------------------

// #1001 defect 6: "A duplicate config entry inflates every figure and reports
// clean." Listing one repo twice corrupted FIVE figures and published
// reposWithGaps: [].
func TestDefect6_DuplicateConfigRefused(t *testing.T) {
	const dup = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
    - example-org/beta
    - example-org/alpha
  product-b:
    - example-net/gamma
  product-c:
    - example-org/delta
  org:
    - example-org/orgrepo
`
	h := newHarness(t, dup)
	h.healthy(t, testDate)

	if rc := h.run(t, testDate); rc != exitRefused {
		t.Fatalf("want exit %d (refusal), got %d — a duplicate must not be reduced", exitRefused, rc)
	}
	if _, err := os.Stat(filepath.Join(h.root, "reports", "daily", testDate, "rollup.json")); err == nil {
		t.Fatalf("a roll-up was written despite the refusal")
	}
	if _, err := loadConfig(h.cfgPath); err == nil {
		t.Fatalf("loadConfig accepted a config listing example-org/alpha twice")
	} else if !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("refusal does not name the duplicate: %v", err)
	}

	// Cross-grouping duplication double-counts the all-products total, so it
	// is refused too.
	const crossDup = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
  product-b:
    - example-org/alpha
  product-c: []
  org: []
`
	h2 := newHarness(t, crossDup)
	if _, err := loadConfig(h2.cfgPath); err == nil {
		t.Fatalf("loadConfig accepted the same repo in two product groupings")
	}
}

// --- defect 7 -------------------------------------------------------------

// #1001 defect 7: "A repo that is never read reports as checked-clean."
// repoBaseName collapsed owner/name to name, so example-org/alpha and
// example-net/alpha resolved to one source: the first was counted twice and the
// second was never read, with reposWithGaps: [].
func TestDefect7_CollisionRefusedFullSpecKey(t *testing.T) {
	const collide = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
    - example-net/alpha
  product-b: []
  product-c: []
  org: []
`
	h := newHarness(t, collide)
	if rc := h.run(t, testDate); rc != exitRefused {
		t.Fatalf("want exit %d (refusal) on a base-name collision, got %d", exitRefused, rc)
	}
	if _, err := loadConfig(h.cfgPath); err == nil {
		t.Fatalf("loadConfig accepted two repos sharing the base name \"alpha\"")
	} else if !strings.Contains(err.Error(), "share the base name") {
		t.Errorf("refusal does not name the collision: %v", err)
	}

	// And keying is on the FULL spec: a snapshot entry carrying only a base
	// name satisfies nothing and is reported as unconfigured drift, rather
	// than silently standing in for the configured repo.
	const ok = `products:
  - product-a
  - product-b
  - product-c
org: org
groupings:
  product-a:
    - example-org/alpha
  product-b: []
  product-c: []
  org: []
`
	h2 := newHarness(t, ok)
	h2.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("alpha", 9, 9, `{"bug":9}`, 9, true)))
	if rc := h2.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}
	prodA := h2.top(t, testDate).Products["product-a"].figures
	wantState(t, "product-a total", prodA.PRsOpenByState.Total, stateCouldNotCheck)
	cov := prodA.PRsOpenByState.Total.Cov
	if len(cov.Missing) != 1 || cov.Missing[0] != "example-org/alpha" {
		t.Errorf("configured repo must be reported missing, got missing=%v", cov.Missing)
	}
	if len(cov.Unexpected) != 1 || cov.Unexpected[0] != "alpha" {
		t.Errorf("the base-name entry must be reported as unconfigured drift, got unexpected=%v",
			cov.Unexpected)
	}
}

// --- defect 8 -------------------------------------------------------------

// #1001 defect 8: "A source at the upstream truncation cap publishes as
// checked-clean." `grep -n truncat aggregate.go` returned nothing: the
// reducer had no concept of a truncated source and summed a capped count as a
// complete one.
func TestDefect8_TruncationWithheldOrMarked(t *testing.T) {
	// (a) DECLARED truncation: the figure refuses to publish a count.
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4}`, 1, true),
		`{"repo":"example-org/beta","prsOpenByState":{"draft":500,"ready":500},`+
			`"issuesOpenByLabel":{"bug":1},"issuesOpenUnlabeled":1,`+
			`"truncated":{"prsOpenByState":true,"issuesOpenByLabel":false,"issuesOpenUnlabeled":false}}`))
	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}
	prodA := h.top(t, testDate).Products["product-a"].figures
	wantState(t, "product-a prsOpenByState.total", prodA.PRsOpenByState.Total, stateWithheld)
	if got := prodA.PRsOpenByState.Total.Cov.Truncated; len(got) != 1 || got[0] != "example-org/beta" {
		t.Errorf("truncated source not recorded on the figure: %v", got)
	}
	// The untruncated issue figures in the same row are unaffected.
	if got := mustValue(t, "product-a issuesOpenUnlabeled", prodA.IssuesOpenUnlabeled); got != 2 {
		t.Errorf("unlabelled issues: want 2, got %d", got)
	}

	// (b) UNDECLARED truncation: measured, but marked at the number, because
	// the reducer could not check.
	h2 := newHarness(t, fullConfig)
	h2.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		repoEntry("example-org/alpha", 2, 2, `{"bug":4}`, 1, false),
		repoEntry("example-org/beta", 1, 2, `{"bug":1}`, 1, false)))
	h2.run(t, testDate)
	prodA2 := h2.top(t, testDate).Products["product-a"].figures
	if got := mustValue(t, "product-a total", prodA2.PRsOpenByState.Total); got != 7 {
		t.Errorf("undeclared truncation must not withhold the figure; want 7, got %d", got)
	}
	if len(prodA2.PRsOpenByState.Total.Cov.TruncationUndeclared) != 2 {
		t.Fatalf("undeclared truncation not recorded: %v",
			prodA2.PRsOpenByState.Total.Cov.TruncationUndeclared)
	}
	row := headlineRow(t, h2.markdown(t, testDate), "product-a")
	if !strings.Contains(row, "7 ~") {
		t.Errorf("an undeclared-truncation figure renders as a bare complete count:\n%s", row)
	}
	if !strings.Contains(h2.markdown(t, testDate), "may be a floor") {
		t.Errorf("rollup.md carries no legend for the truncation mark")
	}
}

// --- re-scope: schema shape -----------------------------------------------

// The ruling: prsOpenByReviewDecision is emitted null/absent until #1002
// lands — NEVER zero.
func TestReviewDecisionIsNullNeverZero(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.run(t, testDate)

	data, err := os.ReadFile(filepath.Join(h.root, "reports", "daily", testDate, "rollup.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	all, _ := raw["allProducts"].(map[string]any)
	v, present := all["prsOpenByReviewDecision"]
	if !present {
		t.Fatalf("prsOpenByReviewDecision key missing entirely")
	}
	if v != nil {
		t.Fatalf("prsOpenByReviewDecision must be null until #1002, got %#v", v)
	}
	if note, _ := all["prsOpenByReviewDecisionNote"].(string); !strings.Contains(note, "#1002") {
		t.Errorf("the null section carries no explanation: %q", note)
	}
	if !strings.Contains(h.markdown(t, testDate), "**Not collected.**") {
		t.Errorf("rollup.md does not say the review-decision section is uncollected")
	}

	// Once a source DOES declare it, the section becomes a real measure.
	h2 := newHarness(t, fullConfig)
	h2.healthy(t, testDate)
	h2.snapshot(t, "product-a", testDate, snapshotBody("product-a", testDate,
		`{"repo":"example-org/alpha","prsOpenByState":{"draft":1,"ready":1},`+
			`"issuesOpenByLabel":{},"issuesOpenUnlabeled":0,`+
			`"prsOpenByReviewDecision":{"APPROVED":2},`+
			`"truncated":{"prsOpenByState":false,"issuesOpenByLabel":false,`+
			`"issuesOpenUnlabeled":false,"prsOpenByReviewDecision":false}}`,
		repoEntry("example-org/beta", 1, 1, `{}`, 0, true)))
	h2.run(t, testDate)
	prodA := h2.top(t, testDate).Products["product-a"].figures
	if prodA.PRsOpenByReviewDecision == nil {
		t.Fatalf("a declared review-decision breakdown was still emitted as null")
	}
	// beta declared nothing, so the figure is withheld, not a partial map.
	if prodA.PRsOpenByReviewDecision.State != stateWithheld {
		t.Errorf("partial review-decision coverage: want %q, got %q",
			stateWithheld, prodA.PRsOpenByReviewDecision.State)
	}
}

// The ruling: commits-to-main and throughput are OUT of scope — house DORA
// stays with the upstream harvest. This asserts they cannot creep back in.
func TestCommitsAndThroughputAreOutOfScope(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.run(t, testDate)

	data, err := os.ReadFile(filepath.Join(h.root, "reports", "daily", testDate, "rollup.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"commitsToMain", "throughput", "mergedPRCount", "avgLeadTimeHours"} {
		if strings.Contains(string(data), banned) {
			t.Errorf("rollup.json carries out-of-scope field %q", banned)
		}
	}
	md := h.markdown(t, testDate)
	for _, banned := range []string{"Commits to main", "Merged PRs", "lead time"} {
		if strings.Contains(md, banned) {
			t.Errorf("rollup.md carries out-of-scope column %q", banned)
		}
	}
}

// The committed output is the roll-up only: six small files a day, and no raw
// snapshot anywhere in the tree.
func TestCommittedOutputIsSixFilesOnly(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.run(t, testDate)

	var got []string
	err := filepath.Walk(filepath.Join(h.root, "reports"), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(h.root, p)
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"reports/daily/" + testDate + "/rollup.json":           true,
		"reports/daily/" + testDate + "/rollup.md":             true,
		"reports/daily/" + testDate + "/product-a/rollup.json": true,
		"reports/daily/" + testDate + "/product-b/rollup.json": true,
		"reports/daily/" + testDate + "/product-c/rollup.json": true,
		"reports/daily/" + testDate + "/org/rollup.json":       true,
	}
	if len(got) != len(want) {
		t.Fatalf("want exactly %d committed files, got %d: %v", len(want), len(got), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected file in the committed tree: %s", g)
		}
	}
}

// A snapshot that exists and does not parse is checked-failed, distinct from
// no snapshot at all — the same three-state discipline one level up.
func TestUnparseableSnapshotVsAbsentOne(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.snapshot(t, "product-b", testDate, "{ not json at all")
	h.run(t, testDate)

	top := h.top(t, testDate)
	if got := top.Products["product-b"].Source; got != sourceUnreadable {
		t.Errorf("product-b source: want %q, got %q", sourceUnreadable, got)
	}
	// product-c's snapshot IS present; drop it to compare.
	h2 := newHarness(t, fullConfig)
	h2.healthy(t, testDate)
	if err := os.Remove(filepath.Join(h2.snapDir, "metrics-snapshot-product-b-"+testDate+".json")); err != nil {
		t.Fatal(err)
	}
	h2.run(t, testDate)
	if got := h2.top(t, testDate).Products["product-b"].Source; got != sourceAbsent {
		t.Errorf("product-b source: want %q, got %q", sourceAbsent, got)
	}
}

// A snapshot stamped with a different date is not this day's data.
func TestSnapshotForAnotherDateIsNotUsed(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	h.snapshot(t, "product-b", testDate, snapshotBody("product-b", "2026-08-01",
		repoEntry("example-net/gamma", 99, 99, `{}`, 99, true)))
	if rc := h.run(t, testDate); rc != exitPublished {
		t.Fatalf("want exit %d, got %d", exitPublished, rc)
	}
	top := h.top(t, testDate)
	if got := top.Products["product-b"].Source; got != sourceDateMismatch {
		t.Errorf("want %q, got %q", sourceDateMismatch, got)
	}
	wantState(t, "product-b total", top.Products["product-b"].PRsOpenByState.Total, stateCouldNotCheck)
}

// Two artifacts claiming one grouping is ambiguity the reducer must not guess
// about.
func TestTwoSnapshotsOneGroupingRefused(t *testing.T) {
	h := newHarness(t, fullConfig)
	h.healthy(t, testDate)
	writeFile(t, filepath.Join(h.snapDir, "nested", "metrics-snapshot-product-a-"+testDate+".json"),
		snapshotBody("product-a", testDate, repoEntry("example-org/alpha", 1, 1, `{}`, 1, true)))
	if rc := h.run(t, testDate); rc != exitRefused {
		t.Fatalf("want exit %d (refusal), got %d", exitRefused, rc)
	}
}

// The shipped example config must pass every config refusal — a guard that
// would fire against the documented schema is not a guard, it is an outage.
func TestExampleDomainsConfigPasses(t *testing.T) {
	cfg, err := loadConfig("domains.example.yaml")
	if err != nil {
		t.Fatalf("the shipped domains.example.yaml is refused by its own validator: %v", err)
	}
	if len(cfg.Products) == 0 {
		t.Fatalf("domains.example.yaml parsed but declares no products")
	}
	if len(cfg.Groupings[cfg.Products[0]]) == 0 {
		t.Fatalf("domains.example.yaml parsed but the first product grouping is empty")
	}
}
