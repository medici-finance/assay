package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHumanStampRe(t *testing.T) {
	tests := []struct {
		line   string
		want   string // the captured name
		wantOk bool
	}{
		{"human:alex", "alex", true},
		{"human:ALEX", "ALEX", true},
		{"Reviewed: 2026-07-10 human:alex", "alex", true},
		{"| done | 2026-07-10 opus-verifier | 2026-07-10 human:alex |", "alex", true},
		{"some human:alex text", "alex", true},
		{"no stamp here", "", false},
		{"human: not keeping", "", false}, // space after colon — no word chars, no match
		{"", "", false},

		// ---- the anchor, BOTH directions ----
		//
		// REJECT — a human-shaped substring inside a larger token is NOT a stamp.
		// These all matched before the anchor was added; `superhuman:x` yielding
		// name "x" is what made `authorized-by: superhuman:alex` an authorization.
		{"superhuman:x", "", false},
		{"superhuman:alex", "", false},
		{"SUPERHUMAN:alex", "", false},
		{"non-human:alex", "", false},
		{"sub-human:alex", "", false},
		{"inhuman:alex", "", false},
		{"a_human:alex", "", false},
		{"transhuman:alex and posthuman:alex", "", false},
		//
		// ACCEPT — every legitimate surrounding a real stamp appears in must
		// still parse. Tightening the anchor may not cost a genuine stamp.
		{"(human:alex)", "alex", true},
		{"**human:alex**", "alex", true},
		{"`human:alex`", "alex", true},
		{"authorized-by: human:alex", "alex", true},
		{"authorized-by:human:alex", "alex", true},
		{"- human:alex", "alex", true},
		{"|human:alex|", "alex", true},
		{"\thuman:alex", "alex", true},
		{"\"human:alex\"", "alex", true},
	}
	for _, tc := range tests {
		m := humanStampRe.FindStringSubmatch(tc.line)
		gotOk := m != nil
		if gotOk != tc.wantOk {
			t.Errorf("humanStampRe on %q: got ok=%v, want ok=%v", tc.line, gotOk, tc.wantOk)
			continue
		}
		if !gotOk {
			continue
		}
		if m[1] != tc.want {
			t.Errorf("humanStampRe on %q: got name=%q, want %q", tc.line, m[1], tc.want)
		}
	}
}

func TestStampsInDiff(t *testing.T) {
	diff := `diff --git a/docs/streams/methodology/README.md b/docs/streams/methodology/README.md
index abc..def 100644
--- a/docs/streams/methodology/README.md
+++ b/docs/streams/methodology/README.md
@@ -50,7 +50,7 @@
-| 01 | Some brief | 0 | S | verified | 2026-07-10 opus-verifier | — |
+| 01 | Some brief | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 human:alex |
diff --git a/docs/streams/methodology/brief-03.md b/docs/streams/methodology/brief-03.md
index ghi..jkl 100644
--- a/docs/streams/methodology/brief-03.md
+++ b/docs/streams/methodology/brief-03.md
@@ -10,6 +10,8 @@
 ## Evidence
+| 1 | go test ./... | PASS | human:alex (non-implementer) |
+| 2 | go run statusgen | 0 | human:alex |
`
	stamps := stampsInDiff(diff)
	if len(stamps) != 2 {
		t.Fatalf("got %d stamps, want 2: %+v", len(stamps), stamps)
	}
	// First stamp: human:alex on an added row in the README
	if stamps[0].Name != "alex" {
		t.Errorf("stamp[0].Name = %q, want alex", stamps[0].Name)
	}
	if stamps[0].File != "docs/streams/methodology/README.md" {
		t.Errorf("stamp[0].File = %q, want docs/streams/methodology/README.md", stamps[0].File)
	}
	// Second stamp: human:alex in Evidence section
	if stamps[1].Name != "alex" {
		t.Errorf("stamp[1].Name = %q, want alex", stamps[1].Name)
	}
	if stamps[1].File != "docs/streams/methodology/brief-03.md" {
		t.Errorf("stamp[1].File = %q, want docs/streams/methodology/brief-03.md", stamps[1].File)
	}
}

func TestStampsInDiffDeduplication(t *testing.T) {
	// Two human:alex stamps in the same file should deduplicate to one.
	diff := `diff --git a/docs/streams/x/README.md b/docs/streams/x/README.md
--- a/docs/streams/x/README.md
+++ b/docs/streams/x/README.md
+| 01 | Foo | 0 | S | done | 2026-07-10 human:alex | 2026-07-10 human:alex |
`
	stamps := stampsInDiff(diff)
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want 1 (deduped): %+v", len(stamps), stamps)
	}
	if stamps[0].Name != "alex" {
		t.Errorf("stamp[0].Name = %q, want alex", stamps[0].Name)
	}
}

// TestHumanStampReMultipleOnOneLine guards the one hazard the confusable-name
// anchor introduces. RE2 has no lookbehind, so the boundary is a CONSUMED leading
// character — and a consuming boundary can, in general, eat the separator a later
// match needs. It does not here (the consumed character always precedes the
// match's own "human:", never follows a previous name), and this pins that:
// tightening the pattern must not cost a stamp that used to be found.
func TestHumanStampReMultipleOnOneLine(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"human:alex human:bob", []string{"alex", "bob"}},
		{"human:alex,human:bob", []string{"alex", "bob"}},
		{"(human:alex)(human:bob)", []string{"alex", "bob"}},
		{"| human:alex | human:bob |", []string{"alex", "bob"}},
		{"human:alex and superhuman:bob", []string{"alex"}},
		{"superhuman:alex and human:bob", []string{"bob"}},
	}
	for _, c := range cases {
		var got []string
		for _, m := range humanStampRe.FindAllStringSubmatch(c.line, -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("humanStampRe on %q: got %v, want %v", c.line, got, c.want)
		}
	}
}

// TestConfusableStampNames covers the second half of a
// homoglyph name is invisible to an ASCII-only `\w`, so the old behaviour was
// SILENCE — `--corroborate` printing "no human:<name> stamps found in diff —
// clean" for a diff that visibly stamps human:alex. Silence is the wrong direction
// for an unparseable authorization token, so these are surfaced and go on to fail
// the HumanLoginMap lookup.
//
// The detector is narrow ON PURPOSE, and the second group pins that: this repo's
// own docs and error strings are full of literal `human:<name>` placeholders, and
// flagging those would be a new false rejection on every prose file.
func TestConfusableStampNames(t *testing.T) {
	// Detected: the name's first rune is a non-ASCII letter.
	flagged := []string{
		"| done | 2026-07-31 opus | 2026-07-31 human:іan |", // Cyrillic і
		"authorized-by: human:аalex",                        // Cyrillic а
		"human:Ιan",                                         // Greek capital iota
	}
	for _, line := range flagged {
		if names := confusableStampNames(line); len(names) == 0 {
			t.Errorf("confusableStampNames(%q) found nothing — a homoglyph stamp must not be silently invisible", line)
		}
	}

	// Not detected: ASCII stamps (humanStampRe's job), placeholders, and prose.
	quiet := []string{
		"human:alex",
		"it needs a \"human:<name>\" token (e.g. 2026-07-15 human:alex)",
		"add `authorized-by: human:<name>` to the frontmatter",
		"superhuman:іan", // not a stamp at all — wrong prefix
		"no stamp here",
		"human: not keeping",
		"",
	}
	for _, line := range quiet {
		if names := confusableStampNames(line); len(names) != 0 {
			t.Errorf("confusableStampNames(%q) flagged %v — this must stay quiet or it becomes a false rejection on ordinary prose", line, names)
		}
	}
}

// TestStampsInDiffConfusableIsLoud is the end-to-end direction that matters for
// a diff carrying only a homoglyph stamp must NOT come back as
// "no stamps — clean". It is surfaced, fails to resolve, and the run reports
// MISSING-CORROBORATION.
func TestStampsInDiffConfusableIsLoud(t *testing.T) {
	diff := "diff --git a/docs/streams/x/README.md b/docs/streams/x/README.md\n" +
		"--- a/docs/streams/x/README.md\n" +
		"+++ b/docs/streams/x/README.md\n" +
		"@@ -1,1 +1,1 @@\n" +
		"+| 01 | b | 0 | S | done | 2026-07-31 opus | 2026-07-31 human:іan |\n"

	stamps := stampsInDiff(diff)
	if len(stamps) == 0 {
		t.Fatal("a homoglyph human: stamp produced no stamps — --corroborate would print \"clean\" and exit 0 on it")
	}
	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1)
	for _, r := range results {
		if r.Verdict != verdictMissing {
			t.Errorf("homoglyph stamp %q got verdict %v, want MISSING-CORROBORATION", r.Stamp.Name, r.Verdict)
		}
	}
}

func TestApprovalPhraseRe(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"I approve of this PR. thank you for checking", true},
		{"Approved", true},
		{"I approve this change", true},
		{"sign off on this", true},
		{"sign-off done", true},
		{"this looks good", false},
		{"no approval phrase here", false},
		{"", false},
		// "approve" as a substring of another word should NOT match.
		{"unapproved change", false},
	}
	for _, tc := range tests {
		got := hasApprovalPhrase(tc.body)
		if got != tc.want {
			t.Errorf("hasApprovalPhrase(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

// TestApprovalPhraseLGTM covers the LGTM widening. Every ACCEPT case below is a
// VERBATIM comment authored by `ada` in example-org/tracker or
// medici-finance/assay — harvested 2026-08-02 from
// GET /repos/{repo}/issues/comments, 301 comments, 58 of which contain "lgtm"
// (50 of those being the bare word). Every one of them was rejected by the
// shipped pattern, which is why correctly-signed `gate: human` flips reported
// MISSING-CORROBORATION and the human was asked to re-sign in different words.
//
// The REJECT cases are the other direction, and two of them are the reason this
// widening is not just "add more words":
//
//   - "please go ahead with explicitly pruning" is a real ada comment and a
//     real INSTRUCTION TO DO WORK. "go ahead" appears twice in the corpus and
//     would be a plausible thing to add; it is not added, because it would accept
//     a non-approval.
//   - "don't we need to change a hostname or something? if not lgtm" is also a
//     real ada comment, and it is an APPROVAL ("if not, lgtm"). It is the only
//     occurrence of the sequence "not lgtm" in the corpus. It is carved out by
//     lgtmConditionalApprovalRe so that the `not lgtm` refusal clause added for
//     an independent review (see TestApprovalPhraseLGTMNegations) can block
//     a real refusal without manufacturing a false rejection here. This case MUST
//     keep passing.
func TestApprovalPhraseLGTM(t *testing.T) {
	accept := []string{
		"lgtm",
		"LGTM",
		"Lgtm",
		"lgtm 👍 ",
		"lgtm. let's go",
		"lgtm. flip",
		"lgtm. pls flip",
		"option a lgtm",
		"done. lgtm on 1456",
		"lgtm if you are waiting on a human",
		"don't we need to change a hostname or something? if not lgtm",
		"approved, LGTM",
		"looks good to me",
		"ship it",
		"+1",
		"  +1  ",
		"+1 👍",
	}
	for _, body := range accept {
		if !hasApprovalPhrase(body) {
			t.Errorf("hasApprovalPhrase(%q) = false, want true — this is a real approval and #263 is exactly this rejection", body)
		}
	}

	reject := []string{
		"please go ahead with explicitly pruning",
		"this has my blessing, once your issues are addressed",
		"which pr caused this issue?",
		"why is this still open? you can close it once i make a decision",
		"we're wiping",
		"this looks good",
		"algtm",      // not a whole word
		"lgtmx",      // not a whole word
		"+100 lines", // an inline +1-alike is not an approval
		"+1 line diff",
		"see PR +1234",
		"don't ship it",
		"this does not look good",
		"",
	}
	for _, body := range reject {
		if hasApprovalPhrase(body) {
			t.Errorf("hasApprovalPhrase(%q) = true, want false — widening for #263 must not admit a non-approval", body)
		}
	}
}

// TestApprovalPhraseLGTMNegations is the other half of the LGTM
// widening, and closes finding R1 of an independent review: the widening
// added `lgtm` / `ship it` to approvalPhraseRe WITHOUT the refusal forms
// negationPhraseRe already carried for `approve`, so explicit refusals read as
// sign-offs. Every mustReject row below returned true (ACCEPT) before that fix.
//
// The control rows matter as much as the refusals: this is a trust anchor, so a
// tightening has to be shown not to cost a legitimate approval. The mustAccept
// list therefore re-pins the corpus idiom "if not lgtm" plus every ACCEPT shape
// that shares a trigger word with the new blocklist.
func TestApprovalPhraseLGTMNegations(t *testing.T) {
	mustReject := []string{
		// Refusal verbs governing the new vocabulary — the exact shapes the old
		// verb already refused ("i cannot approve this").
		"i cannot lgtm this",
		"I can not lgtm this",
		"i can't lgtm this",
		"i couldn't lgtm this as it stands",
		"i will not lgtm this",
		"i won't lgtm this",
		"do not lgtm this",
		"don't lgtm this yet",
		"i am unable to lgtm this",
		"i decline to lgtm this",
		"i refuse to lgtm this",
		"i would never lgtm this",
		"i cannot ship it",
		"i will not ship it",
		// Bare "not" + the vocabulary.
		"this is not lgtm, please fix",
		"not lgtm",
		"not yet lgtm",
		"not really lgtm",
		"definitely not ship it",
		// "nack" as a standalone refusal token.
		"nack, not lgtm",
		"nack",
		"NACK — see the inline comments",
		// Negating prefixes: \b treats the hyphen as a word boundary, so plain
		// \blgtm\b matches inside all of these.
		"non-lgtm",
		"NON-LGTM",
		"un-lgtm",
		"nonlgtm",
		"anti-lgtm",
		"no lgtm from me",
		// Asking FOR approval is not giving it.
		"is this lgtm?",
		"is that lgtm or do you want changes",
		"can someone lgtm this?",
		"could you lgtm this please",
		"can i lgtm this myself?",
		"please lgtm when you get a chance",
		"needs an lgtm from alex",
		"pending lgtm",
		"awaiting lgtm from the desk",
		"waiting on lgtm",
		"merged without lgtm",
		"lgtm?",
		"lgtm ?",
		// "ship it" as a noun, not an imperative.
		"the ship it button is greyed out",
		"a ship it label would help here",
		"this ship it flow is broken",
	}
	for _, body := range mustReject {
		if hasApprovalPhrase(body) {
			t.Errorf("hasApprovalPhrase(%q) = true, want false — this is a REFUSAL or a REQUEST for approval, and reading it as sign-off is the R1 finding", body)
		}
	}

	// Both directions. Each of these shares a trigger word with the blocklist
	// above and must still corroborate.
	mustAccept := []string{
		// The one measured corpus idiom in which "not lgtm" IS an approval.
		"don't we need to change a hostname or something? if not lgtm",
		"if not lgtm",
		"if not, lgtm",
		"if not then lgtm",
		// Real corpus comment: contains "waiting", but the lgtm precedes it.
		"lgtm if you are waiting on a human",
		// A caveat followed by an approval — the reason the `not` clause uses a
		// narrow filler set instead of `approve`'s 3-word window.
		"not sure about the naming but lgtm",
		"i'm not certain this is the best approach, but lgtm",
		// Plain approvals unaffected by any of the above.
		"lgtm",
		"LGTM",
		"approved, LGTM",
		"looks good to me",
		"ship it",
		"+1",
	}
	for _, body := range mustAccept {
		if !hasApprovalPhrase(body) {
			t.Errorf("hasApprovalPhrase(%q) = false, want true — the R1 negation grammar must not cost a real approval", body)
		}
	}
}

// TestNegationPhraseRe — T7: negated/conditional text must NOT corroborate.
func TestNegationPhraseRe(t *testing.T) {
	tests := []struct {
		body string
		want bool // should NOT corroborate
	}{
		// Negated phrases — must NOT corroborate.
		{"do not approve this PR", false},
		{"I do NOT approve this change", false},
		{"not approved yet", false},
		{"This is not approved", false},
		{"needs sign-off from human:<name> first", false},
		{"needs sign off from security team", false},
		{"pending approval from ops", false},
		{"awaiting approval", false},
		{"requires approval from lead", false},
		{"we cannot approve this without testing", false},
		{"I am unable to approve this", false},
		{"without approval of the committee", false},
		// T7 review residual bypasses (an independent review, T7 section) —
		// contraction, interposed adverb, and passive forms that slipped the
		// original pattern must all fail to corroborate.
		{"don't approve this", false},
		{"I would not approve this", false},
		{"not yet approved", false},
		{"never approve this", false},
		{"I can not approve this", false},
		{"cannot be approved", false},
		{"NACK, approve later", false},
		// Straightforward approvals — must corroborate.
		{"I approve this PR", true},
		{"approved, LGTM", true},
		{"sign off on this change", true},
		{"sign-off complete", true},
		// Non-approval statements — no match either way.
		{"this looks good", false},
		{"just a comment", false},
		// Unapproved as a word should not match (substring of unapproved).
		{"unapproved change", false},
	}
	for _, tc := range tests {
		got := hasApprovalPhrase(tc.body)
		if got != tc.want {
			t.Errorf("hasApprovalPhrase(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

// TestNegationPhraseDirect — T7: verify the negation regex directly against known
// patterns that must negate an otherwise-matching approval phrase.
func TestNegationPhraseDirect(t *testing.T) {
	mustNegate := []string{
		"do not approve",
		"do NOT approve this",
		"not approved",
		"needs sign-off from ada",
		"needs sign off from team",
		"pending approval",
		"awaiting approval from",
		"requires approval",
		"cannot approve",
		"unable to approve",
		"without approval",
		// T7 review residual bypasses (an independent review): contraction,
		// interposed adverb, passive, and nack/never/decline/refuse forms.
		"don't approve this",
		"I would not approve this",
		"not yet approved",
		"never approve this",
		"I can not approve this",
		"cannot be approved",
		"NACK, approve later",
	}
	shouldNotNegate := []string{
		"approve",
		"approved",
		"sign off",
		"sign-off",
		"pending",
		"needs review",
		"",
	}
	for _, s := range mustNegate {
		if !hasNegationPhrase(s) {
			t.Errorf("hasNegationPhrase(%q) = false, want true (must negate)", s)
		}
	}
	for _, s := range shouldNotNegate {
		if hasNegationPhrase(s) {
			t.Errorf("hasNegationPhrase(%q) = true, want false (should not negate)", s)
		}
	}
}

func TestHumanLoginMap(t *testing.T) {
	// Verify the canonical mapping exists.
	if got, _ := HumanLogin("alex"); got != "ada" {
		t.Errorf("HumanLogin(alex) = %q, want ada (from the fixture ASSAY_HUMAN_LOGIN_MAP)", got)
	}
}

// ---- corroboration verdict tests against fixture PR data -----------------------

// makeData builds a ghPRData from reviews and comments JSON strings.
func makeData(reviewsJSON, commentsJSON string) *ghPRData {
	var reviews []ghReview
	if reviewsJSON != "" {
		if err := json.Unmarshal([]byte(reviewsJSON), &reviews); err != nil {
			panic(err)
		}
	}
	var comments []ghComment
	if commentsJSON != "" {
		if err := json.Unmarshal([]byte(commentsJSON), &comments); err != nil {
			panic(err)
		}
	}
	return &ghPRData{Reviews: reviews, Comments: comments}
}

func TestCheckStamps_ApprovedReview(t *testing.T) {
	// Case 1: stamp + APPROVED review → CORROBORATED
	data := makeData(
		`[{"author":{"login":"ada"},"body":"","state":"APPROVED","submittedAt":"2026-07-10T16:29:18Z","id":"PRR_123"}]`,
		`[]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 1)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Errorf("verdict = %v, want CORROBORATED", results[0].Verdict)
	}
}

func TestCheckStamps_ApprovalComment(t *testing.T) {
	// Case 2: stamp + approval comment → CORROBORATED
	data := makeData(
		`[]`,
		`[{"author":{"login":"ada"},"body":"I approve of this PR. thank you for checking","createdAt":"2026-07-10T16:23:06Z","url":"https://github.com/o/r/pull/2#issuecomment-1"}]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 2)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Errorf("verdict = %v, want CORROBORATED", results[0].Verdict)
	}
	if !strings.Contains(results[0].Evidence, "issuecomment-1") {
		t.Errorf("expected comment URL in evidence, got: %s", results[0].Evidence)
	}
}

func TestCheckStamps_DifferentLoginComment(t *testing.T) {
	// Case 3: stamp + comment by DIFFERENT login → MISSING
	data := makeData(
		`[]`,
		`[{"author":{"login":"someone-else"},"body":"I approve of this PR","createdAt":"2026-07-10T16:23:06Z","url":"https://github.com/o/r/pull/3#issuecomment-1"}]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 3)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION", results[0].Verdict)
	}
}

func TestCheckStamps_AgentCommentQuotingHuman(t *testing.T) {
	// Case 4: stamp + agent comment quoting the human → MISSING (author login is what counts)
	data := makeData(
		`[]`,
		`[{"author":{"login":"example-reviewer-app"},"body":"ada approved this PR. approved.","createdAt":"2026-07-10T16:29:18Z","url":"https://github.com/o/r/pull/4#issuecomment-1"}]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 4)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION (agent comment quoting human doesn't count)", results[0].Verdict)
	}
}

func TestCheckStamps_NoStamps(t *testing.T) {
	// Case 5: no stamps → clean
	stamps := []stamp{}
	results := corroborateStamps(stamps, nil, "o/r", 5)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictNoStamp {
		t.Errorf("verdict = %v, want noStamp", results[0].Verdict)
	}
}

func TestCheckStamps_UnknownName(t *testing.T) {
	// Unknown name (not in HumanLoginMap) → MISSING by definition
	data := makeData(`[]`, `[]`)
	stamps := []stamp{{Name: "unknownperson", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 6)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION", results[0].Verdict)
	}
	if !strings.Contains(results[0].Evidence, "no mapping") {
		t.Errorf("evidence should mention missing mapping, got: %s", results[0].Evidence)
	}
}

func TestCheckStamps_MultipleStamps(t *testing.T) {
	// Two stamps: one corroborated, one missing
	data := makeData(
		`[{"author":{"login":"ada"},"body":"","state":"APPROVED","submittedAt":"2026-07-10T16:29:18Z","id":"PRR_123"}]`,
		`[]`,
	)
	stamps := []stamp{
		{Name: "alex", File: "docs/streams/a/README.md"},
		{Name: "unknown", File: "docs/streams/b/README.md"},
	}
	results := corroborateStamps(stamps, data, "o/r", 7)

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Errorf("stamp[0] verdict = %v, want CORROBORATED", results[0].Verdict)
	}
	if results[1].Verdict != verdictMissing {
		t.Errorf("stamp[1] verdict = %v, want MISSING-CORROBORATION", results[1].Verdict)
	}
}

func TestCheckStamps_ApprovedReviewWinsOverComment(t *testing.T) {
	// When both an APPROVED review and an approval comment exist, the review wins
	// (stronger signal). This tests precedence, not logic.
	data := makeData(
		`[{"author":{"login":"ada"},"body":"LGTM","state":"APPROVED","submittedAt":"2026-07-10T16:29:18Z","id":"PRR_456"}]`,
		`[{"author":{"login":"ada"},"body":"I approve","createdAt":"2026-07-10T16:23:06Z","url":"https://github.com/o/r/pull/8#issuecomment-1"}]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 8)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Errorf("verdict = %v, want CORROBORATED", results[0].Verdict)
	}
	// Evidence should reference the APPROVED review URL, not the comment.
	if !strings.Contains(results[0].Evidence, "PRR_456") {
		t.Errorf("evidence should reference the APPROVED review, got: %s", results[0].Evidence)
	}
}

func TestCheckStamps_CaseInsensitiveLogin(t *testing.T) {
	// Login comparison is case-insensitive.
	data := makeData(
		`[{"author":{"login":"ADA"},"body":"","state":"APPROVED","submittedAt":"2026-07-10T16:29:18Z","id":"PRR_789"}]`,
		`[]`,
	)
	stamps := []stamp{{Name: "alex", File: "docs/streams/m/README.md"}}
	results := corroborateStamps(stamps, data, "o/r", 9)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Errorf("verdict = %v, want CORROBORATED", results[0].Verdict)
	}
}

func TestRepoFromOrigin(t *testing.T) {
	// This is best-effort in unit tests (depends on the git remote).
	// Just verify it doesn't panic and returns something or empty string.
	_ = repoFromOrigin()
}
