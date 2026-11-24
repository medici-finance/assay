package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6908
func TestBoardIssuePRAdmission(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "issue-loop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{7, 8} {
		raw := fmt.Sprintf("---\nschema: placeholder-v1\nbrief: issue-loop/issue-%d\nrepo: example-org/example\nissue: %d\n---\n", n, n)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("issue-%d.md", n)), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	index := deskkit.IssuePRIndex{List: func(string) ([]deskkit.PRRef, bool, error) {
		return []deskkit.PRRef{{Number: 19, State: "MERGED", Body: "Issue: #7"}}, true, nil
	}, Lookup: func(string, string) ([]deskkit.PRRef, error) { return nil, errors.New("unavailable") }}
	report := dispatchReport{}
	for _, id := range []string{"issue-loop/issue-7", "issue-loop/issue-8", "example/10"} {
		report.Rows = append(report.Rows, dispatchRow{Repo: "example-org/example", Root: root, Brief: id})
	}
	filterIssueRows(&report, &index)
	if len(report.Rows) != 1 || report.Rows[0].Brief != "example/10" || report.HeldByIssuePR != 1 || report.IssuePRUnknown != 1 ||
		len(report.AdmissionHeld) != 2 || !strings.Contains(report.AdmissionHeld[0].Reason, "merged PR example-org/example#19") || !strings.Contains(report.AdmissionHeld[1].Reason, "unavailable") {
		t.Fatalf("%+v", report)
	}
}
