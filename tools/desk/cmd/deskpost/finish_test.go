package main

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// finishClaimKey is the correctness lane's review-dispatch claim key of exampleRepo#1 —
// "<short label>--pr-<N>" — and finishSecurityClaimKey the security lane's.
const (
	finishClaimKey         = "tracker--pr-1"
	finishSecurityClaimKey = finishClaimKey + "--security"
)

// finishKeyFor is the claim key a finish of that lane must be given.
func finishKeyFor(lane string) string {
	if lane == "security-review" {
		return finishSecurityClaimKey
	}
	return finishClaimKey
}

func finishRefPath(key string) string { return "/git/refs/dispatch/" + key }

func finishArgs(lane, pr, verdict, head, bodyFile, claim string) []string {
	args := []string{"finish", lane, exampleRepo, pr, "--verdict", verdict, "--head", head, "--body-file", bodyFile}
	if claim != "" {
		args = append(args, "--claim", claim)
	}
	return args
}

// finishHarness is setupFake plus a captured stdout and a forge that answers the claim
// delete. deleteStatus is the status served for the claim's DELETE (0 → the fake's default
// 404, exactly what a forge answers for a ref that is already gone).
type finishHarness struct {
	f            *fakeGH
	out, errOut  *bytes.Buffer
	deleteStatus int
	claimKey     string // the claim whose DELETE this forge answers; the correctness key unless set
}

func setupFinish(t *testing.T) *finishHarness {
	t.Helper()
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	h := &finishHarness{f: f, errOut: errBuf, out: &bytes.Buffer{}, deleteStatus: http.StatusNoContent, claimKey: finishClaimKey}
	saved := stdout
	stdout = h.out
	t.Cleanup(func() { stdout = saved })
	f.intercept = func(method, path string) (int, bool) {
		if method == http.MethodDelete && strings.HasSuffix(path, finishRefPath(h.claimKey)) && h.deleteStatus != 0 {
			return h.deleteStatus, true
		}
		return 0, false
	}
	return h
}

// order returns the position of the first hit matching method+suffix, or -1.
func (h *finishHarness) order(method, suffix string) int {
	h.f.mu.Lock()
	defer h.f.mu.Unlock()
	for i, hit := range h.f.hits {
		if strings.HasPrefix(hit, method+" ") && strings.HasSuffix(hit, suffix) {
			return i
		}
	}
	return -1
}

// deletes counts DELETE requests for ANY ref: a finish must delete its own claim and nothing
// else, so a test that expects none must see none for any key.
func (h *finishHarness) deletes() int {
	h.f.mu.Lock()
	defer h.f.mu.Unlock()
	n := 0
	for _, hit := range h.f.hits {
		if strings.HasPrefix(hit, http.MethodDelete+" ") {
			n++
		}
	}
	return n
}

// releaseRows returns the audit rows the release step wrote.
func releaseRows(t *testing.T) []deskkit.Entry {
	t.Helper()
	var rows []deskkit.Entry
	for _, e := range auditEntries(t) {
		if e.Verb == finishReleaseVerb {
			rows = append(rows, e)
		}
	}
	return rows
}

// postedShape is body as the writer posts it: with its on-behalf-of line appended.
func postedShape(t *testing.T, body string) string {
	t.Helper()
	b, err := deskkit.AppendOnBehalfOf([]byte(body), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	if string(b) == body {
		t.Fatal("fixture defect: the writer appended no on-behalf-of line, so a posted-shape fixture would prove nothing")
	}
	return string(b)
}

// forgetThisSession removes this HOME's audit log — what a re-dispatched reviewer starts
// with — so that only the forge's own state can stop a second post.
func forgetThisSession(t *testing.T) {
	t.Helper()
	p := filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl")
	if err := os.Remove(p); err != nil {
		t.Fatalf("remove the audit log: %v", err)
	}
	if n := len(auditEntries(t)); n != 0 {
		t.Fatalf("the audit log still holds %d row(s)", n)
	}
}

func (h *finishHarness) resultLines() []string {
	s := strings.TrimRight(h.out.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// TestFinishHappyPath: both lanes post, confirm and release IN THAT ORDER and print exactly
// one stdout line carrying the review, its state, the head and the claim outcome.
func TestFinishHappyPath(t *testing.T) {
	for _, tc := range []struct {
		name, lane, verdict, body, wantState, wantVerb string
	}{
		{"correctness approve", "review", "approve", okReviewBody, "APPROVED", "review:correctness:approve"},
		{"security pass", "security-review", "pass", okSecurityBody, "COMMENTED", "review:security:pass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			key := finishKeyFor(tc.lane)
			h.claimKey = key
			bf := writeBody(t, "v.md", tc.body)

			if code := run(finishArgs(tc.lane, "1", tc.verdict, testHead, bf, key)); code != 0 {
				t.Fatalf("finish exit = %d, want 0\nstderr: %s", code, h.errOut.String())
			}
			if h.f.postedReview != 1 {
				t.Fatalf("postedReview = %d, want 1", h.f.postedReview)
			}
			post := h.order(http.MethodPost, "/pulls/1/reviews")
			del := h.order(http.MethodDelete, finishRefPath(key))
			if post < 0 || del < 0 || post > del {
				t.Fatalf("want the review POST before the claim DELETE; post at %d, delete at %d: %v", post, del, h.f.hits)
			}
			// The confirm is a reviews read AFTER the post.
			lastRead := -1
			for i, hit := range h.f.hits {
				if hit == "GET /repos/"+exampleRepo+"/pulls/1/reviews" {
					lastRead = i
				}
			}
			if lastRead < post || lastRead > del {
				t.Fatalf("want a reviews read between the POST (%d) and the DELETE (%d), last read at %d: %v", post, del, lastRead, h.f.hits)
			}

			lines := h.resultLines()
			if len(lines) != 1 {
				t.Fatalf("stdout must carry exactly ONE line, got %d: %q", len(lines), h.out.String())
			}
			want := "deskpost finish: review=" + exampleRepo + "#1/review-1001 state=" + tc.wantState + " head=" + testHead + " claim=released"
			if lines[0] != want {
				t.Fatalf("result line\n got %q\nwant %q", lines[0], want)
			}

			// Two audit rows. The first is the post step's: same verb key the plain verb
			// records. The second is the release step's own.
			entries := auditEntries(t)
			if len(entries) != 2 {
				t.Fatalf("audit rows = %d, want exactly 2: %+v", len(entries), entries)
			}
			if e := entries[0]; e.Result != deskkit.ResultOK || e.Verb != tc.wantVerb || e.HeadSHA == nil || *e.HeadSHA != testHead {
				t.Fatalf("post row = %+v", e)
			}
			rel := entries[1]
			if rel.Verb != finishReleaseVerb || rel.Result != deskkit.ResultOK || rel.Repo != exampleRepo ||
				rel.PR == nil || *rel.PR != 1 || rel.HeadSHA == nil || *rel.HeadSHA != testHead {
				t.Fatalf("release row = %+v", rel)
			}
			if !strings.Contains(rel.Detail, "claim "+key+" released") {
				t.Fatalf("release row detail does not name the claim and the outcome: %q", rel.Detail)
			}
			if rel.BodyDigest != "" {
				t.Fatalf("release row carries a body digest (%q); a check that reads the log for a posted verdict could match it", rel.BodyDigest)
			}
			if rel.ArgsDigest != entries[0].ArgsDigest {
				t.Fatalf("the two rows of one invocation carry different argument digests: %q, %q", entries[0].ArgsDigest, rel.ArgsDigest)
			}
		})
	}
}

// TestFinishShowsTheForgeLinkWhenListed: the result line carries the forge's own link to the
// review when the listing has one, and never a link this tool built.
func TestFinishShowsTheForgeLinkWhenListed(t *testing.T) {
	h := setupFinish(t)
	bf := writeBody(t, "v.md", okReviewBody)
	postBody, err := deskkit.AppendOnBehalfOf([]byte(okReviewBody), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	listed := appReviewAt("APPROVED", testHead, string(postBody))
	listed.ID = 4242
	listed.HTMLURL = "https://forge.example/example-org/tracker/pull/1#review-4242"
	h.f.reviewsAfterFirstRead = []reviewInfo{listed}

	if code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)); code != 0 {
		t.Fatalf("finish exit = %d, want 0\nstderr: %s", code, h.errOut.String())
	}
	if got := h.out.String(); !strings.Contains(got, "review="+listed.HTMLURL+" state=APPROVED") {
		t.Fatalf("result line does not carry the listed link: %q", got)
	}
}

// TestFinishRefusesWhereTheVerdictVerbRefuses: for every refusal, `finish <verb>` and the
// plain verb exit with the SAME code and write the SAME audit result and detail, and finish
// neither confirms nor releases. This is the property "finish is the verb, not a copy of it".
func TestFinishRefusesWhereTheVerdictVerbRefuses(t *testing.T) {
	for _, tc := range []struct {
		name          string
		lane, verdict string
		body          string
		setup         func(f *fakeGH)
		head          string
		wantCode      int
	}{
		{name: "head moved", lane: "review", verdict: "approve", body: okReviewBody,
			setup: func(f *fakeGH) { f.pullHeads = []string{testNewHead} }, head: testOldHead, wantCode: deskkit.ExitRefused},
		{name: "body has no verdict", lane: "review", verdict: "approve", body: "looks fine to me\n",
			head: testHead, wantCode: deskkit.ExitRefused},
		{name: "security marker on the correctness verb", lane: "review", verdict: "approve", body: okSecurityBody,
			head: testHead, wantCode: deskkit.ExitRefused},
		{name: "correctness body on the security verb", lane: "security-review", verdict: "pass", body: okReviewBody,
			head: testHead, wantCode: deskkit.ExitRefused},
		{name: "bad verdict value", lane: "review", verdict: "pass", body: okReviewBody,
			head: testHead, wantCode: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type outcome struct {
				code   int
				rows   int
				result string
				detail string
			}
			observe := func(args func(bf string) []string) (outcome, *finishHarness) {
				h := setupFinish(t)
				if tc.setup != nil {
					tc.setup(h.f)
				}
				bf := writeBody(t, "v.md", tc.body)
				var o outcome
				o.code = run(args(bf))
				entries := auditEntries(t)
				o.rows = len(entries)
				if o.rows > 0 {
					o.result, o.detail = entries[o.rows-1].Result, entries[o.rows-1].Detail
				}
				return o, h
			}
			var plain, fin outcome
			t.Run("plain", func(t *testing.T) {
				plain, _ = observe(func(bf string) []string {
					return []string{tc.lane, exampleRepo, "1", "--verdict", tc.verdict, "--head", tc.head, "--body-file", bf}
				})
			})
			t.Run("finish", func(t *testing.T) {
				var h *finishHarness
				fin, h = observe(func(bf string) []string {
					return finishArgs(tc.lane, "1", tc.verdict, tc.head, bf, finishKeyFor(tc.lane))
				})
				if h.f.postedReview != 0 {
					t.Errorf("a refused finish posted a review")
				}
				if h.deletes() != 0 {
					t.Errorf("a refused finish released the claim: %v", h.f.hits)
				}
				if got := h.out.String(); got != "" {
					t.Errorf("a refused finish printed a result line: %q", got)
				}
				if rows := releaseRows(t); len(rows) != 0 {
					t.Errorf("a refused finish wrote a release row: %+v", rows)
				}
				if !strings.Contains(h.errOut.String(), "STOPPED at step 1 of 3 (post)") && tc.wantCode != 2 {
					t.Errorf("the stop does not name the post step: %s", h.errOut.String())
				}
			})
			if plain.code != tc.wantCode {
				t.Fatalf("plain verb exit = %d, want %d", plain.code, tc.wantCode)
			}
			if fin != plain {
				t.Fatalf("finish diverged from the verb it wraps\n plain: %+v\nfinish: %+v", plain, fin)
			}
		})
	}
}

// TestFinishConfirmsAVerdictAlreadyOnTheChange: this session's log holds nothing, and the
// change already carries this exact verdict at the head — the state a re-dispatched reviewer
// finds. The post path's forge-state check must find it, so nothing is posted; finish then
// confirms THAT review and releases the claim.
//
// The first case is the one that matters: the recorded body is in the shape the writer
// posts, WITH its on-behalf-of line, which is what every review by the reviewer identity
// looks like on a real forge. A fixture without that line passes against a check that
// compares the caller's bytes and so proves nothing about a retry.
func TestFinishConfirmsAVerdictAlreadyOnTheChange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		recorded func(t *testing.T) string
	}{
		{"recorded in the shape the writer posts", func(t *testing.T) string { return postedShape(t, okReviewBody) }},
		{"recorded without an on-behalf-of line", func(*testing.T) string { return okReviewBody }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			prior := appReviewAt("APPROVED", testHead, tc.recorded(t))
			prior.ID = 77
			h.f.reviews = []reviewInfo{prior}
			bf := writeBody(t, "v.md", okReviewBody)

			if code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)); code != 0 {
				t.Fatalf("exit = %d, want 0\nstderr: %s", code, h.errOut.String())
			}
			if h.f.postedReview != 0 {
				t.Fatalf("postedReview = %d, want 0 — the verdict was already recorded, and a second one cannot be withdrawn", h.f.postedReview)
			}
			if h.deletes() != 1 {
				t.Fatalf("the claim was not released: %v", h.f.hits)
			}
			want := "deskpost finish: review=" + exampleRepo + "#1/review-77 state=APPROVED head=" + testHead + " claim=released\n"
			if got := h.out.String(); got != want {
				t.Fatalf("result\n got %q\nwant %q", got, want)
			}
		})
	}
}

// TestFinishRerunFromAnotherSessionPostsOnce: the first run posts and confirms the verdict
// and stops at the release. The re-run comes from a session whose log holds no row for it.
// Exactly one review is on the change afterwards.
func TestFinishRerunFromAnotherSessionPostsOnce(t *testing.T) {
	for _, tc := range []struct{ name, lane, verdict, body, state string }{
		{"correctness", "review", "approve", okReviewBody, "APPROVED"},
		{"security", "security-review", "pass", okSecurityBody, "COMMENTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			h.claimKey = finishKeyFor(tc.lane)
			h.deleteStatus = http.StatusInternalServerError
			bf := writeBody(t, "v.md", tc.body)
			args := finishArgs(tc.lane, "1", tc.verdict, testHead, bf, h.claimKey)
			if code := run(args); code == 0 {
				t.Fatal("first run: exit 0 with the release failing")
			}
			if h.f.postedReview != 1 {
				t.Fatalf("first run: postedReview = %d, want 1", h.f.postedReview)
			}

			forgetThisSession(t)
			h.out.Reset()
			h.deleteStatus = http.StatusNoContent
			if code := run(args); code != 0 {
				t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, h.errOut.String())
			}
			if h.f.postedReview != 1 {
				t.Fatalf("postedReview = %d after a re-run from another session, want 1 — the forge already held this verdict", h.f.postedReview)
			}
			want := "deskpost finish: review=" + exampleRepo + "#1/review-1001 state=" + tc.state + " head=" + testHead + " claim=released\n"
			if got := h.out.String(); got != want {
				t.Fatalf("re-run result\n got %q\nwant %q", got, want)
			}
		})
	}
}

// TestFinishRerunAfterALostPostResponse: the forge answers the review POST with a server
// error, and it holds the review all the same — a write that was accepted and whose answer
// was lost. finish stops at step 1 without saying nothing was posted; the same command again,
// in the same session, posts nothing, confirms the review that is there and releases.
func TestFinishRerunAfterALostPostResponse(t *testing.T) {
	h := setupFinish(t)
	inner := h.f.intercept
	failPost := true
	h.f.intercept = func(method, path string) (int, bool) {
		if failPost && method == http.MethodPost && reReviews.MatchString(path) {
			return http.StatusBadGateway, true
		}
		return inner(method, path)
	}
	bf := writeBody(t, "v.md", okReviewBody)
	args := finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)

	code := run(args)
	if code == 0 {
		t.Fatalf("first run: exit 0 with the POST answered %d", http.StatusBadGateway)
	}
	first := h.errOut.String()
	if !strings.Contains(first, "STOPPED at step 1 of 3 (post)") {
		t.Fatalf("the stop does not name the post step: %s", first)
	}
	if strings.Contains(first, "Nothing was posted") || strings.Contains(first, "nothing was posted") {
		t.Fatalf("the stop says nothing was posted after a write that was sent: %s", first)
	}
	if !strings.Contains(first, "may have left the verdict on the change") {
		t.Fatalf("the stop does not say the verdict may be on the change: %s", first)
	}
	if h.deletes() != 0 {
		t.Fatalf("the claim was released without a confirmed verdict: %v", h.f.hits)
	}

	held := appReviewAt("APPROVED", testHead, postedShape(t, okReviewBody))
	held.ID = 77
	h.f.reviews = []reviewInfo{held}
	failPost = false

	if code := run(args); code != 0 {
		t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, h.errOut.String())
	}
	if h.f.postedReview != 0 {
		t.Fatalf("postedReview = %d on the re-run, want 0 — the forge already held this verdict", h.f.postedReview)
	}
	want := "deskpost finish: review=" + exampleRepo + "#1/review-77 state=APPROVED head=" + testHead + " claim=released\n"
	if got := h.out.String(); got != want {
		t.Fatalf("re-run result\n got %q\nwant %q", got, want)
	}
}

// TestUnreadableReviewsRefuseThePost: the duplicate check reads the change's reviews before
// any post. When that read fails the verb posts nothing and exits non-zero — an unread
// listing is never taken for an empty one — and `finish` stops at its post step with the
// claim still held.
func TestUnreadableReviewsRefuseThePost(t *testing.T) {
	for _, tc := range []struct {
		name string
		args func(bf string) []string
		body string
	}{
		{"review", func(bf string) []string { return reviewArgs(exampleRepo, "1", "approve", testHead, bf) }, okReviewBody},
		{"security-review", func(bf string) []string { return secReviewArgs(exampleRepo, "1", "pass", testHead, bf) }, okSecurityBody},
		{"finish review", func(bf string) []string {
			return finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)
		}, okReviewBody},
		{"finish security-review", func(bf string) []string {
			return finishArgs("security-review", "1", "pass", testHead, bf, finishSecurityClaimKey)
		}, okSecurityBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			h.claimKey = finishClaimKey
			if strings.Contains(tc.name, "security") {
				h.claimKey = finishSecurityClaimKey
			}
			inner := h.f.intercept
			reads := 0
			h.f.intercept = func(method, path string) (int, bool) {
				if method == http.MethodGet && reReviews.MatchString(path) {
					reads++
					return http.StatusInternalServerError, true
				}
				return inner(method, path)
			}
			bf := writeBody(t, "v.md", tc.body)

			code := run(tc.args(bf))
			if reads == 0 {
				t.Fatal("fixture defect: the reviews were never read")
			}
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit = %d, want %d with the reviews unreadable\nstderr: %s", code, deskkit.ExitUnverifiable, h.errOut.String())
			}
			if h.f.postedReview != 0 {
				t.Fatalf("postedReview = %d — a verdict was posted over an unread listing", h.f.postedReview)
			}
			if h.deletes() != 0 {
				t.Fatalf("a claim was released: %v", h.f.hits)
			}
			for _, e := range auditEntries(t) {
				if e.Result == deskkit.ResultOK || e.Result == deskkit.ResultNoop {
					t.Fatalf("an audit row reads %q: %+v", e.Result, e)
				}
			}
			if strings.HasPrefix(tc.name, "finish") && !strings.Contains(h.errOut.String(), "STOPPED at step 1 of 3 (post)") {
				t.Fatalf("finish did not stop at its post step:\n%s", h.errOut.String())
			}
		})
	}
}

// TestFinishConfirmFails: the post succeeds but the verdict cannot be read back at the head —
// finish stops at the confirm step with exit 6 and the claim is NOT released.
func TestFinishConfirmFails(t *testing.T) {
	postBody, err := deskkit.AppendOnBehalfOf([]byte(okReviewBody), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	posted := string(postBody)
	for _, tc := range []struct {
		name  string
		after []reviewInfo
		fail  int // a forced status for the second reviews read, 0 for none
	}{
		{name: "the review is not listed", after: []reviewInfo{}},
		{name: "listed at another commit", after: []reviewInfo{appReviewAt("APPROVED", testOldHead, posted)}},
		{name: "listed in another state", after: []reviewInfo{appReviewAt("CHANGES_REQUESTED", testHead, posted)}},
		{name: "listed with another body", after: []reviewInfo{appReviewAt("APPROVED", testHead, laneABody)}},
		{name: "listed by another author", after: func() []reviewInfo {
			r := appReviewAt("APPROVED", testHead, posted)
			r.User.Login = "someone-else"
			return []reviewInfo{r}
		}()},
		{name: "the read-back fails", fail: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			h.f.reviewsAfterFirstRead = tc.after
			if tc.fail != 0 {
				inner := h.f.intercept
				h.f.intercept = func(method, path string) (int, bool) {
					if method == http.MethodGet && reReviews.MatchString(path) && h.f.postedReview > 0 {
						return tc.fail, true
					}
					return inner(method, path)
				}
			}
			bf := writeBody(t, "v.md", okReviewBody)

			code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit = %d, want %d\nstderr: %s", code, deskkit.ExitUnverifiable, h.errOut.String())
			}
			if h.f.postedReview != 1 {
				t.Fatalf("postedReview = %d, want 1 — the post step ran", h.f.postedReview)
			}
			if h.deletes() != 0 {
				t.Fatalf("the claim was released without a confirmed verdict: %v", h.f.hits)
			}
			if got := h.out.String(); got != "" {
				t.Fatalf("an unconfirmed finish printed a result line: %q", got)
			}
			msg := h.errOut.String()
			for _, want := range []string{"STOPPED at step 2 of 3 (confirm)", "claim " + finishClaimKey + " was NOT released"} {
				if !strings.Contains(msg, want) {
					t.Errorf("stderr lacks %q:\n%s", want, msg)
				}
			}
		})
	}
}

// TestComparableBodyIgnoresTheTrailerOnly: the comparison the duplicate check and the
// confirm share matches a posted body with its trailer and CRLF line ends, and does not match
// a body that differs in substance.
func TestComparableBodyIgnoresTheTrailerOnly(t *testing.T) {
	setupFinish(t)
	postBody, err := deskkit.AppendOnBehalfOf([]byte(okReviewBody), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	if string(postBody) == okReviewBody {
		t.Fatal("fixture defect: the writer appended no trailer, so this test would prove nothing")
	}
	if got := reviewComparableBody(string(postBody)); got != strings.TrimSpace(okReviewBody) {
		t.Fatalf("comparable body = %q, want the caller's body", got)
	}
	crlf := strings.ReplaceAll(string(postBody), "\n", "\r\n")
	if reviewComparableBody(crlf) != reviewComparableBody(string(postBody)) {
		t.Fatal("CRLF line ends changed the comparable body")
	}
	edited := strings.Replace(string(postBody), "No blockers.", "One blocker.", 1)
	if reviewComparableBody(edited) == reviewComparableBody(string(postBody)) {
		t.Fatal("a substantive edit did not change the comparable body")
	}

	// A caller body that itself holds such a line: the writer removes it on the way out, so
	// the posted body and the caller's body must still compare equal.
	planted := "First paragraph.\n" + deskkit.OnBehalfOfPrefix + " human:somebody-else\n" + okReviewBody
	postedPlanted, err := deskkit.AppendOnBehalfOf([]byte(planted), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf(planted): %v", err)
	}
	if strings.Contains(string(postedPlanted), "somebody-else") {
		t.Fatal("fixture defect: the writer kept the planted line, so this case would prove nothing")
	}
	if reviewComparableBody(string(postedPlanted)) != reviewComparableBody(planted) {
		t.Fatalf("a caller body holding its own on-behalf-of line did not compare equal to what was posted from it:\n posted=%q\n caller=%q",
			reviewComparableBody(string(postedPlanted)), reviewComparableBody(planted))
	}
	if reviewBodyDigest(postBody) != reviewBodyDigest([]byte(okReviewBody)) {
		t.Fatal("the digest the duplicate check compares differs between a caller body and the body posted from it")
	}
	if reviewBodyDigest([]byte(edited)) == reviewBodyDigest([]byte(okReviewBody)) {
		t.Fatal("the digest the duplicate check compares is the same for a substantively different body")
	}
}

// TestFinishReleaseFails: verdict posted and confirmed, release fails — non-zero, the message
// says the verdict IS posted and the claim is STILL HELD, and no result line is printed.
func TestFinishReleaseFails(t *testing.T) {
	t.Run("the forge refuses the delete", func(t *testing.T) {
		for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
			h := setupFinish(t)
			h.deleteStatus = status
			bf := writeBody(t, "v.md", okReviewBody)
			code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey))
			assertReleaseStop(t, h, code)
		}
	})
	t.Run("not-found but the claim is still there", func(t *testing.T) {
		h := setupFinish(t)
		h.deleteStatus = http.StatusUnprocessableEntity // the single-ref read still answers 200
		bf := writeBody(t, "v.md", okReviewBody)
		code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey))
		assertReleaseStop(t, h, code)
	})
	t.Run("not-found and the claim cannot be read", func(t *testing.T) {
		h := setupFinish(t)
		h.deleteStatus = http.StatusNotFound
		h.f.claimRefStatus = http.StatusForbidden
		bf := writeBody(t, "v.md", okReviewBody)
		code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey))
		assertReleaseStop(t, h, code)
	})
	t.Run("the release step errors", func(t *testing.T) {
		h := setupFinish(t)
		saved := releaseReviewClaimFn
		releaseReviewClaimFn = func(owner, name, key string) (string, error) {
			return "", errors.New("the store is unreachable")
		}
		t.Cleanup(func() { releaseReviewClaimFn = saved })
		bf := writeBody(t, "v.md", okReviewBody)
		code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey))
		assertReleaseStop(t, h, code)
		if !strings.Contains(h.errOut.String(), "the store is unreachable") {
			t.Errorf("the stop does not carry the release error: %s", h.errOut.String())
		}
	})
}

func assertReleaseStop(t *testing.T, h *finishHarness, code int) {
	t.Helper()
	if code == 0 {
		t.Fatalf("exit 0 with the claim still held\nstderr: %s", h.errOut.String())
	}
	// The release step's own audit row says the claim was not released, and is neither a
	// success nor a "nothing to do".
	rows := releaseRows(t)
	if len(rows) != 1 {
		t.Fatalf("release rows = %d, want exactly 1: %+v", len(rows), rows)
	}
	if r := rows[0]; r.Result == deskkit.ResultOK || r.Result == deskkit.ResultNoop ||
		!strings.Contains(r.Detail, "claim "+finishClaimKey+" NOT released") {
		t.Fatalf("release row = %+v, want a failure naming the claim", r)
	}
	if h.f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1", h.f.postedReview)
	}
	if got := h.out.String(); got != "" {
		t.Fatalf("a failed release printed a result line: %q", got)
	}
	msg := h.errOut.String()
	for _, want := range []string{"STOPPED at step 3 of 3 (release)", "the verdict IS posted and confirmed",
		"claim " + finishClaimKey + " is STILL HELD", "APPROVED at " + testHead} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr lacks %q:\n%s", want, msg)
		}
	}
}

// TestFinishClaimAlreadyGone: a claim that is positively absent is not a failure — the
// result line says so rather than claiming a release this run did not make.
func TestFinishClaimAlreadyGone(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnprocessableEntity} {
		h := setupFinish(t)
		h.deleteStatus = status
		h.f.claimRefStatus = http.StatusNotFound
		bf := writeBody(t, "v.md", okReviewBody)
		if code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)); code != 0 {
			t.Fatalf("delete %d: exit = %d, want 0\nstderr: %s", status, code, h.errOut.String())
		}
		lines := h.resultLines()
		if len(lines) != 1 || !strings.HasSuffix(lines[0], " claim=already-released") {
			t.Fatalf("delete %d: result = %q, want one line ending claim=already-released", status, h.out.String())
		}
		rows := releaseRows(t)
		if len(rows) != 1 || rows[0].Result != deskkit.ResultNoop || !strings.Contains(rows[0].Detail, "claim "+finishClaimKey+" was already released") {
			t.Fatalf("delete %d: release rows = %+v, want one noop row naming the claim", status, rows)
		}
	}
}

// TestFinishRerunAfterAFailedRelease: the documented recovery. The second run posts nothing
// new, confirms the verdict the first run posted, and releases the claim.
func TestFinishRerunAfterAFailedRelease(t *testing.T) {
	h := setupFinish(t)
	h.deleteStatus = http.StatusInternalServerError
	bf := writeBody(t, "v.md", okReviewBody)
	args := finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)
	if code := run(args); code == 0 {
		t.Fatal("first run: exit 0 with the release failing")
	}

	h.deleteStatus = http.StatusNoContent
	if code := run(args); code != 0 {
		t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, h.errOut.String())
	}
	if h.f.postedReview != 1 {
		t.Fatalf("postedReview = %d after the re-run, want 1 — a re-run must not post twice", h.f.postedReview)
	}
	lines := h.resultLines()
	if len(lines) != 1 || !strings.HasSuffix(lines[0], " claim=released") || !strings.Contains(lines[0], "state=APPROVED") {
		t.Fatalf("re-run result = %q", h.out.String())
	}
}

// TestFinishDryRun: only the post step is rehearsed. Nothing is posted, read back or released,
// and no result line claims otherwise.
func TestFinishDryRun(t *testing.T) {
	h := setupFinish(t)
	bf := writeBody(t, "v.md", okReviewBody)
	args := append(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey), "--dry-run")
	if code := run(args); code != 0 {
		t.Fatalf("dry-run exit = %d, want 0\nstderr: %s", code, h.errOut.String())
	}
	if h.f.postedReview != 0 || h.deletes() != 0 {
		t.Fatalf("a dry run wrote: posted=%d deletes=%d", h.f.postedReview, h.deletes())
	}
	if got := h.out.String(); got != "" {
		t.Fatalf("a dry run printed a result line: %q", got)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultDryRun {
		t.Fatalf("audit result = %q, want dryrun", e.Result)
	}
	if rows := releaseRows(t); len(rows) != 0 {
		t.Fatalf("a dry run wrote a release row: %+v", rows)
	}
	if !strings.Contains(h.errOut.String(), "dry run") {
		t.Errorf("stderr does not say it was a dry run: %s", h.errOut.String())
	}
}

// TestFinishArgumentErrorsWriteNothing: every argument error is exit 2 with no forge request,
// no audit row and nothing posted — including a claim key that is not this change's.
func TestFinishArgumentErrorsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		args func(bf string) []string
		want string
	}{
		{"no lane", func(string) []string { return []string{"finish"} }, "usage: deskpost finish"},
		{"unknown lane", func(bf string) []string { return finishArgs("comment", "1", "approve", testHead, bf, finishClaimKey) }, "`review` or `security-review`"},
		{"no claim", func(bf string) []string { return finishArgs("review", "1", "approve", testHead, bf, "") }, "--claim <claim key> is required"},
		{"another change's claim", func(bf string) []string { return finishArgs("review", "1", "approve", testHead, bf, "tracker--pr-2") }, "does not belong to"},
		{"a neighbouring number", func(bf string) []string { return finishArgs("review", "1", "approve", testHead, bf, "tracker--pr-10") }, "does not belong to"},
		{"another repository's claim", func(bf string) []string { return finishArgs("review", "1", "approve", testHead, bf, "other--pr-1") }, "does not belong to"},
		{"a ref path, not a key", func(bf string) []string {
			return finishArgs("review", "1", "approve", testHead, bf, "tracker--pr-1/../../heads/main")
		}, "does not belong to"},
		{"abbreviated head", func(bf string) []string {
			return finishArgs("review", "1", "approve", testHead[:8], bf, finishClaimKey)
		}, "plus --claim"},
		{"unknown flag", func(bf string) []string {
			return append(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey), "--no-such-flag")
		}, "plus --claim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			before := helpAuditBytes(t)
			bf := writeBody(t, "v.md", okReviewBody)
			if code := run(tc.args(bf)); code != 2 {
				t.Fatalf("exit = %d, want 2\nstderr: %s", code, h.errOut.String())
			}
			if !strings.Contains(h.errOut.String(), tc.want) {
				t.Errorf("stderr lacks %q:\n%s", tc.want, h.errOut.String())
			}
			if len(h.f.hits) != 0 {
				t.Errorf("an argument error touched the forge: %v", h.f.hits)
			}
			if after := helpAuditBytes(t); string(after) != string(before) {
				t.Errorf("an argument error wrote an audit row")
			}
		})
	}
}

// TestFinishHelpNoRow: every help spelling prints the screen, exits 0, writes no audit row and
// touches no forge — the property TestTierTwoHelpNoRow pins for the other verbs.
func TestFinishHelpNoRow(t *testing.T) {
	for _, args := range [][]string{
		{"finish", "--help"},
		{"finish", "review", "--help"},
		{"finish", "security-review", "-h"},
		{"finish", "review", exampleRepo, "1", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := setupFinish(t)
			before := helpAuditBytes(t)
			if code := run(args); code != deskkit.ExitOK {
				t.Errorf("exit %d, want 0", code)
			}
			if !strings.Contains(h.errOut.String(), "deskpost finish review|security-review") {
				t.Errorf("no help screen naming finish on stderr: %q", h.errOut.String())
			}
			if after := helpAuditBytes(t); string(after) != string(before) {
				t.Errorf("help appended to the audit ledger")
			}
			if len(h.f.hits) != 0 {
				t.Errorf("a help request touched the forge: %v", h.f.hits)
			}
		})
	}
}

// TestFinishAcceptsThisLanesSuffixedClaim: a re-dispatch's claim key carries further
// `--<segment>` parts; a key of this lane is accepted with them and is the ref that gets
// deleted.
func TestFinishAcceptsThisLanesSuffixedClaim(t *testing.T) {
	for _, tc := range []struct{ name, lane, verdict, body, key string }{
		{"security", "security-review", "pass", okSecurityBody, finishSecurityClaimKey},
		{"security, re-dispatched", "security-review", "pass", okSecurityBody, finishClaimKey + "--r2--security"},
		{"security, upper case", "security-review", "pass", okSecurityBody, finishClaimKey + "--SECURITY"},
		{"correctness, re-dispatched", "review", "approve", okReviewBody, finishClaimKey + "--r2"},
		{"correctness, named", "review", "approve", okReviewBody, finishClaimKey + "--correctness"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			h.claimKey = tc.key
			bf := writeBody(t, "v.md", tc.body)
			if code := run(finishArgs(tc.lane, "1", tc.verdict, testHead, bf, tc.key)); code != 0 {
				t.Fatalf("exit = %d, want 0\nstderr: %s", code, h.errOut.String())
			}
			if h.f.hitCount(http.MethodDelete, finishRefPath(tc.key)) != 1 || h.deletes() != 1 {
				t.Fatalf("want exactly one delete, of this key's ref: %v", h.f.hits)
			}
			if !strings.HasSuffix(strings.TrimSpace(h.out.String()), "claim=released") {
				t.Fatalf("result = %q", h.out.String())
			}
		})
	}
}

// TestFinishRefusesAnotherLanesClaim: a finish of one lane given the other lane's key for the
// same change is an argument error. Nothing is posted, no ref is deleted, no forge request is
// made and no audit row is written — so one lane's finish cannot release the claim the other
// lane's review is still running under.
func TestFinishRefusesAnotherLanesClaim(t *testing.T) {
	for _, tc := range []struct{ name, lane, verdict, body, key, want string }{
		{"security finish, the change's bare key", "security-review", "pass", okSecurityBody, finishClaimKey,
			"is not the security lane's claim key"},
		{"security finish, a correctness re-dispatch key", "security-review", "pass", okSecurityBody, finishClaimKey + "--r2",
			"is not the security lane's claim key"},
		{"security finish, a key that only contains the word", "security-review", "pass", okSecurityBody, finishClaimKey + "--insecurity",
			"is not the security lane's claim key"},
		{"correctness finish, the security key", "review", "approve", okReviewBody, finishSecurityClaimKey,
			"is the security lane's claim key"},
		{"correctness finish, the security key in upper case", "review", "approve", okReviewBody, finishClaimKey + "--SECURITY",
			"is the security lane's claim key"},
		{"correctness finish, a security re-dispatch key", "review", "approve", okReviewBody, finishClaimKey + "--r2--security",
			"is the security lane's claim key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupFinish(t)
			h.claimKey = tc.key
			before := helpAuditBytes(t)
			bf := writeBody(t, "v.md", tc.body)
			if code := run(finishArgs(tc.lane, "1", tc.verdict, testHead, bf, tc.key)); code != 2 {
				t.Fatalf("exit = %d, want 2\nstderr: %s", code, h.errOut.String())
			}
			msg := h.errOut.String()
			for _, want := range []string{tc.want, "Nothing was posted and no claim was released"} {
				if !strings.Contains(msg, want) {
					t.Errorf("stderr lacks %q:\n%s", want, msg)
				}
			}
			if h.f.postedReview != 0 {
				t.Errorf("a finish given another lane's key posted a review")
			}
			if len(h.f.hits) != 0 {
				t.Errorf("a finish given another lane's key touched the forge: %v", h.f.hits)
			}
			if got := h.out.String(); got != "" {
				t.Errorf("a finish given another lane's key printed a result line: %q", got)
			}
			if after := helpAuditBytes(t); string(after) != string(before) {
				t.Errorf("a finish given another lane's key wrote an audit row")
			}
		})
	}
}
