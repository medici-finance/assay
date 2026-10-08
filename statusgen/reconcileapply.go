package main

// reconcileapply.go — the `statusgen reconcile --apply` write arm
// (derived-board/07 follow-on; trailer-only mode and the lint hold,
// derived-board/04).
//
// `reconcile --backfill [--report]` (reconcile.go, reconcilebackfill.go) is
// READ-ONLY: it derives a cell and can write a drift REPORT, but nothing
// writes the derived value back into a stream README's Status column — a
// confirmed wiring gap (a downstream regen workflow expected --backfill to
// flip stale board rows and it never does). --apply closes exactly that gap,
// under conditions narrow enough to stay safe unattended:
//
//   - Only the todo|in-progress → implemented transition is ever written —
//     never verified/done (those need the separate verify-witness fold,
//     out of scope here) and never a demotion.
//   - Only when the derived cell is backed by a REAL merged-PR witness: the
//     normal trailer fold (Source "pr") or, ONLY when --backfill is also
//     given, the declared backfill branch/body match (Source "backfill").
//     Without --backfill the write is trailer-only — the mode the scheduled
//     reconcile job runs, because a branch/body match also catches PRs that
//     merely author or mention a brief. A trailer witness the board should
//     not take is fixed at the trailer (spec §2), not by closing the job's PR. The backfill's OTHER Source=="backfill"
//     shape — a hand-asserted implemented/verified/done with no PR at all —
//     derives `unknown`, not `implemented`, so it is already excluded by the
//     Cell check below; it is exactly what --report leaves for a human to
//     resolve with a `Brief:` trailer, never guessed at here.
//   - The write touches ONLY the row's Status cell. Verified and Reviewed are
//     read back byte-for-byte from the existing cell text and never altered,
//     so a `human:<name>` sign-off stamp already sitting in Reviewed (or
//     anywhere else in the row) survives untouched — that stamp is the sole
//     province of verify-gate-close.yml, a separate control this verb must
//     never approximate.
//   - A row whose current Status is anything other than todo/in-progress
//     (already implemented, verified, done, or blocked) is left completely
//     alone, even when the derivation above found a witness — done/verified
//     rows are immutable to this verb.
//   - A row with no witness at all (still `todo`) is left untouched — exactly
//     what --report already lists for a human, never guessed at here.
//   - A row whose move would add a lint PROBLEM (a risk-gated brief with no
//     design record, say) is HELD: reported with the PROBLEM, never written
//     (reconcilehold.go). The lint itself is never relaxed to let a write in.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appliedRow is one stream-README Status cell reconcile --apply actually
// wrote, for the printed/JSON report of what changed.
type appliedRow struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Path    string `json:"readme"`
	Witness string `json:"witness"`
}

// witnessedImplemented reports whether c is a real PR-backed `implemented`
// witness eligible for --apply: the normal trailer fold or the declared
// backfill branch/body match. A witness-sourced verified/done cell, a
// blocked/none cell, and the backfill's no-PR hand-said `unknown` shape are
// all excluded by construction — none of them is Cell=="implemented" with
// Source in {"pr","backfill"}.
func witnessedImplemented(c BriefCell) bool {
	return c.Cell == "implemented" && (c.Source == "pr" || c.Source == "backfill")
}

// applyReconcileWrites writes the derived `implemented` cell back into each
// witnessed brief's stream README Status cell, and returns the rows it
// actually changed plus the rows it HELD. It is idempotent: a brief already at
// implemented (or beyond) or with no witness is silently skipped, never an
// error, so a re-run with nothing left to do still exits clean.
//
// A row is held — reported, not written — when moving it would add a PROBLEM
// to any Status-keyed lint rule (reconcilehold.go), e.g. a risk-gated brief
// with no design record. The writer never produces a tree its own lint step
// rejects, and never weakens or skips that lint to get there.
func applyReconcileWrites(root string, cells []BriefCell) (applied []appliedRow, held []heldRow, err error) {
	defer beginGitReadSession()()
	var env *lintEnv
	for _, c := range cells {
		if !witnessedImplemented(c) {
			continue
		}
		stream, num, ok := briefStreamNum(c.ID)
		if !ok {
			continue
		}
		path := filepath.Join(root, "docs", "streams", stream, "README.md")
		writable, from, perr := statusCellWritable(path, num)
		if perr != nil {
			return applied, held, fmt.Errorf("%s: %w", path, perr)
		}
		if !writable {
			continue
		}
		if env == nil {
			if env, err = loadLintEnv(root); err != nil {
				return applied, held, fmt.Errorf("loading the board for the lint hold: %w", err)
			}
		}
		var wrote bool
		var wroteFrom string
		added, found, merr := env.tryMove(stream, num, "implemented", func() (func() error, error) {
			orig, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, rerr
			}
			var werr error
			if wrote, wroteFrom, werr = writeStatusCell(path, num, "implemented"); werr != nil {
				return nil, werr
			}
			return func() error { return os.WriteFile(path, orig, 0o644) }, nil
		}, true)
		if merr != nil {
			return applied, held, fmt.Errorf("%s: %w", path, merr)
		}
		if !found {
			added = []string{couldNotCheckRow(c.ID)}
		}
		if len(added) > 0 {
			held = append(held, heldRow{ID: c.ID, From: from, To: "implemented", Path: path, Witness: c.Witness, Problems: added})
			continue
		}
		if wrote {
			applied = append(applied, appliedRow{ID: c.ID, From: wroteFrom, To: "implemented", Path: path, Witness: c.Witness})
		}
	}
	return applied, held, nil
}

// isBriefsTableHeaderRow reports whether cells is a briefs-table header row —
// one naming both a `Brief` and a `Status` column, the same two-name gate
// parseBriefTable requires — and if so, the column index Status sits at.
// Matching by NAME rather than a fixed offset means a hand-written table with
// an extra or reordered column (an inserted `Gate` column, say) still gets
// its Status cell found correctly instead of a positional guess landing on
// the wrong cell.
func isBriefsTableHeaderRow(cells []string) (statusIdx int, ok bool) {
	sawBrief := false
	statusIdx = -1
	for i, c := range cells {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "brief":
			sawBrief = true
		case "status":
			statusIdx = i
		}
	}
	return statusIdx, sawBrief && statusIdx >= 0
}

// isSeparatorOrHeaderNum reports whether a row's first cell marks it as a
// table header (`#`) or markdown separator (`---`) rather than a real brief
// number — the rows writeStatusCell must never mistake for data.
func isSeparatorOrHeaderNum(num string) bool {
	return num == "" || num == "#" || strings.HasPrefix(num, "---")
}

// cellPadding splits a raw table cell into its leading/trailing whitespace and
// its trimmed content, so a replacement value can be written back with the
// exact same padding style the surrounding row already uses.
func cellPadding(cell string) (leading, trailing string) {
	trimmed := strings.TrimSpace(cell)
	idx := strings.Index(cell, trimmed)
	if trimmed == "" || idx < 0 {
		return " ", " "
	}
	return cell[:idx], cell[idx+len(trimmed):]
}

// writeStatusCell edits IN PLACE the Status cell of the row for brief num in
// path's briefs table — marker-wrapped (derived-board/04 generated) or plain
// hand-written, the row shape is identical either way — replacing it with
// newStatus. It writes ONLY when the row's CURRENT Status is "todo" or
// "in-progress" (case-insensitive); any other current value (already
// implemented, verified, done, blocked, or some unrecognised token) is left
// byte-for-byte untouched and wrote is false — that is the narrow-transition
// guarantee, enforced here rather than trusted to the caller. Every other
// cell in the row — Verified, Reviewed, any extra column, and the row's own
// padding style — is preserved exactly; only the Status cell's inner text
// changes.
//
// wrote is false with err nil (never an error) when the README does not
// exist, carries no briefs table, or has no row for num — an --apply caller
// already knows a witness exists for the id; failing to find a row for it is
// a pre-existing board inconsistency, not a failure of this write.
func writeStatusCell(path, num, newStatus string) (wrote bool, from string, err error) {
	return editStatusCell(path, num, newStatus, true)
}

// statusCellWritable reports, WITHOUT writing, whether writeStatusCell would
// change brief num's Status cell in path (the row exists and sits at todo or
// in-progress) and what it reads now. applyReconcileWrites asks this first so
// the lint-hold evaluation only runs for a row the write would really move.
func statusCellWritable(path, num string) (writable bool, from string, err error) {
	return editStatusCell(path, num, "", false)
}

// editStatusCell is writeStatusCell's body; with write=false it stops at the
// decision and touches nothing.
func editStatusCell(path, num, newStatus string, write bool) (wrote bool, from string, err error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	lines := strings.Split(string(raw), "\n")
	statusIdx := -1
	headerWidth := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			statusIdx = -1 // a non-table line ends whatever table preceded it
			continue
		}
		cells := splitRow(line)
		if idx, ok := isBriefsTableHeaderRow(cells); ok {
			statusIdx, headerWidth = idx, len(cells)
			continue
		}
		if statusIdx < 0 || len(cells) != headerWidth {
			continue // separator row, malformed row, or no header seen yet
		}
		rowNum := strings.TrimSpace(cells[0])
		if isSeparatorOrHeaderNum(rowNum) || rowNum != num {
			continue
		}
		cur := cells[statusIdx]
		curTrim := strings.ToLower(strings.TrimSpace(cur))
		if curTrim != "todo" && curTrim != "in-progress" {
			return false, curTrim, nil // narrow transition only — every other state is immutable here
		}
		if !write {
			return true, curTrim, nil
		}
		leading, trailing := cellPadding(cur)
		cells[statusIdx] = leading + newStatus + trailing
		lines[i] = "|" + strings.Join(cells, "|") + "|"
		if werr := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); werr != nil {
			return false, curTrim, werr
		}
		return true, curTrim, nil
	}
	return false, "", nil
}
