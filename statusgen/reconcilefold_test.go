package main

// reconcilefold_test.go — pins that the PRODUCTION callers of DeriveLifecycle
// populate every fold input from real reads (#1787), not just Briefs/PRs/
// LookedAt: the verify witness from the tree's own Evidence audit (the
// verifyrun --check code path), the App approval from the forge reviews read,
// the gate:human ruling from the README Reviewed-cell human: stamp, and the
// blocked overlay from the linked issues' labels. A constructor that leaves
// any of the four maps nil can never derive above implemented — the exact
// defect these tests fail against.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// foldFixtureBrief is one brief file in the fold fixture tree. verifyRow and
// witnessRow are single-row Verify/Evidence tables; an empty witnessRow leaves
// the brief unwitnessed.
type foldFixtureBrief struct {
	num        string
	id         string // frontmatter brief: id
	gate       string
	issues     []int
	witnessRow string // e.g. "| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-10-01 | app[bot] @ 000000000000 |\n"
}

// writeFoldFixture builds a one-stream board (fx/01..NN) under t.TempDir().
// rows maps brief num -> its README Status/Verified/Reviewed cells.
func writeFoldFixture(t *testing.T, briefs []foldFixtureBrief, rows map[string][3]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "fx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var table strings.Builder
	table.WriteString("---\nstream: fx\nstatus: active\npriority: P1\ntrack: platform\n---\n\n" +
		"# Fold fixture stream\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n")
	for _, b := range briefs {
		r := rows[b.num]
		table.WriteString("| " + b.num + " | [Fixture " + b.num + "](./brief-" + b.num + "-fixture.md) | 0 | S | " +
			r[0] + " | " + r[1] + " | " + r[2] + " |\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(table.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, b := range briefs {
		issues := "[]"
		if len(b.issues) > 0 {
			var sb strings.Builder
			sb.WriteString("[")
			for i, n := range b.issues {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(strconv.Itoa(n))
			}
			sb.WriteString("]")
			issues = sb.String()
		}
		var body strings.Builder
		body.WriteString("---\nschema: brief-v1\nbrief: " + b.id + "\ntitle: Fold fixture " + b.num + "\n" +
			"wave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: " + b.gate + "\n" +
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
			"issues: " + issues + "\nauthored: 2026-09-10 by fold fixture\nsources: [\"fixture\"]\n---\n\n" +
			"# Brief " + b.num + "\n\n## Verify\n\n" +
			"| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |\n\n" +
			"## Evidence\n\n")
		if b.witnessRow != "" {
			body.WriteString(witnessHeader + "\n" + b.witnessRow)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief-"+b.num+"-fixture.md"), []byte(body.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// foldPullsServer answers the three reads the wired fold makes: the paged
// pulls list, one reviews read per verified gate:model brief's merged PR, and
// the paged open-issues list. reviewsByPR and issues are keyed for the
// fixture; a nil map (or a path outside the three shapes) is a 500, so a test
// can watch the fail-closed arm.
func foldPullsServer(t *testing.T, pulls []map[string]any, reviewsByPR map[int][]map[string]any, issues []map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/reviews"):
			for n, revs := range reviewsByPR {
				if strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/pulls/"+strconv.Itoa(n)+"/reviews") ||
					strings.Contains(r.URL.Path, "/pulls/"+strconv.Itoa(n)+"/reviews") {
					_ = json.NewEncoder(w).Encode(revs)
					return
				}
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"no reviews fixture"}`))
		case strings.Contains(r.URL.Path, "/issues"):
			if issues == nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"no issues fixture"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(issues)
		case strings.Contains(r.URL.Path, "/pulls"):
			_ = json.NewEncoder(w).Encode(pulls)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"unstubbed path"}`))
		}
	}))
	t.Cleanup(srv.Close)
	prev := reconcileGHClient
	reconcileGHClient = func(token string) *ghClient {
		return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
	}
	t.Cleanup(func() { reconcileGHClient = prev })
}

func foldMergedPull(n int, briefRef, headSHA string) map[string]any {
	return map[string]any{
		"number": n, "state": "closed", "body": "Does it.\n\nBrief: " + briefRef + "\n",
		"merged_at": "2026-10-01T00:00:00Z", "merge_commit_sha": strings.Repeat("a", 40),
		"head": map[string]any{"sha": headSHA, "ref": "feat/fx"},
	}
}

func foldReview(state, commitID string) map[string]any {
	return map[string]any{
		"state": state, "commit_id": commitID,
		"user": map[string]any{"login": "assay-reviewer-app[bot]"},
	}
}

// foldFixtureTree is the standard five-brief tree:
//
//	01 gate:model, witnessed, README implemented — merged PR #11 approved at head
//	02 gate:human, witnessed, README done with a human: Reviewed stamp — no PR needed
//	03 gate:model, unwitnessed, issues [7] — no PR; issue 7 carries `question`
//	04 gate:model, RED witness — merged PR #12 (stays at the PR base)
//	05 gate:model, unwitnessed, todo — nothing anywhere
func foldFixtureTree(t *testing.T) string {
	t.Helper()
	passRow := "| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-10-01 | app[bot] @ 000000000000 |\n"
	redRow := "| 1 | `true` | fail exit=1 | sha256:aaaaaaaaaaaa | 2026-10-01 | app[bot] @ 000000000000 |\n"
	return writeFoldFixture(t,
		[]foldFixtureBrief{
			{num: "01", id: "fx/01", gate: "model", witnessRow: passRow},
			{num: "02", id: "fx/02", gate: "human", witnessRow: passRow},
			{num: "03", id: "fx/03", gate: "model", issues: []int{7}},
			{num: "04", id: "fx/04", gate: "model", witnessRow: redRow},
			{num: "05", id: "fx/05", gate: "model"},
		},
		map[string][3]string{
			"01": {"implemented", "2026-10-01 verifier", "—"},
			"02": {"done", "2026-10-01 verifier", "2026-10-05 human:reviewer"},
			"03": {"todo", "—", "—"},
			"04": {"implemented", "—", "—"},
			"05": {"todo", "—", "—"},
		})
}

const foldPR11Head = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func foldCells(t *testing.T, root string) map[string]BriefCell {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code := runReconcile([]string{"--root", root, "--repo", "o/r", "--json"}, out, errf)
	e, _ := os.ReadFile(errf.Name())
	t.Logf("stderr: %s", e)
	if code != reconcileOK {
		b, _ := os.ReadFile(out.Name())
		t.Fatalf("reconcile exit %d, want 0:\n%s", code, b)
	}
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, b)
	}
	cells := map[string]BriefCell{}
	for _, c := range res.Briefs {
		cells[c.ID] = c
	}
	return cells
}

// TestReconcileFoldWiresAllFourInputs is the class-closing test for #1787: a
// full online reconcile over the fixture tree derives done (both gates),
// blocked, and the red-witness fallback — none of which is reachable while any
// of the four fold maps is left nil by the production constructor.
func TestReconcileFoldWiresAllFourInputs(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := foldFixtureTree(t)
	foldPullsServer(t,
		[]map[string]any{
			foldMergedPull(11, "fx/01", foldPR11Head),
			foldMergedPull(12, "fx/04", strings.Repeat("c", 40)),
		},
		map[int][]map[string]any{
			11: {foldReview("APPROVED", foldPR11Head)},
		},
		[]map[string]any{
			{"number": 7, "labels": []map[string]any{{"name": "question"}}},
		})

	cells := foldCells(t, root)

	if c := cells["fx/01"]; c.Cell != "done" {
		t.Errorf("fx/01: cell = %q (%s %s), want done — witness + App approval at head", c.Cell, c.Source, c.Reason)
	}
	if c := cells["fx/02"]; c.Cell != "done" {
		t.Errorf("fx/02: cell = %q (%s %s), want done — witness + human: Reviewed ruling", c.Cell, c.Source, c.Reason)
	}
	if c := cells["fx/03"]; c.Cell != "blocked" {
		t.Errorf("fx/03: cell = %q (%s %s), want blocked — linked issue 7 carries question", c.Cell, c.Source, c.Reason)
	}
	if c := cells["fx/04"]; c.Cell != "implemented" {
		t.Errorf("fx/04: cell = %q (%s %s), want implemented — a red witness promotes nothing", c.Cell, c.Source, c.Reason)
	}
	if c := cells["fx/05"]; c.Cell != "todo" {
		t.Errorf("fx/05: cell = %q (%s %s), want todo", c.Cell, c.Source, c.Reason)
	}
}

// TestReconcileFoldApprovalsFailClosed pins the three-state arm of the reviews
// read: when the forge reviews call fails, the brief derives no higher than
// verified and the run says so — never a silent done, never a silent drop.
func TestReconcileFoldApprovalsFailClosed(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := foldFixtureTree(t)
	foldPullsServer(t,
		[]map[string]any{
			foldMergedPull(11, "fx/01", foldPR11Head),
			foldMergedPull(12, "fx/04", strings.Repeat("c", 40)),
		},
		nil, // reviews read 500s
		[]map[string]any{})

	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code := runReconcile([]string{"--root", root, "--repo", "o/r", "--json"}, out, errf)
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0", code)
	}
	e, _ := os.ReadFile(errf.Name())
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Briefs {
		if c.ID == "fx/01" && c.Cell != "verified" {
			t.Fatalf("fx/01: cell = %q, want verified — a failed reviews read must not reach done", c.Cell)
		}
	}
	if !strings.Contains(string(e), "could-not-check") {
		t.Fatalf("a failed reviews read must be disclosed on stderr, got: %q", e)
	}
}

// TestReconcileFoldIssueLabelsFailClosed pins the same arm for the issues
// read: a failed labels read leaves the blocked overlay off AND says so.
func TestReconcileFoldIssueLabelsFailClosed(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := foldFixtureTree(t)
	foldPullsServer(t,
		[]map[string]any{
			foldMergedPull(11, "fx/01", foldPR11Head),
			foldMergedPull(12, "fx/04", strings.Repeat("c", 40)),
		},
		map[int][]map[string]any{
			11: {foldReview("APPROVED", foldPR11Head)},
		},
		nil) // issues read 500s

	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code := runReconcile([]string{"--root", root, "--repo", "o/r", "--json"}, out, errf)
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0", code)
	}
	e, _ := os.ReadFile(errf.Name())
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Briefs {
		if c.ID == "fx/03" && c.Cell != "todo" {
			t.Fatalf("fx/03: cell = %q, want todo — a failed issues read applies no blocked overlay", c.Cell)
		}
	}
	if !strings.Contains(string(e), "blocked overlay") {
		t.Fatalf("a failed issues read must be disclosed on stderr, got: %q", e)
	}
}

// TestReconcileFoldOfflineStaysUnknown pins that the wiring changes nothing
// about the offline arm: no tree or forge witness populates --offline.
func TestReconcileFoldOfflineStaysUnknown(t *testing.T) {
	root := foldFixtureTree(t)
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code := runReconcile([]string{"--root", root, "--offline", "--json"}, out, errf)
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0", code)
	}
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Briefs {
		if c.Cell != "unknown" {
			t.Fatalf("%s: offline cell = %q, want unknown for every brief", c.ID, c.Cell)
		}
	}
}

// TestRegenDriftComparatorDerivesFromWiredFold pins the SECOND production
// caller (#1787 names both): the regen drift comparator folds the same wired
// inputs and keys derived cells against README rows id-shape-tolerantly — a
// hierarchical brief-v2 id must still find its row.
func TestRegenDriftComparatorDerivesFromWiredFold(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := foldFixtureTree(t)
	foldPullsServer(t,
		[]map[string]any{
			foldMergedPull(11, "fx/01", foldPR11Head),
			foldMergedPull(12, "fx/04", strings.Repeat("c", 40)),
		},
		map[int][]map[string]any{
			11: {foldReview("APPROVED", foldPR11Head)},
		},
		[]map[string]any{
			{"number": 7, "labels": []map[string]any{{"name": "question"}}},
		})

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		s.Board = "generated"
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	notices := regenDriftNotices(root, streams, "o/r", false, "", errf)
	joined := strings.Join(notices, "\n")
	// fx/01: README says implemented, the wired fold derives done → drift.
	if !strings.Contains(joined, "fx/01") || !strings.Contains(joined, `"done"`) {
		t.Errorf("fx/01 implemented-vs-done drift must be NOTICEd, got:\n%s", joined)
	}
	// fx/02: README says done, the fold derives done → no drift line for it.
	if strings.Contains(joined, "fx/02") {
		t.Errorf("fx/02 agrees (done) and must produce no drift NOTICE, got:\n%s", joined)
	}
	// fx/03: README says todo, the fold derives blocked → drift.
	if !strings.Contains(joined, "fx/03") || !strings.Contains(joined, `"blocked"`) {
		t.Errorf("fx/03 todo-vs-blocked drift must be NOTICEd, got:\n%s", joined)
	}
}

// TestDriftNoticeHierarchicalID pins the comparator's join key directly:
// before the fix, a hierarchical (brief-v2) cell id never matched the
// stream/NN-shaped asserted map, so the comparator was blind on a v2 tree.
func TestDriftNoticeHierarchicalID(t *testing.T) {
	s := &Stream{
		Name:   "fx",
		Briefs: []Brief{{Num: "01", Status: "todo"}},
	}
	derived := []BriefCell{
		{ID: "cell:repo:fx:01", Cell: "implemented", Witness: "PR #7 (merged abc1234)"},
	}
	notices := assertedVsDerivedNotices(s, derived)
	if len(notices) != 1 {
		t.Fatalf("want exactly 1 drift NOTICE for the hierarchical id, got %d: %v", len(notices), notices)
	}
}
