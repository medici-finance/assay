package main

// UNRUN as a first-class board state (methodology-metrics/37, absorbing
// F-impl-claims-unproven).
//
// THE POINT — UNRUN IS DERIVED, NEVER DECLARED.
//
// A Verify table enumerates the checks a brief must survive. An Evidence table
// records the checks that were actually run, by whom, on what date. The gap
// between the two is the only honest measure of how much of a brief has been
// proven, and it is COMPUTED here from those two artifacts — not read off a
// flag anybody sets.
//
// That property is the whole design constraint. The prior convention was a
// literal "UNRUN" string a verifier typed into an Evidence cell: informative
// when written, worthless as a board state, because OMITTING it costs nothing.
// A state an author can decline to enter proves nothing about the briefs that
// do not carry it. So the derivation runs the other way round:
//
//	a Verify row is UNRUN unless a completed Evidence row for that row exists.
//
// Silence is UNRUN. Clearing the state requires ADDING an artifact — an
// Evidence row naming the item, dated, with a runner — and no amount of not
// writing something can produce a clean board. The literal "UNRUN"/"deferred"
// marker is still honoured, but only ADDITIVELY: it can move a row INTO the
// unrun set (a verifier saying "I did not run this" is believed), never out of
// it. There is no token, field or phrasing an implementer can use to assert
// that a row was run.
//
// TWO LIFECYCLE STAGES, ONE DERIVATION.
//
// The same computed quantity is read at two points on the board:
//
//   - `implemented` (F-impl-claims-unproven's stage): a brief that has sat at
//     `implemented` past the threshold with ZERO of its Verify rows covered is
//     an uncorroborated claim aging on the record. NOTICE — it prioritises the
//     verify-desk queue, it does not accuse anyone. This is the finding's
//     recommendation 3, expressed as run-coverage rather than "empty Evidence"
//     (strictly stronger: a brief with prose in `## Evidence` and no completed
//     row still reads as zero-coverage).
//
//   - `verified`/`done` (brief 37's stage): an UNRUN RISK-BEARING row — the
//     live/mutating end-to-end check — renders the `‡` board token, and a
//     `done` TRANSITION carrying one with no routed follow-up is a hard
//     PROBLEM.
//
// See docs/streams/methodology-metrics/brief-37-unrun-board-state.md
// ("Scope decision") for why the two stages are one brief and not two.
//
// WHY THE `done` GATE IS SCOPED TO THE TRANSITION.
//
// A brief already `done` on main is history: the transition it should have
// blocked happened before the rule existed. Firing a hard PROBLEM at it now
// buys nothing and creates a live incentive to edit a closed brief's Evidence
// table until the board goes green — which is precisely the falsification this
// check exists to catch (the same reasoning that keeps unfailableRowNotices a
// NOTICE, see main.go). So the gate is evaluated against the merge-base with
// origin/main, exactly like grandfatheredIDs: briefs ALREADY `done`/`verified`
// there are grandfathered to a NOTICE; a brief this branch newly closes is a
// PROBLEM. The backlog stays visible, the transition is what has teeth.
//
// Unresolvable base (no .git, no origin/main) grandfathers EVERYTHING, with a
// NOTICE naming the degradation. That is not fail-open: the gate's subject is a
// transition, and with no base there is no observable transition to gate — the
// check's domain is empty rather than permissive. (grandfatheredIDs fails the
// other way because ID validity is a property of the STATE, decidable with no
// base at all.)

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// staleImplementedThreshold is how long a brief may sit at `implemented` with
// none of its Verify rows corroborated before the board says so. 72h ≈ three
// working days: long enough that a brief merged and queued for the verify-desk
// on a Friday is not flagged before anyone could reasonably have picked it up,
// short enough that F-impl-claims-unproven's failure mode (an uncorroborated
// claim aging quietly on the public record) is surfaced in the same week it
// lands rather than at the next drain.
const staleImplementedThreshold = 72 * time.Hour

var (
	// unrunMarkerRe matches a verifier's explicit "I did not run this" marker in
	// an Evidence row. ADDITIVE ONLY — see the file header: it can push a row
	// into the unrun set, never pull one out.
	//
	// Matched against the row with inline code spans stripped (see
	// evidenceRowComplete) — this regex is meant to find a run-STATUS
	// DECLARATION, an unquoted prose token an author writes ("UNRUN",
	// "deferred"), never verbatim command/output CONTENT that happens to
	// contain the same words: a `grep -n 'DO NOT RUN' …`
	// command cell, or a tool's own `0 failed, 0 skipped` counters, are Evidence
	// content, not the row's status.
	unrunMarkerRe = regexp.MustCompile(`(?i)\b(unrun|not[ -]run|deferred|skipped)\b`)

	// riskBearingRowRe matches the row-level tag that marks a Verify row as the
	// live/mutating end-to-end check. Word-boundaried so "deliver" does not read
	// as "live" and "eventually" does not read as "live".
	riskBearingRowRe = regexp.MustCompile(`(?i)\b(risk-bearing|live|mutating|end-to-end)\b`)

	// routingKeywordRe and routingRefRe together define a ROUTED unrun row: the
	// row must both say it was routed and name where. Two independent
	// conditions, because either alone misfires — a bare "#392" appears in
	// output excerpts all the time, and a bare "follow-up" names nothing
	// actionable. Both are things the author must ADD; the default (say
	// nothing) is unrouted, which is the direction that keeps the gate honest.
	routingKeywordRe = regexp.MustCompile(`(?i)(routed|follow-?ups?|tracked by|tracking|deferred to)`)
	routingRefRe     = regexp.MustCompile(`(#[0-9]+)|(\b[a-z][a-z0-9-]*/[0-9]{2}[a-z]?\b)|(/issues/[0-9]+)`)
)

// unrunLegend is the one-line STATUS.md legend for the `‡` board token. Kept
// beside the derivation it describes so the two cannot drift.
const unrunLegend = "_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it (methodology-metrics/37). UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._"

// verifyItem is one row of a brief's `## Verify` table.
type verifyItem struct {
	ID   string // the "#" cell, or the 1-based ordinal when the table has no "#" column
	Text string // Command + Expect, joined — the text risk-bearing tagging reads
}

// evidenceRow is one row of a table in a brief's `## Evidence` section.
type evidenceRow struct {
	ID     string
	Text   string // the whole raw row
	Date   string
	Runner string
}

// unrunFinding is one Verify row with no completed Evidence row behind it.
type unrunFinding struct {
	ID          string
	RiskBearing bool
	Routed      bool
	Reason      string
}

// parseVerifyItems extracts the rows of a `## Verify` table. It locates the
// header naming Command and Expect (the same anchor verifyTableHasRow uses) and
// reads the data rows below it. Cells are split escape-aware (splitRowEscaped)
// so a command containing `\|` does not shred the row — the same hazard
// verifyrows.go documents at length.
func parseVerifyItems(section string) []verifyItem {
	var items []verifyItem
	cmdIdx, expIdx, numIdx := -1, -1, -1
	ordinal := 0
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			cmdIdx, expIdx, numIdx, ordinal = -1, -1, -1, 0
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowEscaped(line)
		if cmdIdx < 0 || expIdx < 0 {
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				case "#":
					numIdx = j
				}
			}
			continue // the header row is not a data row
		}
		if cmdIdx >= len(cells) || expIdx >= len(cells) {
			continue
		}
		cmd := strings.TrimSpace(cells[cmdIdx])
		exp := strings.TrimSpace(cells[expIdx])
		if normalizeMark(cmd) == "" && normalizeMark(exp) == "" {
			continue
		}
		ordinal++
		id := strconv.Itoa(ordinal)
		if numIdx >= 0 && numIdx < len(cells) {
			if v := normalizeRowID(cells[numIdx]); v != "" {
				id = v
			}
		}
		items = append(items, verifyItem{ID: id, Text: cmd + " " + exp})
	}
	return items
}

// parseEvidenceRows extracts every table row from an `## Evidence` section,
// keyed by row ID. An Evidence section may hold SEVERAL tables (an implementer
// run, then an independent re-run, then a re-verify at a later SHA); every one
// is read and the results are unioned, so a row run by any pass counts as run.
// HTML comments are stripped first — the contract comment is not evidence.
func parseEvidenceRows(section string) map[string][]evidenceRow {
	out := map[string][]evidenceRow{}
	stripped := htmlCommentRe.ReplaceAllString(section, "")
	lines := strings.Split(stripped, "\n")
	numIdx, dateIdx, runnerIdx := -1, -1, -1
	ordinal := 0
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			numIdx, dateIdx, runnerIdx, ordinal = -1, -1, -1, 0
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowEscaped(line)
		// A header row is the one immediately followed by a separator row.
		if i+1 < len(lines) && separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			numIdx, dateIdx, runnerIdx, ordinal = -1, -1, -1, 0
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "#", "item", "row":
					numIdx = j
				case "date":
					dateIdx = j
				case "runner":
					runnerIdx = j
				}
			}
			i++ // skip the separator
			continue
		}
		if len(cells) == 0 {
			continue
		}
		ordinal++
		id := strconv.Itoa(ordinal)
		if numIdx >= 0 && numIdx < len(cells) {
			if v := normalizeRowID(cells[numIdx]); v != "" {
				id = v
			}
		}
		// Date/Runner fall back to the last two cells — the same tolerance
		// evidenceVerifierInfo and evidenceHasIndependentRow already apply, so a
		// table that names no columns still reads.
		date, runner := "", ""
		if dateIdx >= 0 && dateIdx < len(cells) {
			date = strings.TrimSpace(cells[dateIdx])
		} else if len(cells) >= 2 {
			date = strings.TrimSpace(cells[len(cells)-2])
		}
		if runnerIdx >= 0 && runnerIdx < len(cells) {
			runner = strings.TrimSpace(cells[runnerIdx])
		} else if len(cells) >= 1 {
			runner = strings.TrimSpace(cells[len(cells)-1])
		}
		out[id] = append(out[id], evidenceRow{
			ID:     id,
			Text:   line,
			Date:   normalizeMark(date),
			Runner: normalizeMark(runner),
		})
	}
	return out
}

// normalizeRowID trims a row's "#" cell to a bare identifier ("`3`" → "3").
func normalizeRowID(cell string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(cell), "`*_"))
}

// evidenceRowComplete reports whether an Evidence row PROVES its Verify row was
// run: it carries a date and a runner, and is not marked UNRUN/deferred. Every
// clause requires the presence of something written; none can be satisfied by
// omission.
//
// unrunMarkerRe is matched against the row with inline code spans blanked out
// (inlineCodeRe, shared with linkcheck.go), not the raw row text. Command and
// Output/Result cells are conventionally backtick-quoted verbatim content
// (codeSpan in verifyrows.go documents the same convention); a run-status
// declaration is unquoted prose. Scanning the raw row let verbatim CONTENT
// trip the marker — a `grep -n 'DO NOT RUN' <file>` command cell, or a tool's
// own `0 failed, 0 skipped` output — and read a genuinely-run, passing row as
// unrun. Blanking code spans first keeps the marker
// anchored to the status declaration while still catching a marker an author
// writes as plain prose ("UNRUN", "not run yet", "deferred").
func evidenceRowComplete(r evidenceRow) bool {
	if r.Date == "" || r.Runner == "" {
		return false
	}
	statusText := inlineCodeRe.ReplaceAllString(r.Text, "")
	return !unrunMarkerRe.MatchString(statusText)
}

// unrunFindings is the derivation: every Verify row with no completed Evidence
// row behind it, tagged with whether it is risk-bearing and whether it was
// routed to a named follow-up.
//
// risk-bearing = the brief's own risk block has any `yes` (an irreversible /
// customer / regulatory / sensitive-data brief's live row is risk-bearing by
// definition) OR the row itself carries a live/mutating tag.
func unrunFindings(risk map[string]string, verify, evidence string) []unrunFinding {
	items := parseVerifyItems(verify)
	if len(items) == 0 {
		return nil // no Verify table: nothing derivable, and nothing claimed
	}
	rows := parseEvidenceRows(evidence)
	briefWideRisk := anyRiskYes(risk)
	var out []unrunFinding
	for _, it := range items {
		matched := rows[it.ID]
		covered := false
		for _, r := range matched {
			if evidenceRowComplete(r) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		f := unrunFinding{
			ID:          it.ID,
			RiskBearing: briefWideRisk || riskBearingRowRe.MatchString(it.Text),
		}
		if len(matched) == 0 {
			// No Evidence row at all. There is nowhere a routing reference
			// could have been written, so this can never read as routed —
			// closing a risk-bearing row requires SAYING where it went.
			f.Reason = "no Evidence row"
		} else {
			f.Reason = "Evidence row is marked unrun/deferred or names no dated runner"
			for _, r := range matched {
				if routingKeywordRe.MatchString(r.Text) && routingRefRe.MatchString(r.Text) {
					f.Routed = true
					break
				}
			}
		}
		out = append(out, f)
	}
	return out
}

// briefArtifacts is the (risk, Verify, Evidence) triple the derivation needs,
// read from a brief file. Deliberately schema-AGNOSTIC, for the same reason
// briefEvidenceBacked is: a legacy no-frontmatter brief is exempt from the hard
// brief-v1 checks yet still renders a bare `done` on the board, and the whole
// point of a board state is that it covers the board. A legacy file simply has
// no risk block, so only its row-level tags can mark a row risk-bearing.
type briefArtifacts struct {
	Risk     map[string]string
	Verify   string
	Evidence string
	Path     string
}

// loadBriefArtifacts finds the brief-NN file for br in stream s and lifts the
// three sections. ok is false when no such file exists.
func loadBriefArtifacts(s *Stream, num string) (briefArtifacts, bool) {
	for _, path := range briefFilePaths(s) {
		_, n, valid := expectedBriefID(path)
		if !valid || n != num {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return briefArtifacts{}, false
		}
		content := strings.ReplaceAll(string(raw), "\r\n", "\n")
		body := content
		risk := map[string]string{}
		if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
			if _, b, err := splitFrontmatter(content); err == nil {
				body = b
			}
			// The risk block is only available on a parseable brief-v1 file;
			// a legacy or malformed one yields an empty map, which downgrades
			// brief-wide risk to row-level tagging rather than erroring.
			if bf, ok, err := parseBriefFile(path); err == nil && ok {
				risk = bf.Risk
			}
		}
		return briefArtifacts{
			Risk:     risk,
			Verify:   extractSectionByPrefix(body, "Verify"),
			Evidence: extractEvidence(body),
			Path:     path,
		}, true
	}
	return briefArtifacts{}, false
}

// briefUnrunFindings is the per-board-row entry point: the unrun derivation for
// the brief file behind a README row.
func briefUnrunFindings(s *Stream, br *Brief) []unrunFinding {
	art, ok := loadBriefArtifacts(s, br.Num)
	if !ok {
		return nil
	}
	return unrunFindings(art.Risk, art.Verify, art.Evidence)
}

// unrunRiskBearing filters a finding set to the risk-bearing rows.
func unrunRiskBearing(fs []unrunFinding) []unrunFinding {
	var out []unrunFinding
	for _, f := range fs {
		if f.RiskBearing {
			out = append(out, f)
		}
	}
	return out
}

// unrunBlocking filters to the risk-bearing rows that were NOT routed — the set
// the `done` gate refuses to close over.
func unrunBlocking(fs []unrunFinding) []unrunFinding {
	var out []unrunFinding
	for _, f := range unrunRiskBearing(fs) {
		if !f.Routed {
			out = append(out, f)
		}
	}
	return out
}

// unrunToken is the board-token suffix for a verified/done row closed over an
// UNRUN risk-bearing Verify row: "‡", or "" for every other row. Deliberately
// independent of the routed/unrouted split — a routed row is legitimately
// closed but its live check STILL did not run, and the board should say so.
func unrunToken(s *Stream, br *Brief) string {
	if br.Status != "verified" && br.Status != "done" {
		return ""
	}
	if len(unrunRiskBearing(briefUnrunFindings(s, br))) > 0 {
		return "‡"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Merge-base grandfathering
// ---------------------------------------------------------------------------

// closedAtBase returns the set of "<stream>/<NN>" briefs already at `verified`
// or `done` at the merge-base of HEAD and origin/main — the closures this
// branch did not make. ok is false when the base cannot be resolved, in which
// case there is no observable transition to gate and callers must grandfather
// everything (see the file header).
// A package-level var so tests can substitute a fake base without building a
// git repository per case — the same pattern listRemoteBranches uses.
var closedAtBase = func(root string, streams []*Stream) (set map[string]bool, ok bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output()
	if err != nil || strings.TrimSpace(string(mb)) == "" {
		return nil, false
	}
	base := strings.TrimSpace(string(mb))
	set = map[string]bool{}
	for _, s := range streams {
		rel, relErr := filepath.Rel(root, s.Dir)
		if relErr != nil {
			continue
		}
		out, showErr := exec.Command("git", "-C", root, "show",
			base+":"+filepath.ToSlash(filepath.Join(rel, "README.md"))).Output()
		if showErr != nil {
			continue // stream did not exist at base: every row in it is new
		}
		content := strings.ReplaceAll(string(out), "\r\n", "\n")
		body := content
		if _, b, fmErr := splitFrontmatter(content); fmErr == nil {
			body = b
		}
		briefs, parseErr := parseBriefTable(body)
		if parseErr != nil {
			continue
		}
		for _, b := range briefs {
			if b.Status == "done" || b.Status == "verified" {
				set[s.Name+"/"+b.Num] = true
			}
		}
	}
	return set, true
}

// ---------------------------------------------------------------------------
// Lint surfaces
// ---------------------------------------------------------------------------

// unrunGateChecks is the `done`/`verified` half of the state (brief 37).
//
// PROBLEM: a brief this branch newly closed carries an UNRUN risk-bearing
// Verify row with no routed follow-up — the transition the gate exists to
// block.
//
// NOTICE: the same shape on a brief already closed at the merge-base (the
// grandfathered backlog, kept visible and never silently green), plus every
// ROUTED unrun risk-bearing row (legitimately closed, still unproven).
func unrunGateChecks(root string, streams []*Stream) (problems, notices []string) {
	grandfathered, baseOK := closedAtBase(root, streams)
	degraded := false
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "done" && br.Status != "verified" {
				continue
			}
			findings := briefUnrunFindings(s, br)
			risky := unrunRiskBearing(findings)
			if len(risky) == 0 {
				continue
			}
			id := s.Name + "/brief-" + br.Num
			blocking := unrunBlocking(findings)
			if len(blocking) == 0 {
				notices = append(notices, fmt.Sprintf(
					"%s: %s over UNRUN risk-bearing Verify row(s) %s — routed to a named follow-up, so the transition is allowed, but the live check is still unproven; renders %s‡ (methodology-metrics/37)",
					id, br.Status, rowIDList(risky), br.Status))
				continue
			}
			if !baseOK || grandfathered[s.Name+"/"+br.Num] {
				if !baseOK {
					degraded = true
				}
				notices = append(notices, fmt.Sprintf(
					"%s: %s carries UNRUN risk-bearing Verify row(s) %s with no routed follow-up (%s) — pre-existing closure, grandfathered to a NOTICE; renders %s‡ (methodology-metrics/37)",
					id, br.Status, rowIDList(blocking), blocking[0].Reason, br.Status))
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: cannot close as %s — Verify row(s) %s are risk-bearing (live/mutating) and UNRUN (%s), with no routed follow-up. Either run the row and record a dated Evidence row for it, or route it to a named follow-up brief/issue IN the Evidence row (brief-23 live-verify pattern, mandatory). UNRUN is derived from Verify-vs-Evidence coverage, not declared — methodology-metrics/37",
				id, br.Status, rowIDList(blocking), blocking[0].Reason))
		}
	}
	if degraded {
		notices = append(notices, "UNRUN `done`-gate is running degraded: origin/main could not be resolved, so no brief can be shown to have been closed on THIS branch and every offender is grandfathered to a NOTICE. If this is CI, fetch origin/main before the lint step (methodology-metrics/37)")
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// staleImplementedNotices is the `implemented` half of the state
// (F-impl-claims-unproven recommendation 3, expressed as run-coverage).
//
// A brief at `implemented` whose Verify table has ZERO covered rows, and which
// has sat there past staleImplementedThreshold, is an uncorroborated claim
// aging on the public record. NOTICE only — sitting at `implemented` awaiting
// the verify-desk is the normal, correct state; the AGE is what makes it a
// signal, and the age comes from the historian (or the stream's git touch),
// never from anything the implementer writes.
//
// briefTouch maps "<stream>/<NN>" → last recorded transition time (mm/01); a
// brief absent from it falls back to the stream's git LastTouch, exactly as
// nextup/roadmap do. A brief with neither is skipped: unknown age is not
// rendered as a guess (mm/17 discipline).
func staleImplementedNotices(streams []*Stream, briefTouch map[string]time.Time, now time.Time) []string {
	var notices []string
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "implemented" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok {
				continue
			}
			items := parseVerifyItems(art.Verify)
			if len(items) == 0 {
				continue // no Verify table: verifySectionProblems' business, not this one's
			}
			if len(unrunFindings(art.Risk, art.Verify, art.Evidence)) < len(items) {
				continue // at least one row is corroborated: not a fully-unrun claim
			}
			ref := s.LastTouch
			if t, found := briefTouch[s.Name+"/"+br.Num]; found {
				ref = t
			}
			if ref.IsZero() {
				continue
			}
			age := now.Sub(ref)
			if age < staleImplementedThreshold {
				continue
			}
			notices = append(notices, fmt.Sprintf(
				"%s/brief-%s: at `implemented` for %s with 0 of %d Verify row(s) corroborated — an implementer-asserted state nothing has yet checked (F-impl-claims-unproven). Prioritise it in the verify-desk drain (methodology-metrics/37)",
				s.Name, br.Num, renderAge(age), len(items)))
		}
	}
	sort.Strings(notices)
	return notices
}

// rowIDList renders the Verify row IDs of a finding set as "#1, #5".
func rowIDList(fs []unrunFinding) string {
	ids := make([]string, 0, len(fs))
	for _, f := range fs {
		ids = append(ids, "#"+f.ID)
	}
	return strings.Join(ids, ", ")
}
