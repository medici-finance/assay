package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadVFStreams copies the verifier-floor close fixtures (testdata/verifyfloor)
// into a temp root. They live apart from testdata/verifygate because the
// verify-issues selection tests pin that tree's exact row set.
func loadVFStreams(t *testing.T) (string, []*Stream) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifyfloor")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

func vfRow(t *testing.T, root, num string) *Brief {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range streams {
		if st.Name == "vf" {
			if row := findRow(st, num); row != nil {
				return row
			}
		}
	}
	t.Fatalf("no vf/%s row", num)
	return nil
}

// TestCloseVerifyRefusesBelowFloor pins the two-stamp model (#1170): a human
// done close of a gate:human brief whose Verified cell — or the cell the
// implemented→done path would stamp from Evidence — names a runner below the
// methodology/19 verifier floor is REFUSED with no write. Before the fix every
// one of these flipped, and the flip landed on main red under the floor lint.
func TestCloseVerifyRefusesBelowFloor(t *testing.T) {
	tests := []struct {
		name  string
		brief string
		want  []string // substrings the refusal must carry
	}{
		{
			"verified path: Verified cell names a below-floor runner",
			"vf/01",
			[]string{"verifier floor", "sonnet-verifier", "two-stamp", "re-verify", "close the card again"},
		},
		{
			"implemented path: the cell it would stamp from Evidence is below the floor",
			"vf/03",
			[]string{"verifier floor", "sonnet-verifier", "two-stamp", "re-verify"},
		},
		{
			"verified path: cell clears but Evidence rows were only run below the floor",
			"vf/04",
			[]string{"verifier floor", "sonnet-verifier", "two-stamp", "Evidence"},
		},
	}
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := loadVFStreams(t)
			readme := filepath.Join(root, "docs/streams/vf/README.md")
			before, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			err = closeVerify(root, tc.brief, now)
			if err == nil {
				t.Fatalf("close-verify %s must REFUSE a below-floor Verified cell (two-stamp model); it flipped", tc.brief)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("refusal must name %q (runner + floor + remedy); got %q", w, err)
				}
			}
			after, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("a refused close-verify %s must not write the README", tc.brief)
			}
		})
	}
}

// TestCloseVerifyFlipsAfterFloorRestamp is the other half of the model: once a
// floor-tier re-verify stamp leads the Verified cell and its rows are in the
// Evidence, the same human close flips verified → done.
func TestCloseVerifyFlipsAfterFloorRestamp(t *testing.T) {
	root, _ := loadVFStreams(t)
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	if err := closeVerify(root, "vf/02", now); err != nil {
		t.Fatalf("close-verify vf/02 (floor-tier re-stamp landed) must flip: %v", err)
	}
	row := vfRow(t, root, "02")
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	if row.Reviewed != "2026-07-12 human:reviewer" {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-12 human:reviewer")
	}
	// The re-stamped cell is left exactly as the re-verifier wrote it.
	if row.Verified != "2026-07-10 opus-verifier; prior 2026-07-08 sonnet-verifier" {
		t.Errorf("verified cell rewritten to %q", row.Verified)
	}
	// And the flipped row is one the SAME binary's --lint accepts: the whole
	// point is that a flip this accepts never lands red.
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := checkBriefFiles(streams, streams)
	for _, p := range problems {
		if strings.Contains(p, "brief-02-") && strings.Contains(p, "verifier floor") {
			t.Errorf("the flipped vf/02 row fails the floor lint the close was meant to pre-empt: %s", p)
		}
	}
	// Siblings untouched.
	if r := vfRow(t, root, "01"); r.Status != "verified" {
		t.Errorf("vf/01 status changed to %q", r.Status)
	}
}

// TestCloseVerifyFloorRefusalKeyword pins the phrase the close workflow keys
// its reopen-and-comment branch on. Rewording it silently downgrades a floor
// refusal to the generic "left closed" path.
func TestCloseVerifyFloorRefusalKeyword(t *testing.T) {
	root, _ := loadVFStreams(t)
	err := closeVerify(root, "vf/01", time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "verifier floor") {
		t.Fatalf("floor refusal must carry the literal phrase \"verifier floor\"; got %v", err)
	}
}
