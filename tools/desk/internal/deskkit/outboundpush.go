package deskkit

// outboundpush.go — the push path of the outbound-write check.
//
// Text that leaves by `git push` never crosses a Forge, so the decorator cannot see it. The
// two places a push is already inspected — deskpr's scanWrite, before deskpr pushes, and
// the deskpushguard pre-push hook, for every push — call OutboundCheckPush, which runs the
// SAME OutboundCheck over:
//
//   - kind `ref`    the branch name being pushed;
//   - kind `commit` the FULL message of every commit in base..head (subject, body,
//     trailers) — a commit message on a public repository is as published as a PR body;
//   - kind `file`   the ADDED lines of the base...head diff, per file, placed at their real
//     line numbers in the new file so a refusal's :line points at the line to fix. A
//     removed line cannot introduce a disclosure, and scanning removals would refuse the
//     very branch that deletes one. The path of a file the range ADDS is checked too.
//
// deskpr's existing whole-diff credential arms keep their breadth (removed and context lines
// included); this pass is additional, never a replacement.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// EnvPushGuardScanOverride carries an audited override reason into the deskpushguard
// pre-push hook. deskpr sets it on its own push child to exactly the reason its
// --force-scan-override took (and CLEARS it when none was given, so an ambient value can
// never override a push deskpr refused nothing on): the hook re-runs the same check, and
// without it would refuse the override deskpr already recorded. A hand-run push may set it
// too; either way the check validates the reason and writes the audit row itself, and a
// withheld-identifier or ruling-claim finding stays refused.
const EnvPushGuardScanOverride = "DESKPUSHGUARD_SCAN_OVERRIDE"

// OutboundPush names one push to check.
type OutboundPush struct {
	// Dir is any directory inside the checkout.
	Dir string
	// Repo is the push target, owner/name ("" when it cannot be derived: the public layers
	// then run, as for any target whose visibility is not stated).
	Repo string
	// Base is the revision the pushed range is measured from (the remote's main).
	Base string
	// Head is the revision being pushed.
	Head string
	// Branch is the destination branch name ("" skips the ref check).
	Branch string
	// Role is the custody the push goes out under.
	Role string
}

// OutboundCheckPush runs OutboundCheck over the ref, every commit message in the pushed range
// and the range's added lines. A failure to READ any of them is Unverifiable (exit 6), never
// a pass: could-not-check is not clean.
func OutboundCheckPush(p OutboundPush) error {
	if p.Branch != "" {
		if err := OutboundCheck(OutboundWrite{Role: p.Role, Repo: p.Repo, Kind: OutboundKindRef,
			Fields: []OutboundField{{Name: "branch", Text: p.Branch}}}); err != nil {
			return err
		}
	}
	top, err := gitcore.Toplevel(p.Dir)
	if err != nil {
		return Unverifiable("outbound check: cannot locate the checkout to read the pushed range", err)
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return Unverifiable("outbound check: cannot open the checkout to read the pushed range", err)
	}
	hashes, err := pushRangeHashes(repo, p.Base, p.Head)
	if err != nil {
		return Unverifiable(fmt.Sprintf("outbound check: cannot list the commits in %s..%s", p.Base, p.Head), err)
	}
	for _, h := range hashes {
		msg, merr := repo.CommitMessage(h)
		if merr != nil {
			return Unverifiable("outbound check: cannot read commit "+shortHash(h)+"'s message", merr)
		}
		if err := OutboundCheck(OutboundWrite{Role: p.Role, Repo: p.Repo, Kind: OutboundKindCommit,
			Fields: []OutboundField{{Name: "commit " + shortHash(h), Text: msg}}}); err != nil {
			return err
		}
	}
	if len(hashes) == 0 {
		return nil // nothing new: the diff is empty by definition
	}
	diff, err := repo.DiffSymmetric(p.Base, p.Head, 0)
	if err != nil {
		return Unverifiable(fmt.Sprintf("outbound check: cannot diff %s...%s", p.Base, p.Head), err)
	}
	var fields []OutboundField
	sources := map[string]string{}
	for _, fa := range AddedLinesByFile(diff) {
		if fa.New {
			fields = append(fields, OutboundField{Name: "path", Text: fa.Path})
		}
		fields = append(fields, OutboundField{Name: fa.Path, Text: fa.Text})
		// A brief file's full head-side content is the evidence the session-id arm's one
		// exemption needs (#2022, briefIDExemptLine): the added lines alone cannot show a line
		// sits inside the frontmatter. Read for brief paths only. A read that fails leaves
		// no entry, which means no exemption — the refusal stands, so could-not-read never
		// narrows the scan.
		if isBriefPath(fa.Path) {
			if src, rerr := repo.FileAt(p.Head, fa.Path); rerr == nil {
				sources[fa.Path] = src
			}
		}
	}
	return OutboundCheck(OutboundWrite{Role: p.Role, Repo: p.Repo, Kind: OutboundKindFile, Fields: fields,
		FileSources: sources})
}

// pushRangeHashes is `git rev-list base..head`: every commit reachable from head and not
// from base.
func pushRangeHashes(repo *gitcore.Repo, base, head string) ([]string, error) {
	baseAnc, err := repo.Log(base)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(baseAnc))
	for _, h := range baseAnc {
		seen[h] = true
	}
	headAnc, err := repo.Log(head)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, h := range headAnc {
		if !seen[h] {
			out = append(out, h)
		}
	}
	return out, nil
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// FileAdditions is one file's added lines from a unified diff.
type FileAdditions struct {
	// Path is the new-side path.
	Path string
	// New is true when the diff adds the file (or renames it to Path).
	New bool
	// Text holds the added lines at their new-file line numbers; every other line is blank.
	Text string
}

// maxPlacedLine bounds how far down a file added lines are placed at their real line
// numbers; an addition beyond it is appended after the placed ones (still checked, with an
// approximate :line) rather than allocating an arbitrarily long blank prefix.
const maxPlacedLine = 200000

// AddedLinesByFile parses a unified diff (any context width) into each file's ADDED lines.
// A removed or context line never appears in the result.
func AddedLinesByFile(diff string) []FileAdditions {
	var out []FileAdditions
	var cur *FileAdditions
	var lines []string
	var overflow []string
	inHunk := false
	newLine := 0
	flush := func() {
		if cur != nil {
			cur.Text = strings.Join(append(lines, overflow...), "\n")
			if strings.TrimSpace(cur.Text) != "" || cur.New {
				out = append(out, *cur)
			}
		}
		cur, lines, overflow, inHunk = nil, nil, nil, false
	}
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			flush()
			cur = &FileAdditions{}
		case cur == nil:
			continue
		case !inHunk && strings.HasPrefix(ln, "+++ "):
			p := strings.TrimPrefix(ln, "+++ ")
			if p == "/dev/null" {
				cur.Path = ""
			} else {
				cur.Path = strings.TrimPrefix(p, "b/")
			}
		case !inHunk && (strings.HasPrefix(ln, "new file mode") || strings.HasPrefix(ln, "rename to ")):
			cur.New = true
		case strings.HasPrefix(ln, "@@"):
			inHunk = true
			newLine = hunkNewStart(ln)
		case inHunk && strings.HasPrefix(ln, "+"):
			if cur.Path == "" {
				continue
			}
			if newLine >= 1 && newLine <= maxPlacedLine {
				for len(lines) < newLine-1 {
					lines = append(lines, "")
				}
				if len(lines) == newLine-1 {
					lines = append(lines, ln[1:])
				} else {
					overflow = append(overflow, ln[1:])
				}
			} else {
				overflow = append(overflow, ln[1:])
			}
			newLine++
		case inHunk && strings.HasPrefix(ln, " "):
			newLine++
		}
	}
	flush()
	// A deleted file (new side /dev/null) has no path and nothing added.
	kept := out[:0]
	for _, f := range out {
		if f.Path != "" {
			kept = append(kept, f)
		}
	}
	return kept
}

// hunkNewStart reads c from "@@ -a,b +c,d @@".
func hunkNewStart(h string) int {
	i := strings.Index(h, " +")
	if i < 0 {
		return 0
	}
	rest := h[i+2:]
	end := strings.IndexAny(rest, ", ")
	if end < 0 {
		end = len(rest)
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0
	}
	return n
}
