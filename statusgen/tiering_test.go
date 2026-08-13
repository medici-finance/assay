package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTiering covers the optional per-stream `tiering:` frontmatter field:
// free-text dispatcher guidance, never a gate, never a
// Next-up score input.

func strPtr(s string) *string { return &s }

func TestTiering(t *testing.T) {
	t.Run("frontmatter round-trip: present value parses", func(t *testing.T) {
		const readme = `---
stream: frontend
status: active
priority: P1
tiering: implement=sonnet verify=fable
---

# Frontend
`
		dir := t.TempDir()
		p := writeTemp(t, dir, "README.md", readme)
		s, err := parseStreamREADME(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Tiering == nil || *s.Tiering != "implement=sonnet verify=fable" {
			t.Errorf("tiering not parsed: %+v", s.Tiering)
		}
	})

	t.Run("frontmatter round-trip: absent field parses as nil", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "operator")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := writeTemp(t, dir, "README.md", sampleReadme)
		s, err := parseStreamREADME(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Tiering != nil {
			t.Errorf("expected nil Tiering for a README without the field, got %q", *s.Tiering)
		}
	})

	t.Run("renders in Notes when present", func(t *testing.T) {
		s := mkStream("frontend", "active", "P1")
		s.Track = "product"
		s.Tiering = strPtr("implement=sonnet verify=fable")
		out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
		if !strings.Contains(out, "implement=sonnet verify=fable") {
			t.Errorf("tiering text missing from roll-up Notes:\n%s", out)
		}
	})

	t.Run("renders nothing when absent", func(t *testing.T) {
		s := mkStream("frontend", "active", "P1")
		s.Track = "product"
		out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
		// Assert on a would-be value token too, not just the field name — a
		// real free-text value never contains the literal word "tiering".
		if strings.Contains(out, "tiering") || strings.Contains(out, "implement=") {
			t.Errorf("no tiering text expected with the field unset:\n%s", out)
		}
	})

	t.Run("whitespace-only value is a PROBLEM and renders nothing", func(t *testing.T) {
		s := mkStream("frontend", "active", "P1")
		s.Track = "product"
		s.Tiering = strPtr("   ")
		problems, _ := check([]*Stream{s}, nil)
		found := false
		for _, p := range problems {
			if strings.Contains(p, "tiering present but empty") {
				found = true
			}
		}
		if !found {
			t.Errorf("whitespace-only tiering must be flagged like empty, got %v", problems)
		}
		s.External = "https://github.com/x/y"
		out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
		if strings.Contains(out, "https://github.com/x/y ·") {
			t.Errorf("whitespace-only tiering must not render a · joiner after the external pointer:\n%s", out)
		}
	})

	t.Run("· separated from external pointer when both present", func(t *testing.T) {
		s := mkStream("frontend", "active", "P1")
		s.Track = "product"
		s.External = "https://github.com/x/y"
		s.Tiering = strPtr("implement=sonnet")
		out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
		if !strings.Contains(out, "→ https://github.com/x/y · implement=sonnet") {
			t.Errorf("expected external + tiering joined with · , got:\n%s", out)
		}
	})

	t.Run("empty string value is a PROBLEM", func(t *testing.T) {
		s := mkStream("frontend", "active", "P1")
		s.Tiering = strPtr("")
		problems, _ := check([]*Stream{s}, nil)
		found := false
		for _, p := range problems {
			if strings.Contains(p, "tiering") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a problem for an empty tiering value, got %v", problems)
		}
	})

	t.Run("empty string parses without error (validation is check()'s job, not parse's)", func(t *testing.T) {
		const readme = `---
stream: frontend
status: active
priority: P1
tiering: ""
---

# Frontend
`
		dir := t.TempDir()
		p := writeTemp(t, dir, "README.md", readme)
		s, err := parseStreamREADME(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Tiering == nil || *s.Tiering != "" {
			t.Errorf("expected a non-nil pointer to empty string, got %+v", s.Tiering)
		}
	})
}
