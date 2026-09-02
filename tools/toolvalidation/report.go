package main

import (
	"bufio"
	"strings"
)

// muhar's three per-mutation verdict tokens, exactly as Result.String() emits
// them in tools/desk/cmd/muhar/harness.go. The report line format is
// `  %-16s %s` — two leading spaces, the token left-padded to 16, a single
// space, then the mutation name (with an optional `  (err)` suffix on a
// could-not-mutate line). We parse against these literal tokens rather than a
// loose regexp so a change to muhar's vocabulary breaks this parser loudly
// (TestReportTokensMatchMuhar guards the pairing) instead of silently
// mis-reading a verdict.
const (
	tokCaught       = "CAUGHT"
	tokNotCaught    = "NOT_CAUGHT"
	tokCouldNot     = "COULD_NOT_MUTATE"
	brokenMarker    = "HARNESS BROKEN"
	healthyMarker   = "Harness healthy:"
	totalsMarker    = "Totals:"
	instrumentBlind = "HARNESS BROKEN — run discarded. (exit 2)"
)

// verdict is the pack's own vocabulary for one mutation's outcome. It maps
// muhar's three states straight through and adds couldNotCheck — the state a
// mutation lands in when muhar produced NO trustworthy verdict for it at all
// (the harness exited 2, or its report was missing/unparseable). Keeping
// couldNotCheck distinct from couldNotMutate is the whole point of the
// three-state instrument rule: a healthy run that could not plant one edit
// (could-not-mutate) is a different fact from a run that produced no verdicts
// (could-not-check), and neither may be read as a pass.
type verdict string

const (
	vCaught        verdict = "caught"
	vNotCaught     verdict = "NOT CAUGHT"
	vCouldNotMut   verdict = "could-not-mutate"
	vCouldNotCheck verdict = "could-not-check"
)

// reportOutcome is the parsed result of one captured muhar report. Broken is
// true when the report is a HARNESS BROKEN discard (muhar exit 2): in that case
// Verdicts is empty by construction, because muhar prints no per-mutation lines
// for a discarded run, and every mutation the spec declares must therefore be
// rendered could-not-check.
type reportOutcome struct {
	Broken       bool
	BrokenReason string
	// Verdicts pairs each per-mutation line's name-portion with its verdict, in
	// report order. For a sharded gate the caller unions the outcomes of every
	// shard before matching against the spec.
	Verdicts []namedVerdict
}

type namedVerdict struct {
	// Name is the report line's name-portion: the mutation name verbatim, with
	// any trailing `  (err)` on a could-not-mutate line kept as-is. Matching
	// against a spec's mutation name is prefix-aware (matchVerdict) so an error
	// suffix never defeats the correlation and a name that itself ends in
	// parentheses is never truncated.
	Name    string
	Verdict verdict
}

// parseReport reads one muhar report's text. It never errors: an unrecognised
// body (neither the healthy header nor the broken marker) is treated as a
// broken/absent verdict source, because a report the assembler cannot make
// sense of must contribute could-not-check, never a silent pass.
func parseReport(text string) reportOutcome {
	if strings.Contains(text, brokenMarker) {
		return reportOutcome{Broken: true, BrokenReason: brokenReason(text)}
	}
	if !strings.Contains(text, healthyMarker) {
		return reportOutcome{Broken: true, BrokenReason: "report is neither a healthy muhar run nor a HARNESS BROKEN discard — unparseable"}
	}
	var out reportOutcome
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if v, name, ok := parseVerdictLine(line); ok {
			out.Verdicts = append(out.Verdicts, namedVerdict{Name: name, Verdict: v})
		}
	}
	return out
}

// parseVerdictLine matches one `  <TOKEN padded-to-16> <name...>` line. It
// returns the mapped verdict and the name-portion (everything after the token
// and its trailing spaces). Non-verdict lines (the healthy header, Totals, the
// could-not-mutate NOTE) return ok=false.
func parseVerdictLine(line string) (verdict, string, bool) {
	if !strings.HasPrefix(line, "  ") {
		return "", "", false
	}
	body := strings.TrimLeft(line, " ")
	for tok, v := range map[string]verdict{
		tokCaught:    vCaught,
		tokNotCaught: vNotCaught,
		tokCouldNot:  vCouldNotMut,
	} {
		if body == tok {
			// A verdict token with no name is malformed; ignore it.
			return "", "", false
		}
		if strings.HasPrefix(body, tok+" ") {
			name := strings.TrimLeft(body[len(tok):], " ")
			if name == "" {
				return "", "", false
			}
			return v, name, true
		}
	}
	return "", "", false
}

// brokenReason lifts the human-readable reason line muhar prints under the
// HARNESS BROKEN header (`HARNESS BROKEN — run discarded.\n  <reason>\n`).
func brokenReason(text string) string {
	sc := bufio.NewScanner(strings.NewReader(text))
	seen := false
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, brokenMarker) {
			seen = true
			continue
		}
		if seen {
			if r := strings.TrimSpace(line); r != "" {
				return r
			}
		}
	}
	return "harness broken (no reason line captured)"
}

// matchVerdict finds the verdict for a spec mutation name among a report's
// per-mutation lines. It matches an exact name (caught / NOT CAUGHT) OR a line
// whose name-portion is the spec name followed by muhar's `  (` error suffix
// (could-not-mutate). Prefix-with-`  (` rather than a bare prefix so a mutation
// whose name is a prefix of another's cannot steal its verdict, and so a name
// that legitimately contains parentheses is never truncated. A name absent from
// every line returns could-not-check: the spec declared a mutation the report
// does not account for.
func matchVerdict(specName string, verdicts []namedVerdict) verdict {
	for _, nv := range verdicts {
		if nv.Name == specName {
			return nv.Verdict
		}
	}
	for _, nv := range verdicts {
		if strings.HasPrefix(nv.Name, specName+"  (") {
			return nv.Verdict
		}
	}
	return vCouldNotCheck
}
