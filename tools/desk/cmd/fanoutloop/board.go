package main

import (
	"fmt"
	"os"
	"os/exec"
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
	// WriteScopes is the row's ADVISORY write-scope set, derived from the
	// brief's Context `files:` list. Carried onto the Item so `plan` can warn on overlap with
	// in-flight claims. Advisory only — never gates dispatch. The zero value is could-not-derive.
	WriteScopes loopengine.WriteScopeSet
	// Labels is the issue's authoritative GitHub label set, when the board source can supply it (an
	// issue-sourced / live board). It is the AUTHORITATIVE discriminator for a foreign dispatch token
	// (the `review-request` label) — see isForeignDispatchToken. The default STATUS.md reader carries
	// no label column, so rows it produces leave this nil and fall back to the title convention.
	Labels []string
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
		WriteScopes: r.WriteScopes,
		Payload: map[string]string{
			"repo":        r.Repo,
			"kind":        kindBrief,
			"title":       r.Title,
			"out_of_repo": boolStr(r.OutOfRepo),
		},
	}
}

// isIssuePlaceholder reports whether a row is an `issue-<NN>` placeholder filed by the intake
// scanner. These ARE this loop's work: per the worker-desk dispatch spec (Procedure 2 — "INCLUDE
// issue-placeholders — `issue-<NN>` rows ARE yours to dispatch"), a placeholder flows through the
// normal Next-up → worker-desk → draft-PR → review pipeline like any other brief; the intake-desk
// only FILES the placeholder, dispatch belongs here. The check is on the brief NUMBER shape
// (`issue-<NN>`), so it holds regardless of which stream carries a placeholder. It is used to
// recognise this loop's well-known number shape (e.g. for the `<repo>--issue-<NN>` claim key) — it
// is NOT, on its own, a reason to skip a row. The one class SelectQueue drops is a DIFFERENT loop's
// dispatch token; see isForeignDispatchToken.
func isIssuePlaceholder(r BoardRow) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Num)), "issue-")
}

// reviewRequestLabel is the authoritative GitHub label the process desk stamps on a review-loop
// dispatch token; the issue-loop work-scanner (issueboard) already excludes it via the same label
// name (topology labels.system_state). Kept as a local constant because the fanoutloop package is
// stdlib + loopengine only and does not import the topology label set.
const reviewRequestLabel = "review-request"

// hasReviewRequestLabel reports whether the row carries the authoritative `review-request` label
// (case-insensitive), i.e. the board source resolved the issue's real GitHub labels.
func hasReviewRequestLabel(r BoardRow) bool {
	for _, l := range r.Labels {
		if strings.EqualFold(strings.TrimSpace(l), reviewRequestLabel) {
			return true
		}
	}
	return false
}

// isForeignDispatchToken reports whether a row is a dispatch token owned by a DIFFERENT loop's
// consumer, and therefore must NOT be dispatched by worker-desk. The concrete case is a
// `review-request` issue: the process desk files it as a token for the pr-review-desk (review)
// loop, and the issue-loop work-scanner deliberately EXCLUDES it by its `review-request` label
// (the-desk SKILL: the label "distinguishes dispatch tokens from work issues").
//
// The AUTHORITATIVE discriminator is that GitHub `review-request` LABEL (BoardRow.Labels): a
// label-bearing board source (issue-sourced / live) is trusted first, so a review-request token
// whose title deviated from the spec-canonical prefix is still caught (#101 hardening). The
// canonical `review-request: <target> — <type>` title shape — which a genuine `issue-<NN>` work
// placeholder never carries — remains a FALLBACK for label-less board sources (the default STATUS.md
// reader has no label column). The two are OR'd, so adding the label check never lets a token
// through that the title check previously caught. This is the ONLY class SelectQueue drops as
// another loop's consumer; `issue-<NN>` work placeholders are this loop's own and dispatch normally.
func isForeignDispatchToken(r BoardRow) bool {
	if hasReviewRequestLabel(r) {
		return true
	}
	t := strings.ToLower(strings.TrimSpace(r.Title))
	return t == "review-request" ||
		strings.HasPrefix(t, "review-request:") ||
		strings.HasPrefix(t, "review-request ")
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
	kindRework = "rework"
)

// toReworkItem is toItem's REWORK sibling: same BoardRow shape (an `Awaiting implementer
// rework` row resolves through the identical Stream/Num/brief-file machinery as a Next-up row),
// tagged with the rework kind so classOf (main.go, example-stream/05) buckets it into the
// reservation's rework class rather than fresh. A separate method rather than a toItem
// parameter because kindBrief must stay the ordinary, no-argument default — the common case
// should not carry a kind argument nobody reading toItem's call sites would expect.
func (r BoardRow) toReworkItem(targetSHA string) loopengine.Item {
	it := r.toItem(targetSHA)
	it.Payload["kind"] = kindRework
	return it
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func isOutOfRepo(it loopengine.Item) bool { return it.Payload["out_of_repo"] == "true" }

// --- default STATUS.md board reader ---------------------------------------------------------

var nextUpBriefRe = regexp.MustCompile(`^\s*(\S+)\s*(?:—|-)\s*(.*)$`)

// readNextUp parses the `## Next up` section of the root repo's STATUS.md — statusgen's own output,
// with priority / staleness / caps / dep-incomplete already applied — into ordered BoardRows,
// resolving each row to its brief file + frontmatter. It is the default Board source; the live
// cutover substitutes a source that regenerates the board off a fresh origin/main fetch each cycle.
//
// The board is read from the already-fetched `refs/remotes/origin/main` ref, NOT the working tree
// (#1674). `git fetch` updates `origin/*` but not a checkout's working tree, so a SIBLING root (any
// board root other than the desk's own worktree, which is freshly branched off origin/main) is
// parked on whatever stale commit it last synced; its working-tree STATUS.md then over-reports every
// row that has since gone done/verified/merged on origin/main, manufacturing phantom dispatches.
// Reading the board from the ref plans every root against origin/main, exactly as the desk's own
// worktree already did. Brief frontmatter is still resolved from the working tree (resolveBrief) —
// a row's dispatchability is decided by the origin/main board; the brief only supplies advisory
// tiering, and a working-tree brief that is missing or stale degrades to the economy default rather
// than dropping the row.
func readNextUp(root, targetSHA string) ([]BoardRow, error) {
	raw, err := statusMDContent(root)
	if err != nil {
		return nil, err
	}
	var rows []BoardRow
	for _, cells := range nextUpTableRows(raw) {
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
		br.BriefPath, br.Effort, br.ExecTier, br.Gate, br.Risk, br.Implementer, br.OutOfRepo, br.WriteScopes = resolveBrief(root, stream, num)
		rows = append(rows, br)
	}
	return rows, nil
}

// statusMDContent returns STATUS.md's content for root, preferring the fetched origin/main ref
// (statusFromOriginMain, #1674) and falling back to the working tree with a stderr NOTE. Every
// STATUS.md section reader (readNextUp, readAwaitingRework) goes through this ONE function, so
// there is exactly one origin/main-vs-working-tree policy to keep correct rather than one per
// section.
func statusMDContent(root string) (string, error) {
	raw, fromRef := statusFromOriginMain(root)
	if !fromRef {
		// could-not-check the ref (root is not a git repo, or origin was never fetched, or STATUS.md
		// is absent on origin/main): fall back to the working-tree STATUS.md and SAY SO — a sibling's
		// working tree can be stale (#1674), so this path is reported on stderr, never silently
		// presented as an origin/main read.
		b, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "fanoutloop: NOTE: could not read STATUS.md from refs/remotes/origin/main under %s — falling back to the working-tree STATUS.md, which may be stale (#1674)\n", root)
		return string(b), nil
	}
	return raw, nil
}

// readAwaitingRework parses the `### Awaiting implementer rework` STATUS.md section
// (statusgen's own classifyAwaiting output, statusgen/emit.go) into BoardRows tagged as REWORK
// items — worker-desk §Sources of work row 5, which this makes a live `plan` source rather than
// a section an operator reads by hand. The section's Brief cell carries only the number and an
// optional `[exec:strong]` tag (`11 [exec:strong]`, statusgen/emit.go's rendering) — unlike the
// Next-up Brief cell (`NUM — Title`), so it takes its own leading-token parse rather than
// nextUpBriefRe's em-dash split.
func readAwaitingRework(root string) ([]BoardRow, error) {
	raw, err := statusMDContent(root)
	if err != nil {
		return nil, err
	}
	var rows []BoardRow
	for _, cells := range sectionTableRows(raw, "Awaiting implementer rework") {
		stream := strings.TrimSpace(cells["stream"])
		briefCell := strings.TrimSpace(cells["brief"])
		if stream == "" || briefCell == "" {
			continue
		}
		fields := strings.Fields(briefCell)
		if len(fields) == 0 {
			continue
		}
		num := fields[0]
		br := BoardRow{Stream: stream, Num: num}
		br.BriefPath, br.Effort, br.ExecTier, br.Gate, br.Risk, br.Implementer, br.OutOfRepo, br.WriteScopes = resolveBrief(root, stream, num)
		rows = append(rows, br)
	}
	return rows, nil
}

// statusFromOriginMain reads STATUS.md from the root repo's already-fetched
// `refs/remotes/origin/main` ref — the board source that fixes the sibling-staleness phantom (#1674).
//
// OFFLINE ENVELOPE. The only git this runs is `git -C <root> show refs/remotes/origin/main:STATUS.md`
// against the LOCAL ref — never `git fetch`, never `git ls-remote`, no network is contacted
// (GIT_TERMINAL_PROMPT=0, matching writescope_io.go's offline reader). The worker-desk boot fetches
// origin BEFORE it plans, so the local ref is already current; `plan` itself stays no-network.
//
// Returns (bytes, true) on success. Any failure — root is not a git repo, the ref is absent because
// origin was never fetched, or STATUS.md does not exist on origin/main — returns ("", false) so the
// caller can fall back to the working-tree read and report the could-not-check, never rounding a
// value it did not read up to a confident answer.
func statusFromOriginMain(root string) (string, bool) {
	cmd := exec.Command("git", "-C", root, "show", "refs/remotes/origin/main:STATUS.md")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
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

// sectionTableRows extracts the pipe-table rows under a `### <name>` (H3) heading whose trimmed
// text STARTS WITH headingPrefix, case-insensitively, stopping at the next `## ` or `### `
// heading. Unlike nextUpTableRows — an exact `## Next up` (H2) match — this is a PREFIX match on
// an H3 because statusgen renders a live count onto the heading (`### Awaiting implementer
// rework (1)`, emit.go) that an exact match would never see.
func sectionTableRows(content, headingPrefix string) []map[string]string {
	lowerPrefix := strings.ToLower(headingPrefix)
	lines := strings.Split(content, "\n")
	inSection := false
	var header map[string]int
	var out []map[string]string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			name := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			inSection = strings.HasPrefix(trimmed, "### ") && strings.HasPrefix(strings.ToLower(name), lowerPrefix)
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

// resolveBrief finds a Next-up row's brief file and parses the frontmatter subset + the
// out-of-repo Context declaration. A row with no resolvable brief file returns zero frontmatter
// (it still dispatches at the economy default) rather than being silently dropped.
func resolveBrief(root, stream, num string) (path, effort, execTier, gate string, risk loopengine.RiskFlags, implementer string, outOfRepo bool, scopes loopengine.WriteScopeSet) {
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+num+"-*.md"))
	if len(matches) == 0 {
		// No resolvable brief file: scopes are UNKNOWN (could-not-derive), never silently clear.
		return "", "", "", "", loopengine.RiskFlags{}, "", false, loopengine.WriteScopeSet{}
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return "", "", "", "", loopengine.RiskFlags{}, "", false, loopengine.WriteScopeSet{}
	}
	rel, rerr := filepath.Rel(root, matches[0])
	if rerr != nil || rel == "" {
		rel = matches[0]
	}
	fm := parseFrontmatter(string(raw))
	return rel, fm.Effort, fm.ExecTier, fm.Gate, fm.Risk, fm.Implementer, declaresOutOfRepo(string(raw)), loopengine.DeriveWriteScopes(string(raw))
}

// declaresOutOfRepo reports whether a brief's Context declares `out-of-repo files:` (paths outside
// the repo, e.g. `~/.claude/skills/**`). That declaration IS the serialization claim: at
// most one such brief may be in flight across all streams. The check is a typed read of the brief
// body, replacing the prose rule the SKILL used to carry.
func declaresOutOfRepo(content string) bool {
	return regexp.MustCompile(`(?im)^\s*out-of-repo files:`).MatchString(content)
}
