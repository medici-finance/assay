package main

// reconcilepartial_test.go — a merged PR that the brief's own `delivery:` list declares a
// PARTIAL delivery (a skeleton, one slice of several) is not a witness that the brief is
// implemented (#2703). It derives in-progress: part landed, the rest is owed. A merge with no
// such claim, a later delivery, or a `full` entry for the same PR stays a witness.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// partialRegistry is a graph-repos.yaml where fx is the reconciled repo (o/r), sib a sibling,
// and hid a withheld alias.
const partialRegistry = `schema: graph-repos-v1
cell: fx
repos:
  fx:  {cell: fx, repo: o/r}
  sib: {cell: fx, repo: o/sib}
  hid: {cell: fx, repo: null, unpublished: true}
`

// skeletonFiles is a PR that lands real deliverable files whose content is still a skeleton —
// no file-shape rule can tell it from a delivery.
var skeletonFiles = []map[string]any{
	{"filename": "docs/streams/dg/conformance-report.md", "status": "added"},
	{"filename": "changelog/dg-report.md", "status": "added"},
}

// declareDelivery writes the registry and appends a delivery: list to brief dg/<nn>.
func declareDelivery(t *testing.T, root, nn string, entries ...string) {
	t.Helper()
	reg := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
	if err := os.WriteFile(reg, []byte(partialRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", "dg", "brief-"+nn+"-*.md"))
	if len(matches) != 1 {
		t.Fatalf("brief dg/%s: %v", nn, matches)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	block := "delivery:\n"
	for _, e := range entries {
		block += "  - " + e + "\n"
	}
	s := strings.Replace(string(raw), "issues: []\n", "issues: []\n"+block, 1)
	if s == string(raw) {
		t.Fatalf("brief dg/%s has no issues: [] line to anchor on", nn)
	}
	if err := os.WriteFile(matches[0], []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cellOf returns the derived cell for id from reconcile --json output.
func cellOf(t *testing.T, stdout, id string) BriefCell {
	t.Helper()
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	for _, c := range res.Briefs {
		if canonicalBriefKey(c.ID) == id {
			return c
		}
	}
	t.Fatalf("no cell for %s in:\n%s", id, stdout)
	return BriefCell{}
}

// The reported shape: the only merged trailer PR is a declared skeleton.
// regression: #2703
func TestPartialMergeNoWitness(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "fx#30", covers: partial, note: "report skeleton; every row still to run"}`)
	authoringServer(t, []map[string]any{
		mergedPull(30, "feat/dg-report", "Brief: dg/03\n"),
	}, map[int][]map[string]any{30: skeletonFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 0 {
		t.Fatalf("a declared-partial merge flipped a row: %v", got)
	}
	c := cellOf(t, stdout, "dg/03")
	if c.Cell != "in-progress" || !strings.Contains(c.Witness, "partial") {
		t.Fatalf("dg/03 want in-progress on a declared partial, got %+v", c)
	}
}

// The planted SECOND instance: another brief (a todo row), another PR, several slices.
// regression: #2703
func TestPartialSecondInstance(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "05",
		`{in: "fx#41", covers: partial, note: "slice 1"}`,
		`{in: "fx#44", covers: partial, note: "slice 2"}`)
	authoringServer(t, []map[string]any{
		mergedPull(44, "feat/dg-05-b", "Brief: dg/05\n"),
		mergedPull(41, "feat/dg-05-a", "Brief: dg/05\n"),
	}, map[int][]map[string]any{41: deliveryFiles, 44: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 0 {
		t.Fatalf("declared slices flipped a row: %v", got)
	}
	if c := cellOf(t, stdout, "dg/05"); c.Cell != "in-progress" || !strings.HasPrefix(c.Witness, "PR #44") {
		t.Fatalf("dg/05 want in-progress at the latest slice #44, got %+v", c)
	}
}

// The valid-implementation control: an authoring PR, a declared slice, then the delivery. The
// delivery is the witness.
// regression: #2703
func TestPartialThenDelivery(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "fx#30", covers: partial, note: "slice 1"}`)
	authoringServer(t, []map[string]any{
		mergedPull(35, "feat/dg-03-rest", "Brief: dg/03\n"),
		mergedPull(31, "docs/dg-author", "Brief: dg/03\n"),
		mergedPull(30, "feat/dg-03-a", "Brief: dg/03\n"),
	}, map[int][]map[string]any{35: deliveryFiles, 31: authoringFiles, 30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	got := appliedIDs(t, stdout)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #35") {
		t.Fatalf("want dg/03 witnessed by delivery PR #35, got %v", got)
	}
}

// A later `full` entry for the same PR supersedes the partial (append-only history).
// regression: #2703
func TestPartialSupersededFull(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03",
		`{in: "fx#30", covers: partial, note: "first pass"}`,
		`{in: "fx#30", covers: full}`)
	authoringServer(t, []map[string]any{
		mergedPull(30, "feat/dg-03", "Brief: dg/03\n"),
	}, map[int][]map[string]any{30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want dg/03 witnessed by PR #30 (full supersedes partial), got %v", got)
	}
}

// A partial claim naming a SIBLING repo says nothing about this repo's PR of the same number.
// regression: #2703
func TestPartialSiblingIgnored(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "sib#30", covers: partial, note: "sibling slice"}`)
	authoringServer(t, []map[string]any{
		mergedPull(30, "feat/dg-03", "Brief: dg/03\n"),
	}, map[int][]map[string]any{30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want dg/03 witnessed by PR #30, got %v", got)
	}
}

// A partial claim that cannot be resolved is could-not-check: unknown, and --apply refuses.
// regression: #2703
func TestPartialUnresolvedUnknown(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	for name, in := range map[string]string{"withheld": "hid#30", "unknown-alias": "zz#30", "bare": "#30"} {
		t.Run(name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			declareDelivery(t, root, "03", `{in: "`+in+`", covers: partial, note: "x"}`)
			before := readFixtureReadme(t, root)
			authoringServer(t, []map[string]any{
				mergedPull(30, "feat/dg-03", "Brief: dg/03\n"),
			}, map[int][]map[string]any{30: deliveryFiles}, nil)

			code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
			if code != reconcileCouldNotCheck {
				t.Fatalf("exit %d, want %d:\n%s", code, reconcileCouldNotCheck, stdout)
			}
			if readFixtureReadme(t, root) != before {
				t.Fatal("an unresolved partial claim must write nothing")
			}
		})
	}
}

// The --backfill reader takes the same exemption: a declared-partial branch match is not the
// witness; an older delivery for the same brief is.
// regression: #2703
func TestPartialBackfillSkips(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "fx#31", covers: partial, note: "skeleton"}`)
	commitFixture(t, root)
	authoringServer(t, []map[string]any{
		mergedPull(31, "feat/dg-03-skeleton", ""),
		mergedPull(30, "feat/dg-03-impl", ""),
	}, map[int][]map[string]any{31: skeletonFiles, 30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	got := appliedIDs(t, stdout)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want dg/03 witnessed by PR #30 (not declared-partial #31), got %v", got)
	}
}

// The regen drift comparator shares the reader: no NOTICE derives dg/03 "implemented".
// regression: #2703
func TestPartialRegenDriftAgrees(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "fx#30", covers: partial, note: "skeleton"}`)
	authoringServer(t, []map[string]any{
		mergedPull(30, "feat/dg-report", "Brief: dg/03\n"),
	}, map[int][]map[string]any{30: skeletonFiles}, nil)
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
	for _, n := range regenDriftNotices(root, streams, "o/r", false, "", errf) {
		if strings.Contains(n, "dg/03") {
			t.Fatalf("the board's in-progress dg/03 is what a declared partial derives; got:\n%s", n)
		}
	}
}

// The pure fold: a Partial merge is in-progress, and does not hide a later real merge.
// regression: #2703
func TestDerivePartialInProgress(t *testing.T) {
	b := BriefIdent{ID: "s/01", Gate: "model", Version: 1}
	got := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: []PRRecord{
		{BriefRef: "s/01", Number: 9, State: prMerged, MergeSHA: "0123456789ab", Partial: true},
	}})
	if got.Cell != "in-progress" || got.MergedPR != "" || !strings.Contains(got.Witness, "PR #9") {
		t.Fatalf("a declared-partial merge must derive in-progress with no merge on record, got %+v", got)
	}
	got = deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: []PRRecord{
		{BriefRef: "s/01", Number: 9, State: prMerged, MergeSHA: "0123456789ab", Partial: true},
		{BriefRef: "s/01", Number: 7, State: prMerged, MergeSHA: "fedcba987654"},
	}})
	if got.Cell != "implemented" || !strings.HasPrefix(got.Witness, "PR #7") {
		t.Fatalf("an unflagged merge stays the witness, got %+v", got)
	}
}

// A VERIFY FAIL does not mean the implementation never landed: a merged delivery with a failing
// witness stays implemented (the implementation is awaiting a passing verification).
// regression: #2703
func TestVerifyFailStaysImpl(t *testing.T) {
	b := BriefIdent{ID: "s/01", Gate: "model", Version: 1}
	got := deriveSingle(t, b, LifecycleInput{LookedAt: true,
		PRs: []PRRecord{
			{BriefRef: "s/01", Number: 7, State: prMerged, MergeSHA: "fedcba987654"},
		},
		Witnesses: map[string]WitnessInfo{"s/01": {Passed: false, Version: 1, Released: false}},
	})
	if got.Cell != "implemented" || !strings.HasPrefix(got.MergedPR, "PR #7") {
		t.Fatalf("a failed verify must not undo the merge on record, got %+v", got)
	}
}
