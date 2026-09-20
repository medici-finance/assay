package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRosterParsesReadsTheCellHome is the port's half of a proof the behavioural suites can only
// make against the shell oracle.
//
// The oracle shells out to `env HOME=<cell home> deskroster …` and a stub can record the $HOME it
// was handed; the port asks deskkit the SAME question in-process — the reuse brief
// the port's brief requires — so there is no subprocess to observe. What still has to be true
// is the property, not the mechanism: the roster that answers is the CELL's, and the caller's own
// HOME is restored afterwards.
func TestRosterParsesReadsTheCellHome(t *testing.T) {
	cellHome := t.TempDir()
	otherHome := t.TempDir()
	// The CELL's roster loads; the "operator's" does not exist at all.
	cfgDir := filepath.Join(cellHome, ".config", "assay")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\n" +
		"ASSAY_ALLOWED_REPOS=example-org/example-repo\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	// deskkit resolves ASSAY_CONFIG_HOME ahead of HOME; a cell read must not be answered by
	// whatever the launching shell had pointed that at.
	t.Setenv("ASSAY_CONFIG_HOME", "")
	t.Setenv("HOME", otherHome)

	c := &Cell{Env: envWith(map[string]string{}), Name: "demo", Home: cellHome, Config: cfgDir}
	if !c.rosterParses() {
		t.Error("the cell home's roster loads, so the row must be ok")
	}
	if got := os.Getenv("HOME"); got != otherHome {
		t.Errorf("HOME was left at %q — the override must be undone", got)
	}

	// And the negative: a cell home with no roster at all must NOT be answered by some other
	// home that happens to have one.
	empty := t.TempDir()
	c2 := &Cell{Env: envWith(map[string]string{}), Name: "demo", Home: empty,
		Config: filepath.Join(empty, ".config", "assay")}
	if c2.rosterParses() {
		t.Error("a cell with no roster must MISS, not inherit another home's answer")
	}
}

// TestRosterAllowedReposUsesDeskkitsKey pins the two things that row worth having: the variable
// NAME comes from deskkit (one definition in the tree, not a second literal here), and the value
// is the RAW last-active line, which is what the oracle's exact-scope comparison reads.
func TestRosterAllowedReposUsesDeskkitsKey(t *testing.T) {
	if deskkit.EnvAllowedRepos != "ASSAY_ALLOWED_REPOS" {
		t.Fatalf("deskkit.EnvAllowedRepos = %q — this test pins the seam, not the spelling", deskkit.EnvAllowedRepos)
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".config", "assay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "#ASSAY_ALLOWED_REPOS=commented/out\n" +
		"ASSAY_ALLOWED_REPOS=first/one\n" +
		"ASSAY_ALLOWED_REPOS=example-org/example-repo\n"
	if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Cell{Env: envWith(map[string]string{}), Home: dir, Config: cfg}
	if got := c.rosterScopeLine(); got != "example-org/example-repo" {
		t.Errorf("rosterScopeLine = %q — want the LAST active line, raw", got)
	}
	// No roster at all is "", never a panic and never a match.
	c2 := &Cell{Env: envWith(map[string]string{}), Home: t.TempDir(), Config: t.TempDir()}
	if got := c2.rosterScopeLine(); got != "" {
		t.Errorf("no roster ⇒ %q, want empty", got)
	}
}
