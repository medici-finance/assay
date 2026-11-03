package main

// repair.go — creating the durable REPAIR OBLIGATION a failed/blocked verify owes (example-stream/17).
//
// example-stream/16 records WHY a failed verification should not simply re-run (a wake receipt).
// This is the other half: an ACTIONABLE failed verification is durable worker work that must survive
// the reporting agent. When Land observes a FAIL or a BLOCKED/NEEDS_CONTEXT outcome, it builds a
// deskkit.RepairObligation from the STRUCTURED Result — the failing rows, a reproduction and the
// deliverable repo — and records it through RepairSink. The obligation's immutable key is derived
// from (repo, brief, source receipt, failing rows), so a re-land of the same failure reconciles to
// the SAME obligation rather than manufacturing a second — the "ambiguous remote response must be
// reconciled, not blindly reposted" rule, enforced by construction of the key.
//
// This is scheduling state, NEVER acceptance: creating the obligation does not close anything and
// does not touch the board. It sits beside the existing FileBug landing (a change failure is still
// filed), additive to it.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// RepairObligationSink records (creates or reconciles) a repair obligation durably. It is an
// interface so the SAFE dry-run default can be swapped for the real projection-appending + issue
// filing sink at cutover, and so tests can assert exactly what Land records. RecordRepairObligation
// returns a handle (the projection row / tracking issue url) and reconciles an idempotent re-post
// rather than duplicating it.
type RepairObligationSink interface {
	RecordRepairObligation(o deskkit.RepairObligation) (handle string, err error)
}

// dryRunRepairSink is the DEFAULT RepairObligationSink. It records nothing durable and files no
// issue — the dry-run ground rules forbid it and the autonomous cutover is BLOCKED-ON-HUMAN — it
// prints what the real sink WOULD append to docs/streams/repair-obligations.jsonl and file.
type dryRunRepairSink struct{ out io.Writer }

func (d dryRunRepairSink) RecordRepairObligation(o deskkit.RepairObligation) (string, error) {
	line, _ := o.MarshalLine()
	fmt.Fprintf(d.out, "[dry-run] record repair obligation %s (state=%s, blocker=%s, repo=%s, rows=%v): %s\n",
		o.ID, o.State, o.BlockerKind, o.TargetRepo(), o.Rows, string(line))
	return "dry-run://repair/" + o.ID, nil
}

// recordRepairObligation is Land's helper: it builds the obligation for a failed/blocked Result and
// records it through the sink. It is called ONLY for a non-passing verdict — a PASS/flip records no
// obligation. A record failure is surfaced on emit but does NOT change Land's existing return: the
// FileBug escalation is the human-visible path, and the record is additive to it (the cutover sink
// hardens "inability to file is an unlanded obligation" once real filing replaces the emitter).
func (v *VerifyLoop) recordRepairObligation(r loopengine.Result) {
	o := buildRepairObligation(r, v.now(), v.RootRepo)
	if _, err := v.repairSink().RecordRepairObligation(o); err != nil {
		fmt.Fprintf(v.emit(), "verifyloop: NOTE: could not record repair obligation %s: %v — the FileBug escalation still landed\n", o.ID, err)
	}
}

// buildRepairObligation maps a non-passing Result to a repair obligation. The blocker kind is
// classified from the verdict: a VERIFY FAIL is a change failure a worker must repair
// (implementation); a BLOCKED / NEEDS_CONTEXT outcome is left UNKNOWN — explicit triage, never an
// invented implementation bug (the interface-contract rule). deskkit.NewRepairObligation applies the
// routing: implementation → needs-assignment/worker, unknown → needs-assignment/verifier triage.
//
// The brief is the item's statusgen row key (itemKey: its ID minus the multi-root "<owner>/<repo>:"
// prefix), never the raw ID — the prefixed form is not a brief id and would leave the obligation
// (and the filing identity derived from it) unable to name its brief. The repository is the item's
// own deliverable repo, else homeRepo (the single-root plan's own repository, where a same-repo
// item's deliverable lands); with neither it stays empty and the filing keeps its legacy shape.
func buildRepairObligation(r loopengine.Result, ts, homeRepo string) deskkit.RepairObligation {
	repo := strings.TrimSpace(r.Item.Payload["repo"])
	if repo == "" {
		repo = strings.TrimSpace(homeRepo)
	}
	brief := strings.TrimSpace(itemKey(r.Item))
	rows := failingRowNumbers(r)
	receiptID := repairReceiptID(repo, brief, r.Item.TargetSHA, r.Verdict)
	blockerKind := deskkit.BlockerUnknown
	if r.Verdict == loopengine.VerdictFail {
		blockerKind = deskkit.BlockerImplementation
	}
	return deskkit.NewRepairObligation(
		repo, brief, receiptID, rows, blockerKind,
		repairReproduction(r), repairExpected(r), "", ts)
}

// failingRowNumbers returns the 1-based Verify-row numbers that did not pass. A FAIL names the rows
// whose command exited non-zero; a BLOCKED/NEEDS_CONTEXT result that ran no row still yields at
// least one row (row 1) so the obligation names a concrete failing row rather than an empty set.
func failingRowNumbers(r loopengine.Result) []int {
	var rows []int
	for i, row := range r.Rows {
		if row.Exit != 0 {
			rows = append(rows, i+1)
		}
	}
	if len(rows) == 0 {
		rows = []int{1}
	}
	return rows
}

// repairReproduction renders the exact failing command(s) + exit code(s), so a replacement worker
// reproduces the failure rather than re-reading the brief cold.
func repairReproduction(r loopengine.Result) string {
	var b strings.Builder
	for _, row := range r.Rows {
		if row.Exit != 0 {
			fmt.Fprintf(&b, "`%s` exited %d: %s\n", row.Command, row.Exit, strings.TrimSpace(row.Output))
		}
	}
	if b.Len() == 0 {
		fmt.Fprintf(&b, "verify %s: %s", r.Verdict, failSummary(r))
	}
	return strings.TrimSpace(b.String())
}

// repairExpected states what the failing rows must do once repaired — the acceptance the
// independent reverification will re-run.
func repairExpected(r loopengine.Result) string {
	var cmds []string
	for _, row := range r.Rows {
		if row.Exit != 0 {
			cmds = append(cmds, "`"+row.Command+"` exits 0")
		}
	}
	if len(cmds) == 0 {
		return "the brief's Verify table passes on merged main"
	}
	return strings.Join(cmds, "; ")
}

// repairReceiptID is the deterministic source-receipt id the obligation references. It is stable for
// a given (repo, brief, sha, verdict), so a re-land of the same failed verification yields the SAME
// receipt id, and therefore the SAME obligation id — idempotent re-posting, reconciled not
// duplicated. It stands in for example-stream/16's wake receipt id until receipt WRITING is wired
// into landing at cutover; the shape (a short content hash) matches deskkit's own id derivation.
func repairReceiptID(repo, brief, sha, verdict string) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(repo), strings.TrimSpace(brief), strings.TrimSpace(sha), strings.TrimSpace(verdict),
	}, "\x00")))
	return "receipt/" + hex.EncodeToString(h[:])[:16]
}

// --- filing identity (example-stream/49) -----------------------------------------------------------
//
// The bug a failed or blocked verify files is one observation of a CASE, and the case already has
// an identity: the repair obligation's immutable ID. repairFilingIdentity hands that identity to
// the filer (deskfile new --filing-brief/--filing-case with --raised-by verifier), and
// fileBug asks the repository whether an OPEN issue by this role already carries it —
// through the forge read the loop already owns, never a private cache — before anything is filed.

// FilingIdentity is the structured identity a filing carries: the canonical brief and the case id
// of the repair obligation, in the repository the obligation names.
type FilingIdentity struct {
	Repo  string
	Brief string
	Case  string
}

// repairFilingIdentity derives the identity from the obligation a Result builds. ok is false for
// an obligation that cannot carry one (no deliverable repository, a malformed brief), which keeps
// the legacy, identity-free filing.
func repairFilingIdentity(o deskkit.RepairObligation) (FilingIdentity, string, bool) {
	key, err := o.FilingKey(verifyloopRole)
	if err != nil {
		return FilingIdentity{}, "", false
	}
	return FilingIdentity{Repo: o.Repo, Brief: deskkit.CanonicalBriefID(o.Brief), Case: o.ID}, key, true
}

// FiledCaseSource is the existing issue source's exact read: the open issues in repo that carry
// key, were raised by the verifier and were written by a trusted author. Issues that carry the
// marker but fail the author check come back in untrusted (reported, never treated as the case's
// filing). An error (including a truncated read) is unknown.
type FiledCaseSource interface {
	FiledCases(repo deskkit.ForgeRepo, key string) (found, untrusted []deskkit.IssueSearchResult, err error)
}

// IdentifiedBugFiler is the optional Durable capability that files with a structured identity.
// A Durable without it files exactly as before.
type IdentifiedBugFiler interface {
	FileBugFor(it loopengine.Item, detail string, id FilingIdentity) (string, error)
}

// FiledCases implements FiledCaseSource over the per-repo forge this source already resolves
// (allowlist, credential and request observer included); the typed forge does the paged,
// complete-or-error search.
func (s *forgeIssueSource) FiledCases(repo deskkit.ForgeRepo, key string) (found, untrusted []deskkit.IssueSearchResult, err error) {
	fg, err := s.forge(repo)
	if err != nil {
		return nil, nil, err
	}
	hits, err := fg.SearchIssues(repo, deskkit.SearchIssuesInput{FilingKey: key})
	if err != nil {
		return nil, nil, err
	}
	found, untrusted = deskkit.MatchFiledIssues(hits, key, verifyloopRole)
	return found, untrusted, nil
}

// fileBug files the bug for a non-passing Result, once per case. The order is the brief's:
// exact identity first, then whatever the Durable's own (title) matching does.
//   - identity found open: reconcile to it, print its number, file nothing;
//   - the exact read failed or was truncated: unknown, file nothing and say so;
//   - not found, or no identity / no exact source: file as before (with the identity when the
//     Durable can carry it).
func (v *VerifyLoop) fileBug(d Durable, r loopengine.Result, detail string) (string, error) {
	id, key, ok := repairFilingIdentity(buildRepairObligation(r, v.now(), v.RootRepo))
	if !ok {
		return d.FileBug(r.Item, detail)
	}
	if src, isSrc := v.Issues.(FiledCaseSource); isSrc {
		owner, name, _ := strings.Cut(id.Repo, "/")
		found, untrusted, err := src.FiledCases(deskkit.ForgeRepo{Owner: owner, Name: name}, key)
		if err != nil {
			fmt.Fprintf(v.emit(), "verifyloop: could-not-check whether %s case %s is already filed (%s) — nothing filed\n",
				id.Brief, id.Case, oneLine(err))
			return "", fmt.Errorf("could not check whether %s case %s is already filed: %w", id.Brief, id.Case, err)
		}
		for _, u := range untrusted {
			fmt.Fprintf(v.emit(), "verifyloop: NOTICE: %s#%d carries %s case %s's filing-key marker but its author is not trusted — ignored, not the case's filing\n",
				id.Repo, u.Number, id.Brief, id.Case)
		}
		if len(found) > 0 {
			fmt.Fprintf(v.emit(), "verifyloop: %s case %s is already filed as %s#%d — reconciled, nothing filed\n",
				id.Brief, id.Case, id.Repo, found[0].Number)
			return found[0].URL, nil
		}
	}
	if f, ok := d.(IdentifiedBugFiler); ok {
		return f.FileBugFor(r.Item, detail, id)
	}
	return d.FileBug(r.Item, detail)
}
