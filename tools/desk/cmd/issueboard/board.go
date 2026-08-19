package main

// board.go — the data model, GET-only gh fetchers, local placeholder/intake readers,
// the pure classify function, and the report renderers. Every gh call here is a READ
// (`issue list`, `issue view`) — no mutating verb, ever. The PATH-shim test
// (issueboard_test.go) proves this end to end by enumerating every recorded gh
// invocation, mirroring tools/desk/cmd/deskboard/board.go + deskboard_test.go.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// ---------------------------------------------------------------------------
// owned repos + excluded labels.
//
// Both used to be hand-synced copies of statusgen's scanRepos /
// scanExcludedLabels, kept equal by a comment asking you to. The comment did not
// work: `excludedLabels` here was MISSING `review-request`, which statusgen's set
// carries (#829) — so this board asked for a placeholder on exactly the class of
// issue the scanner deliberately excludes, and the divergence became its own
// issue, which is how every one of these copies ends.
//
// Neither is a copy now. The repo set reads the roster (deskkit.ScanRepos /
// ASSAY_SCAN_REPOS — the identical key statusgen reads); the label sets read
// `topology.yaml` through tools/desk/internal/topology, the same declared source
// statusgen derives its copy from. One declared source per fact, with the
// derivations diffed in CI by TestTopologyDriftRegistry (#276).
// ---------------------------------------------------------------------------

// ownedRepos is the intake SCAN scope: the owned-repo set the issue lane READS.
// It reads the roster (ASSAY_SCAN_REPOS, via deskkit.ScanRepos) rather than
// compiling in one organisation's slugs — the desk-side half of the owned-repo
// roster externalised out of source. statusgen's
// scanRepos reads the IDENTICAL key, so ONE roster value now feeds the board's read
// surface and the scanner's write surface both. That is what keeps them from
// drifting into the "clean front door while issues sit unmonitored" bug (#784) the
// two hand-synced compiled lists produced — a single source of truth, not two lists
// a comment begged you to keep equal. The single-source coupling is enforced by
// TestOwnedReposEqualsStatusgenScanRepos (ownedrepos_coupling_test.go): it checks
// that this reads deskkit.ScanRepos and that statusgen consumes the same env key.
//
// It is a DISTINCT set from deskkit.AllowedRepos() (the WRITE boundary,
// ASSAY_ALLOWED_REPOS). The scan scope covers repos the desk is the front door for
// that are not write targets (e.g. site repos), and deliberately HOLDS OUT a
// write-boundary repo from this READ surface: the desk still POSTS to it
// (deskpost/deskpr/deskreply gate on deskkit.IsAllowedRepo, which still lists it),
// so narrowing this READ surface does not change the write boundary. Unset
// yields an empty read surface (fail safe).
func ownedRepos() []string { return deskkit.ScanRepos() }

// excludedLabelSet returns the system-STATE labels that disqualify an open issue
// from needing a placeholder: each names a closeable state the desk machinery
// emits, not a unit of work. Declared once in topology.yaml
// (labels.system_state), with the rationale for each entry beside it.
func excludedLabelSet() map[string]bool { return topology.Compiled().SystemStateLabelSet() }

// decisionLabelSet returns the labels that mark an issue decision-owed — the
// SLA-escalation scope (#703 classifyIssue spec): the
// needs-decision-class label plus the `question` escalation label. A
// decision-owed open issue is either AWAIT (fresh) or ESCALATE (past the SLA) —
// see classifyIssue. Declared once in topology.yaml (labels.decision_owed).
func decisionLabelSet() map[string]bool { return topology.Compiled().DecisionOwedLabelSet() }

// hasDecisionLabel reports whether any label marks the issue decision-owed
// (case-insensitive, mirrors hasExcludedLabel's matching style).
func hasDecisionLabel(labels []string) bool {
	set := decisionLabelSet()
	for _, l := range labels {
		if set[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// escalateSLADays is the default silence threshold before a decision-owed issue
// escalates — the market-scan "silent 6d" figure (desk-console-design.md §13.3's
// open threshold question). One line to change; overridable per invocation via
// issueboard's --sla-days flag (main.go).
const escalateSLADays = 6

// looksLikeBot mirrors deskkit/rosterconfig.go's unexported looksLikeBot (a
// documented duplicate, same pattern as the ownedRepos()/excludedLabels comment
// above: a small check kept in sync rather than exported across an unrelated
// module boundary). Used only to keep a bot's own comments from resetting the
// escalation clock — a desk ping must never look like a human answer.
func looksLikeBot(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return strings.HasSuffix(l, "[bot]") ||
		strings.HasPrefix(l, "app/") ||
		strings.HasSuffix(l, "-app") ||
		strings.HasSuffix(l, "-bot")
}

// lastHumanResponseAt computes the escalation clock's start: the latest
// human-authored comment timestamp, or the issue's own creation time if no human
// has commented yet — the question was posed then, so silence is measured from
// that point ("last human response" per the brief, using the board's existing
// bot-vs-human identity split; a bot/desk comment never moves this forward).
func lastHumanResponseAt(issueCreatedAt time.Time, events []deskkit.ContentEvent) time.Time {
	last := issueCreatedAt
	for _, e := range events {
		if looksLikeBot(e.Author) {
			continue
		}
		if e.CreatedAt.After(last) {
			last = e.CreatedAt
		}
	}
	return last
}

// escalationAgeDays is the whole-days age since the escalation clock's last reset.
func escalationAgeDays(lastHumanResponse, now time.Time) int {
	return int(now.Sub(lastHumanResponse).Hours() / 24)
}

// escalationExceedsSLA is the pure threshold check (mirrors classifyIntakeAge):
// exactly at the SLA is NOT yet over — the same boundary convention the
// intake-staleness SLA already uses.
func escalationExceedsSLA(ageDays, slaDays int) bool {
	return ageDays > slaDays
}

// ---------------------------------------------------------------------------
// gh runner (the single choke point every GET goes through)
// ---------------------------------------------------------------------------

// ghRun shells out to the real `gh` binary and returns stdout. It is a package var so
// the PATH-shim test can exercise the REAL exec path (a fake gh first in PATH) and so
// nothing here can silently swap in a mutating call. A non-zero gh exit becomes an
// error carrying gh's stderr.
var ghRun = func(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %v", strings.Join(args, " "), err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// data model (mirrors the gh JSON shapes)
// ---------------------------------------------------------------------------

type ghLabel struct {
	Name string `json:"name"`
}

type ghAuthor struct {
	Login string `json:"login"` // gh CLI renders App authors as "app/<slug>"
}

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    ghAuthor  `json:"author"`
	Labels    []ghLabel `json:"labels"`
	CreatedAt time.Time `json:"createdAt"` // escalation clock baseline
}

// ---------------------------------------------------------------------------
// GET-only fetchers
// ---------------------------------------------------------------------------

// fetchOpenIssues returns every open issue for repo (state=open only — the caller
// cross-references against local placeholders to detect closed ones without ever
// querying gh for closed issues).
func fetchOpenIssues(repo string) ([]ghIssue, error) {
	out, err := ghRun("issue", "list", "-R", repo, "--state", "open", "--limit", "1000",
		"--json", "number,title,author,labels,createdAt")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read open issues for "+repo, err)
	}
	var issues []ghIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, deskkit.Unverifiable("cannot parse issue list for "+repo, err)
	}
	return issues, nil
}

// fetchIssueTitle resolves a single issue's title (used for RETIRE rows, where the
// issue is no longer in the open list so its title isn't otherwise known). Best
// effort: the caller falls back to a placeholder string on error rather than failing
// the whole board — RETIRE is already fully determined without the title.
func fetchIssueTitle(repo string, num int) (string, error) {
	out, err := ghRun("issue", "view", strconv.Itoa(num), "-R", repo, "--json", "title")
	if err != nil {
		return "", err
	}
	var v struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", err
	}
	return v.Title, nil
}

// fetchIssueBlessed evaluates the blessing for an issue whose AUTHOR is
// untrusted: ONE bounded `gh api graphql` read (deskkit.IssueTrustQuery) per such
// issue — trusted authors cost zero extra calls. The read is a GraphQL QUERY (the
// PATH-shim test allows graphql invocations only when no mutation appears). It needs
// lastEditedAt (content-edit tracking; REST updated_at moves on labels) and numeric
// databaseIds (recycled-login defense). An incomplete payload (any connection past
// first:100) is NOT blessed — fail closed to quarantine. A fetch/parse error is
// Unverifiable (exit 6): a blessing we cannot verify is never guessed either way.
func fetchIssueBlessed(repo string, num int) (bool, error) {
	bodyEdited, events, complete, err := fetchIssueTrustPayload(repo, num)
	if err != nil {
		return false, err
	}
	if !complete {
		return false, nil // overflowed thread — quarantine, never silently admit
	}
	return deskkit.Blessed(bodyEdited, events), nil
}

// fetchIssueTrustPayload is the shared GraphQL read behind fetchIssueBlessed (the
// trust gate) and fetchIssueEvents (the escalation clock):
// ONE bounded `gh api graphql` read (deskkit.IssueTrustQuery) per issue, parsed into
// the item's body-edit time and its content events. A query, never a mutation — the
// PATH-shim test allows graphql invocations only when no mutation appears.
func fetchIssueTrustPayload(repo string, num int) (bodyEdited time.Time, events []deskkit.ContentEvent, complete bool, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return time.Time{}, nil, false, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	out, err := ghRun("api", "graphql",
		"-f", "query="+deskkit.IssueTrustQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(num))
	if err != nil {
		return time.Time{}, nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot read events for %s#%d", repo, num), err)
	}
	bodyEdited, events, complete, perr := deskkit.ParseIssueTrustPayload(out)
	if perr != nil {
		return time.Time{}, nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot parse events for %s#%d", repo, num), perr)
	}
	return bodyEdited, events, complete, nil
}

// fetchIssueEvents reads a decision-owed issue's content-event history for the
// escalation clock: last-human-response timestamp. Only
// decision-owed issues (needs-decision/question label) pay this extra read;
// everything else costs nothing extra. An incomplete (overflowed) thread fails
// closed (Unverifiable) rather than guessing the clock from a partial page — unlike
// the trust gate, which can safely treat overflow as "not blessed".
func fetchIssueEvents(repo string, num int) ([]deskkit.ContentEvent, error) {
	_, events, complete, err := fetchIssueTrustPayload(repo, num)
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, deskkit.Unverifiable(fmt.Sprintf("event history for %s#%d overflowed a single page — escalation clock needs the full thread", repo, num), nil)
	}
	return events, nil
}

// labelNames flattens gh's label objects to their names, dropping blanks.
func labelNames(labels []ghLabel) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if n := strings.TrimSpace(l.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// hasExcludedLabel reports whether any label is in the system-state set
// (case-insensitive).
func hasExcludedLabel(labels []string) bool {
	set := excludedLabelSet()
	for _, l := range labels {
		if set[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// local readers — placeholders (docs/streams/issue-loop/) and intake
// (docs/streams/intake/). Both are stdlib-only, no gh, no third-party YAML: a
// small line-based frontmatter scan (identical technique to
// tools/statusgen's own applyCloseOut/unblockPlaceholderFile in-place editors)
// is all a flat key: value frontmatter block needs.
// ---------------------------------------------------------------------------

const (
	issueLoopDir        = "docs/streams/issue-loop"
	intakeDir           = "docs/streams/intake"
	intakeThresholdDays = 3 // the 3-day intake-age SLA
)

// placeholderNameRe matches a placeholder filename: issue-<NN>.md, optionally
// prefixed by a <repo-slug>- for a foreign-repo issue (e.g. agents-issue-21.md).
// Capture group 1 is the issue number. Mirrors tools/statusgen/placeholder.go's
// placeholderNameRe exactly.
var placeholderNameRe = regexp.MustCompile(`^(?:[a-z0-9][a-z0-9-]*-)?issue-([0-9]+)\.md$`)

// parseFrontmatter reads a file's YAML-ish frontmatter block (between --- markers)
// as a flat map of trimmed key -> trimmed, unquoted value. It is deliberately not a
// real YAML parser (stdlib-only, issue #703): every field this tool reads (repo,
// status, blocked, id, date, title, disposition) is a plain scalar on its own line.
func parseFrontmatter(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("%s: missing frontmatter", path)
	}
	fields := map[string]string{}
	closed := false
	for _, ln := range lines[1:] {
		t := strings.TrimSpace(ln)
		if t == "---" {
			closed = true
			break
		}
		key, val, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"`)
	}
	if !closed {
		return nil, fmt.Errorf("%s: unterminated frontmatter", path)
	}
	return fields, nil
}

// placeholderRow is the parsed frontmatter of one docs/streams/issue-loop placeholder
// file this tool cares about.
type placeholderRow struct {
	Path    string
	Issue   int
	Repo    string
	Status  string
	Blocked string
}

// loadPlaceholders scans root/docs/streams/issue-loop for placeholder-named files and
// parses each one's frontmatter. A missing directory is not an error (empty result) —
// a fresh checkout may not have created it yet; a present-but-malformed placeholder
// file IS an error (fail closed — never silently drop a placeholder from the board).
func loadPlaceholders(root string) ([]placeholderRow, error) {
	dir := filepath.Join(root, issueLoopDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, deskkit.Unverifiable("cannot read "+issueLoopDir, err)
	}
	var out []placeholderRow
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := placeholderNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fields, err := parseFrontmatter(path)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot parse placeholder "+path, err)
		}
		out = append(out, placeholderRow{
			Path:    path,
			Issue:   num,
			Repo:    fields["repo"],
			Status:  strings.ToLower(fields["status"]),
			Blocked: fields["blocked"],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Issue < out[j].Issue
	})
	return out, nil
}

// intakeRow is the parsed frontmatter of one docs/streams/intake entry, restricted to
// disposition: new (or empty, which defaults to new — mirrors
// tools/statusgen/intake_alarm.go's intakeAlarm rule exactly).
type intakeRow struct {
	Path    string
	ID      string
	Title   string
	Date    string
	Days    int
	BadDate bool
	Over    bool
}

// classifyIntakeAge reports whether an age in days is past the untriaged threshold
// (the 3-day intake-age SLA). Pure, trivial, unit-tested alongside classifyIssue.
func classifyIntakeAge(days int) bool {
	return days > intakeThresholdDays
}

// loadIntakeRows scans root/docs/streams/intake for entries with disposition: new (or
// empty) and computes each one's age in days against now. A missing directory is not
// an error (empty result); a malformed entry file IS (fail closed).
func loadIntakeRows(root string, now time.Time) ([]intakeRow, error) {
	dir := filepath.Join(root, intakeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, deskkit.Unverifiable("cannot read "+intakeDir, err)
	}
	var out []intakeRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fields, err := parseFrontmatter(path)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot parse intake entry "+path, err)
		}
		disposition := strings.TrimSpace(fields["disposition"])
		if disposition != "" && disposition != "new" {
			continue
		}
		row := intakeRow{Path: path, ID: fields["id"], Title: fields["title"], Date: fields["date"]}
		parsed, perr := time.Parse("2006-01-02", strings.TrimSpace(fields["date"]))
		if perr != nil {
			row.BadDate = true
		} else {
			row.Days = int(now.Sub(parsed).Hours() / 24)
			row.Over = classifyIntakeAge(row.Days)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Days != out[j].Days {
			return out[i].Days > out[j].Days // oldest first
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ---------------------------------------------------------------------------
// pure classifier — unit-tested directly, no gh, no filesystem.
// ---------------------------------------------------------------------------

const (
	actEscalate          = "ESCALATE"
	actCreatePlaceholder = "CREATE-PLACEHOLDER"
	actRetire            = "RETIRE"
	actAwait             = "AWAIT"
	actNone              = "NONE"
)

// issueClassifyInput carries everything the pure classifier needs for one repo+issue.
type issueClassifyInput struct {
	open            bool // issue is currently open (per the live gh read)
	hasPlaceholder  bool // a docs/streams/issue-loop placeholder file exists for it
	placeholderDone bool // that placeholder's status is "done" (already retired)
	excludedLabel   bool // the open issue carries a system-emitted label
	blocked         bool // that placeholder carries blocked: awaiting-issue-response
	decisionOwed    bool // carries a needs-decision/question label
	agedPastSLA     bool // age since last human response > SLA
}

// classifyIssue maps one repo+issue's state to its ACTION (issue #703 spec):
//   - ESCALATE           — open, decision-owed, aged past the SLA. Checked first and
//     unconditionally: an aged decision outranks whatever the placeholder machinery
//     below would otherwise say (top of issueActionPrio too).
//   - AWAIT              — placeholder exists, issue open, worker parked a question
//     (blocked: awaiting-issue-response) — OR the issue is decision-owed but still
//     within the SLA: both are "waiting on a human", the same class.
//   - CREATE-PLACEHOLDER — open issue, no placeholder, not a system-label state.
//   - RETIRE             — placeholder exists, its issue is now closed, not yet retired.
//   - NONE                — everything else (already retired, excluded label with no
//     placeholder, or a normal open+unblocked placeholder — nothing to do).
func classifyIssue(in issueClassifyInput) string {
	if in.open && in.decisionOwed {
		if in.agedPastSLA {
			return actEscalate
		}
		return actAwait
	}
	if in.hasPlaceholder {
		if !in.open {
			if in.placeholderDone {
				return actNone
			}
			return actRetire
		}
		if in.blocked {
			return actAwait
		}
		return actNone
	}
	if !in.open {
		return actNone // closed issue that never got a placeholder — nothing to do
	}
	if in.excludedLabel {
		return actNone // system-emitted label — a closeable state, not work
	}
	return actCreatePlaceholder
}

var issueActionPrio = map[string]int{
	actEscalate:          0,
	actCreatePlaceholder: 1,
	actRetire:            2,
	actAwait:             3,
	actNone:              4,
}

// ---------------------------------------------------------------------------
// board assembly
// ---------------------------------------------------------------------------

type issueBoardRow struct {
	Repo    string
	Number  int
	Title   string
	Action  string
	AgeDays int // escalation-clock age in days; meaningful only when Action is ESCALATE
}

// externalRow is an open issue quarantined by the trust gate: untrusted author and no
// blessing comment. Quarantined-visible — listed and counted, never actionable; one
// comment from the blessing authority admits it to the actionable lane on the next run.
type externalRow struct {
	Repo   string
	Number int
	Title  string
	Author string
}

// computeIssueBoard assembles the issue lane: one row per open issue across
// ownedRepos, plus one RETIRE/NONE row per existing placeholder whose issue is no
// longer open. Every gh call is a READ (issue list, issue view, comments GET).
//
// Trust gate (deskkit/trust.go): an open issue whose author is untrusted AND that
// carries no blessing comment never reaches classifyIssue — it is diverted to the
// external/unblessed list. Comment authors are fetched ONLY for untrusted-author
// issues (bounded API growth). RETIRE rows for closed issues are NOT gated: retiring
// removes OUR local placeholder; it does not act on the issue's content.
//
// Escalation: decision-owed issues (needs-decision/question
// label) that reach this point — i.e. already in the owned/blessed lane, past the
// trust gate above — pay ONE extra bounded events read to compute the escalation
// clock. slaDays is the silence threshold (escalateSLADays by default, overridable
// per invocation).
func computeIssueBoard(root string, now time.Time, slaDays int) ([]issueBoardRow, []externalRow, error) {
	// Fail closed on an EMPTY scan scope BEFORE any read (#777). ownedRepos() is
	// ScanRepos(): unset ASSAY_SCAN_REPOS yields zero repos, the loop below then runs
	// zero iterations, makes no gh call, and the empty result renders as "(no open
	// issues across owned repos)" at exit 0 — a clean-and-empty board over a front door
	// that owns 200+ issues. An empty sweep is COULD-NOT-CHECK, never green-and-empty:
	// this is the scan-scope twin of the #489 BoardScope() guard, which only ever
	// covered the WRITE set. Both the `board` and `issues` paths route through here, so
	// this is the one place every scan-dependent verb provably passes; the intake lane
	// reads local files only and deliberately does NOT gate on the scan scope.
	if err := deskkit.ScanScopeError(); err != nil {
		return nil, nil, err
	}
	placeholders, err := loadPlaceholders(root)
	if err != nil {
		return nil, nil, err
	}
	byKey := map[string]placeholderRow{}
	for _, ph := range placeholders {
		byKey[ph.Repo+"#"+strconv.Itoa(ph.Issue)] = ph
	}

	var rows []issueBoardRow
	var external []externalRow
	for _, repo := range ownedRepos() {
		issues, err := fetchOpenIssues(repo)
		if err != nil {
			return nil, nil, err
		}
		seen := map[string]bool{}

		for _, iss := range issues {
			key := repo + "#" + strconv.Itoa(iss.Number)
			seen[key] = true // even quarantined issues are OPEN — never mistake one for closed

			// Trust gate: untrusted author → the ONE bounded trust-events read; no
			// CURRENT blessing (bless-then-edit aware) → quarantine (visible,
			// not actionable).
			if !deskkit.TrustedAuthor(iss.Author.Login) {
				blessed, cerr := fetchIssueBlessed(repo, iss.Number)
				if cerr != nil {
					return nil, nil, cerr
				}
				if !blessed {
					external = append(external, externalRow{Repo: repo, Number: iss.Number, Title: iss.Title, Author: iss.Author.Login})
					continue
				}
			}

			decisionOwed := hasDecisionLabel(labelNames(iss.Labels))
			var ageDays int
			var agedPastSLA bool
			if decisionOwed {
				events, everr := fetchIssueEvents(repo, iss.Number)
				if everr != nil {
					return nil, nil, everr
				}
				last := lastHumanResponseAt(iss.CreatedAt, events)
				ageDays = escalationAgeDays(last, now)
				agedPastSLA = escalationExceedsSLA(ageDays, slaDays)
			}

			ph, hasPh := byKey[key]
			action := classifyIssue(issueClassifyInput{
				open:            true,
				hasPlaceholder:  hasPh,
				placeholderDone: hasPh && ph.Status == "done",
				excludedLabel:   hasExcludedLabel(labelNames(iss.Labels)),
				blocked:         hasPh && ph.Blocked == "awaiting-issue-response",
				decisionOwed:    decisionOwed,
				agedPastSLA:     agedPastSLA,
			})
			rows = append(rows, issueBoardRow{Repo: repo, Number: iss.Number, Title: iss.Title, Action: action, AgeDays: ageDays})
		}

		for key, ph := range byKey {
			if ph.Repo != repo || seen[key] {
				continue
			}
			action := classifyIssue(issueClassifyInput{open: false, hasPlaceholder: true, placeholderDone: ph.Status == "done"})
			title, terr := fetchIssueTitle(repo, ph.Issue)
			if terr != nil || title == "" {
				title = "(title unavailable — issue closed)"
			}
			rows = append(rows, issueBoardRow{Repo: repo, Number: ph.Issue, Title: title, Action: action})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if issueActionPrio[rows[i].Action] != issueActionPrio[rows[j].Action] {
			return issueActionPrio[rows[i].Action] < issueActionPrio[rows[j].Action]
		}
		if rows[i].Repo != rows[j].Repo {
			return rows[i].Repo < rows[j].Repo
		}
		return rows[i].Number < rows[j].Number
	})
	sort.SliceStable(external, func(i, j int) bool {
		if external[i].Repo != external[j].Repo {
			return external[i].Repo < external[j].Repo
		}
		return external[i].Number < external[j].Number
	})
	return rows, external, nil
}

// ---------------------------------------------------------------------------
// reports
// ---------------------------------------------------------------------------

// Report is what a subcommand hands back to main: a table renderer and the audit
// Detail (a short summary of what was found).
type Report struct {
	render func(io.Writer)
	detail string
}

func summarizeIssues(rows []issueBoardRow, external []externalRow) string {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Action]++
	}
	return fmt.Sprintf("issues: escalate=%d create=%d retire=%d await=%d none=%d external=%d",
		counts[actEscalate], counts[actCreatePlaceholder], counts[actRetire], counts[actAwait], counts[actNone], len(external))
}

func summarizeIntake(rows []intakeRow) string {
	over := 0
	for _, r := range rows {
		if r.Over {
			over++
		}
	}
	return fmt.Sprintf("intake: untriaged=%d over=%d", len(rows), over)
}

func renderIssueLane(w io.Writer, rows []issueBoardRow) {
	fmt.Fprintln(w, "ISSUE LANE")
	fmt.Fprintf(w, "%-20s %-40s %s\n", "ACTION", "REPO#NUM", "TITLE")
	for _, r := range rows {
		t := title(r.Title, 70)
		if r.Action == actEscalate {
			t = fmt.Sprintf("%s [age %dd]", t, r.AgeDays) // render age in the row
		}
		fmt.Fprintf(w, "%-20s %-40s %s\n", r.Action, shortRepo(r.Repo)+"#"+strconv.Itoa(r.Number), t)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no open issues across owned repos)")
	}
}

// renderExternalLane lists quarantined issues: visible so the human sees what awaits their
// blessing, never actionable. One comment from the blessing authority admits it. All
// public-origin text (title, author) renders INERT — quoted, control characters
// escaped — so the quarantine listing itself cannot inject into the human/agent
// reading it (Clinejection-style title payloads).
func renderExternalLane(w io.Writer, rows []externalRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "EXTERNAL / UNBLESSED — %d issue(s) quarantined (untrusted author, no current blessing; NOT actionable — a comment from %s admits, edits after a blessing re-quarantine)\n", len(rows), blessAuthorityName())
	fmt.Fprintf(w, "%-40s %-28s %s\n", "REPO#NUM", "AUTHOR", "TITLE")
	for _, r := range rows {
		author := r.Author
		if author == "" {
			author = "(unknown)"
		}
		fmt.Fprintf(w, "%-40s %-28s %s\n", shortRepo(r.Repo)+"#"+strconv.Itoa(r.Number), inert(author, 26), inert(r.Title, 60))
	}
}

func renderIntakeLane(w io.Writer, rows []intakeRow) {
	fmt.Fprintln(w, "INTAKE LANE (untriaged, disposition: new)")
	fmt.Fprintf(w, "%-24s %-6s %-6s %s\n", "ID", "AGE-D", "OVER?", "TITLE")
	for _, r := range rows {
		age := strconv.Itoa(r.Days)
		if r.BadDate {
			age = "?"
		}
		over := ""
		if r.Over {
			over = "OVER"
		}
		fmt.Fprintf(w, "%-24s %-6s %-6s %s\n", r.ID, age, over, trunc(r.Title, 70))
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no untriaged intake entries)")
	}
}

func cmdBoard(root string, now time.Time, slaDays int) (*Report, error) {
	issues, external, err := computeIssueBoard(root, now, slaDays)
	if err != nil {
		return nil, err
	}
	intake, err := loadIntakeRows(root, now)
	if err != nil {
		return nil, err
	}
	return &Report{
		detail: summarizeIssues(issues, external) + "; " + summarizeIntake(intake),
		render: func(w io.Writer) {
			renderIssueLane(w, issues)
			renderExternalLane(w, external)
			fmt.Fprintln(w)
			renderIntakeLane(w, intake)
		},
	}, nil
}

func cmdIssues(root string, now time.Time, slaDays int) (*Report, error) {
	issues, external, err := computeIssueBoard(root, now, slaDays)
	if err != nil {
		return nil, err
	}
	return &Report{
		detail: summarizeIssues(issues, external),
		render: func(w io.Writer) {
			renderIssueLane(w, issues)
			renderExternalLane(w, external)
		},
	}, nil
}

func cmdIntake(root string, now time.Time) (*Report, error) {
	intake, err := loadIntakeRows(root, now)
	if err != nil {
		return nil, err
	}
	return &Report{
		detail: summarizeIntake(intake),
		render: func(w io.Writer) { renderIntakeLane(w, intake) },
	}, nil
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

// shortRepo renders a repo slug as the short board label. It is a thin reading of
// the ONE shared deskkit resolver (deskkit.RepoShortLabel), the SAME mechanism
// deskboard prints through — issueboard no longer keeps its own private
// suffix→label switch (which had drifted to a strict subset of deskboard's). The
// generic default is the repo's last path segment; house short names live in the
// adopter's ASSAY_REPO_ALIASES config, never in this tree.
func shortRepo(s string) string {
	return deskkit.RepoShortLabel(s)
}

func trunc(s string, n int) string {
	if len([]rune(s)) > n {
		r := []rune(s)
		return string(r[:n-1]) + "…"
	}
	return s
}

// inert renders public-origin text (titles, author strings) as quoted terminal-safe
// data — control characters, ANSI escapes, and quotes are escaped — so quarantine
// output is DATA, never an instruction channel.
func inert(s string, n int) string {
	return strconv.Quote(trunc(s, n))
}

// title renders a public-origin title into an ACTIONABLE lane: ANSI escapes and control
// characters are stripped, then it is truncated. The quarantine lane keeps inert(),
// which quotes the payload so an injection attempt stays visible as evidence.
func title(s string, n int) string {
	return trunc(deskkit.StripControl(s), n)
}

// blessAuthorityName renders the configured blessing authority for a human-facing
// line. With the roster unconfigured there IS no authority, and the message says so
// rather than naming nobody — an unconfigured gate must read as refused, never as
// quiet.
func blessAuthorityName() string {
	if l := deskkit.BlessAuthorityLogin(); l != "" {
		return l
	}
	return "the (UNCONFIGURED) blessing authority"
}
