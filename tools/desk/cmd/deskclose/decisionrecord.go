package main

// decisionrecord.go — the human-decided lane's human-decision-v1 record
// (schema, field table and never-recorded list: the human-decision-v1 schema doc).
//
// The record is composed from what the lane already FETCHED and verified — the item (body,
// creation time), the issue's comment thread, and the author-verified ruling comment — and
// written twice by independent paths: as a hidden block in the close comment (the forge copy,
// riding the existing charged write) and, after the close succeeds, as one line in
// decision-records.jsonl beside the audit log (the local copy). Neither copy ever gates the
// close: an invalid record is not written at all, and a local append failure leaves the forge
// copy standing, and a block the outbound check refuses is dropped from the comment (the close
// proceeds on the prose alone, and the local copy still lands). Every outcome is named in the
// lane's audit detail as `decision-record: …`.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Test seams, in the dirOverride mould: production uses the wall clock and stderr.
var (
	decisionRecordNow              = time.Now
	decisionRecordStderr io.Writer = os.Stderr
)

// The audit statuses this lane can name (each follows the close's own detail).
const (
	recordStatusComposed       = "decision-record: forge copy in the close comment"
	recordStatusWritten        = "decision-record: forge+local"
	recordStatusLocalUnwritten = "decision-record: local-unwritten"
	// The outbound check refused the comment WITH the block: the prose-only comment was posted
	// instead, so the local line is the only copy (or, when that append fails too, none).
	recordStatusBlockRefused  = "decision-record: local only (the outbound check refused the block)"
	recordStatusNoCopyWritten = "decision-record: unrecorded (the outbound check refused the block; local-unwritten)"
)

// composeTriageRecord builds and encodes the record for a human-decided close. It returns the
// encoded line (nil when the record is not written) and the audit status naming why.
func composeTriageRecord(r triageReq, it item, ruling ghComment) ([]byte, string) {
	fg, fr, err := forgeForFn(r.repo)
	if err != nil {
		return nil, "decision-record: unrecorded (forge unresolved)"
	}
	comments, err := fg.ListCommentsTyped(fr, r.number, deskkit.TargetIssue)
	if err != nil {
		return nil, "decision-record: unrecorded (comment thread unreadable)"
	}
	rul := deskkit.Comment{DatabaseID: ruling.ID, Body: ruling.Body, CreatedAt: ruling.CreatedAt}
	if deskkit.DecisionRulingIndex(comments, rul) < 0 {
		// The ruling was verified on an earlier read of this thread; a re-read that no longer
		// carries it cannot say what came before it.
		return nil, "decision-record: unrecorded (the ruling is not in the re-read thread)"
	}
	// The body's last edit time, for the record's edit check. A read failure, or a forge that
	// does not report body history, is `unverifiable` in the record — never `unedited`.
	bodyEditedAt, bodyEditKnown := "", false
	if tp, terr := fg.IssueTrustEvents(fr, r.number); terr == nil && tp != nil && tp.BodyHistoryKnown {
		bodyEditKnown = true
		if !tp.BodyEdited.IsZero() {
			bodyEditedAt = tp.BodyEdited.UTC().Format(time.RFC3339)
		}
	}
	tracker := ""
	if r.tracker != nil {
		tracker = renderRef(*r.tracker)
	}
	rec := deskkit.ComposeDecisionRecord(deskkit.DecisionInput{
		Repo:           r.repo,
		Issue:          r.number,
		Tracker:        tracker,
		IssueBody:      it.Body,
		IssueCreatedAt: it.CreatedAt,
		Comments:       comments,
		Ruling:         rul,
		Now:            decisionRecordNow(),

		IssueBodyEditedAt:  bodyEditedAt,
		IssueBodyEditKnown: bodyEditKnown,
	})
	line, err := deskkit.EncodeDecisionRecord(rec)
	if err != nil {
		var de *deskkit.DecisionRecordError
		if errors.As(err, &de) {
			return nil, "decision-record: invalid (" + de.Field + ")"
		}
		return nil, "decision-record: invalid (encoding)"
	}
	return line, recordStatusComposed
}

// appendTriageRecord writes the local copy after the close succeeded. A failure is reported
// on stderr and in the returned audit status, and never fails the close. forgePosted says
// whether the close comment carries the block (false when the outbound check refused it).
func appendTriageRecord(line []byte, forgePosted bool) string {
	if err := deskkit.AppendDecisionRecord(line); err != nil {
		msg := recordStatusLocalUnwritten + " (" + deskkit.StripControl(err.Error()) +
			"; the close comment's block holds the record)"
		if !forgePosted {
			msg = recordStatusNoCopyWritten + " (" + deskkit.StripControl(err.Error()) + ")"
		}
		fmt.Fprintln(decisionRecordStderr, "deskclose: "+msg)
		return msg
	}
	if !forgePosted {
		return recordStatusBlockRefused
	}
	return recordStatusWritten
}

// outboundRefused reports whether a failed comment write was refused by the outbound check,
// which refuses BEFORE any write reaches the forge: a refusal (exit 5) anywhere in the
// error's chain. postComment wraps every write failure as could-not-check, so the chain, not
// the outer code, carries the answer.
func outboundRefused(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if de, ok := e.(*deskkit.DeskError); ok && de.Code == deskkit.ExitRefused {
			return true
		}
	}
	return false
}

// joinDetail appends a record status to an audit detail ("" adds nothing).
func joinDetail(detail, status string) string {
	if strings.TrimSpace(status) == "" {
		return detail
	}
	return detail + "; " + status
}
