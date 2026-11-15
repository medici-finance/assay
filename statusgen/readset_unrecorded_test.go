package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// recordedReadSets makes coverageOptionsFor honour read sets for the rest of
// t, as a build whose recorder shipped would (readSetRecorderShipped): the
// recorded-path tests drive the real callers through this one seam.
func recordedReadSets(t *testing.T) {
	t.Helper()
	was := honourReadSets
	honourReadSets = true
	t.Cleanup(func() { honourReadSets = was })
}

// regression: #2026
// TestCoverageTokenUnrecorded — build-less-brittle/15, review S20: with no
// read-set recorder in this build, autoflip's dry run, the reconcile fold and
// --coverage each read a row carrying a read-set token exactly as the same
// row and file with no token, for each landing and later change the review
// drove, and a held row names the missing recorder. Every changed case is
// held, as the row without a token is, and no read-set observation (file,
// introduction walk, go floor) is wired into the callers at all.
func TestCoverageTokenUnrecorded(t *testing.T) {
	// No observation is wired: no read-set file, introduction history walk
	// or go floor runs on any caller while the switch is off.
	if opts := coverageOptionsFor(t.TempDir(), nil); opts.readSets != nil {
		t.Fatalf("with no recorder shipped, coverageOptionsFor wires the read-set observation into every caller")
	}
	// The unrelated file exists at the witness, so a later edit leaves every
	// directory listing as it was.
	other := []string{".claude-plugin/marketplace.json", "{}\n"}
	unrelated := func(t *testing.T, root string) {
		mustWriteFile(t, root, ".claude-plugin/marketplace.json", "{\"v\":2}\n")
	}
	cases := []struct {
		name   string
		o      afRS
		change func(t *testing.T, root string)
	}{
		{"verifier, listing, unrelated change", afRS{recorded: []string{"a.go", "b.go", "go.mod", "list ."}, extra: other}, unrelated},
		{"verifier, no listing, compiled file added", afRS{recorded: []string{"a.go", "b.go", "go.mod"}},
			func(t *testing.T, root string) {
				mustWriteFile(t, root, "c.go", "package af\n\nfunc C() int { return 3 }\n")
			}},
		{"go row, no compile step, code edited", afRS{noGoRun: true, recorded: []string{"a.go", "go.mod"}},
			func(t *testing.T, root string) {
				mustWriteFile(t, root, "b.go", "package af\n\nfunc B() int { return 2 }\n")
			}},
		{"non-verifier landing, unrelated change", afRS{mail: vfMailWorker, recorded: []string{"a.go", "b.go", "go.mod"}, extra: other}, unrelated},
		{"verifier, no later change", afRS{recorded: []string{"a.go", "b.go", "go.mod"}}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(noToken bool) (modelFlipResult, bool, string) {
				o := c.o
				o.noToken = noToken
				root := afReadSetBuild(t, o)
				if c.change != nil {
					c.change(t, root)
					mustGitCommitAll(t, root, "fixture: later change")
				}
				return coverageAF01(t, root)
			}
			flip, folded, line := run(false)
			tflip, tfolded, tline := run(true)
			verdict := func(l string) string { f := strings.Fields(l); return strings.Join(f[:min(2, len(f))], " ") }
			if flip.Outcome != tflip.Outcome || folded != tfolded || verdict(line) != verdict(tline) {
				t.Fatalf("token row: autoflip %v (%s), fold %v, %q; the row without it: %v (%s), fold %v, %q",
					flip.Outcome, flip.Reason, folded, line, tflip.Outcome, tflip.Reason, tfolded, tline)
			}
			if c.change == nil {
				return
			}
			if flip.Outcome == flipDone || folded || verdict(line) != "af/01 held" {
				t.Errorf("want af/01 held through all three callers; got %v (%s), fold %v, %q", flip.Outcome, flip.Reason, folded, line)
			}
			if !strings.Contains(line, evaluation.ReasonNoRecorder) {
				t.Errorf("--coverage %q does not name the missing recorder", line)
			}
		})
	}
}
