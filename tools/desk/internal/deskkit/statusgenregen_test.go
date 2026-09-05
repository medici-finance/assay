package deskkit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeStatusgen writes a shell shim that records its argv (one per line) to
// <dir>/argv and echoes a line to stdout, then points STATUSGEN_BIN at it. It
// returns the argv path. Skips on non-unix (the shim is a shell script).
func fakeStatusgen(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim fake statusgen is unix-only")
	}
	dir := t.TempDir()
	argv := filepath.Join(dir, "argv")
	shim := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + argv + "; done\necho \"fake statusgen ran: $*\"\n"
	bin := filepath.Join(dir, "statusgen")
	if err := os.WriteFile(bin, []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(StatusgenBinEnv, bin)
	return argv
}

func statusgenRegenMigration() Migration {
	return Migration{
		ID: "0001-v0.13.0-to-v1.0.0-derived-board", From: "v0.13.0", To: "v1.0.0",
		Notes: "what changed",
		Apply: []Step{{StatusgenRegen: &StatusgenRegen{Verb: "migrate", Args: []string{"brief-v1-to-v2"}}}},
	}
}

func TestStatusgenRegen_RunsPinnedBinary(t *testing.T) {
	argv := fakeStatusgen(t)
	root := t.TempDir()
	actions, err := RunMigrations(root, []Migration{statusgenRegenMigration()}, false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(actions) != 1 || !actions[0].Changed {
		t.Fatalf("actions = %+v", actions)
	}
	if !strings.Contains(actions[0].Desc, "fake statusgen ran") {
		t.Errorf("step desc did not capture statusgen output: %q", actions[0].Desc)
	}
	got, _ := os.ReadFile(argv)
	gs := string(got)
	for _, want := range []string{"migrate", "brief-v1-to-v2", "--root"} {
		if !strings.Contains(gs, want) {
			t.Errorf("statusgen was not invoked with %q; argv:\n%s", want, gs)
		}
	}
	if strings.Contains(gs, "--dry-run") {
		t.Errorf("apply run must NOT pass --dry-run; argv:\n%s", gs)
	}
}

func TestStatusgenRegen_DryRunForwardsFlag(t *testing.T) {
	argv := fakeStatusgen(t)
	root := t.TempDir()
	actions, err := RunMigrations(root, []Migration{statusgenRegenMigration()}, true)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(actions) != 1 || !strings.HasPrefix(actions[0].Desc, "WOULD") {
		t.Errorf("dry-run should report a WOULD action: %+v", actions)
	}
	got, _ := os.ReadFile(argv)
	if !strings.Contains(string(got), "--dry-run") {
		t.Errorf("dry-run must forward --dry-run to statusgen; argv:\n%s", got)
	}
}

func TestStatusgenRegen_UnknownVerbRefused(t *testing.T) {
	fakeStatusgen(t)
	root := t.TempDir()
	mig := Migration{
		ID: "0001", From: "v0.13.0", To: "v1.0.0", Notes: "x",
		Apply: []Step{{StatusgenRegen: &StatusgenRegen{Verb: "frobnicate", Args: []string{"whatever"}}}},
	}
	if _, err := RunMigrations(root, []Migration{mig}, false); !IsRefused(err) {
		t.Fatalf("unknown verb must be refused, got %v", err)
	}
	// Unknown TARGET under a known verb is likewise refused.
	mig2 := Migration{
		ID: "0001", From: "v0.13.0", To: "v1.0.0", Notes: "x",
		Apply: []Step{{StatusgenRegen: &StatusgenRegen{Verb: "migrate", Args: []string{"brief-v2-to-v3"}}}},
	}
	if _, err := RunMigrations(root, []Migration{mig2}, false); !IsRefused(err) {
		t.Fatalf("unknown target must be refused, got %v", err)
	}
}

func TestStatusgenRegen_ParsesInMigrationFile(t *testing.T) {
	dir := t.TempDir()
	migDir := filepath.Join(dir, MigrationsDir)
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: \"0001-v0.13.0-to-v1.0.0\"\nfrom: v0.13.0\nto: v1.0.0\napply:\n" +
		"  - statusgen-regen:\n      verb: migrate\n      args: [brief-v1-to-v2]\n" +
		"  - ensure-line:\n      file: docs/UPGRADING.txt\n      text: \"v1.0.0: brief-v2\"\n---\n\n## What changed\n\nbody\n"
	if err := os.WriteFile(filepath.Join(migDir, "0001-x.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	migs, err := LoadMigrations(migDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(migs) != 1 || len(migs[0].Apply) != 2 {
		t.Fatalf("migs = %+v", migs)
	}
	if migs[0].Apply[0].StatusgenRegen == nil || migs[0].Apply[0].StatusgenRegen.Verb != "migrate" {
		t.Errorf("statusgen-regen step not parsed: %+v", migs[0].Apply[0])
	}
	if migs[0].Apply[1].EnsureLine == nil {
		t.Errorf("ensure-line step not parsed: %+v", migs[0].Apply[1])
	}
}
