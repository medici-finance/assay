package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Awaiting-board bucketing: every awaiting (implemented/verified) brief in an
// active stream lands in exactly one OWNED queue, and each row names its owner
// and its next act. The desk-actionable queue narrows to rows that need a
// judgement; everything another party can move is that party's queue.
//
// The spec is the bucket table below; bucketAwaiting implements it as one pure
// function, first match wins:
//
//	| Order | Condition                                                  | Bucket              | Owner         | Next act                              |
//	|-------|------------------------------------------------------------|---------------------|---------------|---------------------------------------|
//	| 1     | gate human or irreversible yes, live verdict PASS; or a    | human gate          | driver        | close the sign-off card               |
//	|       | verified gate:human brief with no live FAIL                |                     |               | (a recorded blocker: the blocker)     |
//	| 2a    | recorded blocker, kind implementation / check-definition,  | implementer rework  | worker        | fix, cite the issue                   |
//	|       | blocker names an issue                                     |                     |               |                                       |
//	| 2b    | recorded blocker, kind human-action                        | human gate          | driver        | the human action, cite the ref        |
//	| 3     | status verified, gate model, Reviewed empty, no recorded   | runner-pending      | CI auto-flip  | none; stuck after one main run → file |
//	|       | blocker                                                    |                     |               |                                       |
//	| 4     | an unrun row is check:cluster, a billed probe, or          | environment-blocked | operator      | the exact command, verbatim           |
//	|       | could-not-check with an exact command; or a recorded       |                     |               | (a record alone: the blocker)         |
//	|       | blocker of kind environment                                |                     |               |                                       |
//	| 5     | gate model, >= 1 unrun row, all runnable offline on Linux, | runner-pending      | verify runner | none                                  |
//	|       | no FAIL in Evidence, no recorded blocker                   |                     |               |                                       |
//	| 6     | a judgement row                                            | desk-actionable     | verify-desk   | dispatch one judge                    |
//	| 7     | otherwise                                                  | desk-actionable     | verify-desk   | triage, then re-bucket                |
//
// A "recorded blocker" is the brief's latest verify-outcome record when its
// outcome is a hold (blocked, needs-context), whatever the Evidence says, or a
// fail (verify-fail, fail) unless the Evidence's live verdict is PASS and the
// record is dated before the PASS run (recordedBlocker). A fail and a live PASS
// whose dates cannot be read are could-not-check. On row 1 a recorded blocker
// keeps the brief with the driver and replaces the sign-off act with the
// blocker; on rows 3 and 5 it stops the runner-pending placement.
//
// THREE-STATE. A condition the function cannot read yields bucketCouldNotCheck
// with the reason in nextAct, never a bucket: an unreadable Evidence section
// (a brief file that exists but cannot be read, or an Evidence section holding
// an unterminated HTML comment, which hides every row after it), an unreadable
// verify-outcome store, a record dated beyond the clock-skew tolerance, a
// latest record whose outcome is none of the recognised values, or an Evidence
// section whose last verdict is FAIL with no outcome record naming the brief
// (the blocker class the rework arm keys on is unrecorded). A could-not-check
// row is never silently desk-actionable, and it counts toward the
// verification-debt measure (debtCounts) so it cannot switch the alarm off.
//
// Defaults this file decides where the table is silent (each is reversible):
//
//   - "blocker issue open" (row 2a) cannot be read offline; the board renders
//     without network. The condition holds when the latest outcome's
//     blocker_ref names an issue (`#N`, `owner/repo#N`, `alias#N`, or an
//     `/issues/N` URL), presumed open until a newer outcome record supersedes
//     it. A ref of "none …" fails the condition.
//   - A recorded hold (blocked, needs-context) is routed by its kind whatever
//     the Evidence verdict: a verifier writes a hold beside a PASS and the two
//     do not contradict. A recorded fail is ordered against a live PASS by day
//     (record ts in UTC against livePassDate); a fail dated the same day as the
//     PASS run is kept. A blocker whose kind is absent, unknown, or
//     implementation/check-definition without an issue ref falls through to the
//     judgement arm (kind absent or unknown) or triage.
//   - A gate:human (or irreversible) brief the driver would sign off stays with
//     the driver when a recorded blocker holds it; the next act names the
//     blocker instead of the sign-off (heldSignOffAct). Routing it by kind like a
//     gate:model brief is the alternative.
//   - Row 5 requires an unrun row (the runner has nothing to run on an
//     all-settled brief) and no recorded FAIL in the Evidence or the records: a
//     recorded FAIL or hold means the runner already ran, so re-running is not
//     the next act; the row falls to the judgement or triage arm instead.
//   - A brief carrying `blocked-by: env` in its frontmatter lands in
//     environment-blocked at row 4's position, next act "no command recorded".
//   - Row 1 reads the Evidence's live verdict (lastVerifyVerdict, last writer
//     wins) as PASS, whatever the emphasis of the line. The sign-off card itself
//     is raised only for the strict bold marker (hasVerifyPass) or a verified
//     gate:human brief; a row on this arm with no card says so in its next act.
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

	nextActCloseCard = "close the sign-off card"
	nextActAutoFlip  = "none; stuck after one main run → file"
	nextActNone      = "none"
	nextActJudge     = "dispatch one judge"
	nextActTriage    = "triage, then re-bucket"
	nextActNoEnvCmd  = "no command recorded (`blocked-by: env`)"
	// nextActSignOffNoCard is row 1's act for a brief no sign-off card exists
	// for (irreversible without gate: human, or an unbolded PASS).
	nextActSignOffNoCard = "sign off the PASS verdict (no sign-off card is raised for this brief)"
	nextActHumanAction   = "human action, cite "
	// nextActHeldPrefix leads row 1's act when a recorded blocker holds the
	// brief: the driver clears the blocker before any sign-off.
	nextActHeldPrefix = "resolve the recorded blocker before sign-off: "
	// nextActEnvBlocker leads a prose environment act; nextActCell keeps it out
	// of a code span (a command renders as one).
	nextActEnvBlocker  = "environment blocker, cite "
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
	// TS is the record's RFC 3339 timestamp; recordedBlocker orders a fail
	// record against the Evidence's live PASS by its day.
	TS string
}

// awaitOutcomes is the outcome-record input. Unreadable, when non-empty, is the
// reason the store could not be read; Latest is nil when no record names the
// brief.
type awaitOutcomes struct {
	Unreadable string
	Latest     *awaitOutcome
	// Future is true when a record naming the brief carries a ts beyond the
	// clock-skew tolerance. Such a record never wins the newest-ts comparison,
	// so Latest alone cannot tell the brief's state: could-not-check.
	Future bool
}

var (
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

	// Readability of the outcome records, before any arm reads them — row 1
	// included, since a recorded blocker changes row 1's next act.
	if oc.Unreadable != "" {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel + "verify-outcome records unreadable: " + oc.Unreadable
	}
	if oc.Future {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel +
			"a verify-outcome record for this brief is dated beyond the clock-skew tolerance — it never wins, so the latest state is unknown"
	}
	if oc.Latest != nil && classifyOutcome(oc.Latest.Outcome) == outcomeUnrecognised {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel +
			fmt.Sprintf("the latest verify-outcome record has an unrecognised outcome %q", oc.Latest.Outcome)
	}
	if oc.Latest == nil && verdict == verdictFail {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel +
			"Evidence's last verdict is FAIL but no verify-outcome record names this brief — the blocker class is unrecorded"
	}

	// The recorded blocker (recordedBlocker): a hold always; a fail unless it is
	// dated before the live PASS run. A fail beside a live PASS that the two
	// dates cannot order is could-not-check, never a silent drop.
	blocker, unordered := recordedBlocker(oc.Latest, verdict, ev.Text)
	if unordered != "" {
		return bucketCouldNotCheck, ownerVerifyDesk, couldNotCheckLabel + unordered
	}

	// 1. Human gate: the gate is human (or the change is irreversible) AND the
	// Evidence's live verdict is PASS, whatever the emphasis of the line. A
	// verified gate:human brief always has its sign-off card raised
	// (verifyIssues, Path A), so it is the driver's whatever its Evidence says
	// — unless the Evidence's live verdict is a FAIL. Either way a recorded
	// blocker keeps the row with the driver but replaces the sign-off act with
	// the blocker: signing off over a recorded hold is not the next act.
	if ((b.Gate == "human" || b.Irreversible) && verdict == verdictPass) ||
		(b.Gate == "human" && b.Status == "verified" && verdict != verdictFail) {
		if blocker != nil {
			return bucketHumanGate, ownerDriver, heldSignOffAct(blocker)
		}
		return bucketHumanGate, ownerDriver, signOffAct(b, ev.Text)
	}

	kind := ""
	ref := ""
	if blocker != nil {
		kind = strings.ToLower(strings.TrimSpace(blocker.BlockerKind))
		ref = blockerIssueRef(blocker.BlockerRef)
	}

	// 2. A recorded blocker, routed by its kind to the party that owns it (the
	// owners deskkit's wake receipts name: worker, brief author, human, operator).
	switch {
	case (kind == "implementation" || kind == "check-definition") && ref != "":
		return bucketRework, ownerWorker, "fix, cite " + ref
	case kind == "human-action":
		return bucketHumanGate, ownerDriver, humanActionAct(blocker.BlockerRef)
	}

	// 3. A verified gate:model row with an empty Reviewed cell is CI's flip —
	// unless a recorded blocker holds it, which the later arms route.
	if b.Status == "verified" && b.Gate == "model" && reviewedEmpty(b.Reviewed) && blocker == nil {
		return bucketRunnerPending, ownerCIAutoFlip, nextActAutoFlip
	}

	unrun := unrunAwaitRows(rows.Rows, ev.Text)

	// 4. Environment-blocked: a row no offline verifier can run, or a recorded
	// environment blocker.
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
	if kind == "environment" {
		return bucketEnvBlocked, ownerOperator, envBlockerAct(blocker.BlockerRef)
	}
	if b.BlockedBy == "env" {
		return bucketEnvBlocked, ownerOperator, nextActNoEnvCmd
	}

	// 5. Runner-pending: a gate:model brief with at least one unrun row, every
	// unrun row of which the offline runner can execute on Linux, with no FAIL
	// in the Evidence and no fail or hold recorded. An empty unrun set is not
	// runner work: the runner has nothing left to run.
	if b.Gate == "model" && len(rows.Rows) > 0 && len(unrun) > 0 && verdict != verdictFail && blocker == nil {
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
	if judgementRow(rows, unrun, ev.Text, blocker) {
		return bucketDeskActionable, ownerVerifyDesk, nextActJudge
	}

	// 7. Everything else is the desk's to triage.
	return bucketDeskActionable, ownerVerifyDesk, nextActTriage
}

// signOffAct is row 1's next act. A sign-off card is raised for a gate:human
// brief that is verified, or implemented with the strict bold PASS marker and
// no held row (verifyIssues); for any other brief on this row no card exists,
// and the act says so instead of naming one.
func signOffAct(b awaitBrief, evidence string) string {
	if b.Gate == "human" {
		if b.Status == "verified" {
			return nextActCloseCard
		}
		if held, _ := verifyPassHeldContradiction(evidence); !held && hasVerifyPass(evidence) {
			return nextActCloseCard
		}
	}
	return nextActSignOffNoCard
}

// isoDateRe matches a calendar date as Evidence writes it (YYYY-MM-DD).
var isoDateRe = regexp.MustCompile(`\b(20[0-9]{2}-[01][0-9]-[0-3][0-9])\b`)

// recordedBlocker returns the latest outcome record when it holds the brief:
//
//   - a hold (blocked, needs-context) always does. A verifier writes a hold
//     beside a PASS on behaviour (a row it could not settle, an act only a
//     human can take); the two do not contradict, so a PASS clears nothing.
//   - a fail (verify-fail, fail) does unless the Evidence's live verdict is
//     PASS and the record is dated BEFORE the PASS run (livePassDate): then the
//     fix landed and the brief was re-verified after the record, and the record
//     decides nothing. A fail dated the same day as the PASS run or later holds.
//     When the record's ts or the PASS run's date cannot be read, the two cannot
//     be ordered, and unordered names that disagreement (could-not-check).
//
// Any other outcome holds nothing.
func recordedBlocker(latest *awaitOutcome, verdict, evidence string) (blocker *awaitOutcome, unordered string) {
	if latest == nil {
		return nil, ""
	}
	switch classifyOutcome(latest.Outcome) {
	case outcomeHold:
		return latest, ""
	case outcomeFail:
		if verdict != verdictPass {
			return latest, ""
		}
		passDay := livePassDate(evidence)
		recDay := recordDate(latest.TS)
		if passDay == "" || recDay == "" {
			return nil, fmt.Sprintf("the latest verify-outcome record is a fail (%s) and the Evidence's live verdict is PASS, but the two cannot be ordered (record date %q, PASS run date %q)",
				strings.ToLower(strings.TrimSpace(latest.Outcome)), recDay, passDay)
		}
		if recDay < passDay {
			return nil, ""
		}
		return latest, ""
	}
	return nil, ""
}

// livePassDate is the date of the run that wrote the Evidence's live PASS: the
// newest YYYY-MM-DD on the lines walkVerdictLines reads, up to and including
// the line holding the last verdict token, when that token is PASS. "" when the
// live verdict is not PASS or no date precedes it. Dates written after the PASS
// line are not read: they may belong to a later note, and a later date would
// make the PASS look newer than a record it does not supersede. Reading only
// what precedes the line can make the PASS look older, never newer, so the
// error keeps a record rather than dropping one.
func livePassDate(evidence string) string {
	newest, atVerdict, last := "", "", verdictNone
	walkVerdictLines(evidence, func(line, v string) {
		for _, m := range isoDateRe.FindAllStringSubmatch(line, -1) {
			if validISODate(m[1]) && m[1] > newest {
				newest = m[1]
			}
		}
		if v != verdictNone {
			last, atVerdict = v, newest
		}
	})
	if last != verdictPass {
		return ""
	}
	return atVerdict
}

// validISODate reports whether s is a real calendar date (YYYY-MM-DD).
func validISODate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// recordDate is the UTC calendar day of a verify-outcome record's ts, or ""
// when the ts is not RFC 3339.
func recordDate(ts string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(ts))
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// heldSignOffAct is row 1's act when a recorded blocker holds a brief the
// driver would otherwise sign off: the blocker, with its kind and ref, in
// place of the sign-off. The kind is echoed only from the recognised set, so
// record text reaches the cell through the ref alone (as the other acts).
func heldSignOffAct(blocker *awaitOutcome) string {
	kind := strings.ToLower(strings.TrimSpace(blocker.BlockerKind))
	switch kind {
	case "implementation", "check-definition", "human-action", "environment":
	default:
		kind = "no recognised kind"
	}
	outcome := "fail"
	if classifyOutcome(blocker.Outcome) == outcomeHold {
		outcome = "hold"
	}
	ref := blockerRefText(blocker.BlockerRef)
	if ref == noBlockerRef {
		return fmt.Sprintf("%s%s (%s), %s", nextActHeldPrefix, outcome, kind, ref)
	}
	return fmt.Sprintf("%s%s (%s), cite %s", nextActHeldPrefix, outcome, kind, ref)
}

// humanActionAct is the next act of a recorded human-action blocker.
func humanActionAct(ref string) string {
	return nextActHumanAction + blockerRefText(ref)
}

// envBlockerAct is the next act of a recorded environment blocker with no
// unrun row to name a command for.
func envBlockerAct(ref string) string {
	return nextActEnvBlocker + blockerRefText(ref)
}

// blockerRefText renders a blocker_ref for a next act: the ref when the record
// carries one, else a statement that none was recorded.
func blockerRefText(ref string) string {
	if r := strings.TrimSpace(ref); r != "" && !strings.EqualFold(r, "none") {
		return r
	}
	return noBlockerRef
}

// noBlockerRef is blockerRefText's text for a record with no blocker ref.
const noBlockerRef = "no blocker ref recorded"

// outcomeClass is the meaning of a verify-outcome record's `outcome` value.
type outcomeClass int

const (
	outcomeUnrecognised outcomeClass = iota
	outcomePass
	outcomeFail
	outcomeHold
)

// classifyOutcome reads an outcome value. This mirrors the vocabulary of
// deskkit's WakeReceipt.IsFailedOrBlocked (tools/desk/internal/deskkit/verifywake.go):
// statusgen is a separate Go module and keeps its own copy of the pure record
// rules, byte-identical by hand (see verifyoutcomes.go). A value neither list
// names is unrecognised, which the bucketing treats as could-not-check.
func classifyOutcome(outcome string) outcomeClass {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "verified":
		return outcomePass
	case "verify-fail", "fail":
		return outcomeFail
	case "blocked", "needs_context", "needs-context":
		return outcomeHold
	}
	return outcomeUnrecognised
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
func judgementRow(rows awaitRows, unrun []unrunAwaitRow, evidence string, blocker *awaitOutcome) bool {
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
	if blocker != nil {
		k := strings.ToLower(strings.TrimSpace(blocker.BlockerKind))
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
	future     map[string]bool
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
	key := s.Name + "/" + num
	out := awaitOutcomes{Future: idx.future[key]}
	if o, ok := idx.latest[key]; ok {
		out.Latest = &o
	}
	return out
}

func loadAwaitOutcomeIndex(root string) awaitOutcomeIndex {
	if _, err := os.Stat(root); err != nil {
		return awaitOutcomeIndex{unreadable: fmt.Sprintf("cannot stat root %s: %v", root, err)}
	}
	records, err := readVerifyOutcomeRecords(root)
	if err != nil {
		return awaitOutcomeIndex{unreadable: err.Error()}
	}
	latest, future := latestOutcomePerBrief(records)
	idx := awaitOutcomeIndex{latest: map[string]awaitOutcome{}, future: future}
	for key, rec := range latest {
		var o struct {
			Outcome     string `json:"outcome"`
			BlockerKind string `json:"blocker_kind"`
			BlockerRef  string `json:"blocker_ref"`
		}
		if jerr := json.Unmarshal(rec.Raw, &o); jerr != nil {
			return awaitOutcomeIndex{unreadable: fmt.Sprintf("verify-outcome record %s: %v", rec.Source, jerr)}
		}
		idx.latest[key] = awaitOutcome{Outcome: o.Outcome, BlockerKind: o.BlockerKind, BlockerRef: o.BlockerRef, TS: rec.TS}
	}
	return idx
}
