package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestIssueTitleShortUnchanged(t *testing.T) {
	got := issueTitle("needs-decision: ", "dc/06", "Short title")
	want := "needs-decision: dc/06 — Short title"
	if got != want {
		t.Errorf("issueTitle = %q, want %q", got, want)
	}
}

func TestIssueTitleTruncatedAtGitHubLimit(t *testing.T) {
	long := strings.Repeat("é", 300) // multi-byte runes: cap must count runes, not bytes
	got := issueTitle("verify-gate: ", "stream/99", long)
	if n := utf8.RuneCountInString(got); n != maxIssueTitleLen {
		t.Errorf("truncated title is %d runes, want exactly %d", n, maxIssueTitleLen)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated title must end with an ellipsis, got %q", got[len(got)-12:])
	}
	if !strings.HasPrefix(got, "verify-gate: stream/99 — ") {
		t.Errorf("prefix lost in truncation: %q", got[:40])
	}
}

func TestIssueTitleExactlyAtLimitUnchanged(t *testing.T) {
	prefix := "needs-decision: "
	brief := "dc/06"
	pad := maxIssueTitleLen - utf8.RuneCountInString(prefix+brief+" — ")
	title := strings.Repeat("x", pad)
	got := issueTitle(prefix, brief, title)
	if utf8.RuneCountInString(got) != maxIssueTitleLen || strings.HasSuffix(got, "…") {
		t.Errorf("a title exactly at the limit must pass through untruncated; got %d runes", utf8.RuneCountInString(got))
	}
}
