package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// regression: #2485
func TestCrossRepoReceiptRouting(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(t.TempDir(), "brief.md")
	binding := map[string]any{"Version": 2, "Run": "run", "Repo": "example-org/tracker", "Source": "source", "SourceRepo": "example-org/source", "TrackingSource": "tracking", "Brief": "docs/brief.md", "Model": "model", "Tier": "strong"}
	receipt := map[string]any{"Issue": 7, "BriefRoot": root, "Binding": binding}
	raw, _ := json.Marshal(receipt)
	line, path, err := verifierAdmissionReceipt(raw, original)
	if err != nil || path != filepath.Join(root, "docs/brief.md") || !strings.Contains(line, "source-repo=example-org/source source=source tracking-source=tracking") {
		t.Fatalf("cross-repo route: %q %q %v", line, path, err)
	}
	for _, bad := range []string{"../escape.md", "/absolute.md", ".", "docs/../brief.md", ""} {
		binding["Brief"] = bad
		raw, _ = json.Marshal(receipt)
		if _, _, err := verifierAdmissionReceipt(raw, original); err == nil {
			t.Fatalf("accepted path %q", bad)
		}
	}
	binding["Brief"] = "docs/brief.md"
	for _, key := range []string{"Repo", "SourceRepo", "TrackingSource"} {
		value := binding[key]
		binding[key] = ""
		raw, _ = json.Marshal(receipt)
		if _, _, err := verifierAdmissionReceipt(raw, original); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
		binding[key] = value
	}
	receipt["BriefRoot"] = "relative"
	raw, _ = json.Marshal(receipt)
	if _, _, err := verifierAdmissionReceipt(raw, original); err == nil {
		t.Fatal("accepted relative tracking home")
	}
	binding["Version"] = 3
	raw, _ = json.Marshal(receipt)
	if _, _, err := verifierAdmissionReceipt(raw, original); err == nil {
		t.Fatal("accepted unknown receipt version")
	}
	binding["Version"] = 1
	raw, _ = json.Marshal(receipt)
	if _, path, err := verifierAdmissionReceipt(raw, original); err != nil || path != original {
		t.Fatalf("same-repo path changed: %s %v", path, err)
	}
}
