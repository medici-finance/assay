package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// verifiedCellRe matches a dated runner in a Verified column: "YYYY-MM-DD <token...>".
var verifiedCellRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \S`)

// verifiedTokenRe captures the first whitespace-token immediately after the
// leading date in a Verified cell (e.g. "2026-07-08 sonnet-verifier" -> "sonnet-verifier").
var verifiedTokenRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s+(\S+)`)

// separatorRowRe matches a markdown table separator row ("|---|:--|--:|"),
// i.e. a row containing only pipes, dashes, colons and whitespace.
var separatorRowRe = regexp.MustCompile(`^[\s:|-]+$`)

// belowFloorModels is the SINGLE, explicit list of MODEL FAMILIES that sit
// below the risk-keyed verifier floor (methodology/19) — the runners a
// risk-flagged brief may NOT be verified by.
//
// The criterion is CAPABILITY, NOT PRICE. This distinction is the whole point
// of the list and the reason it is an explicit named set rather than a pattern:
// the floor exists to stop a model that is not strong enough to actually
// re-run a Verify table from rubber-stamping a risk-flagged brief. A model that
// is merely inexpensive to call is not thereby weak, and must not be listed
// here. Concretely: `glm-5.2` is cheap on PRICE and strong on CAPABILITY, so it
// is deliberately absent — an earlier `glm` entry in this list conflated the
// two and produced false rejections of legitimate verifications.
//
// The keys are model-family names, matched against the NAME SEGMENTS of the
// runner token (see belowFloorRunner), never as bare substrings. Substring
// matching is what made the old list unsafe: `mini` silently swallows
// `gemini-*`, `lite` swallows `elite-*`, and `glm` swallowed every `glm-5.2-*`
// verifier. `mini`/`flash`/`lite` stay because they are real family names of
// small models (`gpt-4o-mini`, `gemini-2.5-flash`, `…-lite`), and segment
// matching now makes them mean exactly that.
//
// To CHANGE the floor, add or remove a family here — this is the one place the
// list lives. Add a family only if you are claiming it is too WEAK to verify a
// risk-flagged brief; never because it is cheap to run.
var belowFloorModels = map[string]bool{
	"deepseek": true,
	"sonnet":   true,
	"haiku":    true,
	"mini":     true,
	"flash":    true,
	"lite":     true,
}

// belowFloorRunner reports whether a runner token names a model family from
// belowFloorModels. The token is lowercased and split into name segments on
// any non-alphanumeric separator, and each segment has its trailing version
// digits stripped, so "sonnet-verifier", "via-deepseek", "claude-haiku-4.5"
// and "sonnet5" all match their family while "glm-5.2-verifier",
// "gemini-verifier" and "elite-verifier" do not.
func belowFloorRunner(token string) bool {
	segs := strings.FieldsFunc(strings.ToLower(token), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, seg := range segs {
		if belowFloorModels[strings.TrimRight(seg, "0123456789")] {
			return true
		}
	}
	return false
}

// humanRunnerName returns the <name> of a runner token of the form
// "human:<name>", and ok=false when the token is not a human stamp at all.
//
// The prefix test is exact and at the START of the token, matching
// hasHumanReviewer and the anchored humanStampRe in corroborate.go: neither
// "superhuman:ian" nor "non-human:ian" is a human stamp (assay-toolkit#243).
// The name is the leading ASCII-word run after the colon, so trailing
// punctuation ("human:ian,") is tolerated while a confusable/homoglyph name
// ("human:іan", Cyrillic і) yields an EMPTY name — which then fails the
// known-human lookup below rather than passing unexamined.
func humanRunnerName(token string) (name string, ok bool) {
	const prefix = "human:"
	if !strings.HasPrefix(strings.ToLower(token), prefix) {
		return "", false
	}
	rest := token[len(prefix):]
	end := 0
	for end < len(rest) {
		c := rest[end]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' {
			end++
			continue
		}
		break
	}
	return rest[:end], true
}

// verifierFloorFailure reports whether a Verified cell FAILS the verifier floor
// (methodology/19), and if so returns a self-contained reason clause the caller
// can splice into its problem line. A cell with no dated runner returns
// ("", false): that gap is attributionProblems' to report, not the floor's.
//
// THE #280 FIX — an unbacked `human:` token no longer buys the exemption.
// The previous implementation opened with `if hasHumanReviewer(verified) { return
// ..., false }` — a blanket early return on a "human:" token appearing ANYWHERE in
// the cell. That is defensible on its face (if a human ran the table, tier does not
// apply) and was load-bearing in exactly the wrong way: appending " human:ian" to a
// Verified cell silently switched the whole control off, whether or not a human ran
// anything. `2026-07-31 sonnet-verifier human:ian` cleared the floor. The desk found
// a false `human:ian` in the Verified cell on four separate PRs in one day
// (oit#1601, #1439, #1442, #1607); on #1607 the false attribution was not sitting
// next to a passing gate, it WAS the thing keeping the gate from firing, and
// correcting the cell to the honest runner turned CI red immediately. The perverse
// incentive that creates — the cheapest way to clear the new red is to put
// `human:<name>` back — is why this needs a mechanism and not vigilance.
//
// The exemption is now scoped to the one thing it was ever meant to mean: the
// RUNNER token itself — the first token after the date, i.e. WHO RAN THE TABLE —
// is a human whose name resolves in the configured human-login map. That preserves the legitimate
// accept (a genuine `2026-07-31 human:ian` still clears, and so does
// `human:ian (sonnet-assisted)`) while the forgery (`sonnet-verifier human:ian`)
// is caught, because the human token is no longer standing where the runner goes.
//
// An UNRESOLVABLE human runner token fails LOUD rather than passing silently. Note
// that it would otherwise pass by accident: belowFloorRunner("human:bob") is false
// — "human" and "bob" are not model families — so an unknown or homoglyph name
// would clear the floor with no message at all. Per assay-toolkit#280: a token that
// cannot be validated is not "gate satisfied".
//
// The Verified/Reviewed split is deliberately NOT enforced here. House convention
// is that Verified names who ran the table and Reviewed carries the human sign-off,
// so a `human:` token in Verified is arguably always misplaced — but a human really
// can run a Verify table, and flagging that would be a new false rejection. This
// change only stops the token from SUPPRESSING a check it does not speak to.
func verifierFloorFailure(verified string) (reason string, failed bool) {
	m := verifiedTokenRe.FindStringSubmatch(verified)
	if m == nil {
		return "", false
	}
	runner := m[1]

	if name, isHuman := humanRunnerName(runner); isHuman {
		if _, known := HumanLogin(name); known {
			return "", false // a human really did run it — the floor does not apply
		}
		return fmt.Sprintf("the runner token %q claims a human, but %q resolves to no known human "+
			"(the configured "+scanEnvHumanLoginMap+" map) — an unverifiable human token does not satisfy the floor; "+
			"name the runner that actually ran the table, or add the human to the map (itself a reviewed change)",
			runner, name), true
	}

	if belowFloorRunner(runner) {
		return fmt.Sprintf("runner %q is a model family the floor rejects on capability, not price", runner), true
	}
	return "", false
}

// attributionProblems is the brief-16 runner-attribution check: a `verified`/
// `done` brief-v1 brief's verification must be attributable to a runner
// distinct from the implementer. Scope is brief-v1 files only (the same
// schema opt-in gate as checkBriefFiles) whose README-row status is
// `verified` or `done` — legacy no-frontmatter briefs and todo/in-progress/
// implemented rows are exempt. It reuses parseBriefFile/briefFilePaths/
// expectedBriefID from brieffile.go rather than re-parsing brief files.
//
// This is BEST-EFFORT and says so in its checks, not just this comment:
// `authored:` names the brief's AUTHOR, not necessarily who later ran the
// verification, and every session in this repo shares one git identity — a
// file-level check cannot see true cross-session independence. It catches
// the detectable cases (missing dated runner, a verifier token matching the
// author or reading as "the implementer"/"self", and Evidence rows that are
// all implementer-attributed) and nothing more (brief-16 scope-honesty note).
func attributionProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				// Malformed files are reported by checkBriefFiles; legacy/
				// opted-out files are exempt here as everywhere else.
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}

			var row *Brief
			for i := range s.Briefs {
				if s.Briefs[i].Num == num {
					row = &s.Briefs[i]
					break
				}
			}
			if row == nil || (row.Status != "verified" && row.Status != "done") {
				continue // no row, or a status this check does not apply to
			}

			label := fmt.Sprintf("%s/brief-%s", s.Name, num)

			if !verifiedCellRe.MatchString(row.Verified) {
				add(`%s: Verified cell must name a dated runner ("YYYY-MM-DD <runner>")`, label)
				continue // no dated runner to extract a token from
			}

			if reason := selfVerificationReason(bf.Authored, row.Verified); reason != "" {
				add("%s: Verified runner looks like self-verification (%s)", label, reason)
			}

			if !evidenceHasIndependentRow(bf.Evidence) {
				add("%s: verified requires an independent (non-implementer) Evidence row", label)
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// authorToken extracts the author's identifying token from an `authored:`
// frontmatter value of the form "YYYY-MM-DD by <author> ...". ok is false
// when there is no " by " marker to key off (e.g. a bare-date authored value)
// — the self-verification comparison is then skipped rather than guessed at.
func authorToken(authored string) (tok string, ok bool) {
	_, after, found := strings.Cut(authored, " by ")
	if !found {
		return "", false
	}
	fields := strings.Fields(after)
	if len(fields) == 0 {
		return "", false
	}
	return strings.ToLower(fields[0]), true
}

// selfVerificationReason reports why a Verified cell reads as self-verification,
// or "" if it doesn't. Caller must have already confirmed verifiedCellRe matches.
func selfVerificationReason(authored, verified string) string {
	m := verifiedTokenRe.FindStringSubmatch(verified)
	if m == nil {
		return ""
	}
	verifierTok := strings.ToLower(m[1])

	if authorTok, ok := authorToken(authored); ok && verifierTok == authorTok {
		return fmt.Sprintf("verifier %q matches the brief's author %q", verifierTok, authorTok)
	}
	if implementerAttributed(verifierTok) {
		return fmt.Sprintf("verifier %q names the implementer, not an independent runner", verifierTok)
	}
	if verifierTok == "self" {
		return fmt.Sprintf("verifier %q is literally \"self\"", verifierTok)
	}
	return ""
}

// implementerAttributed reports whether a Runner cell names the implementer.
// "non-implementer" asserts the opposite and must not match — strip it before
// the substring test so `sonnet verifier (non-implementer)` reads as
// independent while `implementer (Opus 4.8)` still reads as implementer-run.
func implementerAttributed(runnerCell string) bool {
	return strings.Contains(strings.ReplaceAll(runnerCell, "non-implementer", ""), "implementer")
}

// evidenceHasIndependentRow reports whether an Evidence section contains at
// least one markdown table content row (outside HTML comments, beyond a
// header/separator pair) whose last (Runner) cell is not implementer-
// attributed. A section may hold more than one table (e.g. an
// "implementer run" table followed by an "independent re-run" table); each
// table's own header+separator pair is skipped in turn.
func evidenceHasIndependentRow(evidence string) bool {
	stripped := htmlCommentRe.ReplaceAllString(evidence, "")
	lines := strings.Split(stripped, "\n")
	runnerIdx := -1 // -1 = no header seen yet for this table: use the last cell
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			runnerIdx = -1 // table ended; the next table names its own columns
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue // an orphan separator row
		}
		// A header row is immediately followed by a separator row; note where
		// its "Runner" column sits (if named), then skip both rows.
		if i+1 < len(lines) && separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			runnerIdx = -1
			for j, c := range splitRow(line) {
				if strings.EqualFold(strings.TrimSpace(c), "runner") {
					runnerIdx = j
				}
			}
			i++
			continue
		}
		cells := splitRow(line)
		if len(cells) == 0 {
			continue
		}
		idx := len(cells) - 1
		if runnerIdx >= 0 && runnerIdx < len(cells) {
			idx = runnerIdx
		}
		runner := strings.ToLower(strings.TrimSpace(cells[idx]))
		if runner == "" {
			continue
		}
		if !implementerAttributed(runner) {
			return true
		}
	}
	return false
}
