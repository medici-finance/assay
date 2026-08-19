package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// briefWithDomain renders a minimal, valid brief-v1 file. domainLine is the raw
// `domain:` frontmatter line (e.g. "domain: complex") or "" to omit the field.
func briefWithDomain(id, domainLine string) string {
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
	b.WriteString("authored: 2026-08-16 by fixture\n")
	b.WriteString("sources: [\"fixture\"]\n")
	b.WriteString("---\n\n# " + id + "\n\nBody.\n")
	return b.String()
}

// writeCynefinFixture lays down a one-stream tree exercising every domain bucket
// and a done (inactive) brief, and returns the loaded streams.
func writeCynefinFixture(t *testing.T) (root string, streams []*Stream) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "am")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := `---
stream: am
status: active
priority: P1
track: platform
---

# AM (cynefin fixtures)

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Complex](./brief-01-complex.md) | 0 | S | todo | — | — |
| 02 | [Complicated](./brief-02-comp.md) | 0 | S | in-progress | — | — |
| 03 | [Untagged](./brief-03-untagged.md) | 0 | S | todo | — | — |
| 04 | [Clear done](./brief-04-done.md) | 0 | S | done | 2026-08-16 v | 2026-08-16 model:x |
| 05 | [Bad domain](./brief-05-bad.md) | 0 | S | todo | — | — |
`
	files := map[string]string{
		"README.md":            readme,
		"brief-01-complex.md":  briefWithDomain("am/01", "domain: complex"),
		"brief-02-comp.md":     briefWithDomain("am/02", "domain: complicated"),
		"brief-03-untagged.md": briefWithDomain("am/03", ""),
		"brief-04-done.md":     briefWithDomain("am/04", "domain: clear"),
		"brief-05-bad.md":      briefWithDomain("am/05", "domain: turbo"),
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

// TestCynefinDomainParse covers the schema addition: an explicit `domain:` is
// parsed, and an ABSENT domain defaults to complicated at read time (never an
// error).
func TestCynefinDomainParse(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "brief-01-x.md")
	if err := os.WriteFile(explicit, []byte(briefWithDomain("am/01", "domain: complex")), 0o644); err != nil {
		t.Fatal(err)
	}
	bf, ok, err := parseBriefFile(explicit)
	if err != nil || !ok {
		t.Fatalf("parse explicit: ok=%v err=%v", ok, err)
	}
	if bf.Domain != "complex" {
		t.Errorf("Domain = %q, want complex", bf.Domain)
	}

	absent := filepath.Join(dir, "brief-02-y.md")
	if err := os.WriteFile(absent, []byte(briefWithDomain("am/02", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	bf2, ok, err := parseBriefFile(absent)
	if err != nil || !ok {
		t.Fatalf("parse absent: ok=%v err=%v", ok, err)
	}
	if bf2.Domain != "" {
		t.Errorf("absent Domain = %q, want empty", bf2.Domain)
	}
	if got := effectiveDomain(bf2.Domain); got != "complicated" {
		t.Errorf("effectiveDomain(absent) = %q, want complicated (the safe Ordered default)", got)
	}
}

// TestCynefinDomainLint covers the additive lint: a present-but-unrecognized
// domain is a hard PROBLEM naming the bad token; a valid domain and an ABSENT
// domain are both clean (the field is optional, never errors on absence).
func TestCynefinDomainLint(t *testing.T) {
	_, streams := writeCynefinFixture(t)
	problems, _ := checkBriefFiles(streams, streams)

	if !hasProblem(problems, "brief-05-bad.md", "invalid domain", "turbo") {
		t.Errorf("want invalid-domain problem naming 'turbo'; got:\n%s", strings.Join(problems, "\n"))
	}
	if hasProblem(problems, "brief-01-complex.md", "invalid domain") {
		t.Errorf("valid domain: complex must not be flagged; got:\n%s", strings.Join(problems, "\n"))
	}
	if hasProblem(problems, "brief-03-untagged.md", "invalid domain") {
		t.Errorf("absent domain must never be flagged (optional field); got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestCynefinReport covers the view: distribution over active work only (done
// excluded), untagged AND invalid-domain briefs land in the Disorder bucket and
// list, and the three-state verdict is checked-failed when Disorder is non-empty.
func TestCynefinReport(t *testing.T) {
	_, streams := writeCynefinFixture(t)
	rep := computeCynefin(streams, nil, "weekly")

	if rep.Total != 4 {
		t.Errorf("Total = %d, want 4 (01,02,03,05 active; 04 done excluded)", rep.Total)
	}
	if rep.Distribution["complex"] != 1 || rep.Distribution["complicated"] != 1 {
		t.Errorf("distribution = %v, want complex=1 complicated=1", rep.Distribution)
	}
	if rep.Distribution["clear"] != 0 {
		t.Errorf("clear = %d, want 0 (the only clear brief is done, excluded)", rep.Distribution["clear"])
	}
	if rep.Distribution[disorderDomain] != 2 {
		t.Errorf("disorder count = %d, want 2 (untagged + invalid)", rep.Distribution[disorderDomain])
	}
	if len(rep.Disorder) != 2 || rep.Disorder[0] != "am/03" || rep.Disorder[1] != "am/05" {
		t.Errorf("Disorder = %v, want [am/03 am/05]", rep.Disorder)
	}
	if rep.State != cynefinFailed {
		t.Errorf("State = %q, want %q (Disorder non-empty)", rep.State, cynefinFailed)
	}
}

// TestCynefinReportCouldNotCheck covers the three-state floor: no active
// brief-v1 work → could-not-check (absence of evidence is NOT reported clean).
func TestCynefinReportCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "am")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := `---
stream: am
status: active
priority: P1
track: platform
---

# AM

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Done](./brief-01-done.md) | 0 | S | done | 2026-08-16 v | 2026-08-16 model:x |
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-done.md"), []byte(briefWithDomain("am/01", "domain: clear")), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	rep := computeCynefin(streams, nil, "weekly")
	if rep.State != cynefinUnknown {
		t.Errorf("State = %q, want %q (no active work)", rep.State, cynefinUnknown)
	}
	if rep.Total != 0 {
		t.Errorf("Total = %d, want 0", rep.Total)
	}
}

// TestCynefinDrift covers the historian join: transitions are bucketed by period
// and attributed to the transitioning brief's current domain.
func TestCynefinDrift(t *testing.T) {
	_, streams := writeCynefinFixture(t)
	history := []HistoryEntry{
		{Ts: "2026-08-11T10:00:00Z", Brief: "am/01", From: "todo", To: "in-progress"},
		{Ts: "2026-08-12T10:00:00Z", Brief: "am/02", From: "todo", To: "in-progress"},
	}
	rep := computeCynefin(streams, history, "weekly")
	if len(rep.Drift) == 0 {
		t.Fatalf("Drift empty, want at least one period")
	}
	var complexN, complicatedN, total int
	for _, bk := range rep.Drift {
		complexN += bk.Distribution["complex"]
		complicatedN += bk.Distribution["complicated"]
		total += bk.Transitions
	}
	if complexN != 1 || complicatedN != 1 || total != 2 {
		t.Errorf("drift totals complex=%d complicated=%d total=%d, want 1/1/2", complexN, complicatedN, total)
	}
}
