package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const placeholderRepo = "medici-finance/assay"

// printedProgress is the argv deskdispatch's NOT LAUNCHED line spells for a branchless
// (detached review/verify) claim: `progress <key> --repo <repo> --branch - --token-file <f>`,
// with no --owner, so the holder comes from DESK_SESSION exactly as at acquire.
func printedProgress(t *testing.T, key string, branchArgs ...string) []string {
	args := []string{"progress", key, "--repo", placeholderRepo}
	args = append(args, branchArgs...)
	return append(args, "--token-file", filepath.Join(t.TempDir(), "token"))
}

// regression: #6896
func TestPrintedProgressAdvances(t *testing.T) {
	for _, branch := range [][]string{{"--branch", "-"}, {"--branch=-"}} {
		t.Run(strings.Join(branch, " "), func(t *testing.T) {
			t.Setenv("DESK_SESSION", "verifier-1")
			f := newStore()
			run, _, se := harness(t, f)
			if rc := run("acquire", "assay--issue-77", "--repo", placeholderRepo); rc != exitOK {
				t.Fatalf("acquire rc = %d: %s", rc, se.String())
			}
			if rc := run(printedProgress(t, "assay--issue-77", branch...)...); rc != exitOK {
				t.Fatalf("printed progress rc = %d, want 0: %s", rc, se.String())
			}
			msg := f.claims["assay--issue-77"].msg
			if fieldOf(msg, "state") != "dispatched" || fieldOf(msg, "branch") != "-" || fieldOf(msg, "owner") != "verifier-1" {
				t.Fatalf("claim not advanced as printed: %s", msg)
			}
		})
	}
}

// The same printed command for a claim acquired through a work reference: the selected
// subject resolves against the named repository and survives the advance.
// regression: #6896
func TestPrintedWorkRefProgress(t *testing.T) {
	catalog, peers := claimWorkFixture(t)
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(repo string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		if repo != placeholderRepo {
			return deskkit.WorkCatalog{}, nil, false, deskkit.Unverifiable("source repository has no published owning root", nil)
		}
		return catalog, peers, true, nil
	}
	for _, branch := range [][]string{{"--branch", "-"}, {"--branch=-"}} {
		t.Run(strings.Join(branch, " "), func(t *testing.T) {
			t.Setenv("DESK_SESSION", "verifier-1")
			f := newStore()
			run, _, se := harness(t, f)
			if rc := run("acquire", "stream:b-08@2", "--repo", placeholderRepo); rc != exitOK {
				t.Fatalf("acquire rc = %d: %s", rc, se.String())
			}
			if rc := run(printedProgress(t, "stream:b-08@2", branch...)...); rc != exitOK {
				t.Fatalf("printed progress rc = %d, want 0: %s", rc, se.String())
			}
			msg := f.claims["repo--stream--08"].msg
			subject, present, err := deskkit.DecodeClaimSubject(msg)
			if fieldOf(msg, "state") != "dispatched" || fieldOf(msg, "branch") != "-" || err != nil || !present || subject != catalog.Records[0].Subject {
				t.Fatalf("work claim not advanced with its subject: %v %s", err, msg)
			}
		})
	}
}

// A value flag never consumes the next flag as its value. Before #6896 `--repo --branch -`
// made "--branch" the repository and refused "unknown flag -", and `--repo --branch=-` named a
// repository "--branch=-" whose work source has no owning root. Both now name the flag whose
// value is missing, and nothing is read or written.
// regression: #6896
func TestValueFlagNeverSwallowed(t *testing.T) {
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		t.Fatal("work source read for an argv that names no repository")
		return deskkit.WorkCatalog{}, nil, false, nil
	}
	cases := []struct {
		args []string
		flag string
	}{
		{[]string{"progress", "assay--issue-77", "--repo", "--branch", "-"}, "--repo"},
		{[]string{"progress", "stream:b-08@2", "--repo", "--branch=-"}, "--repo"},
		{[]string{"progress", "assay--issue-77", "--repo", placeholderRepo, "--branch"}, "--branch"},
		{[]string{"progress", "assay--issue-77", "--repo", placeholderRepo, "--branch", ""}, "--branch"},
		{[]string{"progress", "assay--issue-77", "--repo", placeholderRepo, "--branch="}, "--branch"},
		{[]string{"progress", "assay--issue-77", "--repo", placeholderRepo, "--branch", "--owner", "x"}, "--branch"},
		{[]string{"steal", "assay--issue-77", "--repo", placeholderRepo, "--reason", "--owner", "x"}, "--reason"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			f := newStore()
			f.seedClaim("assay--issue-77", "holder", "claimed", "", 0)
			before := f.claims["assay--issue-77"]
			run, _, se := harness(t, f)
			if rc := run(c.args...); rc != exitRefused {
				t.Fatalf("rc = %d, want 5: %s", rc, se.String())
			}
			if want := "refused: " + c.flag + " requires a value"; !strings.Contains(se.String(), want) {
				t.Fatalf("stderr does not say %q: %s", want, se.String())
			}
			if f.claims["assay--issue-77"] != before {
				t.Fatal("a refused argv changed the claim")
			}
		})
	}
}
