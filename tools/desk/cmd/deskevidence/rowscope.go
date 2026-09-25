package main

// rowscope.go — row-scoped landings for a target whose remote content carries the
// generated Briefs table.
//
// WHY. A whole-file Contents-API PUT has no notion of a table region: it commits
// whatever the caller handed it, byte for byte. When the caller's local copy of a
// shared stream README is even one landing stale, that PUT silently reverts every
// row the caller's copy had not yet seen — independent of how careful the landing
// that corrected those rows was, because the revert happens on THIS tool's write,
// not on theirs. A board row that had just been corrected was put back to `todo`
// forty-nine seconds later by exactly this shape.
//
// THE FIX. For a target whose REMOTE content carries the marker-wrapped generated
// table, the caller must name which row(s) it means (`--row <NN>`, repeatable).
// The committed content is then REBASED onto the remote: only the named rows'
// lines come from the local file; every other byte — every other row, the header,
// the separator, and all prose outside the markers — comes from the remote
// unchanged. A stale local copy therefore cannot revert anything it did not name.
//
// THE MARKERS. `<!-- statusgen:briefs:begin -->` / `<!-- statusgen:briefs:end -->`
// are statusgen's own convention (statusgen/readmetable.go, a SEPARATE Go module —
// deskevidence cannot import it, so the literals are re-declared here). They must
// stay byte-identical to statusgen's constants or the two tools would disagree
// about where the table starts and ends; TestRowScopeMarkersMatchStatusgen pins
// the literal against a copy of statusgen's own source comment so the two cannot
// drift silently.
//
// ROW IDENTITY. A row is keyed by the first cell of a table line between the two
// markers (the brief's own fact). The header row's first cell is "#" and the
// separator row's first cell starts with "---"; neither is a keyed data row and
// neither is ever a valid --row target.
//
// TWO LAYERS. rebaseNamedRows is the PRE-CHECK: it builds the rebased content
// against the fetch cmdEvidence already made. rowScopeWriteTimeCheck is the WRITE
// OP's own, independent re-enforcement: immediately before the commit it re-fetches
// the target — a SEPARATE read — and refuses if the content about to be written
// disagrees with that fresh read on any row it does not name. A race that changes
// the table between the pre-check and the write is caught by a component reading
// the table a second time, independent of the first read — the same shape the
// #1709 append-only shrink guard uses (pre-check here, re-enforced post-fetch in
// the write op).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	tableMarkerBegin = "<!-- statusgen:briefs:begin -->"
	tableMarkerEnd   = "<!-- statusgen:briefs:end -->"
)

// stringSliceFlag is a repeatable flag.Value — flag.Parse calls Set once per
// occurrence of --row, so `--row 04 --row 19` collects both.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// tableRowKey returns the first cell of a markdown table LINE (trimmed) and whether
// the line is a genuine keyed data row. The header row keys on "#" and the
// separator row's first cell starts with "---"; neither is a data row, so neither
// can ever be named by --row.
func tableRowKey(line string) (key string, isDataRow bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return "", false
	}
	rest := strings.TrimPrefix(t, "|")
	first := rest
	if idx := strings.Index(rest, "|"); idx >= 0 {
		first = rest[:idx]
	}
	key = strings.TrimSpace(first)
	if key == "" || key == "#" || strings.HasPrefix(key, "---") {
		return "", false
	}
	return key, true
}

// rowLines splits content into lines and, when content carries a complete,
// well-ordered marker-wrapped region, returns every keyed data row's line index
// within that region. ok is false when either marker is missing or out of order —
// the same "not a table target" signal hasTableMarkers reports.
func rowLines(content []byte) (lines []string, keyed map[string]int, ok bool) {
	lines = strings.Split(string(content), "\n")
	beginIdx, endIdx := -1, -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if beginIdx < 0 {
			if t == tableMarkerBegin {
				beginIdx = i
			}
			continue
		}
		if endIdx < 0 && t == tableMarkerEnd {
			endIdx = i
			break
		}
	}
	if beginIdx < 0 || endIdx < 0 {
		return lines, nil, false
	}
	keyed = map[string]int{}
	for i := beginIdx + 1; i < endIdx; i++ {
		if key, isRow := tableRowKey(lines[i]); isRow {
			keyed[key] = i
		}
	}
	return lines, keyed, true
}

// hasTableMarkers reports whether content carries a complete, well-ordered
// generated-table region — the signal that a direct-write target is in scope for
// row-scoped landings at all. A target with no such region behaves exactly as
// before this brief (Interface Contract: "Non-table targets behave exactly as
// today").
func hasTableMarkers(content []byte) bool {
	_, _, ok := rowLines(content)
	return ok
}

// rebaseNamedRows is the PRE-CHECK half of the Interface Contract (items 2-4): it
// takes remote's line structure as the base and replaces ONLY the named rows'
// lines with local's lines for those same keys. Every other byte — every other
// row, the header, the separator, and everything outside the markers — is
// remote's, unchanged, so a stale local copy cannot revert anything.
//
// staleForeign names every row present in BOTH tables, keyed by its first cell,
// but NOT among rows, whose local line differs from remote's: the landing still
// proceeds (rebased) but the caller is told, one entry per foreign row, so a
// verifier who meant a second row finds out (item 3) rather than being told
// nothing changed.
//
// A named row absent from remote's table is refused (item 4: the table changed
// under the caller, who must re-fetch). A named row absent from LOCAL'S table is
// refused the same way — there is no line to rebase in from.
func rebaseNamedRows(remote, local []byte, rows []string) (rebased []byte, staleForeign []string, err error) {
	remoteLines, remoteKeyed, ok := rowLines(remote)
	if !ok {
		return nil, nil, deskkit.Refused("refused: remote content does not carry a complete " +
			tableMarkerBegin + " / " + tableMarkerEnd + " region — cannot row-scope this landing")
	}
	localLines, localKeyed, _ := rowLines(local)

	named := map[string]bool{}
	for _, r := range rows {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		named[r] = true
		if _, ok := remoteKeyed[r]; !ok {
			return nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the remote table — the table changed under this landing; re-fetch and retry", r))
		}
		if _, ok := localKeyed[r]; !ok {
			return nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the local file being landed", r))
		}
	}

	out := append([]string(nil), remoteLines...)
	for r := range named {
		out[remoteKeyed[r]] = localLines[localKeyed[r]]
	}

	var stale []string
	for key, ridx := range remoteKeyed {
		if named[key] {
			continue
		}
		lidx, ok := localKeyed[key]
		if !ok {
			continue
		}
		if localLines[lidx] != remoteLines[ridx] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	return []byte(strings.Join(out, "\n")), stale, nil
}

// rowScopeWriteTimeCheck is the WRITE OP's own, independent re-enforcement
// (Interface Contract item 5): "only named rows differ from the content fetched
// at write time". It re-fetches the target fresh — a read wholly separate from
// the one the pre-check and rebase were built against — and refuses if any row
// the pending commit carries for a FOREIGN (non-named) key disagrees with that
// fresh fetch, in either direction (the row's text differs, or the row exists on
// only one side). Rows that already matched are unaffected; this never widens
// what a landing may write, it only ever adds a reason to refuse one.
func rowScopeWriteTimeCheck(fg deskkit.Forge, fr deskkit.ForgeRepo, targetRepoPath, branch string, rows []string, commitContent []byte) error {
	fresh, ferr := fg.ReadFile(fr, deskkit.ReadFileInput{File: targetRepoPath, Ref: branch})
	if ferr != nil {
		if deskkit.IsForgeNotFound(ferr) {
			return deskkit.Refused("refused: " + targetRepoPath +
				" no longer exists as of the write-time re-fetch — the table changed under this landing; re-fetch and retry")
		}
		return deskkit.Unverifiable("row-scope write-time re-check: cannot re-fetch "+targetRepoPath+"@"+branch, ferr)
	}
	freshLines, freshKeyed, ok := rowLines(fresh.Content)
	if !ok {
		return deskkit.Refused("refused: " + targetRepoPath +
			" no longer carries a complete generated-table region as of the write-time re-fetch — the table changed under this landing; re-fetch and retry")
	}
	commitLines, commitKeyed, _ := rowLines(commitContent)

	named := map[string]bool{}
	for _, r := range rows {
		named[strings.TrimSpace(r)] = true
	}

	mismatchedSet := map[string]bool{}
	for key, fidx := range freshKeyed {
		if named[key] {
			continue
		}
		cidx, ok := commitKeyed[key]
		if !ok || commitLines[cidx] != freshLines[fidx] {
			mismatchedSet[key] = true
		}
	}
	for key := range commitKeyed {
		if named[key] {
			continue
		}
		if _, ok := freshKeyed[key]; !ok {
			mismatchedSet[key] = true
		}
	}
	if len(mismatchedSet) > 0 {
		mismatched := make([]string, 0, len(mismatchedSet))
		for key := range mismatchedSet {
			mismatched = append(mismatched, key)
		}
		sort.Strings(mismatched)
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s changed under this landing — foreign row(s) %s differ from the content fetched at write time; re-fetch and retry",
			targetRepoPath, strings.Join(mismatched, ", ")))
	}
	return nil
}
