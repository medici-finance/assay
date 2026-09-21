package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const auditPackTestdataRoot = "testdata/auditpack"

func TestLoadReleaseRecord(t *testing.T) {
	t.Run("found with frontmatter", func(t *testing.T) {
		rec, err := loadReleaseRecord(auditPackTestdataRoot, "v1.0.0-full")
		if err != nil {
			t.Fatalf("loadReleaseRecord: %v", err)
		}
		if rec.Release != "v1.0.0-full" {
			t.Errorf("Release = %q, want v1.0.0-full", rec.Release)
		}
		if len(rec.Briefs) != 1 || rec.Briefs[0] != "fx/01" {
			t.Errorf("Briefs = %v, want [fx/01]", rec.Briefs)
		}
		if len(rec.Requirements) != 2 {
			t.Errorf("Requirements = %v, want 2 entries", rec.Requirements)
		}
	})

	t.Run("found, legacy prose-only note", func(t *testing.T) {
		rec, err := loadReleaseRecord(auditPackTestdataRoot, "v0.9.0-legacy")
		if err != nil {
			t.Fatalf("loadReleaseRecord: %v", err)
		}
		if rec.Release != "v0.9.0-legacy" {
			t.Errorf("Release = %q, want v0.9.0-legacy", rec.Release)
		}
		if len(rec.Briefs) != 0 || len(rec.Requirements) != 0 {
			t.Errorf("a legacy prose-only note must have EMPTY scope, got briefs=%v requirements=%v", rec.Briefs, rec.Requirements)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := loadReleaseRecord(auditPackTestdataRoot, "v-does-not-exist")
		if err != errReleaseNotFound {
			t.Fatalf("loadReleaseRecord(nonexistent) error = %v, want errReleaseNotFound", err)
		}
	})
}

func TestParseReviewedCell(t *testing.T) {
	cases := []struct {
		cell        string
		date, ident string
		pr          int
		sha         string
	}{
		{"2026-01-03 test-reviewer (approved PR #42 @ 1111111111111111111111111111111111111111)",
			"2026-01-03", "test-reviewer", 42, "1111111111111111111111111111111111111111"},
		{"2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931)",
			"2026-09-02", "assay-reviewer-app[bot]", 156, "112b206fee74b470016be325dc7c2dfeff670931"},
		{"2026-01-03 opus-4.8[1m]-verifier", "2026-01-03", "opus-4.8[1m]-verifier", 0, ""},
		{"—", "", "", 0, ""},
		{"", "", "", 0, ""},
	}
	for _, c := range cases {
		date, ident, pr, sha := parseReviewedCell(c.cell)
		if date != c.date || ident != c.ident || pr != c.pr || sha != c.sha {
			t.Errorf("parseReviewedCell(%q) = (%q,%q,%d,%q), want (%q,%q,%d,%q)",
				c.cell, date, ident, pr, sha, c.date, c.ident, c.pr, c.sha)
		}
	}
}

func TestCollectAuditPackRequirements(t *testing.T) {
	rec, err := loadReleaseRecord(auditPackTestdataRoot, "v1.0.0-full")
	if err != nil {
		t.Fatalf("loadReleaseRecord: %v", err)
	}
	reqs, paths, readmes, omitted, err := collectAuditPackRequirements(auditPackTestdataRoot, rec)
	if err != nil {
		t.Fatalf("collectAuditPackRequirements: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d requirements, want 2: %+v", len(reqs), reqs)
	}

	var ok, gap *auditPackRequirement
	for i := range reqs {
		switch reqs[i].ID {
		case "REQ-fixture-ok-01":
			ok = &reqs[i]
		case "REQ-fixture-gap-01":
			gap = &reqs[i]
		}
	}
	if ok == nil || gap == nil {
		t.Fatalf("missing expected requirement ids in %+v", reqs)
	}

	// REQ-fixture-ok-01: one resolved, done backing brief -> satisfied, with
	// its PR/review detail parsed off the Reviewed cell.
	if ok.State != rollupStateSatisfied {
		t.Errorf("REQ-fixture-ok-01 state = %q, want %q", ok.State, rollupStateSatisfied)
	}
	if len(ok.Briefs) != 1 || ok.Briefs[0].Brief != "fx/01" || !ok.Briefs[0].Resolved {
		t.Fatalf("REQ-fixture-ok-01 backing = %+v", ok.Briefs)
	}
	if ok.Briefs[0].PR != 42 || ok.Briefs[0].ReviewedBy != "test-reviewer" || ok.Briefs[0].ReviewedDate != "2026-01-03" {
		t.Errorf("REQ-fixture-ok-01 PR/review detail = %+v, want PR 42 by test-reviewer on 2026-01-03", ok.Briefs[0])
	}

	// REQ-fixture-gap-01: satisfied-by names fx/99, which does not exist ->
	// could-not-check, a reason, and NEVER satisfied (task item 3).
	if gap.State != rollupStateCouldNotCheck {
		t.Errorf("REQ-fixture-gap-01 state = %q, want %q", gap.State, rollupStateCouldNotCheck)
	}
	if gap.Reason == "" {
		t.Error("REQ-fixture-gap-01 must carry a could-not-check reason")
	}

	// The could-not-check requirement is recorded in `omitted` — visible, never
	// silent (task item 3) — while a genuinely resolved one is not.
	foundGapOmission := false
	for _, o := range omitted {
		if strings.Contains(o.Path, "REQ-fixture-gap-01") {
			foundGapOmission = true
			if !strings.Contains(o.Reason, "could-not-check") {
				t.Errorf("omitted reason for the gap requirement must say could-not-check, got %q", o.Reason)
			}
		}
		if strings.Contains(o.Path, "REQ-fixture-ok-01") {
			t.Errorf("REQ-fixture-ok-01 resolved cleanly and must not appear in omitted, got %+v", o)
		}
	}
	if !foundGapOmission {
		t.Errorf("expected an omitted entry for REQ-fixture-gap-01, got %+v", omitted)
	}

	if _, ok := paths["fx/01"]; !ok {
		t.Errorf("expected fx/01 in the brief-path index, got %v", paths)
	}
	if _, ok := readmes["fx"]; !ok {
		t.Errorf("expected fx in the stream-README index, got %v", readmes)
	}
}

func TestAuditPackCoverageAgreementCatchesDroppedBacking(t *testing.T) {
	rec, err := loadReleaseRecord(auditPackTestdataRoot, "v1.0.0-full")
	if err != nil {
		t.Fatalf("loadReleaseRecord: %v", err)
	}
	reqs, _, _, _, err := collectAuditPackRequirements(auditPackTestdataRoot, rec)
	if err != nil {
		t.Fatalf("collectAuditPackRequirements: %v", err)
	}

	t.Run("agrees when nothing is mutated", func(t *testing.T) {
		agree, mine, rollup, detail := auditPackCoverageAgreement(auditPackTestdataRoot, reqs)
		if !agree {
			t.Fatalf("expected agreement on the unmutated collection, got mine=%d rollup=%d detail=%q", mine, rollup, detail)
		}
		if mine != rollup {
			t.Errorf("mine=%d != rollup=%d despite agree=true", mine, rollup)
		}
	})

	// LAYER-INDEPENDENCE (Verify row 4 / rule 16): simulate "the collector
	// skips one brief's Evidence" by dropping REQ-fixture-ok-01's one backing
	// entry from THIS SIDE ONLY, leaving the rollup call inside
	// auditPackCoverageAgreement completely untouched. Fail-first: this
	// assertion fails (agree stays true, mine still equals rollup) against the
	// code as it stood before auditPackCoverageAgreement existed to catch it.
	t.Run("disagrees when this side drops a backing entry", func(t *testing.T) {
		mutated := make([]auditPackRequirement, len(reqs))
		copy(mutated, reqs)
		for i := range mutated {
			if mutated[i].ID == "REQ-fixture-ok-01" {
				mutated[i].Briefs = nil // the mutation: collector "forgot" its one backing brief
			}
		}
		agree, mine, rollup, detail := auditPackCoverageAgreement(auditPackTestdataRoot, mutated)
		if agree {
			t.Fatalf("expected disagreement after dropping REQ-fixture-ok-01's backing brief, got agree=true mine=%d rollup=%d", mine, rollup)
		}
		if mine == rollup {
			t.Errorf("mine=%d == rollup=%d, want a real mismatch", mine, rollup)
		}
		if !strings.Contains(detail, "REQ-fixture-ok-01") {
			t.Errorf("detail must name the disagreeing requirement, got %q", detail)
		}
	})
}

func TestBuildAuditPackBundle(t *testing.T) {
	rec, err := loadReleaseRecord(auditPackTestdataRoot, "v1.0.0-full")
	if err != nil {
		t.Fatalf("loadReleaseRecord: %v", err)
	}
	paths, bundle, omitted, doc, err := buildAuditPackBundle(auditPackTestdataRoot, rec)
	if err != nil {
		t.Fatalf("buildAuditPackBundle: %v", err)
	}

	want := []string{
		"docs/release-notes/v1.0.0-full.md",
		"docs/streams/fx/README.md",
		"docs/streams/fx/brief-01-ok.md",
		"docs/streams/requirements/fixture-ok-01.md",
		"docs/streams/requirements/fixture-gap-01.md",
		auditPackReportPath,
	}
	for _, w := range want {
		if _, ok := bundle[w]; !ok {
			t.Errorf("expected %s in the bundle, got paths=%v", w, paths)
		}
	}
	if len(omitted) != 1 {
		t.Fatalf("expected exactly 1 omitted entry (the gap requirement), got %+v", omitted)
	}
	if doc.Coverage.RequirementsInScope != 2 {
		t.Errorf("Coverage.RequirementsInScope = %d, want 2", doc.Coverage.RequirementsInScope)
	}
	if doc.Coverage.ResolvedBackingLinks != doc.Coverage.RollupBackingLinks {
		t.Errorf("collector/rollup coverage disagree on the unmutated fixture: %+v", doc.Coverage)
	}

	// audit-pack-report.json must itself be valid JSON carrying both
	// requirement ids, so a consumer can read the pack without cross-
	// referencing manifest.json's omitted array by hand.
	var reparsed auditPackDocument
	if err := json.Unmarshal(bundle[auditPackReportPath], &reparsed); err != nil {
		t.Fatalf("audit-pack-report.json did not round-trip as JSON: %v", err)
	}
	if len(reparsed.Requirements) != 2 {
		t.Errorf("reparsed report has %d requirements, want 2", len(reparsed.Requirements))
	}
}

func TestRunAuditPackExport(t *testing.T) {
	dir := t.TempDir()
	generated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("release not found", func(t *testing.T) {
		out := filepath.Join(dir, "notfound.tgz")
		rc := runAuditPackExport(auditPackTestdataRoot, "v-does-not-exist", out, generated)
		if rc == 0 {
			t.Fatal("expected non-zero exit for a release with no docs/release-notes/<tag>.md")
		}
		if _, err := os.Stat(out); err == nil {
			t.Error("a not-found release must not write a pack")
		}
	})

	t.Run("invalid release tag shape", func(t *testing.T) {
		out := filepath.Join(dir, "bad.tgz")
		rc := runAuditPackExport(auditPackTestdataRoot, "../escape", out, generated)
		if rc != 2 {
			t.Fatalf("rc = %d, want 2 (usage error) for a path-escaping release value", rc)
		}
	})

	t.Run("happy path is reproducible byte-for-byte", func(t *testing.T) {
		outA := filepath.Join(dir, "a.tgz")
		outB := filepath.Join(dir, "b.tgz")
		if rc := runAuditPackExport(auditPackTestdataRoot, "v1.0.0-full", outA, generated); rc != 0 {
			t.Fatalf("first run rc = %d, want 0", rc)
		}
		if rc := runAuditPackExport(auditPackTestdataRoot, "v1.0.0-full", outB, generated); rc != 0 {
			t.Fatalf("second run rc = %d, want 0", rc)
		}
		bytesA, err := os.ReadFile(outA)
		if err != nil {
			t.Fatal(err)
		}
		bytesB, err := os.ReadFile(outB)
		if err != nil {
			t.Fatal(err)
		}
		if string(bytesA) != string(bytesB) {
			t.Error("two runs with the same -generated value must produce byte-identical tarballs")
		}

		// manifest.json preserves the existing sha256/omitted shape (task:
		// "Preserve manifest.json's existing shape ... do not fork it").
		manifestRaw := readTarFile(t, outA, "manifest.json")
		var manifest evidenceManifest
		if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
			t.Fatalf("manifest.json did not parse as evidenceManifest: %v", err)
		}
		if len(manifest.Files) == 0 {
			t.Error("manifest.Files is empty")
		}
		for _, f := range manifest.Files {
			if f.SHA256 == "" {
				t.Errorf("file %s has no sha256", f.Path)
			}
		}
		foundGapOmission := false
		for _, o := range manifest.Omitted {
			if strings.Contains(o.Path, "REQ-fixture-gap-01") {
				foundGapOmission = true
			}
		}
		if !foundGapOmission {
			t.Error("manifest.json's omitted array must record the could-not-check requirement")
		}
	})

	t.Run("legacy prose-only release is found with an empty pack, not an error", func(t *testing.T) {
		out := filepath.Join(dir, "legacy.tgz")
		rc := runAuditPackExport(auditPackTestdataRoot, "v0.9.0-legacy", out, generated)
		if rc != 0 {
			t.Fatalf("rc = %d, want 0 for a found release with empty declared scope", rc)
		}
		reportRaw := readTarFile(t, out, auditPackReportPath)
		var doc auditPackDocument
		if err := json.Unmarshal(reportRaw, &doc); err != nil {
			t.Fatalf("audit-pack-report.json did not parse: %v", err)
		}
		if len(doc.Requirements) != 0 {
			t.Errorf("expected 0 requirements for the legacy release's empty scope, got %d", len(doc.Requirements))
		}
	})
}

// readTarFile extracts one member's bytes from a gzip-compressed tar at path.
func readTarFile(t *testing.T, path, name string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("%s not found in %s", name, path)
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == name {
			raw, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
	}
}
