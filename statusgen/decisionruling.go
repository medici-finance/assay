package main

// decisionruling.go — the RULING-LINK lane of `statusgen --corroborate` for
// design-decision records (every record under docs/streams/decisions/, by
// convention DR-<slug>.md; schema decision-v1, registers-v1 §7.5).
//
// The problem it closes. A record's `decided-by: "human:<name>"` stamp used to be
// corroborated only against an approval by that human ON THE PR THAT ADDS THE
// RECORD. The human rules on the record's decision ISSUE, not on the PR, so either
// every record cost a second approval or the literal `human:<name>` placeholder was
// written — and humanStampRe does not match `<`, so --corroborate did not read the
// placeholder at all. On a public repository a real name cannot be written, so the
// placeholder was the only option there, and it was unchecked.
//
// The grammar (documented ONCE, in registers-v1 §7.5 and the register README):
//
//	decided-by: "human:<name>"      # a real mapped name, OR the literal placeholder
//	ruling: "https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<ID>"
//
// What this lane does, for every record the PR ADDS OR EDITS (any added line in
// the record file — the same diff scope the rest of --corroborate keeps):
//
//   - `ruling:` present → resolve the comment through the forge REST API and PASS
//     only when ALL hold: the link parses; it points into the PR's own repository;
//     the comment exists; the comment really sits on the issue the link names; it
//     was not edited after it was posted (updated_at == created_at); its author is
//     not a bot; its author's login maps through the human-login map
//     (ASSAY_HUMAN_LOGIN_MAP) to a human — and, when decided-by names a real human,
//     to THAT human; the comment's own text names the record id (DR-<slug>); and
//     the issue is a decision issue for the SAME record, i.e. it carries the
//     decision-gate marker naming this DR or a brief whose `design:` cites it.
//     Every failure is a NAMED reason (rulingReason) and fails closed. A passing
//     ruling corroborates exactly ONE name — the human who wrote it; any other
//     real name the same decided-by carries is MISSING (wrong-author).
//   - `ruling:` absent and decided-by names no real human (the placeholder) → a
//     PROBLEM (MISSING-CORROBORATION, reason placeholder-unratified). This is the
//     hole the lane exists to close.
//   - `ruling:` absent and decided-by names a real human → unchanged: the existing
//     PR anchors (approved review, approval comment, closed decision issue) decide.
//
// A PRESENT-but-failing ruling link fails the stamp even when a PR anchor would
// have corroborated the name: the record then asserts a provenance that is false,
// and the stricter reading is the one a security assertion takes.
//
// What it does NOT establish. It proves WHO ruled and WHERE (a mapped human, on
// this record's decision issue). It does not read WHAT the comment says — an
// approval and a rejection are both rulings; whether the record reflects the
// ruling is the review gate's judgement; the refusal of an edited comment is what
// keeps the text that judgement reads the text the human wrote. Those limits are
// stated in registers-v1 §7.4.
//
// Split, as in decisiongateanchor.go: resolveRuling is the testable core — it takes
// an injected *ghClient (the httpDoer seam, faked in tests by an httptest server),
// the human-login map and the record's citing briefs as parameters, and reads
// nothing global. The disk/diff plumbing around it is small and pure.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// decisionPlaceholderName is the stamp name this lane records for a DR whose
// decided-by names no real human — the literal `human:<name>` placeholder, or any
// value humanStampRe cannot parse a name from. It cannot collide with a real name:
// humanStampRe's names are [0-9A-Za-z_]+ and confusableStampNames never yields `<`.
// It is NEVER looked up in the human-login map.
const decisionPlaceholderName = "<name>"

// rulingURLGrammar is the one documented ruling-link form, quoted in messages.
const rulingURLGrammar = "https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<ID>"

// decisionRulingURLRe is rulingURLGrammar as a regexp. Anchored at both ends: a
// pull-request URL, a bare issue URL, a comment on a different host, or trailing
// text are all malformed.
var decisionRulingURLRe = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/issues/([1-9][0-9]*)#issuecomment-([1-9][0-9]*)$`)

// decisionGateMarkerRe finds every decision-gate marker in an issue body.
var decisionGateMarkerRe = regexp.MustCompile(`<!--\s*decision-gate:\s*(\S+?)\s*-->`)

// issueAPIURLRe extracts owner/repo/number from a REST issue_url.
var issueAPIURLRe = regexp.MustCompile(`/repos/([^/]+)/([^/]+)/issues/([0-9]+)$`)

// rulingLink is a parsed ruling: URL.
type rulingLink struct {
	Repo      string // "owner/repo"
	Issue     int
	CommentID int64
}

// parseRulingURL parses a ruling: value against the one documented grammar.
func parseRulingURL(raw string) (rulingLink, bool) {
	m := decisionRulingURLRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return rulingLink{}, false
	}
	n, err1 := strconv.Atoi(m[3])
	id, err2 := strconv.ParseInt(m[4], 10, 64)
	if err1 != nil || err2 != nil {
		return rulingLink{}, false
	}
	return rulingLink{Repo: m[1] + "/" + m[2], Issue: n, CommentID: id}, true
}

// rulingReason names WHY a ruling did not corroborate. Every refusal carries one;
// the string is what the report prints, so an operator can act on it without
// reading this file.
type rulingReason string

const (
	rulingOK                    rulingReason = ""
	rulingPlaceholderUnratified rulingReason = "placeholder-unratified"
	rulingRecordUnreadable      rulingReason = "record-unreadable"
	rulingMalformed             rulingReason = "malformed-link"
	rulingUnresolvable          rulingReason = "unresolvable-link"
	rulingDeleted               rulingReason = "deleted-comment"
	rulingBotAuthor             rulingReason = "bot-author"
	rulingWrongAuthor           rulingReason = "wrong-author"
	rulingUnrelatedIssue        rulingReason = "unrelated-issue"
	rulingEditedComment         rulingReason = "edited-comment"
	rulingRecordNotNamed        rulingReason = "record-not-named"
)

// decisionRecordStamp is one DR record the PR touched, as read from the checkout.
type decisionRecordStamp struct {
	File      string   // repo-relative path, as it appears in stamp.File
	ID        string   // the record's frontmatter id when it is a valid DR-<slug>; "" otherwise, so the ruling lane refuses (fail closed, record-not-named)
	DecidedBy string   // raw decided-by value
	Names     []string // real human names parsed from DecidedBy (lowercase); empty = placeholder
	Ruling    string   // raw ruling: value ("" = absent)
	LoadErr   string   // non-empty when the record could not be read/parsed
}

// rulingOutcome is the resolved verdict for one record's ruling link.
type rulingOutcome struct {
	Present bool         // the record carries a ruling: (or could not be read)
	Reason  rulingReason // rulingOK on a pass
	Detail  string       // human-readable evidence (pass) or explanation (refusal)
	Author  string       // the ruling comment's author login, when observed
	Name    string       // the human name the author resolved to, on a pass
	Names   []string     // the record's real decided-by names (empty = placeholder)
}

// rulingOutcomes maps a DR record file (stamp.File) to its outcome.
type rulingOutcomes map[string]rulingOutcome

// appliesTo reports whether this outcome decides the stamp named name. The
// placeholder is always decided here (it has no other anchor). A real name is
// decided here only when the record carries a ruling link naming it; with no link
// the existing PR anchors decide, exactly as before.
func (o rulingOutcome) appliesTo(name string) bool {
	if name == decisionPlaceholderName {
		return true
	}
	if !o.Present {
		return false
	}
	for _, n := range o.Names {
		if n == name {
			return true
		}
	}
	return false
}

// rulesFor reports whether a PASSING outcome corroborates the stamp named name. A
// ruling corroborates exactly ONE name — the human the comment's author resolved
// to (o.Name) — or, for a placeholder record (no real names), the placeholder
// stamp. It never extends to another name the same decided-by carries: that human
// did not write this comment.
func (o rulingOutcome) rulesFor(name string) bool {
	if name == decisionPlaceholderName {
		return len(o.Names) == 0
	}
	return o.Name != "" && name == o.Name
}

// rulingStampResult renders the outcome as a corroborateResult for stamp s.
func rulingStampResult(s stamp, o rulingOutcome) corroborateResult {
	if o.Present && o.Reason == rulingOK {
		if o.rulesFor(s.Name) {
			return corroborateResult{
				Stamp:   s,
				Verdict: verdictCorroborated,
				Login:   o.Author,
				Evidence: fmt.Sprintf("ruling comment by %s (mapped human %q) on this record's decision issue: %s",
					o.Author, o.Name, o.Detail),
			}
		}
		// A co-signer: the record's ruling link is present and resolves, but to a
		// DIFFERENT human. One human's ruling never corroborates another's name.
		return corroborateResult{
			Stamp:   s,
			Verdict: verdictMissing,
			Login:   o.Author,
			Evidence: fmt.Sprintf("ruling %s: the ruling comment %s is by %s (mapped human %q), but decided-by also names human:%s — a ruling link corroborates only the human who wrote it (registers-v1 §7.5)",
				rulingWrongAuthor, o.Detail, o.Author, o.Name, s.Name),
		}
	}
	reason, detail := o.Reason, o.Detail
	if !o.Present {
		reason = rulingPlaceholderUnratified
		detail = "decided-by names no real human and the record carries no ruling: link — neither a real name corroborated on the PR nor a resolvable ruling (registers-v1 §7.5)"
	}
	return corroborateResult{
		Stamp:    s,
		Verdict:  verdictMissing,
		Login:    o.Author,
		Evidence: fmt.Sprintf("ruling %s: %s", reason, detail),
	}
}

// resolveRuling is the testable core. It resolves rec's ruling link through c and
// returns the outcome. prRepo is the repository the record lives in; humanLogins
// is the human-login map (name → login, names lowercase); citingBriefs are the
// "<stream>/<NN>" ids of briefs whose `design:` cites rec.ID. It reads nothing
// global and fails CLOSED in every direction: a pass needs every condition.
func resolveRuling(c *ghClient, prRepo string, rec decisionRecordStamp, humanLogins map[string]string, citingBriefs []string) rulingOutcome {
	out := rulingOutcome{Present: true, Names: rec.Names}
	refuse := func(r rulingReason, format string, a ...any) rulingOutcome {
		out.Reason = r
		out.Detail = fmt.Sprintf(format, a...)
		return out
	}
	if rec.LoadErr != "" {
		return refuse(rulingRecordUnreadable, "the record could not be read from the checkout (%s) — nothing to corroborate against", rec.LoadErr)
	}
	if strings.TrimSpace(rec.Ruling) == "" {
		out.Present = false
		return out
	}
	// The ruling is bound to the record by its DR-<slug> id (condition 5, and the
	// decision-gate marker of condition 6). A record with no valid frontmatter id
	// has no id a comment could name, so condition 5 cannot hold: refuse under its
	// reason before any forge contact, with a detail that says why. The register
	// lint reports the same record offline.
	if rec.ID == "" {
		return refuse(rulingRecordNotNamed, "the record %s carries no valid DR-<slug> frontmatter id, so no ruling comment can name it — fix its id: (statusgen --lint reports it)", rec.File)
	}
	link, ok := parseRulingURL(rec.Ruling)
	if !ok {
		return refuse(rulingMalformed, "ruling %q is not an issue-comment URL of the form %s", rec.Ruling, rulingURLGrammar)
	}
	if !strings.EqualFold(link.Repo, prRepo) {
		return refuse(rulingUnrelatedIssue, "ruling links %s#%d, outside %s where the record lives — a decision issue for this record is in its own repository", link.Repo, link.Issue, prRepo)
	}
	if c == nil {
		return refuse(rulingUnresolvable, "no forge client — the ruling link could not be resolved")
	}

	// 1. The comment.
	body, status, err := c.GetIssueComment(link.Repo, link.CommentID)
	switch {
	case err != nil:
		return refuse(rulingUnresolvable, "fetching comment %d on %s: %v", link.CommentID, link.Repo, err)
	case status == http.StatusNotFound || status == http.StatusGone:
		return refuse(rulingDeleted, "comment %d on %s: %s — the ruling comment is deleted or never existed", link.CommentID, link.Repo, httpReason(status, body))
	case status != http.StatusOK:
		return refuse(rulingUnresolvable, "comment %d on %s: %s", link.CommentID, link.Repo, httpReason(status, body))
	}
	var cm struct {
		User struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
		IssueURL  string `json:"issue_url"`
		HTMLURL   string `json:"html_url"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	if jerr := json.Unmarshal(body, &cm); jerr != nil || cm.User.Login == "" {
		return refuse(rulingUnresolvable, "comment %d on %s: HTTP 200 but the payload did not parse as an issue comment with an author", link.CommentID, link.Repo)
	}
	out.Author = cm.User.Login

	// 2. The comment really sits on the issue the link names (a link can pair
	// issue #A with a comment id from issue #B).
	im := issueAPIURLRe.FindStringSubmatch(cm.IssueURL)
	if im == nil {
		return refuse(rulingUnresolvable, "comment %d on %s: the payload names no parent issue (issue_url %q)", link.CommentID, link.Repo, cm.IssueURL)
	}
	onRepo, onNum := im[1]+"/"+im[2], im[3]
	if !strings.EqualFold(onRepo, link.Repo) || onNum != strconv.Itoa(link.Issue) {
		return refuse(rulingUnrelatedIssue, "comment %d sits on %s#%s, not on %s#%d as the ruling link claims", link.CommentID, onRepo, onNum, link.Repo, link.Issue)
	}

	// 2b. The comment is unedited: the text a reviewer reads must be the text the
	// human wrote. Timestamps that cannot be read cannot show that — a
	// could-not-check (unresolvable-link), never a pass.
	created, cerr := time.Parse(time.RFC3339, cm.CreatedAt)
	updated, uerr := time.Parse(time.RFC3339, cm.UpdatedAt)
	if cerr != nil || uerr != nil {
		return refuse(rulingUnresolvable, "comment %d on %s: created_at %q / updated_at %q did not parse — whether the comment was edited after it was posted could not be checked", link.CommentID, link.Repo, cm.CreatedAt, cm.UpdatedAt)
	}
	if !updated.Equal(created) {
		return refuse(rulingEditedComment, "comment %d on %s was edited after it was posted (created_at %s, updated_at %s) — link an unedited ruling comment; to correct a ruling, the human posts a new comment", link.CommentID, link.Repo, cm.CreatedAt, cm.UpdatedAt)
	}

	// 3. A bot never rules. Checked before the map so a mis-mapped bot login is
	// still refused by its type.
	if strings.EqualFold(cm.User.Type, "Bot") || strings.HasSuffix(strings.ToLower(cm.User.Login), "[bot]") {
		return refuse(rulingBotAuthor, "comment %d is authored by %s (type %q) — a bot account is not a human allowed to decide", link.CommentID, cm.User.Login, cm.User.Type)
	}

	// 4. The author maps to a human allowed to decide — and to THE human the stamp
	// names, when it names one.
	if len(rec.Names) > 0 {
		matched := ""
		var want []string
		for _, n := range rec.Names {
			login, known := humanLogins[n]
			if !known {
				want = append(want, fmt.Sprintf("human:%s (no mapping in %s)", n, scanEnvHumanLoginMap))
				continue
			}
			want = append(want, fmt.Sprintf("human:%s → %s", n, login))
			if strings.EqualFold(login, cm.User.Login) {
				matched = n
				break
			}
		}
		if matched == "" {
			return refuse(rulingWrongAuthor, "comment %d is authored by %s, but decided-by names %s", link.CommentID, cm.User.Login, strings.Join(want, ", "))
		}
		out.Name = matched
	} else {
		name := humanNameForLogin(humanLogins, cm.User.Login)
		if name == "" {
			return refuse(rulingWrongAuthor, "comment %d is authored by %s, which maps to no human in %s — not a human allowed to decide", link.CommentID, cm.User.Login, scanEnvHumanLoginMap)
		}
		out.Name = name
	}

	// 5. The comment names THIS record, in text the human wrote (and, by 2b, did
	// not edit afterwards). The issue-body marker checked in 6 is editable after
	// the ruling by anyone with write access, so it cannot alone bind the comment
	// to the record.
	if !commentNamesRecord(cm.Body, rec.ID) {
		return refuse(rulingRecordNotNamed, "comment %d on %s does not name the record %s — the ruling comment must name the record it rules on", link.CommentID, link.Repo, rec.ID)
	}

	// 6. The issue is a decision issue for THIS record.
	ibody, istatus, ierr := c.GetIssue(link.Repo, link.Issue)
	switch {
	case ierr != nil:
		return refuse(rulingUnresolvable, "fetching issue %s#%d: %v", link.Repo, link.Issue, ierr)
	case istatus != http.StatusOK:
		return refuse(rulingUnresolvable, "issue %s#%d: %s", link.Repo, link.Issue, httpReason(istatus, ibody))
	}
	var iss struct {
		Body        string          `json:"body"`
		PullRequest json.RawMessage `json:"pull_request"`
	}
	if jerr := json.Unmarshal(ibody, &iss); jerr != nil {
		return refuse(rulingUnresolvable, "issue %s#%d: HTTP 200 but the payload did not parse", link.Repo, link.Issue)
	}
	if len(iss.PullRequest) > 0 && string(iss.PullRequest) != "null" {
		return refuse(rulingUnrelatedIssue, "%s#%d is a pull request, not a decision issue", link.Repo, link.Issue)
	}
	if !issueRulesOnRecord(iss.Body, rec.ID, link.Repo, citingBriefs) {
		return refuse(rulingUnrelatedIssue, "%s#%d carries no decision-gate marker for %s or for a brief whose design: cites it — the comment is not on this record's decision issue", link.Repo, link.Issue, rec.ID)
	}

	out.Reason = rulingOK
	url := cm.HTMLURL
	if url == "" {
		url = rec.Ruling
	}
	out.Detail = url
	return out
}

// humanNameForLogin reverse-resolves a login through the human-login map. ""
// means the login is not a mapped human. Deterministic when two names map to one
// login (the lexically first name wins).
func humanNameForLogin(humanLogins map[string]string, login string) string {
	var names []string
	for n, l := range humanLogins {
		if strings.EqualFold(l, login) {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}

// commentNamesRecord reports whether text names the record id as a whole token:
// the id is not preceded or followed by a character that could continue an id
// ([A-Za-z0-9_-]), so a longer id that merely starts with this one does not count.
// Case-sensitive, as record ids are (decisionIDRe).
func commentNamesRecord(text, recordID string) bool {
	if recordID == "" {
		return false
	}
	idChar := func(b byte) bool {
		return b == '-' || b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	}
	for from := 0; ; {
		i := strings.Index(text[from:], recordID)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(recordID)
		if (start == 0 || !idChar(text[start-1])) && (end == len(text) || !idChar(text[end])) {
			return true
		}
		from = start + 1
	}
}

// issueRulesOnRecord reports whether an issue body carries a decision-gate marker
// for the record: either the record's own id (`<!-- decision-gate: DR-<slug> -->`)
// or a brief that cites it via `design:`, in either brief-id form — the canonical
// "<stream>/<NN>" or the colon work-item form ending "<repo>:<stream>:<NN>". The
// colon form must name the record's own repository when it names one.
func issueRulesOnRecord(body, recordID, repo string, citingBriefs []string) bool {
	repoName := repo
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repoName = repo[i+1:]
	}
	for _, m := range decisionGateMarkerRe.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if id == recordID {
			return true
		}
		stream, num, ok := briefStreamNum(id)
		if !ok {
			continue
		}
		if parts := strings.Split(id, ":"); len(parts) >= 3 && !strings.EqualFold(parts[len(parts)-3], repoName) {
			continue // a colon-form id naming another repository
		}
		for _, b := range citingBriefs {
			bs, bn, bok := briefStreamNum(b)
			if bok && bs == stream && normBriefNum(bn) == normBriefNum(num) {
				return true
			}
		}
	}
	return false
}

// normBriefNum drops leading zeros so "05" and "5" compare equal.
func normBriefNum(n string) string {
	t := strings.TrimLeft(n, "0")
	if t == "" {
		return "0"
	}
	return t
}

// ---- disk / diff plumbing ------------------------------------------------------

// isDecisionRecordPath reports whether a repo-relative path is a record in the
// DECISIONS register. Scoped to the register directory on purpose: a DR-shaped
// file anywhere else is not a record this lane gates.
//
// The file set is exactly the one parseDecisionsDir reads: every .md directly under
// the register directory except README.md, whatever its basename. The design gate
// resolves a brief's design: by the record's frontmatter id, not by its file name, so
// a record whose basename is not DR-shaped is still an approval the gate accepts. The
// online lane must therefore re-read its decided-by too. Keying this lane on a
// DR-shaped basename let such a record's decided-by pass the register lint while the
// online lane never looked at it.
func isDecisionRecordPath(path string) bool {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "docs/streams/"+decisionsDirName+"/") {
		return false
	}
	base := strings.TrimPrefix(p, "docs/streams/"+decisionsDirName+"/")
	if base == "" || strings.Contains(base, "/") {
		return false
	}
	return strings.HasSuffix(base, ".md") && base != "README.md"
}

// decisionRecordsInDiff returns every decision record file (isDecisionRecordPath) the
// diff ADDS OR EDITS (at least one added line), in first-seen order. A declared fixture corpus is skipped
// exactly as stampsInDiff skips it.
func decisionRecordsInDiff(root, diff string) []string {
	var out []string
	seen := map[string]bool{}
	// The one shared diff walker (addedDiffLines, corroboratescope.go) already skips
	// a declared fixture corpus; a decision record is never a .patch file, so the
	// walker's embedded-patch rule never touches this lane.
	for _, al := range addedDiffLines(root, diff) {
		cur := al.File
		if cur == "" || seen[cur] || !isDecisionRecordPath(cur) {
			continue
		}
		seen[cur] = true
		out = append(out, cur)
	}
	return out
}

// loadDecisionRecordStamp reads one DR record from the checkout. A read or parse
// failure is carried in LoadErr (fail closed downstream), never dropped.
//
// The record's ID is its FRONTMATTER id, not its file name: that is the id the
// design gate resolves a brief's design: by, the id the register lint validates, and
// the id registers-v1 §7.5 condition 5 requires the ruling comment to name. The
// register sets no file-name rule (§7.2), so a record's basename may be any name. An
// id that is absent or not DR-<slug> leaves ID empty, and the ruling lane refuses it
// (record-not-named, with a detail naming the missing id); the register lint reports
// the same record.
func loadDecisionRecordStamp(root, file string) decisionRecordStamp {
	rec := decisionRecordStamp{File: file}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		rec.LoadErr = err.Error()
		return rec
	}
	fm, _, ferr := splitFrontmatter(string(raw))
	if ferr != nil {
		rec.LoadErr = ferr.Error()
		return rec
	}
	var e decisionEntry
	if uerr := yaml.Unmarshal([]byte(fm), &e); uerr != nil {
		rec.LoadErr = uerr.Error()
		return rec
	}
	if id := strings.TrimSpace(e.ID); decisionIDRe.MatchString(id) {
		rec.ID = id
	}
	rec.DecidedBy = e.DecidedBy
	rec.Ruling = e.Ruling
	for _, m := range humanStampRe.FindAllStringSubmatch(e.DecidedBy, -1) {
		n := strings.ToLower(m[1])
		dup := false
		for _, x := range rec.Names {
			dup = dup || x == n
		}
		if !dup {
			rec.Names = append(rec.Names, n)
		}
	}
	return rec
}

// addDecisionRecordStamps makes every touched record's decided-by visible to the
// stamp lane: a real name not already found on an added line is appended as a
// stamp (so a PR that edits a record's body still re-gates its approver), and a
// record whose decided-by names no real human — or that could not be read — gets
// the placeholder stamp, which only the ruling lane can corroborate.
func addDecisionRecordStamps(stamps []stamp, recs []decisionRecordStamp) []stamp {
	have := map[string]bool{}
	for _, s := range stamps {
		have[s.Name+"\x00"+s.File] = true
	}
	add := func(name, file, line string) {
		if have[name+"\x00"+file] {
			return
		}
		have[name+"\x00"+file] = true
		stamps = append(stamps, stamp{Name: name, File: file, Line: line, Unresolved: true})
	}
	for _, r := range recs {
		line := strings.TrimSpace("decided-by: " + r.DecidedBy)
		if r.LoadErr != "" || len(r.Names) == 0 {
			add(decisionPlaceholderName, r.File, line)
			continue
		}
		for _, n := range r.Names {
			add(n, r.File, line)
		}
	}
	return stamps
}

// decisionCitingBriefs returns the "<stream>/<NN>" ids of every brief under
// root/docs/streams (active or archived under done/) whose `design:` cites id. An
// empty id cites nothing: it would otherwise match every brief with no design:.
func decisionCitingBriefs(root, id string) []string {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	var out []string
	for _, pat := range []string{
		filepath.Join(root, "docs", "streams", "*", "brief-*.md"),
		filepath.Join(root, "docs", "streams", "*", archiveDirName, "brief-*.md"),
	} {
		paths, _ := filepath.Glob(pat)
		for _, p := range paths {
			bf, ok, err := parseBriefFile(p)
			if err != nil || !ok || strings.TrimSpace(bf.Design) != id {
				continue
			}
			m := briefNameRe.FindStringSubmatch(filepath.Base(p))
			if m == nil {
				continue
			}
			dir := filepath.Dir(p)
			if filepath.Base(dir) == archiveDirName {
				dir = filepath.Dir(dir)
			}
			out = append(out, filepath.Base(dir)+"/"+m[1])
		}
	}
	sort.Strings(out)
	return out
}

// resolveDecisionRulings is the runner glue: one outcome per touched record.
func resolveDecisionRulings(c *ghClient, root, repo string, recs []decisionRecordStamp, humanLogins map[string]string) rulingOutcomes {
	out := rulingOutcomes{}
	for _, r := range recs {
		// Name the forge target BEFORE first contact, so an operator sees what is
		// about to be read, not only the verdict afterwards.
		if r.LoadErr == "" && strings.TrimSpace(r.Ruling) != "" {
			fmt.Fprintf(os.Stderr, "statusgen --corroborate: resolving ruling link %s for %s\n", strings.TrimSpace(r.Ruling), r.File)
		}
		out[r.File] = resolveRuling(c, repo, r, humanLogins, decisionCitingBriefs(root, r.ID))
	}
	return out
}

// rulingForgeClient builds the production client. The token comes from GH_TOKEN,
// then GITHUB_TOKEN, then `gh auth token` — the same credential the rest of
// --corroborate reaches through the gh CLI. An empty token still reaches a public
// repository; a private one then 404s, which fails closed (deleted-comment).
func rulingForgeClient() *ghClient {
	tok := strings.TrimSpace(os.Getenv("GH_TOKEN"))
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if tok == "" {
		if b, err := exec.Command("gh", "auth", "token").Output(); err == nil {
			tok = strings.TrimSpace(string(b))
		}
	}
	return newGHClient(tok)
}
