package deskkit

// briefstable.go: the generated Briefs table of a stream README, shared by the
// row-scoped landing (deskevidence) and verifier admission at Evidence time.
// Both bind the same property: only the named rows' lifecycle cells change.
// The markers are statusgen's own convention (statusgen/readmetable.go, a
// separate module); deskevidence's TestRowScopeMarkersMatchStatusgen pins them.

import (
	"fmt"
	"sort"
	"strings"
)

const (
	TableMarkerBegin = "<!-- statusgen:briefs:begin -->"
	TableMarkerEnd   = "<!-- statusgen:briefs:end -->"
)

// lifecycleColumns are the header names of the cells a row-scoped landing may change
// on a named row — the same three statusgen preserves across a regen.
var lifecycleColumns = []string{"Status", "Verified", "Reviewed"}

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

// BriefsTable is one parsed generated-table region.
type BriefsTable struct {
	Lines  []string
	Keyed  map[string]int // data-row key -> line index
	Dups   []string       // keys that appear more than once, sorted
	Header []string       // header row cells, trimmed (nil when no header row)
}

// ParseBriefsTable locates the generated-table region exactly as statusgen's
// extractRegion does — the FIRST occurrence of each marker literal anywhere in the
// content, the end after the begin — and additionally requires each marker to stand
// alone on its (trimmed) line. found reports whether either literal appears at all;
// ok whether a complete, line-anchored, well-ordered region was parsed.
func ParseBriefsTable(content []byte) (t BriefsTable, found, ok bool) {
	s := string(content)
	t.Lines = strings.Split(s, "\n")
	bi := strings.Index(s, TableMarkerBegin)
	ei := strings.Index(s, TableMarkerEnd)
	found = bi >= 0 || ei >= 0
	if bi < 0 || ei < 0 || ei < bi+len(TableMarkerBegin) {
		return t, found, false
	}
	bl, el := strings.Count(s[:bi], "\n"), strings.Count(s[:ei], "\n")
	if bl == el || strings.TrimSpace(t.Lines[bl]) != TableMarkerBegin || strings.TrimSpace(t.Lines[el]) != TableMarkerEnd {
		return t, found, false
	}
	t.Keyed = map[string]int{}
	seen := map[string]int{}
	for i := bl + 1; i < el; i++ {
		ln := t.Lines[i]
		if t.Header == nil && strings.HasPrefix(strings.TrimSpace(ln), "|") {
			if first := cellSpans(ln); len(first) > 0 && strings.TrimSpace(ln[first[0][0]:first[0][1]]) == "#" {
				for _, sp := range first {
					t.Header = append(t.Header, strings.TrimSpace(ln[sp[0]:sp[1]]))
				}
				continue
			}
		}
		if key, isRow := tableRowKey(ln); isRow {
			seen[key]++
			t.Keyed[key] = i
		}
	}
	for key, n := range seen {
		if n > 1 {
			t.Dups = append(t.Dups, key)
		}
	}
	sort.Strings(t.Dups)
	return t, found, true
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

// RebaseNamedRows is the PRE-CHECK half of the Interface Contract (items 2-4): it
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
func RebaseNamedRows(remote, local []byte, rows []string) (rebased []byte, staleForeign, staleAuthoring []string, err error) {
	rt, _, ok := ParseBriefsTable(remote)
	if !ok {
		return nil, nil, nil, Refused("refused: remote content does not carry a complete " +
			TableMarkerBegin + " / " + TableMarkerEnd + " region — cannot row-scope this landing")
	}
	lt, _, _ := ParseBriefsTable(local)
	if len(rt.Dups) > 0 {
		return nil, nil, nil, Refused(fmt.Sprintf(
			"refused: the remote table carries row key(s) %s more than once — which row a --row names is ambiguous; fix the table first",
			strings.Join(rt.Dups, ", ")))
	}
	if len(lt.Dups) > 0 {
		return nil, nil, nil, Refused(fmt.Sprintf(
			"refused: the local file's table carries row key(s) %s more than once — which line to land is ambiguous",
			strings.Join(lt.Dups, ", ")))
	}
	lifeIdx, ok := lifecycleIndexes(rt.Header)
	if !ok {
		return nil, nil, nil, Refused("refused: the remote table's header does not name the " +
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
		if _, ok := rt.Keyed[r]; !ok {
			return nil, nil, nil, Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the remote table — the table changed under this landing; re-fetch and retry", r))
		}
		if _, ok := lt.Keyed[r]; !ok {
			return nil, nil, nil, Refused(fmt.Sprintf(
				"refused: --row %s names a row absent from the local file being landed", r))
		}
	}
	sort.Strings(order)

	out := append([]string(nil), rt.Lines...)
	var authoring []string
	for _, r := range order {
		rline, lline := rt.Lines[rt.Keyed[r]], lt.Lines[lt.Keyed[r]]
		if i := strings.IndexByte(lline, '\r'); i >= 0 && i != len(lline)-1 {
			return nil, nil, nil, Refused(fmt.Sprintf(
				"refused: --row %s: the local line carries a carriage return mid-line — it would render as more than one row", r))
		}
		rs, ls := cellSpans(rline), cellSpans(lline)
		if len(rs) != len(rt.Header) {
			return nil, nil, nil, Refused(fmt.Sprintf(
				"refused: --row %s: the remote row has %d cells, the header %d — cannot locate its lifecycle cells", r, len(rs), len(rt.Header)))
		}
		if len(ls) != len(rt.Header) {
			return nil, nil, nil, Refused(fmt.Sprintf(
				"refused: --row %s: the local row has %d cells, the header %d — a malformed row would lose its lifecycle cells on the next regen", r, len(ls), len(rt.Header)))
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
		out[rt.Keyed[r]] = nl
	}

	var stale []string
	for key, ridx := range rt.Keyed {
		if named[key] {
			continue
		}
		if lidx, ok := lt.Keyed[key]; ok && lt.Lines[lidx] != rt.Lines[ridx] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	return []byte(strings.Join(out, "\n")), stale, authoring, nil
}
