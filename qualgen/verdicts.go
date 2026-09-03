package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// verdicts.go is leg 2 of the sweep lane: agent verification. For each suspect
// it must verify, it assembles a size-capped context pack (the suspect's file
// region + the linter's raw evidence), invokes the configured AgentVerifier,
// and then — EMITTER-side, a different component than the verifier — enforces
// the evidence contract before the verdict is recorded. This second layer is
// the point: a buggy or overconfident verifier that returns `confirmed` with no
// evidence pointer cannot self-confirm, because leg 2 reclassifies it
// could-not-verify. The report (leg 3) then renders the evidence verbatim so a
// human re-adjudicates cheaply.

// maxContextBytes caps the file region carried in a context pack. The pack is
// meant to be enough surrounding code to adjudicate a lead, never the whole
// file; a runaway module cannot balloon the pack past this.
const maxContextBytes = 8 * 1024

// contextMarginLines is how many lines above and below the flagged region the
// context pack includes.
const contextMarginLines = 8

// verifyOutcome is one suspect's leg-2 result: the recorded verdict plus
// whether the emitter had to reclassify it for want of evidence.
type verifyOutcome struct {
	Verdict      verifier.Verdict
	Reclassified bool
}

// verifySuspects runs leg 2 over the suspects it is handed (the orchestrator
// passes only those that need verifying — new fingerprints, or all under
// --reverify-all). repoPath is the read-only target the context region is read
// from. It returns one verdict per suspect, in input order.
func verifySuspects(repoPath string, av verifier.AgentVerifier, suspects []verifier.Suspect) ([]verifyOutcome, error) {
	if len(suspects) > 0 && av == nil {
		return nil, fmt.Errorf("qualgen sweep: %d suspect(s) to verify but no verifier configured", len(suspects))
	}
	out := make([]verifyOutcome, 0, len(suspects))
	for _, s := range suspects {
		pack := assembleContext(repoPath, s)
		v, err := av.Verify(s, pack)
		if err != nil {
			// A verifier that could not produce a verdict is could-not-verify,
			// never a silent confirm or drop (spec §3.2).
			out = append(out, verifyOutcome{Verdict: verifier.Verdict{
				Fingerprint: s.Fingerprint,
				Class:       verifier.ClassCouldNotVerify,
				Rationale:   fmt.Sprintf("verifier error: %v", err),
			}})
			continue
		}
		v.Fingerprint = s.Fingerprint
		out = append(out, enforceEvidence(v))
	}
	return out, nil
}

// enforceEvidence is the emitter-side evidence gate. An ACTIONABLE verdict
// (confirmed / needs-human) with an empty evidence pointer is malformed by
// construction — a Rationale paragraph is not evidence — so it is reclassified
// could-not-verify with the original class recorded in the rationale. A
// false-positive or already-could-not-verify verdict is left untouched (neither
// is rendered as actionable, so neither needs a pointer).
func enforceEvidence(v verifier.Verdict) verifyOutcome {
	if v.Class.Actionable() && strings.TrimSpace(v.EvidencePointer) == "" {
		orig := v.Class
		note := fmt.Sprintf("reclassified from %q: an actionable verdict with an empty evidence pointer is not evidence", orig)
		if strings.TrimSpace(v.Rationale) != "" {
			note = v.Rationale + " — " + note
		}
		return verifyOutcome{
			Reclassified: true,
			Verdict: verifier.Verdict{
				Fingerprint:     v.Fingerprint,
				Class:           verifier.ClassCouldNotVerify,
				EvidencePointer: "",
				Rationale:       note,
			},
		}
	}
	return verifyOutcome{Verdict: v}
}

// assembleContext builds the size-capped context pack for a suspect: the file
// region around the flagged lines plus the raw linter evidence. A file that
// cannot be read still yields a pack (empty region) — the verifier decides what
// to do with thin context; leg 2 never fails the whole run for one unreadable
// suspect file.
func assembleContext(repoPath string, s verifier.Suspect) verifier.ContextPack {
	pack := verifier.ContextPack{
		Category:    s.Category,
		RawEvidence: s.RawEvidence,
	}
	pack.FileRegion = readRegion(filepath.Join(repoPath, filepath.FromSlash(s.File)), s.LineStart, s.LineEnd)
	return pack
}

// readRegion reads a byte- and line-bounded window of a file around
// [start,end]. It returns "" when the file cannot be opened. The window is
// capped at maxContextBytes so an oversized-module suspect cannot inflate the
// pack.
func readRegion(path string, start, end int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	lo := start - contextMarginLines
	if lo < 1 {
		lo = 1
	}
	hi := end + contextMarginLines

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		if n < lo {
			continue
		}
		if n > hi {
			break
		}
		if b.Len()+len(sc.Text())+1 > maxContextBytes {
			b.WriteString("… (context truncated at cap)\n")
			break
		}
		fmt.Fprintf(&b, "%d: %s\n", n, sc.Text())
	}
	return b.String()
}
