package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// briefWithDomainAndVerify renders a brief-v1 fixture with a controllable
// Verify section and body, so the mismatch diagnostic can be exercised against
// both ordered-managed and probe-shaped complex briefs.
func briefWithDomainAndVerify(id, domainLine, verify, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("brief: " + id + "\n")
	b.WriteString("title: Cynefin fixture " + id + "\n")
	b.WriteString("wave: 0\n")
	b.WriteString("depends: []\n")
	b.WriteString("unblocks: []\n")
	b.WriteString("effort: S\n")
	b.WriteString("gate: model\n")
	if strings.TrimSpace(domainLine) != "" {
		b.WriteString(domainLine + "\n")
	}
	b.WriteString("risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n")
	b.WriteString("issues: []\n")
	b.WriteString("schema: brief-v1\n")
	b.WriteString("authored: 2026-08-22 by fixture\n")
	b.WriteString("sources: [\"fixture\"]\n")
	b.WriteString("---\n\n# " + id + "\n\n")
	if verify != "" {
		b.WriteString(verify + "\n")
	}
	b.WriteString(body + "\n")
	return b.String()
}

// writeMismatchFixture lays down a stream mixing the four shapes the
// diagnostic must tell apart:
//   01 complex, ONE verify row, no probe language   → mismatch
//   02 complex, one row, probe marker in body       → clean (it probes)
//   03 complex, TWO verify rows, no probe language  → clean (not single-answer)
//   04 complicated, one row, no probe language      → clean (ordered domain, ordered tools — not a mismatch)
func writeMismatchFixture(t *testing.T) (root string, streams []*Stream) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "mm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := `---
stream: mm
status: active
priority: P1
track: platform
---

# MM (mismatch fixtures)

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Mismatch](./brief-01-mismatch.md) | 0 | S | todo | — | — |
| 02 | [Probing](./brief-02-probing.md) | 0 | S | todo | — | — |
| 03 | [Multi-row](./brief-03-multirow.md) | 0 | S | todo | — | — |
| 04 | [Ordered](./brief-04-ordered.md) | 0 | S | todo | — | — |
`
	oneRow := `## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`make check`" + ` | exit 0 |
`
	twoRows := `## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`make check`" + ` | exit 0 |
| 2 | ` + "`make bench`" + ` | prints a table |
`
	files := map[string]string{
		"README.md":             readme,
		"brief-01-mismatch.md":  briefWithDomainAndVerify("mm/01", "domain: complex", oneRow, "Body with no exploration language.\n"),
		"brief-02-probing.md":   briefWithDomainAndVerify("mm/02", "domain: complex", oneRow, "We will run a probe and amplify what works.\n"),
		"brief-03-multirow.md":  briefWithDomainAndVerify("mm/03", "domain: complex", twoRows, "Body with no exploration language.\n"),
		"brief-04-ordered.md":   briefWithDomainAndVerify("mm/04", "domain: complicated", oneRow, "Body with no exploration language.\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	streams, _, err = loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

// TestCynefinMismatch covers the money diagnostic: a complex brief managed with
// ordered tools (single-answer Verify, no probe marker) is flagged; a probing
// complex brief, a multi-row complex brief, and an ordered brief are not — the
// flag keys on the management SHAPE, not on the complex tag alone.
func TestCynefinMismatch(t *testing.T) {
	_, streams := writeMismatchFixture(t)
	got := computeCynefinMismatch(streams)

	if len(got) != 1 || got[0].ID != "mm/01" {
		t.Fatalf("mismatch = %+v, want exactly [mm/01]", got)
	}
	if len(got[0].Signals) != 2 {
		t.Errorf("signals = %v, want the two ordered-management signals", got[0].Signals)
	}
}

// TestCynefinMismatchInReport covers the wiring: the mismatch list and the
// complex measures ride the --cynefin report (JSON shape), and the measures are
// three-state — probe-rate is computable, learning-velocity and surprise name
// their absent source as could-not-check, never a fabricated zero.
func TestCynefinMismatchInReport(t *testing.T) {
	_, streams := writeMismatchFixture(t)
	rep := computeCynefin(streams, nil, "weekly")

	if len(rep.Mismatch) != 1 || rep.Mismatch[0].ID != "mm/01" {
		t.Errorf("report mismatch = %+v, want [mm/01]", rep.Mismatch)
	}
	pr := rep.ComplexMeasures.ProbeRate
	if pr.State != cynefinClean || pr.Value == nil {
		t.Fatalf("probe-rate = %+v, want checked-clean with a value", pr)
	}
	// mm/02 is the one probing brief of three active complex briefs.
	if want := 1.0 / 3.0; *pr.Value < want-1e-9 || *pr.Value > want+1e-9 {
		t.Errorf("probe-rate value = %v, want %v", *pr.Value, want)
	}
	if rep.ComplexMeasures.LearningVelocity.State != cynefinUnknown {
		t.Errorf("learning-velocity state = %q, want could-not-check (source not wired)", rep.ComplexMeasures.LearningVelocity.State)
	}
	if rep.ComplexMeasures.LearningVelocity.Value != nil {
		t.Errorf("learning-velocity value = %v, want nil (a measure with no source is not zero)", *rep.ComplexMeasures.LearningVelocity.Value)
	}
	if rep.ComplexMeasures.Surprise.State != cynefinUnknown {
		t.Errorf("surprise state = %q, want could-not-check (source not wired)", rep.ComplexMeasures.Surprise.State)
	}
}

// TestCynefinMismatchNoComplex covers the three-state floor: with no active
// complex briefs, probe-rate reports could-not-check — absence of the cohort is
// never reported as a clean zero rate.
func TestCynefinMismatchNoComplex(t *testing.T) {
	_, streams := writeCynefinFixture(t) // has one complex brief? No: fixture am/01 IS complex — use a made-empty tree instead
	_ = streams
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "oo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := `---
stream: oo
status: active
priority: P1
track: platform
---

# OO

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Ordered](./brief-01-ordered.md) | 0 | S | todo | — | — |
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	oneRow := `## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`true`" + ` | exit 0 |
`
	if err := os.WriteFile(filepath.Join(dir, "brief-01-ordered.md"),
		[]byte(briefWithDomainAndVerify("oo/01", "domain: complicated", oneRow, "Body.\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	pr := computeCynefinComplexMeasures(streams2).ProbeRate
	if pr.State != cynefinUnknown || pr.Value != nil {
		t.Errorf("probe-rate with no complex cohort = %+v, want could-not-check with no value", pr)
	}
}
