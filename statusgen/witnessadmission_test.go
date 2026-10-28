package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Public PR #2295 at merge 9137784: exact paths and patch, with the brief's
// Evidence retained at its original coordinates. See the fixture's provenance.
func reportedEvidence(t *testing.T) prShape {
	t.Helper()
	b, err := os.ReadFile("testdata/witness-admission/evidence2295.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Shape prShape `json:"shape"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if ok, why := evidenceOnlyReason(v.Shape); !ok {
		t.Fatalf("fixture is not Evidence: %s", why)
	}
	return v.Shape
}

func evidenceFiles(shape prShape) []map[string]any {
	var out []map[string]any
	for _, f := range shape.Files {
		out = append(out, map[string]any{"filename": f, "status": "modified"})
	}
	return out
}

// regression: #2703
func TestEvidenceAdmission(t *testing.T) {
	for _, backfill := range []bool{false, true} {
		for _, prior := range []bool{false, true} {
			t.Run(fmt.Sprint(backfill)+"/"+fmt.Sprint(prior), func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "test-token")
				root := copyDesignGateFixture(t)
				commitFixture(t, root)
				body := "Brief: dg/03\n"
				if backfill {
					body = ""
				}
				pulls := []map[string]any{mergedPull(2295, "verify/dg-03", body)}
				shape := reportedEvidence(t)
				files := map[int][]map[string]any{2295: evidenceFiles(shape)}
				if prior {
					pulls = append(pulls, mergedPull(1196, "feat/dg-03", body))
					files[1196] = deliveryFiles
				}
				authoringServer(t, pulls, files, nil)
				src := &fakeFlipSource{shapes: map[int]prShape{2295: shape}}
				foldFlipSeam(t, src)
				args := []string{"--apply", "--root", root, "--repo", "o/r", "--json"}
				if backfill {
					args = append(args, "--backfill")
				}
				code, out := runReconcileCaptured(t, args...)
				if code != reconcileOK {
					t.Fatalf("exit %d: %s", code, out)
				}
				got := cellOf(t, out, "dg/03")
				if prior {
					if got.Cell != "implemented" || !strings.Contains(applyWitness(got), "PR #1196 ") {
						t.Fatalf("delivery must remain witness: %+v", got)
					}
				} else if got.Cell == "implemented" || got.MergedPR != "" || len(appliedIDs(t, out)) != 0 {
					t.Fatalf("Evidence-only PR admitted: %+v", got)
				}
			})
		}
	}
}

// regression: #2703
func TestEvidenceReadUnknown(t *testing.T) {
	for _, kind := range []string{"read error", "missing head", "missing patch"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "test-token")
			root := copyDesignGateFixture(t)
			shape := reportedEvidence(t)
			authoringServer(t, []map[string]any{mergedPull(2295, "verify/dg", "Brief: dg/03\n")}, map[int][]map[string]any{2295: evidenceFiles(shape)}, nil)
			src := &fakeFlipSource{shapes: map[int]prShape{2295: shape}}
			switch kind {
			case "read error":
				src.shapeErrs = map[int]error{2295: errors.New("shape unavailable")}
			case "missing head":
				shape.Heads = nil
				src.shapes[2295] = shape
			case "missing patch":
				shape.Patches = nil
				src.shapes[2295] = shape
			}
			foldFlipSeam(t, src)
			before := readFixtureReadme(t, root)
			code, out := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
			if code != reconcileCouldNotCheck || readFixtureReadme(t, root) != before {
				t.Fatalf("unread shape must be unknown and write nothing: exit %d %s", code, out)
			}
			code, out = runReconcileCaptured(t, "--root", root, "--repo", "o/r", "--json")
			if code != reconcileOK || cellOf(t, out, "dg/03").Cell != "unknown" {
				t.Fatalf("read-only derivation must disclose unknown: exit %d %s", code, out)
			}
		})
	}
}

// regression: #2703
func TestEvidenceDriftAndCost(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	shape := reportedEvidence(t)
	reads := authoringServer(t, []map[string]any{
		mergedPull(2295, "verify/dg", "Brief: dg/03\n"),
		mergedPull(1196, "feat/dg", "Brief: dg/03\n"),
		mergedPull(2300, "verify/dg05", "Brief: dg/05\n"),
	}, map[int][]map[string]any{2295: evidenceFiles(shape), 1196: deliveryFiles, 2300: {{"filename": "docs/streams/verify-outcomes/dg/05.json", "status": "added"}}}, nil)
	src := &fakeFlipSource{shapes: map[int]prShape{2295: shape}}
	foldFlipSeam(t, src)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		s.Board = "generated"
	}
	ef, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	notices := regenDriftNotices(root, streams, "o/r", false, "", ef)
	found := false
	for _, n := range notices {
		if strings.Contains(n, "dg/03") && strings.Contains(n, "PR #1196 ") {
			found = true
		}
		if strings.Contains(n, "PR #2295 ") || strings.Contains(n, "PR #2300 ") {
			t.Fatalf("non-delivery provenance: %s", n)
		}
	}
	if !found {
		t.Fatalf("delivery provenance absent: %v", notices)
	}
	if len(*reads) != 3 || len(src.shapesSeen) != 1 {
		t.Fatalf("cohort needs 3 file-list reads, 1 shape read: files=%v shapes=%v", *reads, src.shapesSeen)
	}
	t.Logf("cohort: files=%d shapes=%d (one ordinary delivery, one Evidence PR, one outcome-only second instance)", len(*reads), len(src.shapesSeen))
}

// Current corpus is the admission input, not a PR-number exception in code.
// Public source scopes: #735 migration-blocking statusgen fixes only; #736
// migration only; #945 backfill only; #1010 apply only; #1194 rollout slice.
// All leave brief Task items 2–4 undelivered. #735 also carries the trailer,
// so leaving it out lets it become the witness once the other four drop.
// regression: #2703
func TestRolloutSourceAdmission(t *testing.T) {
	d := readDeclaredPartials("..", "medici-finance/assay")
	prs := []PRRecord{}
	// The partial members come from the coverage-checked cohort (derived-board/10), never a
	// hand-kept list: candidate-cohort.json is joined to a complete inventory by TestCandidateCoverage.
	_, cohort := candMustLoad(t)
	var partials []int
	for _, m := range cohort.Members {
		if m.Disposition == "partial" {
			partials = append(partials, m.Number)
		}
	}
	if len(partials) == 0 {
		t.Fatal("the cohort records no partial members")
	}
	for _, n := range partials {
		if !d.partial(n, "derived-board/07") {
			t.Errorf("rollout PR #%d is not declared partial", n)
		}
		prs = append(prs, PRRecord{BriefRef: "derived-board/07", Number: n, State: prMerged, MergeSHA: "abcdef0"})
	}
	markDeclaredPartials(prs, d, nil)
	c := deriveSingle(t, BriefIdent{ID: "derived-board/07"}, LifecycleInput{LookedAt: true, PRs: prs})
	if c.Cell != "in-progress" || c.MergedPR != "" {
		t.Fatalf("rollout slice became complete: %+v", c)
	}
}

// PR #2655 delivers the analyzer action and local controls, but its body
// disclaims deployment. The current brief's Tasks 1 and 5 still require CI
// acquisition and the human-applied workflow before delivery is complete.
// regression: #2703
func TestCISliceSourceAdmission(t *testing.T) {
	d := readDeclaredPartials("..", "medici-finance/assay")
	if !d.partial(2655, "desk-tools/34") {
		t.Error("analyzer PR #2655 is not declared partial")
	}
	prs := []PRRecord{{BriefRef: "desk-tools/34", Number: 2655, State: prMerged, MergeSHA: "7efb00c"}}
	markDeclaredPartials(prs, d, nil)
	c := deriveSingle(t, BriefIdent{ID: "desk-tools/34"}, LifecycleInput{LookedAt: true, PRs: prs})
	if c.Cell != "in-progress" || c.MergedPR != "" {
		t.Fatalf("local analyzer slice became complete: %+v", c)
	}
}

// Merged slices of two briefs whose own Task items stay owed. harness-portability/13:
// PRs #621 and #3705 land the protocol, run-log scaffold and README row, while Task 3
// (bundle PARITY/RELEASE-NOTES record) and Task 4 (the first live run and its log) are
// still owed. desk-supervision/08: PR #1266 delivers the kit and arm artifacts, and #3239
// repairs its checks, but both record that the observer logs Task 3 reads are absent,
// so Task 3's wedge counts and Task 4's decision line remain owed. The control derives
// the same merges without the brief's claims and must reach implemented, so the
// brief metadata, not the fold, is what holds each row in progress.
// regression: #2703
func TestOwedTaskSourceAdmission(t *testing.T) {
	d := readDeclaredPartials("..", "medici-finance/assay")
	for _, tc := range []struct {
		brief string
		prs   []int
	}{
		{"harness-portability/13", []int{621, 3705}},
		{"desk-supervision/08", []int{1266, 3239}},
	} {
		t.Run(tc.brief, func(t *testing.T) {
			var prs, control []PRRecord
			for _, n := range tc.prs {
				if !d.partial(n, tc.brief) {
					t.Errorf("PR #%d is not declared partial for %s", n, tc.brief)
				}
				pr := PRRecord{BriefRef: tc.brief, Number: n, State: prMerged, MergeSHA: "abcdef0"}
				prs = append(prs, pr)
				control = append(control, pr)
			}
			markDeclaredPartials(prs, d, nil)
			b := BriefIdent{ID: tc.brief}
			if c := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: control}); c.Cell != "implemented" {
				t.Fatalf("control: undeclared merges must derive implemented: %+v", c)
			}
			if c := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: prs}); c.Cell != "in-progress" || c.MergedPR != "" {
				t.Fatalf("slice with owed Task items became complete: %+v", c)
			}
		})
	}
}

// The same cohort enters trailer admission and then backfill. One cached file
// read per PR and one shared shape read for the Evidence candidate suffice.
// A real acceptance-definition delivery stays admissible even with VERIFY FAIL.
// regression: #2703
func TestAdmissionCacheAndDelivery(t *testing.T) {
	shape := reportedEvidence(t)
	definition := reportedEvidence(t)
	for p := range definition.Patches {
		definition.Patches[p] = "@@ -2 +2 @@\n-brief: desk-tools/26\n+brief: desk-tools/27\n"
	}
	reads := authoringServer(t, nil, map[int][]map[string]any{
		2295: evidenceFiles(shape), 1196: deliveryFiles, 3000: evidenceFiles(definition),
	}, nil)
	src := &fakeFlipSource{shapes: map[int]prShape{2295: shape, 3000: definition}}
	foldFlipSeam(t, src)
	cl := newAuthoringClassifier(reconcileGHClient("test-token"), "o/r", nil)
	for run := 0; run < 2; run++ {
		for _, tc := range []struct {
			n        int
			excluded bool
		}{{2295, true}, {1196, false}, {3000, false}} {
			got, err := cl.nonDelivery(tc.n, "desk-tools/27")
			if err != nil || got != tc.excluded {
				t.Fatalf("PR #%d excluded=%v err=%v", tc.n, got, err)
			}
		}
	}
	if len(*reads) != 3 || len(src.shapesSeen) != 2 {
		t.Fatalf("read cache drift: files=%v shapes=%v", *reads, src.shapesSeen)
	}
	b := BriefIdent{ID: "desk-tools/27", Version: 1}
	c := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: []PRRecord{{BriefRef: b.ID, Number: 1196, State: prMerged, MergeSHA: "a4700d2"}}, Witnesses: map[string]WitnessInfo{b.ID: {Passed: false, Version: 1}}})
	if c.Cell != "implemented" || !strings.Contains(c.MergedPR, "PR #1196 ") {
		t.Fatalf("failed verification erased implementation: %+v", c)
	}
}

// A second brief with an outcome-only PR exercises the untouched sibling path.
// regression: #2703
func TestOutcomeSecondInstance(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	authoringServer(t, []map[string]any{mergedPull(2300, "verify/dg-05", "Brief: dg/05\n")}, map[int][]map[string]any{2300: {{"filename": "docs/streams/verify-outcomes/dg/05.json", "status": "added"}}}, nil)
	code, out := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK || cellOf(t, out, "dg/05").Cell != "todo" || len(appliedIDs(t, out)) != 0 {
		t.Fatalf("second instance: outcome-only PR #2300 became an implementation witness: exit %d, cell %+v", code, cellOf(t, out, "dg/05"))
	}
}
