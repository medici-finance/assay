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

func TestRefuseIfTreeTooNew_StampedBelowV1(t *testing.T) {
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

func TestRefuseIfTreeTooNew_IgnoresV1Tree(t *testing.T) {
	root := migrateFixtureTree(t, true) // still brief-v1
	if code := refuseIfTreeTooNew([]string{root}, "v0.13.0", &bytes.Buffer{}); code != 0 {
		t.Errorf("v1 tree must not trip the gate even on an old binary, exit=%d", code)
	}
}

func TestAssayVersions_PinTagConsistency(t *testing.T) {
	root := t.TempDir()
	// Differing artifact tags → PROBLEM.
	mixed := "assay v1.0.0\nstatusgen v1.0.0 aaaa\ndesk-tools-linux-amd64 v0.13.0 bbbb\n"
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(mixed), 0o644); err != nil {
		t.Fatal(err)
	}
	p, ok := sameTagPinLint(root)
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
	if _, ok := sameTagPinLint(root); ok {
		t.Error("consistent tags should not be a problem")
	}

	// Absent file → not applicable, never a false PROBLEM.
	empty := t.TempDir()
	if _, ok := sameTagPinLint(empty); ok {
		t.Error("absent .assay-versions must not be a problem")
	}
}

func TestSameTagPinLint_ExemptionMarker(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}

	// Sub-case (a): umbrella artifacts share a tag; a guard binary of the SAME
	// desk-tools family is frozen on an earlier tag by a recorded ruling and
	// carries the exemption marker → NO problem.
	frozenGuard := "assay v1.0.0\n" +
		"statusgen v1.0.0 aaaa\n" +
		"desk-tools v1.0.0 bbbb\n" +
		"desk-tools-guard v0.13.0 cccc # same-tag: exempt — frozen by maintainer ruling\n"
	if p, ok := sameTagPinLint(write(t, frozenGuard)); ok {
		t.Errorf("exempt frozen guard should clear the lint, got PROBLEM: %s", p)
	}

	// Sub-case (b): a separate-repository artifact on its own release cadence,
	// exempted, alongside same-tag umbrella artifacts → NO problem.
	foreign := "assay v1.0.0\n" +
		"statusgen v1.0.0 aaaa\n" +
		"desk-tools v1.0.0 bbbb\n" +
		"reconciler v2.4.1 dddd # same-tag: exempt — separate release cadence\n"
	if p, ok := sameTagPinLint(write(t, foreign)); ok {
		t.Errorf("exempt foreign artifact should clear the lint, got PROBLEM: %s", p)
	}

	// Negative: a genuine UNexempted mixed-tag state STILL PROBLEMs (the real
	// check is not weakened). Here two umbrella artifacts disagree with no marker.
	mixed := "assay v1.0.0\n" +
		"statusgen v1.0.0 aaaa\n" +
		"desk-tools v0.13.0 bbbb\n" +
		"reconciler v2.4.1 dddd # same-tag: exempt — separate release cadence\n"
	p, ok := sameTagPinLint(write(t, mixed))
	if !ok {
		t.Fatal("unexempted mixed tags must still PROBLEM")
	}
	if !strings.Contains(p, "artifact tags differ") {
		t.Errorf("message missing 'artifact tags differ': %s", p)
	}
	// The exempt artifact's off-tag must NOT appear in the mixed-state message.
	if strings.Contains(p, "v2.4.1") {
		t.Errorf("exempt artifact leaked into the PROBLEM message: %s", p)
	}
}
