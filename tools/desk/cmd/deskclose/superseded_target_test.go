package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// superseded_target_test.go — an item reference is accepted only WHOLE, and is echoed
// only as the re-rendered parse, never as the text it was read from.
//
// A proposal marker's Superseded-By value and a disposition record's Evidence are text
// another identity wrote. The receipts, dry-run lines and audit detail that echo a target
// are tab-separated and land in an operator's terminal, so a value carrying a control
// sequence or a separator ahead of a well-formed `owner/repo#N` must be refused as a
// reference — not accepted on its tail and then printed.

// junkPrefixes are values that END in a valid reference but carry something before it.
var junkPrefixes = map[string]string{
	"ESC CSI":      "\x1b[2J",
	"bare ESC":     "\x1b]0;x\x07",
	"TAB":          "lead\t",
	"CR":           "lead\r",
	"leading text": "see ",
}

// junkBytes are the fragments none of the prefixes above may leave in any output.
var junkBytes = []string{"\x1b", "\r", "lead", "see "}

func assertNoJunk(t *testing.T, where, s string) {
	t.Helper()
	for _, j := range junkBytes {
		if strings.Contains(s, j) {
			t.Fatalf("%s echoed %q from the raw target:\n%q", where, j, s)
		}
	}
}

// regression: class superseded-target-echo
func TestRefParseIsAnchored(t *testing.T) {
	ref := testRepo + "#40"
	for name, junk := range junkPrefixes {
		t.Run(name, func(t *testing.T) {
			if got, ok := normalizeItemRef(testRepo, junk+ref); ok {
				t.Fatalf("normalizeItemRef accepted %q as %q", junk+ref, got)
			}
			if refsAgree(junk+ref, ref) {
				t.Fatalf("refsAgree read %q as naming %s", junk+ref, ref)
			}
		})
	}
	// Positive controls: every documented spelling still parses and agrees.
	for _, good := range []string{"40", "#40", ref, " " + ref + " ", "https://github.com/" + testRepo + "/pull/40"} {
		if !refsAgree(good, ref) {
			t.Fatalf("refsAgree must read %q as naming %s", good, ref)
		}
		if got, ok := normalizeItemRef(testRepo, good); !ok || got != ref {
			t.Fatalf("normalizeItemRef(%q) = %q, %v; want %s", good, got, ok, ref)
		}
	}
}

// regression: class superseded-target-echo
func TestProposalJunkTarget(t *testing.T) {
	for name, junk := range junkPrefixes {
		for _, half := range []string{"confirm", "dispute"} {
			t.Run(name+"/"+half, func(t *testing.T) {
				s, rul := prWorld(t)
				s.plantProposal(testRepo, 90, workerLogin, junk+testRepo+"#40")
				args := []string{modeSuperseded, "-R", testRepo, "90", "--by", testRepo + "#40",
					"--rulings", rul, "--dry-run"}
				if half == "dispute" {
					args = append(args, "--dispute", "not the same scope")
				}
				code, out := execCLI(args...)
				err := execErr(args...)
				if code != deskkit.ExitRefused || err == nil || !deskkit.IsRefused(err) {
					t.Fatalf("a proposal target with leading junk must be refused; exit %d, err %v\n%q", code, err, out)
				}
				assertNoJunk(t, "stdout", out)
				assertNoJunk(t, "the refusal", err.Error())
				if len(s.writes()) != 0 {
					t.Fatalf("a refused proposal wrote: %v", s.writes())
				}
			})
		}
	}
}

// The receipts render the PARSED target: a well-formed proposal is echoed in its
// canonical repository-qualified form whatever spelling the marker used.
//
// regression: class superseded-target-echo
func TestProposalTargetRerendered(t *testing.T) {
	s, rul := prWorld(t)
	s.plantProposal(testRepo, 90, workerLogin, "#40")
	code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40",
		"--rulings", rul, "--dry-run", "--dispute", "x")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "target="+testRepo+"#40\n") {
		t.Fatalf("the dry-run receipt must carry the canonical target:\n%s", out)
	}
}

// The SECOND site of the class: the disposition record's Evidence is compared through
// the same reference reader. Evidence with junk ahead of the ref must not agree with the
// declared target, so the close is refused rather than run on a tail match.
//
// regression: class superseded-target-echo
func TestEvidenceJunkDisagrees(t *testing.T) {
	for name, junk := range junkPrefixes {
		t.Run(name, func(t *testing.T) {
			s, rul := prWorld(t)
			// JSON-encode the record: dispJSON's %q spells ESC as a Go escape, which is
			// not JSON, and an unparseable record is could-not-check before any comparison.
			ev, _ := json.Marshal(junk + testRepo + "#40")
			s.disps[testRepo+"#90"] = strings.Replace(dispJSON(dispCheckedFailed, verdictSuperseded, "EV"),
				`"EV"`, string(ev), 1)
			s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
			code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40",
				"--rulings", rul, "--dry-run")
			if code != deskkit.ExitRefused {
				t.Fatalf("evidence %q must not agree with %s#40; exit %d\n%q", junk+testRepo+"#40", testRepo, code, out)
			}
		})
	}
}
