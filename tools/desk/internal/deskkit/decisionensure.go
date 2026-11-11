package deskkit

// decisionensure.go — the decision-issue gate's `ensure` verb, in Go.
//
// A `gate: human` brief waits on a human decision; `ensure` files ONE self-contained decision
// issue for it, unless the brief already has one. It is the protocol the repo-local helper
// `tools/decision-issue.sh` speaks, ported so a repo that does not carry that script still has
// a gate (cmd/deskdecision is its command; deskdispatch prefers it on PATH, exactly as it
// prefers deskclaim-ref over the claim script). Both implementations must agree on:
//
//   - the argument contract: ensure <brief-file> [--repo O/R] [--at creation|start|spec]
//     [--held] [--dry-run];
//   - the exit codes: 0 ok · 5 refused · 6 could-not-check (the port adds 4, rate-limited,
//     when its write budget refuses a filing — a state the helper has no budget to reach);
//   - the dedupe protocol: the brief's frontmatter `decision-issue: <N>` (READ, never written),
//     then a body marker `<!-- decision-gate: <id> -->`, then the legacy
//     `<!-- needs-decision: <id> -->` marker — under either spelling of the brief id, every
//     issue state — so a brief that ever had a decision issue never gets a second one;
//   - the composed issue: the same title, the same body, byte for byte, so either side's
//     filing dedupes against the other's.
//
// ORDER IS LOAD-BEARING. Every existing-record lookup runs before the body is composed and
// gated: a brief that already has an issue reports `exists:` and is never refused over a body
// nobody would file. A lookup that cannot be completed is could-not-check (6) and nothing is
// created — a duplicate gate issue is the expensive direction.
//
// The verb resolves the forge lazily, only once a filing could be needed: a brief that is not
// gated, already done, or not yet ripe never reads a credential.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DecisionEnsureForge is the forge surface `ensure` uses: reads for the dedupe sweeps, the
// label probe, and the two writes (file the issue, label it).
type DecisionEnsureForge interface {
	GetIssue(repo ForgeRepo, number int) (*Issue, error)
	ListIssues(repo ForgeRepo, in IssueListQuery) (*IssueList, error)
	SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error)
	ListLabels(repo ForgeRepo) ([]string, error)
	FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error)
	ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error)
}

// DecisionEnsureRequest is one `ensure` invocation.
type DecisionEnsureRequest struct {
	BriefFile string
	// Repo is the --repo value; empty means OriginRepo answers it.
	Repo string
	// OriginRepo reads the owner/name of the working directory's origin remote.
	OriginRepo func() (string, error)
	// At is the calling stage: creation, start or spec.
	At     string
	Held   bool
	DryRun bool
	// Forge resolves the forge serving the target repo. It is called at most once, and only
	// after every local check has passed.
	Forge func(repo ForgeRepo) (DecisionEnsureForge, error)
	// BeforeCreate, when set, runs once just before the issue is filed — never on a dry run
	// or any path that files nothing. Its error stops the filing with the error's own class:
	// refused (5), rate-limited (4: the write budget is spent; re-run later), else
	// could-not-check (6). The command charges its write budget here.
	BeforeCreate func(repo string) error
	Out          io.Writer
	Err          io.Writer
}

// DecisionEnsureResult is the verb's exit code and the leading word of its state line
// (not-gated, not-needed, deferred, exists, created, dry-run, refused, unverifiable), plus
// the target repo once known and the issue an exists or created outcome names.
type DecisionEnsureResult struct {
	Code   int
	State  string
	Repo   string
	Number int
	URL    string
	// Attempted is true once the create call was made: an unverifiable outcome after it may
	// have reached the forge; one before it provably wrote nothing.
	Attempted bool
}

const (
	decisionLogPrefix   = "decision-issue: "
	decisionLabelColor  = "d93f0b"
	decisionLabelPurpos = "a human decision is needed"
)

var decisionStageRank = map[string]int{"creation": 0, "start": 1, "spec": 2}

// EnsureDecisionIssue runs `ensure`. Every outcome is one state line: stdout for the ok
// outcomes, stderr for refusals and could-not-checks.
func EnsureDecisionIssue(req DecisionEnsureRequest) DecisionEnsureResult {
	e := &decisionEnsure{req: req}
	r := e.run()
	r.Repo = e.repo
	return r
}

type decisionEnsure struct {
	req  DecisionEnsureRequest
	repo string
}

func (e *decisionEnsure) log(format string, a ...any) {
	fmt.Fprintf(e.req.Out, decisionLogPrefix+format+"\n", a...)
}

func (e *decisionEnsure) err(format string, a ...any) {
	fmt.Fprintf(e.req.Err, decisionLogPrefix+format+"\n", a...)
}

func (e *decisionEnsure) ok(state, format string, a ...any) DecisionEnsureResult {
	e.log(format, a...)
	return DecisionEnsureResult{Code: ExitOK, State: state}
}

func (e *decisionEnsure) refused(format string, a ...any) DecisionEnsureResult {
	e.err("refused: "+format, a...)
	return DecisionEnsureResult{Code: ExitRefused, State: "refused"}
}

func (e *decisionEnsure) unverifiable(format string, a ...any) DecisionEnsureResult {
	e.err("unverifiable: "+format, a...)
	return DecisionEnsureResult{Code: ExitUnverifiable, State: "unverifiable"}
}

// unverifiableCause is unverifiable plus one line naming the underlying error, so a
// could-not-check says which read failed and why.
func (e *decisionEnsure) unverifiableCause(cause error, format string, a ...any) DecisionEnsureResult {
	r := e.unverifiable(format, a...)
	if cause != nil {
		e.err("cause: %s", StripControl(cause.Error()))
	}
	return r
}

func (e *decisionEnsure) run() DecisionEnsureResult {
	req := e.req
	at := req.At
	if _, ok := decisionStageRank[at]; !ok {
		return e.refused("--at must be creation|start|spec (got '%s')", at)
	}
	if fi, err := os.Stat(req.BriefFile); err != nil || !fi.Mode().IsRegular() {
		return e.refused("brief file not found: %s", req.BriefFile)
	}
	raw, err := os.ReadFile(req.BriefFile)
	if err != nil {
		return e.refused("brief file not found: %s", req.BriefFile)
	}
	lines := decisionFileLines(string(raw))
	fm := decisionFrontmatter(lines)
	if strings.TrimRight(strings.Join(fm, "\n"), "\n") == "" {
		return e.refused("%s has no frontmatter block — not a brief-v1 file", req.BriefFile)
	}
	briefID := decisionFMScalar(fm, "brief")
	title := decisionFMScalar(fm, "title")
	gate := decisionFMScalar(fm, "gate")
	trigger := decisionFMScalar(fm, "decision-trigger")
	if briefID == "" {
		return e.refused("frontmatter has no brief: id")
	}
	if gate != "human" {
		shown := gate
		if shown == "" {
			shown = "<absent>"
		}
		return e.ok("not-gated", "not-gated: %s gate=%s — no decision issue applies", briefID, shown)
	}
	if !req.Held && decisionBoardRowDone(req.BriefFile) {
		return e.ok("not-needed", "not-needed: %s is already done on the board (if the decision-gate hold refused a change to it, re-run with --held)", briefID)
	}
	if trigger == "" {
		trigger = "start"
	}
	if _, ok := decisionStageRank[trigger]; !ok {
		return e.refused("%s decision-trigger must be creation|start|spec (got '%s')", briefID, trigger)
	}

	block := decisionHumanBlock(lines)
	if trigger == "spec" {
		placeholder := block != "" && decisionBlockIsPlaceholder(block)
		if block == "" || placeholder {
			if at == "spec" {
				if placeholder {
					return e.refused("%s decision-trigger=spec but its '## Human decision' section is still only a placeholder (an HTML comment or a 'Decision-trigger: spec' marker line) — the spec step must author it (context, numbered options, a 'Default if no answer:' line) before this stage can file (that authored section IS what ripens a spec-trigger brief)", briefID)
				}
				return e.refused("%s decision-trigger=spec but the brief has no '## Human decision' section — the spec step must author it before this stage can file (that authored section IS what ripens a spec-trigger brief)", briefID)
			}
			if placeholder {
				return e.ok("deferred", "deferred: %s decision-trigger=spec — the '## Human decision' section is only a placeholder, not yet authored; the executor authors it at the pickup design/plan step and commits it into the brief, then re-run with --at spec", briefID)
			}
			return e.ok("deferred", "deferred: %s decision-trigger=spec — Human decision section not yet authored; the pickup design/plan step authors it, then re-run with --at spec", briefID)
		}
	} else if decisionStageRank[at] < decisionStageRank[trigger] {
		return e.ok("deferred", "deferred: %s decision-trigger=%s — the %s stage owns the filing (this run is --at %s)", briefID, trigger, trigger, at)
	}

	// --- dedupe: every existing-record lookup runs before the body is composed ---
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		var oerr error
		if req.OriginRepo != nil {
			repo, oerr = req.OriginRepo()
		} else {
			oerr = errors.New("no origin reader")
		}
		if oerr != nil || strings.TrimSpace(repo) == "" {
			return e.unverifiable("no --repo given and the cwd has no origin remote")
		}
	}
	owner, name, okRepo := strings.Cut(repo, "/")
	if !okRepo || owner == "" || name == "" {
		return e.refused("--repo must be owner/name (got '%s')", repo)
	}
	fr := ForgeRepo{Owner: owner, Name: name}
	e.repo = repo

	fmIssue := decisionFMScalar(fm, "decision-issue")
	var f DecisionEnsureForge
	forge := func() (DecisionEnsureForge, error) {
		if f != nil {
			return f, nil
		}
		if req.Forge == nil {
			return nil, Unverifiable("no forge resolver", nil)
		}
		got, ferr := req.Forge(fr)
		if ferr != nil {
			return nil, ferr
		}
		if got == nil {
			return nil, Unverifiable("the forge resolver returned no forge", nil)
		}
		f = got
		return f, nil
	}
	if fmIssue != "" {
		num := strings.TrimPrefix(fmIssue, "#")
		if !decisionDigitsRe.MatchString(num) {
			return e.refused("%s frontmatter decision-issue: '%s' is not an issue number — set it to the bare number of the decision issue (or remove it) so ensure does not have to guess whether one exists", briefID, fmIssue)
		}
		n, _ := strconv.Atoi(num)
		var item *Issue
		if g, ferr := forge(); ferr == nil {
			item, err = g.GetIssue(fr, n)
		} else {
			err = ferr
		}
		if err != nil || item == nil || item.Number != n {
			return e.unverifiableCause(err, "%s frontmatter names decision-issue #%s, which could not be read on %s — refusing to create (a duplicate gate issue is the expensive direction); retry next cycle", briefID, num, repo)
		}
		r := e.ok("exists", "exists: #%s (%s) — %s — named by the brief's frontmatter decision-issue: %s — a brief that ever had a decision issue never gets a second one", num, decisionStateDisplay(item), decisionIssueURL(item, repo, n), num)
		r.Number, r.URL = n, decisionIssueURL(item, repo, n)
		return r
	}

	g, ferr := forge()
	if ferr != nil {
		return e.unverifiableCause(ferr, "could not sweep %s for existing decision issues — refusing to create (a duplicate gate issue is the expensive direction); retry next cycle", repo)
	}
	cur := normalizeDecisionBriefID(briefID)
	alt := otherDecisionBriefSpelling(briefID)
	var bodies []decisionCandidate

	// Sweep 1: the labelled population — the cheap read that also covers a fresh filing the
	// search index has not caught up with. Its failure is tolerated: the label may not exist
	// yet, and the body searches below cover that case.
	bodies = append(bodies, decisionLabelSweep(g, fr, cur)...)

	// Sweep 2: the current marker, under each spelling the id has.
	hits, serr := g.SearchIssues(fr, SearchIssuesInput{DecisionGateMarkers: true, DecisionGateBrief: briefID})
	if serr != nil {
		return e.unverifiableCause(serr, "could not sweep %s for existing decision issues — refusing to create (a duplicate gate issue is the expensive direction); retry next cycle", repo)
	}
	bodies = append(bodies, decisionSearchCandidates(hits)...)
	if alt != "" {
		hits, serr = g.SearchIssues(fr, SearchIssuesInput{DecisionGateMarkers: true, DecisionGateBrief: alt})
		if serr != nil {
			return e.unverifiableCause(serr, "could not sweep %s for existing decision issues under the legacy spelling %s — refusing to create; retry next cycle", repo, alt)
		}
		bodies = append(bodies, decisionSearchCandidates(hits)...)
	}
	hit, found := decisionDedupeMatch(bodies, cur)

	// Sweep 3: the legacy marker, searched only when no current marker matched.
	if !found {
		var legacy []decisionCandidate
		hits, serr = g.SearchIssues(fr, SearchIssuesInput{DecisionGateMarkers: true, DecisionGateLegacy: true, DecisionGateBrief: briefID})
		if serr != nil {
			return e.unverifiableCause(serr, "could not sweep %s for legacy %s: decision issues — refusing to create; retry next cycle", repo, DecisionLegacyMarkerPrefix)
		}
		legacy = append(legacy, decisionSearchCandidates(hits)...)
		if alt != "" {
			hits, serr = g.SearchIssues(fr, SearchIssuesInput{DecisionGateMarkers: true, DecisionGateLegacy: true, DecisionGateBrief: alt})
			if serr != nil {
				return e.unverifiableCause(serr, "could not sweep %s for legacy %s: decision issues under the spelling %s — refusing to create; retry next cycle", repo, DecisionLegacyMarkerPrefix, alt)
			}
			legacy = append(legacy, decisionSearchCandidates(hits)...)
		}
		hit, found = decisionDedupeMatch(legacy, cur)
	}
	if found {
		item, gerr := g.GetIssue(fr, hit.number)
		if gerr != nil || item == nil || item.Number != hit.number {
			return e.unverifiableCause(gerr, "candidate decision issue #%d for %s could not be read on %s — refusing to create; retry next cycle", hit.number, briefID, repo)
		}
		r := e.ok("exists", "exists: #%d (%s) — %s — matched marker %s: %s — a brief that ever had a decision issue never gets a second one", hit.number, decisionStateDisplay(item), decisionIssueURL(item, repo, hit.number), hit.kind, hit.id)
		r.Number, r.URL = hit.number, decisionIssueURL(item, repo, hit.number)
		return r
	}

	// --- compose ---
	issueTitle := "Decision: " + briefID + " — " + title
	var core string
	if block != "" {
		if !decisionHasParseableDefault(block) {
			return e.refused("%s has a '## Human decision' section with no parseable default — every author-written section must carry either 'Default if no answer: <text> after <YYYY-MM-DD>' or the literal 'Default if no answer: none — blocks until answered' (a default written by the tool on the author's behalf is exactly what this forbids)", briefID)
		}
		core = block
	} else {
		e.err("NOTICE: no author default — blocks (no '## Human decision' section; falling back to the synthesized body, which the tool composes with its own 'none' default — an author-written one is preferred)")
		core = decisionSynthesizedCore(fm)
	}
	boardID := alt
	if boardID == "" {
		boardID = briefID
	}
	body := decisionIssueBody(briefID, title, boardID, core)

	// --- self-containment lint: the reader sees only this body ---
	if bad := decisionGrepN(body, decisionLinkRe); len(bad) > 0 {
		r := e.refused("composed body is not self-contained — it carries a markdown link or URL. The reader must be able to decide from the issue alone; inline the content instead (fix the brief's '## Human decision' section or its gate-why/why). Offending lines:")
		for _, l := range bad {
			fmt.Fprintln(e.req.Err, l)
		}
		return r
	}
	if paths := decisionGrepN(body, decisionPathRe); len(paths) > 0 {
		e.err("NOTICE: body mentions repo-relative paths — fine as prose about what the work touches, but the reader must not NEED to open them:")
		for _, l := range paths {
			fmt.Fprintln(e.req.Err, l)
		}
	}

	if req.DryRun {
		e.log("dry-run: would file on %s as:", repo)
		fmt.Fprintf(e.req.Out, "--- title ---\n%s\n--- body ---\n%s\n--- end ---\n", issueTitle, body)
		e.log("dry-run: no existing decision issue for %s on %s — a real run would create one", briefID, repo)
		return DecisionEnsureResult{Code: ExitOK, State: "dry-run"}
	}

	// --- create ---
	if req.BeforeCreate != nil {
		if berr := req.BeforeCreate(repo); berr != nil {
			switch ExitCodeOf(berr) {
			case ExitRefused:
				return e.refused("issue create was not attempted on %s: %s", repo, StripControl(berr.Error()))
			case ExitRateLimited:
				// The write budget is spent: wait, then re-run. Nothing was filed.
				e.err("rate-limited: issue create was not attempted on %s: %s", repo, StripControl(berr.Error()))
				return DecisionEnsureResult{Code: ExitRateLimited, State: "rate-limited"}
			}
			return e.unverifiable("issue create was not attempted on %s: %s", repo, StripControl(berr.Error()))
		}
	}
	// The label is a routing annotation; a repo without it must not veto the gate, so the
	// filing degrades to unlabeled with a notice rather than creating the label unasked.
	haveLabel := false
	if labels, lerr := g.ListLabels(fr); lerr == nil {
		haveLabel = hasLabel(labels, DecisionLabel)
	}
	if !haveLabel {
		e.err("NOTICE: label '%s' does not exist on %s — filing UNLABELED (a reader of the %s queue will not see it); create the label on the repo to route later filings", DecisionLabel, repo, DecisionLabel)
	}
	ref, cerr := g.FileIssue(fr, IssueInput{Title: issueTitle, Body: body + "\n"})
	if cerr != nil || ref == nil || ref.Number <= 0 {
		if cerr == nil {
			cerr = errors.New("the forge returned no issue")
		}
		if ExitCodeOf(cerr) == ExitRefused {
			return e.refused("issue create was refused on %s: %s", repo, StripControl(cerr.Error()))
		}
		r := e.unverifiable("issue create failed on %s: %s", repo, StripControl(cerr.Error()))
		r.Attempted = true
		return r
	}
	url := StripControl(ref.URL)
	if url == "" {
		url = fmt.Sprintf("%s#%d", repo, ref.Number)
	}
	if haveLabel {
		if _, aerr := g.ApplyLabels(fr, ref.Number, LabelChange{
			Target: TargetIssue,
			Add:    []LabelSpec{{Name: DecisionLabel, Color: decisionLabelColor, Description: decisionLabelPurpos}},
		}); aerr != nil {
			e.err("NOTICE: the issue was filed but the '%s' label could not be applied to it (%s) — label it by hand", DecisionLabel, StripControl(aerr.Error()))
		}
	}
	r := e.ok("created", "decision-issue: %s created for %s (trigger=%s, filed at %s)", url, briefID, trigger, at)
	r.Number, r.URL, r.Attempted = ref.Number, url, true
	return r
}

var (
	decisionDigitsRe  = regexp.MustCompile(`^[0-9]+$`)
	decisionLinkRe    = regexp.MustCompile(`\]\(|https?://`)
	decisionPathRe    = regexp.MustCompile(`docs/streams/|\.\./`)
	decisionDatedRe   = regexp.MustCompile(`Default if no answer:.*after[ \t\n\v\f\r]+[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	decisionMarkerCap = regexp.MustCompile(`decision-gate:\s*([^\s>]+)`)
	decisionLegacyCap = regexp.MustCompile(`<!--\s*needs-decision:\s*([^\s>]+)`)
	decisionSpecLine  = regexp.MustCompile(`(?i)^[ \t\n\v\f\r]*decision-trigger:[ \t\n\v\f\r]*spec([^[:alnum:]_-]|$)`)
)

const decisionBlocksDefault = "Default if no answer: none — blocks until answered"

func decisionIsSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

func decisionTrimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && decisionIsSpace(s[i]) {
		i++
	}
	for j > i && decisionIsSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

func decisionBlank(s string) bool { return decisionTrimSpace(s) == "" }

// decisionFileLines splits text into lines the way a line-oriented reader sees them: a
// trailing newline ends the last line rather than starting an empty one.
func decisionFileLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// decisionFrontmatter is the block between a first line that is exactly `---` and the next
// line that is `---` plus optional trailing whitespace (or the end of the file).
func decisionFrontmatter(lines []string) []string {
	if len(lines) == 0 || lines[0] != "---" {
		return nil
	}
	var out []string
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "---") && decisionBlank(l[3:]) {
			break
		}
		out = append(out, l)
	}
	return out
}

// decisionFMScalar reads one frontmatter key: the inline value (a trailing ` # comment`
// dropped; a bare block indicator `>-`, `>`, `|`, `|-` read as empty), joined with each
// following more-indented non-blank line; the LAST occurrence of the key wins; one leading
// and one trailing quote character are removed.
func decisionFMScalar(fm []string, key string) string {
	out := ""
	collecting := false
	for _, l := range fm {
		if collecting {
			if l != "" && decisionIsSpace(l[0]) && !decisionBlank(l) {
				line := strings.TrimLeft(l, " \t\n\v\f\r")
				if out == "" {
					out = line
				} else {
					out += " " + line
				}
				continue
			}
			collecting = false
		}
		if !strings.HasPrefix(l, key+":") {
			continue
		}
		val := strings.TrimLeft(l[len(key)+1:], " \t\n\v\f\r")
		val = decisionStripComment(val)
		switch val {
		case ">-", ">", "|", "|-":
			val = ""
		}
		out = val
		collecting = true
	}
	if out != "" && (out[0] == '"' || out[0] == '\'') {
		out = out[1:]
	}
	if out != "" && (out[len(out)-1] == '"' || out[len(out)-1] == '\'') {
		out = out[:len(out)-1]
	}
	return out
}

// decisionStripComment drops the first run of whitespace followed by `#`, and the rest.
func decisionStripComment(v string) string {
	for i := 1; i < len(v); i++ {
		if v[i] == '#' && decisionIsSpace(v[i-1]) {
			j := i - 1
			for j > 0 && decisionIsSpace(v[j-1]) {
				j--
			}
			return v[:j]
		}
	}
	return v
}

// decisionBoardRowDone reports whether the stream README beside the brief has a row linking
// to it whose status cell (the sixth `|` field) reads exactly `done`. A missing README or row
// is not done.
func decisionBoardRowDone(briefFile string) bool {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(briefFile), "README.md"))
	if err != nil {
		return false
	}
	needle := "](" + filepath.Base(briefFile) + ")"
	for _, row := range decisionFileLines(string(raw)) {
		if !strings.Contains(row, needle) {
			continue
		}
		fields := strings.SplitN(row, "|", 7)
		return len(fields) > 5 && decisionTrimSpace(fields[5]) == "done"
	}
	return false
}

// decisionHumanBlock is the text after the first `## Human decision` heading up to the next
// `## ` heading, with leading blank lines and trailing whitespace-only lines removed.
func decisionHumanBlock(lines []string) string {
	var blk []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## Human decision") {
			in = true
			continue
		}
		if in && strings.HasPrefix(l, "## ") {
			break
		}
		if in {
			blk = append(blk, l)
		}
	}
	start := 0
	for start < len(blk) && strings.Trim(blk[start], " \t") == "" {
		start++
	}
	blk = blk[start:]
	end := len(blk)
	for end > 0 && decisionBlank(blk[end-1]) {
		end--
	}
	return strings.Join(blk[:end], "\n")
}

// decisionBlockIsPlaceholder: a section with nothing but closed HTML comments and blank
// lines, or exactly one other line that opens with a `Decision-trigger: spec` marker and
// carries no default line. An unclosed comment is authored content.
func decisionBlockIsPlaceholder(block string) bool {
	s := block + "\n"
	var b strings.Builder
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			break
		}
		b.WriteString(s[:i])
		rest := s[i+4:]
		j := strings.Index(rest, "-->")
		if j < 0 {
			return false
		}
		s = rest[j+3:]
	}
	b.WriteString(s)
	var residue []string
	for _, l := range strings.Split(b.String(), "\n") {
		if !decisionBlank(l) {
			residue = append(residue, l)
		}
	}
	if len(residue) == 0 {
		return true
	}
	if len(residue) != 1 || strings.Contains(residue[0], "Default if no answer:") {
		return false
	}
	return decisionSpecLine.MatchString(residue[0])
}

func decisionHasParseableDefault(block string) bool {
	for _, l := range strings.Split(block, "\n") {
		if decisionDatedRe.MatchString(l) || strings.Contains(l, decisionBlocksDefault) {
			return true
		}
	}
	return false
}

func decisionOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// decisionSynthesizedCore is the body for a brief with no `## Human decision` section,
// derived from its own frontmatter (gate-why, why, risk, effort) plus the standard options.
func decisionSynthesizedCore(fm []string) string {
	core := "## Context\n\n" +
		decisionOr(decisionFMScalar(fm, "gate-why"), "(the brief records no gate-why — the risk rationale below is all the automation could derive; ask the dispatcher for a proper Human decision section if this is not enough to decide on)") + "\n\n" +
		decisionFMScalar(fm, "why") + "\n\n" +
		"Risk answers: " + decisionOr(decisionFMScalar(fm, "risk"), "(not recorded)") + ". Effort: " + decisionOr(decisionFMScalar(fm, "effort"), "?") + ".\n\n" +
		"## Options\n\n" +
		"1. **Approve as briefed** — the work proceeds and lands exactly as described above; your approval is the human gate it is waiting on.\n" +
		"2. **Approve with changes** — state the changes as a comment on this issue; the work is re-dispatched with them folded in before it lands.\n" +
		"3. **Hold / reject** — state why as a comment; the item is parked and cannot advance past its human gate.\n\n" +
		"Default if no answer: none — the gate blocks until answered."
	return core
}

// decisionIssueBody is the filed body, without its final newline: the marker, the reply
// instruction naming the board id, and the decision core.
func decisionIssueBody(briefID, title, boardID, core string) string {
	body := "<!-- " + DecisionGateMarkerPrefix + ": " + briefID + " -->\n\n" +
		"A human decision is needed on work item `" + briefID + "` — **" + title + "**. Everything needed to\n" +
		"decide is in this issue.\n\n" +
		"How to reply: comment from your own account with the board id of this item and the chosen option,\n" +
		"as `" + boardID + ": <option>` (for example `" + boardID + ": 1`); the comment is the decision\n" +
		"record. Name the board id in your own words — a bare option number or a one-word \"approved\"\n" +
		"is not accepted as a ruling when the item later moves on the board. Then close the issue.\n\n" +
		core
	return strings.TrimRight(body, "\n")
}

// decisionGrepN returns `N:line` for each body line re matches, N counted from 1.
func decisionGrepN(body string, re *regexp.Regexp) []string {
	var out []string
	for i, l := range strings.Split(body, "\n") {
		if re.MatchString(l) {
			out = append(out, fmt.Sprintf("%d:%s", i+1, l))
		}
	}
	return out
}

func decisionStateDisplay(item *Issue) string {
	s := strings.ToUpper(strings.TrimSpace(item.State))
	for _, l := range item.Labels {
		if l == DecidedLabel {
			return s + ", " + DecidedLabel
		}
	}
	return s
}

func decisionIssueURL(item *Issue, repo string, n int) string {
	if item != nil && strings.TrimSpace(item.URL) != "" {
		return StripControl(item.URL)
	}
	return fmt.Sprintf("%s#%d", repo, n)
}

// decisionCandidate is one issue body a sweep returned.
type decisionCandidate struct {
	number int
	body   string
}

// decisionMarkerHit is the matched issue and the marker that matched it.
type decisionMarkerHit struct {
	number int
	kind   string
	id     string
}

func decisionSearchCandidates(hits []IssueSearchResult) []decisionCandidate {
	out := make([]decisionCandidate, 0, len(hits))
	for _, h := range hits {
		out = append(out, decisionCandidate{number: h.Number, body: h.Body})
	}
	return out
}

// decisionLabelSweep reads the labelled population. A label listing carries no bodies, so
// only issues titled as a decision issue for this brief (`Decision: <id> — `) are read for
// their marker; a read that fails is skipped, as the whole sweep's failure is.
func decisionLabelSweep(g DecisionEnsureForge, fr ForgeRepo, cur string) []decisionCandidate {
	list, err := g.ListIssues(fr, IssueListQuery{State: IssueStateAll, Label: DecisionLabel})
	if err != nil || list == nil {
		return nil
	}
	var out []decisionCandidate
	for _, s := range list.Issues {
		rest, ok := strings.CutPrefix(s.Title, "Decision: ")
		if !ok {
			continue
		}
		id, _, ok := strings.Cut(rest, " — ")
		if !ok || normalizeDecisionBriefID(strings.TrimSpace(id)) != cur {
			continue
		}
		item, gerr := g.GetIssue(fr, s.Number)
		if gerr != nil || item == nil {
			continue
		}
		out = append(out, decisionCandidate{number: s.Number, body: item.Body})
	}
	return out
}

// decisionDedupeMatch takes, per body, the first `decision-gate:` marker and the first
// `<!-- needs-decision:` marker; an issue matches when either names the current brief under
// either spelling. The lowest-numbered match wins; within one issue the current marker is
// reported before the legacy one.
func decisionDedupeMatch(cands []decisionCandidate, cur string) (decisionMarkerHit, bool) {
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].number < cands[j].number })
	for _, c := range cands {
		if m := decisionMarkerCap.FindStringSubmatch(c.body); m != nil && normalizeDecisionBriefID(m[1]) == cur {
			return decisionMarkerHit{number: c.number, kind: DecisionGateMarkerPrefix, id: m[1]}, true
		}
		if m := decisionLegacyCap.FindStringSubmatch(c.body); m != nil && normalizeDecisionBriefID(m[1]) == cur {
			return decisionMarkerHit{number: c.number, kind: DecisionLegacyMarkerPrefix, id: m[1]}, true
		}
	}
	return decisionMarkerHit{}, false
}
