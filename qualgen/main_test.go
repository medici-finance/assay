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

// TestModeScaffolding is Verify row 7: report / pr / check are each recognized,
// parse flags, print a `not yet implemented` NOTICE, and exit 0.
func TestModeScaffolding(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"report", []string{"report", "--out", "/tmp/x"}},
		{"pr", []string{"pr", "1", "--out", "/tmp/x"}},
		{"check", []string{"check", "qualgen/main.go", "--out", "/tmp/x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := dispatch(c.args, &out, &errb)
			if rc != 0 {
				t.Fatalf("exit %d, stderr=%s", rc, errb.String())
			}
			if !strings.Contains(out.String(), "not yet implemented") {
				t.Fatalf("expected a not-yet-implemented NOTICE, got %q", out.String())
			}
			if !strings.Contains(out.String(), "NOTICE") {
				t.Fatalf("expected a NOTICE line, got %q", out.String())
			}
		})
	}
}

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
