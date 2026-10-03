package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestRootsOverride(t *testing.T) {
	installRosterEnv(t, fixtureRoster)
	t.Setenv(deskkit.RootsEnv, "example-org/examples=./custom-examples,example-org/agents=./custom-agents")
	var out string
	err := runCaptured(t, &out, func() error { return cmdRepos([]string{"--scope", "roots"}) })
	if err != nil {
		t.Fatal(err)
	}
	want := "example-org/agents\troot=./custom-agents\nexample-org/examples\troot=./custom-examples\n"
	var rows strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			rows.WriteString(line + "\n")
		}
	}
	if rows.String() != want {
		t.Fatalf("configured rows = %q, want %q", rows.String(), want)
	}
}

func TestRootsInvalid(t *testing.T) {
	installRosterEnv(t, fixtureRoster)
	for _, mapping := range []string{
		"example-org/agents", "example-org/agents=", ",,",
		"example-org/outside=./outside",
		"example-org/agents=./one,example-org/agents=./two",
		"example-org/agents=./valid,example-org/outside=./outside",
	} {
		t.Run(mapping, func(t *testing.T) {
			t.Setenv(deskkit.RootsEnv, mapping)
			var out string
			err := runCaptured(t, &out, func() error { return cmdRepos([]string{"--scope", "roots"}) })
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("error = %v, want refused", err)
			}
			if err == nil || !strings.Contains(err.Error(), deskkit.RootsEnv) {
				t.Fatalf("error does not name invalid roots: %v", err)
			}
			if strings.TrimSpace(out) != "" {
				t.Fatalf("invalid configuration printed a partial inventory: %q", out)
			}
		})
	}
}

func TestRootsDefault(t *testing.T) {
	t.Setenv(deskkit.RootsEnv, "")
	var out string
	err := runCaptured(t, &out, func() error { return cmdRepos([]string{"--scope", "roots"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"example-org/tracker\troot=.\n", "medici-finance/assay\troot=../assay\n"} {
		if !strings.Contains(out, row) {
			t.Fatalf("default inventory lacks %q: %q", row, out)
		}
	}
}

func TestRootsLeaveTopology(t *testing.T) {
	installRosterEnv(t, fixtureRoster+"ASSAY_SCAN_REPOS=example-org/agents\n")
	for _, scope := range []string{"topology", "all", "write", "scan"} {
		var original, overridden string
		t.Setenv(deskkit.RootsEnv, "")
		if err := runCaptured(t, &original, func() error { return cmdRepos([]string{"--scope", scope}) }); err != nil {
			t.Fatal(err)
		}
		t.Setenv(deskkit.RootsEnv, "example-org/outside=./outside")
		if err := runCaptured(t, &overridden, func() error { return cmdRepos([]string{"--scope", scope}) }); err != nil {
			t.Fatal(err)
		}
		if original != overridden {
			t.Fatalf("roots override changed %s inventory", scope)
		}
	}
}
