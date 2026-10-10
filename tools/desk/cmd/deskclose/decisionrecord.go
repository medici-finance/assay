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
// copy standing. Every outcome is named in the lane's audit detail as `decision-record: …`.

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
		Ruling:         deskkit.Comment{DatabaseID: ruling.ID, Body: ruling.Body, CreatedAt: ruling.CreatedAt},
		Now:            decisionRecordNow(),
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
// on stderr and in the returned audit status, and never fails the close.
func appendTriageRecord(line []byte) string {
	if err := deskkit.AppendDecisionRecord(line); err != nil {
		msg := recordStatusLocalUnwritten + " (" + deskkit.StripControl(err.Error()) +
			"; the close comment's block holds the record)"
		fmt.Fprintln(decisionRecordStderr, "deskclose: "+msg)
		return msg
	}
	return recordStatusWritten
}

// joinDetail appends a record status to an audit detail ("" adds nothing).
func joinDetail(detail, status string) string {
	if strings.TrimSpace(status) == "" {
		return detail
	}
	return detail + "; " + status
}
