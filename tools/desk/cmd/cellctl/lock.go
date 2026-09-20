package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// takeSessionLock is layer 2 of a scrubbed cell's session lock: `mkdir`, which is atomic on
// every filesystem this runs on, holding the pid of whatever should be judged "alive".
//
// A mkdir lock, deliberately, and NOT a build-tagged flock: os.Mkdir is portable, and a native
// Windows cellctl is a consequence this brief must not make harder (the Windows-port stream owns
// delivering one). It is refused with exit 4 and the exact message the oracle pins ONLY when the
// held pid is still alive — a stale lock (a dead pid, or an unreadable pid file) is taken over
// rather than left to wedge every future boot. `status`/`down` are the tools that report/clear a
// stale lock explicitly for a human who is just looking; `desk` itself does not wait on one.
func (c *Cell) takeSessionLock(lockdir, session string) {
	if err := os.Mkdir(lockdir, 0o700); err == nil {
		writePid(lockdir, strconv.Itoa(os.Getpid()))
		return
	}
	held := readPid(lockdir)
	if held != "" && pidAlive(held) {
		fmt.Fprintf(os.Stderr, "cellctl: cell %s is already running (pid %s, session %s); cellctl down %s to release\n",
			c.Name, held, session, c.Name)
		exitWith(4)
	}
	writePid(lockdir, strconv.Itoa(os.Getpid()))
}

func writePid(lockdir, pid string) {
	_ = os.WriteFile(filepath.Join(lockdir, "pid"), []byte(pid+"\n"), 0o600)
}
