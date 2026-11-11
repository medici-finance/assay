package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// treeStateFiles are the files a deskd started with its default flags creates
// in its working directory. A test run must leave none of them behind: a run
// that checks the tree is unchanged after the tests (a verifier's admission
// bracket does) is otherwise voided. Every test that runs the binary goes
// through deskdCmd, which points them into t.TempDir(); this guard is the
// backstop for a test that does not. TestMain calls it.
var treeStateFiles = []string{"deskd-index.db", "deskd-index.db-shm", "deskd-index.db-wal"}

// treeLeakGuard records which state files already exist and returns a check
// to run after m.Run(). The check writes one FAIL line per state file that
// appeared during the run and returns the exit code the package must use.
func treeLeakGuard(stderr io.Writer) func(code int) int {
	paths := treeStatePaths()
	before := map[string]bool{}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			before[p] = true
		}
	}
	return func(code int) int {
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil && !before[p] {
				fmt.Fprintf(stderr, "FAIL: the cmd/deskd tests left %s in the tree — run the binary through deskdCmd so its state lands under t.TempDir()\n", p)
				code = 1
			}
		}
		return code
	}
}

// treeStatePaths is every path a leaked state file could land at: the
// package directory (the test's own cwd) and the module root (where deskdCmd
// runs the binary).
func treeStatePaths() []string {
	cwd, err := filepath.Abs(".")
	if err != nil {
		return nil
	}
	roots := []string{cwd}
	for dir := cwd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if dir != cwd {
				roots = append(roots, dir)
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	var out []string
	for _, r := range roots {
		for _, f := range treeStateFiles {
			out = append(out, filepath.Join(r, f))
		}
	}
	return out
}
