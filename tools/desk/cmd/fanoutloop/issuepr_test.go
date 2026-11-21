package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6908
func TestPlanNamesIssuePR(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "issue-loop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{7} {
		raw := fmt.Sprintf("---\nschema: placeholder-v1\nbrief: issue-loop/issue-%d\nrepo: example-org/example\nissue: %d\n---\n", n, n)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("issue-%d.md", n)), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := &FanoutLoop{Root: root, issueBoardRepo: "example-org/example", DryRun: true, Rework: func() ([]BoardRow, error) { return nil, nil }, issueIndex: &deskkit.IssuePRIndex{List: func(string) ([]deskkit.PRRef, bool, error) {
		return []deskkit.PRRef{{Number: 19, State: "OPEN", Body: "Issue: #7"}}, false, nil
	}},
		Board: func() ([]BoardRow, error) {
			return []BoardRow{{Stream: "issue-loop", Num: "issue-7", Status: "todo"}}, nil
		}}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "0 item(s) to dispatch") || !strings.Contains(got, "open PR example-org/example#19") {
		t.Fatal(got)
	}
}
