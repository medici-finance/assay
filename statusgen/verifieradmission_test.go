package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestVerifierAdmissionBeforeRows(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "fixture", "fixture@example.invalid")
	t.Setenv("DESK_LOOP", "verify-desk")
	// An unavailable checker is fail-closed; never consult a developer installation.
	t.Setenv("PATH", t.TempDir())
	brief := filepath.Join(root, "brief.md")
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + string(rune(96)) + "touch ran-one" + string(rune(96)) + " | exit 0 |\n| 2 | " + string(rune(96)) + "touch ran-two" + string(rune(96)) + " | exit 0 |\n\n## Evidence\n\n"
	if err := os.WriteFile(brief, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	got, _ := captureVerifyrun(t, []string{"--brief", brief, "--root", root, "--dry-run"})
	if got.code == 0 {
		t.Error("unstamped verifier admitted")
	}
	for _, name := range []string{"ran-one", "ran-two"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("Verify row executed before admission: %s", name)
		}
	}
}

// TestDeskLoopCleared pins TestMain's clearing of DESK_LOOP: it re-runs this
// test binary with DESK_LOOP set to a verify loop and requires the child to
// see none, so no test here can inherit a verify loop and reach the real
// deskdispatch through admission, whatever the CI environment carries.
func TestDeskLoopCleared(t *testing.T) {
	if os.Getenv("STATUSGEN_DESKLOOP_PROBE") == "1" {
		if v, ok := os.LookupEnv("DESK_LOOP"); ok {
			t.Fatalf("the test binary inherited DESK_LOOP=%q", v)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeskLoopCleared$", "-test.count=1")
	cmd.Env = append(os.Environ(), "DESK_LOOP=verify-desk", "STATUSGEN_DESKLOOP_PROBE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("a DESK_LOOP set by the caller reached the tests: %v\n%s", err, out)
	}
}
