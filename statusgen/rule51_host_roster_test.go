package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #7304
func TestRule51HostLintRoster(t *testing.T) {
	slForge(t, rrBoundForge())
	root, _ := admittedFixture(t, true)
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	bin := buildStatusgenBinary(t)
	hostHome := t.TempDir()
	roster := filepath.Join(hostHome, ".config", "assay", "roster.env")
	if err := os.MkdirAll(filepath.Dir(roster), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roster, []byte(fixtureRoster+darFixtureExtra()+"UNUSED_FIXTURE_VALUE=private-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	useHome(t, hostHome)
	prevMint := runReadCredentialMint
	runReadCredentialMint = func(_, out string) (string, error) {
		if err := os.WriteFile(out+".perms", []byte(rcRead), 0o600); err != nil {
			return "", err
		}
		return out, os.WriteFile(out, []byte(rcMinted), 0o600)
	}
	t.Cleanup(func() { runReadCredentialMint = prevMint })

	dir := t.TempDir()
	log := filepath.Join(dir, "forge-output")
	native := filepath.Join(dir, "pinned-statusgen")
	// Capture the actual configuration refusal on the old implementation.
	// The wrapper forwards the unmodified output and exit code of the built
	// statusgen. The goodrepo fixture has no forge subjects to corroborate.
	script := "#!/bin/sh\n" +
		"test -f \"$HOME/.config/assay/roster.env\" && grep -q 'UNUSED_FIXTURE_VALUE' \"$HOME/.config/assay/roster.env\" && exit 9\n" +
		shellQuoteArg(bin) + " \"$@\" > " + shellQuoteArg(log) + " 2>&1\n" +
		"rc=$?; cat " + shellQuoteArg(log) + "; exit \"$rc\"\n"
	if err := os.WriteFile(native, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	prevExe := rule51Executable
	rule51Executable = func() (string, error) { return native, nil }
	t.Cleanup(func() { rule51Executable = prevExe })
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	brief := filepath.Join(t.TempDir(), "brief.md")
	ordinary := "| 2 | `test ! -f \"$HOME/.config/assay/roster.env\"` | exit 0 |\n"
	otherLint := "| 3 | `statusgen --lint` | exit 0 |\n"
	putFile(t, brief, "# Fixture\n\n## Verify\n\n"+r51Verify("statusgen --root . --lint")+ordinary+otherLint+"\n## Evidence\n")
	run := func() string {
		_, out := captureVerifyrunOut(t, []string{"--brief", brief, "--root", root, "--rule51", "--dry-run"})
		return out
	}
	out := run()
	if strings.Count(out, "rule51=pass forge-rc=0") != 2 || !strings.Contains(out, "row 2: pass") {
		t.Fatalf("host rule-51 run:\n%s\npinned lint output:\n%s", out, getFile(t, log))
	}
	if strings.Contains(out, rcMinted) || strings.Contains(out, "private-fixture") {
		t.Fatal("private fixture values appeared in witness output")
	}
	if err := os.Remove(roster); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	out = run()
	if strings.Count(out, "rule51=could-not-run forge-rc=-") != 2 || strings.Contains(out, rcMinted) || strings.Contains(out, "private-fixture") {
		t.Fatalf("missing roster must refuse the stamp without printing private values: %s", out)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("pinned lint ran without the qualified host roster: %v", err)
	}
}
