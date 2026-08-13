package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #348 / #252: a CI read that 403s under the App installation token used to
// surface as the bare line
//
//	deskpost: GitHub API GET /repos/o/r/commits/<sha>/status?... returned HTTP 403
//
// which is indistinguishable from a transient API failure. It took FOUR occurrences (and
// then twenty-two) before anyone read it as "the installation does not hold statuses:
// read", and every one of those flips was re-run through a raw `gh pr ready` that has none
// of this tool's preconditions. These tests pin the diagnosis, not the plumbing: same exit
// code, same zero side effects, a message that names the permission.

// force403 makes the fake return 403 for every request whose path ends in suffix.
func force403(f *fakeGH, suffix string) {
	f.intercept = func(method, path string) (int, bool) {
		if strings.HasSuffix(path, suffix) {
			return http.StatusForbidden, true
		}
		return 0, false
	}
}

func TestReady403OnCombinedStatusNamesStatusesRead(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	force403(f, "/status")

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("403 on combined status exit = %d, want %d (fail-closed)", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatal("a 403 on the CI read must never reach the flip")
	}
	msg := errBuf.String()
	for _, want := range []string{
		"statuses: read",         // the permission, named
		"PERMISSION answer",      // not a transient failure
		"combined commit status", // which precondition was lost
		"do NOT route around",    // and what not to do about it
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("403 message does not mention %q:\n%s", want, msg)
		}
	}
	// #448: a 403 on a CI READ never reaches — let alone lands — a write, so
	// it must audit as the non-charging ResultUnwritten, not ResultUnverifiable (which
	// charges the outward-write budget on the theory the call may have reached the
	// remote). The exit code above is unchanged; only the audited RESULT — and therefore
	// the budget bill — differs from before this fix.
	if e := lastAudit(t); e.Result != deskkit.ResultUnwritten {
		t.Fatalf("audit result = %q, want unwritten", e.Result)
	}
}

func TestReady403OnCheckRunsNamesChecksRead(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = checksWith("completed", "success")
	force403(f, "/check-runs")

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("403 on check-runs exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatal("a 403 on the CI read must never reach the flip")
	}
	if msg := errBuf.String(); !strings.Contains(msg, "checks: read") || !strings.Contains(msg, "check runs") {
		t.Fatalf("403 message does not name checks: read / the check-runs rollup:\n%s", msg)
	}
}

// The installation is named because the grant is per-APP but the TOKEN embeds the
// permission set at mint time — a token minted before the grant must be re-minted to see the
// grant take effect, and the installation cannot always be confirmed from the refusal alone.
// An operator reading the refusal should not have to work out which of the two it is.
func TestReady403NamesTheInstallation(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	force403(f, "/status")

	run(readyArgs(exampleRepo))
	if msg := errBuf.String(); !strings.Contains(msg, "100000002") {
		t.Fatalf("403 message does not name the installation it minted for:\n%s", msg)
	}
}

// A NON-403 failure must not acquire a permission story it has no evidence for. 500 is a
// real transient class and saying "grant statuses: read" about one would send an operator
// after a permission they already hold — the mirror of the defect being fixed.
func TestReadyNon403KeepsPlainMessage(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.intercept = func(method, path string) (int, bool) {
		if strings.HasSuffix(path, "/status") {
			return http.StatusInternalServerError, true
		}
		return 0, false
	}

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("500 exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	msg := errBuf.String()
	if strings.Contains(msg, "statuses: read") || strings.Contains(msg, "PERMISSION answer") {
		t.Fatalf("a 500 must not be reported as a permission gap:\n%s", msg)
	}
	if !strings.Contains(msg, "HTTP 500") {
		t.Fatalf("500 message lost the status code:\n%s", msg)
	}
}

// appScopeFor is a lookup over the path shapes github.go itself builds. An unrecognised
// path yields "" and the caller degrades to the generic 403 text — the hint is never a
// gate, so a missing entry costs a sentence, not correctness.
func TestAppScopeFor(t *testing.T) {
	cases := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/repos/o/r/commits/abc/status?per_page=100&page=1", "statuses: read"},
		{http.MethodGet, "/repos/o/r/commits/abc/check-runs?per_page=100&page=1", "checks: read"},
		{http.MethodGet, "/repos/o/r/pulls/7", "pull_requests: read"},
		{http.MethodGet, "/repos/o/r/pulls/7/files?per_page=100&page=1", "pull_requests: read"},
		{http.MethodPost, "/repos/o/r/pulls/7/reviews", "pull_requests: write"},
		{http.MethodPost, "/repos/o/r/issues/7/comments", "issues: write"},
		{http.MethodPost, "/graphql", ""},
		{http.MethodPost, "/app/installations/1/access_tokens", ""},
	}
	for _, c := range cases {
		if got := appScopeFor(c.method, c.path); got != c.want {
			t.Errorf("appScopeFor(%s %s) = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}

// The diagnosis rides on apiError.Error(), so every App-token call gets it — not just the
// two CI reads that happened to be measured. A 403 on the review POST is the same class of
// answer and reads the same way.
func TestReview403NamesPullRequestsWrite(t *testing.T) {
	f, errBuf := setupFake(t)
	bf := writeBody(t, "rev.md", okReviewBody)
	f.intercept = func(method, path string) (int, bool) {
		if method == http.MethodPost && strings.HasSuffix(path, "/reviews") {
			return http.StatusForbidden, true
		}
		return 0, false
	}

	code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("403 on review POST exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if msg := errBuf.String(); !strings.Contains(msg, "pull_requests: write") {
		t.Fatalf("403 on the review POST does not name pull_requests: write:\n%s", msg)
	}
}
