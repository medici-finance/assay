package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stalled_test.go — the stalled detector's fixtures. The property under test is not "does it
// print rows": it is that every NON-listing is a decision the code can defend. A fresh
// push, a fresh author reply and a merged PR are all excluded for DIFFERENT reasons, and
// a signal that could not be read is LABELLED rather than resolved either way (#236).

const stalledAppReviewJSON = `[{"user":{"login":"assay-reviewer-app[bot]"},"state":"CHANGES_REQUESTED",` +
	`"commit_id":"abc123","submitted_at":"2026-08-01T00:00:00Z"}]`

// stalledRepo pins the PR list to ONE repo (via DESKBOARD_GH_PR_REPO) so row counts are
// the property under test, not the size of the compiled-in repo set (which grows).
const stalledRepo = "example-org/tracker"

// ONE App identity, TWO renderings — and the fixtures must use each one where the real
// API emits it, or they test a GitHub that does not exist.
//
//	gh pr list --json author  →  "app/assay-worker-app"     (the gh CLI's rendering)
//	GET /issues/<n>/comments  →  "assay-worker-app[bot]"    (the REST rendering)
//	GET /commits/<sha>        →  "assay-worker-app[bot]"    (.author/.committer login)
//
// Verified live against an App-authored PR. A fixture
// that served the gh form on a REST endpoint would let a comparison that can NEVER match
// in production pass here — which is exactly what the first cut of this file did.
const (
	stalledAuthorGH   = "app/assay-worker-app"    // as the PR list renders the author
	stalledAuthorREST = "assay-worker-app[bot]"   // as every REST payload renders the same App
	stalledOtherREST  = "assay-reviewer-app[bot]" // a DIFFERENT App, in the REST rendering
)

// stalledPRList returns a one-PR list fixture with the given state/draft/branch.
func stalledPRList(state string, draft bool, branch string) string {
	b, err := json.Marshal([]map[string]any{{
		"number": 42, "title": "a stalled draft", "state": state, "isDraft": draft,
		"author":            map[string]string{"login": stalledAuthorGH},
		"headRefOid":        "abc123",
		"headRefName":       branch,
		"mergeStateStatus":  "BLOCKED",
		"statusCheckRollup": []any{},
	}})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// commitJSON renders GET /repos/<r>/commits/<sha> with the head commit attributed to
// `by` (the REST login rendering), or to nobody when `by` is empty — GitHub emits a
// null `author`/`committer` for a commit whose email maps to no account.
func commitJSON(t time.Time, by string) string {
	attribution := `"author":null,"committer":null`
	if by != "" {
		attribution = `"author":{"login":"` + by + `"},"committer":{"login":"` + by + `"}`
	}
	return `{` + attribution + `,"commit":{"committer":{"date":"` + t.UTC().Format(time.RFC3339) + `"}}}`
}

// authorPush is the common case: the head commit was pushed by the PR author.
func authorPush(t time.Time) string { return commitJSON(t, stalledAuthorREST) }

func commentsJSON(login string, at time.Time) string {
	return `[{"user":{"login":"` + login + `"},"created_at":"` + at.UTC().Format(time.RFC3339) + `"}]`
}

// stalledSetup installs the shim and the default "this PR IS stalled" fixture set:
// one open draft, CHANGES_REQUESTED at head, head pushed 10 days ago, no author
// comments, 5 commits behind main. Individual tests override one knob to flip one
// signal — so each case proves that ONE signal is what moved the verdict.
func stalledSetup(t *testing.T) string {
	t.Helper()
	logPath := installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", stalledRepo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", stalledPRList("OPEN", true, "feat/stream-a-04-x"))
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", stalledAppReviewJSON)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", authorPush(time.Now().Add(-10*24*time.Hour)))
	t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_COMPARE_JSON", `{"status":"behind","behind_by":5,"files":[]}`)
	return logPath
}

// runStalled runs the verb in JSON mode and decodes the report. It fails the test on any
// non-zero exit — every fixture here is an exit-0 case by construction, including the
// unassessable one (partial data is labelled, not fatal).
func runStalled(t *testing.T, args ...string) stalledReport {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"stalled", "--json"}, args...)
	if code := run(argv, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(%v) = exit %d, stderr=%s", argv, code, errb.String())
	}
	var rep stalledReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("decoding stalled JSON: %v\nraw: %s", err, out.String())
	}
	return rep
}

// TestStalled_ListsAStalledDraft — the positive fixture. An open draft, blocked at head,
// silent on BOTH channels for longer than the window, is listed with the fields the
// dispatch consumer reads.
func TestStalled_ListsAStalledDraft(t *testing.T) {
	stalledSetup(t)
	rep := runStalled(t)

	if len(rep.Stalled) != 1 {
		t.Fatalf("want exactly 1 stalled row, got %d: %+v", len(rep.Stalled), rep.Stalled)
	}
	if len(rep.Unassessable) != 0 {
		t.Errorf("want no unassessable rows, got %+v", rep.Unassessable)
	}
	r := rep.Stalled[0]
	switch {
	case r.Repo != stalledRepo:
		t.Errorf("repo = %q, want %q", r.Repo, stalledRepo)
	case r.Number != 42:
		t.Errorf("number = %d, want 42", r.Number)
	case r.HeadSHA != "abc123":
		t.Errorf("headSHA = %q, want abc123", r.HeadSHA)
	case r.Disposition != "shepherd":
		t.Errorf("disposition = %q, want shepherd (only 5 commits behind, brief not done)", r.Disposition)
	case r.BehindMain != 5:
		t.Errorf("behindMain = %d, want 5", r.BehindMain)
	case r.LastReviewAt != "2026-08-01T00:00:00Z":
		t.Errorf("lastReviewAt = %q, want the App's decisive review timestamp", r.LastReviewAt)
	case r.LastAuthorCommentAt != "":
		t.Errorf("lastAuthorCommentAt = %q, want empty (the author never commented)", r.LastAuthorCommentAt)
	case r.DaysStalled < 9.5 || r.DaysStalled > 10.5:
		t.Errorf("daysStalled = %v, want ~10", r.DaysStalled)
	}
	if rep.MinAgeHours != stalledMinAgeDefault {
		t.Errorf("minAgeHours = %d, want the %d-hour default", rep.MinAgeHours, stalledMinAgeDefault)
	}
}

// TestStalled_FreshPushNotListed — a draft the author pushed to inside the window is
// working, not abandoned. Same CHANGES_REQUESTED verdict as the positive fixture: the
// push clock is the only difference.
func TestStalled_FreshPushNotListed(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", authorPush(time.Now().Add(-2*time.Hour)))

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("a draft pushed 2h ago must not be listed; got %+v", rep.Stalled)
	}
	if len(rep.Unassessable) != 0 {
		t.Errorf("nothing was unreadable; got unassessable %+v", rep.Unassessable)
	}
}

// TestStalled_FreshAuthorCommentNotListed — the other channel. The head is 10 days old,
// but the author replied on the thread an hour ago: a responder is present, so the PR is
// not abandoned. This is the case a push-only clock would get wrong.
//
// The reply arrives in the REST rendering (`<slug>[bot]`) while the PR list named the
// author in the gh rendering (`app/<slug>`) — the ONLY shape production ever presents.
// The fleet is largely App-authored, so if this comparison does not cross the two
// renderings, the responder clock is dead for nearly every PR the verb exists to triage.
func TestStalled_FreshAuthorCommentNotListed(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON",
		commentsJSON(stalledAuthorREST, time.Now().Add(-1*time.Hour)))

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("a draft whose App author replied 1h ago must not be listed; got %+v", rep.Stalled)
	}
}

// TestStalled_OtherPartyCommentDoesNotReset — the counterpart: a comment by SOMEONE ELSE
// is not responder activity. If it reset the clock, the reviewer nagging an abandoned PR
// would hide it from the very list meant to find it.
//
// Both a different App and a human are exercised. The App case alone is not enough: while
// the author filter compared incompatible renderings it held for the WRONG reason (no REST
// login matched ANYTHING), so it would have stayed green with the filter deleted outright.
func TestStalled_OtherPartyCommentDoesNotReset(t *testing.T) {
	for _, other := range []string{stalledOtherREST, "ada"} {
		t.Run(other, func(t *testing.T) {
			stalledSetup(t)
			t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON",
				commentsJSON(other, time.Now().Add(-1*time.Hour)))

			rep := runStalled(t)
			if len(rep.Stalled) != 1 {
				t.Fatalf("a %s comment is not author activity; want the PR still listed, got %+v", other, rep.Stalled)
			}
			if got := rep.Stalled[0].LastAuthorCommentAt; got != "" {
				t.Errorf("lastAuthorCommentAt = %q, want empty — that comment was not the author's", got)
			}
		})
	}
}

// TestStalled_AuthorIdentityCrossesRenderings — Finding 1, pinned end to end and in BOTH
// directions at once. One run, two comments an hour old: one by the PR author (REST
// rendering) and one by a different App. A detector that cannot cross the renderings sees
// neither as the author and lists the PR; one that folds too eagerly — matching any bot,
// or matching on slug alone — sees the reviewer as the author and would also suppress the
// row below. Only a comparison that resolves BOTH correctly produces no rows here and one
// row in the sibling test above.
func TestStalled_AuthorIdentityCrossesRenderings(t *testing.T) {
	stalledSetup(t)
	now := time.Now()
	t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON",
		`[{"user":{"login":"`+stalledOtherREST+`"},"created_at":"`+now.Add(-3*time.Hour).UTC().Format(time.RFC3339)+`"},`+
			`{"user":{"login":"`+stalledAuthorREST+`"},"created_at":"`+now.Add(-1*time.Hour).UTC().Format(time.RFC3339)+`"}]`)

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Fatalf("the App author replied 1h ago (REST rendering of the gh-rendered PR author); "+
			"it must not be listed as stalled. got %+v", rep.Stalled)
	}
	if len(rep.Unassessable) != 0 {
		t.Errorf("nothing was unreadable; got unassessable %+v", rep.Unassessable)
	}
}

// TestStalled_AuthorCommentIsReportedForAppAuthors — the reporting half of Finding 1. An
// omitted `lastAuthorCommentAt` reads to a dispatch consumer as the established fact "the
// author never replied". While the renderings could not be compared, that field was
// ALWAYS omitted for App-authored PRs — a claim nothing had checked.
func TestStalled_AuthorCommentIsReportedForAppAuthors(t *testing.T) {
	stalledSetup(t)
	replied := time.Now().Add(-5 * 24 * time.Hour).UTC().Truncate(time.Second)
	t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON", commentsJSON(stalledAuthorREST, replied))

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("silent on both channels for 5 days: want 1 row, got %+v", rep.Stalled)
	}
	r := rep.Stalled[0]
	if got, want := r.LastAuthorCommentAt, replied.Format(time.RFC3339); got != want {
		t.Errorf("lastAuthorCommentAt = %q, want %q — the App author's reply was found but not reported", got, want)
	}
	// The comment is NEWER than the 10-day-old push, so it governs the stall age.
	if r.DaysStalled < 4.5 || r.DaysStalled > 5.5 {
		t.Errorf("daysStalled = %v, want ~5 — measured from the author's reply, not the older push", r.DaysStalled)
	}
}

// TestStalled_StallWindowBoundaryIsExact — the tool's PRIMARY threshold, pinned at the
// tick. A wall-clock fixture can never land exactly on the boundary, so `<= minAge` and
// `< minAge` are indistinguishable through the verb; the predicate is tested directly.
// The rule: a stall requires STRICTLY MORE than minAge of silence, so activity exactly
// minAge old is still active. Both one-character mutations go red here.
func TestStalled_StallWindowBoundaryIsExact(t *testing.T) {
	const minAge = 48 * time.Hour
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		since time.Duration // how long ago the last activity was
		want  bool
	}{
		{"one nanosecond inside the window", minAge - time.Nanosecond, false},
		{"exactly at the window — a tie is ACTIVE, not stalled", minAge, false},
		{"one nanosecond past the window", minAge + time.Nanosecond, true},
		{"an hour past the window", minAge + time.Hour, true},
		{"activity in the future (clock skew) is never stalled", -time.Hour, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isStalled(now, now.Add(-c.since), minAge); got != c.want {
				t.Errorf("isStalled(silent for %v, window %v) = %v, want %v", c.since, minAge, got, c.want)
			}
		})
	}
}

// TestStalled_ForeignMergePushDoesNotResetTheClock — the one FALSE-QUIET path in this
// verb. `prsync --push` merges origin/main and pushes; the desk's loop tells workers to
// merge main on conflict. If any such push reset the clock, a draft its author abandoned
// months ago would read as freshly pushed forever and never reach the list this verb
// exists to build. Only a push attributable to the AUTHOR counts as author activity.
func TestStalled_ForeignMergePushDoesNotResetTheClock(t *testing.T) {
	stalledSetup(t)
	// Somebody else merged main onto the branch an hour ago. The author has not been
	// seen since the reviewer's decisive review.
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", commitJSON(time.Now().Add(-1*time.Hour), stalledOtherREST))

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("a merge-currency push by another party must not hide an abandoned draft; got %+v", rep.Stalled)
	}
	r := rep.Stalled[0]
	if r.PushIsAuthors {
		t.Error("pushIsAuthors = true, but the head commit is attributed to another account")
	}
	if r.LastPushBy != stalledOtherREST {
		t.Errorf("lastPushBy = %q, want %q — the row must name who actually pushed", r.LastPushBy, stalledOtherREST)
	}
	// With no author activity at all, the clock runs from the decisive review, NOT from
	// the stranger's merge commit (which would report ~0 days) and NOT from the zero
	// time (which would report a stall measured in millennia).
	reviewedAt, err := time.Parse(time.RFC3339, "2026-08-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Since(reviewedAt).Hours() / 24
	if r.DaysStalled < want-0.5 || r.DaysStalled > want+0.5 {
		t.Errorf("daysStalled = %v, want ~%v (measured from the decisive review)", r.DaysStalled, want)
	}
}

// TestStalled_AuthorsOwnMergePushIsActivity — the counterpart. When the AUTHOR is the one
// keeping the branch current, that is a live session, and suppressing it would turn the
// fix above into a false-alarm generator.
func TestStalled_AuthorsOwnMergePushIsActivity(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", authorPush(time.Now().Add(-1*time.Hour)))

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("the AUTHOR pushed 1h ago; that is activity, not abandonment. got %+v", rep.Stalled)
	}
}

// TestStalled_UnattributedPushStaysOnTheAuthorsClock — missing data must not manufacture
// a stall. GitHub attributes a commit to no account when its email maps to no user; that
// is UNKNOWN, not "somebody else", and reading it as a foreign push would invent an
// abandonment the tool cannot evidence (#236, in the direction that costs a false alarm).
func TestStalled_UnattributedPushStaysOnTheAuthorsClock(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", commitJSON(time.Now().Add(-1*time.Hour), ""))

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("an unattributed push is unknown, not foreign; it must not create a stall. got %+v", rep.Stalled)
	}
}

// TestStalled_NotBlockingAtHeadNotListed — CHANGES_REQUESTED at an OLD head means the
// author DID push past the finding; the verdict is stale, not blocking, and the PR is
// the reviewer's move, not the worker's. Guards the #268 reduction's at-head semantics
// through the verb.
func TestStalled_NotBlockingAtHeadNotListed(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"assay-reviewer-app[bot]"},"state":"CHANGES_REQUESTED",`+
			`"commit_id":"OLDHEAD","submitted_at":"2026-08-01T00:00:00Z"}]`)

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("a verdict at an old head is stale, not blocking; got %+v", rep.Stalled)
	}
}

// TestStalled_NonDraftNotListed — the board's subject is the DRAFT backlog; a ready PR is
// the desk's/human's move and has its own verbs.
func TestStalled_NonDraftNotListed(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", stalledPRList("OPEN", false, "feat/stream-a-04-x"))

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("a non-draft must not be listed; got %+v", rep.Stalled)
	}
}

// TestStalled_MergedNeverAppears — #247. A MERGED PR is finished; its head compared
// against main reads exactly like a live branch, so the state must be read BEFORE any
// comparison. The same fixture that lists when OPEN must vanish when MERGED, and the
// enumeration itself must ask for --state open.
func TestStalled_MergedNeverAppears(t *testing.T) {
	for _, state := range []string{"MERGED", "CLOSED"} {
		t.Run(state, func(t *testing.T) {
			stalledSetup(t)
			t.Setenv("DESKBOARD_GH_PRLIST_JSON", stalledPRList(state, true, "feat/stream-a-04-x"))

			rep := runStalled(t)
			if len(rep.Stalled) != 0 {
				t.Errorf("a %s PR must never appear on the dispatch list; got %+v", state, rep.Stalled)
			}
			if len(rep.Unassessable) != 0 {
				t.Errorf("a %s PR is a decided exclusion, not an unassessable one; got %+v", state, rep.Unassessable)
			}
		})
	}
}

// TestStalled_EnumeratesOpenPRsOnly — the belt to the #247 braces: the merged case above
// only proves the guard, this proves the enumeration never asks for merged PRs at all.
func TestStalled_EnumeratesOpenPRsOnly(t *testing.T) {
	logPath := stalledSetup(t)
	runStalled(t)

	sawOpenList := false
	for _, fields := range readInvocations(t, logPath) {
		line := strings.Join(fields, " ")
		// The open-PR enumeration is a `gh api graphql` read. It must ask ONLY for
		// open PRs — `pullRequests(states:OPEN …)` — never a states-less or merged query.
		if !strings.Contains(line, "pullRequests(") {
			continue
		}
		if !strings.Contains(line, "pullRequests(states:OPEN") {
			t.Errorf("open-PR graphql without states:OPEN: %s", line)
			continue
		}
		sawOpenList = true
	}
	if !sawOpenList {
		t.Error("no `pullRequests(states:OPEN …)` graphql invocation recorded — the open-only enumeration is unproven")
	}
}

// TestStalled_UnassessableRowOnAPIError — the #236 shape, inverted. One PR's review read
// fails; that PR becomes a LABELLED unassessable row (naming which signal failed), the
// run still exits 0, and — critically — it is NOT reported as clean.
func TestStalled_UnassessableRowOnAPIError(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_FAIL_PATH", "/reviews")

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("the verdict was unreadable — nothing may be asserted stalled; got %+v", rep.Stalled)
	}
	if len(rep.Unassessable) != 1 {
		t.Fatalf("want exactly 1 unassessable row, got %d: %+v", len(rep.Unassessable), rep.Unassessable)
	}
	u := rep.Unassessable[0]
	if u.Number != 42 || u.Repo != stalledRepo {
		t.Errorf("unassessable row = %s#%d, want %s#42", u.Repo, u.Number, stalledRepo)
	}
	if !strings.Contains(strings.ToLower(u.Reason), "verdict") {
		t.Errorf("reason %q does not name the signal that could not be read", u.Reason)
	}
}

// TestStalled_UnassessableOnEachSignal — every stall signal has its own labelled failure.
// A verb that only labelled the FIRST read it makes would report the others as clean.
func TestStalled_UnassessableOnEachSignal(t *testing.T) {
	cases := []struct {
		failPath string
		wantWord string
	}{
		{"/reviews", "verdict"},
		{"/commits/", "push date"},
		{"/comments", "comments"},
	}
	for _, c := range cases {
		t.Run(c.failPath, func(t *testing.T) {
			stalledSetup(t)
			t.Setenv("DESKBOARD_GH_FAIL_PATH", c.failPath)

			rep := runStalled(t)
			if len(rep.Stalled) != 0 {
				t.Errorf("want no stalled rows when %s is unreadable; got %+v", c.failPath, rep.Stalled)
			}
			if len(rep.Unassessable) != 1 {
				t.Fatalf("want 1 unassessable row for %s, got %+v", c.failPath, rep.Unassessable)
			}
			if !strings.Contains(rep.Unassessable[0].Reason, c.wantWord) {
				t.Errorf("reason %q does not name %q", rep.Unassessable[0].Reason, c.wantWord)
			}
		})
	}
}

// TestStalled_PartialDataKeepsTheBoard — Task 3's exact wording: an API error on
// ONE PR yields an unassessable row AND exit 0 for the rest. #42's comment read fails;
// #43, whose reads all succeed, is still listed.
func TestStalled_PartialDataKeepsTheBoard(t *testing.T) {
	stalledSetup(t)
	// Two PRs. The shim fails ONLY #42's comment read (the URL carries the PR number).
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"broken","state":"OPEN","isDraft":true,"author":{"login":"app/assay-worker-app"},`+
			`"headRefOid":"abc123","headRefName":"feat/a","mergeStateStatus":"BLOCKED","statusCheckRollup":[]},`+
			`{"number":43,"title":"readable","state":"OPEN","isDraft":true,"author":{"login":"app/assay-worker-app"},`+
			`"headRefOid":"abc123","headRefName":"feat/b","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_FAIL_PATH", "issues/42/comments")

	rep := runStalled(t)
	if len(rep.Unassessable) != 1 || rep.Unassessable[0].Number != 42 {
		t.Fatalf("want #42 labelled unassessable, got %+v", rep.Unassessable)
	}
	if len(rep.Stalled) != 1 || rep.Stalled[0].Number != 43 {
		t.Fatalf("want #43 still listed alongside the labelled failure, got %+v", rep.Stalled)
	}
}

// TestStalled_UnstatedPRStateIsLabelled — "not stated" must never read as "open" (#247's
// root shape). A PR list that omits `state` produces a labelled row, not a silent drop
// and not an assumed-open listing.
func TestStalled_UnstatedPRStateIsLabelled(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"stateless","isDraft":true,"author":{"login":"app/assay-worker-app"},`+
			`"headRefOid":"abc123","headRefName":"feat/a","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Errorf("a PR with no stated state must not be listed as stalled; got %+v", rep.Stalled)
	}
	if len(rep.Unassessable) != 1 {
		t.Fatalf("want the stateless PR labelled unassessable, got %+v", rep.Unassessable)
	}
	if !strings.Contains(rep.Unassessable[0].Reason, "state not stated") {
		t.Errorf("reason %q does not name the missing state", rep.Unassessable[0].Reason)
	}
}

// TestStalled_RepoListFailureFailsClosed — the granularity boundary. A per-PR read
// failure is labelled; a whole-repo LIST failure is fatal (exit 6), because "no stalled
// drafts in that repo" would be a claim nothing verified.
func TestStalled_RepoListFailureFailsClosed(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_FAIL_REPO", stalledRepo)

	var out, errb bytes.Buffer
	if code := run([]string{"stalled", "--json"}, &out, &errb); code != deskkit.ExitUnverifiable {
		t.Fatalf("run = exit %d, want %d (unverifiable); stdout=%s", code, deskkit.ExitUnverifiable, out.String())
	}
}

// TestStalled_MinAgeHoursFlag — the threshold is a flag, per the brief. A 10-day-old head
// is inside a 30-day window (not stalled) and outside a 1-hour one (stalled).
func TestStalled_MinAgeHoursFlag(t *testing.T) {
	stalledSetup(t)
	if rep := runStalled(t, "--min-age-hours", "720"); len(rep.Stalled) != 0 {
		t.Errorf("10 days is inside a 720h window; want no rows, got %+v", rep.Stalled)
	}
	if rep := runStalled(t, "--min-age-hours", "1"); len(rep.Stalled) != 1 {
		t.Errorf("10 days is outside a 1h window; want 1 row, got %+v", rep.Stalled)
	}
}

// TestStalled_MinAgeHoursRefusesGarbage — a threshold this tool cannot parse is
// a refusal (exit 5), never a silent fall back to the default. A board run with the
// operator's intended window silently replaced is a board they did not ask for.
func TestStalled_MinAgeHoursRefusesGarbage(t *testing.T) {
	stalledSetup(t)
	for _, bad := range []string{"abc", "0", "-5", ""} {
		var out, errb bytes.Buffer
		code := run([]string{"stalled", "--json", "--min-age-hours", bad}, &out, &errb)
		if code != deskkit.ExitRefused {
			t.Errorf("--min-age-hours %q = exit %d, want %d (refused)", bad, code, deskkit.ExitRefused)
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"stalled", "--min-age-hours"}, &out, &errb); code != deskkit.ExitRefused {
		t.Errorf("a dangling --min-age-hours = exit %d, want %d (refused)", code, deskkit.ExitRefused)
	}
}

// TestStalled_CloseCandidateBehindMain — the advisory hint fires on POSITIVE evidence
// only, and the threshold is a boundary, not a vibe: 200 is not "> 200".
func TestStalled_CloseCandidateBehindMain(t *testing.T) {
	cases := []struct {
		behind int
		want   string
	}{
		{5, "shepherd"},
		{behindMainCloseThreshold, "shepherd"},
		{behindMainCloseThreshold + 1, "close-candidate"},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.behind), func(t *testing.T) {
			stalledSetup(t)
			t.Setenv("DESKBOARD_GH_COMPARE_JSON",
				`{"status":"behind","behind_by":`+strconv.Itoa(c.behind)+`,"files":[]}`)

			rep := runStalled(t)
			if len(rep.Stalled) != 1 {
				t.Fatalf("behind=%d: want 1 row, got %+v", c.behind, rep.Stalled)
			}
			if got := rep.Stalled[0].Disposition; got != c.want {
				t.Errorf("behind=%d: disposition = %q, want %q", c.behind, got, c.want)
			}
		})
	}
}

// TestStalled_UnreadableCompareStaysShepherd — the advisory layer degrades to the SAFE
// side. A compare that could not be read must not become "0 behind → current", and it
// must never produce a close-candidate the tool cannot justify.
func TestStalled_UnreadableCompareStaysShepherd(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_FAIL_PATH", "/compare/")

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("an unreadable advisory hint must not drop the row; got %+v", rep.Stalled)
	}
	r := rep.Stalled[0]
	if r.Disposition != "shepherd" {
		t.Errorf("disposition = %q, want shepherd — the compare was never read", r.Disposition)
	}
	if r.BehindMain != -1 {
		t.Errorf("behindMain = %d, want -1 (not assessed) — 0 would read as 'current'", r.BehindMain)
	}
}

// TestStalled_MalformedCompareIsNotAssessed — the OTHER compare failure mode. The
// unreadable-compare test above exercises a TRANSPORT failure (the call fails); this one
// exercises a 200 whose body this code did not understand — no `status` field. Without
// the guard, the body's zero `behind_by` reads as "0 commits behind, perfectly current",
// a number nothing parsed. The subtest with behind_by past the threshold is the one that
// matters: it proves a malformed body cannot FABRICATE a close-candidate.
func TestStalled_MalformedCompareIsNotAssessed(t *testing.T) {
	bodies := map[string]string{
		"no status field":                  `{"behind_by":250}`,
		"empty status field":               `{"status":"","behind_by":250}`,
		"neither status nor behind_by":     `{"files":[]}`,
		"status absent, behind_by is zero": `{"behind_by":0}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			stalledSetup(t)
			t.Setenv("DESKBOARD_GH_COMPARE_JSON", body)

			rep := runStalled(t)
			if len(rep.Stalled) != 1 {
				t.Fatalf("an unparseable advisory hint must not drop the row; got %+v", rep.Stalled)
			}
			r := rep.Stalled[0]
			if r.BehindMain != -1 {
				t.Errorf("behindMain = %d, want -1 (not assessed) — no status field means nothing parsed", r.BehindMain)
			}
			if r.Disposition != "shepherd" {
				t.Errorf("disposition = %q, want shepherd — a body nothing understood cannot justify close-candidate", r.Disposition)
			}
		})
	}
}

// TestStalled_CloseCandidateOnDoneBrief — the second close-candidate trigger: the branch
// claims a brief that statusgen reports as finished, so the draft is orphaned work.
func TestStalled_CloseCandidateOnDoneBrief(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", stalledPRList("OPEN", true, "feat/stream-a-04-stalewatch"))
	t.Setenv("DESKBOARD_GATE_SCORES_JSON",
		`[{"brief":"stream-a/04","status":"done","repo":"`+stalledRepo+`"}]`)

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("want 1 row, got %+v", rep.Stalled)
	}
	if got := rep.Stalled[0].Disposition; got != "close-candidate" {
		t.Errorf("disposition = %q, want close-candidate (owning brief done)", got)
	}
}

// TestStalled_BriefStatusIsRepoScoped — brief IDs are unique only within a repo root. A
// brief finished in ANOTHER repo must not mark this repo's live draft closeable.
func TestStalled_BriefStatusIsRepoScoped(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", stalledPRList("OPEN", true, "feat/stream-a-04-stalewatch"))
	t.Setenv("DESKBOARD_GATE_SCORES_JSON",
		`[{"brief":"stream-a/04","status":"done","repo":"medici-finance/assay"}]`)

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("want 1 row, got %+v", rep.Stalled)
	}
	if got := rep.Stalled[0].Disposition; got != "shepherd" {
		t.Errorf("disposition = %q, want shepherd — the done brief belongs to another repo", got)
	}
}

// TestStalled_GateScoresFailureIsNotFatal — the advisory layer must not couple
// board availability to a statusgen release being installed. The detector's job survives
// a missing hint; it does not survive a hint that takes the whole board down with it.
func TestStalled_GateScoresFailureIsNotFatal(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GATE_SCORES_JSON", "")
	t.Setenv("DESKBOARD_GATE_SCORES_FAIL", "statusgen not installed")

	rep := runStalled(t)
	if len(rep.Stalled) != 1 {
		t.Fatalf("a missing statusgen must not empty the board; got %+v", rep.Stalled)
	}
	if got := rep.Stalled[0].Disposition; got != "shepherd" {
		t.Errorf("disposition = %q, want the shepherd default when the hint is unavailable", got)
	}
}

// TestStalled_TruncatedEnumerationIsVisibleToBothAudiences — silence must be
// distinguishable from blindness in the shape the CONSUMER reads. fetchOpenPRs already
// WARNs at the listing cap, but on stderr, where a loop piping `--json` cannot see it: a
// capped enumeration would hand that loop a board that looks complete. The measured
// population is 80 open PRs across two repos, so the cap is not far off.
func TestStalled_TruncatedEnumerationIsVisibleToBothAudiences(t *testing.T) {
	stalledSetup(t)
	// prListLimit PRs — exactly the cap, which is what gh returns when there are more.
	// None of them can be stalled (fresh push), so the board is otherwise clean: the
	// truncation flag is the ONLY thing distinguishing "nothing stalled" from "we did
	// not look at everything".
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", authorPush(time.Now()))
	var prs []string
	for i := 1; i <= prListLimit; i++ {
		prs = append(prs, `{"number":`+strconv.Itoa(i)+`,"title":"t","state":"OPEN","isDraft":true,`+
			`"author":{"login":"`+stalledAuthorGH+`"},"headRefOid":"abc123","headRefName":"feat/a",`+
			`"mergeStateStatus":"BLOCKED","statusCheckRollup":[]}`)
	}
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", "["+strings.Join(prs, ",")+"]")

	rep := runStalled(t)
	if len(rep.Stalled) != 0 {
		t.Fatalf("every PR was pushed just now; want a clean board, got %+v", rep.Stalled)
	}
	if len(rep.Truncated) != 1 || rep.Truncated[0] != stalledRepo {
		t.Fatalf("truncated = %+v, want [%s] — a capped enumeration must say so in the JSON", rep.Truncated, stalledRepo)
	}

	var tbl, errb bytes.Buffer
	if code := run([]string{"stalled"}, &tbl, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(stalled) = exit %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(tbl.String(), "TRUNCATED") {
		t.Errorf("the human table must state the truncation too:\n%s", tbl.String())
	}
}

// TestStalled_CompleteEnumerationSaysSoPositively — the counterpart. An uncapped run
// carries an EMPTY truncated array, not a missing field: a consumer must be able to tell
// "we enumerated everything" from "this build did not report".
func TestStalled_CompleteEnumerationSaysSoPositively(t *testing.T) {
	stalledSetup(t)
	var js, errb bytes.Buffer
	if code := run([]string{"stalled", "--json"}, &js, &errb); code != deskkit.ExitOK {
		t.Fatalf("run = exit %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(js.String(), `"truncated": []`) {
		t.Errorf("a complete enumeration must carry an empty array, not null:\n%s", js.String())
	}
}

// TestStalled_DefaultsToTheHumanTable — the brief's output contract: `stalled` is a
// discovery verb, so its bare invocation prints the table and --json selects the machine
// shape the dispatch consumer reads.
func TestStalled_DefaultsToTheHumanTable(t *testing.T) {
	stalledSetup(t)

	var tbl, errb bytes.Buffer
	if code := run([]string{"stalled"}, &tbl, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(stalled) = exit %d, stderr=%s", code, errb.String())
	}
	if json.Valid(bytes.TrimSpace(tbl.Bytes())) {
		t.Errorf("bare `stalled` emitted JSON; the default is the human table:\n%s", tbl.String())
	}
	for _, want := range []string{"DISPOSITION", "shepherd", "#42"} {
		if !strings.Contains(tbl.String(), want) {
			t.Errorf("table output missing %q:\n%s", want, tbl.String())
		}
	}

	var js, errb2 bytes.Buffer
	if code := run([]string{"stalled", "--json"}, &js, &errb2); code != deskkit.ExitOK {
		t.Fatalf("run(stalled --json) = exit %d, stderr=%s", code, errb2.String())
	}
	if !json.Valid(bytes.TrimSpace(js.Bytes())) {
		t.Errorf("`stalled --json` did not emit JSON:\n%s", js.String())
	}

	// Asking for both is a contradiction; emitting one of them silently would hand the
	// operator a shape they did not ask for.
	var both, errb3 bytes.Buffer
	if code := run([]string{"stalled", "--table", "--json"}, &both, &errb3); code != deskkit.ExitRefused {
		t.Errorf("`stalled --table --json` = exit %d, want %d (refused)", code, deskkit.ExitRefused)
	}
}

// TestStalled_EmptyBoardSaysSo — silence must never be the report. A clean board prints a
// positive statement, and the JSON carries empty ARRAYS rather than nulls, so a consumer
// cannot confuse "nothing stalled" with "field absent".
func TestStalled_EmptyBoardSaysSo(t *testing.T) {
	stalledSetup(t)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", authorPush(time.Now()))

	var tbl, errb bytes.Buffer
	if code := run([]string{"stalled"}, &tbl, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(stalled) = exit %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(tbl.String(), "no stalled drafts") {
		t.Errorf("an empty board must say so explicitly:\n%s", tbl.String())
	}

	var js, errb2 bytes.Buffer
	run([]string{"stalled", "--json"}, &js, &errb2)
	if !strings.Contains(js.String(), `"stalled": []`) || !strings.Contains(js.String(), `"unassessable": []`) {
		t.Errorf("empty board JSON must carry empty arrays, not null:\n%s", js.String())
	}
}
