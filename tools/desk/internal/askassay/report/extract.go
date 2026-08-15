package report

import (
	"fmt"
	"sort"
	"strings"
)

// TEXT EXTRACTION, AND WHY IT IS IN-PROCESS
// -----------------------------------------
// A binary artifact can carry text no reviewer ever reads. That is not a
// hypothetical here: a person's given name was found inside customer-facing
// copy in this house, in a form nobody had looked at, because the reviewable
// artifact was the source and the shipped artifact was the render.
//
// A disclosure check over a PDF therefore has to read the PDF, not the source
// that produced it. Two ways to do that:
//
//  1. Shell out to pdftotext. It is present on this machine (poppler 26.08.0,
//     verified), but it is an external dependency of the CHECK, so on a host
//     without it the check silently becomes could-not-check — and a
//     disclosure check that quietly stops running is worse than none.
//  2. Read the bytes here. The writer in pdf.go emits UNCOMPRESSED content
//     streams with plain literal strings for exactly this reason, so the text
//     is recoverable with no external tool at all.
//
// This file does (2). [ExtractText] is therefore total over documents this
// package produced, and TestExtractedTextMatchesPdftotext cross-checks it
// against poppler when poppler is present — and records could-not-check,
// never a pass, when it is not.
//
// THE BOUND ON THE CLAIM, STATED
// ------------------------------
// This extractor understands the subset of PDF that pdf.go writes: literal
// strings drawn with Tj inside BT/ET, no compression, no hex strings, no
// font subsetting, no ToUnicode mapping. It is NOT a general PDF text
// extractor and must not be used to audit a PDF from any other producer —
// against a compressed or subset PDF it would return little or nothing, and
// "no text found" would read as "no disclosure found". [ExtractText] returns
// an error rather than an empty string when it finds no text at all, so that
// failure cannot be mistaken for a clean result.

// ExtractText recovers the drawn text from a PDF this package produced, in
// page order, one drawn string per line.
func ExtractText(pdf []byte) (string, error) {
	s := string(pdf)
	if !strings.HasPrefix(s, "%PDF-") {
		return "", fmt.Errorf("not a PDF: no %%PDF- header")
	}
	if strings.Contains(s, "/Filter") {
		return "", fmt.Errorf("this PDF declares a stream filter — the extractor reads only the uncompressed subset this package writes, and returning its partial result would let 'no text found' pass as 'nothing disclosed'")
	}
	var out []string
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "(")
		if j < 0 {
			break
		}
		start := i + j + 1
		lit, end, ok := readLiteral(s, start)
		if !ok {
			break
		}
		// Only strings that are actually painted count. A literal in the Info
		// dictionary is metadata, and metadata is checked separately.
		rest := s[end:]
		k := 0
		for k < len(rest) && (rest[k] == ' ' || rest[k] == '\n' || rest[k] == '\r' || rest[k] == '\t') {
			k++
		}
		if strings.HasPrefix(rest[k:], "Tj") {
			out = append(out, lit)
		}
		i = end
	}
	if len(out) == 0 {
		return "", fmt.Errorf("no drawn text recovered from a %d-byte PDF — treat this as could-not-check, never as a clean disclosure result", len(pdf))
	}
	return strings.Join(out, "\n"), nil
}

func readLiteral(s string, i int) (string, int, bool) {
	var b strings.Builder
	depth := 1
	for i < len(s) {
		c := s[i]
		switch c {
		case '\\':
			if i+1 >= len(s) {
				return "", i, false
			}
			b.WriteByte(s[i+1])
			i += 2
			continue
		case '(':
			depth++
			b.WriteByte(c)
		case ')':
			depth--
			if depth == 0 {
				return b.String(), i + 1, true
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	return "", i, false
}

// ProbeStderrTerms are the markers of a leak class measured on this machine:
// EVERY desk-tools invocation writes its trust roster to stderr — a block of
// configuration diagnostic lines carrying a human login with its numeric id,
// the bot account slugs with their App ids, the human login map and the full
// private repo list. A report that captured a probe's combined output would
// embed all of that in a binary artifact nobody re-reads.
//
// This package never captures stderr: it consumes [askassay.Answer] values,
// and the probe layer beneath it runs commands through a call site that takes
// stdout only. That is an argument, not a proof, so it is turned into a check
// — [ProbeStderrTerms] is fed to [CheckDisclosure] against every generated
// report by TestGeneratedReportsCarryNoProbeStderr.
//
// The bound, stated: this catches the diagnostic block by its line marker. It
// is not a general secret scanner, and it says nothing about a caller that
// puts stderr into a caption by hand.
func ProbeStderrTerms() []Disclosure {
	return []Disclosure{
		{Term: "assay-config:", Why: "the desk-tools trust-roster diagnostic block, written to stderr on every invocation including read-only verbs"},
		{Term: "TRUSTED_LOGINS", Why: "the trusted-login roster, carried in that diagnostic block"},
		{Term: "BLESS_LOGIN", Why: "the blessing-login setting, carried in that diagnostic block"},
		{Term: "class=write source=config", Why: "the roster diagnostic's own class line"},
	}
}

// Disclosure is one term the extracted text must not contain.
type Disclosure struct {
	// Term is matched case-insensitively against the extracted text.
	Term string
	// Why names the class of the leak, so a hit is actionable.
	Why string
}

// DisclosureHit is one term found in a rendered artifact.
type DisclosureHit struct {
	Term string
	Why  string
	Line string
}

func (h DisclosureHit) String() string {
	return fmt.Sprintf("%q (%s) appears in the rendered text: %s", h.Term, h.Why, h.Line)
}

// CheckDisclosure extracts the text and reports every term found in it.
//
// The three-state contract is explicit in the signature: an error means
// COULD-NOT-CHECK — the text could not be recovered, and the caller must
// report that rather than a pass. A nil error with no hits is checked-clean;
// a nil error with hits is checked-failed. There is no return value that
// conflates "nothing found" with "could not look".
func CheckDisclosure(pdf []byte, terms []Disclosure) ([]DisclosureHit, error) {
	text, err := ExtractText(pdf)
	if err != nil {
		return nil, fmt.Errorf("could-not-check: %w", err)
	}
	lines := strings.Split(text, "\n")
	var hits []DisclosureHit
	for _, t := range terms {
		if strings.TrimSpace(t.Term) == "" {
			continue
		}
		needle := strings.ToLower(t.Term)
		for _, ln := range lines {
			if strings.Contains(strings.ToLower(ln), needle) {
				hits = append(hits, DisclosureHit{Term: t.Term, Why: t.Why, Line: ln})
				break
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Term < hits[j].Term })
	return hits, nil
}
