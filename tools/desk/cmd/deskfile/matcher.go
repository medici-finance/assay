package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Matcher + dedupe-search backend. The design is deliberately simple
// and PRINTS THE SCORE: the human-audit trail matters more than matcher cleverness. A
// subtle matcher error that silently refused legitimate filings OR silently admitted
// duplicates would each be bad, but duplicate-MINTING is the
// expensive error (the fail-closed direction), so the threshold errs toward refusing.
//
// Two stages:
//  1. GitHub search scopes the candidate set to OPEN issues in the target repo whose text
//     matches the title tokens (best-effort; an API failure fails CLOSED for `new`).
//  2. A local Jaccard score over normalised token sets decides whether any candidate is a
//     likely duplicate, with a small boost when the candidate carries the `class` label
//     (a class issue is the canonical collector for its instances — a moderate token match
//     against one should redirect the filer to attach, which is the exact motion the gate
//     exists to force).

const (
	// classLabel is the label that marks a "class issue" — an open issue that collects
	// individual instances as comments rather than each getting its own issue.
	// deskfile only READS it; it does not create the label.
	classLabel = "class"

	// matchThreshold is the score at and above which a candidate counts as a likely
	// duplicate. 0.5 (Jaccard) means half the combined token set is shared — "foo bar
	// baz" vs "foo bar baz qux" scores 0.75, "foo bar" vs "baz qux quux" scores 0.0.
	matchThreshold = 0.5

	// classLabelBoost is added to a candidate's Jaccard score when it carries the `class`
	// label. The boost alone cannot trigger a match (0.0 + 0.15 < 0.5); it lifts a
	// moderate-overlap class issue past the threshold so the gate redirects to attach —
	// the motion the gate exists to force when the target is a class issue.
	classLabelBoost = 0.15

	// searchLimit bounds the candidate set scored locally. GitHub search orders by
	// relevance; the first 20 are far more than a real duplicate would need to appear in.
	searchLimit = "20"
)

// stopWords are tokens dropped before scoring. They carry little signal and would inflate
// overlap between unrelated titles that happen to share common words ("the", "a", "to").
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"of": true, "to": true, "in": true, "on": true, "for": true, "with": true,
	"is": true, "are": true, "be": true, "this": true, "that": true, "it": true,
	"as": true, "at": true, "by": true, "from": true, "not": true, "no": true,
	"if": true, "so": true, "do": true, "does": true,
}

// tokenize normalises a title to its scoring tokens: lowercased, split on any non-
// [a-z0-9] run, with tokens shorter than 2 chars, stopwords and duplicates removed.
// The set-like output is a slice for deterministic iteration; dedupe is by map.
func tokenize(title string) []string {
	s := strings.ToLower(title)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if len(f) < 2 || stopWords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func tokenSet(tokens []string) map[string]bool {
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		m[t] = true
	}
	return m
}

// matchScore returns the duplicate-likelihood score for candidateTitle vs queryTokens,
// in [0, 1]. Jaccard similarity of the normalised token sets, plus classLabelBoost when
// candHasClassLabel (capped at 1.0). A zero-length query or candidate scores 0 — an empty
// title cannot be matched, and a candidate with no scorable tokens cannot duplicate one.
func matchScore(queryTokens []string, candidateTitle string, candHasClassLabel bool) float64 {
	qt := tokenSet(queryTokens)
	ct := tokenSet(tokenize(candidateTitle))
	if len(qt) == 0 || len(ct) == 0 {
		return 0
	}
	inter := 0
	for t := range qt {
		if ct[t] {
			inter++
		}
	}
	if inter == 0 {
		// No shared tokens: the boost alone must not mint a match out of nothing.
		return 0
	}
	union := len(qt) + len(ct) - inter
	score := float64(inter) / float64(union)
	if candHasClassLabel {
		score += classLabelBoost
		if score > 1 {
			score = 1
		}
	}
	return score
}

// candidate is one scored result of a dedupe search.
type candidate struct {
	Number        int     `json:"number"`
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Score         float64 `json:"score"`
	HasClassLabel bool    `json:"hasClassLabel"`
}

// ghSearchHit is the slice of `gh search issues --json` deskfile reads. Labels come back
// as an array of {id,name,...} objects; only the name is needed.
type ghSearchHit struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// errNoScorableTokens is returned by dedupeSearch when the query title normalises to no
// scorable tokens at all (e.g. "a to the", "?!"). tokenize keeps runs of 2+ chars, so "12"
// is NOT an example — it survives tokenization and reaches the search. Such a title can
// never score above the threshold against ANY candidate — matchScore returns 0 for an
// empty query set — so running the search would hand back a guaranteed "no duplicates"
// that proves nothing. The
// callers turn this into a refusal rather than a pass: an un-dedupable title is the one
// input for which the gate cannot do its job, and passing it through is the fail-OPEN
// direction (it is a free bypass of the whole matcher, reachable by any caller that titles
// its issue in stopwords).
var errNoScorableTokens = errors.New("title has no scorable tokens")

// searchQuery builds the free-text query handed to `gh search issues` from the query
// TOKENS, never from the raw title.
//
// The raw title is caller-supplied text going into GitHub's search QUERY LANGUAGE, where
// `word:value` is a qualifier, not a word. Desk issue titles routinely contain colons
// ("bugs-gc: prune closed-issue files"), so the raw title would regularly be reinterpreted
// as a scope — and a rescoped search returns a candidate set that does not contain the
// duplicate, which reads to the gate as "no duplicate exists". That is the fail-open
// direction, reachable by accident and steerable on purpose.
//
// tokenize's output is [a-z0-9] runs only: no colon, quote, parenthesis or `-` survives it,
// so no token can be a qualifier or a negation. Losing phrase intent costs nothing here —
// the local Jaccard scorer, not GitHub relevance, decides what counts as a duplicate, and a
// broader candidate set is the fail-CLOSED direction.
func searchQuery(queryTokens []string) string { return strings.Join(queryTokens, " ") }

// dedupeSearch runs GitHub's issue search scoped to repo's OPEN issues and returns every
// hit scored against title, sorted most-likely-first. A gh/API failure, an unparseable
// response, an EMPTY response, or an un-dedupable title all return an error, and the caller
// fails CLOSED for `new`/`check` (exit 5/6) rather than guess at absence of duplicates.
func dedupeSearch(repo, title string) ([]candidate, error) {
	qtokens := tokenize(title)
	if len(qtokens) == 0 {
		return nil, errNoScorableTokens
	}
	out, err := gh("search", "issues", searchQuery(qtokens), "--repo", repo, "--state", "open",
		"--json", "number,title,url,labels", "--limit", searchLimit)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		// An answered search prints a JSON array — `[]` when there are no hits. NOTHING at
		// all is an UNANSWERED search, not an empty result set, and "the search produced
		// nothing" must never read as "no duplicates exist": that is the one direction this
		// tool says it will not take. Fail closed and let the caller see exit 6.
		return nil, fmt.Errorf("gh search issues produced no output (exit 0, empty stdout) — " +
			"an answered search prints a JSON array; treating this as 'no duplicates' would be fail-open")
	}
	var hits []ghSearchHit
	if err := json.Unmarshal([]byte(out), &hits); err != nil {
		return nil, fmt.Errorf("cannot parse gh search issues JSON: %w", err)
	}
	cands := make([]candidate, 0, len(hits))
	for _, h := range hits {
		hasClass := false
		for _, l := range h.Labels {
			if l.Name == classLabel {
				hasClass = true
				break
			}
		}
		// Sanitize at INGEST, not at each print site. Titles and URLs on the two PUBLIC
		// repos in the fixed set are authored by arbitrary external users, so every field
		// that crosses this boundary is attacker-influenced text on its way to an
		// operator's terminal and to an agent's context. Stripping here means no downstream
		// formatter can leak it by choosing %s over %q. See deskkit.StripControl.
		title := deskkit.StripControl(h.Title)
		cands = append(cands, candidate{
			Number:        h.Number,
			Title:         title,
			URL:           deskkit.StripControl(h.URL),
			Score:         matchScore(qtokens, title, hasClass),
			HasClassLabel: hasClass,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].Number < cands[j].Number // stable tie-break for deterministic output
	})
	return cands, nil
}

// matchesAbove returns the candidates scoring at or above matchThreshold, preserving the
// most-likely-first order from dedupeSearch. Empty when no duplicate is likely.
func matchesAbove(cands []candidate) []candidate {
	out := make([]candidate, 0)
	for _, c := range cands {
		if c.Score >= matchThreshold {
			out = append(out, c)
		}
	}
	return out
}

func topScore(cands []candidate) float64 {
	max := 0.0
	for _, c := range cands {
		if c.Score > max {
			max = c.Score
		}
	}
	return max
}

// formatCandidates renders the above-threshold candidate list for the refusal message.
// Each line shows number, score and title so the human-audit trail is self-explanatory.
//
// The title is printed with %q, matching the two sibling print sites in deskfile.go
// (the top-candidate refusal lines). StripControl deliberately KEEPS tab and newline —
// deskboard and issueboard need them for layout — but this renderer is the one place a
// title is folded into a multi-row LIST: a `\n` in a StripControl'd title would still let
// a crafted remote title fabricate an extra candidate row (bogus number, score and
// "class issue" marker) that never came from the search. %q escapes both, so no title can
// span more than the one line it was assigned. TestNoRawControlBytesInAnyOutput's
// assertNoTerminalActiveBytes deliberately skips \n and \t (they are legitimate elsewhere),
// so it does not see this path — the row-shaped output here is what makes it load-bearing.
func formatCandidates(cands []candidate) string {
	var b strings.Builder
	for _, c := range cands {
		fmt.Fprintf(&b, "  - #%d (score %.2f%s): %q\n", c.Number, c.Score,
			classMarker(c.HasClassLabel), c.Title)
	}
	return b.String()
}

func classMarker(hasClass bool) string {
	if hasClass {
		return ", class issue"
	}
	return ""
}
