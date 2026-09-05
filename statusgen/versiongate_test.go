package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateBelowV1(t *testing.T) {
	cases := map[string]bool{
		"v0.13.0": true,
		"v0.25.1": true,
		"v0.9.1":  true,
		"v1.0.0":  false,
		"v1.2.3":  false,
		"dev":     false, // unstamped → latest
		"":        false,
		"garbage": false,
	}
	for tag, want := range cases {
		if got := gateBelowV1(tag); got != want {
			t.Errorf("gateBelowV1(%q)=%v, want %v", tag, got, want)
		}
	}
}

func TestRefuseIfTreeTooNewRefusesStampedBelowV1OnV2Tree(t *testing.T) {
	root := migrateFixtureTree(t, true)
	// Migrate the fixture to brief-v2 so the tree carries v2 briefs.
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("setup migrate failed: %s", errb.String())
	}
	var gerr bytes.Buffer
	// A stamped build below v1.0.0 must refuse.
	if code := refuseIfTreeTooNew([]string{root}, "v0.13.0", &gerr); code != statusgenExitTreeTooNew {
		t.Fatalf("stamped v0.13.0 on v2 tree: exit=%d, want %d", code, statusgenExitTreeTooNew)
	}
	if !strings.Contains(gerr.String(), "tree is brief-v2") {
		t.Errorf("stderr missing refusal message: %s", gerr.String())
	}
	// An unstamped/latest build must NOT refuse.
	if code := refuseIfTreeTooNew([]string{root}, "dev", &bytes.Buffer{}); code != 0 {
		t.Errorf("dev build should not be gated, exit=%d", code)
	}
	if code := refuseIfTreeTooNew([]string{root}, "v1.0.0", &bytes.Buffer{}); code != 0 {
		t.Errorf("v1.0.0 build should not be gated, exit=%d", code)
	}
}

func TestRefuseIfTreeTooNewIgnoresV1Tree(t *testing.T) {
	root := migrateFixtureTree(t, true) // still brief-v1
	if code := refuseIfTreeTooNew([]string{root}, "v0.13.0", &bytes.Buffer{}); code != 0 {
		t.Errorf("v1 tree must not trip the gate even on an old binary, exit=%d", code)
	}
}

func TestAssayVersionsPinTagConsistency(t *testing.T) {
	root := t.TempDir()
	// Differing artifact tags → PROBLEM.
	mixed := "assay v1.0.0\nstatusgen v1.0.0 aaaa\ndesk-tools-linux-amd64 v0.13.0 bbbb\n"
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(mixed), 0o644); err != nil {
		t.Fatal(err)
	}
	p, ok := assayVersionsPinTagConsistency(root)
	if !ok {
		t.Fatal("expected a PROBLEM for differing tags")
	}
	if !strings.Contains(p, "artifact tags differ") {
		t.Errorf("message missing 'artifact tags differ': %s", p)
	}

	// Same artifact tags (umbrella line ignored) → no problem.
	same := "assay v1.0.0\nstatusgen v1.0.0 aaaa\ndesk-tools v1.0.0 bbbb\n"
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(same), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := assayVersionsPinTagConsistency(root); ok {
		t.Error("consistent tags should not be a problem")
	}

	// Absent file → not applicable, never a false PROBLEM.
	empty := t.TempDir()
	if _, ok := assayVersionsPinTagConsistency(empty); ok {
		t.Error("absent .assay-versions must not be a problem")
	}
}
