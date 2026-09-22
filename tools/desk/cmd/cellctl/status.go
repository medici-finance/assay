package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// cmdLs lists the cells under $CELLS_ROOT. No cells yet is a LEGITIMATE state, not a failure: an
// unmatched glob and an empty root both print nothing and exit 0.
func cmdLs() {
	e := newEnvFromProcess()
	root := cellsRoot(e)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, en := range entries {
		// The oracle's glob is `"$CELLS_ROOT"/*/`, which follows a symlinked directory too.
		st, serr := os.Stat(filepath.Join(root, en.Name()))
		if serr != nil || !st.IsDir() {
			continue
		}
		if fi, ferr := os.Stat(filepath.Join(root, en.Name(), "cell.env")); ferr == nil && !fi.IsDir() {
			fmt.Println(en.Name())
		}
	}
}

// cmdStatus is a scrubbed-cell-only READ — `running <session>` / `stopped` / `stale-lock <pid>`
// — never a precondition check (that is `check`'s job), exit 0 in every case that is not a load
// error.
func cmdStatus(cell string) {
	c := loadCell(cell)
	if c.Kind != "scrubbed" {
		die("status is only defined for a scrubbed cell (kind=%s)", c.Kind)
	}
	fmt.Println(c.statusLine())
}

func (c *Cell) statusLine() string {
	sessname := c.Name + "-cell"
	lockdir := filepath.Join(c.Dir, "run", "lock.d")
	st, err := os.Stat(lockdir)
	if err != nil || !st.IsDir() {
		return "stopped"
	}
	pid := readPid(lockdir)
	if pid != "" && pidAlive(pid) {
		return "running " + sessname
	}
	return "stale-lock " + pid
}

func readPid(lockdir string) string {
	raw, err := os.ReadFile(filepath.Join(lockdir, "pid"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// pidAlive is the oracle's `kill -0 <pid>`: the process exists and this user may signal it.
//
// It shells to `kill` rather than sending signal 0 in-process ON PURPOSE. Signal 0 in Go needs
// the low-level system-call package, and this package uses NONE of it — brief
// the port's brief asserts exactly that, because a native-Windows cellctl is a
// consequence this work must not make worse (internal/deskkit already carries the unix-only
// system-call sites the Windows-port stream owns; cellctl adds none of its own). One fork on a rare path
// is the price of that.
func pidAlive(pid string) bool {
	n, err := strconv.Atoi(pid)
	if err != nil || n <= 0 {
		return false
	}
	return exec.Command("kill", "-0", strconv.Itoa(n)).Run() == nil
}
