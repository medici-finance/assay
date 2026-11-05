package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Historical-notice reporting (statusgen/28).
//
// `--lint` reports advisory findings about authoring rules that were added AFTER
// most briefs were closed. A closed record is not rewritten to satisfy a later
// lint (editing a shipped brief to green a gate is the falsification this
// toolkit exists to catch), so those findings can never be acted on, yet they
// used to be interleaved with the findings an author CAN act on. This file is
// the ONE owner of how a notice line is classified and reported:
//
//   - which rule a line belongs to (noticeRuleInventory + noticeFragmentRules —
//     the only classification table),
//   - which record it is about and that record's standing (recordIndex). The
//     record is the notice's SUBJECT — the path in the emitter's own leading
//     slot — never a path that merely appears later in text the emitter quotes,
//   - actionable vs historical, and the counts for each,
//   - the three renderings of one report: default text, `--all` detail text and
//     `--lint --json`. All three are views of ONE noticeReport value, so their
//     totals cannot disagree.
//
// What it never does: remove a finding, change a PROBLEM or an exit code, or turn
// a could-not-check into a clean. A finding is HISTORICAL only when its rule is in
// the inventory AND it is about a brief file whose README row was already
// verified/done at the merge-base AND its rule is not held actionable
// (noticeHeldActionable). An active, unknown-status, newly closed or
// non-brief subject stays ACTIONABLE — the stricter checks for closures this
// branch makes are untouched. A count may rise when a new detector discovers old
// debt; that is history becoming visible, not a newly closed defective brief.

// noticeRuleDef is one included rule identity and its inventory family.
type noticeRuleDef struct {
	ID     string
	Family string // "A" structural Verify rows; "B" strength / dereference / metadata
}

// noticeRuleInventory is the closed set of rule identities this report
// accounts for. Spellings are the emitters' own constants (or, for the older
// checks, the literal tag they print). Two guards watch it:
// TestHistoricalNoticeReport drives the real emitters named in notice28Rules end
// to end (not every rule below), and TestNoticeInventoryMatchesEmitters checks
// each phrase-matched rule against its own emitting file. A rule outside both is
// recognised by its bracket tag or constant only.
var noticeRuleInventory = []noticeRuleDef{
	{ruleERELiteralPipe, "A"}, {ruleGrepZeroCount, "A"}, {ruleExitSwallowed, "A"},
	{ruleRE2LiteralPipe, "A"}, {ruleMetavar, "A"}, {ruleGoRunExit, "A"},
	{ruleBREAlternation, "A"}, {ruleShreddedCell, "A"}, {ruleMovingRef, "A"},
	{rulePortability, "A"}, {ruleGoTestRunVacuous, "A"}, {ruleProseLedCommand, "A"},
	{ruleCmdMarkerAmbiguous, "A"}, {ruleCmdMarkerOverrides, "A"},
	{ruleCmdMarkerNotHonoured, "A"}, {ruleCmdMarkerVacuous, "A"},
	{ruleGrepPipeZeroCount, "A"}, {ruleVerifyRowPortability, "A"},

	{ruleTriviallyGreen, "B"}, {ruleNoOutputAssert, "B"}, {ruleTableNoFiles, "B"},
	{ruleIDDereference, "B"}, {ruleRiskFilesCrossRead, "B"},
	{ruleWhyAbsent, "B"}, {ruleWhySubstance, "B"}, {ruleExecTierWhyAbsent, "B"},
	{ruleOrderingGate, "B"}, {ruleSecurityReviewAbsent, "B"},
	{ruleConsumersProse, "B"}, {ruleConsumersMissingList, "B"}, {ruleConsumersNoVerifyRow, "B"},
	{ruleConsumersUnrouted, "B"}, {ruleConsumersFollowupNoTarget, "B"},
	{ruleConsumersFollowupOneWay, "B"}, {ruleConsumersOutOfScopeNoReason, "B"},
}

// Rule identities for the checks whose emitted line carries no bracketed tag.
// The consumers-* names are the lintRuleRegistry spellings (enforcementstatus.go).
const (
	ruleVerifyRowPortability        = "verify-row-portability"
	ruleIDDereference               = "id-dereference"
	ruleWhyAbsent                   = "why-absent"
	ruleWhySubstance                = "why-substance-invalid"
	ruleExecTierWhyAbsent           = "exec-tier-why-absent"
	ruleOrderingGate                = "ordering-gate"
	ruleSecurityReviewAbsent        = "security-review-absent"
	ruleConsumersProse              = "consumers-prose"
	ruleConsumersMissingList        = "consumers-missing-list"
	ruleConsumersNoVerifyRow        = "consumers-no-verify-row"
	ruleConsumersUnrouted           = "consumers-unrouted"
	ruleConsumersFollowupNoTarget   = "consumers-followup-no-target"
	ruleConsumersFollowupOneWay     = "consumers-followup-one-way"
	ruleConsumersOutOfScopeNoReason = "consumers-out-of-scope-no-reason"
	noticeStatusNotARecord          = "not-a-record"
	noticeStatusUnknown             = "unknown"
	noticeStatusActive              = "active"
	noticeStatusClosedThisBranch    = "closed-this-branch"
	noticeStatusDonePreExisting     = "done-pre-existing"
	noticeStatusVerifiedPreExisting = "verified-pre-existing"
	noticeRuleUnclassified          = "(unclassified)"
	noticeReportSchema              = "statusgen.lint-notice-report/v1"
	noticeSummaryTag                = "[historical-notices]"
	noticeCouldNotCheckTag          = "[notice-report-could-not-check]"
)

// noticeFragmentRules classify the emitters whose line has no bracketed tag by a
// phrase of the message itself. emitter is the one production file that spells the
// phrase; TestNoticeInventoryMatchesEmitters reads that file only, so the
// registry text in enforcementstatus.go can never keep a stale phrase green.
// Order matters only for overlapping phrases.
var noticeFragmentRules = []struct{ rule, fragment, emitter string }{
	{ruleOrderingGate, "ordering-gate: ", "orderinggate.go"},
	{ruleWhyAbsent, " has no why:", "brieffile.go"},
	{ruleWhySubstance, " fails the substance floor", "brieffile.go"},
	{ruleExecTierWhyAbsent, "has exec-tier: strong but no exec-tier-why", "brieffile.go"},
	{ruleSecurityReviewAbsent, "records no security-review", "brieffile.go"},
	{ruleConsumersProse, "consumers: is a prose paragraph", "consumers.go"},
	{ruleConsumersMissingList, "enumerates no consumers:", "consumers.go"},
	{ruleConsumersNoVerifyRow, "carries consumers: but no Verify row", "consumers.go"},
	{ruleConsumersUnrouted, "names no routing", "consumers.go"},
	{ruleConsumersFollowupNoTarget, "routes to a follow-up but names no", "consumers.go"},
	{ruleConsumersFollowupOneWay, "the coverage claim is one-way", "consumers.go"},
	{ruleConsumersOutOfScopeNoReason, "is routed out-of-scope with no substantive reason", "consumers.go"},
}

// noticeHeldActionable lists inventory rules that are COUNTED per rule but never
// demoted to historical, with the reason. security-review-absent is held because
// the recorded grandfather boundary (docs/streams/statusgen/README.md, Family B
// row for ordering and legacy review metadata) covers only closures corroborated
// as dated before 2026-08-15, and that date test is unbuilt (statusgen/31, task
// 4): "closed at the merge-base" alone also covers closures made after the rule
// took effect, which are a live gap and not frozen history. Remove the entry
// only together with a corroborated-date test whose unknown dates stay
// could-not-check.
var noticeHeldActionable = map[string]string{
	ruleSecurityReviewAbsent: "no corroborated closure-date boundary yet (statusgen/31)",
}

var (
	noticeRuleFamily = func() map[string]string {
		m := make(map[string]string, len(noticeRuleInventory))
		for _, r := range noticeRuleInventory {
			m[r.ID] = r.Family
		}
		return m
	}()
	noticeTagRe = regexp.MustCompile(`\[([A-Za-z0-9-]+)\]`)
	// noticeSubjectRe reads the emitter's leading slot: an optional
	// "ordering-gate: " or "[tag] " prefix, then the subject file path, an
	// optional ":<line>", then ":" or a space. Everything after it is message
	// text, which may quote author prose (a brief path, a tag, a phrase) that
	// says nothing about what the notice is about.
	noticeSubjectRe   = regexp.MustCompile(`^(?:ordering-gate: )?(?:\[[A-Za-z0-9-]+\] )?(\S+?\.md)(?::\d+)?(?::|\s|$)`)
	noticeBriefPathRe = regexp.MustCompile(`^(?:.*/)?([A-Za-z0-9._-]+)/(brief-[0-9]+[A-Za-z0-9._-]*\.md)$`)
)

// noticeSubject returns the file path a notice line is ABOUT: the path in the
// emitter's leading slot. ok is false when the line does not open with a file
// path (an aggregate, a stream-level or tree-level notice).
func noticeSubject(line string) (subject string, ok bool) {
	m := noticeSubjectRe.FindStringSubmatchIndex(line)
	if m == nil {
		return "", false
	}
	return line[m[2]:m[3]], true
}

// noticeHead is the emitter-owned opening of a line: everything before the first
// backtick or double quote, where an emitter starts quoting author text.
func noticeHead(line string) string {
	if i := strings.IndexAny(line, "`\""); i >= 0 {
		return line[:i]
	}
	return line
}

// noticeQuotedRe matches one quoted span of author text: a Go-%q double-quoted
// string (escapes honoured) or a backtick span.
var noticeQuotedRe = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`")

// noticeUnquoted is the line with every quoted span cut out: the emitter's own
// words, wherever they sit relative to the quoted author text.
func noticeUnquoted(line string) string {
	return noticeQuotedRe.ReplaceAllString(line, " ")
}

// noticeRuleOf returns the inventory rule a notice line belongs to, or "". A
// bracket tag counts only in the emitter-owned head (before the first quoted
// span), a fragment only outside every quoted span, and ordering-gate only as the
// line's own prefix, so a notice that quotes author text naming a rule cannot be
// filed under it.
func noticeRuleOf(line string) string {
	for _, m := range noticeTagRe.FindAllStringSubmatch(noticeHead(line), -1) {
		if _, ok := noticeRuleFamily[m[1]]; ok {
			return m[1]
		}
	}
	for _, f := range noticeFragmentRules {
		if f.rule == ruleOrderingGate {
			if strings.HasPrefix(line, f.fragment) {
				return f.rule
			}
			continue
		}
		// Emitter-owned text only: the consumers and brief-file emitters quote the
		// author's entry (%q), and a phrase inside that quote is author prose,
		// not the emitter's statement. The emitter's own phrase usually FOLLOWS
		// the quoted entry, so the quoted spans are cut out, not the tail.
		if strings.Contains(noticeUnquoted(line), f.fragment) {
			return f.rule
		}
	}
	return ""
}

// noticeIsCouldNotCheck reports whether a line states a could-not-check (the
// third state). Such a finding is counted under its own tally and never
// reported as a clean.
func noticeIsCouldNotCheck(line string) bool {
	return strings.Contains(line, "COULD-NOT-CHECK") || strings.Contains(line, "could-not-check")
}

// noticeRecordKey is the stream-qualified brief file a notice is ABOUT: its
// subject path, when that is a brief file. A notice whose subject is a README,
// CLAUDE.md or no file at all is not about a record, whatever brief paths its
// quoted text contains.
func noticeRecordKey(line string) (key string, isBrief bool) {
	subject, ok := noticeSubject(line)
	if !ok {
		return "", false
	}
	if m := noticeBriefPathRe.FindStringSubmatch(filepath.ToSlash(subject)); m != nil {
		return m[1] + "/" + m[2], true
	}
	return "", false
}

// recordInfo is one brief file's standing at report time.
type recordInfo struct {
	ID     string // <stream>/<NN>
	Status string // README row status; "" when the file has no README row
	Pre    bool   // closed at the merge-base (or base unresolved: grandfathered)
}

// recordIndex maps brief files to their standing. baseOK false means the
// merge-base could not be resolved: closed records are then treated as
// pre-existing (the established grandfathering fallback), and the report says
// so as a could-not-check state instead of presenting it as established fact.
type recordIndex struct {
	byKey  map[string]recordInfo
	baseOK bool
	closed int
}

func isClosedStatus(s string) bool { return s == "done" || s == "verified" }

// buildRecordIndex records every brief file's README standing. grandfathered is
// closedAtBase's answer; the same predicate the closure gates use, never a
// second one.
func buildRecordIndex(streams []*Stream, grandfathered map[string]bool, baseOK bool) *recordIndex {
	idx := &recordIndex{byKey: map[string]recordInfo{}, baseOK: baseOK}
	for _, s := range streams {
		statusByNum := map[string]string{}
		for _, b := range s.Briefs {
			statusByNum[b.Num] = b.Status
		}
		for _, path := range briefFilePaths(s) {
			id, num, ok := expectedBriefID(path)
			if !ok {
				continue
			}
			status := statusByNum[num]
			info := recordInfo{ID: id, Status: status}
			if isClosedStatus(status) {
				idx.closed++
				info.Pre = !baseOK || grandfathered[s.Name+"/"+num]
			}
			idx.byKey[filepath.Base(s.Dir)+"/"+filepath.Base(path)] = info
		}
	}
	return idx
}

// noticeFinding is one notice line with its classification.
type noticeFinding struct {
	Rule          string `json:"rule"`
	Family        string `json:"family,omitempty"`
	Status        string `json:"status"`
	Path          string `json:"path,omitempty"`
	CouldNotCheck bool   `json:"could_not_check,omitempty"`
	Line          string `json:"line"`
}

// classifyNotice classifies one notice line (no NOTICE: prefix) and says whether
// it is historical.
func classifyNotice(line string, idx *recordIndex) (f noticeFinding, historical bool) {
	f = noticeFinding{Rule: noticeRuleOf(line), Line: line, CouldNotCheck: noticeIsCouldNotCheck(line)}
	included := f.Rule != ""
	if included {
		f.Family = noticeRuleFamily[f.Rule]
	} else {
		f.Rule = noticeRuleUnclassified
	}
	key, isBrief := noticeRecordKey(line)
	switch {
	case !isBrief:
		f.Status = noticeStatusNotARecord
	case idx == nil:
		f.Status = noticeStatusUnknown
	default:
		f.Path = key
		info, found := idx.byKey[key]
		switch {
		case !found || info.Status == "":
			f.Status = noticeStatusUnknown
		case isClosedStatus(info.Status) && info.Pre:
			f.Status = info.Status + "-pre-existing"
		case isClosedStatus(info.Status):
			f.Status = noticeStatusClosedThisBranch
		default:
			f.Status = noticeStatusActive
		}
	}
	_, held := noticeHeldActionable[f.Rule]
	historical = included && !held && (f.Status == noticeStatusDonePreExisting || f.Status == noticeStatusVerifiedPreExisting)
	return f, historical
}

// couldNotCheckState is a run-level statement of something this report could not
// establish.
type couldNotCheckState struct {
	Scope  string `json:"scope"`
	Reason string `json:"reason"`
}

// ruleCount is one rule's tallies.
type ruleCount struct {
	Rule          string `json:"rule"`
	Family        string `json:"family,omitempty"`
	Actionable    int    `json:"actionable"`
	Historical    int    `json:"historical"`
	Records       int    `json:"historical_records"`
	CouldNotCheck int    `json:"could_not_check"`
}

// statusCount is one record standing's tallies.
type statusCount struct {
	Status     string `json:"status"`
	Actionable int    `json:"actionable"`
	Historical int    `json:"historical"`
}

// noticeTotals are the report-wide tallies.
type noticeTotals struct {
	Notices    int `json:"notices"`
	Actionable int `json:"actionable"`
	Historical int `json:"historical"`
}

// noticeReport is the ONE value every rendering is a view of.
type noticeReport struct {
	Schema        string               `json:"schema"`
	Verdict       string               `json:"verdict"`
	Problems      int                  `json:"problems"`
	Totals        noticeTotals         `json:"totals"`
	ByRule        []ruleCount          `json:"by_rule"`
	ByStatus      []statusCount        `json:"by_status"`
	CouldNotCheck []couldNotCheckState `json:"could_not_check"`
	Actionable    []noticeFinding      `json:"actionable"`
	Historical    []noticeFinding      `json:"historical"`
}

// buildNoticeReport partitions notices (lines without the NOTICE: prefix). idx
// nil means standings are unavailable: every finding stays actionable and the
// report says so.
func buildNoticeReport(notices []string, idx *recordIndex) *noticeReport {
	rep := &noticeReport{Schema: noticeReportSchema, Actionable: []noticeFinding{}, Historical: []noticeFinding{}, CouldNotCheck: []couldNotCheckState{}}
	rules := map[string]*ruleCount{}
	histRecords := map[string]map[string]bool{}
	statuses := map[string]*statusCount{}
	for _, n := range notices {
		f, hist := classifyNotice(n, idx)
		rc := rules[f.Rule]
		if rc == nil {
			rc = &ruleCount{Rule: f.Rule, Family: f.Family}
			rules[f.Rule] = rc
		}
		sc := statuses[f.Status]
		if sc == nil {
			sc = &statusCount{Status: f.Status}
			statuses[f.Status] = sc
		}
		if f.CouldNotCheck {
			rc.CouldNotCheck++
		}
		if hist {
			rc.Historical++
			sc.Historical++
			if histRecords[f.Rule] == nil {
				histRecords[f.Rule] = map[string]bool{}
			}
			histRecords[f.Rule][f.Path] = true
			rep.Historical = append(rep.Historical, f)
		} else {
			rc.Actionable++
			sc.Actionable++
			rep.Actionable = append(rep.Actionable, f)
		}
	}
	byLine := func(s []noticeFinding) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Line < s[j].Line })
	}
	byLine(rep.Actionable)
	byLine(rep.Historical)
	for r, rc := range rules {
		rc.Records = len(histRecords[r])
		rep.ByRule = append(rep.ByRule, *rc)
	}
	sort.Slice(rep.ByRule, func(i, j int) bool { return rep.ByRule[i].Rule < rep.ByRule[j].Rule })
	for _, sc := range statuses {
		rep.ByStatus = append(rep.ByStatus, *sc)
	}
	sort.Slice(rep.ByStatus, func(i, j int) bool { return rep.ByStatus[i].Status < rep.ByStatus[j].Status })
	rep.ByRule = nonNilRuleCounts(rep.ByRule)
	rep.ByStatus = nonNilStatusCounts(rep.ByStatus)
	rep.Totals = noticeTotals{Notices: len(notices), Actionable: len(rep.Actionable), Historical: len(rep.Historical)}
	switch {
	case idx == nil:
		rep.CouldNotCheck = append(rep.CouldNotCheck, couldNotCheckState{
			Scope:  "record-standing",
			Reason: "no record index was built for this run, so no finding could be placed against a closed record; every finding is reported as actionable",
		})
	case !idx.baseOK && idx.closed > 0:
		rep.CouldNotCheck = append(rep.CouldNotCheck, couldNotCheckState{
			Scope:  "closure-baseline",
			Reason: fmt.Sprintf("the merge-base could not be resolved, so %d closed record(s) were all treated as pre-existing (historical); a closure made on this branch cannot be told apart from older history in this run", idx.closed),
		})
	}
	for _, rc := range rep.ByRule {
		if rc.CouldNotCheck > 0 {
			rep.CouldNotCheck = append(rep.CouldNotCheck, couldNotCheckState{
				Scope:  "rule:" + rc.Rule,
				Reason: fmt.Sprintf("%d finding(s) state could-not-check (counted in the totals above, never reported as a clean)", rc.CouldNotCheck),
			})
		}
	}
	return rep
}

func nonNilRuleCounts(s []ruleCount) []ruleCount {
	if s == nil {
		return []ruleCount{}
	}
	return s
}

func nonNilStatusCounts(s []statusCount) []statusCount {
	if s == nil {
		return []statusCount{}
	}
	return s
}

// historicalSummaryLines renders the bounded per-rule historical totals: one
// line per rule that has any, so the output size is bounded by the number of
// rules, not by the size of the history.
func historicalSummaryLines(rep *noticeReport) []string {
	var out []string
	for _, rc := range rep.ByRule {
		if rc.Historical == 0 {
			continue
		}
		var done, verified int
		for _, f := range rep.Historical {
			if f.Rule != rc.Rule {
				continue
			}
			if f.Status == noticeStatusDonePreExisting {
				done++
			} else {
				verified++
			}
		}
		line := fmt.Sprintf("%s %s: %d finding(s) on %d closed record(s) (%s: %d, %s: %d)",
			noticeSummaryTag, rc.Rule, rc.Historical, rc.Records,
			noticeStatusDonePreExisting, done, noticeStatusVerifiedPreExisting, verified)
		cnc := 0
		for _, f := range rep.Historical {
			if f.Rule == rc.Rule && f.CouldNotCheck {
				cnc++
			}
		}
		if cnc > 0 {
			line += fmt.Sprintf("; %d of them could-not-check", cnc)
		}
		line += " — closed records are not rewritten; --all lists each finding"
		out = append(out, line)
	}
	return out
}

// renderNoticeText writes the text form. Default: actionable findings plus the
// bounded per-rule historical totals. detail (--all): every finding as its own
// NOTICE line, as before this report existed. Both end with one plain totals
// line that is not a NOTICE (so a consumer counting NOTICE lines still counts
// findings, not summaries).
func renderNoticeText(w io.Writer, rep *noticeReport, detail bool) {
	var lines []string
	for _, f := range rep.Actionable {
		lines = append(lines, "NOTICE: "+f.Line)
	}
	if detail {
		for _, f := range rep.Historical {
			lines = append(lines, "NOTICE: "+f.Line)
		}
	} else {
		for _, s := range historicalSummaryLines(rep) {
			lines = append(lines, "NOTICE: "+s)
		}
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	fmt.Fprintf(w, "statusgen: notices: %d actionable, %d historical (%d total); historical = findings on records already closed at the merge-base — counted, never hidden\n",
		rep.Totals.Actionable, rep.Totals.Historical, rep.Totals.Notices)
	for _, c := range rep.CouldNotCheck {
		if strings.HasPrefix(c.Scope, "rule:") {
			continue // already visible in the per-rule summary line
		}
		// NOTICE-prefixed and tagged so the consumers that keep only
		// PROBLEM:/NOTICE: lines (difflint, the lint audit, evidence readers) carry
		// it, under one stable rule key, instead of dropping it. It is a state of
		// the report, not a finding, and is not counted in the totals.
		fmt.Fprintf(w, "NOTICE: %s %s: %s\n", noticeCouldNotCheckTag, c.Scope, c.Reason)
	}
}

// renderNoticeJSON is the deterministic machine report: fixed field order,
// sorted slices, no maps, no timestamps.
func renderNoticeJSON(w io.Writer, rep *noticeReport) {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: notice report could not be encoded:", err)
		return
	}
	fmt.Fprintln(w, string(b))
}

// ---- run wiring ----

var (
	// lintDetailAll is --all: restore each historical finding as its own line.
	lintDetailAll bool
	// lintJSONMode is --lint --json.
	lintJSONMode bool
	// activeRecordIndex is the current run's record standings (nil before the
	// streams are loaded).
	activeRecordIndex *recordIndex
	// pendingNoticeReport holds the report for the JSON verdict.
	pendingNoticeReport *noticeReport
)

// emitLintNotices is the run's notice emitter in EVERY mode: it reports through
// the shared noticeReport, so a pre-existing closed record's findings are
// counted and summarised per rule wherever they are produced (the write, check
// and record runs reach the same unfailable-row audit as --lint). Only --lint
// has the opt-in detail (--all) and the machine form (--json); every other mode
// gets the default bounded text rendering.
func emitLintNotices(mode string, notices []string) {
	rep := buildNoticeReport(notices, activeRecordIndex)
	if mode == "lint" && lintJSONMode {
		pendingNoticeReport = rep
		return
	}
	renderNoticeText(os.Stderr, rep, lintDetailAll)
}

// emitLintJSONVerdict writes the JSON document in place of the LINT: line.
func emitLintJSONVerdict(n int) {
	rep := pendingNoticeReport
	if rep == nil {
		rep = buildNoticeReport(nil, nil)
		rep.CouldNotCheck = append(rep.CouldNotCheck, couldNotCheckState{
			Scope:  "notice-report",
			Reason: "the run ended before it produced its notices; the counts above are empty because nothing was checked, not because nothing was found",
		})
	}
	pendingNoticeReport = nil
	rep.Problems = n
	rep.Verdict = "PASS"
	if n > 0 {
		rep.Verdict = "FAIL"
	}
	renderNoticeJSON(os.Stdout, rep)
}
