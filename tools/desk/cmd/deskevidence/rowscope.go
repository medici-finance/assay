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
// LIFECYCLE cells (the header's Status / Verified / Reviewed columns) come from the
// local file; every other byte — the named row's authoring cells, every other row,
// the header, the separator, and all prose outside the markers — comes from the
// remote unchanged. A stale local copy therefore cannot revert anything it did not
// name, nor the authoring cells of a row it did.
//
// THE MARKERS. `<!-- statusgen:briefs:begin -->` / `<!-- statusgen:briefs:end -->`
// are statusgen's own convention (statusgen/readmetable.go, a SEPARATE Go module —
// deskevidence cannot import it, so the literals are re-declared here).
// TestRowScopeMarkersMatchStatusgen reads statusgen/readmetable.go from the repo tree
// and fails if either literal drifts from statusgen's own constants. The region is
// located exactly as statusgen's extractRegion locates it — the FIRST occurrence of
// each literal anywhere in the content — and must then stand alone on its line. A
// README carrying either literal whose region does not parse that way is REFUSED,
// never silently treated as a non-table target (fail closed, not open).
//
// ROW IDENTITY. A row is keyed by the first cell of a table line between the two
// markers (the brief's own fact). The header row's first cell is "#" and the
// separator row's first cell starts with "---"; neither is a keyed data row and
// neither is ever a valid --row target. A key that appears more than once in a
// table is refused — rebasing onto an ambiguous key would be a guess.
//
// TWO LAYERS. rebaseNamedRows is the PRE-CHECK: it builds the rebased content
// against the fetch cmdEvidence already made. rowScopeWriteTimeCheck is the WRITE
// OP's own, independent re-enforcement: immediately before the commit it re-fetches
// the target — a SEPARATE read — and refuses unless the content about to be written
// is exactly that fresh read with only the named rows' lifecycle cells changed. It
// returns the fresh read's content id, which the write then carries as
// WriteFileInput.ExpectedSHA: the backend refuses if its own fetch sees a different
// id, and cites that id as the forge's conditional-write precondition, so a change
// landing after the re-check is refused by the forge rather than overwritten.

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	tableMarkerBegin = "<!-- statusgen:briefs:begin -->"
	tableMarkerEnd   = "<!-- statusgen:briefs:end -->"
)

// lifecycleColumns are the header names of the cells a row-scoped landing may change
// on a named row — the same three statusgen preserves across a regen.
var lifecycleColumns = []string{"Status", "Verified", "Reviewed"}

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

// cellSpans returns the [start,end) byte span of every cell of a table line, split
// on UNESCAPED pipes exactly as statusgen's splitRow splits (`\|` is cell content),
// with the zero-length leading/trailing cells a `| a | b |` line produces dropped.
func cellSpans(line string) [][2]int {
	var delims []int
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			i++
		case line[i] == '|':
			delims = append(delims, i)
		}
	}
	spans := make([][2]int, 0, len(delims)+1)
	prev := 0
	for _, d := range delims {
		spans = append(spans, [2]int{prev, d})
		prev = d + 1
	}
	spans = append(spans, [2]int{prev, len(line)})
	empty := func(s [2]int) bool { return strings.TrimSpace(line[s[0]:s[1]]) == "" }
	start, end := 0, len(spans)
	if start < end && empty(spans[start]) {
		start++
	}
	if end > start && empty(spans[end-1]) {
		end--
	}
	return spans[start:end]
}

// briefsTable is one parsed generated-table region.
type briefsTable struct {
	lines  []string
	keyed  map[string]int // data-row key -> line index
	dups   []string       // keys that appear more than once, sorted
	header []string       // header row cells, trimmed (nil when no header row)
}

// parseBriefsTable locates the generated-table region exactly as statusgen's
// extractRegion does — the FIRST occurrence of each marker literal anywhere in the
// content, the end after the begin — and additionally requires each marker to stand
// alone on its (trimmed) line. found reports whether either literal appears at all;
// ok whether a complete, line-anchored, well-ordered region was parsed.
func parseBriefsTable(content []byte) (t briefsTable, found, ok bool) {
	s := string(content)
	t.lines = strings.Split(s, "\n")
	bi := strings.Index(s, tableMarkerBegin)
	ei := strings.Index(s, tableMarkerEnd)
	found = bi >= 0 || ei >= 0
	if bi < 0 || ei < 0 || ei < bi+len(tableMarkerBegin) {
		return t, found, false
	}
	bl, el := strings.Count(s[:bi], "\n"), strings.Count(s[:ei], "\n")
	if bl == el || strings.TrimSpace(t.lines[bl]) != tableMarkerBegin || strings.TrimSpace(t.lines[el]) != tableMarkerEnd {
		return t, found, false
	}
	t.keyed = map[string]int{}
	seen := map[string]int{}
	for i := bl + 1; i < el; i++ {
		ln := t.lines[i]
		if t.header == nil && strings.HasPrefix(strings.TrimSpace(ln), "|") {
			if first := cellSpans(ln); len(first) > 0 && strings.TrimSpace(ln[first[0][0]:first[0][1]]) == "#" {
				for _, sp := range first {
					t.header = append(t.header, strings.TrimSpace(ln[sp[0]:sp[1]]))
				}
				continue
			}
		}
		if key, isRow := tableRowKey(ln); isRow {
			seen[key]++
			t.keyed[key] = i
		}
	}
	for key, n := range seen {
		if n > 1 {
			t.dups = append(t.dups, key)
		}
	}
	sort.Strings(t.dups)
	return t, found, true
}

// hasTableMarkers reports whether content carries a complete, line-anchored
// generated-table region — the signal that a direct-write target is in scope for
// row-scoped landings at all.
func hasTableMarkers(content []byte) bool {
	_, _, ok := parseBriefsTable(content)
	return ok
}

// rowScopeMalformed reports the fail-closed case: a stream README whose content
// carries either marker literal, yet no region statusgen and this tool would agree on
// parses out of it. Treating it as a non-table target would silently disarm the
// guard; the caller refuses instead. Only README.md files are statusgen table targets —
// a brief or doc that merely QUOTES the literals is not one.
func rowScopeMalformed(targetRepoPath string, content []byte) bool {
	if path.Base(targetRepoPath) != "README.md" {
		return false
	}
	_, found, ok := parseBriefsTable(content)
	return found && !ok
}

// lifecycleIndexes returns the header positions of the lifecycle columns.
func lifecycleIndexes(header []string) ([]int, bool) {
	idx := make([]int, 0, len(lifecycleColumns))
	for _, want := range lifecycleColumns {
		at := -1
		for i, h := range header {
			if h == want {
				at = i
				break
			}
		}
		if at < 0 {
			return nil, false
		}
		idx = append(idx, at)
	}
	return idx, true
}

// rebaseNamedRows is the PRE-CHECK half of the Interface Contract (items 2-4): it
// takes remote's line structure as the base and, on each named row, replaces ONLY
// the lifecycle cells with local's cells for that same key. Every other byte — the
// named row's authoring cells, every other row, the header, the separator, and
// everything outside the markers — is remote's, unchanged.
//
// staleForeign names every row present in BOTH tables but NOT named, whose local line
// differs from remote's; staleAuthoring names every named row whose local AUTHORING
// cells differ from remote's. Neither blocks the landing — nothing of either is
// written — but the caller is told (item 3).
//
// Refused: remote has no complete region; either table has a duplicated key; a named
// row is absent from either table (item 4); the header lacks a lifecycle column; a
// named row's line on either side has a different cell count from the header; a
// named local line carries a carriage return anywhere but its very end.
func rebaseNamedRows(remote, local []byte, rows []string) (rebased []byte, staleForeign, staleAuthoring []string, err error) {
	rt, _, ok := parseBriefsTable(remote)
	if !ok {
		return nil, nil, nil, deskkit.Refused("refused: remote content does not carry a complete " +
			tableMarkerBegin + " / " + tableMarkerEnd + " region — cannot row-scope this landing")
	}
	lt, _, _ := parseBriefsTable(local)
	if len(rt.dups) > 0 {
		return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
			"refused: the remote table carries row key(s) %s more than once — which row a --row names is ambiguous; fix the table first",
			strings.Join(rt.dups, ", ")))
	}
	if len(lt.dups) > 0 {
		return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
			"refused: the local file's table carries row key(s) %s more than once — which line to land is ambiguous",
			strings.Join(lt.dups, ", ")))
	}
	lifeIdx, ok := lifecycleIndexes(rt.header)
	if !ok {
		return nil, nil, nil, deskkit.Refused("refused: the remote table's header does not name the " +
			strings.Join(lifecycleColumns, " / ") + " columns — cannot row-scope this landing")
	}

	named := map[string]bool{}
	var order []string
	for _, r := range rows {
		r = strings.TrimSpace(r)
		if r == "" || named[r] {
			continue
		}
		named[r] = true
		order = append(order, r)
		if _, ok := rt.keyed[r]; !ok {
			return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the remote table — the table changed under this landing; re-fetch and retry", r))
		}
		if _, ok := lt.keyed[r]; !ok {
			return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the local file being landed", r))
		}
	}
	sort.Strings(order)

	out := append([]string(nil), rt.lines...)
	var authoring []string
	for _, r := range order {
		rline, lline := rt.lines[rt.keyed[r]], lt.lines[lt.keyed[r]]
		if i := strings.IndexByte(lline, '\r'); i >= 0 && i != len(lline)-1 {
			return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s: the local line carries a carriage return mid-line — it would render as more than one row", r))
		}
		rs, ls := cellSpans(rline), cellSpans(lline)
		if len(rs) != len(rt.header) {
			return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s: the remote row has %d cells, the header %d — cannot locate its lifecycle cells", r, len(rs), len(rt.header)))
		}
		if len(ls) != len(rt.header) {
			return nil, nil, nil, deskkit.Refused(fmt.Sprintf(
				"refused: --row %s: the local row has %d cells, the header %d — a malformed row would lose its lifecycle cells on the next regen", r, len(ls), len(rt.header)))
		}
		isLife := map[int]bool{}
		for _, i := range lifeIdx {
			isLife[i] = true
		}
		for i := range rs {
			if !isLife[i] && strings.TrimSpace(rline[rs[i][0]:rs[i][1]]) != strings.TrimSpace(lline[ls[i][0]:ls[i][1]]) {
				authoring = append(authoring, r)
				break
			}
		}
		// Splice right-to-left so earlier spans' offsets stay valid.
		sorted := append([]int(nil), lifeIdx...)
		sort.Sort(sort.Reverse(sort.IntSlice(sorted)))
		nl := rline
		for _, i := range sorted {
			nl = nl[:rs[i][0]] + lline[ls[i][0]:ls[i][1]] + nl[rs[i][1]:]
		}
		out[rt.keyed[r]] = nl
	}

	var stale []string
	for key, ridx := range rt.keyed {
		if named[key] {
			continue
		}
		if lidx, ok := lt.keyed[key]; ok && lt.lines[lidx] != rt.lines[ridx] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	return []byte(strings.Join(out, "\n")), stale, authoring, nil
}

// foreignRowDiff names every non-named key whose line differs between a and b, or that
// exists on only one side — the diagnostic a write-time refusal reports.
func foreignRowDiff(a, b briefsTable, named map[string]bool) []string {
	set := map[string]bool{}
	for key, ai := range a.keyed {
		if named[key] {
			continue
		}
		if bi, ok := b.keyed[key]; !ok || b.lines[bi] != a.lines[ai] {
			set[key] = true
		}
	}
	for key := range b.keyed {
		if named[key] {
			continue
		}
		if _, ok := a.keyed[key]; !ok {
			set[key] = true
		}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// rowScopeWriteTimeCheck is the WRITE OP's own, independent re-enforcement
// (Interface Contract item 5): "only named rows differ from the content fetched at
// write time". It re-fetches the target fresh — a read wholly separate from the one
// the pre-check was built against — re-runs the rebase of the pending commit onto
// that fresh read, and refuses unless the result is byte-identical to the commit:
// any foreign row, header, separator, prose, or named-row authoring cell that moved
// in the window is a refusal. It returns the fresh read's content id for the write's
// ExpectedSHA precondition, which carries the same guarantee through to the forge's
// own conditional write. It never widens what a landing may write.
func rowScopeWriteTimeCheck(fg deskkit.Forge, fr deskkit.ForgeRepo, targetRepoPath, branch string, rows []string, commitContent []byte) (freshSHA string, err error) {
	fresh, ferr := fg.ReadFile(fr, deskkit.ReadFileInput{File: targetRepoPath, Ref: branch})
	if ferr != nil {
		if deskkit.IsForgeNotFound(ferr) {
			return "", deskkit.Refused("refused: " + targetRepoPath +
				" no longer exists as of the write-time re-fetch — the table changed under this landing; re-fetch and retry")
		}
		return "", deskkit.Unverifiable("row-scope write-time re-check: cannot re-fetch "+targetRepoPath+"@"+branch, ferr)
	}
	ft, _, ok := parseBriefsTable(fresh.Content)
	if !ok {
		return "", deskkit.Refused("refused: " + targetRepoPath +
			" no longer carries a complete generated-table region as of the write-time re-fetch — the table changed under this landing; re-fetch and retry")
	}
	expected, _, _, rerr := rebaseNamedRows(fresh.Content, commitContent, rows)
	if rerr != nil {
		return "", rerr
	}
	if !bytes.Equal(expected, commitContent) {
		named := map[string]bool{}
		for _, r := range rows {
			named[strings.TrimSpace(r)] = true
		}
		ct, _, _ := parseBriefsTable(commitContent)
		if moved := foreignRowDiff(ft, ct, named); len(moved) > 0 {
			return "", deskkit.Refused(fmt.Sprintf(
				"refused: %s changed under this landing — foreign row(s) %s differ from the content fetched at write time; re-fetch and retry",
				targetRepoPath, strings.Join(moved, ", ")))
		}
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: %s changed under this landing — content outside the named row(s)' lifecycle cells differs from the content fetched at write time; re-fetch and retry",
			targetRepoPath))
	}
	return fresh.SHA, nil
}
