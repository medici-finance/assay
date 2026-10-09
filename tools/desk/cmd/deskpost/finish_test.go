package main

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// finishClaimKey is a review-dispatch claim key of exampleRepo#1 — "<short label>--pr-<N>".
const finishClaimKey = "tracker--pr-1"

const finishClaimRefPath = "/git/refs/dispatch/" + finishClaimKey

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
}

func setupFinish(t *testing.T) *finishHarness {
	t.Helper()
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	h := &finishHarness{f: f, errOut: errBuf, out: &bytes.Buffer{}, deleteStatus: http.StatusNoContent}
	saved := stdout
	stdout = h.out
	t.Cleanup(func() { stdout = saved })
	f.intercept = func(method, path string) (int, bool) {
		if method == http.MethodDelete && strings.HasSuffix(path, finishClaimRefPath) && h.deleteStatus != 0 {
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

func (h *finishHarness) deletes() int { return h.f.hitCount(http.MethodDelete, finishClaimRefPath) }

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
			bf := writeBody(t, "v.md", tc.body)

			if code := run(finishArgs(tc.lane, "1", tc.verdict, testHead, bf, finishClaimKey)); code != 0 {
				t.Fatalf("finish exit = %d, want 0\nstderr: %s", code, h.errOut.String())
			}
			if h.f.postedReview != 1 {
				t.Fatalf("postedReview = %d, want 1", h.f.postedReview)
			}
			post := h.order(http.MethodPost, "/pulls/1/reviews")
			del := h.order(http.MethodDelete, finishClaimRefPath)
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

			// One audit row, and it is the post step's: same verb key the plain verb records.
			entries := auditEntries(t)
			if len(entries) != 1 {
				t.Fatalf("audit rows = %d, want exactly 1: %+v", len(entries), entries)
			}
			if e := entries[0]; e.Result != deskkit.ResultOK || e.Verb != tc.wantVerb || e.HeadSHA == nil || *e.HeadSHA != testHead {
				t.Fatalf("audit = %+v", e)
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
					return finishArgs(tc.lane, "1", tc.verdict, tc.head, bf, finishClaimKey)
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

// TestFinishConfirmsAVerdictAlreadyOnTheChange: when the post path finds this exact verdict
// already recorded at the head (its own duplicate guard — nothing is posted), finish still
// confirms THAT review and releases the claim. The recorded body here carries no trailer, so
// this is also the case that needs the trailer taken off the expected side.
func TestFinishConfirmsAVerdictAlreadyOnTheChange(t *testing.T) {
	h := setupFinish(t)
	prior := appReviewAt("APPROVED", testHead, okReviewBody)
	prior.ID = 77
	h.f.reviews = []reviewInfo{prior}
	bf := writeBody(t, "v.md", okReviewBody)

	if code := run(finishArgs("review", "1", "approve", testHead, bf, finishClaimKey)); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, h.errOut.String())
	}
	if h.f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — the verdict was already recorded", h.f.postedReview)
	}
	if h.deletes() != 1 {
		t.Fatalf("the claim was not released: %v", h.f.hits)
	}
	want := "deskpost finish: review=" + exampleRepo + "#1/review-77 state=APPROVED head=" + testHead + " claim=released\n"
	if got := h.out.String(); got != want {
		t.Fatalf("result\n got %q\nwant %q", got, want)
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

// TestFinishConfirmIgnoresTheTrailerOnly: the confirm matches a posted body with its trailer
// and CRLF line ends, and does not match a body that differs in substance.
func TestFinishConfirmIgnoresTheTrailerOnly(t *testing.T) {
	setupFinish(t)
	postBody, err := deskkit.AppendOnBehalfOf([]byte(okReviewBody), "", exampleRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	if string(postBody) == okReviewBody {
		t.Fatal("fixture defect: the writer appended no trailer, so this test would prove nothing")
	}
	if got := finishComparableBody(string(postBody)); got != strings.TrimSpace(okReviewBody) {
		t.Fatalf("comparable body = %q, want the caller's body", got)
	}
	crlf := strings.ReplaceAll(string(postBody), "\n", "\r\n")
	if finishComparableBody(crlf) != finishComparableBody(string(postBody)) {
		t.Fatal("CRLF line ends changed the comparable body")
	}
	edited := strings.Replace(string(postBody), "No blockers.", "One blocker.", 1)
	if finishComparableBody(edited) == finishComparableBody(string(postBody)) {
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
	if finishComparableBody(string(postedPlanted)) != finishComparableBody(planted) {
		t.Fatalf("a caller body holding its own on-behalf-of line did not compare equal to what was posted from it:\n posted=%q\n caller=%q",
			finishComparableBody(string(postedPlanted)), finishComparableBody(planted))
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

// TestFinishAcceptsALaneSuffixedClaim: a re-dispatch's claim key carries a --<lane> suffix;
// it is this change's claim and is the ref that gets deleted.
func TestFinishAcceptsALaneSuffixedClaim(t *testing.T) {
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	out := &bytes.Buffer{}
	saved := stdout
	stdout = out
	t.Cleanup(func() { stdout = saved })
	const key = finishClaimKey + "--security"
	f.intercept = func(method, path string) (int, bool) {
		if method == http.MethodDelete && strings.HasSuffix(path, "/git/refs/dispatch/"+key) {
			return http.StatusNoContent, true
		}
		return 0, false
	}
	bf := writeBody(t, "v.md", okSecurityBody)
	if code := run(finishArgs("security-review", "1", "pass", testHead, bf, key)); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if f.hitCount(http.MethodDelete, "/git/refs/dispatch/"+key) != 1 {
		t.Fatalf("the suffixed claim ref was not deleted: %v", f.hits)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.String()), "claim=released") {
		t.Fatalf("result = %q", out.String())
	}
}
