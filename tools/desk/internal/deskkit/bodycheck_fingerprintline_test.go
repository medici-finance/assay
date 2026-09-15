package deskkit

import (
	"strings"
	"testing"
)

// TestBodyCheckPGPFingerprintAnnotatedLine pins the #1161 widening of Rule 3: a 40-char
// UPPERCASE-hex run is admitted when the SAME LINE names it as a fingerprint — the word
// `fingerprint`, `fpr`, `pgp` or `gpg`, case-insensitively, anywhere on that line, before or
// after the run — because a brief legitimately names a signing key in prose ("the key
// fingerprint is …") and again inside a Verify-row command (`gpg --fingerprint …`), neither
// of which wears a `pgp:`/`fp:` recipient field.
//
// The bound stays hard in every other direction: the annotation must be on the SAME line
// (a neighbouring line does not reach), it must be a standalone word (a letter glued to its
// left is not the word), the run must still be EXACTLY 40 uppercase hex (a real secret is not
// pure uppercase hex and is never laundered by the word), and the original pgp:/fp: anchor
// keeps working unchanged.
func TestBodyCheckPGPFingerprintAnnotatedLine(t *testing.T) {
	// Obviously synthetic fingerprint SHAPES, split so the branch diff this test rides in
	// never carries a contiguous bare 40-hex run on an added line.
	fpr := "0123456789ABCDEF0123" + "456789ABCDEF01234567"
	fpr2 := "FEDCBA9876543210FEDC" + "BA9876543210FEDCBA98"

	pass := []struct {
		name string
		body string
	}{
		{"prose, word before the run", "The signing key fingerprint is " + fpr + " and must not change.\n"},
		{"prose, word after the run", "Key " + fpr + " is the release fingerprint.\n"},
		{"verify-row gpg command", "| 2 | `gpg --fingerprint " + fpr + "` | `1` |\n"},
		{"verify-row grep with a trailing annotation", "| 3 | `grep -c '" + fpr + "' keys.txt` (pgp key) | `1` |\n"},
		{"gpg --with-colons fpr record", "fpr:::::::::" + fpr + ":\n"},
		{"uppercase word", "GPG KEY " + fpr + "\n"},
		{"mixed-case word", "Fingerprint: " + fpr + "\n"},
		{"word behind an underscore", "key_fingerprint=" + fpr + "\n"},
		{"word behind a flag dash", "--gpg-sign " + fpr + "\n"},
		{"two annotated runs on one line", "pgp keys " + fpr + " and " + fpr2 + "\n"},
		{"original pgp: anchor still admits", "creation_rules:\n  - pgp: " + fpr + "\n"},
		{"original fp: anchor still admits", "recipients:\n  - fp: " + fpr + "\n"},
		{"annotated run on one line, clean prose on the others",
			"# Brief\n\nThe key fingerprint is " + fpr + ".\n\nNothing else here.\n"},
	}
	for _, c := range pass {
		t.Run("pass/"+c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err != nil {
				t.Errorf("an uppercase-hex fingerprint annotated on its own line (%s) was refused: %v", c.name, err)
			}
		})
	}

	refuse := []struct {
		name string
		body string
	}{
		{"bare run, no annotation anywhere", "The signing key is " + fpr + " and must not change.\n"},
		{"annotation on the PREVIOUS line only", "This is the gpg fingerprint:\n" + fpr + "\n"},
		{"annotation on the NEXT line only", fpr + "\n(that was the pgp fingerprint)\n"},
		{"word glued to a letter on its left is not the word", "keyfpr=" + fpr + "\n"},
		{"word glued to a digit on its left is not the word", "x9gpg " + fpr + "\n"},
		{"39 uppercase hex chars with the word", "fingerprint " + fpr[:39] + "\n"},
		{"41 uppercase hex chars with the word", "fingerprint " + fpr + "A\n"},
		{"lowercase-suffixed mixed-case run with the word", "fingerprint " + fpr[:39] + "a\n"},
		{"a real secret behind the word is not pure uppercase hex", "gpg key " + scanSecret40 + "\n"},
		{"a real secret behind the word, ghp arm", "fingerprint ghp_" + strings.Repeat("c", 36) + "\n"},
		{"annotated run plus a bare run on another line",
			"gpg fingerprint " + fpr + "\nalso " + fpr2 + "\n"},
	}
	for _, c := range refuse {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err == nil {
				t.Errorf("%s was admitted — the widened Rule 3 must not exempt it", c.name)
			}
		})
	}
}
