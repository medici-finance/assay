package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// rowIsBacked implements the "point quality" distinction for a
// verified/done brief row: `backed` means the row's status can be trusted
// at face value — its `## Evidence` section has real, independently-attributed
// content, and the Verified cell (plus, for `done`, the Reviewed cell) names a
// dated/attributed runner in brief-16's convention. `unbacked` means the row
// LOOKS the same as a backed one but isn't — exactly the false-confidence
// signal point quality exists to surface (a SCADA reading with no quality flag is a lie).
//
// It returns the per-row reasons a row is unbacked (empty when backed) so the
// `--lint` NOTICE can name WHY, not just that, a row is flagged.
//
// This is a RENDERING check only: it never
// blocks and never changes what counts as verified/done — those lifecycle
// gates belong to brief-02/03/16. Two of those gates (checkBriefFiles,
// attributionProblems) already enforce a hard version of this same idea, but
// ONLY for brief-v1 schema-opted files; a legacy (no-frontmatter) brief is
// explicitly EXEMPT from both (see testdata/attribution/.../brief-07-legacy.md)
// yet still renders as a bare `done` today. rowIsBacked closes that visibility
// gap: it is schema-agnostic and evaluates every verified/done row, opted-in or
// not. Its criteria deliberately mirror brief-16's (dated runner, independent
// Evidence row, attributed Reviewed) so point quality never contradicts the
// hard gate — a row the gate would pass must not render `*`, and vice-versa.
func rowIsBacked(s *Stream, br *Brief) (bool, []string) {
	if br.Status != "verified" && br.Status != "done" {
		return true, nil // out of scope for this check: never flagged
	}
	var reasons []string
	if !verifiedCellRe.MatchString(br.Verified) {
		reasons = append(reasons, `Verified cell is not a dated runner ("YYYY-MM-DD <runner>")`)
	}
	// A `done` row must also name an attributed reviewer — a dated runner OR a
	// "human:<name>" token. This mirrors checkBriefFiles'
	// human-gate acceptance (hasHumanReviewer) so point quality never
	// contradicts it: a bare-but-human-attributed Reviewed cell that legitimately
	// closes a done brief must not then render `done*`. A `verified` row does not
	// require a Reviewed cell yet (checks.go draws this same line — done needs
	// both Verified and Reviewed, verified only Verified).
	if br.Status == "done" && !reviewedIsAttributed(br.Reviewed) {
		reasons = append(reasons, `Reviewed cell names no dated runner or "human:<name>" reviewer`)
	}
	if !briefEvidenceBacked(s, br) {
		reasons = append(reasons, "Evidence section is empty or has no independent (non-implementer) runner row")
	}
	return len(reasons) == 0, reasons
}

// reviewedIsAttributed reports whether a Reviewed cell carries real
// attribution: a leading date ("YYYY-MM-DD <runner>", the same shape as a
// Verified cell) or a "human:<name>" token. A bare mark
// ("✓", "grandfathered") is neither. Accepting the human token keeps point
// quality consistent with checkBriefFiles' human-gate acceptance
// (hasHumanReviewer), which does not itself require a date.
func reviewedIsAttributed(reviewed string) bool {
	return verifiedCellRe.MatchString(reviewed) || hasHumanReviewer(reviewed)
}

// briefEvidenceBacked looks up the brief-*.md file matching br's number in
// stream s's directory and reports whether its `## Evidence` section is
// backed: it has real content AND at least one independent (non-implementer)
// runner row. Unlike parseBriefFile, this does NOT require `schema: brief-v1`
// opt-in — the whole point is to see through the exemption that lets a legacy
// brief render an unbacked `done` with no visible flag. Requiring an
// independent row (not merely content) matches brief-16's own attribution gate
// (attributionProblems → evidenceHasIndependentRow) and closes the legacy
// blind spot: a no-frontmatter brief with an implementer-only Evidence table is
// exempt from that hard gate yet must still read unbacked here. No matching
// file, no Evidence section, or an implementer-only table all count as NOT
// backed: quality cannot be confirmed from nothing.
func briefEvidenceBacked(s *Stream, br *Brief) bool {
	for _, path := range briefFilePaths(s) {
		_, num, ok := expectedBriefID(path)
		if !ok || num != br.Num {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		content := strings.ReplaceAll(string(raw), "\r\n", "\n")
		body := content
		if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
			if _, b, err := splitFrontmatter(content); err == nil {
				body = b
			}
			// An unterminated frontmatter fence is a real error elsewhere
			// (checkBriefFiles catches it for schema-opted files); here we
			// simply fall back to treating the whole content as body, which
			// will not contain a clean "## Evidence" heading and so reads
			// as unbacked — a safe, non-crashing default.
		}
		ev := extractEvidence(body)
		return evidenceHasContent(ev) && evidenceHasIndependentRow(ev)
	}
	return false // no matching brief file: nothing to confirm quality from
}

// qualityToken renders a brief's Status with its point-quality suffixes:
//   - `*` — unbacked: the row's Evidence/attribution cannot confirm it.
//   - `‡` — closed over an UNRUN risk-bearing Verify
//     row (the live/mutating check never ran, derived from Verify-vs-Evidence
//     coverage — see unrun.go).
//
// The two are orthogonal and both may apply (`done*‡`). `*` stays purely
// cosmetic; `‡` is the RENDERING half of brief 37 and is likewise cosmetic here
// — its teeth live in unrunGateChecks, which gates the `done` transition.
func qualityToken(s *Stream, br *Brief) string {
	token := br.Status
	if backed, _ := rowIsBacked(s, br); !backed {
		token += "*"
	}
	return token + unrunToken(s, br)
}

// qualityNotices is the --lint NOTICE list (never a hard problem) of
// unbacked verified/done rows across all streams — the false-confidence
// points point quality exists to make visible to the desk. Each notice names the
// specific reason(s) the row is unbacked. Sorted for stable output.
func qualityNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "verified" && br.Status != "done" {
				continue
			}
			if backed, reasons := rowIsBacked(s, br); !backed {
				notices = append(notices, fmt.Sprintf(
					"%s/brief-%s: %s row is unbacked (%s) — point quality, renders as %s* in STATUS.md",
					s.Name, br.Num, br.Status, strings.Join(reasons, "; "), br.Status))
			}
		}
	}
	sort.Strings(notices)
	return notices
}
