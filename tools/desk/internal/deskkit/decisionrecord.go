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
	decRecommendWordRe  = regexp.MustCompile(`(?i)recommend`)
	// Only the leading letter of "Recommended" case-varies, exactly as the oracle's ERE.
	decTrailingRecRe = regexp.MustCompile(`[ \t]*[—-]?[ \t]*\(?[Rr]ecommended\)?[ \t]*$`)
	decLeadingRecRe  = regexp.MustCompile(`^\(?[Rr]ecommended\)?[ \t,:;—-]*`)
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
// heading, the first four lettered list lines, the first one mentioning "recommend" as
// the recommended option, each text de-markdowned, control-stripped and freed of its
// "(recommended)" marker.
func ParseDecisionOptions(body string) DecisionOptions {
	var bl []string
	for _, l := range decLines(body) {
		if !decHTMLCommentRe.MatchString(l) {
			bl = append(bl, l)
		}
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
	rec := -1
	for i, o := range raw {
		if decRecommendWordRe.MatchString(o.txt) {
			rec = i
			break
		}
	}
	out := DecisionOptions{Recommended: -1}
	for i, o := range raw {
		v := decStrip(decClean(decDemd(o.txt)))
		v = decTrailingRecRe.ReplaceAllString(v, "")
		v = decLeadingRecRe.ReplaceAllString(v, "")
		v = decStrip(v)
		if v == "" {
			continue // dropped, as the oracle drops it; a dropped mark is no mark
		}
		if i == rec {
			out.Recommended = len(out.Texts)
		}
		out.Letters = append(out.Letters, o.let)
		out.Texts = append(out.Texts, v)
	}
	return out
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

// The ruling grammar — the oracle's `isruling` (assay-inbox.sh): the first content line
// names an offered letter ("B", "B.", "Option C — …") or ratifies ("ratified", "I ratify",
// "approved"); a question, a refusal or a hold anywhere in the comment is not a ruling.
var (
	rulingLetterRe  = regexp.MustCompile(`^(?:[Oo]ption\s+)?([A-D1-4])(?:$|[.)!:,]|\s*[—–-])`)
	rulingRatifyRe  = regexp.MustCompile(`(?i)^(?:i\s+)?(?:ratif(?:y|ied)|approved?)\b`)
	rulingNegatesRe = regexp.MustCompile(`(?i)\b(?:no|not|don'?t|didn'?t|won'?t|hold|wait|later|undecided|unsure|pending)\b|n't\b`)
	relayAnswerRe   = regexp.MustCompile(`\b[Aa]nswer:\s*([A-D1-4])(?:$|[^A-Za-z0-9])`)
	decisionBriefRe = regexp.MustCompile(`<!-- needs-decision: ([^>\s][^>]*?) -->`)
)

// isTrustedApp reports a roster-trusted APP account (login AND id), not minimized — the only
// author whose comment can be an ask (anchor) or a relay a ratification resolves through.
func isTrustedApp(c Comment) bool {
	l := strings.ToLower(c.Author.Login)
	if !strings.HasSuffix(l, "[bot]") && !strings.HasPrefix(l, "app/") {
		return false
	}
	return !c.Minimized && TrustedAuthorID(c.Author.Login, c.Author.ID)
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

// ParseRulingPick resolves the ruling to an offered id of the anchor ask, or to one of the
// closed tokens `ambiguous` / `unparsed`, and says how it was picked.
//
//   - body is the issue body; comments is the thread in order, and rulingIdx the ruling's
//     position in it (only comments before it are read); anchorIdx is DecisionAnchor's answer
//     over those comments (-1 = the body).
//   - A DIRECT letter is read in the anchor's lettering. When the anchor is a relay and the
//     body states options too, the body's own lettering and the inbox walk's re-lettering of
//     it are alternatives the ruler could have read: a letter they disagree on is ambiguous.
//     A direct letter against a BODY anchor is the body's own letter.
//   - A RATIFICATION resolves through the newest roster-trusted App comment before the ruling
//     carrying `Answer: <letter>`; an untrusted or minimized relay is never read, so with no
//     trusted relay the pick is unparsed. When that relay is itself the anchor, its letter is
//     the relay's own; otherwise the letter may be the walk's re-lettering of the anchor (and,
//     for a relay anchor, the body's) — a disagreement is ambiguous.
//   - A question or a refusal/hold anywhere in the ruling is unparsed.
func ParseRulingPick(body string, comments []Comment, anchorIdx, rulingIdx int, ruling Comment) (picked, via string) {
	if rulingIdx < 0 || rulingIdx > len(comments) {
		rulingIdx = len(comments)
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
	all := strings.Join(ls, " ")
	if len(ls) == 0 || strings.Contains(all, "?") || rulingNegatesRe.MatchString(all) {
		return DecisionPickUnparsed, DecisionViaDirect
	}
	first := ls[0]

	if m := rulingLetterRe.FindStringSubmatch(first); m != nil {
		var alts []lettering
		if anchorIdx >= 0 && bodyOpts.Stated() {
			alts = append(alts, sourceLettering(bodyOpts), walkLettering(bodyOpts))
		}
		return resolveLetter(m[1], anchor, alts), DecisionViaDirect
	}

	if !rulingRatifyRe.MatchString(first) {
		return DecisionPickUnparsed, DecisionViaDirect
	}
	for i := len(before) - 1; i >= 0; i-- {
		c := before[i]
		if !isTrustedApp(c) {
			continue
		}
		m := relayAnswerRe.FindStringSubmatch(strings.Join(rulingContentLines(c.Body), "\n"))
		if m == nil {
			continue
		}
		var alts []lettering
		if i != anchorIdx {
			alts = append(alts, walkLettering(anchor))
			if anchorIdx >= 0 && bodyOpts.Stated() {
				alts = append(alts, sourceLettering(bodyOpts), walkLettering(bodyOpts))
			}
		}
		return resolveLetter(m[1], anchor, alts), DecisionViaRatified
	}
	return DecisionPickUnparsed, DecisionViaRatified
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
	Comments       []Comment // the thread, in order
	Ruling         Comment   // the verified ruling comment (its body and creation time)
	Now            time.Time // the recording time
}

// ComposeDecisionRecord builds the record. It does not validate: ValidateDecisionRecord does,
// and a record that fails it is never written.
func ComposeDecisionRecord(in DecisionInput) HumanDecisionRecord {
	rulingIdx := len(in.Comments)
	for i, c := range in.Comments {
		if (in.Ruling.DatabaseID != 0 && c.DatabaseID == in.Ruling.DatabaseID) ||
			(in.Ruling.ID != "" && c.ID == in.Ruling.ID) {
			rulingIdx = i
			break
		}
	}
	before := in.Comments[:rulingIdx]
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
	if m := decisionBriefRe.FindStringSubmatch(in.IssueBody); m != nil {
		rec.Brief = m[1]
	}

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
	rec.Picked, rec.PickedVia = ParseRulingPick(in.IssueBody, in.Comments, anchorIdx, rulingIdx, in.Ruling)
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
// options_source / picked_via; more than four options; latency that disagrees with the
// timestamps or is negative; a newline in any field. The error is a *DecisionRecordError
// naming the field.
func ValidateDecisionRecord(r HumanDecisionRecord) error {
	strs := []struct{ name, v string }{
		{"schema", r.Schema}, {"repo", r.Repo}, {"tracker", r.Tracker}, {"brief", r.Brief},
		{"options_source", r.OptionsSource}, {"recommended", r.Recommended}, {"picked", r.Picked},
		{"picked_via", r.PickedVia}, {"ruler", r.Ruler}, {"ruling_sha256", r.RulingSHA256},
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
// the desk state directory (0600, O_APPEND), beside the audit log.
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
	f, err := os.OpenFile(filepath.Join(dir, decisionRecordsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Unverifiable("cannot open decision-records.jsonl", err)
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
