package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These checks guard instruction drift, not outgoing bodies: an unrecognised
// private locator still needs the pre-submit audience check required by R7.
func TestR7BoundaryClause(t *testing.T) {
	s, err := parseSource(realSource(t))
	if err != nil {
		t.Fatal(err)
	}
	r7 := s.Rules[6].Body
	for _, term := range []string{
		"file:line + mechanism", "tokens/keys/PII", "private or org-internal",
		"forges, trackers and CI", "full URLs", "clone URLs", "host+path",
		"trust boundary", "role+number", "no hostname, path or query",
		"same-venue same-visibility", "original trust boundary",
	} {
		if !strings.Contains(strings.ToLower(r7), strings.ToLower(term)) {
			t.Errorf("R7 missing boundary requirement %q", term)
		}
	}
}

var redactWord = regexp.MustCompile(`(?i)\bredact(?:ion|ed|s|ing)?\b`)

// A break is a blank line or the start of any CommonMark list item: a bullet
// (-, * or +) or an ordered marker (1-9 digits then . or )), each followed by
// a space or tab, at any indentation.
var redactionBreak = regexp.MustCompile(`\n[ \t]*\n|\n[ \t]*(?:[-*+]|[0-9]{1,9}[.)])[ \t]`)

// Enumerate the instruction corpus rather than pinning the two reported skill
// locations. Every paragraph expanding redaction of secrets/defects must defer
// to R7, and cannot retain the old exclusive permission. Blank lines and list
// markers delimit paragraphs/list items so an unrelated R7 pointer elsewhere
// cannot launder an expansion.
func redactionRefs(root string) ([]string, error) {
	var findings []string
	for _, sub := range []string{
		"plugins/assay/skills", "plugins/assay/references",
		"tools/desk/cmd/deskdispatch/references", ".claude/guardrails",
	} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(sub)), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			content := string(raw)
			start := 0
			breaks := redactionBreak.FindAllStringIndex(content, -1)
			breaks = append(breaks, []int{len(content), len(content)})
			for _, boundary := range breaks {
				p := content[start:boundary[0]]
				line := strings.Count(content[:start], "\n") + 1
				text := strings.Join(strings.Fields(p), " ")
				lower := strings.ToLower(text)
				if redactWord.MatchString(text) && (strings.Contains(lower, "secret") || strings.Contains(lower, "defect")) {
					if !strings.Contains(text, "R7") || strings.Contains(lower, "redact only") || strings.Contains(lower, "public record only") {
						findings = append(findings, fmt.Sprintf("%s:%d: redaction expansion must defer to R7 without exclusive secrets/public-only permission", filepath.ToSlash(rel), line))
					}
				}
				start = boundary[1]
			}
			return nil
		})
		if err != nil {
			return nil, err // unreadable corpus never clears the guard
		}
	}
	return findings, nil
}

func TestRedactionReferences(t *testing.T) {
	findings, err := redactionRefs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		t.Error(finding)
	}
}

func TestRedactionPositiveControl(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"plugins/assay/skills", "plugins/assay/references", "tools/desk/cmd/deskdispatch/references", ".claude/guardrails"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(sub)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plant := filepath.Join(root, "plugins/assay/skills/second-site.md")
	for _, tc := range []struct {
		text string
		bad  bool
		line int
	}{
		{"Redact only secrets; full defect detail goes on every issue.\n", true, 1},
		{"Redact only secrets; full defect detail goes on every issue (R7).\n", true, 1},
		{"Apply R7 when redacting secret material and reporting defect detail.\n", false, 0},
		{"Redact secret material.\n\nUnrelated paragraph (R7).\n", true, 1},
		{"- Apply R7.\n- Redact secret material.\n", true, 2},
		// Every Markdown list-item marker is a boundary: an unrelated R7
		// mention in the preceding item must not satisfy the next one.
		{"* Apply R7.\n* Redact secret material.\n", true, 2},
		{"+ Apply R7.\n+ Redact secret material.\n", true, 2},
		{"1. Apply R7.\n2. Redact secret material.\n", true, 2},
		{"1) Apply R7.\n2) Redact secret material.\n", true, 2},
		{"-\tApply R7.\n-\tRedact secret material.\n", true, 2},
		{"- Intro.\n  * Apply R7.\n  * Redact secret material.\n", true, 3},
		{"- Intro.\n   10. Apply R7.\n   11. Redact secret material.\n", true, 3},
		// Negative controls: an item carrying its own R7 reference stays
		// clean, including across a wrapped continuation line, and emphasis
		// at line start is not a list marker.
		{"* Redact secret material per R7.\n* Unrelated item.\n", false, 0},
		{"1. Redact secret material,\n   applying R7.\n2. Unrelated item.\n", false, 0},
		{"Redact secret material\n**per R7**.\n", false, 0},
	} {
		t.Run(tc.text, func(t *testing.T) {
			if err := os.WriteFile(plant, []byte(tc.text), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, err := redactionRefs(root)
			if err != nil {
				t.Fatal(err)
			}
			if (len(findings) > 0) != tc.bad {
				t.Fatalf("positive control %q: findings %v, want bad=%v", tc.text, findings, tc.bad)
			}
			if tc.bad && !strings.Contains(findings[0], fmt.Sprintf("second-site.md:%d:", tc.line)) {
				t.Fatalf("guard did not name the planted second site: %v", findings)
			}
		})
	}
}
