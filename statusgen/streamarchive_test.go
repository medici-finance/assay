package main

import (
	"strings"
	"testing"
)

// realBrief builds a README-table (non-placeholder) brief row with the given
// status.
func realBrief(num, status string) Brief {
	return Brief{Num: num, Status: status}
}

// placeholderBrief builds a synthetic placeholder brief row (schema
// placeholder-v1) with the given status, mirroring what attachPlaceholders
// appends to s.Briefs.
func placeholderBrief(num, status string) Brief {
	return Brief{Num: num, Status: status, Schema: "placeholder-v1"}
}

// TestStreamArchiveReadyFiresWhenAllBriefsDone verifies a stream whose every
// README-table brief is done surfaces exactly one archive-candidate NOTICE that
// carries the stream name and the brief count.
func TestStreamArchiveReadyFiresWhenAllBriefsDone(t *testing.T) {
	s := &Stream{Name: "ground-truth", Briefs: []Brief{
		realBrief("01", "done"),
		realBrief("02", "done"),
		realBrief("03", "done"),
	}}
	got := streamArchiveReadyNotices([]*Stream{s})
	if len(got) != 1 {
		t.Fatalf("want 1 notice, got %d: %v", len(got), got)
	}
	msg := got[0]
	// The remediation must name the archival act statusgen actually enforces
	// (checks.go: a `done` stream must live under docs/archive/), naming the
	// concrete destination, and must NOT prescribe the per-placeholder
	// <stream>/done/ sweep, which reds the lint and never clears the notice.
	for _, want := range []string{
		"ground-truth", "all 3 briefs done", "archive candidate", "never auto-flipped",
		"status: done", "docs/archive/", "docs/streams/ground-truth",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("notice missing %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "into done/") || strings.Contains(msg, "briefs into") {
		t.Errorf("notice must not prescribe the per-placeholder done/ sweep: %s", msg)
	}
}

// TestStreamArchiveReadyDetectOnly asserts the surface is notice-only: it never
// mutates the streams (no auto-flip) — the archival stays a desk act.
func TestStreamArchiveReadyDetectOnly(t *testing.T) {
	s := &Stream{Name: "s", Status: "active", Briefs: []Brief{realBrief("01", "done")}}
	_ = streamArchiveReadyNotices([]*Stream{s})
	if s.Status != "active" {
		t.Errorf("detector mutated stream status to %q — must detect, not auto-flip", s.Status)
	}
	if got := s.Briefs[0].Status; got != "done" {
		t.Errorf("detector mutated brief status to %q — must not touch state", got)
	}
}

// TestStreamArchiveReadySuppressedByOpenWork verifies any non-done brief holds
// the whole stream off the archive-ready list, whatever the lifecycle stage.
func TestStreamArchiveReadySuppressedByOpenWork(t *testing.T) {
	for _, openStatus := range []string{"todo", "in-progress", "implemented", "verified", "blocked"} {
		s := &Stream{Name: "s", Briefs: []Brief{
			realBrief("01", "done"),
			realBrief("02", openStatus),
		}}
		got := streamArchiveReadyNotices([]*Stream{s})
		if len(got) != 0 {
			t.Errorf("status %q: want no notice while work is open, got %v", openStatus, got)
		}
	}
}

// TestStreamArchiveReadySingularGrammar verifies the notice reads "1 brief"
// (singular) for a one-brief stream, not "1 briefs".
func TestStreamArchiveReadySingularGrammar(t *testing.T) {
	s := &Stream{Name: "solo", Briefs: []Brief{realBrief("01", "done")}}
	got := streamArchiveReadyNotices([]*Stream{s})
	if len(got) != 1 {
		t.Fatalf("want 1 notice, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "all 1 brief done") || strings.Contains(got[0], "all 1 briefs done") {
		t.Errorf("want singular \"1 brief\", got: %s", got[0])
	}
}

// TestStreamArchiveReadySkipsEmptyStream verifies a stream with no briefs is not
// an archive candidate (there is nothing to archive).
func TestStreamArchiveReadySkipsEmptyStream(t *testing.T) {
	s := &Stream{Name: "empty"}
	if got := streamArchiveReadyNotices([]*Stream{s}); len(got) != 0 {
		t.Errorf("empty stream should not fire, got %v", got)
	}
}

// TestStreamArchiveReadySkipsPlaceholderOnlyStream verifies a stream carrying
// only done placeholder rows (e.g. the perpetual issue-loop intake stream) is
// never a stream-archival candidate: it has no README-table brief to archive.
func TestStreamArchiveReadySkipsPlaceholderOnlyStream(t *testing.T) {
	s := &Stream{Name: "issue-loop", Briefs: []Brief{
		placeholderBrief("issue-300", "done"),
		placeholderBrief("issue-301", "done"),
	}}
	if got := streamArchiveReadyNotices([]*Stream{s}); len(got) != 0 {
		t.Errorf("placeholder-only stream should not fire, got %v", got)
	}
}

// TestStreamArchiveReadyOpenPlaceholderBlocks verifies an open placeholder issue
// holds the stream off the list even when every README-table brief is done — the
// completeness check spans the whole brief set, real rows and placeholders alike.
func TestStreamArchiveReadyOpenPlaceholderBlocks(t *testing.T) {
	s := &Stream{Name: "s", Briefs: []Brief{
		realBrief("01", "done"),
		realBrief("02", "done"),
		placeholderBrief("issue-500", "todo"),
	}}
	if got := streamArchiveReadyNotices([]*Stream{s}); len(got) != 0 {
		t.Errorf("an open placeholder must block archive-ready, got %v", got)
	}
}

// TestStreamArchiveReadyDonePlaceholderCounts verifies a fully-complete stream
// with a done placeholder still fires, and the reported N counts the real
// README-table briefs (not the placeholder rows).
func TestStreamArchiveReadyDonePlaceholderCounts(t *testing.T) {
	s := &Stream{Name: "s", Briefs: []Brief{
		realBrief("01", "done"),
		realBrief("02", "done"),
		placeholderBrief("issue-500", "done"),
	}}
	got := streamArchiveReadyNotices([]*Stream{s})
	if len(got) != 1 {
		t.Fatalf("want 1 notice, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "all 2 briefs done") {
		t.Errorf("N should count the 2 real briefs, got: %s", got[0])
	}
}

// TestStreamArchiveReadyMultiStreamSorted verifies the notices are deterministic
// (sorted) and only the fully-done streams appear.
func TestStreamArchiveReadyMultiStreamSorted(t *testing.T) {
	streams := []*Stream{
		{Name: "zebra", Briefs: []Brief{realBrief("01", "done")}},
		{Name: "alpha", Briefs: []Brief{realBrief("01", "done")}},
		{Name: "busy", Briefs: []Brief{realBrief("01", "in-progress")}},
	}
	got := streamArchiveReadyNotices(streams)
	if len(got) != 2 {
		t.Fatalf("want 2 notices, got %d: %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "stream alpha:") || !strings.HasPrefix(got[1], "stream zebra:") {
		t.Errorf("notices not sorted: %v", got)
	}
}
