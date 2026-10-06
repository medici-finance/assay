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
// deskevidence cannot import it, so the literals are re-declared in deskkit's
// briefstable.go, which verifier admission shares).
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

// The table core lives in deskkit (briefstable.go), shared with verifier
// admission; these names keep this file's own vocabulary.
const (
	tableMarkerBegin = deskkit.TableMarkerBegin
	tableMarkerEnd   = deskkit.TableMarkerEnd
)

type briefsTable = deskkit.BriefsTable

func parseBriefsTable(content []byte) (deskkit.BriefsTable, bool, bool) {
	return deskkit.ParseBriefsTable(content)
}
func rebaseNamedRows(remote, local []byte, rows []string) ([]byte, []string, []string, error) {
	return deskkit.RebaseNamedRows(remote, local, rows)
}

// stringSliceFlag is a repeatable flag.Value — flag.Parse calls Set once per
// occurrence of --row, so `--row 04 --row 19` collects both.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
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

// foreignRowDiff names every non-named key whose line differs between a and b, or that
// exists on only one side — the diagnostic a write-time refusal reports.
func foreignRowDiff(a, b briefsTable, named map[string]bool) []string {
	set := map[string]bool{}
	for key, ai := range a.Keyed {
		if named[key] {
			continue
		}
		if bi, ok := b.Keyed[key]; !ok || b.Lines[bi] != a.Lines[ai] {
			set[key] = true
		}
	}
	for key := range b.Keyed {
		if named[key] {
			continue
		}
		if _, ok := a.Keyed[key]; !ok {
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
