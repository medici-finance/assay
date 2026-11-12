package main

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet). It
// also points the Cursor attribution check at an empty scratch home with its config-location
// overrides unset, so no test reads the real host's Cursor CLI config.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "deskinstall-home")
	if err != nil {
		panic(err)
	}
	cursorbundle.HostHome = func() (string, error) { return home, nil }
	cursorbundle.HostEnv = func(string) string { return "" }
	code := gitquiet.Run(m)
	os.RemoveAll(home)
	os.Exit(code)
}
