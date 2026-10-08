package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Awaiting-board bucketing: every awaiting (implemented/verified) brief in an
// active stream lands in exactly one OWNED queue, and each row names its owner
// and its next act. The desk-actionable queue narrows to rows that need a
// judgement; everything another party can move is that party's queue.
//
// The spec is the bucket table below; bucketAwaiting implements it as one pure
// function, first match wins:
//
//	| Order | Condition                                                         | Bucket              | Owner         | Next act                               |
//	|-------|-------------------------------------------------------------------|---------------------|---------------|----------------------------------------|
//	| 1     | gate human or irreversible yes, Evidence carries **VERIFY: PASS** | human gate          | driver        | close the sign-off card                |
//	| 2     | last outcome verify-fail, blocker implementation/check-definition, | implementer rework  | worker        | fix, cite the issue                    |
//	|       | blocker issue open                                                |                     |               |                                        |
//	| 3     | status verified, gate model, Reviewed empty                       | runner-pending      | CI auto-flip  | none; stuck after one main run → file  |
//	| 4     | any unrun row is check:cluster, a billed probe, or                | environment-blocked | operator      | the exact command, verbatim            |
//	|       | could-not-check with an exact command                             |                     |               |                                        |
//	| 5     | gate model, every unrun row runnable offline on Linux             | runner-pending      | verify runner | none                                   |
//	| 6     | a judgement row                                                   | desk-actionable     | verify-desk   | dispatch one judge                     |
//	| 7     | otherwise                                                         | desk-actionable     | verify-desk   | triage, then re-bucket                 |
//
// THREE-STATE. A condition the function cannot read yields bucketCouldNotCheck
// with the reason in nextAct, never a bucket: an unreadable Evidence section
// (a brief file that exists but cannot be read, or an Evidence section holding
// an unterminated HTML comment, which hides every row after it), an unreadable
// verify-outcome store, or an Evidence section whose last verdict is FAIL with
// no outcome record naming the brief (the blocker class the rework arm keys on
// is unrecorded). A could-not-check row is never silently desk-actionable.
//
// Defaults this file decides where the table is silent (each is reversible):
//
//   - "blocker issue open" (row 2) cannot be read offline; the board renders
//     without network. The condition holds when the latest outcome's
//     blocker_ref names an issue (`#N`, `owner/repo#N`, `alias#N`, or an
//     `/issues/N` URL), presumed open until a newer outcome record supersedes
//     it. A ref of "none …" fails the condition.
//   - Row 5 also requires that the Evidence's last verdict is not FAIL: a
//     recorded FAIL means the runner already ran, so re-running is not the next
//     act; the row falls to the judgement or triage arm instead.
//   - A brief carrying `blocked-by: env` in its frontmatter lands in
//     environment-blocked at row 4's position, next act "no command recorded".
//   - "Evidence carries **VERIFY: PASS**" (row 1) reads as: the live verdict
//     (lastVerifyVerdict, last writer wins) is PASS AND a VERIFY: PASS token
//     sits inside a bold span (boldPassRe), which keeps the live marker forms
//     this board already routes to the human gate.
//   - Row 4's "exact command, verbatim" is the unrun Verify row's own Command
//     cell (one enclosing code span removed). A could-not-check Evidence row
//     qualifies when it carries a code span, but its free text is not parsed
//     for the command: the first code span of an Evidence row is as often a
//     fragment (`$VAR`, a repo name) as the command itself.
//   - A billed / live / externally-authenticated probe row has no statusgen
//     marker yet; billedProbeRowRe is the textual stand-in.
//   - A presence-gate prose deliverable is recognised by its Verify section
//     stating that it gates presence (presenceGateRe), the wording the brief
//     template and the brief spec require.
//   - An unrun `gate:model` or `gate:human` CLASS row is a judgement row: the
//     row class itself says a judge decides.

// awaitingBucket is one owned queue of the Awaiting board.
type awaitingBucket int

const (
	bucketCouldNotCheck awaitingBucket = iota
	bucketHumanGate
	bucketRework
	bucketEnvBlocked
	bucketRunnerPending
	bucketDeskActionable
)

// bucketRenderOrder is the fixed display order: the four owned queues, then the
// desk's judgement queue. Could-not-check renders after them, outside the five.
var bucketRenderOrder = []awaitingBucket{
	bucketHumanGate,
	bucketRework,
	bucketEnvBlocked,
	bucketRunnerPending,
	bucketDeskActionable,
}

func (b awaitingBucket) heading() string {
	switch b {
	case bucketHumanGate:
		return "Awaiting human gate"
	case bucketRework:
		// fanoutloop's REWORK source parses this exact heading; keep it.
		return "Awaiting implementer rework"
	case bucketEnvBlocked:
		return "Environment-blocked"
	case bucketRunnerPending:
		return "Runner-pending"
	case bucketDeskActionable:
		return "Desk-actionable"
	}
	return "Could-not-check"
}

func (b awaitingBucket) String() string {
	switch b {
	case bucketHumanGate:
		return "human gate"
	case bucketRework:
		return "implementer rework"
	case bucketEnvBlocked:
		return "environment-blocked"
	case bucketRunnerPending:
		return "runner-pending"
	case bucketDeskActionable:
		return "desk-actionable"
	}
	return "could-not-check"
}

// Owners and next acts, named once so the renderer and the doc cannot drift.
const (
	ownerDriver       = "driver"
	ownerWorker       = "worker"
	ownerCIAutoFlip   = "CI auto-flip"
	ownerOperator     = "operator"
	ownerVerifyRunner = "verify runner"
	ownerVerifyDesk   = "verify-desk"

	nextActCloseCard   = "close the sign-off card"
	nextActAutoFlip    = "none; stuck after one main run → file"
	nextActNone        = "none"
	nextActJudge       = "dispatch one judge"
	nextActTriage      = "triage, then re-bucket"
	nextActNoEnvCmd    = "no command recorded (`blocked-by: env`)"
	couldNotCheckLabel = "could-not-check: "
)

// awaitBrief is the frontmatter and README-row data the bucketing reads.
type awaitBrief struct {
	Status       string // implemented | verified
	Gate         string // model | human | "" (legacy)
	Irreversible bool   // risk.irreversible == yes
	Reviewed     string // the README Reviewed cell; "" or "—" = empty
	BlockedBy    string // "env" or ""
}

// awaitRows is the brief's Verify table: its parsed rows plus the raw section
// text (the presence-gate statement is prose, not a row).
type awaitRows struct {
	Rows    []verifyRowCells
	Section string
}

// awaitEvidence is the brief's Evidence section. Unreadable, when non-empty, is
// the reason the section could not be read at all.
type awaitEvidence struct {
	Text       string
	Unreadable string
}

// awaitOutcome is the latest verify-outcome record naming the brief.
type awaitOutcome struct {
	Outcome     string
	BlockerKind string
	BlockerRef  string
}

// awaitOutcomes is the outcome-record input. Unreadable, when non-empty, is the
// reason the store could not be read; Latest is nil when no record names the
// brief.
type awaitOutcomes struct {
	Unreadable string
	Latest     *awaitOutcome
}

var (
	// boldPassRe is the bold PASS marker: a VERIFY: PASS token inside a bold
	// span — `**VERIFY: PASS**`, and the live forms that wrap it in a longer
	// bold phrase (`**Non-implementer verifier run — VERIFY: PASS**`,
	// `**VERIFY: PASS — all rows green.**`). An unbolded token does not count.
	boldPassRe = regexp.MustCompile(`\*\*[^*\n]*VERIFY:[ \t]*PASS\b[^*\n]*\*\*`)
	// riskValueNamedRe is the derivation-pending marker a judgement row carries.
	riskValueNamedRe = regexp.MustCompile(`RISK-VALUE:\s*NAMED,\s*NOT DERIVED`)
	// presenceGateRe recognises a Verify section that states it gates PRESENCE
	// (a prose deliverable whose quality is owned by a review).
	presenceGateRe = regexp.MustCompile(`(?i)\bpresence[- ]gates?\b|\bgates?\s+presence\b`)
	// billedProbeRowRe is the textual stand-in for the live / billed /
	// externally-authenticated probe row until a marker exists.
	billedProbeRowRe = regexp.MustCompile(`(?i)\b(billed|metered|live[- ]probe|externally[- ]authenticated)\b`)
	// couldNotCheckRe matches a could-not-check disposition in an Evidence row.
	couldNotCheckRe = regexp.MustCompile(`(?i)\bcould-not-check\b`)
	// blockerIssueRefRe recognises an issue reference in a blocker_ref.
	blockerIssueRefRe = regexp.MustCompile(`(^|[\s(\[])([A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?)?#[0-9]+\b|/issues/[0-9]+\b`)
	// firstCodeSpanRe captures the content of the first inline code span.
	firstCodeSpanRe = regexp.MustCompile("(`+)([^`\n]+?)(`+)")
)

// unrunAwaitRow is one Verify row with no settled run behind it.
type unrunAwaitRow struct {
	cells verifyRowCells
	// cncExact is true when a could-not-check Evidence row for this Verify row
	// names a command (carries a code span).
	cncExact bool
	// clusterParked is true when an Evidence row carries the
	// cluster-pending marker for this row.
	clusterParked bool
}

// bucketAwaiting places one awaiting brief in its owned queue. It is pure: every
// input is passed in, nothing is read from disk or the network.
func bucketAwaiting(b awaitBrief, rows awaitRows, ev awaitEvidence, oc awaitOutcomes) (bucket awaitingBucket, owner, nextAct string) {
	// Readability first: every arm below reads the Evidence.
	if ev.Unreadable != "" {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel + ev.Unreadable
	}
	if _, at := stripRowComments(ev.Text); at >= 0 {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel +
			"Evidence holds an unterminated HTML comment (`" + unterminatedCommentLine(ev.Text, at) + "`) — every row after it is hidden"
	}
	verdict := lastVerifyVerdict(ev.Text)

	// 1. Human gate: the gate is human (or the change is irreversible) AND the
	// Evidence carries the bold PASS marker as its live verdict.
	if (b.Gate == "human" || b.Irreversible) && verdict == verdictPass && boldPassRe.MatchString(ev.Text) {
		return bucketHumanGate, ownerDriver, nextActCloseCard
	}

	// 2. Implementer rework, read from the outcome records.
	if oc.Unreadable != "" {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel + "verify-outcome records unreadable: " + oc.Unreadable
	}
	if oc.Latest == nil && verdict == verdictFail {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel +
			"Evidence's last verdict is FAIL but no verify-outcome record names this brief — the blocker class is unrecorded"
	}
	lastFail := oc.Latest != nil && isFailOutcome(oc.Latest.Outcome)
	if lastFail {
		kind := strings.ToLower(strings.TrimSpace(oc.Latest.BlockerKind))
		if kind == "implementation" || kind == "check-definition" {
			if ref := blockerIssueRef(oc.Latest.BlockerRef); ref != "" {
				return bucketRework, ownerWorker, "fix, cite " + ref
			}
		}
	}

	// 3. A verified gate:model row with an empty Reviewed cell is CI's flip.
	if b.Status == "verified" && b.Gate == "model" && reviewedEmpty(b.Reviewed) {
		return bucketRunnerPending, ownerCIAutoFlip, nextActAutoFlip
	}

	unrun := unrunAwaitRows(rows.Rows, ev.Text)

	// 4. Environment-blocked: a row no offline verifier can run.
	for _, u := range unrun {
		switch {
		case u.cells.class() == classCheckCluster || u.clusterParked:
			return bucketEnvBlocked, ownerOperator, codeSpanContent(u.cells.Command)
		case billedProbeRowRe.MatchString(u.cells.Command + " " + u.cells.Expect + " " + u.cells.Class):
			return bucketEnvBlocked, ownerOperator, codeSpanContent(u.cells.Command)
		case u.cncExact:
			return bucketEnvBlocked, ownerOperator, codeSpanContent(u.cells.Command)
		}
	}
	if b.BlockedBy == "env" {
		return bucketEnvBlocked, ownerOperator, nextActNoEnvCmd
	}

	// 5. Runner-pending: a gate:model brief every unrun row of which the
	// offline runner can execute on Linux, with no FAIL already recorded.
	if b.Gate == "model" && len(rows.Rows) > 0 && verdict != verdictFail && !lastFail {
		all := true
		for _, u := range unrun {
			if !runnableOfflineLinux(u.cells) {
				all = false
				break
			}
		}
		if all {
			return bucketRunnerPending, ownerVerifyRunner, nextActNone
		}
	}

	// 6. A judgement row.
	if judgementRow(rows, unrun, ev.Text, oc.Latest) {
		return bucketDeskActionable, ownerVerifyDesk, nextActJudge
	}

	// 7. Everything else is the desk's to triage.
	return bucketDeskActionable, ownerVerifyDesk, nextActTriage
}

// isFailOutcome reports whether an outcome-record `outcome` is a verify failure.
func isFailOutcome(outcome string) bool {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "verify-fail", "fail":
		return true
	}
	return false
}

// blockerIssueRef returns the blocker_ref verbatim (trimmed) when it names an
// issue, else "".
func blockerIssueRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || !blockerIssueRefRe.MatchString(ref) {
		return ""
	}
	return ref
}

// reviewedEmpty reports whether a README Reviewed cell is empty.
func reviewedEmpty(cell string) bool {
	c := strings.TrimSpace(cell)
	return c == "" || c == "—" || c == "-"
}

// unrunAwaitRows returns the Verify rows with no settled run behind them. A row
// is settled when some Evidence row for it is complete (dated, a runner named,
// not marked unrun) and is not itself a could-not-check or a cluster-pending
// park — both of those record that the row did NOT run.
func unrunAwaitRows(rows []verifyRowCells, evidence string) []unrunAwaitRow {
	evRows := parseEvidenceRows(evidence)
	var out []unrunAwaitRow
	for i, r := range rows {
		id := normalizeRowID(r.Num)
		if id == "" {
			id = strconv.Itoa(i + 1)
		}
		u := unrunAwaitRow{cells: r}
		settled := false
		for _, er := range evRows[id] {
			prose := inlineCodeRe.ReplaceAllString(er.Text, "")
			if clusterMarkerRe.MatchString(er.Text) {
				u.clusterParked = true
				continue
			}
			if couldNotCheckRe.MatchString(prose) {
				if firstCodeSpanRe.MatchString(er.Text) {
					u.cncExact = true
				}
				continue
			}
			if evidenceRowComplete(er) {
				settled = true
			}
		}
		if !settled {
			out = append(out, u)
		}
	}
	return out
}

// runnableOfflineLinux reports whether the offline verify runner can execute a
// row on a Linux host: a check class (hermetic or the legacy env-bound
// default), the POSIX shell, and not a billed / live probe.
func runnableOfflineLinux(r verifyRowCells) bool {
	switch r.class() {
	case classCheckCI, classCheck:
	default:
		return false
	}
	if r.shell() != "sh" {
		return false
	}
	return !billedProbeRowRe.MatchString(r.Command + " " + r.Expect + " " + r.Class)
}

// judgementRow reports whether the brief carries a row only a judge can settle.
func judgementRow(rows awaitRows, unrun []unrunAwaitRow, evidence string, latest *awaitOutcome) bool {
	if riskValueNamedRe.MatchString(evidence) {
		return true
	}
	if presenceGateRe.MatchString(rows.Section) {
		return true
	}
	for _, u := range unrun {
		if c := u.cells.class(); c == classGateModel || c == classGateHuman {
			return true
		}
	}
	if latest != nil && isFailOutcome(latest.Outcome) {
		k := strings.ToLower(strings.TrimSpace(latest.BlockerKind))
		if k == "" || k == "unknown" {
			return true
		}
	}
	return false
}

// codeSpanContent returns a Command cell's text with one enclosing code span
// removed, so the next act carries the command itself, verbatim.
func codeSpanContent(cell string) string {
	c := strings.TrimSpace(cell)
	if m := firstCodeSpanRe.FindStringSubmatch(c); m != nil && len(m[1]) == len(m[3]) &&
		strings.HasPrefix(c, m[1]) && strings.HasSuffix(c, m[3]) && len(m[0]) == len(c) {
		return strings.TrimSpace(m[2])
	}
	return c
}

// ---------------------------------------------------------------------------
// Inputs: the only part of the bucketing that reads disk.
// ---------------------------------------------------------------------------

// awaitInputs gathers bucketAwaiting's four inputs for one README row from data
// the renderer already has: the README row, the brief file (frontmatter risk,
// Verify table, Evidence section) and the per-file verify-outcome records.
//
// A README row with NO brief file (a legacy row) is not unreadable — there is no
// Evidence section to read — so it buckets on the README-carried gate and
// Evidence with no Verify rows. A brief file that EXISTS but cannot be read is
// unreadable Evidence: could-not-check.
func awaitInputs(s *Stream, br *Brief) (awaitBrief, awaitRows, awaitEvidence, awaitOutcomes) {
	b := awaitBrief{Status: br.Status, Gate: br.Gate, Reviewed: br.Reviewed, BlockedBy: br.BlockedBy}
	var rows awaitRows
	ev := awaitEvidence{Text: br.Evidence}
	if path := awaitBriefPath(s, br.Num); path != "" {
		raw, err := readFileMemo(path)
		if err != nil {
			ev = awaitEvidence{Unreadable: fmt.Sprintf("cannot read %s: %v", path, err)}
		} else {
			content := strings.ReplaceAll(string(raw), "\r\n", "\n")
			body := content
			if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
				if _, bd, ferr := splitFrontmatter(content); ferr == nil {
					body = bd
				}
				if bf, ok, perr := parseBriefFile(path); perr == nil && ok {
					b.Irreversible = strings.EqualFold(strings.TrimSpace(bf.Risk["irreversible"]), "yes")
					if b.Gate == "" {
						b.Gate = bf.Gate
					}
					if b.BlockedBy == "" {
						b.BlockedBy = bf.BlockedBy
					}
				}
			}
			section := extractSectionByPrefix(body, "Verify")
			verifyRowTable(section, func(r verifyRowCells) { rows.Rows = append(rows.Rows, r) })
			rows.Section = section
			ev = awaitEvidence{Text: extractEvidence(body)}
		}
	}
	return b, rows, ev, awaitOutcomesFor(s, br.Num)
}

// awaitBriefPath returns the brief file for README row num, or "" when none.
func awaitBriefPath(s *Stream, num string) string {
	for _, path := range briefFilePaths(s) {
		if _, n, valid := expectedBriefID(path); valid && n == num {
			return path
		}
	}
	return ""
}

// awaitOutcomeIndex is one root's verify-outcome store reduced to the latest
// record per brief key, or the reason the store could not be read.
type awaitOutcomeIndex struct {
	latest     map[string]awaitOutcome
	unreadable string
}

var (
	awaitOutcomeMu    sync.Mutex
	awaitOutcomeCache = map[string]awaitOutcomeIndex{}
)

// awaitOutcomesFor returns the outcome input for one brief. The store is read
// once per root per process. A stream with no root (an in-memory board) has an
// empty store.
func awaitOutcomesFor(s *Stream, num string) awaitOutcomes {
	if s.Root == "" {
		return awaitOutcomes{}
	}
	awaitOutcomeMu.Lock()
	idx, ok := awaitOutcomeCache[s.Root]
	if !ok {
		idx = loadAwaitOutcomeIndex(s.Root)
		awaitOutcomeCache[s.Root] = idx
	}
	awaitOutcomeMu.Unlock()
	if idx.unreadable != "" {
		return awaitOutcomes{Unreadable: idx.unreadable}
	}
	if o, ok := idx.latest[s.Name+"/"+num]; ok {
		return awaitOutcomes{Latest: &o}
	}
	return awaitOutcomes{}
}

func loadAwaitOutcomeIndex(root string) awaitOutcomeIndex {
	if _, err := os.Stat(root); err != nil {
		return awaitOutcomeIndex{unreadable: fmt.Sprintf("cannot stat root %s: %v", root, err)}
	}
	records, err := readVerifyOutcomeRecords(root)
	if err != nil {
		return awaitOutcomeIndex{unreadable: err.Error()}
	}
	latest, _ := latestOutcomePerBrief(records)
	idx := awaitOutcomeIndex{latest: map[string]awaitOutcome{}}
	for key, rec := range latest {
		var o struct {
			Outcome     string `json:"outcome"`
			BlockerKind string `json:"blocker_kind"`
			BlockerRef  string `json:"blocker_ref"`
		}
		if jerr := json.Unmarshal(rec.Raw, &o); jerr != nil {
			return awaitOutcomeIndex{unreadable: fmt.Sprintf("verify-outcome record %s: %v", rec.Source, jerr)}
		}
		idx.latest[key] = awaitOutcome{Outcome: o.Outcome, BlockerKind: o.BlockerKind, BlockerRef: o.BlockerRef}
	}
	return idx
}
