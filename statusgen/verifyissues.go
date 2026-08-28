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

// verifyMarker renders the hidden per-brief marker. It is the first line of the
// issue body (idempotency) and the close-back mapping key.
func verifyMarker(brief string) string {
	return "<!-- verify-gate: " + brief + " -->"
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
		set[strings.TrimSpace(m)] = true
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

// hasVerifyPass reports whether evidence contains the **VERIFY: PASS** marker
// indicating a model verifier has run and passed the brief's Verify table.
//
// DELIBERATELY STRICT, and deliberately NOT the same test as
// lastVerifyVerdict below. This one is a GATE: it decides whether a gate:human
// brief at `implemented` may advance to a human sign-off and whether a verify
// issue is emitted. A gate must fail CLOSED — an Evidence body whose
// pass is written in some looser form is refused, and the fix is to write the
// canonical marker, not to loosen the gate. The two live side by side so the
// asymmetry is visible rather than discovered.
func hasVerifyPass(evidence string) bool {
	return strings.Contains(evidence, "**VERIFY: PASS**")
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

// evidenceVerifierInfo extracts the date and runner from the first data row of an
// Evidence table. Returns empty strings when no Date or Runner column is found.
// The Evidence table convention is:
//
//	| # | Command | Exit | Result | Date | Runner |
//	|---|---------|------|--------|------|--------|
//	| 1 | ...     | 0    | ...    | 2026-07-09 | opus-verifier |
//
// Because splitRow naively splits on | (commands often contain pipe characters),
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
// cell with a dated human sign-off. For ANY gate:human brief at `implemented`
// whose Evidence records a model verify pass, it accepts `implemented` and advances
// in one step: Verified cell stamped from the recorded model verifier + its date,
// Reviewed cell stamped `human:<closer>` + the close date — the recorded independent
// run, not the close time, is what the Verified cell attests. It refuses (error, NO
// write) for any other state, and for an implemented brief lacking either the strict
// **VERIFY: PASS** marker or a Date/Runner Evidence row (fail-closed). It never
// touches STATUS.md (single-writer rule) — status-regen regenerates it on the
// resulting push.
func closeVerify(root, briefID string, now time.Time) error {
	streams, _, err := loadStreams(root)
	if err != nil {
		return err
	}
	streamName, num, ok := strings.Cut(briefID, "/")
	if !ok || streamName == "" || num == "" {
		return fmt.Errorf("brief id %q is not a <stream>/<NN> id", briefID)
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
		// Standard path: verified → done.
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
		date, runner := evidenceVerifierInfo(bf.Evidence)
		if date == "" || runner == "" {
			return fmt.Errorf("refusing: brief %s has **VERIFY: PASS** but no verifier date/runner found in Evidence table — need a table with Date and Runner columns to stamp the Verified cell", briefID)
		}
		verifiedStamp := date + " " + runner
		updated, err := flipRowToDone(string(raw), num, reviewedStamp, verifiedStamp)
		if err != nil {
			return fmt.Errorf("%s: %w", readme, err)
		}
		return os.WriteFile(readme, []byte(updated), 0o644)

	default:
		return fmt.Errorf("refusing: brief %s status is %q, not verified (or implemented with a recorded **VERIFY: PASS**) — nothing to sign off", briefID, row.Status)
	}
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
