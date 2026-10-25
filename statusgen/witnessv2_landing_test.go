package main

// witnessv2_landing_test.go — the landing commit's parents and message come
// from the commit object itself; the first-parent log only says which commit
// added which record (statusgen/36, review SEC-1).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #4409 review SEC-1 — a landing commit is judged by its own
// commit object: a plain single-parent commit on main lands in no recognised
// form, offline, whatever its message holds.
func TestWitnessLandingReadFromCommitObject(t *testing.T) {
	f := newGitV2Fixture(t).build()
	root := f.Root
	// Main goes back to the base; the records and Evidence land again as one
	// plain commit with no change-request landing of its own.
	gv2Git(t, root, "reset", "-q", "--hard", f.Base)
	gv2Git(t, root, "checkout", "-q", "-b", "fixture/side", f.Base)
	gv2Write(t, root, gv2BriefRel(), "side\n")
	gv2Git(t, root, "commit", "-q", "-am", "fixture: side")
	side := gv2Git(t, root, "rev-parse", "HEAD")
	gv2Git(t, root, "checkout", "-q", "main")
	gv2Git(t, root, "checkout", f.Feature, "--", ".")
	msg := "Evidence: witness rows\x1f\x1e" + f.Base + "\x1f" + side + " " + f.Feature + "\x1fMerge pull request #" + gv2CR + " from example-org/fixture-branch\n"
	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	gv2Git(t, root, "commit", "-q", "--cleanup=verbatim", "-F", msgFile)
	landing := gv2Git(t, root, "rev-parse", "HEAD")

	c := f.coverage()
	for _, id := range []string{"1", "2"} {
		cl := claimFor(t, c, id)
		if cl.Released || cl.Result == covPass {
			t.Fatalf("row %s: a plain commit's message must not make it a landing: released %s: %s", id, cl.Result, cl.Reason)
		}
		if g := evaluation.WitnessGap(cl.Reason); g != evaluation.GapCRUnverified {
			t.Errorf("row %s: gap %q, want %s (%s)", id, g, evaluation.GapCRUnverified, cl.Reason)
		}
		if !strings.Contains(cl.Reason, landing) {
			t.Errorf("row %s: the reason must name the commit that added the record, %s: %s", id, landing, cl.Reason)
		}
	}
}

// regression: #4409 review SEC-1 — the additions log is read as commit ids and
// record paths only; any other field fails the whole read.
func TestParseWitnessAdditions(t *testing.T) {
	c1 := strings.Repeat("a", 40)
	c2 := strings.Repeat("b", 64)
	rec := evaluation.WitnessRecordDir + "s/01/" + strings.Repeat("c", 64) + ".json"
	rec2 := evaluation.WitnessRecordDir + "s/01/" + strings.Repeat("d", 64) + ".json"
	got, ok := parseWitnessAdditions([]byte(c1 + "\x00\n" + rec + "\x00" + c2 + "\x00\n" + rec2 + "\x00" + rec + "\x00"))
	want := []evaluation.WitnessAddition{{Path: rec, Commit: c1}, {Path: rec2, Commit: c2}, {Path: rec, Commit: c2}}
	if !ok || len(got) != len(want) {
		t.Fatalf("got %+v ok=%v, want %+v", got, ok, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("addition %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	for name, out := range map[string]string{
		"a short commit id":           strings.Repeat("a", 39) + "\x00\n" + rec + "\x00",
		"an uppercase commit id":      strings.ToUpper(c1) + "\x00\n" + rec + "\x00",
		"a path before any commit":    rec + "\x00" + c1 + "\x00",
		"a path outside the register": c1 + "\x00\nREADME.md\x00",
	} {
		if _, ok := parseWitnessAdditions([]byte(out)); ok {
			t.Errorf("%s: the read must fail", name)
		}
	}
	// A path field is taken whole, never split into a second commit.
	got, ok = parseWitnessAdditions([]byte(c1 + "\x00\n" + rec + "\x1f" + c2 + "\x00"))
	if !ok || len(got) != 1 || got[0].Commit != c1 {
		t.Errorf("a path field: got %+v ok=%v, want one addition by %s", got, ok, c1)
	}
}
