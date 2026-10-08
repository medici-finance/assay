package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The method is a prose consumer of the same closed vocabulary as deskdisposition.
// Pin its boundary without adding a second eligibility engine to interpret prose.
func TestResumeTriageContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for path, required := range map[string][]string{
		"plugins/assay/skills/worker-desk/references/dispatch-runbook.md": {
			"Advisory comments do not establish disposition", "no disposition write",
			"base/conflict defect", "question", "help wanted", "needs-decision",
			"could-not-check", "live dispatch claim", "model-stamp OK",
		},
		"plugins/assay/skills/worker-desk/SKILL.md": {
			"Advisory comments do not establish disposition", "base/conflict defect",
			"no disposition write", "dispatch-eligible",
		},
	} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(raw)), " ")
		for _, term := range required {
			if !strings.Contains(text, term) {
				t.Errorf("%s: missing triage boundary %q", path, term)
			}
		}
	}
	// Discover every worker-method markdown file, including future references.
	err := filepath.WalkDir(filepath.Join(root, "plugins/assay/skills/worker-desk"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if badTriageText(string(raw)) {
			t.Errorf("%s: generic marker/live-work disposition rule", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A second site must also trip the guard, not only the original guard paragraph.
	for _, plant := range []string{
		"An advisory build-status marker makes the PR ALREADY-TRIAGED.",
		"`NEEDS-REBASE` when it is\nstill live work.",
	} {
		if !badTriageText(plant) {
			t.Fatalf("planted sibling escaped: %s", plant)
		}
	}
}

func badTriageText(text string) bool {
	text = strings.Join(strings.Fields(text), " ")
	return strings.Contains(text, "ALREADY-TRIAGED") || strings.Contains(text, "`NEEDS-REBASE` when it is still live work")
}
