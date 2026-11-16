package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// These behavioral tests deliberately use only APIs present at the original
// delivery. The immutable external harness can run the same tests against old
// source; a missing new metadata type cannot manufacture its failure.
func v2ConsumerFixture(t *testing.T, configure func(*gitV2Fixture)) *gitV2Fixture {
	t.Helper()
	f := newGitV2Fixture(t)
	readmeBytes, err := os.ReadFile(filepath.Join("..", "docs", "streams", "witnesses", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	f.ExtraFiles = map[string]string{"tools/x/go.sum": "", "docs/streams/witnesses/README.md": string(readmeBytes)}
	if configure != nil {
		configure(f)
	}
	f.build()
	raw, err := os.ReadFile(f.BriefPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "@ [w2] cr:") {
		t.Fatal("fixture has no actual v2 cells")
	}
	text := strings.Replace(string(raw), "inputs: "+f.Inputs, "inputs: "+f.Inputs+"\nfiles: `tools/x/pkg/a.go`", 1)
	text = strings.Replace(text, "\n## Review", "\n**VERIFY: PASS** — consumer flow fixture.\n\n## Review", 1)
	if err := os.WriteFile(f.BriefPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(f.Root, "docs", "streams", gv2Stream, "README.md")
	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte(strings.Replace(string(b), "| todo |", "| verified |", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	gv2GitAs(t, f.Root, fixtureVerifierName, fixtureVerifierEmail, "add", "-A")
	gv2GitAs(t, f.Root, fixtureVerifierName, fixtureVerifierEmail, "commit", "-q", "-m", "fixture: strict consumer PASS")
	return f
}

func v2ConsumerSource(f *gitV2Fixture) *fakeFlipSource {
	return &fakeFlipSource{
		commits:     map[string][]string{filepath.Base(f.BriefPath): {f.Base}},
		prs:         map[string]int{f.Base: 101},
		states:      map[int]prReviewState{101: {Merged: true, HeadSHA: f.Base, Reviews: []ghReview{{Author: ghAuthor{Login: afReviewer}, State: "APPROVED", CommitOID: f.Base, Id: "PRR_v2"}}}},
		shapes:      map[int]prShape{101: {Files: []string{"tools/x/pkg/a.go"}, BriefTrailers: 1, Briefs: []string{gv2BriefID}}},
		trailerHits: map[string][]int{gv2BriefID: {101}},
	}
}

type v2ConsumerResult struct {
	flip        modelFlipResult
	fold        WitnessInfo
	cell        BriefCell
	reviewReads int
}

func v2ConsumerResults(t *testing.T, f *gitV2Fixture, dryRun bool) v2ConsumerResult {
	t.Helper()
	streams, _, err := loadStreams(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	src := v2ConsumerSource(f)
	flips, err := autoFlipModel(f.Root, streams, src, ghReviewer(afReviewer), time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), dryRun)
	if err != nil {
		t.Fatal(err)
	}
	out := v2ConsumerResult{flip: afResult(t, flips, gv2BriefID), reviewReads: len(src.seen)}
	data, fstreams, err := loadReconcileFoldData(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	tree := treeFoldInputs(f.Root, fstreams, data)
	out.fold = tree.witnesses[gv2BriefID]
	cells := DeriveLifecycle(LifecycleInput{Briefs: []BriefIdent{{ID: gv2BriefID, Gate: "model", Version: 1}}, PRs: []PRRecord{{Number: 101, BriefRef: gv2BriefID, State: prMerged, MergeSHA: f.Base}}, Witnesses: tree.witnesses, Undecided: tree.undecided, LookedAt: true})
	if len(cells) != 1 {
		t.Fatalf("fold has %d cells", len(cells))
	}
	out.cell = cells[0]
	return out
}

// Coverage released valid v2 records while the actual fold discarded them
// because it only parsed the legacy execution-SHA cell.
// regression: #5684
func TestV2FoldConsumerFlow(t *testing.T) {
	f := v2ConsumerFixture(t, nil)
	if c := f.coverage(); !c.Released || len(c.Claims) != 2 {
		t.Fatalf("fixture did not release two actual v2 claims: %+v", c)
	}
	out := v2ConsumerResults(t, f, true)
	if !out.fold.Passed || !out.fold.Released || out.fold.Version != 1 || out.cell.Cell != "verified" {
		t.Fatalf("actual fold did not consume released v2: %+v", out)
	}
	if !strings.Contains(out.cell.Witness, "[w2] landing:"+f.Merge) || strings.Contains(out.cell.Witness, "landing:"+f.Base) {
		t.Fatalf("fold must display verified landing, not informational commit: %s", out.cell.Witness)
	}
	if out.flip.Outcome != flipDone || out.reviewReads == 0 {
		t.Fatalf("actual autoflip did not complete its decision: %+v", out)
	}
	data, _, err := loadReconcileFoldData(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flipLatestPass(data[0].evidence); err == nil {
		t.Fatal("the lower legacy write parser unexpectedly accepted a v2 run")
	}
}

// regression: #5684
func TestV2ConsumerImplicitDeps(t *testing.T) {
	cases := []struct{ name, path, content string }{
		{"source", "tools/x/pkg/a.go", "package pkg\n\nfunc A() int { return 3 }\n"},
		{"module", "tools/x/go.mod", "module example.test/x\n\ngo 1.25\n\n// changed\n"},
		{"lock", "tools/x/go.sum", "\n"},
		{"tool-pin", "paired-versions.yaml", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := v2ConsumerFixture(t, nil)
			before := v2ConsumerResults(t, f, true)
			if before.flip.Outcome != flipDone || !before.fold.Released || before.cell.Cell != "verified" {
				t.Fatalf("positive control failed: %+v", before)
			}
			path, content := tc.path, tc.content
			if tc.name == "tool-pin" {
				path = "plugins/assay/paired-versions.yaml"
				content = strings.Replace(gv2Pins, "statusgen/v1.0.59", "statusgen/v1.0.60", 1)
			}
			gv2Write(t, f.Root, path, content)
			gv2Git(t, f.Root, "add", "-A")
			gv2Git(t, f.Root, "commit", "-q", "-m", "fixture: changed required "+tc.name)
			after := v2ConsumerResults(t, f, true)
			if after.flip.Outcome == flipDone || after.fold.Released || after.cell.Cell == "verified" || after.cell.Cell == "done" {
				t.Fatalf("changed required %s was accepted by a real caller: %+v", tc.name, after)
			}
			if after.reviewReads != 0 {
				t.Fatalf("held coverage reached live review reads: %+v", after)
			}
		})
	}
}

// regression: #5684
func TestV2ConsumerFixtureWrite(t *testing.T) {
	f := v2ConsumerFixture(t, nil)
	out := v2ConsumerResults(t, f, false)
	if out.flip.Outcome != flipDone {
		t.Fatalf("autoflip fixture write refused: %+v", out)
	}
	b, err := os.ReadFile(filepath.Join(f.Root, "docs", "streams", gv2Stream, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "| done |") {
		t.Fatalf("real autoflip did not update the isolated fixture: %s", b)
	}
}

func consumerRewriteBrief(t *testing.T, f *gitV2Fixture, old, next string) {
	t.Helper()
	b, err := os.ReadFile(f.BriefPath)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(b), old, next, 1)
	if changed == string(b) {
		t.Fatal("fixture edit matched no text")
	}
	if err := os.WriteFile(f.BriefPath, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
}

// regression: #5684
func TestV2ConsumerInvalidRecords(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, *gitV2Fixture)
	}{
		{"missing-record", func(t *testing.T, f *gitV2Fixture) {
			if err := os.Remove(filepath.Join(f.Root, evaluation.WitnessRecordPath(gv2BriefID, f.RecordIDs["1"]))); err != nil {
				t.Fatal(err)
			}
		}},
		{"foreign-row", func(t *testing.T, f *gitV2Fixture) { consumerRewriteBrief(t, f, f.RecordIDs["1"], f.RecordIDs["2"]) }},
		{"malformed-record-id", func(t *testing.T, f *gitV2Fixture) { consumerRewriteBrief(t, f, f.RecordIDs["1"], "abcd") }},
		{"foreign-change-request", func(t *testing.T, f *gitV2Fixture) { consumerRewriteBrief(t, f, "cr:"+gv2CR+" ", "cr:8 ") }},
		{"missing-landing", func(t *testing.T, f *gitV2Fixture) {
			b, err := os.ReadFile(filepath.Join(f.Root, evaluation.WitnessRecordPath(gv2BriefID, f.RecordIDs["1"])))
			if err != nil {
				t.Fatal(err)
			}
			record, _, err := evaluation.ParseWitnessRecord(b)
			if err != nil {
				t.Fatal(err)
			}
			record.Informational.Commit = strings.Repeat("f", 40)
			canonical, id, err := evaluation.EncodeWitnessRecord(record)
			if err != nil {
				t.Fatal(err)
			}
			gv2Write(t, f.Root, evaluation.WitnessRecordPath(gv2BriefID, id), string(canonical))
			consumerRewriteBrief(t, f, f.RecordIDs["1"], id)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := v2ConsumerFixture(t, nil)
			before := v2ConsumerResults(t, f, true)
			if before.flip.Outcome != flipDone || !before.fold.Released {
				t.Fatalf("positive control failed: %+v", before)
			}
			tc.mutate(t, f)
			gv2GitAs(t, f.Root, fixtureVerifierName, fixtureVerifierEmail, "add", "-A")
			gv2GitAs(t, f.Root, fixtureVerifierName, fixtureVerifierEmail, "commit", "-q", "-m", "fixture: invalid "+tc.name)
			out := v2ConsumerResults(t, f, true)
			if out.flip.Outcome == flipDone || out.fold.Released || out.cell.Cell == "verified" || out.cell.Cell == "done" {
				t.Fatalf("invalid %s reached a real consumer: %+v", tc.name, out)
			}
			if out.reviewReads != 0 {
				t.Fatal("invalid coverage reached review fetch")
			}
		})
	}
}

// regression: #5684
func TestV2ConsumerLatestBad(t *testing.T) {
	f := v2ConsumerFixture(t, func(f *gitV2Fixture) {
		f.Wits = append(f.Wits, gv2Witness{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)", RecordCommand: "false"})
	})
	c := f.coverage()
	if c.Released || !strings.Contains(claimFor(t, c, "1").Reason, "witness-command-cell-mismatch") {
		t.Fatalf("latest bad witness borrowed the older good one: %+v", c)
	}
	out := v2ConsumerResults(t, f, true)
	if out.flip.Outcome == flipDone || out.fold.Released || out.cell.Cell == "verified" {
		t.Fatalf("latest bad witness reached a real consumer: %+v", out)
	}
}
