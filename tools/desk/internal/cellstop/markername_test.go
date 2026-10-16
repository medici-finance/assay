package cellstop

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2971 — deskkit restates the marker names (cellstop imports
// deskkit, so it cannot import these); the cell-scoped stop flags read them, so a rename
// here that deskkit missed would silently leave every cell flag unread.
func TestCellMarkerNamesMatchDeskkit(t *testing.T) {
	if EnvCell != deskkit.CellMarkerEnv {
		t.Fatalf("cellstop.EnvCell = %q, deskkit.CellMarkerEnv = %q", EnvCell, deskkit.CellMarkerEnv)
	}
	if EnvCellRole != deskkit.CellRoleMarkerEnv {
		t.Fatalf("cellstop.EnvCellRole = %q, deskkit.CellRoleMarkerEnv = %q", EnvCellRole, deskkit.CellRoleMarkerEnv)
	}
}
