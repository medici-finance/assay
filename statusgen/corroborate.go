package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// It SHIPS EMPTY. The map used to be a compiled-in
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
// There are TWO map-gated consumers, both gate-affecting. They differ in what a
// hit buys, not in whether it buys anything:
//
//	authorizedByVerifiedHuman (registers.go) a hit GRANTS the human authorisation.
//	corroborateStamps    (here)           a hit turns MISSING-CORROBORATION into
//	                                      CORROBORATED, which is an exit-status flip
//	                                      on `--corroborate` (runCorroborate returns 1
//	                                      only when anyMissing).
//
// A THIRD consumer, verifierFloorFailure (attribution.go), also resolves through this
// CURRENT map — a hit is the "confirmed NOW" half of its human-token exemption. The
// floor additionally consults FormerHumanLogin (the former-humans map) for the
// "confirmed HISTORICALLY" half: it clears for current ∪ former and rejects a name
// confirmed by neither. So the floor IS map-widened again (a mapped name clears where
// an unmapped, never-confirmed one fails), which is the tightening back toward main
// that #104's shape-based form had dropped. This function supplies only the current
// half; the former half lives in FormerHumanLogin and is NOT consulted by the two
// identity gates below — a departed human cannot approve today.
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

// FormerHumanLogin maps the human-name portion of a human:<name> token (lowercase) to
// the GitHub login of a DEPARTED human — one who was a confirmed human at some past
// point but is no longer in the current ASSAY_HUMAN_LOGIN_MAP. It reads the dedicated
// ASSAY_FORMER_HUMAN_LOGIN_MAP roster surface, parsed and bot-shape-validated exactly
// like the current map (see rosterconfig.go).
//
// It exists for ONE consumer: the verifier floor's human-token exemption
// (verifierFloorFailure / runnerClearsFloor, attribution.go), as the "confirmed
// HISTORICALLY" half of confirmedHumanEver. A Verified cell records who signed off
// THEN; when a human leaves and is dropped from the current map, their historical
// stamps must still clear the model-capability floor — but a name that was NEVER a
// confirmed human must NOT. So the floor clears for current ∪ former and rejects
// everything else, and this function supplies the former half.
//
// It is DELIBERATELY NOT consulted by the identity gates. --corroborate
// (corroborateStamps) and the register human-authorisation key
// (authorizedByVerifiedHuman) resolve names through HumanLogin — the CURRENT map —
// ONLY, and still require an APPROVED review / approval comment. "Only confirmed
// humans can approve" is those gates, and a departed human cannot approve today; the
// former map widens the capability floor's leaver carve-out, never the approval
// authority. Like the current map, an UNSET former map is the stricter direction (no
// former human is recognised, so only the current map can clear the floor).
func FormerHumanLogin(name string) (string, bool) {
	login, ok := scanEffectiveConfig().FormerHumanLogins[strings.ToLower(strings.TrimSpace(name))]
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
	// CommitOID is the commit the review reports having been submitted AGAINST.
	// `gh pr view --json reviews` does not return it; the REST reviews endpoint
	// does, as `commit_id`, and that is the source the model-path auto-flip
	// reads (see autoflip.go). It is therefore EMPTY on everything fetchPRData
	// returns, which is why approvedReviewAt below compares it only when the
	// caller asks for a SHA — a human-stamp corroboration must not start
	// failing because this field is unpopulated on its transport.
	//
	// WHAT THIS FIELD IS NOT. It is not an established staleness signal. A
	// review's `commit_id` has been observed once to disagree with the head
	// named in the review's own body, and a follow-up sweep failed to establish
	// any direction or frequency for that disagreement — see rule 40 in
	// docs/brief-rules.md, which states it at that strength and forbids
	// upgrading it to a direction. So a comparison against this field is a
	// best-effort FILTER, not a proof of currency: an equal SHA does not
	// establish that the approval is current, it only fails to contradict it.
	// Every caller must therefore treat this field as load-bearing in the
	// refusing direction only.
	CommitOID string `json:"-"`
}

// approvedReviewAt returns the first APPROVED review authored by login. When
// atSHA is non-empty the review must ALSO have been submitted against that
// exact commit.
//
// This is the toolkit's ONE approval-checker, shared by both consumers on
// purpose: corroborateStamps (human-stamp corroboration, mm/15) calls it with
// an empty atSHA, and the model-path auto-flip (mm/39) calls it with the PR's
// merged head. A second, parallel implementation of an authorization check is
// how the two drift apart — one gets a fix the other does not, and the looser
// one becomes the way in.
//
// atSHA is compared case-insensitively but NOT by prefix: an abbreviated SHA
// must not satisfy a full one. "no review found" is the only false return, so
// every caller has to treat it as a refusal rather than a maybe.
func approvedReviewAt(reviews []ghReview, login, atSHA string) (ghReview, bool) {
	for _, r := range reviews {
		if !strings.EqualFold(r.Author.Login, login) || r.State != "APPROVED" {
			continue
		}
		if atSHA != "" && !strings.EqualFold(r.CommitOID, atSHA) {
			continue
		}
		return r, true
	}
	return ghReview{}, false
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
// WHY IT WAS WIDENED. The pattern was
// `\b(approve[ds]?|sign[- ]?off)\b`, which does not match `lgtm`. Measured against
// the real corpus — every issue/PR comment authored by an adopting team's human reviewer
// across its repos, 301 comments, harvested 2026-08-02 via
// GET /repos/{repo}/issues/comments:
//
//	lgtm                      58 comments (50 of them the bare word "lgtm")
//	approve* / signoff        11 comments   <- all the old pattern could see
//	looks good                 2 comments
//	ship it / bare "+1"        0 comments
//
// So the shipped check missed ~84% of this human's actual approvals (58 of 69) and
// reported MISSING-CORROBORATION on correctly-signed gates. `lgtm` is a common
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
// `ship it` and a bare `+1` are accepted because the widening asks for them
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
// it rejects someone — the widening asked for a documented home for the phrase
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
// hasApprovalPhrase has one caller, --corroborate, which is now a LIVE gate — wired
// into the pull_request lint job of .github/workflows/statusgen.yml, where an
// unmatched refusal that reads as approval would let a self-issued human stamp pass.
// The pattern should keep being tightened toward a broader negation grammar as new
// refusal phrasings are observed.
//
// The negation grammar for the vocabulary the widening ADDED (`lgtm`,
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
// the widening added. It exists because that widening shipped `lgtm` and
// `ship it` into approvalPhraseRe WITHOUT the refusal forms negationPhraseRe had
// long carried for `approve` — so explicit refusals ("i cannot lgtm this",
// "nack, not lgtm", "non-lgtm") read as sign-offs while the identical shapes on
// the older verb ("i cannot approve this") were correctly refused. That asymmetry
// was the blocking finding on an independent review; this pattern closes it.
//
// It is kept SEPARATE from negationPhraseRe for two reasons: the carve-out above
// applies to this half only, and `lgtm` needs a different filler window from
// `approve` (see the `not` clause below).
//
// MEASURED COST, both directions, against the same corpus that motivated the widening
// (every human-reviewer-authored issue/PR comment across the reviewer's repos,
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
	// Neutralise the measured "if not, lgtm" idiom before scanning the added
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
// The leading `(?:^|[^0-9A-Za-z_-])` is the whole point. The
// previous pattern was a bare `human:(\w+)`, an UNANCHORED substring match, so
// `superhuman:x` matched as a human stamp with name "x" and `non-human:x` matched
// as name "x". That is a forgeable authorization token: authorizedByVerifiedHuman
// in registers.go gates register field-gutting on this regex, so
// `authorized-by: superhuman:alex` authorized the gut. brieffile.go's
// hasHumanReviewer already got this right (strings.Fields + HasPrefix, and its
// doc comment names `superhuman:x` as the case that must NOT count) — the two
// halves of the same toolkit disagreed about what a human stamp is, and
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
// This is the second half of the confusable-name fix. `\w` in RE2 is ASCII-only, so a
// confusable name does not match humanStampRe at all, and the failure mode is
// SILENT: --corroborate reports "no human:<name> stamps found in diff — clean" for
// a diff that, to any human reading it, plainly stamps `human:alex`. Silence on an
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

// corroborateExcludedFixturePrefix is the ONE path prefix whose human:<name>
// stamps the corroborate scan deliberately skips. It is the education/08 tutorial
// corpus, which teaches the human:<name> notation itself and therefore uses the
// REAL token for FICTIONAL personas (`human:<name>` for names like alex or sam)
// that can never map to a GitHub login. Corroborating a teaching fixture is a
// category error: it does not
// drive any board or brief, so no forged done-flip can hide behind it.
//
// This exclusion is scoped behind a security-review:
// "A, scoped to the exact tutorial-skeleton path, behind a security-review ... The
// exclusion must be tight (exact path prefix, not 'any testdata') and reviewed,
// since a loose exclusion is an evasion surface."
//
// It is a NAMED CONSTANT, not a magic string in a loop, so the whole exclusion set
// is auditable in one place — this is the deliberately-chosen carve-out the review
// inspects. TWO invariants make it an exclusion and not an evasion surface:
//
//   - It is a PATH check, never a NAME check. alex/sam are not special-cased; a
//     forged `human:<name>` on any file OUTSIDE this prefix still requires corroboration.
//   - The TRAILING SLASH is load-bearing. It is the path boundary: a lookalike
//     sibling like ".../assay-tutorial-skeleton-evil/..." does NOT begin with this
//     prefix (its next character after "skeleton" is "-", not "/"), so the carve-out
//     cannot be widened by a confusable path. Do not drop the slash.
//
// This constant stays a SET OF ONE hardcoded prefix. Its own rule was that a
// second exclusion needs "a fresh human ruling, not an edit to this constant."
//
// GENERALIZATION AUTHORIZED — Ian 2026-09-01 desk walk. That fresh human ruling
// is now recorded: rather than adding a second hardcoded prefix here, the desk
// walk authorized a DECLARED, opt-in mechanism — a corpus excludes itself by
// dropping the fixtureCorpusMarkerName marker at its root (fixturecorpus.go),
// consulted by isExcludedFixturePath IN ADDITION TO this prefix. This prefix is
// KEPT unchanged (the education corpus need not carry a marker). The generalized
// path preserves both invariants above: it is a PATH check, and it is
// DECLARED + FAIL-CLOSED — absent a marker on disk, a subtree is corroborated
// exactly as today. No path-name / testdata / examples heuristic is added.
const corroborateExcludedFixturePrefix = "docs/streams/education/assay-tutorial-skeleton/"

// isExcludedFixturePath reports whether path (repo-relative, resolved against
// root) is a fixture-corpus path whose human:<name> stamps the corroborate scan
// deliberately skips. It is excluded when EITHER:
//
//   - it lies UNDER corroborateExcludedFixturePrefix — the original, hardcoded
//     set-of-one. Because the constant carries its trailing slash, this is a
//     proper path-boundary match: a plain HasPrefix here cannot leak to a
//     "...-skeleton-evil" sibling that merely shares the leading substring; OR
//   - it lies at or under a directory that DECLARES the fixtureCorpusMarkerName
//     marker on disk (isFixtureCorpusPath) — the generalized, opt-in mechanism.
//
// GENERALIZATION AUTHORIZED — Ian 2026-09-01 desk walk. The set-of-one below was
// deliberately closed: its own doc demanded that any second exclusion be "a fresh
// human ruling, not an edit to this constant." That ruling is now recorded — the
// desk walk authorized generalizing the exclusion beyond the education corpus to
// ANY corpus that DECLARES itself a fixture corpus with the marker file, while
// KEEPING the education prefix intact. The two invariants of the original
// carve-out are preserved by the generalized path: it is a PATH check (never a
// NAME check), and it is DECLARED + FAIL-CLOSED (no marker on disk => the subtree
// is corroborated exactly as before). See fixturecorpus.go.
func isExcludedFixturePath(root, path string) bool {
	return strings.HasPrefix(path, corroborateExcludedFixturePrefix) ||
		isFixtureCorpusPath(root, path)
}

// stampRow records where in a stream-board status table a human:<name> stamp was
// found, so the pre-existing exemption (see stampIsPreExisting) can compare the
// stamp's own cell against the SAME brief's row at the PR merge-base. It is
// populated only for a stamp on a recognizable brief status-table row; a stamp in
// a brief's frontmatter, an Evidence table, or any non-board line carries none,
// and such a stamp is never treated as pre-existing (it stays fully gated).
type stampRow struct {
	BriefKey  string // the row's stable brief id (the brief-NN number), matched across a re-render
	CellIndex int    // index of the pipe-delimited cell that carried this stamp
	Cell      string // that cell's raw text, compared byte-for-byte (trimmed) against the base row
}

// stamp describes a single human:<name> occurrence found in a PR diff.
type stamp struct {
	Name string // the <name> portion (e.g. "alex")
	File string // the file it appeared in
	Line string // the full added line (first 120 chars, for context)

	// Rows lists every brief status-table row this (name,file) stamp was found on
	// (accumulated across the dedup below). It is the input to the pre-existing
	// exemption: a stamp is pre-existing only when EVERY row it appears on is
	// byte-identical to that brief's row at the merge-base, so a genuinely NEW
	// (or edited) row anywhere forces the whole (name,file) stamp to stay gated.
	Rows []stampRow
	// PreExisting is set by markPreExisting once the base rows are read. When true,
	// the stamp's cell matched the merge-base for every row it appears on — it was
	// authored and corroborated on some earlier PR, so this diff's re-render does
	// not re-gate it. It does NOT fail the run.
	PreExisting bool
}

// ---- brief status-table row parsing (pre-existing exemption) ---------------------

// briefLinkRe extracts the stable brief id from a stream-board row's Brief cell —
// the numeric prefix of the `[title](brief-NN-...md)` link. The number is what
// survives a table RE-RENDER (line numbers shift, titles may be reflowed), so it
// is the row key the pre-existing exemption matches on, never the line position.
var briefLinkRe = regexp.MustCompile(`\]\(brief-([0-9]+)`)

// briefNumRe matches a bare brief number in the leading `#` cell — the fallback
// row key for a status table whose Brief cell carries no markdown link.
var briefNumRe = regexp.MustCompile(`^[0-9]+$`)

// splitTableCells splits a markdown table row on `|`, dropping the leading and
// trailing pipe, and returns the inner cells with their surrounding whitespace
// intact (callers trim when they compare). A line that is not a pipe row yields
// nil. Brief titles and Reviewed cells never contain a literal `|`, so a naive
// split is exact for the rows this scan cares about.
func splitTableCells(line string) []string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return nil
	}
	t = strings.TrimSuffix(strings.TrimPrefix(t, "|"), "|")
	return strings.Split(t, "|")
}

// briefRowKey returns the stable brief id of a stream-board status-table row, or
// "" when the line is not such a row. The status table is `| # | Brief | Wave |
// Effort | Status | Verified | Reviewed |` (7 cells); an Evidence table is 4
// cells, so the >=6-cell floor excludes it. The id is the brief-NN number from the
// Brief cell's link when present (immune to renumbering and re-titling), else the
// bare number in the `#` cell.
func briefRowKey(line string) string {
	cells := splitTableCells(line)
	if len(cells) < 6 {
		return ""
	}
	if m := briefLinkRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	if first := strings.TrimSpace(cells[0]); briefNumRe.MatchString(first) {
		return first
	}
	return ""
}

// cellAt returns the cell at index idx of a pipe row (ok=false when the row has
// too few cells — e.g. the base row was re-shaped, in which case the stamp is not
// treated as pre-existing, the fail-closed direction).
func cellAt(line string, idx int) (string, bool) {
	cells := splitTableCells(line)
	if idx < 0 || idx >= len(cells) {
		return "", false
	}
	return cells[idx], true
}

// findStampCell returns the index and raw text of the cell that carries the
// human:<name> stamp for name (lowercased), so the pre-existing check compares the
// stamp's OWN cell — the Reviewed cell in practice, but located by content so a
// stamp recorded in another column is still compared against the right cell.
func findStampCell(cells []string, name string) (int, string, bool) {
	needle := "human:" + name
	for i, c := range cells {
		if strings.Contains(strings.ToLower(c), needle) {
			return i, c, true
		}
	}
	return 0, "", false
}

// briefRowsByKey indexes a board file's content by brief-row key -> full row line,
// for the base-side lookup. Only recognizable status-table rows are indexed.
func briefRowsByKey(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if k := briefRowKey(line); k != "" {
			out[k] = line
		}
	}
	return out
}

// briefRowsAtRef reads the board file at repo-relative path as it stood at the
// given git ref (the PR merge-base) and indexes its brief rows by key. It is a
// package var so tests substitute base content without a git fixture. A read
// failure (no git, shallow clone, file absent at the base — i.e. added by this
// branch) yields nil, so NO row matches and nothing is exempted: the fail-closed
// direction, the same one consumerEntriesAtBase chose for the same reason.
var briefRowsAtRef = func(root, ref, path string) map[string]string {
	if ref == "" {
		return nil
	}
	content, err := exec.Command("git", "-C", root, "show", ref+":"+filepath.ToSlash(path)).Output()
	if err != nil {
		return nil
	}
	return briefRowsByKey(string(content))
}

// prMergeBaseSHA resolves the PR's merge-base commit against the local HEAD (the
// PR head in CI), so `git show <merge-base>:<path>` reads the base version of a
// board file. It reads the PR's base branch name from the API, then resolves the
// merge-base locally. "" on any failure — the caller then exempts nothing.
func prMergeBaseSHA(root, repo string, pr int) string {
	baseRef := ghPRBaseRef(repo, pr)
	if baseRef == "" {
		return ""
	}
	for _, ref := range []string{"origin/" + baseRef, baseRef} {
		out, err := exec.Command("git", "-C", root, "merge-base", ref, "HEAD").Output()
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return ""
}

// ghPRBaseRef returns the PR's base branch name (e.g. "main").
func ghPRBaseRef(repo string, pr int) string {
	out, err := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", pr),
		"--repo", repo, "--json", "baseRefName", "-q", ".baseRefName").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// stampIsPreExisting reports whether a stamp's cell is byte-identical (trimmed) to
// the SAME brief's cell at the merge-base, for EVERY board row it was found on. A
// stamp with no recorded board row, or one whose brief is absent from the base, or
// whose cell text differs (a date or name was edited), is NOT pre-existing and
// stays fully gated. Requiring ALL rows to match is what keeps a genuinely new,
// uncorroborated row from hiding behind a pre-existing one that shares its name.
func stampIsPreExisting(s stamp, baseRows map[string]string) bool {
	if len(s.Rows) == 0 || baseRows == nil {
		return false
	}
	for _, r := range s.Rows {
		if r.BriefKey == "" {
			return false
		}
		baseRow, ok := baseRows[r.BriefKey]
		if !ok {
			return false // brief not present at the base: this branch added the row
		}
		baseCell, ok := cellAt(baseRow, r.CellIndex)
		if !ok || strings.TrimSpace(baseCell) != strings.TrimSpace(r.Cell) {
			return false // re-shaped or edited cell — not a byte-identical re-render
		}
	}
	return true
}

// markPreExisting sets PreExisting on each stamp using the pre-read base rows,
// keyed by the stamp's file. Pure (no git/network) so the exemption's semantics
// are table-testable end to end.
func markPreExisting(stamps []stamp, baseRowsByFile map[string]map[string]string) {
	for i := range stamps {
		stamps[i].PreExisting = stampIsPreExisting(stamps[i], baseRowsByFile[stamps[i].File])
	}
}

// stampsInDiff parses a unified diff and returns every human:<name> stamp found on
// ADDED lines (lines starting with "+" but not "+++"). Each stamp records the name,
// the file it was added to, and the line content for context.
//
// root is the checkout root against which a declared fixture-corpus marker is
// resolved (see isExcludedFixturePath / fixturecorpus.go). It is "" when there is
// no checkout to consult, in which case only the hardcoded education prefix
// excludes — the marker mechanism fails closed to "fully scanned".
func stampsInDiff(root, diff string) []stamp {
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
		// A stamp on a file under a fixture corpus is deliberately NOT
		// corroborated — the corpus carries human:<name> notation with fictional
		// personas (or captured content) that can never map to a login. This
		// covers the education tutorial-skeleton prefix AND any subtree that
		// DECLARES itself a fixture corpus with the on-disk marker (Ian 2026-09-01
		// desk walk). See isExcludedFixturePath. This is a PATH check, so a forged
		// human:<name> anywhere OUTSIDE a declared corpus is unaffected.
		if isExcludedFixturePath(root, curFile) {
			continue
		}
		// Strip the leading "+" for matching.
		content := strings.TrimPrefix(trimmed, "+")
		lineCtx := content
		if len(lineCtx) > 120 {
			lineCtx = lineCtx[:120] + "..."
		}
		// When the added line is a recognizable brief status-table row, record
		// where each stamp sits so the pre-existing exemption can compare that
		// exact cell against the same brief's row at the merge-base.
		briefKey := briefRowKey(content)
		var cells []string
		if briefKey != "" {
			cells = splitTableCells(content)
		}
		for _, m := range humanStampRe.FindAllStringSubmatch(content, -1) {
			name := strings.ToLower(m[1])
			s := stamp{
				Name: name,
				File: curFile,
				Line: strings.TrimSpace(lineCtx),
			}
			if briefKey != "" {
				if idx, cell, ok := findStampCell(cells, name); ok {
					s.Rows = []stampRow{{BriefKey: briefKey, CellIndex: idx, Cell: cell}}
				}
			}
			out = append(out, s)
		}
		// A confusable-name stamp is recorded too, under its raw name. It will
		// fail the human-login lookup and report MISSING-CORROBORATION — loud
		// refusal rather than the silent "no stamps — clean" it used to produce.
		for _, name := range confusableStampNames(content) {
			out = append(out, stamp{
				Name: strings.ToLower(name),
				File: curFile,
				Line: strings.TrimSpace(lineCtx),
			})
		}
	}
	// Deduplicate by (name, file) — multiple human:<name> on the same line or
	// in the same file count as one stamp for that file. Row provenance is
	// ACCUMULATED across the merge (not dropped with the duplicate), so the
	// pre-existing exemption sees every board row this (name,file) appears on and
	// can require them all to be byte-identical to the base.
	seen := map[string]int{}
	var deduped []stamp
	for _, s := range out {
		key := s.Name + "\x00" + s.File
		if idx, ok := seen[key]; ok {
			deduped[idx].Rows = append(deduped[idx].Rows, s.Rows...)
			continue
		}
		seen[key] = len(deduped)
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
	// verdictCitationUncheckable is the three-state middle: the instrument never
	// observed the cited artifact (a transient/auth/rate-limit fetch failure), so
	// it can neither confirm nor deny the citation. It is NOT rounded down to
	// verdictMissing — an absence the check never observed must not be fabricated
	// (clause 4 / clause 8). It does not fail the gate.
	verdictCitationUncheckable
	// verdictPreExisting is a human:<name> stamp whose Reviewed cell is
	// byte-identical to the SAME brief's row at the PR merge-base. The stamp was
	// authored and corroborated on some earlier PR; this diff merely re-renders
	// its row (a board migration re-emits whole tables), so re-gating it against
	// THIS PR's reviews is a category error — its corroboration lives on its own
	// PR, not here. Like a corroborated stamp, it does NOT fail the run.
	verdictPreExisting
)

func (v verdict) String() string {
	switch v {
	case verdictCorroborated:
		return "CORROBORATED"
	case verdictMissing:
		return "MISSING-CORROBORATION"
	case verdictCitationUncheckable:
		return "COULD-NOT-CHECK"
	case verdictPreExisting:
		return "PRE-EXISTING"
	default:
		return ""
	}
}

// ---- PR data retrieval -----------------------------------------------------------

// ghPRFile is one entry of the GitHub "List pull request files" REST API
// (GET /repos/{owner}/{repo}/pulls/{pull_number}/files). Only the two fields
// the corroborate scan needs are decoded: the path and its unified-diff hunks.
type ghPRFile struct {
	Filename string `json:"filename"`
	Patch    string `json:"patch"`
}

// prFilesToDiff reconstructs a unified-diff string (parseable by stampsInDiff)
// from the PR "files" API entries: each file contributes a "+++ b/<name>"
// header followed by its patch hunks. Files with no patch (binary/oversize)
// contribute only the header — no added lines, nothing to corroborate.
func prFilesToDiff(files []ghPRFile) string {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "+++ b/%s\n", f.Filename)
		if f.Patch != "" {
			b.WriteString(f.Patch)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// fetchPRDiff returns a unified diff of the given repo's PR, reconstructed from
// the paginated "List pull request files" REST API.
func fetchPRDiff(repo string, pr int) (string, error) {
	// gh pr diff caps at 300 files (HTTP 406 on larger PRs). The paginated
	// "List PR files" API has no such cap; reconstruct a unified diff from the
	// per-file patch fields.
	cmd := exec.Command("gh", "api", "--paginate", "--slurp",
		fmt.Sprintf("repos/%s/pulls/%d/files", repo, pr))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh api pulls/%d/files: %w", pr, err)
	}
	// --slurp + --paginate yields an array of per-page arrays: [[...],[...]].
	var pages [][]ghPRFile
	if err := json.Unmarshal(out, &pages); err != nil {
		return "", fmt.Errorf("unmarshal PR %d files: %w", pr, err)
	}
	var files []ghPRFile
	for _, p := range pages {
		files = append(files, p...)
	}
	return prFilesToDiff(files), nil
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
//
// There are THREE accepted corroboration anchors, checked in order and none
// replacing another:
//
//  1. an APPROVED review by the named human on the PR (strongest signal);
//  2. an explicit approval COMMENT by the named human on the PR;
//  3. a linked, human-CLOSED needs-decision issue carrying this record's per-record
//     decision-gate marker (the sanctioned ratification channel — see
//     decisionGateCorroboration and the house tracker's ruling (Option 1: a linked
//     decision issue closed by the blessed login corroborates)). This third anchor is
//     ADDITIVE: it fires for a stamp on either a brief-<NN>.md brief OR a DR-<slug>.md
//     decision record whose file links such an issue, and it never weakens anchors 1
//     and 2, which are unchanged.
//
// gates carries the pre-fetched decision-issue state for the third anchor, keyed by
// the record file (brief or DR) the stamp was found in; it is empty/nil when only the
// PR anchors are in play (every existing caller and test), so the two PR anchors
// decide exactly as before.
func corroborateStamps(stamps []stamp, data *ghPRData, repo string, pr int, gates decisionGateLinks) []corroborateResult {
	if len(stamps) == 0 {
		return []corroborateResult{{Verdict: verdictNoStamp}}
	}
	var results []corroborateResult
	for _, s := range stamps {
		// Pre-existing exemption: a stamp whose cell is byte-identical to the same
		// brief's row at the merge-base was authored and corroborated on an earlier
		// PR. This diff only re-renders its row, so it is reported PRE-EXISTING and
		// does NOT fail the run — never re-gated against THIS PR's reviews. A NEW or
		// EDITED cell never reaches here (markPreExisting left PreExisting false), so
		// it stays fully gated below exactly as before.
		if s.PreExisting {
			results = append(results, corroborateResult{
				Stamp:    s,
				Verdict:  verdictPreExisting,
				Evidence: "Reviewed cell is byte-identical to this brief's row at the PR merge-base — pre-existing stamp, corroborated on its original PR; this diff only re-renders the row",
			})
			continue
		}
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

		// Check APPROVED reviews by this login first (stronger signal). No SHA
		// is required here: this path corroborates that the named human ACTED,
		// not that they signed a particular tree.
		if data != nil {
			if r, ok := approvedReviewAt(data.Reviews, login, ""); ok {
				results = append(results, corroborateResult{
					Stamp:    s,
					Verdict:  verdictCorroborated,
					Login:    login,
					Evidence: fmt.Sprintf("APPROVED review by %s: %s", login, reviewURL(repo, pr, r)),
				})
				goto nextStamp
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

		// Third anchor: a linked, human-closed needs-decision issue carrying this
		// record's per-record decision-gate marker — the sanctioned ratification
		// channel (the house tracker's ruling). A gate:human decision brief OR a
		// DR-<slug>.md decision record whose ruling was recorded by CLOSING its
		// decision-issue (rather than as a PR approval) corroborates through this path.
		// It requires no PR data, so it is checked after — and independently of — the
		// two PR anchors above.
		if ev, ok := decisionGateCorroboration(s, gates); ok {
			results = append(results, corroborateResult{
				Stamp:    s,
				Verdict:  verdictCorroborated,
				Login:    login,
				Evidence: ev,
			})
			goto nextStamp
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

// stampResultsFail reports whether any stamp result would fail the run — i.e. it
// is MISSING-CORROBORATION. It is the exit-code condition runCorroborate applies to
// the stamp lane (verdictPreExisting, like verdictCorroborated, does NOT fail).
func stampResultsFail(results []corroborateResult) bool {
	for _, r := range results {
		if r.Verdict == verdictMissing {
			return true
		}
	}
	return false
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
//
// Before checking a stamp against this PR, it marks any stamp whose cell is
// byte-identical to the same brief's row at the PR merge-base as PRE-EXISTING
// (markPreExisting): that stamp was authored and corroborated on its own PR, so a
// board migration that re-emits whole tables does not re-gate it here. PRE-EXISTING,
// like CORROBORATED, does not fail the run.
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

	// The stamp scan below honours the same declared fixture-corpus markers the
	// lint does (isExcludedFixturePath -> isFixtureCorpusPath), and no lint runs
	// on this path — so announce them here too. Without this the only visible
	// trace of a skipped subtree would be stamps that never appear in the report.
	emitFixtureCorpusNotices(".", os.Stderr)

	prStrs := strings.Split(prsArg, ",")
	var allResults []corroborateResult
	var allCitationResults []citationResult
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
		// Fetch the PR diff ONCE and reuse it for both the stamp scan and the
		// prose/commit acceptance-citation scan.
		diff, err := fetchPRDiff(repo, pr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: PR #%d: %v\n", pr, err)
			return 1
		}

		// --- human:<name> STAMP corroboration ---
		stamps := stampsInDiff(".", diff)
		if len(stamps) == 0 {
			fmt.Printf("PR #%d: no human:<name> stamps found in diff — clean\n", pr)
		} else {
			data, err := fetchPRData(repo, pr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "statusgen: PR #%d: %v\n", pr, err)
				return 1
			}
			// Third corroboration anchor (the house tracker's ruling): pre-fetch the
			// needs-decision issues each brief-file OR DR-<slug>.md-record stamp links,
			// so a gate:human decision brief or a decision record ratified by CLOSING
			// its decision-issue can corroborate even without a PR approval anchor.
			// Empty when no record links such an issue — the two PR anchors then decide
			// exactly as before.
			gates := gatherDecisionGateLinks(".", repo, stamps)
			// Pre-existing exemption: read each touched board file as it stood at
			// the PR merge-base and mark any stamp whose cell is byte-identical to
			// its brief's base row. Such a stamp was corroborated on its ORIGINAL
			// PR; a board migration that re-emits whole tables must not re-gate it
			// here. A merge-base that cannot be resolved (or a base file that cannot
			// be read) exempts nothing — the fail-closed direction.
			mb := prMergeBaseSHA(".", repo, pr)
			baseRowsByFile := map[string]map[string]string{}
			for i := range stamps {
				f := stamps[i].File
				if _, ok := baseRowsByFile[f]; !ok {
					baseRowsByFile[f] = briefRowsAtRef(".", mb, f)
				}
			}
			markPreExisting(stamps, baseRowsByFile)
			for _, r := range corroborateStamps(stamps, data, repo, pr, gates) {
				allResults = append(allResults, r)
				if r.Verdict == verdictMissing {
					anyMissing = true
				}
			}
		}

		// --- human-acceptance / human-ruling CITATION corroboration ---
		for _, r := range checkCitationCorroboration(".", repo, diff, pr) {
			allCitationResults = append(allCitationResults, r)
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
		case verdictPreExisting:
			fmt.Printf("human:%s in %s PRE-EXISTING — %s\n", r.Stamp.Name, label, r.Evidence)
		case verdictMissing:
			fmt.Printf("human:%s in %s MISSING-CORROBORATION — %s\n", r.Stamp.Name, label, r.Evidence)
		}
	}

	if len(allResults) == 0 {
		fmt.Println("(no stamps found across all PRs)")
	}

	// State the accepted wording at the point of rejection.
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

	// --- acceptance/ruling CITATION section ---
	// A separate lane from the stamp report above: it reads FREE-PROSE and
	// commit-message claims that a named human accepted/ruled on something, and
	// requires an artifact by that human on the CITED issue/PR.
	if len(allCitationResults) > 0 {
		fmt.Println()
		fmt.Println("# human-acceptance / human-ruling citations")
		fmt.Println("# Scope: a claim in tracked prose or a commit message that a configured")
		fmt.Println("# human ACCEPTED or RULED ON something must be backed by a comment or")
		fmt.Println("# review authored by that human on the cited issue/PR (>=1 hit). An")
		fmt.Println("# unlinked claim, or one with no such artifact, is fabricated authority.")
		fmt.Println()
		for _, r := range allCitationResults {
			switch r.Verdict {
			case verdictCorroborated:
				fmt.Printf("citation of %s in %s CORROBORATED — %s\n",
					r.Citation.Name, r.Citation.Source, r.Evidence)
			case verdictMissing:
				fmt.Printf("citation of %s in %s MISSING-CORROBORATION — %s\n",
					r.Citation.Name, r.Citation.Source, r.Evidence)
			case verdictCitationUncheckable:
				// Surfaced, never a gate failure: the cited artifact could not be
				// read, so the check neither corroborates nor condemns the citation.
				fmt.Printf("citation of %s in %s COULD-NOT-CHECK — %s\n",
					r.Citation.Name, r.Citation.Source, r.Evidence)
			}
		}
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
