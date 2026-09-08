package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit/untrustcorpus"
)

// corpusRoot is the untrusted-read positive-control corpus (brief 02), reached from this
// package's test working directory. The scanner asserts itself against THIS corpus rather
// than re-authoring samples — the dependency the brief mandates.
const corpusRoot = "untrustcorpus/testdata"

// semgrepAvailable reports whether the Semgrep engine can be resolved. The code leg is
// Semgrep-driven, so several assertions below are conditioned on it: with Semgrep present
// a code-exec sample must FLAG and a benign sample must be CLEAN; with it absent the code
// leg is could-not-check (never rounded up to clean), and that is asserted as itself.
func semgrepAvailable() bool {
	_, err := exec.LookPath("semgrep")
	return err == nil
}

// familyForReason maps a corpus entry's expected reason marker to the family that must
// carry it. A benign entry's reason ("plain-text", "source-fragment") maps to "" — no
// family should fail.
func familyForReason(reason string) string {
	switch reason {
	case "exfil-callout":
		return FamilyExfil
	case "unicode-tagblock", "bidi-override", "unicode-zerowidth", "imperative-lure":
		return FamilyInjection
	case "base64-exec", "install-hook-override", "command-overwrite", "steg-extract-exec":
		return FamilyCodeExec
	}
	return ""
}

// TestUntrustscanCorpus is the positive-and-negative control: it iterates brief 02's
// manifest and asserts every attack sample FLAGS on the expected family with the expected
// marker, and every benign sample raises NO flag. This is Verify row 1.
func TestUntrustscanCorpus(t *testing.T) {
	samples, err := untrustcorpus.Load(corpusRoot)
	if err != nil {
		t.Fatalf("load corpus %q: %v", corpusRoot, err)
	}
	if len(samples) == 0 {
		t.Fatalf("corpus %q has no samples", corpusRoot)
	}
	sg := semgrepAvailable()
	if !sg {
		t.Logf("semgrep not on PATH: code-leg assertions accept could-not-check (fail-closed), never clean")
	}

	for _, s := range samples {
		s := s
		t.Run(s.ID, func(t *testing.T) {
			v := UntrustScan(s.Bytes, UntrustScanOptions{})

			switch s.Expect.Scan {
			case "flag":
				fam := familyForReason(s.Expect.Reason)
				if fam == "" {
					t.Fatalf("%s: expect flag but reason %q maps to no family", s.ID, s.Expect.Reason)
				}
				res := v.Families[fam]
				if fam == FamilyCodeExec && !sg {
					// Semgrep-driven leg with no engine: could-not-check is the honest,
					// fail-closed answer — never rounded up to a pass, and here never a
					// silent clean over an attack sample.
					if res.State != UntrustStateCNC {
						t.Fatalf("%s: codeexec with no semgrep = %q, want %q", s.ID, res.State, UntrustStateCNC)
					}
					return
				}
				if res.State != UntrustStateFailed {
					t.Fatalf("%s: family %s state = %q, want %q (markers=%v, overall=%q)",
						s.ID, fam, res.State, UntrustStateFailed, res.Markers, v.Overall)
				}
				if !containsMarker(res.Markers, s.Expect.Reason) {
					t.Fatalf("%s: family %s flagged but marker %q not among %v",
						s.ID, fam, s.Expect.Reason, res.Markers)
				}
				if v.Overall != UntrustStateFailed {
					t.Fatalf("%s: overall = %q, want %q", s.ID, v.Overall, UntrustStateFailed)
				}
			case "clean":
				for name, res := range v.Families {
					if res.State == UntrustStateFailed {
						t.Fatalf("%s: benign sample FLAGGED false on family %s (markers=%v)",
							s.ID, name, res.Markers)
					}
				}
				if sg && v.Overall != UntrustStateClean {
					t.Fatalf("%s: benign overall = %q, want %q (semgrep present)", s.ID, v.Overall, UntrustStateClean)
				}
			default:
				t.Fatalf("%s: corpus expect.scan = %q, want flag|clean", s.ID, s.Expect.Scan)
			}
		})
	}
}

// TestUntrustscanNeutralizesInvisibles pins the neutralised-rendering property Verify row 3
// asserts: after neutralisation the rendering carries NO Tag-block codepoint (and no other
// invisible steering codepoint), while the visible text survives.
func TestUntrustscanNeutralizesInvisibles(t *testing.T) {
	samples, err := untrustcorpus.Load(corpusRoot)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	var checked int
	for _, s := range samples {
		if s.Class != "injection-unicode" {
			continue
		}
		checked++
		neu := UntrustNeutralize(s.Bytes)
		for _, r := range neu {
			if r >= 0xE0000 && r <= 0xE007F {
				t.Errorf("%s: neutralised rendering still carries a Tag-block codepoint U+%04X", s.ID, r)
			}
			if isNeutralizeEscape(r) {
				t.Errorf("%s: neutralised rendering still carries an invisible codepoint U+%04X", s.ID, r)
			}
		}
		if !strings.Contains(neu, untrustFenceHeader) || !strings.Contains(neu, untrustFenceFooter) {
			t.Errorf("%s: neutralised rendering is not fenced as inert data", s.ID)
		}
	}
	if checked == 0 {
		t.Fatalf("no injection-unicode sample in corpus — cannot test neutralisation")
	}
}

// TestUntrustscanCodeLegFailsClosed pins Verify row 6: with the code leg's engine forced
// unavailable, the codeexec family is could-not-check — NEVER clean — even on a sample the
// engine would otherwise flag.
func TestUntrustscanCodeLegFailsClosed(t *testing.T) {
	samples, err := untrustcorpus.Load(corpusRoot)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	for _, s := range samples {
		if s.Class != "code-exec" && s.Class != "install-hook" {
			continue
		}
		v := UntrustScan(s.Bytes, UntrustScanOptions{DisableSemgrep: true})
		if got := v.Families[FamilyCodeExec].State; got != UntrustStateCNC {
			t.Fatalf("%s: codeexec with disabled engine = %q, want %q (must never be clean)",
				s.ID, got, UntrustStateCNC)
		}
		if v.Overall == UntrustStateClean {
			t.Fatalf("%s: overall clean with a disabled code leg — could-not-check must never round up to clean", s.ID)
		}
	}
}

// TestUntrustscanExfilAndInjectionAreSemgrepIndependent proves the regex families answer
// without Semgrep: the exfil and injection samples still FLAG when the code leg is
// disabled, because they are not the code leg.
func TestUntrustscanExfilAndInjectionAreSemgrepIndependent(t *testing.T) {
	samples, err := untrustcorpus.Load(corpusRoot)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	for _, s := range samples {
		fam := ""
		switch s.Class {
		case "exfil-callout":
			fam = FamilyExfil
		case "injection-unicode", "injection-imperative":
			fam = FamilyInjection
		default:
			continue
		}
		v := UntrustScan(s.Bytes, UntrustScanOptions{DisableSemgrep: true})
		if v.Families[fam].State != UntrustStateFailed {
			t.Fatalf("%s: family %s = %q with code leg disabled, want %q (regex family is semgrep-independent)",
				s.ID, fam, v.Families[fam].State, UntrustStateFailed)
		}
	}
}

// TestUntrustscanBenignPlainText is a direct negative control independent of the corpus
// loader: ordinary prose raises no flag on the regex families.
func TestUntrustscanBenignPlainText(t *testing.T) {
	v := UntrustScan([]byte("The quarterly report is attached for your review."), UntrustScanOptions{DisableSemgrep: true})
	if v.Families[FamilyExfil].State == UntrustStateFailed {
		t.Errorf("exfil false positive on plain prose: %v", v.Families[FamilyExfil].Markers)
	}
	if v.Families[FamilyInjection].State == UntrustStateFailed {
		t.Errorf("injection false positive on plain prose: %v", v.Families[FamilyInjection].Markers)
	}
}

// --- only-widens house rule pack -----------------------------------------------------

// writeCallout writes an executable shell script to a fresh 0755 dir and returns its
// absolute path. The dir is 0755 (not group/world-writable) so deskkit.Callout accepts it.
func writeCallout(t *testing.T, script string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "untrustscan-callout-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	p := filepath.Join(dir, "pack")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write callout: %v", err)
	}
	return p
}

// TestUntrustscanCalloutOnlyWidens proves the house rule pack can ADD a detection (turn a
// clean family failed) but its output can never CLEAR a built-in flag.
func TestUntrustscanCalloutOnlyWidens(t *testing.T) {
	// A pack that flags exfil AND tries to "clear" injection by simply not mentioning it.
	pack := writeCallout(t, "#!/bin/sh\ncat >/dev/null\nprintf '{\"markers\":[{\"family\":\"exfil\",\"id\":\"house-exfil-rule\"}]}\\n'\n")

	// Content that already trips the built-in injection detector (a bidi override).
	content := []byte("safe text \u202e reversed")
	v := UntrustScan(content, UntrustScanOptions{DisableSemgrep: true, CalloutPath: pack})

	// The pack widened exfil clean -> failed.
	if v.Families[FamilyExfil].State != UntrustStateFailed {
		t.Errorf("callout did not widen exfil: %+v", v.Families[FamilyExfil])
	}
	if !containsMarker(v.Families[FamilyExfil].Markers, "house-exfil-rule") {
		t.Errorf("callout marker not added: %v", v.Families[FamilyExfil].Markers)
	}
	// The built-in injection flag STANDS — the pack cannot clear it by omission.
	if v.Families[FamilyInjection].State != UntrustStateFailed {
		t.Errorf("built-in injection flag was cleared by the rule pack: %+v", v.Families[FamilyInjection])
	}
}

// TestUntrustscanCalloutFailsClosed proves a configured pack that cannot answer degrades a
// clean family to could-not-check (never leaves it clean) and never clears a built-in flag.
func TestUntrustscanCalloutFailsClosed(t *testing.T) {
	pack := writeCallout(t, "#!/bin/sh\nexit 3\n") // answers with a non-zero exit

	// Injection flags built-in; exfil is clean.
	content := []byte("hello \u202e world")
	v := UntrustScan(content, UntrustScanOptions{DisableSemgrep: true, CalloutPath: pack})

	if v.Families[FamilyExfil].State != UntrustStateCNC {
		t.Errorf("clean exfil family not degraded to could-not-check on callout failure: %+v", v.Families[FamilyExfil])
	}
	if v.Families[FamilyInjection].State != UntrustStateFailed {
		t.Errorf("built-in injection flag lost on callout failure: %+v", v.Families[FamilyInjection])
	}
}
