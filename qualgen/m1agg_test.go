package main

import (
	"strings"
	"testing"
	"time"
)

// testMinedAt is a fixed run stamp for aggregator tests (the value the metrics
// snapshot records under `mined_at`).
var testMinedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// storeWith writes the given commits and their diffs into a fresh temp-rooted
// Store, so the aggregator is exercised through the real Store read path.
func storeWith(t *testing.T, commits []Commit, diffs []FileDiff) *Store {
	t.Helper()
	store := NewStore(t.TempDir())
	for _, c := range commits {
		if err := store.Append(KindCommit, c); err != nil {
			t.Fatalf("append commit: %v", err)
		}
	}
	for _, fd := range diffs {
		if err := store.Append(KindDiff, fd); err != nil {
			t.Fatalf("append diff: %v", err)
		}
	}
	return store
}

// findMetric returns the single metric record at the given metric/grain/key.
func findMetric(t *testing.T, recs []MetricRecord, metric, grain, key string) MetricRecord {
	t.Helper()
	for _, r := range recs {
		if r.Metric == metric && r.Grain == grain && r.Key == key {
			return r
		}
	}
	t.Fatalf("no metric %s/%s/%q in %d records", metric, grain, key, len(recs))
	return MetricRecord{}
}

// TestUnclassifiedIdentityClass is Verify row 5: an author matching no configured
// class lands in an explicit `unclassified` class and is never silently merged
// into human / agent / automation.
func TestUnclassifiedIdentityClass(t *testing.T) {
	idmap := &IdentityMap{Rules: []IdentityRule{
		{Pattern: `humans\.dev$`, Class: IdentityHuman},
		{Pattern: `\[bot\]`, Class: IdentityAgent},
	}}

	human := Commit{AuthorName: "Dev", AuthorEmail: "dev@humans.dev", AuthorRaw: "Dev <dev@humans.dev>"}
	agent := Commit{AuthorName: "worker-app[bot]", AuthorEmail: "x@y", AuthorRaw: "worker-app[bot] <x@y>"}
	stranger := Commit{AuthorName: "Someone Else", AuthorEmail: "someone@nowhere.example", AuthorRaw: "Someone Else <someone@nowhere.example>"}

	if got := idmap.Classify(human); got != IdentityHuman {
		t.Fatalf("human commit: got %q, want %q", got, IdentityHuman)
	}
	if got := idmap.Classify(agent); got != IdentityAgent {
		t.Fatalf("agent commit: got %q, want %q", got, IdentityAgent)
	}
	got := idmap.Classify(stranger)
	if got != IdentityUnclassified {
		t.Fatalf("unmapped author: got %q, want %q — must NOT be merged into a known class", got, IdentityUnclassified)
	}
	if got == IdentityHuman || got == IdentityAgent || got == IdentityAutomation {
		t.Fatalf("unmapped author was laundered into known class %q", got)
	}

	// And through the aggregator: the unclassified author's churn lands under an
	// explicit `unclassified` identity grain, distinct from the known classes.
	day := stranger.AuthorWhen // zero time is fine; churn just needs ordering
	_ = day
	commits := []Commit{
		{SHA: "s1", AuthorName: stranger.AuthorName, AuthorEmail: stranger.AuthorEmail, AuthorRaw: stranger.AuthorRaw},
	}
	diffs := []FileDiff{measuredLineDiff("s1", "z.go", addLine("net new line of code"))}
	store := storeWith(t, commits, diffs)
	cfg := DefaultM1Config()
	cfg.Identity = idmap
	if err := aggregateM1(store, cfg, testMinedAt); err != nil {
		t.Fatalf("aggregateM1: %v", err)
	}
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	r := findMetric(t, recs, MetricChurnRate, GrainIdentity, IdentityUnclassified)
	if r.Key != IdentityUnclassified {
		t.Fatalf("expected an explicit unclassified identity grain, got %q", r.Key)
	}
}

// TestCopyPasteRatioValue is Verify row 6: on a corpus with EXACTLY one copied
// block and one moved block, the emitted copy/paste ratio is the `measured`
// value 0.5 — dereferencing the computed number against a known-answer fixture.
func TestCopyPasteRatioValue(t *testing.T) {
	moved := []string{"func moved(a int) {", "    b := a + 41", "    c := b - 1", "    _ = c", "}"}
	copied := []string{"func copied(x int) {", "    y := x * 7", "    z := y % 3", "    _ = z", "}"}

	// Commit 1: a moved block (source lines deleted, block re-added).
	c1 := Commit{SHA: "c1", AuthorName: "dev", AuthorRaw: "dev <d@e>"}
	movedLines := []LineChange{}
	for _, s := range moved {
		movedLines = append(movedLines, delLine(s))
	}
	movedLines = append(movedLines, ctxLine("// anchor context line here"))
	for _, s := range moved {
		movedLines = append(movedLines, addLine(s))
	}
	d1 := measuredLineDiff("c1", "moved.go", movedLines...)

	// Commit 2: a copied block (source lines remain as context, block added).
	c2 := Commit{SHA: "c2", AuthorName: "dev", AuthorRaw: "dev <d@e>"}
	copiedLines := []LineChange{}
	for _, s := range copied {
		copiedLines = append(copiedLines, ctxLine(s))
	}
	copiedLines = append(copiedLines, ctxLine("// anchor context line here"))
	for _, s := range copied {
		copiedLines = append(copiedLines, addLine(s))
	}
	d2 := measuredLineDiff("c2", "copied.go", copiedLines...)

	store := storeWith(t, []Commit{c1, c2}, []FileDiff{d1, d2})
	if err := aggregateM1(store, DefaultM1Config(), testMinedAt); err != nil {
		t.Fatalf("aggregateM1: %v", err)
	}
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	r := findMetric(t, recs, MetricCopyPasteRatio, GrainRepo, "")
	if r.Value.State != StateMeasured {
		t.Fatalf("copy_paste_ratio state: got %q, want measured", r.Value.State)
	}
	if r.Value.Value != 0.5 {
		t.Fatalf("copy_paste_ratio: got %v, want 0.5 (1 copied / (1 moved + 1 copied))", r.Value.Value)
	}
	// Honest-claims discipline: labelled per the published definitions, never as
	// an equivalence claim.
	if r.Basis != basisPublishedDefinitions {
		t.Fatalf("basis: got %q, want %q", r.Basis, basisPublishedDefinitions)
	}
	if strings.Contains(strings.ToLower(r.Note), "equivalent") && !strings.Contains(r.Note, "not a GitClear-equivalence") {
		t.Fatalf("note must not claim GitClear-equivalence: %q", r.Note)
	}
}

// TestPerPackageBlockAttributionIsPrecise is the guard for review finding #3: a
// commit spanning two packages must credit a copied/moved block ONLY to the
// package that gained its added lines, never to every package the commit
// touched. Fail-first: replacing the per-file roll-up in aggregateM1 with the
// old whole-commit `mergeTaxonomy(pt, ct)` reddens this with pkgB reported as
// `copy_paste_ratio: got state=measured value=1, want measured-zero`.
func TestPerPackageBlockAttributionIsPrecise(t *testing.T) {
	block := []string{"func dup(x int) {", "    y := x * 7", "    z := y % 3", "    _ = z", "}"}

	// pkgA/a.go: a copied block — the source remains as context, the block is
	// re-added identically.
	var aLines []LineChange
	for _, s := range block {
		aLines = append(aLines, ctxLine(s))
	}
	aLines = append(aLines, ctxLine("// anchor context line here"))
	for _, s := range block {
		aLines = append(aLines, addLine(s))
	}
	dA := measuredLineDiff("c1", "pkgA/a.go", aLines...)

	// pkgB/b.go: plain net-new lines in the SAME commit — no duplication at all.
	dB := measuredLineDiff("c1", "pkgB/b.go",
		addLine("package pkgB"), addLine("const answer = 42"), addLine(`var who = "b"`))

	store := storeWith(t,
		[]Commit{{SHA: "c1", AuthorName: "dev", AuthorRaw: "dev <d@e>"}},
		[]FileDiff{dA, dB})
	if err := aggregateM1(store, DefaultM1Config(), testMinedAt); err != nil {
		t.Fatalf("aggregateM1: %v", err)
	}
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}

	// pkgA gained the copied block: its ratio is the measured 1.0.
	a := findMetric(t, recs, MetricCopyPasteRatio, GrainPackage, "pkgA")
	if a.Value.State != StateMeasured || a.Value.Value != 1 {
		t.Fatalf("pkgA copy_paste_ratio: got state=%q value=%v, want measured 1", a.Value.State, a.Value.Value)
	}
	// pkgB did NOT gain any block: a cross-directory commit must not inflate it
	// with pkgA's copied block. Its ratio is measured-zero, not 1.
	b := findMetric(t, recs, MetricCopyPasteRatio, GrainPackage, "pkgB")
	if b.Value.State != StateMeasuredZero {
		t.Fatalf("pkgB copy_paste_ratio: got state=%q value=%v, want measured-zero (no block landed in pkgB — cross-dir over-attribution)", b.Value.State, b.Value.Value)
	}
}

// TestMineMetricsCarryRunStamp is the guard for review finding #2: because the
// metrics table is append-only, every emitted M1 record must carry the run's
// MinedAt stamp — the same ordering key the sibling hotspot/ownership/coupling
// families record — so a trend consumer can select the latest snapshot per
// (metric,grain,key). Fail-first: dropping `MinedAt: minedAt` from aggregateM1's
// emit reddens this with a zero `mined_at`.
func TestMineMetricsCarryRunStamp(t *testing.T) {
	commits := []Commit{{SHA: "s1", AuthorName: "dev", AuthorRaw: "dev <d@e>"}}
	diffs := []FileDiff{measuredLineDiff("s1", "z.go", addLine("net new line of code"))}
	store := storeWith(t, commits, diffs)
	if err := aggregateM1(store, DefaultM1Config(), testMinedAt); err != nil {
		t.Fatalf("aggregateM1: %v", err)
	}
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("aggregateM1 produced no metrics")
	}
	for _, r := range recs {
		if !r.MinedAt.Equal(testMinedAt) {
			t.Fatalf("metric %s/%s/%q mined_at: got %v, want %v — an append-only snapshot with no run stamp cannot be ordered",
				r.Metric, r.Grain, r.Key, r.MinedAt, testMinedAt)
		}
	}
}

// TestMineEmitsM1Metrics is Verify row 7's Go-level dereference: a full mine over
// a real git fixture writes metrics.jsonl carrying the copy/paste ratio labelled
// per the published-definitions honest-claims discipline.
func TestMineEmitsM1Metrics(t *testing.T) {
	requireGit(t)
	repo := initFixtureRepo(t)
	out := t.TempDir()

	if err := mine(repo, out, &strings.Builder{}); err != nil {
		t.Fatalf("mine: %v", err)
	}
	store := NewStore(out)
	recs, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("mine produced no M1 metrics")
	}
	r := findMetric(t, recs, MetricCopyPasteRatio, GrainRepo, "")
	if r.Basis != basisPublishedDefinitions {
		t.Fatalf("copy_paste_ratio basis: got %q, want %q", r.Basis, basisPublishedDefinitions)
	}
	if !strings.Contains(r.Note, basisPublishedDefinitions) {
		t.Fatalf("note must carry the published-definitions label, got %q", r.Note)
	}
}
