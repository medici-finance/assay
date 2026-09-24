package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadCHRoot copies the close-verify contradiction fixtures
// (testdata/closeheld) into a temp root. They live apart from
// testdata/verifygate because the verify-issues selection tests pin that
// tree's exact row set, and apart from testdata/verifyfloor because those
// fixtures pin the verifier-floor reads only.
func loadCHRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/closeheld")); err != nil {
		t.Fatal(err)
	}
	return root
}

func chRow(t *testing.T, root, num string) *Brief {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range streams {
		if st.Name == "ch" {
			if row := findRow(st, num); row != nil {
				return row
			}
		}
	}
	t.Fatalf("no ch/%s row", num)
	return nil
}

// TestCloseVerifyVerifiedRefusesContradiction pins the verified → done close
// path to the same Evidence read the implemented → done path makes: a
// `verified` brief whose Evidence carries a **VERIFY: PASS** contradicted by an
// un-routed HELD or could-not-check row, or whose most recent verdict is a
// FAIL, is REFUSED with no write. Before the fix the verified path ran only the
// verifier-floor read, and every one of these flipped to done.
func TestCloseVerifyVerifiedRefusesContradiction(t *testing.T) {
	tests := []struct {
		name  string
		brief string
		want  []string // substrings the refusal must carry
	}{
		{
			"verified path: PASS contradicted by an un-routed HELD row",
			"ch/01",
			[]string{"ch/01", "**VERIFY: PASS**", "HELD", "not a flip signal"},
		},
		{
			"verified path: most recent verdict is VERIFY: FAIL",
			"ch/02",
			[]string{"ch/02", "VERIFY: FAIL", "not a flip signal", "verified"},
		},
		{
			"verified path: PASS contradicted by an un-routed could-not-check row",
			"ch/07",
			[]string{"ch/07", "**VERIFY: PASS**", "could-not-check", "not a flip signal"},
		},
	}
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := loadCHRoot(t)
			readme := filepath.Join(root, "docs/streams/ch/README.md")
			before, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			err = closeVerify(root, tc.brief, now)
			if err == nil {
				t.Fatalf("close-verify %s must REFUSE a verified brief whose Evidence contradicts the flip; it flipped", tc.brief)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("refusal must name %q; got %q", w, err)
				}
			}
			after, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("a refused close-verify %s must not write the README", tc.brief)
			}
			if r := chRow(t, root, strings.TrimPrefix(tc.brief, "ch/")); r.Status != "verified" {
				t.Errorf("%s status changed to %q on a refused close", tc.brief, r.Status)
			}
		})
	}
}

// TestCloseVerifyHeldRefusalSameTextBothPaths pins "refusing identically": the
// verified-path HELD refusal and the implemented-path HELD refusal are the
// same sentence, differing only in the brief id and the quoted row.
func TestCloseVerifyHeldRefusalSameTextBothPaths(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	errV := closeVerify(loadCHRoot(t), "ch/01", now)
	errI := closeVerify(loadCHRoot(t), "ch/04", now)
	if errV == nil || errI == nil {
		t.Fatalf("both HELD fixtures must refuse: verified=%v implemented=%v", errV, errI)
	}
	norm := func(s, id string) string { return strings.ReplaceAll(s, id, "<id>") }
	if norm(errV.Error(), "ch/01") != norm(errI.Error(), "ch/04") {
		t.Errorf("verified and implemented HELD refusals differ:\n verified:    %s\n implemented: %s", errV, errI)
	}
}

// TestCloseVerifyVerifiedCleanStillCloses is the control: a verified brief with
// a clean record, one whose hold is genuinely routed to a follow-up, and one
// whose earlier FAIL was superseded by a later PASS all still close to done.
func TestCloseVerifyVerifiedCleanStillCloses(t *testing.T) {
	for _, brief := range []string{"ch/03", "ch/05", "ch/06"} {
		t.Run(brief, func(t *testing.T) {
			root := loadCHRoot(t)
			now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
			if err := closeVerify(root, brief, now); err != nil {
				t.Fatalf("close-verify %s must flip: %v", brief, err)
			}
			row := chRow(t, root, strings.TrimPrefix(brief, "ch/"))
			if row.Status != "done" {
				t.Errorf("status = %q, want done", row.Status)
			}
			if row.Reviewed != "2026-07-12 human:reviewer" {
				t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-12 human:reviewer")
			}
			if row.Verified != "2026-07-10 opus-verifier" {
				t.Errorf("verified cell rewritten to %q", row.Verified)
			}
		})
	}
}
