package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Cell-scoped stop flags.
//
// Two flags live in the same state directory as STOP and STOP.<loop>:
//
//	STOP.cell.<cell>         stops every loop of one cell
//	STOP.cell.<cell>.<loop>  stops one loop of one cell (<loop> resolves through loopnames.go)
//
// A session belongs to a cell ONLY through the process-environment marker cellctl stamps on
// every child it launches (ASSAY_CELL, its role in ASSAY_CELL_ROLE). DESK_CELL is the comms
// lane's variable and is deliberately not read here. A session with no marker is untouched by
// every cell flag. Cell flags only ever ADD a stop: they are resolved after DISABLED and STOP,
// and they never clear or override STOP.<loop>.
const (
	// CellMarkerEnv / CellRoleMarkerEnv mirror cellstop.EnvCell / cellstop.EnvCellRole. They
	// are restated here because cellstop imports deskkit (the reverse import would be a
	// cycle); cellstop's TestCellMarkerNamesMatchDeskkit pins the pairs equal.
	CellMarkerEnv     = "ASSAY_CELL"
	CellRoleMarkerEnv = "ASSAY_CELL_ROLE"
	// cellStopPrefix is the state-dir file-name prefix of both cell-scoped flags.
	cellStopPrefix = "STOP.cell."
)

// cellNameRe is the cell-name rule a marker must satisfy before any flag path is built from
// it: lower-case letters, digits and '-', nothing else — so a marker can never carry a '/',
// a '.' or a '..' into a file name under the state directory.
var cellNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidCellName reports whether name satisfies the cell-name rule cell-scoped stop flags use.
func ValidCellName(name string) bool { return cellNameRe.MatchString(name) }

// CellStopFlagName is the state-dir file name that stops every loop of cell (loop == "") or
// one loop of it. It is the single place the name is built; callers must have validated cell
// with ValidCellName first.
func CellStopFlagName(cell, loop string) string {
	if loop == "" {
		return cellStopPrefix + cell
	}
	return cellStopPrefix + cell + "." + loop
}

// sessionCell returns the cell this process belongs to, per its ASSAY_CELL marker: "" when
// there is no marker (cell flags do not apply). A marker that fails the cell-name rule is
// Unverifiable (exit 6) — the tool cannot tell which STOP.cell.* file speaks for this
// session, so it cannot report "no cell stop held"; and no path is built from the value.
func sessionCell() (string, error) {
	cell := os.Getenv(CellMarkerEnv)
	if cell == "" {
		return "", nil
	}
	if !ValidCellName(cell) {
		return "", Unverifiable(fmt.Sprintf(
			"cell stop flag unverifiable: %s=%q fails the cell-name rule [a-z0-9-], so no "+
				"STOP.cell.<cell> file can be checked for it (and no path is built from it) — "+
				"this is could-not-check, NOT 'no cell stop held'", CellMarkerEnv, cell), nil)
	}
	return cell, nil
}

// cellRoleFlagNames is the loop name class of the session's ASSAY_CELL_ROLE, when that role
// names a known loop. A role that is no loop (cellctl's comms service runs with role "comms")
// contributes nothing: that process is still covered by the cell-wide flag and by DESK_LOOP.
func cellRoleFlagNames() []string {
	role := os.Getenv(CellRoleMarkerEnv)
	if role == "" {
		return nil
	}
	names, known := LoopFlagNames(role)
	if !known {
		return nil
	}
	return names
}

// cellLoopFlagState checks STOP.cell.<cell>.<name> for every name in names. cell == "" (no
// marker) skips the layer.
func cellLoopFlagState(dir, cell string, names []string) (armed bool, reason string, err error) {
	if cell == "" {
		return false, "", nil
	}
	for _, name := range names {
		a, r, ferr := flagFileState(dir, CellStopFlagName(cell, name))
		if ferr != nil {
			return false, "", ferr
		}
		if a {
			return true, r, nil
		}
	}
	return false, "", nil
}

// cellsRootDir resolves the cells root the same way cellctl does: $CELLS_ROOT, else
// $XDG_DATA_HOME (default ~/.local/share) + /assay/cells. "" when no home can be found.
func cellsRootDir() string {
	if v := os.Getenv("CELLS_ROOT"); v != "" {
		return v
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "assay", "cells")
}

// cellRegistered reports whether cell has a registration (a cell.env) under the cells root as
// this process resolves it, and names that root ("" when none could be resolved).
func cellRegistered(cell string) (registered bool, root string) {
	root = cellsRootDir()
	if root == "" {
		return false, ""
	}
	st, err := os.Stat(filepath.Join(root, cell, "cell.env"))
	return err == nil && !st.IsDir(), root
}

// cellFlagInertReason explains why a STOP.cell.* file halts nothing, or — for a cell this
// shell cannot find a registration for — why the board cannot show it live; "" when it is live.
// rest is the file name with the "STOP.cell." prefix removed: "<cell>" or "<cell>.<loop>".
// A cell name cannot contain '.', so the first '.' (if any) separates cell from loop.
func cellFlagInertReason(rest string) string {
	cell, loop, hasLoop := strings.Cut(rest, ".")
	if !ValidCellName(cell) {
		return fmt.Sprintf("%q fails the cell-name rule [a-z0-9-], so no session can carry it and this flag halts nothing", cell)
	}
	if hasLoop && !IsKnownLoopName(loop) {
		return loop + " matches no known loop, so this flag halts nothing"
	}
	// The registration is read from THIS process's cells root ($CELLS_ROOT, else the XDG
	// default), which is not necessarily the root the cell's sessions were launched from. So this
	// case names the root it looked in and says what the finding does NOT prove: the Guard keys
	// on the session's marker alone and still honours the flag.
	if ok, root := cellRegistered(cell); !ok {
		if root == "" {
			return "no cells root could be resolved from this shell (no CELLS_ROOT, no home) to look for cell " + cell +
				" — if sessions of that cell exist this flag is LIVE; check before removing it"
		}
		return "no registration for cell " + cell + " under " + root + " as seen from this shell " +
			"(CELLS_ROOT / XDG_DATA_HOME) — a cell registered under another root is still stopped by this flag, " +
			"so check the cell's own root before removing it"
	}
	return ""
}
