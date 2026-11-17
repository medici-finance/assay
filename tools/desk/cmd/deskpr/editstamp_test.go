package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6695
// The report blamed a body-only `deskpr edit` for rewriting the authored model stamp. The
// verb writes no model-stamp label: a body edit on a stamped PR lands with ZERO label writes,
// so the stamp it leaves is byte-identical to the one it found.
func TestEditLeavesModelStamp(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
	t.Setenv("FAKEGH_PR_LABELS", "authored-model:claude-opus-5-5,authored-tier:strong")
	bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")

	if rc := run([]string{"edit", "--body-file", bodyPath}); rc != deskkit.ExitOK {
		t.Fatalf("edit rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := editCalls(*calls); len(got) != 1 {
		t.Fatalf("want exactly 1 `gh pr edit` call, got %d: %v", len(got), got)
	}
	if len(curForge.labelChanges) != 0 {
		t.Fatalf("a body-only edit wrote labels: %+v", curForge.labelChanges)
	}
}
