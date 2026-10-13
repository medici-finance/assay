package main

// format.go — the five-part decision format builder, ported line-for-line from the
// oracle's write_format_program() JQFMT (assay-inbox.sh:425-540). walk and html both
// render this ONE builder's output in the oracle so the two surfaces of the
// `ask-decision` contract cannot drift apart; this port keeps that property by giving
// `render` (walk.go) and the (deferred) html renderer the same buildRendered call.
//
// format_parity_test.go asserts this Go port against the REAL jq program — extracted
// verbatim from the oracle at test time and run through the system `jq` binary on
// identical fixture input — rather than against a second, hand-written expectation, so a
// future edit to either side that silently diverges is caught byte-for-byte.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// comment is one issue comment: the author login and body text the oracle's `gh issue
// view --json body,comments` carries. GitHub renders a comment with no body as an empty
// string, never null, so there is no null-body case to filter here (the oracle's `.body
// != null` guard has nothing to exclude on this forge).
type comment struct {
	Author string
	Body   string
}

// issueDetail is one issue's body + comments, or the "could not be read" state the oracle
// renders as `{detailUnavailable:true}` — a DETAIL fetch failing is reported as an unread
// item, never as an empty one (format.go's $blind path).
type issueDetail struct {
	Body        string
	Comments    []comment
	Unavailable bool
}

// option is one rendered choice.
type option struct {
	Letter      string
	Text        string
	Recommended bool
}

// rendered is one item's five-part block — the oracle's per-item object
// (assay-inbox.sh:522-539), minus the fields (repo/number/url/title/labels/ageDays/
// index/total) that walk.go/html already hold on the source item and would otherwise
// duplicate.
type rendered struct {
	Header        string
	Context       []string
	Options       []option
	OptionsStated bool
	Reply         string
	Command       string
	Verification  string
	Unread        bool
}

var (
	contextHeadingRe   = regexp.MustCompile(`(?i)^[ \t]*#{1,6}[ \t]*(Context|Situation|Ask|Summary|Problem)\b`)
	htmlCommentLineRe  = regexp.MustCompile(`^[ \t]*<!--`)
	headingLineRe      = regexp.MustCompile(`^[ \t]*#{1,6}[ \t]`)
	quoteOrTableLineRe = regexp.MustCompile(`^[|>]`)
	unblocksLineRe     = regexp.MustCompile(`(?i)^(unblocks|blocks|depends|gates)\b`)
	desknoteAuthorRe   = regexp.MustCompile(`(?i)\[bot\]$|desk`)

	// Reply shape's command line (the oracle's $answerPhrase / $forkLetters /
	// $defaultFull / $driverLogin, assay-inbox.sh JQFMT).
	answerHeadingRe = regexp.MustCompile(`(?i)^[ \t]*#{1,6}[ \t]*How[ \t]+to[ \t]+(answer|reply|respond)\b`)
	quotedPhraseRe  = regexp.MustCompile(`["“]([^"“”]+)["”]`)
	optLetterRe     = regexp.MustCompile(`^(?:[Oo]ption[[:space:]]+)?([A-Da-d1-4])(?:$|[.)!:,]|[[:space:]]*[—–-])`)
	forkOptionRe    = regexp.MustCompile(`(?i)^option:[[:space:]]*`)
	forkDefaultRe   = regexp.MustCompile(`(?i)^default:[[:space:]]*`)
	loginShapeRe    = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
)

// commandNotDerivable is Reply shape's command line when the issue states no ruling
// phrase: the desk fills it; the renderer never composes a ruling of its own.
const commandNotDerivable = "not derivable — desk to fill"

// optLetter is the oracle's optletter def: the upper-cased option letter a fork-test
// `option:` / `default:` value leads with, or "" when it leads with none.
func optLetter(s string) string {
	m := optLetterRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1])
}

// shQuote is jq's @sh on a string: wrapped in single quotes, each embedded single quote
// closed, backslash-escaped and reopened.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// replyCommand is the oracle's $command: the copy-pasteable ruling comment for the
// recommended option, derived ONLY from a phrase the issue itself states — the first
// non-blank double-quoted phrase of a "How to answer/reply/respond" section, else a fork-test
// `default:` value naming one of the block's own `option:` letters. GH_TOKEN is SET to
// the driver's own token so the comment cannot post under an exported App token or the
// wrong active account. The login is $DRIVER_LOGIN when login-shaped, else the
// `<driver-login>` placeholder.
func replyCommand(it item, bl []string) string {
	ruling := ""
	found := false
phrases:
	for _, l := range section(bl, answerHeadingRe) {
		for _, m := range quotedPhraseRe.FindAllStringSubmatch(cleanText(demd(l)), -1) {
			if p := stripWS(m[1]); p != "" {
				ruling, found = p, true
				break phrases
			}
		}
	}
	if !found {
		var forkLetters []string
		defaultFull, haveDefault := "", false
		for _, l := range nonblankLines(bl) {
			v := cleanText(demd(stripWS(l)))
			if forkOptionRe.MatchString(v) {
				if ol := optLetter(forkOptionRe.ReplaceAllString(v, "")); ol != "" {
					forkLetters = append(forkLetters, ol)
				}
			}
			if !haveDefault && forkDefaultRe.MatchString(v) {
				defaultFull, haveDefault = stripWS(forkDefaultRe.ReplaceAllString(v, "")), true
			}
		}
		if dl := optLetter(defaultFull); dl != "" {
			for _, fl := range forkLetters {
				if fl == dl {
					ruling, found = defaultFull, true
					break
				}
			}
		}
	}
	if !found {
		return commandNotDerivable
	}
	login := os.Getenv("DRIVER_LOGIN")
	if !loginShapeRe.MatchString(login) {
		login = "<driver-login>"
	}
	return fmt.Sprintf(`GH_TOKEN="$(gh auth token --user %s)" gh issue comment %d -R %s --body %s`,
		login, it.Number, it.Repo, shQuote(ruling))
}

// toLines is the oracle's `lines` def: strip every \r, then split on \n.
func toLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
}

// stripWS is the oracle's `strip` def: trim leading/trailing POSIX [:space:].
func stripWS(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
	})
}

// hasNonSpace reports whether s carries at least one non-whitespace rune — the oracle's
// `nonblank` filter test, applied per line.
func hasNonSpace(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\v' && r != '\f' && r != '\r' {
			return true
		}
	}
	return false
}

// nonblankLines is the oracle's `nonblank` def: drop blank/whitespace-only lines.
func nonblankLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if hasNonSpace(l) {
			out = append(out, l)
		}
	}
	return out
}

// demd is the oracle's `demd` def: drop markdown bold markers and backticks.
func demd(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

// runeTrunc cuts s to at most n runes — jq string slicing is by codepoint, not byte.
func runeTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// bodyLines is the oracle's $bl: the body split to lines with pure HTML-comment lines
// (the desk tools' marker channel — never prose for the driver) removed.
func bodyLines(body string) []string {
	var out []string
	for _, l := range toLines(body) {
		if !htmlCommentLineRe.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// section is the oracle's section($re) def: the lines strictly BETWEEN the first heading
// matching re and the next heading (any level), or nil when re matches nothing.
func section(bl []string, re *regexp.Regexp) []string {
	h := -1
	for i, l := range bl {
		if re.MatchString(l) {
			h = i
			break
		}
	}
	if h == -1 {
		return nil
	}
	rest := bl[h+1:]
	stop := -1
	for i, l := range rest {
		if headingLineRe.MatchString(l) {
			stop = i
			break
		}
	}
	if stop == -1 {
		return rest
	}
	return rest[:stop]
}

// buildRendered is the oracle's write_format_program applied to one item: the sorted
// queue entry (labels/createdAt/url/repo/number), its detail (body+comments or blind),
// and its 0-based position k of n in the queue.
func buildRendered(it item, d issueDetail, k, n int, now time.Time) rendered {
	bl := bodyLines(d.Body)

	// ---- Context --------------------------------------------------------------------
	csec := section(bl, contextHeadingRe)
	var ctxraw []string
	if len(nonblankLines(csec)) > 0 {
		ctxraw = csec
	} else {
		for _, l := range bl {
			if !headingLineRe.MatchString(l) {
				ctxraw = append(ctxraw, l)
			}
		}
	}
	ctxraw = nonblankLines(ctxraw)
	cleaned := make([]string, 0, len(ctxraw))
	for _, l := range ctxraw {
		v := cleanText(demd(stripWS(l)))
		if quoteOrTableLineRe.MatchString(v) {
			continue
		}
		cleaned = append(cleaned, v)
	}
	if len(cleaned) > 2 {
		cleaned = cleaned[:2]
	}

	var ctx []string
	switch {
	case d.Unavailable:
		ctx = []string{"could-not-check: this issue's body and comments could not be read (detail fetch failed) — the item is UNREAD, not empty"}
	case len(cleaned) > 0:
		ctx = cleaned
	default:
		ctx = []string{"context not stated in the issue body — desk to fill before asking"}
	}

	labelNames := make([]string, 0, len(it.Labels))
	for _, l := range it.Labels {
		labelNames = append(labelNames, cleanText(l))
	}
	var esc []string
	for _, l := range labelNames {
		for _, e := range labels {
			if l == e {
				esc = append(esc, l)
				break
			}
		}
	}
	blockedTail := "no escalation label"
	if len(esc) > 0 {
		blockedTail = strings.Join(esc, ", ")
	}
	blocked := "blocked on the driver: carries " + blockedTail

	var unb []string
	blSet := make(map[string]bool, len(ctx))
	for _, c := range ctx {
		blSet[c] = true
	}
	for _, l := range nonblankLines(bl) {
		v := cleanText(demd(stripWS(l)))
		if !unblocksLineRe.MatchString(v) {
			continue
		}
		if blSet[v] {
			continue
		}
		unb = append(unb, v)
		break
	}

	var note []string
	if lc, ok := latestDeskComment(d.Comments); ok {
		nb := nonblankLines(toLines(lc.Body))
		if len(nb) > 2 {
			nb = nb[:2]
		}
		parts := make([]string, 0, len(nb))
		for _, l := range nb {
			parts = append(parts, demd(stripWS(l)))
		}
		snippet := runeTrunc(cleanText(strings.Join(parts, " ")), 180)
		author := lc.Author
		if author == "" {
			author = "?"
		}
		note = []string{"latest desk note (" + cleanText(author) + "): " + snippet}
	}

	ageStr := "unknown"
	if t, err := time.Parse(time.RFC3339, it.CreatedAt); err == nil {
		days := int(now.UTC().Sub(t.UTC()).Hours() / 24)
		ageStr = fmt.Sprintf("%dd", days)
	}
	gate := "gate: " + strings.Join(labelNames, ", ") + " · age " + ageStr + " · " + cleanText(it.URL)

	context := append(append(append([]string{}, ctx...), blocked), unb...)
	context = append(context, note...)
	context = append(context, gate)
	if len(context) > 6 {
		context = context[:6]
	}

	// ---- Options ---------------------------------------------------------------------
	// The parse is deskkit's (lifted there so the human-decision-v1 record reads the SAME
	// options the walk shows). The walk shows the recommended option
	// first, re-lettered A–D; with no option marked, the first stated one leads, unchanged.
	parsed := deskkit.ParseDecisionOptions(d.Body)
	letterFor := []string{"A", "B", "C", "D"}
	var opts []option
	for i, idx := range parsed.Walk() {
		opts = append(opts, option{Letter: letterFor[i], Text: parsed.Texts[idx], Recommended: i == 0})
	}

	stated := len(opts) > 0
	options := opts
	if !stated {
		options = []option{{Letter: "A", Text: "options not yet stated — desk to fill", Recommended: true}}
	}

	var reply string
	if stated {
		letters := make([]string, 0, len(options))
		for _, o := range options {
			letters = append(letters, o.Letter)
		}
		reply = "reply with one letter (" + strings.Join(letters, "/") + ") — nothing else is needed."
	} else {
		reply = "reply with the ruling in one line; the desk restates it as lettered options before acting."
	}

	next := "reports the queue drained."
	if k+1 < n {
		next = fmt.Sprintf("presents question %d of %d.", k+2, n)
	}
	verification := fmt.Sprintf(
		"the desk records the ruling on %s#%d as a relayed decision (never in the driver's voice), "+
			"moves the escalation label per the vocabulary, re-reads the issue to confirm the label moved, then %s",
		it.Repo, it.Number, next)

	header := fmt.Sprintf("%s#%d — question %d of %d", it.Repo, it.Number, k+1, n)

	return rendered{
		Header:        header,
		Context:       context,
		Options:       options,
		OptionsStated: stated,
		Reply:         reply,
		Command:       replyCommand(it, bl),
		Verification:  verification,
		Unread:        d.Unavailable,
	}
}

// latestDeskComment is the oracle's $lc: the LAST comment whose author reads as a bot or
// "desk" (case-insensitive), falling back to the last comment overall when none match.
func latestDeskComment(cs []comment) (comment, bool) {
	if len(cs) == 0 {
		return comment{}, false
	}
	for i := len(cs) - 1; i >= 0; i-- {
		if desknoteAuthorRe.MatchString(cs[i].Author) {
			return cs[i], true
		}
	}
	return cs[len(cs)-1], true
}
