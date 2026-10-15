package deskkit

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPerfRecordCarriesNoIdentifiers is the SPOF row: a wrapped run whose argv carries a repo
// slug, a PR number, a branch name and an absolute path, with DESK_SESSION and a token-shaped
// variable in the environment, writes a record whose bytes contain none of the six sentinels.
// regression: F-dt23-perf-record
func TestPerfRecordCarriesNoIdentifiers(t *testing.T) {
	state := perfFixture(t)
	t.Setenv("DESK_LOOP", "worker-desk")
	// Built at run time so no token-shaped literal sits in the source.
	tokenish := "ghp_" + strings.Repeat("Xq7", 12)
	session := "sess-sentinel-" + strconv.Itoa(424242)
	t.Setenv("DESK_SESSION", session)
	t.Setenv("GH_TOKEN", tokenish)

	repo := "example-org/example-repo"
	pr := "987654321"
	branch := "feat/sentinel-branch-xq"
	abs := filepath.Join(t.TempDir(), "sentinel-path-xq")
	args := []string{"create", "--repo", repo, "--pr", pr, "--branch", branch, "--body-file", abs}

	PerfRun("deskpr", []string{"create", "update"}, args, func([]string) int { return 0 })
	// And the same argv where the subcommand slot itself is a sentinel: the verb maps to "other".
	PerfRun("deskpr", []string{"create", "update"}, []string{repo, pr}, func([]string) int { return 0 })
	PerfStep("deskboot", branch, 0, 0)

	recs, raw := perfLines(t, state)
	if len(recs) != 3 {
		t.Fatalf("want 3 records, got %d", len(recs))
	}
	for _, sentinel := range []string{repo, pr, branch, abs, session, tokenish} {
		if strings.Contains(string(raw), sentinel) {
			t.Errorf("the perf record carries the planted identifier %q:\n%s", sentinel, raw)
		}
	}
	// Belt and braces: no planted fragment either (the org half of the slug, the path's base).
	for _, frag := range []string{"example-org", "sentinel", "Xq7Xq7"} {
		if strings.Contains(string(raw), frag) {
			t.Errorf("the perf record carries a fragment %q of a planted identifier", frag)
		}
	}
}
