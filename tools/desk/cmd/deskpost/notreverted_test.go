package main

// `deskpost ready`'s refuse-only not-reverted check (medici-finance/assay#6254). deskflip refuses a
// head the post-flip backstop already returned to draft (#6253 review F2); the other live path to
// the identical ready mutation must clear the same condition, or a reviewer reaching for
// `ready` re-flips a reverted head and the backstop reverts it again. The check reads the
// comment thread, which the #513 anti-forgery control forbade: the driver ruled it may, ONLY as
// a read that can refuse and never admit
// (https://github.com/medici-finance/assay/issues/6254#issuecomment-7086317256). The planted
// comment tests that prove comments cannot satisfy a gate are in noopforgery_test.go.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// revertComment is a backstop revert comment for head, as the reviewer App posts it.
func revertComment(id int, login, head string) map[string]any {
	return map[string]any{
		"id":       id,
		"html_url": "https://example.test/pull/1#issuecomment-900",
		"user":     map[string]any{"login": login},
		"body": "Returned to draft by the post-flip check: right after this change was marked ready, " +
			"the forge reported its merge state as `DIRTY` at head `" + head + "`.\n\n" +
			deskkit.ReadyBackstopRevertMarker(head),
	}
}

func approvedGreenFake(t *testing.T) *fakeGH {
	t.Helper()
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	return f
}

// A trusted revert marker for the current head stops the flip as could-not-check, names the
// comment, and writes nothing.
//
// regression: #6254
func TestReadyNotRevertedRefusesHead(t *testing.T) {
	f := approvedGreenFake(t)
	f.issueComments = []map[string]any{revertComment(900, reviewerBotDisplay(), testHead)}

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d: a reverted head is could-not-check", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatal("a head the backstop already reverted was flipped again")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "already returned PR #1 to draft") || !strings.Contains(d, "issuecomment-900") {
		t.Fatalf("the refusal should name the revert and the comment, got: %s", d)
	}
}

// A new head clears the condition: a marker naming an earlier head holds nothing.
//
// regression: #6254
func TestReadyNotRevertedNewHeadFlips(t *testing.T) {
	f := approvedGreenFake(t)
	f.issueComments = []map[string]any{revertComment(900, reviewerBotDisplay(), testOldHead)}

	if code := run(readyArgs(exampleRepo)); code != 0 || f.flips != 1 {
		t.Fatalf("exit = %d flips = %d, want a flip: the marker names an earlier head", code, f.flips)
	}
}

// A stranger cannot hold a head out of ready by planting the marker.
//
// regression: #6254
func TestReadyNotRevertedIgnoresStranger(t *testing.T) {
	f := approvedGreenFake(t)
	f.issueComments = []map[string]any{revertComment(900, "example-stranger", testHead)}

	if code := run(readyArgs(exampleRepo)); code != 0 || f.flips != 1 {
		t.Fatalf("exit = %d flips = %d, want a flip: an untrusted marker is ignored", code, f.flips)
	}
}

// The revert comment is the newest on the thread, so the read walks every page.
//
// regression: #6254
func TestReadyNotRevertedReadsEveryPage(t *testing.T) {
	f := approvedGreenFake(t)
	for i := 0; i < 100; i++ {
		f.issueComments = append(f.issueComments, map[string]any{
			"id": 1000 + i, "body": "noise", "user": map[string]any{"login": "example-human"}})
	}
	f.issueComments = append(f.issueComments, revertComment(900, reviewerBotDisplay(), testHead))

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitUnverifiable || f.flips != 0 {
		t.Fatalf("exit = %d flips = %d, want the marker on page 2 to hold the head", code, f.flips)
	}
	if n := f.hitCount("GET", "/issues/1/comments"); n != 2 {
		t.Fatalf("comment page reads = %d, want 2", n)
	}
}

// The walk is bounded like deskflip's read of the same thread: a thread still full at the last
// page under the cap is could-not-check, never "not reverted", and the walk stops at the cap.
//
// regression: #6254
func TestReadyNotRevertedBoundedWalkFailsClosed(t *testing.T) {
	f := approvedGreenFake(t)
	for i := 0; i < maxChangeCommentPages*100+1; i++ {
		f.issueComments = append(f.issueComments, map[string]any{
			"id": 1000 + i, "body": "noise", "user": map[string]any{"login": "example-human"}})
	}

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable || f.flips != 0 {
		t.Fatalf("exit = %d flips = %d, want could-not-check: the thread was not read to the end", code, f.flips)
	}
	if n := f.hitCount("GET", "/issues/1/comments"); n != maxChangeCommentPages {
		t.Fatalf("comment page reads = %d, want the walk to stop at %d", n, maxChangeCommentPages)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "not read to the end") {
		t.Fatalf("the refusal should say the thread was not read whole, got: %s", d)
	}
}

// A thread that cannot be read is could-not-check, never "not reverted".
//
// regression: #6254
func TestReadyNotRevertedUnreadableFailsClosed(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := approvedGreenFake(t)
			f.intercept = func(method, path string) (int, bool) {
				if method == "GET" && strings.HasSuffix(path, "/issues/1/comments") {
					return status, true
				}
				return 0, false
			}
			code := run(readyArgs(exampleRepo))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit = %d, want %d: an unreadable thread is could-not-check", code, deskkit.ExitUnverifiable)
			}
			if f.flips != 0 {
				t.Fatalf("FAILS OPEN: an unreadable comment thread (%d) flipped the PR", status)
			}
		})
	}
}
