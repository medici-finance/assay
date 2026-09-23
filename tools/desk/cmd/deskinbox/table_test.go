package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCleanTextStripsControlBytesOnly(t *testing.T) {
	in := "hi\x1bthere\x07!"
	want := "hithere!"
	if got := cleanText(in); got != want {
		t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
	}
	// Ordinary printable text (including non-ASCII) survives untouched.
	if got := cleanText("héllo · wörld"); got != "héllo · wörld" {
		t.Errorf("cleanText must not touch non-control runes, got %q", got)
	}
}

func TestTruncateTitle(t *testing.T) {
	short := "a short title"
	if got := truncateTitle(short); got != short {
		t.Errorf("short title must pass through unchanged, got %q", got)
	}
	long := strings.Repeat("x", 80)
	got := truncateTitle(long)
	want := strings.Repeat("x", 57) + "..."
	if got != want {
		t.Errorf("truncateTitle(80 x's) = %q, want %q", got, want)
	}
}

func TestAgeOf(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if got := ageOf("2026-09-10T00:00:00Z", now); got != "12d" {
		t.Errorf("ageOf = %q, want 12d", got)
	}
	// Unparseable createdAt falls back to the raw string, matching the oracle's own
	// fallback when neither GNU nor BSD `date` can parse it.
	if got := ageOf("not-a-date", now); got != "not-a-date" {
		t.Errorf("ageOf(unparseable) = %q, want the raw string back", got)
	}
}

func TestRenderTableMarksRankZeroAndFormatsColumns(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	items := []item{
		{Repo: "example-org/example-repo", Number: 1, Title: "urgent one", URL: "https://example.invalid/1",
			CreatedAt: "2026-09-01T00:00:00Z", Labels: []string{"urgent"}, Rank: 0},
		{Repo: "example-org/example-repo", Number: 2, Title: "a question", URL: "https://example.invalid/2",
			CreatedAt: "2026-09-01T00:00:00Z", Labels: []string{"question"}, Rank: 2},
	}
	var buf bytes.Buffer
	renderTable(&buf, items, now)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 rows, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "**") {
		t.Errorf("rank-0 row must be marked with **, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("non-rank-0 row must be marked with two spaces, got %q", lines[1])
	}
	if !strings.Contains(lines[0], "#1") || !strings.Contains(lines[0], "urgent one") {
		t.Errorf("row missing expected fields: %q", lines[0])
	}
}
