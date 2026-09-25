package deskkit

import (
	"strings"
	"testing"
)

// TestParseDecidedItemsTableTests is the table test the brief's Task item 1 asks for: empty
// list, missing field, round-trip.
func TestParseDecidedItemsTableTests(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
		want    []DecidedItem
	}{
		{
			name:    "empty list refuses",
			in:      "",
			wantErr: true,
		},
		{
			name:    "whitespace-only refuses",
			in:      "   \n\n  \n",
			wantErr: true,
		},
		{
			name: "missing cost field refuses",
			in: "1. decision: ship the one-line patch\n" +
				"   alternative: wait for a ruling\n",
			wantErr: true,
		},
		{
			name: "missing alternative field refuses",
			in: "1. decision: ship the one-line patch\n" +
				"   cost: one revert\n",
			wantErr: true,
		},
		{
			name: "one well-formed item round-trips",
			in: "1. decision: ship the one-line patch\n" +
				"   alternative: ask first\n" +
				"   cost: one revert\n",
			want: []DecidedItem{
				{Decision: "ship the one-line patch", Alternative: "ask first", Cost: "one revert"},
			},
		},
		{
			name: "two well-formed items round-trip in order",
			in: "1. decision: first choice\n" +
				"   alternative: none workable — no other option existed\n" +
				"   cost: decline this PR\n" +
				"2. decision: second choice\n" +
				"   alternative: the other option\n" +
				"   cost: one revert\n",
			want: []DecidedItem{
				{Decision: "first choice", Alternative: "none workable — no other option existed", Cost: "decline this PR"},
				{Decision: "second choice", Alternative: "the other option", Cost: "one revert"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDecidedItems([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDecidedItems(%q) = %v, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDecidedItems(%q) unexpected error: %v", c.in, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("ParseDecidedItems(%q) = %d items, want %d: %+v", c.in, len(got), len(c.want), got)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("item %d = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestRenderDecidedBlockRoundTrips proves RenderDecidedBlock's output parses back through
// ParseDeskDecidedBlockInBody to the same items — the full round-trip a create+edit cycle
// depends on.
func TestRenderDecidedBlockRoundTrips(t *testing.T) {
	items := []DecidedItem{
		{Decision: "ship the one-line patch", Alternative: "ask first", Cost: "one revert"},
		{Decision: "second thing", Alternative: "none workable — no alternative existed", Cost: "decline this PR"},
	}
	block := RenderDecidedBlock(items)
	if !strings.HasPrefix(block, DeskDecidedHeading) {
		t.Fatalf("rendered block does not start with the fixed heading: %q", block)
	}
	if !strings.Contains(block, DeskDecidedMarker) {
		t.Fatalf("rendered block is missing the marker: %q", block)
	}
	body := "Some PR description.\n\n" + block
	got, found, err := ParseDeskDecidedBlockInBody(body)
	if err != nil {
		t.Fatalf("ParseDeskDecidedBlockInBody: %v", err)
	}
	if !found {
		t.Fatal("ParseDeskDecidedBlockInBody did not find the rendered block")
	}
	if len(got) != len(items) {
		t.Fatalf("round-trip got %d items, want %d: %+v", len(got), len(items), got)
	}
	for i := range items {
		if got[i] != items[i] {
			t.Errorf("round-trip item %d = %+v, want %+v", i, got[i], items[i])
		}
	}
}

func TestHasDeskDecidedHeading(t *testing.T) {
	if HasDeskDecidedHeading("no heading here") {
		t.Error("false positive on a body with no heading")
	}
	if !HasDeskDecidedHeading("intro\n\n## Desk-decided\n\nsome hand-written text\n") {
		t.Error("false negative on a body carrying the heading")
	}
	if HasDeskDecidedHeading("### Not Desk-decided really\n") {
		t.Error("false positive: a heading that merely CONTAINS the text must not match")
	}
}

func TestReplaceOrAppendDeskDecidedBlockReplacesInPlace(t *testing.T) {
	items1 := []DecidedItem{{Decision: "first", Alternative: "alt", Cost: "cost"}}
	items2 := []DecidedItem{{Decision: "second", Alternative: "alt2", Cost: "cost2"}}
	body := "Intro text.\n\n" + RenderDecidedBlock(items1) + "\n## Another Section\n\nmore text\n"

	updated := ReplaceOrAppendDeskDecidedBlock(body, RenderDecidedBlock(items2))

	if strings.Contains(updated, "first") {
		t.Errorf("old block content survived the replace: %q", updated)
	}
	if !strings.Contains(updated, "second") {
		t.Errorf("new block content is missing: %q", updated)
	}
	if !strings.Contains(updated, "## Another Section") || !strings.Contains(updated, "more text") {
		t.Errorf("content after the block was not preserved: %q", updated)
	}
	if !strings.Contains(updated, "Intro text.") {
		t.Errorf("content before the block was not preserved: %q", updated)
	}
	// Exactly one heading occurrence — replace, not append-beside.
	if n := strings.Count(updated, DeskDecidedHeading); n != 1 {
		t.Errorf("got %d occurrences of the heading, want 1: %q", n, updated)
	}
}

func TestReplaceOrAppendDeskDecidedBlockAppendsWhenAbsent(t *testing.T) {
	body := "Intro text with no block.\n"
	block := RenderDecidedBlock([]DecidedItem{{Decision: "d", Alternative: "a", Cost: "c"}})
	got := ReplaceOrAppendDeskDecidedBlock(body, block)
	if !strings.Contains(got, "Intro text with no block.") {
		t.Errorf("original body content lost: %q", got)
	}
	if !strings.Contains(got, DeskDecidedHeading) {
		t.Errorf("block was not appended: %q", got)
	}
}

func TestParseDeskDecidedBlockInBodyNotFound(t *testing.T) {
	items, found, err := ParseDeskDecidedBlockInBody("nothing here")
	if found || err != nil || items != nil {
		t.Fatalf("got (%v, %v, %v), want (nil, false, nil)", items, found, err)
	}
}

func TestParseDeskDecidedBlockInBodyMalformedMissingMarker(t *testing.T) {
	body := "## Desk-decided\n\n1. decision: x\n   alternative: y\n   cost: z\n"
	items, found, err := ParseDeskDecidedBlockInBody(body)
	if !found {
		t.Fatal("expected found=true for a heading with no marker")
	}
	if err == nil {
		t.Fatal("expected a malformed error when the marker is missing")
	}
	if items != nil {
		t.Errorf("malformed block should return nil items, got %v", items)
	}
}

func TestParseDeskDecidedBlockInBodyMalformedEmptyList(t *testing.T) {
	body := DeskDecidedHeading + "\n" + DeskDecidedMarker + "\n\nno items here\n"
	_, found, err := ParseDeskDecidedBlockInBody(body)
	if !found {
		t.Fatal("expected found=true")
	}
	if err == nil {
		t.Fatal("expected a malformed error for an empty list")
	}
}

func TestUndeclaredDeskDecisionLines(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{name: "absent", body: "Verdict: approve\n", want: nil},
		{
			name: "present, plain",
			body: "Verdict: approve\nUndeclared-desk-decision: chose a default for the retry backoff\n",
			want: []string{"chose a default for the retry backoff"},
		},
		{
			name: "present, emphasised",
			body: "**Undeclared-desk-decision: chose a default**\n",
			want: []string{"chose a default"},
		},
		{
			name: "fenced still counts (block direction)",
			body: "```\nUndeclared-desk-decision: inside a fence\n```\n",
			want: []string{"inside a fence"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UndeclaredDeskDecisionLines(c.body)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}
