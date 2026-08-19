package deskkit

// disposition.go is the CANONICAL schema for a PR DISPOSITION RECORD — a worker's
// terminal verdict on a pull request that no longer needs code work.
//
// The defect it closes: a worker that investigates an orphaned PR and
// concludes "this is superseded, recommend close" writes that conclusion as a PROSE
// COMMENT. The next orphan sweep reads PR-level signals only (reviewDecision, CI
// conclusion, staleness) and cannot see the conclusion at all, so it re-dispatches the
// same PR to a fresh worker who re-derives the same answer. In one 2026-08-12 cycle 8 of
// 10 completed orphan dispatches were re-derivations of an already-posted conclusion;
// one PR was re-derived FOUR times across three weeks.
//
// The fix is derive-or-diff applied to a verdict: the conclusion gets exactly ONE
// declared source — a structured record on the PR — and every consumer reads that record
// instead of re-deriving it. The record is written in two forms, deliberately:
//
//   - a LABEL (`disposition:superseded`), because that is what a sweep can filter on in
//     the query it already runs (`gh pr list --json number,labels`) with no extra API
//     call per PR; and
//   - a MARKER COMMENT (the `<!-- desk-disposition v1 -->` envelope below), because a
//     label carries no evidence link, no author and no date, and a disposition without
//     its evidence is the same unfalsifiable assertion the prose comment was.
//
// Neither half is redundant: the label is the INDEX, the marker is the RECORD. A reader
// that finds one without the other says so (see ReadDisposition) rather than silently
// picking a winner.
//
// WHO DOES WHAT. The worker WRITES the record. The record does not close anything —
// closing a PR is a human-authorized event and belongs to `deskclose`,
// which consumes these records as its work queue. That split is why the schema is
// designed for a SWEEP to read, not for a human to skim: deskclose must be able to pull
// verdict + evidence off a PR without an LLM in the loop.
//
// THREE-STATE. A sweep that cannot read labels/comments (API error, rate limit,
// truncated page) reports could-not-check and treats the PR as NOT dispatch-eligible.
// It never reports "no disposition" for a read it could not perform — a checker that
// could not look must never report green.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DispositionVerdict is the closed vocabulary of terminal worker verdicts.
// It is closed on purpose: an open vocabulary is a prose comment with extra steps.
type DispositionVerdict string

const (
	// DispositionSuperseded — the work this PR carries already landed through a
	// DIFFERENT branch or PR. Evidence is the superseding PR/commit. Not
	// dispatch-eligible; queued for deskclose.
	DispositionSuperseded DispositionVerdict = "SUPERSEDED"
	// DispositionResolvedElsewhere — the OUTCOME this PR targets was reached by
	// another path: the issue it closes is already closed by someone else, or the
	// brief row it implements already reached its target status on main. The diff
	// itself may never have landed anywhere. Evidence is the issue/brief that
	// settled it. Not dispatch-eligible; queued for deskclose.
	DispositionResolvedElsewhere DispositionVerdict = "RESOLVED-ELSEWHERE"
	// DispositionNeedsRebase — live work, mechanically blocked (conflicting with
	// main, stale base). This one IS still dispatch-eligible: a worker can act on it.
	// It is in the vocabulary so "I looked, and it is still real work" is recordable
	// — otherwise the only recordable outcomes are terminal ones and a sweep learns
	// nothing from an investigation that found live work.
	DispositionNeedsRebase DispositionVerdict = "NEEDS-REBASE"
)

// DispositionVerdicts returns the vocabulary in declaration order. Callers render
// usage text and validate input from this — there is no second list to drift.
func DispositionVerdicts() []DispositionVerdict {
	return []DispositionVerdict{DispositionSuperseded, DispositionResolvedElsewhere, DispositionNeedsRebase}
}

// SuppressesDispatch reports whether a PR carrying this verdict must NOT be handed to a
// fresh worker. The terminal verdicts suppress; NEEDS-REBASE does not.
//
// This is the single predicate the orphan sweep asks. It lives here rather than in the
// sweep so that adding a verdict cannot leave one consumer classifying it as dispatchable
// and another as terminal.
func (v DispositionVerdict) SuppressesDispatch() bool {
	return v == DispositionSuperseded || v == DispositionResolvedElsewhere
}

// Label renders the verdict as its GitHub label, e.g. "disposition:superseded".
// Lower-case because GitHub label search is case-preserving and the sweep's filter is a
// literal string compare.
func (v DispositionVerdict) Label() string {
	return dispositionLabelPrefix + strings.ToLower(string(v))
}

const dispositionLabelPrefix = "disposition:"

// ParseDispositionVerdict accepts either the bare vocabulary word or its label form, in
// any case, and returns the canonical verdict. Unknown input is REFUSED rather than
// coerced — a typo'd verdict that silently became a valid one would write a wrong
// terminal record onto a live PR.
func ParseDispositionVerdict(s string) (DispositionVerdict, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	t = strings.TrimPrefix(t, strings.ToUpper(dispositionLabelPrefix))
	for _, v := range DispositionVerdicts() {
		if t == string(v) {
			return v, nil
		}
	}
	var names []string
	for _, v := range DispositionVerdicts() {
		names = append(names, string(v))
	}
	return "", Refused(fmt.Sprintf("unknown disposition verdict %q — the vocabulary is closed: %s",
		StripControl(s), strings.Join(names, " | ")))
}

// ParseDispositionLabel maps a GitHub label back to a verdict. ok is false for any label
// that is not a disposition label.
func ParseDispositionLabel(label string) (DispositionVerdict, bool) {
	l := strings.ToLower(strings.TrimSpace(label))
	if !strings.HasPrefix(l, dispositionLabelPrefix) {
		return "", false
	}
	v, err := ParseDispositionVerdict(l)
	if err != nil {
		return "", false
	}
	return v, true
}

// Disposition is one complete record.
//
// Evidence is REQUIRED for the terminal verdicts. "Superseded" with no link to the thing
// that superseded it is exactly the unfalsifiable claim this record replaces — the next
// reader would have to re-derive it, which is the defect.
type Disposition struct {
	Verdict    DispositionVerdict
	Evidence   string // URL or owner/repo#N of the superseding PR / settling issue
	RecordedBy string // session tag or App slug — forensics, self-reported
	RecordedAt string // YYYY-MM-DD
}

// DispositionMarkerOpen is the envelope that identifies a disposition comment. It is an
// HTML comment so it is invisible in rendered Markdown, and it is versioned so a future
// schema change is detectable rather than silently mis-parsed.
const DispositionMarkerOpen = "<!-- desk-disposition v1 -->"

// Validate checks a record is complete enough to be worth writing.
func (d Disposition) Validate() error {
	if _, err := ParseDispositionVerdict(string(d.Verdict)); err != nil {
		return err
	}
	if d.Verdict.SuppressesDispatch() && strings.TrimSpace(d.Evidence) == "" {
		return Refused(fmt.Sprintf("a %s disposition requires --evidence: a terminal verdict with no "+
			"link to what settled it is the unfalsifiable prose comment this record exists to replace",
			d.Verdict))
	}
	if strings.TrimSpace(d.RecordedAt) == "" {
		return Refused("disposition requires a recorded date (YYYY-MM-DD)")
	}
	return nil
}

// Marker renders the record as the comment body a worker posts on the PR.
//
// The field lines are plain `Key: value` on their own lines INSIDE the envelope, so the
// record survives GitHub's Markdown rendering, `gh pr view --json comments` (which
// returns the raw body), and a plain `grep`. No JSON: a body that must be valid JSON is a
// body a human editing the PR can break, and the record has to tolerate a human adding
// prose underneath it.
func (d Disposition) Marker() string {
	var b strings.Builder
	b.WriteString(DispositionMarkerOpen + "\n")
	fmt.Fprintf(&b, "Disposition: %s\n", d.Verdict)
	if e := strings.TrimSpace(d.Evidence); e != "" {
		fmt.Fprintf(&b, "Evidence: %s\n", e)
	}
	if by := strings.TrimSpace(d.RecordedBy); by != "" {
		fmt.Fprintf(&b, "Recorded-By: %s\n", by)
	}
	fmt.Fprintf(&b, "Recorded-At: %s\n", strings.TrimSpace(d.RecordedAt))
	b.WriteString("\n")
	b.WriteString(dispositionFooter(d.Verdict))
	return b.String()
}

func dispositionFooter(v DispositionVerdict) string {
	if v.SuppressesDispatch() {
		return "This PR is not eligible for orphan re-dispatch. Closing it is a human-authorized " +
			"event — it is queued for `deskclose`, not closed by this record.\n"
	}
	return "This PR is still live work: the orphan sweep may re-dispatch it.\n"
}

var dispositionFieldRe = regexp.MustCompile(`(?i)^\s*(Disposition|Evidence|Recorded-By|Recorded-At)\s*:\s*(.*)$`)

// ParseDispositionMarker extracts the record from a comment body. ok is false when the
// body carries no envelope, or carries one whose Disposition: line is missing or names a
// word outside the vocabulary.
//
// Lines inside a fenced code block are SKIPPED. That is not decoration: this schema is
// quoted verbatim in docs/brief-rules.md and in the two skills, and a documentation
// example must never read as a live verdict on the PR that adds it.
func ParseDispositionMarker(body string) (Disposition, bool) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	fenced := false
	inRecord := false
	var d Disposition
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.Contains(ln, DispositionMarkerOpen) {
			inRecord = true
			continue
		}
		if !inRecord {
			continue
		}
		m := dispositionFieldRe.FindStringSubmatch(ln)
		if m == nil {
			// A blank line does not end the record (the rendered marker has one
			// before its footer); any other non-field line does — prose below the
			// record is the human's, not ours to parse.
			if strings.TrimSpace(ln) == "" {
				continue
			}
			break
		}
		val := strings.TrimSpace(m[2])
		switch strings.ToLower(m[1]) {
		case "disposition":
			v, err := ParseDispositionVerdict(val)
			if err != nil {
				return Disposition{}, false
			}
			d.Verdict = v
		case "evidence":
			d.Evidence = val
		case "recorded-by":
			d.RecordedBy = val
		case "recorded-at":
			d.RecordedAt = val
		}
	}
	if d.Verdict == "" {
		return Disposition{}, false
	}
	return d, true
}

// The three states of a disposition read. These are the standard instrument
// states, spelled the way every other three-state instrument in this tree spells them, so
// a log line from this sweep is greppable alongside the rest.
const (
	// DispositionCheckedClean — the read SUCCEEDED and found no record. The PR is a
	// genuine dispatch candidate.
	DispositionCheckedClean = "checked-clean"
	// DispositionCheckedFailed — the read succeeded and FOUND a record. "Failed" is
	// the instrument's word for "the check fired", not a judgement about the PR.
	DispositionCheckedFailed = "checked-failed"
	// DispositionCouldNotCheck — the read could not be performed. The PR is treated
	// as NOT dispatch-eligible: re-dispatching on an unread record is exactly the
	// re-derivation waste this record closes, and the cost of skipping one live PR for a cycle is one cycle.
	DispositionCouldNotCheck = "could-not-check"
)

// DispositionRead is the result of asking "does this PR carry a disposition?".
type DispositionRead struct {
	State  string // one of the three constants above
	Record Disposition
	// Reason carries the human-readable why for could-not-check, and the
	// index/record disagreement note when only one half of the record is present.
	Reason string
}

// DispatchEligible reports whether the orphan sweep may hand this PR to a fresh worker.
//
// It is false for could-not-check. That is the whole point of the third state: an
// instrument that cannot read must not answer the question it was asked.
func (r DispositionRead) DispatchEligible() bool {
	switch r.State {
	case DispositionCheckedClean:
		return true
	case DispositionCheckedFailed:
		return !r.Record.Verdict.SuppressesDispatch()
	default:
		return false
	}
}

// DispositionLabelsIn returns the disposition verdicts carried by a label set, sorted so
// a multi-label PR classifies deterministically.
func DispositionLabelsIn(labels []string) []DispositionVerdict {
	var out []DispositionVerdict
	for _, l := range labels {
		if v, ok := ParseDispositionLabel(l); ok {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadDispositionIndex is the CHEAP read: labels only.
//
// It exists because the orphan sweep's own query is one `gh pr list --json
// number,...,labels` per repo, and comments are not available from that endpoint. Adding
// a per-PR `gh pr view --json comments` to a sweep over ~80 open PRs across twelve repos
// is exactly the fan-out that trips GitHub's secondary rate limit — and a sweep that
// rate-limits reports could-not-check for the whole repo, which is worse than reading the
// index.
//
// So the sweep filters on the index and never needs the evidence: it is deciding whether
// to DISPATCH, and the verdict alone settles that. deskclose, which needs the evidence to
// act, pays for the full ReadDisposition on the far shorter list this hands it.
func ReadDispositionIndex(labels []string, readErr error) DispositionRead {
	if readErr != nil {
		return DispositionRead{
			State:  DispositionCouldNotCheck,
			Reason: "disposition index read failed: " + StripControl(readErr.Error()),
		}
	}
	vs := DispositionLabelsIn(labels)
	if len(vs) == 0 {
		return DispositionRead{State: DispositionCheckedClean}
	}
	// More than one disposition label is a writer bug; take the restrictive one.
	pick := vs[0]
	for _, v := range vs {
		if v.SuppressesDispatch() {
			pick = v
		}
	}
	return DispositionRead{
		State:  DispositionCheckedFailed,
		Record: Disposition{Verdict: pick},
		Reason: "index read (labels only) — the evidence link lives in the " +
			DispositionMarkerOpen + " comment; fetch it with `deskdisposition read`",
	}
}

// ReadDisposition classifies one PR from what the sweep managed to fetch.
//
// readErr is the transport's own verdict: non-nil means labels or comments could not be
// fetched (API error, rate limit, truncated page) and the answer is could-not-check
// REGARDLESS of what partial data came back. Callers must pass their read error through
// rather than dropping it — a dropped error is how "no dispositions found" becomes a
// silent false green.
func ReadDisposition(labels []string, commentBodies []string, readErr error) DispositionRead {
	if readErr != nil {
		return DispositionRead{
			State:  DispositionCouldNotCheck,
			Reason: "disposition read failed: " + StripControl(readErr.Error()),
		}
	}

	labelVerdicts := DispositionLabelsIn(labels)

	// The LAST marker wins: a worker may supersede its own earlier record (e.g. a
	// NEEDS-REBASE that later turned out to be superseded), and comments arrive in
	// submission order.
	var rec Disposition
	found := false
	for _, body := range commentBodies {
		if d, ok := ParseDispositionMarker(body); ok {
			rec, found = d, true
		}
	}

	switch {
	case !found && len(labelVerdicts) == 0:
		return DispositionRead{State: DispositionCheckedClean}

	case !found:
		// Index without record. Honour the label — a sweep that ignored it would
		// re-dispatch the PR the label was put there to protect — but say the
		// evidence is missing, because deskclose cannot act on a bare label.
		return DispositionRead{
			State:  DispositionCheckedFailed,
			Record: Disposition{Verdict: labelVerdicts[0]},
			Reason: fmt.Sprintf("label %s present with no %s record — verdict honoured, but there is no "+
				"evidence link for deskclose to act on; re-record with `deskdisposition set`",
				labelVerdicts[0].Label(), DispositionMarkerOpen),
		}

	case len(labelVerdicts) == 0:
		return DispositionRead{
			State:  DispositionCheckedFailed,
			Record: rec,
			Reason: fmt.Sprintf("record present but label %s is missing — sweeps that filter on labels "+
				"alone will not see this PR; re-run `deskdisposition set` to restore the index",
				rec.Verdict.Label()),
		}

	default:
		r := DispositionRead{State: DispositionCheckedFailed, Record: rec}
		agreed := false
		for _, v := range labelVerdicts {
			if v == rec.Verdict {
				agreed = true
			}
		}
		if !agreed {
			// Disagreement resolves to the MORE restrictive answer: if either half
			// says terminal, the PR is not dispatched. A disagreement is a bug in
			// the writer, and the cheap direction to be wrong in is "one PR waits
			// for a human" rather than "a terminal PR gets a worker".
			for _, v := range labelVerdicts {
				if v.SuppressesDispatch() && !r.Record.Verdict.SuppressesDispatch() {
					r.Record.Verdict = v
				}
			}
			var ls []string
			for _, v := range labelVerdicts {
				ls = append(ls, v.Label())
			}
			r.Reason = fmt.Sprintf("label/record disagreement: labels %s vs record %s — resolved to %s "+
				"(the more restrictive answer)", strings.Join(ls, ","), rec.Verdict, r.Record.Verdict)
		}
		return r
	}
}
