package main

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestFetchThenPushBudget(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-budget")
	withEnv(t, work)
	asWorker(t)
	for i := 0; i < deskkit.RateLimitPerPRPerHour+1; i++ {
		if code := run([]string{"fetch"}); code != deskkit.ExitOK {
			t.Fatalf("fixture fetch %d: exit %d", i, code)
		}
	}
	if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("fetch-only history prevented real fixture push: exit %d", code)
	}
	upstream := mustGit(t, work, "remote", "get-url", "origin")
	if got, want := mustGit(t, upstream, "rev-parse", "refs/heads/feature-budget"), mustGit(t, work, "rev-parse", "HEAD"); got != want {
		t.Fatalf("remote head %s != local head %s", got, want)
	}
	var fetches, pushes int
	for _, e := range readAudit(t) {
		if e.Tool == "deskgit" && e.Verb == "fetch" && e.Result == deskkit.ResultOK {
			fetches++
		}
		if e.Tool == "deskgit" && e.Verb == "push" && e.Result == deskkit.ResultOK {
			pushes++
		}
	}
	if fetches != deskkit.RateLimitPerPRPerHour+1 || pushes != 1 {
		t.Fatalf("audit lost operation outcomes: %d fetches, %d pushes", fetches, pushes)
	}
}

func TestPushBudgetRefusalAudit(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-budget-refusal")
	calls := withEnv(t, work)
	called := asWorker(t)
	now := time.Now().UTC()
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{Tool: "deskgit", Verb: "push", Repo: allowedSlug, Result: deskkit.ResultOK, TS: now.Add(-59 * time.Minute).Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitRateLimited {
			t.Fatalf("push at cap: exit %d, want rate limited", code)
		}
	}
	if *called || gitCallWith(*calls, "push") != nil {
		t.Fatal("blocked push reached credentials or transport")
	}
	entries := readAudit(t)
	for _, e := range entries[len(entries)-3:] {
		if e.Tool != "deskgit" || e.Verb != "push" || e.Result != deskkit.ResultRateLimited {
			t.Fatalf("budget refusal misclassified: tool=%s verb=%s result=%s", e.Tool, e.Verb, e.Result)
		}
	}
	// Once the original writes expire, denied attempts must not prolong the cap.
	if err := deskkit.AllowWriteAt("deskgit", allowedSlug, 0, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("refusals renewed the exhausted budget: %v", err)
	}
}
