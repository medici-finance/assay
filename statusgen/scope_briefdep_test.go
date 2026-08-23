package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scope_briefdep_test.go — regression for the scoped-depends resolution bug.
//
// Under --changed/--scope, checkBriefFiles validates only the product-scoped
// subset of streams, but a brief's depends:/unblocks: may legitimately point at
// a stream that scoping dropped — another product's stream, or a paused stream.
// The cross-reference registry must therefore resolve against the FULL house
// set, not the scoped subset, or a valid dependency is falsely reported as an
// "unknown stream" (the required-check false positive it once produced). A genuinely-unknown/typo'd stream must still be caught.

// pausedDepBrief renders a minimal, checkBriefFiles-clean brief-v1 file body.
func pausedDepBrief(id, title string, wave int, depends []string) string {
	deps := "[]"
	if len(depends) > 0 {
		deps = `["` + strings.Join(depends, `", "`) + `"]`
	}
	w := "0"
	if wave == 1 {
		w = "1"
	}
	return "---\n" +
		"brief: " + id + "\n" +
		"title: " + title + "\n" +
		"wave: " + w + "\n" +
		"depends: " + deps + "\n" +
		"unblocks: []\n" +
		"effort: M\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-08-17 by fixture\n" +
		"sources: [\"fixture: valid path\"]\n" +
		"---\n\n# " + title + "\n\nBody.\n"
}

// writePausedDepFixture materializes a two-stream tree modelling the scoped-depends
// scenario: an active, product-scoped stream whose brief depends on a paused
// stream in a DIFFERENT product (so product-scoping drops the paused stream
// from the checked subset), plus a brief whose dependency is genuinely unknown.
func writePausedDepFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// multichain-ui: active, serves example-app (the scoped product).
	mcDir := filepath.Join(root, "docs", "streams", "multichain-ui")
	if err := os.MkdirAll(mcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mcReadme := "---\n" +
		"stream: multichain-ui\n" +
		"status: active\n" +
		"priority: P1\n" +
		"serves: example-app\n" +
		"---\n\n# Multichain UI\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 10 | [Base](./brief-10-base.md) | 0 | M | todo | — | — |\n" +
		"| 11 | [Dust fee panel](./brief-11-dust-fee-panel.md) | 1 | M | todo | — | — |\n" +
		"| 12 | [Ghost dep](./brief-12-ghost-dep.md) | 1 | M | todo | — | — |\n"
	writeFixtureFile(t, filepath.Join(mcDir, "README.md"), mcReadme)
	writeFixtureFile(t, filepath.Join(mcDir, "brief-10-base.md"), pausedDepBrief("multichain-ui/10", "Base", 0, nil))
	// brief-11 depends on a same-product brief AND a paused cross-product brief.
	writeFixtureFile(t, filepath.Join(mcDir, "brief-11-dust-fee-panel.md"),
		pausedDepBrief("multichain-ui/11", "Dust fee panel", 1, []string{"multichain-ui/10", "example-poc/10"}))
	// brief-12 depends on a stream that does not exist anywhere — a real typo.
	writeFixtureFile(t, filepath.Join(mcDir, "brief-12-ghost-dep.md"),
		pausedDepBrief("multichain-ui/12", "Ghost dep", 1, []string{"ghost-poc/10"}))

	// example-poc: PAUSED, serves example-service (a different product — so
	// scoping to example-app drops it from the checked subset).
	mpDir := filepath.Join(root, "docs", "streams", "example-poc")
	if err := os.MkdirAll(mpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mpReadme := "---\n" +
		"stream: example-poc\n" +
		"status: paused\n" +
		"priority: P2\n" +
		"serves: example-service\n" +
		"---\n\n# Example PoC\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 10 | [Funded preprod](./brief-10-funded-preprod.md) | 0 | M | todo | — | — |\n"
	writeFixtureFile(t, filepath.Join(mpDir, "README.md"), mpReadme)
	writeFixtureFile(t, filepath.Join(mpDir, "brief-10-funded-preprod.md"), pausedDepBrief("example-poc/10", "Funded preprod", 0, nil))

	return root
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hasUnknownStream reports whether some problem flags `ref`'s stream unknown.
func hasUnknownStream(problems []string, stream string) bool {
	for _, p := range problems {
		if strings.Contains(p, "unknown stream") && strings.Contains(p, `"`+stream+`"`) {
			return true
		}
	}
	return false
}

// TestChangedScopedDependsResolvesAgainstFullUniverse is the scoped-depends
// regression: a scoped lint run validates only the in-scope briefs, but a valid
// depends: on an out-of-scope / paused stream must resolve against the full
// house set — not be falsely flagged "unknown stream". A genuinely-missing
// stream must STILL be caught (the fix must not weaken unknown-stream
// detection).
func TestChangedScopedDependsResolvesAgainstFullUniverse(t *testing.T) {
	root := writePausedDepFixture(t)
	all, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	// Scope to example-app, exactly as --changed derives when the changed set is
	// confined to multichain-ui. This DROPS the paused example-service stream.
	scoped := filterStreamsByServes(all, "example-app")
	for _, s := range scoped {
		if s.Name == "example-poc" {
			t.Fatalf("fixture invalid: paused stream should be OUT of the example-app scope")
		}
	}

	// THE FIX: resolve refs against the full universe (`all`).
	fixed, _ := checkBriefFiles(scoped, all)
	if hasUnknownStream(fixed, "example-poc") {
		t.Errorf("valid depends on a paused/out-of-scope stream falsely flagged unknown: %v",
			filterUnknown(fixed))
	}
	// The genuine typo is still caught — narrowness of the fix.
	if !hasUnknownStream(fixed, "ghost-poc") {
		t.Errorf("genuinely-unknown stream must still flag unknown-stream: %v", fixed)
	}

	// OLD BEHAVIOUR (registry == scoped subset) reproduced the false positive:
	// this is the false positive that the fix removes.
	old, _ := checkBriefFiles(scoped, scoped)
	if !hasUnknownStream(old, "example-poc") {
		t.Fatalf("pre-fix reproduction failed: scoped-registry lint should have flagged the paused dep unknown — the test no longer proves the regression: %v", old)
	}

	// Whole-house (unscoped) lint was always clean for the paused dep — the bug
	// was scoping-only.
	whole, _ := checkBriefFiles(all, all)
	if hasUnknownStream(whole, "example-poc") {
		t.Errorf("whole-house lint should never flag the paused dep unknown: %v", filterUnknown(whole))
	}
}

func filterUnknown(problems []string) []string {
	var out []string
	for _, p := range problems {
		if strings.Contains(p, "unknown stream") {
			out = append(out, p)
		}
	}
	return out
}

// TestOneSidedDependsIsNoticeNotProblem pins the TIER of the dependency-edge
// reciprocity rule end-to-end (Ian's ruling, assay#92): a one-sided depends edge
// between two brief-v1 briefs must surface through checkBriefFiles as a NOTICE
// (non-fatal, exit 0), NOT a hard PROBLEM. This is the board-level guarantee that
// the rule can be tightened in HEAD without reddening consumers' statusgen-lint on
// their next repin — the reason the tier was lowered rather than the ~104 older
// one-sided edges reconciled up front.
func TestOneSidedDependsIsNoticeNotProblem(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", "recip")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\n" +
		"stream: recip\n" +
		"status: active\n" +
		"priority: P1\n" +
		"---\n\n# Recip\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Base](./brief-01-base.md) | 0 | M | todo | — | — |\n" +
		"| 02 | [Dependent](./brief-02-dependent.md) | 1 | M | todo | — | — |\n"
	writeFixtureFile(t, filepath.Join(sdir, "README.md"), readme)
	// brief-01 is the target and declares NO unblocks (pausedDepBrief always emits
	// `unblocks: []`), so brief-02's `depends: recip/01` is an unreciprocated,
	// one-sided edge — exactly the shape the reciprocity rule flags.
	writeFixtureFile(t, filepath.Join(sdir, "brief-01-base.md"),
		pausedDepBrief("recip/01", "Base", 0, nil))
	writeFixtureFile(t, filepath.Join(sdir, "brief-02-dependent.md"),
		pausedDepBrief("recip/02", "Dependent", 1, []string{"recip/01"}))

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, notices := checkBriefFiles(streams, streams)

	// The one-sided edge must NOT be a hard PROBLEM (that is the regression the
	// tier change prevents on the next release + repin).
	for _, p := range problems {
		if strings.Contains(p, "one-sided") {
			t.Fatalf("a one-sided depends edge must NOT be a PROBLEM (assay#92 lowered it to NOTICE); got problem: %s", p)
		}
	}
	// It must still be FLAGGED — as a NOTICE naming the offending edge, so the
	// data-quality debt stays visible and the rule remains meaningful.
	found := false
	for _, n := range notices {
		if strings.Contains(n, "one-sided") && strings.Contains(n, "recip/02") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("a one-sided depends edge must still surface as a NOTICE naming it; notices: %v", notices)
	}
}
