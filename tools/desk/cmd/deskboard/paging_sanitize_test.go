package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Pagination-to-exhaustion is now the FORGE backend's responsibility (ReviewsAtHead /
// ListOpenIssues page internally and are pinned by the forge golden corpus). What these tests
// keep proving is the deskboard-level behavior that RIDES a complete read: the #216 security
// retraction reduction over a full review set, the verify-gate queue over a full issue set, and
// the ANSI sanitization of hostile titles.

// TestFetchReviewsReadsEveryReview — 101 reviews: the FIRST carries Security-Review: pass and
// the LAST carries Security-Review: fail. The forge returns the whole set (paging is its job),
// so the reduction (#216) must see the retraction and report NOT green.
func TestFetchReviewsReadsEveryReview(t *testing.T) {
	const head = "HEADSHA"
	installFakeForge(t)
	forgeHooks.reviews = func(string, int) ([]deskkit.Review, error) {
		var out []deskkit.Review
		for i := 0; i < 101; i++ {
			body := fmt.Sprintf("## Review %d\n\nVerdict: approve\n", i)
			switch i {
			case 0:
				body = "## Security review\n\nSecurity-Review: pass\n"
			case 100:
				body = "## Security review\n\nRetracted.\n\nSecurity-Review: fail\n"
			}
			out = append(out, deskkit.Review{
				Author: deskkit.Account{Login: reviewerBotDisplay()}, Body: body,
				State: "APPROVED", CommitID: head, SubmittedAt: "2026-07-30T00:00:00Z",
			})
		}
		return out, nil
	}

	got, err := fetchReviews("medici-finance/assay", 1)
	if err != nil {
		t.Fatalf("fetchReviews: %v", err)
	}
	if len(got) != 101 {
		t.Fatalf("fetchReviews returned %d reviews, want 101 — the full set must reach the reduction", len(got))
	}
	if st := reduceReviews(got, head); st.securityPass {
		t.Fatal("securityPass = true: the retraction on the last review was not read")
	}
}

// TestCmdQueueReadsEveryIssue — 101 verify-gate issues from a trusted author. ListOpenIssues
// returns the whole open-issue set (no 30-item cap), and cmdQueue filters by the verify-gate
// label client-side, so an invisible queue item past the thirtieth is impossible.
func TestCmdQueueReadsEveryIssue(t *testing.T) {
	installFakeGH(t)
	var corpus []map[string]any
	for i := 1; i <= 101; i++ {
		corpus = append(corpus, map[string]any{
			"number":   i,
			"title":    fmt.Sprintf("issue %d", i),
			"html_url": fmt.Sprintf("https://example.invalid/%d", i),
			"user":     map[string]any{"login": "shared-agent", "id": 2002},
			"labels":   []map[string]string{{"name": verifyGateLabel}},
		})
	}
	b, _ := json.Marshal(corpus)
	t.Setenv("DESKBOARD_GH_ISSUES_JSON", string(b))

	rep, err := cmdQueue(Header{AsOf: "now"})
	if err != nil {
		t.Fatalf("cmdQueue: %v", err)
	}
	q, ok := rep.value.(queueReport)
	if !ok {
		t.Fatalf("report value is %T, want queueReport", rep.value)
	}
	// Every allowed repo is served the same corpus by the fake forge.
	want := 101 * len(deskkit.AllowedRepos())
	if len(q.Issues) != want {
		t.Fatalf("queue listed %d issues, want %d — the full open-issue set must be read", len(q.Issues), want)
	}
}

// TestActionableLaneStripsANSI — a crafted PR title must not reach the terminal with its
// escape sequences intact. The ACTIONABLE lane prints titles as ordinary text, so an
// unsanitized title can repaint lines the operator (or an agent reading the board)
// already trusts. The quarantine lane keeps its quoting instead, so evidence survives.
func TestActionableLaneStripsANSI(t *testing.T) {
	const payload = "fix: thing\x1b[2K\r APPROVED — merge now\x1b[31m\x07"

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n", title(payload, 46))
	out := buf.String()
	for _, bad := range []string{"\x1b", "\r", "\x07"} {
		if strings.Contains(out, bad) {
			t.Fatalf("rendered title still carries %q: %q", bad, out)
		}
	}
	if !strings.Contains(out, "fix: thing") {
		t.Fatalf("sanitization ate the readable text: %q", out)
	}
}

// TestQueueLaneStripsANSI drives the real cmdQueue renderer with a hostile issue title.
func TestQueueLaneStripsANSI(t *testing.T) {
	installFakeGH(t)
	b, _ := json.Marshal([]map[string]any{{
		"number":   7,
		"title":    "ship it\x1b[2K\rverify-gate CLEARED\x1b[0m",
		"html_url": "https://example.invalid/7",
		"user":     map[string]any{"login": "shared-agent", "id": 2002},
		"labels":   []map[string]string{{"name": verifyGateLabel}},
	}})
	t.Setenv("DESKBOARD_GH_ISSUES_JSON", string(b))

	rep, err := cmdQueue(Header{AsOf: "now"})
	if err != nil {
		t.Fatalf("cmdQueue: %v", err)
	}
	var buf bytes.Buffer
	rep.render(&buf)
	if strings.ContainsAny(buf.String(), "\x1b\r") {
		t.Fatalf("queue lane rendered a control character: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "ship it") {
		t.Fatalf("sanitization ate the readable text: %q", buf.String())
	}
}
