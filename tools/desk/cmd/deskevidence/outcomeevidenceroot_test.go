package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A cross-repository outcome names an Evidence commit that landed through the
// forge after the tracking clone was cut. The read root fetches it before
// either binding reads it; a same-repository root is never fetched into, and a
// commit the landed branches do not hold stays could-not-check.
// regression: F-crossrepo-outcome-evidence
func TestOutcomeEvidenceInRoot(t *testing.T) {
	forge := crossLandRepo(t, "example-org/tracker", map[string]string{"a.txt": "admitted\n"})
	base := crossLandGit(t, forge, "rev-parse", "HEAD")
	crossLandGit(t, forge, "update-ref", "refs/heads/main", base)
	// The tracking clone is cut before anything lands, as admission cuts it.
	seed := filepath.Join(t.TempDir(), "seed")
	crossLandGit(t, forge, "clone", "-q", "--no-hardlinks", forge, seed)
	commit := func(branch, file string) string {
		crossLandGit(t, forge, "checkout", "-q", "-B", branch, base)
		if err := os.WriteFile(filepath.Join(forge, file), []byte(file+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		crossLandGit(t, forge, "add", file)
		crossLandGit(t, forge, "commit", "-q", "-m", file)
		return crossLandGit(t, forge, "rev-parse", "HEAD")
	}
	evidence := commit("verify", "evidence.txt")
	stray := commit("unlanded", "stray.txt")
	crossLandGit(t, forge, "checkout", "-q", "--detach", base)

	for _, tc := range []struct {
		name, sha  string
		receipt    deskkit.VerifierReceipt
		fetchFails bool
		fetch      bool
		code       int
	}{
		{name: "same-repo", sha: evidence, receipt: deskkit.VerifierReceipt{}},
		{name: "present", sha: base, receipt: crossReceipt},
		{name: "landed", sha: evidence, receipt: crossReceipt, fetch: true},
		{name: "unlanded", sha: stray, receipt: crossReceipt, fetch: true, code: deskkit.ExitUnverifiable},
		{name: "fetch-fails", sha: evidence, receipt: crossReceipt, fetchFails: true, fetch: true, code: deskkit.ExitUnverifiable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "tracking")
			crossLandGit(t, seed, "clone", "-q", "--no-hardlinks", seed, root)
			crossLandGit(t, root, "checkout", "-q", "--detach", base)
			var calls []string
			if tc.fetchFails {
				old := landedFetchFn
				landedFetchFn = func(string, string, []string) error {
					calls = append(calls, "fetch")
					return errors.New("fixture forge unreachable")
				}
				t.Cleanup(func() { landedFetchFn = old })
			} else {
				landedForgeFetch(t, forge, &calls)
			}
			err := outcomeEvidenceInRoot(root, tc.receipt, crossRepo, "verify", tc.sha)
			if deskkit.ExitCodeOf(err) != tc.code || (tc.code == 0) != (err == nil) {
				t.Fatalf("want exit %d, got %v", tc.code, err)
			}
			if (len(calls) > 0) != tc.fetch {
				t.Fatalf("fetch calls %v, want fetch=%v", calls, tc.fetch)
			}
			if tc.name == "landed" && !localCommit(root, evidence) {
				t.Fatal("landed Evidence commit still unreadable in the read root")
			}
			if tc.name == "same-repo" && localCommit(root, evidence) {
				t.Fatal("same-repository root was fetched into")
			}
			if crossLandGit(t, root, "rev-parse", "HEAD") != base {
				t.Fatal("read root HEAD moved")
			}
		})
	}
}
