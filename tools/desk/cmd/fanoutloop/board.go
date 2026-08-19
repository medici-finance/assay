package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// BoardRow is one Next-up row resolved to its brief + frontmatter. It is the typed unit the
// board read produces; SelectQueue maps each to a loopengine.Item.
//
// The row's ORDER is authoritative and is NOT re-scored here: statusgen already applied
// priority, staleness exclusion, the 4-per-stream cap and the dep-incomplete exclusion when it
// wrote the board (brief facts — "the adapter consumes, never re-implements"). This adapter
// reads the rows in board order and adds no scoring pass of its own.
type BoardRow struct {
	Repo        string // owner/repo when repo-qualified; "" in the single-repo reference read
	Stream      string
	Num         string
	Title       string
	Effort      string
	ExecTier    string
	Gate        string
	Risk        loopengine.RiskFlags
	Implementer string
	OutOfRepo   bool // brief Context declares `out-of-repo files:` (out-of-repo serialization)
	BriefPath   string
}

// ID is the claim key / stable item ID for a board row. In the multi-repo live wiring it is
// repo-qualified (`at:`/`tracker:`/…, §Brief IDs); the single-repo reference read uses stream/num,
// exactly as verifyloop does.
func (r BoardRow) ID() string {
	if r.Repo != "" {
		return r.Repo + ":" + r.Stream + "/" + r.Num
	}
	return r.Stream + "/" + r.Num
}

func (r BoardRow) toItem(targetSHA string) loopengine.Item {
	return loopengine.Item{
		ID:          r.ID(),
		BriefPath:   r.BriefPath,
		TargetSHA:   targetSHA,
		Risk:        r.Risk,
		Gate:        r.Gate,
		Effort:      r.Effort,
		ExecTier:    r.ExecTier,
		Implementer: r.Implementer,
		Payload: map[string]string{
			"repo":        r.Repo,
			"kind":        kindBrief,
			"title":       r.Title,
			"out_of_repo": boolStr(r.OutOfRepo),
		},
	}
}

// isIssuePlaceholder reports whether a row is an `issue-*` placeholder. Those rows are
// a different loop's consumer, NOT this loop's — the batch adapter skips them so the two
// consumers of the drain-engine contract never double-dispatch one item. The check is on the
// brief NUMBER shape (`issue-<NN>`), so it holds regardless of which stream carries a placeholder.
func isIssuePlaceholder(r BoardRow) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Num)), "issue-")
}

// OrphanPR is one open PR owing a worker action (>4h, no live claim) — the resume source that
// takes PRIORITY over fresh dispatch (drain-before-dispatch). It is supplied by the live
// `gh pr list` sweep at cutover; the OFFLINE reference build performs no network probe (the
// default orphan source is empty), so orphans are injected in tests.
type OrphanPR struct {
	Repo     string
	Number   int
	ID       string // resume claim key — the ID under which the resume-worker claims like a brief
	Branch   string
	Findings string // the PR's open findings, handed to the resume-worker as its task
}

func (o OrphanPR) toItem() loopengine.Item {
	return loopengine.Item{
		ID: o.ID,
		Payload: map[string]string{
			"repo":     o.Repo,
			"kind":     kindOrphan,
			"pr":       strconv.Itoa(o.Number),
			"branch":   o.Branch,
			"findings": o.Findings,
		},
	}
}

const (
	kindBrief  = "brief"
	kindOrphan = "orphan"
)

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func isOutOfRepo(it loopengine.Item) bool { return it.Payload["out_of_repo"] == "true" }

// --- default STATUS.md board reader ---------------------------------------------------------

var nextUpBriefRe = regexp.MustCompile(`^\s*(\S+)\s*(?:—|-)\s*(.*)$`)

// readNextUp parses the `## Next up` section of <root>/STATUS.md — statusgen's own output, with
// priority / staleness / caps / dep-incomplete already applied — into ordered BoardRows, resolving
// each row to its brief file + frontmatter. It is the default Board source; the live cutover
// substitutes a source that regenerates the board off a fresh origin/main fetch each cycle.
func readNextUp(root, targetSHA string) ([]BoardRow, error) {
	raw, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
	if err != nil {
		return nil, err
	}
	var rows []BoardRow
	for _, cells := range nextUpTableRows(string(raw)) {
		stream := strings.TrimSpace(cells["stream"])
		briefCell := strings.TrimSpace(cells["brief"])
		if stream == "" || briefCell == "" {
			continue
		}
		m := nextUpBriefRe.FindStringSubmatch(briefCell)
		if m == nil {
			continue
		}
		num := strings.TrimSpace(m[1])
		title := strings.TrimSpace(m[2])
		br := BoardRow{Stream: stream, Num: num, Title: title}
		br.BriefPath, br.Effort, br.ExecTier, br.Gate, br.Risk, br.Implementer, br.OutOfRepo = resolveBrief(root, stream, num)
		rows = append(rows, br)
	}
	return rows, nil
}

// nextUpTableRows extracts the rows of the `## Next up` pipe table as column-name maps. It reads
// by header name (like statusgen/parse.go) so column order or extra columns do not matter.
func nextUpTableRows(content string) []map[string]string {
	lines := strings.Split(content, "\n")
	inSection := false
	var header map[string]int
	var out []map[string]string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.EqualFold(trimmed, "## Next up")
			header = nil
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitPipes(line)
		if header == nil {
			header = map[string]int{}
			for i, c := range cells {
				header[strings.ToLower(strings.TrimSpace(c))] = i
			}
			continue
		}
		if isSeparatorRow(line) {
			continue
		}
		row := map[string]string{}
		for name, i := range header {
			if i < len(cells) {
				row[name] = strings.TrimSpace(cells[i])
			}
		}
		out = append(out, row)
	}
	return out
}

var sepRowRe = regexp.MustCompile(`^\s*\|?\s*-{2,}`)

func isSeparatorRow(line string) bool { return sepRowRe.MatchString(line) }

func splitPipes(line string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
}

// resolveBrief finds a Next-up row's brief file and parses the frontmatter subset + the
// out-of-repo Context declaration. A row with no resolvable brief file returns zero frontmatter
// (it still dispatches at the economy default) rather than being silently dropped.
func resolveBrief(root, stream, num string) (path, effort, execTier, gate string, risk loopengine.RiskFlags, implementer string, outOfRepo bool) {
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+num+"-*.md"))
	if len(matches) == 0 {
		return "", "", "", "", loopengine.RiskFlags{}, "", false
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return "", "", "", "", loopengine.RiskFlags{}, "", false
	}
	rel, rerr := filepath.Rel(root, matches[0])
	if rerr != nil || rel == "" {
		rel = matches[0]
	}
	fm := parseFrontmatter(string(raw))
	return rel, fm.Effort, fm.ExecTier, fm.Gate, fm.Risk, fm.Implementer, declaresOutOfRepo(string(raw))
}

// declaresOutOfRepo reports whether a brief's Context declares `out-of-repo files:` (paths outside
// the repo, e.g. `~/.claude/skills/**`). That declaration IS the serialization claim: at
// most one such brief may be in flight across all streams. The check is a typed read of the brief
// body, replacing the prose rule the SKILL used to carry.
func declaresOutOfRepo(content string) bool {
	return regexp.MustCompile(`(?im)^\s*out-of-repo files:`).MatchString(content)
}
