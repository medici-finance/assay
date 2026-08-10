package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HumanLogin maps the human-name portion of a human:<name> token (lowercase) to a
// GitHub login. The check scans the PR's reviews and comments for that login
// acting — a mapping IS a claim that name=login, so it is trust configuration, not
// inventory.
//
// It SHIPS EMPTY (assay-selfcontain/05, C7). The map used to be a compiled-in
// single-organisation pair; it is now adopter configuration read from
// ASSAY_HUMAN_LOGIN_MAP, on the same outside-every-ref surface as the trust roster
// (see rosterconfig.go). The documented example is:
//
//	ASSAY_HUMAN_LOGIN_MAP=alice:alice-gh,bob:bob-gh
//
// P1 for this surface: an UNCONFIGURED map keeps meaning MISSING-CORROBORATION —
// an unknown name has no GitHub login to check, so nothing corroborates. It must
// never start PASSING corroboration checks when unset.
//
// There are THREE consumers, and ALL THREE are gate-affecting. They differ in what a
// hit buys, not in whether it buys anything:
//
//	verifierFloorFailure (attribution.go) a hit CLEARS the floor outright.
//	authorizedByVerifiedHuman (registers.go) a hit GRANTS the human authorisation.
//	corroborateStamps    (here)           a hit turns MISSING-CORROBORATION into
//	                                      CORROBORATED, which is an exit-status flip
//	                                      on `--corroborate` (runCorroborate returns 1
//	                                      only when anyMissing).
//
// An earlier draft called the third "not widening on its own", on the ground that the
// mapped login's APPROVED review or approval comment must still be found on the PR.
// That barrier does not hold when the same actor chooses which login the name maps to:
// any account already carrying an approval-shaped comment on the PR will satisfy it,
// including a shared agent account — precisely the identity `human:<name>`
// corroboration exists to distinguish from a human. Measured: with the stamp
// `human:<name>` and one `lgtm` comment authored by a shared-agent account, no map
// gives MISSING-CORROBORATION and a map pointing `<name>` at that account gives
// CORROBORATED.
//
// All three use a hit only to accept and none consults Configured(), so while an empty
// map is the stricter direction, a NON-EMPTY map is strictly wider — including one
// arriving over the CI transport, which `--corroborate` reaches too (it is not
// `--scan-issues`, so scanClassForMode gives it ClassCI under GITHUB_ACTIONS). That
// residual is stated in full at the head of rosterconfig.go and pinned in all three
// gates by TestCIHumanLoginMapResidualBoundary.
func HumanLogin(name string) (string, bool) {
	login, ok := scanEffectiveConfig().HumanLogins[strings.ToLower(strings.TrimSpace(name))]
	return login, ok
}

// ---- gh PR JSON types (the subset --json reviews,comments returns) ----------------

type ghAuthor struct {
	Login string `json:"login"`
}

type ghReview struct {
	Author      ghAuthor `json:"author"`
	Body        string   `json:"body"`
	State       string   `json:"state"`
	SubmittedAt string   `json:"submittedAt"`
	Id          string   `json:"id"`
}

type ghComment struct {
	Author    ghAuthor `json:"author"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"createdAt"`
	URL       string   `json:"url"`
}

// ---- approval-phrase matching ----------------------------------------------------

// approvalPhraseRe matches a PR comment that reads as an explicit approval of the
// PR's content — the human's own words authenticating the human:<name> stamp. The
// phrase must appear as a whole word (word boundary on each side) to avoid matching
// inside another word.
//
// WHAT THIS CHECK IS AND IS NOT. hasApprovalPhrase is only ever reached for a
// comment ALREADY AUTHORED BY the GitHub login the stamp's name resolves to
// (corroborateStamps filters on author before calling it). The forgery resistance
// therefore lives in the identity check, not here; this pattern only asks "did
// that human say yes". Widening it trades false REJECTS away without buying a
// forger anything, because a forger cannot author as the human in the first place.
//
// WHY IT WAS WIDENED (assay-toolkit#263). The pattern was
// `\b(approve[ds]?|sign[- ]?off)\b`, which does not match `lgtm`. Measured against
// the real corpus — every issue/PR comment authored by the house's human reviewer
// across the two active repos, 301 comments, harvested 2026-08-02 via
// GET /repos/{repo}/issues/comments:
//
//	lgtm                      58 comments (50 of them the bare word "lgtm")
//	approve* / signoff        11 comments   <- all the old pattern could see
//	looks good                 2 comments
//	ship it / bare "+1"        0 comments
//
// So the shipped check missed ~84% of this human's actual approvals (58 of 69) and
// reported MISSING-CORROBORATION on correctly-signed gates. `lgtm` is the house
// sign-off, not an edge case.
//
// WHAT WAS DELIBERATELY *NOT* ADDED, on the evidence of the same corpus:
//
//   - "go ahead" (2 comments) — one of the two is "please go ahead with explicitly
//     pruning", an INSTRUCTION TO DO WORK, not an approval of a diff. Accepting it
//     would accept a non-approval, so it is out.
//
//   - "blessing" / "i am good with it" (1 each) — the single "blessing" instance is
//     "this has my blessing, ONCE your issues are addressed", a conditional. Too
//     few and too conditional to key a gate on.
//
//   - BARE "looks good" (2 comments, both genuine approvals) — NOT accepted, on
//     purpose. TestApprovalPhraseRe has pinned `"this looks good"` as a
//     non-approval since before this change, and that judgement is sound: "the
//     diagram looks good" or "looks good so far, but" is not an approval of a
//     diff. Only the unambiguous full phrase `looks good to me` is accepted. The
//     two corpus instances stay false rejections; that is 2 of 69, against the 58
//     this change recovers, and it is the direction that cannot admit a
//     non-approval.
//
// `ship it` and a bare `+1` are accepted because assay-toolkit#263 asks for them
// and neither can match a non-approval in the forms used here (`+1` only as a
// whole body — see hasApprovalPhrase); both are zero-instance in the corpus, so
// they are requested-not-evidenced and buy nothing today either way.
var approvalPhraseRe = regexp.MustCompile(`(?i)\b(approve[ds]?|sign[- ]?off|lgtm|looks\s+good\s+to\s+me|ship\s+it)\b`)

// bareThumbsUpRe matches a comment whose ENTIRE body is a bare "+1" (with optional
// surrounding whitespace, trailing punctuation, or an emoji). It is deliberately
// not part of approvalPhraseRe: an inline "+1" is far more likely to be a diffstat,
// a version bump, or "+1 line" than an approval, whereas a comment that says
// nothing but "+1" can only be one.
var bareThumbsUpRe = regexp.MustCompile(`^\s*\+1[\s.!👍]*$`)

// acceptedApprovalPhrases is the human-readable form of what the two patterns
// above accept. It is printed by --corroborate whenever a stamp comes back
// MISSING-CORROBORATION, so the tool states its own accepted wording at the moment
// it rejects someone — assay-toolkit#263 asked for a documented home for the phrase
// list, and the point of rejection is the one place a reader is guaranteed to look.
var acceptedApprovalPhrases = []string{
	"an APPROVED review (strongest signal — no wording needed)",
	`"lgtm"`,
	`"looks good to me" (a bare "looks good" is NOT enough — say lgtm)`,
	`"approve" / "approved" / "approves"`,
	`"sign-off" / "signoff" / "sign off"`,
	`"ship it"`,
	`a comment whose entire body is "+1"`,
}

// negationPhraseRe matches phrases that negate or condition an approval, making
// the body unreliable as corroboration evidence. A match here means the body
// contains a qualification like "do not approve", "not approved", "needs sign-off
// from X", or "pending approval" — the approval word may be present but the
// CONTEXT negates it. T7: fail-closed — negated text must NOT corroborate.
//
// This is a HEURISTIC BLOCKLIST, not a decision procedure. It catches common
// negated and conditional phrasings but will miss novel or indirect negations.
// The blast radius is nil today (hasApprovalPhrase has one caller, --corroborate,
// wired into no workflow in either repo); when it gains a live consumer the
// pattern should be tightened with a broader negation grammar.
//
// The negation grammar for the vocabulary assay-toolkit#263 ADDED (`lgtm`,
// `ship it`) lives in negationPhraseLGTMRe below, not here — see the comment there
// for why it is a separate pattern and how the one measured conditional-approval
// idiom is carved out of it.
var negationPhraseRe = regexp.MustCompile(`(?i)\b(do\s*n['’]?t\s+approve|not\s+(\w+\s+){0,3}approv(ed?|al)|needs?\s+sign[- ]?off\s+from|pending\s+approval|awaiting\s+approval|requires?\s+approval|without\s+approval|cannot?\s+(\w+\s+){0,2}approv\w*|unable\s+to\s+approve|nack,?\s+approve|never\s+approve|decline\s+(to\s+)?approve|refuse\s+(to\s+)?approve|do(es)?\s*n['’]?t\s+(look|ship)|do\s+not\s+(look\s+good|ship)|not\s+look(ing)?\s+good)\b`)

// lgtmConditionalApprovalRe matches the ONE measured idiom in which the sequence
// "not lgtm" is an APPROVAL rather than a refusal:
//
//	"don't we need to change a hostname or something? if not lgtm"
//
// — i.e. "if not, lgtm". That is the only occurrence of the sequence "not lgtm" in
// the 304-comment human-reviewer corpus (re-harvested 2026-08-02), and it is a genuine
// sign-off. It is neutralised in hasNegationPhrase BEFORE the negation scan runs,
// which is what lets `not lgtm` block a real refusal ("this is not lgtm, please
// fix") without converting this real approval into a false rejection. The
// replacement affects the NEGATION scan only — approvalPhraseRe always sees the
// original body, so the approval still matches.
var lgtmConditionalApprovalRe = regexp.MustCompile(`(?i)\bif\s+not[,:;]?\s+(then[,:]?\s+)?lgtm\b`)

// negationPhraseLGTMRe is the negation grammar for the approval vocabulary
// assay-toolkit#263 added. It exists because the #263 widening shipped `lgtm` and
// `ship it` into approvalPhraseRe WITHOUT the refusal forms negationPhraseRe had
// long carried for `approve` — so explicit refusals ("i cannot lgtm this",
// "nack, not lgtm", "non-lgtm") read as sign-offs while the identical shapes on
// the older verb ("i cannot approve this") were correctly refused. That asymmetry
// was the blocking finding on the assay-toolkit#401 review; this pattern closes it.
//
// It is kept SEPARATE from negationPhraseRe for two reasons: the carve-out above
// applies to this half only, and `lgtm` needs a different filler window from
// `approve` (see the `not` clause below).
//
// MEASURED COST, both directions, against the same corpus that motivated #263
// (every human-reviewer-authored issue/PR comment across the two active repos,
// 304 comments):
//
//   - 58 comments contain `lgtm`. Exactly TWO of them contain any trigger word
//     used below: "lgtm if you are waiting on a human" (the `waiting` clause
//     requires `lgtm` to come AFTER it, so it does not fire) and the "if not lgtm"
//     idiom (carved out above). Every one of the 58 still corroborates.
//   - `nack`, `non-lgtm`, `un-lgtm` and `ship it` are ZERO-instance in the corpus,
//     so blocking them costs nothing measured and closes a refusal that reads as
//     approval.
//
// The `not` clause deliberately uses a NARROW filler set rather than the
// `(\w+\s+){0,3}` window negationPhraseRe uses for `approve`. `approve` is a verb
// that governs an object, so a wide window between "not" and it stays inside one
// clause; `lgtm` is a bare token routinely APPENDED to a caveat ("not sure but
// lgtm"), where a 3-word window would negate a genuine approval. The asymmetry is
// deliberate and is the fail-open direction only for wording nobody has written.
var negationPhraseLGTMRe = regexp.MustCompile(`(?i)(` + strings.Join([]string{
	// Refusal verbs governing the new vocabulary — the direct analogues of the
	// `cannot approve` / `unable to approve` / `never approve` / `decline to
	// approve` clauses negationPhraseRe already carries for the old verb.
	`\b(?:cannot|can\s+not|can['’]t|could\s*n['’]?t|could\s+not|will\s+not|wo\s*n['’]t|do(?:es|id)?\s+not|do(?:es|id)?\s*n['’]t|unable\s+to|refuse[sd]?(?:\s+to)?|declin(?:e[sd]?|ing)(?:\s+to)?|never)\s+(?:\w+\s+){0,2}(?:lgtm|ship\s+it)\b`,
	// Bare "not" + the new vocabulary, narrow filler (see the note above).
	`\bnot\s+(?:yet\s+|really\s+|quite\s+|entirely\s+|an?\s+|the\s+)?(?:lgtm|ship\s+it)\b`,
	// "nack" is a refusal token in its own right, not only in "nack, approve".
	`\bnack\b`,
	// Negating prefixes. `\b` treats the hyphen as a word boundary, so `\blgtm\b`
	// matches inside `non-lgtm` — the prefix has to be blocked explicitly.
	`\b(?:non|un|anti)-?lgtm\b`,
	`\bno\s+lgtm\b`,
	// Asking FOR approval is not giving it.
	`\b(?:is|are)\s+(?:this|it|that|these|those)\s+(?:\w+\s+){0,2}lgtm\b`,
	`\b(?:can|could|would|will)\s+(?:someone|somebody|anyone|any\s?one|you|we|u|i)\s+(?:please\s+)?(?:\w+\s+){0,2}lgtm\b`,
	`\bplease\s+lgtm\b`,
	`\b(?:pending|awaiting|needs?|requires?|waiting\s+(?:for|on))\s+(?:an?\s+|the\s+)?(?:\w+\s+){0,2}lgtm\b`,
	`\bwithout\s+(?:\w+\s+){0,2}lgtm\b`,
	`\blgtm\s*\?`,
	// "ship it" as a NOUN (a button, a label) rather than an imperative.
	`\b(?:the|a|an|this|that)\s+["'“‘]?ship\s+it\b`,
}, "|") + `)`)

// hasNegationPhrase reports whether body contains a negation or conditional
// phrase that would make an approval-phrase match unreliable. If negation is
// present, the body does NOT corroborate regardless of approvalPhraseRe matches.
func hasNegationPhrase(body string) bool {
	if negationPhraseRe.MatchString(body) {
		return true
	}
	// Neutralise the measured "if not, lgtm" idiom before scanning the #263
	// vocabulary, so its `not lgtm` clause blocks refusals without rejecting the
	// one real approval that shares the character sequence.
	return negationPhraseLGTMRe.MatchString(lgtmConditionalApprovalRe.ReplaceAllString(body, " "))
}

// hasApprovalPhrase reports whether body contains an explicit approval phrase as a
// whole word AND contains no negation/conditional phrase that would void it. This
// is the comment-side corroboration path — an APPROVED review is checked
// separately (stronger signal).
func hasApprovalPhrase(body string) bool {
	if hasNegationPhrase(body) {
		return false
	}
	if bareThumbsUpRe.MatchString(body) {
		return true
	}
	return approvalPhraseRe.MatchString(body)
}

// ---- stamp extraction from diff --------------------------------------------------

// humanStampRe matches a human:<name> token — the literal prefix "human:", at a
// TOKEN BOUNDARY, followed by one or more ASCII word characters (the name).
//
// The leading `(?:^|[^0-9A-Za-z_-])` is the whole point (assay-toolkit#243). The
// previous pattern was a bare `human:(\w+)`, an UNANCHORED substring match, so
// `superhuman:x` matched as a human stamp with name "x" and `non-human:x` matched
// as name "x". That is a forgeable authorization token: authorizedByVerifiedHuman
// in registers.go gates register field-gutting on this regex, so
// `authorized-by: superhuman:ian` authorized the gut. brieffile.go's
// hasHumanReviewer already got this right (strings.Fields + HasPrefix, and its
// doc comment names `superhuman:x` as the case that must NOT count) — the two
// halves of the same toolkit disagreed about what a human stamp is, and oit
// brief-33's fixtures side with hasHumanReviewer.
//
// Go's RE2 has no lookbehind, so the boundary is expressed as a consumed leading
// character. That is safe for FindAll: the character consumed by a match is the
// one BEFORE its own "human:", never a separator a later stamp needs, so
// "human:a human:b" still yields both. The hyphen is excluded from the boundary
// class deliberately — `non-human:` and `sub-human:` are not human stamps.
//
// The capture group index is unchanged (m[1] is still the name), so every
// existing call site keeps working.
var humanStampRe = regexp.MustCompile(`(?:^|[^0-9A-Za-z_-])human:([0-9A-Za-z_]+)`)

// looseHumanStampRe matches the same boundary-anchored "human:" prefix followed by
// any run of non-space characters. It exists only to find the stamps humanStampRe
// deliberately cannot parse — see confusableStampNames.
var looseHumanStampRe = regexp.MustCompile(`(?:^|[^0-9A-Za-z_-])human:(\S+)`)

// confusableStampNames returns the names of `human:<name>` occurrences whose name
// begins with a NON-ASCII LETTER — a homoglyph/confusable name such as
// `human:іan` (Cyrillic U+0456 "і", visually identical to ASCII "i").
//
// This is the second half of assay-toolkit#243. `\w` in RE2 is ASCII-only, so a
// confusable name does not match humanStampRe at all, and the failure mode is
// SILENT: --corroborate reports "no human:<name> stamps found in diff — clean" for
// a diff that, to any human reading it, plainly stamps `human:ian`. Silence on an
// unparseable authorization token is the wrong direction; these are surfaced as
// stamps with an unresolvable name so the run says MISSING-CORROBORATION and exits
// non-zero.
//
// The first-rune-is-a-non-ASCII-letter test is deliberately narrow so that prose
// does not trip it: the docs and error strings in this very repo are full of
// literal `human:<name>` placeholders, and `<` is not a letter, so they are
// ignored exactly as they are today.
func confusableStampNames(s string) []string {
	var out []string
	for _, m := range looseHumanStampRe.FindAllStringSubmatch(s, -1) {
		name := m[1]
		r, _ := utf8.DecodeRuneInString(name)
		if r == utf8.RuneError || r < utf8.RuneSelf || !unicode.IsLetter(r) {
			continue // ASCII (handled by humanStampRe) or not name-shaped at all
		}
		out = append(out, name)
	}
	return out
}

// stamp describes a single human:<name> occurrence found in a PR diff.
type stamp struct {
	Name string // the <name> portion (e.g. "ian")
	File string // the file it appeared in
	Line string // the full added line (first 120 chars, for context)
}

// stampsInDiff parses a unified diff and returns every human:<name> stamp found on
// ADDED lines (lines starting with "+" but not "+++"). Each stamp records the name,
// the file it was added to, and the line content for context.
func stampsInDiff(diff string) []stamp {
	lines := strings.Split(diff, "\n")
	var out []stamp
	curFile := ""
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		// Track the current file from diff headers.
		if strings.HasPrefix(trimmed, "diff --git ") {
			// "diff --git a/path b/path"
			fields := strings.Fields(trimmed)
			if len(fields) >= 4 {
				curFile = strings.TrimPrefix(fields[3], "b/")
			}
			continue
		}
		if strings.HasPrefix(trimmed, "+++ ") {
			curFile = strings.TrimPrefix(trimmed, "+++ b/")
			continue
		}
		// Only added lines (not the "+++" header itself).
		if !strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "+++") {
			continue
		}
		// Strip the leading "+" for matching.
		content := strings.TrimPrefix(trimmed, "+")
		lineCtx := content
		if len(lineCtx) > 120 {
			lineCtx = lineCtx[:120] + "..."
		}
		for _, m := range humanStampRe.FindAllStringSubmatch(content, -1) {
			out = append(out, stamp{
				Name: strings.ToLower(m[1]),
				File: curFile,
				Line: strings.TrimSpace(lineCtx),
			})
		}
		// A confusable-name stamp is recorded too, under its raw name. It will
		// fail the human-login lookup and report MISSING-CORROBORATION — loud
		// refusal rather than the silent "no stamps — clean" it used to produce
		// (assay-toolkit#243).
		for _, name := range confusableStampNames(content) {
			out = append(out, stamp{
				Name: strings.ToLower(name),
				File: curFile,
				Line: strings.TrimSpace(lineCtx),
			})
		}
	}
	// Deduplicate by (name, file) — multiple human:<name> on the same line or
	// in the same file count as one stamp for that file.
	seen := map[string]bool{}
	var deduped []stamp
	for _, s := range out {
		key := s.Name + "\x00" + s.File
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, s)
	}
	return deduped
}

// ---- verdict types ---------------------------------------------------------------

type verdict int

const (
	verdictCorroborated verdict = iota
	verdictMissing
	verdictNoStamp
)

func (v verdict) String() string {
	switch v {
	case verdictCorroborated:
		return "CORROBORATED"
	case verdictMissing:
		return "MISSING-CORROBORATION"
	default:
		return ""
	}
}

// ---- PR data retrieval -----------------------------------------------------------

// fetchPRDiff runs `gh pr diff <N>` for the given repo and PR and returns the
// unified diff output.
func fetchPRDiff(repo string, pr int) (string, error) {
	cmd := exec.Command("gh", "pr", "diff", fmt.Sprintf("%d", pr), "--repo", repo)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr diff %d: %w", pr, err)
	}
	return string(out), nil
}

// ghPRData holds the subset of `gh pr view --json reviews,comments` that this check
// needs.
type ghPRData struct {
	Reviews  []ghReview  `json:"reviews"`
	Comments []ghComment `json:"comments"`
}

// fetchPRData runs `gh pr view <N> --json reviews,comments` for the given repo and
// PR and unmarshals the result.
func fetchPRData(repo string, pr int) (*ghPRData, error) {
	cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", pr),
		"--repo", repo, "--json", "reviews,comments")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view %d --json reviews,comments: %w", pr, err)
	}
	var data ghPRData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("unmarshal PR %d data: %w", pr, err)
	}
	return &data, nil
}

// ---- check logic -----------------------------------------------------------------

// corroborateResult is the per-stamp verdict.
type corroborateResult struct {
	Stamp    stamp
	Verdict  verdict
	Evidence string // URL of the corroborating review/comment, or reason for MISSING
	Login    string // resolved GitHub login (empty if unknown name)
}

// corroborateStamps is the testable core: given parsed stamps and pre-fetched PR data,
// it resolves each stamp's name to a GitHub login and checks for corroboration in the
// PR's reviews and comments. Returns one result per stamp.
func corroborateStamps(stamps []stamp, data *ghPRData, repo string, pr int) []corroborateResult {
	if len(stamps) == 0 {
		return []corroborateResult{{Verdict: verdictNoStamp}}
	}
	var results []corroborateResult
	for _, s := range stamps {
		login, known := HumanLogin(s.Name)
		if !known {
			results = append(results, corroborateResult{
				Stamp:    s,
				Verdict:  verdictMissing,
				Login:    "",
				Evidence: fmt.Sprintf("name %q has no mapping in %s — no GitHub login to check", s.Name, scanEnvHumanLoginMap),
			})
			continue
		}

		// Check APPROVED reviews by this login first (stronger signal).
		if data != nil {
			for _, r := range data.Reviews {
				if strings.EqualFold(r.Author.Login, login) && r.State == "APPROVED" {
					results = append(results, corroborateResult{
						Stamp:    s,
						Verdict:  verdictCorroborated,
						Login:    login,
						Evidence: fmt.Sprintf("APPROVED review by %s: %s", login, reviewURL(repo, pr, r)),
					})
					goto nextStamp
				}
			}

			// Check comments by this login containing an explicit approval phrase.
			for _, c := range data.Comments {
				if strings.EqualFold(c.Author.Login, login) && hasApprovalPhrase(c.Body) {
					results = append(results, corroborateResult{
						Stamp:    s,
						Verdict:  verdictCorroborated,
						Login:    login,
						Evidence: fmt.Sprintf("approval comment by %s: %s", login, c.URL),
					})
					goto nextStamp
				}
			}
		}

		// No corroboration found.
		results = append(results, corroborateResult{
			Stamp:    s,
			Verdict:  verdictMissing,
			Login:    login,
			Evidence: fmt.Sprintf("no APPROVED review or explicit approval comment from %s on PR #%d", login, pr),
		})
	nextStamp:
	}
	return results
}

// checkCorroboration runs the full corroboration pipeline for a single PR:
// fetch diff → extract stamps → resolve logins → query reviews/comments → verdict.
func checkCorroboration(repo string, pr int) ([]corroborateResult, error) {
	diff, err := fetchPRDiff(repo, pr)
	if err != nil {
		return nil, err
	}
	stamps := stampsInDiff(diff)
	if len(stamps) == 0 {
		return []corroborateResult{{Verdict: verdictNoStamp}}, nil
	}

	data, err := fetchPRData(repo, pr)
	if err != nil {
		return nil, err
	}

	return corroborateStamps(stamps, data, repo, pr), nil
}

// reviewURL constructs a URL for a review. The gh API does not return a direct
// review URL, so we construct one from the PR and review ID.
func reviewURL(repo string, pr int, r ghReview) string {
	// gh review IDs look like "PRR_kwDOS6FOdM8AAAABFostaA"
	return fmt.Sprintf("https://github.com/%s/pull/%d#pullrequestreview-%s", repo, pr, r.Id)
}

// ---- run entry point -------------------------------------------------------------

// runCorroborate is the entry point for `statusgen --corroborate <pr>`.
// It checks human:<name> stamps in the PR's diff against the PR's reviews and
// comments, printing one verdict line per stamp found. Returns exit code 0
// when all stamps are corroborated, 1 when any stamp is MISSING-CORROBORATION.
func runCorroborate(prsArg string) int {
	if prsArg == "" {
		fmt.Fprintln(os.Stderr, "statusgen: --corroborate requires at least one PR number (comma-separated)")
		return 2
	}

	// Discover the repo from git remote.
	repo := repoFromOrigin()
	if repo == "" {
		fmt.Fprintln(os.Stderr, "statusgen: cannot determine GitHub repo from git remote origin")
		return 1
	}

	prStrs := strings.Split(prsArg, ",")
	var allResults []corroborateResult
	anyMissing := false

	for _, prStr := range prStrs {
		prStr = strings.TrimSpace(prStr)
		if prStr == "" {
			continue
		}
		var pr int
		if _, err := fmt.Sscanf(prStr, "%d", &pr); err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: invalid PR number %q\n", prStr)
			return 2
		}
		results, err := checkCorroboration(repo, pr)
		if err != nil {
			// If gh returns exit status 1 (e.g. PR not found), print and exit 1.
			fmt.Fprintf(os.Stderr, "statusgen: PR #%d: %v\n", pr, err)
			return 1
		}
		if len(results) == 1 && results[0].Verdict == verdictNoStamp {
			fmt.Printf("PR #%d: no human:<name> stamps found in diff — clean\n", pr)
			continue
		}
		for _, r := range results {
			allResults = append(allResults, r)
			if r.Verdict == verdictMissing {
				anyMissing = true
			}
		}
	}

	// Honest-scope header (the brief requires this).
	fmt.Println("# human-stamp corroboration")
	fmt.Println("# Scope: verifies the named human ACTED on the PR (APPROVED review")
	fmt.Println("# or explicit approval comment from their own GitHub account).")
	fmt.Println("# Cannot verify they actually executed a deferred live check —")
	fmt.Println("# that remains what the sign-off MEANS.")
	fmt.Println()

	for _, r := range allResults {
		label := fmt.Sprintf("%s/brief-%s", r.Stamp.Name, "") // fallback
		if r.Stamp.File != "" {
			label = r.Stamp.File
		}
		switch r.Verdict {
		case verdictCorroborated:
			fmt.Printf("human:%s in %s CORROBORATED — %s\n", r.Stamp.Name, label, r.Evidence)
		case verdictMissing:
			fmt.Printf("human:%s in %s MISSING-CORROBORATION — %s\n", r.Stamp.Name, label, r.Evidence)
		}
	}

	if len(allResults) == 0 {
		fmt.Println("(no stamps found across all PRs)")
	}

	// State the accepted wording at the point of rejection (assay-toolkit#263).
	// A MISSING-CORROBORATION line is most often a WORDING miss, not a missing
	// human, and a reader who cannot see the phrase list re-asks the human to
	// re-sign in different words.
	if anyMissing {
		fmt.Println()
		fmt.Println("# A comment from the named human corroborates when it is any of:")
		for _, p := range acceptedApprovalPhrases {
			fmt.Printf("#   - %s\n", p)
		}
		fmt.Println("# (a negation or conditional in the same comment voids it — a refusal")
		fmt.Println("#  such as \"not lgtm\" / \"cannot lgtm\" / \"nack\" / \"non-lgtm\", or a REQUEST")
		fmt.Println("#  for approval such as \"is this lgtm?\" / \"please lgtm\", is not a sign-off)")
	}

	if anyMissing {
		return 1
	}
	return 0
}

// repoFromOrigin returns the "owner/repo" portion of the git remote origin URL.
func repoFromOrigin() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	// SSH: git@<host>:<owner>/<repo>.git
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		_, after, _ := strings.Cut(url, ":")
		return strings.TrimSuffix(after, ".git")
	}
	// HTTPS: https://<host>/<owner>/<repo>.git
	if strings.HasPrefix(url, "https://") {
		rest := strings.TrimPrefix(url, "https://")
		_, after, _ := strings.Cut(rest, "/")
		return strings.TrimSuffix(after, ".git")
	}
	return ""
}
