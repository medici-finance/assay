package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// subjectForge serves the ruling comment and decision issue the landing reads.
type subjectForge struct {
	deskkit.Forge
	comments []deskkit.Comment
	issue    *deskkit.Issue
}

func (f *subjectForge) ListCommentsTyped(deskkit.ForgeRepo, int, deskkit.TargetKind) ([]deskkit.Comment, error) {
	return f.comments, nil
}

func (f *subjectForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	return f.issue, nil
}

const (
	sgRepo    = "example-org/example-repo"
	sgTarget  = "docs/streams/sdlc/brief-18-example.md"
	sgRuling  = "https://github.com/example-org/example-repo/issues/41#issuecomment-9001"
	sgCreated = "2026-09-01T10:00:00Z"
)

func sgBrief(version, evidence string) []byte {
	fm := "---\nbrief: example:ex:sdlc:18\nid: 0d4c1f3e-0000-4000-8000-000000000018\nschema: brief-v2\ngate: human\n" +
		"decision-issue: 41\nruling: \"" + sgRuling + "\"\n"
	if version != "" {
		fm += "version: " + version + "\n"
	}
	return []byte(fm + "---\n\n# Brief 18\n\n## Evidence\n\n" + evidence + "\n## Review\n")
}

// milestone-support/09: the Evidence landing itself refuses a ruling
// that does not name this brief's exact subject, and Evidence that names another
// revision, before and after staging the write.
// regression: #7088
func TestEvidenceRulingSubjectGate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: example\nself: ex\nrepos:\n  ex: {cell: example, repo: " + sgRepo + "}\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := rulingHumansFn
	rulingHumansFn = func() map[string]string { return map[string]string{"example": "example-human"} }
	t.Cleanup(func() { rulingHumansFn = prev })

	forge := func(comment string) *subjectForge {
		return &subjectForge{
			comments: []deskkit.Comment{{DatabaseID: 9001, Author: deskkit.Account{Login: "example-human", Type: "User"},
				Body: comment, CreatedAt: sgCreated, UpdatedAt: sgCreated}},
			issue: &deskkit.Issue{Number: 41, Body: "Decision.\n\n<!-- decision-gate: example:ex:sdlc:18 -->\n"},
		}
	}
	ruled := "Option 1.\n\nRuling-Subject: example:ex:sdlc:b-18@1 card=none\n"
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
	remote := sgBrief("1", "")
	cases := []struct {
		name, comment, version, evidence string
		refused, unverifiable            bool
	}{
		{"bound ruling, legacy Evidence", ruled, "1", "| 1 | sdlc/18 | PASS |", false, false},
		{"bound ruling, Evidence names the same revision", ruled, "1", "| 1 | example:ex:sdlc:b-18@1 | PASS |", false, false},
		{"ruling on another revision", strings.Replace(ruled, "@1", "@2", 1), "1", "", true, false},
		{"ruling on another brief", strings.Replace(ruled, "b-18", "b-19", 1), "1", "", true, false},
		{"brief version moved past the ruling", ruled, "2", "", true, false},
		{"Evidence names another revision", ruled, "1", "| 1 | example:ex:sdlc:b-18@2 | PASS |", true, false},
		{"Evidence names no revision", ruled, "1", "| 1 | example:ex:sdlc:b-18 | PASS |", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := rulingSubjectGateAt(forge(tc.comment), fr, root, sgTarget, remote, sgBrief(tc.version, tc.evidence))
			switch {
			case tc.refused && !deskkit.IsRefused(err):
				t.Fatalf("got %v, want a refusal", err)
			case tc.unverifiable && !deskkit.IsUnverifiable(err):
				t.Fatalf("got %v, want could-not-check", err)
			case !tc.refused && !tc.unverifiable && err != nil:
				t.Fatalf("got %v, want no refusal", err)
			}
		})
	}
}
