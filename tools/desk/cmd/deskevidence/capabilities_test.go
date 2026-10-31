package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

// regression: #4947
func TestDeskCapsPreflight(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		want    int
	}{
		"unclosed":    {"---\nschema: brief-v1\n## Evidence\n| 1 | unresolved |\n", deskkit.ExitUnverifiable},
		"unsupported": {"---\n\"reader-\\x72equirements\": {schema: \"reader-\\x72equirements-v1\", required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n---\n## Evidence\n", deskkit.ExitRefused},
	} {
		t.Run(name, func(t *testing.T) {
			f := setupStamp(t, true)
			root := rootWithFile(t, stampBrief, tc.content)
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != tc.want {
				t.Fatalf("preflight exit = %d, want %d", code, tc.want)
			}
			if f.putCalls != 0 {
				t.Fatalf("preflight failure wrote %d time(s)", f.putCalls)
			}
		})
	}
}
