package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The claim store is resolved from the roster (deskkit.ResolveClaimStore), which is read from
// the config home. A test binary must therefore never read the developer's own roster: this
// points HOME at an EMPTY directory for the whole package, so the roster is absent, the store
// key is unset, and every verb test resolves to the legacy forge-ref store — exactly the
// behaviour this suite asserted before the store was resolved at all. A test that needs a
// configured key writes its own roster with withClaimRoster.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "deskclaim-ref-home-")
	if err != nil {
		panic("cannot create an empty test HOME: " + err.Error())
	}
	prev, had := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	code := m.Run()
	if had {
		_ = os.Setenv("HOME", prev)
	} else {
		_ = os.Unsetenv("HOME")
	}
	_ = os.RemoveAll(home)
	os.Exit(code)
}

// withClaimRoster writes a 0600 roster carrying vals into a private config home for one test.
func withClaimRoster(t *testing.T, vals map[string]string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := ""
	for k, v := range vals {
		body += fmt.Sprintf("%s=%s\n", k, v)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
