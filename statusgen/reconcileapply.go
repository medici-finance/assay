package main

// reconcileapply.go — the `statusgen reconcile --backfill --apply` write arm
// (derived-board/07 follow-on).
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
//     normal trailer fold (Source "pr") or the declared backfill branch/body
//     match (Source "backfill"). The backfill's OTHER Source=="backfill"
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
//
// The read-only form (#2440): `reconcile --backfill` WITHOUT --apply reports, as
// `wouldApply`, the rows --apply would write, so a reviewer can reproduce a
// board promotion with no tree write. It is not a second implementation: both
// forms run reconcileWrites, and only its last step — write to disk, or keep
// the edited README in memory — differs. Nothing about which rows are admitted
// changes between them.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appliedRow is one stream-README Status cell reconcile --apply actually
// wrote — or, on the read-only form (--backfill without --apply), one it WOULD
// write — for the printed/JSON report of what changed. Both lists carry this
// one type, built by the one decision path (reconcileWrites), so the read-only
// report and the write can never describe a row differently. RowBefore and
// RowAfter are the whole table line before and after the edit, byte for byte,
// so a reviewer can compare them with a diff hunk directly.
type appliedRow struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Path      string `json:"readme"`
	Witness   string `json:"witness"`
	RowBefore string `json:"rowBefore"`
	RowAfter  string `json:"rowAfter"`
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
// actually changed. It is idempotent: a brief already at implemented (or
// beyond) or with no witness is silently skipped, never an error, so a
// re-run with nothing left to do still exits clean.
func applyReconcileWrites(root string, cells []BriefCell) ([]appliedRow, error) {
	return reconcileWrites(root, cells, true)
}

// planReconcileWrites is the read-only form of applyReconcileWrites: it
// returns exactly the rows applyReconcileWrites would write on the same tree
// and cells, and writes nothing. It is the same function with the final disk
// write swapped for an in-memory overlay, never a second implementation.
func planReconcileWrites(root string, cells []BriefCell) ([]appliedRow, error) {
	return reconcileWrites(root, cells, false)
}

// reconcileWrites is the ONE decision path behind both --apply and the
// read-only report. Every admission decision — which cells are eligible, which
// README and row they map to, whether the row's current Status permits the
// transition, and what the edited line is — is made here, identically in both
// modes. The modes differ only at the last step: write=true writes the edited
// README to disk; write=false keeps it in an overlay that later reads of the
// same README see, so a second cell mapping to the same file is decided
// against exactly the content --apply would have left there.
func reconcileWrites(root string, cells []BriefCell, write bool) ([]appliedRow, error) {
	var rows []appliedRow
	overlay := map[string]string{}
	for _, c := range cells {
		if !witnessedImplemented(c) {
			continue
		}
		stream, num, ok := briefStreamNum(c.ID)
		if !ok {
			continue
		}
		path := filepath.Join(root, "docs", "streams", stream, "README.md")
		content, ok := overlay[path]
		if !ok {
			raw, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue // no README for the stream: a board inconsistency, not a write failure
			}
			if err != nil {
				return rows, fmt.Errorf("%s: %w", path, err)
			}
			content = string(raw)
		}
		edit, changed := rewriteStatusCell(content, num, "implemented")
		if !changed {
			continue
		}
		if write {
			if err := os.WriteFile(path, []byte(edit.content), 0o644); err != nil {
				return rows, fmt.Errorf("%s: %w", path, err)
			}
		} else {
			overlay[path] = edit.content
		}
		rows = append(rows, appliedRow{
			ID: c.ID, From: edit.from, To: "implemented", Path: path, Witness: c.Witness,
			RowBefore: edit.rowBefore, RowAfter: edit.rowAfter,
		})
	}
	return rows, nil
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

// statusCellEdit is the outcome of rewriteStatusCell: the whole README after
// the edit, the row's previous Status token, and the edited table line before
// and after.
type statusCellEdit struct {
	content   string
	from      string
	rowBefore string
	rowAfter  string
}

// writeStatusCell edits IN PLACE the Status cell of the row for brief num in
// path's briefs table, via rewriteStatusCell (which holds the whole rule), and
// writes the result back only when the rule permits the edit. wrote is false
// with err nil (never an error) when the README does not exist, carries no
// briefs table, or has no row for num — an --apply caller already knows a
// witness exists for the id; failing to find a row for it is a pre-existing
// board inconsistency, not a failure of this write.
func writeStatusCell(path, num, newStatus string) (wrote bool, from string, err error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	edit, changed := rewriteStatusCell(string(raw), num, newStatus)
	if !changed {
		return false, edit.from, nil
	}
	if werr := os.WriteFile(path, []byte(edit.content), 0o644); werr != nil {
		return false, edit.from, werr
	}
	return true, edit.from, nil
}

// rewriteStatusCell is the pure rule behind every Status-cell write. It finds
// the row for brief num in content's briefs table — marker-wrapped
// (derived-board/04 generated) or plain hand-written, the row shape is
// identical either way — and replaces its Status cell with newStatus. It edits
// ONLY when the row's CURRENT Status is "todo" or "in-progress"
// (case-insensitive); any other current value (already implemented, verified,
// done, blocked, or some unrecognised token) is left byte-for-byte untouched
// and changed is false — that is the narrow-transition guarantee, enforced here
// rather than trusted to the caller. Every other cell in the row — Verified,
// Reviewed, any extra column, and the row's own padding style — is preserved
// exactly; only the Status cell's inner text changes. changed is false, with an
// empty from, when content carries no briefs table or has no row for num.
func rewriteStatusCell(content, num, newStatus string) (edit statusCellEdit, changed bool) {
	lines := strings.Split(content, "\n")
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
			return statusCellEdit{from: curTrim}, false // narrow transition only — every other state is immutable here
		}
		leading, trailing := cellPadding(cur)
		cells[statusIdx] = leading + newStatus + trailing
		before := line
		lines[i] = "|" + strings.Join(cells, "|") + "|"
		return statusCellEdit{
			content:   strings.Join(lines, "\n"),
			from:      curTrim,
			rowBefore: before,
			rowAfter:  lines[i],
		}, true
	}
	return statusCellEdit{}, false
}
