// Package cachequiet keeps a test binary off the operator's cell Go-cache. It is test
// support: a package's TestMain calls Isolate before running its tests, and no desk tool
// imports it.
//
// THE DEFECT IT CLOSES. A cell launcher exports ASSAY_GO_CACHE_POLICY into every shell it
// starts, naming the cell's live Go-cache root. Desk tool code reads that variable from the
// process environment: deskdispatch's storage admission runs a non-dry cellcache.Admit (cellcache.Check for anything it cannot prove cheap)
// against the named root, and cellprocess's launch custody takes a lease on it. Both take
// the root's exclusive advisory lock. A test binary run from such a shell inherits the
// variable, so every test that drives those paths takes the operator's real lock, writes
// the operator's real cache report and, under storage pressure, reclaims the operator's
// real cache. While the test run holds the lock, a production dispatch on the same
// machine waits its bounded lock wait (one minute) and returns storage-deferred (exit 6, nothing claimed);
// while production holds it, the tests fail the same way and read as a code regression.
//
// THE FIX. Isolate removes the cell's storage-admission variables (Vars) from the test
// process's environment before any test runs, so neither the test code nor any child it
// starts with the inherited environment can find the live root. A test that exercises
// admission deliberately sets its own policy, rooted under t.TempDir(), with t.Setenv —
// which still works, because it runs after Isolate.
//
// The go toolchain's own variables (GOCACHE, GOMODCACHE, GOPATH) are left alone: a test
// that builds a binary still shares the build cache the way any go command does, and the
// go command's own locking covers that.
//
// THE GUARD. Every package under tools/desk whose own code links internal/cellcache,
// directly or through another package of this module, and which has tests, must call
// Isolate from its TestMain. The guard in this package's tests enforces it.
package cachequiet

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

// Vars are the variables Isolate clears: the managed cache policy, and the free-space floor
// deskdispatch's storage admission applies (DESK_MIN_FREE_BYTES, read in
// cmd/deskdispatch/storage.go), which turns an ordinary dispatch test into a
// storage-deferred one on a machine that exports it.
var Vars = []string{cellcache.EnvPolicy, "DESK_MIN_FREE_BYTES"}

// Isolate clears Vars from this process's environment, and so from every child that
// inherits it. A package's TestMain calls it first. A variable that cannot be cleared
// stops the test binary: running on would put the tests back on the live cache.
func Isolate() {
	for _, k := range Vars {
		if err := os.Unsetenv(k); err != nil {
			fmt.Fprintf(os.Stderr, "cachequiet: cannot clear %s: %v\n", k, err)
			os.Exit(2)
		}
	}
}
