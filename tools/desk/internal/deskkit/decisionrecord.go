package deskkit

// decisionrecord.go — the human-decision-v1 record.
//
// When the driver rules on a decision issue and the desk closes it through deskclose's
// human-decided lane, ONE structured record is written: which options were offered (by id
// and text digest, never the text), which one the ask recommended, which one the ruling
// picked, and how long the decision waited. The field table, closed sets and the
// never-recorded list are the human-decision-v1 schema doc's.
//
// This file holds the pieces that are not deskclose's own:
//
//   - ParseDecisionOptions — the Options parse lifted out of cmd/deskinbox/format.go
//     (itself a port of the bash oracle assay-inbox.sh), so the inbox and the record read
//     ONE parse. The inbox's rendered output is unchanged (TestParityWalk).
//   - DecisionAnchor / ParseRulingPick — which ask the options came from, and which offered
//     id the ruling names (the oracle's `isruling` grammar).
//   - ComposeDecisionRecord / ValidateDecisionRecord / EncodeDecisionRecord — the record.
//   - AppendDecisionRecord — the local copy, decision-records.jsonl beside audit.jsonl.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// The record's closed vocabulary.
const (
	DecisionRecordSchema    = "human-decision-v1"
	DecisionRuler           = "driver" // the ROLE; never a login or account id
	DecisionRecommendedNone = "none"
	DecisionPickAmbiguous   = "ambiguous"
	DecisionPickUnparsed    = "unparsed"
	DecisionSourceBody      = "body"
	DecisionSourceRelay     = "relay"
	DecisionViaDirect       = "direct"
	DecisionViaRatified     = "ratified-relay"
	// edit_state: whether any text the record was read from changed after the ruling.
	DecisionEditUnedited     = "unedited"
	DecisionEditEdited       = "edited"
	DecisionEditUnverifiable = "unverifiable"

	decisionRecordsFile = "decision-records.jsonl"
	maxDecisionOptions  = 4
)

// DecisionOption is one offered option: its id as the anchor ask wrote it, and the sha256
// of its cleaned text. The text itself is never recorded.
type DecisionOption struct {
	ID         string `json:"id"`
	TextSHA256 string `json:"text_sha256"`
}

// HumanDecisionRecord is one ruled close. Field order is the encoding order, so the forge
// block and the local line are the same bytes for the same record.
type HumanDecisionRecord struct {
	Schema              string           `json:"schema"`
	Repo                string           `json:"repo"`
	Issue               int              `json:"issue"`
	Tracker             string           `json:"tracker"`
	Brief               string           `json:"brief,omitempty"`
	Options             []DecisionOption `json:"options"`
	OptionsSource       string           `json:"options_source"`
	Recommended         string           `json:"recommended"`
	Picked              string           `json:"picked"`
	PickedVia           string           `json:"picked_via"`
	PickedIsRecommended bool             `json:"picked_is_recommended"`
	EditState           string           `json:"edit_state"`
	Ruler               string           `json:"ruler"`
	RulingSHA256        string           `json:"ruling_sha256"`
	OpenedAt            string           `json:"opened_at"`
	AskedAt             string           `json:"asked_at"`
	RuledAt             string           `json:"ruled_at"`
	RecordedAt          string           `json:"recorded_at"`
	OpenedToRuledS      int64            `json:"opened_to_ruled_s"`
	AskedToRuledS       int64            `json:"asked_to_ruled_s"`
	ToolSHA             string           `json:"tool_sha"`
}

// ---------------------------------------------------------------- the Options parse

// The Options grammar — byte-for-byte the oracle's (assay-inbox.sh, the `$oraw`/`$opts`
// block), formerly inline in cmd/deskinbox/format.go.
var (
	decOptionsHeadingRe = regexp.MustCompile(`(?i)^[ \t]*#{1,6}[ \t]*Options?\b`)
	decHTMLCommentRe    = regexp.MustCompile(`^[ \t]*<!--`)
	decHeadingLineRe    = regexp.MustCompile(`^[ \t]*#{1,6}[ \t]`)
	decOptionLineRe     = regexp.MustCompile(`^(?:[-*][ \t]*)?(?:\*\*)?[A-Da-d1-4][.)][ \t]`)
	decOptionCaptureRe  = regexp.MustCompile(`^(?:[-*][ \t]*)?(?:\*\*)?([A-Da-d1-4])[.)][ \t]*(.+)$`)
	decRecTokenRe       = regexp.MustCompile(`(?i)\(recommended\)`)
)

// DecisionOptions is one ask's parsed Options section, in SOURCE order (the order and the
// letters the ask itself wrote). Options whose text cleans to empty are already dropped.
type DecisionOptions struct {
	Letters     []string // the source letters, uppercased ("A".."D" or "1".."4")
	Texts       []string // the cleaned option texts
	Recommended int      // index of the option marked "recommended"; -1 when none is marked
}

// Stated reports whether the ask states at least one option.
func (o DecisionOptions) Stated() bool { return len(o.Texts) > 0 }

// Walk returns the option indices in the inbox walk's order: the recommended option first,
// then the rest in source order. The walk letters them A, B, C, D in this order — the
// re-lettering that makes a bare letter in a ruling potentially ambiguous.
func (o DecisionOptions) Walk() []int {
	out := make([]int, 0, len(o.Texts))
	if o.Recommended >= 0 && o.Recommended < len(o.Texts) {
		out = append(out, o.Recommended)
	}
	for i := range o.Texts {
		if i != o.Recommended {
			out = append(out, i)
		}
	}
	return out
}

// ParseDecisionOptions parses the Options section of an ask (an issue body or a relay
// comment): the lines between the first `# Options` heading (any level) and the next
// heading, the first four lettered list lines, and a recommendation only when exactly one
// standalone `(recommended)` marker (decMarkerSpans) appears across those options, every
// marker cut from the text. HTML comments are invisible.
func ParseDecisionOptions(body string) DecisionOptions {
	var bl []string
	inComment := false
	for _, l := range decLines(body) {
		for {
			if inComment {
				end := strings.Index(l, "-->")
				if end < 0 {
					l = ""
					break
				}
				l = l[end+3:]
				inComment = false
				continue
			}
			start := strings.Index(l, "<!--")
			if start < 0 {
				break
			}
			end := strings.Index(l[start+4:], "-->")
			if end < 0 {
				l = l[:start]
				inComment = true
				break
			}
			l = l[:start] + l[start+4+end+3:]
		}
		bl = append(bl, l)
	}
	type rawOpt struct{ let, txt string }
	var raw []rawOpt
	for _, l := range decSection(bl, decOptionsHeadingRe) {
		if !decHasNonSpace(l) {
			continue
		}
		s := decStrip(l)
		if !decOptionLineRe.MatchString(s) {
			continue
		}
		m := decOptionCaptureRe.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		raw = append(raw, rawOpt{let: strings.ToUpper(m[1]), txt: m[2]})
		if len(raw) == maxDecisionOptions {
			break
		}
	}
	rec, marks := -1, 0
	for i, o := range raw {
		if n := len(decMarkerSpans(decStrip(decClean(decDemd(o.txt))))); n > 0 {
			rec = i
			marks += n
		}
	}
	if marks != 1 {
		rec = -1
	}
	out := DecisionOptions{Recommended: -1}
	for i, o := range raw {
		v := decStrip(decClean(decDemd(o.txt)))
		v = decStrip(decCutMarkers(v))
		if v == "" {
			continue // dropped, as the oracle drops it; a dropped mark is no mark
		}
		if i == rec && rec >= 0 {
			out.Recommended = len(out.Texts)
		}
		out.Letters = append(out.Letters, o.let)
		out.Texts = append(out.Texts, v)
	}
	return out
}

// decMarkerSpans is the oracle's `recmark` grammar (assay-inbox.sh): the byte span of every
// standalone `(recommended)` token (any case) in s. The token must open the text or follow
// one space or tab, and must end the text or be followed by a space, a tab or one of
// `.,;:!?`. That following character is never part of the span, so adjacent markers each
// count. The span takes the one space or tab before the token, plus a `—`/`–`/`-`
// separator directly before that when whitespace precedes the separator, so cutting it
// leaves "Keep — (recommended)." as "Keep.". The whitespace is ASCII on both sides: a
// non-breaking space is not a boundary.
func decMarkerSpans(s string) [][2]int {
	var spans [][2]int
	for _, m := range decRecTokenRe.FindAllStringIndex(s, -1) {
		start, end := m[0], m[1]
		if end < len(s) && !strings.ContainsRune(" \t.,;:!?", rune(s[end])) {
			continue
		}
		if start > 0 {
			if c := s[start-1]; c != ' ' && c != '\t' {
				continue
			}
			start--
			for _, dash := range []string{"—", "–", "-"} {
				if !strings.HasSuffix(s[:start], dash) {
					continue
				}
				ws := strings.TrimRight(s[:start-len(dash)], " \t")
				if len(ws) < start-len(dash) {
					start = len(ws)
				}
				break
			}
		}
		spans = append(spans, [2]int{start, end})
	}
	return spans
}

// decCutMarkers removes every span decMarkerSpans finds — the oracle's `cleanoption` gsub.
func decCutMarkers(s string) string {
	var b strings.Builder
	last := 0
	for _, sp := range decMarkerSpans(s) {
		b.WriteString(s[last:sp[0]])
		last = sp[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// The oracle's string defs, private copies of cmd/deskinbox's (that package is a command).
func decLines(s string) []string { return strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") }

func decIsSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
}

func decStrip(s string) string { return strings.TrimFunc(s, decIsSpace) }

func decHasNonSpace(s string) bool {
	for _, r := range s {
		if !decIsSpace(r) {
			return true
		}
	}
	return false
}

func decDemd(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "**", ""), "`", "")
}

func decClean(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// decSection is the oracle's section($re): the lines strictly between the first heading
// matching re and the next heading of any level; nil when nothing matches.
func decSection(bl []string, re *regexp.Regexp) []string {
	for i, l := range bl {
		if !re.MatchString(l) {
			continue
		}
		rest := bl[i+1:]
		for j, r := range rest {
			if decHeadingLineRe.MatchString(r) {
				return rest[:j]
			}
		}
		return rest
	}
	return nil
}

// ---------------------------------------------------------------- the anchor and the pick

// The ruling grammar. Its first-line shapes are the oracle's `isruling` (assay-inbox.sh): the
// first content line names an offered letter ("B", "B.", "Option C — …") or ratifies
// ("ratified", "I ratify", "approved"). The oracle only screens an item for a desk that then
// reads the comment; the record is read by nobody, so the record's reading is stricter: a
// question, a refusal, a hold or a qualifier anywhere in the comment, or any SECOND offered
// id named anywhere in it, is `unparsed`, never a definite pick.
var (
	rulingLetterRe = regexp.MustCompile(`^(?:[Oo]ption\s+)?([A-D1-4])(?:$|[.)!:,]|\s*[—–-])`)
	rulingRatifyRe = regexp.MustCompile(`(?i)^(?:i\s+)?(?:ratif(?:y|ied)|approved?)\b`)
	// A refusal, a hold, or a qualifier that changes what was ratified or picked.
	rulingNegatesRe = regexp.MustCompile(`(?i)\b(?:no|not|none|neither|nor|don'?t|didn'?t|won'?t|hold|wait|later|` +
		`undecided|unsure|pending|reject(?:s|ed|ing)?|except|instead|unless|rather|but|however|actually)\b|n't\b`)
	// An id mention: one offered id standing alone as a token ("B", "B.", "2-3" names 2 and 3).
	rulingIDTokenRe = regexp.MustCompile(`\b[A-D1-4]\b`)
	// A relay's answer: `Answer: <letter>` — the letter alone, then the end or punctuation.
	relayAnswerRe       = regexp.MustCompile(`\b[Aa]nswer:\s*`)
	relayAnswerLetterRe = regexp.MustCompile(`^([A-D1-4])(?:$|[^A-Za-z0-9])`)
	// The brief marker statusgen writes: one whole line `<!-- needs-decision: <stream>/<NN> -->`.
	decisionBriefLineRe = regexp.MustCompile(`^<!-- needs-decision: (\S+) -->$`)
	// A brief id: `<stream>/<NN>`, lower-case stream, at most 64 + 1 + 4 bytes.
	decBriefIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}/[0-9]{1,4}$`)
)

// isTrustedApp reports a roster-trusted APP account — login AND a non-zero account id that
// matches the roster's pin — whose comment is not minimized: the only author whose comment
// can be an ask (anchor) or a relay a ratification resolves through. The id is required: a
// surface that reports no id cannot speak for an App here.
func isTrustedApp(c Comment) bool {
	l := strings.ToLower(c.Author.Login)
	if !strings.HasSuffix(l, "[bot]") && !strings.HasPrefix(l, "app/") {
		return false
	}
	if c.Minimized || c.Author.ID == 0 {
		return false
	}
	return TrustedAuthorID(c.Author.Login, c.Author.ID)
}

// DecisionAnchor returns the index of the ask the options are read from: the NEWEST
// roster-trusted App comment that states options, or -1 for the issue body when no such
// comment exists. Callers pass only the comments that precede the ruling.
func DecisionAnchor(comments []Comment) int {
	for i := len(comments) - 1; i >= 0; i-- {
		if isTrustedApp(comments[i]) && ParseDecisionOptions(comments[i].Body).Stated() {
			return i
		}
	}
	return -1
}

// DecisionRulingIndex returns the ruling's position in the thread, matched by database id or
// node id, or -1 when the thread does not carry it.
func DecisionRulingIndex(comments []Comment, ruling Comment) int {
	for i, c := range comments {
		if (ruling.DatabaseID != 0 && c.DatabaseID == ruling.DatabaseID) ||
			(ruling.ID != "" && c.ID == ruling.ID) {
			return i
		}
	}
	return -1
}

// parseDecisionBrief reads the brief id from the body's marker. Only a marker on a line of its
// own counts (a marker inside a sentence or a `>` quote is not the one statusgen writes); an
// id outside the brief-id grammar, or two markers naming different ids, yields no brief.
func parseDecisionBrief(body string) string {
	brief := ""
	for _, l := range decLines(body) {
		m := decisionBriefLineRe.FindStringSubmatch(decStrip(l))
		if m == nil {
			continue
		}
		if !decBriefIDRe.MatchString(m[1]) || (brief != "" && brief != m[1]) {
			return ""
		}
		brief = m[1]
	}
	return brief
}

// rulingContentLines is the oracle's per-comment line prep: HTML-comment lines dropped,
// blank lines dropped, each stripped, de-markdowned and control-cleaned.
func rulingContentLines(body string) []string {
	var out []string
	for _, l := range decLines(body) {
		if decHTMLCommentRe.MatchString(l) || !decHasNonSpace(l) {
			continue
		}
		out = append(out, decClean(decDemd(decStrip(l))))
	}
	return out
}

// relayAnswers returns every `Answer:` value on the comment's OWN lines — a `>`-quoted line is
// someone else's text and is never read — and whether the comment carries an answer at all.
// A value that is not a lone offered letter is returned as "" (a word answer).
func relayAnswers(body string) (answers []string, ok bool) {
	for _, l := range rulingContentLines(body) {
		if strings.HasPrefix(l, ">") {
			continue
		}
		for _, loc := range relayAnswerRe.FindAllStringIndex(l, -1) {
			ok = true
			if m := relayAnswerLetterRe.FindStringSubmatch(l[loc[1]:]); m != nil {
				answers = append(answers, m[1])
			} else {
				answers = append(answers, "")
			}
		}
	}
	return answers, ok
}

// lettering maps a letter to the option text it names under one way of lettering an ask.
type lettering func(letter string) (string, bool)

func sourceLettering(o DecisionOptions) lettering {
	return func(letter string) (string, bool) {
		for i, l := range o.Letters {
			if l == letter {
				return o.Texts[i], true
			}
		}
		return "", false
	}
}

func walkLettering(o DecisionOptions) lettering {
	walk := o.Walk()
	return func(letter string) (string, bool) {
		k := strings.Index("ABCD", letter)
		if len(letter) != 1 || k < 0 || k >= len(walk) {
			return "", false
		}
		return o.Texts[walk[k]], true
	}
}

// resolveLetter reads letter in the anchor's own lettering and returns the anchor's id for
// it — unless any ALTERNATIVE lettering the ruler could have been reading names a different
// option (or names one where the anchor names none), in which case the letter is
// `ambiguous`. A letter no lettering names is `unparsed`.
func resolveLetter(letter string, anchor DecisionOptions, alts []lettering) string {
	want, ok := sourceLettering(anchor)(letter)
	for _, alt := range alts {
		got, altOK := alt(letter)
		if altOK != ok || got != want {
			return DecisionPickAmbiguous
		}
	}
	if !ok {
		return DecisionPickUnparsed
	}
	return letter
}

// mentionedIDs returns the distinct ids of ids named as standalone tokens anywhere in text.
func mentionedIDs(text string, ids map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, tok := range rulingIDTokenRe.FindAllString(text, -1) {
		if ids[tok] {
			out[tok] = true
		}
	}
	return out
}

// ParseRulingPick resolves the ruling to an offered id of the anchor ask, or to one of the
// closed tokens `ambiguous` / `unparsed`, and says how it was picked.
//
//   - body is the issue body; comments is the thread in order, and rulingIdx the ruling's
//     position in it (only comments before it are read); anchorIdx is DecisionAnchor's answer
//     over those comments (-1 = the body). A rulingIdx that does not locate the ruling in the
//     thread is `unparsed`: what came before it cannot be told apart from what came after.
//   - A question, a refusal, a hold or a qualifier anywhere in the ruling is unparsed.
//   - A DIRECT letter is read in the anchor's lettering, and only when it is the ONE offered id
//     the ruling names anywhere ("A, B", "B — rejected, go with A" are unparsed). When the
//     anchor is a relay and the body states options too, the body's own lettering and the inbox
//     walk's re-lettering of it are alternatives the ruler could have read: a letter they
//     disagree on is ambiguous. A direct letter against a BODY anchor is the body's own letter.
//   - A RATIFICATION names no offered id of its own ("Approved — B" is unparsed). It resolves
//     through the newest roster-trusted App comment AT OR AFTER the anchor that carries an
//     `Answer:` line of its own (a `>`-quoted line is not its own); that relay's answer must be
//     one offered letter, or the pick is unparsed — the search never walks past a newer relay
//     with a word answer to an older one with a letter, and never to a relay older than the
//     ask being ratified. When that relay is itself the anchor, its letter is the relay's own;
//     otherwise the letter may be the walk's re-lettering of the anchor (and, for a relay
//     anchor, the body's) — a disagreement is ambiguous.
func ParseRulingPick(body string, comments []Comment, anchorIdx, rulingIdx int, ruling Comment) (picked, via string) {
	picked, via, _ = parseRulingPick(body, comments, anchorIdx, rulingIdx, ruling)
	return picked, via
}

// parseRulingPick is ParseRulingPick that also returns the index of the relay a ratification
// was resolved through (-1 when none was read), for the record's edit check.
func parseRulingPick(body string, comments []Comment, anchorIdx, rulingIdx int, ruling Comment) (picked, via string, relayIdx int) {
	relayIdx = -1
	if rulingIdx < 0 || rulingIdx >= len(comments) {
		return DecisionPickUnparsed, DecisionViaDirect, relayIdx
	}
	before := comments[:rulingIdx]
	bodyOpts := ParseDecisionOptions(body)
	anchor := bodyOpts
	if anchorIdx >= 0 && anchorIdx < len(before) {
		anchor = ParseDecisionOptions(before[anchorIdx].Body)
	} else {
		anchorIdx = -1
	}

	ls := rulingContentLines(ruling.Body)
	if len(ls) == 0 {
		return DecisionPickUnparsed, DecisionViaDirect, relayIdx
	}
	all := strings.Join(ls, " ")
	first := ls[0]
	letter := rulingLetterRe.FindStringSubmatch(first)
	via = DecisionViaDirect
	if letter == nil && rulingRatifyRe.MatchString(first) {
		via = DecisionViaRatified
	}
	if strings.Contains(all, "?") || rulingNegatesRe.MatchString(all) {
		return DecisionPickUnparsed, via, relayIdx
	}
	ids := map[string]bool{}
	for _, l := range anchor.Letters {
		ids[l] = true
	}
	if anchorIdx >= 0 {
		for _, l := range bodyOpts.Letters {
			ids[l] = true
		}
	}
	named := mentionedIDs(all, ids)

	if letter != nil {
		if len(named) > 1 || (len(named) == 1 && !named[letter[1]]) {
			return DecisionPickUnparsed, via, relayIdx
		}
		var alts []lettering
		if anchorIdx >= 0 && bodyOpts.Stated() {
			alts = append(alts, sourceLettering(bodyOpts), walkLettering(bodyOpts))
		}
		return resolveLetter(letter[1], anchor, alts), via, relayIdx
	}

	if via != DecisionViaRatified || len(named) > 0 {
		return DecisionPickUnparsed, via, relayIdx
	}
	lo := anchorIdx
	if lo < 0 {
		lo = 0
	}
	for i := len(before) - 1; i >= lo; i-- {
		c := before[i]
		if !isTrustedApp(c) {
			continue
		}
		answers, ok := relayAnswers(c.Body)
		if !ok {
			continue
		}
		relayIdx = i
		answer := answers[0]
		for _, a := range answers {
			if a == "" || a != answer {
				return DecisionPickUnparsed, DecisionViaRatified, relayIdx
			}
		}
		var alts []lettering
		if i != anchorIdx {
			alts = append(alts, walkLettering(anchor))
			if anchorIdx >= 0 && bodyOpts.Stated() {
				alts = append(alts, sourceLettering(bodyOpts), walkLettering(bodyOpts))
			}
		}
		return resolveLetter(answer, anchor, alts), DecisionViaRatified, relayIdx
	}
	return DecisionPickUnparsed, DecisionViaRatified, relayIdx
}

// ---------------------------------------------------------------- compose, validate, encode

// DecisionInput is everything ComposeDecisionRecord reads — all of it FETCHED by the
// closing tool: the issue (body, creation time), its comment thread, and the verified ruling.
type DecisionInput struct {
	Repo           string
	Issue          int
	Tracker        string // owner/repo#N (or !N) — where the decided work continues
	IssueBody      string
	IssueCreatedAt string    // RFC3339, as the forge reports it
	Comments       []Comment // the thread, in order (each with the forge's UpdatedAt)
	Ruling         Comment   // the verified ruling comment (its body and creation time)
	Now            time.Time // the recording time
	// The issue body's last edit time, RFC3339 ("" = never edited), and whether the forge
	// answered at all: IssueBodyEditKnown=false means the edit time could not be read, which
	// the record states as edit_state `unverifiable`, never as `unedited`.
	IssueBodyEditedAt  string
	IssueBodyEditKnown bool
}

// ComposeDecisionRecord builds the record. It does not validate: ValidateDecisionRecord does,
// and a record that fails it is never written.
func ComposeDecisionRecord(in DecisionInput) HumanDecisionRecord {
	// A ruling the thread does not carry has no "before": no relay is read and the pick is
	// unparsed (parseRulingPick), rather than every later comment counting as earlier.
	rulingIdx := DecisionRulingIndex(in.Comments, in.Ruling)
	var before []Comment
	if rulingIdx >= 0 {
		before = in.Comments[:rulingIdx]
	}
	anchorIdx := DecisionAnchor(before)

	rec := HumanDecisionRecord{
		Schema:        DecisionRecordSchema,
		Repo:          in.Repo,
		Issue:         in.Issue,
		Tracker:       in.Tracker,
		Options:       []DecisionOption{},
		OptionsSource: DecisionSourceBody,
		Recommended:   DecisionRecommendedNone,
		Ruler:         DecisionRuler,
		RulingSHA256:  Sha256Hex([]byte(in.Ruling.Body)),
		OpenedAt:      utcStamp(in.IssueCreatedAt),
		RuledAt:       utcStamp(in.Ruling.CreatedAt),
		RecordedAt:    in.Now.UTC().Format(time.RFC3339),
		ToolSHA:       decisionToolSHA(),
	}
	rec.Brief = parseDecisionBrief(in.IssueBody)

	opts := ParseDecisionOptions(in.IssueBody)
	rec.AskedAt = rec.OpenedAt
	if anchorIdx >= 0 {
		opts = ParseDecisionOptions(before[anchorIdx].Body)
		rec.OptionsSource = DecisionSourceRelay
		rec.AskedAt = utcStamp(before[anchorIdx].CreatedAt)
	}
	for i, l := range opts.Letters {
		rec.Options = append(rec.Options, DecisionOption{ID: l, TextSHA256: Sha256Hex([]byte(opts.Texts[i]))})
	}
	if opts.Recommended >= 0 {
		rec.Recommended = opts.Letters[opts.Recommended]
	}
	var relayIdx int
	rec.Picked, rec.PickedVia, relayIdx = parseRulingPick(in.IssueBody, in.Comments, anchorIdx, rulingIdx, in.Ruling)

	// The edit check: the record states what stood when the driver ruled, or says it cannot.
	// An edit after the ruling to any text it was read from leaves no definite pick and no
	// recommendation standing — the values read now may not be the ones the ruler saw.
	read := []Comment{}
	if anchorIdx >= 0 {
		read = append(read, before[anchorIdx])
	}
	if relayIdx >= 0 && relayIdx != anchorIdx {
		read = append(read, before[relayIdx])
	}
	var rulingRead *Comment
	if rulingIdx >= 0 {
		rulingRead = &in.Comments[rulingIdx]
	}
	rec.EditState = decisionEditState(in, rulingRead, read)
	if rec.EditState == DecisionEditEdited {
		rec.Picked, rec.Recommended = DecisionPickUnparsed, DecisionRecommendedNone
	}
	rec.PickedIsRecommended = rec.Recommended != DecisionRecommendedNone && rec.Picked == rec.Recommended

	opened, oerr := time.Parse(time.RFC3339, rec.OpenedAt)
	asked, aerr := time.Parse(time.RFC3339, rec.AskedAt)
	ruled, rerr := time.Parse(time.RFC3339, rec.RuledAt)
	if oerr == nil && rerr == nil {
		rec.OpenedToRuledS = int64(ruled.Sub(opened) / time.Second)
	}
	if aerr == nil && rerr == nil {
		rec.AskedToRuledS = int64(ruled.Sub(asked) / time.Second)
	}
	return rec
}

// decisionEditState compares every text the record was read from with the ruling's creation
// time: the issue body (always read — options, alternative lettering, brief), the anchor and
// resolving relay (read), and the ruling itself (edited at all after it was posted). Any edit
// after the ruling is `edited`; otherwise any edit time the forge did not report is
// `unverifiable`; otherwise `unedited`. A forge update time can move for reasons other than
// a text edit, so `edited` errs toward saying less, never more.
func decisionEditState(in DecisionInput, ruling *Comment, read []Comment) string {
	ruled, err := time.Parse(time.RFC3339, strings.TrimSpace(in.Ruling.CreatedAt))
	if err != nil || ruling == nil {
		return DecisionEditUnverifiable
	}
	edited, unknown := false, false
	after := func(stamp string, t time.Time) {
		u, err := time.Parse(time.RFC3339, strings.TrimSpace(stamp))
		switch {
		case err != nil:
			unknown = true
		case u.After(t):
			edited = true
		}
	}
	after(ruling.UpdatedAt, ruled)
	for _, c := range read {
		after(c.UpdatedAt, ruled)
	}
	switch {
	case !in.IssueBodyEditKnown:
		unknown = true
	case strings.TrimSpace(in.IssueBodyEditedAt) != "":
		after(in.IssueBodyEditedAt, ruled)
	}
	switch {
	case edited:
		return DecisionEditEdited
	case unknown:
		return DecisionEditUnverifiable
	}
	return DecisionEditUnedited
}

// utcStamp normalises a forge timestamp to RFC3339 UTC; an unparseable or empty one is
// returned as given, for the validator to refuse by name.
func utcStamp(s string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}

// DecisionRecordError names the ONE field a record was refused on.
type DecisionRecordError struct {
	Field  string
	Reason string
}

func (e *DecisionRecordError) Error() string {
	return fmt.Sprintf("human-decision-v1 record invalid (%s): %s", e.Field, e.Reason)
}

func decRefuse(field, reason string) error { return &DecisionRecordError{Field: field, Reason: reason} }

var (
	decOptionIDRe = regexp.MustCompile(`^[A-D1-4]$`)
	decSHA256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ValidateDecisionRecord refuses a record that could mislead or leak: ruler other than the
// role `driver`; picked outside the offered ids ∪ {ambiguous, unparsed}; an unknown
// options_source / picked_via / edit_state; an edited record with a definite pick or
// recommendation; a brief outside the brief-id grammar; more than four options, a duplicate
// or malformed option id or a malformed digest; latency that disagrees with the timestamps or
// is negative; a newline in any field. The error is a *DecisionRecordError
// naming the field.
func ValidateDecisionRecord(r HumanDecisionRecord) error {
	strs := []struct{ name, v string }{
		{"schema", r.Schema}, {"repo", r.Repo}, {"tracker", r.Tracker}, {"brief", r.Brief},
		{"options_source", r.OptionsSource}, {"recommended", r.Recommended}, {"picked", r.Picked},
		{"picked_via", r.PickedVia}, {"edit_state", r.EditState}, {"ruler", r.Ruler}, {"ruling_sha256", r.RulingSHA256},
		{"opened_at", r.OpenedAt}, {"asked_at", r.AskedAt}, {"ruled_at", r.RuledAt},
		{"recorded_at", r.RecordedAt}, {"tool_sha", r.ToolSHA},
	}
	for _, o := range r.Options {
		strs = append(strs, struct{ name, v string }{"options", o.ID}, struct{ name, v string }{"options", o.TextSHA256})
	}
	for _, s := range strs {
		if strings.ContainsAny(s.v, "\r\n") {
			return decRefuse(s.name, "contains a newline")
		}
	}
	switch {
	case r.Schema != DecisionRecordSchema:
		return decRefuse("schema", "must be "+DecisionRecordSchema)
	case r.Ruler != DecisionRuler:
		return decRefuse("ruler", "must be the role string "+DecisionRuler+", never a login or id")
	case r.Repo == "":
		return decRefuse("repo", "empty")
	case r.Issue <= 0:
		return decRefuse("issue", "must be a positive issue number")
	case r.Tracker == "":
		return decRefuse("tracker", "empty")
	case len(r.Options) > maxDecisionOptions:
		return decRefuse("options", fmt.Sprintf("%d options; at most %d", len(r.Options), maxDecisionOptions))
	case r.Brief != "" && !decBriefIDRe.MatchString(r.Brief):
		return decRefuse("brief", "must be a brief id <stream>/<NN> (lower-case stream, at most 64 characters) or absent")
	}
	ids := map[string]bool{}
	for _, o := range r.Options {
		if !decOptionIDRe.MatchString(o.ID) || ids[o.ID] {
			return decRefuse("options", "id must be one distinct letter A-D or digit 1-4")
		}
		if !decSHA256Re.MatchString(o.TextSHA256) {
			return decRefuse("options", "text_sha256 must be a sha256 hex digest")
		}
		ids[o.ID] = true
	}
	switch r.OptionsSource {
	case DecisionSourceBody, DecisionSourceRelay:
	default:
		return decRefuse("options_source", "must be body or relay")
	}
	if r.Recommended != DecisionRecommendedNone && !ids[r.Recommended] {
		return decRefuse("recommended", "must be an offered id or none")
	}
	if r.Picked != DecisionPickAmbiguous && r.Picked != DecisionPickUnparsed && !ids[r.Picked] {
		return decRefuse("picked", "must be an offered id, ambiguous or unparsed")
	}
	switch r.PickedVia {
	case DecisionViaDirect, DecisionViaRatified:
	default:
		return decRefuse("picked_via", "must be direct or ratified-relay")
	}
	switch r.EditState {
	case DecisionEditUnedited, DecisionEditUnverifiable:
	case DecisionEditEdited:
		if r.Picked != DecisionPickUnparsed || r.Recommended != DecisionRecommendedNone {
			return decRefuse("edit_state", "an edited record carries picked unparsed and recommended none")
		}
	default:
		return decRefuse("edit_state", "must be unedited, edited or unverifiable")
	}
	if want := ids[r.Picked] && r.Picked == r.Recommended; r.PickedIsRecommended != want {
		return decRefuse("picked_is_recommended", "disagrees with picked and recommended")
	}
	if !decSHA256Re.MatchString(r.RulingSHA256) {
		return decRefuse("ruling_sha256", "must be a sha256 hex digest")
	}
	stamps := map[string]time.Time{}
	for _, s := range []struct{ name, v string }{
		{"opened_at", r.OpenedAt}, {"asked_at", r.AskedAt}, {"ruled_at", r.RuledAt}, {"recorded_at", r.RecordedAt},
	} {
		t, err := time.Parse(time.RFC3339, s.v)
		if err != nil || t.UTC().Format(time.RFC3339) != s.v {
			return decRefuse(s.name, "must be an RFC3339 UTC timestamp")
		}
		stamps[s.name] = t
	}
	for _, l := range []struct {
		name, from string
		got        int64
	}{
		{"opened_to_ruled_s", "opened_at", r.OpenedToRuledS},
		{"asked_to_ruled_s", "asked_at", r.AskedToRuledS},
	} {
		want := int64(stamps["ruled_at"].Sub(stamps[l.from]) / time.Second)
		if l.got < 0 || want < 0 {
			return decRefuse(l.name, "negative latency")
		}
		if l.got != want {
			return decRefuse(l.name, fmt.Sprintf("is %d but ruled_at - %s is %d", l.got, l.from, want))
		}
	}
	if r.ToolSHA == "" {
		return decRefuse("tool_sha", "empty")
	}
	return nil
}

// EncodeDecisionRecord validates and encodes the record as ONE JSON line (no trailing
// newline). The same bytes go into the close comment's hidden block and the local file.
// encoding/json escapes <, > and &, so the line can never close the HTML comment around it.
func EncodeDecisionRecord(r HumanDecisionRecord) ([]byte, error) {
	if err := ValidateDecisionRecord(r); err != nil {
		return nil, err
	}
	return json.Marshal(r)
}

// DecisionRecordBlock wraps an encoded record as the hidden block appended to the close
// comment: `<!-- human-decision-v1 {…} -->`.
func DecisionRecordBlock(line []byte) string {
	return "<!-- " + DecisionRecordSchema + " " + string(line) + " -->"
}

// AppendDecisionRecord appends one encoded record as one line to decision-records.jsonl in
// the desk state directory (0600, O_APPEND), beside the audit log. A symlink or any other
// non-regular file at that path is refused rather than written through, and an existing file
// is brought back to 0600. The write is ONE short O_APPEND write; there is no lock.
func AppendDecisionRecord(line []byte) error {
	if len(line) == 0 || strings.ContainsAny(string(line), "\r\n") || !json.Valid(line) {
		return Refused("refusing to append a decision record that is not one JSON line")
	}
	dir, err := deskDir()
	if err != nil {
		return Unverifiable("cannot resolve the desk state directory for decision-records.jsonl", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Unverifiable("cannot create the desk state directory", err)
	}
	p := filepath.Join(dir, decisionRecordsFile)
	if fi, err := os.Lstat(p); err == nil && !fi.Mode().IsRegular() {
		return Refused("refusing to append to decision-records.jsonl: it is not a regular file (" + fi.Mode().Type().String() + ")")
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Unverifiable("cannot open decision-records.jsonl", err)
	}
	// The path is checked again after the open: a link swapped in between the check and the
	// open would make the opened file differ from what the path names now.
	if lfi, lerr := os.Lstat(p); lerr != nil || !lfi.Mode().IsRegular() {
		_ = f.Close()
		return Refused("refusing to append to decision-records.jsonl: the path changed while it was opened")
	} else if ffi, ferr := f.Stat(); ferr != nil || !os.SameFile(lfi, ffi) {
		_ = f.Close()
		return Refused("refusing to append to decision-records.jsonl: the opened file is not the file at the path")
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return Unverifiable("cannot set decision-records.jsonl to 0600", err)
	}
	if _, err := f.Write(append(append([]byte(nil), line...), '\n')); err != nil {
		_ = f.Close()
		return Unverifiable("cannot append to decision-records.jsonl", err)
	}
	if err := f.Close(); err != nil {
		return Unverifiable("cannot close decision-records.jsonl", err)
	}
	return nil
}

// decisionToolSHA is the build's source SHA — the value every audit Entry carries
// ("unpinned" for an unstamped build).
func decisionToolSHA() string {
	s, _ := Version()
	return s
}
