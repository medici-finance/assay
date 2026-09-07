package deskkit

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// The --explain SURFACE for a secret-scan refusal.
//
// A refusal that names only the run LENGTH is a refusal the caller works around: a 24-hour
// sweep found one PR body refused six times (then breaker-tripped) because the caller had
// to GUESS which span tripped it. --explain names the rule and the LOCATION so a refused
// caller can act on the first round.
//
// The one thing it must never do is print the offending span — the refusal must not become
// the leak. So ScanFinding carries a REDACTED shape (first 2 + last 2 characters, middle
// elided, plus a character-class summary), never the bytes; and the desk verbs print it
// ONLY when --explain is passed, so a transcript's default shape is unchanged.

// ScanFinding is the structured explanation attached to a secret-scan refusal via
// errors.As. It is reachable as `var f *ScanFinding; errors.As(err, &f)` and printed by
// the desk verbs under --explain (see MaybeExplain).
type ScanFinding struct {
	// Rule is the id of the arm that refused: high-entropy-run, decrypted-k8s-secret,
	// pem-block, github-token, aws-key-id, jwt, sops-block.
	Rule string
	// Line is the 1-based line of the first offending span on the scanned surface.
	Line int
	// Length is the length in bytes of the offending span (0 when a span length is not
	// meaningful for the rule).
	Length int
	// Shape is a REDACTED descriptor — first 2 + last 2 characters with the middle as
	// "…", plus a character-class summary (hex, digits, digits+/, base64). It NEVER
	// contains the span itself.
	Shape string
}

// Error lets *ScanFinding satisfy the error interface, which is what makes it a legal
// errors.As target (errors.As requires the target's element type to implement error). The
// finding is never returned AS the error of a refusal — it rides in DeskError.Finding — so
// this string is only ever seen by code that deliberately formats a finding.
func (f *ScanFinding) Error() string { return f.Explain() }

// Explain renders the finding as the single line the desk verbs print under --explain. It
// names the rule and the location and carries only the redacted shape.
func (f *ScanFinding) Explain() string {
	return fmt.Sprintf("scan-explain: rule=%s line=%d length=%d shape=%s",
		f.Rule, f.Line, f.Length, f.Shape)
}

// MaybeExplain prints err's ScanFinding to w when explain is true and err carries one. It
// is a no-op otherwise, so the default (no --explain) output of every verb is unchanged.
func MaybeExplain(w io.Writer, explain bool, err error) {
	if !explain || err == nil {
		return
	}
	var f *ScanFinding
	if errors.As(err, &f) && f != nil {
		fmt.Fprintln(w, f.Explain())
	}
}

// lineOf returns the 1-based line number of byteOffset within s.
func lineOf(s string, byteOffset int) int {
	if byteOffset < 0 || byteOffset > len(s) {
		return 1
	}
	return 1 + strings.Count(s[:byteOffset], "\n")
}

// runClass summarises a run's character class for a finding's Shape, most-specific first.
func runClass(run string) string {
	onlyHex, onlyDigit, onlyDigitSlash := true, true, true
	for i := 0; i < len(run); i++ {
		c := run[i]
		isDigit := c >= '0' && c <= '9'
		isHex := isDigit || (c >= 'a' && c <= 'f')
		if !isHex {
			onlyHex = false
		}
		if !isDigit {
			onlyDigit = false
		}
		if !(isDigit || c == '/') {
			onlyDigitSlash = false
		}
	}
	switch {
	case onlyDigit:
		return "digits"
	case onlyHex:
		return "hex"
	case onlyDigitSlash:
		return "digits+/"
	default:
		return "base64"
	}
}

// redactShape returns the finding's Shape for run: first 2 + last 2 characters, the middle
// elided, plus the character-class summary. The full span is NEVER reconstructable from it
// — only its two ends and its class survive.
func redactShape(run string) string {
	class := runClass(run)
	if len(run) < 5 {
		return "… (" + class + ")"
	}
	return run[:2] + "…" + run[len(run)-2:] + " (" + class + ")"
}
