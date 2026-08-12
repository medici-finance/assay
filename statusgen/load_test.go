package main

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleFindingsLegacy = `# Findings

## F-01 — 2026-07-08 — Brief-11 blocker text stale

refactor-auth Brief 16 landed; blocker text was outdated.

Affects: deploy-hardening/brief-01
Resolved: yes

## F-02 — 2026-07-08 — Something open

Body.

Affects: operator, frontend/brief-02
Ack: 2026-07-09 desk
Resolved: no
`

func TestParseFindingsLegacyFile(t *testing.T) {
	p := writeTemp(t, t.TempDir(), "FINDINGS.md", sampleFindingsLegacy)
	fs, err := parseFindings(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2", len(fs))
	}
	if fs[0].ID != "F-01" || !fs[0].Resolved || len(fs[0].Affects) != 1 {
		t.Errorf("F-01 wrong: %+v", fs[0])
	}
	if fs[0].Ack != "" {
		t.Errorf("F-01 has no Ack line, got %q", fs[0].Ack)
	}
	if fs[1].Resolved || len(fs[1].Affects) != 2 || fs[1].Affects[1] != "frontend/brief-02" {
		t.Errorf("F-02 wrong: %+v", fs[1])
	}
	if fs[1].Ack != "2026-07-09 desk" {
		t.Errorf("F-02 Ack wrong: %q", fs[1].Ack)
	}
}

func TestParseFindingsMissingFile(t *testing.T) {
	fs, err := parseFindings(filepath.Join(t.TempDir(), "nope.md"))
	if err != nil || fs != nil {
		t.Errorf("missing file should be nil, nil; got %v, %v", fs, err)
	}
}

func TestLoadStreams(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", "operator")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, sdir, "README.md", sampleReadme)
	streams, findings, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Name != "operator" {
		t.Errorf("got %d streams, findings %v", len(streams), findings)
	}
	// findings may be nil (no findings/ dir) or empty
}

func TestParseFindingsLegacyResolvedVariants(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		resolved bool
	}{
		{name: "yes", line: "Resolved: yes", resolved: true},
		{name: "true", line: "Resolved: true", resolved: true},
		{name: "no", line: "Resolved: no", resolved: false},
		{name: "false", line: "Resolved: false", resolved: false},
		{name: "yes with prose", line: "Resolved: yes -- fixed in PR #123", resolved: true},
		{name: "true with prose", line: "Resolved: true -- resolved by desk", resolved: true},
		{name: "no with prose", line: "Resolved: no -- waiting on upstream", resolved: false},
		{name: "yes with extra spaces", line: "Resolved:   yes  ", resolved: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "# Findings\n\n## F-01 — 2026-07-08 — Test\n\nBody.\n\n" + tc.line + "\n"
			path := filepath.Join(t.TempDir(), "FINDINGS.md")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			fs, err := parseFindings(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(fs) != 1 {
				t.Fatalf("got %d findings, want 1", len(fs))
			}
			if fs[0].Resolved != tc.resolved {
				t.Errorf("Resolved = %v, want %v for line %q", fs[0].Resolved, tc.resolved, tc.line)
			}
		})
	}
}
