package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The writer side's on-behalf-of stamp (deskevidence, principalstamp.go) must produce rows
// this lint accepts. deskevidence's own test pins that a landing of the UNSTAMPED fixture
// writes exactly the STAMPED golden; this reads both from that test's testdata and runs the
// attribution lint over them, under a roster of the same shape (login example-operator,
// neutral name operator), so the two halves cannot drift apart unnoticed.

const stampFixtureDir = "../tools/desk/cmd/deskevidence/testdata/principal-stamp"

func stampLintProblems(t *testing.T, fixture string) []string {
	t.Helper()
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:      "example-operator:4001",
		scanEnvTrustedLogins:   "example-operator:4001,shared-agent:4002",
		scanEnvTrustedBotSlugs: "verifier=assay-verifier-app:300000005",
		scanEnvHumanLoginMap:   "operator:example-operator",
	})
	b, err := os.ReadFile(filepath.Join(stampFixtureDir, fixture))
	if err != nil {
		t.Fatalf("reading the writer-side fixture: %v", err)
	}
	streams := loadPrincipalFixture(t, "evidence-no-principal")
	path := filepath.Join(streams[0].Dir, "brief-01-no-principal.md")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(streams[0].Root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := principalAttributionProblems(streams2)
	return problems
}

// regression: #2976
// TestStampedGoldenPassesLint: the stamped golden carries no attribution problem.
func TestStampedGoldenPassesLint(t *testing.T) {
	if problems := stampLintProblems(t, "brief-01-stamped.md"); hasProblem(problems, "mp/brief-01") {
		t.Fatalf("the writer-stamped rows must pass the attribution lint; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestUnstampedFixtureFailsLint is the control: the same rows before the stamp are PROBLEMs,
// so the pass above is the stamp's doing, not a fixture the lint never reads.
func TestUnstampedFixtureFailsLint(t *testing.T) {
	problems := stampLintProblems(t, "brief-01-unstamped.md")
	if !hasProblem(problems, "mp/brief-01", "on-behalf-of principal") {
		t.Fatalf("want a no-principal problem on the unstamped fixture; got:\n%s", strings.Join(problems, "\n"))
	}
}
