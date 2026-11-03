// Package adapter maps GitHub objects onto the platform-free domain types
// (design §3.4): issue → Escalation, PR → WorkItem, label → Gate,
// review → Verdict, login → Actor.
//
// Everything platform-shaped stops at this package boundary. Everything
// UNCERTAIN is labelled here too: the adapter is the only layer that knows
// whether a fact was read, derived, or guessed at, so it is the only layer
// that can honestly stamp a domain.Assessment on it.
package adapter

import (
	"strings"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// OptionsMarker is the grammar version marker (spec §1.4). Its PRESENCE
// selects typed-v2 parsing; its absence is prose-v1 and means
// `Options = nil` — exactly today's semantics, with no migration and no
// retroactive edit of any open issue.
const OptionsMarker = "<!-- assay:options v2 -->"

// optionSep is the id/label separator in a v2 option heading: U+2014 EM DASH
// with a single space either side, per the spec's literal grammar block.
//
// This is a byte-level fact worth stating: a parser matching ASCII "-" or
// "--" here fails on every real body, and fails SILENTLY — it would find the
// marker, find no options, and render an empty typed list. That failure mode
// is why [ParseOptions] distinguishes "marker present, nothing parsed" from
// "no marker" and refuses to publish the first as an empty list.
const optionSep = " — "

// consequencePrefix opens the one line that says what happens if a choice is
// taken. The FIRST such line in an option block is the consequence; later
// ones fall to rationale (spec §1.4).
const consequencePrefix = "Consequence:"

// OptionsEncoding names which of the two wire encodings a body used.
type OptionsEncoding string

const (
	// EncodingProseV1 — no marker. Options is nil; the fork lives in prose.
	// This is not a degraded parse, it is the other half of a dual contract.
	EncodingProseV1 OptionsEncoding = "prose-v1"

	// EncodingTypedV2 — marker present and the block parsed.
	EncodingTypedV2 OptionsEncoding = "typed-v2"

	// EncodingTypedV2Malformed — marker present, block did NOT parse.
	// Design §10's malformed-escalation rule applies: the item is rendered
	// RAW and never dropped, because an unparseable question is still a
	// stalled loop. The row is served with options at could-not-check.
	EncodingTypedV2Malformed OptionsEncoding = "typed-v2-malformed"
)

// ParseResult is the outcome of reading an escalation body's options.
type ParseResult struct {
	Encoding OptionsEncoding
	Options  []domain.Option
	Rev      string
	// Rationale is the free prose retained beside the options (spec §1.1.1):
	// it explains the options, it is no longer how a choice is expressed.
	Rationale string
	// Problems names why a v2 block did not parse. Non-empty exactly when
	// Encoding is EncodingTypedV2Malformed.
	Problems []string
}

// bodyProbe is the declared source for anything derived from an issue body.
// One source per fact (derive-or-diff): the body as GitHub returned it on
// this poll, and nothing else. Notably NOT the issue title, NOT a comment,
// and NOT a previously indexed copy.
func bodyProbe(window string) domain.Probe {
	return domain.Probe{
		Source: "gh api graphql: repository.issue(number:).body — the body as returned by this poll",
		Method: domain.MethodGraphQLAliased,
		Window: window,
		Limit:  0, // a single object fetch; there is no list and no cap
	}
}

// Assessment converts the parse outcome into a three-state verdict for the
// `options` field.
//
// prose-v1 is CHECKED-CLEAN with a nil list: the adapter looked, and there
// genuinely are no typed options. A malformed v2 block is COULD-NOT-CHECK:
// the adapter looked, found a claim that typed options exist, and could not
// read them. Collapsing those two into "options: null, everything fine"
// would silently downgrade a broken escalation into a legacy one.
func (r ParseResult) Assessment() domain.Assessment {
	switch r.Encoding {
	case EncodingProseV1:
		return domain.Clean(bodyProbe("body carries no " + OptionsMarker + " marker ⇒ prose-v1, Options = nil (spec §1.4 back-compat)"))
	case EncodingTypedV2:
		return domain.Clean(bodyProbe("body carries the v2 marker; option headings parsed"))
	default:
		return domain.Unchecked(
			bodyProbe("body carries the v2 marker; option block did not parse"),
			"typed-v2 marker present but unparseable ("+strings.Join(r.Problems, "; ")+
				") — rendered raw, never dropped (design §10 malformed-escalation rule)")
	}
}

// ParseOptions reads an escalation body under BOTH wire encodings (ruling D6:
// the adapter parses v1 and v2 from day one, so an amendment to the
// escalation wire contract never blocks this parser in either direction).
//
// The grammar (spec §1.4), reproduced so the parser and the spec can be
// diffed by eye:
//
//	### Options
//	<!-- assay:options v2 -->
//	#### `ship-partial` — Ship the partial index
//	Consequence: inbox live today; roadmap pane lags a week.
//	Pros: … Cons: … (free prose — rationale)
//
//	#### `wait-for-fix` — Hold until #NNN lands
//	Consequence: nothing ships this week; no rework risk.
//
// Rules: the marker versions the grammar; each `#### `<id>` — <label>`
// heading opens an option; the FIRST `Consequence:` line is its consequence;
// remaining prose is rationale.
//
// Three things this deliberately does NOT do:
//
//   - It does not infer options from prose when the marker is absent. A
//     guessed option list is worse than none: the recorded answer binds by
//     id, and an id nobody wrote is an id nobody meant.
//   - It does not partially publish. If any heading in a marked block is
//     malformed, the whole block is malformed — publishing the readable
//     subset would number the options wrongly for §2's number keys.
//   - It does not repair the body. Open issues are never edited
//     retroactively to inject v2 grammar (spec §1.4).
func ParseOptions(body string) ParseResult {
	if !strings.Contains(body, OptionsMarker) {
		return ParseResult{Encoding: EncodingProseV1, Options: nil, Rev: "", Rationale: strings.TrimSpace(body)}
	}

	_, after, _ := strings.Cut(body, OptionsMarker)
	lines := strings.Split(after, "\n")

	var (
		opts     []domain.Option
		problems []string
		rat      []string
		cur      *domain.Option
		curHasC  bool
		seen     = map[domain.OptionID]bool{}
	)

	flush := func() {
		if cur == nil {
			return
		}
		if !curHasC {
			problems = append(problems, "option `"+string(cur.ID)+"` has no `"+consequencePrefix+"` line")
		}
		opts = append(opts, *cur)
		cur = nil
		curHasC = false
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)

		// A heading at H3 or shallower ends the options block: the grammar
		// nests options under `### Options`, so the next same-or-higher
		// heading is a different section.
		if isHeadingAtOrAbove(trimmed, 3) {
			flush()
			break
		}

		if strings.HasPrefix(trimmed, "#### ") {
			flush()
			id, label, err := parseOptionHeading(trimmed)
			if err != "" {
				problems = append(problems, err)
				continue
			}
			if seen[id] {
				problems = append(problems, "duplicate option id `"+string(id)+"` — ids must be unique within one escalation (spec §1.1.1)")
				continue
			}
			seen[id] = true
			cur = &domain.Option{ID: id, Label: label}
			continue
		}

		if cur != nil && !curHasC && strings.HasPrefix(trimmed, consequencePrefix) {
			cur.Consequence = strings.TrimSpace(strings.TrimPrefix(trimmed, consequencePrefix))
			curHasC = true
			continue
		}

		if trimmed != "" {
			rat = append(rat, trimmed)
		}
	}
	flush()

	if len(opts) == 0 {
		problems = append(problems, "marker present but no `#### `<id>` — <label>` heading parsed "+
			"(note: the id/label separator is U+2014 EM DASH, not an ASCII hyphen)")
	}

	res := ParseResult{Rationale: strings.Join(rat, "\n")}
	if len(problems) > 0 {
		// Malformed: keep whatever parsed for a human to look at, but do
		// NOT publish it as the option set — a partial list would misnumber
		// the number keys the answer flow binds to.
		res.Encoding = EncodingTypedV2Malformed
		res.Problems = problems
		res.Options = nil
		res.Rev = ""
		return res
	}
	res.Encoding = EncodingTypedV2
	res.Options = opts
	res.Rev = domain.OptionsRev(opts)
	return res
}

// isHeadingAtOrAbove reports whether line is an ATX heading of level <= n.
func isHeadingAtOrAbove(line string, n int) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	return level <= n && level < len(line) && line[level] == ' '
}

// parseOptionHeading reads `#### `<id>` — <label>`. It returns a non-empty
// problem string instead of an error value because every caller wants the
// human-readable reason, not a sentinel.
func parseOptionHeading(line string) (domain.OptionID, string, string) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "####"))

	if !strings.HasPrefix(rest, "`") {
		return "", "", "option heading " + quote(line) + " does not open with a backticked id"
	}
	idPart, labelPart, found := strings.Cut(rest[1:], "`")
	if !found {
		return "", "", "option heading " + quote(line) + " has an unclosed backtick around its id"
	}

	id := domain.OptionID(idPart)
	if !id.Valid() {
		return "", "", "option id `" + idPart + "` does not match the slug grammar [a-z0-9][a-z0-9-]{1,31} (spec §1.1)"
	}

	if !strings.HasPrefix(labelPart, optionSep) {
		return "", "", "option heading " + quote(line) + " does not separate id from label with an em-dash (U+2014) surrounded by single spaces"
	}
	label := strings.TrimSpace(strings.TrimPrefix(labelPart, optionSep))
	if label == "" {
		return "", "", "option `" + idPart + "` has an empty label"
	}
	return id, label, ""
}

func quote(s string) string { return "\"" + s + "\"" }
