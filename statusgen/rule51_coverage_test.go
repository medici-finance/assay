package main

// rule51_coverage_test.go — the coverage side of the rule-51 lane (#4823).
// The closure gate (checkWitnesses) and the coverage library read the same
// witness, and must agree on it: a stamp the closure gate credits releases
// coverage too, and every stamp it refuses stays the recorded failure there.
// Without that, `--coverage`, the model auto-flip and the reconcile fold hold a
// brief the closure gate has already credited.

import (
	"strings"
	"testing"
)

// r51Coverage commits a brief whose one Verify row is cmd, then writes the
// witness the stamp builder returns for that commit, and evaluates coverage.
func r51Coverage(t *testing.T, verifyCmd, witnessCmd string, row func(tree string) string) (Coverage, string) {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	verify := strings.TrimSuffix(r51Verify(verifyCmd), "\n")
	writeCoverageBrief(t, s.Dir, "01", "cov", verify, "")
	tree := mustGitInit(t, root)
	writeCoverageBrief(t, s.Dir, "01", "cov", verify, coverageEvidenceTable(row(tree)))
	cov := evaluateCoverage(root, []*Stream{s}, coverageOptions{})
	c, ok := cov["cov/01"]
	if !ok {
		t.Fatalf("no coverage computed for cov/01 (got %+v)", cov)
	}
	return c, tree
}

// TestRule51CoverageCreditsStampedForgeClean: the row the closure gate
// credits is a released pass in coverage, and the claim says why.
func TestRule51CoverageCreditsStampedForgeClean(t *testing.T) {
	const cmd = "statusgen --root . --lint"
	c, tree := r51Coverage(t, cmd, cmd, func(tree string) string {
		return r51Witness("1", cmd, r51Stamp("1", cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, tree, r51Date), tree, r51Date)
	})
	if !c.Released || len(c.Claims) != 1 || c.Claims[0].Result != covPass {
		t.Fatalf("a stamped forge-clean rule-51 witness at %s is not a released pass in coverage: %+v", tree, c)
	}
	if !strings.Contains(c.Claims[0].Reason, "rule-51 lane") {
		t.Errorf("the pass does not say it is rule-51 credited: %q", c.Claims[0].Reason)
	}
}

// TestRule51CoverageRefusals: each stamp the closure gate refuses stays the
// recorded failure in coverage, held.
func TestRule51CoverageRefusals(t *testing.T) {
	const cmd = "statusgen --root . --lint"
	diffCmd := "statusgen --root . --lint --diff-base refs/remotes/origin/main"
	stamped := func(c, forgeState, forgeRC, pin, credClass, cred, treeSuffix, date string) func(string) string {
		return func(tree string) string {
			return r51Witness("1", c, r51Stamp("1", c, forgeState, forgeRC, pin, credClass, cred, tree+treeSuffix, r51Date), tree+treeSuffix, date)
		}
	}
	cases := []struct {
		name, cmd string
		row       func(string) string
	}{
		{"forge red", cmd, stamped(cmd, "fail", "1", r51Pin, "verifier-run-read", r51Cred, "", r51Date)},
		{"forge could-not-run", cmd, stamped(cmd, "could-not-run", "1", r51Pin, "verifier-run-read", r51Cred, "", r51Date)},
		{"unstamped offline red", cmd, func(tree string) string { return r51Witness("1", cmd, "", tree, r51Date) }},
		{"stamp copied to another date", cmd, stamped(cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, "", "2026-11-20")},
		{"dev pin", cmd, stamped(cmd, "pass", "0", "dev", "verifier-run-read", r51Cred, "", r51Date)},
		{"no credential", cmd, stamped(cmd, "pass", "0", r51Pin, "verifier-run-read", "-", "", r51Date)},
		{"another credential class", cmd, stamped(cmd, "pass", "0", r51Pin, "ambient-login", r51Cred, "", r51Date)},
		{"dirty tree", cmd, stamped(cmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, "+dirty", r51Date)},
		{"diff-base row", diffCmd, stamped(diffCmd, "pass", "0", r51Pin, "verifier-run-read", r51Cred, "", r51Date)},
	}
	for _, tc := range cases {
		c, _ := r51Coverage(t, tc.cmd, tc.cmd, tc.row)
		if c.Released || len(c.Claims) != 1 || c.Claims[0].Result != covFail {
			t.Errorf("%s: want a held fail claim, got %+v", tc.name, c)
		}
	}
}
