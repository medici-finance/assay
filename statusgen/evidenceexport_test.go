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

func TestCollectBundledFiles(t *testing.T) {
	root := "testdata/evidencebundle"

	// Range 2026-07-14 to 2026-07-20 includes the in-range brief (07-15),
	// in-range intake (07-16), and in-range finding (07-17).
	// It excludes the out-of-range brief (07-25), out-of-range intake (07-22),
	// and out-of-range finding (07-23).
	paths, bundle, omitted, err := collectBundledFiles(root, "2026-07-14", "2026-07-20")
	if err != nil {
		t.Fatalf("collectBundledFiles: %v", err)
	}
	if len(omitted) != 0 {
		t.Fatalf("fixture tree is complete, expected no omissions, got %+v", omitted)
	}

	if len(paths) < 3 {
		t.Fatalf("expected at least 3 bundled files, got %d: %v", len(paths), paths)
	}

	// Verify brief files: only the in-range one.
	hasBrief01 := false
	hasBrief02 := false
	for _, p := range paths {
		if strings.Contains(p, "brief-01-in-range") {
			hasBrief01 = true
		}
		if strings.Contains(p, "brief-02-out-of-range") {
			hasBrief02 = true
		}
	}
	if !hasBrief01 {
		t.Error("expected brief-01-in-range.md to be in the bundle")
	}
	if hasBrief02 {
		t.Error("brief-02-out-of-range.md should NOT be in the bundle (authored 2026-07-25)")
	}

	// Verify intake entries: only the in-range one.
	hasInRangeIntake := false
	hasOutOfRangeIntake := false
	for _, p := range paths {
		if strings.Contains(p, "in-range-intake") {
			hasInRangeIntake = true
		}
		if strings.Contains(p, "out-of-range-intake") {
			hasOutOfRangeIntake = true
		}
	}
	if !hasInRangeIntake {
		t.Error("expected in-range intake entry to be in the bundle")
	}
	if hasOutOfRangeIntake {
		t.Error("out-of-range intake entry should NOT be in the bundle (date 2026-07-22)")
	}

	// Verify findings entries: only the in-range one.
	hasInRangeFinding := false
	hasOutOfRangeFinding := false
	for _, p := range paths {
		if strings.Contains(p, "in-range-finding") {
			hasInRangeFinding = true
		}
		if strings.Contains(p, "out-of-range-finding") {
			hasOutOfRangeFinding = true
		}
	}
	if !hasInRangeFinding {
		t.Error("expected in-range finding entry to be in the bundle")
	}
	if hasOutOfRangeFinding {
		t.Error("out-of-range finding entry should NOT be in the bundle (date 2026-07-23)")
	}

	// The stream README must travel with the briefs: the lifecycle Status column
	// and the Verified/Reviewed cells exist ONLY there, and the SOC2 mapping's
	// Testing / Segregation-of-duties / Approval rows point at them. Without this
	// the bundle silently omitted every one of those cells.
	hasStreamReadme := false
	for _, p := range paths {
		if p == "docs/streams/evidencestream/README.md" {
			hasStreamReadme = true
		}
	}
	if !hasStreamReadme {
		t.Errorf("expected the contributing stream's README.md in the bundle, got %v", paths)
	}

	// Verify bundle has content (not empty).
	for _, p := range paths {
		if len(bundle[p]) == 0 {
			t.Errorf("bundle[%q] is empty", p)
		}
	}

	// Verify sorted order.
	for i := 1; i < len(paths); i++ {
		if paths[i-1] > paths[i] {
			t.Errorf("paths not sorted: %q > %q", paths[i-1], paths[i])
		}
	}
}

func TestWriteEvidenceBundle(t *testing.T) {
	root := "testdata/evidencebundle"

	paths, bundle, omitted, err := collectBundledFiles(root, "2026-07-14", "2026-07-20")
	if err != nil {
		t.Fatalf("collectBundledFiles: %v", err)
	}
	if len(omitted) != 0 {
		t.Fatalf("fixture tree is complete, expected no omissions, got %+v", omitted)
	}

	outPath := filepath.Join(t.TempDir(), "bundle.tgz")
	generated := nowFunc()
	if err := writeEvidenceBundle(outPath, paths, bundle, omitted, generated); err != nil {
		t.Fatalf("writeEvidenceBundle: %v", err)
	}

	// Open and verify the tarball.
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	foundManifest := false
	seenPaths := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seenPaths[hdr.Name] = true
		if hdr.Name == "manifest.json" {
			foundManifest = true
			var m evidenceManifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				t.Fatalf("decoding manifest.json: %v", err)
			}
			if m.Version != statusgenVersion {
				t.Errorf("manifest version = %q, want %q", m.Version, statusgenVersion)
			}
			if m.Generated == "" {
				t.Error("manifest generated timestamp is empty")
			}
			if len(m.Files) != len(paths) {
				t.Errorf("manifest has %d files, want %d", len(m.Files), len(paths))
			}
			for _, fe := range m.Files {
				if fe.Path == "" {
					t.Error("manifest entry has empty path")
				}
				if fe.SHA256 == "" {
					t.Errorf("manifest entry %q has empty sha256", fe.Path)
				}
				if len(fe.SHA256) != 64 {
					t.Errorf("manifest entry %q sha256 length = %d, want 64", fe.Path, len(fe.SHA256))
				}
			}
		}
	}

	if !foundManifest {
		t.Error("manifest.json not found in tarball")
	}

	// Every expected path should be present.
	for _, p := range paths {
		if !seenPaths[p] {
			t.Errorf("expected path %q not found in tarball", p)
		}
	}
}

func TestRunEvidenceExport(t *testing.T) {
	root := "testdata/evidencebundle"
	outPath := filepath.Join(t.TempDir(), "bundle.tgz")

	code := runEvidenceExport(root, "2026-07-14", "2026-07-20", outPath, time.Time{})
	if code != 0 {
		t.Fatalf("runEvidenceExport exited %d", code)
	}

	// Verify the file exists and has content.
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}

	// Verify manifest.json is in the tarball.
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	foundManifest := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "manifest.json" {
			foundManifest = true
			break
		}
	}
	if !foundManifest {
		t.Error("manifest.json not found in exported tarball")
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input  string
		wantOk bool
	}{
		{"2026-07-01", true},
		{"2026-01-01", true},
		{"2026-12-31", true},
		{"2026-7-1", false},
		{"not-a-date", false},
		{"", false},
		{"20260701", false},
	}
	for _, tt := range tests {
		_, err := parseDate(tt.input)
		ok := err == nil
		if ok != tt.wantOk {
			t.Errorf("parseDate(%q) ok=%v, want %v (err=%v)", tt.input, ok, tt.wantOk, err)
		}
	}
}

func TestInDateRange(t *testing.T) {
	from, _ := parseDate("2026-07-14")
	to, _ := parseDate("2026-07-20")

	tests := []struct {
		date string
		want bool
	}{
		{"2026-07-13", false},
		{"2026-07-14", true},
		{"2026-07-17", true},
		{"2026-07-20", true},
		{"2026-07-21", false},
	}
	for _, tt := range tests {
		d, err := parseDate(tt.date)
		if err != nil {
			t.Fatal(err)
		}
		if got := inDateRange(d, from, to); got != tt.want {
			t.Errorf("inDateRange(%s, 2026-07-14, 2026-07-20) = %v, want %v", tt.date, got, tt.want)
		}
	}
}

// TestDeterministicOutput verifies the ACTUAL determinism guarantee: two exports
// with the same inputs AND the same supplied generation timestamp are
// byte-identical.
//
// The clock is stubbed and advanced a full second between the two runs, so this
// test can genuinely fail. The previous version called the real clock and passed
// only because both exports landed inside the same wall-clock second — it would
// have gone green against an implementation that stamped the generation time into
// every tar header, which is exactly what the implementation used to do.
func TestDeterministicOutput(t *testing.T) {
	root := "testdata/evidencebundle"
	dir := t.TempDir()
	out1 := filepath.Join(dir, "bundle1.tgz")
	out2 := filepath.Join(dir, "bundle2.tgz")

	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	saved := nowFunc
	defer func() { nowFunc = saved }()

	// Advance the clock a second between runs — if any wall-clock value leaked
	// into the bundle bytes, these two would differ.
	nowFunc = func() time.Time { return base }
	if code := runEvidenceExport(root, "2026-07-14", "2026-07-20", out1, base); code != 0 {
		t.Fatalf("first export exited %d", code)
	}
	nowFunc = func() time.Time { return base.Add(time.Second) }
	if code := runEvidenceExport(root, "2026-07-14", "2026-07-20", out2, base); code != 0 {
		t.Fatalf("second export exited %d", code)
	}

	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Error("two exports with identical inputs and an explicit -generated produced different bundles — must be byte-identical")
	}
}

// TestUnsuppliedTimestampVariesOnlyInManifest pins the honest, narrower claim the
// doc now makes: without an explicit -generated the bundle is NOT byte-identical
// across runs, but the difference is confined to manifest.generated — the file set
// and every content hash still match.
func TestUnsuppliedTimestampVariesOnlyInManifest(t *testing.T) {
	root := "testdata/evidencebundle"
	dir := t.TempDir()
	out1 := filepath.Join(dir, "bundle1.tgz")
	out2 := filepath.Join(dir, "bundle2.tgz")

	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	saved := nowFunc
	defer func() { nowFunc = saved }()

	nowFunc = func() time.Time { return base }
	if code := runEvidenceExport(root, "2026-07-14", "2026-07-20", out1, time.Time{}); code != 0 {
		t.Fatalf("first export exited %d", code)
	}
	nowFunc = func() time.Time { return base.Add(90 * time.Second) }
	if code := runEvidenceExport(root, "2026-07-14", "2026-07-20", out2, time.Time{}); code != 0 {
		t.Fatalf("second export exited %d", code)
	}

	m1 := readManifest(t, out1)
	m2 := readManifest(t, out2)

	if m1.Generated == m2.Generated {
		t.Fatalf("clock stub did not take effect: both manifests say %q", m1.Generated)
	}
	if len(m1.Files) != len(m2.Files) || len(m1.Files) == 0 {
		t.Fatalf("file sets differ or are empty: %d vs %d", len(m1.Files), len(m2.Files))
	}
	for i := range m1.Files {
		if m1.Files[i] != m2.Files[i] {
			t.Errorf("file entry %d differs across runs: %+v vs %+v", i, m1.Files[i], m2.Files[i])
		}
	}

	// The load-bearing assertion: compare the raw tar ENTRIES, headers included.
	// The manifest hash list would not notice a wall-clock value leaking into a
	// tar ModTime, which is precisely the bug this guards — every non-manifest
	// entry must be identical, modtime and all, across a 90-second clock gap.
	e1 := readTarEntries(t, out1)
	e2 := readTarEntries(t, out2)
	if len(e1) != len(e2) {
		t.Fatalf("entry counts differ: %d vs %d", len(e1), len(e2))
	}
	for name, a := range e1 {
		if name == "manifest.json" {
			continue // the one entry that is allowed to differ
		}
		b, ok := e2[name]
		if !ok {
			t.Errorf("%s present in first bundle, absent from second", name)
			continue
		}
		if !a.modTime.Equal(b.modTime) {
			t.Errorf("%s: modtime leaked the wall clock into the tar header (%s vs %s) — headers must be fixed", name, a.modTime, b.modTime)
		}
		if a.content != b.content {
			t.Errorf("%s: content differs across runs", name)
		}
	}
}

type tarEntry struct {
	modTime time.Time
	content string
}

// readTarEntries reads every member of a bundle, header and body.
func readTarEntries(t *testing.T, path string) map[string]tarEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	out := map[string]tarEntry{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out[hdr.Name] = tarEntry{modTime: hdr.ModTime, content: string(body)}
	}
	return out
}

// TestMissingRegisterDirIsReportedNotSwallowed covers the fail-open bug: a repo
// with no intake/findings directories used to export exit 0 and a well-formed
// manifest that gave a consumer no way to tell the registers were absent. The
// real assay repo has no docs/streams/intake, so this is the live case,
// not a hypothetical one.
func TestMissingRegisterDirIsReportedNotSwallowed(t *testing.T) {
	root := t.TempDir()
	// A stream with one in-range brief, but no register directories at all.
	streamDir := filepath.Join(root, "docs", "streams", "lonely")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "---\nschema: brief-v1\nbrief: lonely/01\ntitle: Lonely\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk:\n  regulatory: no\n  customer: no\n  irreversible: no\n  sensitive-data: no\nissues: []\nauthored: 2026-07-15 by test\nsources: []\n---\n\n## Task\n\nnothing\n"
	if err := os.WriteFile(filepath.Join(streamDir, "brief-01-lonely.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "README.md"), []byte(`---
stream: lonely
status: active
priority: P1
track: product
serves: assay
---
# Lonely

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [B](./brief-01-lonely.md) | 0 | S | todo | — | — |
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, omitted, err := collectBundledFiles(root, "2026-07-14", "2026-07-20")
	if err != nil {
		t.Fatalf("collectBundledFiles: %v", err)
	}

	var sawIntake, sawFindings bool
	for _, o := range omitted {
		if strings.Contains(o.Path, "intake") {
			sawIntake = true
		}
		if strings.Contains(o.Path, "findings") {
			sawFindings = true
		}
	}
	if !sawIntake || !sawFindings {
		t.Errorf("missing register dirs must be recorded in omitted, got %+v", omitted)
	}

	// And the export must signal incompleteness rather than exiting clean.
	out := filepath.Join(t.TempDir(), "bundle.tgz")
	if code := runEvidenceExport(root, "2026-07-14", "2026-07-20", out, time.Time{}); code != 3 {
		t.Errorf("an incomplete bundle must exit 3, got %d", code)
	}

	// The omissions must be readable from the bundle itself, not just stderr.
	m := readManifest(t, out)
	if len(m.Omitted) == 0 {
		t.Error("manifest.omitted must record what the bundle is missing")
	}
}

// TestUnparseableEntryIsReported covers the other half of the fail-open bug: a
// corrupt register entry used to be dropped with no counter, no warning, and no
// effect on the exit code.
func TestUnparseableEntryIsReported(t *testing.T) {
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "README.md"), []byte(`---
stream: s
status: active
priority: P1
track: product
serves: assay
---
# S

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [B](./brief-01-x.md) | 0 | S | todo | — | — |
`), 0o644); err != nil {
		t.Fatal(err)
	}
	intakeDir := filepath.Join(root, "docs", "streams", "intake")
	findingsDir := filepath.Join(root, "docs", "streams", "findings")
	for _, d := range []string{intakeDir, findingsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A well-formed entry and one with a corrupted date.
	good := "---\nid: I-01\ndate: 2026-07-16\ntitle: Good\ndisposition: open\n---\n\nbody\n"
	bad := "---\nid: I-02\ndate: not-a-date\ntitle: Bad\ndisposition: open\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(intakeDir, "2026-07-16-good.md"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intakeDir, "2026-07-16-bad.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, omitted, err := collectBundledFiles(root, "2026-07-14", "2026-07-20")
	if err != nil {
		t.Fatalf("collectBundledFiles: %v", err)
	}
	found := false
	for _, o := range omitted {
		if strings.Contains(o.Path, "bad.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("an unparseable register entry must be reported, got %+v", omitted)
	}
}

// TestSplitLayoutIntakeEntriesBundled guards the finding from the issue-loop/15
// review: the collector used to register docs/streams/intake and read it with
// a single root-level os.ReadDir, silently skipping every entry under a
// split-layout subdir (new/, decision-needed/, watching/, completed/,
// rejected/) — and not even recording them in `omitted`, since it never saw
// the subdir files at all. An in-range entry that exists ONLY under a subdir
// must now be bundled just like a root-level one.
func TestSplitLayoutIntakeEntriesBundled(t *testing.T) {
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "README.md"), []byte(`---
stream: s
status: active
priority: P1
track: product
serves: assay
---
# S

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [B](./brief-01-x.md) | 0 | S | todo | — | — |
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two subdir-only entries (no root-level intake files at all): one
	// in-range, one out-of-range, exactly mirroring the root-level
	// in-range/out-of-range pair the flat-layout fixture already covers.
	newDir := filepath.Join(root, "docs", "streams", "intake", "new")
	completedDir := filepath.Join(root, "docs", "streams", "intake", "completed")
	for _, d := range []string{newDir, completedDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inRange := "---\nid: I-split-in\ndate: 2026-07-16\ntitle: Split in-range\ndisposition: new\n---\n\nbody\n"
	outOfRange := "---\nid: I-split-out\ndate: 2026-07-22\ntitle: Split out-of-range\ndisposition: scoped\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(newDir, "2026-07-16-in.md"), []byte(inRange), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(completedDir, "2026-07-22-out.md"), []byte(outOfRange), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, bundle, omitted, err := collectBundledFiles(root, "2026-07-14", "2026-07-20")
	if err != nil {
		t.Fatalf("collectBundledFiles: %v", err)
	}
	for _, o := range omitted {
		if strings.Contains(o.Path, "intake") {
			t.Errorf("split-layout intake dir must not be reported as missing/omitted, got %+v", omitted)
		}
	}

	var sawInRange, sawOutOfRange bool
	for p := range bundle {
		if strings.Contains(p, "2026-07-16-in.md") {
			sawInRange = true
		}
		if strings.Contains(p, "2026-07-22-out.md") {
			sawOutOfRange = true
		}
	}
	if !sawInRange {
		t.Errorf("the in-range split-layout intake entry must be bundled, got paths %v", paths)
	}
	if sawOutOfRange {
		t.Error("the out-of-range split-layout intake entry must NOT be bundled")
	}
}

// readManifest extracts and decodes manifest.json from a bundle.
func readManifest(t *testing.T, path string) evidenceManifest {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name != "manifest.json" {
			continue
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		var m evidenceManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("manifest.json is not valid JSON: %v", err)
		}
		return m
	}
	t.Fatalf("manifest.json not found in %s", path)
	return evidenceManifest{}
}
