package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet). It
// first clears the cell's storage-admission variables, so no test reaches the operator's
// live Go-cache root through an inherited policy (see package cachequiet; #2491). A re-executed
// helper process (the admission fixture's claim tool or dispatcher) runs its role and exits first.
func TestMain(m *testing.M) {
	// Make every test an unheld window before anything runs, helper processes included, so no
	// test reads a holding container's real role-secret variable or mount (#6065): drop the
	// variables from this process (runCmdEnv's child strip reads it directly) and pin the two
	// seams. A role-secret test sets its own window with rsWindow or t.Setenv.
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); deskkit.IsRoleSecretEnvName(name) {
			os.Unsetenv(name)
		}
	}
	roleSecretEnviron = func() []string { return nil }
	roleSecretMountGlob = filepath.Join(os.DevNull, "role-*")
	if code, ok := admitHelperMain(); ok {
		os.Exit(code)
	}
	cachequiet.Isolate()
	os.Exit(gitquiet.Run(m))
}
