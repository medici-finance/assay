package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// verifyRepoSlug is the GitHub owner/repo the rendered issue bodies link into —
// the adopter's HOME repo, read from the roster (ASSAY_HOME_REPO) rather than
// compiled in. It used to be a constant naming ONE organisation; externalising it
// is what lets statusgen ship clean of hard-coded org
// slugs while --verify-issues stays fully offline: the roster is read from the
// config home / CI transport, never from a git-remote or the network. Unset
// yields "" — the rendered brief link degrades to a repo-relative path rather
// than an absolute github.com URL, which is harmless (the body is still emitted).
func verifyRepoSlug() string { return scanEffectiveConfig().HomeRepo }

// verifyReviewer is the human sign-off token stamped into the Reviewed cell by
// --close-verify. The date is appended at run time via time.Now().
const verifyReviewer = "human:reviewer"

// verifyIssue is one emitted element of the --verify-issues JSON array — a
// self-contained GitHub issue payload for a brief awaiting human:<name>'s human
// sign-off (verified → done gate). The workflow feeds title/labels/body
// straight to `gh issue create`; marker is the idempotency key.
type verifyIssue struct {
	Brief  string   `json:"brief"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Marker string   `json:"marker"`
	Body   string   `json:"body"`
}

// verifyMarkerRe matches a hidden idempotency marker in an issue body (or a
// bare marker line). Tolerant of raw issue bodies so the workflow can pipe
// `gh issue list --json body` output straight in.
var verifyMarkerRe = regexp.MustCompile(`<!-- verify-gate: [^>]*? -->`)

// normalizeBriefKey collapses either brief-key form to the canonical
// <stream>/<NN> identity used for verify-gate card idempotency and close-back.
//
// A brief-v1 key is already <stream>/<NN>; a brief-v2 key is the fully-qualified
// <cell>:<repo>:<stream>:<NN> (parseBriefV2ID). Both name the SAME brief within a
// single tree — one repo's issue set — so the flag-day v1→v2 migration must not
// re-file a card that already exists in the other form. This reduces the v2 form
// to its trailing <stream>/<NN>; any other shape (a bare <stream>/<NN>, or a
// malformed id) is returned unchanged so it fails downstream on its own terms
// rather than being silently rewritten here. Rendering the marker in this
// canonical form (rather than the colon form) keeps a NEW card matching every
// pre-migration card, and keeps the extracted id inside the brief-name grammar
// (<stream>/<NN>) the close side already speaks.
func normalizeBriefKey(key string) string {
	key = strings.TrimSpace(key)
	if _, _, stream, num, ok := parseBriefV2ID(key); ok {
		return stream + "/" + num
	}
	return key
}

// verifyMarker renders the hidden per-brief marker. It is the first line of the
// issue body (idempotency) and the close-back mapping key. The brief key is
// normalized to the canonical <stream>/<NN> form so a brief-v2 (colon) key and a
// brief-v1 (slash) key produce ONE marker — the render half of the two-forms-
// one-identity contract (loadExistingMarkers is the match half).
func verifyMarker(brief string) string {
	return "<!-- verify-gate: " + normalizeBriefKey(brief) + " -->"
}

// normalizeMarker reduces a full `<!-- verify-gate: KEY -->` marker to its
// canonical form by normalizing KEY. loadExistingMarkers routes every extracted
// marker through it so a legacy <stream>/<NN> card and a brief-v2
// <cell>:<repo>:<stream>:<NN> card land on the SAME set key — the same canonical
// form verifyMarker renders — and thus compare equal.
func normalizeMarker(marker string) string {
	inner := strings.TrimSpace(marker)
	inner = strings.TrimPrefix(inner, "<!-- verify-gate:")
	inner = strings.TrimSuffix(inner, "-->")
	return verifyMarker(strings.TrimSpace(inner))
}

// gateReasons returns the risk keys answered "yes" (why the brief is
// gate: human), in the canonical key order for deterministic output.
func gateReasons(risk map[string]string) []string {
	var out []string
	for _, k := range canonicalRiskKeys {
		if risk[k] == "yes" {
			out = append(out, k)
		}
	}
	return out
}

// anyRiskYes reports whether any risk key in the brief's risk block is "yes".
func anyRiskYes(risk map[string]string) bool {
	for _, v := range risk {
		if v == "yes" {
			return true
		}
	}
	return false
}

// findRow returns the README brief-table row for a given brief number in a
// stream, or nil if absent.
func findRow(s *Stream, num string) *Brief {
	for i := range s.Briefs {
		if s.Briefs[i].Num == num {
			return &s.Briefs[i]
		}
	}
	return nil
}

// loadExistingMarkers reads the --existing-markers file and returns the set of
// verify-gate markers already present in existing issues. A missing/empty path
// or a non-existent file yields an empty set (nothing exists yet). It extracts
// every `<!-- verify-gate: … -->` occurrence, so it accepts either one marker
// per line OR raw issue bodies.
//
// Every extracted marker is routed through normalizeMarker so a card carrying a
// brief-v2 <cell>:<repo>:<stream>:<NN> key and a card carrying the legacy
// <stream>/<NN> key collapse to ONE set entry — the same canonical form
// verifyMarker renders. Without this, the flag-day v1→v2 migration re-filed a
// duplicate card for every already-carded brief (issue #804): the render key
// changed form, the exact-string match missed, and the open side re-emitted.
func loadExistingMarkers(path string) (map[string]bool, error) {
	set := map[string]bool{}
	if path == "" {
		return set, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	for _, m := range verifyMarkerRe.FindAllString(string(raw), -1) {
		set[normalizeMarker(m)] = true
	}
	return set, nil
}

// briefBody re-reads a brief file and returns its body (frontmatter stripped).
// Used to lift the Verify section and PR links for the issue body; parseBriefFile
// only retains the Evidence section, so this reads the file once more rather than
// widening that struct.
func briefBody(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
		if _, b, err := splitFrontmatter(content); err == nil {
			return b
		}
	}
	return content
}

// extractSectionByPrefix returns the lines between the first `## <prefix>…`
// heading and the next `## ` heading (or EOF). Unlike extractEvidence (which
// requires an exact heading), this matches a decorated heading such as
// `## Verify (executable — no prose-only DoD items)` via its prefix.
func extractSectionByPrefix(body, prefix string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if rest, ok := strings.CutPrefix(t, "## "); ok && strings.HasPrefix(rest, prefix) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// verifyVerdictBoldRe is the ratified regex for a BOLD VERIFY verdict marker:
// `**VERIFY: PASS**` or `**VERIFY: FAIL**`, with
// arbitrary prose between the verdict token and the closing `**` —
// `**VERIFY: PASS (4/4 offline-runnable rows)**`, `**VERIFY: PASS — all 6
// rows green.**` — but the verdict token itself is anchored to PASS|FAIL:
// `**VERIFY: BLOCKED (human-gate)**` and any other spelling never match,
// whatever prose surrounds it. Card and flip tooling (hasVerifyPass below,
// and autoflip.go's decideModelFlip) both gate on THIS regex, never a looser
// substring test — one wording, quoted, not two that can drift apart. Named
// distinctly from verifyMarkerRe above, which matches a different thing (the
// hidden `<!-- verify-gate: … -->` idempotency marker in an ISSUE body).
var verifyVerdictBoldRe = regexp.MustCompile(`\*\*VERIFY: (PASS|FAIL)\b[^*]*\*\*`)

// hasVerifyPass reports whether evidence contains a bold **VERIFY: PASS...**
// marker indicating a model verifier has run and passed the brief's Verify
// table.
//
// DELIBERATELY STRICT, and deliberately NOT the same test as
// lastVerifyVerdict below. This one is a GATE: it decides whether a gate:human
// brief at `implemented` may advance to a human sign-off and whether a verify
// issue is emitted. A gate must fail CLOSED — an Evidence body whose
// pass is written in some looser form is refused, and the fix is to write the
// canonical marker, not to loosen the gate. The two live side by side so the
// asymmetry is visible rather than discovered.
func hasVerifyPass(evidence string) bool {
	for _, m := range verifyVerdictBoldRe.FindAllStringSubmatch(evidence, -1) {
		if m[1] == "PASS" {
			return true
		}
	}
	return false
}

// verifyVerdictRe matches a VERIFY verdict marker anywhere in a line of an
// Evidence body. The marker is written in many surface forms in practice —
// `**VERIFY: PASS**`, `**Non-implementer verifier run — VERIFY: PASS** · …`,
// `**VERIFY: PASS — all 6 rows green.**`, `### … — VERIFY: FAIL (…)`,
// `VERIFY:FAIL` — so the emphasis and surrounding prose are not part of the
// test; the verdict token is.
var verifyVerdictRe = regexp.MustCompile(`VERIFY:[ \t]*(PASS|FAIL)`)

// verifyVerdict is the outcome of the LAST verdict recorded in an Evidence
// body: "pass", "fail", or "" when none was recorded.
const (
	verdictNone = ""
	verdictPass = "pass"
	verdictFail = "fail"
)

// lastVerifyVerdict returns the LAST verdict recorded in an Evidence body.
//
// LAST WRITER WINS, not "any occurrence ever". Evidence sections ACCUMULATE:
// a brief that failed, was reworked and then passed carries both markers, and
// the only honest reading of the record is the most recent verdict. A bare
// `strings.Contains(evidence, "VERIFY: FAIL")` pins such a brief in rework
// forever — live instance at the time of writing: three 2026-07-16/17 FAIL records followed by the 2026-07-20 PASS
// that promoted it.
//
// Three contexts are stripped before the scan because a marker inside them is
// a QUOTATION, not a verdict: fenced code blocks, blockquote lines, and
// struck-through (`~~…~~`) spans.
//
// KNOWN LIMIT, stated rather than papered over: a verdict token embedded in
// ordinary prose ("no VERIFY: FAIL was observed") still reads as a verdict.
// Distinguishing that needs structure the Evidence convention does not carry.
// This function classifies a BOARD SEGMENT — a wrong answer misfiles a row,
// it does not let anything through a gate (that is hasVerifyPass, above, which
// stays strict). `VERIFY: PARTIAL` is deliberately neither: it leaves the
// verdict unchanged rather than inventing a sixth segment.
func lastVerifyVerdict(evidence string) string {
	verdict := verdictNone
	inFence := false
	for _, line := range strings.Split(evidence, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, ">") {
			continue
		}
		line = strikethroughRe.ReplaceAllString(line, "")
		for _, m := range verifyVerdictRe.FindAllStringSubmatch(line, -1) {
			if m[1] == "PASS" {
				verdict = verdictPass
			} else {
				verdict = verdictFail
			}
		}
	}
	return verdict
}

// strikethroughRe matches a struck-through span — a retracted or superseded
// record, never a live verdict.
var strikethroughRe = regexp.MustCompile(`~~[^~]*~~`)

// heldOrCouldNotCheckRe matches a per-row "HELD" or "could-not-check"
// disposition an Evidence row can carry — a verifier saying "this row is not
// actually settled" even while the brief's Evidence also carries an overall
// **VERIFY: PASS** marker.
var heldOrCouldNotCheckRe = regexp.MustCompile(`(?i)\b(HELD|could-not-check)\b`)

// heldOrCouldNotCheckRe is a bare word-boundary match: on its own it cannot
// tell "this row IS held" from "this row is NOT held" or "there are ZERO held
// rows", because all three contain the marker word. Clean, fully-passing
// Evidence routinely says the latter two — "VERIFY: PASS ... no
// could-not-check", "summary: 0 HELD, 7 PASS" — and refusing it is a false
// positive. heldOccurrenceNegated below excuses exactly those shapes.
//
// It is a narrowing of a flip-refusal control, so every rule in it is written
// to FAIL CLOSED: a wrongly excused occurrence is a false NEGATIVE — a PASS
// proceeding over a genuinely held row — which is worse for a refusal gate
// than the false positive it fixes. A cue word or a "0" directly in front of
// the marker is therefore NOT enough on its own; what sits in front of the
// cue, and what follows the marker, must also read as a count or a negation.
//
//   - Word cue: "no", "not" or "zero", then only ASCII whitespace, then the
//     marker. It does NOT excuse when the text before the cue ends in "?"
//     (an answer: "available? no HELD"), "=" or "|" (a field or table-cell
//     value: "runner=no HELD", "| no HELD |"), or ":" (a field value:
//     "row 3 green: no HELD", "row 3: not HELD") — unless that colon closes
//     a count label (heldCountLabelRe: "summary: no could-not-check").
//   - Zero cue: a standalone "0", then only ASCII whitespace, then the marker.
//     It excuses only in a COUNT position: at the start of the line, right
//     after a count label and its colon ("summary: 0 HELD"), or right after
//     ", " / "; " that closes another count item ("7 PASS, 0 HELD"). An exit
//     code ("exit 0", "exit: 0", "rc: 0", "exit codes: 1, 0"), a row label
//     ("row 0"), a decimal or version ("2.0", "v1.0") or a parenthesis
//     ("(0 HELD)") is not a count position and never excuses.
//   - Hold reason: an occurrence followed by a hold reason
//     (heldReasonAfterRe: "HELD pending runner", "HELD until …", "HELD for
//     human read") is refused even behind a valid cue — a negated or zero
//     count that also gives a reason for holding contradicts itself.
//   - Struck text: a struck-through span (`~~…~~`) removed between a cue and
//     its marker is replaced by a sentinel for this check, so struck text can
//     never join a cue to a marker ("not ~~yet green, still~~ HELD").
//
// Checked per OCCURRENCE (on the text immediately around it, not anywhere on
// the line), so a negated or zero-counted mention never excuses a different,
// genuine occurrence elsewhere on the same line or another line. Matching is
// ASCII: a Unicode lookalike, a non-breaking space or markup between cue and
// marker ("no **HELD**") does not match and so refuses.
var (
	heldWordCueRe = regexp.MustCompile(`(?i)\b(?:no|not|zero)[ \t]+$`)
	heldZeroCueRe = regexp.MustCompile(`\b0[ \t]+$`)
	// heldCountLabelRe is a count/summary label closed by a colon, as the
	// text before a cue ends. Deliberately a short allowlist: an exit,
	// status or result label is NOT a count label.
	heldCountLabelRe = regexp.MustCompile(`(?i)\b(?:summary|totals?|counts?|tally):$`)
	// heldCountItemRe is a prior count item closed by "," or ";" ("7 PASS,").
	heldCountItemRe = regexp.MustCompile(`(?i)\b\d+[ \t]+[a-z][a-z-]*[,;]$`)
	// heldReasonAfterRe is a hold reason directly after the marker.
	heldReasonAfterRe = regexp.MustCompile(`(?i)^[ \t]*(?:pending|awaiting|until|because|blocked|due|for)\b`)
)

// struckSentinel stands in for a removed struck span in the negation lookback.
const struckSentinel = "\x00"

// heldOccurrenceNegated reports whether the HELD/could-not-check occurrence at
// clean[h[0]:h[1]] is a negation or zero count rather than a live disposition,
// under the rules on heldOrCouldNotCheckRe above. cuts are the offsets in
// clean where struck spans were removed.
func heldOccurrenceNegated(clean string, cuts []int, h []int) bool {
	if heldReasonAfterRe.MatchString(clean[h[1]:]) {
		return false
	}
	var b strings.Builder
	prev := 0
	for _, c := range cuts {
		if c > h[0] {
			break
		}
		b.WriteString(clean[prev:c])
		b.WriteString(struckSentinel)
		prev = c
	}
	b.WriteString(clean[prev:h[0]])
	lookback := b.String()

	if loc := heldWordCueRe.FindStringIndex(lookback); loc != nil {
		before := strings.TrimRight(lookback[:loc[0]], " \t")
		switch {
		case before == "":
			return true
		case strings.HasSuffix(before, ":"):
			return heldCountLabelRe.MatchString(before)
		case strings.HasSuffix(before, "?"), strings.HasSuffix(before, "="),
			strings.HasSuffix(before, "|"), strings.HasSuffix(before, struckSentinel):
			return false
		}
		return true
	}
	if loc := heldZeroCueRe.FindStringIndex(lookback); loc != nil {
		before := strings.TrimRight(lookback[:loc[0]], " \t")
		return before == "" || heldCountLabelRe.MatchString(before) || heldCountItemRe.MatchString(before)
	}
	return false
}

// stripStruck removes struck-through spans from line, returning the cleaned
// text and the offsets in it where a span was removed.
func stripStruck(line string) (string, []int) {
	locs := strikethroughRe.FindAllStringIndex(line, -1)
	if locs == nil {
		return line, nil
	}
	var b strings.Builder
	cuts := make([]int, 0, len(locs))
	prev := 0
	for _, l := range locs {
		b.WriteString(line[prev:l[0]])
		cuts = append(cuts, b.Len())
		prev = l[1]
	}
	b.WriteString(line[prev:])
	return b.String(), cuts
}

// verifyPassHeldContradiction reports whether evidence both carries a strict
// hasVerifyPass marker AND, on some line that is not a genuinely routed
// deferral, also says HELD or could-not-check. The first offending line is
// returned for the caller's message.
//
// A **VERIFY: PASS** line is NOT a flip signal
// on its own when the same Evidence entry contradicts it this way — the model
// autoflip (autoflip.go's decideModelFlip) and the verify-gate card/closeVerify
// below both refuse on a true return rather than trusting the whole-brief
// marker. A row the verifier explicitly routed to a follow-up does NOT
// contradict the marker: that row was knowingly excluded from the PASS, not
// silently left unsettled. "Routed" is the SAME shape unrun.go requires (its
// routingKeywordRe + routingRefRe pair) — a routing phrase such as "deferred
// to" PLUS a corroborating reference — never a bare substring "deferred", so
// negated prose ("NOT deferred, still broken") cannot suppress the
// contradiction: the gate fails closed.
//
// The routing check is per OCCURRENCE, not per line: a line can carry a
// genuinely routed disposition and a second, unrelated, un-routed
// HELD/could-not-check mention at once (e.g. "deferred to X; separately, the
// smoke run HELD, no runner online"), and a routing keyword+reference that
// merely appears somewhere on the line does not say WHICH occurrence it
// corroborates. Each HELD/could-not-check occurrence is cleared only by a
// routing keyword AND a corroborating reference that occur AT OR AFTER its
// own position — never by routing tokens stated only before it.
//
// Fenced code, blockquotes and struck-through spans are stripped first — the
// same hygiene lastVerifyVerdict applies — so a marker QUOTED inside one of
// those is not read as a live disposition.
func verifyPassHeldContradiction(evidence string) (bool, string) {
	if !hasVerifyPass(evidence) {
		return false, ""
	}
	return unroutedHeldLine(evidence)
}

// unroutedHeldLine is verifyPassHeldContradiction's line scan WITHOUT the
// strict-PASS precondition: it reports the first line that says HELD or
// could-not-check on an occurrence not genuinely routed to a follow-up AND
// not negated or zero-counted (heldOccurrenceNegated), under exactly the hygiene and
// routing rules documented on verifyPassHeldContradiction above (which is
// this scan behind hasVerifyPass, unchanged). It exists for a caller whose
// PASS claim is carried by something other than a strict marker —
// closeVerify's `verified` path, where the README row itself already asserts
// the pass — so that caller's read cannot be switched off by how (or
// whether) the marker was written.
func unroutedHeldLine(evidence string) (bool, string) {
	inFence := false
	for _, line := range strings.Split(evidence, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, ">") {
			continue
		}
		clean, cuts := stripStruck(line)
		heldLocs := heldOrCouldNotCheckRe.FindAllStringIndex(clean, -1)
		if heldLocs == nil {
			continue
		}
		// Only a GENUINELY routed occurrence is excluded — the exact shape
		// unrun.go treats as a routed deferral: a routing phrase ("deferred
		// to", a follow-up/tracking keyword) AND a corroborating reference
		// (#N, a stream/NN id, or /issues/N). A bare or negated "deferred"
		// ("NOT deferred", "deferred? no") carries no such reference and so
		// still contradicts the PASS — the gate fails CLOSED, not open.
		//
		// A whole-line "does this line contain routing tokens anywhere"
		// test is not enough: a genuinely routed disposition and a second,
		// unrelated, un-routed HELD/could-not-check mention can share one
		// physical line (an ordinary shape — a verifier's Result cell often
		// reads "deferred to X; separately, the smoke run HELD, no runner
		// online"), and neither routingKeywordRe nor routingRefRe is bound
		// to which occurrence it corroborates. So each occurrence is judged
		// on its own: it is cleared only by a routing keyword AND a
		// corroborating reference that occur AT OR AFTER its own position —
		// never by routing tokens stated only BEFORE it. "row N is HELD ...
		// deferred per X" (the deferral resolves an already-stated hold)
		// still clears; "row N deferred per X ... [and] HELD ..." (a hold
		// stated only after the deferral) does not — that hold is an
		// independent claim the deferral could not have been about, and is
		// refused exactly like an un-routed mention with no deferral at
		// all. This is ordering, not bare same-line proximity.
		keywordLocs := routingKeywordRe.FindAllStringIndex(clean, -1)
		refLocs := routingRefRe.FindAllStringIndex(clean, -1)
		for _, h := range heldLocs {
			// A negated or zero-counted occurrence ("no could-not-check",
			// "summary: 0 HELD") is not a live disposition at all — it is
			// excused outright, the same as a routed one, without needing a
			// routing keyword+reference. Judged on this occurrence's own
			// surroundings only (heldOccurrenceNegated), so a negated mention
			// never excuses a different, genuine occurrence elsewhere.
			if heldOccurrenceNegated(clean, cuts, h) {
				continue
			}
			keywordAfter := false
			for _, k := range keywordLocs {
				if k[0] >= h[0] {
					keywordAfter = true
					break
				}
			}
			refAfter := false
			for _, r := range refLocs {
				if r[0] >= h[0] {
					refAfter = true
					break
				}
			}
			if keywordAfter && refAfter {
				continue // knowingly routed to a named follow-up — excluded from the PASS, not contradicting it
			}
			return true, strings.TrimSpace(clean)
		}
	}
	return false, ""
}

// evidenceVerifierInfo extracts the date and runner from the first data row of an
// Evidence table. Returns empty strings when no Date or Runner column is found.
// The Evidence table convention is:
//
//	| # | Command | Exit | Result | Date | Runner |
//	|---|---------|------|--------|------|--------|
//	| 1 | ...     | 0    | ...    | 2026-07-09 | opus-verifier |
//
// Because splitRow splits on an UNESCAPED | (commands often contain an unescaped
// pipe character, which GFM and splitRow alike read as a real cell delimiter),
// the column indices from the header may not align with data rows. This function
// works around that by reading Date and Runner from the RIGHTMOST two cells of
// each data row: the last cell is Runner (by convention), second-to-last is Date.
// It validates against the header row to confirm the table structure is recognized.
func evidenceVerifierInfo(evidence string) (date, runner string) {
	stripped := htmlCommentRe.ReplaceAllString(evidence, "")
	lines := strings.Split(stripped, "\n")
	tableFound := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			tableFound = false
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		// Header row (immediately followed by a separator) — confirm this table
		// names Date and Runner columns (anywhere, not at a fixed index).
		if i+1 < len(lines) && separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			hasDate, hasRunner := false, false
			for _, c := range splitRow(line) {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "date":
					hasDate = true
				case "runner":
					hasRunner = true
				}
			}
			tableFound = hasDate && hasRunner
			i++ // skip separator
			continue
		}
		if !tableFound {
			continue
		}
		// Data row: read Date from second-to-last cell, Runner from last cell.
		// This survives splitRow miscounting when cell content contains |.
		cells := splitRow(line)
		if len(cells) < 2 {
			continue
		}
		d := strings.TrimSpace(cells[len(cells)-2])
		r := strings.TrimSpace(cells[len(cells)-1])
		if d != "" && d != "—" && r != "" {
			return d, r
		}
	}
	return "", ""
}

// unrunRowsText returns the UNRUN rows from an Evidence section as a single string
// (one row per line), or "" when no row contains UNRUN. Used for the prominent
// rendering on the implemented-at-gate cards.
func unrunRowsText(evidence string) string {
	if !strings.Contains(strings.ToUpper(evidence), "UNRUN") {
		return ""
	}
	stripped := htmlCommentRe.ReplaceAllString(evidence, "")
	lines := strings.Split(stripped, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.Contains(strings.ToUpper(trimmed), "UNRUN") {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}

// prLinks extracts distinct pull-request markdown links from a brief body (any
// link whose URL contains "/pull/"), preserving first-seen order. These are the
// PRs that implemented the brief, recorded in its Evidence.
func prLinks(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		text, url := m[1], m[2]
		if !strings.Contains(url, "/pull/") || seen[url] {
			continue
		}
		seen[url] = true
		out = append(out, fmt.Sprintf("[%s](%s)", text, url))
	}
	return out
}

// renderVerifyBody builds the self-contained markdown issue body for a brief
// awaiting human sign-off. Fully offline: everything is lifted from the brief
// file plus the static repo slug.
func renderVerifyBody(root string, bf *BriefFile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", verifyMarker(bf.Brief))
	fmt.Fprintf(&b, "## Human sign-off required — %s\n\n", bf.Brief)
	fmt.Fprintf(&b, "**%s**\n\n", bf.Title)

	// gate-why (gate-why-rationale): the authored reason THIS brief is gated,
	// surfaced above the mechanical yes-key list so the human reads a real
	// rationale first. Omitted (no error) when absent — pre-backfill briefs
	// simply fall back to the Gate reason line below.
	if gw := strings.TrimSpace(bf.GateWhy); gw != "" {
		fmt.Fprintf(&b, "> **Why you're being asked to sign off:** %s\n\n", gw)
	}

	// why: the brief's VALUE rationale — surfaced above the
	// mechanical gate-reason line so a human reads the real motivation first.
	// Omitted (no error) when absent — pre-backfill briefs simply show no why.
	if w := strings.TrimSpace(bf.Why); w != "" {
		fmt.Fprintf(&b, "> **Why this work exists:** %s\n\n", w)
	}
	reasons := gateReasons(bf.Risk)
	fmt.Fprintf(&b, "**Gate reason** — `gate: human` because these risk answers are `yes`: %s.\n\n",
		strings.Join(reasons, ", "))

	body := briefBody(bf.Path)
	if prs := prLinks(body); len(prs) > 0 {
		fmt.Fprintf(&b, "**Implemented by:** %s\n\n", strings.Join(prs, ", "))
	}
	rel, err := filepath.Rel(root, bf.Path)
	if err != nil {
		rel = bf.Path
	}
	fmt.Fprintf(&b, "**Brief:** [%s](https://github.com/%s/blob/main/%s)\n\n",
		bf.Brief, verifyRepoSlug(), filepath.ToSlash(rel))

	if verify := strings.TrimSpace(extractSectionByPrefix(body, "Verify")); verify != "" {
		fmt.Fprintf(&b, "### Verify\n\n%s\n\n", verify)
	}
	if ev := strings.TrimSpace(bf.Evidence); ev != "" {
		fmt.Fprintf(&b, "### Recorded results (Evidence)\n\n%s\n\n", ev)
	}

	b.WriteString("### Before you close\n\n")
	b.WriteString("- [ ] Verify results reproduce / acceptable\n")
	b.WriteString("- [ ] no unresolved findings against this brief\n")
	b.WriteString("- [ ] I accept this brief as done\n\n")
	b.WriteString("Closing this issue marks the brief `done` (Reviewed: `" + verifyReviewer +
		"`). Only a human closer is honored.\n")
	return b.String()
}

// renderImplementedGateBody builds the self-contained markdown issue body for a
// gate:human brief that is stuck at `implemented` with a recorded model verify
// pass. It is DISTINGUISHED from the ordinary verified→done card: closing advances
// implemented→verified→done in one step; UNRUN/deferred rows are surfaced prominently;
// trade-offs and prior-state remediation are explicitly prompted.
//
// The irreversible flag only changes the heading wording. Both classes reach this
// card by the same route (implemented + recorded **VERIFY: PASS**) and close the
// same way — the Verified cell is stamped from the recorded model run, the Reviewed
// cell from the human closer — so the body is otherwise identical.
func renderImplementedGateBody(root string, bf *BriefFile, irreversible bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", verifyMarker(bf.Brief))
	if irreversible {
		fmt.Fprintf(&b, "## Human sign-off required (irreversible) — %s\n\n", bf.Brief)
	} else {
		fmt.Fprintf(&b, "## Human sign-off required (verified-by-model at implemented) — %s\n\n", bf.Brief)
	}
	fmt.Fprintf(&b, "**%s**\n\n", bf.Title)

	// Explain the one-step advance: closing moves implemented→verified→done.
	date, runner := evidenceVerifierInfo(bf.Evidence)
	fmt.Fprintf(&b, "**Closing this issue advances `implemented → verified → done` in one step.** ")
	if date != "" && runner != "" {
		fmt.Fprintf(&b, "The recorded model verify pass (Evidence: `%s %s`) becomes the Verified stamp; the human closer becomes the Reviewed stamp.\n\n", date, runner)
	} else {
		fmt.Fprintf(&b, "The recorded model verify pass becomes the Verified stamp; the human closer becomes the Reviewed stamp.\n\n")
	}

	// gate-why
	if gw := strings.TrimSpace(bf.GateWhy); gw != "" {
		fmt.Fprintf(&b, "> **Why you're being asked to sign off:** %s\n\n", gw)
	}

	// why
	if w := strings.TrimSpace(bf.Why); w != "" {
		fmt.Fprintf(&b, "> **Why this work exists:** %s\n\n", w)
	}

	reasons := gateReasons(bf.Risk)
	fmt.Fprintf(&b, "**Gate reason** — `gate: human` because these risk answers are `yes`: %s.\n\n",
		strings.Join(reasons, ", "))

	body := briefBody(bf.Path)
	if prs := prLinks(body); len(prs) > 0 {
		fmt.Fprintf(&b, "**Implemented by:** %s\n\n", strings.Join(prs, ", "))
	}
	rel, err := filepath.Rel(root, bf.Path)
	if err != nil {
		rel = bf.Path
	}
	fmt.Fprintf(&b, "**Brief:** [%s](https://github.com/%s/blob/main/%s)\n\n",
		bf.Brief, verifyRepoSlug(), filepath.ToSlash(rel))

	if verify := strings.TrimSpace(extractSectionByPrefix(body, "Verify")); verify != "" {
		fmt.Fprintf(&b, "### Verify\n\n%s\n\n", verify)
	}
	if ev := strings.TrimSpace(bf.Evidence); ev != "" {
		fmt.Fprintf(&b, "### Recorded results (Evidence)\n\n%s\n\n", ev)
	}

	// UNRUN/deferred rows — surfaced prominently.
	if ur := unrunRowsText(bf.Evidence); ur != "" {
		b.WriteString("### Deferred / UNRUN rows\n\n")
		b.WriteString("> Closing accepts these as deferred, or run them first:\n\n")
		fmt.Fprintf(&b, "%s\n\n", ur)
	}

	// Trade-offs section
	b.WriteString("### Trade-offs\n\n")
	tradeoffs := strings.TrimSpace(extractSectionByPrefix(body, "Trade-offs"))
	if tradeoffs != "" {
		fmt.Fprintf(&b, "%s\n\n", tradeoffs)
	} else {
		b.WriteString("> **TRADE-OFFS: not provided** — the brief has no `## Trade-offs` section. A gate:human card should carry a \"Why we want it / What it limits\" analysis.\n\n")
	}

	// Prior-state remediation prompt for risk-classed (money-path) briefs.
	if bf.Gate == "human" || anyRiskYes(bf.Risk) {
		b.WriteString("### Prior-state remediation\n\n")
		b.WriteString("**Does this fix leave prior corrupted state that needs remediation?**\n\n")
		// Check if trade-offs section addresses prior state.
		if strings.Contains(strings.ToLower(tradeoffs), "prior") ||
			strings.Contains(strings.ToLower(tradeoffs), "remediation") ||
			strings.Contains(strings.ToLower(tradeoffs), "corrupt") {
			b.WriteString("(See Trade-offs section above for the prior-state assessment.)\n\n")
		} else {
			b.WriteString("> **PRIOR-STATE: unassessed** — the `## Trade-offs` section does not address whether this fix leaves prior corrupted state. Every money-path gate card must answer this question (yes+what / no+why).\n\n")
		}
	}

	b.WriteString("### Before you close\n\n")
	b.WriteString("- [ ] Model verify pass is acceptable (Evidence reviewed above)\n")
	if unrunRowsText(bf.Evidence) != "" {
		b.WriteString("- [ ] UNRUN/deferred rows accepted as-is, or run first\n")
	}
	b.WriteString("- [ ] Trade-offs understood and accepted\n")
	if bf.Gate == "human" || anyRiskYes(bf.Risk) {
		b.WriteString("- [ ] Prior-state remediation assessed\n")
	}
	b.WriteString("- [ ] I accept this brief as done (implemented → verified → done)\n\n")
	b.WriteString("Closing this issue marks the brief `done` (Verified: model verifier; Reviewed: `" +
		verifyReviewer + "`). Only a human closer is honored.\n")
	return b.String()
}

// verifyIssues computes the newly-eligible verify-gate issues: briefs that are
// gate: human, whose README-row status is exactly `verified`, and whose marker
// is not already in the supplied existing-markers set. Also emits for ANY gate:human
// brief at `implemented` whose Evidence records the strict model verify pass marker
// — so the guarantee "a recorded PASS ⇒ a verify-gate issue exists" holds without
// waiting on a manual implemented→verified README flip. (This generalizes the
// original irreversible-only chicken-and-egg path: irreversible briefs cannot reach
// `verified` without a human sign-off, but the same one-step surfacing is correct for
// every human-gated brief carrying a recorded pass.) Output is sorted by brief id for
// deterministic emission.
func verifyIssues(root string, streams []*Stream, existing map[string]bool) []verifyIssue {
	out := []verifyIssue{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed (reported elsewhere) or legacy/opted-out
			}
			if bf.Gate != "human" {
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			row := findRow(s, num)
			if row == nil {
				continue
			}
			// Path A: standard verified → done gate (existing behavior).
			if row.Status == "verified" {
				// Two-touch model (human:<name>'s decision): the
				// done-close is a DISTINCT acceptance sign-off, separate from the
				// verified-stage human review. It therefore fires for EVERY
				// gate:human + verified brief — including those already carrying a
				// human:<name> at the verified stage. We do NOT filter on
				// hasHumanReviewer; closeVerify appends the acceptance touch without
				// clobbering the prior sign-off.
				marker := verifyMarker(bf.Brief)
				if existing[marker] {
					continue
				}
				out = append(out, verifyIssue{
					Brief:  bf.Brief,
					Title:  issueTitle("verify-gate: ", bf.Brief, bf.Title),
					Labels: []string{"verify-gate"},
					Marker: marker,
					Body:   renderVerifyBody(root, bf),
				})
				continue
			}
			// Path B: any gate:human brief stuck at implemented with a recorded model
			// verify pass. The implemented→verified README flip is a manual verify-desk
			// action that does not happen reliably, so a human-gated brief with a
			// recorded pass would otherwise never surface. Emit it here so the human
			// sign-off can advance implemented→verified→done in one step. The strict
			// hasVerifyPass marker is the fail-closed gate — WHICH briefs are eligible
			// is loosened here; WHAT evidence is required is not.
			if row.Status == "implemented" && hasVerifyPass(bf.Evidence) {
				if held, _ := verifyPassHeldContradiction(bf.Evidence); held {
					// A PASS line is not a flip signal while a non-deferred
					// row still reads HELD/could-not-check — no card is
					// raised on the strength of the marker alone.
					continue
				}
				marker := verifyMarker(bf.Brief)
				if existing[marker] {
					continue
				}
				out = append(out, verifyIssue{
					Brief:  bf.Brief,
					Title:  issueTitle("verify-gate: ", bf.Brief, bf.Title),
					Labels: []string{"verify-gate"},
					Marker: marker,
					Body:   renderImplementedGateBody(root, bf, bf.Risk["irreversible"] == "yes"),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Brief < out[j].Brief })
	return out
}

// runVerifyIssues is the --verify-issues entrypoint: emit the eligible-brief
// JSON array to stdout. Returns a process exit code.
func runVerifyIssues(root, markersPath string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	existing, err := loadExistingMarkers(markersPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: reading markers:", err)
		return 1
	}
	issues := verifyIssues(root, streams, existing)
	enc, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}

// setCell replaces a table cell's trimmed content with newVal while preserving
// its surrounding whitespace, so only the changed token differs in the diff.
func setCell(cell, newVal string) string {
	lead := cell[:len(cell)-len(strings.TrimLeft(cell, " "))]
	trail := cell[len(strings.TrimRight(cell, " ")):]
	return lead + newVal + trail
}

// flipRowToDone rewrites the briefs-table row for brief number num: status →
// done and the Reviewed cell → reviewedStamp. When verifiedStamp is non-empty,
// it also sets the Verified cell (used by the implemented one-step advance
// that stamps both cells in a single write). Every other cell's exact text is
// preserved. Returns an error if the table or row is not found.
func flipRowToDone(raw, num, reviewedStamp, verifiedStamp string) (string, error) {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cols := splitRow(line)
		idx := map[string]int{}
		for j, c := range cols {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		if _, ok := idx["#"]; !ok {
			continue
		}
		if _, ok := idx["brief"]; !ok {
			continue
		}
		if _, ok := idx["status"]; !ok {
			continue
		}
		for k := i + 2; k < len(lines); k++ { // i+1 is the |---| separator
			if !strings.HasPrefix(strings.TrimSpace(lines[k]), "|") {
				break
			}
			cells := splitRow(lines[k])
			if len(cells) < len(cols) {
				continue
			}
			if strings.TrimSpace(cells[idx["#"]]) != num {
				continue
			}
			cells[idx["status"]] = setCell(cells[idx["status"]], "done")
			// Set the Verified cell when provided (implemented one-step path).
			if verifiedStamp != "" {
				if vi, ok := idx["verified"]; ok {
					cells[vi] = setCell(cells[vi], verifiedStamp)
				}
			}
			if ri, ok := idx["reviewed"]; ok {
				// Strictly ADDITIVE (two-touch model): never overwrite existing
				// Reviewed content. If a prior sign-off is recorded (e.g. the
				// verified-stage human review), preserve it byte-for-byte and
				// append a distinguishable acceptance touch; if the cell is empty
				// / em-dash, set the acceptance sign-off directly.
				//
				// The composed cell must ALSO satisfy methodology/19's done-shape
				// lint (brieffile.go: status done needs verifiedCellRe to match,
				// i.e. the cell must START with "YYYY-MM-DD "). When the prior
				// sign-off is itself already dated (e.g. "2026-07-08 human:alex"),
				// appending after it keeps that leading date intact, so existing
				// stays at the front and the acceptance touch trails it. But a
				// prior sign-off can legitimately be UNDATED — a bare "human:alex"
				// is a sanctioned Reviewed-cell value at the verified stage
				// (hasHumanReviewer does not require a date) —
				// and appending after that leaves the cell starting with "human:"
				// instead of a date, which is exactly what the undated-Reviewed-cell case broke
				// (close-verify wrote a value its own --lint then rejected).
				// In that case the new dated stamp must lead instead, with the
				// undated prior preserved as a trailing note.
				existing := strings.TrimSpace(cells[ri])
				switch {
				case normalizeMark(existing) == "":
					cells[ri] = setCell(cells[ri], reviewedStamp)
				case verifiedCellRe.MatchString(existing):
					cells[ri] = setCell(cells[ri], existing+"; accepted "+reviewedStamp)
				default:
					cells[ri] = setCell(cells[ri], reviewedStamp+"; prior "+existing)
				}
			}
			lines[k] = "|" + strings.Join(cells, "|") + "|"
			return strings.Join(lines, "\n"), nil
		}
		return "", fmt.Errorf("no row for #%s in briefs table", num)
	}
	return "", fmt.Errorf("no briefs table found")
}

// closeVerify flips a brief's README row verified → done and stamps the Reviewed
// cell with a dated human sign-off.
//
// Two-stamp model (#1170): before it writes, it reads the Verified cell the done
// row would carry — the README cell on the `verified` path, or the cell it would
// stamp from the brief file's Evidence on the `implemented` path — and the
// brief's Evidence rows, and REFUSES (no write) when either is below the
// methodology/19 verifier floor. The routine drain may verify a human-gated
// brief at a local tier and flip it `verified`; the human done close needs ONE
// floor-tier re-verify stamp landed first. Without this read the flip lands on
// main and its own --lint reddens it after the human has already signed. For ANY gate:human brief at `implemented`
// whose Evidence records a model verify pass, it accepts `implemented` and advances
// in one step: Verified cell stamped from the recorded model verifier + its date,
// Reviewed cell stamped `human:<closer>` + the close date — the recorded independent
// run, not the close time, is what the Verified cell attests. It refuses (error, NO
// write) for any other state, and for an implemented brief lacking either the strict
// **VERIFY: PASS** marker or a Date/Runner Evidence row (fail-closed). On BOTH
// paths it also refuses a PASS contradicted by an un-routed HELD/could-not-check
// row (closeVerifyHeldRefusal); on the verified path it further refuses when the
// most recent recorded verdict is a FAIL (closeVerifyFailRefusal). It never
// touches STATUS.md (single-writer rule) — status-regen regenerates it on the
// resulting push.
func closeVerify(root, briefID string, now time.Time) error {
	streams, _, err := loadStreams(root)
	if err != nil {
		return err
	}
	// Accept either brief-key form: a brief-v1 <stream>/<NN> id or a brief-v2
	// <cell>:<repo>:<stream>:<NN> id (issue #804). Both name one brief in this
	// tree; normalize to the canonical <stream>/<NN> the row lookup below uses.
	briefID = normalizeBriefKey(briefID)
	streamName, num, ok := strings.Cut(briefID, "/")
	if !ok || streamName == "" || num == "" {
		return fmt.Errorf("brief id %q is not a <stream>/<NN> or <cell>:<repo>:<stream>:<NN> id", briefID)
	}
	var s *Stream
	for _, st := range streams {
		if st.Name == streamName {
			s = st
			break
		}
	}
	if s == nil {
		return fmt.Errorf("unknown stream %q", streamName)
	}

	var bf *BriefFile
	for _, path := range briefFilePaths(s) {
		p, ok, err := parseBriefFile(path)
		if err != nil || !ok {
			continue
		}
		if _, n, okName := expectedBriefID(path); okName && n == num {
			bf = p
			break
		}
	}
	if bf == nil {
		return fmt.Errorf("no brief-v1 file found for %s", briefID)
	}
	if bf.Gate != "human" {
		return fmt.Errorf("refusing: brief %s gate is %q, not human — not a human sign-off gate", briefID, bf.Gate)
	}
	row := findRow(s, num)
	if row == nil {
		return fmt.Errorf("no README row for %s", briefID)
	}

	readme := filepath.Join(s.Dir, "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		return err
	}
	// Date-first, matching the repo/CLAUDE.md Reviewed-cell convention
	// ("YYYY-MM-DD human:alex") and every existing row.
	reviewedStamp := now.Format("2006-01-02") + " " + verifyReviewer

	switch row.Status {
	case "verified":
		// Standard path: verified → done. A `verified` row is no licence to
		// skip the Evidence read the implemented path makes: a brief flipped
		// to `verified` over an unresolved hold, or whose record shows a FAIL
		// no later strict PASS answers, must not be closed to `done` with that
		// record standing. So the SAME HELD/could-not-check scan the
		// implemented path runs runs here — keyed on the row's own `verified`
		// claim, NOT on how (or whether) a PASS marker was written — plus the
		// FAIL read, both BEFORE the floor read on the cell the done row will
		// carry (two-stamp model).
		if err := closeVerifyHeldRefusal(briefID, row.Status, bf.Evidence); err != nil {
			return err
		}
		if err := closeVerifyFailRefusal(briefID, row.Status, bf.Evidence); err != nil {
			return err
		}
		if err := closeVerifyFloorRefusal(briefID, bf, row.Verified); err != nil {
			return err
		}
		updated, err := flipRowToDone(string(raw), num, reviewedStamp, "")
		if err != nil {
			return fmt.Errorf("%s: %w", readme, err)
		}
		return os.WriteFile(readme, []byte(updated), 0o644)

	case "implemented":
		// One-step path: the implemented→verified README flip is a manual action
		// that does not happen reliably, so verifyIssues surfaces any gate:human
		// brief at `implemented` with a recorded model verify pass. Accept
		// `implemented` here on the same condition — a recorded pass in Evidence —
		// and advance in one write. The strict marker plus the Date/Runner row are
		// the fail-closed gate; loosening WHICH briefs qualify never loosens WHAT
		// evidence is required.
		if !hasVerifyPass(bf.Evidence) {
			return fmt.Errorf("refusing: brief %s status is %q (not verified) and Evidence has no **VERIFY: PASS** marker — a human-gated brief needs a recorded model verify pass before the human sign-off can advance it", briefID, row.Status)
		}
		if err := closeVerifyHeldRefusal(briefID, row.Status, bf.Evidence); err != nil {
			return err
		}
		date, runner := evidenceVerifierInfo(bf.Evidence)
		if date == "" || runner == "" {
			return fmt.Errorf("refusing: brief %s has **VERIFY: PASS** but no verifier date/runner found in Evidence table — need a table with Date and Runner columns to stamp the Verified cell", briefID)
		}
		verifiedStamp := date + " " + runner
		// The cell this path is about to WRITE comes from the brief file's
		// Evidence, so the floor read is on that computed stamp (two-stamp model).
		if err := closeVerifyFloorRefusal(briefID, bf, verifiedStamp); err != nil {
			return err
		}
		updated, err := flipRowToDone(string(raw), num, reviewedStamp, verifiedStamp)
		if err != nil {
			return fmt.Errorf("%s: %w", readme, err)
		}
		return os.WriteFile(readme, []byte(updated), 0o644)

	default:
		return fmt.Errorf("refusing: brief %s status is %q, not verified (or implemented with a recorded **VERIFY: PASS**) — nothing to sign off", briefID, row.Status)
	}
}

// closeVerifyHeldRefusal is the HELD/could-not-check read a human done close
// makes BEFORE it writes, on BOTH starting states (verified and implemented):
// a pass claim is not a flip signal while a row not genuinely routed to a
// follow-up still reads HELD/could-not-check.
//
// The scan is unroutedHeldLine — verifyPassHeldContradiction's own line scan,
// same hygiene, same routing rule — run WITHOUT the strict-marker
// precondition. On the implemented path that changes nothing: closeVerify has
// already refused any Evidence lacking the strict **VERIFY: PASS** marker, so
// the two reads coincide. On the verified path it is the point: the README row
// already claims a pass, so a loose-form marker (`**Non-implementer verifier
// run — VERIFY: PASS**`) or no marker at all must not switch the read off.
//
// Where the strict marker is present the refusal text is ONE sentence for both
// paths, so the two can never drift into refusing differently; the
// no-strict-marker variant is reachable from the verified path only.
//
// Supersession is NOT inferred: a HELD/could-not-check line from an earlier
// run stays live after a later run executes that row green, because nothing in
// the Evidence convention ties the later row to the earlier one. The verifier
// who resolves a hold strikes the earlier line through (`~~…~~`, which the scan
// strips) or routes it to a named follow-up; an unstruck, unrouted hold refuses
// by design.
func closeVerifyHeldRefusal(briefID, status, evidence string) error {
	held, why := unroutedHeldLine(evidence)
	if !held {
		return nil
	}
	if hasVerifyPass(evidence) {
		return fmt.Errorf("refusing: brief %s carries **VERIFY: PASS** but Evidence also reads %q on a row not marked deferred — a PASS marker is not a flip signal while a non-deferred row still says HELD/could-not-check", briefID, why)
	}
	return fmt.Errorf("refusing: brief %s status is %q, which claims a pass, but Evidence reads %q on a row not marked deferred and carries no strict **VERIFY: PASS** marker — a %q row is not a flip signal while a non-deferred row still says HELD/could-not-check", briefID, status, why, status)
}

// closeVerifyFailRefusal refuses a human done close whose Evidence records a
// FAIL that no later strict **VERIFY: PASS** marker answers. It refuses when
// EITHER read says so:
//
//   - lastVerifyVerdict is FAIL (the most recent verdict token of any form);
//   - verdictFailAfterStrictPass: some FAIL token of any form occurs after the
//     last strict **VERIFY: PASS** marker (or anywhere, when there is none).
//
// A brief that failed, was reworked and then recorded a strict PASS closes;
// one whose latest recorded run failed does not, whatever its README row says.
//
// It runs on the verified path. The implemented path needs no separate read
// for the FAIL-only case — it already refuses any Evidence without a strict
// **VERIFY: PASS** marker — and is deliberately left unchanged here.
//
// Why two reads. lastVerifyVerdict's stated limit — a verdict token inside
// ordinary prose still reads as a verdict — cuts BOTH ways: a prose FAIL
// mention after a real PASS reads as a fail (refuses: the closed direction),
// but a prose PASS mention after a real FAIL ("will record VERIFY: PASS once
// green") reads as a pass, which alone would let the close through.
// verdictFailAfterStrictPass closes that open direction: only the strict bold
// marker can answer a FAIL, and a prose PASS never can. Either read refusing
// refuses; the remedy is a recorded strict **VERIFY: PASS** after the FAIL, or
// striking the superseded FAIL through.
func closeVerifyFailRefusal(briefID, status, evidence string) error {
	if lastVerifyVerdict(evidence) == verdictFail || verdictFailAfterStrictPass(evidence) {
		return fmt.Errorf("refusing: brief %s status is %q but its Evidence records a VERIFY: FAIL that no later strict **VERIFY: PASS** marker answers — a failed run is not a flip signal; re-run the Verify table to a recorded **VERIFY: PASS** before the human sign-off", briefID, status)
	}
	return nil
}

// verdictFailAfterStrictPass reports whether a FAIL verdict token of ANY form
// (verifyVerdictRe) occurs after the last strict bold **VERIFY: PASS** marker
// (verifyVerdictBoldRe), or anywhere when the Evidence has no strict PASS.
// Positions are compared in document order, within a line by offset. Fenced
// code, blockquotes and struck-through spans are stripped first — the same
// hygiene lastVerifyVerdict applies — so a quoted or retracted FAIL is not
// read as a live one. A PASS token that is not in the strict bold form never
// answers a FAIL: that asymmetry is what keeps prose from opening the gate.
func verdictFailAfterStrictPass(evidence string) bool {
	failOpen := false
	inFence := false
	for _, line := range strings.Split(evidence, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, ">") {
			continue
		}
		line = strikethroughRe.ReplaceAllString(line, "")
		type ev struct {
			pos      int
			strictOK bool // a strict bold PASS marker starts here
		}
		var evs []ev
		for _, m := range verifyVerdictBoldRe.FindAllStringSubmatchIndex(line, -1) {
			if line[m[2]:m[3]] == "PASS" {
				evs = append(evs, ev{m[0], true})
			}
		}
		for _, m := range verifyVerdictRe.FindAllStringSubmatchIndex(line, -1) {
			if line[m[2]:m[3]] == "FAIL" {
				evs = append(evs, ev{m[0], false})
			}
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].pos < evs[j].pos })
		for _, e := range evs {
			failOpen = !e.strictOK
		}
	}
	return failOpen
}

// closeVerifyFloorRemedy is the two-stamp remedy every floor refusal names, so
// the card comment the close workflow relays tells the human exactly what lands
// before the card is closed again. It is spelled once here and once in the
// --lint PROBLEM text (brieffile.go); the two must keep saying the same thing.
const closeVerifyFloorRemedy = "a human done close needs a floor-tier re-verify stamp first (two-stamp model, methodology/19): " +
	"re-run the Verify table at a floor-tier runner (a strong-tier model or a confirmed human), append its " +
	"Evidence rows, re-stamp the Verified cell \"YYYY-MM-DD <runner>\" with that pass leading the cell, then " +
	"close the card again"

// closeVerifyFloorRefusal is the verifier-floor read a human done close makes
// BEFORE it writes (two-stamp model, #1170). verifiedCell is the Verified cell
// the done row would carry; bf.Evidence is the brief file's own record of who
// ran each row. It applies the SAME two predicates --lint applies at done
// (verifierFloorFailure on the cell, evidenceFloorFailure on the rows), so a
// flip this accepts is one the lint on main accepts too. The error text names
// the runner, the floor, and the remedy, and always carries the phrase
// "verifier floor" — the close workflow keys its reopen-and-comment branch on
// that phrase, so it must not be reworded away.
//
// Scope is every gate:human brief closeVerify handles, the ruling's literal
// scope: one floor-tier stamp before the human close. That is deliberately ONE
// step wider than the lint, which exempts an irreversible brief from the floor
// because the human-at-verified rule already governs it — a wider refusal here
// can only send a brief back for a re-verify, never land a red flip.
func closeVerifyFloorRefusal(briefID string, bf *BriefFile, verifiedCell string) error {
	if reason, failed := verifierFloorFailure(verifiedCell); failed {
		runner := ""
		if m := verifiedTokenRe.FindStringSubmatch(verifiedCell); m != nil {
			runner = m[1]
		}
		return fmt.Errorf("refusing: brief %s is gate: human but its Verified cell %q names runner %q, which does not clear the verifier floor (%s) — %s",
			briefID, verifiedCell, runner, reason, closeVerifyFloorRemedy)
	}
	if reason, failed := evidenceFloorFailure(bf.Evidence); failed {
		return fmt.Errorf("refusing: brief %s is gate: human but its ## Evidence records rows run only below the verifier floor with no floor-tier re-run curing them (%s) — the Verified cell %q does not speak for those rows — %s",
			briefID, reason, verifiedCell, closeVerifyFloorRemedy)
	}
	return nil
}

// runCloseVerify is the --close-verify entrypoint. Returns a process exit code;
// a refusal (bad state) exits non-zero with NO file write.
func runCloseVerify(root, briefID string) int {
	if err := closeVerify(root, briefID, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Printf("marked %s done (Reviewed: %s)\n", briefID, verifyReviewer)
	return 0
}
