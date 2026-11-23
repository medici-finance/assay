package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// A sanctioned editorial repair of historical Evidence is its own entry in the
// acceptance chain (#6616):
//
//	Evidence-Repair: commit=<full-sha> approval=<issue-or-PR-comment URL>
//
// The byte-prefix check stays strict. A repair only moves the baseline that
// check compares against, from the pre-amendment Evidence to the Evidence at
// the repair commit, and only after the repair is shown to be editorial: it
// leaves Verify untouched, keeps every table row and every non-Date/Runner
// cell byte-identical, splits a combined "Date / Runner" column in every row of
// its table exactly after the leading date or adds one
// `(on-behalf-of human:<name>)` principal to a Runner, and adds only prose that
// carries no marker, verdict, comment, fence, HTML or link-reference definition
// and leaves every reader's whole-document view unchanged. The driver's own unedited comment, read
// through the same forge resolver as Acceptance-Transition approvals, must name
// the brief and its own issue or pull request, and carry a sentence saying
// "I authorize" with no negating or conditional word, question mark or markup
// in it (repairAuthorizes); the repair commit's message must name that issue
// or pull request.
type evidenceRepair struct {
	commit, approval string
	link             rulingLink
}

// evidenceRepairURLRe accepts an issue or pull-request comment. A pull
// request's conversation comments are issue comments on the same number.
var evidenceRepairURLRe = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(issues|pull)/([1-9][0-9]*)#issuecomment-([1-9][0-9]*)$`)

var evidenceRepairPrincipalRe = regexp.MustCompile(`^ \(on-behalf-of human:[0-9A-Za-z_]+\)$`)

var evidenceRepairBriefRe = regexp.MustCompile(`^brief-([0-9]+[a-z]?)-`)

// evidenceRepairMarkers are the protected-record markers a repair may not add,
// checked on the raw added lines and on the readers' normalized view.
var evidenceRepairMarkers = []string{"Verification-Table:", "Acceptance-Transition:", "Evidence-Repair:", "Verification-Attestation:"}

// evidenceRepairs validates the descriptor grammar before any history read.
func evidenceRepairs(run string) ([]evidenceRepair, string) {
	var repairs []evidenceRepair
	seen := map[string]bool{}
	for _, m := range protectedMarkersNamed(normalizedProtectedEvidence(run), "Evidence-Repair:") {
		f := strings.Fields(m.text)
		if !m.literal || len(f) != 3 || !strings.HasPrefix(f[1], "commit=") || !strings.HasPrefix(f[2], "approval=") {
			return nil, "evidence repair: expected Evidence-Repair: commit=<full-sha> approval=<comment-url>"
		}
		c, u := strings.TrimPrefix(f[1], "commit="), strings.TrimPrefix(f[2], "approval=")
		if !isObjectID(c) {
			return nil, "evidence repair: a full immutable repair commit is required"
		}
		p := evidenceRepairURLRe.FindStringSubmatch(u)
		if p == nil {
			return nil, "evidence repair: a valid issue or pull-request comment approval URL is required"
		}
		n, e1 := strconv.Atoi(p[4])
		id, e2 := strconv.ParseInt(p[5], 10, 64)
		if e1 != nil || e2 != nil {
			return nil, "evidence repair: a valid issue or pull-request comment approval URL is required"
		}
		link := rulingLink{Repo: p[1] + "/" + p[2], Issue: n, CommentID: id}
		if seen[c] || seen[fmt.Sprint(id)] {
			return nil, "evidence repair: a repair commit or approval cannot appear twice"
		}
		seen[c], seen[fmt.Sprint(id)] = true, true
		repairs = append(repairs, evidenceRepair{commit: c, approval: u, link: link})
	}
	return repairs, ""
}

// evidenceRepairBaseline returns the Evidence the current record must retain as
// its byte prefix, and the repairs that moved it. historical is the Evidence at
// predecessor (the amendment's first parent). first requires every repair to
// follow predecessor, so no repair can sit unused before the chain.
func evidenceRepairBaseline(path, historical, run, predecessor string, first bool, read func(string) (string, error)) (string, []evidenceRepair, string) {
	repairs, why := evidenceRepairs(run)
	if why != "" || len(repairs) == 0 {
		return historical, nil, why
	}
	dir, file := filepath.Dir(path), filepath.Base(path)
	order, e := transitionFirstParents(dir, predecessor)
	if e != nil {
		return "", nil, "evidence repair: cannot read repair ancestry"
	}
	chain := strings.Fields(string(order))
	position := map[string]int{}
	for i, id := range chain {
		position[id] = i
	}
	byCommit := map[string]evidenceRepair{}
	var applied []string
	for _, r := range repairs {
		if _, ok := position[r.commit]; !ok {
			if first {
				return "", nil, fmt.Sprintf("evidence repair: %.12s is not after the amendment's parent on the observed first-parent chain", r.commit)
			}
			continue
		}
		byCommit[r.commit] = r
		applied = append(applied, r.commit)
	}
	// Apply in first-parent order, never descriptor order.
	for i := 1; i < len(applied); i++ {
		for j := i; j > 0 && position[applied[j]] < position[applied[j-1]]; j-- {
			applied[j], applied[j-1] = applied[j-1], applied[j]
		}
	}
	baseline := historical
	var used []evidenceRepair
	for _, commit := range applied {
		short := fmt.Sprintf("evidence repair %.12s: ", commit)
		// The chain is the first-parent walk from the amendment (chain[0]) to
		// HEAD, so the entry before a repair is its first parent. Reading it
		// from the walk already made adds no native Git read.
		at := position[commit]
		if at == 0 {
			return "", nil, short + "a repair cannot be the amendment commit itself"
		}
		parent := chain[at-1]
		before, e1 := flipEvidenceAt(dir, parent, file)
		after, e2 := flipEvidenceAt(dir, commit, file)
		if e1 != nil || e2 != nil || before == flipAbsent || after == flipAbsent {
			return "", nil, short + "cannot read Evidence around the repair"
		}
		if !strings.HasPrefix(strings.TrimSpace(before), strings.TrimSpace(baseline)) {
			return "", nil, short + "historical Evidence was not preserved before the repair"
		}
		verifyBefore, e1 := read(parent)
		verifyAfter, e2 := read(commit)
		if e1 != nil || e2 != nil || verifyBefore != verifyAfter {
			return "", nil, short + "a repair cannot touch any Verify row"
		}
		if before == after {
			return "", nil, short + "the repair does not change Evidence"
		}
		if why := evidenceRepairEditorial(before, after); why != "" {
			return "", nil, short + why
		}
		msg, e := evidenceRepairMessage(dir, commit)
		if e != nil {
			return "", nil, short + "could-not-check the repair commit message: " + e.Error()
		}
		if !commentNamesRecord(msg, fmt.Sprintf("#%d", byCommit[commit].link.Issue)) {
			return "", nil, short + "the repair commit does not name its approval's issue or pull request"
		}
		baseline = after
		used = append(used, byCommit[commit])
	}
	return baseline, used, ""
}

// evidenceRepairMessage reads a repair commit's full message (git's %B) in
// process. It adds no native Git launch to the statusgen inventory: where the
// in-process reader positively selects native Git, the read fails closed and
// the repair is could-not-check, never admitted.
func evidenceRepairMessage(dir, commit string) (string, error) {
	read := gitCoreForCommits(dir, commit)
	if read.err != nil {
		return "", read.err
	}
	if read.native != "" {
		return "", fmt.Errorf("the in-process reader requires native Git here (%s)", read.native)
	}
	return read.repo.CommitMessage(commit)
}

// evidenceRepairEditorial is the content rule. Table lines pair 1:1 in order;
// only the Date/Runner split and an added principal may differ. Every other
// prior line survives in order; an added line is prose with no marker or
// verdict.
func evidenceRepairEditorial(before, after string) string {
	// A row records whether it starts a table (the line before it is not a
	// row), so the column map resets at every table.
	type row struct {
		line  string
		start bool
	}
	collect := func(s string) (rows []row, text []string) {
		prev := false
		for _, l := range strings.Split(s, "\n") {
			isRow := strings.HasPrefix(strings.TrimSpace(l), "|")
			if isRow {
				rows = append(rows, row{l, !prev})
			} else {
				text = append(text, l)
			}
			prev = isRow
		}
		return rows, text
	}
	oldRows, oldText := collect(before)
	newRows, newText := collect(after)
	if len(oldRows) != len(newRows) {
		return "a repair cannot add or remove table rows"
	}
	// sepNext: the next row of the same table is a separator. The Evidence
	// reader (parseEvidenceRows) takes only a row followed by a separator as a
	// header, so a mid-table data row whose first cell is "#" is a data row
	// here too and cannot reset the column map.
	sepNext := func(i int) bool {
		return i+1 < len(oldRows) && !oldRows[i+1].start && isSeparatorCells(cellsTrimmed(oldRows[i+1].line))
	}
	var header []string
	split := false
	for i := range oldRows {
		if oldRows[i].start != newRows[i].start {
			return "a repair cannot split or join a table"
		}
		if oldRows[i].start {
			header, split = nil, false
		}
		o, n := cellsTrimmed(oldRows[i].line), cellsTrimmed(newRows[i].line)
		head := isHeaderCells(o) && sepNext(i)
		if oldRows[i].line == newRows[i].line {
			if head {
				header, split = o, false
				continue
			}
			// Under a split header every row is split: a header-only or
			// partial split leaves rows that disagree on the runner column.
			if split {
				return "a repair must split every row of a split table"
			}
			continue
		}
		if isSeparatorCells(o) {
			if !isSeparatorCells(n) || len(n) != len(o)+1 || !split {
				return "a repair changed a table separator"
			}
			continue
		}
		if head {
			at := repairColumn(o, "Date / Runner")
			if at < 0 || len(n) != len(o)+1 {
				return "a repair may only split the Date / Runner header"
			}
			want := append(append(append([]string{}, o[:at]...), "Date", "Runner"), o[at+1:]...)
			if strings.Join(want, "\x00") != strings.Join(n, "\x00") {
				return "a repair may only split the Date / Runner header"
			}
			header, split = o, true
			continue
		}
		if header == nil {
			return "a repair changed a row outside any table"
		}
		if split {
			at := repairColumn(header, "Date / Runner")
			if len(o) != len(header) || len(n) != len(o)+1 {
				return "a repair changed a row's shape"
			}
			for j := range o {
				if j == at {
					continue
				}
				k := j
				if j > at {
					k = j + 1
				}
				if o[j] != n[k] {
					return fmt.Sprintf("a repair changed the %s cell of row %s", header[j], o[0])
				}
			}
			if !splitAtDate(o[at], n[at], n[at+1]) {
				return fmt.Sprintf("a repair changed the Date / Runner text of row %s", o[0])
			}
			continue
		}
		runner := repairColumn(header, "Runner")
		if runner < 0 || len(o) != len(n) {
			return "a repair changed a row outside the Date/Runner split"
		}
		for j := range o {
			if j == runner {
				continue
			}
			if o[j] != n[j] {
				name := "unnamed"
				if j < len(header) {
					name = header[j]
				}
				return fmt.Sprintf("a repair changed the %s cell of row %s", name, o[0])
			}
		}
		if !principalOnly(o[runner], n[runner]) {
			return fmt.Sprintf("a repair changed the Runner text of row %s", o[0])
		}
	}
	// Prior prose survives in order; additions are commentary only.
	var added []string
	j := 0
	for _, l := range newText {
		if j < len(oldText) && l == oldText[j] {
			j++
			continue
		}
		added = append(added, l)
	}
	if j != len(oldText) {
		return "a repair removed or altered historical Evidence text"
	}
	for _, l := range added {
		for _, m := range evidenceRepairMarkers {
			if strings.Contains(l, m) {
				return "a repair cannot add a marker"
			}
		}
	}
	if len(flipVerdictTokens(added)) > 0 {
		return "a repair cannot add a verdict"
	}
	return evidenceRepairReaderView(before, after, added)
}

// evidenceRepairReaderView holds what the Evidence readers see, not only the
// raw lines: they strip closed HTML comments, track fences across the whole
// document and read a table as one run of rows. An added line may not open or
// close a comment or a fence, may not split a table, and the whole-document
// view of markers, verdicts and rows must be unchanged.
func evidenceRepairReaderView(before, after string, added []string) string {
	for _, l := range added {
		t := strings.TrimSpace(l)
		if strings.Contains(l, "<!--") || strings.Contains(l, "-->") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			return "a repair cannot add a comment or fence line"
		}
		// Any other HTML start: an unterminated processing instruction, CDATA
		// section or declaration hides the rest of the rendered page, and
		// <details> collapses it. A repair needs none of them.
		if evidenceRepairHTMLRe.MatchString(l) {
			return "a repair cannot add an HTML line"
		}
		// A link-reference or footnote definition is not rendered, and with a
		// title quote left open it swallows the prose after it on the page.
		if evidenceRepairDefRe.MatchString(l) {
			return "a repair cannot add a link-reference or footnote definition"
		}
	}
	if !sameRowRuns(before, after) {
		return "a repair cannot split or join a table"
	}
	nb, na := normalizeProtectedEvidence(before), normalizeProtectedEvidence(after)
	for _, m := range evidenceRepairMarkers {
		if len(protectedMarkersNamed(nb, m)) != len(protectedMarkersNamed(na, m)) {
			return "a repair cannot add a marker"
		}
	}
	sb, ub := stripRowComments(before)
	sa, ua := stripRowComments(after)
	if (ub < 0) != (ua < 0) {
		return "a repair cannot open an unterminated comment"
	}
	vb, va := flipVerdictTokens(strings.Split(sb, "\n")), flipVerdictTokens(strings.Split(sa, "\n"))
	if len(vb) != len(va) {
		return "a repair cannot add a verdict"
	}
	for i := range vb {
		if vb[i].word != va[i].word || vb[i].strict != va[i].strict || vb[i].struck != va[i].struck {
			return "a repair cannot add a verdict"
		}
	}
	// Ids cannot change (every non-Date/Runner cell is byte-identical), so a
	// lost row is the only way the reader's view of rows can differ.
	ra := parseEvidenceRows(after)
	for id, rows := range parseEvidenceRows(before) {
		if len(ra[id]) != len(rows) {
			return "a repair cannot hide an Evidence row"
		}
	}
	return ""
}

// sameRowRuns: the lengths of consecutive table-row runs are unchanged.
func sameRowRuns(before, after string) bool {
	runs := func(s string) []int {
		var out []int
		n := 0
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "|") {
				n++
				continue
			}
			if n > 0 {
				out = append(out, n)
			}
			n = 0
		}
		if n > 0 {
			out = append(out, n)
		}
		return out
	}
	return fmt.Sprint(runs(before)) == fmt.Sprint(runs(after))
}

// splitAtDate: the Date cell is exactly the old cell's leading date, and the
// Runner cell is the rest, unchanged or with one added principal.
func splitAtDate(old, date, runner string) bool {
	i := strings.IndexAny(old, " \t")
	if i <= 0 || date != old[:i] || !flipDateRe.MatchString(date) {
		return false
	}
	rest := strings.TrimSpace(old[i:])
	return rest != "" && principalOnly(rest, runner)
}

// principalOnly: unchanged, or exactly one added on-behalf-of principal.
func principalOnly(old, next string) bool {
	return next == old || (strings.HasPrefix(next, old) && evidenceRepairPrincipalRe.MatchString(next[len(old):]) && !strings.Contains(old, "(on-behalf-of "))
}

func cellsTrimmed(line string) []string {
	cells := briefdoc.SplitRowEscaped(line)
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isHeaderCells(c []string) bool { return len(c) > 1 && c[0] == "#" }

func isSeparatorCells(c []string) bool {
	if len(c) == 0 {
		return false
	}
	for _, s := range c {
		if strings.Trim(s, ":-") != "" || s == "" {
			return false
		}
	}
	return true
}

func repairColumn(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// evidenceRepairBriefID is the stream/NN id a repair approval must name.
func evidenceRepairBriefID(rel string) string {
	m := evidenceRepairBriefRe.FindStringSubmatch(filepath.Base(rel))
	if m == nil {
		return ""
	}
	return filepath.Base(filepath.Dir(rel)) + "/" + m[1]
}

// corroborateEvidenceRepairs reads each used repair's approval through the
// Acceptance-Transition resolver: same repository, unedited, non-bot, authored
// by the driver, naming the brief and authorizing the repair on its thread. A pull-request comment is admitted only here.
func corroborateEvidenceRepairs(repo, rel string, repairs []evidenceRepair, client func() *ghClient) string {
	id := evidenceRepairBriefID(rel)
	if id == "" && len(repairs) > 0 {
		return "evidence repair: cannot derive the brief id an approval must name"
	}
	if len(repairs) == 0 {
		return ""
	}
	// Only the driver (the blessing authority) may approve a repair: any other
	// mapped human's comment is refused by the resolver's decided-by check.
	cfg := scanEffectiveConfig()
	driver := humanNameForLogin(cfg.HumanLogins, cfg.Bless.Login)
	if cfg.Bless.Login == "" || driver == "" {
		return "evidence repair: could-not-check human authority: the driver's login is unset or maps to no human"
	}
	for _, r := range repairs {
		fmt.Fprintln(os.Stderr, "acceptance transition: reading evidence-repair approval", r.approval)
		canonical := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%d", r.link.Repo, r.link.Issue, r.link.CommentID)
		thread := fmt.Sprintf("#%d", r.link.Issue)
		out := resolveRulingOn(client(), repo, decisionRecordStamp{ID: id, Ruling: canonical, Names: []string{driver}}, cfg.HumanLogins, rulingIssueBinding{
			what:      "evidence repair for " + rel,
			allowPull: true,
			// The thread binding is the comment rule below plus the commit
			// message naming the same issue or pull request; a repair
			// approval needs no decision-gate marker on that thread.
			rules: func(rulingIssue, rulingLink) bool { return true },
			comment: func(body string) string {
				if !repairNamesBrief(body, id) {
					return "does not name the brief " + id + " as its own path"
				}
				if !commentNamesRecord(body, thread) || !repairAuthorizes(body) {
					return "does not authorize a repair on " + thread + " (it must say it authorizes and name " + thread + ")"
				}
				return ""
			},
		})
		if !out.Present || out.Reason != rulingOK {
			if out.Reason == rulingUnresolvable {
				return fmt.Sprintf("evidence repair %.12s: could-not-check human authority: %s", r.commit, out.Detail)
			}
			return fmt.Sprintf("evidence repair %.12s: human approval could not be corroborated: %s", r.commit, out.Detail)
		}
	}
	return ""
}

// evidenceRepairHTMLRe is an HTML tag, comment, declaration or processing
// instruction start anywhere on a line.
var evidenceRepairHTMLRe = regexp.MustCompile(`<[A-Za-z/?!]`)

// evidenceRepairDefRe is a Markdown link-reference or footnote definition.
var evidenceRepairDefRe = regexp.MustCompile(`^[ \t]{0,3}\[[^\]]+\]:`)

// repairApprovalHedges are the words that negate or condition a sentence. A
// sentence that mentions authorizing and carries one of them does not approve.
var repairApprovalHedges = map[string]bool{
	"not": true, "no": true, "never": true, "nor": true, "cannot": true,
	"dont": true, "doesnt": true, "wont": true, "cant": true, "isnt": true,
	"unless": true, "if": true, "provided": true, "providing": true, "assuming": true,
	"once": true, "until": true, "when": true, "whenever": true, "after": true,
	"pending": true, "conditional": true, "conditionally": true, "subject": true,
	"would": true, "could": true, "should": true, "might": true, "whether": true,
	"nobody": true, "nothing": true, "none": true, "neither": true,
	"but": true, "however": true, "except": true, "without": true,
}

// repairRefusals are word stems that take an approval back. One anywhere in the
// comment refuses it, whichever sentence carries it.
var repairRefusals = []string{"reject", "refus", "declin", "revok", "rescind", "withdr", "retract", "deny", "denie", "veto"}

// repairPlainRe is plain ASCII prose: no strike, emphasis, character reference,
// tag, quote marker or look-alike letter can hide a word of a sentence that
// mentions authorizing from the reader of the rendered comment.
var repairPlainRe = regexp.MustCompile("^[A-Za-z0-9 \t\n.,:;'\"()#/!?-]*$")

// repairSentenceEndRe ends a sentence: a terminator before whitespace or the
// end. A paragraph break alone does not, so a dangling condition stays with
// the statement it conditions.
var repairSentenceEndRe = regexp.MustCompile(`[.!?]+(\s|$)`)

func repairSentences(body string) []string {
	var out []string
	from := 0
	for _, m := range repairSentenceEndRe.FindAllStringIndex(body, -1) {
		out = append(out, strings.TrimSpace(body[from:m[1]]))
		from = m[1]
	}
	if rest := strings.TrimSpace(body[from:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func repairWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	})
}

// repairAuthorizes: one sentence says "I authorize", in those two adjacent
// words. Every sentence that mentions authorizing (any word containing
// "authori") must be a plain declarative ASCII statement with no question
// mark and no negating or conditional word; other sentences may limit scope
// ("not a new verification claim"). A refusal word anywhere refuses.
func repairAuthorizes(body string) bool {
	norm := strings.NewReplacer("\u2019", "'", "`", "'").Replace(body)
	for _, w := range repairWords(norm) {
		for _, r := range repairRefusals {
			if strings.HasPrefix(w, r) {
				return false
			}
		}
	}
	said := false
	for _, s := range repairSentences(norm) {
		words := repairWords(s)
		mentions, first := false, false
		for i, w := range words {
			mentions = mentions || strings.Contains(w, "authori")
			first = first || (w == "authorize" && i > 0 && words[i-1] == "i")
		}
		if !mentions {
			continue
		}
		if repairHedged(s, words) {
			return false
		}
		said = said || first
	}
	return said
}

// repairHedged: the sentence is a question, is not plain prose, or carries a
// negating or conditional word.
func repairHedged(s string, words []string) bool {
	if strings.Contains(s, "?") {
		return true
	}
	if !repairPlainRe.MatchString(s) {
		return true
	}
	for _, w := range words {
		if repairApprovalHedges[w] || strings.HasSuffix(w, "n't") {
			return true
		}
	}
	return false
}

// repairNamesBrief is commentNamesRecord without path boundaries: the brief id
// cannot be the tail of a longer path such as another stream's directory.
func repairNamesBrief(text, id string) bool {
	if id == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(text[from:], id)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(id)
		before := start == 0 || !strings.ContainsRune("-_/.", rune(text[start-1])) && !isIDByte(text[start-1])
		after := end == len(text) || text[end] != '/' && text[end] != '-' && text[end] != '_' && !isIDByte(text[end])
		if before && after {
			return true
		}
		from = start + 1
	}
}

func isIDByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
