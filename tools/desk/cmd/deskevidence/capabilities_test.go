package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"strings"
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
			before := len(auditEntries(t))
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != tc.want {
				t.Fatalf("preflight exit = %d, want %d", code, tc.want)
			}
			rows := auditEntries(t)
			if len(rows) != before+1 {
				t.Fatalf("preflight wrote %d audit rows, want exactly one", len(rows)-before)
			}
			row := rows[len(rows)-1]
			wantResult := deskkit.ResultRefused
			if tc.want == deskkit.ExitUnverifiable {
				wantResult = deskkit.ResultUnverifiable
			}
			if row.Tool != "deskevidence" || row.Verb != "commit" || row.Result != wantResult || !strings.Contains(row.Detail, "reader capabilities:") || !strings.Contains(row.Detail, stampBrief) {
				t.Fatalf("preflight refusal audit lost context: %+v", row)
			}
			if f.putCalls != 0 {
				t.Fatalf("preflight failure wrote %d time(s)", f.putCalls)
			}
		})
	}
}
