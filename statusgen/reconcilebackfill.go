package main

// reconcilebackfill.go — the `statusgen reconcile --backfill --report` verb
// (derived-board/07).
//
// The normal reconcile derivation (lifecycle.go + ghfetch.go's ListPRs) reads
// ONLY the `Brief:` trailer — no title parsing, no branch-name guessing — and
// that is correct for live derivation. But historical PRs, merged before the
// trailer existed, carry no trailer at all, so the plain derivation reads every
// one of them as `todo`: a silent demotion of work that already happened.
//
// --backfill adds a DECLARED, reviewable, HISTORY-ONLY fallback (spec §7): for a
// brief the normal fold could not witness, look for a merged PR whose branch
// name or body names the brief in `<stream>/<NN>` or `<stream>-<NN>` form. A
// match becomes the witness, tagged so a reader can tell it apart from a real
// trailer link. No match, but the brief's LAST hand-edited stream README (before
// the table became generated) already said `implemented`/`verified`/`done`: the
// cell renders `unknown` with a reason naming the hand-asserted state and the
// commit that asserted it — never silently demoted to `todo` (facts, brief-07).
//
// --report additionally writes docs/streams/board-drift-<date>.md: one row per
// brief where the hand-said cell disagrees with what this run derives, so a
// human reads the drift before the generated table replaces the hand-edited one
// permanently. It never resolves a row itself — linking the PR (adding the
// trailer) or accepting the demotion is a human act in the drift PR.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// generatedTableMarker is the marker the derived-board/04 generated Briefs
// table wraps its region in. Its presence in a historical README snapshot means
// generation had already taken over by that commit — everything at or after it
// is machine-derived, never hand-said.
const generatedTableMarker = "<!-- statusgen:briefs:begin -->"

// backfillIDForms returns the two textual forms (spec §7 / brief-07 facts) a
// historical PR is allowed to carry in its branch name or body for the declared
// backfill fallback to count it as a witness: "<stream>/<NN>" and
// "<stream>-<NN>". No other shape is recognized — the fallback is a literal,
// reviewable substring match, never a fuzzy guess.
func backfillIDForms(stream, num string) []string {
	return []string{stream + "/" + num, stream + "-" + num}
}

// containsBackfillForm reports whether text contains one of the declared
// backfill forms for stream/num. Empty text never matches.
func containsBackfillForm(text, stream, num string) bool {
	if text == "" {
		return false
	}
	for _, form := range backfillIDForms(stream, num) {
		if strings.Contains(text, form) {
			return true
		}
	}
	return false
}

// briefStreamNum extracts the (stream, num) pair a backfill match is keyed on,
// from either a brief-v2 hierarchical id (<cell>:<repo>:<stream>:<NN>) or a
// legacy "<stream>/<NN>" id.
func briefStreamNum(id string) (stream, num string, ok bool) {
	if _, _, stream, num, ok := parseBriefV2ID(id); ok {
		return stream, num, true
	}
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

// matchBackfillPR returns the highest-numbered MERGED pull among pulls whose
// branch name or body names stream/num in a declared backfill form. Only merged
// pulls count — the fallback exists to recover history that already happened,
// never to promise an in-flight PR. ok is false when nothing matches.
func matchBackfillPR(pulls []ghPull, stream, num string) (match ghPull, ok bool) {
	for _, p := range pulls {
		if p.MergedAt == "" {
			continue
		}
		if containsBackfillForm(p.Head.Ref, stream, num) || containsBackfillForm(p.Body, stream, num) {
			if !ok || p.Number > match.Number {
				match, ok = p, true
			}
		}
	}
	return match, ok
}

// handSaidLookup resolves a brief's last hand-asserted lifecycle state before
// its stream README became a generated surface. found is false when the stream
// has no such history (a brief authored straight into the generated era) or the
// brief did not appear in any pre-generation snapshot.
type handSaidLookup func(stream, num string) (state, sha string, found bool)

// handSaidBeforeGeneration walks rel's (docs/streams/<stream>/README.md) git
// history OLDEST-FIRST, tracking the brief's Status cell at each snapshot, and
// stops at the first snapshot that already carries the generated-table marker
// — the migration commit. The tracked (state, sha) at that point is the LAST
// hand-said value, the fallback witness the drift report compares against.
// found is false when the README has no history, no commit ever lacked the
// marker (born generated), or the brief never appeared in a pre-generation row.
func handSaidBeforeGeneration(root, stream, num string) (state, sha string, found bool) {
	rel := filepath.ToSlash(filepath.Join("docs", "streams", stream, "README.md"))
	commits, err := streamReadmeCommits(root, rel)
	if err != nil || len(commits) == 0 {
		return "", "", false
	}
	for _, c := range commits {
		content, ok := readmeAtCommit(root, c.SHA, rel)
		if !ok {
			continue
		}
		if strings.Contains(content, generatedTableMarker) {
			break // generation had taken over at/after this commit — stop here
		}
		_, briefs, ok := parseStreamSnapshot(content, stream)
		if !ok {
			continue
		}
		for _, b := range briefs {
			if b.Num == num {
				state, sha, found = b.Status, c.SHA, true
			}
		}
	}
	return state, sha, found
}

// advancedHandState reports whether state is one the derivation must never
// silently demote out from under (facts, brief-07): implemented, verified, or
// done.
func advancedHandState(state string) bool {
	switch state {
	case "implemented", "verified", "done":
		return true
	default:
		return false
	}
}

// applyReconcileBackfill mutates a copy of cells with the declared backfill
// fallback: only rows the normal fold left at `todo` (no trailer witness) are
// touched. pullsLookedAt false (the raw pull fetch itself failed) leaves those
// rows exactly as the normal fold reported them — a fetch failure is already an
// honest `unknown`/`todo` upstream, backfill adds no guess on top of a failure.
func applyReconcileBackfill(cells []BriefCell, pulls []ghPull, pullsLookedAt bool, lookup handSaidLookup) []BriefCell {
	out := make([]BriefCell, len(cells))
	copy(out, cells)
	for i := range out {
		if out[i].Cell != "todo" || out[i].Source != "pr" {
			continue // already witnessed (trailer PR) or not a PR-derived cell — nothing to backfill
		}
		stream, num, ok := briefStreamNum(out[i].ID)
		if !ok {
			continue
		}
		if pullsLookedAt {
			if pr, matched := matchBackfillPR(pulls, stream, num); matched {
				out[i].Cell = "implemented"
				out[i].Source = "backfill"
				out[i].Reason = ""
				out[i].Witness = fmt.Sprintf("PR #%d (merged %s) — backfill: branch/body match, no trailer", pr.Number, shortSHA(pr.MergeCommitSHA))
				continue
			}
		}
		if lookup == nil {
			continue
		}
		state, sha, found := lookup(stream, num)
		if !found || !advancedHandState(state) {
			continue // hand-said also todo/in-progress, or no history at all — todo stands
		}
		out[i].Cell = "unknown"
		out[i].Source = "backfill"
		out[i].Reason = fmt.Sprintf("no witness — hand-asserted %s at %s", state, shortSHA(sha))
		out[i].Witness = fmt.Sprintf("no PR carries a trailer; last hand edit %s", shortSHA(sha))
	}
	return out
}

// driftRow is one row of the board-drift report: a brief where the hand-said
// lifecycle cell disagrees with what this reconcile run derives.
type driftRow struct {
	ID       string
	HandSaid string
	Derived  string
	Witness  string
}

// buildDriftRows compares every cell's final derived state (post-backfill,
// when --backfill ran) against its hand-said history and returns one row per
// disagreement, sorted by brief id. A brief with no pre-generation history
// (found == false) is never a drift row — there is nothing to compare against.
func buildDriftRows(cells []BriefCell, lookup handSaidLookup) []driftRow {
	if lookup == nil {
		return nil
	}
	var rows []driftRow
	for _, c := range cells {
		stream, num, ok := briefStreamNum(c.ID)
		if !ok {
			continue
		}
		state, sha, found := lookup(stream, num)
		if !found || state == c.Cell {
			continue
		}
		derived := c.Cell
		if c.Cell == "unknown" && c.Source == "backfill" {
			derived = fmt.Sprintf("unknown (%s)", c.Reason)
		}
		witness := c.Witness
		if witness == "" {
			witness = fmt.Sprintf("no PR carries a trailer; last hand edit %s", shortSHA(sha))
		}
		rows = append(rows, driftRow{ID: c.ID, HandSaid: state, Derived: derived, Witness: witness})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

// escapeDriftCell keeps a witness string from breaking the markdown table it is
// rendered into.
func escapeDriftCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// renderDriftReport is the pure render of the board-drift report body, given
// the repo it was run against and its rows. Pure so the shape is unit-testable
// with no filesystem.
func renderDriftReport(repo string, generated time.Time, rows []driftRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Board drift report — %s\n\n", repo)
	fmt.Fprintf(&b, "Generated %s by `statusgen reconcile --backfill --report`. Every row below is a\n", generated.UTC().Format(time.RFC3339))
	b.WriteString("brief whose last hand-edited stream README (before its table became generated)\n")
	b.WriteString("disagrees with what this run derives from PR history. A row is resolved by\n")
	b.WriteString("linking the PR — add the `Brief:` trailer to its (already-merged) body, which\n")
	b.WriteString("the next reconcile reads — or by accepting the demotion; never by hand-editing\n")
	b.WriteString("the generated table.\n\n")
	b.WriteString("| Brief | Hand-said | Derived | Witness |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.ID, r.HandSaid, escapeDriftCell(r.Derived), escapeDriftCell(r.Witness))
	}
	if len(rows) == 0 {
		b.WriteString("| _none_ | — | — | every hand-said cell with pre-generation history agrees with the derived cell |\n")
	}
	return b.String()
}

// driftReportPath is the repo-relative path (spec §7) a board-drift report is
// written to: one per calendar day, so a re-run the same day overwrites rather
// than accumulating duplicates.
func driftReportPath(root string, generated time.Time) string {
	return filepath.Join(root, "docs", "streams", fmt.Sprintf("board-drift-%s.md", generated.UTC().Format("2006-01-02")))
}

// writeDriftReport renders and writes the board-drift report, returning the
// path written.
func writeDriftReport(root, repo string, rows []driftRow, generated time.Time) (path string, err error) {
	path = driftReportPath(root, generated)
	if err := os.WriteFile(path, []byte(renderDriftReport(repo, generated, rows)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
