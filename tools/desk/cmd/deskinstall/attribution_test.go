package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cursorHome points the attribution check at a scratch home holding body as its global
// Cursor CLI config, and returns the config path.
func cursorHome(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	saved := cursorbundle.HostHome
	t.Cleanup(func() { cursorbundle.HostHome = saved })
	cursorbundle.HostHome = func() (string, error) { return home, nil }
	p := cursorbundle.CLIConfigPath(home)
	mustWrite(t, p, body)
	return p
}

// regression: #5892
// Placement and --check report the host's attribution setting when it is on, keep their own
// exit codes, and leave the user's config byte-identical.
func TestHarnessAttributionReport(t *testing.T) {
	body := `{"attribution":{"attributeCommitsToAgent":true,"attributePRsToAgent":true}}`
	cfg := cursorHome(t, body)
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	repo := t.TempDir()
	base := []string{"--harness", "cursor", "--bundle", bundle, "--repo", repo, "--forge", "github"}
	for _, args := range [][]string{base, append(append([]string{}, base...), "--check")} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("%v: exit %d, want %d: %s", args, code, deskkit.ExitOK, errb.String())
		}
		for _, key := range []string{cursorbundle.AttrCommitsKey, cursorbundle.AttrPRsKey} {
			if !strings.Contains(errb.String(), "WARNING: Cursor CLI attribution: "+key+" is on") {
				t.Errorf("%v: stderr does not report %s:\n%s", args, key, errb.String())
			}
		}
		if !strings.Contains(errb.String(), cfg) {
			t.Errorf("%v: stderr does not name %s", args, cfg)
		}
	}
	if got, err := os.ReadFile(cfg); err != nil || string(got) != body {
		t.Fatalf("the installer changed the user's Cursor config: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "cli.json")); !os.IsNotExist(err) {
		t.Fatalf("the installer placed a project cli.json: %v", err)
	}
}

// With both settings false the report is one stdout line and stderr stays empty.
func TestHarnessAttributionOff(t *testing.T) {
	cursorHome(t, `{"attribution":{"attributeCommitsToAgent":false,"attributePRsToAgent":false}}`)
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	var out, errb bytes.Buffer
	code := run([]string{"--harness", "cursor", "--bundle", bundle, "--repo", t.TempDir(), "--forge", "github"}, &out, &errb)
	if code != deskkit.ExitOK || errb.Len() != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "Cursor CLI attribution: off") {
		t.Fatalf("stdout lacks the off line: %q", out.String())
	}
}
