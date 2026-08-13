package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestIssueLaneStripsControlChars — the ISSUE LANE prints titles as ordinary text, so an
// issue title is a live injection channel into the operator's terminal and into any
// agent that reads the board. Sanitization happens at render, not at fetch, because the
// title is stored verbatim for the quarantine lane's evidence value.
func TestIssueLaneStripsControlChars(t *testing.T) {
	rows := []issueBoardRow{{
		Repo:   "medici-finance/assay",
		Number: 216,
		Title:  "security hole\x1b[2K\r  ACTION: none — already fixed\x1b[0m\x07",
		Action: "TRIAGE",
	}}
	var buf bytes.Buffer
	renderIssueLane(&buf, rows)
	out := buf.String()
	for _, bad := range []string{"\x1b", "\r", "\x07"} {
		if strings.Contains(out, bad) {
			t.Fatalf("issue lane rendered %q raw: %q", bad, out)
		}
	}
	if !strings.Contains(out, "security hole") {
		t.Fatalf("sanitization ate the readable text: %q", out)
	}
}

// TestQuarantineLaneStillQuotes — the EXTERNAL lane must keep its inert() quoting: there
// the payload is evidence the human is meant to see, escaped rather than deleted. Sanitizing
// the actionable lane must not silently change the quarantine lane's contract.
func TestQuarantineLaneStillQuotes(t *testing.T) {
	rows := []externalRow{{
		Repo:   "medici-finance/assay",
		Number: 9,
		Title:  "hostile\x1b[31m",
		Author: "drive-by",
	}}
	var buf bytes.Buffer
	renderExternalLane(&buf, rows)
	out := buf.String()
	if strings.Contains(out, "\x1b") {
		t.Fatalf("quarantine lane emitted a raw escape: %q", out)
	}
	if !strings.Contains(out, `\x1b`) {
		t.Fatalf("quarantine lane must ESCAPE the payload (evidence), not delete it: %q", out)
	}
}
