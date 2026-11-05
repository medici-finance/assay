package main

import (
	"strings"
	"testing"
)

// runnersource_test.go — #5530: the rows of one latest-PASS run may differ in
// the derived source tag at the end of the Runner cell, and only there. A
// container bracket row reads `(forge-identity)` or `(git-config)`; a row the
// verifier recorded itself reads `(model verifier)` or carries no tag. Every
// other difference still refuses.

const rsOBO = " (on-behalf-of human:pat)"

// rsRun lays one row per cell (all at vfRunner @ vfSHA plus the tail) under
// one strict PASS.
func rsRun(tails ...string) string {
	var rows []string
	for i, tl := range tails {
		rows = append(rows, rfRow(string(rune('1'+i)), vfRunner+" @ "+vfSHA+tl))
	}
	return witnessTableFor(rows...) + "\n\n" + vfPass
}

// regression: #5530
func TestFlipSourceTagOneRunner(t *testing.T) {
	cases := []struct {
		name  string
		tails []string
		stamp string // the derived Verified stamp's tail, when accepted
	}{
		{"forge-model", []string{rsOBO + " (forge-identity)", " (model verifier)" + rsOBO}, ""},
		{"forge-none", []string{rsOBO + " (forge-identity)", rsOBO}, ""},
		{"gitcfg-forge", []string{rsOBO + " (git-config)", rsOBO + " (forge-identity)"}, ""},
		{"no-obo-mixed", []string{" (forge-identity)", ""}, ""},
		{"uniform-kept", []string{rsOBO + " (forge-identity)", rsOBO + " (forge-identity)"}, " (forge-identity)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := flipLatestPass(rsRun(c.tails...))
			if err != nil {
				t.Fatalf("one runner refused: %v", err)
			}
			if want := "2026-08-13 " + vfRunner + " @ " + vfSHA + c.stamp; p.Stamp != want || p.Runner != vfRunner {
				t.Fatalf("runner %q stamp %q, want %q", p.Runner, p.Stamp, want)
			}
		})
	}
}

// regression: #5530
func TestFlipSourceTagNearMiss(t *testing.T) {
	at := vfRunner + " @ " + vfSHA
	cases := []struct {
		name  string
		cells []string
		want  string
	}{
		{"other-login", []string{at + rsOBO + " (forge-identity)", "other-runner[bot] @ " + vfSHA + rsOBO}, "runner mismatch"},
		{"implementer", []string{at + rsOBO + " (forge-identity)", "assay-worker-app[bot] @ " + vfSHA + rsOBO + " (model verifier)"}, "runner mismatch"},
		{"other-sha", []string{at + rsOBO + " (forge-identity)", vfRunner + " @ c988d175ab12" + rsOBO}, "sha mismatch"},
		{"other-principal", []string{at + rsOBO + " (forge-identity)", at + " (on-behalf-of human:alex)"}, "runner mismatch"},
		{"principal-dropped", []string{at + rsOBO + " (forge-identity)", at + " (model verifier)"}, "runner mismatch"},
		{"unknown-tag", []string{at + rsOBO + " (forge-identity)", at + rsOBO + " (ci-env)"}, "runner mismatch"},
		{"tag-near-miss", []string{at + rsOBO + " (forge-identity)", at + rsOBO + " (Model Verifier)"}, "runner mismatch"},
		{"tag-padded", []string{at + rsOBO + " (forge-identity)", at + rsOBO + " ( forge-identity)"}, "runner mismatch"},
		{"other-suffix", []string{at + rsOBO + " (forge-identity)", at + rsOBO + " (claude-x)"}, "runner mismatch"},
		{"dirty", []string{at + rsOBO + " (forge-identity)", at + "+dirty" + rsOBO}, "runner mismatch"},
		{"dirty-bare", []string{at + " (forge-identity)", at + "+dirty"}, "runner mismatch"},
		{"no-sha", []string{at + rsOBO + " (forge-identity)", vfRunner + rsOBO}, "sha mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rows []string
			for i, cell := range c.cells {
				rows = append(rows, rfRow(string(rune('1'+i)), cell))
			}
			_, err := flipLatestPass(witnessTableFor(rows...) + "\n\n" + vfPass)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want a refusal naming %q", err, c.want)
			}
		})
	}
}

// regression: #5530
// TestVflipSourceTagMixed: the whole verb flips the issue's shape — one row
// through the bracket, one recorded by the verifier — and writes a stamp that
// names no source; the same rows committed by the implementer still refuse.
func TestVflipSourceTagMixed(t *testing.T) {
	mixed := func() vfOpts {
		o := vfDefaults()
		o.evidence = strings.Replace(o.evidence, vfRunner+" @ "+vfSHA+" |", vfRunner+" @ "+vfSHA+rsOBO+" (forge-identity) |", 1)
		o.evidence = strings.Replace(o.evidence, vfRunner+" @ "+vfSHA+" |", vfRunner+" @ "+vfSHA+" (model verifier)"+rsOBO+" |", 1)
		return o
	}
	root, readme := vfFixture(t, mixed())
	code, out, errOut := runVF(t, root)
	if code != verifyflipExitOK || !strings.Contains(out, "verified-stamp: "+vfStamp+"\n") {
		t.Fatalf("exit = %d out=%q err=%q, want stamp %q", code, out, errOut, vfStamp)
	}
	if after := getFile(t, readme); !strings.Contains(after, "| verified | "+vfStamp+" | "+vfReview+" |") {
		t.Fatalf("README after flip:\n%s", after)
	}
	o := mixed()
	o.author = vfMailWorker
	root, readme = vfFixture(t, o)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
	vfRefuses(t, mixed(), "runner mismatch", "--runner", "assay-worker-app[bot]")
}
