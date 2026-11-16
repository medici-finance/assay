package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// junkURLs are web URLs whose number and project path name the same item, with text that
// is not a host name or a path segment planted in the host or in the owner segment. The URL
// arm captured the host as "anything up to the first slash" and split the path with no
// charset check, so each of these used to parse as the reference.
var junkURLs = map[string]string{
	"url host mention": "https://@example-user **x** y/" + testRepo + "/pull/40",
	"url host TAB":     "https://lead\tx/" + testRepo + "/pull/40",
	"url host CSI":     "https://x\x1b[2J/" + testRepo + "/pull/40",
	"url path space":   "https://github.com/see " + testRepo + "/pull/40",
	"url path TAB":     "https://github.com/lead\t" + testRepo + "/pull/40",
	"url path CSI":     "https://github.com/\x1b[2J" + testRepo + "/pull/40",
	"url path md":      "https://github.com/**x**" + testRepo + "/pull/40",
}

// junkRefs is every planted value: each junk prefix ahead of the short form, and each
// junk URL.
func junkRefs() map[string]string {
	out := map[string]string{}
	for name, junk := range junkPrefixes {
		out[name] = junk + testRepo + "#40"
	}
	for name, u := range junkURLs {
		out[name] = u
	}
	return out
}

// junkBytes are the fragments none of the planted values may leave in any output.
var junkBytes = []string{"\x1b", "\r", "lead", "see ", "@example-user", "**"}

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
	for name, bad := range junkRefs() {
		t.Run(name, func(t *testing.T) {
			if got, ok := normalizeItemRef(testRepo, bad); ok {
				t.Fatalf("normalizeItemRef accepted %q as %q", bad, got)
			}
			if refsAgree(bad, ref) {
				t.Fatalf("refsAgree read %q as naming %s", bad, ref)
			}
			if got := echoRef(testRepo, bad); got != unparseableRef {
				t.Fatalf("echoRef(%q) = %q; want the fixed placeholder", bad, got)
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
	for name, bad := range junkRefs() {
		for _, half := range []string{"confirm", "dispute"} {
			t.Run(name+"/"+half, func(t *testing.T) {
				s, rul := prWorld(t)
				s.plantProposal(testRepo, 90, workerLogin, bad)
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
	for name, bad := range junkRefs() {
		t.Run(name, func(t *testing.T) {
			s, rul := prWorld(t)
			plantRecord(s, bad, "worker-session")
			s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
			code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40",
				"--rulings", rul, "--dry-run")
			if code != deskkit.ExitRefused {
				t.Fatalf("evidence %q must not agree with %s#40; exit %d\n%q", bad, testRepo, code, out)
			}
		})
	}
}

// plantRecord replaces #90's disposition record with one carrying the given Evidence and
// RecordedBy. The record is JSON-encoded: dispJSON's %q spells ESC as a Go escape, which is
// not JSON, and an unparseable record is could-not-check before any comparison.
func plantRecord(s *stubRemote, evidence, recordedBy string) {
	ev, _ := json.Marshal(evidence)
	by, _ := json.Marshal(recordedBy)
	rec := strings.Replace(dispJSON(dispCheckedFailed, verdictSuperseded, "EV"), `"EV"`, string(ev), 1)
	s.disps[testRepo+"#90"] = strings.Replace(rec, `"worker-session"`, string(by), 1)
}

// privateAudit points HOME at a fresh directory (with the fixture roster) so the test can
// read back every audit line the run wrote, and returns a reader for the whole log.
func privateAudit(t *testing.T) func() string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	return func() string {
		b, _ := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
		return string(b)
	}
}

// Every output a record's Evidence or a proposal's target can reach — stdout (receipts,
// the dry-run close comment, the dry-run proposal body), the refusal (stderr and the
// reviewer's input) and the audit log — carries none of the planted text. The refusals do
// not repeat it, and the one accepted spelling with a foreign host is echoed only as the
// re-rendered reference.
//
// regression: class superseded-target-echo
func TestEvidenceNeverEchoed(t *testing.T) {
	type run struct {
		name string
		args func(rul string) []string
		as   string
	}
	runs := []run{
		{"confirm", func(rul string) []string {
			return []string{modeSuperseded, "-R", testRepo, "90", "--by", testRepo + "#40", "--rulings", rul, "--dry-run"}
		}, reviewerLogin},
		{"propose", func(rul string) []string {
			return []string{modeSuperseded, "-R", testRepo, "90", "--by", testRepo + "#40", "--rulings", rul, "--dry-run"}
		}, workerLogin},
	}
	for name, bad := range junkRefs() {
		for _, r := range runs {
			t.Run(name+"/"+r.name, func(t *testing.T) {
				audit := privateAudit(t)
				s, rul := prWorld(t)
				s.viewer = r.as
				plantRecord(s, bad, "worker-session")
				if r.name == "confirm" {
					s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
				}
				code, out := execCLI(r.args(rul)...)
				if code != deskkit.ExitRefused {
					t.Fatalf("evidence %q must be refused; exit %d\n%q", bad, code, out)
				}
				err := execErr(r.args(rul)...)
				assertNoJunk(t, "stdout", out)
				assertNoJunk(t, "the refusal", err.Error())
				assertNoJunk(t, "the audit log", audit())
				if len(s.writes()) != 0 {
					t.Fatalf("a refused run wrote: %v", s.writes())
				}
			})
		}
	}
}

// A record that DOES agree is echoed only through its parse: a foreign-host URL renders as
// the canonical reference (the host is not repeated), and a self-reported RecordedBy that
// is not one plain word renders as a fixed placeholder — in the dry-run close comment and
// in the dry-run proposal body alike.
//
// regression: class superseded-target-echo
func TestRecordRerendered(t *testing.T) {
	for _, as := range []string{reviewerLogin, workerLogin} {
		t.Run(as, func(t *testing.T) {
			audit := privateAudit(t)
			s, rul := prWorld(t)
			s.viewer = as
			plantRecord(s, "https://elsewhere.example/"+testRepo+"/pull/40", "@example-user **x**")
			if as == reviewerLogin {
				s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
			}
			code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40",
				"--rulings", rul, "--dry-run")
			if code != deskkit.ExitOK {
				t.Fatalf("want exit 0, got %d\n%s", code, out)
			}
			if !strings.Contains(out, "evidence "+testRepo+"#40 (recorded 2026-08-12 by "+unreadableField+")") {
				t.Fatalf("the record line must carry the re-rendered Evidence and the placeholder:\n%s", out)
			}
			for _, j := range []string{"elsewhere", "@example-user", "**x**"} {
				if strings.Contains(out, j) || strings.Contains(audit(), j) {
					t.Fatalf("output echoed %q from the record:\n%s", j, out)
				}
			}
		})
	}
}

// When no target is declared, the record's Evidence stands in for it — only through its
// parse. Junk Evidence is refused without being repeated; a foreign-host URL returns the
// canonical reference.
//
// regression: class superseded-target-echo
func TestEvidenceTargetParsed(t *testing.T) {
	for name, bad := range junkRefs() {
		t.Run(name, func(t *testing.T) {
			s, _ := prWorld(t)
			plantRecord(s, bad, "worker-session")
			it, err := fetchItem(testRepo, 90, "")
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := verifyLane(&closeReq{repo: testRepo, number: 90, mode: modeSuperseded}, it)
			if err == nil || !deskkit.IsRefused(err) {
				t.Fatalf("junk Evidence must be refused as a target; got %q, %v", got, err)
			}
			assertNoJunk(t, "the refusal", err.Error())
		})
	}
	s, _ := prWorld(t)
	plantRecord(s, "https://elsewhere.example/"+testRepo+"/pull/40", "worker-session")
	it, err := fetchItem(testRepo, 90, "")
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := verifyLane(&closeReq{repo: testRepo, number: 90, mode: modeSuperseded}, it)
	if err != nil || got != testRepo+"#40" {
		t.Fatalf("verifyLane = %q, %v; want the canonical %s#40", got, err, testRepo)
	}
}
