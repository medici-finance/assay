package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #197: `deskpost review --head <sha>` exists so a verdict cannot attach to a
// commit it never assessed. On exit 4 the documented fallback was a raw
//
//	gh pr review <n> -R <repo> --request-changes --body-file F
//
// which has NO head-pinning flag — it attaches to whatever the head is when it runs. On
// #195 that landed a CHANGES_REQUESTED written against 151ebe99 onto 2bbf529c, a commit
// that had already fixed two of its three findings. Under a saturated limiter the fallback
// is the COMMON path, so the protection was off more often than on.
//
// --wait removes the fork: the refusal degrades to WAITING, not to a less-safe command.

// fakeClock replaces BOTH clock seams: a "sleep" records its duration and advances the
// clock the retry loop AND the rate limiter read from (attemptOutward passes timeNow() to
// deskkit.AllowWriteAt). Advancing only the loop's clock would produce a test that proves
// deskpost retries and proves nothing about whether the retry can ever succeed — the
// budget would still be spent on every pass, forever.
type fakeClock struct {
	slept  []time.Duration
	offset time.Duration
}

func (c *fakeClock) install(t *testing.T) {
	t.Helper()
	oldSleep, oldNow := sleepFor, timeNow
	sleepFor = func(d time.Duration) {
		c.slept = append(c.slept, d)
		c.offset += d
	}
	timeNow = func() time.Time { return oldNow().Add(c.offset) }
	t.Cleanup(func() { sleepFor, timeNow = oldSleep, oldNow })
}

// exhaustBudget spends the hourly outward-write budget on real comment posts, so the next
// call gets a genuine exit-4 from deskkit's limiter rather than a synthetic one.
func exhaustBudget(t *testing.T, f *fakeGH) {
	t.Helper()
	for i := 0; i < 30; i++ {
		bf := writeBody(t, "fill.md", "filler comment "+time.Duration(i).String()+"\n")
		if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf}); code == deskkit.ExitRateLimited {
			return
		}
	}
	t.Fatalf("budget never exhausted after 30 posts (postedCmt=%d)", f.postedCmt)
}

// The whole point, end to end: exit 4 becomes a wait and then a HEAD-PINNED post through
// this tool, instead of an exit 4 the caller hands to a raw `gh` command that cannot pin a
// head at all.
func TestWaitSleepsThenPosts(t *testing.T) {
	f, errBuf := setupFake(t)
	exhaustBudget(t, f)
	before := f.postedCmt

	var clk fakeClock
	clk.install(t)
	bf := writeBody(t, "real.md", "the comment that matters\n")
	code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--wait", "70m"})

	if code != 0 {
		t.Fatalf("exit = %d, want 0 — --wait must ride out the budget and post", code)
	}
	if len(clk.slept) != 1 {
		t.Fatalf("slept %v, want exactly one wait", clk.slept)
	}
	if clk.slept[0] <= 0 {
		t.Fatalf("slept for %s — a non-positive wait is a spin", clk.slept[0])
	}
	if f.postedCmt != before+1 {
		t.Fatalf("postedCmt = %d, want %d — the post must land after the wait", f.postedCmt, before+1)
	}
	if !strings.Contains(errBuf.String(), "sleeping") {
		t.Fatalf("--wait did not say it was waiting:\n%s", errBuf.String())
	}
	if e := lastAudit(t); e.Result != deskkit.ResultOK {
		t.Fatalf("audit result = %q, want ok", e.Result)
	}
}

// The wait is BOUNDED by what the caller asked for. A retry-after past the budget returns
// the plain exit 4 and says so, rather than silently blocking for an hour — an unbounded
// in-tool wait is its own hazard.
func TestWaitRefusesWhenRetryAfterExceedsBudget(t *testing.T) {
	f, errBuf := setupFake(t)
	exhaustBudget(t, f)

	var clk fakeClock
	clk.install(t)
	bf := writeBody(t, "real.md", "the comment that matters\n")
	code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--wait", "1s"})

	if code != deskkit.ExitRateLimited {
		t.Fatalf("exit = %d, want 4", code)
	}
	if len(clk.slept) != 0 {
		t.Fatalf("slept %v despite the retry-after exceeding --wait", clk.slept)
	}
	if !strings.Contains(errBuf.String(), "past the --wait budget") {
		t.Fatalf("stderr does not explain why it did not wait:\n%s", errBuf.String())
	}
	if f.postedCmt == 0 {
		t.Fatal("harness never posted anything — the budget was not really exercised")
	}
}

// Default behaviour is unchanged: no --wait, no waiting, plain exit 4.
func TestNoWaitFlagKeepsExit4(t *testing.T) {
	f, _ := setupFake(t)
	exhaustBudget(t, f)

	var clk fakeClock
	clk.install(t)
	bf := writeBody(t, "real.md", "the comment that matters\n")
	if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf}); code != deskkit.ExitRateLimited {
		t.Fatalf("exit = %d, want 4", code)
	}
	if len(clk.slept) != 0 {
		t.Fatalf("slept %v without --wait", clk.slept)
	}
}

// --wait never waits out anything except the rate limiter. A refusal (exit 5) or an
// unverifiable (exit 6) is an answer about the world; retrying it is the refusal loop the
// non-progress breaker exists to stop.
func TestWaitDoesNotRetryARefusal(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", "no heading and no verdict line\n")

	var clk fakeClock
	clk.install(t)
	args := append(reviewArgs(exampleRepo, "1", "approve", testHead, bf), "--wait", "30m")
	if code := run(args); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want 5", code)
	}
	if len(clk.slept) != 0 {
		t.Fatalf("slept %v on a refusal — --wait is for the limiter only", clk.slept)
	}
	if f.postedReview != 0 {
		t.Fatal("no POST")
	}
}

func TestWaitFlagValidation(t *testing.T) {
	cases := []struct {
		val, wantMsg string
	}{
		{"soon", "is not a duration"},
		{"91m", "out of range"},
		{"90h", "out of range"},
		{"-5m", "out of range"},
	}
	for _, c := range cases {
		_, errBuf := setupFake(t)
		bf := writeBody(t, "c.md", "x\n")
		code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--wait", c.val})
		if code != 2 {
			t.Errorf("--wait %s exit = %d, want 2", c.val, code)
		}
		if !strings.Contains(errBuf.String(), c.wantMsg) {
			t.Errorf("--wait %s stderr does not contain %q:\n%s", c.val, c.wantMsg, errBuf.String())
		}
	}
}
