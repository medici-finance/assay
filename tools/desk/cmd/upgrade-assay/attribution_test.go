package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
)

// TestMain points the Cursor attribution check at an empty scratch home with its
// config-location overrides unset, so no test reads the real host's Cursor CLI config.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "upgrade-assay-home")
	if err != nil {
		panic(err)
	}
	cursorbundle.HostHome = func() (string, error) { return home, nil }
	cursorbundle.HostEnv = func(string) string { return "" }
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// regression: #5892
// The Cursor upgrade path reports an on attribution setting, keeps its exit code, and never
// edits the user's config.
func TestCursorUpgradeAttribution(t *testing.T) {
	home := t.TempDir()
	saved := cursorbundle.HostHome
	t.Cleanup(func() { cursorbundle.HostHome = saved })
	cursorbundle.HostHome = func() (string, error) { return home, nil }
	cfg := cursorbundle.CLIConfigPath(home)
	body := `{"attribution":{"attributeCommitsToAgent":true}}`
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	root := writeDerivedFixture(t)
	materializeCursor(t, root, cursorRelease(t, "v0.13.0"))
	args := []string{"--root", root, "--to", "v0.13.0", "--harness", "cursor", "--forge", "github"}
	for _, a := range [][]string{args, append(append([]string{}, args...), "--check")} {
		code, out := run2(t, a...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", a, code, out)
		}
		if !strings.Contains(out, "WARNING: Cursor CLI attribution: "+cursorbundle.AttrCommitsKey+" is on") {
			t.Fatalf("%v: no attribution warning:\n%s", a, out)
		}
	}
	if got, err := os.ReadFile(cfg); err != nil || string(got) != body {
		t.Fatalf("upgrade changed the user's Cursor config: %q, %v", got, err)
	}
}
