package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// written_test.go — a placeholder exit is recorded only for a member the scan wrote (issue 6601).
//
// The defect: the scanner skipped an inbound issue (an excluded label), the lane still credited
// every batch member with the placeholder exit, the leak check read CLEAN, and the session's later
// `land` for the real disposition was refused as already exited. These drive whole passes against
// the carrier world (carriers_test.go), whose scanner skips the issues in w.skip, over the real
// GitLab and GitHub backends.

const skipped = "medici-finance/assay#602"

// wantParked asserts the skipped member left no exit and is named under EMITTED with its land hint.
func wantParked(t *testing.T, loop *ScanLoop, out string) {
	t.Helper()
	for _, r := range loop.Ledger().Records() {
		if r.ItemID == skipped {
			t.Fatalf("the skipped member was ledgered as %s (%s) though the scan wrote no placeholder for it:\n%s",
				r.Exit, r.Artifact, out)
		}
	}
	if !strings.Contains(out, "EMITTED") || !strings.Contains(out, "scanloop land --item "+skipped) {
		t.Fatalf("the skipped member is not named under EMITTED with its land command:\n%s", out)
	}
	if strings.Contains(out, "dispatch-error") {
		t.Fatalf("a skipped member failed the dispatch:\n%s", out)
	}
	if !strings.Contains(out, "leak check: CLEAN") {
		t.Fatalf("the leak check did not run clean:\n%s", out)
	}
}

// skippedBesideOther: the scanner writes another open issue's placeholder and skips the inbound
// one — the reported shape: a carrier opens, and the inbound member was ledgered against it.
func skippedBesideOther(t *testing.T, kind string) {
	w := newCarrierWorld(t)
	w.serveForge(t, kind)
	w.issues = []int{601}
	w.skip = map[int]bool{602: true}
	loop, out, err := w.passL(t, 602, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	wantFiles(t, w, w.newest(), 601) // the scan's own delta is still carried
	wantParked(t, loop, out)
}

// regression: #6601
func TestUnwritten_GitLabNotLedgered(t *testing.T) { skippedBesideOther(t, "gitlab") }

// regression: #6601
func TestUnwritten_GitHubNotLedgered(t *testing.T) { skippedBesideOther(t, "github") }

// Everything the scan wrote is on an open sealed carrier: no new carrier, and the skipped member
// must not be credited with that carrier's number.
// regression: #6601
func TestUnwritten_AllCarried(t *testing.T) {
	w := newCarrierWorld(t)
	w.serveForge(t, "gitlab")
	w.noCoalesce = true
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if out, err := w.pass(t, 601, t0); err != nil {
		t.Fatalf("seed pass errored: %v\n%s", err, out)
	}
	sealed := w.newest()
	w.skip = map[int]bool{602: true}
	loop, out, err := w.passL(t, 602, t0.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	if w.newest() != sealed {
		t.Fatalf("an all-carried pass opened carrier #%d:\n%s", w.newest(), out)
	}
	wantParked(t, loop, out)
}

// A member whose placeholder the scan wrote but another open carrier already carries is excluded
// from this pass's own carrier; its exit must name the carrier that holds it, not the new one.
// regression: #6601
func TestCredit_CarriedArtifact(t *testing.T) {
	w := newCarrierWorld(t)
	w.serveForge(t, "gitlab")
	w.noCoalesce = true
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if out, err := w.pass(t, 601, t0); err != nil {
		t.Fatalf("seed pass errored: %v\n%s", err, out)
	}
	sealed := w.newest()
	w.issues = append(w.issues, 603) // another open issue the scan writes this pass
	loop, out, err := w.passL(t, 601, t0.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	wantFiles(t, w, w.newest(), 603)
	recs := loop.Ledger().Records()
	want := fmt.Sprintf("medici-finance/assay#%d", sealed)
	if len(recs) != 1 || recs[0].ItemID != "medici-finance/assay#601" || recs[0].Artifact != want {
		t.Fatalf("ledger = %+v, want #601's exit to name carrier %s, which carries its placeholder:\n%s", recs, want, out)
	}
}

// The scan wrote nothing at all: no commit, no carrier, no dispatch error, no exit.
// regression: #6601
func TestUnwritten_ZeroDeltaNoPR(t *testing.T) {
	w := newCarrierWorld(t)
	w.serveForge(t, "github")
	w.skip = map[int]bool{602: true}
	loop, out, err := w.passL(t, 602, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	if n := w.newest(); n != 0 {
		t.Fatalf("a zero-delta scan opened carrier #%d:\n%s", n, out)
	}
	wantParked(t, loop, out)
}

// A GitLab nested-group slug stems with its LAST segment, as the scanner names the file; the
// first-slash cut read `sub/project` and missed an existing placeholder, so an update was
// misclassified as a new issue and sent through the scan, which then skipped it.
// regression: #6601
func TestHasPlaceholder_NestedGroup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project-issue-12.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		repo string
		want bool
	}{
		{"group/sub/project", true},
		{"group/project", true},
		{"group/sub/other", false},
	} {
		got, err := HasPlaceholder(root, c.repo, 12)
		if err != nil || got != c.want {
			t.Fatalf("HasPlaceholder(%s, 12) = %v, %v; want %v", c.repo, got, err, c.want)
		}
	}
}

// creditMembers credits by the scanner's file name only, and only for changed paths.
// regression: #6601
func TestCreditMembers_ByName(t *testing.T) {
	changed := parseStatusZ("?? " + deskkit.ScanDir + "/project-issue-3.md\x00?? " + deskkit.ScanDir + "/issue-4.md\x00")
	credited, unwritten := creditMembers([]string{"group/sub/project#3", "group/sub/project#5", "o/home#4", "not-an-id"}, changed)
	if len(credited) != 2 || credited["group/sub/project#3"] == "" || credited["o/home#4"] == "" {
		t.Fatalf("credited = %v", credited)
	}
	if strings.Join(unwritten, ",") != "group/sub/project#5,not-an-id" {
		t.Fatalf("unwritten = %v", unwritten)
	}
}
