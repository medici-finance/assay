package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionDefaultIsDev pins the pin-checkability contract: an unstamped build
// answers "dev" rather than inventing a release number.
func TestVersionDefaultIsDev(t *testing.T) {
	if qualgenVersion != "dev" {
		t.Fatalf("in-package default qualgenVersion = %q, want \"dev\"", qualgenVersion)
	}
	var out, errb bytes.Buffer
	if rc := dispatch([]string{"--version"}, &out, &errb); rc != 0 {
		t.Fatalf("--version exit %d", rc)
	}
	if strings.TrimSpace(out.String()) != "dev" {
		t.Fatalf("--version printed %q", out.String())
	}
}

// TestModeScaffolding used to pin the still-stubbed `pr` mode's NOTICE
// output; `pr` is no longer a stub — it emits the per-PR risk-feature feed
// (quality/08) and is exercised by the TestPR_* suite in pr_test.go instead.
// `report` renders the trend view (quality/05, TestReport_* suite); `check`
// runs the brittleness screen (quality/09, TestCheck_* suite in
// check_test.go). No mode remains scaffolding-only.

// TestUnknownModeIsUsageError pins that an unknown mode is a usage error (exit 2),
// not a silent success.
func TestUnknownModeIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := dispatch([]string{"frobnicate"}, &out, &errb); rc != 2 {
		t.Fatalf("unknown mode should exit 2, got %d", rc)
	}
}

// TestMineRequiresOut pins that mine without --out (and without --in-repo) is a
// usage error rather than a silent write into the mined repo.
func TestMineRequiresOut(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := dispatch([]string{"mine", "--repo", "."}, &out, &errb); rc != 2 {
		t.Fatalf("mine without --out should exit 2, got %d (stderr=%s)", rc, errb.String())
	}
}
